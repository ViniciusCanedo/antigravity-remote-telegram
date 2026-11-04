## Why

The Antigravity Telegram Remote bot requires full session control over the Cascade panel of the Antigravity IDE via Chrome DevTools Protocol (CDP). This includes:
- Starting new conversations (`startNewChat`) with reliable coordinate-based mouse clicks and verifying that button state transitions to disabled (`not-allowed`).
- Reading the active conversation title and active state (`getCurrentSessionInfo`) from the panel header.
- Scraping up to 20 past conversation sessions (`listAllSessions`) from the current workspace by opening the "Past Conversations" modal, excluding sessions from other workspaces (below the `"Other Conversations"` boundary), expanding the list via `"Show N more..."` when needed, and closing the modal via the Escape key.
- Activating conversations by title (`activateSessionByTitle`) using both direct side-panel element matching and fallback modal search in "Past Conversations".

Migrating `ChatSessionService` from TypeScript (`src/services/chatSessionService.ts`) to Go (`internal/cdp/session.go`) provides an independent, pool-compatible, and thread-safe service consumed by `/new`, `/chat`, and `/join` commands with 100% fidelity to the original implementation.

## What Changes

- Create `internal/cdp/session.go`:
  - `SessionCDPClient` interface mirroring methods of `*Client` needed by session management (`Call`, `GetContexts`, `WaitForCascadePanelReady`).
  - Structs: `SessionListItem`, `ChatSessionInfo`, `SessionActionResult`, `ActivateSessionOptions`.
  - Constants: `ActivateSessionMaxWait`, `ActivateSessionRetryInterval`, `ListSessionsTarget`.
  - DOM/JS Scripts (100% faithful to TypeScript):
    - `GET_NEW_CHAT_BUTTON_SCRIPT`: query selector `[data-tooltip-id="new-conversation-tooltip"]`, check `cursor === 'pointer'`, compute center coordinates.
    - `GET_CHAT_TITLE_SCRIPT`: query `.antigravity-agent-side-panel`, header `div[class*="border-b"]`, title `div[class*="text-ellipsis"]`.
    - `FIND_PAST_CONVERSATIONS_BUTTON_SCRIPT`: strategies with `[data-past-conversations-toggle]`, `[data-tooltip-id*="history"]`, SVG `lucide-history`.
    - `SCRAPE_PAST_CONVERSATIONS_SCRIPT`: scrape `div[class*="cursor-pointer"]`, exclude items below `"Other Conversations"` boundary, filter timestamps and action labels, detect active session via `focusBackground`.
    - `FIND_SHOW_MORE_BUTTON_SCRIPT`: locate `"Show N more..."` link coordinates.
    - `BuildActivateChatByTitleScript(title)`: dynamic script searching panel for matching title with exact and substring matching.
    - `BuildActivateViaPastConversationsScript(title)`: modal opening, search input typing, and item selection.
  - `ChatSessionService` struct with `SleepFunc` injection hook and methods:
    - `NewChatSessionService()`
    - `ListAllSessions(cdpService SessionCDPClient) []SessionListItem`
    - `StartNewChat(cdpService SessionCDPClient) SessionActionResult`
    - `GetCurrentSessionInfo(cdpService SessionCDPClient) ChatSessionInfo`
    - `ActivateSessionByTitle(cdpService SessionCDPClient, title string, opts ...ActivateSessionOptions) SessionActionResult`
- Create `internal/cdp/session_test.go`:
  - 21 unit test cases mirroring `tests/services/chatSessionService.test.ts` 1:1.
  - Fast execution using mocked `SessionCDPClient` and customized `SleepFunc`.

## Capabilities

### New Capabilities
- `cdp-session`: Session management via CDP for Antigravity Cascade panel, including starting new chat, querying session header info, listing recent workspace sessions from Past Conversations, and activating sessions by title.

### Modified Capabilities

## Impact

- Provides session management operations consumed by Telegram bot command handlers (`/new`, `/chat`, `/join`) and topic router.
- Zero new external dependencies beyond Go standard library and existing project modules.
