//go:build integration
// +build integration

package core_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
)

// webpBytes is enough of a WebP for http.DetectContentType: RIFF, a length, then WEBPVP8.
func webpBytes(size int) []byte {
	head := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	return append(head, bytes.Repeat([]byte{0}, size-len(head))...)
}

// pngBytes starts with the PNG signature.
func pngBytes(size int) []byte {
	head := []byte("\x89PNG\r\n\x1a\n")
	return append(head, bytes.Repeat([]byte{0}, size-len(head))...)
}

// uploadAvatar asks for an avatar ticket declaring WebP, PUTs content to it, and answers the key.
func uploadAvatar(t *testing.T, helper *testhelpers.ApiTestHelper, content []byte) string {
	t.Helper()
	w := helper.DoRequest("POST", "/storage/uploads", map[string]interface{}{
		"purpose": "avatar", "content_type": "image/webp", "size": len(content),
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("ticket: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var ticket struct {
		Key       string            `json:"key"`
		UploadURL string            `json:"upload_url"`
		Headers   map[string]string `json:"headers"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &ticket)

	req, _ := http.NewRequest(http.MethodPut, ticket.UploadURL, bytes.NewReader(content))
	for name, value := range ticket.Headers {
		req.Header.Set(name, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("upload: expected 200 from the bucket, got %d", res.StatusCode)
	}
	return ticket.Key
}

// loginWithoutTenant registers a user who belongs to no tenant: an avatar is the user's, so its
// ticket and claim must not need one (MEDIA-001).
func loginWithoutTenant(t *testing.T, helper *testhelpers.ApiTestHelper, email string) {
	t.Helper()
	if _, err := helper.Register(email, "$Password2025", "Ana", "Quispe"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := helper.Login(email, "$Password2025"); err != nil {
		t.Fatalf("login: %v", err)
	}
}

// TestClaimAvatarPublishesImmutableWebP - ticket, PUT, claim: the answer is a media URL that serves
// the bytes as WebP with the immutable cache, and the pending key is not part of it.
func TestClaimAvatarPublishesImmutableWebP(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	loginWithoutTenant(t, helper, "avatar-claim@test.com")

	content := webpBytes(2048)
	key := uploadAvatar(t, helper, content)
	assert.True(t, strings.HasPrefix(key, "pending/"+helper.GetUserID()+"/avatar/"), key)

	w := helper.DoRequest("POST", "/storage/uploads/claim", map[string]interface{}{"key": key}, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var claimed struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &claimed)
	assert.Contains(t, claimed.URL, "/avatar/")
	assert.True(t, strings.HasSuffix(claimed.URL, ".webp"), claimed.URL)

	res, err := http.Get(claimed.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "image/webp", res.Header.Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", res.Header.Get("Cache-Control"))

	again := helper.DoRequest("POST", "/storage/uploads/claim", map[string]interface{}{"key": key}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, again.Code, "a claimed upload cannot be claimed twice: "+again.Body.String())
}

// TestClaimAvatarRejectsDisguisedBytes - the signed type is the uploader's claim; the bytes decide.
// PNG bytes signed as WebP are refused and their pending object deleted.
func TestClaimAvatarRejectsDisguisedBytes(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	loginWithoutTenant(t, helper, "avatar-png@test.com")

	key := uploadAvatar(t, helper, pngBytes(2048))

	w := helper.DoRequest("POST", "/storage/uploads/claim", map[string]interface{}{"key": key}, map[string]string{})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "storage.file-invalid")

	again := helper.DoRequest("POST", "/storage/uploads/claim", map[string]interface{}{"key": key}, map[string]string{})
	assert.Equal(t, http.StatusNotFound, again.Code, "a refused upload's pending object is gone")
}

// TestClaimAvatarRejectsAnotherUsersKey - the key names its owner and its purpose; anyone else's
// pending upload, or a key no public ticket issues, reads as not found.
func TestClaimAvatarRejectsAnotherUsersKey(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	loginWithoutTenant(t, helper, "avatar-owner@test.com")
	key := uploadAvatar(t, helper, webpBytes(1024))

	loginWithoutTenant(t, helper, "avatar-thief@test.com")
	mine := "pending/" + helper.GetUserID() + "/"
	for _, k := range []string{
		key,
		mine + "tracking-document/" + "00000000-0000-4000-8000-000000000000.webp",
		mine + "avatar/../x.webp",
		mine + "avatar/00000000-0000-4000-8000-000000000000.gif",
	} {
		w := helper.DoRequest("POST", "/storage/uploads/claim", map[string]interface{}{"key": k}, map[string]string{})
		assert.Equal(t, http.StatusNotFound, w.Code, k+": "+w.Body.String())
	}
}
