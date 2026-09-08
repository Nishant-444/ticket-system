package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"ticket-system/internal/auth"
	"ticket-system/internal/models"
)

type contextKey string

const (
	userIDKey    contextKey = "userID"
	userEmailKey contextKey = "userEmail"
)

// AuthMiddleware validates incoming Bearer JWT tokens and injects claims into request context.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				sendJSONError(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				sendJSONError(w, "invalid authorization header format, expected: Bearer <token>", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims, err := auth.ValidateToken(tokenString, secret)
			if err != nil {
				sendJSONError(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userEmailKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the authenticated user ID from context.
func GetUserID(ctx context.Context) (int64, bool) {
	val := ctx.Value(userIDKey)
	if val == nil {
		return 0, false
	}
	id, ok := val.(int64)
	return id, ok
}

// GetUserEmail retrieves the authenticated user email from context.
func GetUserEmail(ctx context.Context) (string, bool) {
	val := ctx.Value(userEmailKey)
	if val == nil {
		return "", false
	}
	email, ok := val.(string)
	return email, ok
}

// CORSMiddleware enables CORS headers for cross-origin browser interactions.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
	})
}
