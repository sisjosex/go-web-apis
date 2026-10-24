package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"

	"josex/web/modules/core/api"
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
func portalErrorResponse(c *gin.Context, err error) error {
	var trackingErr *trackingErrors.TrackingError
	if errors.As(err, &trackingErr) && trackingErr.Code == trackingErrors.RiderNotFound {
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, trackingErr.Code))
	}
	return api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
}

// ListNotificationsInput is ListNotifications's request.
type ListNotificationsInput struct {
	Query api.Query[models.ListNotificationsQuery]
}

// ListNotifications is GET /notifications: The account's notices.
func (ctrl *PortalController) ListNotifications(ctx context.Context, in *ListNotificationsInput) (*api.Out[*models.ListNotificationsResponse], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}
	var query models.ListNotificationsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.NotificationInvalid, utils.ExtractValidationError(c, err)))
	}

	result, err := ctrl.notifications.List(c.Request.Context(), tenantID, userID, query)
	if err != nil {
		return nil, portalErrorResponse(c, err)
	}
	return api.OK(result)
}

// MarkNotificationsReadInput is MarkNotificationsRead's request.
type MarkNotificationsReadInput struct {
	JSON api.Body[models.MarkNotificationsReadDto]
}

// MarkNotificationsRead is POST /notifications/read: Mark notices read.
func (ctrl *PortalController) MarkNotificationsRead(ctx context.Context, in *MarkNotificationsReadInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}
	var dto models.MarkNotificationsReadDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.NotificationInvalid, utils.ExtractValidationError(c, err)))
	}

	if err := ctrl.notifications.MarkRead(c.Request.Context(), tenantID, userID, dto); err != nil {
		return nil, portalErrorResponse(c, err)
	}
	return api.Done()
}

// NotificationSettings is GET /notification-settings: Which pushes each rider mutes.
func (ctrl *PortalController) NotificationSettings(ctx context.Context, _ *api.None) (*api.Out[[]models.NotificationSetting], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}

	settings, err := ctrl.notifications.Settings(c.Request.Context(), tenantID, userID)
	if err != nil {
		return nil, portalErrorResponse(c, err)
	}
	return api.OK(settings)
}

// SetNotificationSettingsInput is SetNotificationSettings's request.
type SetNotificationSettingsInput struct {
	JSON api.Body[[]models.NotificationSetting]
}

// SetNotificationSettings is PUT /notification-settings: Replace what riders mute.
func (ctrl *PortalController) SetNotificationSettings(ctx context.Context, in *SetNotificationSettingsInput) (*api.Out[[]models.NotificationSetting], error) {
	c := api.Gin(ctx)
	tenantID, userID, err := driverIDs(c)
	if err != nil {
		return nil, err
	}
	// gin validates each element of a bound slice.
	var body []models.NotificationSetting
	if err := in.JSON.Into(ctx, &body); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, trackingErrors.NotificationInvalid, utils.ExtractValidationError(c, err)))
	}
	if len(body) > maxSettingsRiders {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, trackingErrors.NotificationInvalid))
	}

	settings, err := ctrl.notifications.SetSettings(c.Request.Context(), tenantID, userID, body)
	if err != nil {
		return nil, portalErrorResponse(c, err)
	}
	return api.OK(settings)
}

// Me is GET /api/v1/mobile/portal/me: The account's riders in every operator.
func (ctrl *PortalController) Me(ctx context.Context, _ *api.None) (*api.Out[models.PortalMe], error) {
	c := api.Gin(ctx)
	userID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.PermissionDenied))
	}
	user, memberships, err := ctrl.directory.PortalAccount(c.Request.Context(), userID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	if len(memberships) == 0 {
		return nil, api.Fail(http.StatusForbidden, coreErrors.BuildErrorSingle(c, tenancyErrors.PermissionDenied))
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
	return api.OK(models.PortalMe{User: user, Tenants: tenants})
}
