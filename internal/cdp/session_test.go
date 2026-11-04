package cdp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockSessionCDPClient struct {
	callFunc                 func(method string, params any) (json.RawMessage, error)
	getContextsFunc          func() []Context
	waitForCascadePanelReady func(timeout, pollInterval time.Duration) bool
	calls                    []recordedCDPCall
	mu                       sync.Mutex
}

type recordedCDPCall struct {
	Method string
	Params any
}

func (m *mockSessionCDPClient) Call(method string, params any) (json.RawMessage, error) {
	m.mu.Lock()
	m.calls = append(m.calls, recordedCDPCall{Method: method, Params: params})
	m.mu.Unlock()
	if m.callFunc != nil {
		return m.callFunc(method, params)
	}
	return json.RawMessage(`{}`), nil
}

func (m *mockSessionCDPClient) GetContexts() []Context {
	if m.getContextsFunc != nil {
		return m.getContextsFunc()
	}
	return []Context{
		{ID: 42, Name: "Electron Isolated Context", URL: ""},
	}
}

func (m *mockSessionCDPClient) WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool {
	if m.waitForCascadePanelReady != nil {
		return m.waitForCascadePanelReady(timeout, pollInterval)
	}
	return false
}

func evalValue(val any) (json.RawMessage, error) {
	b, err := json.Marshal(map[string]any{
		"result": map[string]any{
			"value": val,
		},
	})
	return json.RawMessage(b), err
}

// -----------------------------------------------------------------------------
// 1. StartNewChat() tests (6 tests matching chatSessionService.test.ts:20-106)
// -----------------------------------------------------------------------------

func TestChatSessionService_StartNewChat_ClicksWhenEnabledAndVerifiesDisabled(t *testing.T) {
	mock := &mockSessionCDPClient{}
	callCount := 0
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		if method == "Runtime.evaluate" {
			callCount++
			if callCount == 1 {
				// 1st call: Get button state: enabled
				return evalValue(map[string]any{
					"found":   true,
					"enabled": true,
					"x":       100,
					"y":       50,
				})
			}
			// 2nd call: Verification after click: changed to disabled
			return evalValue(map[string]any{
				"found":   true,
				"enabled": false,
				"x":       100,
				"y":       50,
			})
		}
		return json.RawMessage(`{}`), nil
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.StartNewChat(mock)
	if !result.OK {
		t.Fatalf("expected result.OK == true, got false (error: %s)", result.Error)
	}

	var foundMousePressed bool
	for _, call := range mock.calls {
		if call.Method == "Input.dispatchMouseEvent" {
			if pMap, ok := call.Params.(map[string]any); ok {
				if pMap["type"] == "mousePressed" && pMap["x"] == 100 && pMap["y"] == 50 {
					foundMousePressed = true
					break
				}
			}
		}
	}
	if !foundMousePressed {
		t.Fatalf("expected Input.dispatchMouseEvent with type mousePressed at (100, 50)")
	}
}

func TestChatSessionService_StartNewChat_NoOpWhenAlreadyDisabled(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		if method == "Runtime.evaluate" {
			return evalValue(map[string]any{
				"found":   true,
				"enabled": false,
				"cursor":  "not-allowed",
				"x":       100,
				"y":       50,
			})
		}
		return json.RawMessage(`{}`), nil
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.StartNewChat(mock)
	if !result.OK {
		t.Fatalf("expected result.OK == true, got false (error: %s)", result.Error)
	}

	for _, call := range mock.calls {
		if call.Method == "Input.dispatchMouseEvent" {
			t.Fatalf("expected Input.dispatchMouseEvent to NOT be called, but it was")
		}
	}
}

func TestChatSessionService_StartNewChat_FailsWhenButtonNotFound(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		if method == "Runtime.evaluate" {
			return evalValue(map[string]any{"found": false})
		}
		return json.RawMessage(`{}`), nil
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.StartNewChat(mock)
	if result.OK {
		t.Fatalf("expected result.OK == false, got true")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Fatalf("expected error containing 'not found', got %s", result.Error)
	}
}

