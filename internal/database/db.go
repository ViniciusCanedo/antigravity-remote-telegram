package database

import (
	"database/sql"
	"fmt"
)

// SchemaDDLs contains the DDL statements for all core database tables.
const (
	ChatSessionsDDL = `
CREATE TABLE IF NOT EXISTS chat_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL UNIQUE,
    category_id TEXT NOT NULL,
    workspace_path TEXT NOT NULL,
    session_number INTEGER NOT NULL,
    display_name TEXT,
    is_renamed INTEGER NOT NULL DEFAULT 0,
    guild_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);`

	WorkspaceBindingsDDL = `
CREATE TABLE IF NOT EXISTS workspace_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL UNIQUE,
    workspace_path TEXT NOT NULL,
    guild_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);`

	TemplatesDDL = `
CREATE TABLE IF NOT EXISTS templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    prompt TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE VIEW IF NOT EXISTS prompt_templates AS SELECT * FROM templates;`

	SchedulesDDL = `
CREATE TABLE IF NOT EXISTS schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cron_expression TEXT NOT NULL,
    prompt TEXT NOT NULL,
    workspace_path TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);`
)

// InitSchema executes schema migrations for all tables in the SQLite database.
func InitSchema(db *sql.DB) error {
	ddls := []string{
		ChatSessionsDDL,
		WorkspaceBindingsDDL,
		TemplatesDDL,
		SchedulesDDL,
	}

	for _, ddl := range ddls {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("failed to execute schema DDL: %w", err)
		}
	}

	return nil
}
