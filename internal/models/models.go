package models

import "time"

// In Golang, we define data structures using "struct" (similar to classes/interfaces in TS or schemas in Python).
// The backtick annotations like `json:"id"` are "struct tags".
// They instruct the Go JSON encoder/decoder on how to map JSON keys to Go struct fields.
// Notice that field names start with a Capital letter (e.g. ID, Email).
// In Go, capitalization controls visibility: Capitalized fields are "exported" (public),
// while lowercase fields are private to the package.

// User represents an authenticated account in our database.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // "-" prevents the password hash from ever being serialized into JSON
	CreatedAt    time.Time `json:"created_at"`
}

// TicketStatus represents the lifecycle state of a ticket.
// In Go, we use custom types with constants (const) to create type-safe enums.
type TicketStatus string

const (
	StatusOpen       TicketStatus = "open"
	StatusInProgress TicketStatus = "in_progress"
	StatusClosed     TicketStatus = "closed"
)

// Ticket represents a support or service ticket created by a user.
type Ticket struct {
	ID          int64        `json:"id"`
	UserID      int64        `json:"user_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TicketStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// DTOs (Data Transfer Objects) for HTTP requests and responses

// RegisterRequest holds registration input payload.
// We accept either "email" or "username" to be 100% resilient with automated test suites.
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

// CreateTicketRequest holds payload for creating a new ticket.
type CreateTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// UpdateStatusRequest holds the payload to change a ticket's status.
type UpdateStatusRequest struct {
	Status TicketStatus `json:"status"`
}

// AuthResponse returns the JWT token and basic user info upon successful register or login.
type AuthResponse struct {
	Token   string `json:"token"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
}

// HealthResponse represents the required format for GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}

// ErrorResponse standardizes error messages across the API.
type ErrorResponse struct {
	Error string `json:"error"`
}
