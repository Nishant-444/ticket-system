package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"ticket-system/internal/auth"
	"ticket-system/internal/models"
)

// LEARNING NOTE: Context Keys in Go
// -------------------------------------------------------------
// In languages like JavaScript, you can attach arbitrary properties to `req.user`.
// In Go, state is passed down the request pipeline via `r.Context()`.
// To avoid key collisions between different third-party packages, Go strongly advises
// against using raw strings as context keys.
// Instead, we define an unexported (lowercase) custom type: `type contextKey string`.
// Because it is unexported, no other package can accidentally overwrite our keys!

type contextKey string

const (
	userIDKey    contextKey = "userID"
	userEmailKey contextKey = "userEmail"
)

// LEARNING NOTE: The Go Middleware Pattern
// -------------------------------------------------------------
// In Express (Node.js), middleware is `(req, res, next) => next()`.
// In Go, middleware is a higher-order function:
//
//     func(next http.Handler) http.Handler
//
// It takes the next handler in the chain, wraps it with pre/post processing logic,
// and returns a new http.Handler.

// AuthMiddleware intercepts requests, validates the Bearer JWT, and injects user identity into Context.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// http.HandlerFunc is an adapter that lets ordinary functions act as http.Handler
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// STEP 1: Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				sendJSONError(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// STEP 2: Verify "Bearer <token>" format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				sendJSONError(w, "invalid authorization header format, expected: Bearer <token>", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// STEP 3: Cryptographically verify token signature and expiration
			claims, err := auth.ValidateToken(tokenString, secret)
			if err != nil {
				sendJSONError(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// STEP 4: Inject claims into Request Context
			// `context.WithValue` creates a child context containing the new key-value pair.
			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)

			// STEP 5: Forward request to the next handler with enriched context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LEARNING NOTE: Type Assertions in Go
// Context values are stored as `any` (interface{}).
// To use them as concrete types like `int64`, we use a "comma-ok" type assertion:
// `id, ok := val.(int64)`
// If the type matches, `ok` is true; otherwise it safely returns false without panicking.

// GetUserID retrieves the authenticated user's ID from context.
func GetUserID(ctx context.Context) (int64, bool) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return 0, false
	}
	id, ok := val.(int64)
	return id, ok
}

// GetUserEmail retrieves the authenticated user's email from context.
func GetUserEmail(ctx context.Context) (string, bool) {
	val := ctx.Value(userEmailKey)
	if val == nil {
		return "", false
	}
	email, ok := val.(string)
	return email, ok
}

// CORSMiddleware enables cross-origin requests for web client access.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle HTTP OPTIONS preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sendJSONError writes a standard ErrorResponse JSON payload.
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
	})
}
