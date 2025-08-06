package jwt

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid = errors.New("invalid token")
	ErrTokenExpired = errors.New("token has expired")
	ErrUserClaim    = errors.New("user claim missing or invalid")
)

// GenerateJWT generates a JWT with "user" claim and 1 year expiry
func GenerateJWT(userID string) (string, error) {
	secret := []byte(os.Getenv("JWT_SECRET"))

	claims := jwt.MapClaims{
		"user": userID,
		"exp":  time.Now().Add(365 * 24 * time.Hour).Unix(),
	}

	fmt.Printf("Parsed claims: %+v\n", claims)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

// ParseJWT validates and parses the JWT, returning the userID
func ParseJWT(tokenStr string) (string, error) {
	secret := []byte(os.Getenv("JWT_SECRET"))

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil || !token.Valid {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", err
	}
	log.Println("Parsed claims:", claims)


	userValue, exists := claims["user"]
	if !exists || userValue == nil {
		return "", fmt.Errorf("user claim is missing or nil")
	}

	userID, ok := userValue.(string)
	if !ok {
		return "", fmt.Errorf("user claim is not a string")
	}

	return userID, nil
}


