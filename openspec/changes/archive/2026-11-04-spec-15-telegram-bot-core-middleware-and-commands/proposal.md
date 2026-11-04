## Why

Porting the Telegram bot core lifecycle, authentication middleware, command parser, and foundational command handlers from TypeScript to Go enables running antigravity-telegram-remote as a native, single-binary Go service while preserving 100% functional and behavioral fidelity with the original system.

## What Changes

- Implement `internal/bot/middleware.go` providing `WithAuth`, `IsUserAllowed`, and `AuthMiddleware` for whitelisting allowed Telegram user IDs and silently dropping or rejecting unauthorized interactions.
- Implement `internal/bot/message_parser.go` providing `ParseMessageContent` to accurately tokenize bot commands, quoted arguments, and unquoted arguments.
- Implement `internal/bot/handlers_chat.go` providing `ChatCommandHandler` and command handling for `/chat`, `/start`, `/help`, `/new`, `/mode`, `/model`, `/stop`, `/screenshot`, `/status`, `/ping`, and `/autoaccept`.
- Implement `internal/bot/handlers_project.go` providing `WorkspaceCommandHandler` and `/project` / `/workspace` listing.
- Implement `internal/bot/handlers_session.go` providing `JoinCommandHandler` and `/join` session picker (ADR-005).
- Implement `internal/bot/handlers_template.go` providing `SlashCommandHandler` and template CRUD commands (`/template`, `/template_add`, `/template_delete`).
- Implement `internal/bot/handlers_cleanup.go` providing `CleanupCommandHandler` and inactive session purging (`/cleanup`).
- Implement `internal/bot/bot.go` orchestrating bot initialization, SQLite WAL lifecycle, Antigravity launcher checks, bridge pool event handlers, command registration (`setMyCommands`), and channel state synchronization.

## Capabilities

### New Capabilities
- `bot-core`: Telegram bot core lifecycle, authentication middleware, message parser, command dispatching, and command handlers.

### Modified Capabilities

## Impact

- `internal/bot` package receives core bot structs, middleware, message parser, and command handlers.
- Integrates with `internal/bridge`, `internal/cdp`, `internal/database`, `internal/services`, `internal/config`, and `internal/bot/ui`.
