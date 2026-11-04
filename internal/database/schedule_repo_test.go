package database

import (
	"testing"
)

func TestScheduleRepo_TableInitialization(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schedules'").Scan(&tableName)
	if err != nil {
		t.Fatalf("expected schedules table to exist: %v", err)
	}
	if tableName != "schedules" {
		t.Errorf("expected table name schedules, got %q", tableName)
	}
}

func TestScheduleRepo_Create_SavesScheduleAndReturnsID(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	record, err := repo.Create(CreateScheduleInput{
		CronExpression: "0 9 * * *",
		Prompt:         "テストプロンプト: ビルドを実行",
		WorkspacePath:  "/home/user/project",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if record.ID <= 0 {
		t.Errorf("expected positive ID, got %d", record.ID)
	}
	if record.CronExpression != "0 9 * * *" {
		t.Errorf("expected cronExpression '0 9 * * *', got %q", record.CronExpression)
	}
	if record.Prompt != "テストプロンプト: ビルドを実行" {
		t.Errorf("expected prompt 'テストプロンプト: ビルドを実行', got %q", record.Prompt)
	}
	if record.WorkspacePath != "/home/user/project" {
		t.Errorf("expected workspacePath '/home/user/project', got %q", record.WorkspacePath)
	}
	if !record.Enabled {
		t.Errorf("expected enabled to be true")
	}
}

func TestScheduleRepo_Create_MultipleSchedules(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateScheduleInput{
		CronExpression: "0 9 * * *",
		Prompt:         "朝のビルド",
		WorkspacePath:  "/project1",
		Enabled:        true,
	})
	_, _ = repo.Create(CreateScheduleInput{
		CronExpression: "0 18 * * *",
		Prompt:         "夕方のテスト",
		WorkspacePath:  "/project2",
		Enabled:        true,
	})

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(all))
	}
}

func TestScheduleRepo_FindAll_RetrievesAll(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateScheduleInput{
		CronExpression: "*/5 * * * *",
		Prompt:         "5分毎のヘルスチェック",
		WorkspacePath:  "/service",
		Enabled:        true,
	})
	_, _ = repo.Create(CreateScheduleInput{
		CronExpression: "0 0 * * 0",
		Prompt:         "毎週日曜の定期メンテナンス",
		WorkspacePath:  "/maintenance",
		Enabled:        false,
	})

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 schedules, got %d", len(all))
	}
	if all[0].Prompt != "5分毎のヘルスチェック" {
		t.Errorf("expected first prompt '5分毎のヘルスチェック', got %q", all[0].Prompt)
	}
	if all[1].Prompt != "毎週日曜の定期メンテナンス" {
		t.Errorf("expected second prompt '毎週日曜の定期メンテナンス', got %q", all[1].Prompt)
	}
	if all[1].Enabled != false {
		t.Errorf("expected second enabled to be false")
	}
}

func TestScheduleRepo_FindAll_Empty(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
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

func TestScheduleRepo_FindById_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	created, err := repo.Create(CreateScheduleInput{
		CronExpression: "30 12 * * *",
		Prompt:         "お昼のリマインダー",
		WorkspacePath:  "/reminder",
		Enabled:        true,
	})
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
	if found.CronExpression != "30 12 * * *" {
		t.Errorf("expected cronExpression '30 12 * * *', got %q", found.CronExpression)
	}
	if found.Prompt != "お昼のリマインダー" {
		t.Errorf("expected prompt 'お昼のリマインダー', got %q", found.Prompt)
	}
}

func TestScheduleRepo_FindById_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	found, err := repo.FindById(9999)
	if err != nil {
		t.Fatalf("FindById() error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil for non-existent ID, got %+v", found)
	}
}

func TestScheduleRepo_FindEnabled_RetrievesOnlyEnabled(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateScheduleInput{
		CronExpression: "0 9 * * *",
		Prompt:         "有効なジョブ",
		WorkspacePath:  "/active",
		Enabled:        true,
	})
	_, _ = repo.Create(CreateScheduleInput{
		CronExpression: "0 18 * * *",
		Prompt:         "無効なジョブ",
		WorkspacePath:  "/inactive",
		Enabled:        false,
	})

	enabled, err := repo.FindEnabled()
	if err != nil {
		t.Fatalf("FindEnabled() failed: %v", err)
	}
	if len(enabled) != 1 {
		t.Fatalf("expected 1 enabled schedule, got %d", len(enabled))
	}
	if enabled[0].Prompt != "有効なジョブ" {
		t.Errorf("expected prompt '有効なジョブ', got %q", enabled[0].Prompt)
	}
}

