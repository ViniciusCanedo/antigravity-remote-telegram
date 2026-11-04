package bot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type mockForumTopicAPI struct {
	mu           sync.Mutex
	createCalls  int
	createdName  string
	createdChat  int64
	createResult int
	createErr    error

	editCalls int
	editErr   error

	closeCalls int
	closeErr   error

	deleteCalls int
	deleteErr   error
}

func (m *mockForumTopicAPI) CreateForumTopic(ctx context.Context, chatID int64, name string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	m.createdChat = chatID
	m.createdName = name
	return m.createResult, m.createErr
}

func (m *mockForumTopicAPI) EditForumTopic(ctx context.Context, chatID int64, messageThreadID int, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.editCalls++
	return m.editErr
}

func (m *mockForumTopicAPI) CloseForumTopic(ctx context.Context, chatID int64, messageThreadID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalls++
	return m.closeErr
}

func (m *mockForumTopicAPI) DeleteForumTopic(ctx context.Context, chatID int64, messageThreadID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls++
	return m.deleteErr
}

func TestSanitizeName(t *testing.T) {
	mgr := NewTelegramTopicManager(nil, 123)

	cases := []struct {
		input    string
		expected string
	}{
		{"my-repo/", "my-repo"},
		{"a/b/c", "a-b-c"},
		{"a///b", "a-b"},
		{"---a-b---", "a-b"},
		{"///project///", "project"},
		{"normal-name", "normal-name"},
	}

	for _, c := range cases {
		actual := mgr.SanitizeName(c.input)
		if actual != c.expected {
			t.Errorf("SanitizeName(%q) = %q, expected %q", c.input, actual, c.expected)
		}
	}

	// Test truncation to 128 characters
	longName := strings.Repeat("a", 150)
	sanitizedLong := mgr.SanitizeName(longName)
	if len([]rune(sanitizedLong)) != 128 {
		t.Errorf("expected 128 runes, got %d", len([]rune(sanitizedLong)))
	}
}

func TestEnsureTopic_CacheHitAndMiss(t *testing.T) {
	ctx := context.Background()
	api := &mockForumTopicAPI{
		createResult: 100,
	}
	mgr := NewTelegramTopicManager(api, 123456)

	// First call - cache miss
	res1, err := mgr.EnsureTopic(ctx, "my-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res1.Created {
		t.Errorf("expected Created: true on first call")
	}
	if res1.TopicID != 100 {
		t.Errorf("expected TopicID: 100, got %d", res1.TopicID)
	}
	if api.createCalls != 1 {
		t.Errorf("expected 1 create call, got %d", api.createCalls)
	}
	if api.createdName != "🗂️ my-repo" {
		t.Errorf("expected topic name '🗂️ my-repo', got %q", api.createdName)
	}
	if api.createdChat != 123456 {
		t.Errorf("expected chatID 123456, got %d", api.createdChat)
	}

	// Second call - cache hit
	res2, err := mgr.EnsureTopic(ctx, "my-repo")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if res2.Created {
		t.Errorf("expected Created: false on second call")
	}
	if res2.TopicID != 100 {
		t.Errorf("expected TopicID: 100, got %d", res2.TopicID)
	}
	if api.createCalls != 1 {
		t.Errorf("expected still 1 create call after cache hit, got %d", api.createCalls)
	}
}

func TestEnsureTopic_Error(t *testing.T) {
	ctx := context.Background()
	api := &mockForumTopicAPI{
		createErr: errors.New("network failure"),
	}
	mgr := NewTelegramTopicManager(api, 123456)

	res, err := mgr.EnsureTopic(ctx, "fail-repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if res.TopicID != 0 {
		t.Errorf("expected TopicID 0 on error, got %d", res.TopicID)
	}
}

func TestCreateSessionTopic(t *testing.T) {
	ctx := context.Background()
	api := &mockForumTopicAPI{
		createResult: 200,
	}
	mgr := NewTelegramTopicManager(api, 123456)

	topicID, err := mgr.CreateSessionTopic(ctx, "1-fix-bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if topicID != 200 {
		t.Errorf("expected topicID 200, got %d", topicID)
	}
	if api.createdName != "💬 1-fix-bug" {
		t.Errorf("expected topic name '💬 1-fix-bug', got %q", api.createdName)
	}

	// Error case
	api.createErr = errors.New("cannot create session topic")
	_, err = mgr.CreateSessionTopic(ctx, "err-session")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTopicManager_RenameCloseDelete(t *testing.T) {
	ctx := context.Background()
	api := &mockForumTopicAPI{
		editErr:   errors.New("edit error"),
		closeErr:  errors.New("close error"),
		deleteErr: errors.New("delete error"),
	}
	mgr := NewTelegramTopicManager(api, 123456)

	// Verify methods suppress errors without panicking
	mgr.RenameTopic(ctx, 10, "new-name")
	mgr.CloseTopic(ctx, 10)
	mgr.DeleteTopic(ctx, 10)

	if api.editCalls != 1 {
		t.Errorf("expected 1 edit call, got %d", api.editCalls)
	}
	if api.closeCalls != 1 {
		t.Errorf("expected 1 close call, got %d", api.closeCalls)
	}
	if api.deleteCalls != 1 {
		t.Errorf("expected 1 delete call, got %d", api.deleteCalls)
	}
}

func TestTopicManager_RegisterAndGetTopicID(t *testing.T) {
	mgr := NewTelegramTopicManager(nil, 123456)

	mgr.RegisterTopic("ws1", 42)

	id, ok := mgr.GetTopicID("ws1")
	if !ok || id != 42 {
		t.Errorf("expected (42, true), got (%d, %v)", id, ok)
	}

	id2, ok2 := mgr.GetTopicID("unknown")
	if ok2 || id2 != 0 {
		t.Errorf("expected (0, false), got (%d, %v)", id2, ok2)
	}

	mgr.SetChatID(999999)
	// Trigger EnsureTopic with mock API to confirm updated chatID
	api := &mockForumTopicAPI{createResult: 300}
	mgr.api = api
	_, _ = mgr.EnsureTopic(context.Background(), "ws2")
	if api.createdChat != 999999 {
		t.Errorf("expected updated chatID 999999, got %d", api.createdChat)
	}
}