func TestChatSessionService_StartNewChat_FailsWhenNoContextsAvailable(t *testing.T) {
	mock := &mockSessionCDPClient{
		getContextsFunc: func() []Context {
			return nil
		},
		waitForCascadePanelReady: func(timeout, pollInterval time.Duration) bool {
			return false
		},
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.StartNewChat(mock)
	if result.OK {
		t.Fatalf("expected result.OK == false, got true")
	}
	if !strings.Contains(result.Error, "No contexts available") {
		t.Fatalf("expected error containing 'No contexts available', got %s", result.Error)
	}
}

func TestChatSessionService_StartNewChat_FailsOnCDPException(t *testing.T) {
	mock := &mockSessionCDPClient{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return nil, errors.New("WebSocket切断")
		},
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.StartNewChat(mock)
	if result.OK {
		t.Fatalf("expected result.OK == false, got true")
	}
	if result.Error == "" {
		t.Fatalf("expected result.Error to be defined, got empty")
	}
}

func TestChatSessionService_StartNewChat_FailsWhenStateDoesNotChangeAfterClick(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		if method == "Runtime.evaluate" {
			return evalValue(map[string]any{
				"found":   true,
				"enabled": true,
				"x":       100,
				"y":       50,
			})
		}
		return json.RawMessage(`{}`), nil
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.StartNewChat(mock)
	if result.OK {
		t.Fatalf("expected result.OK == false, got true")
	}
	if !strings.Contains(result.Error, "state did not change") {
		t.Fatalf("expected error containing 'state did not change', got %s", result.Error)
	}
}

// -----------------------------------------------------------------------------
// 2. GetCurrentSessionInfo() tests (4 tests matching chatSessionService.test.ts:108-150)
// -----------------------------------------------------------------------------

func TestChatSessionService_GetCurrentSessionInfo_ReturnsActiveChatTitle(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		return evalValue(map[string]any{
			"title":         "テストチャット",
			"hasActiveChat": true,
		})
	}

	service := NewChatSessionService()
	info := service.GetCurrentSessionInfo(mock)
	if info.Title != "テストチャット" {
		t.Fatalf("expected title 'テストチャット', got %s", info.Title)
	}
	if !info.HasActiveChat {
		t.Fatalf("expected HasActiveChat == true, got false")
	}
}

func TestChatSessionService_GetCurrentSessionInfo_DefaultAgentTitleIsInactive(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		return evalValue(map[string]any{
			"title":         "Agent",
			"hasActiveChat": false,
		})
	}

	service := NewChatSessionService()
	info := service.GetCurrentSessionInfo(mock)
	if info.Title != "Agent" {
		t.Fatalf("expected title 'Agent', got %s", info.Title)
	}
	if info.HasActiveChat {
		t.Fatalf("expected HasActiveChat == false, got true")
	}
}

func TestChatSessionService_GetCurrentSessionInfo_FallbackOnCDPError(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		return nil, errors.New("CDPエラー")
	}

	service := NewChatSessionService()
	info := service.GetCurrentSessionInfo(mock)
	if info.Title != "(Failed to retrieve)" {
		t.Fatalf("expected title '(Failed to retrieve)', got %s", info.Title)
	}
	if info.HasActiveChat {
		t.Fatalf("expected HasActiveChat == false, got true")
	}
}

func TestChatSessionService_GetCurrentSessionInfo_FallbackOnNullValue(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		return evalValue(nil)
	}

	service := NewChatSessionService()
	info := service.GetCurrentSessionInfo(mock)
	if info.Title != "(Failed to retrieve)" {
		t.Fatalf("expected title '(Failed to retrieve)', got %s", info.Title)
	}
	if info.HasActiveChat {
		t.Fatalf("expected HasActiveChat == false, got true")
	}
}

// -----------------------------------------------------------------------------
// 3. ActivateSessionByTitle() tests (6 tests matching chatSessionService.test.ts:152-279)
// -----------------------------------------------------------------------------

func TestChatSessionService_ActivateSessionByTitle_NoOpWhenAlreadyActive(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		return evalValue(map[string]any{
			"title":         "target-session",
			"hasActiveChat": true,
		})
	}

	service := NewChatSessionService()
	result := service.ActivateSessionByTitle(mock, "target-session")
	if !result.OK {
		t.Fatalf("expected OK: true, got %v", result)
	}
}

func TestChatSessionService_ActivateSessionByTitle_FailsOnVerificationTitleMismatch(t *testing.T) {
	evaluateCallCount := 0
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		if method != "Runtime.evaluate" {
			return json.RawMessage(`{}`), nil
		}
		evaluateCallCount++
		pMap, _ := params.(map[string]any)
		expr, _ := pMap["expression"].(string)

		if evaluateCallCount == 1 {
			return evalValue(map[string]any{
				"title":         "old-session",
				"hasActiveChat": true,
			})
		}
		if strings.Contains(expr, "Chat title not found in side panel") {
			return evalValue(map[string]any{"ok": true})
		}
		return evalValue(map[string]any{
			"title":         "different-session",
			"hasActiveChat": true,
		})
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.ActivateSessionByTitle(mock, "target-session")
	if result.OK {
		t.Fatalf("expected result.OK == false, got true")
	}
	if !strings.Contains(result.Error, "did not match target title") {
		t.Fatalf("expected error containing 'did not match target title', got %s", result.Error)
	}
}

