package config

import (
	"fmt"
	utilities "josex/web/utils"
	"strconv"
)

var (
	ApplicationPort   string
	ApplicationHost   string
	LanguageDirectory string
	LanguageDefault   string
	DatabaseUrl       string
	AppMode           string
	JwtSecret         string
	JwtExpiration     int
)

func init() {
	ApplicationHost = utilities.GetEnv("APP_HOST", "127.0.0.1")
	ApplicationPort = utilities.GetEnv("APP_PORT", "8080")
	LanguageDirectory = utilities.GetEnv("LANGUAGE_DIRECTORY", "lang")
	LanguageDefault = utilities.GetEnv("LANGUAGE_DEFAULT", "en")
	DatabaseUrl = utilities.GetEnv("MIGRATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/web?sslmode=disable")
	AppMode = utilities.GetEnv("GIN_MODE", "release")
	JwtSecret = utilities.GetEnv("JWT_SECRET", "secret")
	expirationConfig, err := strconv.Atoi(utilities.GetEnv("JWT_EXPIRATION", "300"))

	if err != nil {
		fmt.Errorf("Error al convertir JWT_EXPIRATION: %v\n", err)
	} else {
		JwtExpiration = expirationConfig
	}
}
