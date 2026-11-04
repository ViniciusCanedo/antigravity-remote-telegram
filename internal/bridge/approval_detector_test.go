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

type cdpCallRecord struct {
	Method string
	Params map[string]any
}

type mockCdpEvaluator struct {
	mu               sync.Mutex
	primaryContextID *int64
	callFunc         func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error)
	calls            []cdpCallRecord
	wsName           string
}

func newMockCDP(contextID *int64) *mockCdpEvaluator {
	return &mockCdpEvaluator{
		primaryContextID: contextID,
	}
}

func (m *mockCdpEvaluator) Call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	m.mu.Lock()
	m.calls = append(m.calls, cdpCallRecord{Method: method, Params: params})
	callFunc := m.callFunc
	m.mu.Unlock()

	if callFunc != nil {
		return callFunc(ctx, method, params)
	}
	return makeEvaluateResponse(nil), nil
}

func (m *mockCdpEvaluator) GetPrimaryContextId() *int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.primaryContextID
}

func (m *mockCdpEvaluator) GetCurrentWorkspaceName() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.wsName
}

func (m *mockCdpEvaluator) getCalls() []cdpCallRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]cdpCallRecord, len(m.calls))
	copy(copied, m.calls)
	return copied
}

func (m *mockCdpEvaluator) getLastCall() *cdpCallRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return nil
	}
	last := m.calls[len(m.calls)-1]
	return &last
}

func makeEvaluateResponse(val any) json.RawMessage {
	valBytes, _ := json.Marshal(val)
	var resp runtimeEvaluateResponse
	resp.Result.Value = valBytes
	raw, _ := json.Marshal(resp)
	return raw
}

