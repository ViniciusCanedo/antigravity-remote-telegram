## ADDED Requirements

### Requirement: CDP Candidate Ports & Availability
The `internal/cdp` package SHALL provide candidate ports (`9222, 9223, 9333, 9444, 9555, 9666`) and functions to verify TCP port availability on `127.0.0.1` and find the first available port.

#### Scenario: Port availability check
- **WHEN** `IsPortAvailable(port)` is called on a free local port
- **THEN** it returns `true`; if the port is bound and listening, it returns `false`

#### Scenario: Finding first available port
- **WHEN** `FindAvailablePort(ports)` is called with a slice of candidate ports
- **THEN** it returns the first port that is not in use, or `(0, false)` if all are occupied

### Requirement: CDP HTTP Target Discovery
The `internal/cdp` package SHALL scan configured ports via HTTP `GET http://127.0.0.1:<port>/json/list` and discover the correct workbench target adhering to strict 3-tier fallback precedence.

#### Scenario: Strict target discovery ignores Launchpad and selects workbench
- **WHEN** `/json/list` returns both `Launchpad` and `workbench.html` pages
- **THEN** `DiscoverTarget()` selects the workbench page, extracts the workspace title, and returns its `webSocketDebuggerUrl`

#### Scenario: Target discovery fallback tiers
- **WHEN** strict page targets are absent
- **THEN** discovery falls back to non-page workbench/Antigravity/Cascade targets without Launchpad, and finally to any target matching Antigravity or Launchpad keywords

### Requirement: WebSocket JSON-RPC Client & Execution Context Tracking
The `internal/cdp` package SHALL establish a WebSocket connection to the DevTools debugger URL, initialize `Runtime.enable`, track execution contexts from `Runtime.executionContextCreated` and `Runtime.executionContextDestroyed`, and support thread-safe JSON-RPC calls.

#### Scenario: Connection and context tracking
- **WHEN** `Connect()` is invoked
- **THEN** `Runtime.enable` is sent, execution contexts are tracked in the contexts slice, and `GetPrimaryContextID()` returns the ID of the `cascade-panel` context

#### Scenario: Waiting for cascade-panel context
- **WHEN** `WaitForCascadePanelReady(timeout, interval)` is called
- **THEN** it polls contexts until a context whose URL contains `cascade-panel` appears, returning `true`, or returns `false` on timeout

#### Scenario: Thread-safe RPC calls with timeout
- **WHEN** `Call(method, params)` is called
- **THEN** a unique sequential ID is assigned, the request is sent over WebSocket, and the result is returned or an error is returned if timeout expires or socket disconnects

### Requirement: Cross-Platform Workspace Launching
The `internal/cdp` package SHALL launch Antigravity when no responding CDP port is found, using platform-specific executable paths without shell interpolation (`exec.Command`), and poll for the new workbench page.

#### Scenario: Darwin launch with open -a fallback
- **WHEN** launching on macOS and direct CLI path fails
- **THEN** it falls back to `open -a Antigravity --args --remote-debugging-port=<port> <path>`

#### Scenario: Windows launch with LOCALAPPDATA
- **WHEN** launching on Windows with `LOCALAPPDATA` set
- **THEN** it executes `%LOCALAPPDATA%\Programs\Antigravity\Antigravity.exe` with remote debugging port and new-window flags

#### Scenario: Linux launch with ANTIGRAVITY_PATH
- **WHEN** launching on Linux with `ANTIGRAVITY_PATH` set
- **THEN** it executes the specified binary path directly without shell interpolation

### Requirement: Disconnection & Pending Call Cleanup
The `internal/cdp` package SHALL reject all pending RPC calls on disconnection, stop heartbeats, and emit the `disconnected` event without memory leaks.

#### Scenario: Pending calls rejected on disconnect
- **WHEN** the WebSocket disconnects while calls are pending
- **THEN** all pending calls are immediately rejected with an error and the pending calls map is emptied

#### Scenario: Disconnect quietly suppresses events during switch
- **WHEN** switching workspaces and performing CDP probes
- **THEN** existing connections are closed quietly without emitting `disconnected` or triggering auto-reconnect

### Requirement: Auto-Reconnection Lifecycle
The `internal/cdp` package SHALL automatically attempt reconnection upon unexpected disconnection when `maxReconnectAttempts > 0`, respecting configured delay intervals and emitting `reconnected` or `reconnectFailed`.

#### Scenario: Auto-reconnect to same workspace path
- **WHEN** disconnected unexpectedly with known `currentWorkspacePath`
- **THEN** `tryReconnect()` reconnects to the same workspace path and emits `reconnected` on success

#### Scenario: Reconnection fails after max attempts
- **WHEN** reconnection attempts fail up to `maxReconnectAttempts`
- **THEN** it emits `reconnectFailed` with the final error and stops attempting

### Requirement: Workspace Connection Pool & Detector Management
The `internal/cdp` package SHALL manage isolated `PoolClient` instances per workspace project name, re-validate existing connections, serialize concurrent connection attempts with singleflight locking, and manage background detector lifecycles.

#### Scenario: Connection caching and re-validation
- **WHEN** `GetOrConnect(workspacePath)` is called twice for the same workspace
- **THEN** the second call re-validates the connection and returns the cached instance

#### Scenario: Singleflight concurrent connection locking
- **WHEN** two goroutines concurrently call `GetOrConnect` for the same workspace
- **THEN** only one connection is initiated and both goroutines receive the same client instance

#### Scenario: Detector lifecycle management
- **WHEN** an active detector is replaced or the workspace is disconnected
- **THEN** the detector's `Stop()` method is called and it is removed from the registry
