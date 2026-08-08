package services

import (
	"archive/zip"
	"bytes"
	"io"
	"path/filepath"
	"strings"

	importConfig "josex/web/modules/import/config"
	importErrors "josex/web/modules/import/errors"
	importModels "josex/web/modules/import/models"
)

// extractImages inflates a companion image archive into memory, keyed by the
// lowercased basename of each entry so a CSV cell can name a file regardless of
// the folder structure inside the archive.
//
// Every input here is attacker-controlled, so the caps are the point of this
// function, not an optimization:
//
//   - an entry's path is reduced to its basename — no traversal, no nested dirs;
//   - directories and empty names are skipped;
//   - the extension must be in the configured allow-list;
//   - each entry is read through a limited reader, so a zip bomb is refused
//     during inflation rather than after it;
//   - the inflated total and the entry count are capped as well.
//
// maxEntries is passed in (rather than read from config here) because it is the
// engine's row limit: an archive can never carry more images than importable rows.
func extractImages(data []byte, maxEntries int) (importModels.ImportImages, error) {
	maxTotalBytes := int64(importConfig.MaxImagesZipMB()) << 20
	if int64(len(data)) > maxTotalBytes {
		return nil, importErrors.ErrImagesTooLarge
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, importErrors.ErrImagesInvalid
	}

	maxEntryBytes := int64(importConfig.MaxImageSizeKB()) << 10
	allowed := allowedImageExtensions()

	images := make(importModels.ImportImages)
	var inflated int64

	for _, entry := range reader.File {
		name := imageEntryName(entry.Name)
		if name == "" {
			continue
		}

		if !allowed[imageExtension(name)] {
			return nil, importErrors.ErrImagesEntryFormat
		}
		if len(images) >= maxEntries {
			return nil, importErrors.ErrImagesTooManyFiles
		}
		// Declared size is a hint an attacker controls; it is checked here only
		// to refuse an obvious oversize before spending any work on it.
		if int64(entry.UncompressedSize64) > maxEntryBytes {
			return nil, importErrors.ErrImagesEntryTooLarge
		}

		content, err := readImageEntry(entry, maxEntryBytes)
		if err != nil {
			return nil, err
		}

		inflated += int64(len(content))
		if inflated > maxTotalBytes {
			return nil, importErrors.ErrImagesTooLarge
		}

		images[name] = content
	}

	return images, nil
}

// readImageEntry inflates one entry through a limited reader so the real
// (post-inflation) size is enforced, not the size the archive declares.
func readImageEntry(entry *zip.File, maxEntryBytes int64) ([]byte, error) {
	file, err := entry.Open()
	if err != nil {
		return nil, importErrors.ErrImagesInvalid
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxEntryBytes+1))
	if err != nil {
		return nil, importErrors.ErrImagesInvalid
	}
	if int64(len(content)) > maxEntryBytes {
		return nil, importErrors.ErrImagesEntryTooLarge
	}

	return content, nil
}

// imageEntryName reduces an archive path to the lowercased basename it will be
// looked up by, and returns "" for directories, metadata and traversal attempts.
func imageEntryName(path string) string {
	if strings.HasSuffix(path, "/") || strings.HasSuffix(path, `\`) {
		return ""
	}

	// Archives written on Windows use backslashes, which filepath.Base does not
	// split on Linux — normalize first so "a\b.png" cannot survive as one name.
	normalized := strings.ReplaceAll(path, `\`, "/")
	name := strings.TrimSpace(filepath.Base(normalized))

	if name == "" || name == "." || name == ".." {
		return ""
	}
	// __MACOSX/._foo.png resource forks are not images.
	if strings.HasPrefix(name, "._") {
		return ""
	}

	return strings.ToLower(name)
}

// imageExtension returns an entry's lowercased extension without the dot.
func imageExtension(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

// allowedImageExtensions indexes the configured formats for lookup.
func allowedImageExtensions() map[string]bool {
	formats := importConfig.AllowedImageFormats()
	allowed := make(map[string]bool, len(formats))
	for _, format := range formats {
		allowed[strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))] = true
	}
	return allowed
}
