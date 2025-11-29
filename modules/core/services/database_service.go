package services

import (
	"context"
	"database/sql"
	"josex/web/config"
	coreConfig "josex/web/modules/core/config"
	"log"
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
}

type databaseService struct {
	pool *pgxpool.Pool
}

func NewDatabaseService() DatabaseService {
	return &databaseService{}
}

func (ds *databaseService) InitDatabase(ctx context.Context) {
	retryInterval := 5 * time.Second

	// Use modular config
	coreConf := config.ModularAppConfig.Core
	dataBaseUrl := coreConf.DatabaseURL

	for {
		pool, err := connectDatabase(ctx, dataBaseUrl, coreConf.DatabasePoolSize)
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
		}

		ds.pool = pool

		// Run modular migrations
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

	ds.pool.Close()
	ds.pool = nil
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
