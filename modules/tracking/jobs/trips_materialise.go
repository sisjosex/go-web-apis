package jobs

import (
	"context"
	"fmt"
	"log"

	coreJobs "josex/web/modules/core/jobs"
	coreServices "josex/web/modules/core/services"
	trackingRepos "josex/web/modules/tracking/repositories"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	// TaskTripsMaterialise builds every tenant's trips for the next two weeks; the scheduler fires it
	// daily.
	TaskTripsMaterialise = "tracking:trips-materialise"
	// TripsMaterialiseCron is 02:00 UTC. The window is each route's own local today..today+14, so the
	// hour only decides how early a new day's fifteenth trip exists; route.changed covers the rest.
	TripsMaterialiseCron = "0 2 * * *"
)

// MaterialiseResult is one tenant's share of a pass.
type MaterialiseResult struct {
	Tenant coreJobs.Tenant
	Trips  int
	Err    error
}

// RunTripsMaterialisePass calls the materialiser once per tenant, over all of its routes: one SP call
// per tenant, never one per route. A tenant that fails is reported and the walk continues.
func RunTripsMaterialisePass(ctx context.Context, db coreServices.DatabaseService, tenants []coreJobs.Tenant) []MaterialiseResult {
	repo := trackingRepos.NewTrackingRepository(db)
	results := make([]MaterialiseResult, 0, len(tenants))
	for _, tenant := range tenants {
		result := MaterialiseResult{Tenant: tenant}
		tenantID, err := uuid.Parse(tenant.ID)
		if err == nil {
			tenantCtx := context.WithValue(ctx, coreServices.TenantDatabaseURLKey, tenant.DatabaseURL)
			result.Trips, err = repo.MaterialiseTrips(tenantCtx, tenantID, nil, nil, nil)
		}
		result.Err = err
		results = append(results, result)
	}
	return results
}

// TripsMaterialiseHandler is the TaskTripsMaterialise handler: the pass over whatever tenants exist
// when it fires.
func TripsMaterialiseHandler(db coreServices.DatabaseService, tenants coreJobs.TenantLister) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		list, err := tenants(ctx)
		if err != nil {
			return err
		}
		failed := 0
		for _, r := range RunTripsMaterialisePass(ctx, db, list) {
			if r.Err != nil {
				log.Printf("⚠️  trips materialise %s: %v", r.Tenant.ID, r.Err)
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("trips materialise failed for %d of %d tenants", failed, len(list))
		}
		return nil
	}
}
