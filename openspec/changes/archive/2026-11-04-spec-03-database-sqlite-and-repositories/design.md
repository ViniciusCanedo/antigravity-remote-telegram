## Context

The original TypeScript implementation of `antigravity-telegram-remote` relied on `better-sqlite3`, a synchronous SQLite library implemented via Node.js native C++ addons. The Go port eliminates runtime C++ dependencies and external dynamic libraries by adopting `modernc.org/sqlite`, a pure Go transpilation of SQLite that compiles cleanly with `CGO_ENABLED=0`.

The persistence layer stores configuration and state across four core models:
1. `chat_sessions`: Tracks Telegram topics/channels linked to Antigravity sessions.
2. `workspace_bindings`: Maps Telegram channels to workspace file paths.
3. `templates`: Stores reusable prompt macros for quick dispatch.
4. `schedules`: Tracks cron expressions and prompts for automated execution.

## Goals / Non-Goals

**Goals:**
- Provide zero-CGO SQLite persistence compatible with cross-compilation across macOS, Linux, and Windows.
- Enable Write-Ahead Logging (WAL) and configurable busy timeout (`5000ms`) for smooth concurrent reading and writing.
- Support isolated in-memory instances (`OpenInMemory`) with single-connection constraints for fast, zero-side-effect unit testing.
- Maintain identical database schemas, column names, constraints, and semantics from the TypeScript implementation.
- Encapsulate prepared statements inside repository structs with proper lifecycle management (`Close()`).

**Non-Goals:**
- Heavy ORM frameworks (e.g. GORM, Ent) — lightweight `database/sql` queries match the TypeScript repository design closely.
- Schema migration versioning tools (e.g. Goose, Migrate) — existing tables use lightweight `CREATE TABLE IF NOT EXISTS` idempotency.
- Multi-node distributed database replication — Antigravity Telegram Remote is a single-instance remote daemon.

## Decisions

### 1. Pure-Go SQLite Driver (`modernc.org/sqlite`)
- **Rationale**: ADR-002 establishes that the application must compile with `CGO_ENABLED=0` to ensure effortless single-binary cross-platform releases without needing target platform C cross-compilers.
- **Alternative considered**: `github.com/mattn/go-sqlite3`. Rejected because it requires CGO and GCC/Clang on build systems.

### 2. Connection Pooling & Test Isolation
- **Rationale**: SQLite in `:memory:` mode creates an isolated database per connection. To ensure all repository operations within a unit test target the same in-memory instance, `OpenInMemory()` sets `db.SetMaxOpenConns(1)`. For disk files, standard pooling with WAL mode and `PRAGMA busy_timeout = 5000;` is applied.
- **Alternative considered**: Unique named in-memory databases (`file:memdb1?mode=memory&cache=shared`). Rejected because `db.SetMaxOpenConns(1)` with `:memory:` is simpler and completely leak-free per test execution.

### 3. Precompiled Prepared Statements
- **Rationale**: Each repository prepares its queries during instantiation (`New...Repository`) and holds them in private fields, matching the TypeScript `this.stmtX = db.prepare(...)` pattern for optimal execution efficiency.
- **Alternative considered**: Query preparation on every call. Rejected due to unnecessary parse and compile overhead on SQLite.

### 4. Pointer Types for Optional Fields and Partial Updates
- **Rationale**: Go lacks TypeScript's optional property syntax (`foo?: string`). Structs like `UpdateScheduleInput` and `UpdateTemplateInput` use pointer fields (`*string`, `*bool`) to distinguish between unset fields (`nil`) and explicitly set zero-values (`""`, `false`).

## Risks / Trade-offs

- [Risk] Concurrent writes encountering `SQLITE_BUSY`:
  → Mitigation: Set `PRAGMA busy_timeout = 5000;` upon opening disk databases, giving SQLite 5 seconds to wait for write locks before returning an error.
- [Risk] Memory leaks if prepared statements are left open:
  → Mitigation: Each repository provides a `Close() error` method that closes all prepared statements cleanly.
- [Risk] SQLite integer boolean mapping (0 vs 1):
  → Mitigation: Repositories explicitly convert Go booleans to `0` and `1` upon insertion/update, and check `row.enabled == 1` upon query.
