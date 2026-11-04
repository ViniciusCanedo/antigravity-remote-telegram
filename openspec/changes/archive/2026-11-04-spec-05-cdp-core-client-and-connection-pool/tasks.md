## 1. CDP Ports Implementation (`internal/cdp/ports.go`)

- [x] 1.1 Implement `DefaultCDPPorts`, `IsPortAvailable`, and `FindAvailablePort` in `internal/cdp/ports.go`
- [x] 1.2 Implement `internal/cdp/ports_test.go` covering port checks and first available port selection

## 2. CDP Core Client Implementation (`internal/cdp/client.go`)

- [x] 2.1 Implement structs, options, interfaces (`WebSocketConn`), and constructor `NewClient`
- [x] 2.2 Implement HTTP `/json/list` discovery and 3-tier target selection in `discoverTarget`
- [x] 2.3 Implement WebSocket connection handshake, read loop, context tracking (`Runtime.executionContextCreated`/`Destroyed`), and heartbeat
- [x] 2.4 Implement thread-safe RPC `Call` with pending calls, timers, and response unmarshaling
- [x] 2.5 Implement `Disconnect` and `clearPendingCalls` preventing memory leaks
- [x] 2.6 Implement `discoverAndConnectForWorkspace`, active probing (`probeWorkbenchPages`, `probeWorkspaceFolderPath`), and `launchAndConnectWorkspace`
- [x] 2.7 Implement `tryReconnect` with exponential backoff and event emission
- [x] 2.8 Implement `WaitForCascadePanelReady` and `GetPrimaryContextID`

## 3. CDP Core Client Unit Tests

- [x] 3.1 Implement `internal/cdp/client_test.go` (mirrors `tests/services/cdpService.test.ts`)
- [x] 3.2 Implement `internal/cdp/client_workspace_test.go` (mirrors `tests/services/cdpService.workspace.test.ts`)
- [x] 3.3 Implement `internal/cdp/client_error_test.go` (mirrors `tests/services/cdpService.errorHandling.test.ts`)
- [x] 3.4 Implement `internal/cdp/client_security_test.go` (mirrors `tests/services/cdpService.security.test.ts`)

## 4. Connection Pool Implementation (`internal/cdp/pool.go`)

- [x] 4.1 Implement `BackgroundDetector`, `PoolClient`, `ClientFactory`, and `ConnectionPool` structs
- [x] 4.2 Implement `extractProjectName`, `GetOrConnect` with singleflight locking and re-validation
- [x] 4.3 Implement detector registries with auto-stopping on replacement
- [x] 4.4 Implement `DisconnectWorkspace`, `DisconnectAll`, and lifecycle event bridging
- [x] 4.5 Implement `GetActiveWorkspaceNames` and `GetConnected`

## 5. Connection Pool Unit Tests

- [x] 5.1 Implement `internal/cdp/pool_test.go` (mirrors `tests/services/cdpConnectionPool.test.ts`)
- [x] 5.2 Implement `internal/cdp/pool_bugfix_test.go` (mirrors `tests/services/cdpConnectionPool.bugfix.test.ts`)

## 6. Verification & Quality Gates

- [x] 6.1 Run `go test -v -race ./...` and ensure 100% tests pass
- [x] 6.2 Run `go vet ./...`
- [x] 6.3 Run `gofmt -w .` and check `gofmt -l .` is clean
- [x] 6.4 Run `go build ./...`
