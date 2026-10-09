package controllers

import (
	"errors"
	"net/http"

	billingErrors "josex/web/modules/billing/errors"
	billingModels "josex/web/modules/billing/models"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leebenson/conform"
)

// tenantID is the business the tenant chain resolved from X-Tenant-Slug.
func tenantID(c *gin.Context) (uuid.UUID, bool) {
	raw, ok := c.Get("tenant_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return uuid.Nil, false
	}
	return id, true
}

// GetTenantPlan godoc
// @Summary The business's plan
// @Description Plan, cycle, status and end of the X-Tenant-Slug business, the limits it has now (the free plan's once expired), the price list and where to pay (BILLING-001).
// @Tags Billing
// @Produce json
// @Success 200 {object} billingModels.TenantPlan
// @Router /billing/tenant/plan [get]
// @Security ApiKeyAuth
func (ctrl *BillingController) GetTenantPlan(c *gin.Context) {
	id, ok := tenantID(c)
	if !ok {
		return
	}
	plan, err := ctrl.billingService.GetTenantPlan(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, plan)
}

// GetTenantUsage godoc
// @Summary The business's usage against its limits
// @Description Riders, vehicles, products and members the business holds, each with its plan limit (-1 unlimited).
// @Tags Billing
// @Produce json
// @Success 200 {object} billingModels.UsageResponse
// @Router /billing/usage [get]
// @Security ApiKeyAuth
func (ctrl *BillingController) GetTenantUsage(c *gin.Context) {
	id, ok := tenantID(c)
	if !ok {
		return
	}
	usage, err := ctrl.billingService.GetTenantUsage(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, usage)
}

// NotifyPayment godoc
// @Summary "Ya pagué"
// @Description Records the business's bank QR payment for a plan and cycle, for the platform to confirm (BILLING-001 D3).
// @Tags Billing
// @Accept json
// @Produce json
// @Param body body billingModels.NotifyPaymentDto true "Plan, cycle and bank reference"
// @Success 201 {object} billingModels.TenantPayment
// @Router /billing/payments/notice [post]
// @Security ApiKeyAuth
func (ctrl *BillingController) NotifyPayment(c *gin.Context) {
	id, ok := tenantID(c)
	if !ok {
		return
	}
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}
	var dto billingModels.NotifyPaymentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)
	payment, err := ctrl.billingService.NotifyPayment(c.Request.Context(), id, userID, &dto)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusCreated, payment)
}

// ListNotifiedPayments godoc
// @Summary Payments waiting for confirmation
// @Description The platform's queue of notified payments, oldest first; super_admin only.
// @Tags Billing
// @Produce json
// @Success 200 {array} billingModels.NotifiedPayment
// @Router /billing/payments/pending [get]
// @Security ApiKeyAuth
func (ctrl *BillingController) ListNotifiedPayments(c *gin.Context) {
	payments, err := ctrl.billingService.ListNotifiedPayments(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"payments": payments})
}

// ConfirmPayment godoc
// @Summary Confirm a payment
// @Description Completes a notified payment and extends the business's plan one cycle from the later of now and its end (BILLING-001 D3); super_admin only.
// @Tags Billing
// @Produce json
// @Param id path string true "Payment ID"
// @Success 200 {object} billingModels.TenantPayment
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /billing/payments/{id}/confirm [post]
// @Security ApiKeyAuth
func (ctrl *BillingController) ConfirmPayment(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}
	payment, err := ctrl.billingService.ConfirmPayment(c.Request.Context(), paymentID, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case billingErrors.PaymentNotFound:
				c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, billingErrors.PaymentNotFound))
				return
			case billingErrors.PaymentNotPending:
				c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, billingErrors.PaymentNotPending))
				return
			}
		}
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, payment)
}

// paymentWriteError answers the codes a payment write raises; reports whether it wrote the response.
func paymentWriteError(c *gin.Context, err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	switch pgErr.Message {
	case billingErrors.PaymentNotFound, billingErrors.TenantNotFound:
		c.JSON(http.StatusNotFound, coreErrors.BuildErrorSingle(c, pgErr.Message))
		return true
	case billingErrors.PaymentNotPending:
		c.JSON(http.StatusConflict, coreErrors.BuildErrorSingle(c, pgErr.Message))
		return true
	case billingErrors.AdjustReasonRequired:
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, pgErr.Message))
		return true
	}
	return false
}

// RejectPayment godoc
// @Summary Reject a payment
// @Description Turns a notified payment down with the reason the customer is emailed; the plan does not change (BILLING-002 D3); super_admin only.
// @Tags Billing
// @Accept json
// @Produce json
// @Param id path string true "Payment ID"
// @Param body body billingModels.RejectPaymentDto true "Why"
// @Success 200 {object} billingModels.TenantPayment
// @Failure 404 {object} coreErrors.ErrorResponse
// @Failure 409 {object} coreErrors.ErrorResponse
// @Router /billing/payments/{id}/reject [post]
// @Security ApiKeyAuth
func (ctrl *BillingController) RejectPayment(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}
	var dto billingModels.RejectPaymentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)
	payment, err := ctrl.billingService.RejectPayment(c.Request.Context(), paymentID, dto.Reason, userID)
	if err != nil {
		if !paymentWriteError(c, err) {
			c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		}
		return
	}
	c.JSON(http.StatusOK, payment)
}

// ListTenantSubscriptions godoc
// @Summary Every business and its plan
// @Description The platform's businesses list: plan, cycle, status, end and the notice under review, searched by name or code (BILLING-002 D3); super_admin only.
// @Tags Billing
// @Produce json
// @Param search query string false "Name or code"
// @Param page query int false "Page" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} billingModels.ListTenantSubscriptionsResponse
// @Router /billing/admin/subscriptions [get]
// @Security ApiKeyAuth
func (ctrl *BillingController) ListTenantSubscriptions(c *gin.Context) {
	var query billingModels.ListTenantSubscriptionsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}
	out, err := ctrl.billingService.ListTenantSubscriptions(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// AdjustTenantSubscription godoc
// @Summary Set a business's plan by hand
// @Description Plan, cycle, status and end with a required reason; recorded as an adjustment with before and after (BILLING-002 D3); super_admin only.
// @Tags Billing
// @Accept json
// @Produce json
// @Param business_id path string true "Business ID"
// @Param body body billingModels.AdjustSubscriptionDto true "The plan and why"
// @Success 200 {object} billingModels.TenantPlan
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /billing/admin/tenants/{business_id}/subscription [put]
// @Security ApiKeyAuth
func (ctrl *BillingController) AdjustTenantSubscription(c *gin.Context) {
	// The business the platform acts on, named in the path — never the caller's own tenant.
	tenantID, err := uuid.Parse(c.Param("business_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
		return
	}
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}
	var dto billingModels.AdjustSubscriptionDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}
	_ = conform.Strings(&dto)
	plan, err := ctrl.billingService.AdjustTenantSubscription(c.Request.Context(), tenantID, &dto, userID)
	if err != nil {
		if !paymentWriteError(c, err) {
			c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		}
		return
	}
	c.JSON(http.StatusOK, plan)
}
