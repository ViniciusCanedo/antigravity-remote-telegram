package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Constants for action callback_data prefixes and pool defaults.
const (
	// Action prefixes for Approval buttons.
	ApproveActionPrefix     = "approve_action"
	AlwaysAllowActionPrefix = "always_allow_action"
	DenyActionPrefix        = "deny_action"

	// Action prefixes for Planning buttons.
	PlanningOpenActionPrefix    = "planning_open_action"
	PlanningProceedActionPrefix = "planning_proceed_action"

	// Action prefixes for Error Popup buttons.
	ErrorPopupDismissActionPrefix   = "error_popup_dismiss_action"
	ErrorPopupCopyDebugActionPrefix = "error_popup_copy_debug_action"
	ErrorPopupRetryActionPrefix     = "error_popup_retry_action"

	// Defaults for CDP connection pool initialization.
	DefaultPoolCdpCallTimeout       = 15000 * time.Millisecond
	DefaultPoolMaxReconnectAttempts = 3
	DefaultPoolReconnectDelay       = 3000 * time.Millisecond
)

// GetCurrentChatTitleScript extracts the active chat/session title from the Antigravity side panel.
// Source: src/services/cdpBridgeManager.ts lines 51-60.
const GetCurrentChatTitleScript = `(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    if (!panel) return '';
    const header = panel.querySelector('div[class*="border-b"]');
    if (!header) return '';
    const titleEl = header.querySelector('div[class*="text-ellipsis"]');
    const title = titleEl ? (titleEl.textContent || '').trim() : '';
    if (!title || title === 'Agent') return '';
    return title;
})()`

// TelegramChannel represents a Telegram chat target: chat_id + optional message_thread_id.
// Source: interface TelegramChannel (src/services/cdpBridgeManager.ts lines 17-20).
type TelegramChannel struct {
	ChatID   int64 `json:"chatId"`
	ThreadID *int  `json:"threadId,omitempty"`
}

// ExecutionContextInfo represents a JavaScript execution context info returned by CDP.
type ExecutionContextInfo struct {
	ID int64 `json:"id"`
}

// CdpServiceWithContexts extends CdpEvaluator with a method returning all active execution contexts.
type CdpServiceWithContexts interface {
	CdpEvaluator
	GetContexts() []ExecutionContextInfo
}

// BridgeConnectionPool defines the connection pool methods consumed by CdpBridgeManager.
type BridgeConnectionPool interface {
	GetConnected(workspaceName string) CdpServiceWithContexts
	GetActiveWorkspaceNames() []string
	GetApprovalDetector(projectName string) *ApprovalDetector
	RegisterApprovalDetector(projectName string, detector *ApprovalDetector)
	GetPlanningDetector(projectName string) *PlanningDetector
	RegisterPlanningDetector(projectName string, detector *PlanningDetector)
	GetErrorPopupDetector(projectName string) *ErrorPopupDetector
	RegisterErrorPopupDetector(projectName string, detector *ErrorPopupDetector)
	GetUserMessageDetector(projectName string) *UserMessageDetector
	RegisterUserMessageDetector(projectName string, detector *UserMessageDetector)
}

// InlineButton represents a Telegram inline keyboard button.
type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// InlineKeyboardMarkup represents a Telegram inline keyboard markup.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineButton `json:"inline_keyboard"`
}

// TelegramBotMessenger abstracts sending and editing Telegram messages.
type TelegramBotMessenger interface {
	SendMessage(ctx context.Context, chatID int64, threadID *int, htmlText string, keyboard *InlineKeyboardMarkup) (int, error)
	ClearMessageReplyMarkup(ctx context.Context, chatID int64, messageID int) error
}

// AutoAcceptChecker abstracts the AutoAcceptService.
type AutoAcceptChecker interface {
	IsEnabled() bool
	Handle(mode string) (AutoAcceptResult, error)
}

// AutoAcceptResult mirrors the return value of AutoAcceptService.handle().
type AutoAcceptResult struct {
	Success bool
	Enabled bool
	Changed bool
	Message string
}

// QuotaFetcher abstracts the QuotaService.
type QuotaFetcher interface {
	FetchQuota(ctx context.Context) (any, error)
}

// PlanNotificationBuilder builds the HTML text and inline keyboard for a planning notification.
type PlanNotificationBuilder func(info PlanningInfo, projectName string, targetChannelStr string) (text string, keyboard *InlineKeyboardMarkup)

