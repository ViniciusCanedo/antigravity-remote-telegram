# bot-core Specification

## Purpose
TBD - created by archiving change spec-15-telegram-bot-core-middleware-and-commands. Update Purpose after archive.
## Requirements
### Requirement: Bot Authentication Middleware
The `internal/bot` package SHALL provide `WithAuth`, `IsUserAllowed`, and `AuthMiddleware` to enforce whitelisting of Telegram user IDs.

#### Scenario: Authorized user interaction
- **WHEN** user ID is in the allowed list
- **THEN** execution proceeds to the next handler

#### Scenario: Unauthorized message interaction
- **WHEN** user ID is not in the allowed list and interaction is a regular message
- **THEN** the message is silently dropped without invoking the next handler

#### Scenario: Unauthorized callback query interaction
- **WHEN** user ID is not in the allowed list and interaction is a callback query
- **THEN** `AnswerCallbackQuery` is invoked with "You do not have permission." and next handler is not called

### Requirement: Message Content Parser
The `internal/bot` package SHALL provide `ParseMessageContent` to determine whether a message is a slash command and tokenize its arguments while respecting quoted segments.

#### Scenario: Plain text message
- **WHEN** a message does not start with "/"
- **THEN** `IsCommand` is false and original `Text` is preserved

#### Scenario: Single slash message
- **WHEN** a message contains only "/"
- **THEN** `IsCommand` is false and `Text` is "/"

#### Scenario: Command with unquoted arguments
- **WHEN** a message begins with "/mode gpt-4"
- **THEN** `IsCommand` is true, `CommandName` is "mode", and `Args` is `["gpt-4"]`

#### Scenario: Command with double-quoted arguments
- **WHEN** a message contains double quotes such as `/template add "Name" "Prompt content"`
- **THEN** arguments inside quotes are extracted with outer quotes stripped

### Requirement: Template Slash Command Handler
The `internal/bot` package SHALL provide `SlashCommandHandler` to manage prompt templates in SQLite.

#### Scenario: Unknown slash command
- **WHEN** `HandleCommand` is called with an unknown command name
- **THEN** it returns an error message indicating unknown command

#### Scenario: List templates when empty
- **WHEN** `HandleCommand` is called with "template" and no args and repository is empty
- **THEN** it returns a message indicating no templates registered

#### Scenario: List templates with items
- **WHEN** `HandleCommand` is called with "template" and registered templates exist
- **THEN** it returns the formatted list of template names

#### Scenario: Add template success
- **WHEN** `HandleCommand` is called with "template" and `["add", "name", "prompt"]`
- **THEN** it saves the template in the repository and returns success confirmation

#### Scenario: Add template missing arguments
- **WHEN** `HandleCommand` is called with "template" and fewer than 3 arguments
- **THEN** it returns missing arguments usage message

#### Scenario: Delete template
- **WHEN** `HandleCommand` is called with "template" and `["delete", "name"]`
- **THEN** it removes the template if it exists or reports not found

### Requirement: Workspace Command Handler
The `internal/bot` package SHALL provide `WorkspaceCommandHandler` to resolve workspace paths for channels.

#### Scenario: Workspace found for channel
- **WHEN** channel has an existing workspace binding
- **THEN** it resolves the absolute workspace path and returns true

#### Scenario: Workspace not found for channel
- **WHEN** channel has no workspace binding
- **THEN** it returns empty string and false

### Requirement: Join Command Handler
The `internal/bot` package SHALL provide `JoinCommandHandler` to resolve project paths and expose session picker (ADR-005).

#### Scenario: Project path resolution
- **WHEN** `ResolveProjectPath` is invoked with a project name
- **THEN** it delegates to `workspaceService.GetWorkspacePath`

### Requirement: Cleanup Command Handler
The `internal/bot` package SHALL provide `CleanupCommandHandler` to find and purge inactive sessions and workspace bindings.

#### Scenario: Purge channel resources
- **WHEN** `CleanupByChannelID` is invoked for a given channel ID
- **THEN** it deletes chat sessions and bindings associated with that channel

#### Scenario: Find inactive sessions
- **WHEN** `FindInactiveSessions` is called with guild ID and days cutoff
- **THEN** it filters sessions created prior to the cutoff threshold

### Requirement: Telegram Bot Lifecycle and Core Commands
The `internal/bot` package SHALL provide `Bot` struct, `StartBot`, channel key helpers, pool connection listeners, and command dispatchers.

#### Scenario: Channel key formatting and decomposition
- **WHEN** `ChannelKey` is called with chat ID and thread ID
- **THEN** it formats as `<chatId>:<threadId>` when thread is non-zero, or `<chatId>` when zero

#### Scenario: Command menu registration
- **WHEN** bot starts up
- **THEN** it registers native bot commands with the Telegram API

