# Design: CDP Chat Session Service

## Architecture & Data Flow

`ChatSessionService` provides session navigation and lifecycle management for the Cascade panel within the Antigravity Electron application via Chrome DevTools Protocol (CDP).

```
[ Telegram Commands: /new, /chat, /join ]
                   │
                   ▼
       [ *cdp.ChatSessionService ]
                   │ (Stateless, accepts SessionCDPClient per call)
                   ├── StartNewChat(client)
                   │    ├── Check button state ([data-tooltip-id="new-conversation-tooltip"])
                   │    ├── If disabled (cursor != 'pointer') -> return OK (already empty)
                   │    ├── If enabled -> Input.dispatchMouseEvent center (x, y)
                   │    └── Verify state transitions to disabled
                   │
                   ├── GetCurrentSessionInfo(client)
                   │    └── Query .antigravity-agent-side-panel header div[class*="text-ellipsis"]
                   │
                   ├── ListAllSessions(client)
                   │    ├── Click Past Conversations toggle button via CDP coordinates
                   │    ├── Scrape sessions above "Other Conversations" boundary
                   │    ├── If < 20 sessions, click "Show N more..." and re-scrape
                   │    ├── Send Escape key (keyDown/keyUp) to close modal
                   │    └── Return sessions[:20]
                   │
                   └── ActivateSessionByTitle(client, title, opts)
                        ├── Short-circuit if current session already matches title
                        ├── Retry loop: try direct side panel click -> fallback to Past Conversations modal
                        └── Verify that panel header reflects the target session title
```

## Key Components

### 1. `SessionCDPClient` Interface
Decouples `ChatSessionService` from concrete `*Client` or pool implementation, enabling simple unit testing with mocks and safe concurrent usage:
```go
type SessionCDPClient interface {
    Call(method string, params any) (json.RawMessage, error)
    GetContexts() []Context
    WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool
}
```

### 2. Multi-Context Evaluation (`evaluateOnAnyContext`)
Iterates over available CDP contexts (`cdpService.GetContexts()`), executing `Runtime.evaluate` with `returnByValue: true` and returning the first non-null/truthy value. Errors in non-target contexts are ignored, allowing seamless execution even when launchpad or auxiliary contexts are present.

### 3. Coordinate-Based Native Clicks (`cdpMouseClick`)
Because synthetic DOM `element.click()` can be ignored or suppressed by Electron webview controls, all primary actions (new chat, past conversations toggle, show more link) find element bounding boxes in the DOM and dispatch native mouse events via CDP:
1. `Input.dispatchMouseEvent` with `type: "mouseMoved"`
2. `Input.dispatchMouseEvent` with `type: "mousePressed"`, `button: "left"`, `clickCount: 1`
3. `Input.dispatchMouseEvent` with `type: "mouseReleased"`, `button: "left"`, `clickCount: 1`

### 4. DOM Boundary Filtering in Past Conversations Scrape
The Past Conversations panel lists conversations from both the current project and other workspaces. Sessions located below the `"Other Conversations"` header candidate are excluded using vertical bounding box comparison:
```javascript
if (row.getBoundingClientRect().top >= boundaryTop) continue;
```
Furthermore, timestamp labels (`"1 hr ago"`) and UI action texts (`"Show 5 more"`, `"close"`) are filtered out.

### 5. Dual-Path Activation & Verification
`ActivateSessionByTitle` first attempts an in-panel direct search using `BuildActivateChatByTitleScript`. If the conversation is not visible in the current view (or if direct selection resulted in an unexpected chat), it falls back to `BuildActivateViaPastConversationsScript`, which opens the modal, types into the search input, dispatches enter/click, and confirms the active header title via `GetCurrentSessionInfo`.
