package utils

import (
	"math/rand"
	"time"
)

// Configuración de generación de contraseñas aleatorias
const (
	letterBytes      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numberBytes      = "0123456789"
	specialCharBytes = "!@#$%^&*(),.?\":{}|<>"
	passwordLength   = 8
)

// Helper function to generate a random character from a string
func randomCharFrom(chars string, randGen *rand.Rand) byte {
	return chars[randGen.Intn(len(chars))]
}

// Function to generate a random password that meets the required criteria
func GenerateRandomPassword() string {
	randGen := rand.New(rand.NewSource(time.Now().UnixNano()))

	var password []byte

	// Add at least one uppercase letter
	password = append(password, randomCharFrom("ABCDEFGHIJKLMNOPQRSTUVWXYZ", randGen))

	// Add at least one number
	password = append(password, randomCharFrom(numberBytes, randGen))

	// Add at least one special character
	password = append(password, randomCharFrom(specialCharBytes, randGen))

	// Fill the rest with random characters from all sets
	allChars := letterBytes + numberBytes + specialCharBytes
	for len(password) < passwordLength {
		password = append(password, randomCharFrom(allChars, randGen))
	}

	// Shuffle the password to avoid predictable patterns
	rand.Shuffle(len(password), func(i, j int) {
		password[i], password[j] = password[j], password[i]
	})

	return string(password)
}
