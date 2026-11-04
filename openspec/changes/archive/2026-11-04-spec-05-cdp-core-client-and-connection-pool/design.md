## Context

The CDP layer connects the Antigravity Telegram Remote service to the local running Antigravity Electron/Chromium IDE. It requires establishing JSON-RPC communication over WebSocket to the Chromium DevTools backend, discovering available window targets via HTTP `/json/list`, launching the IDE process if not already running, keeping the connection alive via heartbeat, tracking V8 execution contexts (`cascade-panel`), and handling workspace switching and disconnections cleanly across multiple workspaces.

## Goals / Non-Goals

**Goals:**
- Implement `ports.go` with `DefaultCDPPorts`, `IsPortAvailable`, and `FindAvailablePort`.
- Implement `client.go` with `Client`, `ClientOptions`, `Context`, `TargetInfo`, `WebSocketConn`, `EventHandler`, thread-safe JSON-RPC calls, target discovery, process launching, execution context tracking, heartbeat, and auto-reconnection.
- Implement `pool.go` with `ConnectionPool`, `BackgroundDetector`, `ClientFactory`, `PoolClient`, singleflight connection locks, detector registration, workspace isolation, and lifecycle events.
- Ensure 100% test coverage matching all original test suites in `tests/services/cdpService.*` and `tests/services/cdpConnectionPool.*`.
- Eliminate shell injection risks by using direct `exec.Command` execution without shell invocation.
- Guarantee race-free concurrent execution (`-race` clean) for goroutines invoking `Call()`, background readers, and heartbeats.

**Non-Goals:**
- DOM selector evaluation and message injection (handled in Spec 06).
- Implementation of specific background detector logic (Approval, ErrorPopup, Planning, UserMessage detectors themselves are handled in subsequent specs; here we provide the detector lifecycle management interfaces).

## Decisions

- **WebSocket Library & Abstraction**:
  - Use `github.com/coder/websocket` for standard production connections.
  - Wrap the WebSocket operations in a `WebSocketConn` interface (`WriteMessage`, `ReadMessage`, `Ping`, `SetPongHandler`, `Close`, `Terminate`).
  - Provide `DialWebSocketFunc` hook in `ClientOptions` allowing tests to mock WebSocket interactions synchronously or point to `httptest.Server`.
- **Concurrency & Thread-Safety**:
  - Protect `Client` state (`contexts`, `pendingCalls`, `isConnectedFlag`, `isSwitchingWorkspace`, `isReconnecting`) with `sync.RWMutex`.
  - Protect WebSocket writes (`WriteMessage`, `Ping`) with a dedicated `writeMu sync.Mutex` to prevent concurrent write collisions on the underlying socket.
  - Manage background reading in a dedicated goroutine (`readLoop`) that unmarshals incoming JSON-RPC responses and matches them with `pendingCall` channels.
- **Heartbeat & Keep-Alive**:
  - Run a 30s heartbeat ticker. On each tick, send a Ping. If no Pong has been received within 40s, terminate the socket to trigger reconnection.
  - When closing or disconnecting quietly, signal `heartbeatStopCh` to terminate the heartbeat goroutine immediately.
- **Pending Call Cleanup**:
  - `clearPendingCalls(err)` stops all active timeouts and delivers `err` to all waiting channels, avoiding goroutine leaks and stalled callers.
- **Connection Pool Singleflight**:
  - In `ConnectionPool.GetOrConnect`, concurrent calls for the same workspace use an `inflightConnect` struct with a `done chan struct{}`. The first caller initiates the connection while subsequent callers await the channel, ensuring only one client is instantiated per workspace during concurrent requests.
- **Detector Lifecycle & Stale Connection Eviction**:
  - Re-validate existing connected clients with `discoverAndConnectForWorkspace`. If re-validation fails, call `Disconnect()` and remove the stale client.
  - If a cached connection is already disconnected, remove it without calling redundant `Disconnect()`.
  - When replacing an existing active detector, stop the old one first.
  - On `workspace:reconnectFailed`, trigger `DisconnectWorkspace` which stops all registered detectors for that workspace.

## Risks / Trade-offs

- [Risk] Concurrent writes to `WebSocketConn` causing corruption or panics.
  - → [Mitigation] Explicit `writeMu sync.Mutex` around all socket write operations.
- [Risk] Memory leaks from orphaned `pendingCall` timers upon sudden connection drops.
  - → [Mitigation] Explicit `clearPendingCalls` iteration stopping each `time.Timer` and removing all map entries.
- [Risk] Shell command injection when launching Antigravity CLI.
  - → [Mitigation] Direct invocation of `exec.Command` with argument slices, never using `sh -c` or shell strings.
