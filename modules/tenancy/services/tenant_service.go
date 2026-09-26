package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"josex/web/config"
	coreConfig "josex/web/modules/core/config"
	coreServices "josex/web/modules/core/services"
	"josex/web/modules/tenancy/interfaces"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type tenantService struct {
	tenantRepository interfaces.TenantRepository
	dbService        coreServices.DatabaseService
}

// NewTenantService creates a new instance of TenantService
func NewTenantService(tenantRepository interfaces.TenantRepository, dbService coreServices.DatabaseService) interfaces.TenantService {
	return &tenantService{
		tenantRepository: tenantRepository,
		dbService:        dbService,
	}
}

// CreateTenant creates a new tenant
func (s *tenantService) CreateTenant(ctx context.Context, dto *models.CreateTenantDto, creatorUserID uuid.UUID) (*models.TenantDetailResponse, error) {
	// Validate custom database URL permission
	tenancyConf := config.ModularAppConfig.Tenancy
	if dto.DatabaseURL != nil && *dto.DatabaseURL != "" {
		if !tenancyConf.AllowCustomDatabaseURLs {
			return nil, fmt.Errorf("tenant.database-url.not-allowed")
		}
	}

	tenant, err := s.tenantRepository.CreateTenant(ctx, dto, creatorUserID)
	if err != nil {
		return nil, err
	}

	// Run migrations on tenant database if custom database_url is provided
	if tenant.DatabaseURL != nil && *tenant.DatabaseURL != "" {
		log.Printf("🔄 Running migrations for tenant '%s' on custom database...", tenant.Slug)

		if err := s.runTenantMigrations(*tenant.DatabaseURL); err != nil {
			log.Printf("⚠️  Failed to run migrations for tenant '%s': %v", tenant.Slug, err)
			// Don't fail tenant creation, but log the error
			// Admin can manually run migrations later via endpoint
		} else {
			log.Printf("✅ Migrations completed successfully for tenant '%s'", tenant.Slug)
		}
	}

	// Convert to response (exclude sensitive database_url)
	response := &models.TenantDetailResponse{
		ID:          tenant.ID,
		Slug:        tenant.Slug,
		Name:        tenant.Name,
		SchemaName:  tenant.SchemaName,
		IsActive:    tenant.IsActive,
		IsSuspended: tenant.IsSuspended,
		Settings:    tenant.Settings,
		CreatedAt:   tenant.CreatedAt,
		UpdatedAt:   tenant.UpdatedAt,
	}

	return response, nil
}

// runTenantMigrations executes all migrations on a tenant's database
// Excludes 'tenancy' module as it should only exist in the main database
func (s *tenantService) runTenantMigrations(databaseURL string) error {
	// Open connection to tenant database
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to tenant database: %w", err)
	}
	defer sqlDB.Close()

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping tenant database: %w", err)
	}

	// Create a copy of core config with excluded modules
	// Strategy: Each Tenant DB has its own auth/users (local tenant users)
	// Only tenancy management stays in Main DB (multi-tenancy control)
	mainConfig := config.ModularAppConfig.Core
	excludedModules := map[string]bool{
		"tenancy": true, // Tenant management only in Main DB
		"geo":     true, // OSM places are the same for every tenant: the server's own DB (INFRA-003)
	}

	// Always include 'core' module for base extensions (uuid-ossp, etc.)
	tenantEnabledModules := []string{"core"}
	for _, module := range mainConfig.EnabledModules {
		if !excludedModules[module] && module != "core" {
			tenantEnabledModules = append(tenantEnabledModules, module)
		}
	}

	// Create a custom config for tenant migrations (copy of core config)
	tenantCoreConfig := &coreConfig.CoreConfig{
		DatabaseURL:           mainConfig.DatabaseURL,
		DatabasePoolSize:      mainConfig.DatabasePoolSize,
		EnabledModules:        tenantEnabledModules,             // Filtered list without 'tenancy', 'auth', 'users'
		ExcludedFromMigration: mainConfig.ExcludedFromMigration, // Keep exclusions from main config
		AppMode:               mainConfig.AppMode,
		AppHost:               mainConfig.AppHost,
		AppPort:               mainConfig.AppPort,
		FrontendURL:           mainConfig.FrontendURL,
		LogLevel:              mainConfig.LogLevel,
		AllowedOrigins:        mainConfig.AllowedOrigins,
	}

	log.Printf("📋 Migrating modules for tenant DB: %v (excluded: tenancy)", tenantEnabledModules)

	// Create migration service for tenant database with filtered modules
	migrationService := coreServices.NewMigrationService(sqlDB, tenantCoreConfig)

	// Run filtered module migrations
	if err := migrationService.RunMigrations(); err != nil {
		return fmt.Errorf("migration execution failed: %w", err)
	}

	return nil
}

