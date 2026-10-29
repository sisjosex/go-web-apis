package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// RiderReadsService is the guardian's rider reads (TRACK-061): the day, the history, the cut-offs.
type RiderReadsService struct {
	repo interfaces.RiderReadsRepository
	live *LiveService
}

func NewRiderReadsService(repo interfaces.RiderReadsRepository, live *LiveService) *RiderReadsService {
	return &RiderReadsService{repo: repo, live: live}
}

// Today answers the rider's day as JSON and its ETag, the hash of that body. The trip in progress
// gets its ETA only inside TRACK-030 D1's window — pickup pending or done, dropoff neither done nor
// no_show; a failed estimate is logged and the day still answers.
func (s *RiderReadsService) Today(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID) ([]byte, string, error) {
	day, err := s.repo.RiderToday(ctx, tenantID, riderID, scopeUserID, guardianUserID)
	if err != nil {
		return nil, "", err
	}
	for i := range day.Trips {
		if trip := &day.Trips[i]; etaWindow(trip) {
			s.fillEta(ctx, tenantID, riderID, scopeUserID, guardianUserID, trip)
			break
		}
	}
	body, err := json.Marshal(day)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	return body, `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func etaWindow(trip *models.RiderTodayTrip) bool {
	if trip.Status != "in_progress" || trip.Pickup == nil {
		return false
	}
	if trip.Pickup.TaskStatus != "pending" && trip.Pickup.TaskStatus != "done" {
		return false
	}
	return trip.Dropoff == nil || (trip.Dropoff.TaskStatus != "done" && trip.Dropoff.TaskStatus != "no_show")
}

// fillEta reads the trip's stop estimates into the rider's pickup and drop-off.
func (s *RiderReadsService) fillEta(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID, trip *models.RiderTodayTrip) {
	stops := s.tripEtas(ctx, tenantID, riderID, scopeUserID, guardianUserID, trip.TripID)
	for _, e := range stops {
		for _, stop := range []*models.RiderTodayStop{trip.Pickup, trip.Dropoff} {
			// A cached estimate may still list a stop the rider has just been picked up at.
			if stop != nil && stop.TaskStatus == "pending" && stop.TripStopID == e.TripStopID {
				at := e.EtaAt
				stop.EtaAt = &at
			}
		}
	}
}

// tripEtas reads the trip's shared eta:{trip} entry, and only when it is cold computes it through the
// rider's live snapshot, as the guardian's map does; nil when neither has the trip.
func (s *RiderReadsService) tripEtas(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID, guardianUserID *uuid.UUID, tripID uuid.UUID) []models.StopEta {
	if eta, ok := s.live.CachedEta(ctx, tripID); ok {
		return eta.Eta
	}
	live, _, err := s.live.Rider(ctx, tenantID, riderID, scopeUserID, guardianUserID)
	if err != nil {
		log.Printf("⚠️  rider eta %s: %v", tripID, err)
		return nil
	}
	if live.TripID == nil || *live.TripID != tripID {
		return nil
	}
	return live.Eta
}

// StopEta is when the bus is expected at one of the rider's stops on a running trip — the guardian home's
// "8 min" (MOBILE-038 D1); nil without an estimate.
func (s *RiderReadsService) StopEta(ctx context.Context, tenantID, riderID, guardianUserID, tripID, tripStopID uuid.UUID) *time.Time {
	for _, e := range s.tripEtas(ctx, tenantID, riderID, nil, &guardianUserID, tripID) {
		if e.TripStopID == tripStopID {
			at := e.EtaAt
			return &at
		}
	}
	return nil
}

func (s *RiderReadsService) History(ctx context.Context, tenantID, riderID uuid.UUID, query models.RiderDaysQuery, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderHistory, error) {
	return s.repo.RiderHistory(ctx, tenantID, riderID, query, scopeUserID, guardianUserID)
}

func (s *RiderReadsService) AbsenceCutoffs(ctx context.Context, tenantID, riderID uuid.UUID, query models.RiderDaysQuery, scopeUserID, guardianUserID *uuid.UUID) (*models.RiderAbsenceCutoffs, error) {
	return s.repo.RiderAbsenceCutoffs(ctx, tenantID, riderID, query, scopeUserID, guardianUserID)
}
