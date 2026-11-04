package bot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"antigravity-telegram-remote/internal/bot/ui"
	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/config"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/logger"
	"antigravity-telegram-remote/internal/platform"
	"antigravity-telegram-remote/internal/voice"
	"antigravity-telegram-remote/internal/workspace"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	_ "modernc.org/sqlite"
)

// TelegramChannel uniquely identifies a chat or forum topic in Telegram.
type TelegramChannel struct {
	ChatID   int64
	ThreadID int // 0 when not inside a forum topic
}

// PlanEditPendingState stores channel state waiting for plan edit instructions.
type PlanEditPendingState struct {
	ProjectName string
}

// BotCommandEntry represents a command item sent to setMyCommands.
type BotCommandEntry struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ResolvedWorkspaceCdp encapsulates the outcome of ResolveWorkspaceAndCdp.
type ResolvedWorkspaceCdp struct {
	Cdp           cdp.PoolClient
	ProjectName   string
	WorkspacePath string
}

// ClickStopButtonScript evaluates stop button click inside Antigravity agent panel.
const ClickStopButtonScript = `(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    const scopes = [panel, document].filter(Boolean);

    for (const scope of scopes) {
        const el = scope.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
        if (el && typeof el.click === 'function') {
            el.click();
            return { ok: true, method: 'tooltip-id' };
        }
    }

    const normalize = (value) => (value || '').toLowerCase().replace(/\s+/g, ' ').trim();
    const STOP_PATTERNS = [
        /^stop$/,
        /^stop generating$/,
        /^stop response$/,
        /^停止$/,
        /^生成を停止$/,
        /^応答を停止$/,
    ];
    const isStopLabel = (value) => {
        const normalized = normalize(value);
        if (!normalized) return false;
        return STOP_PATTERNS.some((re) => re.test(normalized));
    };
    for (const scope of scopes) {
        const buttons = scope.querySelectorAll('button, [role="button"]');
        for (let i = 0; i < buttons.length; i++) {
            const btn = buttons[i];
            const labels = [
                btn.textContent || '',
                btn.getAttribute('aria-label') || '',
                btn.getAttribute('title') || '',
            ];
            if (labels.some(isStopLabel) && typeof btn.click === 'function') {
                btn.click();
                return { ok: true, method: 'text-fallback' };
            }
        }
    }

    return { ok: false, error: 'Stop button not found' };
})()`

// BotCommands defines native Telegram bot commands registered via setMyCommands.
var BotCommands = []models.BotCommand{
	{Command: "start", Description: "Tin nhắn chào mừng"},
	{Command: "help", Description: "Hiển thị tất cả lệnh"},
	{Command: "project", Description: "Chọn một dự án"},
	{Command: "new", Description: "Bắt đầu phiên chat mới"},
	{Command: "chat", Description: "Thông tin phiên hiện tại"},
	{Command: "mode", Description: "Thay đổi chế độ thực thi"},
	{Command: "model", Description: "Thay đổi mô hình LLM"},
	{Command: "stop", Description: "Dừng việc tạo kết quả"},
	{Command: "screenshot", Description: "Chụp màn hình Antigravity"},
	{Command: "template", Description: "Hiển thị mẫu lời nhắc"},
	{Command: "template_add", Description: "Đăng ký một mẫu"},
	{Command: "template_delete", Description: "Xóa một mẫu"},
	{Command: "autoaccept", Description: "Bật/tắt chế độ tự động duyệt"},
	{Command: "status", Description: "Tổng quan trạng thái bot"},
	{Command: "ping", Description: "Kiểm tra độ trễ"},
}

// BotDeps contains all dependencies required to instantiate Bot.
type BotDeps struct {
	Config               *config.AppConfig
	DB                   *sql.DB
	Bridge               *bridge.CdpBridge
	Pool                 *cdp.ConnectionPool
	ModeService          *bridge.ModeService
	ModelService         *bridge.ModelService
	TemplateRepo         *database.TemplateRepository
	WorkspaceBindingRepo *database.WorkspaceBindingRepository
	ChatSessionRepo      *database.ChatSessionRepository
	WorkspaceService     *workspace.WorkspaceService
	ChatSessionService   *cdp.ChatSessionService
	TitleGenerator       *bridge.TitleGeneratorService
	PromptDispatcher     *bridge.PromptDispatcher
	TopicManager         *TelegramTopicManager
	SlashCommandHandler  *SlashCommandHandler
	CleanupHandler       *CleanupCommandHandler
	ChatHandler          *ChatCommandHandler
	WorkspaceHandler     *WorkspaceCommandHandler
	JoinHandler          *JoinCommandHandler
	TgBot                *tgbot.Bot
}

// Bot encapsulates dependencies, services, repositories and synchronized state for the bot.
type Bot struct {
	Config               *config.AppConfig
	DB                   *sql.DB
	Bridge               *bridge.CdpBridge
	Pool                 *cdp.ConnectionPool
	ModeService          *bridge.ModeService
	ModelService         *bridge.ModelService
	TemplateRepo         *database.TemplateRepository
	WorkspaceBindingRepo *database.WorkspaceBindingRepository
	ChatSessionRepo      *database.ChatSessionRepository
	WorkspaceService     *workspace.WorkspaceService
	ChatSessionService   *cdp.ChatSessionService
	TitleGenerator       *bridge.TitleGeneratorService
	PromptDispatcher     *bridge.PromptDispatcher
	TopicManager         *TelegramTopicManager
	SlashCommandHandler  *SlashCommandHandler
	CleanupHandler       *CleanupCommandHandler
	ChatHandler          *ChatCommandHandler
	WorkspaceHandler     *WorkspaceCommandHandler
	JoinHandler          *JoinCommandHandler
	TgBot                *tgbot.Bot
	telegramSender       TelegramSender
	fileResolver         TelegramFileResolver
	workspaceResolver    func(ctx context.Context, ch TelegramChannel) *ResolvedWorkspaceCdp
	whisperInspector     *voice.WhisperInspector
	whisperRunner        voice.WhisperRunner
	httpClient           *http.Client

	stateMu                 sync.RWMutex
	userStopRequested       map[string]struct{}
	planEditPendingChannels map[string]PlanEditPendingState
	planContentCache        map[string][]string
}

