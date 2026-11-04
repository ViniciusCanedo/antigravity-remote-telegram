package database

import (
	"testing"
)

func TestChatSessionRepo_TableInitialization(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='chat_sessions'").Scan(&tableName)
	if err != nil {
		t.Fatalf("expected chat_sessions table to exist: %v", err)
	}
	if tableName != "chat_sessions" {
		t.Errorf("expected table name chat_sessions, got %q", tableName)
	}
}

func TestChatSessionRepo_Create_ReturnsIDAndDefaults(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	result, err := repo.Create(CreateChatSessionInput{
		ChannelID:     "ch-1",
		CategoryID:    "cat-1",
		WorkspacePath: "my-project",
		SessionNumber: 1,
		GuildID:       "guild-1",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if result.ID <= 0 {
		t.Errorf("expected positive ID, got %d", result.ID)
	}
	if result.ChannelID != "ch-1" {
		t.Errorf("expected channelId ch-1, got %q", result.ChannelID)
	}
	if result.CategoryID != "cat-1" {
		t.Errorf("expected categoryId cat-1, got %q", result.CategoryID)
	}
	if result.WorkspacePath != "my-project" {
		t.Errorf("expected workspacePath my-project, got %q", result.WorkspacePath)
	}
	if result.SessionNumber != 1 {
		t.Errorf("expected sessionNumber 1, got %d", result.SessionNumber)
	}
	if result.DisplayName != nil {
		t.Errorf("expected displayName nil, got %v", result.DisplayName)
	}
	if result.IsRenamed != false {
		t.Errorf("expected isRenamed false, got %v", result.IsRenamed)
	}
	if result.GuildID != "guild-1" {
		t.Errorf("expected guildId guild-1, got %q", result.GuildID)
	}
}

func TestChatSessionRepo_Create_DuplicateChannelIDThrows(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, err = repo.Create(CreateChatSessionInput{
		ChannelID:     "ch-1",
		CategoryID:    "cat-1",
		WorkspacePath: "proj",
		SessionNumber: 1,
		GuildID:       "guild-1",
	})
	if err != nil {
		t.Fatalf("first Create() failed: %v", err)
	}

	_, err = repo.Create(CreateChatSessionInput{
		ChannelID:     "ch-1",
		CategoryID:    "cat-1",
		WorkspacePath: "proj",
		SessionNumber: 2,
		GuildID:       "guild-1",
	})
	if err == nil {
		t.Fatalf("expected error on duplicate channel ID, got nil")
	}
}

func TestChatSessionRepo_FindByChannelId_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, err = repo.Create(CreateChatSessionInput{
		ChannelID:     "ch-1",
		CategoryID:    "cat-1",
		WorkspacePath: "proj",
		SessionNumber: 1,
		GuildID:       "guild-1",
	})
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	found, err := repo.FindByChannelId("ch-1")
	if err != nil {
		t.Fatalf("FindByChannelId() failed: %v", err)
	}
	if found == nil {
		t.Fatalf("expected found record, got nil")
	}
	if found.WorkspacePath != "proj" {
		t.Errorf("expected workspacePath proj, got %q", found.WorkspacePath)
	}
	if found.IsRenamed != false {
		t.Errorf("expected isRenamed false, got %v", found.IsRenamed)
	}
}

func TestChatSessionRepo_FindByChannelId_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
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

func TestChatSessionRepo_FindByCategoryId_SortedBySessionNumber(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-1", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 1, GuildID: "guild-1"})
	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-2", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 2, GuildID: "guild-1"})
	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-3", CategoryID: "cat-2", WorkspacePath: "other", SessionNumber: 1, GuildID: "guild-1"})

	results, err := repo.FindByCategoryId("cat-1")
	if err != nil {
		t.Fatalf("FindByCategoryId() failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].SessionNumber != 1 {
		t.Errorf("expected first session number 1, got %d", results[0].SessionNumber)
	}
	if results[1].SessionNumber != 2 {
		t.Errorf("expected second session number 2, got %d", results[1].SessionNumber)
	}
}

func TestChatSessionRepo_FindByCategoryId_Empty(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	results, err := repo.FindByCategoryId("nonexistent")
	if err != nil {
		t.Fatalf("FindByCategoryId() failed: %v", err)
	}
	if results == nil {
		t.Fatalf("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected empty array, got %d items", len(results))
	}
}

func TestChatSessionRepo_GetNextSessionNumber_EmptyReturns1(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	next, err := repo.GetNextSessionNumber("cat-1")
	if err != nil {
		t.Fatalf("GetNextSessionNumber() failed: %v", err)
	}
	if next != 1 {
		t.Errorf("expected 1, got %d", next)
	}
}

func TestChatSessionRepo_GetNextSessionNumber_ReturnsMaxPlus1(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-1", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 1, GuildID: "guild-1"})
	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-2", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 3, GuildID: "guild-1"})

	next, err := repo.GetNextSessionNumber("cat-1")
	if err != nil {
		t.Fatalf("GetNextSessionNumber() failed: %v", err)
	}
	if next != 4 {
		t.Errorf("expected 4, got %d", next)
	}
}

func TestChatSessionRepo_GetNextSessionNumber_IsolatedPerCategory(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-1", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 5, GuildID: "guild-1"})

	next, err := repo.GetNextSessionNumber("cat-2")
	if err != nil {
		t.Fatalf("GetNextSessionNumber() failed: %v", err)
	}
	if next != 1 {
		t.Errorf("expected 1, got %d", next)
	}
}

func TestChatSessionRepo_UpdateDisplayName_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-1", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 1, GuildID: "guild-1"})

	updated, err := repo.UpdateDisplayName("ch-1", "React認証バグ修正")
	if err != nil {
		t.Fatalf("UpdateDisplayName() failed: %v", err)
	}
	if !updated {
		t.Errorf("expected updated to be true")
	}

	found, err := repo.FindByChannelId("ch-1")
	if err != nil {
		t.Fatalf("FindByChannelId() failed: %v", err)
	}
	if found == nil || found.DisplayName == nil || *found.DisplayName != "React認証バグ修正" {
		t.Errorf("expected displayName 'React認証バグ修正', got %v", found)
	}
	if !found.IsRenamed {
		t.Errorf("expected isRenamed to be true")
	}
}

func TestChatSessionRepo_UpdateDisplayName_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	updated, err := repo.UpdateDisplayName("nonexistent", "title")
	if err != nil {
		t.Fatalf("UpdateDisplayName() error: %v", err)
	}
	if updated {
		t.Errorf("expected false for non-existent channel ID")
	}
}

