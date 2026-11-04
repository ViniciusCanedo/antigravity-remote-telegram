## ADDED Requirements

### Requirement: Path Traversal Prevention and Symlink Evaluation
The `workspace` package SHALL provide `ResolveSafePath(inputPath, baseDir)` to resolve paths relative to `baseDir`, preventing directory traversal escapes (`..`, absolute paths outside baseDir) and symlink bypass attempts (`filepath.EvalSymlinks`).

#### Scenario: Resolves valid path within base directory
- **WHEN** `ResolveSafePath` is called with a relative path within `baseDir`
- **THEN** it returns the resolved absolute path without error

#### Scenario: Rejects path traversal attempting to escape base directory
- **WHEN** `ResolveSafePath` is called with a relative path attempting to navigate outside `baseDir` (e.g. `../config/secret.json` or `a/../../outside`)
- **THEN** it returns an error containing `"Path traversal detected"`

#### Scenario: Rejects absolute path outside base directory
- **WHEN** `ResolveSafePath` is called with an absolute path not contained within `baseDir` (e.g. `/etc/passwd` or `/tmp/evil`)
- **THEN** it returns an error containing `"Path traversal detected"`

#### Scenario: Allows absolute path inside base directory
- **WHEN** `ResolveSafePath` is called with an absolute path already located inside `baseDir`
- **THEN** it returns the path without error

#### Scenario: Resolves exact base directory
- **WHEN** `ResolveSafePath` is called with `"."`
- **THEN** it returns the cleaned base directory path

#### Scenario: Rejects symlink bypass escaping base directory
- **WHEN** `ResolveSafePath` resolves a symlink whose realpath on disk falls outside `baseDir`
- **THEN** it returns an error containing `"Path traversal detected (symlink bypass attempt)"`

### Requirement: Workspace Directory Management
The `WorkspaceService` SHALL provide lifecycle management for workspace directories rooted at `baseDir`, including base directory creation, listing non-hidden subdirectories, path validation, and existence checks.

#### Scenario: Ensure base directory creates directory when missing
- **WHEN** `EnsureBaseDir()` is called and `baseDir` does not exist
- **THEN** the directory is created on disk with permissions `0755`

#### Scenario: Ensure base directory succeeds when directory exists
- **WHEN** `EnsureBaseDir()` is called on an existing directory
- **THEN** no error is returned

#### Scenario: Scan workspaces returns sorted non-hidden subdirectories
- **WHEN** `ScanWorkspaces()` is called on `baseDir` containing subdirectories and files
- **THEN** only non-hidden directory names are returned in lexicographically sorted order

#### Scenario: Scan workspaces ignores files and dot directories
- **WHEN** `ScanWorkspaces()` is called on a directory containing files and dot directories (e.g. `.git`)
- **THEN** files and hidden directories are excluded from the result

#### Scenario: Validate workspace path resolves safely
- **WHEN** `ValidatePath(relativePath)` is called
- **THEN** it delegates to `ResolveSafePath` and returns the resolved path or traversal error

#### Scenario: Workspace exists returns true only for directories
- **WHEN** `Exists(workspaceName)` is called
- **THEN** it returns `true, nil` for an existing directory, `false, nil` for a non-existent entry or a file, and error if path traversal is detected

### Requirement: Background Cron Schedule Service
The `ScheduleService` SHALL manage scheduled jobs backed by a `ScheduleRepositoryInterface` and a `CronEngine`.

#### Scenario: Restore enabled schedules on startup
- **WHEN** `RestoreAll(jobCallback)` is invoked
- **THEN** all enabled schedules in the repository are registered with the cron engine, returning the count of restored jobs

#### Scenario: Add new schedule validates cron and persists
- **WHEN** `AddSchedule(cronExpression, prompt, workspacePath, jobCallback)` is called with a valid cron expression
- **THEN** the cron expression is validated, persisted with `Enabled: true`, scheduled with the cron engine, and the created record is returned

#### Scenario: Add schedule rejects invalid cron expression
- **WHEN** `AddSchedule` is called with an invalid cron expression
- **THEN** an error `"Invalid cron expression: <expression>"` is returned without saving to the database or scheduling

#### Scenario: Remove schedule stops task and deletes from database
- **WHEN** `RemoveSchedule(scheduleId)` is called
- **THEN** the active running cron task is stopped and the schedule is deleted from SQLite

#### Scenario: Stop all schedules on shutdown
- **WHEN** `StopAll()` is called
- **THEN** all active running cron tasks are stopped and the internal active tasks map is cleared

#### Scenario: List all schedules
- **WHEN** `ListSchedules()` is called
- **THEN** all schedules from the repository are returned

### Requirement: Execution Mode Service
The `ModeService` SHALL manage the bot's execution mode between `fast` and `plan` with thread-safe access.

