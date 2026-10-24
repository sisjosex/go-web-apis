package controllers

import (
	"context"
	"net/http"
	"strconv"

	"josex/web/config"
	billingErrors "josex/web/modules/billing/errors"
	billingInterfaces "josex/web/modules/billing/interfaces"
	billingModels "josex/web/modules/billing/models"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"

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

// GetPlanInfo is GET /plan: Get current plan info and feature limits.
func (ctrl *BillingController) GetPlanInfo(ctx context.Context, _ *api.None) (*api.Out[*billingModels.PlanInfo], error) {
	c := api.Gin(ctx)
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}

	info, err := ctrl.billingService.GetPlanInfo(c.Request.Context(), userID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(info)
}

// GetSubscription is GET /subscription: Get current subscription.
func (ctrl *BillingController) GetSubscription(ctx context.Context, _ *api.None) (*api.Out[any], error) {
	c := api.Gin(ctx)
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}

	sub, err := ctrl.billingService.GetSubscription(c.Request.Context(), userID)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	if sub == nil {
		return api.OK[any](gin.H{"subscription": nil})
	}

	return api.OK[any](sub)
}

// UpsertSubscriptionInput is UpsertSubscription's request.
type UpsertSubscriptionInput struct {
	JSON api.Body[billingModels.UpsertSubscriptionDto]
}

// UpsertSubscription is PUT /subscription: Create or upgrade a subscription.
func (ctrl *BillingController) UpsertSubscription(ctx context.Context, in *UpsertSubscriptionInput) (*api.Out[*billingModels.Subscription], error) {
	c := api.Gin(ctx)
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}

	var dto billingModels.UpsertSubscriptionDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
	}

	sub, err := ctrl.billingService.UpsertSubscription(c.Request.Context(), userID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(sub)
}

// RecordPaymentInput is RecordPayment's request.
type RecordPaymentInput struct {
	JSON api.Body[billingModels.RecordPaymentDto]
}

// RecordPayment is POST /payments: Record a payment.
func (ctrl *BillingController) RecordPayment(ctx context.Context, in *RecordPaymentInput) (*api.Out[*billingModels.Payment], error) {
	c := api.Gin(ctx)
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}

	var dto billingModels.RecordPaymentDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
	}

	payment, err := ctrl.billingService.RecordPayment(c.Request.Context(), userID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}

	return api.OK(payment)
}

// ListPaymentsInput is ListPayments's request.
type ListPaymentsInput struct {
	Page  string `query:"page" default:"1"`
	Limit string `query:"limit"`
}

// ListPayments is GET /payments: List payment history.
func (ctrl *BillingController) ListPayments(ctx context.Context, in *ListPaymentsInput) (*api.Out[*billingModels.PaymentListResponse], error) {
	c := api.Gin(ctx)
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}

	billingConf := config.ModularAppConfig.Billing
	page, _ := strconv.Atoi(in.Page)
	limit, _ := strconv.Atoi(in.Limit)

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
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	return api.OK(resp)
}

// extractUserID pulls the authenticated user's UUID from the Gin context.
func extractUserID(c *gin.Context) (uuid.UUID, error) {
	userIDStr, _ := c.Get("user_id")
	return uuid.Parse(userIDStr.(string))
}
