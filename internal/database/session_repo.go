package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// ChatSessionRecord corresponds to ChatSessionRecord in src/database/chatSessionRepository.ts.
type ChatSessionRecord struct {
	ID            int64   `json:"id"`
	ChannelID     string  `json:"channelId"`
	CategoryID    string  `json:"categoryId"`
	WorkspacePath string  `json:"workspacePath"`
	SessionNumber int     `json:"sessionNumber"`
	DisplayName   *string `json:"displayName"`
	IsRenamed     bool    `json:"isRenamed"`
	GuildID       string  `json:"guildId"`
	CreatedAt     string  `json:"createdAt,omitempty"`
}

// CreateChatSessionInput corresponds to CreateChatSessionInput in src/database/chatSessionRepository.ts.
type CreateChatSessionInput struct {
	ChannelID     string
	CategoryID    string
	WorkspacePath string
	SessionNumber int
	GuildID       string
}

// ChatSessionRepository manages the persistence of chat-to-session mappings in SQLite.
type ChatSessionRepository struct {
	db                       *sql.DB
	stmtCreate               *sql.Stmt
	stmtFindByChannelID      *sql.Stmt
	stmtFindByCategoryID     *sql.Stmt
	stmtGetNextSessionNumber *sql.Stmt
	stmtUpdateDisplayName    *sql.Stmt
	stmtFindByDisplayName    *sql.Stmt
	stmtDeleteByChannelID    *sql.Stmt
}

// NewChatSessionRepository initializes the chat_sessions table and prepares cached statements.
func NewChatSessionRepository(db *sql.DB) (*ChatSessionRepository, error) {
	ddl := `
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
	)`
	if _, err := db.Exec(ddl); err != nil {
		return nil, fmt.Errorf("failed to create chat_sessions table: %w", err)
	}

	repo := &ChatSessionRepository{db: db}

	var err error
	repo.stmtCreate, err = db.Prepare(
		"INSERT INTO chat_sessions (channel_id, category_id, workspace_path, session_number, guild_id) VALUES (?, ?, ?, ?, ?)",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stmtCreate: %w", err)
	}

	repo.stmtFindByChannelID, err = db.Prepare(
		"SELECT id, channel_id, category_id, workspace_path, session_number, display_name, is_renamed, guild_id, created_at FROM chat_sessions WHERE channel_id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByChannelID: %w", err)
	}

	repo.stmtFindByCategoryID, err = db.Prepare(
		"SELECT id, channel_id, category_id, workspace_path, session_number, display_name, is_renamed, guild_id, created_at FROM chat_sessions WHERE category_id = ? ORDER BY session_number ASC",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByCategoryID: %w", err)
	}

	repo.stmtGetNextSessionNumber, err = db.Prepare(
		"SELECT MAX(session_number) as max_num FROM chat_sessions WHERE category_id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtGetNextSessionNumber: %w", err)
	}

	repo.stmtUpdateDisplayName, err = db.Prepare(
		"UPDATE chat_sessions SET display_name = ?, is_renamed = 1 WHERE channel_id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtUpdateDisplayName: %w", err)
	}

	repo.stmtFindByDisplayName, err = db.Prepare(
		"SELECT id, channel_id, category_id, workspace_path, session_number, display_name, is_renamed, guild_id, created_at FROM chat_sessions WHERE workspace_path = ? AND display_name = ? ORDER BY id DESC LIMIT 1",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByDisplayName: %w", err)
	}

	repo.stmtDeleteByChannelID, err = db.Prepare(
		"DELETE FROM chat_sessions WHERE channel_id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtDeleteByChannelID: %w", err)
	}

	return repo, nil
}

func scanSessionRecord(scanner interface{ Scan(dest ...any) error }) (*ChatSessionRecord, error) {
	var rec ChatSessionRecord
	var displayName sql.NullString
	var isRenamed int
	var createdAt sql.NullString

	err := scanner.Scan(
		&rec.ID,
		&rec.ChannelID,
		&rec.CategoryID,
		&rec.WorkspacePath,
		&rec.SessionNumber,
		&displayName,
		&isRenamed,
		&rec.GuildID,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	if displayName.Valid {
		rec.DisplayName = &displayName.String
	}
	rec.IsRenamed = (isRenamed == 1)
	if createdAt.Valid {
		rec.CreatedAt = createdAt.String
	}

	return &rec, nil
}

// Create inserts a new chat session and returns the inserted record.
func (r *ChatSessionRepository) Create(input CreateChatSessionInput) (ChatSessionRecord, error) {
	result, err := r.stmtCreate.Exec(
		input.ChannelID,
		input.CategoryID,
		input.WorkspacePath,
		input.SessionNumber,
		input.GuildID,
	)
	if err != nil {
		return ChatSessionRecord{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return ChatSessionRecord{}, err
	}

	return ChatSessionRecord{
		ID:            id,
		ChannelID:     input.ChannelID,
		CategoryID:    input.CategoryID,
		WorkspacePath: input.WorkspacePath,
		SessionNumber: input.SessionNumber,
		DisplayName:   nil,
		IsRenamed:     false,
		GuildID:       input.GuildID,
	}, nil
}

// FindByChannelId returns the chat session for a channel, or nil if not found.
func (r *ChatSessionRepository) FindByChannelId(channelId string) (*ChatSessionRecord, error) {
	row := r.stmtFindByChannelID.QueryRow(channelId)
	rec, err := scanSessionRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// FindByCategoryId returns all sessions within a category, sorted by session_number ASC.
func (r *ChatSessionRepository) FindByCategoryId(categoryId string) ([]ChatSessionRecord, error) {
	rows, err := r.stmtFindByCategoryID.Query(categoryId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]ChatSessionRecord, 0)
	for rows.Next() {
		rec, err := scanSessionRecord(rows)
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

// GetNextSessionNumber returns the next session number within a category (MAX + 1, or 1 if none).
func (r *ChatSessionRepository) GetNextSessionNumber(categoryId string) (int, error) {
	var maxNum sql.NullInt64
	row := r.stmtGetNextSessionNumber.QueryRow(categoryId)
	if err := row.Scan(&maxNum); err != nil {
		return 0, err
	}
	if !maxNum.Valid {
		return 1, nil
	}
	return int(maxNum.Int64) + 1, nil
}

// UpdateDisplayName updates the session display name and sets is_renamed to 1.
func (r *ChatSessionRepository) UpdateDisplayName(channelId string, displayName string) (bool, error) {
	result, err := r.stmtUpdateDisplayName.Exec(displayName, channelId)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// FindByDisplayName finds the most recent session by display name within a workspace.
func (r *ChatSessionRepository) FindByDisplayName(workspacePath string, displayName string) (*ChatSessionRecord, error) {
	row := r.stmtFindByDisplayName.QueryRow(workspacePath, displayName)
	rec, err := scanSessionRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// DeleteByChannelId deletes the session for the specified channel ID.
func (r *ChatSessionRepository) DeleteByChannelId(channelId string) (bool, error) {
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

// Close closes all cached prepared statements.
func (r *ChatSessionRepository) Close() error {
	var errs []error
	stmts := []*sql.Stmt{
		r.stmtCreate,
		r.stmtFindByChannelID,
		r.stmtFindByCategoryID,
		r.stmtGetNextSessionNumber,
		r.stmtUpdateDisplayName,
		r.stmtFindByDisplayName,
		r.stmtDeleteByChannelID,
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
