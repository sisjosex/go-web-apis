package services

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	trackingConfig "josex/web/modules/tracking/config"
	trackingErrors "josex/web/modules/tracking/errors"

	"github.com/google/uuid"
)

// How many bytes are read to decide what a file really is. http.DetectContentType never looks
// further than this, so reading more would only cost memory.
const sniffLen = 512

// allowedDocumentTypes maps the extensions a compliance document may be scanned as to the content
// types the bytes must actually sniff as. Both must agree (TRACK-016 D3): an extension alone is the
// uploader's claim, and a sniff alone would let a .exe through under a name the browser trusts.
var allowedDocumentTypes = map[string][]string{
	"pdf":  {"application/pdf"},
	"jpg":  {"image/jpeg"},
	"jpeg": {"image/jpeg"},
	"png":  {"image/png"},
}

// DocumentFileStore keeps compliance document scans on disk, outside the web root and never under
// MEDIA_ROOT: a licence scan is not public content, and the only way to it is the authenticated
// download endpoint (security.md).
//
// A stored file is named after its document's id, not after what the uploader called it. The
// original name is a column, shown only in the Content-Disposition of a download, so nothing a
// caller chose ever reaches the filesystem or a URL.
type DocumentFileStore struct {
	root     string
	maxBytes int64
}

func NewDocumentFileStore(cfg *trackingConfig.TrackingConfig) *DocumentFileStore {
	return &DocumentFileStore{
		root:     cfg.DocumentsRoot,
		maxBytes: int64(cfg.MaxDocumentKB) * 1024,
	}
}

// Path is where one document's file lives. Every part of it is generated — two UUIDs and an
// extension this package chose — so no caller-supplied string can walk out of the root.
func (s *DocumentFileStore) Path(tenantID uuid.UUID, documentID uuid.UUID, ext string) string {
	return filepath.Join(s.root, tenantID.String(), documentID.String()+"."+ext)
}

// Save validates the upload and writes it, replacing whatever the document had before. It answers
// with the original filename and the byte count, which are what the row stores.
//
// The size is checked twice: once against the header the client sent, so an oversized upload is
// refused before it is read, and once while copying, because that header is only a claim.
func (s *DocumentFileStore) Save(tenantID uuid.UUID, documentID uuid.UUID, header *multipart.FileHeader) (name string, size int64, ext string, err error) {
	if header == nil {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid}
	}
	if header.Size > s.maxBytes {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid}
	}

	ext = strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
	wanted, ok := allowedDocumentTypes[ext]
	if !ok {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid}
	}

	src, err := header.Open()
	if err != nil {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid, Err: err}
	}
	defer func() { _ = src.Close() }()

	head := make([]byte, sniffLen)
	read, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid, Err: err}
	}
	head = head[:read]
	if !sniffMatches(head, wanted) {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid}
	}

	dest := s.Path(tenantID, documentID, ext)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileFailed, Err: err}
	}
	out, err := os.Create(dest)
	if err != nil {
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileFailed, Err: err}
	}
	// Closed explicitly below on every path: a write that only fails at Close would otherwise leave
	// a truncated file the row claims is whole.
	defer func() { _ = out.Close() }()

	// LimitReader by one byte past the cap: a body longer than its own Content-Length claim stops
	// here rather than filling the disk.
	written, err := io.Copy(out, io.LimitReader(io.MultiReader(strings.NewReader(string(head)), src), s.maxBytes+1))
	if err != nil {
		_ = os.Remove(dest)
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileFailed, Err: err}
	}
	if written > s.maxBytes {
		_ = os.Remove(dest)
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileInvalid}
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dest)
		return "", 0, "", &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileFailed, Err: err}
	}

	return filepath.Base(header.Filename), written, ext, nil
}

// Remove deletes a document's file if it had one. A missing file is not an error: the row is
// already gone, and there is nothing left to reconcile.
func (s *DocumentFileStore) Remove(tenantID uuid.UUID, documentID uuid.UUID, ext *string) {
	if ext == nil || *ext == "" {
		return
	}
	_ = os.Remove(s.Path(tenantID, documentID, *ext))
}

// Open hands back the stored file for streaming. A row that claims a file the disk does not have
// reads as not-found, which is what the caller can act on.
func (s *DocumentFileStore) Open(tenantID uuid.UUID, documentID uuid.UUID, ext *string) (*os.File, error) {
	if ext == nil || *ext == "" {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileNotFound}
	}
	file, err := os.Open(s.Path(tenantID, documentID, *ext))
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DocumentFileNotFound, Err: err}
	}
	return file, nil
}

// sniffMatches reports whether the bytes are one of the content types the extension promised.
func sniffMatches(head []byte, wanted []string) bool {
	// DetectContentType may add parameters ("text/plain; charset=utf-8"); only the type matters.
	detected, _, _ := strings.Cut(http.DetectContentType(head), ";")
	for _, want := range wanted {
		if detected == want {
			return true
		}
	}
	return false
}
