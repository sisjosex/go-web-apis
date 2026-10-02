// Package storagetest hands tests the buckets of .env.test on the dev compose's local S3
// (INFRA-007 D3): the same store production code runs against, so a test needs it up, as it needs
// PostgreSQL.
package storagetest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"

	coreConfig "josex/web/modules/core/config"
	"josex/web/modules/core/services/storage"
)

// Media is the test media bucket; its objects are public at CORE_MEDIA_PUBLIC_BASE_URL.
func Media(t testing.TB) storage.ObjectStore {
	cfg := load(t)
	return client(t, cfg).Bucket(cfg.StorageMediaBucket)
}

// Documents is the test documents bucket.
func Documents(t testing.TB) storage.ObjectStore {
	cfg := load(t)
	return client(t, cfg).Bucket(cfg.StorageDocumentsBucket)
}

// load reads .env.test from the nearest parent that has one. godotenv never overrides a variable
// already set, so a test's own t.Setenv wins.
func load(t testing.TB) *coreConfig.CoreConfig {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("storagetest: %v", err)
	}
	for ; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if path := filepath.Join(dir, ".env.test"); fileExists(path) {
			_ = godotenv.Load(path)
			break
		}
	}
	return coreConfig.LoadCoreConfig()
}

func client(t testing.TB, cfg *coreConfig.CoreConfig) *storage.Client {
	t.Helper()
	c, err := storage.NewClient(cfg.StorageEndpoint, cfg.StorageAccessKey, cfg.StorageSecretKey)
	if err != nil {
		t.Fatalf("storagetest: %v", err)
	}
	return c
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
