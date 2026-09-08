package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ticket-system/internal/auth"
	"ticket-system/internal/database"
	"ticket-system/internal/middleware"
	"ticket-system/internal/models"
)

// LEARNING NOTE: Dependency Injection in Go
// -------------------------------------------------------------
// In object-oriented languages like Java or TypeScript, we often use classes and decorators
// for dependency injection (e.g. @Inject, @Controller).
// In Go, the idiomatic approach is much simpler:
// 1. Define a struct (Handler) that holds references to external dependencies (*database.DB, jwtSecret).
// 2. Attach handler methods to that struct using "pointer receivers" like `(h *Handler) Method(...)`.
// 3. Provide a constructor function (NewHandler) to initialize the struct cleanly.

// Handler bundles dependencies needed by our HTTP endpoints.
type Handler struct {
	db        *database.DB
	jwtSecret string
}

// NewHandler initializes a new Handler instance with required dependencies.
func NewHandler(db *database.DB, jwtSecret string) *Handler {
	return &Handler{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

// Health handles GET /health.
// Required contract: returns HTTP 200 with {"status": "ok"}.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	// Learner's takeaway: Standardizing health check payloads makes microservice monitoring straightforward.
	respondJSON(w, http.StatusOK, models.HealthResponse{
		Status: "ok",
	})
}

// Register handles POST /auth/register.
// Decodes credentials, hashes password, saves user to DB, and returns JWT.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest

	// LEARNING NOTE: JSON Decoding in Go
	// Instead of loading the entire request body string into memory with ioutil.ReadAll,
	// Go provides json.NewDecoder(r.Body).Decode(&req).
	// This streams and deserializes JSON directly into our struct, which is memory-efficient.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Support both "email" and "username" fields for maximum client compatibility
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = strings.TrimSpace(req.Username)
	}
	password := strings.TrimSpace(req.Password)

	// Validate required fields and minimum password security
	if email == "" || password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if len(password) < 6 {
		respondError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	// STEP 1: Hash the plain-text password using bcrypt (cost 10)
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// STEP 2: Persist the user into SQLite
	// LEARNING NOTE: Parameterized queries ("?") protect against SQL injection vulnerabilities.
	now := time.Now().UTC()
	result, err := h.db.Exec(
		"INSERT INTO users (email, password_hash, created_at) VALUES (?, ?, ?)",
		email, hashedPassword, now,
	)
	if err != nil {
		// Detect SQLite UNIQUE constraint violation if the email is already registered
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			respondError(w, http.StatusConflict, "user with this email already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// In Go, LastInsertId() retrieves the autoincremented primary key of the new row.
	userID, err := result.LastInsertId()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve user ID")
		return
	}

	// STEP 3: Issue a JWT token immediately so the user can begin authenticating
	token, err := auth.GenerateToken(userID, email, h.jwtSecret, 24*time.Hour)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate authentication token")
		return
	}

	user := &models.User{
		ID:        userID,
		Email:     email,
		CreatedAt: now,
	}

	// Respond with HTTP 201 Created and the new session token
	respondJSON(w, http.StatusCreated, models.AuthResponse{
		Token:   token,
		Message: "user registered successfully",
		User:    user,
	})
}

// Login handles POST /auth/login.
// Verifies credentials against the bcrypt hash and returns a signed JWT token.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = strings.TrimSpace(req.Username)
	}
	password := strings.TrimSpace(req.Password)

	if email == "" || password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// STEP 1: Query the user record from SQLite by email
	// QueryRow is used when expecting at most one row.
	var user models.User
	row := h.db.QueryRow(
		"SELECT id, email, password_hash, created_at FROM users WHERE email = ?",
		email,
	)

	// LEARNING NOTE: row.Scan reads columns in the exact order requested in SELECT.
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Security best practice: Return generic "invalid email or password" to prevent user enumeration
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		respondError(w, http.StatusInternalServerError, "database error")
		return
	}

	// STEP 2: Compare candidate password with stored bcrypt hash
	if !auth.CheckPassword(password, user.PasswordHash) {
		respondError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// STEP 3: Issue signed JWT token with 24-hour expiration
	token, err := auth.GenerateToken(user.ID, user.Email, h.jwtSecret, 24*time.Hour)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, models.AuthResponse{
		Token:   token,
		Message: "login successful",
		User:    &user,
	})
}