// CdpBridge manages shared CDP state, background detectors, and session/workspace channel routing.
// Source: interface CdpBridge (src/services/cdpBridgeManager.ts lines 22-32).
type CdpBridge struct {
	mu                         sync.RWMutex
	Pool                       BridgeConnectionPool
	Quota                      QuotaFetcher
	AutoAccept                 AutoAcceptChecker
	LastActiveWorkspace        *string
	LastActiveChannel          *TelegramChannel
	ApprovalChannelByWorkspace map[string]TelegramChannel
	ApprovalChannelBySession   map[string]TelegramChannel
	BotApi                     TelegramBotMessenger
	BotToken                   string
	PlanUIBuilder              PlanNotificationBuilder
	TranslateFunc              func(key string) string
	Logger                     *slog.Logger
}

// ParsedApprovalCustomID represents the parsed outcome of an approval callback_data.
type ParsedApprovalCustomID struct {
	Action      string  // "approve" | "always_allow" | "deny"
	ProjectName *string // nil when absent
	ChannelID   *string // nil when absent
}

// ParsedPlanningCustomID represents the parsed outcome of a planning callback_data.
type ParsedPlanningCustomID struct {
	Action      string  // "open" | "proceed"
	ProjectName *string // nil when absent
	ChannelID   *string // nil when absent
}

// ParsedErrorPopupCustomID represents the parsed outcome of an error popup callback_data.
type ParsedErrorPopupCustomID struct {
	Action      string  // "dismiss" | "copy_debug" | "retry"
	ProjectName *string // nil when absent
	ChannelID   *string // nil when absent
}

// InitCdpBridge initializes a new CdpBridge state.
// Source: src/services/cdpBridgeManager.ts lines 188-209.
func InitCdpBridge(autoApproveDefault bool, pool BridgeConnectionPool, quota QuotaFetcher, autoAccept AutoAcceptChecker) *CdpBridge {
	return &CdpBridge{
		Pool:                       pool,
		Quota:                      quota,
		AutoAccept:                 autoAccept,
		LastActiveWorkspace:        nil,
		LastActiveChannel:          nil,
		ApprovalChannelByWorkspace: make(map[string]TelegramChannel),
		ApprovalChannelBySession:   make(map[string]TelegramChannel),
		BotApi:                     nil,
		BotToken:                   "",
		Logger:                     slog.Default(),
	}
}

// normalizeSessionTitle trims and lowercases a session title.
func normalizeSessionTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

// buildSessionRouteKey creates a routing key composed of project name and normalized session title.
func buildSessionRouteKey(projectName, sessionTitle string) string {
	return fmt.Sprintf("%s::%s", projectName, normalizeSessionTitle(sessionTitle))
}

// escapeHTML escapes HTML special characters for Telegram HTML mode.
func escapeHTML(text string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(text)
}

// GetCurrentChatTitle extracts the current active chat title from the Antigravity window.
// Source: src/services/cdpBridgeManager.ts lines 62-78.
func GetCurrentChatTitle(ctx context.Context, cdp CdpServiceWithContexts) *string {
	contexts := cdp.GetContexts()
	for _, c := range contexts {
		callParams := map[string]any{
			"expression":    GetCurrentChatTitleScript,
			"returnByValue": true,
			"contextId":     c.ID,
		}
		raw, err := cdp.Call(ctx, "Runtime.evaluate", callParams)
		if err != nil {
			continue
		}
		var resp runtimeEvaluateResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			continue
		}
		if len(resp.Result.Value) > 0 && string(resp.Result.Value) != "null" {
			var val string
			if err := json.Unmarshal(resp.Result.Value, &val); err == nil {
				val = strings.TrimSpace(val)
				if val != "" && val != "Agent" {
					return &val
				}
			}
		}
	}
	return nil
}

// RegisterApprovalWorkspaceChannel associates a default TelegramChannel with a workspace.
// Source: src/services/cdpBridgeManager.ts lines 80-86.
func RegisterApprovalWorkspaceChannel(bridge *CdpBridge, projectName string, channel TelegramChannel) {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	bridge.ApprovalChannelByWorkspace[projectName] = channel
}

