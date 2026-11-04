## Context

The TypeScript implementation of `antigravity-telegram-remote` handles prompt sending, real-time response monitoring, message handlers (text, photo, voice), and callback queries directly in `src/bot/index.ts` and `src/services/cdpBridgeManager.ts`. In Go, we organize these functionalities into cohesive, testable modules under `internal/bot`:
- `internal/bot/streaming.go`: the streaming pipeline, HTML processing, task queues, and `SendPromptToAntigravity`.
- `internal/bot/callbacks.go`: callback query processing for all 11 button families.
- `internal/bot/handlers_messages.go`: text, photo, voice handling and background PC-to-Telegram mirroring.

## Goals / Non-Goals

**Goals:**
- Implement `SerialTaskQueue` providing sequential FIFO task execution per queue (`general`, `response`, `activity`).
- Implement HTML formatting and recovery utilities: `StripHTMLForFile`, `BalanceHTMLTags`, `StripHTML`, `IsJunkEntry`, `FormatActivityLine`.
- Implement `SendPromptToAntigravity` pipeline:
  - Connection checks and initial status message (`📡 <mode> · <model> \n\n Sending...`).
  - Message injection (with images if provided, falling back to URL references).
  - `ResponseMonitor` lifecycle with 2000ms polling, 1800000ms max duration, 3 stop-gone confirmations.
  - Event stream: `onProcessLog`, `onThinkingLog`, `onProgress`, `onComplete`, `onTimeout`.
  - 5000ms periodic progress editing throttle with monotonic version checking (`liveActivityUpdateVersion`, `liveResponseUpdateVersion`).
  - Quota error handling: update activity and response messages with `⚠️ Quota Reached` and display model selection UI.
  - Collapsible thinking blocks extracted from DOM wrapped in `<blockquote expandable>...</blockquote>`.
  - Chunked response splitting: `<= 4096` edits/sends single message; 2 to 5 chunks sends inline messages `(1/N)`; `> 5` chunks sends 5 inline messages `(1/5+file)` and attaches `response-<timestamp>.md`.
  - Generated image extraction and delivery (`cdp.ExtractLatestResponseImages`).
  - Forum topic renaming and database sync (`chatSessionRepo.UpdateDisplayName`).
- Implement `CallbackContext` interface and `HandleCallbackQuery` routing all 11 callback families:
  1. `mode_select:<mode>`
  2. `model_exhausted_<model>`
  3. `model_btn_<model>` & `model_refresh_btn`
  4. `autoaccept_btn_on`, `autoaccept_btn_off`, `autoaccept_btn_refresh`
  5. `project_select:<path>`, `project_page:<n>`
  6. `template_btn_<id>`
  7. `session_select:<title>`
  8. Approval actions (`approve`, `always_allow`, `deny`)
  9. Planning actions (`open`, `proceed`, `view`, `edit`, `refresh`, `page`)
  10. Error popup actions (`dismiss`, `copy_debug`, `retry`)
  11. Cleanup actions (`cleanup_archive:<days>`, `cleanup_delete:<days>`, `cleanup_cancel`)
- Implement `HandleTextMessage`, `HandlePhotoMessage`, `HandleVoiceMessage`, and `StartPassiveMirrorForUserMessage`.
- Register handlers on `tgbot.Bot`.

**Non-Goals:**
- CLI entrypoint and commands (`internal/cli`, covered in Spec 17).
- Re-implementing detector DOM scripts or selectors (already completed in Specs 09, 10, 11, 12).

## Decisions

- **Queue & Monotonic Versions**: To prevent out-of-order edits between elapsed ticks and final completion, each update increments a version integer. Serial queues execute asynchronously while dropping stale versions.
- **Three-Tier Send Fallback**: When sending Telegram messages, try HTML first; if that fails, try `BalanceHTMLTags(text)`; if that fails, try `StripHTML(text)` in plain text.
- **CallbackContext Abstraction**: Abstract Telegram callback update parameters behind `CallbackContext` interface to enable deterministic, fast unit tests without requiring a real Telegram network client or mock server.
- **Passive Mirroring**: UserMessageDetector triggers `StartPassiveMirrorForUserMessage`, notifying Telegram with `🖥️ Sent from PC:\n\n<text>` and launching `ResponseMonitor.StartPassive()`.

## Risks / Trade-offs

- [Risk] Unclosed HTML tags from DOM extraction causing Telegram API 400 Bad Request → [Mitigation] Robust tag balancing in `BalanceHTMLTags` and plain-text fallback in `sendMsg`.
- [Risk] Concurrent edits to Telegram messages triggering rate limits → [Mitigation] 5000ms periodic throttle and serial execution queues.
