## Why

The Antigravity Telegram Remote system requires DOM manipulation and UI automation over Chrome DevTools Protocol (CDP) on the Antigravity Cascade panel. These operations include:
- Chat editor input focus, clear, text injection, and submission via Enter key.
- Local image file attachment via temporary upload token (`data-agclaw-upload-token`) and `DOM.setFileInputFiles`.
- AI response image extraction (`extractLatestResponseImages`) supporting data URLs and web fetches.
- UI synchronization for execution modes (`fast` / `plan`) and LLM models (e.g., `gpt-4o`, `claude-3-opus`, `gemini-1.5-pro`).
- Screen capture service (`ScreenshotService`) with `Page.captureScreenshot`, JPEG quality, clipping regions, and raw Base64 export.

Migrating these capabilities from TypeScript (`src/services/cdpService.ts` lines 945–1712 and `src/services/screenshotService.ts`) to Go (`internal/cdp/operations.go` and `internal/cdp/screenshot.go`) completes the interaction layer of the CDP package, enabling reliable message and image injection and UI state synchronization with 100% fidelity to the original implementation.

## What Changes

- Create `internal/cdp/operations.go`:
  - Selector definitions: `Selectors.ChatInput`, `SubmitButtonContainer`, `SubmitButtonSVGClasses`, `ContextURLKeyword`.
  - DOM operations on `*Client`:
    - `focusChatInput()`: searches contexts for visible `div[role="textbox"]:not(.xterm-helper-textarea)` and calls `editor.focus()`.
    - `clearInputField()`: dispatches exact 4 `Input.dispatchKeyEvent` events (`Meta+A`/`Ctrl+A` select all, `Backspace` delete).
    - `pressEnterToSend()`: dispatches exact 2 `Input.dispatchKeyEvent` events (`Enter` keyDown/keyUp).
    - `attachImageFiles()`: enables DOM, locates file input or clicks attachment triggers, sets `data-agclaw-upload-token`, queries node ID, calls `DOM.setFileInputFiles`, removes token without duplicate events.
    - `InjectMessage(text)`: focuses editor, clears field, calls `Input.insertText`, pauses 200ms, and presses Enter.
    - `InjectMessageWithImageFiles(text, imageFilePaths)`: focuses editor, clears field, attaches image files, calls `Input.insertText`, and presses Enter.
    - `ExtractLatestResponseImages(maxImages)`: extracts generated images from the Cascade panel, filters out UI icons/avatars, returns `ExtractedResponseImage` list.
    - `GetCurrentMode()`: evaluates mode button text and chevron, mapping UI text to internal mode (`fast`, `plan`).
    - `SetUIMode(modeName)`: clicks mode dropdown button, selects target mode option in dialog, verifies selection, handles errors gracefully.
    - `GetUIModels()`: extracts available model items from dropdown.
    - `GetCurrentModel()`: extracts currently selected model item.
    - `SetUIModel(modelName)`: selects target model by matching text (handling `New` suffix and `bg-gray-500/20` selection), verifies selection.
- Create `internal/cdp/screenshot.go`:
  - `CDPCaller` interface with `Call(method string, params any) (json.RawMessage, error)`.
  - `ScreenshotService` with `Capture(opts ...CaptureOptions) CaptureResult` and `GetBase64(opts ...CaptureOptions) *string`.
  - Support for `png`, `jpeg` (with quality 0-100), `webp`, and `clip` rectangular regions.
- Unit test suites matching original Jest specs:
  - `internal/cdp/operations_injection_test.go` (mirrors `tests/services/cdpService.injection.test.ts`)
  - `internal/cdp/operations_images_test.go` (mirrors `tests/services/cdpService.imageTransfer.test.ts`)
  - `internal/cdp/operations_uisync_test.go` (mirrors `tests/services/cdpService.uiSync.test.ts`)
  - `internal/cdp/screenshot_test.go` (mirrors `tests/services/screenshotService.test.ts`)

## Capabilities

### New Capabilities
- `cdp-operations`: DOM operations on Antigravity Cascade panel, including chat input injection, file attachment via CDP DOM domain, image extraction from assistant responses, UI mode and model synchronization, and full-screen / clipped screenshot capture via `Page.captureScreenshot`.

### Modified Capabilities

## Impact

- Provides prompt injection and image upload capabilities consumed by dispatcher and bot handlers.
- Provides UI state synchronization for `/mode` and `/model` commands.
- Provides screenshot service for `/screenshot` command.
- Zero external dependencies beyond existing Go standard library and `github.com/coder/websocket`.