func TestChatSessionRepo_FindByDisplayName(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-1", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 1, GuildID: "guild-1"})
	_, _ = repo.UpdateDisplayName("ch-1", "Feature X")

	found, err := repo.FindByDisplayName("proj", "Feature X")
	if err != nil {
		t.Fatalf("FindByDisplayName() failed: %v", err)
	}
	if found == nil {
		t.Fatalf("expected found record, got nil")
	}
	if found.ChannelID != "ch-1" {
		t.Errorf("expected channelId ch-1, got %q", found.ChannelID)
	}

	notFound, err := repo.FindByDisplayName("proj", "NonExistent")
	if err != nil {
		t.Fatalf("FindByDisplayName() error: %v", err)
	}
	if notFound != nil {
		t.Errorf("expected nil for non-existent display name, got %+v", notFound)
	}
}

func TestChatSessionRepo_DeleteByChannelId_Existing(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
	}
	defer repo.Close()

	_, _ = repo.Create(CreateChatSessionInput{ChannelID: "ch-1", CategoryID: "cat-1", WorkspacePath: "proj", SessionNumber: 1, GuildID: "guild-1"})

	deleted, err := repo.DeleteByChannelId("ch-1")
	if err != nil {
		t.Fatalf("DeleteByChannelId() failed: %v", err)
	}
	if !deleted {
		t.Errorf("expected deleted to be true")
	}

	found, err := repo.FindByChannelId("ch-1")
	if err != nil {
		t.Fatalf("FindByChannelId() error: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil after deletion, got %+v", found)
	}
}

func TestChatSessionRepo_DeleteByChannelId_NonExistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	defer db.Close()

	repo, err := NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("NewChatSessionRepository() failed: %v", err)
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

func TestChatSessionRepo_ClosedDBError(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory() failed: %v", err)
	}
	db.Close()

	_, err = NewChatSessionRepository(db)
	if err == nil {
		t.Fatalf("expected error when creating repository on closed db, got nil")
	}
}
