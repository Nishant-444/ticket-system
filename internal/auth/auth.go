package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// In Go, custom errors are typically declared as package-level variables
// using errors.New(...) so callers can test against them with errors.Is().
var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

// JWTClaims defines what payload data we embed inside the signed JWT.
// RegisteredClaims provides standard claims like ExpiresAt, IssuedAt, etc.
type JWTClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// HashPassword hashes a raw password string using the industry-standard bcrypt algorithm.
// In Go, functions can return multiple values: (result, error).
// It is standard practice to return (string, error).
func HashPassword(password string) (string, error) {
	// bcrypt.DefaultCost is 10, providing a great balance between security and CPU efficiency.
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	// We convert the byte slice []byte back to a human-readable string.
	return string(hashedBytes), nil
}

// CheckPassword compares a plaintext password with a stored bcrypt hash.
// If they match, it returns true; otherwise false.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken creates a signed JWT string containing the user's ID and Email.
func GenerateToken(userID int64, email string, secret string, duration time.Duration) (string, error) {
	// 1. Create the claims object with an expiration timestamp.
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 2. Specify the signing algorithm (HMAC-SHA256 is the standard).
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. Sign the token with our secret key (converted to []byte).
	signedString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return signedString, nil
}

// ValidateToken parses and cryptographically verifies an incoming JWT token string.
// If valid, it returns the extracted JWTClaims.
func ValidateToken(tokenString string, secret string) (*JWTClaims, error) {
	// ParseWithClaims parses the string and verifies the signature using the Keyfunc callback.
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC (prevents "none" algorithm exploit)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	// In Go, "type assertion" (claims, ok := ...) checks if an interface holds a specific concrete type.
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