func ptrInt64(v int64) *int64 {
	return &v
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: Call onApprovalRequired when an approval button is detected
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_CallsOnApprovalRequiredWhenDetected(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "許可",
		DenyText:    "拒否",
		Description: "ファイルへの書き込みを許可しますか？",
	}
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	var calledInfo *ApprovalInfo
	callsCount := 0
	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
		OnApprovalRequired: func(info ApprovalInfo) {
			calledInfo = &info
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onApprovalRequired to be called 1 time, got %d", callsCount)
	}
	if calledInfo == nil {
		t.Fatalf("expected calledInfo not to be nil")
	}
	if calledInfo.ApproveText != "許可" || calledInfo.DenyText != "拒否" {
		t.Errorf("unexpected button texts: approve=%s, deny=%s", calledInfo.ApproveText, calledInfo.DenyText)
	}
	if !strings.Contains(calledInfo.Description, "ファイルへの書き込み") {
		t.Errorf("unexpected description: %s", calledInfo.Description)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: Do not call the callback when no approval button exists
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_DoesNotCallCallbackWhenNull(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	callsCount := 0
	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
		OnApprovalRequired: func(info ApprovalInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 0 {
		t.Fatalf("expected onApprovalRequired not to be called, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: No duplicate calls for the same approval button detected consecutively
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_DeduplicatesConsecutiveDetections(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "許可",
		DenyText:    "拒否",
		Description: "重複テスト: run command",
	}
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	callsCount := 0
	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
		OnApprovalRequired: func(info ApprovalInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onApprovalRequired to be called exactly 1 time, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 4: approveButton() can click the button
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_ApproveButtonClicksViaCDP(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.ApproveButton(ctx, "Allow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ApproveButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call to be recorded")
	}
	if lastCall.Method != "Runtime.evaluate" {
		t.Errorf("expected Runtime.evaluate, got %s", lastCall.Method)
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Allow") {
		t.Errorf("expected expression to contain 'Allow', got %s", expr)
	}
	retByVal, _ := lastCall.Params["returnByValue"].(bool)
	if !retByVal {
		t.Errorf("expected returnByValue to be true")
	}
	ctxID, _ := lastCall.Params["contextId"].(int64)
	if ctxID != 42 {
		t.Errorf("expected contextId to be 42, got %v", ctxID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 5: denyButton() can click (deny) the button
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_DenyButtonClicksViaCDP(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.DenyButton(ctx, "Deny")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected DenyButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call to be recorded")
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Deny") {
		t.Errorf("expected expression to contain 'Deny', got %s", expr)
	}
	ctxID, _ := lastCall.Params["contextId"].(int64)
	if ctxID != 42 {
		t.Errorf("expected contextId to be 42, got %v", ctxID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 6: alwaysAllowButton() can directly click Allow This Conversation
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_AlwaysAllowButtonDirectClick(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.AlwaysAllowButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected AlwaysAllowButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call to be recorded")
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Allow This Conversation") {
		t.Errorf("expected expression to contain 'Allow This Conversation', got %s", expr)
	}
	ctxID, _ := lastCall.Params["contextId"].(int64)
	if ctxID != 42 {
		t.Errorf("expected contextId to be 42, got %v", ctxID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 7: alwaysAllowButton() can click after expanding dropdown
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_AlwaysAllowButtonExpandsDropdownThenClicks(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	var expanded bool
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		expr, _ := params["expression"].(string)
		if strings.Contains(expr, "ALLOW_ONCE_PATTERNS") {
			expanded = true
			return makeEvaluateResponse(clickScriptResult{OK: true, Reason: "toggle-button"}), nil
		}
		if expanded && strings.Contains(expr, "Allow This Conversation") {
			return makeEvaluateResponse(clickScriptResult{OK: true}), nil
		}
		return makeEvaluateResponse(clickScriptResult{OK: false}), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
		SleepFunc: func(ctx context.Context, d time.Duration) error {
			return nil
		},
	})

	ctx := context.Background()
	ok, err := detector.AlwaysAllowButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected AlwaysAllowButton to return true")
	}

	calls := mockCDP.getCalls()
	hasDropdownExpand := false
	hasAllowThisConversation := false
	for _, c := range calls {
		expr, _ := c.Params["expression"].(string)
		if strings.Contains(expr, "ALLOW_ONCE_PATTERNS") {
			hasDropdownExpand = true
		}
		if strings.Contains(expr, "Allow This Conversation") {
			hasAllowThisConversation = true
		}
	}
	if !hasDropdownExpand {
		t.Errorf("expected call containing ALLOW_ONCE_PATTERNS")
	}
	if !hasAllowThisConversation {
		t.Errorf("expected call containing Allow This Conversation")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 8: Polling stops after stop()
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_StopHaltsPolling(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "Allow",
		DenyText:    "Deny",
		Description: "some action",
	}
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	var mu sync.Mutex
	callsCount := 0
	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService:   mockCDP,
		PollInterval: 10 * time.Millisecond,
		OnApprovalRequired: func(info ApprovalInfo) {
			mu.Lock()
			callsCount++
			mu.Unlock()
		},
	})

	detector.Start()
	time.Sleep(35 * time.Millisecond)

	mu.Lock()
	countBeforeStop := callsCount
	mu.Unlock()
	if countBeforeStop != 1 {
		t.Fatalf("expected exactly 1 call before stop, got %d", countBeforeStop)
	}

	_ = detector.Stop()
	if detector.IsActive() {
		t.Errorf("expected IsActive() to be false after Stop()")
	}

	time.Sleep(40 * time.Millisecond)
	mu.Lock()
	countAfterStop := callsCount
	mu.Unlock()

	if countAfterStop != countBeforeStop {
		t.Errorf("expected callsCount to remain %d, got %d", countBeforeStop, countAfterStop)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 9: Monitoring continues on CDP error
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_ContinuesOnCDPError(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	firstCall := true
	mockInfo := ApprovalInfo{
		ApproveText: "許可",
		DenyText:    "拒否",
		Description: "エラー後リカバリ",
	}
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return nil, errors.New("CDP error")
		}
		return makeEvaluateResponse(mockInfo), nil
	}

	var calledInfo *ApprovalInfo
	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
		OnApprovalRequired: func(info ApprovalInfo) {
			calledInfo = &info
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx) // Error cycle
	if calledInfo != nil {
		t.Fatalf("expected no callback on error cycle")
	}

	detector.PollOnce(ctx) // Success cycle
	if calledInfo == nil {
		t.Fatalf("expected callback on recovery cycle")
	}
	if calledInfo.ApproveText != "許可" {
		t.Errorf("expected ApproveText '許可', got %s", calledInfo.ApproveText)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 10: getLastDetectedInfo() can retrieve detected info
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_GetLastDetectedInfoReturnsDetected(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "Accept",
		DenyText:    "Decline",
		Description: "テストアクション",
	}
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	if detector.GetLastDetectedInfo() != nil {
		t.Fatalf("expected GetLastDetectedInfo() to be nil before detection")
	}

	ctx := context.Background()
	detector.PollOnce(ctx)

	info := detector.GetLastDetectedInfo()
	if info == nil {
		t.Fatalf("expected GetLastDetectedInfo() not to be nil")
	}
	if info.ApproveText != "Accept" || info.DenyText != "Decline" || info.Description != "テストアクション" {
		t.Errorf("unexpected info values: %+v", info)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 11: lastDetectedInfo resets when the button disappears
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_GetLastDetectedInfoResetsOnDisappear(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "Allow",
		DenyText:    "Deny",
		Description: "action",
	}
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(nil), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	if detector.GetLastDetectedInfo() == nil {
		t.Fatalf("expected info to be present after detection")
	}

	detector.PollOnce(ctx)
	if detector.GetLastDetectedInfo() != nil {
		t.Fatalf("expected info to be reset to nil after disappearance")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 12: approveButton() without arguments uses detected approveText
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_ApproveButtonUsesDetectedText(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "承認する",
		DenyText:    "拒否",
		Description: "action",
	}
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	ok, err := detector.ApproveButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ApproveButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "承認する") {
		t.Errorf("expected expression to contain '承認する', got %s", expr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 13: denyButton() without arguments uses detected denyText
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_DenyButtonUsesDetectedText(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "Allow",
		DenyText:    "キャンセル",
		Description: "action",
	}
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	ok, err := detector.DenyButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected DenyButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "キャンセル") {
		t.Errorf("expected expression to contain 'キャンセル', got %s", expr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 14: calls without contextId parameter when contextId is null
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_OmitsContextIdWhenNil(t *testing.T) {
	mockCDP := newMockCDP(nil) // contextId is nil
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	detector := NewApprovalDetector(ApprovalDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call to be recorded")
	}
	if _, exists := lastCall.Params["contextId"]; exists {
		t.Errorf("expected contextId to be omitted when nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 15: onResolved fires when detected buttons disappear
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_CallsOnResolvedWhenButtonsDisappear(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := ApprovalInfo{
		ApproveText: "Allow",
		DenyText:    "Deny",
		Description: "action",
	}
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(nil), nil
	}

	resolvedCalls := 0
	detector := NewApprovalDetector(ApprovalDetectorOptions{
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
// Test 16: onResolved does not fire when no prior detection
// ─────────────────────────────────────────────────────────────────────────────
func TestApprovalDetector_DoesNotCallOnResolvedWithoutPriorDetection(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	resolvedCalls := 0
	detector := NewApprovalDetector(ApprovalDetectorOptions{
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
