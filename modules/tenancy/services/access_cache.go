package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// INFRA-011 D2: a request's tenant access — membership, role, permissions — is read from Valkey, shared
// by every replica, for accessTTL; a membership write deletes the user's entry, so a removed member is
// refused on the next request on any replica. A miss reads sp_verify_user_tenant_access as before, and
// without Valkey every request does.

// accessTTL bounds a change no write deletes (a custom role's permissions edited) to 30 s.
const accessTTL = 30 * time.Second

var accessValkey coreServices.ValkeyService

// UseAccessCache turns the cache on; nil keeps every request on the database.
func UseAccessCache(valkey coreServices.ValkeyService) { accessValkey = valkey }

// accessKey is one hash per user, a field per workspace slug, so one DEL forgets all of them.
func accessKey(userID uuid.UUID) string { return "tenant-access:" + userID.String() }

// cachedAccessRow is TenantAccessInfo with the fields it keeps out of its JSON.
type cachedAccessRow struct {
	models.TenantAccessInfo
	DatabaseURL *string         `json:"database_url"`
	Permissions map[string]bool `json:"permissions"`
}

func loadAccess(ctx context.Context, userID uuid.UUID, slug string) (*models.TenantAccessInfo, bool) {
	if accessValkey == nil {
		return nil, false
	}
	raw, err := accessValkey.Client().HGet(ctx, accessKey(userID), slug).Bytes()
	if err != nil {
		return nil, false
	}
	var row cachedAccessRow
	if json.Unmarshal(raw, &row) != nil {
		return nil, false
	}
	info := row.TenantAccessInfo
	info.DatabaseURL = row.DatabaseURL
	info.Permissions = row.Permissions
	return &info, true
}

func storeAccess(ctx context.Context, userID uuid.UUID, slug string, info *models.TenantAccessInfo) {
	if accessValkey == nil {
		return
	}
	raw, err := json.Marshal(cachedAccessRow{TenantAccessInfo: *info, DatabaseURL: info.DatabaseURL, Permissions: info.Permissions})
	if err != nil {
		return
	}
	pipe := accessValkey.Client().TxPipeline()
	pipe.HSet(ctx, accessKey(userID), slug, raw)
	pipe.Expire(ctx, accessKey(userID), accessTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("⚠️  tenant access cache: %v", err)
	}
}

// InvalidateAccess forgets every workspace's access of a user; their next request reads it again.
func InvalidateAccess(ctx context.Context, userID uuid.UUID) {
	if accessValkey == nil {
		return
	}
	if err := accessValkey.Client().Del(ctx, accessKey(userID)).Err(); err != nil {
		log.Printf("⚠️  tenant access cache: invalidate %s: %v", userID, err)
	}
}

// tenantKey is a workspace by slug, for the chains with no user (TRACK-060): kept accessTTL, so a
// suspension reaches them within 30 s.
func tenantKey(slug string) string { return "tenant-slug:" + slug }

func loadTenant(ctx context.Context, slug string) (*models.Tenant, bool) {
	if accessValkey == nil {
		return nil, false
	}
	raw, err := accessValkey.Client().Get(ctx, tenantKey(slug)).Bytes()
	if err != nil {
		return nil, false
	}
	var tenant models.Tenant
	if json.Unmarshal(raw, &tenant) != nil {
		return nil, false
	}
	return &tenant, true
}

func storeTenant(ctx context.Context, slug string, tenant *models.Tenant) {
	if accessValkey == nil {
		return
	}
	raw, err := json.Marshal(tenant)
	if err != nil {
		return
	}
	if err := accessValkey.Client().Set(ctx, tenantKey(slug), raw, accessTTL).Err(); err != nil {
		log.Printf("⚠️  tenant cache: %v", err)
	}
}

// forgetTenant drops a workspace's cached row, so its next read on any replica sees a write (TENANCY-007 D1).
func forgetTenant(ctx context.Context, slug string) {
	if accessValkey == nil {
		return
	}
	if err := accessValkey.Client().Del(ctx, tenantKey(slug)).Err(); err != nil {
		log.Printf("⚠️  tenant cache: forget %s: %v", slug, err)
	}
}
