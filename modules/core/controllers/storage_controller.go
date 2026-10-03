package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	coreErrors "josex/web/modules/core/errors"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/services/storage"
	"josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
)

// uploadTicketTTL is how long a signed upload URL stays good (D1): long enough for a slow phone
// connection, short enough that a leaked one is soon worthless.
const uploadTicketTTL = 5 * time.Minute

// sniffLen is all http.DetectContentType ever looks at, so the claim reads no more than this.
const sniffLen = 512

// cleanupTimeout bounds deleting a claimed pending object after the answer; the bucket's lifecycle
// rule sweeps whatever it misses.
const cleanupTimeout = 30 * time.Second

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
	var req UploadTicketRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
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

	// A public purpose's key is under the user; any other's under the tenant the chain set as a
	// string — no tenant, no ticket.
	owner, err := uuid.Parse(c.GetString("tenant_id"))
	if purpose.Public {
		owner, err = uuid.Parse(c.GetString("user_id"))
	}
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, coreErrors.UserInsufficientPermissions))
		return
	}
	key := storage.PendingKey(owner, ext)
	if purpose.Public {
		key = storage.PublicPendingKey(owner, purpose.Name, ext)
	}

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

// TenantUnlessPublic runs the tenant chain in front of a ticket for a tenant's purpose and skips it
// for a public one: an avatar belongs to the user, who may have no tenant, or one whose chain refuses
// their role (a driver, a guardian). The body is bound once and cached for CreateUpload.
func (ctrl *StorageController) TenantUnlessPublic(tenant gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UploadTicketRequest
		if err := c.ShouldBindBodyWith(&req, binding.JSON); err == nil {
			if purpose, ok := ctrl.purposes.Lookup(req.Purpose); ok && purpose.Public {
				c.Next()
				return
			}
		}
		tenant(c)
	}
}

// ClaimUploadRequest names the pending upload to publish.
type ClaimUploadRequest struct {
	Key string `json:"key" binding:"required"`
}

// ClaimedUpload is where a published image is served.
type ClaimedUpload struct {
	URL string `json:"url"`
}

// ClaimUpload godoc
// @Summary Publish an uploaded image
// @Description Checks a public purpose's upload (its size, and bytes that are the signed type) and publishes it at an immutable URL of the media bucket
// @Tags Storage
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param claim body ClaimUploadRequest true "The ticket's key"
// @Success 200 {object} ClaimedUpload
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 422 {object} coreErrors.ErrorResponse
// @Router /storage/uploads/claim [post]
func (ctrl *StorageController) ClaimUpload(c *gin.Context) {
	user, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, coreErrors.UserInsufficientPermissions))
		return
	}
	var req ClaimUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, coreErrors.StorageUploadInvalid, utils.ExtractValidationError(c, err)))
		return
	}
	purpose, name, contentType, ok := ctrl.publicPending(user, req.Key)
	if !ok {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadNotFound))
		return
	}

	// Two store calls (INFRA-007 D1): a range read gives the bytes to sniff and the total size, a
	// server-side copy publishes them with the checked type and the immutable cache.
	ctx := c.Request.Context()
	head, total, err := purpose.Store.GetRange(ctx, req.Key, 0, sniffLen)
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadNotFound))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	detected, _, _ := strings.Cut(http.DetectContentType(head), ";")
	if total > purpose.MaxBytes || detected != contentType {
		if err := purpose.Store.Delete(ctx, req.Key); err != nil {
			log.Printf("⚠️  storage: delete refused upload %s: %v", req.Key, err)
		}
		c.JSON(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, coreErrors.StorageFileInvalid))
		return
	}

	published := purpose.Name + "/" + name
	err = purpose.Store.Copy(ctx, req.Key, published, storage.PutOptions{ContentType: contentType, CacheControl: coreServices.ImmutableCache})
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, ClaimedUpload{URL: coreServices.MediaURL(published)})

	// The pending object goes after the answer, detached from the request's cancellation.
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if err := purpose.Store.Delete(ctx, req.Key); err != nil {
			log.Printf("⚠️  storage: delete claimed upload %s: %v", req.Key, err)
		}
	}()
}

// publicPending checks a key is one a public purpose's ticket could have issued to this user —
// pending/<user>/<purpose>/<uuid>.<ext> — and answers the purpose, the name it is published under
// (<uuid>.<ext>) and the content type its bytes must sniff as.
func (ctrl *StorageController) publicPending(user uuid.UUID, key string) (storage.Purpose, string, string, bool) {
	rest, _ := strings.CutPrefix(key, storage.PendingPrefix(user))
	purposeName, name, _ := strings.Cut(rest, "/")
	purpose, ok := ctrl.purposes.Lookup(purposeName)
	if rest == key || !ok || !purpose.Public {
		return storage.Purpose{}, "", "", false
	}
	id, ext, _ := strings.Cut(name, ".")
	if _, err := uuid.Parse(id); err != nil {
		return storage.Purpose{}, "", "", false
	}
	for contentType, typeExt := range purpose.Types {
		if typeExt == ext {
			return purpose, name, contentType, true
		}
	}
	return storage.Purpose{}, "", "", false
}
