package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	coreErrors "josex/web/modules/core/errors"
	importConfig "josex/web/modules/import/config"
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
// @Param        images         formData  file    false  "Optional ZIP of images referenced by the CSV (e.g. profile_picture)"
// @Success      200  {object}  importModels.ValidateResponse
// @Failure      400  {object}  coreErrors.ErrorResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /import/validate [post]
// @Security     ApiKeyAuth
func (ctrl *ImportController) Validate(c *gin.Context) {
	parsed, ok := ctrl.readUpload(c)
	if !ok {
		return
	}

	result, err := ctrl.importService.Validate(ctrl.buildContext(c, parsed), parsed.resource, parsed.data)
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
// @Param        images         formData  file    false  "Optional ZIP of images referenced by the CSV (e.g. profile_picture)"
// @Success      200  {object}  importModels.ImportResponse
// @Failure      400  {object}  coreErrors.ErrorResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /import [post]
// @Security     ApiKeyAuth
func (ctrl *ImportController) Import(c *gin.Context) {
	parsed, ok := ctrl.readUpload(c)
	if !ok {
		return
	}

	result, err := ctrl.importService.Import(ctrl.buildContext(c, parsed), parsed.resource, parsed.data)
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

// Schema godoc
// @Summary      Column schema for a resource
// @Description  Returns the resource's columns with their type, required flag and optional validation pattern, so a client can render and check an editor per column without knowing the resource.
// @Tags         Import
// @Produce      json
// @Param        Authorization  header    string  true   "Bearer Token"
// @Param        X-Tenant-Slug  header    string  true   "Tenant slug"
// @Param        resource       query     string  true   "Resource key (e.g. users)"
// @Success      200  {object}  importModels.SchemaResponse
// @Failure      400  {object}  coreErrors.ErrorResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Router       /import/schema [get]
// @Security     ApiKeyAuth
func (ctrl *ImportController) Schema(c *gin.Context) {
	result, err := ctrl.importService.Schema(c.Query("resource"))
	if err != nil {
		ctrl.mapError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// upload is one parsed multipart request: the CSV bytes, the optional resource
// key, the parsed options, and the images inflated from the optional archive.
type upload struct {
	data     []byte
	resource string
	options  importModels.ImportOptions
	images   importModels.ImportImages
}

// readUpload reads and size-checks the CSV file, the optional resource and
// options fields, and the optional companion image archive. On any failure it
// writes the error response and returns ok=false.
func (ctrl *ImportController) readUpload(c *gin.Context) (upload, bool) {
	maxBytes := ctrl.maxFileSizeBytes()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileRequired))
		return upload{}, false
	}
	if fileHeader.Size > maxBytes {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileTooLarge))
		return upload{}, false
	}

	data, err := readMultipart(fileHeader, maxBytes)
	if err != nil {
		ctrl.mapError(c, err)
		return upload{}, false
	}

	var options importModels.ImportOptions
	if optionsRaw := c.PostForm("options"); optionsRaw != "" {
		if err := json.Unmarshal([]byte(optionsRaw), &options); err != nil {
			c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportOptionsInvalid))
			return upload{}, false
		}
	}

	images, ok := ctrl.readImages(c)
	if !ok {
		return upload{}, false
	}

	return upload{data: data, resource: c.PostForm("resource"), options: options, images: images}, true
}

// readImages inflates the optional `images` archive. Absent is not an error —
// the CSV alone is a valid upload.
func (ctrl *ImportController) readImages(c *gin.Context) (importModels.ImportImages, bool) {
	archiveHeader, err := c.FormFile("images")
	if err != nil {
		return nil, true
	}

	maxBytes := int64(importConfig.MaxImagesZipMB()) << 20
	if archiveHeader.Size > maxBytes {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesTooLarge))
		return nil, false
	}

	archive, err := readMultipart(archiveHeader, maxBytes)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesInvalid))
		return nil, false
	}

	images, err := ctrl.importService.ExtractImages(archive)
	if err != nil {
		ctrl.mapError(c, err)
		return nil, false
	}

	return images, true
}

// readMultipart reads an uploaded part through a limited reader so a lying
// Content-Length cannot get past the cap.
func readMultipart(header *multipart.FileHeader, maxBytes int64) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, importErrors.ErrFileInvalid
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, importErrors.ErrFileInvalid
	}
	if int64(len(data)) > maxBytes {
		return nil, importErrors.ErrFileTooLarge
	}

	return data, nil
}

// buildContext assembles the import scope from tenancy/auth middleware context
// plus the parsed upload.
func (ctrl *ImportController) buildContext(c *gin.Context, parsed upload) importModels.ImportContext {
	importCtx := importModels.ImportContext{
		Lang:    c.GetString("lang"),
		Options: parsed.options,
		Images:  parsed.images,
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
	case errors.Is(err, importErrors.ErrFileNoRows):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileNoRows))
	case errors.Is(err, importErrors.ErrFileInvalid):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileInvalid))
	case errors.Is(err, importErrors.ErrFileTooLarge):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileTooLarge))
	case errors.Is(err, importErrors.ErrImagesInvalid):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesInvalid))
	case errors.Is(err, importErrors.ErrImagesTooLarge):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesTooLarge))
	case errors.Is(err, importErrors.ErrImagesTooManyFiles):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesTooManyFiles))
	case errors.Is(err, importErrors.ErrImagesEntryTooLarge):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesEntryTooLarge))
	case errors.Is(err, importErrors.ErrImagesEntryFormat):
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesEntryFormat))
	default:
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
}
