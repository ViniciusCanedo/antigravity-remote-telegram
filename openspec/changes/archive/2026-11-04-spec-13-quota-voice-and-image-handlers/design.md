# Design: QuotaService, VoiceHandler, and ImageHandler

## Overview

This design documents the port of `src/services/quotaService.ts`, `src/utils/voiceHandler.ts`, and `src/utils/imageHandler.ts` to Golang (`internal/quota`, `internal/voice`, and `internal/bot`).

```
+-----------------------------------------------------------------------------------+
|                                 Bot Core Layer                                    |
|                                                                                   |
|  +--------------------+    +----------------------+    +-----------------------+  |
|  |   internal/quota   |    |    internal/voice    |    |     internal/bot      |  |
|  |     Service        |    |  TranscribeVoice     |    |   ImageHandler        |  |
|  +---------+----------+    +----------+-----------+    +-----------+-----------+  |
+------------|--------------------------|----------------------------|--------------+
             |                          |                            |
             v                          v                            v
  +--------------------+     +----------------------+     +----------------------+
  | language_server    |     | ffmpeg + whisper-cli |     | Telegram Bot API     |
  | HTTPS :<port>      |     | (16kHz WAV + base.en)|     | /file/bot<token>/... |
  | GetUserStatus      |     |                      |     |                      |
  +--------------------+     +----------------------+     +----------------------+
```

## 1. `internal/quota` Architecture

### 1.1 Process & Port Discovery
- `CommandExecutor`: Abstraction `func(ctx context.Context, command string) (stdout string, stderr string, err error)` for shell command execution.
- `ProcessDiscoverer`: Interface allowing alternative or mock process/port discovery mechanisms.
- Standard discovery runs `pgrep -fl language_server` and scans for `--csrf_token`.
- Port discovery runs `lsof -nP -a -iTCP -sTCP:LISTEN -p <pid>` and extracts TCP listening ports.

### 1.2 Loopback HTTPS Communication
- Queries `https://127.0.0.1:<port>/exa.language_server_pb.LanguageServerService/GetUserStatus` via `POST`.
- Headers sent:
  - `Content-Type: application/json`
  - `Connect-Protocol-Version: 1`
  - `X-Codeium-Csrf-Token: <csrfToken>`
- Configured with `tls.Config{InsecureSkipVerify: true}` and timeout `2000ms` since Antigravity uses a self-signed loopback certificate.

### 1.3 Caching and Single-Retry Resilience
- Thread-safe via `sync.Mutex`.
- Caches `cachedPid`, `cachedCsrfToken`, `cachedPort`.
- If PID or Token changes, cache is invalidated.
- If request to `cachedPort` fails, `cachedPort` is cleared and `fetchQuotaWithRetry` triggers exactly 1 retry (`retryCount < 1`) by re-scanning listening ports.

## 2. `internal/voice` Architecture

### 2.1 Availability Verification (`CheckWhisperAvailability`)
- Inspects setup sequentially:
  1. Package existence (`nodejs-whisper` directory or local whisper setup).
  2. Executable existence (`whisper-cli` regular file).
  3. Model presence (`base.en` in models directory).
- Returns explicit user-facing error message string pointer or `nil` when available.

### 2.2 Download and Storage
- `DownloadTelegramVoice` queries `botApi.GetFile(voice.FileID)`.
- Validates non-empty file path and HTTP 200 response with non-empty content.
- Writes file to temporary directory (`/tmp/antigravity-telegram-remote-voice/<timestamp>-<file_unique_id><ext>`).

### 2.3 Audio Processing & Transcription
- `TranscribeVoice` registers deferred removal of the downloaded voice file.
- Default pipeline:
  - Invokes `ffmpeg -y -i <voicePath> -ar 16000 -ac 1 -c:a pcm_s16le <wavPath>`.
  - Removes intermediate `.wav` via `defer os.Remove(wavPath)`.
  - Invokes `whisper-cli -m <modelPath> -f <wavPath> -nt`.
- Injected `WhisperRunner` allows deterministic unit testing without requiring system binaries.
- Post-processes output with `StripWhisperTimestamps` to remove `[00:00:00.000 --> 00:00:02.000]` tags.

## 3. `internal/bot` Image Handling Architecture

### 3.1 Validation and Sanitization
- `IsImageAttachment`: MIME type prefix `image/` or regex match on extensions (`.png`, `.jpg`, `.jpeg`, `.webp`, `.gif`, `.bmp`).
- `MimeTypeToExtension`: Maps common MIME types to extension (`jpeg/jpg -> jpg`, `webp -> webp`, `gif -> gif`, `bmp -> bmp`, default `png`).
- `SanitizeFileName`: Cleans filenames by replacing characters outside `[a-zA-Z0-9._-]` with `-`, collapsing `-+` into `-`, and trimming edge hyphens.

### 3.2 Prompt Augmentation
- `BuildPromptWithAttachmentUrls`: Appends `[Telegram Attached Images]` metadata section with numbered references and URLs, or provides default prompt when empty.

### 3.3 Inbound Telegram Photo Processing
- `DownloadTelegramImages`: Accepts up to 4 photos per message, resolves file paths via Telegram API, downloads content to `/tmp/antigravity-telegram-remote-images/`, and returns `InboundImageAttachment` slices.
- `CleanupInboundImageAttachments`: Deletes temporary files.

### 3.4 Outbound Extracted Image Conversion
- `ToTelegramInputFile`: Converts `ExtractedResponseImage` to `TelegramInputFileResult` (`Buffer []byte`, `Name string`).
- Decodes Base64 data (handling raw base64 and data URLs) or downloads remote HTTP/HTTPS images.
- Ensures valid image extensions on generated filenames.

## Tradeoffs and Decisions

1. **Dependency Injection for Exec and HTTP**:
   - Both `quota.Service` and `voice.TranscribeVoice` accept functional options / runner overrides to enable hermetic unit tests without subprocesses or live network calls.
2. **Deterministic Temporary Directories**:
   - `os.TempDir()` is used with dedicated subdirectories (`antigravity-telegram-remote-voice` and `antigravity-telegram-remote-images`) matching original TypeScript naming for consistency.
3. **Loopback TLS Insecurity**:
   - `InsecureSkipVerify: true` is scoped strictly to `127.0.0.1` requests to language_server, matching original TypeScript `rejectUnauthorized: false`.
