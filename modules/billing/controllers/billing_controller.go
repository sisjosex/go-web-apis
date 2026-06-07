package controllers

import (
	"net/http"
	"strconv"

	"josex/web/config"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"
	billingErrors "josex/web/modules/billing/errors"
	billingInterfaces "josex/web/modules/billing/interfaces"
	billingModels "josex/web/modules/billing/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BillingController handles HTTP requests for the billing module.
type BillingController struct {
	billingService billingInterfaces.BillingService
}

// NewBillingController constructs a new BillingController.
func NewBillingController(billingService billingInterfaces.BillingService) *BillingController {
	return &BillingController{billingService: billingService}
}

// GetPlanInfo godoc
// @Summary Get current plan info and feature limits
// @Description Returns the authenticated user's active plan, subscription status, expiry date, and all feature limits.
// @Tags Billing
// @Produce json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {object} billingModels.PlanInfo
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /billing/plan [get]
// @Security ApiKeyAuth
func (ctrl *BillingController) GetPlanInfo(c *gin.Context) {
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}

	info, err := ctrl.billingService.GetPlanInfo(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, info)
}

// GetSubscription godoc
// @Summary Get current subscription
// @Description Returns the authenticated user's active subscription, or null if none.
// @Tags Billing
// @Produce json
// @Param Authorization header string true "Bearer Token"
// @Success 200 {object} billingModels.Subscription
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /billing/subscription [get]
// @Security ApiKeyAuth
func (ctrl *BillingController) GetSubscription(c *gin.Context) {
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}

	sub, err := ctrl.billingService.GetSubscription(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	if sub == nil {
		c.JSON(http.StatusOK, gin.H{"subscription": nil})
		return
	}

	c.JSON(http.StatusOK, sub)
}

// UpsertSubscription godoc
// @Summary Create or upgrade a subscription
// @Description Creates a new subscription or upgrades the existing one. Cancels the old subscription when switching plans.
// @Tags Billing
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer Token"
// @Param request body billingModels.UpsertSubscriptionDto true "Subscription details"
// @Success 200 {object} billingModels.Subscription
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /billing/subscription [put]
// @Security ApiKeyAuth
func (ctrl *BillingController) UpsertSubscription(c *gin.Context) {
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}

	var dto billingModels.UpsertSubscriptionDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	sub, err := ctrl.billingService.UpsertSubscription(c.Request.Context(), userID, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, sub)
}

// RecordPayment godoc
// @Summary Record a payment
// @Description Records a payment against an active subscription.
// @Tags Billing
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer Token"
// @Param request body billingModels.RecordPaymentDto true "Payment details"
// @Success 201 {object} billingModels.Payment
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /billing/payments [post]
// @Security ApiKeyAuth
func (ctrl *BillingController) RecordPayment(c *gin.Context) {
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}

	var dto billingModels.RecordPaymentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
		return
	}

	payment, err := ctrl.billingService.RecordPayment(c.Request.Context(), userID, &dto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, payment)
}

// ListPayments godoc
// @Summary List payment history
// @Description Returns paginated payment history for the authenticated user.
// @Tags Billing
// @Produce json
// @Param Authorization header string true "Bearer Token"
// @Param page  query int false "Page number (default: 1)"
// @Param limit query int false "Items per page"
// @Success 200 {object} billingModels.PaymentListResponse
// @Failure 401 {object} coreErrors.ErrorResponse
// @Router /billing/payments [get]
// @Security ApiKeyAuth
func (ctrl *BillingController) ListPayments(c *gin.Context) {
	userID, err := extractUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
		return
	}

	billingConf := config.ModularAppConfig.Billing
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(billingConf.DefaultPageSize)))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = billingConf.DefaultPageSize
	}
	if limit > billingConf.MaxPageSize {
		limit = billingConf.MaxPageSize
	}

	resp, err := ctrl.billingService.ListPayments(c.Request.Context(), userID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, resp)
}

// extractUserID pulls the authenticated user's UUID from the Gin context.
func extractUserID(c *gin.Context) (uuid.UUID, error) {
	userIDStr, _ := c.Get("user_id")
	return uuid.Parse(userIDStr.(string))
}
