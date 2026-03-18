package controllers

import (
	"net/http"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/sales/models"
	"josex/web/modules/sales/services"
	"josex/web/modules/sales/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CustomerController handles customer HTTP requests
type CustomerController struct {
	service *services.CustomerService
}

// NewCustomerController creates a new customer controller
func NewCustomerController(service *services.CustomerService) *CustomerController {
	return &CustomerController{
		service: service,
	}
}

// CreateCustomer godoc
// @Summary Create a new customer
// @Tags Sales - Customers
// @Accept json
// @Produce json
// @Param request body models.CreateCustomerRequestDto true "Customer data"
// @Success 201 {object} models.Customer
// @Failure 400 {object} coreErrors.ErrorResponse
// @Router /sales/customers [post]
func (ctrl *CustomerController) CreateCustomer(c *gin.Context) {
	var dto models.CreateCustomerRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	customer, err := ctrl.service.CreateCustomer(c.Request.Context(), &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": customer})
}

// GetCustomer godoc
// @Summary Get a customer by ID
// @Tags Sales - Customers
// @Produce json
// @Param id path string true "Customer ID"
// @Success 200 {object} models.Customer
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/customers/{id} [get]
func (ctrl *CustomerController) GetCustomer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	customer, err := ctrl.service.GetCustomerByID(c.Request.Context(), id)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": customer})
}

// UpdateCustomer godoc
// @Summary Update a customer
// @Tags Sales - Customers
// @Accept json
// @Produce json
// @Param id path string true "Customer ID"
// @Param request body models.UpdateCustomerRequestDto true "Updated customer data"
// @Success 200 {object} models.Customer
// @Failure 400 {object} coreErrors.ErrorResponse
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/customers/{id} [patch]
func (ctrl *CustomerController) UpdateCustomer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	var dto models.UpdateCustomerRequestDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.failed", err.Error()))
		return
	}

	customer, err := ctrl.service.UpdateCustomer(c.Request.Context(), id, &dto)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": customer})
}

// DeleteCustomer godoc
// @Summary Delete a customer
// @Tags Sales - Customers
// @Produce json
// @Param id path string true "Customer ID"
// @Success 204
// @Failure 404 {object} coreErrors.ErrorResponse
// @Router /sales/customers/{id} [delete]
func (ctrl *CustomerController) DeleteCustomer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, "validation.invalid-uuid"))
		return
	}

	err = ctrl.service.DeleteCustomer(c.Request.Context(), id)
	if err != nil {
		status := utils.GetHTTPStatusFromError(err)
		c.JSON(status, coreErrors.BuildError(c, err))
		return
	}

	c.Status(http.StatusNoContent)
}
