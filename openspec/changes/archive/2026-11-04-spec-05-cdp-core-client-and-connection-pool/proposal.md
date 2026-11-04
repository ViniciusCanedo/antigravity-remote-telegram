## Why

The Antigravity Telegram Remote system relies on the Chrome DevTools Protocol (CDP) to communicate with Antigravity instances (VS Code-based IDE). This requires target discovery across multiple local TCP candidate ports, cross-platform binary launching, WebSocket JSON-RPC 2.0 communication, V8 execution context tracking (particularly `cascade-panel`), heartbeat ping/pong keep-alive, auto-reconnection, and a thread-safe connection pool managing isolated connections and background detectors per workspace.

Migrating these components from TypeScript (`src/utils/cdpPorts.ts`, `src/services/cdpService.ts`, `src/services/cdpConnectionPool.ts`) into native Go (`internal/cdp`) establishes the core CDP infrastructure with thread-safe concurrency, deterministic lifecycle management, singleflight connection locking, zero memory leaks on disconnect, and 100% behavioral fidelity with the original system.

## What Changes

- Create `internal/cdp/ports.go`:
  - Definition of `DefaultCDPPorts` (`[9222, 9223, 9333, 9444, 9555, 9666]`).
  - `IsPortAvailable(port int) bool` using temporary `net.Listen` on `127.0.0.1`.
  - `FindAvailablePort(ports []int) (int, bool)` to select the first free candidate port.
- Create `internal/cdp/client.go`:
  - JSON-RPC 2.0 client using `github.com/coder/websocket` over WebSocket.
  - Pluggable `WebSocketConn` abstraction for testability.
  - Target discovery via HTTP `/json/list` across all candidate ports with 3 priority tiers (workbench vs Launchpad).
  - Cross-platform process launcher (`darwin`, `windows`, `linux`) without shell interpolation (`exec.Command` directly), including macOS `open -a Antigravity` fallback.
  - V8 execution context tracking via `Runtime.executionContextCreated` and `Runtime.executionContextDestroyed`.
  - Context ready waiter (`WaitForCascadePanelReady`) and primary context ID resolution.
  - Synchronous `Call(method, params)` with ID counter, response matching, and timeout handling.
  - Memory-leak-free disconnection rejecting `pendingCalls` and terminating heartbeat.
  - Ping/pong heartbeat (30s ping, 40s pong timeout) with automatic reconnection.
  - Deterministic automatic reconnection with exponential delay and `reconnectFailed` event emission.
- Create `internal/cdp/pool.go`:
  - `ConnectionPool` managing independent `PoolClient` instances keyed by workspace project name (`extractProjectName`).
  - Re-validation of existing connections during `GetOrConnect(workspacePath)`.
  - Singleflight concurrency lock preventing race conditions during concurrent connection requests.
  - Background detector registries (`approvalDetectors`, `errorPopupDetectors`, `planningDetectors`, `userMessageDetectors`) with automatic stopping on replacement and workspace disconnect.
  - Workspace lifecycle events: `workspace:disconnected`, `workspace:reconnected`, `workspace:reconnectFailed`.
- Unit test suites matching original Jest specs:
  - `internal/cdp/ports_test.go`
  - `internal/cdp/client_test.go`
  - `internal/cdp/client_workspace_test.go`
  - `internal/cdp/client_error_test.go`
  - `internal/cdp/client_security_test.go`
  - `internal/cdp/pool_test.go`
  - `internal/cdp/pool_bugfix_test.go`

## Capabilities

### New Capabilities
- `cdp-core`: CDP port candidate management, JSON-RPC WebSocket client, V8 execution context tracking, cross-platform workspace launcher, auto-reconnection, and workspace connection pool with detector lifecycle management.

### Modified Capabilities

## Impact

- Provides the core CDP communication engine consumed by Spec 06 (DOM interaction, prompt injection, approval clicking) and all subsequent background detectors and orchestrators.
- Eliminates Node.js `ws` dependency and shell execution vulnerabilities.
- Guarantees thread-safe WebSocket reads/writes under concurrent Go routines.
