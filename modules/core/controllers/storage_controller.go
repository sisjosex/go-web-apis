package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"josex/web/modules/core/api"
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

// CreateUploadInput is CreateUpload's request.
type CreateUploadInput struct {
	JSON api.Body[UploadTicketRequest]
}

// CreateUpload is POST /storage/uploads: Request an upload ticket.
func (ctrl *StorageController) CreateUpload(ctx context.Context, in *CreateUploadInput) (*api.Out[UploadTicket], error) {
	c := api.Gin(ctx)
	var req UploadTicketRequest
	if err := in.JSON.IntoCached(ctx, &req); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, coreErrors.StorageUploadInvalid, utils.ExtractValidationError(c, err)))
	}
	purpose, ok := ctrl.purposes.Lookup(req.Purpose)
	if !ok {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadInvalid))
	}
	ext, ok := purpose.Types[req.ContentType]
	if !ok || req.Size > purpose.MaxBytes {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadInvalid))
	}

	// A public purpose's key is under the user; any other's under the tenant the chain set as a
	// string — no tenant, no ticket.
	owner, err := uuid.Parse(c.GetString("tenant_id"))
	if purpose.Public {
		owner, err = uuid.Parse(c.GetString("user_id"))
	}
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, coreErrors.UserInsufficientPermissions))
	}
	key := storage.PendingKey(owner, ext)
	if purpose.Public {
		key = storage.PublicPendingKey(owner, purpose.Name, ext)
	}

	url, headers, err := purpose.Store.PresignPut(c.Request.Context(), key, req.ContentType, req.Size, uploadTicketTTL)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	flat := make(map[string]string, len(headers))
	for name := range headers {
		flat[name] = headers.Get(name)
	}
	return api.OK(UploadTicket{
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

// ClaimUploadInput is ClaimUpload's request.
type ClaimUploadInput struct {
	JSON api.Body[ClaimUploadRequest]
}

// ClaimUpload checks a public purpose's upload (its size, and bytes that are the signed type) and
// publishes it at an immutable URL of the media bucket.
func (ctrl *StorageController) ClaimUpload(ctx context.Context, in *ClaimUploadInput) (*api.Out[ClaimedUpload], error) {
	c := api.Gin(ctx)
	user, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, coreErrors.UserInsufficientPermissions))
	}
	var req ClaimUploadRequest
	if err := in.JSON.Into(ctx, &req); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, coreErrors.StorageUploadInvalid, utils.ExtractValidationError(c, err)))
	}
	purpose, name, contentType, ok := ctrl.publicPending(user, req.Key)
	if !ok {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadNotFound))
	}

	// Two store calls (INFRA-007 D1): a range read gives the bytes to sniff and the total size, a
	// server-side copy publishes them with the checked type and the immutable cache.
	head, total, err := purpose.Store.GetRange(ctx, req.Key, 0, sniffLen)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, coreErrors.StorageUploadNotFound))
	}
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	detected, _, _ := strings.Cut(http.DetectContentType(head), ";")
	if total > purpose.MaxBytes || detected != contentType {
		if err := purpose.Store.Delete(ctx, req.Key); err != nil {
			log.Printf("⚠️  storage: delete refused upload %s: %v", req.Key, err)
		}
		return nil, api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorSingle(c, coreErrors.StorageFileInvalid))
	}

	published := purpose.Name + "/" + name
	err = purpose.Store.Copy(ctx, req.Key, published, storage.PutOptions{ContentType: contentType, CacheControl: coreServices.ImmutableCache})
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	// The pending object goes once published, detached from the request's cancellation.
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		if err := purpose.Store.Delete(ctx, req.Key); err != nil {
			log.Printf("⚠️  storage: delete claimed upload %s: %v", req.Key, err)
		}
	}()
	return api.OK(ClaimedUpload{URL: coreServices.MediaURL(published)})
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
