package interfaces

import (
	"context"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// DriverRepository is the driver's-phone slice of the tracking repository (TRACK-011).
type DriverRepository interface {
	DriverToday(ctx context.Context, tenantID, userID uuid.UUID) ([]byte, error)
	DriverStartTrip(ctx context.Context, tenantID, userID, tripID uuid.UUID) (*models.TripDetail, error)
	DriverSync(ctx context.Context, tenantID, userID uuid.UUID, ops []byte) ([]models.DriverOpResult, error)
	// The rider card and the incident report (TRACK-027).
	DriverResolveRider(ctx context.Context, tenantID, userID uuid.UUID, code string) (*models.DriverRider, error)
	DriverFindRiders(ctx context.Context, tenantID, userID uuid.UUID, query string) ([]models.DriverRider, error)
	DriverIncident(ctx context.Context, tenantID, userID uuid.UUID, dto *models.DriverIncidentDto) (*models.DriverIncident, error)
}

// DriverService is what the /mobile/driver endpoints call. Today answers the day and its ETag, the
// hash of the day's data (generated_at left out, so an unchanged day keeps its ETag).
type DriverService interface {
	Today(ctx context.Context, tenantID, userID uuid.UUID) (*models.DriverToday, string, error)
	StartTrip(ctx context.Context, tenantID, userID, tripID uuid.UUID) (*models.TripDetail, error)
	Sync(ctx context.Context, tenantID, userID uuid.UUID, ops []models.DriverSyncOp) ([]models.DriverOpResult, error)
	ResolveRider(ctx context.Context, tenantID, userID uuid.UUID, code string) (*models.DriverRider, error)
	FindRiders(ctx context.Context, tenantID, userID uuid.UUID, query string) ([]models.DriverRider, error)
	ReportIncident(ctx context.Context, tenantID, userID uuid.UUID, dto *models.DriverIncidentDto) (*models.DriverIncident, error)
}
