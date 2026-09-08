package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ticket-system/internal/database"
	"ticket-system/internal/handlers"
	"ticket-system/internal/middleware"
)

func main() {
	// Configuration via environment variables with production defaults
	port := getEnv("PORT", "8080")
	jwtSecret := getEnv("JWT_SECRET", "super-secret-ticket-system-jwt-key-eva-bharat-2026")
	dbPath := getEnv("DB_PATH", "./data/tickets.db")

	log.Println("==================================================")
	log.Println("     EVA BHARAT - TICKET SYSTEM BACKEND (GOLANG)  ")
	log.Println("==================================================")
	log.Printf("Starting service on port: %s", port)
	log.Printf("Database path: %s", dbPath)

	// Initialize SQLite database
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Fatal: Could not initialize database: %v", err)
	}
	defer db.Close()

	// Initialize handlers and middleware
	handler := handlers.NewHandler(db, jwtSecret)
	authMiddleware := middleware.AuthMiddleware(jwtSecret)

	// Setup HTTP routing with Go standard library ServeMux
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("POST /auth/register", handler.Register)
	mux.HandleFunc("POST /auth/login", handler.Login)

	// Protected routes
	mux.Handle("POST /tickets", authMiddleware(http.HandlerFunc(handler.CreateTicket)))
	mux.Handle("GET /tickets", authMiddleware(http.HandlerFunc(handler.ListTickets)))
	mux.Handle("GET /tickets/{id}", authMiddleware(http.HandlerFunc(handler.GetTicket)))
	mux.Handle("PATCH /tickets/{id}/status", authMiddleware(http.HandlerFunc(handler.UpdateTicketStatus)))

	// Web UI dashboard
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(frontendHTML))
	})

	wrappedMux := middleware.CORSMiddleware(mux)

	// Server configuration
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      wrappedMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server asynchronously
	go func() {
		log.Printf("Server listening on http://0.0.0.0:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server ListenAndServe error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT or SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly.")
}

