# response-monitor Specification

## Purpose
TBD - created by archiving change spec-10-response-monitor. Update Purpose after archive.
## Requirements
### Requirement: ResponseMonitor State Lifecycle and Initialization
The `monitor` package SHALL provide `NewResponseMonitor(options ResponseMonitorOptions) *ResponseMonitor`, `Start(ctx context.Context) error`, `StartPassive(ctx context.Context) error`, and `Stop() error`.

#### Scenario: Active mode start captures baseline and sets waiting phase
- **WHEN** `Start` is called on a new `ResponseMonitor`
- **THEN** it sets `isRunning = true`, sets phase to `"waiting"`, captures baseline text and process logs, injects `DOM_OBSERVER`, and begins periodic polling.

#### Scenario: Passive mode start sets generating phase immediately
- **WHEN** `StartPassive` is called on an active generation session
- **THEN** it sets `isRunning = true`, `generationStarted = true`, sets phase to `"generating"`, captures baseline, and detects streaming text updates on the first poll without waiting for the stop button.

#### Scenario: Stop stops timers and unregisters binding listener
- **WHEN** `Stop` is called on an active monitor
- **THEN** `isRunning` becomes `false`, all active timers (`pollTimer`, `timeoutTimer`, `domChangedDebounceTimer`) are cancelled, and the `Runtime.bindingCalled` event listener is removed from `cdpService`.

### Requirement: CDP Evaluation and Stop Button Confirmation
The `ResponseMonitor` SHALL evaluate `COMBINED_POLL` (and `RESPONSE_STRUCTURED` in structured mode) and confirm generation completion when the stop button is missing for `stopGoneConfirmCount` consecutive polls.

#### Scenario: Stop button presence initiates thinking phase
- **WHEN** `COMBINED_POLL` returns `isGenerating = true` and no text has been extracted yet
- **THEN** `generationStarted` is set to `true`, phase transitions to `"thinking"`, and `stopGoneCount` resets to 0.

#### Scenario: Stop button disappearance confirms completion after threshold
- **WHEN** generation has started and `isGenerating = false` for `stopGoneConfirmCount` consecutive polls without active planning dialog
- **THEN** phase transitions to `"complete"`, `Stop()` is called, and `onComplete(finalText, meta)` is invoked with the final accumulated text.

#### Scenario: Stop button reappearing resets gone count
- **WHEN** `isGenerating = false` occurs fewer than `stopGoneConfirmCount` times and then `isGenerating = true` occurs
- **THEN** `stopGoneCount` is reset to 0 and `onComplete` is not triggered.

#### Scenario: Continuous streaming text does not reset stop gone count
- **WHEN** `isGenerating = false` and new text updates continue arriving in each poll
- **THEN** `stopGoneCount` continues incrementing, triggering `onComplete` on the Nth poll with the newest accumulated text.

### Requirement: Baseline Text Suppression
The `ResponseMonitor` SHALL suppress baseline text extracted prior to the current generation turn.

#### Scenario: Pre-existing text does not trigger progress
- **WHEN** `currentText` matches `baselineText` and `lastText == nil`
- **THEN** `onProgress` is not invoked.

#### Scenario: Baseline suppression does not block completion
- **WHEN** generation starts with text identical to baseline and finishes without new text
- **THEN** the monitor completes with an empty string `""` once the stop button disappears.

### Requirement: Quota Exhaustion Detection
The `ResponseMonitor` SHALL detect quota limit errors reported in the DOM.

#### Scenario: Quota error without existing text completes immediately
- **WHEN** `COMBINED_POLL` reports `quotaError = true` and no response text has been emitted
- **THEN** phase transitions to `"quotaReached"`, the monitor stops, and `onComplete("", {Source: SourceLegacy})` is triggered immediately.

#### Scenario: Quota error with existing text flags quotaDetected
- **WHEN** `COMBINED_POLL` reports `quotaError = true` and partial response text exists
- **THEN** `GetQuotaDetected()` returns `true`, and generation continues until normal completion delivers the partial text.

### Requirement: Interactive Planning Dialog Deferral
The `ResponseMonitor` SHALL detect when an interactive planning dialog with `Open` and `Proceed` buttons is active.

#### Scenario: Planning dialog defers completion
- **WHEN** `isGenerating = false` and `planningActive = true`
- **THEN** `stopGoneCount` is reset to 0, preventing completion until the user resolves the dialog.

### Requirement: Log Deduplication and Routing
The `ResponseMonitor` SHALL emit deduplicated process logs and thinking logs.

#### Scenario: Deduplicates repeated process logs
- **WHEN** repeated process log entries are retrieved across polls
- **THEN** `onProcessLog` is called only for newly encountered normalized lines.

#### Scenario: Routes thinking lines without polluting process logs
- **WHEN** thinking lines are classified from structured DOM extraction
- **THEN** `onThinkingLog` is called with reasoning content, and keys are registered in `seenProcessLogKeys` to prevent double-emission.

### Requirement: Stop Button Click Action
The `ResponseMonitor` SHALL provide `ClickStopButton(ctx context.Context) ClickStopButtonResult` to click the stop button via CDP.

#### Scenario: Successfully clicks stop button
- **WHEN** `ClickStopButton` is called and the button is present
- **THEN** the click expression is evaluated via CDP, `Stop()` is called if running, and `{Ok: true, Method: "tooltip-id"}` or `{Ok: true, Method: "text-fallback"}` is returned.

