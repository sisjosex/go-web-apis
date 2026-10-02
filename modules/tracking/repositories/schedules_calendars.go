package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	coreModels "josex/web/modules/core/models"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The schedule and calendar slice of TRACK-018. It sits beside tracking_repository.go for the same
// reason services/document_files.go does: one slice, one file, rather than a third of a thousand-line
// file belonging to a feature nobody is reading about.

// ==================== SCHEDULES ====================

// dateArg is what pgx sends for a nullable DATE parameter: a DateOnly the caller may not have sent.
func dateArg(d *coreModels.DateOnly) *time.Time {
	if d == nil {
		return nil
	}
	t := time.Time(*d)
	return &t
}

// scanRouteSchedule is the one scan order every schedule read shares — list, single and write alike.
func scanRouteSchedule(s *models.RouteSchedule) []any {
	return []any{
		&s.ID, &s.RouteID, &s.DaysOfWeek, &s.StartTime, &s.ValidFrom, &s.ValidUntil,
		&s.CalendarID, &s.CalendarName, &s.CreatedAt, &s.UpdatedAt,
	}
}

// mapScheduleError turns the SP's own codes into module codes, falling back to fallbackCode for
// anything else. The controller maps the module code to a status. The guard raises schedule.range;
// the table's CHECK is the backstop, told apart by constraint name.
func mapScheduleError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.ConstraintName == "chk_route_schedule_range" {
			return &trackingErrors.TrackingError{Code: trackingErrors.ScheduleRange, Err: pgErr}
		}
		switch pgErr.Message {
		case "route.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.RouteNotFound, Err: pgErr}
		case "schedule.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.ScheduleNotFound, Err: pgErr}
		case "schedule.overlap":
			return &trackingErrors.TrackingError{Code: trackingErrors.ScheduleOverlap, Err: pgErr}
		case "schedule.split-date":
			return &trackingErrors.TrackingError{Code: trackingErrors.ScheduleSplitDate, Err: pgErr}
		case "schedule.range":
			return &trackingErrors.TrackingError{Code: trackingErrors.ScheduleRange, Err: pgErr}
		case "schedule.days-of-week":
			return &trackingErrors.TrackingError{Code: trackingErrors.ScheduleDaysOfWeek, Err: pgErr}
		case "calendar.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.CalendarNotFound, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

func (r *TrackingRepository) ListRouteSchedules(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID) ([]*models.RouteSchedule, error) {
	rows, err := r.dbService.Query(ctx, `SELECT * FROM tracking.sp_list_route_schedules($1, $2)`, tenantID, routeID)
	if err != nil {
		return nil, mapScheduleError(err, trackingErrors.ScheduleListFailed)
	}
	defer rows.Close()
	schedules := []*models.RouteSchedule{}
	for rows.Next() {
		var schedule models.RouteSchedule
		if err := rows.Scan(scanRouteSchedule(&schedule)...); err != nil {
			return nil, err
		}
		schedules = append(schedules, &schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, mapScheduleError(err, trackingErrors.ScheduleListFailed)
	}
	return schedules, nil
}

func (r *TrackingRepository) CreateRouteSchedule(ctx context.Context, tenantID uuid.UUID, routeID uuid.UUID, dto *models.CreateRouteScheduleDto) (*models.RouteSchedule, error) {
	var schedule models.RouteSchedule
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_route_schedule(
			p_tenant_id    := $1,
			p_route_id     := $2,
			p_days_of_week := $3,
			p_start_time   := $4,
			p_valid_from   := $5,
			p_valid_until  := $6,
			p_calendar_id  := $7
		)
	`, tenantID, routeID, dto.DaysOfWeek, dto.StartTime, time.Time(dto.ValidFrom),
		dateArg(dto.ValidUntil), dto.CalendarID,
	).Scan(scanRouteSchedule(&schedule)...)
	if err != nil {
		return nil, mapScheduleError(err, trackingErrors.ScheduleCreateFailed)
	}
	return &schedule, nil
}

func (r *TrackingRepository) UpdateRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID, dto *models.UpdateRouteScheduleDto) (*models.RouteSchedule, error) {
	var schedule models.RouteSchedule
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_update_route_schedule(
			p_tenant_id    := $1,
			p_schedule_id  := $2,
			p_days_of_week := $3,
			p_start_time   := $4,
			p_valid_from   := $5,
			p_valid_until  := $6,
			p_calendar_id  := $7,
			p_clear_valid_until := $8,
			p_clear_calendar    := $9
		)
	`, tenantID, scheduleID, dto.DaysOfWeek, dto.StartTime, dateArg(dto.ValidFrom),
		dateArg(dto.ValidUntil), dto.CalendarID, dto.ClearValidUntil, dto.ClearCalendar,
	).Scan(scanRouteSchedule(&schedule)...)
	if err != nil {
		return nil, mapScheduleError(err, trackingErrors.ScheduleUpdateFailed)
	}
	return &schedule, nil
}

func (r *TrackingRepository) DeleteRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_route_schedule($1, $2)`, tenantID, scheduleID).Scan(&deleted)
	if err != nil {
		return mapScheduleError(err, trackingErrors.ScheduleDeleteFailed)
	}
	if !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.ScheduleDeleteFailed}
	}
	return nil
}

// SplitRouteSchedule returns both halves the SP produced. They come back as two rows labelled by a
// role column, so the two writes stay in one statement and one transaction.
func (r *TrackingRepository) SplitRouteSchedule(ctx context.Context, tenantID uuid.UUID, scheduleID uuid.UUID, dto *models.SplitRouteScheduleDto) (*models.SplitRouteScheduleResponse, error) {
	changes, err := json.Marshal(dto.Changes)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.ScheduleSplitFailed, Err: err}
	}
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_split_route_schedule(
			p_tenant_id   := $1,
			p_schedule_id := $2,
			p_from_date   := $3,
			p_changes     := $4
		)
	`, tenantID, scheduleID, time.Time(dto.FromDate), changes)
	if err != nil {
		return nil, mapScheduleError(err, trackingErrors.ScheduleSplitFailed)
	}
	defer rows.Close()

	result := &models.SplitRouteScheduleResponse{}
	for rows.Next() {
		var role string
		var schedule models.RouteSchedule
		if err := rows.Scan(append([]any{&role}, scanRouteSchedule(&schedule)...)...); err != nil {
			return nil, err
		}
		if role == "closed" {
			result.Closed = &schedule
		} else {
			result.Created = &schedule
		}
	}
	if err := rows.Err(); err != nil {
		return nil, mapScheduleError(err, trackingErrors.ScheduleSplitFailed)
	}
	return result, nil
}

