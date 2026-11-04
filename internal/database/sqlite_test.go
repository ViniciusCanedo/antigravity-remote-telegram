package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenInMemory(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping() failed: %v", err)
	}
}

func TestOpen_FileCreationAndDirCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "antigravity-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "subdir", "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", dbPath, err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping() failed: %v", err)
	}

	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Fatalf("failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode wal, got %q", journalMode)
	}
}

func TestOpen_InvalidDirectoryError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "antigravity-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a regular file where directory is expected
	blockingFile := filepath.Join(tempDir, "file.txt")
	if err := os.WriteFile(blockingFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write blocking file: %v", err)
	}

	invalidPath := filepath.Join(blockingFile, "sub", "test.db")
	_, err = Open(invalidPath)
	if err == nil {
		t.Fatalf("expected error when directory creation fails, got nil")
	}
}

func TestInitSchema(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema() failed: %v", err)
	}

	tables := []string{"chat_sessions", "workspace_bindings", "templates", "schedules"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not created: %v", table, err)
		}
	}

	// Verify view prompt_templates exists
	var viewName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='view' AND name='prompt_templates'").Scan(&viewName)
	if err != nil {
		t.Errorf("view prompt_templates not created: %v", err)
	}
}

func TestInitSchema_ClosedDBError(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	db.Close()

	if err := InitSchema(db); err == nil {
		t.Fatalf("expected error executing InitSchema on closed db, got nil")
	}
}
