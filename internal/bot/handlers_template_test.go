package bot

import (
	"errors"
	"strings"
	"testing"

	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/i18n"
)

type mockTemplateStore struct {
	findAllFn      func() ([]database.TemplateRecord, error)
	findByNameFn   func(name string) (*database.TemplateRecord, error)
	createFn       func(input database.CreateTemplateInput) (database.TemplateRecord, error)
	deleteByNameFn func(name string) (bool, error)
}

func (m *mockTemplateStore) FindAll() ([]database.TemplateRecord, error) {
	if m.findAllFn != nil {
		return m.findAllFn()
	}
	return nil, nil
}

func (m *mockTemplateStore) FindByName(name string) (*database.TemplateRecord, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(name)
	}
	return nil, nil
}

func (m *mockTemplateStore) Create(input database.CreateTemplateInput) (database.TemplateRecord, error) {
	if m.createFn != nil {
		return m.createFn(input)
	}
	return database.TemplateRecord{ID: 1, Name: input.Name, Prompt: input.Prompt}, nil
}

func (m *mockTemplateStore) DeleteByName(name string) (bool, error) {
	if m.deleteByNameFn != nil {
		return m.deleteByNameFn(name)
	}
	return false, nil
}

func TestSlashCommandHandler_UnknownCommand(t *testing.T) {
	handler := NewSlashCommandHandlerWithStore(&mockTemplateStore{})
	result := handler.HandleCommand("unknown", []string{})
	if result.Success {
		t.Fatal("expected Success to be false")
	}
	if !strings.Contains(result.Message, "Unknown command") {
		t.Fatalf("expected message to contain 'Unknown command', got %q", result.Message)
	}
}

func TestSlashCommandHandler_RejectsPluralAlias(t *testing.T) {
	handler := NewSlashCommandHandlerWithStore(&mockTemplateStore{})
	result := handler.HandleCommand("templates", []string{})
	if result.Success {
		t.Fatal("expected Success to be false")
	}
	if !strings.Contains(result.Message, "Unknown command") {
		t.Fatalf("expected message to contain 'Unknown command', got %q", result.Message)
	}
}

func TestSlashCommandHandler_ListTemplates(t *testing.T) {
	store := &mockTemplateStore{
		findAllFn: func() ([]database.TemplateRecord, error) {
			return []database.TemplateRecord{
				{ID: 1, Name: "PR作成", Prompt: "PRを作成して"},
				{ID: 2, Name: "エラー調査", Prompt: "エラーを調査して"},
			}, nil
		},
	}
	handler := NewSlashCommandHandlerWithStore(store)
	result := handler.HandleCommand("template", []string{})
	if !result.Success {
		t.Fatal("expected Success to be true")
	}
	if !strings.Contains(result.Message, "PR作成") || !strings.Contains(result.Message, "エラー調査") {
		t.Fatalf("expected message to contain template names, got %q", result.Message)
	}
}

func TestSlashCommandHandler_ListEmpty(t *testing.T) {
	store := &mockTemplateStore{
		findAllFn: func() ([]database.TemplateRecord, error) {
			return []database.TemplateRecord{}, nil
		},
	}
	handler := NewSlashCommandHandlerWithStore(store)
	result := handler.HandleCommand("template", []string{})
	if !result.Success {
		t.Fatal("expected Success to be true")
	}
	expected := i18n.T("📝 No templates registered.")
	if !strings.Contains(result.Message, expected) && !strings.Contains(result.Message, "No templates registered") {
		t.Fatalf("expected message to contain 'No templates registered', got %q", result.Message)
	}
}