// CreateTicket handles POST /tickets.
// Creates a new ticket owned by the authenticated user with initial status "open".
func (h *Handler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	// Retrieve authenticated user ID attached to the request context by AuthMiddleware
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.CreateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	now := time.Now().UTC()
	// Contract requirement: All newly created tickets must begin with status "open"
	status := models.StatusOpen

	result, err := h.db.Exec(
		"INSERT INTO tickets (user_id, title, description, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		userID, title, req.Description, status, now, now,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}

	ticketID, err := result.LastInsertId()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve ticket id")
		return
	}

	ticket := models.Ticket{
		ID:          ticketID,
		UserID:      userID,
		Title:       title,
		Description: req.Description,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	respondJSON(w, http.StatusCreated, ticket)
}

// ListTickets handles GET /tickets.
// Returns only the tickets created by the authenticated user (Ownership Isolation).
func (h *Handler) ListTickets(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Query strictly by user_id so users can never see another user's tickets
	rows, err := h.db.Query(
		"SELECT id, user_id, title, description, status, created_at, updated_at FROM tickets WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch tickets")
		return
	}
	// LEARNING NOTE: Always `defer rows.Close()` to release database connection back to the pool.
	defer rows.Close()

	// LEARNING NOTE: Slices in Go
	// We use `make([]models.Ticket, 0)` so an empty list serializes as `[]` in JSON instead of `null`.
	tickets := make([]models.Ticket, 0)
	for rows.Next() {
		var t models.Ticket
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to scan ticket data")
			return
		}
		tickets = append(tickets, t)
	}

	// Check if any error occurred during iteration
	if err := rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, "error iterating tickets")
		return
	}

	respondJSON(w, http.StatusOK, tickets)
}

// GetTicket handles GET /tickets/{id}.
// Returns the ticket only if it belongs to the authenticated user.
func (h *Handler) GetTicket(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// LEARNING NOTE: Go 1.22 Path Parameters
	// In Go 1.22+, `r.PathValue("id")` extracts the URL path variable declared in the route pattern.
	idStr := r.PathValue("id")
	ticketID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	var ticket models.Ticket
	row := h.db.QueryRow(
		"SELECT id, user_id, title, description, status, created_at, updated_at FROM tickets WHERE id = ?",
		ticketID,
	)
	err = row.Scan(&ticket.ID, &ticket.UserID, &ticket.Title, &ticket.Description, &ticket.Status, &ticket.CreatedAt, &ticket.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "ticket not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to fetch ticket")
		return
	}

	// OWNERSHIP AUTHORIZATION CHECK:
	// If the ticket exists but belongs to a different user, reject with 403 Forbidden.
	if ticket.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied: you do not own this ticket")
		return
	}

	respondJSON(w, http.StatusOK, ticket)
}

// UpdateTicketStatus handles PATCH /tickets/{id}/status.
// Enforces the contract lifecycle rules:
// - open -> in_progress -> closed
// - closed -> cannot move back to open or in_progress
func (h *Handler) UpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := r.PathValue("id")
	ticketID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ticket id")
		return
	}

	var req models.UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate target status value against allowed enum set
	newStatus := req.Status
	switch newStatus {
	case models.StatusOpen, models.StatusInProgress, models.StatusClosed:
		// Valid status
	default:
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid status '%s', allowed: open, in_progress, closed", newStatus))
		return
	}

	// STEP 1: Fetch current ticket from DB to check ownership and state
	var ticket models.Ticket
	row := h.db.QueryRow(
		"SELECT id, user_id, title, description, status, created_at, updated_at FROM tickets WHERE id = ?",
		ticketID,
	)
	err = row.Scan(&ticket.ID, &ticket.UserID, &ticket.Title, &ticket.Description, &ticket.Status, &ticket.CreatedAt, &ticket.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "ticket not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "database error")
		return
	}

	// STEP 2: Verify ownership
	if ticket.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied: you do not own this ticket")
		return
	}

	// STEP 3: State Machine Validation:
	// Rule A: A closed ticket cannot be reopened or changed back.
	if ticket.Status == models.StatusClosed && newStatus != models.StatusClosed {
		respondError(w, http.StatusBadRequest, "a closed ticket cannot be reopened")
		return
	}

	// Rule B: Enforce forward progression: cannot move in_progress back to open.
	if ticket.Status == models.StatusInProgress && newStatus == models.StatusOpen {
		respondError(w, http.StatusBadRequest, "cannot move in_progress ticket back to open")
		return
	}

	// STEP 4: Update the ticket status in SQLite
	now := time.Now().UTC()
	_, err = h.db.Exec(
		"UPDATE tickets SET status = ?, updated_at = ? WHERE id = ? AND user_id = ?",
		newStatus, now, ticketID, userID,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update ticket status")
		return
	}

	ticket.Status = newStatus
	ticket.UpdatedAt = now

	respondJSON(w, http.StatusOK, ticket)
}

// -------------------------------------------------------------
// HELPER FUNCTIONS: Consistent JSON Response Formatting
// -------------------------------------------------------------

// respondJSON sets application/json header, writes status code, and serializes payload.
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// respondError formats errors into the standard JSON schema: {"error": "..."}.
func respondError(w http.ResponseWriter, statusCode int, message string) {
	respondJSON(w, statusCode, models.ErrorResponse{
		Error: message,
	})
}