func TestScheduleRepo_Delete_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	created, err := repo.Create(CreateScheduleInput{
		CronExpression: "0 0 * * *",
		Prompt:         "削除対象",
		WorkspacePath:  "/delete-me",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	deleted, err := repo.Delete(created.ID)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	if !deleted {
		t.Errorf("expected deleted to be true")
	}

	found, err := repo.FindById(created.ID)
	if err != nil {
		t.Fatalf("FindById() error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil after deletion, got %+v", found)
	}
}

func TestScheduleRepo_Delete_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	deleted, err := repo.Delete(9999)
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	if deleted {
		t.Errorf("expected false when deleting non-existent ID")
	}
}

func TestScheduleRepo_Update_ToggleEnabled(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	created, err := repo.Create(CreateScheduleInput{
		CronExpression: "0 9 * * *",
		Prompt:         "更新対象",
		WorkspacePath:  "/update-me",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	disabled := false
	updated, err := repo.Update(created.ID, UpdateScheduleInput{Enabled: &disabled})
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}
	if !updated {
		t.Errorf("expected updated to be true")
	}

	found, err := repo.FindById(created.ID)
	if err != nil {
		t.Fatalf("FindById() failed: %v", err)
	}
	if found == nil || found.Enabled != false {
		t.Errorf("expected enabled to be false, got %+v", found)
	}
}

func TestScheduleRepo_Update_CronExpression(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	created, err := repo.Create(CreateScheduleInput{
		CronExpression: "0 9 * * *",
		Prompt:         "テスト",
		WorkspacePath:  "/test",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	newCron := "0 12 * * *"
	updated, err := repo.Update(created.ID, UpdateScheduleInput{CronExpression: &newCron})
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}
	if !updated {
		t.Errorf("expected updated to be true")
	}

	found, err := repo.FindById(created.ID)
	if err != nil {
		t.Fatalf("FindById() failed: %v", err)
	}
	if found == nil || found.CronExpression != "0 12 * * *" {
		t.Errorf("expected cronExpression '0 12 * * *', got %+v", found)
	}
}

func TestScheduleRepo_Update_MultipleFields(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	created, err := repo.Create(CreateScheduleInput{
		CronExpression: "0 9 * * *",
		Prompt:         "初期プロンプト",
		WorkspacePath:  "/old-path",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	newPrompt := "更新後プロンプト"
	newPath := "/new-path"
	updated, err := repo.Update(created.ID, UpdateScheduleInput{
		Prompt:        &newPrompt,
		WorkspacePath: &newPath,
	})
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}
	if !updated {
		t.Errorf("expected updated to be true")
	}

	found, err := repo.FindById(created.ID)
	if err != nil {
		t.Fatalf("FindById() failed: %v", err)
	}
	if found == nil || found.Prompt != "更新後プロンプト" || found.WorkspacePath != "/new-path" {
		t.Errorf("expected updated prompt and path, got %+v", found)
	}
}

func TestScheduleRepo_Update_EmptyInput(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	created, err := repo.Create(CreateScheduleInput{
		CronExpression: "0 9 * * *",
		Prompt:         "テスト",
		WorkspacePath:  "/test",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	updated, err := repo.Update(created.ID, UpdateScheduleInput{})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated {
		t.Errorf("expected false when updating with empty input")
	}
}

func TestScheduleRepo_Update_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewScheduleRepository(db)
	if err != nil {
		t.Fatalf("NewScheduleRepository() failed: %v", err)
	}
	defer repo.Close()

	newCron := "0 12 * * *"
	updated, err := repo.Update(9999, UpdateScheduleInput{CronExpression: &newCron})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated {
		t.Errorf("expected false when updating non-existent schedule")
	}
}

func TestScheduleRepo_ClosedDBError(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	db.Close()

	_, err = NewScheduleRepository(db)
	if err == nil {
		t.Fatalf("expected error when creating repository on closed db, got nil")
	}
}
