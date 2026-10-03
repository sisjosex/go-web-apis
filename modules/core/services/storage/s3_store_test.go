package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"josex/web/modules/core/services/storage"
	"josex/web/modules/core/services/storage/storagetest"
)

// TestS3Store_RoundTrip is the port's whole contract against the local S3: put, read a range with
// the total size, copy, delete, and a missing key reading as ErrNotFound.
func TestS3Store_RoundTrip(t *testing.T) {
	ctx := context.Background()
	store := storagetest.Documents(t)
	body := []byte("%PDF-1.4 round trip body")
	src := "roundtrip/" + uuid.NewString() + ".pdf"
	dst := "roundtrip/" + uuid.NewString() + ".pdf"
	t.Cleanup(func() {
		_ = store.Delete(ctx, src)
		_ = store.Delete(ctx, dst)
	})

	if err := store.Put(ctx, src, bytes.NewReader(body), int64(len(body)), storage.PutOptions{ContentType: "application/pdf"}); err != nil {
		t.Fatalf("put: %v", err)
	}

	head, total, err := store.GetRange(ctx, src, 0, 8)
	if err != nil {
		t.Fatalf("get range: %v", err)
	}
	if string(head) != "%PDF-1.4" || total != int64(len(body)) {
		t.Fatalf("expected the first 8 bytes and total %d, got %q and %d", len(body), head, total)
	}

	// A range past a short object's end answers what there is, and still the true total.
	whole, total, err := store.GetRange(ctx, src, 0, 512)
	if err != nil || !bytes.Equal(whole, body) || total != int64(len(body)) {
		t.Fatalf("expected the whole body for a long range, got %q, %d, %v", whole, total, err)
	}

	if err := store.Copy(ctx, src, dst, storage.PutOptions{}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	copied, _, err := store.GetRange(ctx, dst, 0, 512)
	if err != nil || !bytes.Equal(copied, body) {
		t.Fatalf("expected the copy to hold the same bytes, got %q, %v", copied, err)
	}

	if err := store.Delete(ctx, src); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, _, err := store.GetRange(ctx, src, 0, 8); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := store.Delete(ctx, src); err != nil {
		t.Errorf("expected deleting a missing key to be a no-op, got %v", err)
	}
}

// TestS3Store_PresignedPutEnforcesTypeAndLength proves D1's guard: the signed type and length are
// the only ones the store takes.
func TestS3Store_PresignedPutEnforcesTypeAndLength(t *testing.T) {
	ctx := context.Background()
	store := storagetest.Documents(t)
	key := "pending/" + uuid.NewString() + ".pdf"
	t.Cleanup(func() { _ = store.Delete(ctx, key) })
	body := []byte("%PDF-1.4 signed")

	url, headers, err := store.PresignPut(ctx, key, "application/pdf", int64(len(body)), time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if headers.Get("Content-Type") != "application/pdf" {
		t.Fatalf("expected the client to be told its Content-Type, got %v", headers)
	}

	put := func(contentType string, payload []byte) int {
		req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
		req.Header.Set("Content-Type", contentType)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("put: %v", err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}

	if code := put("text/html", body); code != http.StatusForbidden {
		t.Errorf("expected another type to be refused, got %d", code)
	}
	if code := put("application/pdf", append(body, []byte("more")...)); code != http.StatusForbidden {
		t.Errorf("expected another length to be refused, got %d", code)
	}
	if code := put("application/pdf", body); code != http.StatusOK {
		t.Fatalf("expected the signed upload to land, got %d", code)
	}

	download, err := store.PresignGet(ctx, key, "Licence-1.pdf", "application/pdf", time.Minute)
	if err != nil {
		t.Fatalf("presign get: %v", err)
	}
	res, err := http.Get(download)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	got, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || !bytes.Equal(got, body) {
		t.Fatalf("expected the bytes back, got %d %q", res.StatusCode, got)
	}
	if disposition := res.Header.Get("Content-Disposition"); !strings.Contains(disposition, `filename="Licence-1.pdf"`) {
		t.Errorf("expected the download name, got %q", disposition)
	}
}

// TestEmptyTestBucket_RefusesAnyOtherBucket - the bulk delete only ever runs on a *-test bucket.
func TestEmptyTestBucket_RefusesAnyOtherBucket(t *testing.T) {
	client, err := storage.NewClient("http://127.0.0.1:8333", "dev-access-key", "dev-secret-key")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if err := client.EmptyTestBucket(context.Background(), "taypi24-documents"); err == nil {
		t.Fatal("expected a bucket without the -test suffix to be refused")
	}
}
