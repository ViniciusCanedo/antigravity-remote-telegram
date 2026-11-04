## 1. Middleware & Message Parser

- [x] 1.1 Implement `internal/bot/middleware.go` with `WithAuth`, `IsUserAllowed`, and `AuthMiddleware`
- [x] 1.2 Implement unit tests in `internal/bot/middleware_test.go`
- [x] 1.3 Implement `internal/bot/message_parser.go` with `ParseMessageContent`
- [x] 1.4 Implement unit tests in `internal/bot/message_parser_test.go`

## 2. Command Handlers

- [x] 2.1 Implement `internal/bot/handlers_template.go` with `SlashCommandHandler`
- [x] 2.2 Implement unit tests in `internal/bot/handlers_template_test.go`
- [x] 2.3 Implement `internal/bot/handlers_project.go` with `WorkspaceCommandHandler`
- [x] 2.4 Implement unit tests in `internal/bot/handlers_project_test.go`
- [x] 2.5 Implement `internal/bot/handlers_session.go` with `JoinCommandHandler`
- [x] 2.6 Implement unit tests in `internal/bot/handlers_session_test.go`
- [x] 2.7 Implement `internal/bot/handlers_cleanup.go` with `CleanupCommandHandler`
- [x] 2.8 Implement unit tests in `internal/bot/handlers_cleanup_test.go`

## 3. Bot Core & Chat Commands

- [x] 3.1 Implement `internal/bot/handlers_chat.go` with `ChatCommandHandler`
- [x] 3.2 Implement unit tests in `internal/bot/handlers_chat_test.go`
- [x] 3.3 Implement `internal/bot/bot.go` with channel key helpers, `Bot` struct, `StartBot`, command registration, and connection pool listeners
- [x] 3.4 Implement unit tests in `internal/bot/bot_test.go`

## 4. Quality Gate & Archive

- [x] 4.1 Run full quality gate (`go test -v -race ./...`, `go vet ./...`, `gofmt -w .`, `go build ./...`)
- [x] 4.2 Sync spec deltas to `openspec/specs/bot-core/spec.md`
- [x] 4.3 Archive change with `openspec archive -y spec-15-telegram-bot-core-middleware-and-commands`
- [x] 4.4 Re-verify quality gate post-archive
