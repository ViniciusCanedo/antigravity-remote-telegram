# Design: Approval and Planning Detectors (`internal/bridge`)

## Architecture & Data Flow

`ApprovalDetector` and `PlanningDetector` monitor the Antigravity window via Chrome DevTools Protocol (`Runtime.evaluate`), decoupled from specific CDP implementations via the `CdpEvaluator` interface:

```
                  [ Antigravity Electron Window / DOM ]
                                     ▲
                                     │ CDP Runtime.evaluate
                                     │
                             [ CdpEvaluator ]
                                     ▲
                     ┌───────────────┴───────────────┐
                     │                               │
           [ ApprovalDetector ]             [ PlanningDetector ]
         - DetectApprovalScript           - DetectPlanningScript
         - ExpandAlwaysAllowMenuScript    - ExtractPlanContentScript
         - BuildClickScript               - BuildClickScript
                     │                               │
                     ├── OnApprovalRequired(info)    ├── OnPlanningRequired(info)
                     └── OnResolved()                └── OnResolved()
                             │                               │
                             ▼                               ▼
                 [ Telegram Notification / Remote Action Flow ]
```

## Key Components

### 1. `CdpEvaluator` Interface & Protocol Data Structs
```go
type CdpEvaluator interface {
    Call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error)
    GetPrimaryContextId() *int64
}

type runtimeEvaluateResponse struct {
    Result struct {
        Value json.RawMessage `json:"value"`
    } `json:"result"`
}

type clickScriptResult struct {
    OK     bool   `json:"ok"`
    Reason string `json:"reason,omitempty"`
    Error  string `json:"error,omitempty"`
}
```

### 2. `ApprovalDetector` (`internal/bridge/approval_detector.go`)
- **Detection Cycle**:
  - `PollOnce(ctx)` executes `DetectApprovalScript` via `Runtime.evaluate` with `returnByValue: true` and `contextId` (if `GetPrimaryContextId() != nil`).
  - Evaluates visible button pairs (`Allow` / `Allow Once` + `Deny`, optional `Allow This Conversation`).
  - Fallback sequence for action descriptions: (1) `p, .description, [data-testid="description"]` inside dialog, (2) parent element text with `<button>` elements removed (between 5 and 500 chars), (3) `aria-label` attribute on approve button.
  - Deduplication key: `${approveText}::${description}`.
  - State reset and `onResolved`: When the dialog disappears after detection, resets `lastDetectedKey` and invokes `onResolved()`.
- **Remote Actions**:
  - `BuildClickScript(buttonText)`: Helper function serializing button text into JSON string literal and searching for button elements matching exact text, aria-label, or substring matches.
  - `ApproveButton(ctx, buttonText ...)`: Uses argument if provided; falls back to `lastDetectedInfo.ApproveText`; defaults to `"Allow"`.
  - `DenyButton(ctx, buttonText ...)`: Uses argument if provided; falls back to `lastDetectedInfo.DenyText`; defaults to `"Deny"`.
  - `AlwaysAllowButton(ctx)`:
    1. Direct attempt against `[AlwaysAllowText, "Allow This Conversation", "Allow This Chat", "この会話を許可", "Always Allow", "常に許可"]`.
    2. Dropdown expansion via `ExpandAlwaysAllowMenuScript` clicking toggle buttons with `aria-haspopup` or chevron/arrow attributes, or right edge mouse events.
    3. Retry polling up to 5 times spaced by 120ms (`AlwaysAllowExpandRetryDelay`).

### 3. `PlanningDetector` (`internal/bridge/planning_detector.go`)
- **Detection Cycle**:
  - `PollOnce(ctx)` executes `DetectPlanningScript` querying `.notify-user-container` for visible `Open` and `Proceed` buttons.
  - Extracts `planTitle` (`span.inline-flex.break-all`), `planSummary` (`span.text-sm` excluding button labels), and `description` (`.leading-relaxed.select-text`, recursively skipping `PRE`, `CODE`, `STYLE`, `SCRIPT` nodes, truncated to 500 characters).
  - Deduplication key: `${openText}::${proceedText}`.
  - Cooldown: Suppresses notifications if elapsed time since `lastNotifiedAt` is less than `PlanningCooldown = 5000ms`. When same key is detected, updates `lastDetectedInfo` silently.
  - State reset and `onResolved`: When buttons disappear after detection, clears `lastDetectedKey` and `lastDetectedInfo`, but preserves `lastNotifiedAt` for the 5s cooldown window. Invokes `onResolved()`.
- **Remote Actions & Content Extraction**:
  - `ClickOpenButton(ctx, buttonText ...)`: Clicks Open button with fallback to detected text or `"Open"`.
  - `ClickProceedButton(ctx, buttonText ...)`: Clicks Proceed button with fallback to detected text or `"Proceed"`.
  - `ExtractPlanContent(ctx)`: Executes `ExtractPlanContentScript`, converting DOM markdown blocks (`H1`–`H4`, `STRONG`, `EM`, `PRE/CODE`, `A`, `LI`, `P`) into Markdown string. Returns `(*string, error)`.

## Concurrency & Lifecycle

1. **Non-overlapping Polling Loop**:
   - Goroutine started on `Start()` with `timer := time.NewTimer(pollInterval)`.
   - Each poll runs to completion before timer is reset, preventing overlapping evaluations.
   - `Stop()` cancels context, waits on `pollDone` channel, ensuring clean shutdown.
2. **Locking & Callback Safety**:
   - Internal state protected by `sync.RWMutex`.
   - Callbacks (`onApprovalRequired`, `onPlanningRequired`, `onResolved`) are invoked outside lock acquisition to avoid deadlocks.
3. **CDP Error Tolerance**:
   - WebSocket disconnection errors (`"WebSocket is not connected"`, `"WebSocket disconnected"`) return silently without logging.
   - Other CDP errors are logged without terminating the monitoring goroutine.