// RegisterApprovalSessionChannel associates a TelegramChannel with a specific workspace session title,
// and also updates the workspace fallback channel.
// Source: src/services/cdpBridgeManager.ts lines 88-97.
func RegisterApprovalSessionChannel(bridge *CdpBridge, projectName string, sessionTitle string, channel TelegramChannel) {
	if strings.TrimSpace(sessionTitle) == "" {
		return
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	bridge.ApprovalChannelBySession[buildSessionRouteKey(projectName, sessionTitle)] = channel
	bridge.ApprovalChannelByWorkspace[projectName] = channel
}

// ResolveApprovalChannelForCurrentChat resolves the TelegramChannel to notify for a given workspace
// and optional current chat title. Prefers session channel over workspace channel.
// Source: src/services/cdpBridgeManager.ts lines 99-110.
func ResolveApprovalChannelForCurrentChat(bridge *CdpBridge, projectName string, currentChatTitle *string) *TelegramChannel {
	bridge.mu.RLock()
	defer bridge.mu.RUnlock()

	if currentChatTitle != nil && strings.TrimSpace(*currentChatTitle) != "" {
		key := buildSessionRouteKey(projectName, *currentChatTitle)
		if ch, ok := bridge.ApprovalChannelBySession[key]; ok {
			chCopy := ch
			return &chCopy
		}
	}

	if ch, ok := bridge.ApprovalChannelByWorkspace[projectName]; ok {
		chCopy := ch
		return &chCopy
	}

	return nil
}

// BuildApprovalCustomID builds callback_data for approval inline buttons.
// Source: src/services/cdpBridgeManager.ts lines 112-126.
func BuildApprovalCustomID(action string, projectName string, channelID ...string) string {
	prefix := ApproveActionPrefix
	switch action {
	case "always_allow":
		prefix = AlwaysAllowActionPrefix
	case "deny":
		prefix = DenyActionPrefix
	}

	if len(channelID) > 0 && strings.TrimSpace(channelID[0]) != "" {
		return fmt.Sprintf("%s:%s:%s", prefix, projectName, strings.TrimSpace(channelID[0]))
	}
	return fmt.Sprintf("%s:%s", prefix, projectName)
}

// ParseApprovalCustomID parses an approval custom_id string.
// Source: src/services/cdpBridgeManager.ts lines 128-138.
func ParseApprovalCustomID(customID string) *ParsedApprovalCustomID {
	pairs := []struct {
		action string
		prefix string
	}{
		{"approve", ApproveActionPrefix},
		{"always_allow", AlwaysAllowActionPrefix},
		{"deny", DenyActionPrefix},
	}

	for _, p := range pairs {
		if customID == p.prefix {
			return &ParsedApprovalCustomID{Action: p.action, ProjectName: nil, ChannelID: nil}
		}
		prefixWithColon := p.prefix + ":"
		if strings.HasPrefix(customID, prefixWithColon) {
			rest := customID[len(prefixWithColon):]
			parts := strings.Split(rest, ":")
			var proj *string
			if len(parts) > 0 && parts[0] != "" {
				proj = &parts[0]
			}
			var ch *string
			if len(parts) > 1 && parts[1] != "" {
				ch = &parts[1]
			}
			return &ParsedApprovalCustomID{Action: p.action, ProjectName: proj, ChannelID: ch}
		}
	}
	return nil
}

// BuildPlanningCustomID builds callback_data for planning inline buttons.
// Source: src/services/cdpBridgeManager.ts lines 140-148.
func BuildPlanningCustomID(action string, projectName string, channelID ...string) string {
	prefix := PlanningOpenActionPrefix
	if action == "proceed" {
		prefix = PlanningProceedActionPrefix
	}

	if len(channelID) > 0 && strings.TrimSpace(channelID[0]) != "" {
		return fmt.Sprintf("%s:%s:%s", prefix, projectName, strings.TrimSpace(channelID[0]))
	}
	return fmt.Sprintf("%s:%s", prefix, projectName)
}

// ParsePlanningCustomID parses a planning custom_id string.
// Source: src/services/cdpBridgeManager.ts lines 150-160.
func ParsePlanningCustomID(customID string) *ParsedPlanningCustomID {
	pairs := []struct {
		action string
		prefix string
	}{
		{"open", PlanningOpenActionPrefix},
		{"proceed", PlanningProceedActionPrefix},
	}

	for _, p := range pairs {
		if customID == p.prefix {
			return &ParsedPlanningCustomID{Action: p.action, ProjectName: nil, ChannelID: nil}
		}
		prefixWithColon := p.prefix + ":"
		if strings.HasPrefix(customID, prefixWithColon) {
			rest := customID[len(prefixWithColon):]
			parts := strings.Split(rest, ":")
			var proj *string
			if len(parts) > 0 && parts[0] != "" {
				proj = &parts[0]
			}
			var ch *string
			if len(parts) > 1 && parts[1] != "" {
				ch = &parts[1]
			}
			return &ParsedPlanningCustomID{Action: p.action, ProjectName: proj, ChannelID: ch}
		}
	}
	return nil
}

// BuildErrorPopupCustomID builds callback_data for error popup inline buttons.
// Source: src/services/cdpBridgeManager.ts lines 162-174.
func BuildErrorPopupCustomID(action string, projectName string, channelID ...string) string {
	prefix := ErrorPopupDismissActionPrefix
	switch action {
	case "copy_debug":
		prefix = ErrorPopupCopyDebugActionPrefix
	case "retry":
		prefix = ErrorPopupRetryActionPrefix
	}

	if len(channelID) > 0 && strings.TrimSpace(channelID[0]) != "" {
		return fmt.Sprintf("%s:%s:%s", prefix, projectName, strings.TrimSpace(channelID[0]))
	}
	return fmt.Sprintf("%s:%s", prefix, projectName)
}

// ParseErrorPopupCustomID parses an error popup custom_id string.
// Source: src/services/cdpBridgeManager.ts lines 176-186.
func ParseErrorPopupCustomID(customID string) *ParsedErrorPopupCustomID {
	pairs := []struct {
		action string
		prefix string
	}{
		{"dismiss", ErrorPopupDismissActionPrefix},
		{"copy_debug", ErrorPopupCopyDebugActionPrefix},
		{"retry", ErrorPopupRetryActionPrefix},
	}

	for _, p := range pairs {
		if customID == p.prefix {
			return &ParsedErrorPopupCustomID{Action: p.action, ProjectName: nil, ChannelID: nil}
		}
		prefixWithColon := p.prefix + ":"
		if strings.HasPrefix(customID, prefixWithColon) {
			rest := customID[len(prefixWithColon):]
			parts := strings.Split(rest, ":")
			var proj *string
			if len(parts) > 0 && parts[0] != "" {
				proj = &parts[0]
			}
			var ch *string
			if len(parts) > 1 && parts[1] != "" {
				ch = &parts[1]
			}
			return &ParsedErrorPopupCustomID{Action: p.action, ProjectName: proj, ChannelID: ch}
		}
	}
	return nil
}

// GetCurrentCdp returns the active connected CdpService from the bridge, falling back to any connected workspace.
// Source: src/services/cdpBridgeManager.ts lines 211-225.
func GetCurrentCdp(bridge *CdpBridge) CdpServiceWithContexts {
	bridge.mu.RLock()
	lastWs := bridge.LastActiveWorkspace
	pool := bridge.Pool
	logger := bridge.Logger
	bridge.mu.RUnlock()

	if pool == nil {
		return nil
	}

	if lastWs != nil && *lastWs != "" {
		cdp := pool.GetConnected(*lastWs)
		if cdp != nil {
			return cdp
		}
	}

	// Fallback: return any connected workspace
	activeNames := pool.GetActiveWorkspaceNames()
	if len(activeNames) > 0 {
		fallbackName := activeNames[0]
		if logger != nil {
			lastStr := "(none)"
			if lastWs != nil {
				lastStr = *lastWs
			}
			logger.Debug(fmt.Sprintf(`[CdpBridge] Workspace "%s" not connected. Falling back to "%s"`, lastStr, fallbackName))
		}
		return pool.GetConnected(fallbackName)
	}

	if logger != nil {
		logger.Warn("[CdpBridge] No connected workspaces found.")
	}
	return nil
}

// EnsureApprovalDetector ensures an ApprovalDetector is running and registered for projectName.
// Source: src/services/cdpBridgeManager.ts lines 246-314.
func EnsureApprovalDetector(bridge *CdpBridge, cdp CdpServiceWithContexts, projectName string) {
	if bridge.Pool == nil {
		return
	}
	existing := bridge.Pool.GetApprovalDetector(projectName)
	if existing != nil && existing.IsActive() {
		return
	}

	var (
		msgMu             sync.Mutex
		lastMessageID     *int
		lastMessageChatID *int64
	)

	var detector *ApprovalDetector
	detector = NewApprovalDetector(ApprovalDetectorOptions{
		CdpService:   cdp,
		PollInterval: 2000 * time.Millisecond,
		Logger:       bridge.Logger,
		OnResolved: func() {
			msgMu.Lock()
			if lastMessageID == nil || lastMessageChatID == nil || bridge.BotApi == nil {
				msgMu.Unlock()
				return
			}
			msgID := *lastMessageID
			chatID := *lastMessageChatID
			lastMessageID = nil
			lastMessageChatID = nil
			msgMu.Unlock()

			ctx := context.Background()
			_ = bridge.BotApi.ClearMessageReplyMarkup(ctx, chatID, msgID)
		},
		OnApprovalRequired: func(info ApprovalInfo) {
			ctx := context.Background()
			if bridge.Logger != nil {
				bridge.Logger.Debug(fmt.Sprintf("[ApprovalDetector:%s] Approval detected", projectName))
			}

			currentChatTitle := GetCurrentChatTitle(ctx, cdp)
			targetChannel := ResolveApprovalChannelForCurrentChat(bridge, projectName, currentChatTitle)

			if targetChannel == nil || bridge.BotApi == nil {
				if bridge.Logger != nil {
					bridge.Logger.Warn(fmt.Sprintf("[ApprovalDetector:%s] Skipped -- no target channel", projectName))
				}
				return
			}

			var targetChannelStr string
			if targetChannel.ThreadID != nil {
				targetChannelStr = strconv.Itoa(*targetChannel.ThreadID)
			} else {
				targetChannelStr = strconv.FormatInt(targetChannel.ChatID, 10)
			}

			if bridge.AutoAccept != nil && bridge.AutoAccept.IsEnabled() {
				accepted, _ := detector.AlwaysAllowButton(ctx)
				if !accepted {
					accepted, _ = detector.ApproveButton(ctx)
				}
				var text string
				if accepted {
					text = fmt.Sprintf("✅ <b>Auto-approved</b>\nAn action was automatically approved.\n<b>Workspace:</b> %s", escapeHTML(projectName))
				} else {
					text = fmt.Sprintf("⚠️ <b>Auto-approve failed</b>\nManual approval required.\n<b>Workspace:</b> %s", escapeHTML(projectName))
				}
				_, _ = bridge.BotApi.SendMessage(ctx, targetChannel.ChatID, targetChannel.ThreadID, text, nil)
				if accepted {
					return
				}
			}

			var b strings.Builder
			b.WriteString("🔔 <b>Approval Required</b>\n\n")
			if info.Description != "" {
				b.WriteString(escapeHTML(info.Description))
				b.WriteString("\n\n")
			}
			b.WriteString("<b>Allow:</b> ")
			b.WriteString(escapeHTML(info.ApproveText))
			b.WriteString("\n")
			b.WriteString("<b>Allow Chat:</b> ")
			if info.AlwaysAllowText != "" {
				b.WriteString(escapeHTML(info.AlwaysAllowText))
			} else {
				b.WriteString("In Dropdown")
			}
			b.WriteString("\n")
			b.WriteString("<b>Deny:</b> ")
			if info.DenyText != "" {
				b.WriteString(escapeHTML(info.DenyText))
			} else {
				b.WriteString("(None)")
			}
			b.WriteString("\n")
			b.WriteString("<b>Workspace:</b> ")
			b.WriteString(escapeHTML(projectName))

			keyboard := &InlineKeyboardMarkup{
				InlineKeyboard: [][]InlineButton{
					{
						{Text: "✅ Allow", CallbackData: BuildApprovalCustomID("approve", projectName, targetChannelStr)},
						{Text: "✅ Allow Chat", CallbackData: BuildApprovalCustomID("always_allow", projectName, targetChannelStr)},
						{Text: "❌ Deny", CallbackData: BuildApprovalCustomID("deny", projectName, targetChannelStr)},
					},
				},
			}

			msgID, err := bridge.BotApi.SendMessage(ctx, targetChannel.ChatID, targetChannel.ThreadID, b.String(), keyboard)
			if err == nil && msgID > 0 {
				msgMu.Lock()
				lastMessageID = &msgID
				chatIDCopy := targetChannel.ChatID
				lastMessageChatID = &chatIDCopy
				msgMu.Unlock()
			}
		},
	})

	detector.Start()
	bridge.Pool.RegisterApprovalDetector(projectName, detector)
	if bridge.Logger != nil {
		bridge.Logger.Debug(fmt.Sprintf("[ApprovalDetector:%s] Started", projectName))
	}
}

// EnsurePlanningDetector ensures a PlanningDetector is running and registered for projectName.
// Source: src/services/cdpBridgeManager.ts lines 316-362.
func EnsurePlanningDetector(bridge *CdpBridge, cdp CdpServiceWithContexts, projectName string) {
	if bridge.Pool == nil {
		return
	}
	existing := bridge.Pool.GetPlanningDetector(projectName)
	if existing != nil && existing.IsActive() {
		return
	}

	var (
		msgMu             sync.Mutex
		lastMessageID     *int
		lastMessageChatID *int64
	)

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService:   cdp,
		PollInterval: 2000 * time.Millisecond,
		Logger:       bridge.Logger,
		OnResolved: func() {
			msgMu.Lock()
			if lastMessageID == nil || lastMessageChatID == nil || bridge.BotApi == nil {
				msgMu.Unlock()
				return
			}
			msgID := *lastMessageID
			chatID := *lastMessageChatID
			lastMessageID = nil
			lastMessageChatID = nil
			msgMu.Unlock()

			ctx := context.Background()
			_ = bridge.BotApi.ClearMessageReplyMarkup(ctx, chatID, msgID)
		},
		OnPlanningRequired: func(info PlanningInfo) {
			ctx := context.Background()
			if bridge.Logger != nil {
				bridge.Logger.Debug(fmt.Sprintf("[PlanningDetector:%s] Planning detected", projectName))
			}

			currentChatTitle := GetCurrentChatTitle(ctx, cdp)
			targetChannel := ResolveApprovalChannelForCurrentChat(bridge, projectName, currentChatTitle)

			if targetChannel == nil || bridge.BotApi == nil {
				return
			}

			var targetChannelStr string
			if targetChannel.ThreadID != nil {
				targetChannelStr = strconv.Itoa(*targetChannel.ThreadID)
			} else {
				targetChannelStr = strconv.FormatInt(targetChannel.ChatID, 10)
			}

			var text string
			var keyboard *InlineKeyboardMarkup
			if bridge.PlanUIBuilder != nil {
				text, keyboard = bridge.PlanUIBuilder(info, projectName, targetChannelStr)
			} else {
				text = fmt.Sprintf("📋 <b>Plan Available</b>\n\n<b>Title:</b> %s\n<b>Workspace:</b> %s", escapeHTML(info.PlanTitle), escapeHTML(projectName))
				keyboard = &InlineKeyboardMarkup{
					InlineKeyboard: [][]InlineButton{
						{
							{Text: "📖 Open", CallbackData: BuildPlanningCustomID("open", projectName, targetChannelStr)},
							{Text: "▶️ Proceed", CallbackData: BuildPlanningCustomID("proceed", projectName, targetChannelStr)},
						},
					},
				}
			}

			msgID, err := bridge.BotApi.SendMessage(ctx, targetChannel.ChatID, targetChannel.ThreadID, text, keyboard)
			if err == nil && msgID > 0 {
				msgMu.Lock()
				lastMessageID = &msgID
				chatIDCopy := targetChannel.ChatID
				lastMessageChatID = &chatIDCopy
				msgMu.Unlock()
			}
		},
	})

	detector.Start()
	bridge.Pool.RegisterPlanningDetector(projectName, detector)
	if bridge.Logger != nil {
		bridge.Logger.Debug(fmt.Sprintf("[PlanningDetector:%s] Started", projectName))
	}
}

