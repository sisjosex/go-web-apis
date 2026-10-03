package services

import (
	"context"
	"errors"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"josex/web/modules/core/services/storage"
	trackingConfig "josex/web/modules/tracking/config"
	trackingErrors "josex/web/modules/tracking/errors"

	"github.com/google/uuid"
)

// How many bytes are read to decide what a file really is. http.DetectContentType never looks
// further than this, so reading more would only cost a bigger range.
const sniffLen = 512

// DocumentUploadPurpose is the upload ticket's purpose for a compliance document scan (INFRA-007 D1).
const DocumentUploadPurpose = "tracking-document"

// downloadTTL is how long a signed download link opens without a session (D2).
const downloadTTL = 60 * time.Second

// cleanupTimeout bounds the deletes that run after a claim has answered: they are best-effort, and
// the bucket's lifecycle rule sweeps whatever pending/ object they miss.
const cleanupTimeout = 30 * time.Second

// documentTypes maps the extensions a compliance document may be stored as to the content type the
// bytes must sniff as and the download is served with. Both must agree (TRACK-016 D3): the declared
// type is the uploader's claim, and the sniff is what decides.
var documentTypes = map[string]string{
	"pdf":  "application/pdf",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
}

// uploadTypes is what a ticket may declare, and the extension each one is stored under.
var uploadTypes = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
}

// DocumentFiles keeps compliance document scans in the private documents bucket. The client uploads
// them there itself on a signed ticket and claims the key here; nothing reaches a scan but a signed
// link this service hands out after the session is checked (security.md).
//
// A stored file is named after its document's id, not after what the uploader called it. The
// original name is a column, shown only in the app, so nothing a caller chose ever reaches a key.
type DocumentFiles struct {
	store    storage.ObjectStore
	maxBytes int64
}

func NewDocumentFiles(store storage.ObjectStore, cfg *trackingConfig.TrackingConfig) *DocumentFiles {
	return &DocumentFiles{store: store, maxBytes: int64(cfg.MaxDocumentKB) * 1024}
}

// Purpose is what the upload ticket endpoint needs to sign a document's upload.
func (f *DocumentFiles) Purpose() storage.Purpose {
	return storage.Purpose{Name: DocumentUploadPurpose, Store: f.store, Types: uploadTypes, MaxBytes: f.maxBytes}
}

// key is where one document's file lives. Every part of it is generated — two UUIDs and an extension
// this package chose — so no caller-supplied string is ever part of a key.
func key(tenantID, documentID uuid.UUID, ext string) string {
	return tenantID.String() + "/" + documentID.String() + "." + ext
}

// Claim moves an uploaded file from its pending key to the document's own and answers its extension
// and size, which are what the row stores. It costs two store calls: one range read, which gives
// both the bytes to sniff and the total size, and one server-side copy.
//
// A key that is not a pending key of this tenant reads as not found, as does one never uploaded.
// Bytes that are not what their extension says, or too many of them, are refused and their pending
// object deleted before the answer.
func (f *DocumentFiles) Claim(ctx context.Context, tenantID, documentID uuid.UUID, pendingKey string) (ext string, size int64, err error) {
	ext, ok := f.pendingExt(tenantID, pendingKey)
	if !ok {
		return "", 0, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileNotFound}
	}

	head, total, err := f.store.GetRange(ctx, pendingKey, 0, sniffLen)
	if errors.Is(err, storage.ErrNotFound) {
		return "", 0, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileNotFound, Err: err}
	}
	if err != nil {
		return "", 0, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileFailed, Err: err}
	}
	if total > f.maxBytes || !sniffMatches(head, documentTypes[ext]) {
		if err := f.store.Delete(ctx, pendingKey); err != nil {
			log.Printf("⚠️  tracking: delete refused upload %s: %v", pendingKey, err)
		}
		return "", 0, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid}
	}

	if err := f.store.Copy(ctx, pendingKey, key(tenantID, documentID, ext), storage.PutOptions{}); err != nil {
		return "", 0, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileFailed, Err: err}
	}
	return ext, total, nil
}

