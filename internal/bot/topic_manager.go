package bot

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	tgbot "github.com/go-telegram/bot"
)

var (
	trailingSlashRegex = regexp.MustCompile(`/+$`)
	multiHyphenRegex   = regexp.MustCompile(`-{2,}`)
	edgeHyphenRegex    = regexp.MustCompile(`^-+|-+$`)
)

// EnsureTopicResult represents the outcome of EnsureTopic.
type EnsureTopicResult struct {
	TopicID int
	Created bool
}

// ForumTopicAPI abstracts Telegram Bot API forum topic operations.
type ForumTopicAPI interface {
	CreateForumTopic(ctx context.Context, chatID int64, name string) (messageThreadID int, err error)
	EditForumTopic(ctx context.Context, chatID int64, messageThreadID int, name string) error
	CloseForumTopic(ctx context.Context, chatID int64, messageThreadID int) error
	DeleteForumTopic(ctx context.Context, chatID int64, messageThreadID int) error
}

// BotForumTopicAPI implements ForumTopicAPI using *tgbot.Bot.
type BotForumTopicAPI struct {
	b *tgbot.Bot
}

// NewBotForumTopicAPI creates a new BotForumTopicAPI adapter.
func NewBotForumTopicAPI(b *tgbot.Bot) *BotForumTopicAPI {
	return &BotForumTopicAPI{b: b}
}

func (a *BotForumTopicAPI) CreateForumTopic(ctx context.Context, chatID int64, name string) (int, error) {
	topic, err := a.b.CreateForumTopic(ctx, &tgbot.CreateForumTopicParams{
		ChatID: chatID,
		Name:   name,
	})
	if err != nil {
		return 0, err
	}
	return topic.MessageThreadID, nil
}

func (a *BotForumTopicAPI) EditForumTopic(ctx context.Context, chatID int64, messageThreadID int, name string) error {
	_, err := a.b.EditForumTopic(ctx, &tgbot.EditForumTopicParams{
		ChatID:          chatID,
		MessageThreadID: messageThreadID,
		Name:            name,
	})
	return err
}

func (a *BotForumTopicAPI) CloseForumTopic(ctx context.Context, chatID int64, messageThreadID int) error {
	_, err := a.b.CloseForumTopic(ctx, &tgbot.CloseForumTopicParams{
		ChatID:          chatID,
		MessageThreadID: messageThreadID,
	})
	return err
}

func (a *BotForumTopicAPI) DeleteForumTopic(ctx context.Context, chatID int64, messageThreadID int) error {
	_, err := a.b.DeleteForumTopic(ctx, &tgbot.DeleteForumTopicParams{
		ChatID:          chatID,
		MessageThreadID: messageThreadID,
	})
	return err
}

// TelegramTopicManager manages Telegram Forum Topics for workspace and session isolation.
type TelegramTopicManager struct {
	mu         sync.RWMutex
	chatID     int64
	api        ForumTopicAPI
	topicCache map[string]int
}

// NewTelegramTopicManager initializes a new TelegramTopicManager.
func NewTelegramTopicManager(api ForumTopicAPI, chatID int64) *TelegramTopicManager {
	return &TelegramTopicManager{
		api:        api,
		chatID:     chatID,
		topicCache: make(map[string]int),
	}
}

// EnsureTopic ensures a forum topic exists for the workspace, returning existing cached ID or creating one.
func (m *TelegramTopicManager) EnsureTopic(ctx context.Context, workspaceName string) (EnsureTopicResult, error) {
	m.mu.RLock()
	cached, ok := m.topicCache[workspaceName]
	chatID := m.chatID
	m.mu.RUnlock()

	if ok && cached != 0 {
		return EnsureTopicResult{TopicID: cached, Created: false}, nil
	}

	topicName := fmt.Sprintf("🗂️ %s", workspaceName)
	messageThreadID, err := m.api.CreateForumTopic(ctx, chatID, topicName)
	if err != nil {
		slog.Error(fmt.Sprintf("[TopicManager] Failed to create topic for \"%s\":", workspaceName), "error", err)
		return EnsureTopicResult{}, err
	}

	m.mu.Lock()
	m.topicCache[workspaceName] = messageThreadID
	m.mu.Unlock()

	return EnsureTopicResult{TopicID: messageThreadID, Created: true}, nil
}

// CreateSessionTopic creates a new session topic under the current chat context.
func (m *TelegramTopicManager) CreateSessionTopic(ctx context.Context, sessionName string) (int, error) {
	m.mu.RLock()
	chatID := m.chatID
	m.mu.RUnlock()

	topicName := fmt.Sprintf("💬 %s", sessionName)
	messageThreadID, err := m.api.CreateForumTopic(ctx, chatID, topicName)
	if err != nil {
		slog.Error(fmt.Sprintf("[TopicManager] Failed to create session topic \"%s\":", sessionName), "error", err)
		return 0, err
	}

	return messageThreadID, nil
}

// RenameTopic edits a forum topic name, logging and suppressing any API errors.
func (m *TelegramTopicManager) RenameTopic(ctx context.Context, topicID int, newName string) {
	m.mu.RLock()
	chatID := m.chatID
	m.mu.RUnlock()

	if err := m.api.EditForumTopic(ctx, chatID, topicID, newName); err != nil {
		slog.Error(fmt.Sprintf("[TopicManager] Failed to rename topic %d:", topicID), "error", err)
	}
}

// CloseTopic closes (archives) a forum topic, logging and suppressing any API errors.
func (m *TelegramTopicManager) CloseTopic(ctx context.Context, topicID int) {
	m.mu.RLock()
	chatID := m.chatID
	m.mu.RUnlock()

	if err := m.api.CloseForumTopic(ctx, chatID, topicID); err != nil {
		slog.Error(fmt.Sprintf("[TopicManager] Failed to close topic %d:", topicID), "error", err)
	}
}

// DeleteTopic deletes a forum topic, logging and suppressing any API errors.
func (m *TelegramTopicManager) DeleteTopic(ctx context.Context, topicID int) {
	m.mu.RLock()
	chatID := m.chatID
	m.mu.RUnlock()

	if err := m.api.DeleteForumTopic(ctx, chatID, topicID); err != nil {
		slog.Error(fmt.Sprintf("[TopicManager] Failed to delete topic %d:", topicID), "error", err)
	}
}

// SetChatID updates the target chat ID.
func (m *TelegramTopicManager) SetChatID(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatID = chatID
}

// RegisterTopic manually records a (workspaceName, topicID) pair in memory cache.
func (m *TelegramTopicManager) RegisterTopic(workspaceName string, topicID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.topicCache[workspaceName] = topicID
}

// GetTopicID retrieves the cached topic ID for the workspace name if present.
func (m *TelegramTopicManager) GetTopicID(workspaceName string) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.topicCache[workspaceName]
	return id, ok
}

// SanitizeName sanitizes workspace/topic names according to forum topic length and character constraints.
func (m *TelegramTopicManager) SanitizeName(name string) string {
	sanitized := trailingSlashRegex.ReplaceAllString(name, "")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = multiHyphenRegex.ReplaceAllString(sanitized, "-")
	sanitized = edgeHyphenRegex.ReplaceAllString(sanitized, "")

	runes := []rune(sanitized)
	if len(runes) > 128 {
		sanitized = string(runes[:128])
	}
	return sanitized
}
