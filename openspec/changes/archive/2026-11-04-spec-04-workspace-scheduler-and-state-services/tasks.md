# Tasks

## 1. Workspace Sanitization and Directory Management
- [x] 1.1 Implement `ResolveSafePath` in `internal/workspace/sanitize.go` with anti-traversal and symlink escape checks
- [x] 1.2 Write unit tests in `internal/workspace/sanitize_test.go` matching `sanitize.test.ts` and `sanitize.bugfix.test.ts`
- [x] 1.3 Implement `WorkspaceService` in `internal/workspace/service.go` with `EnsureBaseDir`, `ScanWorkspaces`, `ValidatePath`, `GetBaseDir`, `GetWorkspacePath`, and `Exists`
- [x] 1.4 Write unit tests in `internal/workspace/service_test.go` matching `workspaceService.test.ts`

## 2. Background Cron Scheduler
- [x] 2.1 Implement `ScheduleService`, `CronEngine`, and `ScheduledTask` in `internal/scheduler/service.go`
- [x] 2.2 Implement real `robfig/cron/v3` adapter and mock engine for unit tests
- [x] 2.3 Write unit tests in `internal/scheduler/service_test.go` matching `scheduleService.test.ts`

## 3. Bridge State Services (Mode, Model, AutoAccept)
- [x] 3.1 Implement `ModeService` in `internal/bridge/mode_service.go`
- [x] 3.2 Write unit tests in `internal/bridge/mode_service_test.go` matching `modeService.test.ts`
- [x] 3.3 Implement `ModelService` in `internal/bridge/model_service.go`
- [x] 3.4 Write unit tests in `internal/bridge/model_service_test.go` matching `modelService.test.ts`
- [x] 3.5 Implement `AutoAcceptService` in `internal/bridge/auto_accept.go`
- [x] 3.6 Write unit tests in `internal/bridge/auto_accept_test.go` matching `autoAcceptService.test.ts`

## 4. Bridge Title Generator and Prompt Dispatcher
- [x] 4.1 Implement `TitleGeneratorService` in `internal/bridge/title_generator.go` with CDP evaluate script and CJK-aware channel sanitization
- [x] 4.2 Write unit tests in `internal/bridge/title_generator_test.go` matching `titleGeneratorService.test.ts`
- [x] 4.3 Implement `PromptDispatcher` in `internal/bridge/dispatcher.go` with workspace/channel FIFO lock chaining
- [x] 4.4 Write unit tests in `internal/bridge/dispatcher_test.go` matching `promptDispatcher.test.ts` and `promptDispatcher.concurrency.test.ts`

## 5. Verification and Quality Gate
- [x] 5.1 Run `go test -v -race ./...` and ensure all tests pass cleanly
- [x] 5.2 Run `go vet ./...`, `gofmt -w .`, and `go build ./...`
