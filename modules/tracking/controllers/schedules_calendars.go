package controllers

import (
	"errors"
	"net/http"

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
func scheduleErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.RouteNotFound, trackingErrors.ScheduleNotFound, trackingErrors.CalendarNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.ScheduleOverlap:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.ScheduleSplitDate, trackingErrors.ScheduleDaysOfWeek, trackingErrors.ScheduleRange:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// ListRouteSchedules godoc
// @Summary List a route's schedules
// @Description When the route runs: a weekday bitmask (bit 0 Monday) and a departure time, over a range of dates
// @Tags Tracking - Schedules
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Success 200 {array} models.RouteSchedule
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/schedules [get]
func (ctrl *TrackingController) ListRouteSchedules(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	schedules, err := ctrl.trackingService.ListRouteSchedules(c.Request.Context(), tenantID, routeID)
	if err != nil {
		if scheduleErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, schedules)
}

// CreateRouteSchedule godoc
// @Summary Add a recurrence to a route
// @Description days_of_week is a bitmask, bit 0 Monday … bit 6 Sunday; start_time is HH:MM
// @Tags Tracking - Schedules
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param schedule body models.CreateRouteScheduleDto true "Weekdays, departure time and validity"
// @Success 201 {object} models.RouteSchedule
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/schedules [post]
func (ctrl *TrackingController) CreateRouteSchedule(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	routeID, err := uuid.Parse(c.Param("route_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.CreateRouteScheduleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ScheduleCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	schedule, err := ctrl.trackingService.CreateRouteSchedule(c.Request.Context(), tenantID, routeID, &dto)
	if err != nil {
		if scheduleErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, schedule)
}

// UpdateRouteSchedule godoc
// @Summary Edit a schedule in place
// @Description Fixes a schedule that was entered wrong; changing one from a date onward is the split endpoint
// @Tags Tracking - Schedules
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param schedule_id path string true "Schedule ID (UUID)"
// @Param schedule body models.UpdateRouteScheduleDto true "The fields to change"
// @Success 200 {object} models.RouteSchedule
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/schedules/{schedule_id} [patch]
func (ctrl *TrackingController) UpdateRouteSchedule(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	scheduleID, err := uuid.Parse(c.Param("schedule_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateRouteScheduleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ScheduleUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	schedule, err := ctrl.trackingService.UpdateRouteSchedule(c.Request.Context(), tenantID, scheduleID, &dto)
	if err != nil {
		if scheduleErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, schedule)
}

// DeleteRouteSchedule godoc
// @Summary Remove a recurrence from a route
// @Tags Tracking - Schedules
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param schedule_id path string true "Schedule ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/schedules/{schedule_id} [delete]
func (ctrl *TrackingController) DeleteRouteSchedule(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	scheduleID, err := uuid.Parse(c.Param("schedule_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.trackingService.DeleteRouteSchedule(c.Request.Context(), tenantID, scheduleID); err != nil {
		if scheduleErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// SplitRouteSchedule godoc
// @Summary Change a schedule from a date onward
// @Description Closes the schedule the day before from_date and opens a new one carrying the changes, so the days before keep the plan they had
// @Tags Tracking - Schedules
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param route_id path string true "Route ID (UUID)"
// @Param schedule_id path string true "Schedule ID (UUID)"
// @Param split body models.SplitRouteScheduleDto true "The date the change takes effect and what changes"
// @Success 201 {object} models.SplitRouteScheduleResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/routes/{route_id}/schedules/{schedule_id}/split [post]
func (ctrl *TrackingController) SplitRouteSchedule(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	scheduleID, err := uuid.Parse(c.Param("schedule_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.SplitRouteScheduleDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.ScheduleSplitFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.SplitRouteSchedule(c.Request.Context(), tenantID, scheduleID, &dto)
	if err != nil {
		if scheduleErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, result)
}

// ==================== CALENDARS ====================

// calendarErrorResponse maps the calendar codes to statuses and reports whether it has already
// written the response.
func calendarErrorResponse(c *gin.Context, err error) bool {
	var trackingErr *trackingErrors.TrackingError
	if !errors.As(err, &trackingErr) {
		return false
	}
	switch trackingErr.Code {
	case trackingErrors.CalendarNotFound, trackingErrors.OrganizationNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	case trackingErrors.CalendarInUse, trackingErrors.CalendarDuplicateDate:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return true
	}
	return false
}

// ListCalendars godoc
// @Summary List the tenant's calendars
// @Tags Tracking - Calendars
// @Produce json
// @Security BearerAuth
// @Param search query string false "Match the calendar name"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.ListCalendarsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/calendars [get]
func (ctrl *TrackingController) ListCalendars(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var query models.ListCalendarsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarListFailed, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.trackingService.ListCalendars(c.Request.Context(), tenantID, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateCalendar godoc
// @Summary Create a calendar
// @Description A named set of dates a schedule points at — a school year, the public holidays
// @Tags Tracking - Calendars
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param calendar body models.CreateCalendarDto true "Calendar data"
// @Success 201 {object} models.Calendar
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/calendars [post]
func (ctrl *TrackingController) CreateCalendar(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	var dto models.CreateCalendarDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarCreateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	calendar, err := ctrl.trackingService.CreateCalendar(c.Request.Context(), tenantID, &dto)
	if err != nil {
		if calendarErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, calendar)
}

// UpdateCalendar godoc
// @Summary Edit a calendar
// @Tags Tracking - Calendars
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param calendar_id path string true "Calendar ID (UUID)"
// @Param calendar body models.UpdateCalendarDto true "The fields to change"
// @Success 200 {object} models.Calendar
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/calendars/{calendar_id} [patch]
func (ctrl *TrackingController) UpdateCalendar(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	calendarID, err := uuid.Parse(c.Param("calendar_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var dto models.UpdateCalendarDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarUpdateFailed, utils.ExtractValidationError(c, err)))
		return
	}

	calendar, err := ctrl.trackingService.UpdateCalendar(c.Request.Context(), tenantID, calendarID, &dto)
	if err != nil {
		if calendarErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, calendar)
}

// GetCalendar godoc
// @Summary Get a calendar
// @Description One calendar with how many dates it holds
// @Tags Tracking - Calendars
// @Produce json
// @Security BearerAuth
// @Param calendar_id path string true "Calendar ID (UUID)"
// @Success 200 {object} models.Calendar
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/calendars/{calendar_id} [get]
func (ctrl *TrackingController) GetCalendar(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	calendarID, err := uuid.Parse(c.Param("calendar_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	calendar, err := ctrl.trackingService.GetCalendar(c.Request.Context(), tenantID, calendarID)
	if err != nil {
		if calendarErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, calendar)
}

// DeleteCalendar godoc
// @Summary Delete a calendar
// @Description Refused while a schedule still points at it
// @Tags Tracking - Calendars
// @Produce json
// @Security BearerAuth
// @Param calendar_id path string true "Calendar ID (UUID)"
// @Success 204 "No Content"
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/calendars/{calendar_id} [delete]
func (ctrl *TrackingController) DeleteCalendar(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	calendarID, err := uuid.Parse(c.Param("calendar_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}

	if err := ctrl.trackingService.DeleteCalendar(c.Request.Context(), tenantID, calendarID); err != nil {
		if calendarErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.Status(http.StatusNoContent)
}

// ListCalendarDates godoc
// @Summary List a calendar's dates
// @Tags Tracking - Calendars
// @Produce json
// @Security BearerAuth
// @Param calendar_id path string true "Calendar ID (UUID)"
// @Param from query string false "Earliest date (YYYY-MM-DD)"
// @Param to query string false "Latest date (YYYY-MM-DD)"
// @Success 200 {array} models.CalendarDate
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/calendars/{calendar_id}/dates [get]
func (ctrl *TrackingController) ListCalendarDates(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	calendarID, err := uuid.Parse(c.Param("calendar_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	var query models.ListCalendarDatesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarDatesFailed, utils.ExtractValidationError(c, err)))
		return
	}

	dates, err := ctrl.trackingService.ListCalendarDates(c.Request.Context(), tenantID, calendarID, query.From, query.To)
	if err != nil {
		if calendarErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, dates)
}

// ReplaceCalendarDates godoc
// @Summary Replace a calendar's whole date list
// @Description One request is the calendar, so a school year is saved once rather than one holiday at a time
// @Tags Tracking - Calendars
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param calendar_id path string true "Calendar ID (UUID)"
// @Param dates body []models.CalendarDateDto true "The whole list of dates"
// @Success 200 {array} models.CalendarDate
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /tracking/calendars/{calendar_id}/dates [put]
func (ctrl *TrackingController) ReplaceCalendarDates(c *gin.Context) {
	tenantID, ok := ctrl.requireTenantID(c)
	if !ok {
		return
	}
	calendarID, err := uuid.Parse(c.Param("calendar_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	dates := []models.CalendarDateDto{}
	if err := c.ShouldBindJSON(&dates); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.CalendarDatesFailed, utils.ExtractValidationError(c, err)))
		return
	}

	stored, err := ctrl.trackingService.ReplaceCalendarDates(c.Request.Context(), tenantID, calendarID, dates)
	if err != nil {
		if calendarErrorResponse(c, err) {
			return
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, stored)
}
