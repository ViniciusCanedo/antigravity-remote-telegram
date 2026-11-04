## Why

The Antigravity Telegram Remote bot requires robust workspace path resolution with strict path traversal prevention, background cron task scheduling with database persistence, and execution state services for bridge management (execution mode, LLM model selection, auto-approval toggles, prompt topic title generation, and FIFO serialized prompt dispatching). Porting these components from TypeScript to Go provides native OS filesystem sandboxing, deterministic concurrency with Go channels and mutexes, and zero-runtime-dependency cron execution with `github.com/robfig/cron/v3`.

## What Changes

- Implement `internal/workspace/sanitize.go` with `ResolveSafePath` providing anti-traversal validation and symlink escape detection (`filepath.EvalSymlinks`).
- Implement `internal/workspace/service.go` (`WorkspaceService`) providing `EnsureBaseDir`, `ScanWorkspaces`, `ValidatePath`, `GetBaseDir`, `GetWorkspacePath`, and `Exists`.
- Implement `internal/scheduler/service.go` (`ScheduleService`) with `RestoreAll`, `AddSchedule`, `RemoveSchedule`, `StopAll`, and `ListSchedules`, backed by `ScheduleRepositoryInterface` and pluggable `CronEngine` (`robfig/cron/v3` and test mock).
- Implement `internal/bridge/mode_service.go` (`ModeService`) managing `fast` and `plan` execution modes, localized descriptions, and thread-safe switching.
- Implement `internal/bridge/model_service.go` (`ModelService`) managing the 6 selectable LLM models (`gemini-3.1-pro-high`, `gemini-3.1-pro-low`, `gemini-3-flash`, `claude-sonnet-4.6-thinking`, `claude-opus-4.6-thinking`, `gpt-oss-120b-medium`) with thread-safe switching.
- Implement `internal/bridge/auto_accept.go` (`AutoAcceptService`) managing automated dialog approvals with boolean state tracking and command argument normalization.
- Implement `internal/bridge/title_generator.go` (`TitleGeneratorService`) generating topic titles via CDP evaluate script or 40-character fallback with channel-name sanitization (CJK rune support, hyphen collapsing, 80-character cap).
- Implement `internal/bridge/dispatcher.go` (`PromptDispatcher`) ensuring FIFO serialized execution per workspace (`ws:<workspaceName>`) and per channel (`<chatId>:<threadId>`) using chained done channels without cross-request blocking.
- Create 100% unit test suites matching all original Jest test cases across `internal/workspace`, `internal/scheduler`, and `internal/bridge`.

## Capabilities

### New Capabilities
- `workspace-services`: Path traversal sanitization, workspace directory scanning, background cron schedule management, bridge state services (mode, model, auto-accept), CJK-aware channel title generation, and workspace-serialized prompt dispatching.

### Modified Capabilities

## Impact

- Introduces packages `internal/workspace`, `internal/scheduler`, and initial services for `internal/bridge`.
- Connects to `internal/database` via `ScheduleRepositoryInterface`.
- Utilizes `github.com/robfig/cron/v3` for cron parsing and execution.
- Enables safe execution of Telegram commands `/mode`, `/model`, `/autoaccept`, workspace selection, and concurrent prompt sending.
