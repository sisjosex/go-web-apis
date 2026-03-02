// filepath: cmd/testutil/seed.go
package main

import (
	"database/sql"
	"josex/web/modules/core/services"
)

// seedTestData dynamically loads all test fixtures from all modules
// Scans modules/*/tests/fixtures/*.sql and executes them
func seedTestData(db *sql.DB) error {
	fixtureService := services.NewFixtureService(db)
	return fixtureService.LoadFixtures()
}
