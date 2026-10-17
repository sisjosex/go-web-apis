package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// The share links of TRACK-060. A token is <tenant slug>.<32 random bytes, base64url>: the slug finds
// the operator's database, as a GPS device token's tenant id does, and only the token's sha256 is kept.

// newShareToken makes a token for slug and the hash it is stored under.
func newShareToken(slug string) (string, []byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, err
	}
	token := slug + "." + base64.RawURLEncoding.EncodeToString(secret)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

// ParseShareToken answers the slug a token names and the hash it is stored under; ok false for
// anything that is not <slug>.<secret>.
func ParseShareToken(token string) (slug string, hash []byte, ok bool) {
	slug, secret, found := strings.Cut(token, ".")
	if !found || slug == "" || secret == "" {
		return "", nil, false
	}
	sum := sha256.Sum256([]byte(token))
	return slug, sum[:], true
}

func (s *TrackingService) CreateShareLink(ctx context.Context, tenantID uuid.UUID, tenantSlug string, tripID, riderID *uuid.UUID, userID *uuid.UUID, guardian bool, ttlMinutes *int32) (*models.ShareLinkCreated, error) {
	token, hash, err := newShareToken(tenantSlug)
	if err != nil {
		return nil, err
	}
	via := "web"
	if guardian {
		via = "portal"
	}
	link, err := s.trackingRepo.CreateShareLink(ctx, tenantID, tripID, riderID, userID, via, hash, ttlMinutes)
	if err != nil {
		return nil, err
	}
	link.Token = token
	return link, nil
}

func (s *TrackingService) ListShareLinks(ctx context.Context, tenantID, tripID uuid.UUID) ([]*models.ShareLink, error) {
	return s.trackingRepo.ListShareLinks(ctx, tenantID, tripID)
}

func (s *TrackingService) RevokeShareLink(ctx context.Context, tenantID, linkID uuid.UUID, userID *uuid.UUID, ownOnly bool) error {
	return s.trackingRepo.RevokeShareLink(ctx, tenantID, linkID, userID, ownOnly)
}

// Shared is a share link's trip for the public page, nil when the token opens nothing. Pending
// stops carry the trip's ETA from the eta:{trip} cache only: the page never pays a routing call.
func (s *LiveService) Shared(ctx context.Context, tenantID uuid.UUID, tokenHash []byte) (*models.SharedTrip, error) {
	trip, err := s.repo.SharedTrip(ctx, tenantID, tokenHash)
	if err != nil || trip == nil || trip.Status != "in_progress" {
		return trip, err
	}
	eta, ok := s.eta.Cached(ctx, trip.TripID)
	if !ok {
		return trip, nil
	}
	at := make(map[uuid.UUID]time.Time, len(eta.Eta))
	for _, e := range eta.Eta {
		at[e.TripStopID] = e.EtaAt
	}
	for i := range trip.Stops {
		if t, found := at[trip.Stops[i].TripStopID]; found && trip.Stops[i].Status == "pending" {
			trip.Stops[i].EtaAt = &t
		}
	}
	return trip, nil
}