// NewBot instantiates a new Bot with initialized memory maps.
func NewBot(deps BotDeps) *Bot {
	return &Bot{
		Config:                  deps.Config,
		DB:                      deps.DB,
		Bridge:                  deps.Bridge,
		Pool:                    deps.Pool,
		ModeService:             deps.ModeService,
		ModelService:            deps.ModelService,
		TemplateRepo:            deps.TemplateRepo,
		WorkspaceBindingRepo:    deps.WorkspaceBindingRepo,
		ChatSessionRepo:         deps.ChatSessionRepo,
		WorkspaceService:        deps.WorkspaceService,
		ChatSessionService:      deps.ChatSessionService,
		TitleGenerator:          deps.TitleGenerator,
		PromptDispatcher:        deps.PromptDispatcher,
		TopicManager:            deps.TopicManager,
		SlashCommandHandler:     deps.SlashCommandHandler,
		CleanupHandler:          deps.CleanupHandler,
		ChatHandler:             deps.ChatHandler,
		WorkspaceHandler:        deps.WorkspaceHandler,
		JoinHandler:             deps.JoinHandler,
		TgBot:                   deps.TgBot,
		userStopRequested:       make(map[string]struct{}),
		planEditPendingChannels: make(map[string]PlanEditPendingState),
		planContentCache:        make(map[string][]string),
	}
}

// ChannelKey formats the channel key as "<chatId>:<threadId>" or "<chatId>".
func ChannelKey(ch TelegramChannel) string {
	if ch.ThreadID != 0 {
		return fmt.Sprintf("%d:%d", ch.ChatID, ch.ThreadID)
	}
	return fmt.Sprintf("%d", ch.ChatID)
}

// ParseChannelKey parses a string back into a TelegramChannel.
func ParseChannelKey(key string) (TelegramChannel, error) {
	if strings.Contains(key, ":") {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			return TelegramChannel{}, fmt.Errorf("invalid channel key format: %s", key)
		}
		chatID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return TelegramChannel{}, fmt.Errorf("invalid chat ID in channel key: %w", err)
		}
		threadID, err := strconv.Atoi(parts[1])
		if err != nil {
			return TelegramChannel{}, fmt.Errorf("invalid thread ID in channel key: %w", err)
		}
		return TelegramChannel{ChatID: chatID, ThreadID: threadID}, nil
	}

	chatID, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		return TelegramChannel{}, fmt.Errorf("invalid chat ID in channel key: %w", err)
	}
	return TelegramChannel{ChatID: chatID, ThreadID: 0}, nil
}

// GetChannelFromMessage constructs TelegramChannel from message parameters.
func GetChannelFromMessage(chatID int64, messageThreadID int) TelegramChannel {
	return TelegramChannel{ChatID: chatID, ThreadID: messageThreadID}
}

// GetChannelFromCallback constructs TelegramChannel from callback parameters.
func GetChannelFromCallback(chatID int64, messageThreadID int) TelegramChannel {
	return TelegramChannel{ChatID: chatID, ThreadID: messageThreadID}
}

// State management methods (thread-safe)

// AddUserStopRequest adds a channel key to the user stop requested set.
func (b *Bot) AddUserStopRequest(channelKey string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.userStopRequested[channelKey] = struct{}{}
}

// HasUserStopRequest checks if user stop is requested for channelKey.
func (b *Bot) HasUserStopRequest(channelKey string) bool {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	_, ok := b.userStopRequested[channelKey]
	return ok
}

// ConsumeUserStopRequest consumes and removes user stop request for channelKey.
func (b *Bot) ConsumeUserStopRequest(channelKey string) bool {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	_, ok := b.userStopRequested[channelKey]
	delete(b.userStopRequested, channelKey)
	return ok
}

// SetPlanEditPending sets the pending plan edit state for channelKey.
func (b *Bot) SetPlanEditPending(channelKey, projectName string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.planEditPendingChannels[channelKey] = PlanEditPendingState{ProjectName: projectName}
}

// GetPlanEditPending retrieves pending plan edit state for channelKey.
func (b *Bot) GetPlanEditPending(channelKey string) (PlanEditPendingState, bool) {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	state, ok := b.planEditPendingChannels[channelKey]
	return state, ok
}

// ClearPlanEditPending clears pending plan edit state for channelKey.
func (b *Bot) ClearPlanEditPending(channelKey string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	delete(b.planEditPendingChannels, channelKey)
}

// SetPlanContentCache caches paginated plan content for channelKey.
func (b *Bot) SetPlanContentCache(channelKey string, pages []string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.planContentCache[channelKey] = pages
}

// GetPlanContentCache retrieves cached plan content for channelKey.
func (b *Bot) GetPlanContentCache(channelKey string) ([]string, bool) {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	pages, ok := b.planContentCache[channelKey]
	return pages, ok
}

// ClearPlanContentCache removes cached plan content for channelKey.
func (b *Bot) ClearPlanContentCache(channelKey string) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	delete(b.planContentCache, channelKey)
}

// ReplyHTML sends an HTML message to the given Telegram channel.
func (b *Bot) ReplyHTML(ctx context.Context, ch TelegramChannel, text string, keyboard *ui.InlineKeyboardMarkup) error {
	sender := b.getSender()
	if sender == nil {
		return nil
	}

	params := &tgbot.SendMessageParams{
		ChatID:    ch.ChatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if ch.ThreadID != 0 {
		params.MessageThreadID = ch.ThreadID
	}
	if keyboard != nil {
		params.ReplyMarkup = keyboard.ToTelegramModel()
	}

	_, err := sender.SendMessage(ctx, params)
	return err
}

// Adapters for cdp and ui

type modeServiceUIAdapter struct {
	svc *bridge.ModeService
}

func (a modeServiceUIAdapter) GetCurrentMode() string {
	if a.svc == nil {
		return "fast"
	}
	return string(a.svc.GetCurrentMode())
}

func (a modeServiceUIAdapter) SetMode(mode string) {
	if a.svc != nil {
		_ = a.svc.SetMode(mode)
	}
}

type modeCdpUIAdapter struct {
	client cdp.PoolClient
}

func (a modeCdpUIAdapter) GetCurrentMode(ctx context.Context) (string, error) {
	if c, ok := a.client.(*cdp.Client); ok && c != nil {
		res := c.GetCurrentMode()
		if res != nil {
			return *res, nil
		}
	}
	return "", nil
}

type modelsCdpUIAdapter struct {
	client cdp.PoolClient
}

func (a modelsCdpUIAdapter) GetUiModels(ctx context.Context) ([]string, error) {
	if c, ok := a.client.(*cdp.Client); ok && c != nil {
		return c.GetUIModels()
	}
	return []string{}, nil
}

func (a modelsCdpUIAdapter) GetCurrentModel(ctx context.Context) (string, error) {
	if c, ok := a.client.(*cdp.Client); ok && c != nil {
		res := c.GetCurrentModel()
		if res != nil {
			return *res, nil
		}
	}
	return "", nil
}

type screenshotCapturerAdapter struct {
	client cdp.CDPCaller
}

func (a screenshotCapturerAdapter) Capture(ctx context.Context, format string) (ui.ScreenshotResult, error) {
	if a.client == nil {
		return ui.ScreenshotResult{Success: false, Error: "not connected"}, nil
	}
	svc := cdp.NewScreenshotService(cdp.ScreenshotServiceOptions{CDPService: a.client})
	res := svc.Capture(cdp.CaptureOptions{Format: format})
	return ui.ScreenshotResult{
		Success: res.Success,
		Buffer:  res.Buffer,
		Error:   res.Error,
	}, nil
}

type cdpServiceWithContextsAdapter struct {
	client cdp.PoolClient
}

func (a *cdpServiceWithContextsAdapter) Call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	if a.client == nil {
		return nil, fmt.Errorf("client is nil")
	}
	return a.client.Call(method, params)
}

