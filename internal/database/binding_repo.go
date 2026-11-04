package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// WorkspaceBindingRecord corresponds to WorkspaceBindingRecord in src/database/workspaceBindingRepository.ts.
type WorkspaceBindingRecord struct {
	ID            int64  `json:"id"`
	ChannelID     string `json:"channelId"`
	WorkspacePath string `json:"workspacePath"`
	GuildID       string `json:"guildId"`
	CreatedAt     string `json:"createdAt,omitempty"`
}

// CreateWorkspaceBindingInput corresponds to CreateWorkspaceBindingInput in src/database/workspaceBindingRepository.ts.
type CreateWorkspaceBindingInput struct {
	ChannelID     string
	WorkspacePath string
	GuildID       string
}

// WorkspaceBindingRepository manages channel-to-workspace bindings in SQLite.
type WorkspaceBindingRepository struct {
	db                                *sql.DB
	stmtCreate                        *sql.Stmt
	stmtFindByChannelID               *sql.Stmt
	stmtFindByWorkspacePathAndGuildID *sql.Stmt
	stmtFindByGuildID                 *sql.Stmt
	stmtFindAll                       *sql.Stmt
	stmtDeleteByChannelID             *sql.Stmt
	stmtUpsert                        *sql.Stmt
}

// NewWorkspaceBindingRepository initializes the workspace_bindings table and prepares cached statements.
func NewWorkspaceBindingRepository(db *sql.DB) (*WorkspaceBindingRepository, error) {
	ddl := `
	CREATE TABLE IF NOT EXISTS workspace_bindings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id TEXT NOT NULL UNIQUE,
		workspace_path TEXT NOT NULL,
		guild_id TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`
	if _, err := db.Exec(ddl); err != nil {
		return nil, fmt.Errorf("failed to create workspace_bindings table: %w", err)
	}

	repo := &WorkspaceBindingRepository{db: db}

	var err error
	repo.stmtCreate, err = db.Prepare(
		"INSERT INTO workspace_bindings (channel_id, workspace_path, guild_id) VALUES (?, ?, ?)",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stmtCreate: %w", err)
	}

	repo.stmtFindByChannelID, err = db.Prepare(
		"SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings WHERE channel_id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByChannelID: %w", err)
	}

	repo.stmtFindByWorkspacePathAndGuildID, err = db.Prepare(
		"SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings WHERE workspace_path = ? AND guild_id = ? ORDER BY id ASC",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByWorkspacePathAndGuildID: %w", err)
	}

	repo.stmtFindByGuildID, err = db.Prepare(
		"SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings WHERE guild_id = ? ORDER BY id ASC",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByGuildID: %w", err)
	}

	repo.stmtFindAll, err = db.Prepare(
		"SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings ORDER BY id ASC",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindAll: %w", err)
	}

	repo.stmtDeleteByChannelID, err = db.Prepare(
		"DELETE FROM workspace_bindings WHERE channel_id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtDeleteByChannelID: %w", err)
	}

	repo.stmtUpsert, err = db.Prepare(`
		INSERT INTO workspace_bindings (channel_id, workspace_path, guild_id)
		VALUES (?, ?, ?)
		ON CONFLICT(channel_id) DO UPDATE SET
			workspace_path = excluded.workspace_path,
			guild_id = excluded.guild_id
	`)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtUpsert: %w", err)
	}

	return repo, nil
}

func scanBindingRecord(scanner interface{ Scan(dest ...any) error }) (*WorkspaceBindingRecord, error) {
	var rec WorkspaceBindingRecord
	var createdAt sql.NullString

	err := scanner.Scan(
		&rec.ID,
		&rec.ChannelID,
		&rec.WorkspacePath,
		&rec.GuildID,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		rec.CreatedAt = createdAt.String
	}

	return &rec, nil
}

// Create inserts a new workspace binding and returns the created record.
func (r *WorkspaceBindingRepository) Create(input CreateWorkspaceBindingInput) (WorkspaceBindingRecord, error) {
	result, err := r.stmtCreate.Exec(input.ChannelID, input.WorkspacePath, input.GuildID)
	if err != nil {
		return WorkspaceBindingRecord{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return WorkspaceBindingRecord{}, err
	}

	return WorkspaceBindingRecord{
		ID:            id,
		ChannelID:     input.ChannelID,
		WorkspacePath: input.WorkspacePath,
		GuildID:       input.GuildID,
	}, nil
}

// FindByChannelId returns the workspace binding for a channel, or nil if not found.
func (r *WorkspaceBindingRepository) FindByChannelId(channelId string) (*WorkspaceBindingRecord, error) {
	row := r.stmtFindByChannelID.QueryRow(channelId)
	rec, err := scanBindingRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// FindByWorkspacePathAndGuildId returns all bindings matching workspace path and guild ID, ordered by id ASC.
func (r *WorkspaceBindingRepository) FindByWorkspacePathAndGuildId(workspacePath string, guildId string) ([]WorkspaceBindingRecord, error) {
	rows, err := r.stmtFindByWorkspacePathAndGuildID.Query(workspacePath, guildId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]WorkspaceBindingRecord, 0)
	for rows.Next() {
		rec, err := scanBindingRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// FindByGuildId returns all bindings for a given guild ID, ordered by id ASC.
func (r *WorkspaceBindingRepository) FindByGuildId(guildId string) ([]WorkspaceBindingRecord, error) {
	rows, err := r.stmtFindByGuildID.Query(guildId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]WorkspaceBindingRecord, 0)
	for rows.Next() {
		rec, err := scanBindingRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// FindAll returns all workspace bindings, ordered by id ASC.
func (r *WorkspaceBindingRepository) FindAll() ([]WorkspaceBindingRecord, error) {
	rows, err := r.stmtFindAll.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]WorkspaceBindingRecord, 0)
	for rows.Next() {
		rec, err := scanBindingRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// DeleteByChannelId deletes the binding for the specified channel ID.
func (r *WorkspaceBindingRepository) DeleteByChannelId(channelId string) (bool, error) {
	result, err := r.stmtDeleteByChannelID.Exec(channelId)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// Upsert inserts or updates a workspace binding for the specified channel ID and returns the current record.
func (r *WorkspaceBindingRepository) Upsert(input CreateWorkspaceBindingInput) (WorkspaceBindingRecord, error) {
	_, err := r.stmtUpsert.Exec(input.ChannelID, input.WorkspacePath, input.GuildID)
	if err != nil {
		return WorkspaceBindingRecord{}, err
	}

	found, err := r.FindByChannelId(input.ChannelID)
	if err != nil {
		return WorkspaceBindingRecord{}, err
	}
	if found == nil {
		return WorkspaceBindingRecord{}, fmt.Errorf("binding not found after upsert for channel_id %q", input.ChannelID)
	}
	return *found, nil
}

// Close closes all cached prepared statements.
func (r *WorkspaceBindingRepository) Close() error {
	var errs []error
	stmts := []*sql.Stmt{
		r.stmtCreate,
		r.stmtFindByChannelID,
		r.stmtFindByWorkspacePathAndGuildID,
		r.stmtFindByGuildID,
		r.stmtFindAll,
		r.stmtDeleteByChannelID,
		r.stmtUpsert,
	}
	for _, stmt := range stmts {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
