## Why

The Antigravity Telegram Remote system requires real-time detection and remote interaction with user approval dialogs and planning mode cards in the Antigravity Cascade panel. Migrating `ApprovalDetector`, `PlanningDetector`, and `BuildClickScript` from TypeScript (`src/services/approvalDetector.ts`, `src/services/planningDetector.ts`) to Go (`internal/bridge`) provides thread-safe, non-blocking background monitoring over Chrome DevTools Protocol (`Runtime.evaluate`) with 100% fidelity to the original implementation.

## What Changes

- Create package `bridge` at `internal/bridge/`:
  - `CdpEvaluator` interface: Deserializes and isolates CDP evaluation calls (`Call(ctx, method, params)` and `GetPrimaryContextId()`).
  - `ApprovalInfo`, `ApprovalDetectorOptions`, and `ApprovalDetector`:
    - JavaScript detection scripts: `DetectApprovalScript`, `ExpandAlwaysAllowMenuScript`.
    - `BuildClickScript(buttonText string) string`: Generates DOM click script with JSON string escaping.
    - `NewApprovalDetector(options)`: Configures detector with defaults (1500ms interval, slog logger, default sleep).
    - Lifecycle management: `Start()`, `Stop()`, `IsActive()`, and `GetLastDetectedInfo()`.
    - Polling and remote actions: `PollOnce(ctx)`, `ApproveButton(ctx, ...)` with fallback to detected text, `AlwaysAllowButton(ctx)` with direct candidate matching, dropdown expansion, and 5-retry polling, `DenyButton(ctx, ...)` with fallback to detected text.
    - Error handling: Graceful handling of WebSocket disconnection without error logs, logging of other CDP errors, thread-safe state access with `sync.RWMutex`, and invoking callbacks outside the mutex lock.
  - `PlanningInfo`, `PlanningDetectorOptions`, and `PlanningDetector`:
    - JavaScript detection scripts: `DetectPlanningScript` (filtering `PRE`, `CODE`, `STYLE`, `SCRIPT` tags and truncating description to 500 characters), `ExtractPlanContentScript` (DOM HTML-to-Markdown converter).
    - `NewPlanningDetector(options)`: Configures detector with defaults (2000ms interval, slog logger, time.Now clock).
    - Lifecycle management: `Start()`, `Stop()`, `IsActive()`, and `GetLastDetectedInfo()`.
    - Polling and remote actions: `PollOnce(ctx)` with 5s cooldown (`PlanningCooldown = 5 * time.Second`) and `openText::proceedText` key dedup, `ClickOpenButton(ctx, ...)`, `ClickProceedButton(ctx, ...)`, and `ExtractPlanContent(ctx)`.
- Unit test suites matching original Jest specs:
  - `internal/bridge/approval_detector_test.go`: 16 test cases mirroring `tests/services/approvalDetector.test.ts`.
  - `internal/bridge/planning_detector_test.go`: 22 test cases mirroring `tests/services/planningDetector.test.ts`.

## Capabilities

### New Capabilities
- `bridge-detectors`: Background DOM detection and remote execution for Antigravity approval dialogs (`ApprovalDetector`) and planning mode notifications (`PlanningDetector`), including DOM click script generation (`BuildClickScript`) and plan markdown extraction (`ExtractPlanContent`).

### Modified Capabilities

## Impact

- Supplies core background detector services required by the bridge manager (`CdpBridgeManager`) and Telegram bot notification flows.
- Thread-safe, non-overlapping polling routines implemented via goroutines, timers, and context cancellation.
- Zero external dependencies beyond the Go standard library (`context`, `encoding/json`, `fmt`, `log/slog`, `strings`, `sync`, `time`).
