package monitor

import (
	"context"
	"sync"
	"testing"
	"time"
)

func strPtr(s string) *string {
	return &s
}

func cdpResult(val any) map[string]any {
	return map[string]any{
		"result": map[string]any{
			"value": val,
		},
	}
}

func combinedResult(isGenerating bool, quotaError bool, responseText *string, processLogs []string, planningActive bool) map[string]any {
	var respText any = nil
	if responseText != nil {
		respText = *responseText
	}
	var logs []any
	for _, l := range processLogs {
		logs = append(logs, l)
	}
	return map[string]any{
		"result": map[string]any{
			"value": map[string]any{
				"isGenerating":   isGenerating,
				"quotaError":     quotaError,
				"planningActive": planningActive,
				"responseText":   respText,
				"processLogs":    logs,
			},
		},
	}
}

// deterministicMockCdpClient handles concurrent baseline requests deterministically.
type deterministicMockCdpClient struct {
	mu                  sync.Mutex
	baselineText        *string
	baselineLogs        []string
	structuredBase      map[string]any
	pollQueue           []map[string]any
	pollErrors          []error
	structuredPollQueue []map[string]any
	callHistory         []string
	listeners           map[string][]func(params map[string]any)
	primaryCtxID        *int
}

func newDeterministicMockCdpClient() *deterministicMockCdpClient {
	ctxID := 1
	return &deterministicMockCdpClient{
		listeners:    make(map[string][]func(params map[string]any)),
		primaryCtxID: &ctxID,
	}
}

func (m *deterministicMockCdpClient) setBaselineText(text *string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baselineText = text
}

func (m *deterministicMockCdpClient) queuePollResponse(res map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pollQueue = append(m.pollQueue, res)
	m.pollErrors = append(m.pollErrors, nil)
}

func (m *deterministicMockCdpClient) queueStructuredPollResponse(res map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.structuredPollQueue = append(m.structuredPollQueue, res)
}

func (m *deterministicMockCdpClient) queuePollError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pollQueue = append(m.pollQueue, nil)
	m.pollErrors = append(m.pollErrors, err)
}

func (m *deterministicMockCdpClient) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	expr, _ := params["expression"].(string)
	m.callHistory = append(m.callHistory, expr)

	// Check if this is the RESPONSE_TEXT baseline call
	if expr == ResponseSelectors.ResponseText {
		if m.baselineText != nil {
			return cdpResult(*m.baselineText), nil
		}
		return cdpResult(nil), nil
	}

	// Check if this is the PROCESS_LOGS baseline call
	if expr == ResponseSelectors.ProcessLogs {
		if len(m.baselineLogs) > 0 {
			return cdpResult(m.baselineLogs), nil
		}
		return cdpResult(nil), nil
	}

	// Check if this is the RESPONSE_STRUCTURED call (poll or baseline)
	if expr == ResponseSelectors.ResponseStructured {
		if len(m.structuredPollQueue) > 0 {
			res := m.structuredPollQueue[0]
			m.structuredPollQueue = m.structuredPollQueue[1:]
			return res, nil
		}
		if m.structuredBase != nil {
			return m.structuredBase, nil
		}
		return cdpResult(nil), nil
	}

	// Check if this is the DOM_OBSERVER injection
	if expr == ResponseSelectors.DomObserver {
		return cdpResult(map[string]any{"ok": true, "observing": "body"}), nil
	}

	// Check if this is CLICK_STOP_BUTTON
	if expr == ResponseSelectors.ClickStopButton {
		if len(m.pollQueue) > 0 {
			res := m.pollQueue[0]
			err := m.pollErrors[0]
			m.pollQueue = m.pollQueue[1:]
			m.pollErrors = m.pollErrors[1:]
			return res, err
		}
		return cdpResult(map[string]any{"ok": true, "method": "tooltip-id"}), nil
	}

	// Otherwise, it's COMBINED_POLL or custom poll call
	if len(m.pollQueue) > 0 {
		res := m.pollQueue[0]
		err := m.pollErrors[0]
		m.pollQueue = m.pollQueue[1:]
		m.pollErrors = m.pollErrors[1:]
		return res, err
	}

	return cdpResult(nil), nil
}

func (m *deterministicMockCdpClient) GetPrimaryContextID() *int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.primaryCtxID
}

func (m *deterministicMockCdpClient) On(event string, listener func(params map[string]any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners[event] = append(m.listeners[event], listener)
}

func (m *deterministicMockCdpClient) Off(event string, listener func(params map[string]any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.listeners[event]
	if len(list) > 0 {
		m.listeners[event] = list[:len(list)-1]
	}
}

func (m *deterministicMockCdpClient) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.callHistory)
}