func (a *cdpServiceWithContextsAdapter) GetPrimaryContextId() *int64 {
	if a.client == nil {
		return nil
	}
	id := a.client.GetPrimaryContextID()
	if id == nil {
		return nil
	}
	id64 := int64(*id)
	return &id64
}

func (a *cdpServiceWithContextsAdapter) GetCurrentWorkspaceName() string {
	if c, ok := a.client.(*cdp.Client); ok && c != nil {
		return c.GetCurrentWorkspaceName()
	}
	return ""
}

func (a *cdpServiceWithContextsAdapter) GetContexts() []bridge.ExecutionContextInfo {
	if a.client == nil {
		return nil
	}
	rawContexts := a.client.GetContexts()
	infos := make([]bridge.ExecutionContextInfo, 0, len(rawContexts))
	for _, rc := range rawContexts {
		infos = append(infos, bridge.ExecutionContextInfo{
			ID: int64(rc.ID),
		})
	}
	return infos
}

type backgroundDetectorAdapter struct {
	isActive func() bool
	stop     func()
}

func (b backgroundDetectorAdapter) IsActive() bool {
	if b.isActive != nil {
		return b.isActive()
	}
	return false
}

func (b backgroundDetectorAdapter) Start() {}

func (b backgroundDetectorAdapter) Stop() {
	if b.stop != nil {
		b.stop()
	}
}

type bridgePoolAdapter struct {
	pool                 *cdp.ConnectionPool
	mu                   sync.RWMutex
	approvalDetectors    map[string]*bridge.ApprovalDetector
	planningDetectors    map[string]*bridge.PlanningDetector
	errorDetectors       map[string]*bridge.ErrorPopupDetector
	userMessageDetectors map[string]*bridge.UserMessageDetector
}

func newBridgePoolAdapter(pool *cdp.ConnectionPool) *bridgePoolAdapter {
	return &bridgePoolAdapter{
		pool:                 pool,
		approvalDetectors:    make(map[string]*bridge.ApprovalDetector),
		planningDetectors:    make(map[string]*bridge.PlanningDetector),
		errorDetectors:       make(map[string]*bridge.ErrorPopupDetector),
		userMessageDetectors: make(map[string]*bridge.UserMessageDetector),
	}
}

func (a *bridgePoolAdapter) GetConnected(workspaceName string) bridge.CdpServiceWithContexts {
	if a.pool == nil {
		return nil
	}
	client := a.pool.GetConnected(workspaceName)
	if client == nil {
		return nil
	}
	return &cdpServiceWithContextsAdapter{client: client}
}

func (a *bridgePoolAdapter) GetActiveWorkspaceNames() []string {
	if a.pool == nil {
		return []string{}
	}
	return a.pool.GetActiveWorkspaceNames()
}

func (a *bridgePoolAdapter) GetApprovalDetector(projectName string) *bridge.ApprovalDetector {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.approvalDetectors[projectName]
}

func (a *bridgePoolAdapter) RegisterApprovalDetector(projectName string, detector *bridge.ApprovalDetector) {
	a.mu.Lock()
	a.approvalDetectors[projectName] = detector
	a.mu.Unlock()
	if a.pool != nil && detector != nil {
		a.pool.RegisterApprovalDetector(projectName, backgroundDetectorAdapter{
			isActive: detector.IsActive,
			stop:     func() { _ = detector.Stop() },
		})
	}
}

func (a *bridgePoolAdapter) GetPlanningDetector(projectName string) *bridge.PlanningDetector {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.planningDetectors[projectName]
}

func (a *bridgePoolAdapter) RegisterPlanningDetector(projectName string, detector *bridge.PlanningDetector) {
	a.mu.Lock()
	a.planningDetectors[projectName] = detector
	a.mu.Unlock()
	if a.pool != nil && detector != nil {
		a.pool.RegisterPlanningDetector(projectName, backgroundDetectorAdapter{
			isActive: detector.IsActive,
			stop:     func() { _ = detector.Stop() },
		})
	}
}

func (a *bridgePoolAdapter) GetErrorPopupDetector(projectName string) *bridge.ErrorPopupDetector {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.errorDetectors[projectName]
}

func (a *bridgePoolAdapter) RegisterErrorPopupDetector(projectName string, detector *bridge.ErrorPopupDetector) {
	a.mu.Lock()
	a.errorDetectors[projectName] = detector
	a.mu.Unlock()
	if a.pool != nil && detector != nil {
		a.pool.RegisterErrorPopupDetector(projectName, backgroundDetectorAdapter{
			isActive: detector.IsActive,
			stop:     func() { _ = detector.Stop() },
		})
	}
}

func (a *bridgePoolAdapter) GetUserMessageDetector(projectName string) *bridge.UserMessageDetector {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.userMessageDetectors[projectName]
}

func (a *bridgePoolAdapter) RegisterUserMessageDetector(projectName string, detector *bridge.UserMessageDetector) {
	a.mu.Lock()
	a.userMessageDetectors[projectName] = detector
	a.mu.Unlock()
	if a.pool != nil && detector != nil {
		a.pool.RegisterUserMessageDetector(projectName, backgroundDetectorAdapter{
			isActive: detector.IsActive,
			stop:     func() { detector.Stop() },
		})
	}
}

type autoAcceptCheckerAdapter struct {
	svc *bridge.AutoAcceptService
}

func (a autoAcceptCheckerAdapter) IsEnabled() bool {
	if a.svc == nil {
		return false
	}
	return a.svc.IsEnabled()
}

func (a autoAcceptCheckerAdapter) Handle(mode string) (bridge.AutoAcceptResult, error) {
	if a.svc == nil {
		return bridge.AutoAcceptResult{}, fmt.Errorf("auto accept service is nil")
	}
	res := a.svc.Handle(mode)
	return bridge.AutoAcceptResult{
		Success: res.Success,
		Enabled: res.Enabled,
		Changed: res.Changed,
		Message: res.Message,
	}, nil
}

// SetWorkspaceResolver overrides the workspace resolver for tests.
func (b *Bot) SetWorkspaceResolver(fn func(ctx context.Context, ch TelegramChannel) *ResolvedWorkspaceCdp) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.workspaceResolver = fn
}

// SetWhisperInspector overrides the Whisper environment inspector for tests.
func (b *Bot) SetWhisperInspector(insp *voice.WhisperInspector) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.whisperInspector = insp
}

// SetWhisperRunner overrides the Whisper audio transcription runner for tests.
func (b *Bot) SetWhisperRunner(runner voice.WhisperRunner) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.whisperRunner = runner
}

