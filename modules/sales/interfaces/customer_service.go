package interfaces

import (
	"context"

	"josex/web/modules/sales/models"

	"github.com/google/uuid"
)

// CustomerService defines customer business logic operations
type CustomerService interface {
	CreateCustomer(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCustomerRequestDto) (*models.Customer, error)
	GetCustomerByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.Customer, error)
	GetAllCustomers(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.Customer, error)
	UpdateCustomer(ctx context.Context, tenantID uuid.UUID, id uuid.UUID, dto *models.UpdateCustomerRequestDto) (*models.Customer, error)
	DeleteCustomer(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error
}
