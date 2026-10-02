// Package storage is the stored-file port (INFRA-007): Cloudflare R2 in production, a local S3
// (SeaweedFS) in development and tests, the same code against both (D3).
package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// ObjectStore is one bucket. Every call but the presigners is one request to the store; presigning
// is local signing and never leaves the process (D1, D2).
type ObjectStore interface {
	// Put writes an object whole.
	Put(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) error
	// GetRange reads length bytes from offset and reports the object's total size, taken from the
	// Content-Range of the same response — so knowing the size never costs a separate Head.
	GetRange(ctx context.Context, key string, offset, length int64) (data []byte, total int64, err error)
	// Copy duplicates src to dst inside the bucket, server side, overwriting dst.
	Copy(ctx context.Context, src, dst string) error
	// Delete removes an object; a key that is not there is not an error.
	Delete(ctx context.Context, key string) error
	// PresignPut signs a PUT whose Content-Type and Content-Length are part of the signature, so the
	// store refuses any other type or length. headers are what the client must send with it.
	PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (url string, headers http.Header, err error)
	// PresignGet signs a GET that answers as an attachment named filename with contentType.
	PresignGet(ctx context.Context, key, filename, contentType string, ttl time.Duration) (string, error)
}

// PutOptions are the response headers stored with an object.
type PutOptions struct {
	ContentType  string
	CacheControl string
}

// ErrNotFound is the store's answer for a key it does not hold.
var ErrNotFound = errors.New("storage: object not found")
