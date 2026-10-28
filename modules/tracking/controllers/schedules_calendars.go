package controllers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The schedule and calendar handlers of TRACK-018, beside tracking_controller.go rather than inside
// it: one slice, one file.

// ==================== SCHEDULES ====================

// scheduleErrorResponse maps the schedule codes to statuses and reports whether it has already
// written the response.
func scheduleErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.ScheduleNotFound, trackingErrors.CalendarNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.ScheduleOverlap:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.ScheduleSplitDate, trackingErrors.ScheduleDaysOfWeek, trackingErrors.ScheduleRange,
		trackingErrors.ScheduleArrivalOutboundOnly:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return nil
}

// ListRouteSchedulesInput is ListRouteSchedules's request.
type ListRouteSchedulesInput struct {
	RouteID string `path:"route_id"`
}

// ListRouteSchedules is GET /routes/{route_id}/schedules: List a route's schedules.
func (ctrl *TrackingController) ListRouteSchedules(ctx context.Context, in *ListRouteSchedulesInput) (*api.Out[[]*models.RouteSchedule], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	schedules, err := ctrl.trackingService.ListRouteSchedules(c.Request.Context(), tenantID, routeID)
	if err != nil {
		if err := scheduleErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(schedules)
}

// CreateRouteScheduleInput is CreateRouteSchedule's request.
type CreateRouteScheduleInput struct {
	RouteID string `path:"route_id"`
	JSON    api.Body[models.CreateRouteScheduleDto]
}

// CreateRouteSchedule is POST /routes/{route_id}/schedules: Add a recurrence to a route.
func (ctrl *TrackingController) CreateRouteSchedule(ctx context.Context, in *CreateRouteScheduleInput) (*api.Out[*models.RouteSchedule], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	routeID, err := uuid.Parse(in.RouteID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.CreateRouteScheduleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ScheduleCreateFailed, utils.ExtractValidationError(c, err)))
	}
	if (dto.StartTime == nil) == (dto.ArrivalTime == nil) {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.ScheduleTimeChoice))
	}

	schedule, err := ctrl.trackingService.CreateRouteSchedule(c.Request.Context(), tenantID, routeID, &dto)
	if err != nil {
		if err := scheduleErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(schedule)
}

// UpdateRouteScheduleInput is UpdateRouteSchedule's request.
type UpdateRouteScheduleInput struct {
	RouteID    string `path:"route_id"`
	ScheduleID string `path:"schedule_id"`
	JSON       api.Body[models.UpdateRouteScheduleDto]
}

// UpdateRouteSchedule is PATCH /routes/{route_id}/schedules/{schedule_id}: Edit a schedule in place.
func (ctrl *TrackingController) UpdateRouteSchedule(ctx context.Context, in *UpdateRouteScheduleInput) (*api.Out[*models.RouteSchedule], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	scheduleID, err := uuid.Parse(in.ScheduleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateRouteScheduleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ScheduleUpdateFailed, utils.ExtractValidationError(c, err)))
	}
	if dto.StartTime != nil && dto.ArrivalTime != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.ScheduleTimeChoice))
	}

	schedule, err := ctrl.trackingService.UpdateRouteSchedule(c.Request.Context(), tenantID, scheduleID, &dto)
	if err != nil {
		if err := scheduleErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(schedule)
}

// DeleteRouteScheduleInput is DeleteRouteSchedule's request.
type DeleteRouteScheduleInput struct {
	RouteID    string `path:"route_id"`
	ScheduleID string `path:"schedule_id"`
}

