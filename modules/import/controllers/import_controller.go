package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	importConfig "josex/web/modules/import/config"
	importErrors "josex/web/modules/import/errors"
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
	importServices "josex/web/modules/import/services"
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

// Validate is POST /validate: Dry-run validate an uploaded CSV.
func (ctrl *ImportController) Validate(ctx context.Context, _ *UploadInput) (*api.Out[*importModels.ValidateResponse], error) {
	c := api.Gin(ctx)
	parsed, err := ctrl.readUpload(c)
	if err != nil {
		return nil, err
	}

	result, err := ctrl.importService.Validate(ctrl.buildContext(c, parsed), parsed.request)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	return api.OK(result)
}

// Import is POST : Process an uploaded CSV.
func (ctrl *ImportController) Import(ctx context.Context, _ *UploadInput) (*api.Out[*importModels.ImportResponse], error) {
	c := api.Gin(ctx)
	parsed, err := ctrl.readUpload(c)
	if err != nil {
		return nil, err
	}

	result, err := ctrl.importService.Import(ctrl.buildContext(c, parsed), parsed.request)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	return api.OK(result)
}

// Detect is POST /detect: Detect a CSV's resource.
func (ctrl *ImportController) Detect(ctx context.Context, _ *UploadInput) (*api.Out[*importModels.DetectResponse], error) {
	c := api.Gin(ctx)
	parsed, err := ctrl.readUpload(c)
	if err != nil {
		return nil, err
	}

	result, err := ctrl.importService.Detect(parsed.request.Target, parsed.request.Data, parsed.request.FileName)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	return api.OK(result)
}

// Meta is GET /meta: Import limits.
func (ctrl *ImportController) Meta(ctx context.Context, _ *api.None) (*api.Out[importModels.ImportLimits], error) {
	return api.OK(ctrl.importService.Limits())
}

// UploadForm is the multipart form Validate, Import and Detect read.
type UploadForm struct {
	File       string `json:"file" format:"binary" doc:"The file: Excel (.xlsx) or CSV"`
	Resource   string `json:"resource,omitempty" doc:"Resource key; detected from the header when empty"`
	Scope      string `json:"scope,omitempty" doc:"Comma-separated resource or module keys detection may pick from"`
	Options    string `json:"options,omitempty" doc:"JSON-encoded importModels.ImportOptions"`
	Images     string `json:"images,omitempty" format:"binary" doc:"Optional zip of the rows' images"`
	Mapping    string `json:"mapping,omitempty" doc:"JSON {heading as written: field key or null}; null ignores the column"`
	OnExisting string `json:"on_existing,omitempty" enum:"skip,update" doc:"What to do with a row whose record exists; update only for an updatable resource"`
	Remember   string `json:"remember,omitempty" enum:"true,false" doc:"Remember this file shape's mapping (import only)"`
	Edits      string `json:"edits,omitempty" doc:"JSON {line: {field key: value} or null}: cells fixed on the review step, null drops the row"`
}

// UploadInput is the request of the routes that take an uploaded CSV.
type UploadInput struct {
	Form api.Form[UploadForm]
}

// ResourcesInput is Resources's request.
type ResourcesInput struct {
	Scope string `query:"scope"`
}

// Resources is GET /resources: Resources a scope reaches.
func (ctrl *ImportController) Resources(ctx context.Context, in *ResourcesInput) (*api.Out[[]importModels.ImportResourceInfo], error) {
	c := api.Gin(ctx)
	return api.OK(ctrl.importService.Resources(c.Request.Context(), tenantOf(c), c.GetString("lang"), parseScope(in.Scope)))
}

// MappingInput is Mapping's request.
type MappingInput struct {
	Resource string `query:"resource"`
	Headers  string `query:"headers" doc:"JSON array of the file's headings, as written"`
}

// Mapping is GET /mapping: the mapping remembered for a file shape.
func (ctrl *ImportController) Mapping(ctx context.Context, in *MappingInput) (*api.Out[*importModels.MappingResponse], error) {
	c := api.Gin(ctx)
	var headers []string
	if err := json.Unmarshal([]byte(in.Headers), &headers); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportMappingInvalid))
	}
	result, err := ctrl.importService.Mapping(c.Request.Context(), tenantOf(c), in.Resource, headers)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}
	return api.OK(result)
}

// RunsInput is Runs's request.
type RunsInput struct {
	Resource string `query:"resource"`
	Page     int    `query:"page" minimum:"1" default:"1"`
	PageSize int    `query:"page_size" minimum:"1" maximum:"100" default:"20"`
}

// Runs is GET /runs: the tenant's import history, newest first.
func (ctrl *ImportController) Runs(ctx context.Context, in *RunsInput) (*api.Out[*importModels.RunsResponse], error) {
	c := api.Gin(ctx)
	query := importModels.RunsQuery{Resource: in.Resource, Page: max(in.Page, 1), PageSize: in.PageSize}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	result, err := ctrl.importService.Runs(c.Request.Context(), tenantOf(c), query)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}
	return api.OK(result)
}

// parseScope splits the comma-separated scope field into its keys, dropping
// blanks, so `inventory`, `inventory,users` and an empty field all read as
// expected.
func parseScope(raw string) []string {
	var scope []string
	for _, entry := range strings.Split(raw, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			scope = append(scope, entry)
		}
	}
	return scope
}

// TemplateInput is Template's request.
type TemplateInput struct {
	Resource string `query:"resource"`
	Format   string `query:"format" enum:"csv,xlsx" doc:"csv (default) or xlsx"`
}

