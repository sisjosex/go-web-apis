//go:build integration
// +build integration

package services_test

import (
	"context"
	"os"
	"testing"

	"josex/web/config"
	coreServices "josex/web/modules/core/services"
	coreTestHelpers "josex/web/modules/core/testhelpers"
)

func init() {
	coreTestHelpers.InitTestEnvironment()
	config.GetConfig() // initialize ModularAppConfig before any test runs
}

// TestDatabaseService_PrimaryPool_IsReachable verifies that InitDatabase
// establishes the primary pool and basic queries work.
func TestDatabaseService_PrimaryPool_IsReachable(t *testing.T) {
	os.Setenv("SKIP_MIGRATIONS", "true")
	defer os.Unsetenv("SKIP_MIGRATIONS")

	svc := coreServices.NewDatabaseService()
	ctx := context.Background()
	svc.InitDatabase(ctx)
	defer svc.CloseDatabase(ctx)

	if svc.GetPrimaryPool() == nil {
		t.Fatal("primary pool is nil after InitDatabase")
	}

	rows, err := svc.Query(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("query on primary pool failed: %v", err)
	}
	rows.Close()
}

// TestDatabaseService_TenantPool_IsCreatedLazily verifies that GetPoolForTenant
// creates a new pool on first call and returns the same cached instance on
// subsequent calls (double-checked locking pattern).
func TestDatabaseService_TenantPool_IsCreatedLazily(t *testing.T) {
	os.Setenv("SKIP_MIGRATIONS", "true")
	defer os.Unsetenv("SKIP_MIGRATIONS")

	svc := coreServices.NewDatabaseService()
	ctx := context.Background()
	svc.InitDatabase(ctx)
	defer svc.CloseDatabase(ctx)

	// Reuse the test DB URL as "tenant DB" — avoids needing a second real DB
	tenantURL := os.Getenv("DATABASE_URL")
	if tenantURL == "" {
		t.Skip("DATABASE_URL not set — skipping tenant pool test")
	}

	pool1, err := svc.GetPoolForTenant(ctx, tenantURL)
	if err != nil {
		t.Fatalf("GetPoolForTenant first call failed: %v", err)
	}
	if pool1 == nil {
		t.Fatal("GetPoolForTenant returned nil pool")
	}

	pool2, err := svc.GetPoolForTenant(ctx, tenantURL)
	if err != nil {
		t.Fatalf("GetPoolForTenant second call (cached) failed: %v", err)
	}

	// Same URL → same cached pool pointer
	if pool1 != pool2 {
		t.Error("expected the same pool instance on second call (pool should be cached)")
	}

	// Tenant pool and primary pool are different instances even if they point
	// to the same database, because they are created separately.
	if svc.GetPrimaryPool() == pool1 {
		t.Error("tenant pool should be a separate instance from the primary pool")
	}
}

// TestDatabaseService_Query_UsesTenantPoolWhenContextHasURL verifies that
// DatabaseService.resolvePool dispatches to the tenant pool when the context
// contains TenantDatabaseURLKey, and to the primary pool when it does not.
func TestDatabaseService_Query_UsesTenantPoolWhenContextHasURL(t *testing.T) {
	os.Setenv("SKIP_MIGRATIONS", "true")
	defer os.Unsetenv("SKIP_MIGRATIONS")

	svc := coreServices.NewDatabaseService()
	ctx := context.Background()
	svc.InitDatabase(ctx)
	defer svc.CloseDatabase(ctx)

	// --- Primary pool path ---
	// Plain context with no tenant key → uses ds.pool (primary)
	rows, err := svc.Query(ctx, "SELECT current_database()")
	if err != nil {
		t.Fatalf("query without tenant context failed: %v", err)
	}
	var primaryDB string
	for rows.Next() {
		_ = rows.Scan(&primaryDB)
	}
	rows.Close()

	if primaryDB == "" {
		t.Fatal("could not read current_database() from primary pool")
	}

	// --- Tenant pool path (invalid URL) ---
	// Injecting an unreachable URL should cause resolvePool → GetPoolForTenant
	// to fail with a connection error, proving that the tenant path is taken.
	invalidURL := "postgres://nobody:wrong@127.0.0.1:9999/nonexistent?connect_timeout=1"
	tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, invalidURL)

	_, err = svc.Query(tenantCtx, "SELECT 1")
	if err == nil {
		t.Error("expected error when context has unreachable tenant DB URL, but query succeeded")
	}

	t.Logf("tenant pool path correctly returned error: %v", err)

	// --- Tenant pool path (valid URL reusing test DB) ---
	// Confirm that a tenant context with a real URL does reach the DB.
	tenantURL := os.Getenv("DATABASE_URL")
	if tenantURL == "" {
		t.Skip("DATABASE_URL not set — skipping valid tenant URL sub-test")
	}

	tenantCtx2 := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenantURL)
	rows2, err := svc.Query(tenantCtx2, "SELECT current_database()")
	if err != nil {
		t.Fatalf("query with valid tenant URL in context failed: %v", err)
	}
	var tenantDB string
	for rows2.Next() {
		_ = rows2.Scan(&tenantDB)
	}
	rows2.Close()

	if tenantDB == "" {
		t.Fatal("could not read current_database() via tenant pool")
	}
	t.Logf("primary pool → %q, tenant pool → %q (same DB in test env)", primaryDB, tenantDB)
}

// TestDatabaseService_ConcurrentTenantPoolCreation verifies that concurrent
// requests for the same tenant URL don't create duplicate pools (the
// double-checked locking in GetPoolForTenant should prevent this).
func TestDatabaseService_ConcurrentTenantPoolCreation(t *testing.T) {
	os.Setenv("SKIP_MIGRATIONS", "true")
	defer os.Unsetenv("SKIP_MIGRATIONS")

	tenantURL := os.Getenv("DATABASE_URL")
	if tenantURL == "" {
		t.Skip("DATABASE_URL not set")
	}

	svc := coreServices.NewDatabaseService()
	ctx := context.Background()
	svc.InitDatabase(ctx)
	defer svc.CloseDatabase(ctx)

	const goroutines = 20
	pools := make(chan interface{}, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			pool, err := svc.GetPoolForTenant(ctx, tenantURL)
			if err != nil {
				pools <- err
			} else {
				pools <- pool
			}
		}()
	}

	var firstPool interface{}
	for i := 0; i < goroutines; i++ {
		result := <-pools
		if err, ok := result.(error); ok {
			t.Fatalf("goroutine failed: %v", err)
		}
		if firstPool == nil {
			firstPool = result
		} else if result != firstPool {
			t.Error("concurrent GetPoolForTenant calls returned different pool instances (race condition)")
		}
	}
}
