package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	coreConfig "josex/web/modules/core/config"
)

// useTempMediaRoot points the media settings at a throwaway directory. The
// settings overlay is empty in a unit test, so the env fallback is what wins.
func useTempMediaRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	t.Setenv("CORE_MEDIA_ROOT", root)
	t.Setenv("CORE_MEDIA_PUBLIC_BASE_URL", "http://localhost:8080/")

	return root
}

func TestMediaSave_WritesFileAndReturnsItsURL(t *testing.T) {
	root := useTempMediaRoot(t)
	service := NewMediaService()

	url, err := service.Save("avatars", MediaFile{Filename: "Ana.PNG", Content: []byte("ana-bytes")})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	prefix := "http://localhost:8080/media/avatars/"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("expected URL under %s, got %s", prefix, url)
	}
	if !strings.HasSuffix(url, ".png") {
		t.Errorf("expected the extension to be kept and lowercased, got %s", url)
	}

	// The URL's last segment is the on-disk name: that is what makes it fetchable
	// through router.Static("/media", MEDIA_ROOT).
	name := url[strings.LastIndex(url, "/")+1:]
	if strings.Contains(name, "Ana") {
		t.Errorf("expected an opaque name, got %s", name)
	}

	content, err := os.ReadFile(filepath.Join(root, "avatars", name))
	if err != nil {
		t.Fatalf("expected the file on disk under MEDIA_ROOT: %v", err)
	}
	if string(content) != "ana-bytes" {
		t.Errorf("expected the stored bytes, got %q", content)
	}
}

func TestMediaSaveAll_WritesEveryFileKeyedByCallerKey(t *testing.T) {
	root := useTempMediaRoot(t)
	service := NewMediaService()

	files := map[string]MediaFile{}
	for _, name := range []string{"a.png", "b.jpg", "c.gif", "d.jpeg"} {
		files[name] = MediaFile{Filename: name, Content: []byte(name + "-bytes")}
	}

	urls, errs := service.SaveAll("avatars", files)

	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
	if len(urls) != len(files) {
		t.Fatalf("expected %d URLs, got %d", len(files), len(urls))
	}

	for key, url := range urls {
		name := url[strings.LastIndex(url, "/")+1:]
		content, err := os.ReadFile(filepath.Join(root, "avatars", name))
		if err != nil {
			t.Fatalf("expected %s on disk: %v", key, err)
		}
		if string(content) != key+"-bytes" {
			t.Errorf("expected %s to hold its own bytes, got %q", key, content)
		}
	}
}

func TestMediaSaveAll_EmptyInputWritesNothing(t *testing.T) {
	root := useTempMediaRoot(t)
	service := NewMediaService()

	urls, errs := service.SaveAll("avatars", nil)

	if len(urls) != 0 || len(errs) != 0 {
		t.Fatalf("expected an empty result, got urls=%v errs=%v", urls, errs)
	}
	if _, err := os.Stat(filepath.Join(root, "avatars")); !os.IsNotExist(err) {
		t.Error("expected no category directory to be created for an empty batch")
	}
}

// TestMediaSave_StoredFileIsServedAtItsReturnedURL closes the loop the URL only
// promises: it mounts the same router.Static("/media", MEDIA_ROOT) that
// routes.go registers and fetches the path the media service handed back.
func TestMediaSave_StoredFileIsServedAtItsReturnedURL(t *testing.T) {
	useTempMediaRoot(t)
	gin.SetMode(gin.TestMode)
	service := NewMediaService()

	url, err := service.Save("avatars", MediaFile{Filename: "ana.png", Content: []byte("ana-bytes")})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	router := gin.New()
	router.Static("/media", coreConfig.MediaRoot())

	path := strings.TrimPrefix(url, "http://localhost:8080")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected %s to be served, got %d", path, recorder.Code)
	}
	if recorder.Body.String() != "ana-bytes" {
		t.Errorf("expected the stored bytes back, got %q", recorder.Body.String())
	}
}

func TestMediaSave_CategoryCannotEscapeMediaRoot(t *testing.T) {
	root := useTempMediaRoot(t)
	service := NewMediaService()

	url, err := service.Save("../../etc", MediaFile{Filename: "x.png", Content: []byte("x")})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if strings.Contains(url, "..") {
		t.Errorf("expected the category to be sanitized out of the URL, got %s", url)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read MEDIA_ROOT: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "etc" {
		t.Errorf("expected one sanitized directory named etc under MEDIA_ROOT, got %v", entries)
	}
}
