package services

import (
	"context"
	"encoding/json"
	"time"

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
	if live.Status == "in_progress" {
		if position, err = livePosition(live.Position); err != nil {
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

// Rider is the rider's live snapshot (MOBILE-010): one query, and the trip's ETA from the same
// eta:{trip} cache as Trip, cut to the rider's own stops. It also answers when that ETA was computed,
// which the gateway compares before sending an eta frame.
func (s *LiveService) Rider(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderLive, time.Time, error) {
	live, pending, err := s.repo.RiderLive(ctx, tenantID, riderID, scopeUserID, guardianUserID)
	if err != nil || live.TripID == nil {
		return live, time.Time{}, err
	}
	position, err := livePosition(live.Position)
	if err != nil {
		return nil, time.Time{}, err
	}
	eta, err := s.eta.ForTrip(ctx, *live.TripID, position, pending)
	if err != nil {
		return nil, time.Time{}, err
	}
	mine := make(map[uuid.UUID]bool, len(live.Stops))
	for _, stop := range live.Stops {
		mine[stop.TripStopID] = true
	}
	for _, e := range eta.Eta {
		if mine[e.TripStopID] {
			live.Eta = append(live.Eta, e)
		}
	}
	return live, eta.ComputedAt, nil
}

// livePosition reads the position the ETA runs from; nil when the vehicle has not reported.
func livePosition(raw json.RawMessage) (*models.LivePosition, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	position := &models.LivePosition{}
	if err := json.Unmarshal(raw, position); err != nil {
		return nil, err
	}
	return position, nil
}

// CachedEta is the trip's ETA while its cache entry lives — what the gateway tries first on every
// position of a watched trip.
func (s *LiveService) CachedEta(ctx context.Context, tripID uuid.UUID) (TripEta, bool) {
	return s.eta.Cached(ctx, tripID)
}

func (s *LiveService) CanSubscribe(ctx context.Context, tenantID, userID uuid.UUID, guardian bool, channel string) (bool, error) {
	return s.repo.CanSubscribe(ctx, tenantID, userID, guardian, channel)
}