// pendingExt checks a claimed key is one a ticket could have issued to this tenant —
// pending/<tenant>/<uuid>.<ext> with an extension this package stores — and answers the extension.
func (f *DocumentFiles) pendingExt(tenantID uuid.UUID, pendingKey string) (string, bool) {
	name, ok := strings.CutPrefix(pendingKey, storage.PendingPrefix(tenantID))
	if !ok {
		return "", false
	}
	id, ext, ok := strings.Cut(name, ".")
	if _, known := documentTypes[ext]; !ok || !known {
		return "", false
	}
	if _, err := uuid.Parse(id); err != nil {
		return "", false
	}
	return ext, true
}

// AfterClaim deletes what a claim left behind, once the caller has its answer: the pending object,
// and the previous file when it had another extension (one with the same was overwritten by the
// copy). It runs on its own timeout, detached from the request's cancellation.
func (f *DocumentFiles) AfterClaim(ctx context.Context, tenantID, documentID uuid.UUID, pendingKey string, previousExt *string, ext string) {
	keys := []string{pendingKey}
	if previousExt != nil && *previousExt != "" && *previousExt != ext {
		keys = append(keys, key(tenantID, documentID, *previousExt))
	}
	go f.deleteAll(ctx, keys)
}

// Discard takes back a claimed file the row never learnt about. A file of the extension the row
// already names is left alone: the copy replaced it, and deleting it would orphan the row.
func (f *DocumentFiles) Discard(ctx context.Context, tenantID, documentID uuid.UUID, pendingKey string, previousExt *string, ext string) {
	keys := []string{pendingKey}
	if previousExt == nil || *previousExt != ext {
		keys = append(keys, key(tenantID, documentID, ext))
	}
	go f.deleteAll(ctx, keys)
}

// Remove deletes a document's file if it had one, after its row is gone.
func (f *DocumentFiles) Remove(ctx context.Context, tenantID, documentID uuid.UUID, ext *string) {
	if ext == nil || *ext == "" {
		return
	}
	go f.deleteAll(ctx, []string{key(tenantID, documentID, *ext)})
}

// deleteAll outlives the request it was started from: the caller already has its answer, so the
// request's end must not cancel it.
func (f *DocumentFiles) deleteAll(ctx context.Context, keys []string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	for _, k := range keys {
		if err := f.store.Delete(ctx, k); err != nil {
			log.Printf("⚠️  tracking: delete %s: %v", k, err)
		}
	}
}

// DownloadURL signs a 60-second link to a document's file, served as an attachment named filename
// (D2). It is local signing: the store is not asked whether the file is there.
func (f *DocumentFiles) DownloadURL(ctx context.Context, tenantID, documentID uuid.UUID, ext *string, filename string) (string, time.Time, error) {
	if ext == nil || documentTypes[*ext] == "" {
		return "", time.Time{}, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileNotFound}
	}
	url, err := f.store.PresignGet(ctx, key(tenantID, documentID, *ext), filename, documentTypes[*ext], downloadTTL)
	if err != nil {
		return "", time.Time{}, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileFailed, Err: err}
	}
	return url, time.Now().Add(downloadTTL).UTC(), nil
}

// DisplayName keeps only the last segment of what the client called its file, for the row's
// file_name column — 255 characters at most.
func DisplayName(filename string) string {
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	if name == "." || name == "/" {
		return ""
	}
	if runes := []rune(name); len(runes) > 255 {
		name = string(runes[:255])
	}
	return name
}

// sniffMatches reports whether the bytes are the content type the extension promised.
func sniffMatches(head []byte, want string) bool {
	// DetectContentType may add parameters ("text/plain; charset=utf-8"); only the type matters.
	detected, _, _ := strings.Cut(http.DetectContentType(head), ";")
	return detected == want
}
