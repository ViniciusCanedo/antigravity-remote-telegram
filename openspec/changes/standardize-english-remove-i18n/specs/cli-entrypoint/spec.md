## MODIFIED Requirements

### Requirement: Interactive Setup Subcommand
The `internal/cli` package SHALL provide the `setup` subcommand running an interactive 3-step configuration wizard.

#### Scenario: Allowed User IDs validation
- **WHEN** allowed user IDs input is empty
- **THEN** validation fails with "Please enter at least one user ID."
- **WHEN** allowed user IDs input contains non-numeric values
- **THEN** validation fails with a descriptive error specifying the invalid value

#### Scenario: Token verification
- **WHEN** a valid Telegram bot token is entered and validated against Telegram Bot API
- **THEN** bot information (username and first name) is returned and printed

#### Scenario: Workspace directory resolution and completion
- **WHEN** a workspace path starting with `~` is provided
- **THEN** it expands to the user's home directory and validates directory existence
