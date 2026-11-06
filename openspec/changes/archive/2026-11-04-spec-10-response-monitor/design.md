# Design: Response Monitor (`ResponseMonitor`)

## Architecture & Data Flow

`ResponseMonitor` operates as a hybrid state machine driven by periodic CDP polling (default 2000ms) complemented by push notifications via an injected `MutationObserver` on the `.antigravity-agent-side-panel` DOM element.

```
                    +---------------------------+
                    |      Browser / IDE        |
                    | (antigravity-agent-panel) |
                    +---------------------------+
                              |         ^
              MutationObserver|         | Runtime.evaluate
              (bindingCalled) |         | (COMBINED_POLL / RESPONSE_STRUCTURED)
                              v         |
                    +---------------------------+
                    |      ResponseMonitor      |
                    |  - pollMu (poll lock)     |
                    |  - mu (state lock)        |
                    +---------------------------+
                      /       |         |      \
                     v        v         v       v
                onProgress onPhaseChange onComplete onTimeout
                onProcessLog onThinkingLog
```

### State Machine Lifecycle
- **Phases**: `waiting` -> `thinking` -> `generating` -> `complete` (or `timeout` / `quotaReached`).
- **Modes**:
  - `Start(ctx)`: Initializes with `generationStarted = false`, `currentPhase = PhaseWaiting`.
  - `StartPassive(ctx)`: Initializes with `generationStarted = true`, `currentPhase = PhaseGenerating`, immediately detecting streaming changes on existing sessions.

### Polling & CDP Dispatch Flow
1. **Extraction Mode**:
   - `structured`: Runs `COMBINED_POLL` and `RESPONSE_STRUCTURED` in parallel. Uses structured segment classification (`ClassifyAssistantSegments`) for body text, process logs (`tool-call`, `tool-result`), and thinking logs (`thinking`, `thinking-content`). Falls back to `COMBINED_POLL` text and process logs if structured extraction fails or yields no body.
   - `legacy`: Single CDP call to `COMBINED_POLL`, extracting plain text and process logs.
2. **Baseline Suppression**:
   - On startup, baseline text and process log keys are captured in parallel.
   - If initial polled text matches `baselineText` and `lastText == nil`, the text is suppressed (`effectiveText = nil`) to prevent re-sending previous turn outputs. Crucially, completion checks still run even when text is suppressed.
3. **Stop Gone Confirmation**:
   - When `isGenerating == false` and `generationStarted == true`:
     - If `planningActive == true`, resets `stopGoneCount = 0` (defers completion while interactive Open/Proceed dialog is displayed).
     - Otherwise, increments `stopGoneCount`.
     - When `stopGoneCount >= stopGoneConfirmCount`, transitions to `PhaseComplete`, calls `Stop()`, and triggers `onComplete(finalText, meta)`.
4. **Quota Detection**:
   - If `quotaError == true`:
     - If partial text already exists (`lastText != ""`), sets `quotaDetected = true` allowing completion to deliver the partial text.
     - If no text exists, immediately transitions to `PhaseQuotaReached`, stops, and emits `onComplete("", {Source: SourceLegacy})`.
5. **WebSocket Error Backoff / Threshold**:
   - Up to 5 consecutive `"WebSocket is not connected"` or `"WebSocket disconnected"` errors are tolerated before terminating with `onTimeout(lastText)`.
   - Any successful CDP evaluation resets `consecutiveWsErrors = 0`.

### Concurrency & Thread-Safety Model
- `pollMu sync.Mutex`: Serializes poll execution so concurrent timer ticks and DOM change events cannot execute overlapping CDP evaluations.
- `mu sync.Mutex`: Protects state variables (`isRunning`, `currentPhase`, `lastText`, `baselineText`, `stopGoneCount`, `timers`, `consecutiveWsErrors`).
- Callbacks (`onProgress`, `onComplete`, `onTimeout`, etc.) are invoked safely outside locks or defensively to avoid deadlock when callbacks invoke `Stop()` or query monitor state.

### Interfaces
```go
type CdpClient interface {
	Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)
	GetPrimaryContextID() *int
	On(event string, listener func(params map[string]any))
	Off(event string, listener func(params map[string]any))
}
```
This minimal interface allows straightforward unit test mocking while remaining compatible with `cdp.Client`.
