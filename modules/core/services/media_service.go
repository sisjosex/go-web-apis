package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"

	coreConfig "josex/web/modules/core/config"
)

// MediaFile is one file handed to the media service for storage.
type MediaFile struct {
	// Filename is the original name; only its extension is kept.
	Filename string
	Content  []byte
}

// MediaService stores binary files under MEDIA_ROOT and returns the public URL
// each one is served at. It is the app's first stored-file path (D5): local
// disk, one flat directory per category, opaque UUID names.
type MediaService interface {
	// Save writes one file into a category directory (e.g. "avatars") and
	// returns its public URL.
	Save(category string, file MediaFile) (string, error)
	// SaveAll writes many files concurrently and returns the public URL of each,
	// keyed by the caller's own key. A file that fails to write is absent from
	// the result and reported in errs, keyed the same way, so a bulk import can
	// carry on with the files that did land.
	SaveAll(category string, files map[string]MediaFile) (urls map[string]string, errs map[string]error)
}

// saveWorkers bounds the concurrency of a bulk write so a 500-image import does
// not open 500 files at once.
const saveWorkers = 8

type mediaService struct{}

// NewMediaService creates the local-disk media service.
func NewMediaService() MediaService {
	return &mediaService{}
}

func (s *mediaService) Save(category string, file MediaFile) (string, error) {
	directory, err := s.categoryDir(category)
	if err != nil {
		return "", err
	}
	return s.write(category, directory, file)
}

func (s *mediaService) SaveAll(category string, files map[string]MediaFile) (map[string]string, map[string]error) {
	urls := make(map[string]string, len(files))
	errs := make(map[string]error)
	if len(files) == 0 {
		return urls, errs
	}

	directory, err := s.categoryDir(category)
	if err != nil {
		for key := range files {
			errs[key] = err
		}
		return urls, errs
	}

	// The directory is created once above, so the workers only write files.
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
				url, writeErr := s.write(category, directory, j.file)

				mu.Lock()
				if writeErr != nil {
					errs[j.key] = writeErr
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

// categoryDir resolves and creates the directory a category is stored in.
func (s *mediaService) categoryDir(category string) (string, error) {
	directory := filepath.Join(coreConfig.MediaRoot(), sanitizeCategory(category))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	return directory, nil
}

// write stores one file under an opaque UUID name, keeping only the original
// extension, and returns the URL it is served at.
func (s *mediaService) write(category, directory string, file MediaFile) (string, error) {
	name := uuid.NewString()
	if ext := mediaExtension(file.Filename); ext != "" {
		name += "." + ext
	}

	if err := os.WriteFile(filepath.Join(directory, name), file.Content, 0o644); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/media/%s/%s",
		strings.TrimSuffix(coreConfig.MediaPublicBaseURL(), "/"),
		sanitizeCategory(category),
		name,
	), nil
}

// sanitizeCategory keeps a category to a single safe path segment: the caller is
// internal, but this is the boundary where a path is built, so it holds the line.
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
