package utils

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/golang-jwt/jwt"
)

const (
	letterBytes      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numberBytes      = "0123456789"
	specialCharBytes = "!@#$%^&*(),.?\":{}|<>"
	passwordLength   = 8
)

// Helper function to generate a random character from a string
func randomCharFrom(chars string) byte {
	return chars[rand.Intn(len(chars))]
}

// Function to generate a random password that meets the required criteria
func GenerateRandomPassword() string {
	rand.Seed(time.Now().UnixNano()) // Seed for random generator

	var password []byte

	// Add at least one uppercase letter
	password = append(password, randomCharFrom("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))

	// Add at least one number
	password = append(password, randomCharFrom(numberBytes))

	// Add at least one special character
	password = append(password, randomCharFrom(specialCharBytes))

	// Fill the rest with random characters from all sets
	allChars := letterBytes + numberBytes + specialCharBytes
	for len(password) < passwordLength {
		password = append(password, randomCharFrom(allChars))
	}

	// Shuffle the password to avoid predictable patterns
	rand.Shuffle(len(password), func(i, j int) {
		password[i], password[j] = password[j], password[i]
	})

	return string(password)
}

func GenerateAccessToken(userId string, sessionId string, secret string, expirationDate time.Time) (*string, error) {
	// Create the JWT claims, which includes the userId and sessionId
	claims := jwt.MapClaims{
		"user_id":    userId,
		"session_id": sessionId,
		"exp":        expirationDate, // Token expires in 24 hours
		"iat":        time.Now(),     // Issued at
	}

	// Create the token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &tokenString, nil
}
