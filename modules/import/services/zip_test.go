package services

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	importErrors "josex/web/modules/import/errors"
)

// buildZip writes an in-memory archive from name → content pairs, preserving the
// given entry order.
func buildZip(t *testing.T, entries [][2]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, entry := range entries {
		file, err := writer.Create(entry[0])
		if err != nil {
			t.Fatalf("create entry %q: %v", entry[0], err)
		}
		if _, err := file.Write([]byte(entry[1])); err != nil {
			t.Fatalf("write entry %q: %v", entry[0], err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	return buf.Bytes()
}

func TestExtractImages_KeysByLowercasedBasename(t *testing.T) {
	archive := buildZip(t, [][2]string{
		{"Photos/Ana.PNG", "ana-bytes"},
		{"bob.jpg", "bob-bytes"},
	})

	images, err := extractImages(archive, 10)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}
	if content, ok := images.Lookup("ANA.png"); !ok || string(content) != "ana-bytes" {
		t.Errorf("expected a case-insensitive basename hit for ana.png, got %q ok=%v", content, ok)
	}
	if _, ok := images.Lookup("Photos/Ana.PNG"); ok {
		t.Error("expected the folder path to be dropped, not looked up")
	}
}

func TestExtractImages_TraversalEntryIsReducedToItsBasename(t *testing.T) {
	archive := buildZip(t, [][2]string{{"../../../etc/passwd.png", "evil"}})

	images, err := extractImages(archive, 10)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if _, ok := images["passwd.png"]; !ok {
		t.Fatalf("expected the entry to survive as a bare basename, got keys %v", keysOf(images))
	}
	for name := range images {
		if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
			t.Errorf("extracted name %q still carries a path", name)
		}
	}
}

func TestExtractImages_DisallowedExtensionIsRejected(t *testing.T) {
	archive := buildZip(t, [][2]string{
		{"ok.png", "fine"},
		{"payload.svg", "<svg onload=alert(1)>"},
	})

	_, err := extractImages(archive, 10)

	if !errors.Is(err, importErrors.ErrImagesEntryFormat) {
		t.Fatalf("expected ErrImagesEntryFormat, got %v", err)
	}
}

func TestExtractImages_OversizedEntryIsRejected(t *testing.T) {
	// The default per-entry cap is 2048 KB; one entry over it fails the archive.
	oversized := strings.Repeat("x", (2048*1024)+1)
	archive := buildZip(t, [][2]string{{"big.png", oversized}})

	_, err := extractImages(archive, 10)

	if !errors.Is(err, importErrors.ErrImagesEntryTooLarge) {
		t.Fatalf("expected ErrImagesEntryTooLarge, got %v", err)
	}
}

func TestExtractImages_TooManyEntriesIsRejected(t *testing.T) {
	archive := buildZip(t, [][2]string{
		{"a.png", "1"},
		{"b.png", "2"},
		{"c.png", "3"},
	})

	_, err := extractImages(archive, 2)

	if !errors.Is(err, importErrors.ErrImagesTooManyFiles) {
		t.Fatalf("expected ErrImagesTooManyFiles, got %v", err)
	}
}

func TestExtractImages_NotAZipIsRejected(t *testing.T) {
	_, err := extractImages([]byte("first_name,last_name\nAna,Ruiz\n"), 10)

	if !errors.Is(err, importErrors.ErrImagesInvalid) {
		t.Fatalf("expected ErrImagesInvalid, got %v", err)
	}
}

func TestExtractImages_SkipsDirectoriesAndResourceForks(t *testing.T) {
	archive := buildZip(t, [][2]string{
		{"photos/", ""},
		{"__MACOSX/._ana.png", "fork"},
		{"ana.png", "ana-bytes"},
	})

	images, err := extractImages(archive, 10)

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected only the real image, got %v", keysOf(images))
	}
}

func keysOf[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}
