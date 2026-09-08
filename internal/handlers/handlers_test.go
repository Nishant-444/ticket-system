package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"ticket-system/internal/database"
	"ticket-system/internal/handlers"
	"ticket-system/internal/middleware"
	"ticket-system/internal/models"
)

const testSecret = "test-jwt-secret-key-12345"

func setupTestServer(t *testing.T) http.Handler {
	tempDB := t.TempDir() + "/test_tickets.db"
	db, err := database.InitDB(tempDB)
	if err != nil {
		t.Fatalf("failed to init test database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.Remove(tempDB)
	})

	handler := handlers.NewHandler(db, testSecret)
	authMiddleware := middleware.AuthMiddleware(testSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /auth/register", handler.Register)
	mux.HandleFunc("POST /auth/login", handler.Login)
	mux.Handle("POST /tickets", authMiddleware(http.HandlerFunc(handler.CreateTicket)))
	mux.Handle("GET /tickets", authMiddleware(http.HandlerFunc(handler.ListTickets)))
	mux.Handle("GET /tickets/{id}", authMiddleware(http.HandlerFunc(handler.GetTicket)))
	mux.Handle("PATCH /tickets/{id}/status", authMiddleware(http.HandlerFunc(handler.UpdateTicketStatus)))

	return mux
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var res models.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", res.Status)
	}
}

func TestAuthAndTicketLifecycle(t *testing.T) {
	router := setupTestServer(t)

	// Register User A
	userABody, _ := json.Marshal(models.RegisterRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(userABody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("user A registration failed: %d body: %s", rec.Code, rec.Body.String())
	}

	var authA models.AuthResponse
	_ = json.NewDecoder(rec.Body).Decode(&authA)
	if authA.Token == "" {
		t.Fatalf("expected JWT token, got empty string")
	}

	// Register User B
	userBBody, _ := json.Marshal(models.RegisterRequest{
		Email:    "bob@example.com",
		Password: "password123",
	})
	req = httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(userBBody))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("user B registration failed: %d", rec.Code)
	}
	var authB models.AuthResponse
	_ = json.NewDecoder(rec.Body).Decode(&authB)

	// Login User A
	loginABody, _ := json.Marshal(models.LoginRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	req = httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginABody))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("user A login failed: %d", rec.Code)
	}

	// User A creates ticket
	ticketBody, _ := json.Marshal(models.CreateTicketRequest{
		Title:       "Database latency spike",
		Description: "Queries taking > 200ms in production",
	})
	req = httptest.NewRequest(http.MethodPost, "/tickets", bytes.NewReader(ticketBody))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ticket creation failed: %d, body: %s", rec.Code, rec.Body.String())
	}

	var ticket1 models.Ticket
	_ = json.NewDecoder(rec.Body).Decode(&ticket1)
	if ticket1.Status != models.StatusOpen {
		t.Errorf("expected initial status 'open', got '%s'", ticket1.Status)
	}

	// User B lists tickets -> must return 0 tickets (ownership isolation)
	req = httptest.NewRequest(http.MethodGet, "/tickets", nil)
	req.Header.Set("Authorization", "Bearer "+authB.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var bobTickets []models.Ticket
	_ = json.NewDecoder(rec.Body).Decode(&bobTickets)
	if len(bobTickets) != 0 {
		t.Errorf("expected 0 tickets for Bob, got %d", len(bobTickets))
	}

	// User B attempts to access Alice's ticket -> 403 Forbidden
	req = httptest.NewRequest(http.MethodGet, "/tickets/1", nil)
	req.Header.Set("Authorization", "Bearer "+authB.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden when accessing another user's ticket, got %d", rec.Code)
	}

	// User B attempts to update Alice's ticket -> 403 Forbidden
	statusUpdate, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusInProgress})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(statusUpdate))
	req.Header.Set("Authorization", "Bearer "+authB.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden when updating another user's ticket, got %d", rec.Code)
	}

	// User A updates status: open -> in_progress (valid)
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(statusUpdate))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	var updated models.Ticket
	_ = json.NewDecoder(rec.Body).Decode(&updated)
	if updated.Status != models.StatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", updated.Status)
	}

	// User A attempts invalid transition: in_progress -> open (rejected)
	invalidMove, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusOpen})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(invalidMove))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for backward transition in_progress -> open, got %d", rec.Code)
	}

	// User A updates status: in_progress -> closed (valid)
	closeUpdate, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusClosed})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(closeUpdate))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// User A attempts to reopen closed ticket (rejected)
	reopenMove, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusInProgress})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(reopenMove))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when attempting to reopen closed ticket, got %d", rec.Code)
	}
}
