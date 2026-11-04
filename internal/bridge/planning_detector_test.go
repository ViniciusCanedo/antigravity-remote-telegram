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

func makePlanningInfo(overrides ...func(*PlanningInfo)) PlanningInfo {
	info := PlanningInfo{
		OpenText:    "Open",
		ProceedText: "Proceed",
		PlanTitle:   "Implementation Plan",
		PlanSummary: "Add authentication feature",
		Description: "This plan adds user authentication to the app.",
	}
	for _, fn := range overrides {
		fn(&info)
	}
	return info
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: Call onPlanningRequired when planning buttons are detected
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_CallsOnPlanningRequiredWhenDetected(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo()
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	var calledInfo *PlanningInfo
	callsCount := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
		OnPlanningRequired: func(info PlanningInfo) {
			calledInfo = &info
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onPlanningRequired to be called 1 time, got %d", callsCount)
	}
	if calledInfo == nil {
		t.Fatalf("expected calledInfo not to be nil")
	}
	if calledInfo.OpenText != "Open" || calledInfo.ProceedText != "Proceed" || calledInfo.PlanTitle != "Implementation Plan" {
		t.Errorf("unexpected calledInfo fields: %+v", calledInfo)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: Do not call the callback when no planning buttons exist
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_DoesNotCallCallbackWhenNull(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	callsCount := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
		OnPlanningRequired: func(info PlanningInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	if callsCount != 0 {
		t.Fatalf("expected onPlanningRequired not to be called, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: No duplicate calls for the same buttons detected consecutively
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_DeduplicatesConsecutiveDetections(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo()
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	callsCount := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
		OnPlanningRequired: func(info PlanningInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onPlanningRequired to be called 1 time, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 4: Dedup uses openText::proceedText as key
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_DeduplicatesByButtonTextPair(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	info1 := makePlanningInfo(func(i *PlanningInfo) {
		i.PlanTitle = "Plan A"
		i.PlanSummary = "Summary A"
	})
	info2 := makePlanningInfo(func(i *PlanningInfo) {
		i.PlanTitle = "Plan B"
		i.PlanSummary = "Summary B"
	})

	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(info1), nil
		}
		return makeEvaluateResponse(info2), nil
	}

	callsCount := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
		OnPlanningRequired: func(info PlanningInfo) {
			callsCount++
		},
	})

	ctx := context.Background()
	detector.PollOnce(ctx)
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected onPlanningRequired to be called 1 time, got %d", callsCount)
	}

	lastInfo := detector.GetLastDetectedInfo()
	if lastInfo == nil {
		t.Fatalf("expected lastDetectedInfo not to be nil")
	}
	if lastInfo.PlanTitle != "Plan B" {
		t.Errorf("expected PlanTitle to be updated to 'Plan B', got %s", lastInfo.PlanTitle)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 5: Cooldown suppresses rapid re-detection after key reset
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_SuppressesRedetectionWithin5sCooldown(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo()

	simulatedTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var state int
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		switch state {
		case 0:
			return makeEvaluateResponse(mockInfo), nil // detect
		case 1:
			return makeEvaluateResponse(nil), nil // disappear
		default:
			return makeEvaluateResponse(mockInfo), nil // re-detect
		}
	}

	callsCount := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
		NowFunc: func() time.Time {
			return simulatedTime
		},
		OnPlanningRequired: func(info PlanningInfo) {
			callsCount++
		},
	})

	ctx := context.Background()

	// 1. Detect at t=0
	state = 0
	detector.PollOnce(ctx)
	if callsCount != 1 {
		t.Fatalf("expected 1 call on initial detection, got %d", callsCount)
	}

	// 2. Disappear at t=500ms
	simulatedTime = simulatedTime.Add(500 * time.Millisecond)
	state = 1
	detector.PollOnce(ctx)

	// 3. Re-detect at t=1500ms (within 5000ms cooldown)
	simulatedTime = simulatedTime.Add(1000 * time.Millisecond)
	state = 2
	detector.PollOnce(ctx)

	if callsCount != 1 {
		t.Fatalf("expected callsCount to remain 1 due to cooldown, got %d", callsCount)
	}

	// 4. Re-detect after cooldown expires (t=6000ms)
	simulatedTime = simulatedTime.Add(4500 * time.Millisecond)
	detector.PollOnce(ctx)
	if callsCount != 2 {
		t.Fatalf("expected callsCount to be 2 after cooldown expired, got %d", callsCount)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 6: clickOpenButton() can click the Open button
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ClickOpenButtonViaCDP(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.ClickOpenButton(ctx, "Open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClickOpenButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call to be recorded")
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Open") {
		t.Errorf("expected expression to contain 'Open', got %s", expr)
	}
	ctxID, _ := lastCall.Params["contextId"].(int64)
	if ctxID != 42 {
		t.Errorf("expected contextId to be 42, got %v", ctxID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 7: clickProceedButton() can click the Proceed button
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ClickProceedButtonViaCDP(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	ok, err := detector.ClickProceedButton(ctx, "Proceed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClickProceedButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	if lastCall == nil {
		t.Fatalf("expected CDP call to be recorded")
	}
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Proceed") {
		t.Errorf("expected expression to contain 'Proceed', got %s", expr)
	}
	ctxID, _ := lastCall.Params["contextId"].(int64)
	if ctxID != 42 {
		t.Errorf("expected contextId to be 42, got %v", ctxID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 8: extractPlanContent() returns plan text from DOM
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ExtractPlanContentReturnsMarkdown(t *testing.T) {
	planText := "# Implementation Plan\n\n## Step 1\nDo something\n\n## Step 2\nDo something else"
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(planText), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	content, err := detector.ExtractPlanContent(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil {
		t.Fatalf("expected content not to be nil")
	}
	if *content != planText {
		t.Errorf("expected content %q, got %q", planText, *content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 9: extractPlanContent() returns null when no content
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ExtractPlanContentReturnsNilWhenNull(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	content, err := detector.ExtractPlanContent(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != nil {
		t.Fatalf("expected content to be nil, got %v", *content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 10: Polling stops after stop()
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_StopHaltsPolling(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo()
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	var mu sync.Mutex
	callsCount := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService:   mockCDP,
		PollInterval: 10 * time.Millisecond,
		OnPlanningRequired: func(info PlanningInfo) {
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
// Test 11: Monitoring continues on CDP error
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ContinuesOnCDPError(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	firstCall := true
	mockInfo := makePlanningInfo(func(i *PlanningInfo) {
		i.PlanTitle = "Recovery Plan"
	})
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return nil, errors.New("CDP error")
		}
		return makeEvaluateResponse(mockInfo), nil
	}

	var calledInfo *PlanningInfo
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
		OnPlanningRequired: func(info PlanningInfo) {
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
	if calledInfo.PlanTitle != "Recovery Plan" {
		t.Errorf("expected PlanTitle 'Recovery Plan', got %s", calledInfo.PlanTitle)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 12: getLastDetectedInfo() returns detected info
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_GetLastDetectedInfoReturnsDetected(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo(func(i *PlanningInfo) {
		i.PlanTitle = "Auth Feature"
		i.PlanSummary = "Add OAuth2"
	})
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(mockInfo), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
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
	if info.PlanTitle != "Auth Feature" || info.PlanSummary != "Add OAuth2" {
		t.Errorf("unexpected info fields: %+v", info)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 13: lastDetectedInfo resets when buttons disappear
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_GetLastDetectedInfoResetsOnDisappear(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo()
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(nil), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
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
// Test 14: clickOpenButton() without arguments uses detected openText
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ClickOpenButtonUsesDetectedText(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo(func(i *PlanningInfo) {
		i.OpenText = "Open Plan"
	})
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	ok, err := detector.ClickOpenButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClickOpenButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Open Plan") {
		t.Errorf("expected expression to contain 'Open Plan', got %s", expr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 15: clickProceedButton() without arguments uses detected proceedText
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ClickProceedButtonUsesDetectedText(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo(func(i *PlanningInfo) {
		i.ProceedText = "Start Implementation"
	})
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(clickScriptResult{OK: true}), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	detector.PollOnce(ctx)

	ok, err := detector.ClickProceedButton(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ClickProceedButton to return true")
	}

	lastCall := mockCDP.getLastCall()
	expr, _ := lastCall.Params["expression"].(string)
	if !strings.Contains(expr, "Start Implementation") {
		t.Errorf("expected expression to contain 'Start Implementation', got %s", expr)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 16: calls without contextId parameter when contextId is null
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_OmitsContextIdWhenNil(t *testing.T) {
	mockCDP := newMockCDP(nil) // contextId is nil
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
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
// Test 17: isActive() returns correct state
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_IsActiveLifecycle(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService:   mockCDP,
		PollInterval: 50 * time.Millisecond,
	})

	if detector.IsActive() {
		t.Errorf("expected IsActive() to be false before Start()")
	}

	detector.Start()
	if !detector.IsActive() {
		t.Errorf("expected IsActive() to be true after Start()")
	}

	_ = detector.Stop()
	if detector.IsActive() {
		t.Errorf("expected IsActive() to be false after Stop()")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 18: DETECT_PLANNING_SCRIPT skips PRE/CODE/STYLE in description
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_DetectScriptContainsSkipTags(t *testing.T) {
	script := DetectPlanningScript
	for _, tag := range []string{"'PRE'", "'CODE'", "'STYLE'", "'SCRIPT'"} {
		if !strings.Contains(script, tag) {
			t.Errorf("expected DetectPlanningScript to contain %s", tag)
		}
	}
	if !strings.Contains(script, ".leading-relaxed.select-text") {
		t.Errorf("expected DetectPlanningScript to contain .leading-relaxed.select-text")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 19: EXTRACT_PLAN_CONTENT_SCRIPT contains htmlToMd converter
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ExtractScriptContainsHtmlToMd(t *testing.T) {
	script := ExtractPlanContentScript
	if !strings.Contains(script, "htmlToMd") {
		t.Errorf("expected ExtractPlanContentScript to contain htmlToMd")
	}
	if !strings.Contains(script, "div.relative.pl-4.pr-4") {
		t.Errorf("expected ExtractPlanContentScript to contain container selector")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 20: extractPlanContent() returns null on CDP error
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_ExtractPlanContentReturnsNilOnCDPError(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return nil, errors.New("CDP connection lost")
	}

	detector := NewPlanningDetector(PlanningDetectorOptions{
		CdpService: mockCDP,
	})

	ctx := context.Background()
	content, err := detector.ExtractPlanContent(ctx)
	if err != nil {
		t.Fatalf("expected nil error on CDP error, got %v", err)
	}
	if content != nil {
		t.Fatalf("expected content to be nil on CDP error, got %v", *content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 21: onResolved fires when planning buttons disappear
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_CallsOnResolvedWhenButtonsDisappear(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockInfo := makePlanningInfo()
	firstCall := true
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		if firstCall {
			firstCall = false
			return makeEvaluateResponse(mockInfo), nil
		}
		return makeEvaluateResponse(nil), nil
	}

	resolvedCalls := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
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
// Test 22: onResolved does not fire when no prior detection
// ─────────────────────────────────────────────────────────────────────────────
func TestPlanningDetector_DoesNotCallOnResolvedWithoutPriorDetection(t *testing.T) {
	mockCDP := newMockCDP(ptrInt64(42))
	mockCDP.callFunc = func(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
		return makeEvaluateResponse(nil), nil
	}

	resolvedCalls := 0
	detector := NewPlanningDetector(PlanningDetectorOptions{
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
