package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"

	coreErrors "josex/web/modules/core/errors"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	trackingErrors "josex/web/modules/tracking/errors"
	"josex/web/modules/tracking/interfaces"
	"josex/web/modules/tracking/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PortalController is /mobile/portal (TRACK-012): the guardian's phone. Apart from TrackingController
// because it runs behind the portal chain and the portal level only; every SP resolves the account's
// riders itself.
type PortalController struct {
	notifications interfaces.NotificationService
	// directory and riders serve /me, which spans tenants (TRACK-061 D1).
	directory interfaces.PortalDirectory
	riders    interfaces.TrackingService
}

func NewPortalController(notifications interfaces.NotificationService, directory interfaces.PortalDirectory, riders interfaces.TrackingService) *PortalController {
	return &PortalController{notifications: notifications, directory: directory, riders: riders}
}

// maxSettingsRiders caps one PUT /notification-settings body.
const maxSettingsRiders = 100

// portalErrorResponse answers a rider outside the account's scope 404, anything else 500.
func portalErrorResponse(c *gin.Context, err error) {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
		return
	}
	c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListNotifications godoc
// @Summary The account's notices
// @Description Newest first, the account's own rows only. unread narrows to unread rows; unread_count is the whole badge either way
// @Tags Tracking - Portal
// @Produce json
// @Security BearerAuth
// @Param unread query bool false "Only unread rows"
// @Param page query int false "Page number (default 1)"
// @Param page_size query int false "Page size (default 20, max 100)"
// @Success 200 {object} models.ListNotificationsResponse
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/portal/notifications [get]
func (ctrl *PortalController) ListNotifications(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}
	var query models.ListNotificationsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.NotificationInvalid, utils.ExtractValidationError(c, err)))
		return
	}

	result, err := ctrl.notifications.List(c.Request.Context(), tenantID, userID, query)
	if err != nil {
		portalErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// MarkNotificationsRead godoc
// @Summary Mark notices read
// @Description The ids given (at most 100), or every unread row with all. Another account's id changes nothing
// @Tags Tracking - Portal
// @Accept json
// @Security BearerAuth
// @Param body body models.MarkNotificationsReadDto true "ids or all"
// @Success 204
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/portal/notifications/read [post]
func (ctrl *PortalController) MarkNotificationsRead(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}
	var dto models.MarkNotificationsReadDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.NotificationInvalid, utils.ExtractValidationError(c, err)))
		return
	}

	if err := ctrl.notifications.MarkRead(c.Request.Context(), tenantID, userID, dto); err != nil {
		portalErrorResponse(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// NotificationSettings godoc
// @Summary Which pushes each rider mutes
// @Description One row per rider the account receives notices for; muted empty is every push on. A muted notice still lands in the feed
// @Tags Tracking - Portal
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.NotificationSetting
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/portal/notification-settings [get]
func (ctrl *PortalController) NotificationSettings(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}

	settings, err := ctrl.notifications.Settings(c.Request.Context(), tenantID, userID)
	if err != nil {
		portalErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

// SetNotificationSettings godoc
// @Summary Replace what riders mute
// @Description Each rider named gets exactly the muted list sent; riders not named keep theirs. A rider the account receives no notices for is 404 and nothing is saved. Answers the whole list
// @Tags Tracking - Portal
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body []models.NotificationSetting true "Per rider, the muted types"
// @Success 200 {array} models.NotificationSetting
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/portal/notification-settings [put]
func (ctrl *PortalController) SetNotificationSettings(c *gin.Context) {
	tenantID, userID, ok := driverIDs(c)
	if !ok {
		return
	}
	// gin validates each element of a bound slice.
	var body []models.NotificationSetting
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.NotificationInvalid, utils.ExtractValidationError(c, err)))
		return
	}
	if len(body) > maxSettingsRiders {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.NotificationInvalid))
		return
	}

	settings, err := ctrl.notifications.SetSettings(c.Request.Context(), tenantID, userID, body)
	if err != nil {
		portalErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

// Me godoc
// @Summary The account's riders in every operator
// @Description The guardian's home across operators (TRACK-061 D1): the account, and per tenant where it holds the portal role (newest membership first) its riders as GET /tracking/riders/today answers them. One operator's read failing marks that entry error with riders null; the others still answer. Bearer only, no X-Tenant-Slug
// @Tags Tracking - Portal
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.PortalMe
// @Failure 403 {object} coreErrors.ErrorResponse
// @Failure 500 {object} coreErrors.ErrorResponse
// @Router /mobile/portal/me [get]
func (ctrl *PortalController) Me(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.PermissionDenied))
		return
	}
	user, memberships, err := ctrl.directory.PortalAccount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	if len(memberships) == 0 {
		c.JSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.PermissionDenied))
		return
	}

	// One read per operator, each on its own database, side by side: 1–3 of them for a family.
	tenants := make([]models.PortalTenant, len(memberships))
	var wg sync.WaitGroup
	for i, m := range memberships {
		tenants[i] = models.PortalTenant{Slug: m.Slug, Name: m.Name}
		wg.Add(1)
		go func(entry *models.PortalTenant, m models.PortalMembership) {
			defer wg.Done()
			ctx := c.Request.Context()
			if m.DatabaseURL != "" {
				ctx = context.WithValue(ctx, coreServices.TenantDatabaseURLKey, m.DatabaseURL)
			}
			riders, err := ctrl.riders.ListGuardianRiderStatus(ctx, m.TenantID, userID)
			if err != nil {
				log.Printf("⚠️  portal me: tenant %s: %v", m.Slug, err)
				entry.Error = true
				return
			}
			entry.Riders = riders
		}(&tenants[i], m)
	}
	wg.Wait()
	c.JSON(http.StatusOK, models.PortalMe{User: user, Tenants: tenants})
}
