## ADDED Requirements

### Requirement: Structured DOM Extraction Script Generation
The `monitor` package SHALL provide `ExtractAssistantSegmentsPayloadScript() string` returning an IIFE JavaScript string evaluated via CDP `Runtime.evaluate` to extract assistant responses into typed segments.

#### Scenario: Script generation structure and selectors
- **WHEN** `ExtractAssistantSegmentsPayloadScript()` is called
- **THEN** it returns a non-empty JavaScript IIFE string containing `.antigravity-agent-side-panel`, content selectors (`data-message-role`, `rendered-markdown`), `<details>` extraction, broad activity scan, and feedback button extraction.

### Requirement: Segment Classification and Telegram HTML Formatting
The `monitor` package SHALL provide `ClassifyAssistantSegments(payload any) ClassifyResult` to categorize extracted segments into `FinalOutputText` (converted to Telegram HTML via `formatter.HtmlToTelegramHtml`), `ActivityLines`, `ThinkingLines`, `Feedback`, and `Diagnostics`.

#### Scenario: Separates body, activity, thinking, and feedback
- **WHEN** a valid payload with assistant body, thinking, tool-call, tool-result, and feedback segments is classified
- **THEN** body text is converted to Telegram HTML in `FinalOutputText`, tool-call and tool-result are routed to `ActivityLines`, thinking is routed to `ThinkingLines`, feedback buttons are routed to `Feedback`, `Diagnostics.Source` is `"dom-structured"`, and segment counts match the payload.

#### Scenario: Concatenates multiple body segments with newlines
- **WHEN** a payload has multiple `assistant-body` segments
- **THEN** the body segments are joined with `\n\n` prior to HTML formatting.

#### Scenario: Invalid payload fallback
- **WHEN** an invalid or nil payload is provided to `ClassifyAssistantSegments`
- **THEN** `FinalOutputText` is empty, slices are empty, `Diagnostics.Source` is `"legacy-fallback"`, and `Diagnostics.FallbackReason` is `"invalid-payload"`.

#### Scenario: Routes thinking and thinking-content to thinkingLines
- **WHEN** a payload contains both `thinking` and `thinking-content` segments
- **THEN** both segments are appended to `ThinkingLines` preserving order.

#### Scenario: Leaf activity and MCP output capture
- **WHEN** non-selector activity nodes or MCP outputs (`server / tool`) are captured by Pass 2.5
- **THEN** their normalized text is classified into `ActivityLines`.

#### Scenario: Skips dialog and oversized containers
- **WHEN** elements are inside `role="dialog"` or containers with more than 3 children
- **THEN** they are excluded from activity logs.

#### Scenario: Preserves lists and file references with title attributes
- **WHEN** body HTML contains `<ol>`, `<ul>`, `<p>`, and `<div title="path/file.ts">:54</div>`
- **THEN** `FinalOutputText` preserves ordered list numbering, bullet points, line breaks, and restores `path/file.ts:54`.

### Requirement: Selector Health Diagnostics (`doctorSelector`)
The `cli` package SHALL provide `RunDoctorSelectors(ctx context.Context) (*DoctorSelectorReport, error)` and `RunDoctorSelectorsWithDeps` to audit the 7 core DOM selectors against an active Antigravity session.

#### Scenario: All selectors healthy
- **WHEN** all 7 selectors return non-null, valid data via CDP
- **THEN** all are marked healthy, health score is `7/7`, and report message confirms all core selectors are healthy.

#### Scenario: Some selectors warning or broken
- **WHEN** one or more selectors return null or evaluation errors
- **THEN** warnings or broken errors are logged, and health count reflects only the healthy selectors.

#### Scenario: Connection error handling
- **WHEN** CDP connection fails or client reports not connected
- **THEN** error is logged and returned without panicking.