func TestLeanResponseMonitor_StartCapturesBaselineAndSetsWaiting(t *testing.T) {
	cdp := newDeterministicMockCdpClient()
	cdp.setBaselineText(strPtr("existing text"))

	var phases []ResponsePhase
	var mu sync.Mutex
	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnPhaseChange: func(phase ResponsePhase, text *string) {
			mu.Lock()
			phases = append(phases, phase)
			mu.Unlock()
		},
	})

	err := monitor.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected Start error: %v", err)
	}

	if monitor.GetPhase() != PhaseWaiting {
		t.Errorf("expected phase to be 'waiting', got %s", monitor.GetPhase())
	}

	mu.Lock()
	hasWaiting := false
	for _, p := range phases {
		if p == PhaseWaiting {
			hasWaiting = true
			break
		}
	}
	mu.Unlock()
	if !hasWaiting {
		t.Errorf("expected phases to contain 'waiting', got %v", phases)
	}

	// 3 CDP calls in legacy mode: RESPONSE_TEXT baseline, PROCESS_LOGS baseline, DOM_OBSERVER injection
	if cdp.callCount() != 3 {
		t.Errorf("expected 3 CDP calls during start, got %d", cdp.callCount())
	}

	_ = monitor.Stop()
}

func TestLeanResponseMonitor_StopButtonSetsPhaseThinking(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var phases []ResponsePhase
	var mu sync.Mutex
	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnPhaseChange: func(phase ResponsePhase, text *string) {
			mu.Lock()
			phases = append(phases, phase)
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll: COMBINED_POLL returns isGenerating=true, no text
	cdp.queuePollResponse(combinedResult(true, false, nil, nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	hasThinking := false
	for _, p := range phases {
		if p == PhaseThinking {
			hasThinking = true
			break
		}
	}
	mu.Unlock()

	if !hasThinking {
		t.Errorf("expected phases to contain 'thinking', got %v", phases)
	}

	_ = monitor.Stop()
}

func TestLeanResponseMonitor_TextUpdateTriggersProgressAndGenerating(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var phases []ResponsePhase
	var progressTexts []string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnPhaseChange: func(phase ResponsePhase, text *string) {
			mu.Lock()
			phases = append(phases, phase)
			mu.Unlock()
		},
		OnProgress: func(text string) {
			mu.Lock()
			progressTexts = append(progressTexts, text)
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll: COMBINED_POLL returns isGenerating=true, responseText='Hello'
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Hello"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if len(progressTexts) == 0 || progressTexts[0] != "Hello" {
		t.Errorf("expected progressTexts to contain 'Hello', got %v", progressTexts)
	}
	hasGenerating := false
	for _, p := range phases {
		if p == PhaseGenerating {
			hasGenerating = true
			break
		}
	}
	mu.Unlock()

	if !hasGenerating {
		t.Errorf("expected phases to contain 'generating', got %v", phases)
	}

	_ = monitor.Stop()
}

func TestLeanResponseMonitor_StopGone3TimesTriggersComplete(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: isGenerating=true, text="response"
	cdp.queuePollResponse(combinedResult(true, false, strPtr("response"), nil, false))
	monitor.poll(context.Background())

	// Poll 2: isGenerating=false (gone count 1)
	cdp.queuePollResponse(combinedResult(false, false, strPtr("response"), nil, false))
	monitor.poll(context.Background())
	mu.Lock()
	if completedText != nil {
		t.Errorf("expected completedText to be nil at gone count 1")
	}
	mu.Unlock()

	// Poll 3: isGenerating=false (gone count 2)
	cdp.queuePollResponse(combinedResult(false, false, strPtr("response"), nil, false))
	monitor.poll(context.Background())
	mu.Lock()
	if completedText != nil {
		t.Errorf("expected completedText to be nil at gone count 2")
	}
	mu.Unlock()

	// Poll 4: isGenerating=false (gone count 3) -> complete
	cdp.queuePollResponse(combinedResult(false, false, strPtr("response"), nil, false))
	monitor.poll(context.Background())
	mu.Lock()
	if completedText == nil || *completedText != "response" {
		t.Errorf("expected completedText to be 'response', got %v", completedText)
	}
	mu.Unlock()
}

func TestLeanResponseMonitor_StopReappearingResetsCounter(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: stop=true, text="resp"
	cdp.queuePollResponse(combinedResult(true, false, strPtr("resp"), nil, false))
	monitor.poll(context.Background())

	// Poll 2: stop=false (gone 1)
	cdp.queuePollResponse(combinedResult(false, false, strPtr("resp"), nil, false))
	monitor.poll(context.Background())

	// Poll 3: stop=false (gone 2)
	cdp.queuePollResponse(combinedResult(false, false, strPtr("resp"), nil, false))
	monitor.poll(context.Background())

	// Poll 4: stop=true again -> resets counter
	cdp.queuePollResponse(combinedResult(true, false, strPtr("resp"), nil, false))
	monitor.poll(context.Background())

	// Poll 5: stop=false (gone 1 again)
	cdp.queuePollResponse(combinedResult(false, false, strPtr("resp"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if completedText != nil {
		t.Errorf("expected completedText to be nil after reset, got %v", completedText)
	}
	mu.Unlock()

	_ = monitor.Stop()
}

func TestLeanResponseMonitor_TextChangeDoesNotResetStopGoneCounter(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: stop=true, text='first'
	cdp.queuePollResponse(combinedResult(true, false, strPtr("first"), nil, false))
	monitor.poll(context.Background())

	// Poll 2: stop=false (gone 1), text unchanged
	cdp.queuePollResponse(combinedResult(false, false, strPtr("first"), nil, false))
	monitor.poll(context.Background())

	// Poll 3: stop=false (gone 2), text changed — counter must NOT reset
	cdp.queuePollResponse(combinedResult(false, false, strPtr("first updated"), nil, false))
	monitor.poll(context.Background())

	// Poll 4: stop=false (gone 3) — should complete despite text change in poll 3
	cdp.queuePollResponse(combinedResult(false, false, strPtr("first updated"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if completedText == nil || *completedText != "first updated" {
		t.Errorf("expected completedText to be 'first updated', got %v", completedText)
	}
	mu.Unlock()
}

func TestLeanResponseMonitor_ContinuousTextUpdatesAfterStopGone(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: stop=true, text='token1'
	cdp.queuePollResponse(combinedResult(true, false, strPtr("token1"), nil, false))
	monitor.poll(context.Background())

	// Poll 2: stop=false, text='token1 token2'
	cdp.queuePollResponse(combinedResult(false, false, strPtr("token1 token2"), nil, false))
	monitor.poll(context.Background())

	// Poll 3: stop=false, text='token1 token2 token3'
	cdp.queuePollResponse(combinedResult(false, false, strPtr("token1 token2 token3"), nil, false))
	monitor.poll(context.Background())

	// Poll 4: stop=false, text='token1 token2 token3 final'
	cdp.queuePollResponse(combinedResult(false, false, strPtr("token1 token2 token3 final"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if completedText == nil || *completedText != "token1 token2 token3 final" {
		t.Errorf("expected completedText to be 'token1 token2 token3 final', got %v", completedText)
	}
	mu.Unlock()
}

func TestLeanResponseMonitor_BaselineTextSuppressed(t *testing.T) {
	cdp := newDeterministicMockCdpClient()
	cdp.setBaselineText(strPtr("old response"))

	var progressTexts []string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnProgress: func(text string) {
			mu.Lock()
			progressTexts = append(progressTexts, text)
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: COMBINED_POLL returns text='old response' (same as baseline)
	cdp.queuePollResponse(combinedResult(false, false, strPtr("old response"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	for _, text := range progressTexts {
		if text == "old response" {
			t.Errorf("expected baseline text 'old response' to be suppressed, but was emitted in onProgress")
		}
	}
	mu.Unlock()

	_ = monitor.Stop()
}

func TestLeanResponseMonitor_BaselineSuppressionDoesNotBlockCompletion(t *testing.T) {
	cdp := newDeterministicMockCdpClient()
	cdp.setBaselineText(strPtr("old response"))

	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: generation starts but extracted text is still baseline (suppressed)
	cdp.queuePollResponse(combinedResult(true, false, strPtr("old response"), nil, false))
	monitor.poll(context.Background())

	// Poll 2-4: stop disappears 3 times, text remains baseline
	for i := 0; i < 3; i++ {
		cdp.queuePollResponse(combinedResult(false, false, strPtr("old response"), nil, false))
		monitor.poll(context.Background())
	}

	mu.Lock()
	if completedText == nil || *completedText != "" {
		t.Errorf("expected completedText to be '', got %v", completedText)
	}
	mu.Unlock()
}

func TestLeanResponseMonitor_TimeoutTriggersOnTimeout(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	timedOut := make(chan string, 1)

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		MaxDurationMs:        30, // 30ms timeout for test speed
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnTimeout: func(lastText string) {
			timedOut <- lastText
		},
	})

	_ = monitor.Start(context.Background())

	select {
	case text := <-timedOut:
		if text != "" {
			t.Errorf("expected timeout text to be empty, got '%s'", text)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for OnTimeout callback")
	}

	if monitor.GetPhase() != PhaseTimeout {
		t.Errorf("expected phase to be 'timeout', got %s", monitor.GetPhase())
	}
}

func TestLeanResponseMonitor_QuotaDetectionNoTextImmediateComplete(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll: COMBINED_POLL returns quotaError=true, no text
	cdp.queuePollResponse(combinedResult(false, true, nil, nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if completedText == nil || *completedText != "" {
		t.Errorf("expected completedText to be '', got %v", completedText)
	}
	mu.Unlock()

	if monitor.GetPhase() != PhaseQuotaReached {
		t.Errorf("expected phase to be 'quotaReached', got %s", monitor.GetPhase())
	}
}

func TestLeanResponseMonitor_ClickStopButtonSuccess(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService: cdp,
	})

	_ = monitor.Start(context.Background())

	cdp.queuePollResponse(cdpResult(map[string]any{
		"ok":     true,
		"method": "tooltip-id",
	}))

	res := monitor.ClickStopButton(context.Background())
	if !res.Ok || res.Method != "tooltip-id" {
		t.Errorf("expected {Ok: true, Method: 'tooltip-id'}, got %+v", res)
	}

	if monitor.IsActive() {
		t.Errorf("expected monitor to be stopped after ClickStopButton")
	}
}

func TestLeanResponseMonitor_SubscribesToBindingCalled(t *testing.T) {
	cdp := newDeterministicMockCdpClient()
	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService: cdp,
	})

	if len(cdp.listeners["Runtime.bindingCalled"]) != 1 {
		t.Fatalf("expected 1 listener registered on Runtime.bindingCalled, got %d", len(cdp.listeners["Runtime.bindingCalled"]))
	}

	_ = monitor.Stop()

	if len(cdp.listeners["Runtime.bindingCalled"]) != 0 {
		t.Fatalf("expected 0 listeners after stop, got %d", len(cdp.listeners["Runtime.bindingCalled"]))
	}
}

func TestLeanResponseMonitor_DefaultPollIntervalIs2000ms(t *testing.T) {
	cdp := newDeterministicMockCdpClient()
	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService: cdp,
	})

	if monitor.pollInterval != 2000*time.Millisecond {
		t.Errorf("expected default pollInterval to be 2000ms, got %v", monitor.pollInterval)
	}
}

func TestLeanResponseMonitor_HasGetLastExtractionSource(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:     cdp,
		ExtractionMode: ExtractionModeLegacy,
	})

	if monitor.GetLastExtractionSource() != nil {
		t.Errorf("expected GetLastExtractionSource to be nil before poll, got %v", monitor.GetLastExtractionSource())
	}

	_ = monitor.Start(context.Background())

	cdp.queuePollResponse(combinedResult(true, false, strPtr("test text"), nil, false))
	monitor.poll(context.Background())

	src := monitor.GetLastExtractionSource()
	if src == nil || *src != SourceLegacy {
		t.Errorf("expected GetLastExtractionSource to be 'legacy', got %v", src)
	}

	_ = monitor.Stop()
}

func TestLeanResponseMonitor_SingleCdpCallInLegacyMode(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:     cdp,
		ExtractionMode: ExtractionModeLegacy,
	})

	_ = monitor.Start(context.Background())
	callsAfterStart := cdp.callCount()

	// Poll with no text change: 1 COMBINED_POLL call
	cdp.queuePollResponse(combinedResult(false, false, nil, nil, false))
	monitor.poll(context.Background())

	if cdp.callCount()-callsAfterStart != 1 {
		t.Errorf("expected 1 CDP call in poll, got %d", cdp.callCount()-callsAfterStart)
	}

	// Poll with text change: still 1 call
	callsBefore := cdp.callCount()
	cdp.queuePollResponse(combinedResult(true, false, strPtr("new text"), nil, false))
	monitor.poll(context.Background())

	if cdp.callCount()-callsBefore != 1 {
		t.Errorf("expected 1 CDP call in poll with text, got %d", cdp.callCount()-callsBefore)
	}

	_ = monitor.Stop()
}
