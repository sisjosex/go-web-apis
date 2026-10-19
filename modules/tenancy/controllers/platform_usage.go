package controllers

import (
	"context"
	"net/http"
	"time"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/models"

	"github.com/gin-gonic/gin"
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

// ListUsage godoc
// @Summary What each business used per day
// @Description Per business: its days between from and to (default the last 30, at most 366) with vehicles active (a trip completed or a GPS point), trips completed, GPS points and riders boarded, plus the range's peak and mean active vehicles. super_admin only
// @Tags Platform
// @Produce json
// @Security BearerAuth
// @Param from query string false "First day (YYYY-MM-DD)"
// @Param to query string false "Last day (YYYY-MM-DD)"
// @Param tenant_id query string false "One business (UUID)"
// @Success 200 {object} models.PlatformUsageResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Router /platform/usage [get]
func (pc *PlatformUsageController) ListUsage(c *gin.Context) {
	var query models.PlatformUsageQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, tenancyErrors.UsageListFailed, utils.ExtractValidationError(c, err)))
		return
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
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.UsageRangeInvalid))
		return
	case to.Sub(from) > usageMaxDays*24*time.Hour:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, tenancyErrors.UsageRangeTooWide))
		return
	}
	var tenantID *uuid.UUID
	if query.TenantID != nil {
		id := uuid.MustParse(*query.TenantID)
		tenantID = &id
	}
	tenants, err := pc.usage.ListUsageDaily(c.Request.Context(), from.Format(time.DateOnly), to.Format(time.DateOnly), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, models.PlatformUsageResponse{Tenants: tenants})
}
