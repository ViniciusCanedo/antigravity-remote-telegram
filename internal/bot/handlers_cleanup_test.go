package bot

import (
	"errors"
	"testing"
	"time"

	"antigravity-telegram-remote/internal/database"
)

type mockCleanupSessionStore struct {
	deleteByChannelIdFn func(channelID string) (bool, error)
	findByChannelIdFn   func(channelID string) (*database.ChatSessionRecord, error)
}

func (m *mockCleanupSessionStore) DeleteByChannelId(channelID string) (bool, error) {
	if m.deleteByChannelIdFn != nil {
		return m.deleteByChannelIdFn(channelID)
	}
	return false, nil
}

func (m *mockCleanupSessionStore) FindByChannelId(channelID string) (*database.ChatSessionRecord, error) {
	if m.findByChannelIdFn != nil {
		return m.findByChannelIdFn(channelID)
	}
	return nil, nil
}

type mockCleanupBindingStore struct {
	deleteByChannelIdFn func(channelID string) (bool, error)
	findByGuildIdFn     func(guildID string) ([]database.WorkspaceBindingRecord, error)
}

func (m *mockCleanupBindingStore) DeleteByChannelId(channelID string) (bool, error) {
	if m.deleteByChannelIdFn != nil {
		return m.deleteByChannelIdFn(channelID)
	}
	return false, nil
}

func (m *mockCleanupBindingStore) FindByGuildId(guildID string) ([]database.WorkspaceBindingRecord, error) {
	if m.findByGuildIdFn != nil {
		return m.findByGuildIdFn(guildID)
	}
	return nil, nil
}

func TestCleanupButtonConstants(t *testing.T) {
	if CleanupArchiveBtn != "cleanup_archive" {
		t.Fatalf("expected CleanupArchiveBtn 'cleanup_archive', got %q", CleanupArchiveBtn)
	}
	if CleanupDeleteBtn != "cleanup_delete" {
		t.Fatalf("expected CleanupDeleteBtn 'cleanup_delete', got %q", CleanupDeleteBtn)
	}
	if CleanupCancelBtn != "cleanup_cancel" {
		t.Fatalf("expected CleanupCancelBtn 'cleanup_cancel', got %q", CleanupCancelBtn)
	}
}

func TestCleanupByChannelID_Success(t *testing.T) {
	sessionDeleted := ""
	bindingDeleted := ""

	sessionStore := &mockCleanupSessionStore{
		deleteByChannelIdFn: func(channelID string) (bool, error) {
			sessionDeleted = channelID
			return true, nil
		},
	}
	bindingStore := &mockCleanupBindingStore{
		deleteByChannelIdFn: func(channelID string) (bool, error) {
			bindingDeleted = channelID
			return true, nil
		},
	}

	handler := NewCleanupCommandHandlerWithStores(sessionStore, bindingStore)
	err := handler.CleanupByChannelID("channel-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionDeleted != "channel-123" {
		t.Fatalf("expected session delete for 'channel-123', got %q", sessionDeleted)
	}
	if bindingDeleted != "channel-123" {
		t.Fatalf("expected binding delete for 'channel-123', got %q", bindingDeleted)
	}
}

func TestCleanupByChannelID_SessionErrorAbortsBindingDelete(t *testing.T) {
	bindingCalled := false

	sessionStore := &mockCleanupSessionStore{
		deleteByChannelIdFn: func(channelID string) (bool, error) {
			return false, errors.New("DB error")
		},
	}
	bindingStore := &mockCleanupBindingStore{
		deleteByChannelIdFn: func(channelID string) (bool, error) {
			bindingCalled = true
			return true, nil
		},
	}

	handler := NewCleanupCommandHandlerWithStores(sessionStore, bindingStore)
	err := handler.CleanupByChannelID("ch-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if bindingCalled {
		t.Fatal("expected binding repo not to be called when session repo fails")
	}
}

func TestFindInactiveSessions(t *testing.T) {
	oldDate := time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02 15:04:05")
	recentDate := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02 15:04:05")

	bindingStore := &mockCleanupBindingStore{
		findByGuildIdFn: func(guildID string) ([]database.WorkspaceBindingRecord, error) {
			return []database.WorkspaceBindingRecord{
				{ChannelID: "ch-old", WorkspacePath: "old-ws", CreatedAt: oldDate},
				{ChannelID: "ch-recent", WorkspacePath: "recent-ws", CreatedAt: recentDate},
			}, nil
		},
	}

	sessionStore := &mockCleanupSessionStore{
		findByChannelIdFn: func(channelID string) (*database.ChatSessionRecord, error) {
			if channelID == "ch-old" {
				return &database.ChatSessionRecord{
					ChannelID: "ch-old",
					CreatedAt: oldDate,
				}, nil
			}
			return nil, nil
		},
	}

	handler := NewCleanupCommandHandlerWithStores(sessionStore, bindingStore)
	inactive, err := handler.FindInactiveSessions("guild-1", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(inactive) != 1 {
		t.Fatalf("expected 1 inactive session, got %d", len(inactive))
	}
	if inactive[0].Binding.ChannelID != "ch-old" {
		t.Fatalf("expected 'ch-old', got %q", inactive[0].Binding.ChannelID)
	}
}
