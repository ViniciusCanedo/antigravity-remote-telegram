## Why

Antigravity Telegram Remote requires handlers for model quota queries via local Antigravity `language_server`, local voice transcription via Whisper, and bidirectional image attachment handling. Porting from TypeScript (`src/services/quotaService.ts`, `src/utils/voiceHandler.ts`, `src/utils/imageHandler.ts`) to Go provides:
1. `internal/quota`: Local HTTPS client for `GetUserStatus` with `X-Codeium-Csrf-Token`, port caching, and 1-retry fallback.
2. `internal/voice`: Local audio pipeline for `.ogg` download, ffmpeg conversion, whisper-cli transcription, timestamp stripping, and cleanup.
3. `internal/bot`: Image validation, sanitization, prompt URL augmentation, temp photo download, and base64/URL conversion to Telegram input files.

## What Changes

- Create `internal/quota/service.go` and `internal/quota/service_test.go`:
  - `QuotaInfo`, `ModelQuota`, and `UserStatusData` domain models.
  - Process and port discovery via `pgrep` and `lsof`.
  - Local HTTPS client querying `/exa.language_server_pb.LanguageServerService/GetUserStatus` with loopback TLS and CSRF header.
  - Thread-safe caching and 1-retry on cached port failure.
- Create `internal/voice/transcriber.go` and `internal/voice/transcriber_test.go`:
  - Environment inspection via `CheckWhisperAvailability`.
  - `DownloadTelegramVoice` to `/tmp/antigravity-telegram-remote-voice/`.
  - `TranscribeVoice` with deferred cleanup, ffmpeg conversion, and timestamp stripping.
- Create `internal/bot/image_handler.go` and `internal/bot/image_handler_test.go`:
  - Validation: `IsImageAttachment`, `MimeTypeToExtension`, `SanitizeFileName`.
  - Prompt enrichment: `BuildPromptWithAttachmentUrls`.
  - Inbound download: `DownloadTelegramImages` and `CleanupInboundImageAttachments`.
  - Outbound conversion: `ToTelegramInputFile` decoding base64/data URLs or fetching HTTP/HTTPS images.

## Capabilities

### New Capabilities
- `quota-voice-image`: Local model quota querying, voice message transcription via Whisper, and bidirectional image attachment handling.

### Modified Capabilities

## Impact

- Provides `QuotaService` for `/status` and `/models` Telegram bot commands.
- Enables Telegram voice message processing without cloud dependencies.
- Handles image attachments for multimodal interactions with Antigravity.
