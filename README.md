# 🎫 Ticket Management System - Golang Backend

A lightweight, robust, and production-ready backend service built with **Golang** and **SQLite** for managing support tickets with **JWT-based authentication**, **strict ownership authorization**, and **lifecycle state progression**.

---

## 📌 Project Overview

This service satisfies all requirements of the **EVA Bharat Backend Development Intern Assignment**:
- **Authentication**: User registration and login with passwords hashed using **bcrypt** (cost 10) and stateless authentication via **JWT (JSON Web Tokens)**.
- **Ownership Authorization**: Users can only view, retrieve, and update tickets created by their own account. Attempts by other users return `403 Forbidden`.
- **Status Lifecycle Progression**:
  - `open` &rarr; `in_progress` &rarr; `closed`
  - Tickets cannot move backward (e.g. `in_progress` &rarr; `open`).
  - Closed tickets **cannot be reopened** (attempting to change status of a `closed` ticket returns `400 Bad Request`).
- **Standard Library Go Routing**: Utilizes Go 1.22+ native pattern routing (`net/http`) without bloated router dependencies.
- **Embedded Persistent Storage**: Pure-Go SQLite (`modernc.org/sqlite`) enabling zero-CGO static builds that run anywhere without external database infrastructure.
- **Interactive UI Dashboard**: Includes an embedded, responsive web interface served directly at `/` for browser-based evaluation.

---

## 🚀 Quick Start (Local Run Contract)

### Option 1: Docker (Recommended)

Honoring the assignment run contract:

```bash
# 1. Build the Docker image
docker build -t ticket-system .

# 2. Run the Docker container
docker run -p 8080:8080 ticket-system

# 3. Verify health
curl http://localhost:8080/health
```

**Expected Health Response:**
```json
{
  "status": "ok"
}
```

### Option 2: Native Go Run

If you have Go installed locally:

```bash
# Clone or navigate to the directory
cd ticket-system

# Run tests
go test -v ./...

# Run the server
go run cmd/server/main.go
```

The server starts on `http://localhost:8080`. Open your browser to `http://localhost:8080` to access the interactive web dashboard.

---

## 📡 REST API Reference

All protected endpoints require the HTTP header:
```http
Authorization: Bearer <your_jwt_token>
```

| Method | Endpoint | Auth Required | Purpose |
| :--- | :--- | :---: | :--- |
| `GET` | `/health` | No | Health check verification |
| `POST` | `/auth/register` | No | Register a new user account |
| `POST` | `/auth/login` | No | Authenticate user and obtain JWT |
| `POST` | `/tickets` | **Yes** | Create a new ticket (initial status: `open`) |
| `GET` | `/tickets` | **Yes** | List all tickets owned by the current user |
| `GET` | `/tickets/{id}` | **Yes** | Get own ticket by ID |
| `PATCH` | `/tickets/{id}/status` | **Yes** | Update own ticket status |

---

## 🧪 API Examples with `curl`

### 1. Health Check
```bash
curl -X GET http://localhost:8080/health
```
**Response (200 OK):**
```json
{
  "status": "ok"
}
```

### 2. User Registration
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "securepassword123"}'
```
**Response (201 Created):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsIn...",
  "message": "user registered successfully",
  "user": {
    "id": 1,
    "email": "alice@example.com",
    "created_at": "2026-09-08T05:17:34Z"
  }
}
```

### 3. User Login
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "securepassword123"}'
```
**Response (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsIn...",
  "message": "login successful",
  "user": {
    "id": 1,
    "email": "alice@example.com",
    "created_at": "2026-09-08T05:17:34Z"
  }
}
```

### 4. Create Ticket
```bash
curl -X POST http://localhost:8080/tickets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"title": "Bug in billing", "description": "Payment webhook timeouts"}'
```
**Response (201 Created):**
```json
{
  "id": 1,
  "user_id": 1,
  "title": "Bug in billing",
  "description": "Payment webhook timeouts",
  "status": "open",
  "created_at": "2026-09-08T05:20:00Z",
  "updated_at": "2026-09-08T05:20:00Z"
}
```

### 5. List Logged-in User's Tickets
```bash
curl -X GET http://localhost:8080/tickets \
  -H "Authorization: Bearer <TOKEN>"
```
**Response (200 OK):**
```json
[
  {
    "id": 1,
    "user_id": 1,
    "title": "Bug in billing",
    "description": "Payment webhook timeouts",
    "status": "open",
    "created_at": "2026-09-08T05:20:00Z",
    "updated_at": "2026-09-08T05:20:00Z"
  }
]
```

### 6. Get Own Ticket by ID
```bash
curl -X GET http://localhost:8080/tickets/1 \
  -H "Authorization: Bearer <TOKEN>"
```
*Note: If another user attempts to retrieve this ticket, the API returns `403 Forbidden`.*

