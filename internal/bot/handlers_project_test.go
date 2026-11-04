package bot

import (
	"testing"

	"antigravity-telegram-remote/internal/database"
)

type mockWorkspaceBindingStore struct {
	findByChannelIdFn func(channelID string) (*database.WorkspaceBindingRecord, error)
}

func (m *mockWorkspaceBindingStore) FindByChannelId(channelID string) (*database.WorkspaceBindingRecord, error) {
	if m.findByChannelIdFn != nil {
		return m.findByChannelIdFn(channelID)
	}
	return nil, nil
}

type mockWorkspacePathResolver struct {
	getWorkspacePathFn func(workspaceName string) (string, error)
}

func (m *mockWorkspacePathResolver) GetWorkspacePath(workspaceName string) (string, error) {
	if m.getWorkspacePathFn != nil {
		return m.getWorkspacePathFn(workspaceName)
	}
	return "", nil
}

func TestWorkspaceCommandHandler_GetWorkspaceForChannel_Found(t *testing.T) {
	bindingStore := &mockWorkspaceBindingStore{
		findByChannelIdFn: func(channelID string) (*database.WorkspaceBindingRecord, error) {
			if channelID == "channel-1" {
				return &database.WorkspaceBindingRecord{
					WorkspacePath: "my-project",
				}, nil
			}
			return nil, nil
		},
	}
	resolver := &mockWorkspacePathResolver{
		getWorkspacePathFn: func(workspaceName string) (string, error) {
			if workspaceName == "my-project" {
				return "/home/user/my-project", nil
			}
			return "", nil
		},
	}

	handler := NewWorkspaceCommandHandlerWithInterfaces(bindingStore, nil, resolver)
	path, ok := handler.GetWorkspaceForChannel("channel-1")
	if !ok {
		t.Fatal("expected ok to be true")
	}
	if path != "/home/user/my-project" {
		t.Fatalf("expected '/home/user/my-project', got %q", path)
	}
}

func TestWorkspaceCommandHandler_GetWorkspaceForChannel_NotFound(t *testing.T) {
	calledResolver := false
	bindingStore := &mockWorkspaceBindingStore{
		findByChannelIdFn: func(channelID string) (*database.WorkspaceBindingRecord, error) {
			return nil, nil
		},
	}
	resolver := &mockWorkspacePathResolver{
		getWorkspacePathFn: func(workspaceName string) (string, error) {
			calledResolver = true
			return "", nil
		},
	}

	handler := NewWorkspaceCommandHandlerWithInterfaces(bindingStore, nil, resolver)
	path, ok := handler.GetWorkspaceForChannel("unknown-channel")
	if ok {
		t.Fatal("expected ok to be false")
	}
	if path != "" {
		t.Fatalf("expected empty path, got %q", path)
	}
	if calledResolver {
		t.Fatal("expected workspace resolver not to be called when binding is not found")
	}
}
