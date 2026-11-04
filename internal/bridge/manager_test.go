package bridge

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// Helper for string pointers.
func ptrString(s string) *string {
	return &s
}

// Helper for int pointers.
func ptrInt(i int) *int {
	return &i
}

// mockCdpWithContexts implements CdpServiceWithContexts.
type mockCdpWithContexts struct {
	mockCdpEvaluator
	contexts []ExecutionContextInfo
}

func (m *mockCdpWithContexts) GetContexts() []ExecutionContextInfo {
	return m.contexts
}

// mockBridgeConnectionPool implements BridgeConnectionPool for testing.
type mockBridgeConnectionPool struct {
	mu                  sync.Mutex
	connected           map[string]CdpServiceWithContexts
	activeWorkspaces    []string
	approvalDetectors   map[string]*ApprovalDetector
	planningDetectors   map[string]*PlanningDetector
	errorPopupDetectors map[string]*ErrorPopupDetector
	userDetectors       map[string]*UserMessageDetector
}

func newMockBridgePool() *mockBridgeConnectionPool {
	return &mockBridgeConnectionPool{
		connected:           make(map[string]CdpServiceWithContexts),
		approvalDetectors:   make(map[string]*ApprovalDetector),
		planningDetectors:   make(map[string]*PlanningDetector),
		errorPopupDetectors: make(map[string]*ErrorPopupDetector),
		userDetectors:       make(map[string]*UserMessageDetector),
	}
}

func (p *mockBridgeConnectionPool) GetConnected(workspaceName string) CdpServiceWithContexts {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connected[workspaceName]
}

func (p *mockBridgeConnectionPool) GetActiveWorkspaceNames() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activeWorkspaces
}

func (p *mockBridgeConnectionPool) GetApprovalDetector(projectName string) *ApprovalDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.approvalDetectors[projectName]
}

func (p *mockBridgeConnectionPool) RegisterApprovalDetector(projectName string, detector *ApprovalDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.approvalDetectors[projectName] = detector
}

func (p *mockBridgeConnectionPool) GetPlanningDetector(projectName string) *PlanningDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.planningDetectors[projectName]
}

func (p *mockBridgeConnectionPool) RegisterPlanningDetector(projectName string, detector *PlanningDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.planningDetectors[projectName] = detector
}

func (p *mockBridgeConnectionPool) GetErrorPopupDetector(projectName string) *ErrorPopupDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.errorPopupDetectors[projectName]
}

func (p *mockBridgeConnectionPool) RegisterErrorPopupDetector(projectName string, detector *ErrorPopupDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errorPopupDetectors[projectName] = detector
}

func (p *mockBridgeConnectionPool) GetUserMessageDetector(projectName string) *UserMessageDetector {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.userDetectors[projectName]
}

func (p *mockBridgeConnectionPool) RegisterUserMessageDetector(projectName string, detector *UserMessageDetector) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userDetectors[projectName] = detector
}

// mockAutoAccept implements AutoAcceptChecker.
type mockAutoAccept struct {
	enabled bool
}

func (m *mockAutoAccept) IsEnabled() bool {
	return m.enabled
}

func (m *mockAutoAccept) Handle(mode string) (AutoAcceptResult, error) {
	return AutoAcceptResult{Success: true, Enabled: m.enabled}, nil
}

// mockQuota implements QuotaFetcher.
type mockQuota struct{}

func (m *mockQuota) FetchQuota(ctx context.Context) (any, error) {
	return nil, nil
}

// mockMessenger implements TelegramBotMessenger.
type mockMessenger struct {
	mu           sync.Mutex
	sentMessages []struct {
		ChatID   int64
		ThreadID *int
		Text     string
		Keyboard *InlineKeyboardMarkup
	}
	clearedMarkups []struct {
		ChatID    int64
		MessageID int
	}
	nextMessageID int
}

func newMockMessenger() *mockMessenger {
	return &mockMessenger{nextMessageID: 100}
}

