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
	"github.com/prometheus/client_golang/prometheus"
)

var (
	totalConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pgx_total_connections",
		Help: "Total number of connections",
	})

	idleConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pgx_idle_connections",
		Help: "Number of idle connections",
	})

	acquiredConns = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "pgx_acquired_connections",
		Help: "Number of acquired connections",
	})
)

type databaseService struct {
	pool *pgxpool.Pool
}

func NewDatabaseService() interfaces.DatabaseService {
	return &databaseService{}
}

func (ds *databaseService) InitDatabase() {
	retryInterval := 5 * time.Second

	dataBaseUrl := config.DatabaseUrl

	for {
		config, err := pgxpool.ParseConfig(dataBaseUrl)
		config.MaxConns = 50

		if err != nil {
			log.Fatalf("Error parsing database URL: %s", dataBaseUrl)
		}

		pool, err := pgxpool.NewWithConfig(context.Background(), config)

		ds.pool = pool

		if err == nil {

			if err := runMigrations(dataBaseUrl); err != nil {
				log.Println("Error running migrations:", err)
			} else {
				log.Println("Running migrations completed successfully")
			}

			return

		} else {

			log.Println("Error connecting to database:", err)
		}

		time.Sleep(retryInterval)
	}
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

func (ds *databaseService) CloseDatabase() {
	ds.pool.Close()
	log.Println("Closing database")
}

func runMigrations(databaseURL string) error {

	// Crear una nueva instancia de migración
	m, err := migrate.New(
		"file://migrations",
		databaseURL)
	if err != nil {
		return err
	}

	// Ejecutar las migraciones
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
