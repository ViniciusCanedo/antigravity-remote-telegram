## Why

Porting the Telegram bot response streaming pipeline, message handlers (text, photo, voice), callback query router, and passive PC-to-Telegram mirroring from TypeScript to Go completes the interactive communication loop of `antigravity-telegram-remote`. This allows Telegram users to receive real-time streaming progress, collapsable thinking logs (`<blockquote expandable>`), chunked responses, generated images, and interactive inline buttons for approvals, plans, models, and sessions.

## What Changes

- Implement `internal/bot/streaming.go` providing:
  - `SerialTaskQueue` for FIFO serialized message dispatching per category (`general`, `response`, `activity`).
  - `StripHTMLForFile`, `BalanceHTMLTags`, `StripHTML`, `IsJunkEntry`, and `FormatActivityLine`.
  - `SendPromptToAntigravity` pipeline coordinating prompt injection, `ResponseMonitor`, 5000ms progress throttle, three-tier HTML fallback, `<blockquote expandable>` thinking blocks, chunked responses (`<=4096`, 2-5 chunks, `>5` chunks with `.md` attachment), generated images extraction, topic renaming, and quota limit error fallback with model selection UI.
  - `StartPassiveMirrorForUserMessage` to mirror PC activity and initiate passive response monitoring (`startPassive`).
- Implement `internal/bot/callbacks.go` providing:
  - `CallbackContext` interface abstracting Telegram callback query events and reply methods.
  - `HandleCallbackQuery` routing all 11 inline button families: `mode_select:`, `model_exhausted_`, `model_btn_`, `model_refresh_btn`, `autoaccept_btn_`, `project_select:`, `project_page:`, `template_btn_`, `session_select:`, approval actions (`approve_action`, `always_allow_action`, `deny_action`), planning actions (`open`, `proceed`, `view`, `edit`, `refresh`, `page`), error popup actions (`dismiss`, `copy_debug`, `retry`), and cleanup actions (`cleanup_archive:`, `cleanup_delete:`, `cleanup_cancel`).
- Implement `internal/bot/handlers_messages.go` providing:
  - `HandleTextMessage`: handles plan edit feedback interception (`planEditPendingChannels`), slash command text matching, echo hash recording on `UserMessageDetector`, session auto-start/activation, and prompt dispatch.
  - `HandlePhotoMessage`: downloads highest-resolution photo, dispatches prompt with caption, and cleans up temporary files.
  - `HandleVoiceMessage`: verifies local Whisper installation, downloads `.ogg` voice, transcribes audio, and executes command or forwards prompt.
  - Telegram bot event registration tying handlers to `tgbot.Bot`.
- Implement unit and integration tests across `streaming_test.go`, `callbacks_test.go`, and `handlers_messages_test.go`.

## Capabilities

### New Capabilities
- `bot-streaming`: Response streaming, message handlers, callback query router, and passive PC-to-Telegram mirroring.

### Modified Capabilities

## Impact

- Completes interactive messaging and callback handling in `internal/bot`.
- Integrates with `internal/monitor`, `internal/formatter`, `internal/bridge`, `internal/cdp`, `internal/bot/ui`, `internal/voice`, and `internal/database`.
