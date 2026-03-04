package repositories

import (
	"context"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/sales/models"

	"github.com/google/uuid"
)

// CustomerRepository implements customer data operations
type CustomerRepository struct {
	dbService coreServices.DatabaseService
}

// NewCustomerRepository creates a new customer repository
func NewCustomerRepository(dbService coreServices.DatabaseService) *CustomerRepository {
	return &CustomerRepository{
		dbService: dbService,
	}
}

// CreateCustomer creates a new customer
func (r *CustomerRepository) CreateCustomer(ctx context.Context, dto *models.CreateCustomerRequestDto, tenantID uuid.UUID) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_create_customer($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tenantID,
		dto.Name,
		dto.Email,
		dto.PhoneNumber,
		dto.Address,
		dto.City,
		dto.State,
		dto.PostalCode,
		dto.Country,
	).Scan(
		&customer.ID,
		&customer.TenantID,
		&customer.Name,
		&customer.Email,
		&customer.PhoneNumber,
		&customer.Address,
		&customer.City,
		&customer.State,
		&customer.PostalCode,
		&customer.Country,
		&customer.Status,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	return customer, err
}

// GetCustomerByID retrieves a customer by ID
func (r *CustomerRepository) GetCustomerByID(ctx context.Context, id uuid.UUID) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_get_customer_by_id($1)`,
		id,
	).Scan(
		&customer.ID,
		&customer.TenantID,
		&customer.Name,
		&customer.Email,
		&customer.PhoneNumber,
		&customer.Address,
		&customer.City,
		&customer.State,
		&customer.PostalCode,
		&customer.Country,
		&customer.Status,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)
	return customer, err
}

// GetCustomersByTenant retrieves all customers for a tenant
func (r *CustomerRepository) GetCustomersByTenant(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.Customer, error) {
	var customers []models.Customer
	rows, err := r.dbService.Query(ctx,
		`CALL sales.sp_get_customers_by_tenant($1, $2, $3)`,
		tenantID, limit, offset,
	)
	if err != nil {
		return customers, err
	}
	defer rows.Close()

	for rows.Next() {
		var c models.Customer
		err := rows.Scan(
			&c.ID, &c.TenantID, &c.Name, &c.Email, &c.PhoneNumber,
			&c.Address, &c.City, &c.State, &c.PostalCode, &c.Country,
			&c.Status, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			continue
		}
		customers = append(customers, c)
	}
	return customers, nil
}

// UpdateCustomer updates a customer
func (r *CustomerRepository) UpdateCustomer(ctx context.Context, id uuid.UUID, dto *models.UpdateCustomerRequestDto) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_update_customer($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id,
		dto.Name,
		dto.PhoneNumber,
		dto.Address,
		dto.City,
		dto.State,
		dto.PostalCode,
		dto.Country,
	).Scan(
		&customer.ID, &customer.TenantID, &customer.Name, &customer.Email, &customer.PhoneNumber,
		&customer.Address, &customer.City, &customer.State, &customer.PostalCode, &customer.Country,
		&customer.Status, &customer.CreatedAt, &customer.UpdatedAt,
	)
	return customer, err
}

// DeleteCustomer deletes a customer
func (r *CustomerRepository) DeleteCustomer(ctx context.Context, id uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `CALL sales.sp_delete_customer($1)`, id)
	return err
}

// GetCustomerByEmail retrieves a customer by email
func (r *CustomerRepository) GetCustomerByEmail(ctx context.Context, email string, tenantID uuid.UUID) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`CALL sales.sp_get_customer_by_email($1, $2)`,
		email, tenantID,
	).Scan(
		&customer.ID, &customer.TenantID, &customer.Name, &customer.Email, &customer.PhoneNumber,
		&customer.Address, &customer.City, &customer.State, &customer.PostalCode, &customer.Country,
		&customer.Status, &customer.CreatedAt, &customer.UpdatedAt,
	)
	return customer, err
}
