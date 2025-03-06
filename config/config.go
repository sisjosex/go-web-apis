package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {

	// Web server config
	AppMode string
	AppHost string
	AppPort string

	// Database
	DatabaseUrl      string
	DatabasePoolSize int32

	// JWT config
	JwtSecretKey                string
	JwtRefreshKey               string
	JwtExpirationSeconds        int32
	JwtRefreshExpirationSeconds int32

	// SMTP config
	SmtpHost string
	SmtpPort int
	SmtpUser string
	SmtpPass string
	SmtpFrom string

	// Frontend
	FrontendUrl string
}

var AppConfig *Config

func init() {
	loadConfig()
}

func loadConfig() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Asignar valores con fallback si no están definidos
	AppConfig = &Config{
		AppMode: getEnv("APP_MODE", "debug"),
		AppHost: getEnv("APP_HOST", "127.0.0.1"),
		AppPort: getEnv("APP_PORT", "8080"),

		DatabaseUrl:      getEnv("DATABASE_URL", ""),
		DatabasePoolSize: getEnvAsInt32("DATABASE_POOL_SIZE", 10),

		JwtSecretKey:                getEnv("JWT_SECRET_KEY", ""),
		JwtRefreshKey:               getEnv("JWT_REFRESH_KEY", ""),
		JwtExpirationSeconds:        getEnvAsInt32("JWT_EXPIRATION_SECONDS", 300), // 5 minutes by default
		JwtRefreshExpirationSeconds: getEnvAsInt32("JWT_REFRESH_SECONDS", 10080),  // 7 days by default

		/* SMTP config */
		SmtpHost: getEnv("SMTP_HOST", "mail.smtp2go.com"),
		SmtpPort: getEnvAsInt("SMTP_PORT", 2525),
		SmtpUser: getEnv("SMTP_USER", ""),
		SmtpPass: getEnv("SMTP_PASS", ""),
		SmtpFrom: getEnv("SMTP_FROM", ""),

		FrontendUrl: getEnv("FRONTEND_URL", ""),
	}
}

// getEnv get an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return int(intValue)
		}
		log.Printf("Warning: Could not convert %s to int32, using default: %d", key, defaultValue)
	}
	return defaultValue
}

func getEnvAsInt32(key string, defaultValue int32) int32 {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return int32(intValue)
		}
		log.Printf("Warning: Could not convert %s to int32, using default: %d", key, defaultValue)
	}
	return defaultValue
}