// EnsureErrorPopupDetector ensures an ErrorPopupDetector is running and registered for projectName.
// Source: src/services/cdpBridgeManager.ts lines 364-419.
func EnsureErrorPopupDetector(bridge *CdpBridge, cdp CdpServiceWithContexts, projectName string) {
	if bridge.Pool == nil {
		return
	}
	existing := bridge.Pool.GetErrorPopupDetector(projectName)
	if existing != nil && existing.IsActive() {
		return
	}

	var (
		msgMu             sync.Mutex
		lastMessageID     *int
		lastMessageChatID *int64
	)

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService:   cdp,
		PollInterval: 3000 * time.Millisecond,
		Logger:       bridge.Logger,
		OnResolved: func() {
			msgMu.Lock()
			if lastMessageID == nil || lastMessageChatID == nil || bridge.BotApi == nil {
				msgMu.Unlock()
				return
			}
			msgID := *lastMessageID
			chatID := *lastMessageChatID
			lastMessageID = nil
			lastMessageChatID = nil
			msgMu.Unlock()

			ctx := context.Background()
			_ = bridge.BotApi.ClearMessageReplyMarkup(ctx, chatID, msgID)
		},
		OnErrorPopup: func(info ErrorPopupInfo) {
			ctx := context.Background()
			if bridge.Logger != nil {
				bridge.Logger.Debug(fmt.Sprintf("[ErrorPopupDetector:%s] Error popup detected", projectName))
			}

			currentChatTitle := GetCurrentChatTitle(ctx, cdp)
			targetChannel := ResolveApprovalChannelForCurrentChat(bridge, projectName, currentChatTitle)

			if targetChannel == nil || bridge.BotApi == nil {
				return
			}

			var targetChannelStr string
			if targetChannel.ThreadID != nil {
				targetChannelStr = strconv.Itoa(*targetChannel.ThreadID)
			} else {
				targetChannelStr = strconv.FormatInt(targetChannel.ChatID, 10)
			}

			bodyText := info.Body
			if bodyText == "" {
				if bridge.TranslateFunc != nil {
					bodyText = bridge.TranslateFunc("An error occurred in the Antigravity agent.")
				} else {
					bodyText = "An error occurred in the Antigravity agent."
				}
			}

			titleText := info.Title
			if titleText == "" {
				titleText = "Agent Error"
			}

			runes := []rune(bodyText)
			if len(runes) > ErrorPopupTelegramBodyMaxLen {
				bodyText = string(runes[:ErrorPopupTelegramBodyMaxLen])
			}

			buttonsText := strings.Join(info.Buttons, ", ")
			if buttonsText == "" {
				buttonsText = "(None)"
			}

			var b strings.Builder
			b.WriteString(fmt.Sprintf("❌ <b>%s</b>\n\n", escapeHTML(titleText)))
			b.WriteString(escapeHTML(bodyText))
			b.WriteString("\n\n")
			b.WriteString(fmt.Sprintf("<b>Buttons:</b> %s\n", escapeHTML(buttonsText)))
			b.WriteString(fmt.Sprintf("<b>Workspace:</b> %s", escapeHTML(projectName)))

			keyboard := &InlineKeyboardMarkup{
				InlineKeyboard: [][]InlineButton{
					{
						{Text: "🔇 Dismiss", CallbackData: BuildErrorPopupCustomID("dismiss", projectName, targetChannelStr)},
						{Text: "📋 Copy Debug", CallbackData: BuildErrorPopupCustomID("copy_debug", projectName, targetChannelStr)},
						{Text: "🔄 Retry", CallbackData: BuildErrorPopupCustomID("retry", projectName, targetChannelStr)},
					},
				},
			}

			msgID, err := bridge.BotApi.SendMessage(ctx, targetChannel.ChatID, targetChannel.ThreadID, b.String(), keyboard)
			if err == nil && msgID > 0 {
				msgMu.Lock()
				lastMessageID = &msgID
				chatIDCopy := targetChannel.ChatID
				lastMessageChatID = &chatIDCopy
				msgMu.Unlock()
			}
		},
	})

	detector.Start()
	bridge.Pool.RegisterErrorPopupDetector(projectName, detector)
	if bridge.Logger != nil {
		bridge.Logger.Debug(fmt.Sprintf("[ErrorPopupDetector:%s] Started", projectName))
	}
}

// EnsureUserMessageDetector ensures a UserMessageDetector is running and registered for projectName.
// Source: src/services/cdpBridgeManager.ts lines 421-439.
func EnsureUserMessageDetector(
	bridge *CdpBridge,
	cdp CdpServiceWithContexts,
	projectName string,
	onUserMessage func(info UserMessageInfo),
) {
	if bridge.Pool == nil {
		return
	}
	existing := bridge.Pool.GetUserMessageDetector(projectName)
	if existing != nil && existing.IsActive() {
		return
	}

	detector := NewUserMessageDetector(UserMessageDetectorOptions{
		CdpService:    cdp,
		PollInterval:  2000 * time.Millisecond,
		OnUserMessage: onUserMessage,
		Logger:        bridge.Logger,
	})

	detector.Start()
	bridge.Pool.RegisterUserMessageDetector(projectName, detector)
	if bridge.Logger != nil {
		bridge.Logger.Debug(fmt.Sprintf("[UserMessageDetector:%s] Started", projectName))
	}
}
