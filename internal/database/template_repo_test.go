package database

import (
	"testing"
)

func TestTemplateRepo_TableInitialization(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='templates'").Scan(&tableName)
	if err != nil {
		t.Fatalf("expected templates table to exist: %v", err)
	}
	if tableName != "templates" {
		t.Errorf("expected table name templates, got %q", tableName)
	}
}

func TestTemplateRepo_Create_SavesNameAndPrompt(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	result, err := repo.Create(CreateTemplateInput{
		Name:   "PR作成",
		Prompt: "PRを作成してください。変更点をまとめてください。",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if result.ID <= 0 {
		t.Errorf("expected positive ID, got %d", result.ID)
	}
	if result.Name != "PR作成" {
		t.Errorf("expected name 'PR作成', got %q", result.Name)
	}
	if result.Prompt != "PRを作成してください。変更点をまとめてください。" {
		t.Errorf("expected prompt 'PRを作成してください。変更点をまとめてください。', got %q", result.Prompt)
	}
}

func TestTemplateRepo_Create_MultipleTemplates(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})
	_, _ = repo.Create(CreateTemplateInput{Name: "エラー調査", Prompt: "エラーを調査して"})

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(all))
	}
}

func TestTemplateRepo_Create_DuplicateNameThrows(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	_, err = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})
	if err != nil {
		t.Fatalf("first Create() failed: %v", err)
	}

	_, err = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "別のプロンプト"})
	if err == nil {
		t.Fatalf("expected error on duplicate template name, got nil")
	}
}

func TestTemplateRepo_FindAll_RetrievesAll(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})
	_, _ = repo.Create(CreateTemplateInput{Name: "エラー調査", Prompt: "エラーを調査して"})

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(all))
	}
	if all[0].Name != "PR作成" {
		t.Errorf("expected first name 'PR作成', got %q", all[0].Name)
	}
	if all[1].Name != "エラー調査" {
		t.Errorf("expected second name 'エラー調査', got %q", all[1].Name)
	}
}

func TestTemplateRepo_FindAll_Empty(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() failed: %v", err)
	}
	if all == nil {
		t.Fatalf("expected non-nil empty slice, got nil")
	}
	if len(all) != 0 {
		t.Errorf("expected empty array, got %d items", len(all))
	}
}

func TestTemplateRepo_FindById(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	created, err := repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	found, err := repo.FindById(created.ID)
	if err != nil {
		t.Fatalf("FindById() failed: %v", err)
	}
	if found == nil {
		t.Fatalf("expected found record, got nil")
	}
	if found.Name != "PR作成" {
		t.Errorf("expected name 'PR作成', got %q", found.Name)
	}

	notFound, err := repo.FindById(9999)
	if err != nil {
		t.Fatalf("FindById() error: %v", err)
	}
	if notFound != nil {
		t.Errorf("expected nil for non-existent ID, got %+v", notFound)
	}
}

func TestTemplateRepo_FindByName_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})

	found, err := repo.FindByName("PR作成")
	if err != nil {
		t.Fatalf("FindByName() failed: %v", err)
	}
	if found == nil {
		t.Fatalf("expected found record, got nil")
	}
	if found.Name != "PR作成" {
		t.Errorf("expected name 'PR作成', got %q", found.Name)
	}
}

func TestTemplateRepo_FindByName_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	found, err := repo.FindByName("存在しないテンプレート")
	if err != nil {
		t.Fatalf("FindByName() error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil for non-existent name, got %+v", found)
	}
}

func TestTemplateRepo_DeleteByName_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})

	deleted, err := repo.DeleteByName("PR作成")
	if err != nil {
		t.Fatalf("DeleteByName() failed: %v", err)
	}
	if !deleted {
		t.Errorf("expected deleted to be true")
	}

	found, err := repo.FindByName("PR作成")
	if err != nil {
		t.Fatalf("FindByName() error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil after deletion, got %+v", found)
	}
}

func TestTemplateRepo_DeleteByName_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	deleted, err := repo.DeleteByName("存在しない")
	if err != nil {
		t.Fatalf("DeleteByName() error: %v", err)
	}
	if deleted {
		t.Errorf("expected false when deleting non-existent name")
	}
}

func TestTemplateRepo_UpdateByName_UpdatesPrompt(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})

	newPrompt := "新しいPR作成プロンプト"
	updated, err := repo.UpdateByName("PR作成", UpdateTemplateInput{Prompt: &newPrompt})
	if err != nil {
		t.Fatalf("UpdateByName() failed: %v", err)
	}
	if !updated {
		t.Errorf("expected updated to be true")
	}

	found, err := repo.FindByName("PR作成")
	if err != nil {
		t.Fatalf("FindByName() failed: %v", err)
	}
	if found == nil || found.Prompt != "新しいPR作成プロンプト" {
		t.Errorf("expected updated prompt, got %+v", found)
	}
}

func TestTemplateRepo_UpdateByName_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	newPrompt := "新しいプロンプト"
	updated, err := repo.UpdateByName("存在しない", UpdateTemplateInput{Prompt: &newPrompt})
	if err != nil {
		t.Fatalf("UpdateByName() error: %v", err)
	}
	if updated {
		t.Errorf("expected false when updating non-existent name")
	}
}

func TestTemplateRepo_UpdateByName_NilPrompt(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("NewTemplateRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateTemplateInput{Name: "PR作成", Prompt: "PRを作成して"})

	updated, err := repo.UpdateByName("PR作成", UpdateTemplateInput{Prompt: nil})
	if err != nil {
		t.Fatalf("UpdateByName() error: %v", err)
	}
	if updated {
		t.Errorf("expected false when prompt is nil")
	}
}

func TestTemplateRepo_ClosedDBError(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	db.Close()

	_, err = NewTemplateRepository(db)
	if err == nil {
		t.Fatalf("expected error when creating repository on closed db, got nil")
	}
}
