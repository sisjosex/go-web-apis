package controllers

import (
	"context"
	"net/http"
	"time"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

const (
	usageDefaultDays = 30
	usageMaxDays     = 366
)

// UsageReader is what GET /platform/usage reads: tenancy.usage_daily (TRACK-013 D3).
type UsageReader interface {
	ListUsageDaily(ctx context.Context, from, to string, tenantID *uuid.UUID) ([]models.TenantUsage, error)
}

// PlatformUsageController answers the platform's read of what each tenant used.
type PlatformUsageController struct {
	usage UsageReader
}

// NewPlatformUsageController creates a PlatformUsageController.
func NewPlatformUsageController(usage UsageReader) *PlatformUsageController {
	return &PlatformUsageController{usage: usage}
}

// ListUsageInput is ListUsage's request.
type ListUsageInput struct {
	Query api.Query[models.PlatformUsageQuery]
}

// ListUsage is GET /usage: What each business used per day.
func (pc *PlatformUsageController) ListUsage(ctx context.Context, in *ListUsageInput) (*api.Out[models.PlatformUsageResponse], error) {
	c := api.Gin(ctx)
	var query models.PlatformUsageQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.UsageListFailed, utils.ExtractValidationError(c, err)))
	}
	to := time.Now().UTC().Truncate(24 * time.Hour)
	if query.To != nil {
		to, _ = time.Parse(time.DateOnly, *query.To)
	}
	from := to.AddDate(0, 0, 1-usageDefaultDays)
	if query.From != nil {
		from, _ = time.Parse(time.DateOnly, *query.From)
	}
	switch {
	case to.Before(from):
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.UsageRangeInvalid))
	case to.Sub(from) > usageMaxDays*24*time.Hour:
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.UsageRangeTooWide))
	}
	var tenantID *uuid.UUID
	if query.TenantID != nil {
		id := uuid.MustParse(*query.TenantID)
		tenantID = &id
	}
	tenants, err := pc.usage.ListUsageDaily(c.Request.Context(), from.Format(time.DateOnly), to.Format(time.DateOnly), tenantID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(models.PlatformUsageResponse{Tenants: tenants})
}
