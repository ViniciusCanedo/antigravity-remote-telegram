## Why

The Antigravity Telegram Remote system requires continuous, non-invasive monitoring of AI response generation within the Antigravity IDE webview panel via Chrome DevTools Protocol (CDP). It must reliably detect when generation starts (stop button appearing), extract streaming text updates with baseline suppression, stream thinking and process activity logs without cross-contamination, detect quota exhaustion errors, defer completion while interactive planning dialogs (`Open` / `Proceed`) are active, and detect generation completion when the stop button remains absent across consecutive confirmation intervals (`stopGoneConfirmCount`).

This change implements `ResponseMonitor` in pure Go, porting `src/services/responseMonitor.ts` with 100% architectural and behavioral fidelity, full thread-safety, push-driven mutation observation, and comprehensive unit tests matching the original Jest test suites.

## What Changes

- Implement `internal/monitor/response_monitor.go`:
  - `ResponsePhase` constants: `PhaseWaiting`, `PhaseThinking`, `PhaseGenerating`, `PhaseComplete`, `PhaseTimeout`, `PhaseQuotaReached`.
  - `ExtractionMode` constants: `ExtractionModeStructured`, `ExtractionModeLegacy`.
  - `ExtractionSource` constants: `SourceStructured`, `SourceLegacy`.
  - Structs: `CompleteMeta`, `ClickStopButtonResult`, `CombinedPollPayload`, `ResponseMonitorOptions`.
  - `CdpClient` interface: `Call(ctx, method, params)`, `GetPrimaryContextID()`, `On(event, listener)`, `Off(event, listener)`.
  - `ResponseMonitor`: Thread-safe state machine managing polling intervals (`2000ms`), global timeout (`300000ms`), DOM mutation debouncing (`200ms` / `500ms`), stop button confirmation, baseline text suppression, and process/thinking log deduplication.
  - Public methods: `NewResponseMonitor`, `Start`, `StartPassive`, `Stop`, `GetPhase`, `GetQuotaDetected`, `IsActive`, `GetLastText`, `GetLastExtractionSource`, `ClickStopButton`.
- Implement unit tests covering 100% of the specification:
  - `internal/monitor/response_monitor_lean_test.go`: Lean API lifecycle, poll interval, baseline suppression, stop gone confirmations, quota detection, timeout, push binding.
  - `internal/monitor/response_monitor_selectors_test.go`: 29 structural assertions on `ResponseSelectors` scripts.
  - `internal/monitor/response_monitor_stop_button_test.go`: Stop button detection robustness across fixed-position elements, aria-label, tooltip-id, and non-stop button filtering.
  - `internal/monitor/response_monitor_test.go`: Incremental progress deduplication, stop/cleanup, WebSocket error handling, phase transitions, structured body/thinking/process log routing, planning dialog deferral, context ID forwarding, and passive mode.

## Capabilities

### New Capabilities
- `response-monitor`: CDP-based response monitoring state machine supporting active and passive modes, structured and legacy extraction, push and polling updates, stop button detection, quota detection, and log deduplication.

### Modified Capabilities

## Impact

- Enables real-time streaming of LLM responses to Telegram bots.
- Delivers robust error handling and quota limit handling with partial text recovery.
- Zero CGO dependencies and 100% concurrency safety with `sync.Mutex`.
