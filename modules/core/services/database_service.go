package services

import (
	"context"
	"database/sql"
	"josex/web/config"
	coreConfig "josex/web/modules/core/config"
	"log"
	"os"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DatabaseService interface {
	InitDatabase(ctx context.Context)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Execute(ctx context.Context, query string, args ...interface{}) (int64, error)
	CloseDatabase(ctx context.Context)
	BeginTransaction(ctx context.Context) (pgx.Tx, error)
	// Tenant-specific methods
	GetPoolForTenant(ctx context.Context, databaseURL string) (*pgxpool.Pool, error)
	GetPrimaryPool() *pgxpool.Pool
}

type databaseService struct {
	pool        *pgxpool.Pool
	tenantPools map[string]*pgxpool.Pool // Cache of tenant database pools
}

func NewDatabaseService() DatabaseService {
	return &databaseService{
		tenantPools: make(map[string]*pgxpool.Pool),
	}
}

func (ds *databaseService) InitDatabase(ctx context.Context) {
	retryInterval := 5 * time.Second

	// Use modular config
	coreConf := config.ModularAppConfig.Core
	dataBaseUrl := coreConf.DatabaseURL

	// Check if we should skip migrations (for tests)
	skipMigrations := os.Getenv("SKIP_MIGRATIONS") == "true"

	for {
		pool, err := connectDatabase(ctx, dataBaseUrl, coreConf.DatabasePoolSize)
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
		}

		ds.pool = pool

		// Run modular migrations (unless skipped)
		if skipMigrations {
			log.Println("⏭️  Skipping migrations (SKIP_MIGRATIONS=true)")
			return
		}

		if err := ds.runModularMigrations(dataBaseUrl); err == nil {
			log.Println("✅ All modular migrations completed successfully")
			return
		} else {
			log.Printf("Failed to run migrations: %v", err)
		}

		time.Sleep(retryInterval)
	}
}

// Conecta a la base de datos con el tamaño de pool especificado
func connectDatabase(ctx context.Context, dbURL string, poolSize int32) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}

	config.MaxConns = poolSize
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	// Verificar conexión inmediatamente
	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}
	defer conn.Release()

	return pool, nil
}

func (ds *databaseService) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return ds.pool.Query(ctx, query, args...)
}

func (ds *databaseService) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return ds.pool.QueryRow(ctx, query, args...)
}

func (ds *databaseService) Execute(ctx context.Context, query string, args ...interface{}) (int64, error) {
	commandTag, err := ds.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return commandTag.RowsAffected(), nil
}

func (ds *databaseService) CloseDatabase(ctx context.Context) {
	if ds.pool == nil {
		log.Println("Database pool is nil. Skipping close.")
		return
	}

	// Close primary pool
	ds.pool.Close()
	ds.pool = nil

	// Close all tenant pools
	for dbURL, pool := range ds.tenantPools {
		log.Printf("Closing tenant pool: %s", dbURL)
		pool.Close()
	}
	ds.tenantPools = make(map[string]*pgxpool.Pool)

	log.Println("Database closed")
}

// runModularMigrations executes migrations for all enabled modules
func (ds *databaseService) runModularMigrations(databaseURL string) error {
	// Open standard SQL connection for migration library
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	// Don't defer close here - close after all migrations

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return err
	}

	// Load core config to get enabled modules
	coreConf := config.ModularAppConfig.Core
	if coreConf == nil {
		coreConf = coreConfig.LoadCoreConfig()
	}

	// Create migration service with module configuration
	migrationService := NewMigrationService(sqlDB, coreConf)

	// Print module status
	migrationService.PrintModuleStatus()

	// Run migrations
	migrateErr := migrationService.RunMigrations()

	// Close SQL connection after all migrations
	sqlDB.Close()

	return migrateErr
}

func (ds *databaseService) BeginTransaction(ctx context.Context) (pgx.Tx, error) {
	tx, err := ds.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// GetPoolForTenant returns a connection pool for a tenant database
// If the pool doesn't exist, it creates and caches it
func (ds *databaseService) GetPoolForTenant(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	// Check if pool already exists in cache
	if pool, exists := ds.tenantPools[databaseURL]; exists {
		return pool, nil
	}

	// Get tenant pool size from config
	tenancyConf := config.ModularAppConfig.Tenancy
	poolSize := int32(5) // Default
	if tenancyConf != nil {
		poolSize = tenancyConf.TenantDatabasePoolSize
	}

	// Create new pool
	pool, err := connectDatabase(ctx, databaseURL, poolSize)
	if err != nil {
		return nil, err
	}

	// Cache the pool
	ds.tenantPools[databaseURL] = pool
	log.Printf("Created tenant database pool for: %s (pool size: %d)", databaseURL, poolSize)

	return pool, nil
}

// GetPrimaryPool returns the primary database pool
func (ds *databaseService) GetPrimaryPool() *pgxpool.Pool {
	return ds.pool
}
