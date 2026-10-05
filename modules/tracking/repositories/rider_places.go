package repositories

import (
	"context"
	"errors"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The rider places slice of TRACK-044 D4: one slice, one file.

var riderPlaceCodes = map[string]string{
	"rider-place.not-found": trackingErrors.RiderPlaceNotFound,
	"rider-place.default":   trackingErrors.RiderPlaceDefault,
}

func mapRiderPlaceError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if code, ok := riderPlaceCodes[pgErr.Message]; ok {
			return &trackingErrors.TrackingError{Code: code, Err: pgErr}
		}
	}
	return scopedErr(err, fallbackCode)
}

func scanRiderPlaces(rows pgx.Rows) ([]*models.RiderPlace, error) {
	defer rows.Close()
	places := []*models.RiderPlace{}
	for rows.Next() {
		var p models.RiderPlace
		if err := rows.Scan(&p.ID, &p.Label, &p.Address, &p.Latitude, &p.Longitude, &p.IsDefault); err != nil {
			return nil, err
		}
		places = append(places, &p)
	}
	return places, rows.Err()
}

func (r *TrackingRepository) ListRiderPlaces(ctx context.Context, tenantID, riderID uuid.UUID, scopeUserID *uuid.UUID) ([]*models.RiderPlace, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_rider_places($1, $2, $3)`, tenantID, riderID, scopeUserID)
	if err != nil {
		return nil, mapRiderPlaceError(err, trackingErrors.RiderPlaceListFailed)
	}
	places, err := scanRiderPlaces(rows)
	if err != nil {
		return nil, mapRiderPlaceError(err, trackingErrors.RiderPlaceListFailed)
	}
	return places, nil
}

func (r *TrackingRepository) CreateRiderPlace(ctx context.Context, tenantID, riderID uuid.UUID, dto *models.CreateRiderPlaceDto, scopeUserID *uuid.UUID) (*models.RiderPlace, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_create_rider_place(
			p_tenant_id     := $1,
			p_rider_id      := $2,
			p_label         := $3,
			p_address       := $4,
			p_latitude      := $5,
			p_longitude     := $6,
			p_is_default    := $7,
			p_scope_user_id := $8
		)
	`, tenantID, riderID, dto.Label, dto.Address, dto.Latitude, dto.Longitude, dto.IsDefault, scopeUserID)
	if err != nil {
		return nil, mapRiderPlaceError(err, trackingErrors.RiderPlaceCreateFailed)
	}
	places, err := scanRiderPlaces(rows)
	if err != nil {
		return nil, mapRiderPlaceError(err, trackingErrors.RiderPlaceCreateFailed)
	}
	return places[0], nil
}

func (r *TrackingRepository) UpdateRiderPlace(ctx context.Context, tenantID, riderID, placeID uuid.UUID, dto *models.UpdateRiderPlaceDto, scopeUserID *uuid.UUID) (*models.RiderPlace, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_update_rider_place(
			p_tenant_id     := $1,
			p_rider_id      := $2,
			p_place_id      := $3,
			p_label         := $4,
			p_address       := $5,
			p_latitude      := $6,
			p_longitude     := $7,
			p_is_default    := $8,
			p_scope_user_id := $9
		)
	`, tenantID, riderID, placeID, dto.Label, dto.Address, dto.Latitude, dto.Longitude, dto.IsDefault, scopeUserID)
	if err != nil {
		return nil, mapRiderPlaceError(err, trackingErrors.RiderPlaceUpdateFailed)
	}
	places, err := scanRiderPlaces(rows)
	if err != nil {
		return nil, mapRiderPlaceError(err, trackingErrors.RiderPlaceUpdateFailed)
	}
	return places[0], nil
}

func (r *TrackingRepository) DeleteRiderPlace(ctx context.Context, tenantID, riderID, placeID uuid.UUID, scopeUserID *uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `SELECT tracking.sp_delete_rider_place($1, $2, $3, $4)`, tenantID, riderID, placeID, scopeUserID)
	if err != nil {
		return mapRiderPlaceError(err, trackingErrors.RiderPlaceDeleteFailed)
	}
	return nil
}
