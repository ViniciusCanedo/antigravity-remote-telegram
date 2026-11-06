package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestResponseMonitor_IncrementalProgressAndNoDuplicate(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var progressUpdates []string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnProgress: func(text string) {
			mu.Lock()
			progressUpdates = append(progressUpdates, text)
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: 'Hello'
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Hello"), nil, false))
	monitor.poll(context.Background())

	// Poll 2: 'Hello World'
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Hello World"), nil, false))
	monitor.poll(context.Background())

	// Poll 3: 'Hello World' (unchanged)
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Hello World"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	defer mu.Unlock()

	if len(progressUpdates) != 2 {
		t.Fatalf("expected exactly 2 onProgress calls, got %d: %v", len(progressUpdates), progressUpdates)
	}
	if progressUpdates[0] != "Hello" || progressUpdates[1] != "Hello World" {
		t.Errorf("unexpected progress updates: %v", progressUpdates)
	}

	_ = monitor.Stop()
}

func TestResponseMonitor_Stop_StopsPollingAndCallbacks(t *testing.T) {
	t.Run("stops polling after Stop()", func(t *testing.T) {
		cdp := newDeterministicMockCdpClient()
		monitor := NewResponseMonitor(ResponseMonitorOptions{
			CdpService:     cdp,
			PollIntervalMs: 50000,
		})

		_ = monitor.Start(context.Background())
		if !monitor.IsActive() {
			t.Errorf("expected monitor to be active")
		}

		_ = monitor.Stop()
		if monitor.IsActive() {
			t.Errorf("expected monitor to be stopped")
		}

		// Subsequent poll call should abort immediately because isRunning is false
		countBefore := cdp.callCount()
		monitor.poll(context.Background())
		if cdp.callCount() != countBefore {
			t.Errorf("expected no CDP calls after stop, got callCount=%d vs %d", cdp.callCount(), countBefore)
		}
	})

	t.Run("continues monitoring on transient CDP error", func(t *testing.T) {
		cdp := newDeterministicMockCdpClient()
		var progressUpdates []string
		var mu sync.Mutex

		monitor := NewResponseMonitor(ResponseMonitorOptions{
			CdpService:           cdp,
			PollIntervalMs:       50000,
			StopGoneConfirmCount: 3,
			ExtractionMode:       ExtractionModeLegacy,
			OnProgress: func(text string) {
				mu.Lock()
				progressUpdates = append(progressUpdates, text)
				mu.Unlock()
			},
		})

		_ = monitor.Start(context.Background())

		// Poll 1: Transient CDP error
		cdp.queuePollError(errors.New("Evaluation failed: ReferenceError: foo is not defined"))
		monitor.poll(context.Background())

		// Monitor remains alive
		if !monitor.IsActive() {
			t.Errorf("expected monitor to remain active after non-WS error")
		}

		// Poll 2: Succeeds
		cdp.queuePollResponse(combinedResult(true, false, strPtr("recovered"), nil, false))
		monitor.poll(context.Background())

		mu.Lock()
		if len(progressUpdates) != 1 || progressUpdates[0] != "recovered" {
			t.Errorf("expected progress update 'recovered', got %v", progressUpdates)
		}
		mu.Unlock()

		_ = monitor.Stop()
	})

	t.Run("stops and calls OnTimeout after 5 consecutive WebSocket errors", func(t *testing.T) {
		cdp := newDeterministicMockCdpClient()
		timedOut := make(chan string, 1)

		monitor := NewResponseMonitor(ResponseMonitorOptions{
			CdpService:           cdp,
			PollIntervalMs:       50000,
			StopGoneConfirmCount: 3,
			ExtractionMode:       ExtractionModeLegacy,
			OnTimeout: func(lastText string) {
				timedOut <- lastText
			},
		})

		_ = monitor.Start(context.Background())

		// First 4 WebSocket errors: monitor should continue running
		for i := 1; i <= 4; i++ {
			cdp.queuePollError(errors.New("WebSocket is not connected"))
			monitor.poll(context.Background())
			if !monitor.IsActive() {
				t.Fatalf("monitor should still be active at error #%d", i)
			}
		}

		// 5th WebSocket error: triggers stop and timeout
		cdp.queuePollError(errors.New("WebSocket is not connected"))
		monitor.poll(context.Background())

		if monitor.IsActive() {
			t.Errorf("expected monitor to stop after 5 consecutive WebSocket errors")
		}

		select {
		case <-timedOut:
			// Success
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("expected OnTimeout to be called on 5th WS error")
		}
	})
}

func TestResponseMonitor_PhaseTransitions(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var recordedPhases []ResponsePhase
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 1, // Complete immediately on 1 stop gone for test
		ExtractionMode:       ExtractionModeLegacy,
		OnPhaseChange: func(phase ResponsePhase, text *string) {
			mu.Lock()
			recordedPhases = append(recordedPhases, phase)
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background()) // sets PhaseWaiting

	// Poll 1: Stop button appears -> PhaseThinking
	cdp.queuePollResponse(combinedResult(true, false, nil, nil, false))
	monitor.poll(context.Background())

	// Poll 2: Text arrives -> PhaseGenerating
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Streaming content"), nil, false))
	monitor.poll(context.Background())

	// Poll 3: Stop gone -> PhaseComplete
	cdp.queuePollResponse(combinedResult(false, false, strPtr("Streaming content"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	defer mu.Unlock()

	expectedOrder := []ResponsePhase{PhaseWaiting, PhaseThinking, PhaseGenerating, PhaseComplete}
	if len(recordedPhases) != len(expectedOrder) {
		t.Fatalf("expected %d phase transitions, got %d: %v", len(expectedOrder), len(recordedPhases), recordedPhases)
	}

	for i, expected := range expectedOrder {
		if recordedPhases[i] != expected {
			t.Errorf("phase[%d] = %s, expected %s", i, recordedPhases[i], expected)
		}
	}
}

func TestResponseMonitor_StructuredPayloadBodyProcessAndThinking(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var progressUpdates []string
	var processLogs []string
	var thinkingLogs []string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeStructured,
		OnProgress: func(text string) {
			mu.Lock()
			progressUpdates = append(progressUpdates, text)
			mu.Unlock()
		},
		OnProcessLog: func(text string) {
			mu.Lock()
			processLogs = append(processLogs, text)
			mu.Unlock()
		},
		OnThinkingLog: func(text string) {
			mu.Lock()
			thinkingLogs = append(thinkingLogs, text)
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Structured payload mock:
	// segments:
	// 1. assistant-body: "<b>Hello World</b>"
	// 2. tool-call: "Reading file src/index.ts"
	// 3. thinking: "Thinking about the implementation..."
	// 4. feedback: "Good"
	structuredPayload := map[string]any{
		"source":      "dom-structured",
		"extractedAt": time.Now().UnixMilli(),
		"segments": []any{
			map[string]any{
				"kind": "assistant-body",
				"text": "<b>Hello World</b>",
				"role": "assistant",
			},
			map[string]any{
				"kind": "tool-call",
				"text": "Reading file src/index.ts",
				"role": "assistant",
			},
			map[string]any{
				"kind": "thinking",
				"text": "Thinking about the implementation...",
				"role": "assistant",
			},
			map[string]any{
				"kind": "feedback",
				"text": "Good",
				"role": "assistant",
			},
		},
	}

	// Poll 1: COMBINED_POLL returns generating, RESPONSE_STRUCTURED returns the structured payload
	cdp.queuePollResponse(combinedResult(true, false, nil, nil, false))
	cdp.queueStructuredPollResponse(cdpResult(structuredPayload))
	monitor.poll(context.Background())

	mu.Lock()
	if len(progressUpdates) != 1 || progressUpdates[0] != "<b>Hello World</b>" {
		t.Errorf("expected progress update '<b>Hello World</b>', got %v", progressUpdates)
	}
	if len(processLogs) != 1 || processLogs[0] != "Reading file src/index.ts" {
		t.Errorf("expected process log 'Reading file src/index.ts', got %v", processLogs)
	}
	if len(thinkingLogs) != 1 || thinkingLogs[0] != "Thinking about the implementation..." {
		t.Errorf("expected thinking log 'Thinking about the implementation...', got %v", thinkingLogs)
	}
	mu.Unlock()

	// Poll 2: Identical payload — logs and body should be deduplicated
	cdp.queuePollResponse(combinedResult(true, false, nil, nil, false))
	cdp.queueStructuredPollResponse(cdpResult(structuredPayload))
	monitor.poll(context.Background())

	mu.Lock()
	if len(progressUpdates) != 1 {
		t.Errorf("expected progressUpdates to remain length 1 due to dedup, got %d", len(progressUpdates))
	}
	if len(processLogs) != 1 {
		t.Errorf("expected processLogs to remain length 1 due to dedup, got %d", len(processLogs))
	}
	if len(thinkingLogs) != 1 {
		t.Errorf("expected thinkingLogs to remain length 1 due to dedup, got %d", len(thinkingLogs))
	}
	mu.Unlock()

	_ = monitor.Stop()
}

func TestResponseMonitor_PlanningDialogDefersCompletion(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 2,
		ExtractionMode:       ExtractionModeLegacy,
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Poll 1: Stop button present
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Need user confirmation"), nil, false))
	monitor.poll(context.Background())

	// Poll 2: Stop button gone, BUT planningActive=true
	cdp.queuePollResponse(combinedResult(false, false, strPtr("Need user confirmation"), nil, true))
	monitor.poll(context.Background())

	// Poll 3: Still planningActive=true
	cdp.queuePollResponse(combinedResult(false, false, strPtr("Need user confirmation"), nil, true))
	monitor.poll(context.Background())

	mu.Lock()
	if completedText != nil {
		t.Errorf("expected completion to be deferred while planningActive=true, but complete was called")
	}
	if monitor.stopGoneCount != 0 {
		t.Errorf("expected stopGoneCount to remain 0 during planning, got %d", monitor.stopGoneCount)
	}
	mu.Unlock()

	// Poll 4: planning dialog closed (planningActive=false), gone count 1
	cdp.queuePollResponse(combinedResult(false, false, strPtr("Need user confirmation"), nil, false))
	monitor.poll(context.Background())

	// Poll 5: gone count 2 -> completed!
	cdp.queuePollResponse(combinedResult(false, false, strPtr("Need user confirmation"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if completedText == nil || *completedText != "Need user confirmation" {
		t.Errorf("expected completion after dialog resolved, got %v", completedText)
	}
	mu.Unlock()
}

func TestResponseMonitor_AttachesPrimaryContextId(t *testing.T) {
	cdp := newDeterministicMockCdpClient()
	ctxID := 42
	cdp.primaryCtxID = &ctxID

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService: cdp,
	})

	params := monitor.buildEvaluateParams("1 + 1")
	if val, ok := params["contextId"].(int); !ok || val != 42 {
		t.Errorf("expected contextId: 42 in evaluate params, got %v", params["contextId"])
	}
}

func TestResponseMonitor_StartPassive(t *testing.T) {
	cdp := newDeterministicMockCdpClient()
	cdp.setBaselineText(strPtr("Existing baseline"))

	var progressUpdates []string
	var completedText *string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 1, // complete after 1 stop-gone
		ExtractionMode:       ExtractionModeLegacy,
		OnProgress: func(text string) {
			mu.Lock()
			progressUpdates = append(progressUpdates, text)
			mu.Unlock()
		},
		OnComplete: func(finalText string, meta *CompleteMeta) {
			mu.Lock()
			completedText = &finalText
			mu.Unlock()
		},
	})

	err := monitor.StartPassive(context.Background())
	if err != nil {
		t.Fatalf("unexpected StartPassive error: %v", err)
	}

	// Should start directly in PhaseGenerating (not PhaseWaiting)
	if monitor.GetPhase() != PhaseGenerating {
		t.Errorf("expected StartPassive phase to be 'generating', got %s", monitor.GetPhase())
	}

	// Poll 1: Stop button is active, updated response text arrives -> immediately triggers OnProgress
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Updated response text"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if len(progressUpdates) != 1 || progressUpdates[0] != "Updated response text" {
		t.Errorf("expected OnProgress called with 'Updated response text', got %v", progressUpdates)
	}
	mu.Unlock()

	// Poll 2: Stop button disappears -> triggers OnComplete immediately (threshold 1)
	cdp.queuePollResponse(combinedResult(false, false, strPtr("Updated response text"), nil, false))
	monitor.poll(context.Background())

	mu.Lock()
	if completedText == nil || *completedText != "Updated response text" {
		t.Errorf("expected OnComplete with 'Updated response text', got %v", completedText)
	}
	mu.Unlock()
}

func TestResponseMonitor_DomChangedEventDebouncing(t *testing.T) {
	cdp := newDeterministicMockCdpClient()

	var progressUpdates []string
	var mu sync.Mutex

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService:           cdp,
		PollIntervalMs:       50000,
		StopGoneConfirmCount: 3,
		ExtractionMode:       ExtractionModeLegacy,
		OnProgress: func(text string) {
			mu.Lock()
			progressUpdates = append(progressUpdates, text)
			mu.Unlock()
		},
	})

	_ = monitor.Start(context.Background())

	// Queue poll response for when event triggers
	cdp.queuePollResponse(combinedResult(true, false, strPtr("Event pushed update"), nil, false))

	// Fire Runtime.bindingCalled
	listeners := cdp.listeners["Runtime.bindingCalled"]
	if len(listeners) == 0 {
		t.Fatalf("no listener on Runtime.bindingCalled")
	}

	for _, l := range listeners {
		l(map[string]any{"name": "onantigravity-telegram-remoteDOMChange"})
	}

	// Debounce is 500ms when not generating; wait for it
	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	if len(progressUpdates) != 1 || progressUpdates[0] != "Event pushed update" {
		t.Errorf("expected event push to trigger progress 'Event pushed update', got %v", progressUpdates)
	}
	mu.Unlock()

	_ = monitor.Stop()
}
