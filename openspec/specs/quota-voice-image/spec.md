# quota-voice-image Specification

## Purpose
TBD - created by archiving change spec-13-quota-voice-and-image-handlers. Update Purpose after archive.
## Requirements
### Requirement: Quota Service Process Discovery and Caching
The `internal/quota` package SHALL discover the running `language_server` process and its active TCP listening ports, query model quotas over local loopback HTTPS, cache successful discoveries, and retry once upon cached port failure.

#### Scenario: Returns empty slice when language_server process is not found
- **WHEN** `pgrep -fl language_server` fails or does not find a running process
- **THEN** `FetchQuota(ctx)` returns an empty slice `[]ModelQuota{}` and makes no network requests

#### Scenario: Discovers and caches port for identical PID and CSRF token
- **WHEN** process discovery succeeds and port discovery finds a listening port serving `GetUserStatus`
- **THEN** `FetchQuota(ctx)` returns model configurations and caches the port, so subsequent calls reuse the cached port without re-running port discovery

#### Scenario: Retries once with fresh port scan when cached port fails
- **WHEN** a previously cached port fails to respond to `GetUserStatus`
- **THEN** `FetchQuota(ctx)` invalidates the cached port, triggers a fresh port scan, and retries the request once

#### Scenario: Parses varied JSON responses from language_server
- **WHEN** `GetUserStatus` returns varied model configurations with different label/model keys and float or string quota fractions
- **THEN** `ParseUserStatusResponseBody` parses them into `UserStatusData` with normalized `QuotaInfo`

### Requirement: Local Voice Message Download and Whisper Transcription
The `internal/voice` package SHALL check Whisper local installation availability, download voice messages from the Telegram Bot API to a temporary directory, and transcribe audio using Whisper CLI with automatic timestamp stripping and guaranteed file cleanup.

#### Scenario: Check Whisper setup availability
- **WHEN** `CheckWhisperAvailability` checks the environment
- **THEN** it verifies package installation, regular binary existence, and `base.en` model file presence in order, returning instructions if any check fails or `nil` if all pass

#### Scenario: Downloads voice file from Telegram to temp directory
- **WHEN** `DownloadTelegramVoice` is called with valid voice metadata
- **THEN** it resolves the file path, downloads the audio to `os.TempDir()/antigravity-telegram-remote-voice`, and returns the local path

#### Scenario: Fails when Telegram file path is empty or download fails
- **WHEN** `botApi.GetFile` returns empty path, HTTP status is not OK, or the file is empty
- **THEN** `DownloadTelegramVoice` returns a descriptive error

#### Scenario: Default extension for extensionless voice file
- **WHEN** Telegram `file_path` has no file extension
- **THEN** `DownloadTelegramVoice` appends `.ogg`

#### Scenario: Transcribes voice and cleans up file on failure or success
- **WHEN** `TranscribeVoice` is executed
- **THEN** it cleans up the downloaded voice file via `defer`, runs transcription, and strips timestamp prefixes like `[00:00:00.000 --> 00:00:02.000]`

### Requirement: Bidirectional Image Attachment Processing
The `internal/bot` package SHALL validate image attachments, sanitize filenames, build prompt attachment references, download inbound Telegram photos, clean up temporary images, and convert extracted response images to Telegram input files.

#### Scenario: Validates image content types and extensions
- **WHEN** `IsImageAttachment` is called with content types or filenames
- **THEN** it returns `true` for `image/*` MIME types or recognized image extensions (`png`, `jpg`, `jpeg`, `webp`, `gif`, `bmp`), and `false` otherwise

#### Scenario: Sanitizes attachment filenames
- **WHEN** `SanitizeFileName` is provided arbitrary filename strings with invalid characters or hyphens
- **THEN** it replaces invalid characters with hyphens, collapses consecutive hyphens, and trims outer hyphens

#### Scenario: Builds prompt with attachment URLs
- **WHEN** `BuildPromptWithAttachmentUrls` is called with a prompt and attachment slice
- **THEN** it appends a formatted `[Telegram Attached Images]` block with numbered filenames and local URLs

#### Scenario: Downloads up to 4 inbound Telegram photos
- **WHEN** `DownloadTelegramImages` receives photo metadata
- **THEN** it limits processing to 4 photos, downloads them to `os.TempDir()/antigravity-telegram-remote-images`, and returns `InboundImageAttachment` slices

#### Scenario: Converts base64 and remote URLs to Telegram input files
- **WHEN** `ToTelegramInputFile` receives extracted images with base64 data or HTTP/HTTPS URLs
- **THEN** it decodes the buffer, assigns a valid sanitized image filename with appropriate extension, and returns `TelegramInputFileResult`

