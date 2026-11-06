package monitor

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"antigravity-telegram-remote/internal/logger"
)

// ResponsePhase represents the life-cycle phases of AI response generation.
type ResponsePhase string

const (
	PhaseWaiting      ResponsePhase = "waiting"
	PhaseThinking     ResponsePhase = "thinking"
	PhaseGenerating   ResponsePhase = "generating"
	PhaseComplete     ResponsePhase = "complete"
	PhaseTimeout      ResponsePhase = "timeout"
	PhaseQuotaReached ResponsePhase = "quotaReached"
)

// ExtractionMode defines the extraction mode ("structured" or "legacy").
type ExtractionMode string

const (
	ExtractionModeStructured ExtractionMode = "structured"
	ExtractionModeLegacy     ExtractionMode = "legacy"
)

// ExtractionSource indicates whether the extracted text originates from structured DOM segments or legacy plain text.
type ExtractionSource string

const (
	SourceStructured ExtractionSource = "structured"
	SourceLegacy     ExtractionSource = "legacy"
)

// CompleteMeta provides metadata passed to OnComplete callback.
type CompleteMeta struct {
	Source ExtractionSource `json:"source"`
}

// ClickStopButtonResult represents the outcome of calling ClickStopButton().
type ClickStopButtonResult struct {
	Ok     bool   `json:"ok"`
	Method string `json:"method,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CombinedPollPayload maps the JSON payload returned by RESPONSE_SELECTORS.COMBINED_POLL.
type CombinedPollPayload struct {
	IsGenerating   bool     `json:"isGenerating"`
	QuotaError     bool     `json:"quotaError"`
	PlanningActive bool     `json:"planningActive"`
	ResponseText   *string  `json:"responseText"`
	ProcessLogs    []string `json:"processLogs"`
}

// CdpClient defines the minimal CDP client interface required by ResponseMonitor.
type CdpClient interface {
	Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)
	GetPrimaryContextID() *int
	On(event string, listener func(params map[string]any))
	Off(event string, listener func(params map[string]any))
}

// ResponseMonitorOptions configures a ResponseMonitor instance.
type ResponseMonitorOptions struct {
	CdpService           CdpClient
	PollIntervalMs       int                                        // default: 2000
	MaxDurationMs        int                                        // default: 300000; if < 0 disabled
	StopGoneConfirmCount int                                        // default: 3
	ExtractionMode       ExtractionMode                             // default: os.Getenv("EXTRACTION_MODE") == "legacy" ? "legacy" : "structured"
	OnProgress           func(text string)                          // optional
	OnComplete           func(finalText string, meta *CompleteMeta) // optional
	OnTimeout            func(lastText string)                      // optional
	OnPhaseChange        func(phase ResponsePhase, text *string)    // optional
	OnProcessLog         func(text string)                          // optional
	OnThinkingLog        func(text string)                          // optional
}

// ResponseMonitor coordinates response streaming observation and state transitions.
type ResponseMonitor struct {
	mu                   sync.Mutex
	pollMu               sync.Mutex
	cdpService           CdpClient
	pollInterval         time.Duration
	maxDuration          time.Duration
	stopGoneConfirmCount int
	extractionMode       ExtractionMode
	onProgress           func(text string)
	onComplete           func(finalText string, meta *CompleteMeta)
	onTimeout            func(lastText string)
	onPhaseChange        func(phase ResponsePhase, text *string)
	onProcessLog         func(text string)
	onThinkingLog        func(text string)

	pollTimer               *time.Timer
	timeoutTimer            *time.Timer
	domChangedDebounceTimer *time.Timer
	isRunning               bool
	lastText                *string
	baselineText            *string
	generationStarted       bool
	currentPhase            ResponsePhase
	stopGoneCount           int
	quotaDetected           bool
	seenProcessLogKeys      map[string]struct{}
	seenThinkingLogKeys     map[string]struct{}
	structuredDiagLogged    bool
	lastExtractionSource    *ExtractionSource
	consecutiveWsErrors     int
	bindingListener         func(params map[string]any)
}

// NewResponseMonitor creates a new ResponseMonitor instance and registers the push binding listener.
func NewResponseMonitor(options ResponseMonitorOptions) *ResponseMonitor {
	pollIntervalMs := options.PollIntervalMs
	if pollIntervalMs <= 0 {
		pollIntervalMs = 2000
	}

	maxDurationMs := options.MaxDurationMs
	if maxDurationMs == 0 {
		maxDurationMs = 300000
	}

	stopGoneConfirmCount := options.StopGoneConfirmCount
	if stopGoneConfirmCount <= 0 {
		stopGoneConfirmCount = 3
	}

	extractionMode := options.ExtractionMode
	if extractionMode == "" {
		if os.Getenv("EXTRACTION_MODE") == "legacy" {
			extractionMode = ExtractionModeLegacy
		} else {
			extractionMode = ExtractionModeStructured
		}
	}

	var maxDuration time.Duration
	if maxDurationMs > 0 {
		maxDuration = time.Duration(maxDurationMs) * time.Millisecond
	}

	m := &ResponseMonitor{
		cdpService:           options.CdpService,
		pollInterval:         time.Duration(pollIntervalMs) * time.Millisecond,
		maxDuration:          maxDuration,
		stopGoneConfirmCount: stopGoneConfirmCount,
		extractionMode:       extractionMode,
		onProgress:           options.OnProgress,
		onComplete:           options.OnComplete,
		onTimeout:            options.OnTimeout,
		onPhaseChange:        options.OnPhaseChange,
		onProcessLog:         options.OnProcessLog,
		onThinkingLog:        options.OnThinkingLog,
		currentPhase:         PhaseWaiting,
		seenProcessLogKeys:   make(map[string]struct{}),
		seenThinkingLogKeys:  make(map[string]struct{}),
	}

	m.bindingListener = func(params map[string]any) {
		name, _ := params["name"].(string)
		if name == "onantigravity-telegram-remoteDOMChange" {
			m.mu.Lock()
			running := m.isRunning
			m.mu.Unlock()
			if running {
				m.handleDomChangedEvent()
			}
		}
	}

	if m.cdpService != nil {
		m.cdpService.On("Runtime.bindingCalled", m.bindingListener)
	}

	return m
}

// Start begins response monitoring in active mode (waiting for generation to start).
func (m *ResponseMonitor) Start(ctx context.Context) error {
	return m.initMonitoring(ctx, false)
}

// StartPassive begins response monitoring in passive mode (assuming generation has already started).
func (m *ResponseMonitor) StartPassive(ctx context.Context) error {
	return m.initMonitoring(ctx, true)
}

func (m *ResponseMonitor) initMonitoring(ctx context.Context, passive bool) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return nil
	}
	m.isRunning = true
	m.lastText = nil
	m.baselineText = nil
	m.generationStarted = passive
	if passive {
		m.currentPhase = PhaseGenerating
	} else {
		m.currentPhase = PhaseWaiting
	}
	m.stopGoneCount = 0
	m.quotaDetected = false
	m.seenProcessLogKeys = make(map[string]struct{})
	m.seenThinkingLogKeys = make(map[string]struct{})
	m.structuredDiagLogged = false
	m.lastExtractionSource = nil
	m.consecutiveWsErrors = 0

	cdp := m.cdpService
	extractionMode := m.extractionMode
	initialPhase := m.currentPhase
	onPhaseChange := m.onPhaseChange
	maxDuration := m.maxDuration
	m.mu.Unlock()

	if onPhaseChange != nil {
		onPhaseChange(initialPhase, nil)
	}

	// Capture baselines in parallel (text + process logs + optional structured)
	if cdp != nil {
		var baseResult map[string]any
		var logResult map[string]any
		var structuredBaseline map[string]any

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			res, err := cdp.Call(ctx, "Runtime.evaluate", m.buildEvaluateParams(ResponseSelectors.ResponseText))
			if err == nil {
				baseResult = res
			}
		}()

		go func() {
			defer wg.Done()
			res, err := cdp.Call(ctx, "Runtime.evaluate", m.buildEvaluateParams(ResponseSelectors.ProcessLogs))
			if err == nil {
				logResult = res
			}
		}()

		if extractionMode == ExtractionModeStructured {
			wg.Add(1)
			go func() {
				defer wg.Done()
				res, err := cdp.Call(ctx, "Runtime.evaluate", m.buildEvaluateParams(ResponseSelectors.ResponseStructured))
				if err == nil {
					structuredBaseline = res
				}
			}()
		}

		wg.Wait()

		m.mu.Lock()
		// Baseline text
		baseVal := extractCDPValue(baseResult)
		if s, ok := baseVal.(string); ok {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				m.baselineText = &trimmed
			}
		}

		// Baseline process log keys
		logVal := extractCDPValue(logResult)
		if slice, ok := logVal.([]any); ok {
			for _, item := range slice {
				if s, ok := item.(string); ok {
					line := strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
					if line != "" {
						l := len(line)
						if l > 200 {
							l = 200
						}
						m.seenProcessLogKeys[line[:l]] = struct{}{}
					}
				}
			}
		} else if slice, ok := logVal.([]string); ok {
			for _, s := range slice {
				line := strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
				if line != "" {
					l := len(line)
					if l > 200 {
						l = 200
					}
					m.seenProcessLogKeys[line[:l]] = struct{}{}
				}
			}
		}

		// Structured baseline activity and thinking lines
		if structuredBaseline != nil {
			structVal := extractCDPValue(structuredBaseline)
			if structVal != nil {
				classified := ClassifyAssistantSegments(structVal)
				if classified.Diagnostics.Source == "dom-structured" {
					for _, line := range classified.ActivityLines {
						l := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
						if l != "" {
							if len(l) > 200 {
								l = l[:200]
							}
							m.seenProcessLogKeys[l] = struct{}{}
						}
					}
					for _, line := range classified.ThinkingLines {
						l := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
						if l != "" {
							if len(l) > 200 {
								l = l[:200]
							}
							m.seenThinkingLogKeys[l] = struct{}{}
						}
					}
				}
			}
		}
		m.mu.Unlock()
	}

	// Set timeout timer
	if maxDuration > 0 {
		m.mu.Lock()
		m.timeoutTimer = time.AfterFunc(maxDuration, func() {
			m.handleTimeout()
		})
		m.mu.Unlock()
	}

	// Inject the MutationObserver for real-time push notifications
	if cdp != nil {
		_, err := cdp.Call(ctx, "Runtime.evaluate", m.buildEvaluateParams(ResponseSelectors.DomObserver))
		if err != nil {
			logger.Warn("[ResponseMonitor] Failed to inject MutationObserver (falling back to pure polling):", err)
		} else {
			logger.Debug("[ResponseMonitor] MutationObserver injected successfully")
		}
	}

	m.mu.Lock()
	mode := "Monitoring"
	if passive {
		mode = "Passive monitoring"
	}
	baselineLen := 0
	if m.baselineText != nil {
		baselineLen = len(*m.baselineText)
	}
	logger.Debug(fmt.Sprintf("── %s started | poll=%v timeout=%v baseline=%dch",
		mode, m.pollInterval, m.maxDuration, baselineLen))

	m.schedulePollLocked()
	m.mu.Unlock()

	return nil
}

// Stop interrupts response monitoring and cancels all active timers.
func (m *ResponseMonitor) Stop() error {
	m.mu.Lock()
	m.isRunning = false
	if m.pollTimer != nil {
		m.pollTimer.Stop()
		m.pollTimer = nil
	}
	if m.timeoutTimer != nil {
		m.timeoutTimer.Stop()
		m.timeoutTimer = nil
	}
	if m.domChangedDebounceTimer != nil {
		m.domChangedDebounceTimer.Stop()
		m.domChangedDebounceTimer = nil
	}
	cdp := m.cdpService
	listener := m.bindingListener
	m.mu.Unlock()

	if cdp != nil && listener != nil {
		cdp.Off("Runtime.bindingCalled", listener)
	}
	return nil
}

// GetPhase returns the current phase of the response monitor.
func (m *ResponseMonitor) GetPhase() ResponsePhase {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentPhase
}

// GetQuotaDetected reports whether a quota exhaustion error was observed during monitoring.
func (m *ResponseMonitor) GetQuotaDetected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.quotaDetected
}

// IsActive reports whether monitoring is currently running.
func (m *ResponseMonitor) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}

// GetLastText returns the most recently extracted response text.
func (m *ResponseMonitor) GetLastText() *string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastText
}

// GetLastExtractionSource returns the most recent extraction source ("structured" or "legacy").
func (m *ResponseMonitor) GetLastExtractionSource() *ExtractionSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastExtractionSource
}

// ClickStopButton clicks the stop button in the DOM and stops the monitor if running.
func (m *ResponseMonitor) ClickStopButton(ctx context.Context) ClickStopButtonResult {
	if m.cdpService == nil {
		return ClickStopButtonResult{Ok: false, Error: "CDP service not configured"}
	}

	params := m.buildEvaluateParams(ResponseSelectors.ClickStopButton)
	result, err := m.cdpService.Call(ctx, "Runtime.evaluate", params)
	if err != nil {
		return ClickStopButtonResult{Ok: false, Error: err.Error()}
	}

	m.mu.Lock()
	running := m.isRunning
	m.mu.Unlock()
	if running {
		_ = m.Stop()
	}

	val := extractCDPValue(result)
	if mVal, ok := val.(map[string]any); ok {
		var res ClickStopButtonResult
		if okVal, ok := mVal["ok"].(bool); ok {
			res.Ok = okVal
		}
		if methodVal, ok := mVal["method"].(string); ok {
			res.Method = methodVal
		}
		if errVal, ok := mVal["error"].(string); ok {
			res.Error = errVal
		}
		return res
	}

	return ClickStopButtonResult{Ok: false, Error: "CDP evaluation returned empty"}
}

func (m *ResponseMonitor) handleTimeout() {
	m.mu.Lock()
	if !m.isRunning || m.currentPhase == PhaseComplete || m.currentPhase == PhaseQuotaReached {
		m.mu.Unlock()
		return
	}
	lastText := ""
	if m.lastText != nil {
		lastText = *m.lastText
	}
	onTimeout := m.onTimeout
	m.mu.Unlock()

	m.setPhase(PhaseTimeout, &lastText)
	_ = m.Stop()

	if onTimeout != nil {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[ResponseMonitor] timeout callback failed:", r)
			}
		}()
		onTimeout(lastText)
	}
}

func (m *ResponseMonitor) handleDomChangedEvent() {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}

	debounce := 500 * time.Millisecond
	if m.currentPhase == PhaseGenerating {
		debounce = 200 * time.Millisecond
	}

	if m.domChangedDebounceTimer != nil {
		m.domChangedDebounceTimer.Stop()
	}

	m.domChangedDebounceTimer = time.AfterFunc(debounce, func() {
		m.mu.Lock()
		if !m.isRunning {
			m.mu.Unlock()
			return
		}
		logger.Debug("[ResponseMonitor] Event-triggered poll")
		if m.pollTimer != nil {
			m.pollTimer.Stop()
			m.pollTimer = nil
		}
		m.mu.Unlock()

		m.poll(context.Background())

		m.mu.Lock()
		if m.isRunning {
			m.schedulePollLocked()
		}
		m.mu.Unlock()
	})
	m.mu.Unlock()
}

func (m *ResponseMonitor) setPhase(phase ResponsePhase, text *string) {
	m.mu.Lock()
	if m.currentPhase == phase {
		m.mu.Unlock()
		return
	}
	m.currentPhase = phase
	onPhaseChange := m.onPhaseChange
	m.mu.Unlock()

	length := 0
	if text != nil {
		length = len(*text)
	}
	switch phase {
	case PhaseThinking:
		logger.Phase("Thinking")
	case PhaseGenerating:
		logger.Phase(fmt.Sprintf("Generating (%d chars)", length))
	case PhaseComplete:
		logger.Done(fmt.Sprintf("Complete (%d chars)", length))
	case PhaseTimeout:
		logger.Warn(fmt.Sprintf("Timeout (%d chars captured)", length))
	case PhaseQuotaReached:
		logger.Warn("Quota Reached")
	default:
		logger.Phase(string(phase))
	}

	if onPhaseChange != nil {
		defer func() {
			_ = recover()
		}()
		onPhaseChange(phase, text)
	}
}

func (m *ResponseMonitor) schedulePollLocked() {
	if !m.isRunning {
		return
	}
	if m.pollTimer != nil {
		m.pollTimer.Stop()
	}
	m.pollTimer = time.AfterFunc(m.pollInterval, func() {
		m.poll(context.Background())
		m.mu.Lock()
		if m.isRunning {
			m.schedulePollLocked()
		}
		m.mu.Unlock()
	})
}

func (m *ResponseMonitor) buildEvaluateParams(expression string) map[string]any {
	params := map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if m.cdpService != nil {
		if ctxID := m.cdpService.GetPrimaryContextID(); ctxID != nil {
			params["contextId"] = *ctxID
		}
	}
	return params
}

func (m *ResponseMonitor) emitNewProcessLogs(entries []string) {
	for _, line := range entries {
		normalized := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
		if normalized == "" {
			continue
		}
		keyLen := len(normalized)
		if keyLen > 200 {
			keyLen = 200
		}
		key := normalized[:keyLen]

		m.mu.Lock()
		if _, seen := m.seenProcessLogKeys[key]; seen {
			m.mu.Unlock()
			continue
		}
		m.seenProcessLogKeys[key] = struct{}{}
		onProcessLog := m.onProcessLog
		m.mu.Unlock()

		if onProcessLog != nil {
			sliceLen := len(normalized)
			if sliceLen > 300 {
				sliceLen = 300
			}
			func() {
				defer func() { _ = recover() }()
				onProcessLog(normalized[:sliceLen])
			}()
		}
	}
}

func (m *ResponseMonitor) emitNewThinkingLogs(entries []string) {
	for _, line := range entries {
		normalized := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
		if normalized == "" {
			continue
		}
		keyLen := len(normalized)
		if keyLen > 200 {
			keyLen = 200
		}
		key := normalized[:keyLen]

		m.mu.Lock()
		if _, seen := m.seenThinkingLogKeys[key]; seen {
			m.mu.Unlock()
			continue
		}
		m.seenThinkingLogKeys[key] = struct{}{}
		// Also mark as seen in process logs to prevent cross-contamination
		m.seenProcessLogKeys[key] = struct{}{}
		onThinkingLog := m.onThinkingLog
		m.mu.Unlock()

		sliceLen := len(normalized)
		if sliceLen > 80 {
			sliceLen = 80
		}
		logger.Debug("[ResponseMonitor] Emitting thinking entry:", normalized[:sliceLen])

		if onThinkingLog != nil {
			maxLen := len(normalized)
			if maxLen > 2000 {
				maxLen = 2000
			}
			func() {
				defer func() { _ = recover() }()
				onThinkingLog(normalized[:maxLen])
			}()
		}
	}
}

func (m *ResponseMonitor) poll(ctx context.Context) {
	m.pollMu.Lock()
	defer m.pollMu.Unlock()

	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}
	cdp := m.cdpService
	extractionMode := m.extractionMode
	m.mu.Unlock()

	if cdp == nil {
		return
	}

	var isGenerating bool
	var quotaDetected bool
	var planningActive bool
	var currentText *string
	var structuredHandledLogs bool
	var lastExtractionSource ExtractionSource = SourceLegacy

	if extractionMode == ExtractionModeStructured {
		var combinedResult map[string]any
		var combinedErr error
		var structuredResult map[string]any

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			params := m.buildEvaluateParams(ResponseSelectors.CombinedPoll)
			combinedResult, combinedErr = cdp.Call(ctx, "Runtime.evaluate", params)
		}()

		go func() {
			defer wg.Done()
			params := m.buildEvaluateParams(ResponseSelectors.ResponseStructured)
			res, err := cdp.Call(ctx, "Runtime.evaluate", params)
			if err == nil {
				structuredResult = res
			}
		}()

		wg.Wait()

		if combinedErr != nil {
			if isWebSocketError(combinedErr) {
				m.handleWsError()
				return
			}
			logger.Error("[ResponseMonitor] poll error:", combinedErr)
			return
		}

		combinedVal := extractCDPValue(combinedResult)
		combined := parseCombinedPollPayload(combinedVal)
		isGenerating = combined.IsGenerating
		quotaDetected = combined.QuotaError
		planningActive = combined.PlanningActive

		// Try structured extraction first
		if structuredResult != nil {
			payload := extractCDPValue(structuredResult)
			if payload != nil {
				classified := ClassifyAssistantSegments(payload)
				if classified.Diagnostics.Source == "dom-structured" {
					trimmed := strings.TrimSpace(classified.FinalOutputText)
					if trimmed != "" {
						currentText = &trimmed
					}
					lastExtractionSource = SourceStructured
					structuredHandledLogs = true

					m.mu.Lock()
					if !m.structuredDiagLogged {
						m.structuredDiagLogged = true
						logger.Debug("[ResponseMonitor] Structured extraction OK — segments:", classified.Diagnostics.SegmentCounts)
					}
					m.mu.Unlock()

					if len(classified.ActivityLines) > 0 {
						m.emitNewProcessLogs(classified.ActivityLines)
					}
					if len(classified.ThinkingLines) > 0 {
						logger.Debug("[ResponseMonitor] Thinking lines found:", len(classified.ThinkingLines))
						m.emitNewThinkingLogs(classified.ThinkingLines)
					}
				} else {
					m.mu.Lock()
					if !m.structuredDiagLogged {
						m.structuredDiagLogged = true
						logger.Warn(
							"[ResponseMonitor:poll] Structured extraction failed — reason:",
							classified.Diagnostics.FallbackReason,
						)
					}
					m.mu.Unlock()
				}
			}
		}

		// Fallback to legacy text from combined result
		if currentText == nil {
			currentText = combined.ResponseText
			lastExtractionSource = SourceLegacy
		}

		// Process logs from combined result if structured didn't handle logs
		if !structuredHandledLogs && len(combined.ProcessLogs) > 0 {
			m.emitNewProcessLogs(combined.ProcessLogs)
		}
	} else {
		// Legacy mode: single combined CDP call
		params := m.buildEvaluateParams(ResponseSelectors.CombinedPoll)
		combinedResult, combinedErr := cdp.Call(ctx, "Runtime.evaluate", params)
		if combinedErr != nil {
			if isWebSocketError(combinedErr) {
				m.handleWsError()
				return
			}
			logger.Error("[ResponseMonitor] poll error:", combinedErr)
			return
		}

		combinedVal := extractCDPValue(combinedResult)
		combined := parseCombinedPollPayload(combinedVal)
		isGenerating = combined.IsGenerating
		quotaDetected = combined.QuotaError
		planningActive = combined.PlanningActive
		currentText = combined.ResponseText
		lastExtractionSource = SourceLegacy

		if len(combined.ProcessLogs) > 0 {
			m.emitNewProcessLogs(combined.ProcessLogs)
		}
	}

	// CDP calls succeeded — reset WebSocket error counter
	m.mu.Lock()
	m.consecutiveWsErrors = 0
	m.lastExtractionSource = &lastExtractionSource
	m.mu.Unlock()

	// Handle stop button appearing
	if isGenerating {
		m.mu.Lock()
		if !m.generationStarted {
			m.generationStarted = true
			m.mu.Unlock()
			m.setPhase(PhaseThinking, nil)
		} else {
			m.mu.Unlock()
		}

		m.mu.Lock()
		m.stopGoneCount = 0
		m.mu.Unlock()
	}

	// Handle quota detection
	if quotaDetected {
		m.mu.Lock()
		hasText := m.lastText != nil && strings.TrimSpace(*m.lastText) != ""
		logger.Warn(fmt.Sprintf("[ResponseMonitor] quota detected hasText=%v", hasText))
		if hasText {
			m.quotaDetected = true
			m.mu.Unlock()
		} else {
			onComplete := m.onComplete
			m.mu.Unlock()

			emptyText := ""
			m.setPhase(PhaseQuotaReached, &emptyText)
			_ = m.Stop()
			if onComplete != nil {
				func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error("[ResponseMonitor] complete callback failed:", r)
						}
					}()
					onComplete("", &CompleteMeta{Source: SourceLegacy})
				}()
			}
			return
		}
	}

	// Baseline suppression: do not emit progress for pre-existing text.
	// IMPORTANT: do not early-return here; completion logic must still run.
	m.mu.Lock()
	effectiveText := currentText
	if currentText != nil && m.baselineText != nil && *currentText == *m.baselineText && m.lastText == nil {
		effectiveText = nil
	}
	lastTextVal := m.lastText
	m.mu.Unlock()

	// Text change handling
	textChanged := effectiveText != nil && (lastTextVal == nil || *effectiveText != *lastTextVal)
	if textChanged {
		m.mu.Lock()
		m.lastText = effectiveText
		currPhase := m.currentPhase
		genStarted := m.generationStarted
		onProgress := m.onProgress
		m.mu.Unlock()

		if currPhase == PhaseWaiting || currPhase == PhaseThinking {
			m.setPhase(PhaseGenerating, effectiveText)
			if !genStarted {
				m.mu.Lock()
				m.generationStarted = true
				m.mu.Unlock()
			}
		}

		if onProgress != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("[ResponseMonitor] progress callback failed:", r)
					}
				}()
				onProgress(*effectiveText)
			}()
		}
	}

	// Completion: stop button gone N consecutive times
	m.mu.Lock()
	genStarted := m.generationStarted
	isRunning := m.isRunning
	m.mu.Unlock()

	if !isGenerating && genStarted {
		if planningActive {
			m.mu.Lock()
			m.stopGoneCount = 0
			m.mu.Unlock()
			logger.Info("[ResponseMonitor] Planning dialog active — deferring completion")
		} else {
			m.mu.Lock()
			m.stopGoneCount++
			count := m.stopGoneCount
			threshold := m.stopGoneConfirmCount
			m.mu.Unlock()

			if count >= threshold && isRunning {
				m.mu.Lock()
				finalText := ""
				if m.lastText != nil {
					finalText = *m.lastText
				}
				source := SourceLegacy
				if m.lastExtractionSource != nil {
					source = *m.lastExtractionSource
				}
				onComplete := m.onComplete
				m.mu.Unlock()

				m.setPhase(PhaseComplete, &finalText)
				_ = m.Stop()

				if onComplete != nil {
					func() {
						defer func() {
							if r := recover(); r != nil {
								logger.Error("[ResponseMonitor] complete callback failed:", r)
							}
						}()
						onComplete(finalText, &CompleteMeta{Source: source})
					}()
				}
				return
			}
		}
	}
}

func (m *ResponseMonitor) handleWsError() {
	m.mu.Lock()
	m.consecutiveWsErrors++
	count := m.consecutiveWsErrors
	lastText := ""
	if m.lastText != nil {
		lastText = *m.lastText
	}
	onTimeout := m.onTimeout
	m.mu.Unlock()

	if count >= 5 {
		logger.Error(fmt.Sprintf("[ResponseMonitor] %d consecutive WebSocket errors — stopping monitor", count))
		_ = m.Stop()
		if onTimeout != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("[ResponseMonitor] timeout callback failed:", r)
					}
				}()
				onTimeout(lastText)
			}()
		}
	}
}

func isWebSocketError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "WebSocket is not connected") || strings.Contains(msg, "WebSocket disconnected")
}

func extractCDPValue(res map[string]any) any {
	if res == nil {
		return nil
	}
	if r, ok := res["result"].(map[string]any); ok {
		if val, exists := r["value"]; exists {
			return val
		}
		return r
	}
	if val, exists := res["value"]; exists {
		return val
	}
	return res
}

func parseCombinedPollPayload(val any) CombinedPollPayload {
	var payload CombinedPollPayload
	m, ok := val.(map[string]any)
	if !ok {
		return payload
	}
	if b, ok := m["isGenerating"].(bool); ok {
		payload.IsGenerating = b
	}
	if b, ok := m["quotaError"].(bool); ok {
		payload.QuotaError = b
	}
	if b, ok := m["planningActive"].(bool); ok {
		payload.PlanningActive = b
	}
	if s, ok := m["responseText"].(string); ok {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			payload.ResponseText = &trimmed
		}
	}
	if logs, ok := m["processLogs"].([]any); ok {
		for _, item := range logs {
			if s, ok := item.(string); ok {
				payload.ProcessLogs = append(payload.ProcessLogs, s)
			}
		}
	} else if logs, ok := m["processLogs"].([]string); ok {
		payload.ProcessLogs = append(payload.ProcessLogs, logs...)
	}
	return payload
}
