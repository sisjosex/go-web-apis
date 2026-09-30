package services

import (
	"context"
	"database/sql"
	"josex/web/config"
	coreConfig "josex/web/modules/core/config"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// tenantDBURLKey is a typed context key to avoid collisions
type tenantDBURLKey struct{}

// TenantDatabaseURLKey is the context key used by tenant middleware to inject
// the tenant's database URL into the request context
var TenantDatabaseURLKey = tenantDBURLKey{}

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
	mu          sync.RWMutex             // Protects tenantPools
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
		pool, err := connectDatabase(ctx, dataBaseUrl, coreConf.DatabasePoolSize, "josex_primary")
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
			time.Sleep(retryInterval)
			continue
		}

		ds.pool = pool

		// Run modular migrations (unless skipped)
		if skipMigrations {
			log.Println("⏭️  Skipping migrations (SKIP_MIGRATIONS=true)")
			LoadSettings(ctx, ds)
			return
		}

		if err := RunModularMigrations(dataBaseUrl); err == nil {
			log.Println("✅ All modular migrations completed successfully")
			// core.settings exists only once migrations have run.
			LoadSettings(ctx, ds)
			return
		} else {
			log.Printf("Failed to run migrations: %v", err)
		}

		time.Sleep(retryInterval)
	}
}

// connectDatabase creates a pgxpool with the given pool size and application_name.
// appName is surfaced in pg_stat_activity and helps distinguish primary vs tenant pools.
func connectDatabase(ctx context.Context, dbURL string, poolSize int32, appName string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}

	config.MaxConns = poolSize
	// A tenant nobody is using hands its connections back within minutes, so the budget of
	// max_connections (INFRA-004 D1) is spent on the tenants that are active, not on every pool ever opened.
	config.MaxConnIdleTime = 5 * time.Minute
	if appName != "" {
		config.ConnConfig.RuntimeParams["application_name"] = appName
	}
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

// resolvePool returns the appropriate pool based on tenant context.
// If the context contains a non-empty tenant database URL, it returns
// (or lazily creates) the tenant pool. Otherwise returns the primary pool.
func (ds *databaseService) resolvePool(ctx context.Context) (*pgxpool.Pool, error) {
	if dbURL, ok := ctx.Value(TenantDatabaseURLKey).(string); ok && dbURL != "" {
		return ds.GetPoolForTenant(ctx, dbURL)
	}
	return ds.pool, nil
}

func (ds *databaseService) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	pool, err := ds.resolvePool(ctx)
	if err != nil {
		return nil, err
	}
	return pool.Query(ctx, query, args...)
}

func (ds *databaseService) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	pool, err := ds.resolvePool(ctx)
	if err != nil {
		// pgx.Row with error — return a no-op row that surfaces the error on Scan
		return errorRow{err}
	}
	return pool.QueryRow(ctx, query, args...)
}

func (ds *databaseService) Execute(ctx context.Context, query string, args ...interface{}) (int64, error) {
	pool, err := ds.resolvePool(ctx)
	if err != nil {
		return 0, err
	}
	commandTag, err := pool.Exec(ctx, query, args...)
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
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for _, pool := range ds.tenantPools {
		pool.Close()
	}
	ds.tenantPools = make(map[string]*pgxpool.Pool)

	log.Println("Database closed")
}

// RunModularMigrations executes migrations for all enabled modules against the
// given database. It carries no state, so the migrate job (`cli migrate`) runs
// it without opening a pool the way InitDatabase does.
func RunModularMigrations(databaseURL string) error {
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

	// Load core config to get enabled modules. ModularAppConfig itself is nil
	// when the caller never went through config.GetConfig() — the migrate job
	// deliberately does not, so the whole pointer is checked, not just .Core.
	var coreConf *coreConfig.CoreConfig
	if config.ModularAppConfig != nil {
		coreConf = config.ModularAppConfig.Core
	}
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
	pool, err := ds.resolvePool(ctx)
	if err != nil {
		return nil, err
	}
	return pool.Begin(ctx)
}

// errorRow implements pgx.Row and always returns the stored error on Scan.
// Used when resolvePool fails inside QueryRow (which can't return an error).
type errorRow struct{ err error }

func (e errorRow) Scan(...interface{}) error { return e.err }

// GetPoolForTenant returns a connection pool for a tenant database.
// Pools are cached and created lazily. Safe for concurrent use.
func (ds *databaseService) GetPoolForTenant(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	// Fast path: pool already exists
	ds.mu.RLock()
	pool, exists := ds.tenantPools[databaseURL]
	ds.mu.RUnlock()
	if exists {
		return pool, nil
	}

	// Slow path: create and cache the pool
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Double-check after acquiring write lock
	if pool, exists = ds.tenantPools[databaseURL]; exists {
		return pool, nil
	}

	// Get tenant pool size from config (TENANCY_DATABASE_POOL_SIZE, default 5)
	tenancyConf := config.ModularAppConfig.Tenancy
	poolSize := int32(5)
	if tenancyConf != nil {
		poolSize = tenancyConf.TenantDatabasePoolSize
	}

	pool, err := connectDatabase(ctx, databaseURL, poolSize, "josex_tenant")
	if err != nil {
		return nil, err
	}

	ds.tenantPools[databaseURL] = pool
	log.Printf("Created tenant database pool (pool size: %d)", poolSize)

	return pool, nil
}

// GetPrimaryPool returns the primary database pool
func (ds *databaseService) GetPrimaryPool() *pgxpool.Pool {
	return ds.pool
}
