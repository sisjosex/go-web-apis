package repositories

import (
	"context"
	"errors"

	coreServices "josex/web/modules/core/services"
	"josex/web/modules/sales/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
func (r *CustomerRepository) CreateCustomer(ctx context.Context, tenantID uuid.UUID, dto *models.CreateCustomerRequestDto) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM sales.sp_create_customer($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
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
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return customer, nil
}

// GetCustomerByID retrieves a customer by ID
func (r *CustomerRepository) GetCustomerByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM sales.sp_get_customer_by_id($1, $2)`,
		tenantID,
		id,
	).Scan(
		&customer.ID,
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
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return customer, nil
}

// GetAllCustomers retrieves all customers
func (r *CustomerRepository) GetAllCustomers(ctx context.Context, tenantID uuid.UUID, limit int, offset int) ([]models.Customer, error) {
	var customers []models.Customer
	rows, err := r.dbService.Query(ctx,
		`SELECT * FROM sales.sp_get_all_customers($1, $2, $3)`,
		tenantID, limit, offset,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c models.Customer
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Email, &c.PhoneNumber,
			&c.Address, &c.City, &c.State, &c.PostalCode, &c.Country,
			&c.Status, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if customers == nil {
		customers = []models.Customer{}
	}
	return customers, nil
}

// UpdateCustomer updates a customer
func (r *CustomerRepository) UpdateCustomer(ctx context.Context, tenantID uuid.UUID, id uuid.UUID, dto *models.UpdateCustomerRequestDto) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM sales.sp_update_customer($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		tenantID,
		id,
		dto.Name,
		dto.PhoneNumber,
		dto.Address,
		dto.City,
		dto.State,
		dto.PostalCode,
		dto.Country,
	).Scan(
		&customer.ID, &customer.Name, &customer.Email, &customer.PhoneNumber,
		&customer.Address, &customer.City, &customer.State, &customer.PostalCode, &customer.Country,
		&customer.Status, &customer.CreatedAt, &customer.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return customer, nil
}

// DeleteCustomer deletes a customer
func (r *CustomerRepository) DeleteCustomer(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `CALL sales.sp_delete_customer($1, $2)`, tenantID, id)
	return err
}

// GetCustomerByEmail retrieves a customer by email
func (r *CustomerRepository) GetCustomerByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*models.Customer, error) {
	customer := &models.Customer{}
	err := r.dbService.QueryRow(ctx,
		`SELECT * FROM sales.sp_get_customer_by_email($1, $2)`,
		tenantID,
		email,
	).Scan(
		&customer.ID, &customer.Name, &customer.Email, &customer.PhoneNumber,
		&customer.Address, &customer.City, &customer.State, &customer.PostalCode, &customer.Country,
		&customer.Status, &customer.CreatedAt, &customer.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return customer, nil
}
