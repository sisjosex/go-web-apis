package tracking

import (
	"context"

	"josex/web/modules/core/registry"

	"github.com/google/uuid"
)

// TENANCY-008: the checklist reads tracking's steps through the SP the tracking home already uses (TRACK-050 D2).
func init() {
	registry.RegisterSetupChecker("tracking", func(ctx context.Context, q registry.RowQuerier, tenantID uuid.UUID) (map[string]bool, error) {
		var companies, drivers, vehicles, routes, riders bool
		err := q.QueryRow(ctx, `SELECT * FROM tracking.sp_tracking_setup($1)`, tenantID).Scan(&companies, &drivers, &vehicles, &routes, &riders)
		if err != nil {
			return nil, err
		}
		return map[string]bool{"companies": companies, "drivers": drivers, "vehicles": vehicles, "routes": routes, "riders": riders}, nil
	})
}
