package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens or creates a SQLite database at dbPath (or ":memory:").
// When dbPath is not ":memory:", it ensures that the parent directory exists.
// It sets PRAGMA journal_mode = WAL; and PRAGMA busy_timeout = 5000;.
func Open(dbPath string) (*sql.DB, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %q: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if dbPath == ":memory:" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(10)
	}

	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to set WAL journal mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	return db, nil
}

// OpenInMemory opens an isolated in-memory SQLite database instance for unit tests.
func OpenInMemory() (*sql.DB, error) {
	return Open(":memory:")
}
