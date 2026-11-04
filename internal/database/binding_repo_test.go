package database

import (
	"testing"
)

func TestWorkspaceBindingRepo_TableInitialization(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='workspace_bindings'").Scan(&tableName)
	if err != nil {
		t.Fatalf("expected workspace_bindings table to exist: %v", err)
	}
	if tableName != "workspace_bindings" {
		t.Errorf("expected table name workspace_bindings, got %q", tableName)
	}
}

func TestWorkspaceBindingRepo_Create_ReturnsID(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	result, err := repo.Create(CreateWorkspaceBindingInput{
		ChannelID:     "123456",
		WorkspacePath: "my-project",
		GuildID:       "guild-1",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if result.ID <= 0 {
		t.Errorf("expected positive ID, got %d", result.ID)
	}
	if result.ChannelID != "123456" {
		t.Errorf("expected channelId 123456, got %q", result.ChannelID)
	}
	if result.WorkspacePath != "my-project" {
		t.Errorf("expected workspacePath my-project, got %q", result.WorkspacePath)
	}
	if result.GuildID != "guild-1" {
		t.Errorf("expected guildId guild-1, got %q", result.GuildID)
	}
}

func TestWorkspaceBindingRepo_Create_DuplicateChannelIDThrows(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	_, err = repo.Create(CreateWorkspaceBindingInput{
		ChannelID:     "123456",
		WorkspacePath: "proj-a",
		GuildID:       "guild-1",
	})
	if err != nil {
		t.Fatalf("first Create() failed: %v", err)
	}

	_, err = repo.Create(CreateWorkspaceBindingInput{
		ChannelID:     "123456",
		WorkspacePath: "proj-b",
		GuildID:       "guild-1",
	})
	if err == nil {
		t.Fatalf("expected error on duplicate channel ID, got nil")
	}
}

func TestWorkspaceBindingRepo_FindByChannelId_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "123456", WorkspacePath: "my-project", GuildID: "guild-1"})

	found, err := repo.FindByChannelId("123456")
	if err != nil {
		t.Fatalf("FindByChannelId() failed: %v", err)
	}
	if found == nil {
		t.Fatalf("expected found record, got nil")
	}
	if found.WorkspacePath != "my-project" {
		t.Errorf("expected workspacePath my-project, got %q", found.WorkspacePath)
	}
}

func TestWorkspaceBindingRepo_FindByChannelId_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	found, err := repo.FindByChannelId("nonexistent")
	if err != nil {
		t.Fatalf("FindByChannelId() error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil for non-existent channel ID, got %+v", found)
	}
}

func TestWorkspaceBindingRepo_FindByGuildId_RetrievesAllForGuild(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "111", WorkspacePath: "proj-a", GuildID: "guild-1"})
	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "222", WorkspacePath: "proj-b", GuildID: "guild-1"})
	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "333", WorkspacePath: "proj-c", GuildID: "guild-2"})

	results, err := repo.FindByGuildId("guild-1")
	if err != nil {
		t.Fatalf("FindByGuildId() failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].WorkspacePath != "proj-a" {
		t.Errorf("expected first workspacePath proj-a, got %q", results[0].WorkspacePath)
	}
	if results[1].WorkspacePath != "proj-b" {
		t.Errorf("expected second workspacePath proj-b, got %q", results[1].WorkspacePath)
	}
}

func TestWorkspaceBindingRepo_FindByGuildId_Empty(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	results, err := repo.FindByGuildId("nonexistent")
	if err != nil {
		t.Fatalf("FindByGuildId() failed: %v", err)
	}
	if results == nil {
		t.Fatalf("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d items", len(results))
	}
}

func TestWorkspaceBindingRepo_FindByWorkspacePathAndGuildId(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "111", WorkspacePath: "proj-a", GuildID: "guild-1"})
	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "222", WorkspacePath: "proj-a", GuildID: "guild-1"})
	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "333", WorkspacePath: "proj-b", GuildID: "guild-1"})

	results, err := repo.FindByWorkspacePathAndGuildId("proj-a", "guild-1")
	if err != nil {
		t.Fatalf("FindByWorkspacePathAndGuildId() failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestWorkspaceBindingRepo_FindAll_RetrievesAll(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "111", WorkspacePath: "proj-a", GuildID: "guild-1"})
	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "222", WorkspacePath: "proj-b", GuildID: "guild-2"})

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}
}

func TestWorkspaceBindingRepo_FindAll_Empty(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
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

func TestWorkspaceBindingRepo_DeleteByChannelId_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "123456", WorkspacePath: "my-project", GuildID: "guild-1"})

	deleted, err := repo.DeleteByChannelId("123456")
	if err != nil {
		t.Fatalf("DeleteByChannelId() failed: %v", err)
	}
	if !deleted {
		t.Errorf("expected deleted to be true")
	}

	found, err := repo.FindByChannelId("123456")
	if err != nil {
		t.Fatalf("FindByChannelId() error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil after deletion, got %+v", found)
	}
}

func TestWorkspaceBindingRepo_DeleteByChannelId_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	deleted, err := repo.DeleteByChannelId("nonexistent")
	if err != nil {
		t.Fatalf("DeleteByChannelId() error: %v", err)
	}
	if deleted {
		t.Errorf("expected false when deleting non-existent channel ID")
	}
}

func TestWorkspaceBindingRepo_Upsert_CreatesWhenNotExist(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	result, err := repo.Upsert(CreateWorkspaceBindingInput{
		ChannelID:     "123456",
		WorkspacePath: "proj-a",
		GuildID:       "guild-1",
	})
	if err != nil {
		t.Fatalf("Upsert() failed: %v", err)
	}
	if result.ChannelID != "123456" {
		t.Errorf("expected channelId 123456, got %q", result.ChannelID)
	}
	if result.WorkspacePath != "proj-a" {
		t.Errorf("expected workspacePath proj-a, got %q", result.WorkspacePath)
	}
}

func TestWorkspaceBindingRepo_Upsert_UpdatesWhenExists(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("NewWorkspaceBindingRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateWorkspaceBindingInput{ChannelID: "123456", WorkspacePath: "proj-a", GuildID: "guild-1"})

	result, err := repo.Upsert(CreateWorkspaceBindingInput{
		ChannelID:     "123456",
		WorkspacePath: "proj-b",
		GuildID:       "guild-1",
	})
	if err != nil {
		t.Fatalf("Upsert() failed: %v", err)
	}
	if result.WorkspacePath != "proj-b" {
		t.Errorf("expected workspacePath proj-b, got %q", result.WorkspacePath)
	}

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 item in total, got %d", len(all))
	}
}

func TestWorkspaceBindingRepo_ClosedDBError(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	db.Close()

	_, err = NewWorkspaceBindingRepository(db)
	if err == nil {
		t.Fatalf("expected error when creating repository on closed db, got nil")
	}
}
