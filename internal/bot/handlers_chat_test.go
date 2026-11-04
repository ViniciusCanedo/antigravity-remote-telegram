package bot

import (
	"testing"

	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/workspace"
)

func TestChatCommandHandler_GettersAndOptionalPool(t *testing.T) {
	sessionService := &cdp.ChatSessionService{}
	sessionRepo := &database.ChatSessionRepository{}
	bindingRepo := &database.WorkspaceBindingRepository{}
	workspaceService := workspace.NewWorkspaceService("/tmp")

	// Test with pool nil
	handlerWithoutPool := NewChatCommandHandler(
		sessionService,
		sessionRepo,
		bindingRepo,
		workspaceService,
	)

	if handlerWithoutPool.GetSessionService() != sessionService {
		t.Fatal("expected GetSessionService to return sessionService")
	}
	if handlerWithoutPool.GetSessionRepo() != sessionRepo {
		t.Fatal("expected GetSessionRepo to return sessionRepo")
	}
	if handlerWithoutPool.pool != nil {
		t.Fatal("expected pool to be nil")
	}

	// Test with optional pool provided
	pool := cdp.NewConnectionPool(cdp.ClientOptions{})
	handlerWithPool := NewChatCommandHandler(
		sessionService,
		sessionRepo,
		bindingRepo,
		workspaceService,
		pool,
	)

	if handlerWithPool.pool != pool {
		t.Fatal("expected pool to match provided pool")
	}
}
