// utils/env.go

package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var envLoaded bool

// LoadEnv carga las variables de entorno desde el archivo .env si aún no se ha cargado.
func LoadEnv() {
	if !envLoaded {
		err := godotenv.Load()
		if err != nil {
			fmt.Println(".env not found")
		}
		envLoaded = true
	}
}

// GetEnv obtiene el valor de una variable de entorno o un valor predeterminado.
func GetEnv(key, defaultValue string) string {
	LoadEnv() // Cargar el archivo .env si aún no se ha hecho

	value := os.Getenv(key)
	if value == "" {
		value = defaultValue
	}
	return value
}
