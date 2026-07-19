package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	coreErrors "josex/web/modules/core/errors"
	importErrors "josex/web/modules/import/errors"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
)

type ImportController struct {
	importService importInterfaces.ImportService
}

func NewImportController(importService importInterfaces.ImportService) *ImportController {
	return &ImportController{importService: importService}
}

// maxFileSizeBytes is the configured upload cap in bytes.
func (ctrl *ImportController) maxFileSizeBytes() int64 {
	return int64(ctrl.importService.Limits().MaxFileSizeMB) << 20
}

// Validate godoc
// @Summary      Dry-run validate an uploaded CSV
// @Description  Detects the resource from the CSV header (or the explicit resource field), validates every row, and returns per-row valid|invalid|duplicate feedback keyed by CSV line number. Writes nothing.
// @Tags         Import
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization  header    string  true   "Bearer Token"
// @Param        X-Tenant-Slug  header    string  true   "Tenant slug"
// @Param        file           formData  file    true   "CSV file (max 5 MB, 500 rows)"
// @Param        resource       formData  string  false  "Resource key (auto-detected from the header when omitted)"
// @Param        options        formData  string  false  "Resource-specific options as a JSON object"
// @Success      200  {object}  importModels.ValidateResponse
// @Failure      400  {object}  coreErrors.ErrorResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /import/validate [post]
// @Security     ApiKeyAuth
func (ctrl *ImportController) Validate(c *gin.Context) {
	data, resource, options, ok := ctrl.readUpload(c)
	if !ok {
		return
	}

	result, err := ctrl.importService.Validate(ctrl.buildContext(c, options), resource, data)
	if err != nil {
		ctrl.mapError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// Import godoc
// @Summary      Process an uploaded CSV
// @Description  Detects the resource, creates each valid row, and returns per-row created|failed|skipped outcomes plus a persisted run summary (who/when/counts/warnings).
// @Tags         Import
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization  header    string  true   "Bearer Token"
// @Param        X-Tenant-Slug  header    string  true   "Tenant slug"
// @Param        file           formData  file    true   "CSV file (max 5 MB, 500 rows)"
// @Param        resource       formData  string  false  "Resource key (auto-detected from the header when omitted)"
// @Param        options        formData  string  false  "Resource-specific options as a JSON object"
// @Success      200  {object}  importModels.ImportResponse
// @Failure      400  {object}  coreErrors.ErrorResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /import [post]
// @Security     ApiKeyAuth
func (ctrl *ImportController) Import(c *gin.Context) {
	data, resource, options, ok := ctrl.readUpload(c)
	if !ok {
		return
	}

	result, err := ctrl.importService.Import(ctrl.buildContext(c, options), resource, data)
	if err != nil {
		ctrl.mapError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// Meta godoc
// @Summary      Import limits
// @Description  Returns the configured upload constraints (max file size, max rows) so the UI hint stays in sync with the server.
// @Tags         Import
// @Produce      json
// @Param        Authorization  header    string  true  "Bearer Token"
// @Param        X-Tenant-Slug  header    string  true  "Tenant slug"
// @Success      200  {object}  importModels.ImportLimits
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /import/meta [get]
// @Security     ApiKeyAuth
func (ctrl *ImportController) Meta(c *gin.Context) {
	c.JSON(http.StatusOK, ctrl.importService.Limits())
}

// Template godoc
// @Summary      Download a CSV template
// @Description  Returns a CSV containing just the header row for a resource, built from its descriptor's columns.
// @Tags         Import
// @Produce      text/csv
// @Param        Authorization  header    string  true   "Bearer Token"
// @Param        X-Tenant-Slug  header    string  true   "Tenant slug"
// @Param        resource       query     string  true   "Resource key (e.g. users)"
// @Success      200  {file}    string
// @Failure      400  {object}  coreErrors.ErrorResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /import/template [get]
// @Security     ApiKeyAuth
func (ctrl *ImportController) Template(c *gin.Context) {
	resource := c.Query("resource")
	data, err := ctrl.importService.Template(resource)
	if err != nil {
		ctrl.mapError(c, err)
		return
	}

	filename := fmt.Sprintf("import-template-%s.csv", resource)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

// readUpload reads and size-checks the CSV file plus the optional resource and
// options fields. On any failure it writes the error response and returns ok=false.
func (ctrl *ImportController) readUpload(c *gin.Context) ([]byte, string, importModels.ImportOptions, bool) {
	maxBytes := ctrl.maxFileSizeBytes()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileRequired))
		return nil, "", nil, false
	}
	if fileHeader.Size > maxBytes {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileTooLarge))
		return nil, "", nil, false
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileInvalid))
		return nil, "", nil, false
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileInvalid))
		return nil, "", nil, false
	}
	if int64(len(data)) > maxBytes {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileTooLarge))
		return nil, "", nil, false
	}

	var options importModels.ImportOptions
	if optionsRaw := c.PostForm("options"); optionsRaw != "" {
		if err := json.Unmarshal([]byte(optionsRaw), &options); err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportOptionsInvalid))
			return nil, "", nil, false
		}
	}

	return data, c.PostForm("resource"), options, true
}

// buildContext assembles the import scope from tenancy/auth middleware context.
func (ctrl *ImportController) buildContext(c *gin.Context, options importModels.ImportOptions) importModels.ImportContext {
	importCtx := importModels.ImportContext{
		Lang:    c.GetString("lang"),
		Options: options,
	}
	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tid, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			importCtx.TenantID = tid
		}
	}
	if userIDStr, ok := c.Get("user_id"); ok {
		if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
			importCtx.PerformedBy = &uid
		}
	}
	return importCtx
}

// mapError translates a service sentinel error to the matching HTTP status and code.
func (ctrl *ImportController) mapError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, importErrors.ErrUnknownFormat):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportDetectUnknownFormat))
	case errors.Is(err, importErrors.ErrTooManyRows):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportTooManyRows))
	case errors.Is(err, importErrors.ErrFileEmpty):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileEmpty))
	case errors.Is(err, importErrors.ErrFileInvalid):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileInvalid))
	default:
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
}
