package models

import "time"

// User represents an authenticated user entity.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// TicketStatus represents the lifecycle state of a ticket.
type TicketStatus string

const (
	StatusOpen       TicketStatus = "open"
	StatusInProgress TicketStatus = "in_progress"
	StatusClosed     TicketStatus = "closed"
)

// Ticket represents a service or issue ticket owned by a user.
type Ticket struct {
	ID          int64        `json:"id"`
	UserID      int64        `json:"user_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TicketStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// RegisterRequest defines the input payload for user registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginRequest defines credentials for authentication.
type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateTicketRequest defines the payload required to create a ticket.
type CreateTicketRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// UpdateStatusRequest defines the target status payload.
type UpdateStatusRequest struct {
	Status TicketStatus `json:"status"`
}

// AuthResponse defines the payload returned upon registration or login.
type AuthResponse struct {
	Token   string `json:"token"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
}

// HealthResponse defines the health check status payload.
type HealthResponse struct {
	Status string `json:"status"`
}

// ErrorResponse standardizes error responses across all endpoints.
type ErrorResponse struct {
	Error string `json:"error"`
}
