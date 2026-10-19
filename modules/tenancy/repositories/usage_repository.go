package repositories

import (
	"context"
	"encoding/json"
	"time"

	"josex/web/modules/core/services"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// UsageRepository reads and writes tenancy.usage_daily, what each tenant used per day (TRACK-013 D3).
// It always runs on the platform database.
type UsageRepository struct {
	dbService services.DatabaseService
}

// NewUsageRepository creates a UsageRepository.
func NewUsageRepository(dbService services.DatabaseService) *UsageRepository {
	return &UsageRepository{dbService: dbService}
}

// UpsertUsageDaily writes one tenant's day, replacing what a previous pass wrote.
func (r *UsageRepository) UpsertUsageDaily(ctx context.Context, tenantID uuid.UUID, day time.Time, vehiclesActive, tripsCompleted int, positions int64, ridersBoarded int) error {
	_, err := r.dbService.Execute(ctx, `
		SELECT tenancy.sp_usage_daily_upsert(
			p_tenant_id       := $1,
			p_day             := $2::DATE,
			p_vehicles_active := $3,
			p_trips_completed := $4,
			p_positions       := $5,
			p_riders_boarded  := $6
		)
	`, tenantID, day.Format(time.DateOnly), vehiclesActive, tripsCompleted, positions, ridersBoarded)
	return err
}

// ListUsageDaily answers each tenant's days between from and to, one tenant when tenantID is set.
func (r *UsageRepository) ListUsageDaily(ctx context.Context, from, to string, tenantID *uuid.UUID) ([]models.TenantUsage, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT tenant_id, slug, name, days, vehicles_active_max, vehicles_active_avg, trips_completed
		FROM tenancy.sp_list_usage_daily(p_from := $1::DATE, p_to := $2::DATE, p_tenant_id := $3::UUID)
	`, from, to, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.TenantUsage{}
	for rows.Next() {
		var u models.TenantUsage
		var days []byte
		if err := rows.Scan(&u.TenantID, &u.Slug, &u.Name, &days, &u.VehiclesActiveMax, &u.VehiclesActiveAvg, &u.TripsCompleted); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(days, &u.Days); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
