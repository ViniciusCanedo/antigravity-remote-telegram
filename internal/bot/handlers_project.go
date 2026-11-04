package bot

import (
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/workspace"
)

type workspaceBindingStore interface {
	FindByChannelId(channelID string) (*database.WorkspaceBindingRecord, error)
}

type workspacePathResolver interface {
	GetWorkspacePath(workspaceName string) (string, error)
}

// WorkspaceCommandHandler provides utilities for resolving workspace paths for channels.
type WorkspaceCommandHandler struct {
	bindingRepo      workspaceBindingStore
	chatSessionRepo  any
	workspaceService workspacePathResolver
}

// NewWorkspaceCommandHandler creates a new WorkspaceCommandHandler.
func NewWorkspaceCommandHandler(
	bindingRepo *database.WorkspaceBindingRepository,
	chatSessionRepo *database.ChatSessionRepository,
	workspaceService *workspace.WorkspaceService,
) *WorkspaceCommandHandler {
	return &WorkspaceCommandHandler{
		bindingRepo:      bindingRepo,
		chatSessionRepo:  chatSessionRepo,
		workspaceService: workspaceService,
	}
}

// NewWorkspaceCommandHandlerWithInterfaces creates a WorkspaceCommandHandler with interfaces for testing.
func NewWorkspaceCommandHandlerWithInterfaces(
	bindingRepo workspaceBindingStore,
	chatSessionRepo any,
	workspaceService workspacePathResolver,
) *WorkspaceCommandHandler {
	return &WorkspaceCommandHandler{
		bindingRepo:      bindingRepo,
		chatSessionRepo:  chatSessionRepo,
		workspaceService: workspaceService,
	}
}

// GetWorkspaceForChannel returns the resolved absolute workspace path bound to channelID, or ("", false) if not found.
func (h *WorkspaceCommandHandler) GetWorkspaceForChannel(channelID string) (string, bool) {
	if h.bindingRepo == nil {
		return "", false
	}
	binding, err := h.bindingRepo.FindByChannelId(channelID)
	if err != nil || binding == nil {
		return "", false
	}

	if h.workspaceService == nil {
		return binding.WorkspacePath, true
	}

	path, err := h.workspaceService.GetWorkspacePath(binding.WorkspacePath)
	if err != nil {
		return "", false
	}
	return path, true
}
