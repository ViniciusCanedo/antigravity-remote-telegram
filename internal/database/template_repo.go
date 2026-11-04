package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// TemplateRecord corresponds to TemplateRecord in src/database/templateRepository.ts.
type TemplateRecord struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Prompt    string `json:"prompt"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// CreateTemplateInput corresponds to CreateTemplateInput in src/database/templateRepository.ts.
type CreateTemplateInput struct {
	Name   string
	Prompt string
}

// UpdateTemplateInput corresponds to UpdateTemplateInput in src/database/templateRepository.ts.
type UpdateTemplateInput struct {
	Prompt *string
}

// TemplateRepository manages prompt templates in SQLite.
type TemplateRepository struct {
	db               *sql.DB
	stmtCreate       *sql.Stmt
	stmtFindAll      *sql.Stmt
	stmtFindByID     *sql.Stmt
	stmtFindByName   *sql.Stmt
	stmtDeleteByName *sql.Stmt
	stmtUpdateByName *sql.Stmt
}

// NewTemplateRepository initializes the templates table and prepares cached statements.
func NewTemplateRepository(db *sql.DB) (*TemplateRepository, error) {
	ddl := `
	CREATE TABLE IF NOT EXISTS templates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		prompt TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`
	if _, err := db.Exec(ddl); err != nil {
		return nil, fmt.Errorf("failed to create templates table: %w", err)
	}

	repo := &TemplateRepository{db: db}

	var err error
	repo.stmtCreate, err = db.Prepare(
		"INSERT INTO templates (name, prompt) VALUES (?, ?)",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stmtCreate: %w", err)
	}

	repo.stmtFindAll, err = db.Prepare(
		"SELECT id, name, prompt, created_at FROM templates ORDER BY id ASC",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindAll: %w", err)
	}

	repo.stmtFindByID, err = db.Prepare(
		"SELECT id, name, prompt, created_at FROM templates WHERE id = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByID: %w", err)
	}

	repo.stmtFindByName, err = db.Prepare(
		"SELECT id, name, prompt, created_at FROM templates WHERE name = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtFindByName: %w", err)
	}

	repo.stmtDeleteByName, err = db.Prepare(
		"DELETE FROM templates WHERE name = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtDeleteByName: %w", err)
	}

	repo.stmtUpdateByName, err = db.Prepare(
		"UPDATE templates SET prompt = ? WHERE name = ?",
	)
	if err != nil {
		_ = repo.Close()
		return nil, fmt.Errorf("failed to prepare stmtUpdateByName: %w", err)
	}

	return repo, nil
}

func scanTemplateRecord(scanner interface{ Scan(dest ...any) error }) (*TemplateRecord, error) {
	var rec TemplateRecord
	var createdAt sql.NullString

	err := scanner.Scan(
		&rec.ID,
		&rec.Name,
		&rec.Prompt,
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

// Create inserts a new template and returns the created record.
func (r *TemplateRepository) Create(input CreateTemplateInput) (TemplateRecord, error) {
	result, err := r.stmtCreate.Exec(input.Name, input.Prompt)
	if err != nil {
		return TemplateRecord{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return TemplateRecord{}, err
	}

	return TemplateRecord{
		ID:     id,
		Name:   input.Name,
		Prompt: input.Prompt,
	}, nil
}

// FindAll returns all templates ordered by id ASC.
func (r *TemplateRepository) FindAll() ([]TemplateRecord, error) {
	rows, err := r.stmtFindAll.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]TemplateRecord, 0)
	for rows.Next() {
		rec, err := scanTemplateRecord(rows)
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

// FindById returns the template with the specified ID, or nil if not found.
func (r *TemplateRepository) FindById(id int64) (*TemplateRecord, error) {
	row := r.stmtFindByID.QueryRow(id)
	rec, err := scanTemplateRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// FindByName returns the template with the specified name, or nil if not found.
func (r *TemplateRepository) FindByName(name string) (*TemplateRecord, error) {
	row := r.stmtFindByName.QueryRow(name)
	rec, err := scanTemplateRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rec, nil
}

// DeleteByName deletes the template with the specified name.
func (r *TemplateRepository) DeleteByName(name string) (bool, error) {
	result, err := r.stmtDeleteByName.Exec(name)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// UpdateByName partially updates the prompt for the specified template name.
func (r *TemplateRepository) UpdateByName(name string, input UpdateTemplateInput) (bool, error) {
	if input.Prompt != nil {
		result, err := r.stmtUpdateByName.Exec(*input.Prompt, name)
		if err != nil {
			return false, err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return false, err
		}
		return rowsAffected > 0, nil
	}
	return false, nil
}

// Close closes all cached prepared statements.
func (r *TemplateRepository) Close() error {
	var errs []error
	stmts := []*sql.Stmt{
		r.stmtCreate,
		r.stmtFindAll,
		r.stmtFindByID,
		r.stmtFindByName,
		r.stmtDeleteByName,
		r.stmtUpdateByName,
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