### 7. Update Ticket Status
```bash
curl -X PATCH http://localhost:8080/tickets/1/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"status": "in_progress"}'
```
**Response (200 OK):**
```json
{
  "id": 1,
  "user_id": 1,
  "title": "Bug in billing",
  "description": "Payment webhook timeouts",
  "status": "in_progress",
  "created_at": "2026-09-08T05:20:00Z",
  "updated_at": "2026-09-08T05:22:15Z"
}
```

---

## 🔒 Status Flow & Business Rules

1. **Permitted Status Values**: `open`, `in_progress`, `closed`.
2. **Progression**:
   - `open` &rarr; `in_progress` (Allowed)
   - `open` &rarr; `closed` (Allowed)
   - `in_progress` &rarr; `closed` (Allowed)
   - `in_progress` &rarr; `open` (Rejected: `400 Bad Request`)
   - `closed` &rarr; `open` or `in_progress` (Rejected: `400 Bad Request`, closed tickets cannot be reopened).
3. **Ownership Isolation**:
   - User B cannot access `GET /tickets/{id}` for User A's ticket (returns `403 Forbidden`).
   - User B cannot update `PATCH /tickets/{id}/status` for User A's ticket (returns `403 Forbidden`).
   - `GET /tickets` returns strictly the tickets where `user_id == current_user_id`.

---

## ⚙️ Environment Variables

Copy `.env.example` to `.env` to configure:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | Port for the HTTP server |
| `JWT_SECRET` | `super-secret-...` | Secret string for HMAC-SHA256 JWT signature |
| `DB_PATH` | `./data/tickets.db` | Filepath for SQLite database file |

---

## 🚢 Deployment Guide (Render Free Web Service)

This repository includes a multi-stage `Dockerfile` ready for zero-config deployment on Render.

1. **Push to GitHub**:
   - Push this `ticket-system` folder to a new public GitHub repository.
2. **Log into Render** ([render.com](https://render.com)):
   - Click **New +** &rarr; **Web Service**.
   - Connect your GitHub repository.
3. **Configuration**:
   - **Name**: `eva-ticket-system` (or your choice)
   - **Environment**: `Docker`
   - **Region**: Any (e.g., Singapore or Oregon)
   - **Branch**: `main`
   - **Plan**: `Free`
4. **Environment Variables**:
   - `PORT`: `8080`
   - `JWT_SECRET`: (Set any secure random string)
   - `DB_PATH`: `/data/tickets.db`
5. **Deploy**:
   - Render will build the Docker container and provide a live URL, e.g.:
     - **App URL**: `https://<your-service-name>.onrender.com`
     - **Health Check URL**: `https://<your-service-name>.onrender.com/health`

---

## 🎓 Learning Go from Scratch: Guide for Backend Engineers

If you are coming from Node.js, Python, or Java, here is a quick guide to how Go concepts in this codebase work:

### 1. Packages & Project Structure
- Every Go file starts with `package <name>`. Files in the same folder share the same package name.
- `package main` with `func main()` is the entrypoint for executable binaries.
- Capitalized symbols (e.g. `User`, `InitDB`, `NewHandler`) are **exported** (public). Lowercase symbols (e.g. `migrate`, `sendJSONError`) are **unexported** (private).

### 2. Structs & JSON Tags (`internal/models/models.go`)
- Go has no `class` keyword; it uses `struct` to group fields.
- Backtick tags like ` + "`json:\"title\"`" + ` tell Go's JSON parser how to serialize/deserialize keys.
- `json:"-"` prevents sensitive fields (like password hashes) from ever being sent in JSON.

### 3. Pointers (`*`) & Values
- `db *database.DB` means `db` is a pointer (memory reference), avoiding expensive copying of large structs.
- The `&` operator takes the memory address (e.g. `&models.User{...}`).

### 4. Error Handling (`if err != nil`)
- Go has no `try...catch` exceptions for normal control flow.
- Functions return errors as normal values: `func DoSomething() (Result, error)`.
- You check errors explicitly:
  ```go
  if err != nil {
      // handle error
      return
  }
  ```

### 5. Standard `net/http` Routing (Go 1.22+)
- Go's built-in `http.ServeMux` natively matches HTTP verbs and paths:
  ```go
  mux.HandleFunc("GET /health", handler.Health)
  mux.HandleFunc("GET /tickets/{id}", handler.GetTicket)
  ```
- Path parameters are extracted using `r.PathValue("id")`.

### 6. Context & Middleware (`internal/middleware/auth.go`)
- HTTP requests carry a `context.Context`.
- When JWT authentication succeeds, we attach the `userID` to the request's context:
  ```go
  ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
  next.ServeHTTP(w, r.WithContext(ctx))
  ```
- Handlers retrieve it via `middleware.GetUserID(r.Context())`.

### 7. Zero-CGO SQLite (`modernc.org/sqlite`)
- Standard SQLite drivers require GCC/CGO (a C compiler).
- We use `modernc.org/sqlite`, a 100% pure Go SQLite implementation, making our Docker containers tiny (~26MB) and statically compiled with `CGO_ENABLED=0`.
