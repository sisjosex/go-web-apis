package repositories

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"josex/web/modules/geo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PrimaryPool is all the repository needs from core's DatabaseService. OSM is the same for every
// tenant, so geo.places lives in the server's own database, never in the tenant database a request
// may carry. Resolved per call: the server connects in the background after routes are mounted.
type PrimaryPool interface {
	GetPrimaryPool() *pgxpool.Pool
}

// PlacesRepository reads and replaces geo.places (D2).
type PlacesRepository struct {
	db PrimaryPool
}

func NewPlacesRepository(db PrimaryPool) *PlacesRepository {
	return &PlacesRepository{db: db}
}

func (r *PlacesRepository) Search(ctx context.Context, q string, lat, lng *float64, limit int) ([]models.Place, error) {
	rows, err := r.db.GetPrimaryPool().Query(ctx,
		`SELECT name, kind, city, lat, lng
		 FROM geo.sp_search_places(p_q := $1, p_lat := $2, p_lng := $3, p_limit := $4)`, q, lat, lng, limit)
	if err != nil {
		return nil, err
	}
	places, err := pgx.CollectRows(rows, scanPlace)
	if err != nil {
		return nil, err
	}
	return places, nil
}

// Reverse returns nil, nil when nothing is within reach of the point.
func (r *PlacesRepository) Reverse(ctx context.Context, lat, lng float64) (*models.Place, error) {
	rows, err := r.db.GetPrimaryPool().Query(ctx,
		`SELECT name, kind, city, lat, lng FROM geo.sp_reverse_place(p_lat := $1, p_lng := $2)`, lat, lng)
	if err != nil {
		return nil, err
	}
	place, err := pgx.CollectOneRow(rows, scanPlace)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &place, nil
}

func scanPlace(row pgx.CollectableRow) (models.Place, error) {
	var p models.Place
	err := row.Scan(&p.Name, &p.Kind, &p.City, &p.Lat, &p.Lng)
	return p, err
}

// importColumns are geo.places_import's, in COPY order.
var importColumns = []string{"name", "highway", "place", "amenity", "shop", "city", "geometry"}

// feature is one line of places.geojsonl as `osmium export` writes it.
type feature struct {
	Geometry   json.RawMessage   `json:"geometry"`
	Properties map[string]string `json:"properties"`
}

// Import replaces geo.places with the features in r, one GeoJSON Feature per line, and returns
// how many places resulted. One transaction: a search sees the old set or the new one, never half.
func (r *PlacesRepository) Import(ctx context.Context, src io.Reader) (int64, error) {
	tx, err := r.db.GetPrimaryPool().Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `TRUNCATE geo.places_import`); err != nil {
		return 0, err
	}

	// ReadBytes, not a Scanner: one region polygon can be a line of several MB. Rows stream into
	// COPY, so a 200 k-line file never sits in memory whole.
	lines := bufio.NewReaderSize(src, 1<<20)
	line := 0
	next := func() ([]any, error) {
		raw, err := lines.ReadBytes('\n')
		if len(raw) == 0 {
			if errors.Is(err, io.EOF) {
				return nil, nil
			}
			return nil, err
		}
		line++
		var f feature
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		p := f.Properties
		return []any{
			p["name"], nullable(p["highway"]), nullable(p["place"]), nullable(p["amenity"]),
			nullable(p["shop"]), nullable(p["addr:city"]), string(f.Geometry),
		}, nil
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"geo", "places_import"}, importColumns, pgx.CopyFromFunc(next)); err != nil {
		return 0, err
	}

	var count int64
	if err := tx.QueryRow(ctx, `SELECT geo.sp_replace_places()`).Scan(&count); err != nil {
		return 0, err
	}
	return count, tx.Commit(ctx)
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