func TestSlashCommandHandler_InvokeTemplate(t *testing.T) {
	store := &mockTemplateStore{
		findByNameFn: func(name string) (*database.TemplateRecord, error) {
			if name == "PR作成" {
				return &database.TemplateRecord{
					ID:     1,
					Name:   "PR作成",
					Prompt: "PRを作成してください。",
				}, nil
			}
			return nil, nil
		},
	}
	handler := NewSlashCommandHandlerWithStore(store)
	result := handler.HandleCommand("template", []string{"PR作成"})
	if !result.Success {
		t.Fatal("expected Success to be true")
	}
	if result.Prompt != "PRを作成してください。" {
		t.Fatalf("expected prompt 'PRを作成してください。', got %q", result.Prompt)
	}
}

func TestSlashCommandHandler_InvokeNonExistent(t *testing.T) {
	store := &mockTemplateStore{
		findByNameFn: func(name string) (*database.TemplateRecord, error) {
			return nil, nil
		},
	}
	handler := NewSlashCommandHandlerWithStore(store)
	result := handler.HandleCommand("template", []string{"存在しない"})
	if result.Success {
		t.Fatal("expected Success to be false")
	}
	if !strings.Contains(result.Message, "not found") {
		t.Fatalf("expected message to contain 'not found', got %q", result.Message)
	}
}

func TestSlashCommandHandler_AddTemplate(t *testing.T) {
	var createdInput database.CreateTemplateInput
	store := &mockTemplateStore{
		createFn: func(input database.CreateTemplateInput) (database.TemplateRecord, error) {
			createdInput = input
			return database.TemplateRecord{ID: 1, Name: input.Name, Prompt: input.Prompt}, nil
		},
	}
	handler := NewSlashCommandHandlerWithStore(store)
	result := handler.HandleCommand("template", []string{"add", "テスト実行", "テストを実行して"})
	if !result.Success {
		t.Fatal("expected Success to be true")
	}
	if createdInput.Name != "テスト実行" || createdInput.Prompt != "テストを実行して" {
		t.Fatalf("expected input to match, got %+v", createdInput)
	}
}

func TestSlashCommandHandler_AddMissingArgs(t *testing.T) {
	handler := NewSlashCommandHandlerWithStore(&mockTemplateStore{})
	result := handler.HandleCommand("template", []string{"add"})
	if result.Success {
		t.Fatal("expected Success to be false")
	}
	if !strings.Contains(result.Message, "Missing arguments") {
		t.Fatalf("expected message to contain 'Missing arguments', got %q", result.Message)
	}
}

func TestSlashCommandHandler_AddDuplicatedError(t *testing.T) {
	store := &mockTemplateStore{
		createFn: func(input database.CreateTemplateInput) (database.TemplateRecord, error) {
			return database.TemplateRecord{}, errors.New("duplicate name")
		},
	}
	handler := NewSlashCommandHandlerWithStore(store)
	result := handler.HandleCommand("template", []string{"add", "dup", "prompt"})
	if result.Success {
		t.Fatal("expected Success to be false")
	}
	expected := i18n.T("⚠️ Failed to register template. Name might be duplicated.")
	if !strings.Contains(result.Message, expected) && !strings.Contains(result.Message, "might be duplicated") {
		t.Fatalf("expected duplicate error message, got %q", result.Message)
	}
}

func TestSlashCommandHandler_DeleteTemplate(t *testing.T) {
	store := &mockTemplateStore{
		deleteByNameFn: func(name string) (bool, error) {
			if name == "テスト実行" {
				return true, nil
			}
			return false, nil
		},
	}
	handler := NewSlashCommandHandlerWithStore(store)

	// Existing template
	result := handler.HandleCommand("template", []string{"delete", "テスト実行"})
	if !result.Success {
		t.Fatal("expected Success to be true")
	}

	// Non-existent template
	resultNonExistent := handler.HandleCommand("template", []string{"delete", "存在しない"})
	if resultNonExistent.Success {
		t.Fatal("expected Success to be false for non-existent template")
	}

	// Missing args for delete
	resultMissing := handler.HandleCommand("template", []string{"delete"})
	if resultMissing.Success {
		t.Fatal("expected Success to be false for missing args")
	}
}
