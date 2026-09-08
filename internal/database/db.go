package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	// LEARNING NOTE: Blank Imports in Go
	// -------------------------------------------------------------
	// In Go, `import _ "package"` executes the package's `init()` function without
	// importing symbols directly into your code.
	// modernc.org/sqlite registers itself as a driver named "sqlite" with standard database/sql.
	// Because this driver is 100% pure Go, it requires no C compiler (no CGO),
	// allowing us to compile static binaries and build lightweight Docker images effortlessly.
	_ "modernc.org/sqlite"
)

// LEARNING NOTE: Struct Embedding for Interface Extension
// By embedding `*sql.DB` directly into our `DB` struct, `DB` automatically inherits all
// methods of `*sql.DB` (like Exec, Query, QueryRow, Ping) while letting us attach
// custom helper methods like `migrate()`.
type DB struct {
	*sql.DB
}

// InitDB sets up the SQLite database file, applies WAL pragmas, and runs schema migrations.
func InitDB(dbPath string) (*DB, error) {
	// Ensure parent directory exists (e.g. ./data or /data)
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	// sql.Open initializes the driver handle. It does not open a socket/file immediately.
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Ping sends a probe to verify the database file is readable/writable.
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// LEARNING NOTE: SQLite Concurrency
	// SQLite is file-based. Setting MaxOpenConns to 1 avoids "database is locked" errors
	// during concurrent write operations while Write-Ahead Logging (WAL) handles reads smoothly.
	sqlDB.SetMaxOpenConns(1)

	db := &DB{sqlDB}

	// Execute DDL migrations to ensure tables exist
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Database initialized successfully at: %s", dbPath)
	return db, nil
}

// migrate creates required tables and indexes if they do not already exist.
func (db *DB) migrate() error {
	// 1. Enable foreign keys and WAL mode for reliability
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("failed to execute pragma '%s': %w", p, err)
		}
	}

	// 2. Schema definition for users and tickets
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

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
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	return nil
}
