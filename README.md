# TicketHub - Production Ticket Management Backend

[![Go Test & Build](https://img.shields.io/badge/Build-Passing-brightgreen?style=for-the-badge&logo=go)](https://github.com/Nishant-444)
[![Docker Image](https://img.shields.io/badge/Docker-26.7MB-blue?style=for-the-badge&logo=docker)](https://hub.docker.com)
[![License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)](./LICENSE)

**Version:** 1.0.0  
**Status:** Production-Ready MVP  
**Live API Endpoint:** [https://ticket-system-eva.onrender.com](https://ticket-system-eva.onrender.com) *(Update upon Render deployment)*  
**Tech Stack:** Golang, SQLite, JWT (HMAC-SHA256), Bcrypt, Docker, Alpine Linux, Render  

![Golang](https://img.shields.io/badge/Golang-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-003B57?style=for-the-badge&logo=sqlite&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-000000?style=for-the-badge&logo=jsonwebtokens&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)

---

## Project Overview

**TicketHub** is a high-performance, containerized backend microservice built with **Golang** for managing service and issue tickets. Designed to satisfy all strict requirements of the **EVA Bharat Backend Development Intern Assignment**, the service enforces **stateless JWT authentication**, **strict ownership-based data isolation**, and **one-way lifecycle status progression**.

The application utilizes an embedded, zero-configuration **SQLite database** using a 100% pure-Go driver (`modernc.org/sqlite`), eliminating all C-compiler (CGO) dependencies. This architecture produces a statically linked binary packaged in a minimal Alpine Linux container of only **26.7 MB**, ready for zero-cost cloud deployment on platforms such as Render, Railway, or Fly.io.

### Key Capabilities

- **Ownership Authorization**: Users can view, query, and update only the tickets they authored. Any cross-user access returns `403 Forbidden`.
- **Enforced Lifecycle Flow**: Tickets transition strictly forward: `open` &rarr; `in_progress` &rarr; `closed`. Closed tickets cannot be reopened (`400 Bad Request`).
- **Standard Library Routing**: Implemented using Go 1.22+ native `net/http` pattern routing (`POST /tickets`, `PATCH /tickets/{id}/status`), avoiding heavy framework overhead.
- **Embedded Web Dashboard**: A built-in, responsive web interface served directly at `/` to facilitate immediate browser-based validation by evaluators.

**Live Infrastructure:**

- **Compute:** Render Web Service (Free Tier) / Docker Container
- **Database:** Embedded SQLite 3 with Write-Ahead Logging (WAL) enabled
- **Containerization:** Multi-stage Docker build (Go 1.23 Alpine &rarr; Alpine 3.20)
- **Security:** Bcrypt (cost 10) password hashing, HMAC-SHA256 JWT tokens, CORS policy

---

## System Architecture

```
Client (Web / Mobile / Postman) 
  │
  ▼
[ Render / Cloudflare Edge (HTTPS) ]
  │
  ▼
[ Reverse Proxy / Port 8080 ]
  │
  ▼
[ Go net/http Server (Goroutine Worker Pool) ]
  │
  ├──► CORS Middleware
  ├──► Auth Middleware (JWT Token Verification & Context Injection)
  │
  ├──► Public Handlers (/health, /auth/register, /auth/login)
  └──► Protected Handlers (/tickets, /tickets/{id}, /tickets/{id}/status)
        │
        ▼
   [ Ownership & State Machine Validations ]
        │
        ▼
   [ SQLite Engine (WAL Mode, Parameterized SQL) ]
```

**Request Flow:**

1. HTTP client sends request to the Go backend on port `8080` (or cloud `PORT`).
2. Global `CORSMiddleware` applies browser access headers and intercepts preflight `OPTIONS` calls.
3. If hitting a protected endpoint (`/tickets/*`), `AuthMiddleware` verifies `Authorization: Bearer <token>`, validates cryptographic signature and expiration, and binds `userID` to `r.Context()`.
4. The router dispatches the request to the matching handler method on `Handler`.
5. Handler runs business rules (ownership checks, lifecycle state validation).
6. SQL execution runs through parameterized queries against SQLite to prevent SQL injection.
7. Standardized JSON response returned with proper HTTP status codes (`200`, `201`, `400`, `401`, `403`, `404`).

---

## Core Technology Stack

![Golang](https://img.shields.io/badge/Golang-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-3.45-003B57?style=flat-square&logo=sqlite&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Multi--stage-2496ED?style=flat-square&logo=docker&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-v5.2-000000?style=flat-square&logo=jsonwebtokens&logoColor=white)
![Bcrypt](https://img.shields.io/badge/Bcrypt-v0.28-4A154B?style=flat-square)

| Layer | Technology | Version | Purpose |
| :--- | :--- | :--- | :--- |
| **Language** | Golang | 1.23.6 | Statically typed, high-concurrency backend runtime |
| **HTTP Routing** | Standard Library (`net/http`) | Go 1.22+ | Native pattern-matched routing without third-party frameworks |
| **Database** | SQLite (`modernc.org/sqlite`) | v1.33.1 | Pure-Go embedded relational storage (zero CGO) |
| **Authentication** | JWT (`golang-jwt/jwt/v5`) | v5.2.1 | Stateless HMAC-SHA256 session token issuance and verification |
| **Password Hashing** | Bcrypt (`golang.org/x/crypto`) | v0.28.0 | Adaptive cryptographic password hashing (cost 10) |
| **Containerization** | Docker + Multi-stage Alpine | 3.20 | Minimal 26MB production runtime container |
| **Hosting Platform** | Render / Railway / Fly.io | Free Tier | Public cloud container deployment |

---

## Database Schema

The database follows relational integrity standards with foreign key constraints, automatic timestamps, and indexing on ownership foreign keys.

### Tables Specification

#### 1. `users` Table
Stores registered accounts and securely hashed passwords.

```sql
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL
);
```

#### 2. `tickets` Table
Stores issue tickets owned by specific users.

```sql
CREATE TABLE IF NOT EXISTS tickets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets (user_id);
```

---

## API Architecture

**Base URL:** `http://localhost:8080` (or deployed URL)

### Authentication Flow

```
Register / Login → Verify / Hash Password (Bcrypt) → Generate HMAC-SHA256 JWT
Client attaches Token → Authorization: Bearer <token>
Protected API → Extract Claims → Set user_id in Request Context → Route Execution
```

**Token Strategy:**
- **Token Format:** Signed JWT (HMAC-SHA256)
- **Token Lifetime:** 24 hours
- **Payload Claims:** `user_id` (int64), `email` (string), `exp` (timestamp), `iat` (timestamp)

### Endpoint Categories

#### 1. System & Health
```http
GET    /health                  - Public health check (returns {"status": "ok"})
GET    /                        - Interactive web dashboard (browser testing)
```

#### 2. Authentication
```http
POST   /auth/register           - Register new user account (returns JWT token)
POST   /auth/login              - Authenticate user credentials (returns JWT token)
```

#### 3. Ticket Management (Protected - Requires Bearer Token)
```http
POST   /tickets                 - Create new ticket (initial status: "open")
GET    /tickets                 - List authenticated user's tickets
GET    /tickets/{id}            - Get ticket by ID (enforces ownership)
PATCH  /tickets/{id}/status     - Update ticket status (enforces lifecycle progression)
```

---

## Security Implementation

### Authentication & Authorization

- **Bcrypt Password Protection:** Passwords are never stored in plaintext. Hashed with salt cost 10 using `golang.org/x/crypto/bcrypt`.
- **JWT Cryptographic Verification:** Tokens verified against algorithm confusion attacks (rejecting `none` or asymmetric public key overrides).
- **Ownership Isolation:** 
  - `GET /tickets` executes `SELECT ... WHERE user_id = ?`.
  - `GET /tickets/{id}` and `PATCH /tickets/{id}/status` query the ticket and compare `ticket.UserID == current_user_id`. Mismatches yield `403 Forbidden`.
- **Information Leak Prevention:** Authentication failures return generic `"invalid email or password"` to prevent user enumeration.

### Input Validation & State Machine Rules

- **Status Transition Rules:**
  - `open` &rarr; `in_progress` (Allowed)
  - `open` &rarr; `closed` (Allowed)
  - `in_progress` &rarr; `closed` (Allowed)
  - `in_progress` &rarr; `open` (Rejected: `400 Bad Request`)
  - `closed` &rarr; `open` / `in_progress` (Rejected: `400 Bad Request`, closed tickets cannot be reopened)
- **SQL Injection Prevention:** 100% of queries use parameterized arguments (`?`).

---

## Optimization Strategies

- **Zero-CGO Pure Go SQLite:** Uses `modernc.org/sqlite` instead of `mattn/go-sqlite3`. Compiles statically without requiring GCC, yielding instant builds and zero glibc dependencies.
- **Write-Ahead Logging (WAL):** SQLite configured with `PRAGMA journal_mode = WAL;` to enable concurrent readers alongside atomic writes.
- **Ultra-Lightweight Container:** Multi-stage Docker build yields a final image size of only **26.7 MB** (content size 7.81 MB), allowing near-instant container startup.
- **Offline Module Vendoring:** Pre-vendored dependencies inside `vendor/` allow offline Docker compilation without external network or DNS dependencies.

---

## Deployment Architecture

### Docker Container Specification

```dockerfile
# Stage 1: Build binary using Go 1.23 Alpine
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-s -w" -o /app/server ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.20
WORKDIR /app
RUN mkdir -p /data
COPY --from=builder /app/server /app/server
EXPOSE 8080
ENV PORT=8080
ENV DB_PATH=/data/tickets.db
CMD ["/app/server"]
```

### Free Deployment on Render

1. Create a repository on GitHub and push the `ticket-system` directory:
   ```bash
   git remote add origin https://github.com/<your-username>/ticket-system.git
   git push -u origin main
   ```
2. Log into [Render](https://render.com) and click **New +** &rarr; **Web Service**.
3. Select your GitHub repository.
4. Set **Environment** to **Docker** and **Plan** to **Free**.
5. Set environment variables:
   - `PORT`: `8080`
   - `JWT_SECRET`: `your-production-secret-key`
   - `DB_PATH`: `/data/tickets.db`
6. Click **Deploy Web Service**.

---

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed (Option 1)
- Or [Golang 1.22+](https://go.dev/dl/) installed (Option 2)

### Local Run Contract (Docker)

```bash
# 1. Build the Docker container
docker build -t ticket-system .

# 2. Run the container
docker run -p 8080:8080 ticket-system

# 3. Verify health check
curl http://localhost:8080/health
```

**Expected Health Response:**
```json
{
  "status": "ok"
}
```

### Native Go Run

```bash
# Clone and enter directory
cd ticket-system

# Run tests
go test -v ./...

# Start server
go run cmd/server/main.go
```

The server will start on `http://localhost:8080`. Open `http://localhost:8080` in your web browser to test using the visual dashboard.

### Environment Variables

| Variable | Default | Purpose |
| :--- | :--- | :--- |
| `PORT` | `8080` | Server listening port |
| `JWT_SECRET` | `super-secret-...` | Secret string for HMAC-SHA256 JWT signature |
| `DB_PATH` | `./data/tickets.db` | File path for SQLite database |

---

## Testing

### Automated Test Suite

Run the full suite of unit and integration tests:

```bash
go test -v ./...
```

**Test Coverage Highlights:**
- `TestHealthEndpoint`: Validates `/health` contract and HTTP 200.
- `TestPasswordHashing`: Verifies bcrypt hashing, salt verification, and mismatch detection.
- `TestJWTTokenGenerationAndValidation`: Validates claims encoding, decoding, and signature verification.
- `TestAuthAndTicketLifecycle`: Tests user registration, login, ticket creation, ownership isolation (`403 Forbidden`), and full status lifecycle rules.

### Postman Collection

A complete, pre-configured **Postman Collection (v2.1)** is included:
- **Location:** [`./postman_collection.json`](./postman_collection.json) or [`./docs/ticket_system.postman_collection.json`](./docs/ticket_system.postman_collection.json)
- **Features:** 
  - Dynamic token extraction (Alice token & Bob token automatically captured into collection variables).
  - Automated assertions for status codes and response bodies.
  - Ownership violation tests (`403 Forbidden`).
  - Closed ticket reopening guard tests (`400 Bad Request`).

#### Import Instructions:
1. Open Postman &rarr; Click **Import**.
2. Select `postman_collection.json`.
3. Set the `baseUrl` variable to `http://localhost:8080` (or your deployed URL).
4. Run the collection using Postman Runner.

---

## Project Structure

```
ticket-system/
├── Dockerfile                  # Multi-stage production container definition (~26MB)
├── .dockerignore               # Container build exclusion rules
├── .gitignore                  # Git tracking exclusion rules
├── .env.example                # Sample environment configuration
├── README.md                   # Comprehensive technical documentation
├── postman_collection.json     # Complete Postman testing suite
├── docs/
│   └── ticket_system.postman_collection.json
├── go.mod                      # Module definitions (Go 1.23)
├── go.sum
├── vendor/                     # Self-contained dependencies for offline Docker builds
├── cmd/
│   └── server/
│       └── main.go             # Server initialization, routing & web dashboard
└── internal/
    ├── auth/
    │   ├── auth.go             # Bcrypt hashing & JWT issuance/verification
    │   └── auth_test.go        # Auth package unit tests
    ├── database/
    │   └── db.go               # SQLite initialization, pragmas & migrations
    ├── handlers/
    │   ├── handlers.go         # REST API handlers matching required contract
    │   └── handlers_test.go    # Integration tests (lifecycle, auth, ownership)
    ├── middleware/
    │   └── auth.go             # JWT Bearer token authentication & context injection
    └── models/
        └── models.go           # Structs, type-safe status enums, and DTOs
```

---

## Technical Decisions & Trade-offs

### Why SQLite over In-Memory or PostgreSQL?
- **Persistence:** In-memory storage wipes data on every server restart or container recreation.
- **Simplicity & Zero Cost:** PostgreSQL requires provisioning and managing an external database instance or paying cloud fees. SQLite embeds directly into the application process.
- **Pure-Go Driver:** We use `modernc.org/sqlite`, avoiding CGO compiler requirements, making the binary completely static and compatible with scratch/alpine containers.

### Why Standard Library `net/http` over Frameworks (Gin/Fiber)?
- **Idiomatic Go 1.22+:** Modern Go standard library supports HTTP methods and path parameter extraction (`r.PathValue("id")`).
- **Zero Bloat:** Reduces external dependency tree and prevents dependency vulnerabilities.

### Why Multi-Stage Docker Build?
- **Security & Size:** Compiling in a `golang:1.23-alpine` builder stage and copying solely the binary into an `alpine:3.20` runtime eliminates Go compilers, source code, and build tools from the production container.

---

## Author

**Nishant Sharma**  
GitHub: [@Nishant-444](https://github.com/Nishant-444)  

---

## License

This project is open source and available under the [MIT License](./LICENSE).
