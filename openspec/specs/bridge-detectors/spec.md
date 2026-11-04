# bridge-detectors Specification

## Purpose
TBD - created by archiving change spec-11-approval-and-planning-detectors. Update Purpose after archive.
## Requirements
### Requirement: Approval Dialog Detection and Key Deduplication
The `internal/bridge` package SHALL detect visible approval dialogs in the Antigravity UI using `DetectApprovalScript`, extract button labels and descriptions, and deduplicate notifications using key `${approveText}::${description}`.

#### Scenario: Approval required callback fired on detection
- **WHEN** visible allow and deny buttons are detected in the DOM
- **THEN** `OnApprovalRequired(info)` is invoked with extracted `ApprovalInfo` containing `approveText`, `denyText`, and `description`

#### Scenario: Deduplicate consecutive detections
- **WHEN** the same approval dialog remains visible across polling cycles
- **THEN** `OnApprovalRequired(info)` is invoked only once for identical `${approveText}::${description}` keys

#### Scenario: OnResolved callback fired when dialog disappears
- **WHEN** a previously detected approval dialog disappears from the DOM
- **THEN** `OnResolved()` is invoked if provided, and stored state is reset to nil

#### Scenario: Ignore disappearances without prior detection
- **WHEN** polling detects no approval buttons without prior detection
- **THEN** `OnResolved()` is NOT invoked

### Requirement: Remote Approval and Deny Actions
The `internal/bridge` package SHALL provide remote button click capabilities using `BuildClickScript`, supporting explicit text arguments, fallback to detected texts, and fallback to defaults.

#### Scenario: Remote approve button click with explicit text
- **WHEN** `ApproveButton(ctx, "Allow")` is called
- **THEN** it executes `BuildClickScript("Allow")` with the target `contextId` and returns true on success

#### Scenario: Remote approve button click using detected text
- **WHEN** `ApproveButton(ctx)` is called without arguments after detection
- **THEN** it clicks using `lastDetectedInfo.ApproveText`

#### Scenario: Remote deny button click with fallback
- **WHEN** `DenyButton(ctx)` is called without arguments after detection
- **THEN** it clicks using `lastDetectedInfo.DenyText` (or `"Deny"` default)

### Requirement: Always Allow and Dropdown Expansion
The `internal/bridge` package SHALL support approving for the entire conversation directly or via dropdown menu expansion.

#### Scenario: Direct click of visible Always Allow button
- **WHEN** `AlwaysAllowButton(ctx)` is called and an Always Allow button is visible
- **THEN** it clicks candidate labels (`"Allow This Conversation"`, `"Allow This Chat"`, `"Always Allow"`, etc.) directly

#### Scenario: Dropdown expansion when Always Allow is hidden
- **WHEN** direct candidate clicks fail
- **THEN** it executes `ExpandAlwaysAllowMenuScript` to expand the chevron/dropdown beside `Allow Once`, then retries candidate clicks up to 5 times spaced by 120ms

### Requirement: Planning Mode Dialog Detection and Cooldown
The `internal/bridge` package SHALL detect planning mode `.notify-user-container` elements containing both `Open` and `Proceed` buttons using `DetectPlanningScript`, extracting title, summary, and description with code block filtering, and enforcing a 5000ms cooldown.

#### Scenario: Planning required callback fired on detection
- **WHEN** both `Open` and `Proceed` buttons are visible in `.notify-user-container`
- **THEN** `OnPlanningRequired(info)` is invoked with title, summary, and code-filtered description

#### Scenario: Key deduplication by button text pair
- **WHEN** subsequent detections have identical `openText::proceedText`
- **THEN** `OnPlanningRequired` is not re-triggered and stored info is silently updated

#### Scenario: 5-second cooldown suppresses rapid re-detection
- **WHEN** planning buttons disappear and reappear within 5000ms of the previous notification
- **THEN** `OnPlanningRequired` is suppressed until the cooldown window expires

#### Scenario: OnResolved callback fired when planning buttons disappear
- **WHEN** planning buttons disappear from the DOM after being detected
- **THEN** `OnResolved()` is invoked, and `lastNotifiedAt` is preserved for cooldown tracking

### Requirement: Planning Remote Actions and Content Extraction
The `internal/bridge` package SHALL provide remote click helpers (`ClickOpenButton`, `ClickProceedButton`) and Markdown content extraction (`ExtractPlanContent`).

#### Scenario: Click Open and Proceed buttons
- **WHEN** `ClickOpenButton(ctx)` or `ClickProceedButton(ctx)` is invoked
- **THEN** it clicks the respective button using detected text or default fallbacks

#### Scenario: Extract plan content as Markdown
- **WHEN** `ExtractPlanContent(ctx)` is invoked
- **THEN** it executes `ExtractPlanContentScript`, converting DOM elements (`H1`–`H4`, `STRONG`, `EM`, `PRE/CODE`, `A`, `LI`, `P`) into Markdown and returns the string

#### Scenario: CDP error handling during content extraction
- **WHEN** `ExtractPlanContent(ctx)` encounters a CDP evaluation error
- **THEN** it logs the error and safely returns `(nil, nil)` without panicking

