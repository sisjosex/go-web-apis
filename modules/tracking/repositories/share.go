package repositories

import (
	"context"
	"encoding/json"
	"errors"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The share links of TRACK-060: one slice, one file.

var shareCodes = map[string]string{
	"share.not-found": trackingErrors.ShareNotFound,
	"share.no-trip":   trackingErrors.ShareNoTrip,
	"trip.not-found":  trackingErrors.TripNotFound,
}

func mapShareError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := shareCodes[pgErr.Message]; ok {
			return &trackingErrors.TrackingError{Code: code, Err: pgErr}
		}
	}
	return scopedErr(err, fallbackCode)
}

// CreateShareLink stores tokenHash for a trip, or for riderID's trip of today when tripID is nil.
func (r *TrackingRepository) CreateShareLink(ctx context.Context, tenantID uuid.UUID, tripID, riderID *uuid.UUID, userID *uuid.UUID, via string, tokenHash []byte, ttlMinutes *int32) (*models.ShareLinkCreated, error) {
	var link models.ShareLinkCreated
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_share_link_create(
			p_tenant_id   := $1,
			p_trip_id     := $2,
			p_rider_id    := $3,
			p_user_id     := $4,
			p_via         := $5,
			p_token_hash  := $6,
			p_ttl_minutes := $7
		)
	`, tenantID, tripID, riderID, userID, via, tokenHash, ttlMinutes).Scan(&link.ID, &link.TripID, &link.ExpiresAt)
	if err != nil {
		return nil, mapShareError(err, trackingErrors.ShareCreateFailed)
	}
	return &link, nil
}

func (r *TrackingRepository) ListShareLinks(ctx context.Context, tenantID, tripID uuid.UUID) ([]*models.ShareLink, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_share_links_list(p_tenant_id := $1, p_trip_id := $2)
	`, tenantID, tripID)
	if err != nil {
		return nil, mapShareError(err, trackingErrors.ShareListFailed)
	}
	defer rows.Close()
	links := []*models.ShareLink{}
	for rows.Next() {
		var l models.ShareLink
		if err := rows.Scan(&l.ID, &l.CreatedByName, &l.CreatedVia, &l.ExpiresAt, &l.CreatedAt); err != nil {
			return nil, err
		}
		links = append(links, &l)
	}
	if err := rows.Err(); err != nil {
		return nil, mapShareError(err, trackingErrors.ShareListFailed)
	}
	return links, nil
}

// RevokeShareLink closes a link; ownOnly (a guardian) closes only one userID made.
func (r *TrackingRepository) RevokeShareLink(ctx context.Context, tenantID, linkID uuid.UUID, userID *uuid.UUID, ownOnly bool) error {
	_, err := r.dbService.Execute(ctx, `
		SELECT tracking.sp_share_link_revoke(
			p_tenant_id := $1,
			p_link_id   := $2,
			p_user_id   := $3,
			p_own_only  := $4
		)
	`, tenantID, linkID, userID, ownOnly)
	if err != nil {
		return mapShareError(err, trackingErrors.ShareRevokeFailed)
	}
	return nil
}

// SharedTrip resolves the token and reads its trip in one statement; nil, nil when the link is
// unknown, expired or revoked.
func (r *TrackingRepository) SharedTrip(ctx context.Context, tenantID uuid.UUID, tokenHash []byte) (*models.SharedTrip, error) {
	var trip models.SharedTrip
	var stops, position, path, destination []byte
	err := r.dbService.QueryRow(ctx, `
		SELECT l.trip_id, l.expires_at, p.*
		FROM tracking.sp_share_link_resolve(p_tenant_id := $1, p_token_hash := $2) l
		CROSS JOIN LATERAL tracking.sp_share_trip_public(p_tenant_id := $1, p_trip_id := l.trip_id, p_rider_id := l.rider_id) p
	`, tenantID, tokenHash).Scan(&trip.TripID, &trip.ExpiresAt, &trip.Scope, &trip.Status, &trip.RouteName, &trip.LicensePlate,
		&trip.Direction, &trip.ServiceDate, &trip.Timezone, &trip.PlannedStart, &trip.StartedAt, &trip.EndedAt,
		&trip.RiderFirstName, &stops, &position, &path, &destination)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapShareError(err, trackingErrors.ShareReadFailed)
	}
	if err := json.Unmarshal(stops, &trip.Stops); err != nil {
		return nil, err
	}
	if position != nil {
		trip.Position = &models.SharedPosition{}
		if err := json.Unmarshal(position, trip.Position); err != nil {
			return nil, err
		}
	}
	var story models.LiveStory
	if err := scanStory(&story, path, destination); err != nil {
		return nil, err
	}
	trip.PlannedPath, trip.Destination = story.PlannedPath, story.Destination
	return &trip, nil
}