// GetUserTenants gets user's accessible tenants
func (s *tenantService) GetUserTenants(ctx context.Context, userID uuid.UUID) ([]*models.UserTenantResponse, error) {
	return s.tenantRepository.GetUserTenants(ctx, userID)
}

// UpdateTenant updates tenant information
func (s *tenantService) UpdateTenant(ctx context.Context, tenantID uuid.UUID, dto *models.UpdateTenantDto) (*models.TenantDetailResponse, error) {
	tenant, err := s.tenantRepository.UpdateTenant(ctx, tenantID, dto)
	if err != nil {
		return nil, err
	}

	// Convert to response (exclude sensitive database_url)
	response := &models.TenantDetailResponse{
		ID:          tenant.ID,
		Slug:        tenant.Slug,
		Name:        tenant.Name,
		SchemaName:  tenant.SchemaName,
		IsActive:    tenant.IsActive,
		IsSuspended: tenant.IsSuspended,
		Settings:    tenant.Settings,
		CreatedAt:   tenant.CreatedAt,
		UpdatedAt:   tenant.UpdatedAt,
	}

	return response, nil
}

// AddUserToTenant adds a user to a tenant
func (s *tenantService) AddUserToTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, dto *models.AddUserToTenantDto) error {
	_, err := s.tenantRepository.AddUserToTenant(ctx, tenantID, requesterUserID, dto.UserID, dto.Role)
	return err
}

// RemoveUserFromTenant removes a user from a tenant
func (s *tenantService) RemoveUserFromTenant(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID) error {
	return s.tenantRepository.RemoveUserFromTenant(ctx, tenantID, requesterUserID, userID)
}

// GetTenantBySlug returns tenant by slug (used by middleware for super_admin access)
func (s *tenantService) GetTenantBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	return s.tenantRepository.GetTenantBySlug(ctx, slug)
}

// VerifyUserTenantAccess verifies user has access to tenant (used by middleware)
func (s *tenantService) VerifyUserTenantAccess(ctx context.Context, userID uuid.UUID, slug string) (*models.TenantAccessInfo, error) {
	return s.tenantRepository.VerifyUserTenantAccess(ctx, userID, slug)
}

// CountUserOwnedTenants counts how many tenants a user owns
func (s *tenantService) CountUserOwnedTenants(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.tenantRepository.CountUserOwnedTenants(ctx, userID)
}

// GetTenantEnabledModuleCodes returns a map of enabled module codes for a tenant.
// The tenantService delegates to the moduleRepository when available; this stub
// returns an empty map and is overridden by LoadTenantModules middleware.
func (s *tenantService) GetTenantEnabledModuleCodes(ctx context.Context, tenantID uuid.UUID) (map[string]bool, error) {
	return make(map[string]bool), nil
}

// RunTenantMigrations runs migrations on a specific tenant's database
func (s *tenantService) RunTenantMigrations(ctx context.Context, tenantSlug string) error {
	// Get tenant details
	tenant, err := s.tenantRepository.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}

	// Only run migrations if tenant has custom database_url
	if tenant.DatabaseURL == nil || *tenant.DatabaseURL == "" {
		return fmt.Errorf("tenant uses shared database - migrations run automatically on main DB")
	}

	log.Printf("🔄 Running migrations for tenant '%s' on database: %s", tenantSlug, *tenant.DatabaseURL)

	if err := s.runTenantMigrations(*tenant.DatabaseURL); err != nil {
		log.Printf("❌ Migration failed for tenant '%s': %v", tenantSlug, err)
		return fmt.Errorf("migration execution failed: %w", err)
	}

	log.Printf("✅ Migrations completed successfully for tenant '%s'", tenantSlug)
	return nil
}

// UpdateUserRole updates a user's role within a tenant
func (s *tenantService) UpdateUserRole(ctx context.Context, tenantID uuid.UUID, requesterUserID uuid.UUID, userID uuid.UUID, role string) error {
	return s.tenantRepository.UpdateUserRole(ctx, tenantID, requesterUserID, userID, role)
}
