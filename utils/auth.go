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

func GenerateAccessToken(userId string, sessionId string, secret string, expirationDate time.Time) (*string, error) {

	// Create the JWT claims, which includes the userId and sessionId
	claims := jwt.MapClaims{
		"user_id":    userId,
		"session_id": sessionId,
		"exp":        expirationDate.Unix(), // Token expires in 24 hours
		"iat":        time.Now().Unix(),     // Issued at
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

func ParseAccessToken(tokenString string, secret string) (map[string]interface{}, error) {
	// Parsear el token y validar con la clave secreta
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verificar que el método de firma sea HS256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("Invalid token: %v", err)
	}

	// Extraer claims si el token es válido
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Validar manualmente el tiempo de emisión y expiración con un margen de tolerancia
		if err := validateTokenClaims(claims); err != nil {
			return nil, err
		}
		return claims, nil
	}

	return nil, fmt.Errorf("Invalid token claims")
}

func validateTokenClaims(claims jwt.MapClaims) error {
	now := time.Now().Unix()

	// Verificar el tiempo de emisión (iat) con un margen de 60 segundos
	if iat, ok := claims["iat"].(float64); ok {
		if now < int64(iat)-60 {
			return fmt.Errorf("Token used before issued (iat)")
		}
	}

	// Verificar la expiración (exp)
	if exp, ok := claims["exp"].(float64); ok {
		if now > int64(exp) {
			return fmt.Errorf("Token has expired")
		}
	}

	return nil
}