func TestChatSessionService_ActivateSessionByTitle_FallsBackToPastConversations(t *testing.T) {
	infoCallCount := 0
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		pMap, _ := params.(map[string]any)
		expression, _ := pMap["expression"].(string)

		if strings.Contains(expression, "const header = panel.querySelector('div[class*=\"border-b\"]');") {
			infoCallCount++
			if infoCallCount == 1 {
				return evalValue(map[string]any{
					"title":         "current-session",
					"hasActiveChat": true,
				})
			}
			return evalValue(map[string]any{
				"title":         "target-session",
				"hasActiveChat": true,
			})
		}

		if strings.Contains(expression, "Chat title not found in side panel") {
			return evalValue(map[string]any{
				"ok":    false,
				"error": "not found in side panel",
			})
		}

		if strings.Contains(expression, "Past Conversations button not found") {
			return evalValue(map[string]any{"ok": true})
		}

		return evalValue(nil)
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	result := service.ActivateSessionByTitle(mock, "target-session")
	if !result.OK {
		t.Fatalf("expected OK: true, got %v", result)
	}
}

func TestChatSessionService_ActivateSessionByTitle_RetriesUntilSuccess(t *testing.T) {
	infoCallCount := 0
	directAttemptCount := 0
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		pMap, _ := params.(map[string]any)
		expression, _ := pMap["expression"].(string)

		if strings.Contains(expression, "const header = panel.querySelector('div[class*=\"border-b\"]');") {
			infoCallCount++
			if infoCallCount == 1 {
				return evalValue(map[string]any{
					"title":         "current-session",
					"hasActiveChat": true,
				})
			}
			return evalValue(map[string]any{
				"title":         "target-session",
				"hasActiveChat": true,
			})
		}

		if strings.Contains(expression, "Chat title not found in side panel") {
			directAttemptCount++
			if directAttemptCount < 3 {
				return evalValue(map[string]any{
					"ok":    false,
					"error": "side panel still loading",
				})
			}
			return evalValue(map[string]any{"ok": true})
		}

		if strings.Contains(expression, "Past Conversations button not found") {
			return evalValue(map[string]any{
				"ok":    false,
				"error": "past conversations still loading",
			})
		}

		return evalValue(nil)
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	maxWait := 100 * time.Millisecond
	retryInterval := 1 * time.Millisecond
	result := service.ActivateSessionByTitle(
		mock,
		"target-session",
		ActivateSessionOptions{MaxWait: &maxWait, RetryInterval: &retryInterval},
	)
	if !result.OK {
		t.Fatalf("expected OK: true, got %v", result)
	}
	if directAttemptCount != 3 {
		t.Fatalf("expected directAttemptCount == 3, got %d", directAttemptCount)
	}
}

func TestChatSessionService_ActivateSessionByTitle_FailsOnEmptyTitle(t *testing.T) {
	mock := &mockSessionCDPClient{}
	service := NewChatSessionService()
	result := service.ActivateSessionByTitle(mock, "")
	if result.OK {
		t.Fatalf("expected OK: false, got true")
	}
	if !strings.Contains(result.Error, "empty") {
		t.Fatalf("expected error containing 'empty', got %s", result.Error)
	}
}

func TestChatSessionService_ActivateSessionByTitle_ReportsBothDirectAndPastErrors(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		pMap, _ := params.(map[string]any)
		expression, _ := pMap["expression"].(string)

		if strings.Contains(expression, "const header = panel.querySelector('div[class*=\"border-b\"]');") {
			return evalValue(map[string]any{
				"title":         "current-session",
				"hasActiveChat": true,
			})
		}
		if strings.Contains(expression, "Chat title not found in side panel") {
			return evalValue(map[string]any{
				"ok":    false,
				"error": "direct miss",
			})
		}
		if strings.Contains(expression, "Past Conversations button not found") {
			return evalValue(map[string]any{
				"ok":    false,
				"error": "past miss",
			})
		}
		return evalValue(nil)
	}

	service := NewChatSessionService()
	maxWait := 5 * time.Millisecond
	retryInterval := 1 * time.Millisecond
	result := service.ActivateSessionByTitle(
		mock,
		"target-session",
		ActivateSessionOptions{MaxWait: &maxWait, RetryInterval: &retryInterval},
	)
	if result.OK {
		t.Fatalf("expected result.OK == false, got true")
	}
	if !strings.Contains(result.Error, "direct:") {
		t.Fatalf("expected error containing 'direct:', got %s", result.Error)
	}
	if !strings.Contains(result.Error, "past: past miss") {
		t.Fatalf("expected error containing 'past: past miss', got %s", result.Error)
	}
	if !strings.Contains(result.Error, "after") {
		t.Fatalf("expected error containing 'after', got %s", result.Error)
	}
}

