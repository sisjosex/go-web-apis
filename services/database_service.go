package services

import (
	"context"
	"josex/web/config"
	"josex/web/interfaces"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type databaseService struct {
	pool *pgxpool.Pool
}

func NewDatabaseService() interfaces.DatabaseService {
	return &databaseService{}
}

func (ds *databaseService) InitDatabase(ctx context.Context) {
	retryInterval := 5 * time.Second

	dataBaseUrl := config.AppConfig.DatabaseUrl

	for {
		pool, err := connectDatabase(ctx, dataBaseUrl, config.AppConfig.DatabasePoolSize)
		if err != nil {
			log.Printf("Failed to connect to database: %v", err)
		}

		ds.pool = pool

		if err := runMigrations(dataBaseUrl); err == nil {
			log.Println("Running migrations completed successfully")
			return
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
	conn.Release()

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

// Ejecuta migraciones en la base de datos
func runMigrations(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
