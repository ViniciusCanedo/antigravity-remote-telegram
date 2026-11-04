# Design: CDP DOM Operations, Image Transfer, UI Sync, and Screenshot Service

## Architecture & Data Flow

This module extends `internal/cdp` with high-level browser automation operations targeting the Cascade panel of Antigravity:

```
[ Telegram Handlers / Dispatcher ]
                │
                ▼
        [ *cdp.Client ]
         ├── InjectMessage(text)
         ├── InjectMessageWithImageFiles(text, files)
         ├── ExtractLatestResponseImages(maxImages)
         ├── GetCurrentMode() / SetUIMode(mode)
         └── GetUIModels() / GetCurrentModel() / SetUIModel(model)
                │
                ├── Runtime.evaluate (DOM exploration, focus, tokens)
                ├── Input.insertText / Input.dispatchKeyEvent (typing & Enter)
                └── DOM.enable / DOM.getDocument / DOM.querySelector / DOM.setFileInputFiles
                │
                ▼
         [ *cdp.ScreenshotService ]
         ├── Capture(opts) -> CaptureResult (decodes Base64 to []byte)
         └── GetBase64(opts) -> *string
                │
                └── Page.captureScreenshot (format, quality, clip)
```

## Key Components

### 1. Operations on `*Client` (`internal/cdp/operations.go`)
- **Focus & Clear**:
  - `focusChatInput`: iterates through registered `contexts`. For each context, executes `focusScript` which searches for visible textboxes (`div[role="textbox"]:not(.xterm-helper-textarea)`), focusing the last visible one.
  - `clearInputField`: determines modifier based on OS (`4` for macOS darwin, `2` for others), sends 4 `Input.dispatchKeyEvent` calls (`KeyA` down/up, `Backspace` down/up), and sleeps 50ms.
  - `pressEnterToSend`: sends 2 `Input.dispatchKeyEvent` calls (`Enter` down/up).
- **Message Injection**:
  - `InjectMessage(text)`: validates connection (`isConnectedFlag && ws != nil`). Previews text up to 100 chars in debug log. Focuses input, clears field, inputs text via `Input.insertText`, pauses 200ms, and presses Enter.
  - `InjectMessageWithImageFiles(text, files)`: validates connection, focuses input, clears field, attaches image files via `attachImageFiles`, inputs text, pauses 200ms, and presses Enter.
- **Image Attachment**:
  - Enables DOM domain via `DOM.enable`.
  - Runs `locateInputScript` in the target context. This script searches for `<input type="file">` accepting images, optionally clicking upload trigger buttons (`attach`, `upload`, `image`, etc.) if not initially found. It tags the found input with a random token (`data-agclaw-upload-token`).
  - Calls `DOM.getDocument` to retrieve `root.nodeId`.
  - Calls `DOM.querySelector` with selector `input[data-agclaw-upload-token="..."]` to get the target node ID.
  - Calls `DOM.setFileInputFiles` with the files and node ID.
  - Evaluates `notifyScript` to clean up the token attribute, ensuring no duplicate events are triggered (as `DOM.setFileInputFiles` fires native `change` event in Chromium).
  - Pauses 250ms for DOM settling.
- **Response Image Extraction**:
  - `ExtractLatestResponseImages(maxImages)`: bounds `maxImages` between 1 and 8 (default 4). Uses `primaryContextId`. Queries candidate container selectors (`.antigravity-agent-side-panel`, markdown containers, assistant message containers). Discards UI icons, badges, avatars, and SVGs. Normalizes file names and detects MIME types. Returns `[]ExtractedResponseImage`. Never panics or returns error; returns empty slice on failure.
- **UI Mode & Model Synchronization**:
  - `GetCurrentMode`: evaluates toggle button text against `uiNameMap` (`{"fast":"Fast","plan":"Planning"}`). Returns `*string` or `nil`.
  - `SetUIMode`: validates connection. Clicks mode toggle button, waits 500ms, finds dialog option `.font-medium`, clicks parent option, waits 500ms, and checks new button text. Handles Escape on failure. Catches CDP errors gracefully into `UISyncResult{OK: false, Error: ...}`.
  - `GetUIModels`: extracts deduplicated list of available models from dropdown.
  - `GetCurrentModel`: extracts currently active model with `bg-gray-500/20`.
  - `SetUIModel`: verifies dropdown list is open, matches target model ignoring `/New$/` suffix, clicks option, waits 500ms, and verifies selection.

### 2. Screenshot Service (`internal/cdp/screenshot.go`)
- Decoupled from full `*Client` via `CDPCaller` interface:
  ```go
  type CDPCaller interface {
      Call(method string, params any) (json.RawMessage, error)
  }
  ```
- `CaptureOptions`: supports `Format` (`png`, `jpeg`, `webp`), `Quality` (0-100), `Clip` (`ClipRegion`), and `CaptureBeyondViewport`.
- `Capture`: invokes `Page.captureScreenshot`, checks for non-empty Base64 payload, decodes using `encoding/base64.StdEncoding`, and returns `CaptureResult{Success: true, Buffer: buffer}`.
- `GetBase64`: invokes `Page.captureScreenshot` and returns the raw Base64 string directly.

## Concurrency & Error Semantics

1. **Connection Pre-conditions**:
   - Write/mutating operations (`InjectMessage`, `InjectMessageWithImageFiles`, `SetUIMode`, `GetUIModels`, `SetUIModel`) return an error when disconnected (`"Not connected to CDP. Call connect() first."` or `"Not connected to CDP."`).
   - Read-only operations (`GetCurrentMode`, `GetCurrentModel`, `ExtractLatestResponseImages`) return safe defaults (`nil` or empty slice).
2. **CDP Evaluation Failures**:
   - `SetUIMode` and `SetUIModel` capture CDP evaluation errors and return `UISyncResult{OK: false, Error: err.Error()}` without panicking.
3. **Thread Safety**:
   - `Client.Call` is thread-safe, utilizing internal mutexes for WebSocket message transmission and ID correlation.