// ==================== CALENDARS ====================

// scanCalendar is the one scan order every calendar read shares.
func scanCalendar(c *models.Calendar) []any {
	return []any{
		&c.ID, &c.TenantID, &c.OrganizationID, &c.Name, &c.DatesCount, &c.CreatedAt, &c.UpdatedAt,
	}
}

// mapCalendarError turns the SP's own codes into module codes, falling back to fallbackCode.
func mapCalendarError(err error, fallbackCode string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Message {
		case "calendar.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.CalendarNotFound, Err: pgErr}
		case "calendar.in-use":
			return &trackingErrors.TrackingError{Code: trackingErrors.CalendarInUse, Err: pgErr}
		case "calendar.duplicate-date":
			return &trackingErrors.TrackingError{Code: trackingErrors.CalendarDuplicateDate, Err: pgErr}
		case "organization.not-found":
			return &trackingErrors.TrackingError{Code: trackingErrors.OrganizationNotFound, Err: pgErr}
		}
	}
	return &trackingErrors.TrackingError{Code: fallbackCode, Err: err}
}

// ListCalendars returns one page of the tenant's calendars plus the total the same filters match.
func (r *TrackingRepository) ListCalendars(ctx context.Context, tenantID uuid.UUID, query models.ListCalendarsQuery) ([]*models.Calendar, int64, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_calendars(
			p_tenant_id := $1,
			p_search    := $2,
			p_page      := $3,
			p_page_size := $4
		)
	`, tenantID, query.Search, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, &trackingErrors.TrackingError{Code: trackingErrors.CalendarListFailed, Err: err}
	}
	defer rows.Close()
	calendars := []*models.Calendar{}
	var totalCount int64
	for rows.Next() {
		var calendar models.Calendar
		if err := rows.Scan(append(scanCalendar(&calendar), &totalCount)...); err != nil {
			return nil, 0, err
		}
		calendars = append(calendars, &calendar)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return calendars, totalCount, nil
}

func (r *TrackingRepository) CreateCalendar(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCalendarDto) (*models.Calendar, error) {
	var calendar models.Calendar
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_create_calendar(
			p_tenant_id       := $1,
			p_name            := $2,
			p_organization_id := $3
		)
	`, tenantID, dto.Name, dto.OrganizationID).Scan(scanCalendar(&calendar)...)
	if err != nil {
		return nil, mapCalendarError(err, trackingErrors.CalendarCreateFailed)
	}
	return &calendar, nil
}

