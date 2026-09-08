package models

import "time"

// LEARNING NOTE: Go Structs, Exported Fields, and Struct Tags
// -------------------------------------------------------------
// 1. Data modeling in Go is done using `type Name struct`.
// 2. Field Visibility: In Go, capitalization dictates access.
//    - Capitalized fields (ID, Email) are EXPORTED (accessible outside the package and by JSON encoders).
//    - Lowercase fields are UNEXPORTED (private to the package).
// 3. Struct Tags: The backtick metadata `json:"id"` instructs Go's encoding/json package
//    how to map JSON object keys to struct fields during serialization/deserialization.
// 4. `json:"-"`: Instructs the JSON encoder to completely ignore this field.
//    This ensures password hashes are never exposed in API responses.

// User represents an account in our system.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// LEARNING NOTE: Type-Safe Enums with Custom Types
// Go does not have an `enum` keyword like TypeScript or Java.
// The idiomatic pattern is to define a custom type (`type TicketStatus string`)
// and define `const` values for the valid variants.
// This guarantees compiler type safety across handlers and database functions.

type TicketStatus string

const (
	StatusOpen       TicketStatus = "open"
	StatusInProgress TicketStatus = "in_progress"
	StatusClosed     TicketStatus = "closed"
)

// Ticket represents a service ticket owned by a specific user.
type Ticket struct {
	ID          int64        `json:"id"`
	UserID      int64        `json:"user_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TicketStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// -------------------------------------------------------------
// DTOs (Data Transfer Objects) for HTTP Payloads
// -------------------------------------------------------------

// RegisterRequest holds registration inputs.
// Accepts either "email" or "username" to maximize client and test-suite compatibility.
type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest holds login credentials.
type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateTicketRequest holds payload for new ticket creation.
type CreateTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// UpdateStatusRequest holds payload to transition a ticket's status.
type UpdateStatusRequest struct {
	Status TicketStatus `json:"status"`
}

// AuthResponse returns the issued JWT token and basic user info.
type AuthResponse struct {
	Token   string `json:"token"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
}

// HealthResponse represents the required format for GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}

// ErrorResponse standardizes error responses across all endpoints: {"error": "..."}.
type ErrorResponse struct {
	Error string `json:"error"`
}
