package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	// In Go, the blank import `_ "..."` executes the package's `init()` function without importing any symbols.
	// modernc.org/sqlite registers itself as a driver named "sqlite" with the standard "database/sql" package.
	// Because it is pure Go, it requires no C compiler (no CGO), making Docker builds fast and static!
	_ "modernc.org/sqlite"
)

// DB wraps standard library sql.DB pointer.
// In Go, structs can hold state and methods can be attached to them.
type DB struct {
	*sql.DB
}

// InitDB sets up the SQLite database file, creates tables, and runs pragmas.
func InitDB(dbPath string) (*DB, error) {
	// If dbPath contains directory folders, make sure they exist
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	// sql.Open initializes the driver handle. It does not actually connect to the database yet.
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Ping verifies that a connection can actually be made.
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// For SQLite, setting max open connections prevents "database is locked" errors during concurrent writes.
	// WAL (Write-Ahead Logging) allows concurrent readers and a writer.
	sqlDB.SetMaxOpenConns(1)

	db := &DB{sqlDB}

	// Run initial schema migrations
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Database initialized successfully at: %s", dbPath)
	return db, nil
}

// migrate creates the necessary database tables if they do not already exist.
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
