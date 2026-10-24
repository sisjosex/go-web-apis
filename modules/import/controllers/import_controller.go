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

	result, err := ctrl.importService.Validate(ctrl.buildContext(c, parsed), parsed.target, parsed.data)
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

	result, err := ctrl.importService.Import(ctrl.buildContext(c, parsed), parsed.target, parsed.data)
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

	result, err := ctrl.importService.Detect(parsed.target, parsed.data)
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
	File     string `json:"file" format:"binary" doc:"The CSV"`
	Resource string `json:"resource,omitempty" doc:"Resource key; detected from the header when empty"`
	Scope    string `json:"scope,omitempty" doc:"Comma-separated resource or module keys detection may pick from"`
	Options  string `json:"options,omitempty" doc:"JSON-encoded importModels.ImportOptions"`
	Images   string `json:"images,omitempty" format:"binary" doc:"Optional zip of the rows' images"`
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
	return api.OK(ctrl.importService.Resources(parseScope(in.Scope)))
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
}

// Template answers a CSV with just the header row for a resource, built from its descriptor's columns.
func (ctrl *ImportController) Template(ctx context.Context, in *TemplateInput) (*api.File, error) {
	c := api.Gin(ctx)
	data, err := ctrl.importService.Template(in.Resource)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	filename := fmt.Sprintf("import-template-%s.csv", in.Resource)
	return &api.File{
		ContentType:        "text/csv; charset=utf-8",
		ContentDisposition: "attachment; filename=" + filename,
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
	result, err := ctrl.importService.Schema(in.Resource)
	if err != nil {
		return nil, ctrl.mapError(c, err)
	}

	return api.OK(result)
}

// upload is one parsed multipart request: the CSV bytes, the resource and scope
// it targets, the parsed options, and the images inflated from the optional
// archive.
type upload struct {
	data    []byte
	target  importModels.ImportTarget
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

	target := importModels.ImportTarget{
		Resource: strings.TrimSpace(c.PostForm("resource")),
		Scope:    parseScope(c.PostForm("scope")),
	}
	return upload{data: data, target: target, options: options, images: images}, nil
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
