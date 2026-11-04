## Architecture

The workspace, scheduler, and bridge state subsystem is organized into three packages:

1. `internal/workspace`:
   - `sanitize.go`: Core path validation ensuring all paths resolve strictly within designated base directories, preventing directory traversal (`..`, absolute path escapes) and symlink bypasses (`filepath.EvalSymlinks`).
   - `service.go`: `WorkspaceService` managing workspace root directories, listing subdirectories (excluding hidden files/folders), and verifying workspace existence.

2. `internal/scheduler`:
   - `service.go`: `ScheduleService` orchestrating scheduled cron tasks persisted in `database.ScheduleRepository`. It decouples from direct cron implementations via a `CronEngine` interface (`github.com/robfig/cron/v3` in production, mocks in tests) and supports restoring enabled tasks, adding new schedules, stopping running tasks, and deleting from SQLite.

3. `internal/bridge`:
   - `mode_service.go`: `ModeService` tracking active execution mode (`fast` vs `plan`) with thread-safe `sync.RWMutex`, case-insensitive input handling, and localized message generation.
   - `model_service.go`: `ModelService` managing current LLM model among 6 supported models (`gemini-3.1-pro-high`, `gemini-3.1-pro-low`, `gemini-3-flash`, `claude-sonnet-4.6-thinking`, `claude-opus-4.6-thinking`, `gpt-oss-120b-medium`) with `sync.RWMutex`.
   - `auto_accept.go`: `AutoAcceptService` managing automated tool approval dialog status (`on`, `off`, `status`) and aliases (`enable`, `disabled`, `true`, `false`, `0`, `1`).
   - `title_generator.go`: `TitleGeneratorService` generating session/topic titles via CDP script evaluation (`GENERATE_TITLE_SCRIPT`) or extracting leading text (first 40 characters), stripped of workspace prefix, sanitized for Telegram channel/topic naming (lowercase, hyphens, preserving CJK characters, collapsed hyphens, truncated to 80 chars, falling back to `"untitled"`).
   - `dispatcher.go`: `PromptDispatcher` serializing prompt execution per workspace (`ws:<workspaceName>`) and per channel (`<chatId>:<threadId>` or `<chatId>`). Uses chained `chan struct{}` channels under a mutex so sequential requests wait for predecessors without blocking unrelated workspaces.

## Data Flow & Component Interactions

```
+--------------------------------------------------------------+
|                       Telegram Handlers                      |
+-------+--------------------+---------------------+-----------+
        |                    |                     |
        v                    v                     v
+---------------+    +----------------+    +-------------------+
|  /mode /model |    |  /autoaccept   |    | Inbound Prompts   |
+-------+-------+    +-------+--------+    +---------+---------+
        |                    |                       |
        v                    v                       v
+---------------+    +----------------+    +-------------------+
| ModeService   |    | AutoAccept     |    | PromptDispatcher  |
| ModelService  |    | Service        |    +---------+---------+
+---------------+    +----------------+              | (Workspace/Channel Lock)
                                                     v
+------------------------+                 +-------------------+
| WorkspaceService       |                 | CDP Execution     |
| (ResolveSafePath)      |                 | TitleGenerator    |
+------------------------+                 +-------------------+
        ^
        | (Target Workspace Path)
+-------+----------------+
| ScheduleService        | <---> [database.ScheduleRepository]
| (robfig/cron/v3)       |
+------------------------+
```

## Interfaces & Contracts

### CronEngine & ScheduledTask
```go
type ScheduledTask interface {
    Stop()
    Start()
}

type CronEngine interface {
    Validate(cronExpression string) bool
    Schedule(cronExpression string, fn func()) (ScheduledTask, error)
}
```

### ScheduleRepositoryInterface
```go
type ScheduleRepositoryInterface interface {
    Create(input database.CreateScheduleInput) (database.ScheduleRecord, error)
    FindAll() ([]database.ScheduleRecord, error)
    FindById(id int64) (*database.ScheduleRecord, error)
    FindEnabled() ([]database.ScheduleRecord, error)
    Delete(id int64) (bool, error)
    Update(id int64, input database.UpdateScheduleInput) (bool, error)
}
```

### CdpEvaluator
```go
type CdpEvaluator interface {
    GetPrimaryContextId() *int
    Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)
    GetCurrentWorkspaceName() string
}
```

### PromptDispatcher
```go
type PromptDispatcherDeps struct {
    Bridge         any
    ModeService    *ModeService
    ModelService   *ModelService
    SendPromptImpl SendPromptFunc
}
```

## Concurrency & Thread-Safety

- `PromptDispatcher`: Employs chained completion channels (`done chan struct{}`) guarded by a `sync.Mutex`. When a send request arrives:
  1. The dispatcher acquires the mutex, grabs the current `previous` channel for the workspace/channel key, registers a new `current` channel, and releases the mutex.
  2. Outside the mutex, the request waits on `<-previous.done` (if any).
  3. The request executes `SendPromptImpl` and catches any error (preventing pipeline breakage).
  4. In `defer`, `close(current.done)` unlocks the next waiting request, and under the mutex the key is cleared if it still points to `current`.
- `ModeService`, `ModelService`, and `AutoAcceptService`: Protected by `sync.RWMutex` providing thread-safe concurrent reads and serialized atomic writes.
- `ScheduleService`: Protected by `sync.Mutex` on `activeTasks` map access, adding, removing, and mass stopping tasks.

## Error Handling and Edge Cases

- Path traversal: Strict validation checking both relative path traversal (`..` prefix) and canonical symlink evaluation (`filepath.EvalSymlinks`). If the target directory does not yet exist on disk, canonical evaluation falls back to normalized relative prefix check.
- Non-existent files vs directories: `WorkspaceService.Exists` verifies `info.IsDir()`, ensuring regular files return `false`.
- Invalid cron expressions: Returns error immediately before database insertion or task scheduling.
- Title sanitization: Strips `[ワークスペース: ...]` prefix, converts uppercase to lowercase, replaces non-CJK/non-alphanumeric with hyphens, collapses multiple hyphens, trims leading/trailing hyphens, limits to 80 runes, and defaults to `"untitled"` if empty.
- Legacy `ProcessManager`: Omitted in Go per ADR-006; concurrency and serialization are handled cleanly by `PromptDispatcher`.