// SetHTTPClient overrides the HTTP client used for downloading media files (for tests).
func (b *Bot) SetHTTPClient(client *http.Client) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.httpClient = client
}

// ResolveWorkspaceAndCdp resolves the workspace bound to ch and connects to CDP.
func (b *Bot) ResolveWorkspaceAndCdp(ctx context.Context, ch TelegramChannel) *ResolvedWorkspaceCdp {
	b.stateMu.RLock()
	resolver := b.workspaceResolver
	b.stateMu.RUnlock()
	if resolver != nil {
		return resolver(ctx, ch)
	}

	key := ChannelKey(ch)
	if b.WorkspaceBindingRepo == nil {
		return nil
	}
	binding, err := b.WorkspaceBindingRepo.FindByChannelId(key)
	if err != nil || binding == nil {
		return nil
	}

	if b.WorkspaceService == nil {
		return nil
	}
	workspacePath, err := b.WorkspaceService.GetWorkspacePath(binding.WorkspacePath)
	if err != nil {
		return nil
	}

	pool := b.Pool
	if pool == nil {
		return nil
	}

	client, err := pool.GetOrConnect(workspacePath)
	if err != nil {
		logger.Error(fmt.Sprintf("[resolveWorkspaceAndCdp] Connection failed: %v", err))
		return nil
	}

	projectName := pool.ExtractProjectName(workspacePath)
	if b.Bridge != nil {
		b.Bridge.LastActiveWorkspace = &projectName
		var thPtr *int
		if ch.ThreadID != 0 {
			th := ch.ThreadID
			thPtr = &th
		}
		bridgeCh := bridge.TelegramChannel{ChatID: ch.ChatID, ThreadID: thPtr}
		b.Bridge.LastActiveChannel = &bridgeCh

		cdpWithCtx := &cdpServiceWithContextsAdapter{client: client}
		bridge.RegisterApprovalWorkspaceChannel(b.Bridge, projectName, bridgeCh)
		bridge.EnsureApprovalDetector(b.Bridge, cdpWithCtx, projectName)
		bridge.EnsureErrorPopupDetector(b.Bridge, cdpWithCtx, projectName)
		bridge.EnsurePlanningDetector(b.Bridge, cdpWithCtx, projectName)
	}

	return &ResolvedWorkspaceCdp{
		Cdp:           client,
		ProjectName:   projectName,
		WorkspacePath: workspacePath,
	}
}

// SetupPoolEventListeners sets up connection pool lifecycle event listeners.
func (b *Bot) SetupPoolEventListeners(pool *cdp.ConnectionPool) {
	if pool == nil {
		return
	}

	pool.On("workspace:disconnected", func(projectName string) {
		if b.Bridge == nil || b.Bridge.LastActiveChannel == nil || b.TgBot == nil {
			return
		}
		ch := *b.Bridge.LastActiveChannel
		threadID := 0
		if ch.ThreadID != nil {
			threadID = *ch.ThreadID
		}
		text := fmt.Sprintf("⚠️ <b>%s</b>: Connection lost. Reconnecting…", ui.EscapeHTML(projectName))
		_ = b.ReplyHTML(context.Background(), TelegramChannel{ChatID: ch.ChatID, ThreadID: threadID}, text, nil)
	})

	pool.On("workspace:reconnected", func(projectName string) {
		if b.Bridge == nil || b.Bridge.LastActiveChannel == nil || b.TgBot == nil {
			return
		}
		ch := *b.Bridge.LastActiveChannel
		threadID := 0
		if ch.ThreadID != nil {
			threadID = *ch.ThreadID
		}
		text := fmt.Sprintf("✅ <b>%s</b>: Reconnected.", ui.EscapeHTML(projectName))
		_ = b.ReplyHTML(context.Background(), TelegramChannel{ChatID: ch.ChatID, ThreadID: threadID}, text, nil)
	})

	pool.On("workspace:reconnectFailed", func(projectName string) {
		if b.Bridge == nil || b.Bridge.LastActiveChannel == nil || b.TgBot == nil {
			return
		}
		ch := *b.Bridge.LastActiveChannel
		threadID := 0
		if ch.ThreadID != nil {
			threadID = *ch.ThreadID
		}
		text := fmt.Sprintf("❌ <b>%s</b>: Reconnection failed. Send a message to retry.", ui.EscapeHTML(projectName))
		_ = b.ReplyHTML(context.Background(), TelegramChannel{ChatID: ch.ChatID, ThreadID: threadID}, text, nil)
	})
}

// RegisterBotCommands registers native commands with the Telegram API.
func (b *Bot) RegisterBotCommands(ctx context.Context) error {
	if b.TgBot == nil {
		return nil
	}
	_, err := b.TgBot.SetMyCommands(ctx, &tgbot.SetMyCommandsParams{
		Commands: BotCommands,
	})
	return err
}

// RegisterCommands registers all bot command handlers on TgBot.
func (b *Bot) RegisterCommands() {
	if b.TgBot == nil {
		return
	}

	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypeCommand, b.handleStart)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypeCommand, b.handleHelp)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/project", tgbot.MatchTypeCommand, b.handleProject)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/new", tgbot.MatchTypeCommand, b.handleNew)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/chat", tgbot.MatchTypeCommand, b.handleChat)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/join", tgbot.MatchTypeCommand, b.handleJoin)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/mode", tgbot.MatchTypeCommand, b.handleMode)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/model", tgbot.MatchTypeCommand, b.handleModel)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/stop", tgbot.MatchTypeCommand, b.handleStop)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/screenshot", tgbot.MatchTypeCommand, b.handleScreenshot)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/template", tgbot.MatchTypeCommand, b.handleTemplate)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/template_add", tgbot.MatchTypeCommand, b.handleTemplateAdd)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/template_delete", tgbot.MatchTypeCommand, b.handleTemplateDelete)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/autoaccept", tgbot.MatchTypeCommand, b.handleAutoAccept)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/status", tgbot.MatchTypeCommand, b.handleStatus)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/cleanup", tgbot.MatchTypeCommand, b.handleCleanup)
	b.TgBot.RegisterHandler(tgbot.HandlerTypeMessageText, "/ping", tgbot.MatchTypeCommand, b.handlePing)
}

func (b *Bot) handleStart(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	text := "<b>antigravity-telegram-remote Online</b>\n\n" +
		"Use /help for available commands.\n" +
		"Send any text message to forward it to Antigravity."
	_ = b.ReplyHTML(ctx, ch, text, nil)
}

