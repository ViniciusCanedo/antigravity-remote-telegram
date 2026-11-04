## Why

The Antigravity Telegram Remote system requires structured DOM extraction from the Antigravity IDE webview interface to accurately distinguish assistant body content, model reasoning/thinking streams, tool calls and execution results, and user feedback buttons. Rather than relying solely on raw regex text heuristic parsing which can mix activity logs and thinking traces into the user-facing response, the structured DOM extractor extracts typed segments directly from the DOM tree. Additionally, the selector health auditing tool (`doctorSelector`) validates that all 7 core DOM selectors remain functional against active Antigravity tabs via Chrome DevTools Protocol (CDP), providing instant diagnostic feedback on UI changes.

This migration ports `src/services/assistantDomExtractor.ts` and `src/tools/doctorSelector.ts` from TypeScript to pure Golang with 100% fidelity, full concurrency safety, and zero external runtime dependencies.

## What Changes

- Implement `internal/monitor/selectors.go`:
  - `ResponseSelectorsCatalog`: Struct catalog of all 11 literal JS/IIFE scripts used by Antigravity response monitoring (`ResponseText`, `StopButton`, `PlanningActive`, `ClickStopButton`, `DumpAllTexts`, `ProcessLogs`, `CombinedPoll`, `QuotaError`, `ResponseStructured`, `DomDiagnostic`, `DomObserver`).
  - `ResponseSelectors`: Global singleton instance initialized with the exact extraction scripts.
- Implement `internal/monitor/dom_extractor.go`:
  - `AssistantSegmentKind`: Segment type enumeration (`assistant-body`, `thinking`, `thinking-content`, `tool-call`, `tool-result`, `feedback`).
  - Structs: `AssistantDomSegment`, `AssistantDomSegmentPayload`, `ClassifyDiagnostics`, `ClassifyResult`.
  - `ExtractAssistantSegmentsPayloadScript() string`: Returns the 4-pass browser-side extraction script evaluated via CDP `Runtime.evaluate`.
  - `ClassifyAssistantSegments(payload any) ClassifyResult`: Pure function converting typed payloads or raw CDP JSON/maps into Telegram HTML formatted output text, activity lines, thinking lines, feedback actions, and diagnostic counts.
  - `isValidPayload(value any) (*AssistantDomSegmentPayload, bool)`: Validates typed payloads, maps, and JSON message payloads.
- Implement `internal/cli/doctor_selectors.go`:
  - `SelectorCheckEntry`, `SelectorEvalResult`, `DoctorSelectorReport`.
  - `RunDoctorSelectors(ctx context.Context) (*DoctorSelectorReport, error)`: Connects to Antigravity via CDP and audits the 7 core selectors, outputting status icons and health score (`Score: X/7 Healthy`).
  - `RunDoctorSelectorsWithDeps`: Testable dependency-injected runner accepting custom CDP caller, logger, and output writer.
- Comprehensive unit tests matching 1:1 the original Jest test suites:
  - `internal/monitor/dom_extractor_test.go` matching `tests/services/assistantDomExtractor.test.ts`.
  - `internal/cli/doctor_selectors_test.go` covering all diagnostic branches, healthy, partial, broken, and error scenarios.

## Capabilities

### New Capabilities
- `dom-extractor`: Browser-side DOM extraction IIFE generation, Go-side segment classification with Telegram HTML formatting, and CDP selector health diagnostics.

### Modified Capabilities

## Impact

- Provides the core extraction engine required by `ResponseMonitor` (Spec 10) and the CLI diagnostic commands (Spec 17).
- 100% pure Go standard library implementation with zero CGO dependencies.
