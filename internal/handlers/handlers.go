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

// Handler holds application dependencies for HTTP route handlers.
type Handler struct {
	db        *database.DB
	jwtSecret string
}

// NewHandler creates a new Handler instance with injected dependencies.
func NewHandler(db *database.DB, jwtSecret string) *Handler {
	return &Handler{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, models.HealthResponse{
		Status: "ok",
	})
}

// Register handles POST /auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
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
	if len(password) < 6 {
		respondError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	now := time.Now().UTC()
	result, err := h.db.Exec(
		"INSERT INTO users (email, password_hash, created_at) VALUES (?, ?, ?)",
		email, hashedPassword, now,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			respondError(w, http.StatusConflict, "user with this email already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve user ID")
		return
	}

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

	respondJSON(w, http.StatusCreated, models.AuthResponse{
		Token:   token,
		Message: "user registered successfully",
		User:    user,
	})
}

// Login handles POST /auth/login.
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

	var user models.User
	row := h.db.QueryRow(
		"SELECT id, email, password_hash, created_at FROM users WHERE email = ?",
		email,
	)
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		respondError(w, http.StatusInternalServerError, "database error")
		return
	}

	if !auth.CheckPassword(password, user.PasswordHash) {
		respondError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

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
func (h *Handler) CreateTicket(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) ListTickets(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := h.db.Query(
		"SELECT id, user_id, title, description, status, created_at, updated_at FROM tickets WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch tickets")
		return
	}
	defer rows.Close()

	tickets := make([]models.Ticket, 0)
	for rows.Next() {
		var t models.Ticket
		if err := rows.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to scan ticket data")
			return
		}
		tickets = append(tickets, t)
	}

	if err := rows.Err(); err != nil {
		respondError(w, http.StatusInternalServerError, "error iterating tickets")
		return
	}

	respondJSON(w, http.StatusOK, tickets)
}

// GetTicket handles GET /tickets/{id}.
func (h *Handler) GetTicket(w http.ResponseWriter, r *http.Request) {
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

	if ticket.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied: you do not own this ticket")
		return
	}

	respondJSON(w, http.StatusOK, ticket)
}

// UpdateTicketStatus handles PATCH /tickets/{id}/status.
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

	newStatus := req.Status
	switch newStatus {
	case models.StatusOpen, models.StatusInProgress, models.StatusClosed:
	default:
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid status '%s', allowed: open, in_progress, closed", newStatus))
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
		respondError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Ownership authorization check
	if ticket.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied: you do not own this ticket")
		return
	}

	// State machine flow enforcement
	if ticket.Status == models.StatusClosed && newStatus != models.StatusClosed {
		respondError(w, http.StatusBadRequest, "a closed ticket cannot be reopened")
		return
	}

	if ticket.Status == models.StatusInProgress && newStatus == models.StatusOpen {
		respondError(w, http.StatusBadRequest, "cannot move in_progress ticket back to open")
		return
	}

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

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, statusCode int, message string) {
	respondJSON(w, statusCode, models.ErrorResponse{
		Error: message,
	})
}
