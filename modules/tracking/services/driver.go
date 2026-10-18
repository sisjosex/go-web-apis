package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
)

// driverService is the driver's phone of TRACK-011.
type driverService struct {
	repo   interfaces.DriverRepository
	proofs *ProofFiles
}

func NewDriverService(repo interfaces.DriverRepository, proofs *ProofFiles) interfaces.DriverService {
	return &driverService{repo: repo, proofs: proofs}
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
//
// A task.done naming a photo_key is checked first (MOBILE-032): an upload that is not a WebP of the
// tenant within the cap sends { photo_failed } — the drop-off is never refused for its photo. The
// photo is copied to its task's key only once the SP applied the op, so an op refused for another
// driver's task never touches that task's photo.
func (s *driverService) Sync(ctx context.Context, tenantID, userID uuid.UUID, ops []models.DriverSyncOp) ([]models.DriverOpResult, error) {
	pending := map[uuid.UUID][2]string{}
	for i := range ops {
		op := &ops[i]
		photoKey, ok := op.Args.Proof["photo_key"].(string)
		if op.Op != models.DriverOpTaskDone || !ok {
			continue
		}
		key, err := s.proofs.Check(ctx, tenantID, *op.Args.TripStopTaskID, photoKey)
		if err != nil {
			log.Printf("⚠️  tracking: proof %s of op %s: %v", photoKey, op.ClientOpID, err)
			op.Args.Proof = map[string]any{"photo_failed": true}
			continue
		}
		op.Args.Proof = map[string]any{"photo": key}
		pending[*op.ClientOpID] = [2]string{photoKey, key}
	}

	payload, err := json.Marshal(ops)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.DriverSyncFailed, Err: err}
	}
	results, err := s.repo.DriverSync(ctx, tenantID, userID, payload)
	if err != nil {
		return nil, err
	}
	claimed := make([]string, 0, len(pending))
	for i := range results {
		if keys, ok := pending[results[i].ClientOpID]; ok {
			claimed = append(claimed, keys[0])
			if results[i].Status == models.DriverOpApplied {
				if err := s.proofs.Store(ctx, keys[0], keys[1]); err != nil {
					log.Printf("⚠️  tracking: store proof %s: %v", keys[1], err)
				}
			}
		}
		if results[i].Code != nil {
			code := "tracking." + *results[i].Code
			results[i].Code = &code
		}
	}
	s.proofs.DeletePending(ctx, claimed)
	return results, nil
}

// ProofTicket signs the upload of a drop-off photo of size bytes (MOBILE-032 D1).
func (s *driverService) ProofTicket(ctx context.Context, tenantID uuid.UUID, size int64) (*models.DriverProofTicket, error) {
	return s.proofs.Ticket(ctx, tenantID, size)
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
