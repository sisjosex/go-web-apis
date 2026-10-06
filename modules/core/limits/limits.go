// Package limits is how a module asks for a tenant's plan limit and answers when it is reached
// (BILLING-001 D2) without importing billing: billing registers the resolver at startup, a create
// passes the limit to public.fn_within_limit in the same statement, and the controller turns the
// refusal into 403 billing.limit-reached naming the feature.
package limits

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	coreErrors "josex/web/modules/core/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// Code is the refusal every module answers when a plan limit stops a create.
const Code = "billing.limit-reached"

// The limited features, as billing.plan_limits names them.
const (
	TrackingRiders   = "tracking_riders"
	TrackingVehicles = "tracking_vehicles"
	InventoryItems   = "inventory_items"
	UsersPerTenant   = "users_per_tenant"
)

// Resolver answers a tenant's limit for a feature; -1 is unlimited.
type Resolver func(ctx context.Context, tenantID uuid.UUID, feature string) (int, error)

var resolver Resolver

// SetResolver installs billing's resolver; without one every limit is unlimited.
func SetResolver(r Resolver) { resolver = r }

// For answers the tenant's limit for feature, or -1. A failed read fails open: a plan lookup outage
// must not stop the business from working.
func For(ctx context.Context, tenantID uuid.UUID, feature string) int {
	if resolver == nil {
		return -1
	}
	limit, err := resolver(ctx, tenantID, feature)
	if err != nil {
		log.Printf("⚠️  limits: %s for %s: %v", feature, tenantID, err)
		return -1
	}
	return limit
}

// Detail is what the refusal carries: which limit, how high, how much is used.
type Detail struct {
	Feature string `json:"feature"`
	Limit   int    `json:"limit"`
	Used    int64  `json:"used"`
}

// Reached reports whether err is fn_within_limit's refusal, with its detail.
func Reached(err error) (*Detail, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Message != Code {
		return nil, false
	}
	var detail Detail
	_ = json.Unmarshal([]byte(pgErr.Detail), &detail)
	return &detail, true
}

// Respond answers 403 billing.limit-reached when err is that refusal, and reports whether it did.
func Respond(c *gin.Context, err error) bool {
	detail, ok := Reached(err)
	if !ok {
		return false
	}
	c.JSON(http.StatusForbidden, coreErrors.BuildErrorDetail(c, Code, detail))
	return true
}
