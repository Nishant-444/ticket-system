package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"ticket-system/internal/auth"
	"ticket-system/internal/models"
)

// In Go, context keys should NOT be raw strings to avoid collisions between packages.
// Instead, we define an unexported (lowercase) custom type for context keys.
type contextKey string

const (
	userIDKey    contextKey = "userID"
	userEmailKey contextKey = "userEmail"
)

// AuthMiddleware intercepts incoming HTTP requests to verify the JWT Bearer token.
// In Go, middleware is typically a higher-order function:
// func(http.Handler) http.Handler
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// http.HandlerFunc adapts a normal function into an http.Handler interface
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Read the Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				sendJSONError(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			// 2. Check for the "Bearer " prefix (case-insensitive or exact)
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				sendJSONError(w, "invalid authorization header format, expected: Bearer <token>", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// 3. Cryptographically validate the token
			claims, err := auth.ValidateToken(tokenString, secret)
			if err != nil {
				sendJSONError(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// 4. Inject the authenticated user's info into the request context.
			// In Go, HTTP requests pass state down through context.Context.
			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)

			// 5. Call the next handler in the chain with the enriched context.
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the authenticated user's ID from the request context.
// It returns (userID, true) if found, or (0, false) if not authenticated.
func GetUserID(ctx context.Context) (int64, bool) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return 0, false
	}
	id, ok := val.(int64)
	return id, ok
}

// GetUserEmail retrieves the authenticated user's email from the request context.
func GetUserEmail(ctx context.Context) (string, bool) {
	val := ctx.Value(userEmailKey)
	if val == nil {
		return "", false
	}
	email, ok := val.(string)
	return email, ok
}

// CORSMiddleware handles cross-origin requests for web browser accessibility.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// sendJSONError writes a consistent JSON error response.
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
	})
}
