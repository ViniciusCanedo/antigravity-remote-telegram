package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func makeErrorPopupInfo(overrides ...func(*ErrorPopupInfo)) ErrorPopupInfo {
	info := ErrorPopupInfo{
		Title:   "Agent terminated due to error",
		Body:    "The agent encountered an unexpected error and was terminated.",
		Buttons: []string{"Dismiss", "Copy debug info", "Retry"},
	}
	for _, fn := range overrides {
		fn(&info)
	}
	return info
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: Call onErrorPopup when error popup is detected
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_CallsOnErrorPopupWhenDetected(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo()
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	var calledInfo *ErrorPopupInfo
	callsCount := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnErrorPopup: func(info ErrorPopupInfo) {
			calledInfo = &info
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onErrorPopup to be called 1 time, got %d", callsCount)
	}
	if calledInfo == nil {
		t.Fatalf("expected calledInfo not to be nil")
	}
	if calledInfo.Title != "Agent terminated due to error" {
		t.Errorf("expected Title %q, got %q", "Agent terminated due to error", calledInfo.Title)
	}
	if len(calledInfo.Buttons) != 3 {
		t.Errorf("expected 3 buttons, got %d", len(calledInfo.Buttons))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: Do not call the callback when no error popup exists
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_DoesNotCallCallbackWhenNull(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	callsCount := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnErrorPopup: func(info ErrorPopupInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 0 {
		t.Fatalf("expected onErrorPopup not to be called, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: No duplicate calls for the same error popup detected consecutively
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_DeduplicatesConsecutiveDetections(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo()
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	callsCount := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnErrorPopup: func(info ErrorPopupInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onErrorPopup to be called 1 time, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 4: Dedup uses title + body snippet as key
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_DeduplicatesByTitleAndBodyPrefix(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	info1 := makeErrorPopupInfo(func(i *ErrorPopupInfo) {
		i.Buttons = []string{"Dismiss", "Retry"}
	})
	info2 := makeErrorPopupInfo(func(i *ErrorPopupInfo) {
		i.Buttons = []string{"Dismiss", "Copy debug info", "Retry"}
	})

	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(info1), nil
		}
		return makeEvaluateResponse(info2), nil
	}

	callsCount := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnErrorPopup: func(info ErrorPopupInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onErrorPopup to be called 1 time for same title+body, got %d", callsCount)
	}
	lastInfo := detector.GetLastDetectedInfo()
	if lastInfo == nil || len(lastInfo.Buttons) != 3 {
		t.Fatalf("expected updated info with 3 buttons silently stored, got %+v", lastInfo)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 5: Cooldown suppresses rapid re-detection after key reset
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_SuppressesRedetectionWithin10sCooldown(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo()

	var currentTime time.Time
	nowFunc := func() time.Time {
		return currentTime
	}

	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(mockInfo), nil // t = 500ms: detected
		}
		if callCount == 2 {
			return makeEvaluateResponse(nil), nil // t = 1000ms: disappear
		}
		return makeEvaluateResponse(mockInfo), nil // t = 1500ms: re-detect within cooldown
	}

	callsCount := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		NowFunc:    nowFunc,
		OnErrorPopup: func(info ErrorPopupInfo) {
			callsCount++
		},
	})

	ctx := context.Background()

	currentTime = time.UnixMilli(500)
	detector.PollOnce(ctx)
	if callsCount != 1 {
		t.Fatalf("expected 1 call on initial detection, got %d", callsCount)
	}

	currentTime = time.UnixMilli(1000)
	detector.PollOnce(ctx)
	if detector.GetLastDetectedInfo() != nil {
		t.Fatalf("expected lastDetectedInfo to be reset after disappearance")
	}

	currentTime = time.UnixMilli(1500)
	detector.PollOnce(ctx)
	if callsCount != 1 {
		t.Fatalf("expected still 1 call due to 10s cooldown, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 6: clickDismissButton() can click the Dismiss button
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ClickDismissButtonViaCDP(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.ClickDismissButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClickDismissButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call was made")
	}
	if lastCall.Method != "Runtime.evaluate" {
		t.Errorf("expected Runtime.evaluate, got %s", lastCall.Method)
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Dismiss") {
		t.Errorf("expected script to contain %q, got: %s", "Dismiss", expr)
	}
	if ctxID, ok := lastCall.Params["contextId"].(int64); !ok || ctxID != 42 {
		t.Errorf("expected contextId=42, got %v", lastCall.Params["contextId"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 7: clickCopyDebugInfoButton() can click the Copy debug info button
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ClickCopyDebugInfoButtonViaCDP(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.ClickCopyDebugInfoButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClickCopyDebugInfoButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call was made")
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Copy debug info") {
		t.Errorf("expected script to contain %q, got: %s", "Copy debug info", expr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 8: clickRetryButton() can click the Retry button
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ClickRetryButtonViaCDP(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.ClickRetryButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClickRetryButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call was made")
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Retry") {
		t.Errorf("expected script to contain %q, got: %s", "Retry", expr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 9: readClipboard() returns clipboard text
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ReadClipboardReturnsText(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	debugInfo := "Error: Agent terminated\nStack: at line 42\nVersion: 1.0.0"
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(debugInfo), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	content, err := detector.ReadClipboard(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil || *content != debugInfo {
		t.Fatalf("expected %q, got %v", debugInfo, content)
	}

	lastCall := mockCDP.getLastCall()
	if awaitP, ok := lastCall.Params["awaitPromise"].(bool); !ok || !awaitP {
		t.Errorf("expected awaitPromise=true for ReadClipboard")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 10: readClipboard() returns null when clipboard is empty
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ReadClipboardReturnsNilWhenEmpty(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	content, err := detector.ReadClipboard(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != nil {
		t.Fatalf("expected nil clipboard, got %q", *content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 11: readClipboard() returns null on CDP error
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ReadClipboardReturnsNilOnCDPError(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return nil, errors.New("CDP connection lost")
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	content, err := detector.ReadClipboard(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != nil {
		t.Fatalf("expected nil on error, got %q", *content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 12: Polling stops after stop()
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_StopHaltsPolling(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo()
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	var mu sync.Mutex
	callsCount := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService:   mockCDP,
		PollInterval: 10 * time.Millisecond,
		OnErrorPopup: func(info ErrorPopupInfo) {
			mu.Lock()
			callsCount++
			mu.Unlock()
		},
	})

	detector.Start()
	time.Sleep(30 * time.Millisecond)

	_ = detector.Stop()
	mu.Lock()
	countAfterStop := callsCount
	mu.Unlock()

	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	countFinal := callsCount
	mu.Unlock()

	if countFinal != countAfterStop {
		t.Fatalf("expected polling to stop, count increased from %d to %d", countAfterStop, countFinal)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 13: Monitoring continues on CDP error
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ContinuesOnCDPError(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo(func(i *ErrorPopupInfo) {
		i.Title = "Recovery Error"
	})

	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("CDP error")
		}
		return makeEvaluateResponse(mockInfo), nil
	}

	var receivedInfo *ErrorPopupInfo
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnErrorPopup: func(info ErrorPopupInfo) {
			receivedInfo = &info
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx) // error
	if receivedInfo != nil {
		t.Fatalf("expected no callback on error")
	}

	detector.PollOnce(ctx) // recovery
	if receivedInfo == nil || receivedInfo.Title != "Recovery Error" {
		t.Fatalf("expected callback after recovery with Title 'Recovery Error', got %+v", receivedInfo)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 14: getLastDetectedInfo() returns detected info
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_GetLastDetectedInfoReturnsDetected(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo(func(i *ErrorPopupInfo) {
		i.Title = "Custom Error"
		i.Body = "Something went wrong with the agent."
	})
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	if detector.GetLastDetectedInfo() != nil {
		t.Fatalf("expected nil before detection")
	}

	ctx := context.Background()
	detector.PollOnce(ctx)

	info := detector.GetLastDetectedInfo()
	if info == nil {
		t.Fatalf("expected non-nil info after detection")
	}
	if info.Title != "Custom Error" || info.Body != "Something went wrong with the agent." {
		t.Errorf("unexpected info content: %+v", info)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 15: lastDetectedInfo resets when popup disappears
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_GetLastDetectedInfoResetsOnDisappear(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo()
	callCount := 0
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		callCount++
		if callCount == 1 {
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(nil), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	if detector.GetLastDetectedInfo() == nil {
		t.Fatalf("expected non-nil info after detection")
	}

	detector.PollOnce(ctx)
	if detector.GetLastDetectedInfo() != nil {
		t.Fatalf("expected lastDetectedInfo to reset to nil after disappearance")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 16: isActive() returns correct state
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_IsActiveLifecycle(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService:   mockCDP,
		PollInterval: 10 * time.Millisecond,
	})

	if detector.IsActive() {
		t.Fatalf("expected false before Start")
	}

	detector.Start()
	if !detector.IsActive() {
		t.Fatalf("expected true after Start")
	}

	_ = detector.Stop()
	if detector.IsActive() {
		t.Fatalf("expected false after Stop")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 17: Calls without contextId parameter when contextId is null
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_OmitsContextIdWhenNil(t *testing.T) {
	mockCDP := newMockCDP(nil) // contextId is nil
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected a call to CDP")
	}
	if _, exists := lastCall.Params["contextId"]; exists {
		t.Errorf("expected contextId to be omitted when nil, but found in params")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 18: clickDismissButton() returns false on CDP error
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_ClickDismissButtonReturnsFalseOnError(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return nil, errors.New("CDP error")
	}

	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.ClickDismissButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error returned: %v", err)
	}
	if ok {
		t.Fatalf("expected false on CDP error")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 19: WebSocket disconnect errors are silently ignored
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_SilentlyIgnoresWebSocketDisconnect(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return nil, errors.New("WebSocket is not connected")
	}

	callsCount := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnErrorPopup: func(info ErrorPopupInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 0 {
		t.Errorf("expected 0 calls, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 20: onResolved fires when popup disappears after detection
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_CallsOnResolvedWhenPopupDisappears(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makeErrorPopupInfo()
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(nil), nil
	}

	resolvedCalls := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnResolved: func() {
			resolvedCalls++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx) // detection
	if resolvedCalls != 0 {
		t.Errorf("expected onResolved not to be called on detection")
	}

	detector.PollOnce(ctx) // disappearance
	if resolvedCalls != 1 {
		t.Errorf("expected onResolved to be called exactly 1 time, got %d", resolvedCalls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 21: onResolved does not fire when no prior detection
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorPopupDetector_DoesNotCallOnResolvedWithoutPriorDetection(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	resolvedCalls := 0
	detector := NewErrorPopupDetector(ErrorPopupDetectorOptions{
		CdpService: mockCDP,
		OnResolved: func() {
			resolvedCalls++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)

	if resolvedCalls != 0 {
		t.Errorf("expected onResolved not to be called, got %d", resolvedCalls)
	}
}
