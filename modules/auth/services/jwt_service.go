package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTService interface {
	GenerateAccessToken(userID uuid.UUID, sessionID uuid.UUID) (*string, error)
	GenerateRefreshToken(userID uuid.UUID, sessionID uuid.UUID) (*string, error)
	ValidateToken(token string) (jwt.MapClaims, error)
	RefreshAccessToken(refreshToken string) (*string, error)
}

type jwtService struct {
	accessSecret  string
	refreshSecret string
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewJWTService(accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) JWTService {
	return &jwtService{
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

// 📌 Genera un Access Token con duración corta (15-60 min)
func (j *jwtService) GenerateAccessToken(userID uuid.UUID, sessionID uuid.UUID) (*string, error) {
	claims := jwt.MapClaims{
		"user_id":    userID,
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Second * time.Duration(j.accessTTL)).Unix(),
		"iat":        time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(j.accessSecret))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &tokenString, nil
}

// 📌 Genera un Refresh Token con duración más larga (7-30 días)
func (j *jwtService) GenerateRefreshToken(userID uuid.UUID, sessionID uuid.UUID) (*string, error) {
	claims := jwt.MapClaims{
		"user_id":    userID,
		"session_id": sessionID,
		"exp":        time.Now().Add(time.Second * time.Duration(j.refreshTTL)).Unix(),
		"iat":        time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(j.refreshSecret))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &tokenString, nil
}

// 📌 Valida un Access Token y devuelve los claims si es válido
func (j *jwtService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verificar que el método de firma sea HS256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Devolver la clave secreta para validar la firma
		return []byte(j.accessSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Validar manualmente el tiempo de emisión y expiración con un margen de tolerancia
		if err := validateTokenClaims(claims); err != nil {
			return nil, err
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

func (j *jwtService) ValidateRefreshToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verificar que el método de firma sea HS256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.refreshSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("token parse error: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Validar manualmente el tiempo de emisión y expiración con un margen de tolerancia
		if err := validateTokenClaims(claims); err != nil {
			return nil, err
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// 📌 Renueva un Access Token si el Refresh Token es válido
func (j *jwtService) RefreshAccessToken(refreshToken string) (*string, error) {

	claims, err := j.ValidateRefreshToken(refreshToken)

	if err != nil {
		return nil, err
	}

	userID, userOk := uuid.Parse(claims["user_id"].(string))
	sessionID, sessionOk := uuid.Parse(claims["session_id"].(string))

	if userOk != nil || sessionOk != nil {
		return nil, errors.New("invalid token data")
	}

	// Generar un nuevo Access Token
	return j.GenerateAccessToken(userID, sessionID)
}

func validateTokenClaims(claims jwt.MapClaims) error {
	now := time.Now().Unix()

	// Verificar el tiempo de emisión (iat) con un margen de 60 segundos
	if iat, ok := claims["iat"].(float64); ok {
		if now < int64(iat)-60 { // No es necesario usar iatTime.Unix()
			return fmt.Errorf("token used before issued (iat)")
		}
	}

	// Verificar la expiración (exp)
	if exp, ok := claims["exp"].(float64); ok {
		if now > int64(exp) { // No es necesario usar expTime.Unix()
			return fmt.Errorf("token has expired")
		}
	}

	return nil
}