// Template answers a CSV with just the header row for a resource, built from its descriptor's columns.
func (ctrl *ImportController) Template(ctx context.Context, in *TemplateInput) (*api.File, error) {
	c := api.Gin(ctx)
	data, err := ctrl.importService.Template(in.Resource, in.Format)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	if in.Format == importServices.TemplateXLSX {
		return &api.File{
			ContentType:        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			ContentDisposition: "attachment; filename=" + fmt.Sprintf("import-template-%s.xlsx", in.Resource),
			Body:               data,
		}, nil
	}
	return &api.File{
		ContentType:        "text/csv; charset=utf-8",
		ContentDisposition: "attachment; filename=" + fmt.Sprintf("import-template-%s.csv", in.Resource),
		Body:               data,
	}, nil
}

// SchemaInput is Schema's request.
type SchemaInput struct {
	Resource string `query:"resource"`
}

// Schema is GET /schema: Column schema for a resource.
func (ctrl *ImportController) Schema(ctx context.Context, in *SchemaInput) (*api.Out[*importModels.SchemaResponse], error) {
	c := api.Gin(ctx)
	result, err := ctrl.importService.Schema(in.Resource, c.GetString("lang"))
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	return api.OK(result)
}

// upload is one parsed multipart request: the file and everything the engine
// reads with it, the parsed options, and the images inflated from the optional
// archive.
type upload struct {
	request importModels.ImportRequest
	options importModels.ImportOptions
	images  importModels.ImportImages
}

// readUpload reads and size-checks the CSV file, the optional resource and
// options fields, and the optional companion image archive. On any failure it
// writes the error response and returns ok=false.
func (ctrl *ImportController) readUpload(c *gin.Context) (upload, error) {
	maxBytes := ctrl.maxFileSizeBytes()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return upload{}, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileRequired))
	}
	if fileHeader.Size > maxBytes {
		return upload{}, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileTooLarge))
	}

	data, err := readMultipart(fileHeader, maxBytes)
	if err != nil {
		return upload{}, ctrl.mapError(c, err)
	}

	var options importModels.ImportOptions
	if optionsRaw := c.PostForm("options"); optionsRaw != "" {
		if err := json.Unmarshal([]byte(optionsRaw), &options); err != nil {
			return upload{}, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportOptionsInvalid))
		}
	}

	images, err := ctrl.readImages(c)
	if err != nil {
		return upload{}, err
	}

	request := importModels.ImportRequest{
		Target: importModels.ImportTarget{
			Resource: strings.TrimSpace(c.PostForm("resource")),
			Scope:    parseScope(c.PostForm("scope")),
		},
		Data:       data,
		FileName:   fileHeader.Filename,
		OnExisting: c.PostForm("on_existing"),
		Remember:   c.PostForm("remember") == "true",
	}
	if raw := c.PostForm("mapping"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &request.Mapping); err != nil {
			return upload{}, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportMappingInvalid))
		}
	}
	if raw := c.PostForm("edits"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &request.Edits); err != nil {
			return upload{}, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportEditsInvalid))
		}
	}
	return upload{request: request, options: options, images: images}, nil
}

// readImages inflates the optional `images` archive. Absent is not an error —
// the CSV alone is a valid upload.
func (ctrl *ImportController) readImages(c *gin.Context) (importModels.ImportImages, error) {
	archiveHeader, err := c.FormFile("images")
	if err != nil {
		return nil, nil
	}

	maxBytes := int64(importConfig.MaxImagesZipMB()) << 20
	if archiveHeader.Size > maxBytes {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesTooLarge))
	}

	archive, err := readMultipart(archiveHeader, maxBytes)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesInvalid))
	}

	images, err := ctrl.importService.ExtractImages(archive)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	return images, nil
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
		Ctx:     c.Request.Context(),
		Lang:    c.GetString("lang"),
		Options: parsed.options,
		Images:  parsed.images,
	}
	importCtx.TenantID = tenantOf(c)
	if userIDStr, ok := c.Get("user_id"); ok {
		if uid, err := uuid.Parse(userIDStr.(string)); err == nil {
			importCtx.PerformedBy = &uid
		}
	}
	return importCtx
}

// tenantOf is the request's tenant, or Nil outside one.
func tenantOf(c *gin.Context) uuid.UUID {
	if raw, ok := c.Get("tenant_id"); ok {
		if text, ok := raw.(string); ok {
			if id, err := uuid.Parse(text); err == nil {
				return id
			}
		}
	}
	return uuid.Nil
}

// mapError translates a service sentinel error to the matching HTTP status and code.
func (ctrl *ImportController) mapError(c *gin.Context, err error) error {
	switch {
	case errors.Is(err, importErrors.ErrUnknownFormat):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportDetectUnknownFormat))
	case errors.Is(err, importErrors.ErrOutOfScope):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportDetectOutOfScope))
	case errors.Is(err, importErrors.ErrTooManyRows):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportTooManyRows))
	case errors.Is(err, importErrors.ErrFileEmpty):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileEmpty))
	case errors.Is(err, importErrors.ErrFileNoRows):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileNoRows))
	case errors.Is(err, importErrors.ErrFileInvalid):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileInvalid))
	case errors.Is(err, importErrors.ErrFileTooLarge):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportFileTooLarge))
	case errors.Is(err, importErrors.ErrImagesInvalid):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesInvalid))
	case errors.Is(err, importErrors.ErrImagesTooLarge):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesTooLarge))
	case errors.Is(err, importErrors.ErrImagesTooManyFiles):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesTooManyFiles))
	case errors.Is(err, importErrors.ErrImagesEntryTooLarge):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesEntryTooLarge))
	case errors.Is(err, importErrors.ErrImagesEntryFormat):
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, importErrors.ImportImagesEntryFormat))
	default:
		return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
}