// getEnv retrieves the value of an environment variable or returns a fallback default.
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// Embedded frontend dashboard HTML
const frontendHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>EVA Bharat - Ticket System Dashboard</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg-main: #0B0F19;
      --bg-card: rgba(18, 24, 38, 0.85);
      --bg-input: #151D2E;
      --border-color: rgba(255, 255, 255, 0.08);
      --text-main: #F1F5F9;
      --text-muted: #94A3B8;
      --primary: #3B82F6;
      --primary-hover: #2563EB;
      --accent-cyan: #06B6D4;
      --accent-green: #10B981;
      --accent-amber: #F59E0B;
      --accent-rose: #F43F5E;
      --font-main: 'Outfit', -apple-system, BlinkMacSystemFont, sans-serif;
      --font-mono: 'JetBrains Mono', monospace;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: radial-gradient(circle at 50% 0%, #172554 0%, var(--bg-main) 70%);
      color: var(--text-main);
      font-family: var(--font-main);
      min-height: 100vh;
      padding: 2rem 1rem;
      display: flex;
      flex-direction: column;
      align-items: center;
    }
    .container { width: 100%; max-width: 960px; display: flex; flex-direction: column; gap: 1.75rem; }
    header {
      text-align: center;
      padding: 1.5rem 0;
      border-bottom: 1px solid var(--border-color);
    }
    .badge {
      display: inline-block;
      padding: 0.35rem 0.85rem;
      border-radius: 9999px;
      font-size: 0.75rem;
      font-weight: 600;
      letter-spacing: 0.05em;
      text-transform: uppercase;
      background: rgba(59, 130, 246, 0.15);
      color: #60A5FA;
      border: 1px solid rgba(59, 130, 246, 0.3);
      margin-bottom: 0.75rem;
    }
    h1 { font-size: 2.2rem; font-weight: 700; letter-spacing: -0.02em; margin-bottom: 0.4rem; }
    .subtitle { color: var(--text-muted); font-size: 0.95rem; }
    .status-bar {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: 12px;
      padding: 1rem 1.5rem;
      display: flex;
      justify-content: space-between;
      align-items: center;
      backdrop-filter: blur(12px);
    }
    .auth-status { display: flex; align-items: center; gap: 0.6rem; font-size: 0.9rem; }
    .indicator { width: 10px; height: 10px; border-radius: 50%; background: var(--accent-rose); box-shadow: 0 0 10px var(--accent-rose); }
    .indicator.connected { background: var(--accent-green); box-shadow: 0 0 10px var(--accent-green); }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; }
    @media (max-width: 768px) { .grid { grid-template-columns: 1fr; } }
    .card {
      background: var(--bg-card);
      border: 1px solid var(--border-color);
      border-radius: 16px;
      padding: 1.5rem;
      backdrop-filter: blur(12px);
      box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
    }
    .card-title { font-size: 1.15rem; font-weight: 600; margin-bottom: 1.25rem; display: flex; align-items: center; gap: 0.5rem; }
    .form-group { margin-bottom: 1rem; }
    label { display: block; font-size: 0.8rem; font-weight: 500; color: var(--text-muted); margin-bottom: 0.4rem; }
    input, textarea, select {
      width: 100%;
      background: var(--bg-input);
      border: 1px solid var(--border-color);
      border-radius: 8px;
      padding: 0.65rem 0.85rem;
      color: var(--text-main);
      font-family: inherit;
      font-size: 0.9rem;
      transition: all 0.2s;
    }
    input:focus, textarea:focus, select:focus {
      outline: none;
      border-color: var(--primary);
      box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
    }
    textarea { resize: vertical; min-height: 80px; }
    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 0.5rem;
      padding: 0.65rem 1.25rem;
      border-radius: 8px;
      font-size: 0.9rem;
      font-weight: 600;
      cursor: pointer;
      border: none;
      transition: all 0.2s;
      background: var(--primary);
      color: white;
    }
    .btn:hover { background: var(--primary-hover); transform: translateY(-1px); }
    .btn-secondary { background: rgba(255, 255, 255, 0.08); color: var(--text-main); }
    .btn-secondary:hover { background: rgba(255, 255, 255, 0.15); }
    .btn-sm { padding: 0.35rem 0.75rem; font-size: 0.8rem; border-radius: 6px; }
    .btn-danger { background: rgba(244, 63, 94, 0.2); color: #FDA4AF; border: 1px solid rgba(244, 63, 94, 0.3); }
    .btn-danger:hover { background: var(--accent-rose); color: white; }
    .ticket-list { display: flex; flex-direction: column; gap: 0.85rem; max-height: 480px; overflow-y: auto; padding-right: 0.25rem; }
    .ticket-item {
      background: rgba(255, 255, 255, 0.03);
      border: 1px solid var(--border-color);
      border-radius: 12px;
      padding: 1.1rem;
      display: flex;
      flex-direction: column;
      gap: 0.6rem;
      transition: border 0.2s;
    }
    .ticket-item:hover { border-color: rgba(255, 255, 255, 0.2); }
    .ticket-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 0.5rem; }
    .ticket-title { font-size: 1rem; font-weight: 600; }
    .status-tag {
      font-family: var(--font-mono);
      font-size: 0.72rem;
      font-weight: 600;
      padding: 0.25rem 0.55rem;
      border-radius: 6px;
      text-transform: uppercase;
    }
    .status-open { background: rgba(59, 130, 246, 0.2); color: #93C5FD; border: 1px solid rgba(59, 130, 246, 0.4); }
    .status-in_progress { background: rgba(245, 158, 11, 0.2); color: #FCD34D; border: 1px solid rgba(245, 158, 11, 0.4); }
    .status-closed { background: rgba(16, 185, 129, 0.2); color: #6EE7B7; border: 1px solid rgba(16, 185, 129, 0.4); }
    .ticket-desc { font-size: 0.85rem; color: var(--text-muted); line-height: 1.4; }
    .ticket-footer { display: flex; justify-content: space-between; align-items: center; margin-top: 0.4rem; padding-top: 0.6rem; border-top: 1px solid rgba(255, 255, 255, 0.05); font-size: 0.75rem; color: var(--text-muted); }
    .status-actions { display: flex; gap: 0.4rem; }
    .toast {
      position: fixed;
      bottom: 2rem;
      right: 2rem;
      padding: 0.85rem 1.4rem;
      border-radius: 10px;
      font-size: 0.88rem;
      font-weight: 500;
      display: none;
      z-index: 1000;
      backdrop-filter: blur(12px);
      box-shadow: 0 10px 25px rgba(0, 0, 0, 0.4);
    }
    .toast.success { background: rgba(16, 185, 129, 0.9); color: white; display: block; }
    .toast.error { background: rgba(244, 63, 94, 0.9); color: white; display: block; }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <span class="badge">EVA Bharat &bull; Backend Intern Assignment</span>
      <h1>Ticket Management System</h1>
      <p class="subtitle">Built with Golang, SQLite, JWT Authentication &amp; Ownership Verification</p>
    </header>

    <div class="status-bar">
      <div class="auth-status">
        <div class="indicator" id="authIndicator"></div>
        <span id="authStatusText">Not authenticated</span>
      </div>
      <div style="display: flex; gap: 0.6rem;">
        <button class="btn btn-sm btn-secondary" onclick="checkHealth()">Test /health</button>
        <button class="btn btn-sm btn-danger" id="logoutBtn" style="display: none;" onclick="logout()">Logout</button>
      </div>
    </div>

    <div class="grid">
      <!-- Auth Card -->
      <div class="card" id="authCard">
        <h2 class="card-title">🔐 Authentication</h2>
        <div class="form-group">
          <label>Email Address</label>
          <input type="email" id="authEmail" placeholder="candidate@evabharat.ai">
        </div>
        <div class="form-group">
          <label>Password (min 6 chars)</label>
          <input type="password" id="authPassword" placeholder="••••••••">
        </div>
        <div style="display: flex; gap: 0.75rem; margin-top: 1.25rem;">
          <button class="btn" style="flex: 1;" onclick="submitAuth('/auth/login')">Login</button>
          <button class="btn btn-secondary" style="flex: 1;" onclick="submitAuth('/auth/register')">Register</button>
        </div>
      </div>

      <!-- Ticket Creation Card -->
      <div class="card" id="createCard">
        <h2 class="card-title">📝 Create New Ticket</h2>
        <div class="form-group">
          <label>Title</label>
          <input type="text" id="ticketTitle" placeholder="e.g., Payment gateway timeout issue">
        </div>
        <div class="form-group">
          <label>Description</label>
          <textarea id="ticketDesc" placeholder="Details of the issue or feature request..."></textarea>
        </div>
        <button class="btn" style="width: 100%; margin-top: 0.5rem;" onclick="createTicket()">Submit Ticket</button>
      </div>
    </div>

    <!-- My Tickets Card -->
    <div class="card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.25rem;">
        <h2 class="card-title" style="margin-bottom: 0;">📋 My Tickets (<span id="ticketCount">0</span>)</h2>
        <button class="btn btn-sm btn-secondary" onclick="loadTickets()">Refresh</button>
      </div>
      <div class="ticket-list" id="ticketList">
        <p style="color: var(--text-muted); text-align: center; padding: 2rem;">Please log in to view and manage your tickets.</p>
      </div>
    </div>
  </div>

  <div class="toast" id="toast"></div>

  <script>
    let token = localStorage.getItem("token") || "";
    let userEmail = localStorage.getItem("email") || "";

    function showToast(msg, isSuccess = true) {
      const toast = document.getElementById("toast");
      toast.textContent = msg;
      toast.className = "toast " + (isSuccess ? "success" : "error");
      setTimeout(() => { toast.className = "toast"; }, 3500);
    }

    function updateAuthUI() {
      const indicator = document.getElementById("authIndicator");
      const statusText = document.getElementById("authStatusText");
      const logoutBtn = document.getElementById("logoutBtn");

      if (token) {
        indicator.classList.add("connected");
        statusText.textContent = "Logged in as: " + userEmail;
        logoutBtn.style.display = "inline-flex";
        loadTickets();
      } else {
        indicator.classList.remove("connected");
        statusText.textContent = "Not authenticated";
        logoutBtn.style.display = "none";
        document.getElementById("ticketList").innerHTML = '<p style="color: var(--text-muted); text-align: center; padding: 2rem;">Please log in to view and manage your tickets.</p>';
        document.getElementById("ticketCount").textContent = "0";
      }
    }

    async function checkHealth() {
      try {
        const res = await fetch("/health");
        const data = await res.json();
        showToast("/health status: " + data.status, true);
      } catch (err) {
        showToast("Health check failed: " + err.message, false);
      }
    }

    async function submitAuth(endpoint) {
      const email = document.getElementById("authEmail").value.trim();
      const password = document.getElementById("authPassword").value.trim();

      if (!email || !password) {
        showToast("Please enter email and password", false);
        return;
      }

      try {
        const res = await fetch(endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, password })
        });
        const data = await res.json();

        if (!res.ok) {
          showToast(data.error || "Authentication failed", false);
          return;
        }

        token = data.token;
        userEmail = data.user ? data.user.email : email;
        localStorage.setItem("token", token);
        localStorage.setItem("email", userEmail);
        showToast(endpoint === "/auth/register" ? "Registration successful!" : "Logged in successfully!", true);
        updateAuthUI();
      } catch (err) {
        showToast("Network error: " + err.message, false);
      }
    }

    function logout() {
      token = "";
      userEmail = "";
      localStorage.removeItem("token");
      localStorage.removeItem("email");
      showToast("Logged out", true);
      updateAuthUI();
    }

    async function createTicket() {
      if (!token) {
        showToast("Please log in first", false);
        return;
      }
      const title = document.getElementById("ticketTitle").value.trim();
      const description = document.getElementById("ticketDesc").value.trim();

      if (!title) {
        showToast("Ticket title is required", false);
        return;
      }

      try {
        const res = await fetch("/tickets", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "Authorization": "Bearer " + token
          },
          body: JSON.stringify({ title, description })
        });
        const data = await res.json();

        if (!res.ok) {
          showToast(data.error || "Failed to create ticket", false);
          return;
        }

        showToast("Ticket #" + data.id + " created!", true);
        document.getElementById("ticketTitle").value = "";
        document.getElementById("ticketDesc").value = "";
        loadTickets();
      } catch (err) {
        showToast("Error: " + err.message, false);
      }
    }

    async function loadTickets() {
      if (!token) return;
      try {
        const res = await fetch("/tickets", {
          headers: { "Authorization": "Bearer " + token }
        });
        const data = await res.json();
        if (!res.ok) {
          showToast(data.error || "Failed to load tickets", false);
          return;
        }

        document.getElementById("ticketCount").textContent = data.length;
        const list = document.getElementById("ticketList");
        if (data.length === 0) {
          list.innerHTML = '<p style="color: var(--text-muted); text-align: center; padding: 2rem;">No tickets created yet. Create one above!</p>';
          return;
        }

        list.innerHTML = data.map(t => {
          let buttons = '';
          if (t.status === 'open') {
            buttons = '<button class="btn btn-sm btn-secondary" onclick="updateStatus(' + t.id + ', \'in_progress\')">Move to In Progress &rarr;</button>' +
                      '<button class="btn btn-sm btn-secondary" onclick="updateStatus(' + t.id + ', \'closed\')">Close &times;</button>';
          } else if (t.status === 'in_progress') {
            buttons = '<button class="btn btn-sm btn-secondary" onclick="updateStatus(' + t.id + ', \'closed\')">Close Ticket &times;</button>';
          } else {
            buttons = '<span style="color: var(--accent-green); font-size: 0.8rem;">&check; Ticket Completed</span>';
          }

          return '<div class="ticket-item">' +
            '<div class="ticket-header">' +
              '<span class="ticket-title">#' + t.id + ' &bull; ' + escapeHtml(t.title) + '</span>' +
              '<span class="status-tag status-' + t.status + '">' + t.status.replace('_', ' ') + '</span>' +
            '</div>' +
            '<p class="ticket-desc">' + escapeHtml(t.description || 'No description provided.') + '</p>' +
            '<div class="ticket-footer">' +
              '<span>Created: ' + new Date(t.created_at).toLocaleString() + '</span>' +
              '<div class="status-actions">' + buttons + '</div>' +
            '</div>' +
          '</div>';
        }).join('');
      } catch (err) {
        showToast("Error: " + err.message, false);
      }
    }

    async function updateStatus(id, newStatus) {
      if (!token) return;
      try {
        const res = await fetch("/tickets/" + id + "/status", {
          method: "PATCH",
          headers: {
            "Content-Type": "application/json",
            "Authorization": "Bearer " + token
          },
          body: JSON.stringify({ status: newStatus })
        });
        const data = await res.json();
        if (!res.ok) {
          showToast(data.error || "Status update failed", false);
          return;
        }
        showToast("Ticket #" + id + " updated to " + newStatus, true);
        loadTickets();
      } catch (err) {
        showToast("Error: " + err.message, false);
      }
    }

    function escapeHtml(str) {
      return (str || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
    }

    // Initialize UI on page load
    updateAuthUI();
  </script>
</body>
</html>`
