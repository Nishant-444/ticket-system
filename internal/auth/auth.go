package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// LEARNING NOTE: Sentinel Errors in Go
// -------------------------------------------------------------
// In Go, package-level errors are declared as variables (prefixed with `Err`).
// This allows calling code to inspect specific failures using `errors.Is(err, auth.ErrInvalidToken)`
// rather than matching on brittle error string messages.

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

// LEARNING NOTE: Struct Embedding (Composition)
// -------------------------------------------------------------
// Go does not have class inheritance. Instead, it uses struct composition.
// By embedding `jwt.RegisteredClaims` inside `JWTClaims`, our struct automatically
// gains standard JWT fields (ExpiresAt, IssuedAt, Issuer) while letting us add custom
// application fields (UserID, Email).

// JWTClaims defines what payload data we embed inside the signed token.
type JWTClaims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// LEARNING NOTE: Multiple Return Values & Error Handling
// Functions in Go frequently return `(T, error)`.
// By convention, if `err != nil`, the result value should be discarded.

// HashPassword hashes a raw password string using the industry-standard bcrypt algorithm.
func HashPassword(password string) (string, error) {
	// bcrypt.DefaultCost is 10, providing an optimal balance between security and CPU efficiency.
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	// Convert byte slice []byte back to a string for database storage
	return string(hashedBytes), nil
}

// CheckPassword compares a plaintext password with a stored bcrypt hash in constant time.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken creates a signed HMAC-SHA256 JWT containing user claims.
func GenerateToken(userID int64, email string, secret string, duration time.Duration) (string, error) {
	// 1. Build claims with expiration and issue timestamps
	claims := JWTClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 2. Specify the signing algorithm (HMAC-SHA256 is the standard for symmetric secrets)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. Sign the token with our secret key
	signedString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return signedString, nil
}

// ValidateToken parses and cryptographically verifies an incoming JWT token string.
func ValidateToken(tokenString string, secret string) (*JWTClaims, error) {
	// ParseWithClaims parses the string and verifies the signature using the Keyfunc callback
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// LEARNING NOTE: Algorithm verification
		// We explicitly check that the algorithm is HMAC.
		// This prevents the notorious "algorithm confusion / none" vulnerability where an attacker
		// sends an unsigned token claiming "alg": "none".
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	// Safe type assertion to verify claims struct
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
