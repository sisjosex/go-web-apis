package interfaces

import (
	"context"

	"josex/web/modules/sales/models"

	"github.com/google/uuid"
)

// CustomerRepository defines customer data access operations
type CustomerRepository interface {
	CreateCustomer(ctx context.Context, dto *models.CreateCustomerRequestDto, tenantID uuid.UUID) (*models.Customer, error)
	GetCustomerByID(ctx context.Context, id uuid.UUID) (*models.Customer, error)
	GetCustomersByTenant(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.Customer, error)
	UpdateCustomer(ctx context.Context, id uuid.UUID, dto *models.UpdateCustomerRequestDto) (*models.Customer, error)
	DeleteCustomer(ctx context.Context, id uuid.UUID) error
	GetCustomerByEmail(ctx context.Context, email string, tenantID uuid.UUID) (*models.Customer, error)
}
