## ADDED Requirements

### Requirement: Unified Entrypoint & Cobra Root Command
The `cmd/antigravity-bot` binary and `internal/cli` package SHALL provide the main command entrypoint and Cobra command tree.

#### Scenario: No subcommand with missing configuration
- **WHEN** the CLI is executed with no subcommand and neither `~/.antigravity-telegram-remote/config.json` nor `.env` exists
- **THEN** it prints the welcome box and triggers the interactive setup wizard

#### Scenario: No subcommand with existing configuration
- **WHEN** the CLI is executed with no subcommand and configuration exists
- **THEN** it executes the `start` action

#### Scenario: Global flags resolution
- **WHEN** `--verbose` is provided
- **THEN** log level is set to `debug`
- **WHEN** `--quiet` is provided
- **THEN** log level is set to `error`

### Requirement: Welcome Banner & ASCII Art
The `internal/cli` package SHALL provide `FormatWelcomeBox` and `PrintWelcome` rendering a 50-character wide bordered box.

#### Scenario: ANSI escape code stripping
- **WHEN** measuring visible length of a string containing ANSI escape sequences
- **THEN** `VisibleLength` returns the character count excluding escape codes

#### Scenario: Welcome box formatting
- **WHEN** `FormatWelcomeBox` is called with a version string
- **THEN** all bordered rows have identical visual width, containing version info and command instructions

### Requirement: Start Subcommand
The `internal/cli` package SHALL provide the `start` subcommand to initialize the bot daemon.

#### Scenario: Start execution sequence
- **WHEN** `start` is executed
- **THEN** it resolves log level, prints `Logo`, acquires the instance lock, starts non-blocking update check, and launches `bot.StartBot`

### Requirement: Interactive Setup Subcommand
The `internal/cli` package SHALL provide the `setup` subcommand running an interactive 3-step configuration wizard.

#### Scenario: Allowed User IDs validation
- **WHEN** allowed user IDs input is empty
- **THEN** validation fails with "Vui lòng nhập ít nhất một ID người dùng."
- **WHEN** allowed user IDs input contains non-numeric values
- **THEN** validation fails with a descriptive error specifying the invalid value

#### Scenario: Token verification
- **WHEN** a valid Telegram bot token is entered and validated against Telegram Bot API
- **THEN** bot information (username and first name) is returned and printed

#### Scenario: Workspace directory resolution and completion
- **WHEN** a workspace path starting with `~` is provided
- **THEN** it expands to the user's home directory and validates directory existence

### Requirement: Open Subcommand
The `internal/cli` package SHALL provide the `open` subcommand to discover an available CDP port and launch Antigravity.

#### Scenario: Port discovery
- **WHEN** ports in candidate list `[9222, 9223, 9333, 9444, 9555, 9666]` are checked
- **THEN** the first available port where a TCP listener can bind is selected

#### Scenario: All ports busy
- **WHEN** all candidate CDP ports are in use
- **THEN** an error message is printed and execution terminates with exit code 1

### Requirement: Doctor Subcommand
The `internal/cli` package SHALL provide the `doctor` subcommand to audit environment dependencies and DOM selectors.

#### Scenario: 7 environment checks
- **WHEN** `doctor` is executed without `--selectors`
- **THEN** it checks config directory, config file, `.env` file, required variables, Go runtime version, Antigravity binary installation, and CDP ports

#### Scenario: DOM selectors audit
- **WHEN** `doctor --selectors` is executed
- **THEN** it connects to Antigravity via CDP and audits the 7 core DOM selectors, reporting health count

### Requirement: Update Checker Service
The `internal/cli` package SHALL provide `CheckForUpdates`, `FetchLatestVersion`, `ShouldCheckForUpdates`, and `CompareSemver`.

#### Scenario: 24-hour cooldown
- **WHEN** the cache file contains a timestamp within the last 24 hours
- **THEN** `ShouldCheckForUpdates` returns false and no HTTP request is made

#### Scenario: Newer version available
- **WHEN** the npm registry returns a higher semver version than the current version
- **THEN** an update notification is printed and the cache timestamp is updated

#### Scenario: Network error tolerance
- **WHEN** network requests fail or time out
- **THEN** the error is silently ignored and does not interrupt application flow
