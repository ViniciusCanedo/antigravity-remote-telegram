package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ScheduleRecord corresponds to ScheduleRecord in src/database/scheduleRepository.ts.
type ScheduleRecord struct {
	ID             int64  `json:"id"`
	CronExpression string `json:"cronExpression"`
	Prompt         string `json:"prompt"`
	WorkspacePath  string `json:"workspacePath"`
	Enabled        bool   `json:"enabled"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// CreateScheduleInput corresponds to CreateScheduleInput in src/database/scheduleRepository.ts.
type CreateScheduleInput struct {
	CronExpression string
	Prompt         string
	WorkspacePath  string
	Enabled        bool
}

// UpdateScheduleInput corresponds to UpdateScheduleInput in src/database/scheduleRepository.ts.
type UpdateScheduleInput struct {
	CronExpression *string
	Prompt         *string
	WorkspacePath  *string
	Enabled        *bool
}

// ScheduleRepository manages scheduled jobs in SQLite.
type ScheduleRepository struct {
	db              *sql.DB
	stmtCreate      *sql.Stmt
	stmtFindAll     *sql.Stmt
	stmtFindByID    *sql.Stmt
	stmtFindEnabled *sql.Stmt
	stmtDelete      *sql.Stmt
}

// NewScheduleRepository initializes the schedules table and prepares cached statements.
func NewScheduleRepository(db *sql.DB) (*ScheduleRepository, error) {
	ddl := `
	CREATE TABLE IF NOT EXISTS schedules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cron_expression TEXT NOT NULL,
		prompt TEXT NOT NULL,
		workspace_path TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`
	if _, err := db.Exec(ddl); err != nil {
		return nil, fmt.Errorf("failed to create schedules table: %w", err)
	}

	repo := &ScheduleRepository{db: db}

	var err error
	repo.stmtCreate, err = db.Prepare(
		"INSERT INTO schedules (cron_expression, prompt, workspace_path, enabled) VALUES (?, ?, ?, ?)",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stmtCreate: %w", err)
	}

	repo.stmtFindAll, err = db.Prepare(
		"SELECT id, cron_expression, prompt, workspace_path, enabled, created_at FROM schedules ORDER BY id ASC",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindAll: %w", err)
	}

	repo.stmtFindByID, err = db.Prepare(
		"SELECT id, cron_expression, prompt, workspace_path, enabled, created_at FROM schedules WHERE id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByID: %w", err)
	}

	repo.stmtFindEnabled, err = db.Prepare(
		"SELECT id, cron_expression, prompt, workspace_path, enabled, created_at FROM schedules WHERE enabled = 1 ORDER BY id ASC",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindEnabled: %w", err)
	}

	repo.stmtDelete, err = db.Prepare(
		"DELETE FROM schedules WHERE id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtDelete: %w", err)
	}

	return repo, nil
}

func scanScheduleRecord(scanner interface{ Scan(dest ...any) error }) (*ScheduleRecord, error) {
	var rec ScheduleRecord
	var enabledInt int
	var createdAt sql.NullString

	err := scanner.Scan(
		&rec.ID,
		&rec.CronExpression,
		&rec.Prompt,
		&rec.WorkspacePath,
		&enabledInt,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	rec.Enabled = (enabledInt == 1)
	if createdAt.Valid {
		rec.CreatedAt = createdAt.String
	}

	return &rec, nil
}

// Create inserts a new schedule and returns the created record.
func (r *ScheduleRepository) Create(input CreateScheduleInput) (ScheduleRecord, error) {
	enabledInt := 0
	if input.Enabled {
		enabledInt = 1
	}

	result, err := r.stmtCreate.Exec(
		input.CronExpression,
		input.Prompt,
		input.WorkspacePath,
		enabledInt,
	)
	if err != nil {
		return ScheduleRecord{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return ScheduleRecord{}, err
	}

	return ScheduleRecord{
		ID:             id,
		CronExpression: input.CronExpression,
		Prompt:         input.Prompt,
		WorkspacePath:  input.WorkspacePath,
		Enabled:        input.Enabled,
	}, nil
}

// FindAll returns all schedules ordered by id ASC.
func (r *ScheduleRepository) FindAll() ([]ScheduleRecord, error) {
	rows, err := r.stmtFindAll.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]ScheduleRecord, 0)
	for rows.Next() {
		rec, err := scanScheduleRecord(rows)
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

// FindById returns the schedule with the specified ID, or nil if not found.
func (r *ScheduleRepository) FindById(id int64) (*ScheduleRecord, error) {
	row := r.stmtFindByID.QueryRow(id)
	rec, err := scanScheduleRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// FindEnabled returns only schedules with enabled = 1 ordered by id ASC.
func (r *ScheduleRepository) FindEnabled() ([]ScheduleRecord, error) {
	rows, err := r.stmtFindEnabled.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]ScheduleRecord, 0)
	for rows.Next() {
		rec, err := scanScheduleRecord(rows)
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

// Delete deletes the schedule with the specified ID.
func (r *ScheduleRepository) Delete(id int64) (bool, error) {
	result, err := r.stmtDelete.Exec(id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// Update dynamically updates the fields of a schedule.
func (r *ScheduleRepository) Update(id int64, input UpdateScheduleInput) (bool, error) {
	var sets []string
	var values []any

	if input.CronExpression != nil {
		sets = append(sets, "cron_expression = ?")
		values = append(values, *input.CronExpression)
	}
	if input.Prompt != nil {
		sets = append(sets, "prompt = ?")
		values = append(values, *input.Prompt)
	}
	if input.WorkspacePath != nil {
		sets = append(sets, "workspace_path = ?")
		values = append(values, *input.WorkspacePath)
	}
	if input.Enabled != nil {
		sets = append(sets, "enabled = ?")
		enabledInt := 0
		if *input.Enabled {
			enabledInt = 1
		}
		values = append(values, enabledInt)
	}

	if len(sets) == 0 {
		return false, nil
	}

	values = append(values, id)
	query := fmt.Sprintf("UPDATE schedules SET %s WHERE id = ?", strings.Join(sets, ", "))
	result, err := r.db.Exec(query, values...)
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
func (r *ScheduleRepository) Close() error {
	var errs []error
	stmts := []*sql.Stmt{
		r.stmtCreate,
		r.stmtFindAll,
		r.stmtFindByID,
		r.stmtFindEnabled,
		r.stmtDelete,
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
