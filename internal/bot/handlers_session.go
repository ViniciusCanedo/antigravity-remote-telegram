package bot

import (
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/workspace"
)

// JoinCommandHandler provides utility methods for /join and session switching.
type JoinCommandHandler struct {
	chatSessionService *cdp.ChatSessionService
	chatSessionRepo    *database.ChatSessionRepository
	bindingRepo        *database.WorkspaceBindingRepository
	pool               *cdp.ConnectionPool
	workspaceService   workspacePathResolver
}

// NewJoinCommandHandler creates a new JoinCommandHandler.
func NewJoinCommandHandler(
	chatSessionService *cdp.ChatSessionService,
	chatSessionRepo *database.ChatSessionRepository,
	bindingRepo *database.WorkspaceBindingRepository,
	pool *cdp.ConnectionPool,
	workspaceService *workspace.WorkspaceService,
) *JoinCommandHandler {
	return &JoinCommandHandler{
		chatSessionService: chatSessionService,
		chatSessionRepo:    chatSessionRepo,
		bindingRepo:        bindingRepo,
		pool:               pool,
		workspaceService:   workspaceService,
	}
}

// NewJoinCommandHandlerWithResolver creates a new JoinCommandHandler with a custom workspacePathResolver for testing.
func NewJoinCommandHandlerWithResolver(
	chatSessionService *cdp.ChatSessionService,
	chatSessionRepo *database.ChatSessionRepository,
	bindingRepo *database.WorkspaceBindingRepository,
	pool *cdp.ConnectionPool,
	workspaceService workspacePathResolver,
) *JoinCommandHandler {
	return &JoinCommandHandler{
		chatSessionService: chatSessionService,
		chatSessionRepo:    chatSessionRepo,
		bindingRepo:        bindingRepo,
		pool:               pool,
		workspaceService:   workspaceService,
	}
}

// ResolveProjectPath returns the absolute path for projectName using WorkspaceService.
func (h *JoinCommandHandler) ResolveProjectPath(projectName string) (string, error) {
	if h.workspaceService == nil {
		return projectName, nil
	}
	return h.workspaceService.GetWorkspacePath(projectName)
}
