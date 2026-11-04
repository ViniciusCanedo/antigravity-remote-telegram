package bot

import (
	"time"

	"antigravity-telegram-remote/internal/database"
)

// Cleanup button constants.
const (
	CleanupArchiveBtn = "cleanup_archive"
	CleanupDeleteBtn  = "cleanup_delete"
	CleanupCancelBtn  = "cleanup_cancel"
)

// InactiveSession holds a workspace binding and its optional associated chat session.
type InactiveSession struct {
	Binding database.WorkspaceBindingRecord
	Session *database.ChatSessionRecord
}

type cleanupSessionStore interface {
	DeleteByChannelId(channelID string) (bool, error)
	FindByChannelId(channelID string) (*database.ChatSessionRecord, error)
}

type cleanupBindingStore interface {
	DeleteByChannelId(channelID string) (bool, error)
	FindByGuildId(guildID string) ([]database.WorkspaceBindingRecord, error)
}

// CleanupCommandHandler handles finding and cleaning up inactive chat sessions and bindings.
type CleanupCommandHandler struct {
	chatSessionRepo cleanupSessionStore
	bindingRepo     cleanupBindingStore
}

// NewCleanupCommandHandler creates a new CleanupCommandHandler.
func NewCleanupCommandHandler(
	chatSessionRepo *database.ChatSessionRepository,
	bindingRepo *database.WorkspaceBindingRepository,
) *CleanupCommandHandler {
	return &CleanupCommandHandler{
		chatSessionRepo: chatSessionRepo,
		bindingRepo:     bindingRepo,
	}
}

// NewCleanupCommandHandlerWithStores creates a new CleanupCommandHandler using custom stores for testing.
func NewCleanupCommandHandlerWithStores(
	chatSessionRepo cleanupSessionStore,
	bindingRepo cleanupBindingStore,
) *CleanupCommandHandler {
	return &CleanupCommandHandler{
		chatSessionRepo: chatSessionRepo,
		bindingRepo:     bindingRepo,
	}
}

// CleanupByChannelId deletes session records and workspace bindings for the specified channel.
func (h *CleanupCommandHandler) CleanupByChannelId(channelID string) error {
	if h.chatSessionRepo != nil {
		if _, err := h.chatSessionRepo.DeleteByChannelId(channelID); err != nil {
			return err
		}
	}

	if h.bindingRepo != nil {
		if _, err := h.bindingRepo.DeleteByChannelId(channelID); err != nil {
			return err
		}
	}

	return nil
}

// CleanupByChannelID is an alias for CleanupByChannelId.
func (h *CleanupCommandHandler) CleanupByChannelID(channelID string) error {
	return h.CleanupByChannelId(channelID)
}

// FindInactiveSessions returns sessions and bindings for a guild older than the specified number of days.
func (h *CleanupCommandHandler) FindInactiveSessions(guildID string, days int) ([]InactiveSession, error) {
	if h.bindingRepo == nil {
		return []InactiveSession{}, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	cutoffIso := cutoff.Format("2006-01-02 15:04:05")

	bindings, err := h.bindingRepo.FindByGuildId(guildID)
	if err != nil {
		return nil, err
	}

	var inactive []InactiveSession
	for _, binding := range bindings {
		var session *database.ChatSessionRecord
		if h.chatSessionRepo != nil {
			session, _ = h.chatSessionRepo.FindByChannelId(binding.ChannelID)
		}

		createdAt := binding.CreatedAt
		if session != nil && session.CreatedAt != "" {
			createdAt = session.CreatedAt
		}

		if createdAt != "" && createdAt < cutoffIso {
			inactive = append(inactive, InactiveSession{
				Binding: binding,
				Session: session,
			})
		}
	}

	if inactive == nil {
		inactive = []InactiveSession{}
	}
	return inactive, nil
}
