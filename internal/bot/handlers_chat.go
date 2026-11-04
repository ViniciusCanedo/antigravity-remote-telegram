package bot

import (
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/workspace"
)

// ChatCommandHandler provides utility methods for chat session operations.
type ChatCommandHandler struct {
	chatSessionService *cdp.ChatSessionService
	chatSessionRepo    *database.ChatSessionRepository
	bindingRepo        *database.WorkspaceBindingRepository
	workspaceService   *workspace.WorkspaceService
	pool               *cdp.ConnectionPool
}

// NewChatCommandHandler creates a new ChatCommandHandler instance.
func NewChatCommandHandler(
	chatSessionService *cdp.ChatSessionService,
	chatSessionRepo *database.ChatSessionRepository,
	bindingRepo *database.WorkspaceBindingRepository,
	workspaceService *workspace.WorkspaceService,
	pool ...*cdp.ConnectionPool,
) *ChatCommandHandler {
	var p *cdp.ConnectionPool
	if len(pool) > 0 {
		p = pool[0]
	}
	return &ChatCommandHandler{
		chatSessionService: chatSessionService,
		chatSessionRepo:    chatSessionRepo,
		bindingRepo:        bindingRepo,
		workspaceService:   workspaceService,
		pool:               p,
	}
}

// GetSessionService returns the ChatSessionService instance.
func (h *ChatCommandHandler) GetSessionService() *cdp.ChatSessionService {
	return h.chatSessionService
}

// GetSessionRepo returns the ChatSessionRepository instance.
func (h *ChatCommandHandler) GetSessionRepo() *database.ChatSessionRepository {
	return h.chatSessionRepo
}
