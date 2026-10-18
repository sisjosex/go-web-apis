package services

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"josex/web/modules/core/services/storage"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// ProofUploadPurpose is the upload ticket's purpose for a drop-off proof photo (MOBILE-032 D1).
const ProofUploadPurpose = "trip-proof"

// The phone reduces a proof to WebP of at most 300 KB before it uploads it (MEDIA-001); nothing else
// is signed or claimed.
const (
	proofContentType = "image/webp"
	proofExt         = "webp"
	proofMaxBytes    = 307200
)

// proofTicketTTL is the storage ticket's: long enough for a slow phone connection.
const proofTicketTTL = 5 * time.Minute

// ProofFiles keeps drop-off proof photos in the private documents bucket, under
// trip-proofs/<tenant>/<task>.webp: named by the task, so a mark undone and made again replaces its
// photo (D4). Nothing reaches one but a 60-second signed link.
type ProofFiles struct {
	store storage.ObjectStore
}

func NewProofFiles(store storage.ObjectStore) *ProofFiles {
	return &ProofFiles{store: store}
}

// Purpose registers the proof beside the compliance documents.
func (f *ProofFiles) Purpose() storage.Purpose {
	return storage.Purpose{Name: ProofUploadPurpose, Store: f.store, Types: map[string]string{proofContentType: proofExt}, MaxBytes: proofMaxBytes}
}

func proofKey(tenantID, taskID uuid.UUID) string {
	return "trip-proofs/" + tenantID.String() + "/" + taskID.String() + "." + proofExt
}

// Ticket signs a PUT of size bytes of WebP to a pending key of the tenant.
func (f *ProofFiles) Ticket(ctx context.Context, tenantID uuid.UUID, size int64) (*models.DriverProofTicket, error) {
	if size > proofMaxBytes {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DriverProofInvalid}
	}
	key := storage.PendingKey(tenantID, proofExt)
	url, headers, err := f.store.PresignPut(ctx, key, proofContentType, size, proofTicketTTL)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DriverProofFailed, Err: err}
	}
	flat := make(map[string]string, len(headers))
	for name := range headers {
		flat[name] = headers.Get(name)
	}
	return &models.DriverProofTicket{Key: key, UploadURL: url, Headers: flat, ExpiresAt: time.Now().Add(proofTicketTTL).UTC()}, nil
}

// Check answers whether a claimed key is a pending WebP of this tenant within the cap — one range
// read — and the key it will be stored under for the task.
func (f *ProofFiles) Check(ctx context.Context, tenantID, taskID uuid.UUID, pendingKey string) (string, error) {
	name, ok := strings.CutPrefix(pendingKey, storage.PendingPrefix(tenantID))
	id, ext, _ := strings.Cut(name, ".")
	if _, err := uuid.Parse(id); !ok || ext != proofExt || err != nil {
		return "", errors.New("not a pending proof key of the tenant")
	}
	head, total, err := f.store.GetRange(ctx, pendingKey, 0, sniffLen)
	if err != nil {
		return "", err
	}
	if total > proofMaxBytes || !sniffMatches(head, proofContentType) {
		return "", errors.New("not a WebP within the cap")
	}
	return proofKey(tenantID, taskID), nil
}

// Store copies a checked pending upload to its task's key.
func (f *ProofFiles) Store(ctx context.Context, pendingKey, key string) error {
	return f.store.Copy(ctx, pendingKey, key, storage.PutOptions{})
}

// DeletePending drops claimed uploads once the caller has its answer, detached from the request; the
// bucket's lifecycle rule sweeps whatever it misses.
func (f *ProofFiles) DeletePending(ctx context.Context, keys []string) {
	if len(keys) == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		defer cancel()
		for _, k := range keys {
			if err := f.store.Delete(ctx, k); err != nil {
				log.Printf("⚠️  tracking: delete proof upload %s: %v", k, err)
			}
		}
	}()
}

// URL signs a 60-second link to a stored proof.
func (f *ProofFiles) URL(ctx context.Context, key string) (*models.TripTaskProof, error) {
	url, err := f.store.PresignGet(ctx, key, "proof."+proofExt, proofContentType, downloadTTL)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DriverProofFailed, Err: err}
	}
	return &models.TripTaskProof{URL: url, ExpiresAt: time.Now().Add(downloadTTL).UTC()}, nil
}
