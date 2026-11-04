# cdp-operations Specification

## Purpose
Provides DOM operations on Antigravity Cascade panel, including chat input injection, file attachment via CDP DOM domain, image extraction from assistant responses, UI mode and model synchronization, and full-screen / clipped screenshot capture via Page.captureScreenshot.
## Requirements
### Requirement: DOM Focus, Clear, and Text Injection
The `internal/cdp` package SHALL focus the Cascade panel editor, clear existing text using native OS modifier keys, inject user prompts via `Input.insertText`, and trigger submission via Enter key.

#### Scenario: Successful message injection in cascade-panel context
- **WHEN** `InjectMessage(text)` is invoked and multiple contexts exist
- **THEN** it prioritizes the `cascade-panel` context for editor focus, clears the input, inputs text via `Input.insertText`, sends Enter key events, and returns `InjectResult{OK: true, Method: "enter", ContextID: &cascadePanelId}`

#### Scenario: Fallback context when cascade-panel focus fails
- **WHEN** focusing in the `cascade-panel` context fails
- **THEN** it iterates to subsequent contexts until focus succeeds

#### Scenario: Error when all contexts fail
- **WHEN** all execution contexts fail to locate an editor
- **THEN** it returns `InjectResult{OK: false, Error: "Chat input field not found"}`

#### Scenario: Error when disconnected
- **WHEN** `InjectMessage` is invoked while disconnected
- **THEN** it returns an error stating `"Not connected to CDP. Call connect() first."`

### Requirement: Image File Attachment and Response Image Extraction
The `internal/cdp` package SHALL support uploading local image files into the Cascade chat via CDP DOM file input manipulation and extracting AI-generated images from assistant messages.

#### Scenario: Attaching images and sending message
- **WHEN** `InjectMessageWithImageFiles(text, files)` is called
- **THEN** it focuses the editor, locates or exposes `<input type="file">`, sets `data-agclaw-upload-token`, uses `DOM.getDocument` and `DOM.querySelector` to find node ID, calls `DOM.setFileInputFiles` with file paths, removes upload token, injects text, and sends Enter

#### Scenario: No duplicate input or change events dispatched
- **WHEN** `attachImageFiles` cleans up the upload token
- **THEN** the cleanup script does NOT contain `dispatchEvent` to avoid duplicate events after `DOM.setFileInputFiles`

#### Scenario: Extracting response images
- **WHEN** `ExtractLatestResponseImages(maxImages)` is called
- **THEN** it scans response message containers in Cascade panel, filters out UI icons/avatars, deduplicates images, and extracts up to `maxImages` (bounded between 1 and 8) with name, MIME type, and base64 or URL

### Requirement: UI Execution Mode Synchronization
The `internal/cdp` package SHALL query the current execution mode and switch between `fast` and `plan` modes in the Antigravity UI.

#### Scenario: Get current mode
- **WHEN** `GetCurrentMode()` is called
- **THEN** it evaluates the mode toggle button text and returns `"fast"`, `"plan"`, or `nil` if not found or disconnected

#### Scenario: Switch mode successfully
- **WHEN** `SetUIMode(modeName)` is called with `"fast"` or `"plan"`
- **THEN** it maps internal names to `"Fast"` / `"Planning"`, clicks the toggle button, selects the matching option in the dialog, and returns `UISyncResult{OK: true, Mode: targetUiName}`

#### Scenario: Graceful handling of CDP error during mode switch
- **WHEN** `SetUIMode` encounters a CDP communication error
- **THEN** it returns `UISyncResult{OK: false, Error: err}` without panicking

### Requirement: UI LLM Model Synchronization
The `internal/cdp` package SHALL query available models, read the active model, and select models in the Antigravity UI dropdown.

#### Scenario: Get UI models
- **WHEN** `GetUIModels()` is called while connected
- **THEN** it evaluates model dropdown items, trims `New` suffixes, and returns a deduplicated list of available models

#### Scenario: Get current model
- **WHEN** `GetCurrentModel()` is called
- **THEN** it finds the active model element styled with `bg-gray-500/20` and returns the model name or `nil`

#### Scenario: Set UI model
- **WHEN** `SetUIModel(modelName)` is called
- **THEN** it matches the target model in the model dropdown items, clicks to select, and returns `UISyncResult{OK: true, Model: modelName}`

### Requirement: Screenshot Capture Service
The `internal/cdp` package SHALL provide a `ScreenshotService` that utilizes CDP `Page.captureScreenshot` to capture the IDE UI.

#### Scenario: Capture PNG screenshot as buffer
- **WHEN** `Capture()` is called with default options
- **THEN** it issues `Page.captureScreenshot` with `format: "png"`, decodes base64 data, and returns `CaptureResult{Success: true, Buffer: data}`

#### Scenario: Capture JPEG with quality and clip region
- **WHEN** `Capture(options)` is called with format `"jpeg"`, `quality: 80`, and a `ClipRegion`
- **THEN** it passes `format`, `quality`, and `clip` parameters to CDP and decodes the result

#### Scenario: Direct Base64 string export
- **WHEN** `GetBase64()` is called
- **THEN** it returns the raw Base64 string from CDP or `nil` on failure

