package services

import (
	"context"

	"josex/web/modules/sales/interfaces"
	"josex/web/modules/sales/models"

	"github.com/google/uuid"
)


// CustomerService handles customer business logic
type CustomerService struct {
	repository interfaces.CustomerRepository
}

// NewCustomerService creates a new customer service
func NewCustomerService(repo interfaces.CustomerRepository) *CustomerService {
	return &CustomerService{
		repository: repo,
	}
}

// CreateCustomer creates a new customer
func (s *CustomerService) CreateCustomer(ctx context.Context, dto *models.CreateCustomerRequestDto) (*models.Customer, error) {
	return s.repository.CreateCustomer(ctx, dto)
}

// GetCustomerByID retrieves a customer by ID
func (s *CustomerService) GetCustomerByID(ctx context.Context, id uuid.UUID) (*models.Customer, error) {
	return s.repository.GetCustomerByID(ctx, id)
}

// GetAllCustomers retrieves all customers
func (s *CustomerService) GetAllCustomers(ctx context.Context, limit int, offset int) ([]models.Customer, error) {
	return s.repository.GetAllCustomers(ctx, limit, offset)
}

// UpdateCustomer updates a customer
func (s *CustomerService) UpdateCustomer(ctx context.Context, id uuid.UUID, dto *models.UpdateCustomerRequestDto) (*models.Customer, error) {
	return s.repository.UpdateCustomer(ctx, id, dto)
}

// DeleteCustomer deletes a customer
func (s *CustomerService) DeleteCustomer(ctx context.Context, id uuid.UUID) error {
	return s.repository.DeleteCustomer(ctx, id)
}
