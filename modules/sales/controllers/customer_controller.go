package controllers

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/sales/interfaces"
	"josex/web/modules/sales/models"
)

// CustomerController handles customer HTTP requests
type CustomerController struct {
	service interfaces.CustomerService
}

// NewCustomerController creates a new customer controller
func NewCustomerController(service interfaces.CustomerService) *CustomerController {
	return &CustomerController{
		service: service,
	}
}

// CreateCustomerInput is CreateCustomer's request.
type CreateCustomerInput struct {
	JSON api.Body[models.CreateCustomerRequestDto]
}

// CreateCustomerResponse is CreateCustomer's answer.
type CreateCustomerResponse struct {
	Data *models.Customer `json:"data"`
}

// CreateCustomer is POST : Create a new customer.
func (ctrl *CustomerController) CreateCustomer(ctx context.Context, in *CreateCustomerInput) (*api.Out[CreateCustomerResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	var dto models.CreateCustomerRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
	}

	customer, err := ctrl.service.CreateCustomer(c.Request.Context(), tenantID, &dto)
	if err != nil {
		status := coreErrors.StatusOf(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(CreateCustomerResponse{Data: customer})
}

// GetCustomerInput is GetCustomer's request.
type GetCustomerInput struct {
	ID string `path:"id"`
}

// GetCustomerResponse is GetCustomer's answer.
type GetCustomerResponse struct {
	Data *models.Customer `json:"data"`
}

// GetCustomer is GET /{id}: Get a customer by ID.
func (ctrl *CustomerController) GetCustomer(ctx context.Context, in *GetCustomerInput) (*api.Out[GetCustomerResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	customer, err := ctrl.service.GetCustomerByID(c.Request.Context(), tenantID, id)
	if err != nil {
		status := coreErrors.StatusOf(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(GetCustomerResponse{Data: customer})
}

// UpdateCustomerInput is UpdateCustomer's request.
type UpdateCustomerInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpdateCustomerRequestDto]
}

// UpdateCustomerResponse is UpdateCustomer's answer.
type UpdateCustomerResponse struct {
	Data *models.Customer `json:"data"`
}

// UpdateCustomer is PATCH /{id}: Update a customer.
func (ctrl *CustomerController) UpdateCustomer(ctx context.Context, in *UpdateCustomerInput) (*api.Out[UpdateCustomerResponse], error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	var dto models.UpdateCustomerRequestDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
	}

	customer, err := ctrl.service.UpdateCustomer(c.Request.Context(), tenantID, id, &dto)
	if err != nil {
		status := coreErrors.StatusOf(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.OK(UpdateCustomerResponse{Data: customer})
}

// DeleteCustomerInput is DeleteCustomer's request.
type DeleteCustomerInput struct {
	ID string `path:"id"`
}

// DeleteCustomer is DELETE /{id}: Delete a customer.
func (ctrl *CustomerController) DeleteCustomer(ctx context.Context, in *DeleteCustomerInput) (*api.Empty, error) {
	c := api.Gin(ctx)
	tenantIDRaw, exists := c.Get("tenant_id")
	if !exists || tenantIDRaw == nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}
	tenantID, err := uuid.Parse(tenantIDRaw.(string))
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, "auth.invalid-tenant"))
	}

	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
	}

	err = ctrl.service.DeleteCustomer(c.Request.Context(), tenantID, id)
	if err != nil {
		status := coreErrors.StatusOf(err)
		return nil, api.Fail(status, coreErrors.BuildError(c, err))
	}

	return api.Done()
}
