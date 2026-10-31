package controllers

import (
	"context"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"
	inventoryPerms "josex/web/modules/inventory/permissions"
	tenancyMW "josex/web/modules/tenancy/middleware"

	"github.com/gin-gonic/gin"
)

// CountController serves stock counts (INV-026).
type CountController struct {
	counts interfaces.CountRepository
}

// NewCountController builds the controller.
func NewCountController(counts interfaces.CountRepository) *CountController {
	return &CountController{counts: counts}
}

// CreateCountInput is CreateCount's request.
type CreateCountInput struct {
	JSON api.Body[models.CreateCountDto]
}

// CreateCount is POST /counts: one line per active combination of the scope.
func (ctrl *CountController) CreateCount(ctx context.Context, in *CreateCountInput) (*api.Out[*models.CountDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.CreateCountDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	id, err := ctrl.counts.CreateCount(c.Request.Context(), tenantID, dto, actingUser(c))
	if err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.detail(ctx, id)
}

// ListCountsInput is ListCounts's request.
type ListCountsInput struct {
	Query api.Query[models.ListCountsQuery]
}

// ListCounts is GET /counts.
func (ctrl *CountController) ListCounts(ctx context.Context, in *ListCountsInput) (*api.Out[*models.ListCountsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var query models.ListCountsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	out, err := ctrl.counts.ListCounts(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// hideExpected is D4: a blind count tells who only counts neither what was expected nor the difference.
func hideExpected(c *gin.Context, count *models.CountDetail) {
	if !count.Blind || tenancyMW.CanInContext(c, inventoryPerms.StockAdjust) {
		return
	}
	for i := range count.Lines {
		count.Lines[i].Expected, count.Lines[i].Difference = nil, nil
	}
	count.Progress.WithDifference, count.Progress.DifferenceValue = nil, nil
}

func (ctrl *CountController) detail(ctx context.Context, id string) (*api.Out[*models.CountDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	out, err := ctrl.counts.GetCount(c.Request.Context(), tenantID, id)
	if err != nil {
		return nil, failUnit(c, err)
	}
	hideExpected(c, out)
	return api.OK(out)
}

// CountIDInput names one count.
type CountIDInput struct {
	ID string `path:"id"`
}

// GetCount is GET /counts/{id}.
func (ctrl *CountController) GetCount(ctx context.Context, in *CountIDInput) (*api.Out[*models.CountDetail], error) {
	return ctrl.detail(ctx, in.ID)
}

// SetCountLinesInput is SetCountLines's request.
type SetCountLinesInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.SetCountLinesDto]
}

// SetCountLines is PATCH /counts/{id}/lines.
func (ctrl *CountController) SetCountLines(ctx context.Context, in *SetCountLinesInput) (*api.Out[*models.CountDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.SetCountLinesDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.counts.SetCountLines(c.Request.Context(), tenantID, in.ID, dto.Lines); err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.detail(ctx, in.ID)
}

// ScanCountInput is ScanCount's request.
type ScanCountInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.ScanCountDto]
}

// ScanCount is POST /counts/{id}/scan.
func (ctrl *CountController) ScanCount(ctx context.Context, in *ScanCountInput) (*api.Out[*models.CountDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.ScanCountDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.counts.ScanCount(c.Request.Context(), tenantID, in.ID, dto.Code); err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.detail(ctx, in.ID)
}

func (ctrl *CountController) setStatus(ctx context.Context, id, action string) (*api.Out[*models.CountDetail], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	if err := ctrl.counts.SetCountStatus(c.Request.Context(), tenantID, id, action); err != nil {
		return nil, failUnit(c, err)
	}
	return ctrl.detail(ctx, id)
}

// PauseCount is POST /counts/{id}/pause.
func (ctrl *CountController) PauseCount(ctx context.Context, in *CountIDInput) (*api.Out[*models.CountDetail], error) {
	return ctrl.setStatus(ctx, in.ID, "pause")
}

// ResumeCount is POST /counts/{id}/resume.
func (ctrl *CountController) ResumeCount(ctx context.Context, in *CountIDInput) (*api.Out[*models.CountDetail], error) {
	return ctrl.setStatus(ctx, in.ID, "resume")
}

// SubmitCount is POST /counts/{id}/submit: who only counts sends it to review.
func (ctrl *CountController) SubmitCount(ctx context.Context, in *CountIDInput) (*api.Out[*models.CountDetail], error) {
	return ctrl.setStatus(ctx, in.ID, "submit")
}

// CancelCount is POST /counts/{id}/cancel.
func (ctrl *CountController) CancelCount(ctx context.Context, in *CountIDInput) (*api.Out[*models.CountDetail], error) {
	return ctrl.setStatus(ctx, in.ID, "cancel")
}

// CloseCount is POST /counts/{id}/close: one ADJUSTMENT per line with a difference.
func (ctrl *CountController) CloseCount(ctx context.Context, in *CountIDInput) (*api.Out[models.CloseCountResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	number, err := ctrl.counts.CloseCount(c.Request.Context(), tenantID, in.ID, actingUser(c))
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(models.CloseCountResponse{Status: "closed", AdjustmentNumber: number})
}