#### Scenario: Initial default mode is fast
- **WHEN** `NewModeService()` is created
- **THEN** `GetCurrentMode()` returns `fast`

#### Scenario: Switch mode with valid name case-insensitively
- **WHEN** `SetMode("plan")` or `SetMode("FAST")` is called
- **THEN** it returns `Success: true`, sets the normalized mode, and updates `GetCurrentMode()`

#### Scenario: Reject empty or invalid mode name
- **WHEN** `SetMode("")` or `SetMode("invalid")` is called
- **THEN** it returns `Success: false` with a descriptive error message without changing the current mode

### Requirement: LLM Model Service
The `ModelService` SHALL manage the active LLM model among the 6 supported Antigravity models (`gemini-3.1-pro-high`, `gemini-3.1-pro-low`, `gemini-3-flash`, `claude-sonnet-4.6-thinking`, `claude-opus-4.6-thinking`, `gpt-oss-120b-medium`) with `gemini-3-flash` as default.

#### Scenario: Initial default model is gemini-3-flash
- **WHEN** `NewModelService()` is created
- **THEN** `GetCurrentModel()` returns `gemini-3-flash`

#### Scenario: Switch model with valid name case-insensitively
- **WHEN** `SetModel("claude-sonnet-4.6-thinking")` or `SetModel("GEMINI-3-FLASH")` is called
- **THEN** it returns `Success: true`, updates the current model, and retains the state

#### Scenario: Reject empty or invalid model name
- **WHEN** `SetModel("")` or `SetModel("invalid")` is called
- **THEN** it returns `Success: false` with a descriptive error without changing the current model

### Requirement: Auto-Accept Approval Service
The `AutoAcceptService` SHALL provide a boolean state machine toggling automated dialog approvals, accepting commands `on`, `off`, `status`, and their aliases (`enable`, `disabled`, `true`, `false`, `0`, `1`).

#### Scenario: Query initial status without changing state
- **WHEN** `Handle("status")` or `Handle()` is called
- **THEN** it returns `Success: true`, `Changed: false`, and current enabled state

#### Scenario: Toggle auto-accept on
- **WHEN** `Handle("on")` is called when disabled
- **THEN** it sets enabled to `true`, returning `Success: true` and `Changed: true`

#### Scenario: Toggle auto-accept off
- **WHEN** `Handle("off")` is called when enabled
- **THEN** it sets enabled to `false`, returning `Success: true` and `Changed: true`

#### Scenario: Handle invalid argument without state change
- **WHEN** `Handle("invalid")` is called
- **THEN** it returns `Success: false`, `Changed: false`, and error message containing `"on/off/status"`

### Requirement: Title Generator Service
The `TitleGeneratorService` SHALL generate short topic/session titles via CDP script evaluation or fallback text extraction, sanitizing output for channel/topic names.

#### Scenario: Generate title via fallback extracts and truncates text
- **WHEN** `GenerateTitle(ctx, prompt)` is called without CDP
- **THEN** workspace prefixes are stripped, leading text (up to 40 characters) is extracted, sanitized, and truncated to at most 80 runes

#### Scenario: Empty prompt or special characters returns untitled
- **WHEN** `GenerateTitle` is called with empty text or symbols only
- **THEN** `"untitled"` is returned

#### Scenario: Sanitize for channel name preserves CJK and converts spaces/symbols
- **WHEN** `SanitizeForChannelName` is called
- **THEN** text is lowercased, spaces/special characters become hyphens, CJK characters are preserved, consecutive hyphens collapsed, and leading/trailing hyphens removed

#### Scenario: Fallback to text extraction when CDP call fails
- **WHEN** `GenerateTitle(ctx, prompt, cdp)` is called and CDP evaluation fails or returns `{ok: false}`
- **THEN** title is generated via fallback text extraction

### Requirement: Prompt Dispatcher Concurrency and Serialization
The `PromptDispatcher` SHALL serialize prompt dispatching per workspace (`ws:<workspaceName>`) and per channel (`<chatId>:<threadId>`), ensuring FIFO execution without blocking concurrent requests on different workspaces.

#### Scenario: Serializes prompt sends to the same channel
- **WHEN** two prompt requests are sent sequentially or concurrently to the same channel
- **THEN** the second request does not execute until the first request completes

#### Scenario: Allows parallel prompt sends to different workspaces
- **WHEN** prompt requests are sent to different channels bound to different workspaces
- **THEN** both requests execute concurrently in parallel

#### Scenario: Cleans up lock entries after completion
- **WHEN** a prompt send completes
- **THEN** internal lock maps release the completed lock entry so subsequent sends do not leak or block

#### Scenario: Handles errors without breaking subsequent queue execution
- **WHEN** the underlying `SendPromptImpl` fails with an error
- **THEN** the lock is still released and subsequent queued requests execute normally