// DeleteRouteSchedule is DELETE /routes/{route_id}/schedules/{schedule_id}: Remove a recurrence from a route.
func (ctrl *TrackingController) DeleteRouteSchedule(ctx context.Context, in *DeleteRouteScheduleInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	scheduleID, err := uuid.Parse(in.ScheduleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.trackingService.DeleteRouteSchedule(c.Request.Context(), tenantID, scheduleID); err != nil {
		if err := scheduleErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// SplitRouteScheduleInput is SplitRouteSchedule's request.
type SplitRouteScheduleInput struct {
	RouteID    string `path:"route_id"`
	ScheduleID string `path:"schedule_id"`
	JSON       api.Body[models.SplitRouteScheduleDto]
}

// SplitRouteSchedule is POST /routes/{route_id}/schedules/{schedule_id}/split: Change a schedule from a date onward.
func (ctrl *TrackingController) SplitRouteSchedule(ctx context.Context, in *SplitRouteScheduleInput) (*api.Out[*models.SplitRouteScheduleResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	scheduleID, err := uuid.Parse(in.ScheduleID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.SplitRouteScheduleDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ScheduleSplitFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.SplitRouteSchedule(c.Request.Context(), tenantID, scheduleID, &dto)
	if err != nil {
		if err := scheduleErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// ==================== CALENDARS ====================

// calendarErrorResponse maps the calendar codes to statuses and reports whether it has already
// written the response.
func calendarErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return nil
	}
	switch trackingErr.Code {
	case trackingErrors.CalendarNotFound, trackingErrors.OrganizationNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.CalendarInUse, trackingErrors.CalendarDuplicateDate:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	case trackingErrors.CalendarRangeTooWide:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return nil
}

// ListCalendarsInput is ListCalendars's request.
type ListCalendarsInput struct {
	Query api.Query[models.ListCalendarsQuery]
}

// ListCalendars is GET /calendars: List the tenant's calendars.
func (ctrl *TrackingController) ListCalendars(ctx context.Context, in *ListCalendarsInput) (*api.Out[*models.ListCalendarsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ListCalendarsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarListFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.ListCalendars(c.Request.Context(), tenantID, query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// CreateCalendarInput is CreateCalendar's request.
type CreateCalendarInput struct {
	JSON api.Body[models.CreateCalendarDto]
}

// CreateCalendar is POST /calendars: Create a calendar.
func (ctrl *TrackingController) CreateCalendar(ctx context.Context, in *CreateCalendarInput) (*api.Out[*models.Calendar], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var dto models.CreateCalendarDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarCreateFailed, utils.ExtractValidationError(c, err)))
	}

	calendar, err := ctrl.trackingService.CreateCalendar(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(calendar)
}

// UpdateCalendarInput is UpdateCalendar's request.
type UpdateCalendarInput struct {
	CalendarID string `path:"calendar_id"`
	JSON       api.Body[models.UpdateCalendarDto]
}

// UpdateCalendar is PATCH /calendars/{calendar_id}: Edit a calendar.
func (ctrl *TrackingController) UpdateCalendar(ctx context.Context, in *UpdateCalendarInput) (*api.Out[*models.Calendar], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	calendarID, err := uuid.Parse(in.CalendarID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var dto models.UpdateCalendarDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarUpdateFailed, utils.ExtractValidationError(c, err)))
	}

	calendar, err := ctrl.trackingService.UpdateCalendar(c.Request.Context(), tenantID, calendarID, &dto)
	if err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(calendar)
}

// GetCalendarInput is GetCalendar's request.
type GetCalendarInput struct {
	CalendarID string `path:"calendar_id"`
}

// GetCalendar is GET /calendars/{calendar_id}: Get a calendar.
func (ctrl *TrackingController) GetCalendar(ctx context.Context, in *GetCalendarInput) (*api.Out[*models.Calendar], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	calendarID, err := uuid.Parse(in.CalendarID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	calendar, err := ctrl.trackingService.GetCalendar(c.Request.Context(), tenantID, calendarID)
	if err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(calendar)
}

// DeleteCalendarInput is DeleteCalendar's request.
type DeleteCalendarInput struct {
	CalendarID string `path:"calendar_id"`
}

// DeleteCalendar is DELETE /calendars/{calendar_id}: Delete a calendar.
func (ctrl *TrackingController) DeleteCalendar(ctx context.Context, in *DeleteCalendarInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	calendarID, err := uuid.Parse(in.CalendarID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}

	if err := ctrl.trackingService.DeleteCalendar(c.Request.Context(), tenantID, calendarID); err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}

// ListCalendarDatesInput is ListCalendarDates's request.
type ListCalendarDatesInput struct {
	CalendarID string `path:"calendar_id"`
	Query      api.Query[models.ListCalendarDatesQuery]
}

// ListCalendarDates is GET /calendars/{calendar_id}/dates: List a calendar's dates.
func (ctrl *TrackingController) ListCalendarDates(ctx context.Context, in *ListCalendarDatesInput) (*api.Out[[]*models.CalendarDate], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	calendarID, err := uuid.Parse(in.CalendarID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	var query models.ListCalendarDatesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarDatesFailed, utils.ExtractValidationError(c, err)))
	}

	dates, err := ctrl.trackingService.ListCalendarDates(c.Request.Context(), tenantID, calendarID, query.From, query.To)
	if err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(dates)
}

// ReplaceCalendarDatesInput is ReplaceCalendarDates's request.
type ReplaceCalendarDatesInput struct {
	CalendarID string `path:"calendar_id"`
	JSON       api.Body[[]models.CalendarDateDto]
}

// ReplaceCalendarDates is PUT /calendars/{calendar_id}/dates: Replace a calendar's whole date list.
func (ctrl *TrackingController) ReplaceCalendarDates(ctx context.Context, in *ReplaceCalendarDatesInput) (*api.Out[[]*models.CalendarDate], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	calendarID, err := uuid.Parse(in.CalendarID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	dates := []models.CalendarDateDto{}
	if err := in.JSON.Into(ctx, &dates); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarDatesFailed, utils.ExtractValidationError(c, err)))
	}

	stored, err := ctrl.trackingService.ReplaceCalendarDates(c.Request.Context(), tenantID, calendarID, dates)
	if err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(stored)
}

// ==================== MONTH PAGE (TRACK-076 D1) ====================

// ListScopeCalendarDatesInput is ListScopeCalendarDates's request.
type ListScopeCalendarDatesInput struct {
	Query api.Query[models.ListScopeCalendarDatesQuery]
}

// ListScopeCalendarDates is GET /calendar-dates: The tenant's and its clients' days off in a window.
func (ctrl *TrackingController) ListScopeCalendarDates(ctx context.Context, in *ListScopeCalendarDatesInput) (*api.Out[*models.ListScopeCalendarDatesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var query models.ListScopeCalendarDatesQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarListFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.ListScopeCalendarDates(c.Request.Context(), tenantID, query.From, query.To)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// AddScopeCalendarDatesInput is AddScopeCalendarDates's request.
type AddScopeCalendarDatesInput struct {
	JSON api.Body[models.AddScopeCalendarDatesDto]
}

// AddScopeCalendarDates is POST /calendar-dates: Mark days off for every route or one client's.
func (ctrl *TrackingController) AddScopeCalendarDates(ctx context.Context, in *AddScopeCalendarDatesInput) (*api.Out[*models.AddScopeCalendarDatesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	var dto models.AddScopeCalendarDatesDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarDatesFailed, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.trackingService.AddScopeCalendarDates(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(result)
}

// DeleteScopeCalendarDateInput is DeleteScopeCalendarDate's request.
type DeleteScopeCalendarDateInput struct {
	CalendarID string `path:"calendar_id"`
	Date       string `path:"date"`
}

// DeleteScopeCalendarDate is DELETE /calendar-dates/{calendar_id}/{date}: Give a day back to the routes.
func (ctrl *TrackingController) DeleteScopeCalendarDate(ctx context.Context, in *DeleteScopeCalendarDateInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, err := ctrl.requireTenantID(c)
	if err != nil {
		return nil, err
	}
	calendarID, err := uuid.Parse(in.CalendarID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	date, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.CalendarDatesFailed))
	}

	if err := ctrl.trackingService.DeleteScopeCalendarDate(c.Request.Context(), tenantID, calendarID, date); err != nil {
		if err := calendarErrorResponse(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.Done()
}