func (b *Bot) handleHelp(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	text := "<b>📖 antigravity-telegram-remote Commands</b>\n\n" +
		"<b>💬 Chat</b>\n" +
		"/new — Start a new chat session\n" +
		"/chat — Show current session info\n\n" +
		"<b>⏹️ Control</b>\n" +
		"/stop — Interrupt active LLM generation\n" +
		"/screenshot — Capture Antigravity screen\n\n" +
		"<b>⚙️ Settings</b>\n" +
		"/mode — Display and change execution mode\n" +
		"/model — Display and change LLM model\n\n" +
		"<b>📁 Projects</b>\n" +
		"/project — Display project list\n\n" +
		"<b>📝 Templates</b>\n" +
		"/template — Show templates\n" +
		"/template_add — Register a template\n" +
		"/template_delete — Delete a template\n\n" +
		"<b>🔧 System</b>\n" +
		"/status — Display overall bot status\n" +
		"/autoaccept — Toggle auto-approve mode\n" +
		"/cleanup [days] — Clean up inactive sessions\n" +
		"/ping — Check latency\n\n" +
		"<i>Text messages are sent directly to Antigravity</i>"
	_ = b.ReplyHTML(ctx, ch, text, nil)
}

func (b *Bot) handleMode(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	replyFn := func(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error {
		return b.ReplyHTML(ctx, ch, text, keyboard)
	}
	var modeCdp ui.ModeCdpProvider
	if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
		client := b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
		if client != nil {
			modeCdp = modeCdpUIAdapter{client: client}
		}
	}
	_ = ui.SendModeUI(ctx, replyFn, modeServiceUIAdapter{svc: b.ModeService}, &ui.ModeUIDeps{
		GetCurrentCdp: func() ui.ModeCdpProvider { return modeCdp },
	})
}

func (b *Bot) handleModel(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
	var currentClient cdp.PoolClient
	if resolved != nil {
		currentClient = resolved.Cdp
	} else if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
		currentClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
	}

	parsed := ParseMessageContent(update.Message.Text)
	var modelName string
	if len(parsed.Args) > 0 {
		modelName = strings.TrimSpace(parsed.Args[0])
	}

	if modelName != "" {
		if currentClient == nil {
			_ = b.ReplyHTML(ctx, ch, "Not connected to CDP. Send a message first to connect.", nil)
			return
		}
		if c, ok := currentClient.(*cdp.Client); ok && c != nil {
			res, err := c.SetUIModel(modelName)
			if err == nil && res.OK {
				name := res.Model
				if name == "" {
					name = modelName
				}
				text := fmt.Sprintf("Model changed to <b>%s</b>.", ui.EscapeHTML(name))
				_ = b.ReplyHTML(ctx, ch, text, nil)
			} else {
				errMsg := "Failed to change model."
				if res.Error != "" {
					errMsg = res.Error
				} else if err != nil {
					errMsg = err.Error()
				}
				_ = b.ReplyHTML(ctx, ch, errMsg, nil)
			}
		} else {
			_ = b.ReplyHTML(ctx, ch, "Failed to change model.", nil)
		}
		return
	}

	replyFn := func(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error {
		return b.ReplyHTML(ctx, ch, text, keyboard)
	}

	var quotaFn func(ctx context.Context) ([]ui.ModelQuotaEntry, error)
	if b.Bridge != nil && b.Bridge.Quota != nil {
		quotaFn = func(ctx context.Context) ([]ui.ModelQuotaEntry, error) {
			res, err := b.Bridge.Quota.FetchQuota(ctx)
			if err != nil {
				return nil, err
			}
			if list, ok := res.([]ui.ModelQuotaEntry); ok {
				return list, nil
			}
			return nil, nil
		}
	}

	_ = ui.SendModelsUI(ctx, replyFn, ui.ModelsUIDeps{
		GetCurrentCdp: func() ui.ModelsCdpProvider {
			if currentClient != nil {
				return modelsCdpUIAdapter{client: currentClient}
			}
			return nil
		},
		FetchQuota: quotaFn,
	})
}

