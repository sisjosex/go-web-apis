package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppMode              string
	AppHost              string
	AppPort              string
	DatabaseUrl          string
	DatabasePoolSize     int32
	JwtSecretKey         string
	JwtExpirationSeconds int32
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
		AppMode:              getEnv("APP_MODE", "debug"),
		AppHost:              getEnv("APP_HOST", "127.0.0.1"),
		AppPort:              getEnv("APP_PORT", "8080"),
		DatabaseUrl:          getEnv("DATABASE_URL", ""),
		DatabasePoolSize:     getEnvAsInt("DATABASE_POOL_SIZE", 10),
		JwtSecretKey:         getEnv("JWT_SECRET_KEY", "debug"),
		JwtExpirationSeconds: getEnvAsInt("JWT_EXPIRATION_SECONDS", 300),
	}
}

// getEnv get an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt get an environment variable as int32 or a default value
func getEnvAsInt(key string, defaultValue int32) int32 {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return int32(intValue)
		}
		log.Printf("Warning: Could not convert %s to int32, using default: %d", key, defaultValue)
	}
	return defaultValue
}
