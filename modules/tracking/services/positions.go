package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// GPSStreamField is the one field of a stream entry: the point as sp_ingest_positions takes it.
const GPSStreamField = "p"

// queueTimeout bounds the XADD pipeline: past it the batch is stored inline instead (D2), so a Valkey
// that hangs costs a request this much, not go-redis' default timeouts.
const queueTimeout = 500 * time.Millisecond

// GPSStream is the tenant's stream key (D2): one per tenant, so one consumer goroutine per tenant
// drains it into that tenant's database.
func GPSStream(tenantID string) string {
	return "gps:" + tenantID
}

// PositionIngest accepts batches: to the tenant's stream when Valkey is there, straight into the
// database when it is not.
type PositionIngest struct {
	repo   interfaces.PositionsRepository
	valkey coreServices.ValkeyService
	maxLen int64
	radii  models.IngestRadii
}

func NewPositionIngest(repo interfaces.PositionsRepository, valkey coreServices.ValkeyService, maxLen int64, radii models.IngestRadii) *PositionIngest {
	return &PositionIngest{repo: repo, valkey: valkey, maxLen: maxLen, radii: radii}
}

// DriverVehicle answers the vehicle of the driver account's trip in progress, nil when none.
func (s *PositionIngest) DriverVehicle(ctx context.Context, tenantID, userID uuid.UUID) (*uuid.UUID, error) {
	return s.repo.DriverActiveVehicle(ctx, tenantID, userID)
}

// Ingest queues the points — one XADD per point, pipelined into one round-trip, trimmed to about
// maxLen — and answers queued; with Valkey absent or failing it stores them in one SP call and
// answers stored. Nothing is lost either way.
func (s *PositionIngest) Ingest(ctx context.Context, tenantID uuid.UUID, points []models.StoredPoint) (string, error) {
	if s.valkey != nil {
		err := s.queue(ctx, tenantID, points)
		if err == nil {
			return models.IngestPathQueued, nil
		}
		log.Printf("⚠️  gps ingest %s: queue failed, storing inline: %v", tenantID, err)
	}
	payload, err := json.Marshal(points)
	if err != nil {
		return "", err
	}
	if _, err := s.repo.IngestPositions(ctx, tenantID, payload, s.radii); err != nil {
		return "", err
	}
	return models.IngestPathStored, nil
}

func (s *PositionIngest) queue(ctx context.Context, tenantID uuid.UUID, points []models.StoredPoint) error {
	ctx, cancel := context.WithTimeout(ctx, queueTimeout)
	defer cancel()
	stream := GPSStream(tenantID.String())
	pipe := s.valkey.Client().Pipeline()
	for _, p := range points {
		encoded, err := json.Marshal(p)
		if err != nil {
			return err
		}
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			MaxLen: s.maxLen,
			Approx: true,
			Values: []string{GPSStreamField, string(encoded)},
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}
