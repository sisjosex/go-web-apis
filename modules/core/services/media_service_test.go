package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"josex/web/modules/core/services/storage/storagetest"
)

// fetch reads a stored image the way a browser does: a plain GET on the URL the service returned.
func fetch(t *testing.T, url string) (status int, header http.Header, body string) {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = res.Body.Close() }()
	content, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, string(content)
}

// TestMediaSave_StoredFileIsServedAtItsReturnedURL closes the loop the URL only promises: the image
// answers on its public URL with the extension's type and the immutable cache (INFRA-007 D4).
func TestMediaSave_StoredFileIsServedAtItsReturnedURL(t *testing.T) {
	service := NewMediaService(storagetest.Media(t))

	url, err := service.Save(context.Background(), "avatars", MediaFile{Filename: "Ana.PNG", Content: []byte("ana-bytes")})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if !strings.Contains(url, "/avatars/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("expected a URL under avatars/ with the extension lowercased, got %s", url)
	}
	if strings.Contains(url, "Ana") {
		t.Errorf("expected an opaque name, got %s", url)
	}

	status, header, body := fetch(t, url)
	if status != http.StatusOK || body != "ana-bytes" {
		t.Fatalf("expected the stored bytes at %s, got %d %q", url, status, body)
	}
	if got := header.Get("Content-Type"); got != "image/png" {
		t.Errorf("expected image/png, got %q", got)
	}
	if got := header.Get("Cache-Control"); got != immutableCache {
		t.Errorf("expected the immutable cache, got %q", got)
	}
}

func TestMediaSaveAll_StoresEveryFileKeyedByCallerKey(t *testing.T) {
	service := NewMediaService(storagetest.Media(t))

	files := map[string]MediaFile{}
	for _, name := range []string{"a.png", "b.jpg", "c.gif", "d.jpeg"} {
		files[name] = MediaFile{Filename: name, Content: []byte(name + "-bytes")}
	}

	urls, errs := service.SaveAll(context.Background(), "avatars", files)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(urls) != len(files) {
		t.Fatalf("expected %d URLs, got %d", len(files), len(urls))
	}
	for key, url := range urls {
		if _, _, body := fetch(t, url); body != key+"-bytes" {
			t.Errorf("expected %s to hold its own bytes, got %q", key, body)
		}
	}
}

func TestMediaSaveAll_EmptyInputStoresNothing(t *testing.T) {
	service := NewMediaService(storagetest.Media(t))

	urls, errs := service.SaveAll(context.Background(), "avatars", nil)

	if len(urls) != 0 || len(errs) != 0 {
		t.Fatalf("expected an empty result, got urls=%v errs=%v", urls, errs)
	}
}

// TestMediaSave_RefusesWhatIsNotAnImage: the bucket serves the extension's type, so an extension off
// the allow-list never gets in.
func TestMediaSave_RefusesWhatIsNotAnImage(t *testing.T) {
	service := NewMediaService(storagetest.Media(t))

	for _, name := range []string{"page.html", "script.svg", "noextension"} {
		if _, err := service.Save(context.Background(), "avatars", MediaFile{Filename: name, Content: []byte("x")}); err == nil {
			t.Errorf("expected %s to be refused", name)
		}
	}
}

func TestMediaSave_CategoryCannotEscapeItsPrefix(t *testing.T) {
	service := NewMediaService(storagetest.Media(t))

	url, err := service.Save(context.Background(), "../../etc", MediaFile{Filename: "x.png", Content: []byte("x")})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if strings.Contains(url, "..") || !strings.Contains(url, "/etc/") {
		t.Errorf("expected the category sanitized to etc/, got %s", url)
	}
}
