package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"
)

// ListRiderAccessLogInput is ListRiderAccessLog's request.
type ListRiderAccessLogInput struct {
	RiderID string `path:"rider_id"`
	Query   api.Query[models.ListRiderAccessLogQuery]
}

// ListRiderAccessLog is GET /riders/{rider_id}/audit: Who outside the operator read a rider.
func (ctrl *TrackingController) ListRiderAccessLog(ctx context.Context, in *ListRiderAccessLogInput) (*api.Out[*models.ListRiderAccessLogResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	riderID, err := pathUUID(c, "rider_id")
	if err != nil {
		return nil, err
	}
	var query models.ListRiderAccessLogQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.RiderAccessLogFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.ListRiderAccessLog(c.Request.Context(), tenantID, riderID, query)
	if err != nil {
		var trackingErr *trackingErrors.TrackingError
		if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
			return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}