func (m *mockMessenger) SendMessage(ctx context.Context, chatID int64, threadID *int, htmlText string, keyboard *InlineKeyboardMarkup) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextMessageID++
	id := m.nextMessageID
	m.sentMessages = append(m.sentMessages, struct {
		ChatID   int64
		ThreadID *int
		Text     string
		Keyboard *InlineKeyboardMarkup
	}{ChatID: chatID, ThreadID: threadID, Text: htmlText, Keyboard: keyboard})
	return id, nil
}

func (m *mockMessenger) ClearMessageReplyMarkup(ctx context.Context, chatID int64, messageID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearedMarkups = append(m.clearedMarkups, struct {
		ChatID    int64
		MessageID int
	}{ChatID: chatID, MessageID: messageID})
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: initCdpBridge builds the initial state
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_InitCdpBridgeInitialState(t *testing.T) {
	pool := newMockBridgePool()
	quota := &mockQuota{}
	autoAccept := &mockAutoAccept{enabled: false}

	bridge := InitCdpBridge(false, pool, quota, autoAccept)

	if bridge.LastActiveWorkspace != nil {
		t.Errorf("expected LastActiveWorkspace to be nil")
	}
	if bridge.LastActiveChannel != nil {
		t.Errorf("expected LastActiveChannel to be nil")
	}
	if bridge.AutoAccept.IsEnabled() != false {
		t.Errorf("expected AutoAccept to be disabled")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: getCurrentCdp returns null when not connected
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_GetCurrentCdpReturnsNilWhenDisconnected(t *testing.T) {
	pool := newMockBridgePool()
	bridge := InitCdpBridge(false, pool, &mockQuota{}, &mockAutoAccept{})

	cdp := GetCurrentCdp(bridge)
	if cdp != nil {
		t.Fatalf("expected nil when not connected")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: round-trips build/parse of approval action ID
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_ApprovalCustomIDRoundTrip(t *testing.T) {
	customID := BuildApprovalCustomID("approve", "my-workspace", "123456")
	if customID != "approve_action:my-workspace:123456" {
		t.Fatalf("unexpected customID: %s", customID)
	}

	parsed := ParseApprovalCustomID(customID)
	if parsed == nil {
		t.Fatalf("expected non-nil parsed custom ID")
	}
	if parsed.Action != "approve" {
		t.Errorf("expected action 'approve', got %s", parsed.Action)
	}
	if parsed.ProjectName == nil || *parsed.ProjectName != "my-workspace" {
		t.Errorf("expected project 'my-workspace', got %v", parsed.ProjectName)
	}
	if parsed.ChannelID == nil || *parsed.ChannelID != "123456" {
		t.Errorf("expected channelId '123456', got %v", parsed.ChannelID)
	}

	// Also verify always_allow and deny actions
	allowChatID := BuildApprovalCustomID("always_allow", "my-workspace", "999")
	if allowChatID != "always_allow_action:my-workspace:999" {
		t.Errorf("unexpected always_allow ID: %s", allowChatID)
	}
	parsedAllow := ParseApprovalCustomID(allowChatID)
	if parsedAllow == nil || parsedAllow.Action != "always_allow" {
		t.Errorf("failed parsing always_allow: %+v", parsedAllow)
	}

	denyID := BuildApprovalCustomID("deny", "my-workspace")
	if denyID != "deny_action:my-workspace" {
		t.Errorf("unexpected deny ID: %s", denyID)
	}
	parsedDeny := ParseApprovalCustomID(denyID)
	if parsedDeny == nil || parsedDeny.Action != "deny" || parsedDeny.ChannelID != nil {
		t.Errorf("failed parsing deny: %+v", parsedDeny)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 4: supports legacy approval action IDs without channelId
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_ParseLegacyApprovalCustomIDWithoutChannel(t *testing.T) {
	parsed := ParseApprovalCustomID("approve_action:legacy-workspace")
	if parsed == nil {
		t.Fatalf("expected non-nil parsed legacy ID")
	}
	if parsed.Action != "approve" {
		t.Errorf("expected action 'approve', got %s", parsed.Action)
	}
	if parsed.ProjectName == nil || *parsed.ProjectName != "legacy-workspace" {
		t.Errorf("expected project 'legacy-workspace', got %v", parsed.ProjectName)
	}
	if parsed.ChannelID != nil {
		t.Errorf("expected nil channelId, got %v", parsed.ChannelID)
	}

	// Exact prefix without workspace
	parsedBare := ParseApprovalCustomID("approve_action")
	if parsedBare == nil || parsedBare.Action != "approve" || parsedBare.ProjectName != nil {
		t.Errorf("failed parsing bare prefix: %+v", parsedBare)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 5: routes approval notifications by session title when linked
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_RoutesBySessionTitleWhenLinked(t *testing.T) {
	bridge := InitCdpBridge(false, newMockBridgePool(), &mockQuota{}, &mockAutoAccept{})
	channel := TelegramChannel{ChatID: 1001, ThreadID: ptrInt(1)}
	RegisterApprovalSessionChannel(bridge, "ws-a", "Session Alpha", channel)

	resolved := ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptrString("Session Alpha"))
	if resolved == nil {
		t.Fatalf("expected channel to resolve")
	}
	if resolved.ChatID != 1001 || resolved.ThreadID == nil || *resolved.ThreadID != 1 {
		t.Errorf("unexpected channel: %+v", resolved)
	}

	// Test case-insensitive and whitespace trimmed match
	resolvedCase := ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptrString("  session alpha  "))
	if resolvedCase == nil || resolvedCase.ChatID != 1001 {
		t.Errorf("expected case-insensitive trimmed match, got %+v", resolvedCase)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 6: falls back to workspace channel when session title does not match
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_FallsBackToWorkspaceChannelOnMismatch(t *testing.T) {
	bridge := InitCdpBridge(false, newMockBridgePool(), &mockQuota{}, &mockAutoAccept{})
	wsChannel := TelegramChannel{ChatID: 1001}
	RegisterApprovalWorkspaceChannel(bridge, "ws-a", wsChannel)

	resolved := ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptrString("Unknown Session"))
	if resolved == nil || resolved.ChatID != 1001 {
		t.Fatalf("expected fallback to workspace channel 1001, got %+v", resolved)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 7: falls back to workspace channel when currentChatTitle is null
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_FallsBackToWorkspaceChannelWhenTitleNil(t *testing.T) {
	bridge := InitCdpBridge(false, newMockBridgePool(), &mockQuota{}, &mockAutoAccept{})
	wsChannel := TelegramChannel{ChatID: 1001}
	RegisterApprovalWorkspaceChannel(bridge, "ws-a", wsChannel)

	resolved := ResolveApprovalChannelForCurrentChat(bridge, "ws-a", nil)
	if resolved == nil || resolved.ChatID != 1001 {
		t.Fatalf("expected fallback to workspace channel 1001, got %+v", resolved)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 8: falls back to workspace channel when currentChatTitle is empty
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_FallsBackToWorkspaceChannelWhenTitleEmpty(t *testing.T) {
	bridge := InitCdpBridge(false, newMockBridgePool(), &mockQuota{}, &mockAutoAccept{})
	wsChannel := TelegramChannel{ChatID: 1001}
	RegisterApprovalWorkspaceChannel(bridge, "ws-a", wsChannel)

	resolved := ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptrString(""))
	if resolved == nil || resolved.ChatID != 1001 {
		t.Fatalf("expected fallback to workspace channel 1001, got %+v", resolved)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 9: prefers session channel over workspace channel
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_PrefersSessionChannelOverWorkspaceChannel(t *testing.T) {
	bridge := InitCdpBridge(false, newMockBridgePool(), &mockQuota{}, &mockAutoAccept{})
	wsChannel := TelegramChannel{ChatID: 1001}
	sessionChannel := TelegramChannel{ChatID: 1001, ThreadID: ptrInt(2)}
	RegisterApprovalWorkspaceChannel(bridge, "ws-a", wsChannel)
	RegisterApprovalSessionChannel(bridge, "ws-a", "Session Alpha", sessionChannel)

	resolved := ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptrString("Session Alpha"))
	if resolved == nil || resolved.ThreadID == nil || *resolved.ThreadID != 2 {
		t.Fatalf("expected session channel with thread 2, got %+v", resolved)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 10: returns null when neither session nor workspace is registered
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_ReturnsNilWhenUnregistered(t *testing.T) {
	bridge := InitCdpBridge(false, newMockBridgePool(), &mockQuota{}, &mockAutoAccept{})

	if resolved := ResolveApprovalChannelForCurrentChat(bridge, "ws-unknown", ptrString("Some Title")); resolved != nil {
		t.Errorf("expected nil, got %+v", resolved)
	}
	if resolved := ResolveApprovalChannelForCurrentChat(bridge, "ws-unknown", nil); resolved != nil {
		t.Errorf("expected nil, got %+v", resolved)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 11: round-trips build/parse of planning open action ID
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_PlanningOpenCustomIDRoundTrip(t *testing.T) {
	customID := BuildPlanningCustomID("open", "my-workspace", "123456")
	parsed := ParsePlanningCustomID(customID)
	if parsed == nil {
		t.Fatalf("expected non-nil parsed custom ID")
	}
	if parsed.Action != "open" || parsed.ProjectName == nil || *parsed.ProjectName != "my-workspace" || parsed.ChannelID == nil || *parsed.ChannelID != "123456" {
		t.Errorf("unexpected parsed planning ID: %+v", parsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 12: round-trips build/parse of planning proceed action ID
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_PlanningProceedCustomIDRoundTrip(t *testing.T) {
	customID := BuildPlanningCustomID("proceed", "my-workspace", "789")
	parsed := ParsePlanningCustomID(customID)
	if parsed == nil {
		t.Fatalf("expected non-nil parsed custom ID")
	}
	if parsed.Action != "proceed" || parsed.ProjectName == nil || *parsed.ProjectName != "my-workspace" || parsed.ChannelID == nil || *parsed.ChannelID != "789" {
		t.Errorf("unexpected parsed planning ID: %+v", parsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 13: supports planning action IDs without channelId
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_ParseLegacyPlanningCustomIDWithoutChannel(t *testing.T) {
	parsed := ParsePlanningCustomID("planning_open_action:legacy-workspace")
	if parsed == nil {
		t.Fatalf("expected non-nil parsed legacy ID")
	}
	if parsed.Action != "open" || parsed.ProjectName == nil || *parsed.ProjectName != "legacy-workspace" || parsed.ChannelID != nil {
		t.Errorf("unexpected parsed legacy planning ID: %+v", parsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 14: parsePlanningCustomId returns null for non-planning IDs
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_ParsePlanningCustomIDReturnsNilForInvalid(t *testing.T) {
	if parsed := ParsePlanningCustomID("approve_action:ws-a"); parsed != nil {
		t.Errorf("expected nil for approve action in planning parser, got %+v", parsed)
	}
	if parsed := ParsePlanningCustomID("random_string"); parsed != nil {
		t.Errorf("expected nil for random string in planning parser, got %+v", parsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 15: Error popup custom ID round-trip and legacy parsing
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_ErrorPopupCustomIDRoundTripAndLegacy(t *testing.T) {
	dismissID := BuildErrorPopupCustomID("dismiss", "ws-1", "ch-100")
	if dismissID != "error_popup_dismiss_action:ws-1:ch-100" {
		t.Errorf("unexpected dismiss ID: %s", dismissID)
	}
	parsedDismiss := ParseErrorPopupCustomID(dismissID)
	if parsedDismiss == nil || parsedDismiss.Action != "dismiss" || *parsedDismiss.ProjectName != "ws-1" || *parsedDismiss.ChannelID != "ch-100" {
		t.Errorf("failed parsing dismiss: %+v", parsedDismiss)
	}

	copyDebugID := BuildErrorPopupCustomID("copy_debug", "ws-1")
	if copyDebugID != "error_popup_copy_debug_action:ws-1" {
		t.Errorf("unexpected copy debug ID: %s", copyDebugID)
	}
	parsedCopy := ParseErrorPopupCustomID(copyDebugID)
	if parsedCopy == nil || parsedCopy.Action != "copy_debug" || *parsedCopy.ProjectName != "ws-1" || parsedCopy.ChannelID != nil {
		t.Errorf("failed parsing copy_debug: %+v", parsedCopy)
	}

	retryID := BuildErrorPopupCustomID("retry", "ws-1")
	if retryID != "error_popup_retry_action:ws-1" {
		t.Errorf("unexpected retry ID: %s", retryID)
	}
	parsedRetry := ParseErrorPopupCustomID(retryID)
	if parsedRetry == nil || parsedRetry.Action != "retry" || *parsedRetry.ProjectName != "ws-1" {
		t.Errorf("failed parsing retry: %+v", parsedRetry)
	}

	// Bare prefix
	parsedBare := ParseErrorPopupCustomID("error_popup_dismiss_action")
	if parsedBare == nil || parsedBare.Action != "dismiss" || parsedBare.ProjectName != nil {
		t.Errorf("failed parsing bare error prefix: %+v", parsedBare)
	}

	// Invalid
	if parsed := ParseErrorPopupCustomID("unknown_action:ws"); parsed != nil {
		t.Errorf("expected nil for unknown action, got %+v", parsed)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 16: GetCurrentChatTitle extracts title from context
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_GetCurrentChatTitle(t *testing.T) {
	cdp := &mockCdpWithContexts{
		contexts: []ExecutionContextInfo{{ID: 10}, {ID: 20}},
	}
	cdp.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		ctxID := params["contextId"].(int64)
		if ctxID == 10 {
			// Context 10 returns Agent (should be ignored)
			return makeEvaluateResponse("Agent"), nil
		}
		// Context 20 returns active session title
		return makeEvaluateResponse("Feature X Development"), nil
	}

	ctx := context.Background()
	title := GetCurrentChatTitle(ctx, cdp)
	if title == nil {
		t.Fatalf("expected title to be found")
	}
	if *title != "Feature X Development" {
		t.Errorf("expected 'Feature X Development', got %q", *title)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 17: GetCurrentCdp fallbacks
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_GetCurrentCdpFallback(t *testing.T) {
	pool := newMockBridgePool()
	cdpA := &mockCdpWithContexts{}
	cdpB := &mockCdpWithContexts{}
	pool.connected["ws-a"] = cdpA
	pool.connected["ws-b"] = cdpB
	pool.activeWorkspaces = []string{"ws-b"}

	bridge := InitCdpBridge(false, pool, &mockQuota{}, &mockAutoAccept{})

	// 1. LastActiveWorkspace matches ws-a
	bridge.LastActiveWorkspace = ptrString("ws-a")
	if GetCurrentCdp(bridge) != cdpA {
		t.Errorf("expected cdpA from LastActiveWorkspace")
	}

	// 2. LastActiveWorkspace not connected, fall back to activeWorkspaces[0] (ws-b)
	bridge.LastActiveWorkspace = ptrString("ws-disconnected")
	if GetCurrentCdp(bridge) != cdpB {
		t.Errorf("expected fallback to cdpB")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 18: EnsureApprovalDetector, EnsurePlanningDetector, EnsureErrorPopupDetector, EnsureUserMessageDetector
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_EnsureDetectorsOrchestration(t *testing.T) {
	pool := newMockBridgePool()
	messenger := newMockMessenger()
	bridge := InitCdpBridge(false, pool, &mockQuota{}, &mockAutoAccept{})
	bridge.BotApi = messenger

	cdp := &mockCdpWithContexts{
		contexts: []ExecutionContextInfo{{ID: 42}},
	}
	RegisterApprovalWorkspaceChannel(bridge, "test-ws", TelegramChannel{ChatID: 555})

	// 1. EnsureApprovalDetector
	EnsureApprovalDetector(bridge, cdp, "test-ws")
	detector := pool.GetApprovalDetector("test-ws")
	if detector == nil || !detector.IsActive() {
		t.Fatalf("expected active approval detector in pool")
	}

	// Calling again should be idempotent
	EnsureApprovalDetector(bridge, cdp, "test-ws")
	if pool.GetApprovalDetector("test-ws") != detector {
		t.Errorf("expected same detector instance")
	}
	_ = detector.Stop()

	// 2. EnsurePlanningDetector
	EnsurePlanningDetector(bridge, cdp, "test-ws")
	planDetector := pool.GetPlanningDetector("test-ws")
	if planDetector == nil || !planDetector.IsActive() {
		t.Fatalf("expected active planning detector in pool")
	}
	_ = planDetector.Stop()

	// 3. EnsureErrorPopupDetector
	EnsureErrorPopupDetector(bridge, cdp, "test-ws")
	errorDetector := pool.GetErrorPopupDetector("test-ws")
	if errorDetector == nil || !errorDetector.IsActive() {
		t.Fatalf("expected active error popup detector in pool")
	}
	_ = errorDetector.Stop()

	// 4. EnsureUserMessageDetector
	userCalls := 0
	EnsureUserMessageDetector(bridge, cdp, "test-ws", func(info UserMessageInfo) {
		userCalls++
	})
	userDetector := pool.GetUserMessageDetector("test-ws")
	if userDetector == nil || !userDetector.IsActive() {
		t.Fatalf("expected active user message detector in pool")
	}
	userDetector.Stop()
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 19: Error popup notification and reply markup cleanup on resolution
// ─────────────────────────────────────────────────────────────────────────────
func TestCdpBridgeManager_ErrorPopupNotificationAndCleanup(t *testing.T) {
	pool := newMockBridgePool()
	messenger := newMockMessenger()
	bridge := InitCdpBridge(false, pool, &mockQuota{}, &mockAutoAccept{})
	bridge.BotApi = messenger

	cdp := &mockCdpWithContexts{
		contexts: []ExecutionContextInfo{{ID: 42}},
	}
	mockInfo := makeErrorPopupInfo()
	firstCall := true
	cdp.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(nil), nil
	}

	RegisterApprovalWorkspaceChannel(bridge, "err-ws", TelegramChannel{ChatID: 777})

	EnsureErrorPopupDetector(bridge, cdp, "err-ws")
	detector := pool.GetErrorPopupDetector("err-ws")

	ctx := context.Background()
	// 1. Detection poll -> triggers OnErrorPopup -> sends telegram message
	detector.PollOnce(ctx)

	if len(messenger.sentMessages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(messenger.sentMessages))
	}
	sent := messenger.sentMessages[0]
	if sent.ChatID != 777 {
		t.Errorf("expected ChatID 777, got %d", sent.ChatID)
	}
	if sent.Keyboard == nil || len(sent.Keyboard.InlineKeyboard[0]) != 3 {
		t.Errorf("expected inline keyboard with 3 buttons, got %+v", sent.Keyboard)
	}

	// 2. Disappearance poll -> triggers OnResolved -> calls ClearMessageReplyMarkup
	detector.PollOnce(ctx)

	if len(messenger.clearedMarkups) != 1 {
		t.Fatalf("expected 1 cleared markup call, got %d", len(messenger.clearedMarkups))
	}
	if messenger.clearedMarkups[0].ChatID != 777 || messenger.clearedMarkups[0].MessageID != 101 {
		t.Errorf("unexpected cleared markup: %+v", messenger.clearedMarkups[0])
	}

	_ = detector.Stop()
}
