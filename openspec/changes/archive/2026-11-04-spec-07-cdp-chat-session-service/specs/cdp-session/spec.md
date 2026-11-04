## ADDED Requirements

### Requirement: Start New Chat Session
The `internal/cdp` package SHALL support starting a new chat session by inspecting the "New Chat" button state and clicking its center coordinates via native CDP mouse events.

#### Scenario: Clicks when enabled and verifies disabled
- **WHEN** `StartNewChat` is invoked and the button has `cursor: pointer` (`enabled: true`)
- **THEN** it dispatches `Input.dispatchMouseEvent` (`mouseMoved`, `mousePressed`, `mouseReleased` with `left` button), waits for UI transition, verifies that `enabled` changes to `false`, and returns `SessionActionResult{OK: true}`

#### Scenario: No-op when button is already disabled
- **WHEN** `StartNewChat` is invoked and the button has `cursor: not-allowed` (`enabled: false`)
- **THEN** it immediately returns `SessionActionResult{OK: true}` without dispatching any mouse click events

#### Scenario: Fails when button not found after retries
- **WHEN** the button is not found across all contexts initially and after 5 retry attempts (1s interval)
- **THEN** it returns `SessionActionResult{OK: false, Error: "New chat button not found"}`

#### Scenario: Fails when no contexts available and panel ready times out
- **WHEN** contexts are empty and `WaitForCascadePanelReady` returns `false`
- **THEN** it returns `SessionActionResult{OK: false, Error: "No contexts available (timed out)"}`

#### Scenario: Fails when CDP call throws exception
- **WHEN** a CDP call returns an error during the operation
- **THEN** it catches the error and returns `SessionActionResult{OK: false, Error: message}`

#### Scenario: Fails when button state does not change after click
- **WHEN** the button remains `enabled: true` after dispatching click events
- **THEN** it returns `SessionActionResult{OK: false, Error: "Clicked new chat button but state did not change"}`

### Requirement: Query Current Session Info
The `internal/cdp` package SHALL retrieve the active chat title and determine active chat presence from the Cascade panel header.

#### Scenario: Retrieves chat title from Cascade panel header
- **WHEN** `GetCurrentSessionInfo` is called and the header contains a valid title
- **THEN** it returns `ChatSessionInfo` with the title and `HasActiveChat: true`

#### Scenario: Default title "Agent" marked as inactive
- **WHEN** the header title is `"Agent"`
- **THEN** it returns `ChatSessionInfo{Title: "Agent", HasActiveChat: false}`

#### Scenario: Fallback on CDP error or null result
- **WHEN** CDP call fails or returns `null`
- **THEN** it returns `ChatSessionInfo{Title: "(Failed to retrieve)", HasActiveChat: false}`

### Requirement: List All Sessions from Current Workspace
The `internal/cdp` package SHALL open the "Past Conversations" modal, scrape session items belonging strictly to the current workspace (excluding items below `"Other Conversations"`), click `"Show N more..."` if fewer than 20 sessions are found, close the modal using the `Escape` key, and return up to 20 items.

#### Scenario: Scrapes sessions via CDP mouse click
- **WHEN** `ListAllSessions` is called and Past Conversations toggle is located
- **THEN** it clicks coordinates via `Input.dispatchMouseEvent`, scrapes visible items, closes modal with Escape, and returns the session items

#### Scenario: Expands list via "Show more" button
- **WHEN** initial scrape yields fewer than 20 items and `"Show N more..."` link is present
- **THEN** it clicks the link, waits 500ms, scrapes again, and returns up to 20 sessions

#### Scenario: Returns empty slice when toggle button not found or CDP throws
- **WHEN** Past Conversations button is not found or a CDP call fails
- **THEN** it returns an empty slice `[]SessionListItem{}` without panicking

#### Scenario: Closes modal with Escape key
- **WHEN** scraping completes
- **THEN** it dispatches `Input.dispatchKeyEvent` with `key: "Escape"` (`keyDown` and `keyUp`)

### Requirement: Activate Session by Title
The `internal/cdp` package SHALL activate an existing session by title using direct panel matching or fallback to Past Conversations search modal, verifying that the header title reflects the requested session.

#### Scenario: Short-circuit when already active
- **WHEN** the current active title matches the requested title (case-insensitive)
- **THEN** it returns `SessionActionResult{OK: true}` immediately

#### Scenario: Direct panel match with title verification
- **WHEN** the title is matched in the side panel and verification succeeds
- **THEN** it returns `SessionActionResult{OK: true}`

#### Scenario: Fallback to Past Conversations flow
- **WHEN** direct side panel match fails but Past Conversations search succeeds
- **THEN** it returns `SessionActionResult{OK: true}`

#### Scenario: Retries until success within timeout
- **WHEN** initial attempts fail because UI is loading and subsequent attempt succeeds
- **THEN** it retries within `MaxWait` and returns `SessionActionResult{OK: true}`

#### Scenario: Empty title validation
- **WHEN** an empty or whitespace-only title is supplied
- **THEN** it returns `SessionActionResult{OK: false, Error: "Session title is empty"}`

#### Scenario: Title verification mismatch reporting
- **WHEN** click succeeds but after-state verification yields a different title
- **THEN** it returns `SessionActionResult{OK: false}` with an error describing the title mismatch
