package services

import (
	"context"
	"encoding/json"

	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// LiveService answers the live snapshots (TRACK-025) and the gateway's subscribe check.
type LiveService struct {
	repo interfaces.LiveRepository
	eta  *EtaService
}

func NewLiveService(repo interfaces.LiveRepository, eta *EtaService) *LiveService {
	return &LiveService{repo: repo, eta: eta}
}

func (s *LiveService) Fleet(ctx context.Context, tenantID uuid.UUID, organizationID *string) ([]models.LiveVehicle, error) {
	return s.repo.LiveFleet(ctx, tenantID, organizationID)
}

// Trip is the trip's live snapshot: detail, last position and the ETA to each pending stop — one
// query, and at most one routing call per 30 s per trip (D1).
func (s *LiveService) Trip(ctx context.Context, tenantID, tripID uuid.UUID) (*models.TripLive, TripEta, error) {
	live, pending, err := s.repo.TripLive(ctx, tenantID, tripID)
	if err != nil {
		return nil, TripEta{}, err
	}
	var position *models.LivePosition
	if live.Status == "in_progress" && len(live.Position) > 0 && string(live.Position) != "null" {
		position = &models.LivePosition{}
		if err := json.Unmarshal(live.Position, position); err != nil {
			return nil, TripEta{}, err
		}
	}
	eta, err := s.eta.ForTrip(ctx, tripID, position, pending)
	if err != nil {
		return nil, TripEta{}, err
	}
	live.Eta = eta.Eta
	return live, eta, nil
}

// CachedEta is the trip's ETA while its cache entry lives — what the gateway tries first on every
// position of a watched trip.
func (s *LiveService) CachedEta(ctx context.Context, tripID uuid.UUID) (TripEta, bool) {
	return s.eta.Cached(ctx, tripID)
}

func (s *LiveService) CanSubscribe(ctx context.Context, tenantID, userID uuid.UUID, guardian bool, channel string) (bool, error) {
	return s.repo.CanSubscribe(ctx, tenantID, userID, guardian, channel)
}
