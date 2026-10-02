package controllers

import (
	"net/http"
	"time"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/services/storage"
	"josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// uploadTicketTTL is how long a signed upload URL stays good (D1): long enough for a slow phone
// connection, short enough that a leaked one is soon worthless.
const uploadTicketTTL = 5 * time.Minute

// StorageController hands out upload tickets (INFRA-007 D1): the client sends the bytes straight to
// the bucket and then claims them through the endpoint of whatever they are for.
type StorageController struct {
	purposes *storage.Purposes
}

func NewStorageController(purposes *storage.Purposes) *StorageController {
	return &StorageController{purposes: purposes}
}

// UploadTicketRequest is what the client means to upload.
type UploadTicketRequest struct {
	Purpose     string `json:"purpose" binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	Size        int64  `json:"size" binding:"required,gt=0"`
}

// UploadTicket is where to PUT the bytes, and the key to claim them with afterwards.
type UploadTicket struct {
	Key       string            `json:"key"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// CreateUpload godoc
// @Summary Request an upload ticket
// @Description Signs a 5-minute PUT straight to storage; the declared type and size are part of the signature
// @Tags Storage
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param ticket body UploadTicketRequest true "What is being uploaded"
// @Success 201 {object} UploadTicket
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /storage/uploads [post]
func (ctrl *StorageController) CreateUpload(c *gin.Context) {
	// The tenant chain sets tenant_id as a string; no tenant, no ticket — the key is under it.
	tenant, err := uuid.Parse(c.GetString("tenant_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, coreErrors.UserInsufficientPermissions))
		return
	}

	var req UploadTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, coreErrors.StorageUploadInvalid, utils.ExtractValidationError(c, err)))
		return
	}
	purpose, ok := ctrl.purposes.Lookup(req.Purpose)
	if !ok {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadInvalid))
		return
	}
	ext, ok := purpose.Types[req.ContentType]
	if !ok || req.Size > purpose.MaxBytes {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadInvalid))
		return
	}

	key := storage.PendingKey(tenant, ext)
	url, headers, err := purpose.Store.PresignPut(c.Request.Context(), key, req.ContentType, req.Size, uploadTicketTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	flat := make(map[string]string, len(headers))
	for name := range headers {
		flat[name] = headers.Get(name)
	}
	c.JSON(http.StatusCreated, UploadTicket{
		Key:       key,
		UploadURL: url,
		Headers:   flat,
		ExpiresAt: time.Now().Add(uploadTicketTTL).UTC(),
	})
}
