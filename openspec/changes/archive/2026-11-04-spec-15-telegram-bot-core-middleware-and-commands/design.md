## Context

The TypeScript implementation of `antigravity-telegram-remote` relies on `telegraf` / `grammy` to coordinate bot commands, authentication middleware (`src/middleware/auth.ts`), message parsing (`src/commands/messageParser.ts`), and command handlers for chat, projects/workspaces, session joining, templates, and cleanup.
In this Go port, we utilize `github.com/go-telegram/bot` and modularize the bot core and commands inside `internal/bot`, maintaining complete feature parity with the TypeScript implementation and complying with ADR-005 (interactive session picker) and ADR-006 (unified authentication middleware).

## Goals / Non-Goals

**Goals:**
- Provide `TelegramChannel` struct with formatted channel key utilities (`ChannelKey`, `ParseChannelKey`, `GetChannelFromMessage`, `GetChannelFromCallback`).
- Implement `AuthMiddleware`, `WithAuth`, and `IsUserAllowed` to restrict bot interactions exclusively to `allowedUserIDs`.
- Implement `ParseMessageContent` for command and argument extraction (including quoted string handling).
- Implement `ChatCommandHandler`, `WorkspaceCommandHandler`, `JoinCommandHandler`, `SlashCommandHandler`, and `CleanupCommandHandler`.
- Implement native bot commands (`/start`, `/help`, `/project`, `/new`, `/chat`, `/mode`, `/model`, `/stop`, `/screenshot`, `/template`, `/template_add`, `/template_delete`, `/autoaccept`, `/status`, `/cleanup`, `/ping`) matching original HTML/text formats and translations.
- Register bot commands with Telegram API via `setMyCommands` on startup.
- Handle WebSocket disconnect/reconnect events (`workspace:disconnected`, `workspace:reconnected`, `workspace:reconnectFailed`) on the CDP connection pool.

**Non-Goals:**
- Streaming LLM messages and inline callback handling (covered in Spec 16).
- CLI commands and entrypoint main function (covered in Spec 17).

## Decisions

- **Framework**: Use `github.com/go-telegram/bot` for Telegram Bot API integration, matching other Go bot implementations in the workspace.
- **Middleware Design**: AuthMiddleware checks user ID against `allowedUserIDs`. For non-whitelisted users, callback queries receive an alert with "You do not have permission.", while regular messages are dropped silently.
- **Concurrency & State**: Store pending plan edits, stop requests, and plan caches in `Bot` struct protected by a `sync.RWMutex`.
- **Database Access**: Reuse existing repositories (`TemplateRepository`, `WorkspaceBindingRepository`, `ChatSessionRepository`) and services (`WorkspaceService`, `ChatSessionService`, `TitleGeneratorService`, `PromptDispatcher`).

## Risks / Trade-offs

- [Risk] Telegram thread vs channel distinction → [Mitigation] Explicitly track `ChatID` and `ThreadID` (0 if no thread/topic) and format keys as `<chatId>:<threadId>` or `<chatId>`.
- [Risk] Command argument splitting with double quotes → [Mitigation] Match original regex `/(?:[^\s"]+|"[^"]*")+/g` and strip outer quotes.
