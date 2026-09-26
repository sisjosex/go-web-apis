package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// driverService is the driver's phone of TRACK-011.
type driverService struct {
	repo interfaces.DriverRepository
}

func NewDriverService(repo interfaces.DriverRepository) interfaces.DriverService {
	return &driverService{repo: repo}
}

// Today hashes the day as the database wrote it — jsonb's text is canonical, so the same data is the
// same ETag — and stamps generated_at after, outside the hash.
func (s *driverService) Today(ctx context.Context, tenantID, userID uuid.UUID) (*models.DriverToday, string, error) {
	raw, err := s.repo.DriverToday(ctx, tenantID, userID)
	if err != nil {
		return nil, "", err
	}
	var day models.DriverToday
	if err := json.Unmarshal(raw, &day); err != nil {
		return nil, "", &trackingErrors.TrackingError{Code: trackingErrors.DriverTodayFailed, Err: err}
	}
	day.GeneratedAt = time.Now().UTC()
	sum := sha256.Sum256(raw)
	return &day, `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func (s *driverService) StartTrip(ctx context.Context, tenantID, userID, tripID uuid.UUID) (*models.TripDetail, error) {
	return s.repo.DriverStartTrip(ctx, tenantID, userID, tripID)
}

// Sync answers each rejection with its module code: the SP's code under the module's prefix, the same
// code the web endpoints answer for the same refusal.
func (s *driverService) Sync(ctx context.Context, tenantID, userID uuid.UUID, ops []models.DriverSyncOp) ([]models.DriverOpResult, error) {
	payload, err := json.Marshal(ops)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DriverSyncFailed, Err: err}
	}
	results, err := s.repo.DriverSync(ctx, tenantID, userID, payload)
	if err != nil {
		return nil, err
	}
	for i := range results {
		if results[i].Code != nil {
			code := "tracking." + *results[i].Code
			results[i].Code = &code
		}
	}
	return results, nil
}

func (s *driverService) ResolveRider(ctx context.Context, tenantID, userID uuid.UUID, code string) (*models.DriverRider, error) {
	return s.repo.DriverResolveRider(ctx, tenantID, userID, code)
}

func (s *driverService) FindRiders(ctx context.Context, tenantID, userID uuid.UUID, query string) ([]models.DriverRider, error) {
	return s.repo.DriverFindRiders(ctx, tenantID, userID, query)
}

func (s *driverService) ReportIncident(ctx context.Context, tenantID, userID uuid.UUID, dto *models.DriverIncidentDto) (*models.DriverIncident, error) {
	return s.repo.DriverIncident(ctx, tenantID, userID, dto)
}
