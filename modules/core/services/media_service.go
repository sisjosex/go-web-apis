package services

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"

	coreConfig "josex/web/modules/core/config"
	"josex/web/modules/core/services/storage"
)

// MediaFile is one file handed to the media service for storage.
type MediaFile struct {
	// Filename is the original name; only its extension is kept.
	Filename string
	Content  []byte
}

// MediaService stores public images in the media bucket and returns the URL each one is served at
// (INFRA-007 D4): one flat prefix per category, opaque UUID names that never change.
type MediaService interface {
	// Save stores one file under a category (e.g. "avatars") and returns its public URL.
	Save(ctx context.Context, category string, file MediaFile) (string, error)
	// SaveAll stores many files concurrently and returns the public URL of each,
	// keyed by the caller's own key. A file that fails to store is absent from
	// the result and reported in errs, keyed the same way, so a bulk import can
	// carry on with the files that did land.
	SaveAll(ctx context.Context, category string, files map[string]MediaFile) (urls map[string]string, errs map[string]error)
	// Delete removes the image a URL from Save points at, once no row holds it. A URL this service
	// did not issue (a product's external image, say) is not its to delete and is left alone.
	Delete(ctx context.Context, url string) error
}

// saveWorkers bounds the concurrency of a bulk write so a 500-image import does
// not open 500 uploads at once.
const saveWorkers = 8

// immutableCache is safe because a key is a fresh UUID and is never written twice (D4): the second
// view comes from the browser or Cloudflare's edge, never from the bucket.
const immutableCache = "public, max-age=31536000, immutable"

// mediaTypes is the allow-list of what the media bucket holds. The type is the extension's, never
// sniffed from the bytes: the bucket serves exactly what this map says, behind nosniff.
var mediaTypes = map[string]string{
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
}

type mediaService struct {
	store storage.ObjectStore
}

// NewMediaService creates the media service over the media bucket.
func NewMediaService(store storage.ObjectStore) MediaService {
	return &mediaService{store: store}
}

func (s *mediaService) Save(ctx context.Context, category string, file MediaFile) (string, error) {
	return s.put(ctx, category, file)
}

func (s *mediaService) SaveAll(ctx context.Context, category string, files map[string]MediaFile) (map[string]string, map[string]error) {
	urls := make(map[string]string, len(files))
	errs := make(map[string]error)
	if len(files) == 0 {
		return urls, errs
	}

	type job struct {
		key  string
		file MediaFile
	}
	jobs := make(chan job)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < saveWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				url, putErr := s.put(ctx, category, j.file)

				mu.Lock()
				if putErr != nil {
					errs[j.key] = putErr
				} else {
					urls[j.key] = url
				}
				mu.Unlock()
			}
		}()
	}

	for key, file := range files {
		jobs <- job{key: key, file: file}
	}
	close(jobs)
	wg.Wait()

	return urls, errs
}

func (s *mediaService) Delete(ctx context.Context, url string) error {
	key, ok := strings.CutPrefix(url, strings.TrimSuffix(coreConfig.MediaPublicBaseURL(), "/")+"/")
	if !ok || key == "" {
		return nil
	}
	return s.store.Delete(ctx, key)
}

// put stores one file under an opaque UUID name, keeping only the original
// extension, and returns the URL it is served at.
func (s *mediaService) put(ctx context.Context, category string, file MediaFile) (string, error) {
	ext := mediaExtension(file.Filename)
	contentType, ok := mediaTypes[ext]
	if !ok {
		return "", fmt.Errorf("media: %q is not an image type the media bucket holds", ext)
	}

	key := sanitizeCategory(category) + "/" + uuid.NewString() + "." + ext
	err := s.store.Put(ctx, key, bytes.NewReader(file.Content), int64(len(file.Content)), storage.PutOptions{
		ContentType:  contentType,
		CacheControl: immutableCache,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(coreConfig.MediaPublicBaseURL(), "/") + "/" + key, nil
}

// sanitizeCategory keeps a category to a single safe key segment: the caller is
// internal, but this is the boundary where a key is built, so it holds the line.
func sanitizeCategory(category string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return -1
		}
	}, category)

	if cleaned == "" {
		return "misc"
	}
	return cleaned
}

// mediaExtension returns a lowercased, alphanumeric-only extension without the
// dot, or "" when the filename has none worth keeping.
func mediaExtension(filename string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return ext
}
