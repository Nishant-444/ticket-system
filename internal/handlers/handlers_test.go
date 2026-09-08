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

// LEARNING NOTE: Go Testing Architecture
// -------------------------------------------------------------
// In Go, automated testing is built directly into the standard toolchain:
// 1. Any file named `<name>_test.go` is only compiled during `go test`.
// 2. Test functions must start with `Test` and accept `t *testing.T`.
// 3. Instead of external test runner frameworks, Go provides `net/http/httptest`.
//    This lets us simulate real HTTP requests against our handlers in memory,
//    giving us fast, deterministic integration tests without opening real TCP ports.

const testSecret = "test-jwt-secret-key-12345"

// setupTestServer builds an isolated in-memory test environment.
// Learner's takeaway: Each test should run with its own clean database to avoid flaky tests.
func setupTestServer(t *testing.T) http.Handler {
	// t.TempDir() creates a temporary folder that Go automatically deletes when the test finishes.
	tempDB := t.TempDir() + "/test_tickets.db"

	// Initialize our SQLite schema inside the temporary directory.
	db, err := database.InitDB(tempDB)
	if err != nil {
		t.Fatalf("failed to init test database: %v", err)
	}

	// t.Cleanup registers a teardown function, similar to afterEach() in Jest or tearDown() in unittest.
	t.Cleanup(func() {
		db.Close()
		os.Remove(tempDB)
	})

	// Inject the test database and secret into our handler instance.
	handler := handlers.NewHandler(db, testSecret)
	authMiddleware := middleware.AuthMiddleware(testSecret)

	// Wire up the same routes as our production server in main.go.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /auth/register", handler.Register)
	mux.HandleFunc("POST /auth/login", handler.Login)

	// Wrap protected endpoints with our JWT middleware.
	mux.Handle("POST /tickets", authMiddleware(http.HandlerFunc(handler.CreateTicket)))
	mux.Handle("GET /tickets", authMiddleware(http.HandlerFunc(handler.ListTickets)))
	mux.Handle("GET /tickets/{id}", authMiddleware(http.HandlerFunc(handler.GetTicket)))
	mux.Handle("PATCH /tickets/{id}/status", authMiddleware(http.HandlerFunc(handler.UpdateTicketStatus)))

	return mux
}

// TestHealthEndpoint verifies the public health contract:
// 1. Endpoint responds with HTTP 200.
// 2. Response payload matches {"status": "ok"}.
func TestHealthEndpoint(t *testing.T) {
	router := setupTestServer(t)

	// In Go, httptest.NewRequest creates a mock *http.Request.
	// httptest.NewRecorder creates a mock http.ResponseWriter that captures headers and body.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	// Dispatch request through our router
	router.ServeHTTP(rec, req)

	// Assert HTTP Status Code
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Parse and assert response JSON payload
	var res models.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if res.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", res.Status)
	}
}

