// Package utils provides environment variable utilities for configuration
package utils

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var envLoaded bool

// LoadEnv loads environment variables from .env file if not already loaded
// Smart detection: tries ENV_FILE env var, then auto-detects platform/tenant, then .env
// This is called automatically by GetEnv functions, but can be called manually
// Safe to call multiple times - only loads once
func LoadEnv() {
	if !envLoaded {
		// 1. Check if ENV_FILE is explicitly set
		envFile := os.Getenv("ENV_FILE")
		
		// 2. Auto-detect from running app (platform or tenant)
		if envFile == "" {
			envFile = detectEnvFile()
		}
		
		// 3. Default fallback
		if envFile == "" {
			envFile = ".env"
		}

		err := godotenv.Load(envFile)
		if err != nil {
			log.Printf("📝 Loading environment from: %s", envFile)
		} else {
			log.Printf("✅ Loaded environment from: %s", envFile)
		}
		envLoaded = true
	}
}

// detectEnvFile auto-detects which .env file to load based on the running app
// Returns ".env.platform", ".env.tenant", or "" (fallback to .env)
func detectEnvFile() string {
	// Get the executable name or process name
	// When using go run, this will be something like "platform" or "tenant"
	// When using docker/binary, it will be the binary name
	
	// Try from ENV_FILE_AUTO if set
	if autoDetect := os.Getenv("ENV_FILE_AUTO"); autoDetect != "" {
		return autoDetect
	}

	// Check if running as platform or tenant based on command-line args or executable name
	// os.Args[0] contains the program name
	progName := filepath.Base(os.Args[0])
	
	switch {
	case strings.Contains(progName, "platform"):
		return ".env.platform"
	case strings.Contains(progName, "tenant"):
		return ".env.tenant"
	}
	
	// No auto-detection possible, use default
	return ""
}

// GetEnv gets an environment variable or returns a default value
func GetEnv(key, defaultValue string) string {
	LoadEnv()

	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetEnvAsInt gets an environment variable as int or returns a default value
func GetEnvAsInt(key string, defaultValue int) int {
	LoadEnv()

	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		log.Printf("Warning: Could not convert %s='%s' to int, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

// GetEnvAsInt32 gets an environment variable as int32 or returns a default value
func GetEnvAsInt32(key string, defaultValue int32) int32 {
	LoadEnv()

	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return int32(intValue)
		}
		log.Printf("Warning: Could not convert %s='%s' to int32, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

// GetEnvAsBool gets an environment variable as bool or returns a default value
// Accepts: true, false, 1, 0, yes, no, on, off (case-insensitive)
func GetEnvAsBool(key string, defaultValue bool) bool {
	LoadEnv()

	if value, exists := os.LookupEnv(key); exists {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		default:
			log.Printf("Warning: Invalid boolean value for %s='%s', using default: %v", key, value, defaultValue)
		}
	}
	return defaultValue
}

// GetEnvAsDuration gets an environment variable as time.Duration
// Supports formats: "15m", "1h", "24h", "168h", etc.
// If the value is a number without unit, treats it as minutes
func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	LoadEnv()

	if value, exists := os.LookupEnv(key); exists {
		// Try parsing as duration first (e.g., "15m", "1h")
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}

		// If that fails, try parsing as minutes (backward compatibility)
		if minutes, err := strconv.Atoi(value); err == nil {
			return time.Duration(minutes) * time.Minute
		}

		log.Printf("Warning: Could not convert %s='%s' to duration, using default: %v", key, value, defaultValue)
	}
	return defaultValue
}

// GetEnvAsStringSlice gets an environment variable as a slice of strings
// Splits by comma and trims whitespace
// Example: "core,auth,users" -> ["core", "auth", "users"]
func GetEnvAsStringSlice(key string, defaultValue []string) []string {
	LoadEnv()

	if value, exists := os.LookupEnv(key); exists {
		value = strings.TrimSpace(value)
		if value == "" {
			return []string{}
		}

		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return defaultValue
}

// MustGetEnv gets an environment variable or panics if not set
// Use this for required configuration that has no sensible default
func MustGetEnv(key string) string {
	LoadEnv()

	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	log.Panicf("Required environment variable %s is not set", key)
	return ""
}
