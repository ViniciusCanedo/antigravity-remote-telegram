## Why

The Antigravity Telegram Remote bot requires persistent local storage for managing chat sessions, workspace bindings, prompt templates, and cron schedules across restarts. Migrating from Node.js `better-sqlite3` to a pure-Go SQLite persistence layer (`modernc.org/sqlite`) allows the bot to run as a single static binary with zero external runtime dependencies and without requiring CGO, while preserving WAL concurrency, prepared statements, and schema compatibility.

## What Changes

- Add pure-Go SQLite connection manager (`internal/database/sqlite.go` / `db.go`) supporting both file-based storage and in-memory instances (`:memory:`), automatically creating parent directories, setting `PRAGMA journal_mode = WAL;`, and configuring `busy_timeout`.
- Implement `ChatSessionRepository` (`internal/database/session_repo.go`) managing the `chat_sessions` table: channel-to-session mappings, category-scoped auto-incrementing session numbers, display names, and deletion.
- Implement `WorkspaceBindingRepository` (`internal/database/binding_repo.go`) managing the `workspace_bindings` table: channel-to-workspace path mappings, guild queries, and atomic `ON CONFLICT` upserts.
- Implement `TemplateRepository` (`internal/database/template_repo.go`) managing the `templates` table: reusable prompt templates indexed by unique name with partial prompt update support.
- Implement `ScheduleRepository` (`internal/database/schedule_repo.go`) managing the `schedules` table: cron-based prompt execution schedules, enabled filter queries, and dynamic partial updates.
- Create 100% test coverage matching the original TypeScript Jest test suites in `*_repo_test.go` using isolated in-memory SQLite instances.

## Capabilities

### New Capabilities
- `database-sqlite`: Pure-Go SQLite engine initialization and the four persistence repositories (`chat_sessions`, `workspace_bindings`, `templates`, `schedules`) with WAL mode, prepared statements, and graceful resource cleanup.

### Modified Capabilities

## Impact

- Introduces the `internal/database` package into the Golang codebase.
- Leverages `modernc.org/sqlite` pure-Go driver (`CGO_ENABLED=0` compatible).
- Serves as the persistence dependency for downstream session management, workspace routing, prompt templating, and cron scheduling modules.
