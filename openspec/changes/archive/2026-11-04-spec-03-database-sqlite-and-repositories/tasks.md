## 1. Database Connection Setup

- [x] 1.1 Implement `Open(dbPath)` and `OpenInMemory()` in `internal/database/sqlite.go` with WAL mode and busy timeout
- [x] 1.2 Write unit tests for SQLite connection and pragma validation in `internal/database/sqlite_test.go`

## 2. ChatSessionRepository

- [x] 2.1 Implement `ChatSessionRepository` and structs in `internal/database/session_repo.go`
- [x] 2.2 Implement unit tests for all `ChatSessionRepository` methods in `internal/database/session_repo_test.go`

## 3. WorkspaceBindingRepository

- [x] 3.1 Implement `WorkspaceBindingRepository` and structs in `internal/database/binding_repo.go`
- [x] 3.2 Implement unit tests for all `WorkspaceBindingRepository` methods in `internal/database/binding_repo_test.go`

## 4. TemplateRepository

- [x] 4.1 Implement `TemplateRepository` and structs in `internal/database/template_repo.go`
- [x] 4.2 Implement unit tests for all `TemplateRepository` methods in `internal/database/template_repo_test.go`

## 5. ScheduleRepository

- [x] 5.1 Implement `ScheduleRepository` and structs in `internal/database/schedule_repo.go`
- [x] 5.2 Implement unit tests for all `ScheduleRepository` methods in `internal/database/schedule_repo_test.go`

## 6. Verification & Quality Gates

- [x] 6.1 Run `go test -v -race ./internal/database/...` and ensure 100% pass
- [x] 6.2 Run `go vet ./...`
- [x] 6.3 Run `gofmt -w .` and check `gofmt -l .`
- [x] 6.4 Run `CGO_ENABLED=0 go build ./...`