func (b *Bot) handleTemplate(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	var templates []ui.TemplateItem
	if b.TemplateRepo != nil {
		records, _ := b.TemplateRepo.FindAll()
		for _, rec := range records {
			templates = append(templates, ui.TemplateItem{
				ID:     int(rec.ID),
				Name:   rec.Name,
				Prompt: rec.Prompt,
			})
		}
	}
	replyFn := func(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error {
		return b.ReplyHTML(ctx, ch, text, keyboard)
	}
	_ = ui.SendTemplateUI(ctx, replyFn, templates)
}

func (b *Bot) handleTemplateAdd(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	parsed := ParseMessageContent(update.Message.Text)
	if len(parsed.Args) < 2 {
		_ = b.ReplyHTML(ctx, ch, "Usage: /template_add <name> <prompt>", nil)
		return
	}
	name := parsed.Args[0]
	prompt := strings.Join(parsed.Args[1:], " ")
	if b.SlashCommandHandler != nil {
		res := b.SlashCommandHandler.HandleCommand("template", []string{"add", name, prompt})
		_ = b.ReplyHTML(ctx, ch, res.Message, nil)
	}
}

func (b *Bot) handleTemplateDelete(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	parsed := ParseMessageContent(update.Message.Text)
	if len(parsed.Args) < 1 || parsed.Args[0] == "" {
		_ = b.ReplyHTML(ctx, ch, "Usage: /template_delete <name>", nil)
		return
	}
	name := parsed.Args[0]
	if b.SlashCommandHandler != nil {
		res := b.SlashCommandHandler.HandleCommand("template", []string{"delete", name})
		_ = b.ReplyHTML(ctx, ch, res.Message, nil)
	}
}

func (b *Bot) handleStatus(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	var activeNames []string
	if b.Pool != nil {
		activeNames = b.Pool.GetActiveWorkspaceNames()
	}
	currentMode := "fast"
	if b.ModeService != nil {
		currentMode = string(b.ModeService.GetCurrentMode())
	}
	modeDisplay := currentMode
	switch currentMode {
	case "fast":
		modeDisplay = "Fast"
	case "plan":
		modeDisplay = "Planning"
	}

	autoAcceptStatus := "⚪ OFF"
	if b.Bridge != nil && b.Bridge.AutoAccept != nil && b.Bridge.AutoAccept.IsEnabled() {
		autoAcceptStatus = "🟢 ON"
	}

	text := "<b>🔧 Bot Status</b>\n\n"
	if len(activeNames) > 0 {
		text += fmt.Sprintf("<b>CDP:</b> 🟢 %d project(s) connected\n", len(activeNames))
	} else {
		text += "<b>CDP:</b> ⚪ Disconnected\n"
	}
	text += fmt.Sprintf("<b>Mode:</b> %s\n", ui.EscapeHTML(modeDisplay))
	text += fmt.Sprintf("<b>Auto Approve:</b> %s\n", autoAcceptStatus)

	if len(activeNames) > 0 {
		text += "\n<b>Connected Projects:</b>\n"
		for _, name := range activeNames {
			contexts := 0
			if b.Pool != nil {
				cdpClient := b.Pool.GetConnected(name)
				if cdpClient != nil {
					contexts = len(cdpClient.GetContexts())
				}
			}
			text += fmt.Sprintf("• <b>%s</b> — Contexts: %d\n", ui.EscapeHTML(name), contexts)
		}
	} else {
		text += "\nSend a message to auto-connect to a project."
	}

	_ = b.ReplyHTML(ctx, ch, text, nil)
}

func (b *Bot) handleAutoAccept(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	parsed := ParseMessageContent(update.Message.Text)
	requested := ""
	if len(parsed.Args) > 0 {
		requested = strings.ToLower(strings.TrimSpace(parsed.Args[0]))
	}

	if (requested == "on" || requested == "off") && b.Bridge != nil && b.Bridge.AutoAccept != nil {
		res, err := b.Bridge.AutoAccept.Handle(requested)
		if err == nil {
			_ = b.ReplyHTML(ctx, ch, res.Message, nil)
			return
		}
	}

	replyFn := func(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error {
		return b.ReplyHTML(ctx, ch, text, keyboard)
	}
	if b.Bridge != nil && b.Bridge.AutoAccept != nil {
		_ = ui.SendAutoAcceptUI(ctx, replyFn, b.Bridge.AutoAccept)
	}
}

func (b *Bot) handleCleanup(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	parsed := ParseMessageContent(update.Message.Text)
	days := 7
	if len(parsed.Args) > 0 {
		if d, err := strconv.Atoi(parsed.Args[0]); err == nil && d > 0 {
			days = d
		}
	}

	guildID := fmt.Sprintf("%d", ch.ChatID)
	var inactive []InactiveSession
	if b.CleanupHandler != nil {
		inactive, _ = b.CleanupHandler.FindInactiveSessions(guildID, days)
	}

	if len(inactive) == 0 {
		_ = b.ReplyHTML(ctx, ch, fmt.Sprintf("No inactive sessions older than <b>%d</b> day(s).", days), nil)
		return
	}

	limit := 20
	if len(inactive) < limit {
		limit = len(inactive)
	}

	lines := make([]string, 0, limit)
	for _, item := range inactive[:limit] {
		label := item.Binding.WorkspacePath
		if item.Session != nil && item.Session.DisplayName != nil && *item.Session.DisplayName != "" {
			label = *item.Session.DisplayName
		}
		lines = append(lines, fmt.Sprintf("• %s", ui.EscapeHTML(label)))
	}

	extra := ""
	if len(inactive) > 20 {
		extra = fmt.Sprintf("\n…and %d more", len(inactive)-20)
	}

	kb := ui.NewInlineKeyboard().
		Text("📦 Archive", fmt.Sprintf("%s:%d", CleanupArchiveBtn, days)).
		Text("🗑 Delete", fmt.Sprintf("%s:%d", CleanupDeleteBtn, days)).
		Text("❌ Cancel", CleanupCancelBtn)

	text := fmt.Sprintf("<b>🧹 Cleanup</b>\n\nFound <b>%d</b> session(s) older than <b>%d</b> day(s):\n\n%s%s\n\nChoose an action:",
		len(inactive), days, strings.Join(lines, "\n"), extra)

	_ = b.ReplyHTML(ctx, ch, text, kb)
}

func (b *Bot) handleScreenshot(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	var cdpClient cdp.PoolClient
	if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
		cdpClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
	}

	sendPhotoFn := func(ctx context.Context, filename string, photo []byte, caption string) error {
		if b.TgBot == nil {
			return nil
		}
		params := &tgbot.SendPhotoParams{
			ChatID:  ch.ChatID,
			Caption: caption,
			Photo: &models.InputFileUpload{
				Filename: filename,
				Data:     bytes.NewReader(photo),
			},
		}
		if ch.ThreadID != 0 {
			params.MessageThreadID = ch.ThreadID
		}
		_, err := b.TgBot.SendPhoto(ctx, params)
		return err
	}

	replyFn := func(ctx context.Context, text string) error {
		return b.ReplyHTML(ctx, ch, text, nil)
	}

	var capturer ui.ScreenshotCapturer
	if cdpClient != nil {
		capturer = screenshotCapturerAdapter{client: cdpClient}
	}

	_ = ui.HandleScreenshot(ctx, sendPhotoFn, replyFn, capturer)
}

func (b *Bot) handleStop(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
	var cdpClient cdp.PoolClient
	if resolved != nil {
		cdpClient = resolved.Cdp
	} else if b.Pool != nil && b.Bridge != nil && b.Bridge.LastActiveWorkspace != nil {
		cdpClient = b.Pool.GetConnected(*b.Bridge.LastActiveWorkspace)
	}

	if cdpClient == nil {
		_ = b.ReplyHTML(ctx, ch, "⚠️ Not connected to CDP.", nil)
		return
	}

	callParams := map[string]any{
		"expression":    ClickStopButtonScript,
		"returnByValue": true,
		"awaitPromise":  false,
	}
	if ctxID := cdpClient.GetPrimaryContextID(); ctxID != nil {
		callParams["contextId"] = *ctxID
	}

	resRaw, err := cdpClient.Call("Runtime.evaluate", callParams)
	if err != nil {
		_ = b.ReplyHTML(ctx, ch, fmt.Sprintf("❌ Error during stop: %s", err.Error()), nil)
		return
	}

	var resp struct {
		Result struct {
			Value struct {
				Ok    bool   `json:"ok"`
				Error string `json:"error"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &resp); err == nil && resp.Result.Value.Ok {
		b.AddUserStopRequest(ChannelKey(ch))
		_ = b.ReplyHTML(ctx, ch, "<b>⏹️ Generation Interrupted</b>\nAI response generation was safely stopped.", nil)
	} else {
		errMsg := "Stop button not found."
		if resp.Result.Value.Error != "" {
			errMsg = resp.Result.Value.Error
		}
		_ = b.ReplyHTML(ctx, ch, fmt.Sprintf("<b>⚠️ Could Not Stop</b>\n%s", ui.EscapeHTML(errMsg)), nil)
	}
}

func (b *Bot) handleProject(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	var workspaces []string
	if b.WorkspaceService != nil {
		workspaces = b.WorkspaceService.ScanWorkspaces()
	}
	payload := ui.BuildProjectListUI(workspaces, 0)
	_ = b.ReplyHTML(ctx, ch, payload.Text, payload.Keyboard)
}

func (b *Bot) handleNew(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	key := ChannelKey(ch)

	var workspaceName string
	if b.ChatSessionRepo != nil {
		session, _ := b.ChatSessionRepo.FindByChannelId(key)
		if session != nil {
			workspaceName = session.WorkspacePath
		}
	}
	if workspaceName == "" && b.WorkspaceBindingRepo != nil {
		binding, _ := b.WorkspaceBindingRepo.FindByChannelId(key)
		if binding != nil {
			workspaceName = binding.WorkspacePath
		}
	}

	if workspaceName == "" {
		_ = b.ReplyHTML(ctx, ch, "⚠️ No project is bound to this chat. Use /project to select one.", nil)
		return
	}

	if b.WorkspaceService == nil || b.Pool == nil {
		_ = b.ReplyHTML(ctx, ch, "⚠️ Failed to connect: workspace or connection pool not initialized", nil)
		return
	}

	workspacePath, err := b.WorkspaceService.GetWorkspacePath(workspaceName)
	if err != nil {
		_ = b.ReplyHTML(ctx, ch, fmt.Sprintf("⚠️ Failed to connect: %s", err.Error()), nil)
		return
	}

	client, err := b.Pool.GetOrConnect(workspacePath)
	if err != nil {
		_ = b.ReplyHTML(ctx, ch, fmt.Sprintf("⚠️ Failed to connect: %s", err.Error()), nil)
		return
	}

	if b.ChatSessionService == nil {
		_ = b.ReplyHTML(ctx, ch, "⚠️ ChatSessionService not initialized", nil)
		return
	}

	chatResult := b.ChatSessionService.StartNewChat(client)
	if chatResult.OK {
		_ = b.ReplyHTML(ctx, ch, "<b>💬 New Chat Started</b>\nSend your message now.", nil)
	} else {
		_ = b.ReplyHTML(ctx, ch, fmt.Sprintf("⚠️ Could not start new chat: %s", chatResult.Error), nil)
	}
}

func (b *Bot) handleChat(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	key := ChannelKey(ch)

	var session *database.ChatSessionRecord
	if b.ChatSessionRepo != nil {
		session, _ = b.ChatSessionRepo.FindByChannelId(key)
	}

	if session == nil {
		var activeNames []string
		if b.Pool != nil {
			activeNames = b.Pool.GetActiveWorkspaceNames()
		}

		title := "(CDP Disconnected)"
		hasActiveChat := false

		if len(activeNames) > 0 && b.Pool != nil && b.ChatSessionService != nil {
			client := b.Pool.GetConnected(activeNames[0])
			if client != nil {
				info := b.ChatSessionService.GetCurrentSessionInfo(client)
				title = info.Title
				hasActiveChat = info.HasActiveChat
			}
		}

		statusStr := "⚪ Inactive"
		if hasActiveChat {
			statusStr = "🟢 Active"
		}

		text := fmt.Sprintf("<b>💬 Chat Session Info</b>\n\n<b>Title:</b> %s\n<b>Status:</b> %s\n\n<i>No project bound to this chat thread. Use /project to select one.</i>\n<code>Key: %s</code>",
			ui.EscapeHTML(title), statusStr, ui.EscapeHTML(key))
		_ = b.ReplyHTML(ctx, ch, text, nil)
		return
	}

	var allSessions []database.ChatSessionRecord
	if b.ChatSessionRepo != nil {
		allSessions, _ = b.ChatSessionRepo.FindByCategoryId(session.CategoryID)
	}

	lines := make([]string, 0, len(allSessions))
	for _, s := range allSessions {
		name := fmt.Sprintf("session-%d", s.SessionNumber)
		if s.DisplayName != nil && *s.DisplayName != "" {
			name = *s.DisplayName
		}
		current := ""
		if s.ChannelID == key {
			current = " ← Current"
		}
		lines = append(lines, fmt.Sprintf("• %s%s", name, current))
	}
	sessionList := strings.Join(lines, "\n")

	displayName := "(Unset)"
	if session.DisplayName != nil && *session.DisplayName != "" {
		displayName = *session.DisplayName
	}

	text := fmt.Sprintf("<b>💬 Chat Session Info</b>\n\n<b>Current:</b> #%d — %s\n<b>Project:</b> %s\n<b>Total sessions:</b> %d\n\n<b>Sessions:</b>\n%s",
		session.SessionNumber, ui.EscapeHTML(displayName), ui.EscapeHTML(session.WorkspacePath), len(allSessions), ui.EscapeHTML(sessionList))

	// ADR-005: If connected to CDP, attach session picker keyboard
	var kb *ui.InlineKeyboardMarkup
	if b.Pool != nil && b.ChatSessionService != nil {
		resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
		if resolved != nil && resolved.Cdp != nil {
			availableSessions := b.ChatSessionService.ListAllSessions(resolved.Cdp)
			if len(availableSessions) > 0 {
				sessionItems := make([]ui.SessionListItem, 0, len(availableSessions))
				for _, s := range availableSessions {
					sessionItems = append(sessionItems, ui.SessionListItem{
						Title:    s.Title,
						IsActive: s.IsActive,
					})
				}
				picker := ui.BuildSessionPickerUI(sessionItems)
				kb = picker.Keyboard
			}
		}
	}

	_ = b.ReplyHTML(ctx, ch, text, kb)
}

func (b *Bot) handleJoin(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	key := ChannelKey(ch)

	var workspaceName string
	if b.ChatSessionRepo != nil {
		session, _ := b.ChatSessionRepo.FindByChannelId(key)
		if session != nil {
			workspaceName = session.WorkspacePath
		}
	}
	if workspaceName == "" && b.WorkspaceBindingRepo != nil {
		binding, _ := b.WorkspaceBindingRepo.FindByChannelId(key)
		if binding != nil {
			workspaceName = binding.WorkspacePath
		}
	}

	if workspaceName == "" {
		_ = b.ReplyHTML(ctx, ch, "⚠️ No project is bound to this chat. Use /project to select one.", nil)
		return
	}

	resolved := b.ResolveWorkspaceAndCdp(ctx, ch)
	if resolved == nil || resolved.Cdp == nil || b.ChatSessionService == nil {
		_ = b.ReplyHTML(ctx, ch, "⚠️ Failed to connect to CDP.", nil)
		return
	}

	availableSessions := b.ChatSessionService.ListAllSessions(resolved.Cdp)
	sessionItems := make([]ui.SessionListItem, 0, len(availableSessions))
	for _, s := range availableSessions {
		sessionItems = append(sessionItems, ui.SessionListItem{
			Title:    s.Title,
			IsActive: s.IsActive,
		})
	}

	picker := ui.BuildSessionPickerUI(sessionItems)
	_ = b.ReplyHTML(ctx, ch, picker.Text, picker.Keyboard)
}

func (b *Bot) handlePing(ctx context.Context, botClient *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	ch := GetChannelFromMessage(update.Message.Chat.ID, update.Message.MessageThreadID)
	start := time.Now()
	params := &tgbot.SendMessageParams{
		ChatID: ch.ChatID,
		Text:   "🏓 Pong!",
	}
	if ch.ThreadID != 0 {
		params.MessageThreadID = ch.ThreadID
	}
	msg, err := botClient.SendMessage(ctx, params)
	if err != nil {
		return
	}
	latency := time.Since(start).Milliseconds()
	editParams := &tgbot.EditMessageTextParams{
		ChatID:    ch.ChatID,
		MessageID: msg.ID,
		Text:      fmt.Sprintf("🏓 Pong! Latency: <b>%dms</b>", latency),
		ParseMode: models.ParseModeHTML,
	}
	_, _ = botClient.EditMessageText(ctx, editParams)
}

// StartBot initializes configuration, SQLite WAL, launcher checks, services and starts the bot.
func StartBot(ctx context.Context, cliLogLevel *logger.LogLevel) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if cliLogLevel != nil {
		logger.SetLogLevel(*cliLogLevel)
	} else if cfg.LogLevel != "" {
		logger.SetLogLevel(logger.LogLevel(cfg.LogLevel))
	}

	dbPath := ":memory:"
	if os.Getenv("NODE_ENV") != "test" {
		dbPath = config.GetDefaultDbPath()
		_ = config.EnsureConfigDirExists()
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		logger.Warn(fmt.Sprintf("failed to set WAL mode: %v", err))
	}

	modeService := bridge.NewModeService()
	modelService := bridge.NewModelService()
	templateRepo, err := database.NewTemplateRepository(db)
	if err != nil {
		return fmt.Errorf("failed to initialize template repository: %w", err)
	}
	workspaceBindingRepo, err := database.NewWorkspaceBindingRepository(db)
	if err != nil {
		return fmt.Errorf("failed to initialize workspace binding repository: %w", err)
	}
	chatSessionRepo, err := database.NewChatSessionRepository(db)
	if err != nil {
		return fmt.Errorf("failed to initialize chat session repository: %w", err)
	}
	workspaceService := workspace.NewWorkspaceService(cfg.WorkspaceBaseDir)

	platform.EnsureAntigravityRunning()

	pool := cdp.NewConnectionPool(cdp.ClientOptions{})
	autoAccept := autoAcceptCheckerAdapter{svc: bridge.NewAutoAcceptService(cfg.AutoApproveFileEdits)}
	bridgePool := newBridgePoolAdapter(pool)
	bridgeInstance := bridge.InitCdpBridge(cfg.AutoApproveFileEdits, bridgePool, nil, autoAccept)
	bridgeInstance.BotToken = cfg.TelegramBotToken

	chatSessionService := cdp.NewChatSessionService()
	titleGenerator := bridge.NewTitleGeneratorService()
	promptDispatcher := bridge.NewPromptDispatcher(bridge.PromptDispatcherDeps{
		Bridge:       bridgeInstance,
		ModeService:  modeService,
		ModelService: modelService,
	})

	slashCommandHandler := NewSlashCommandHandler(templateRepo)
	cleanupHandler := NewCleanupCommandHandler(chatSessionRepo, workspaceBindingRepo)
	chatHandler := NewChatCommandHandler(chatSessionService, chatSessionRepo, workspaceBindingRepo, workspaceService, pool)
	workspaceHandler := NewWorkspaceCommandHandler(workspaceBindingRepo, chatSessionRepo, workspaceService)
	joinHandler := NewJoinCommandHandler(chatSessionService, chatSessionRepo, workspaceBindingRepo, pool, workspaceService)

	botInstance := NewBot(BotDeps{
		Config:               &cfg,
		DB:                   db,
		Bridge:               bridgeInstance,
		Pool:                 pool,
		ModeService:          modeService,
		ModelService:         modelService,
		TemplateRepo:         templateRepo,
		WorkspaceBindingRepo: workspaceBindingRepo,
		ChatSessionRepo:      chatSessionRepo,
		WorkspaceService:     workspaceService,
		ChatSessionService:   chatSessionService,
		TitleGenerator:       titleGenerator,
		PromptDispatcher:     promptDispatcher,
		SlashCommandHandler:  slashCommandHandler,
		CleanupHandler:       cleanupHandler,
		ChatHandler:          chatHandler,
		WorkspaceHandler:     workspaceHandler,
		JoinHandler:          joinHandler,
	})

	promptDispatcher.SetSendPromptImpl(func(
		c context.Context,
		bridgeRef any,
		ch bridge.TelegramChannel,
		prompt string,
		cdpEval bridge.CdpEvaluator,
		modeSvc *bridge.ModeService,
		modelSvc *bridge.ModelService,
		inboundImages []bridge.InboundImageAttachment,
		options *bridge.PromptDispatchOptions,
	) error {
		threadID := 0
		if ch.ThreadID != nil {
			threadID = *ch.ThreadID
		}
		targetCh := TelegramChannel{
			ChatID:   ch.ChatID,
			ThreadID: threadID,
		}

		botInbound := make([]InboundImageAttachment, 0, len(inboundImages))
		for _, img := range inboundImages {
			botInbound = append(botInbound, InboundImageAttachment{
				LocalPath: img.LocalPath,
				URL:       img.URL,
				Name:      img.FileName,
				MimeType:  img.MimeType,
			})
		}

		var promptOpts *PromptSendOptions
		if options != nil {
			var csSvc *cdp.ChatSessionService
			if s, ok := options.ChatSessionService.(*cdp.ChatSessionService); ok {
				csSvc = s
			}
			var tm *TelegramTopicManager
			if t, ok := options.TopicManager.(*TelegramTopicManager); ok {
				tm = t
			}
			promptOpts = &PromptSendOptions{
				ChatSessionService: csSvc,
				ChatSessionRepo:    options.ChatSessionRepo,
				TopicManager:       tm,
				TitleGenerator:     options.TitleGenerator,
			}
		}

		var cdpSvc CdpServiceClient
		if clientAdapter, ok := cdpEval.(*cdpServiceWithContextsAdapter); ok && clientAdapter != nil {
			if poolClient, ok := clientAdapter.client.(*cdp.Client); ok {
				cdpSvc = poolClient
			} else if c, ok := clientAdapter.client.(CdpServiceClient); ok {
				cdpSvc = c
			}
		}

		return botInstance.SendPromptToAntigravity(c, targetCh, prompt, cdpSvc, botInbound, promptOpts)
	})

	if cfg.TelegramBotToken != "" {
		botClient, err := tgbot.New(
			cfg.TelegramBotToken,
			tgbot.WithMiddlewares(botInstance.TelegramAuthMiddleware),
		)
		if err != nil {
			return fmt.Errorf("failed to create bot client: %w", err)
		}
		botInstance.TgBot = botClient
		botInstance.TopicManager = NewTelegramTopicManager(NewBotForumTopicAPI(botClient), 0)
		botInstance.RegisterCommands()
		botInstance.RegisterMessageHandlers()
		botInstance.SetupPoolEventListeners(pool)

		logger.Info("Starting antigravity-telegram-remote Telegram bot...")

		shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			<-shutdownCtx.Done()
			logger.Info("Shutting down bot...")
		}()

		_ = botInstance.RegisterBotCommands(shutdownCtx)
		botClient.Start(shutdownCtx)
	}

	return nil
}
