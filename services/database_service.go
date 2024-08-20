package services

import (
	"josex/web/config"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDatabase() {
	retryInterval := 5 * time.Second

	var err error
	for {
		DB, err = gorm.Open(postgres.Open(config.DatabaseUrl), &gorm.Config{})
		sqlDB, _ := DB.DB()
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
		if err == nil {
			if err := runMigrations(config.DatabaseUrl); err != nil {
				log.Println("Error running migrations:", err)
			} else {
				log.Println("Running migrations completed successfully")
			}

			break

		} else {
			log.Println("Error connecting to database:", err)
		}

		time.Sleep(retryInterval)
	}
}

func CloseDatabase() {
	sqlDB, err := DB.DB()
	if err != nil {
		panic("Error while closing the database")
	}
	sqlDB.Close()
	log.Println("Closing database")
}

func runMigrations(databaseURL string) error {
	log.Println("Running migrations...")

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

	log.Println("Migrations applied successfully.")
	return nil
}
