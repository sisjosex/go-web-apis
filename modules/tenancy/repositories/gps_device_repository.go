package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"josex/web/modules/core/services"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type gpsDeviceRepository struct {
	dbService services.DatabaseService
}

// NewGPSDeviceRepository reads tenancy.gps_devices — the platform database, always the primary pool.
func NewGPSDeviceRepository(dbService services.DatabaseService) interfaces.GPSDeviceRepository {
	return &gpsDeviceRepository{dbService: dbService}
}

func (r *gpsDeviceRepository) Resolve(ctx context.Context, tenantID uuid.UUID, tokenHash []byte) (*models.TenantAccessInfo, *uuid.UUID, error) {
	access := &models.TenantAccessInfo{UserRole: models.RoleGPSDevice, Permissions: map[string]bool{}}
	var vehicleID uuid.UUID
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tenancy.sp_gps_device_resolve(p_tenant_id := $1, p_token_hash := $2)`, tenantID, tokenHash).Scan(
		&access.TenantID, &access.Slug, &access.Name, &access.DatabaseURL, &access.SchemaName,
		&access.IsActive, &access.IsSuspended, &vehicleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return access, &vehicleID, nil
}

func (r *gpsDeviceRepository) Issue(ctx context.Context, tenantID, vehicleID uuid.UUID, tokenHash []byte) error {
	_, err := r.dbService.Execute(ctx,
		`SELECT tenancy.sp_gps_device_issue(p_tenant_id := $1, p_vehicle_id := $2, p_token_hash := $3)`,
		tenantID, vehicleID, tokenHash)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Message == "tenant.not-found" {
		return errors.New(tenancyErrors.TenantNotFound)
	}
	return err
}

// GPSDeviceToken builds a device token: the tenant it posts for, a dot, the secret in base64url.
// Only its sha256 is stored.
func GPSDeviceToken(tenantID uuid.UUID, secret []byte) string {
	return tenantID.String() + "." + base64.RawURLEncoding.EncodeToString(secret)
}

// ParseGPSDeviceToken answers the tenant a token names and the hash it is stored under; ok false for
// anything that is not a device token.
func ParseGPSDeviceToken(token string) (tenantID uuid.UUID, hash []byte, ok bool) {
	prefix, secret, found := strings.Cut(token, ".")
	tenantID, err := uuid.Parse(prefix)
	if !found || err != nil || secret == "" {
		return uuid.Nil, nil, false
	}
	sum := sha256.Sum256([]byte(token))
	return tenantID, sum[:], true
}
