package services

import (
	"josex/web/config"
	"log"
	"os"
	"os/exec"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDatabase() {
	retryInterval := 5 * time.Second

	var err error
	for {
		DB, err = gorm.Open(postgres.Open(config.DatabaseUrl), &gorm.Config{})
		if err == nil {
			if err := runMigrations(config.DatabaseUrl); err != nil {
				log.Println("End running migrations...")
			}
			log.Println("running migrations completed")
			break
		}

		time.Sleep(retryInterval)
	}

	// Realizar migraciones aquí si las estás utilizando
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
	cmd := exec.Command("migrate", "-path", "migrations", "-database", databaseURL, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
