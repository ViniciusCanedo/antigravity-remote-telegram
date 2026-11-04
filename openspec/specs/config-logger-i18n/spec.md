# config-logger-i18n Specification

## Purpose
Defines configuration loading, runtime environment merging, ANSI colored console logging, and embedded multi-language internationalization (i18n) for the Antigravity Telegram Remote service.
## Requirements
### Requirement: Structured ANSI Console Logger
The `internal/logger` package SHALL provide an ANSI-colored console logger supporting log levels `debug`, `info`, `warn`, `error`, and `none` with numeric priority filtering, 24-hour timestamp formatting `[HH:MM:SS]`, and dedicated tag formatting for `[INFO]`, `[WARN]`, `[ERROR]`, `[DEBUG]`, `[PHASE]`, `[DONE]`, and `Divider`.

#### Scenario: Default log level filters debug messages
- **WHEN** a logger is initialized with default level `info`
- **THEN** calls to `Info()`, `Warn()`, and `Error()` write to output, while `Debug()` produces no output

#### Scenario: Debug log level emits all messages
- **WHEN** a logger is initialized with level `debug`
- **THEN** calls to `Debug()`, `Info()`, `Warn()`, and `Error()` all emit formatted messages

#### Scenario: None log level suppresses all output
- **WHEN** a logger is initialized with level `none`
- **THEN** all logging calls including `Phase()`, `Done()`, and `Divider()` are suppressed

#### Scenario: Runtime log level escalation
- **WHEN** a logger initialized at `info` has `SetLogLevel("debug")` called
- **THEN** subsequent `Debug()` calls produce output

#### Scenario: Helper formatting and ANSI tags
- **WHEN** `Info()`, `Warn()`, `Error()`, `Debug()`, `Phase()`, and `Done()` are called
- **THEN** they prefix lines with formatted timestamp `[HH:MM:SS]` and respective colored tags (`Cyan`, `Yellow`, `Red`, `Dim`, `Magenta`, `Green`)

#### Scenario: Section divider rendering
- **WHEN** `Divider("Section")` is called at `info` level
- **THEN** it emits `[DONE]` followed by `── Section ──...` padded to at least 50 characters

### Requirement: Runtime Configuration Loading and Precedence
The `internal/config` package SHALL load and merge application configuration adhering to strict precedence: Environment Variables > Persisted JSON (`~/.antigravity-telegram-remote/config.json`) > Defaults.

#### Scenario: Missing required bot token
- **WHEN** configuration is loaded without `TELEGRAM_BOT_TOKEN` in env or persisted config
- **THEN** an error containing `Missing required config: TELEGRAM_BOT_TOKEN` is returned

#### Scenario: Missing required allowed user IDs
- **WHEN** configuration is loaded with valid token but empty `ALLOWED_USER_IDS`
- **THEN** an error containing `Missing required config: ALLOWED_USER_IDS` is returned

#### Scenario: Environment variables override persisted values
- **WHEN** both environment variables and persisted config specify `TELEGRAM_BOT_TOKEN` and `ALLOWED_USER_IDS`
- **THEN** the environment variable values take precedence

#### Scenario: Telegram user ID numeric filtering
- **WHEN** `ALLOWED_USER_IDS` contains comma-separated values with whitespace and non-numeric characters
- **THEN** non-numeric entries and empty strings are filtered out, preserving only valid numeric IDs (`^\d+$`)

#### Scenario: Default configuration fallbacks
- **WHEN** optional settings are omitted from environment and persisted file
- **THEN** `workspaceBaseDir` defaults to `~/Code`, `autoApproveFileEdits` defaults to `false`, `logLevel` defaults to `info`, `extractionMode` defaults to `structured`, and `useTopics` defaults to `true`

#### Scenario: Tilde expansion in workspace base dir
- **WHEN** `workspaceBaseDir` is specified with leading `~` or `~/`
- **THEN** it expands to the user's home directory path

#### Scenario: Response delivery mode resolution
- **WHEN** `ResolveResponseDeliveryMode()` is called
- **THEN** it returns `stream`

### Requirement: Configuration Persistence and Directory Management
The `internal/config` package SHALL manage the application configuration directory (`~/.antigravity-telegram-remote`), check file existence, and persist configuration updates formatted with 2-space indented JSON.

#### Scenario: Path resolution helpers
- **WHEN** `GetConfigDir()`, `GetConfigFilePath()`, and `GetDefaultDbPath()` are invoked
- **THEN** they return paths under the user home directory joined with `.antigravity-telegram-remote`, `config.json`, and `antigravity.db` respectively

#### Scenario: Config directory creation and partial save
- **WHEN** `Save()` is invoked with partial configuration fields
- **THEN** the configuration directory is created if missing, and non-nil fields are merged into existing configuration and written formatted with 2 spaces

#### Scenario: Malformed configuration file error
- **WHEN** `Load()` encounters invalid JSON in `config.json`
- **THEN** it returns an error containing `Failed to parse config file`

### Requirement: Embedded Multi-Language Internationalization (i18n)
The `internal/i18n` package SHALL embed locale JSON dictionaries (`en.json`, `ja.json`, `vi.json`), support setting the active language, provide fallbacks from active language to English to raw key, and perform literal `{{var}}` variable interpolation.

#### Scenario: Variable substitution with double braces
- **WHEN** `T()` is called with a template containing `{{name}}` and variables `map[string]any{"name": "Alice"}`
- **THEN** all occurrences of `{{name}}` are replaced with `Alice`

#### Scenario: Dot-separated variable keys
- **WHEN** a template contains `{{file.name}}` and variables `map[string]any{"file.name": "main.go"}`
- **THEN** the placeholder is replaced literally without regular expression syntax interpretation

#### Scenario: Fallback to English and key
- **WHEN** a translation key is missing in the active language but present in `en`
- **THEN** the English translation is returned; if missing in all dictionaries, the raw key is returned

#### Scenario: Type coercion in variable replacement
- **WHEN** variables contain integers, booleans, or `nil` values
- **THEN** they are formatted as `"42"`, `"true"`, and `"null"` respectively