// -----------------------------------------------------------------------------
// 4. ListAllSessions() tests (5 tests matching chatSessionService.test.ts:281-390)
// -----------------------------------------------------------------------------

func TestChatSessionService_ListAllSessions_ScrapesViaCDPMouseClick(t *testing.T) {
	var calls []string
	var mu sync.Mutex
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		mu.Lock()
		calls = append(calls, method)
		evalCount := 0
		for _, c := range calls {
			if c == "Runtime.evaluate" {
				evalCount++
			}
		}
		mu.Unlock()

		if method == "Runtime.evaluate" {
			if evalCount == 1 {
				return evalValue(map[string]any{"found": true, "x": 200, "y": 30})
			}
			return evalValue(map[string]any{
				"sessions": []map[string]any{
					{"title": "Fix login bug", "isActive": true},
					{"title": "Refactor auth", "isActive": false},
				},
			})
		}
		return json.RawMessage(`{}`), nil
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	sessions := service.ListAllSessions(mock)
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].Title != "Fix login bug" || !sessions[0].IsActive {
		t.Fatalf("expected first session 'Fix login bug' isActive: true, got %+v", sessions[0])
	}
	var hasMouseEvent bool
	for _, call := range mock.calls {
		if call.Method == "Input.dispatchMouseEvent" {
			hasMouseEvent = true
			break
		}
	}
	if !hasMouseEvent {
		t.Fatalf("expected Input.dispatchMouseEvent to be called")
	}
}

func TestChatSessionService_ListAllSessions_ClicksShowMoreWhenBelowTarget(t *testing.T) {
	evaluateCount := 0
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		if method == "Runtime.evaluate" {
			evaluateCount++
			if evaluateCount == 1 {
				return evalValue(map[string]any{"found": true, "x": 200, "y": 30})
			}
			if evaluateCount == 2 {
				return evalValue(map[string]any{
					"sessions": []map[string]any{
						{"title": "Session A", "isActive": true},
						{"title": "Session B", "isActive": false},
						{"title": "Session C", "isActive": false},
					},
				})
			}
			if evaluateCount == 3 {
				return evalValue(map[string]any{"found": true, "x": 150, "y": 300})
			}
			return evalValue(map[string]any{
				"sessions": []map[string]any{
					{"title": "Session A", "isActive": true},
					{"title": "Session B", "isActive": false},
					{"title": "Session C", "isActive": false},
					{"title": "Session D", "isActive": false},
					{"title": "Session E", "isActive": false},
				},
			})
		}
		return json.RawMessage(`{}`), nil
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	sessions := service.ListAllSessions(mock)
	if len(sessions) != 5 {
		t.Fatalf("expected 5 sessions, got %d", len(sessions))
	}
	if evaluateCount != 4 {
		t.Fatalf("expected 4 evaluate calls, got %d", evaluateCount)
	}
}

func TestChatSessionService_ListAllSessions_EmptyWhenButtonNotFound(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		return evalValue(map[string]any{"found": false, "x": 0, "y": 0})
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	sessions := service.ListAllSessions(mock)
	if len(sessions) != 0 {
		t.Fatalf("expected empty sessions, got %d", len(sessions))
	}
}

func TestChatSessionService_ListAllSessions_EmptyOnCDPError(t *testing.T) {
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		return nil, errors.New("WebSocket disconnected")
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	sessions := service.ListAllSessions(mock)
	if len(sessions) != 0 {
		t.Fatalf("expected empty sessions, got %d", len(sessions))
	}
}

func TestChatSessionService_ListAllSessions_ClosesPanelWithEscapeKey(t *testing.T) {
	evalCount := 0
	mock := &mockSessionCDPClient{}
	mock.callFunc = func(method string, params any) (json.RawMessage, error) {
		if method == "Runtime.evaluate" {
			evalCount++
			if evalCount == 1 {
				return evalValue(map[string]any{"found": true, "x": 200, "y": 30})
			}
			items := make([]map[string]any, 10)
			for i := 0; i < 10; i++ {
				items[i] = map[string]any{"title": fmt.Sprintf("S%d", i), "isActive": i == 0}
			}
			return evalValue(map[string]any{"sessions": items})
		}
		return json.RawMessage(`{}`), nil
	}

	service := NewChatSessionService()
	service.SleepFunc = func(time.Duration) {}

	service.ListAllSessions(mock)

	var hasEscapeKey bool
	for _, call := range mock.calls {
		if call.Method == "Input.dispatchKeyEvent" {
			if pMap, ok := call.Params.(map[string]any); ok {
				if pMap["key"] == "Escape" {
					hasEscapeKey = true
					break
				}
			}
		}
	}
	if !hasEscapeKey {
		t.Fatalf("expected Input.dispatchKeyEvent with key 'Escape'")
	}
}
