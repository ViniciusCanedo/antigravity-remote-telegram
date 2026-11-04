## 1. ChatSessionService Structs, Scripts, and Helpers (`internal/cdp/session.go`)

- [x] 1.1 Define interfaces and structs: `SessionCDPClient`, `SessionListItem`, `ChatSessionInfo`, `SessionActionResult`, `ActivateSessionOptions`, `newChatButtonState`, and `coordsState`
- [x] 1.2 Define constants: `ActivateSessionMaxWait`, `ActivateSessionRetryInterval`, and `ListSessionsTarget`
- [x] 1.3 Embed exact client-side JavaScript scripts: `GET_NEW_CHAT_BUTTON_SCRIPT`, `GET_CHAT_TITLE_SCRIPT`, `FIND_PAST_CONVERSATIONS_BUTTON_SCRIPT`, `SCRAPE_PAST_CONVERSATIONS_SCRIPT`, and `FIND_SHOW_MORE_BUTTON_SCRIPT`
- [x] 1.4 Implement script builders: `BuildActivateChatByTitleScript` and `BuildActivateViaPastConversationsScript`
- [x] 1.5 Implement private helpers: `evaluateOnAnyContext`, `cdpMouseClick`, `getNewChatButtonState`, `tryActivateByDirectSidePanel`, `tryActivateByPastConversations`, and `tryActivateWithScript`

## 2. Public Service Operations (`internal/cdp/session.go`)

- [x] 2.1 Implement `NewChatSessionService` constructor
- [x] 2.2 Implement `StartNewChat`: context ready check, button state inspection with retries, coordinate click, and verification
- [x] 2.3 Implement `GetCurrentSessionInfo`: extract title and active chat flag from header
- [x] 2.4 Implement `ListAllSessions`: open Past Conversations, scrape current workspace items, click Show more if < 20, close with Escape, return up to 20 items
- [x] 2.5 Implement `ActivateSessionByTitle`: title validation, current session short-circuit, retry loop with direct and past conversations fallback, and header title verification

## 3. Unit Test Suite (`internal/cdp/session_test.go`)

- [x] 3.1 Implement mock `SessionCDPClient`
- [x] 3.2 Implement `StartNewChat` test suite (6 tests matching lines 20–106 of original TS test)
- [x] 3.3 Implement `GetCurrentSessionInfo` test suite (4 tests matching lines 108–150 of original TS test)
- [x] 3.4 Implement `ActivateSessionByTitle` test suite (6 tests matching lines 152–279 of original TS test)
- [x] 3.5 Implement `ListAllSessions` test suite (5 tests matching lines 281–390 of original TS test)

## 4. Verification & Quality Gates

- [x] 4.1 Run `go test -v -race ./...` and ensure 100% pass
- [x] 4.2 Run `go vet ./...`
- [x] 4.3 Run `gofmt -w .` and verify `gofmt -l .` is clean
- [x] 4.4 Run `go build ./...`