// TestAuthAndTicketLifecycle walks through the complete end-to-end user story:
// - User registration and login
// - JWT token generation
// - Ticket creation with default 'open' status
// - Ownership isolation (User B cannot see or alter User A's tickets)
// - Ticket status progression (open -> in_progress -> closed)
// - Reopening guard (closed tickets cannot be reopened)
func TestAuthAndTicketLifecycle(t *testing.T) {
	router := setupTestServer(t)

	// -------------------------------------------------------------
	// STEP 1: Register User A (Alice)
	// -------------------------------------------------------------
	// In Go, json.Marshal serializes our request struct into a []byte buffer.
	userABody, _ := json.Marshal(models.RegisterRequest{
		Email:    "alice@example.com",
		Password: "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(userABody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify that registration yields HTTP 201 Created
	if rec.Code != http.StatusCreated {
		t.Fatalf("user A registration failed: %d body: %s", rec.Code, rec.Body.String())
	}

	// Extract Alice's JWT token from response
	var authA models.AuthResponse
	_ = json.NewDecoder(rec.Body).Decode(&authA)
	if authA.Token == "" {
		t.Fatalf("expected JWT token in registration response, got empty string")
	}

	// -------------------------------------------------------------
	// STEP 2: Register User B (Bob) for ownership checks
	// -------------------------------------------------------------
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

	// -------------------------------------------------------------
	// STEP 3: Verify User A Login
	// -------------------------------------------------------------
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

	// -------------------------------------------------------------
	// STEP 4: Alice creates a Ticket
	// -------------------------------------------------------------
	ticketBody, _ := json.Marshal(models.CreateTicketRequest{
		Title:       "Database latency spike",
		Description: "Queries taking > 200ms in production",
	})
	req = httptest.NewRequest(http.MethodPost, "/tickets", bytes.NewReader(ticketBody))
	// Attach Alice's bearer token to the Authorization header
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ticket creation failed: %d, body: %s", rec.Code, rec.Body.String())
	}

	var ticket1 models.Ticket
	_ = json.NewDecoder(rec.Body).Decode(&ticket1)

	// Verify contract requirement: Initial status must always be "open"
	if ticket1.Status != models.StatusOpen {
		t.Errorf("expected initial status 'open', got '%s'", ticket1.Status)
	}

	// -------------------------------------------------------------
	// STEP 5: Ownership Isolation - Bob lists his tickets
	// -------------------------------------------------------------
	// Learner's check: Bob should only see his own tickets, which is currently 0.
	req = httptest.NewRequest(http.MethodGet, "/tickets", nil)
	req.Header.Set("Authorization", "Bearer "+authB.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var bobTickets []models.Ticket
	_ = json.NewDecoder(rec.Body).Decode(&bobTickets)
	if len(bobTickets) != 0 {
		t.Errorf("ownership breach: expected 0 tickets for Bob, got %d", len(bobTickets))
	}

	// -------------------------------------------------------------
	// STEP 6: Ownership Check - Bob tries to fetch Alice's ticket
	// -------------------------------------------------------------
	// Learner's check: Must return 403 Forbidden because Bob does not own ticket #1.
	req = httptest.NewRequest(http.MethodGet, "/tickets/1", nil)
	req.Header.Set("Authorization", "Bearer "+authB.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden when accessing another user's ticket, got %d", rec.Code)
	}

	// -------------------------------------------------------------
	// STEP 7: Ownership Check - Bob tries to modify Alice's ticket
	// -------------------------------------------------------------
	// Learner's check: Must return 403 Forbidden to prevent unauthorized state updates.
	statusUpdate, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusInProgress})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(statusUpdate))
	req.Header.Set("Authorization", "Bearer "+authB.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden when updating another user's ticket, got %d", rec.Code)
	}

	// -------------------------------------------------------------
	// STEP 8: Valid Transition: open -> in_progress (by owner Alice)
	// -------------------------------------------------------------
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(statusUpdate))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for valid transition, got %d, body: %s", rec.Code, rec.Body.String())
	}
	var updated models.Ticket
	_ = json.NewDecoder(rec.Body).Decode(&updated)
	if updated.Status != models.StatusInProgress {
		t.Errorf("expected updated status 'in_progress', got '%s'", updated.Status)
	}

	// -------------------------------------------------------------
	// STEP 9: Invalid Transition: in_progress -> open (backward move)
	// -------------------------------------------------------------
	// Contract rule: Status can only advance forward; returning to open is disallowed.
	invalidMove, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusOpen})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(invalidMove))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for backward transition in_progress -> open, got %d", rec.Code)
	}

	// -------------------------------------------------------------
	// STEP 10: Valid Transition: in_progress -> closed
	// -------------------------------------------------------------
	closeUpdate, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusClosed})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(closeUpdate))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for closing ticket, got %d", rec.Code)
	}

	// -------------------------------------------------------------
	// STEP 11: Contract Rule: A closed ticket cannot be reopened
	// -------------------------------------------------------------
	// Contract explicitly mandates: closed -> cannot move back to open or in_progress.
	reopenMove, _ := json.Marshal(models.UpdateStatusRequest{Status: models.StatusInProgress})
	req = httptest.NewRequest(http.MethodPatch, "/tickets/1/status", bytes.NewReader(reopenMove))
	req.Header.Set("Authorization", "Bearer "+authA.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when attempting to reopen a closed ticket, got %d", rec.Code)
	}
}