func (r *TrackingRepository) UpdateCalendar(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, dto *models.UpdateCalendarDto) (*models.Calendar, error) {
	var calendar models.Calendar
	err := r.dbService.QueryRow(ctx, `
		SELECT * FROM tracking.sp_update_calendar(
			p_tenant_id       := $1,
			p_calendar_id     := $2,
			p_name            := $3,
			p_organization_id := $4
		)
	`, tenantID, calendarID, dto.Name, dto.OrganizationID).Scan(scanCalendar(&calendar)...)
	if err != nil {
		return nil, mapCalendarError(err, trackingErrors.CalendarUpdateFailed)
	}
	return &calendar, nil
}

// GetCalendar is one calendar with its date count — what a calendar's own page opens on (TRACK-029).
func (r *TrackingRepository) GetCalendar(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID) (*models.Calendar, error) {
	var calendar models.Calendar
	err := r.dbService.QueryRow(ctx, `SELECT * FROM tracking.sp_get_calendar($1, $2)`, tenantID, calendarID).
		Scan(scanCalendar(&calendar)...)
	if err != nil {
		return nil, mapCalendarError(err, trackingErrors.CalendarNotFound)
	}
	return &calendar, nil
}

func (r *TrackingRepository) DeleteCalendar(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID) error {
	var deleted bool
	err := r.dbService.QueryRow(ctx, `SELECT tracking.sp_delete_calendar($1, $2)`, tenantID, calendarID).Scan(&deleted)
	if err != nil {
		return mapCalendarError(err, trackingErrors.CalendarDeleteFailed)
	}
	if !deleted {
		return &trackingErrors.TrackingError{Code: trackingErrors.CalendarDeleteFailed}
	}
	return nil
}

// scanCalendarDates drains a calendar-date result set; both the read and the replace return it.
func scanCalendarDates(rows pgx.Rows) ([]*models.CalendarDate, error) {
	dates := []*models.CalendarDate{}
	for rows.Next() {
		var date models.CalendarDate
		if err := rows.Scan(&date.Date, &date.Kind, &date.Label); err != nil {
			return nil, err
		}
		dates = append(dates, &date)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dates, nil
}

func (r *TrackingRepository) ListCalendarDates(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, from, to *string) ([]*models.CalendarDate, error) {
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_list_calendar_dates(
			p_tenant_id   := $1,
			p_calendar_id := $2,
			p_from        := $3,
			p_to          := $4
		)
	`, tenantID, calendarID, from, to)
	if err != nil {
		return nil, mapCalendarError(err, trackingErrors.CalendarDatesFailed)
	}
	defer rows.Close()
	dates, err := scanCalendarDates(rows)
	if err != nil {
		return nil, mapCalendarError(err, trackingErrors.CalendarDatesFailed)
	}
	return dates, nil
}

// ReplaceCalendarDates swaps the whole list in one round-trip and returns what is stored.
func (r *TrackingRepository) ReplaceCalendarDates(ctx context.Context, tenantID uuid.UUID, calendarID uuid.UUID, dates []models.CalendarDateDto) ([]*models.CalendarDate, error) {
	payload, err := json.Marshal(dates)
	if err != nil {
		return nil, &trackingErrors.TrackingError{Code: trackingErrors.CalendarDatesFailed, Err: err}
	}
	rows, err := r.dbService.Query(ctx, `
		SELECT * FROM tracking.sp_replace_calendar_dates(
			p_tenant_id   := $1,
			p_calendar_id := $2,
			p_dates       := $3
		)
	`, tenantID, calendarID, payload)
	if err != nil {
		return nil, mapCalendarError(err, trackingErrors.CalendarDatesFailed)
	}
	defer rows.Close()
	stored, err := scanCalendarDates(rows)
	if err != nil {
		return nil, mapCalendarError(err, trackingErrors.CalendarDatesFailed)
	}
	return stored, nil
}
