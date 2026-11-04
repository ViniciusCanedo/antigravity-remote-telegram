package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"antigravity-telegram-remote/internal/logger"
	"antigravity-telegram-remote/internal/platform"

	"github.com/coder/websocket"
)

var titleSplitRegex = regexp.MustCompile(`\s[—–-]\s`)

// ClientOptions mirrors CdpServiceOptions (src/services/cdpService.ts:10-17).
type ClientOptions struct {
	PortsToScan          []int
	CallTimeout          time.Duration
	MaxReconnectAttempts *int
	ReconnectDelay       time.Duration
	LaunchPollInterval   time.Duration
	GOOS                 string

	// Injectable hooks for testing
	HTTPGetJSONFunc       func(url string) ([]TargetInfo, error)
	RunCommandFunc        func(command string, args []string) error
	FindAvailablePortFunc func() (int, bool)
	GetAntigravityCliFunc func() (string, error)
	DialWebSocketFunc     func(ctx context.Context, url string) (WebSocketConn, error)
}

// Context mirrors CdpContext (src/services/cdpService.ts:19-23).
type Context struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// TargetInfo represents a target from /json/list.
type TargetInfo struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// WebSocketConn abstracts WebSocket operations for testability.
type WebSocketConn interface {
	WriteMessage(data []byte) error
	ReadMessage() ([]byte, error)
	Ping() error
	SetPongHandler(h func(appData string) error)
	Close() error
	Terminate() error
}

type coderWebSocketAdapter struct {
	conn *websocket.Conn
}

func defaultDialWebSocket(ctx context.Context, u string) (WebSocketConn, error) {
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	return &coderWebSocketAdapter{conn: conn}, nil
}

func (a *coderWebSocketAdapter) WriteMessage(data []byte) error {
	return a.conn.Write(context.Background(), websocket.MessageText, data)
}

func (a *coderWebSocketAdapter) ReadMessage() ([]byte, error) {
	_, data, err := a.conn.Read(context.Background())
	return data, err
}

func (a *coderWebSocketAdapter) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.conn.Ping(ctx)
}

func (a *coderWebSocketAdapter) SetPongHandler(h func(appData string) error) {
	// Handled internally by coder/websocket
}

func (a *coderWebSocketAdapter) Close() error {
	return a.conn.Close(websocket.StatusNormalClosure, "normal closure")
}

func (a *coderWebSocketAdapter) Terminate() error {
	return a.conn.CloseNow()
}

// EventHandler represents a callback function for Client events.
type EventHandler func(payload any)

type pendingCall struct {
	resultCh chan rpcResponse
	timer    *time.Timer
}

type rpcRequest struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage
	Err    error
}

type rpcErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type incomingMessage struct {
	ID     *int64           `json:"id,omitempty"`
	Method string           `json:"method,omitempty"`
	Params json.RawMessage  `json:"params,omitempty"`
	Result json.RawMessage  `json:"result,omitempty"`
	Error  *rpcErrorPayload `json:"error,omitempty"`
}

// Client implements the CDP JSON-RPC client, target discovery, process launching,
// execution context tracking, heartbeat, and reconnection logic.
type Client struct {
	mu                   sync.RWMutex
	writeMu              sync.Mutex
	ports                []int
	isConnectedFlag      bool
	ws                   WebSocketConn
	contexts             []Context
	pendingCalls         map[int64]*pendingCall
	idCounter            int64
	cdpCallTimeout       time.Duration
	targetURL            string
	maxReconnectAttempts int
	reconnectDelay       time.Duration
	reconnectAttemptCnt  int
	isReconnecting       bool
	currentWorkspaceName string
	currentWorkspacePath string
	isSwitchingWorkspace bool
	heartbeatStopCh      chan struct{}
	lastPongTime         time.Time
	listeners            map[string][]EventHandler
	goos                 string
	launchPollInterval   time.Duration

	getJSONFunc           func(url string) ([]TargetInfo, error)
	runCommandFunc        func(command string, args []string) error
	findAvailablePortFunc func() (int, bool)
	getAntigravityCliFunc func() (string, error)
	dialWebSocketFunc     func(ctx context.Context, url string) (WebSocketConn, error)
}

// NewClient initializes a new Client instance with configured options.
func NewClient(opts ClientOptions) *Client {
	c := &Client{
		pendingCalls: make(map[int64]*pendingCall),
		idCounter:    1,
		listeners:    make(map[string][]EventHandler),
	}

	if len(opts.PortsToScan) > 0 {
		c.ports = make([]int, len(opts.PortsToScan))
		copy(c.ports, opts.PortsToScan)
	} else {
		c.ports = make([]int, len(DefaultCDPPorts))
		copy(c.ports, DefaultCDPPorts)
	}

	if opts.CallTimeout > 0 {
		c.cdpCallTimeout = opts.CallTimeout
	} else {
		c.cdpCallTimeout = 30 * time.Second
	}

	if opts.MaxReconnectAttempts != nil {
		c.maxReconnectAttempts = *opts.MaxReconnectAttempts
	} else {
		c.maxReconnectAttempts = 3
	}

	if opts.ReconnectDelay > 0 {
		c.reconnectDelay = opts.ReconnectDelay
	} else {
		c.reconnectDelay = 2000 * time.Millisecond
	}

	if opts.LaunchPollInterval > 0 {
		c.launchPollInterval = opts.LaunchPollInterval
	} else {
		c.launchPollInterval = 1000 * time.Millisecond
	}

	c.goos = opts.GOOS

	// Injected hooks or defaults
	c.getJSONFunc = opts.HTTPGetJSONFunc
	c.runCommandFunc = opts.RunCommandFunc
	c.findAvailablePortFunc = opts.FindAvailablePortFunc
	c.getAntigravityCliFunc = opts.GetAntigravityCliFunc
	c.dialWebSocketFunc = opts.DialWebSocketFunc

	if c.findAvailablePortFunc == nil {
		c.findAvailablePortFunc = func() (int, bool) {
			return FindAvailablePort(c.ports)
		}
	}
	if c.getAntigravityCliFunc == nil {
		c.getAntigravityCliFunc = func() (string, error) {
			return platform.GetAntigravityCliPathWithDeps(platform.PathDeps{
				GOOS:   c.getOS(),
				Getenv: os.Getenv,
				Stat:   os.Stat,
			})
		}
	}
	if c.dialWebSocketFunc == nil {
		c.dialWebSocketFunc = defaultDialWebSocket
	}

	return c
}

func (c *Client) getOS() string {
	if c.goos != "" {
		return c.goos
	}
	return runtime.GOOS
}

// On registers an event listener for a given event name.
func (c *Client) On(event string, handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listeners[event] = append(c.listeners[event], handler)
}

// Emit dispatches an event to all registered listeners.
func (c *Client) Emit(event string, payload any) {
	c.mu.RLock()
	handlers := make([]EventHandler, len(c.listeners[event]))
	copy(handlers, c.listeners[event])
	c.mu.RUnlock()

	for _, h := range handlers {
		h(payload)
	}
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnectedFlag
}

// GetCurrentWorkspaceName returns the name of the currently active workspace.
func (c *Client) GetCurrentWorkspaceName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentWorkspaceName
}

// GetContexts returns a copy of the currently tracked execution contexts.
func (c *Client) GetContexts() []Context {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make([]Context, len(c.contexts))
	copy(res, c.contexts)
	return res
}

func (c *Client) getJSON(u string) ([]TargetInfo, error) {
	if c.getJSONFunc != nil {
		return c.getJSONFunc(u)
	}
	return defaultHTTPGetJSON(u)
}

func defaultHTTPGetJSON(u string) ([]TargetInfo, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(u)
	if err != nil {
		logger.Warn(fmt.Sprintf("[CdpService] Network error fetching %s: %v", u, err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		err := fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
		logger.Warn(fmt.Sprintf("[CdpService] getJson failed: %v", err))
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var targets []TargetInfo
	if err := json.Unmarshal(body, &targets); err != nil {
		logger.Warn(fmt.Sprintf("[CdpService] Failed to parse JSON from %s: %v", u, err))
		return nil, err
	}
	return targets, nil
}

func (c *Client) scanAllPorts() []TargetInfo {
	var allPages []TargetInfo
	c.mu.RLock()
	ports := make([]int, len(c.ports))
	copy(ports, c.ports)
	c.mu.RUnlock()

	for _, port := range ports {
		targets, err := c.getJSON(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
		if err == nil {
			allPages = append(allPages, targets...)
		}
	}
	return allPages
}

// DiscoverTarget scans candidate ports and identifies the workbench target.
func (c *Client) DiscoverTarget() (string, error) {
	allPages := c.scanAllPorts()
	if len(allPages) == 0 {
		logger.Warn("[CdpService] No CDP targets found on any scanned port.")
	}

	var target *TargetInfo

	// Tier 1: Strict match
	for i := range allPages {
		t := &allPages[i]
		if t.Type == "page" &&
			t.WebSocketDebuggerURL != "" &&
			!strings.Contains(t.Title, "Launchpad") &&
			!strings.Contains(t.URL, "workbench-jetski-agent") &&
			(strings.Contains(t.URL, "workbench") || strings.Contains(t.Title, "Antigravity") || strings.Contains(t.Title, "Cascade")) {
			target = t
			break
		}
	}

	// Tier 2: Fallback without strict page/agent restrictions, excluding Launchpad
	if target == nil {
		for i := range allPages {
			t := &allPages[i]
			if t.WebSocketDebuggerURL != "" &&
				(strings.Contains(t.URL, "workbench") || strings.Contains(t.Title, "Antigravity") || strings.Contains(t.Title, "Cascade")) &&
				!strings.Contains(t.Title, "Launchpad") {
				target = t
				break
			}
		}
	}

	// Tier 3: Broad fallback including Launchpad
	if target == nil {
		for i := range allPages {
			t := &allPages[i]
			if t.WebSocketDebuggerURL != "" &&
				(strings.Contains(t.URL, "workbench") || strings.Contains(t.Title, "Antigravity") || strings.Contains(t.Title, "Cascade") || strings.Contains(t.Title, "Launchpad")) {
				target = t
				break
			}
		}
	}

	if target != nil && target.WebSocketDebuggerURL != "" {
		c.mu.Lock()
		c.targetURL = target.WebSocketDebuggerURL
		if target.Title != "" && c.currentWorkspaceName == "" {
			parts := titleSplitRegex.Split(target.Title, -1)
			if len(parts) > 0 {
				c.currentWorkspaceName = strings.TrimSpace(parts[0])
			}
		}
		resURL := c.targetURL
		c.mu.Unlock()
		return resURL, nil
	}

	return "", errors.New("CDP target not found on any port.")
}

// Connect establishes the WebSocket connection to targetURL and initializes CDP domains.
func (c *Client) Connect() error {
	c.mu.RLock()
	targetURL := c.targetURL
	c.mu.RUnlock()

	if targetURL == "" {
		discovered, err := c.DiscoverTarget()
		if err != nil {
			return err
		}
		targetURL = discovered
	}

	if targetURL == "" {
		return errors.New("Target URL not established.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ws, err := c.dialWebSocketFunc(ctx, targetURL)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("CDP connection timeout for %s", targetURL)
		}
		return err
	}

	c.mu.Lock()
	c.ws = ws
	c.isConnectedFlag = true
	c.lastPongTime = time.Now()
	c.mu.Unlock()

	ws.SetPongHandler(func(appData string) error {
		c.mu.Lock()
		c.lastPongTime = time.Now()
		c.mu.Unlock()
		return nil
	})

	// Start read loop
	go c.readLoop(ws)

	// Initialize Runtime domain to get execution contexts
	_, err = c.Call("Runtime.enable", map[string]any{})
	if err != nil {
		_ = c.Disconnect()
		return err
	}

	// Runtime.addBinding (non-fatal)
	_, err = c.Call("Runtime.addBinding", map[string]any{"name": "onantigravity-telegram-remoteDOMChange"})
	if err != nil {
		logger.Warn(fmt.Sprintf("[CdpService] Runtime.addBinding failed (non-fatal): %v", err))
	}

	// Network.enable (non-fatal)
	_, err = c.Call("Network.enable", map[string]any{})
	if err != nil {
		logger.Warn("[CdpService] Network.enable failed — network event detection disabled")
	}

	c.startHeartbeat()
	return nil
}

func (c *Client) readLoop(ws WebSocketConn) {
	for {
		msg, err := ws.ReadMessage()
		if err != nil {
			c.handleDisconnect(ws)
			return
		}
		c.handleMessage(msg)
	}
}

func (c *Client) handleDisconnect(ws WebSocketConn) {
	c.mu.Lock()
	if c.ws != ws {
		c.mu.Unlock()
		return
	}
	c.stopHeartbeatLocked()
	c.isConnectedFlag = false
	c.clearPendingCallsLocked(errors.New("WebSocket disconnected"))
	c.ws = nil
	c.targetURL = ""
	isSwitching := c.isSwitchingWorkspace
	shouldReconnect := c.maxReconnectAttempts > 0 && !c.isReconnecting
	c.mu.Unlock()

	if isSwitching {
		return
	}

	c.Emit("disconnected", nil)

	if shouldReconnect {
		go c.tryReconnect()
	}
}

func (c *Client) handleMessage(msg []byte) {
	var in incomingMessage
	if err := json.Unmarshal(msg, &in); err != nil {
		logger.Warn(fmt.Sprintf("[CdpService] Failed to parse WebSocket message: %v", err))
		return
	}

	if in.ID != nil {
		c.mu.Lock()
		pCall, found := c.pendingCalls[*in.ID]
		if found {
			delete(c.pendingCalls, *in.ID)
			pCall.timer.Stop()
		}
		c.mu.Unlock()

		if found {
			if in.Error != nil {
				errMsg := in.Error.Message
				if errMsg == "" {
					errMsg = fmt.Sprintf("CDP error code %d", in.Error.Code)
				}
				pCall.resultCh <- rpcResponse{Err: errors.New(errMsg)}
			} else {
				pCall.resultCh <- rpcResponse{Result: in.Result}
			}
		}
	}

	if in.Method == "Runtime.executionContextCreated" {
		var payload struct {
			Context Context `json:"context"`
		}
		if err := json.Unmarshal(in.Params, &payload); err == nil {
			c.mu.Lock()
			c.contexts = append(c.contexts, payload.Context)
			c.mu.Unlock()
		}
	} else if in.Method == "Runtime.executionContextDestroyed" {
		var payload struct {
			ExecutionContextID int `json:"executionContextId"`
		}
		if err := json.Unmarshal(in.Params, &payload); err == nil {
			c.mu.Lock()
			idx := -1
			for i, ctx := range c.contexts {
				if ctx.ID == payload.ExecutionContextID {
					idx = i
					break
				}
			}
			if idx != -1 {
				c.contexts = append(c.contexts[:idx], c.contexts[idx+1:]...)
			}
			c.mu.Unlock()
		}
	}

	if in.Method != "" {
		var params any
		if len(in.Params) > 0 {
			_ = json.Unmarshal(in.Params, &params)
		}
		c.Emit(in.Method, params)
	}
}

// Call performs a JSON-RPC 2.0 call over the WebSocket connection with timeout.
func (c *Client) Call(method string, params any) (json.RawMessage, error) {
	c.mu.RLock()
	ws := c.ws
	connected := c.isConnectedFlag
	timeout := c.cdpCallTimeout
	c.mu.RUnlock()

	if ws == nil || !connected {
		return nil, errors.New("WebSocket is not connected")
	}

	if params == nil {
		params = struct{}{}
	}

	c.mu.Lock()
	id := c.idCounter
	c.idCounter++
	resCh := make(chan rpcResponse, 1)

	timer := time.AfterFunc(timeout, func() {
		c.mu.Lock()
		_, found := c.pendingCalls[id]
		if found {
			delete(c.pendingCalls, id)
		}
		c.mu.Unlock()

		if found {
			resCh <- rpcResponse{
				Err: fmt.Errorf("Timeout calling CDP method %s", method),
			}
		}
	})

	c.pendingCalls[id] = &pendingCall{
		resultCh: resCh,
		timer:    timer,
	}
	c.mu.Unlock()

	req := rpcRequest{
		ID:     id,
		Method: method,
		Params: params,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		c.mu.Lock()
		if pCall, ok := c.pendingCalls[id]; ok {
			pCall.timer.Stop()
			delete(c.pendingCalls, id)
		}
		c.mu.Unlock()
		return nil, err
	}

	c.writeMu.Lock()
	err = ws.WriteMessage(reqBytes)
	c.writeMu.Unlock()
	if err != nil {
		c.mu.Lock()
		if pCall, ok := c.pendingCalls[id]; ok {
			pCall.timer.Stop()
			delete(c.pendingCalls, id)
		}
		c.mu.Unlock()
		return nil, err
	}

	resp := <-resCh
	return resp.Result, resp.Err
}

// Disconnect gracefully closes the active connection and suppresses auto-reconnect.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	c.stopHeartbeatLocked()
	savedMax := c.maxReconnectAttempts
	c.maxReconnectAttempts = 0
	ws := c.ws
	c.ws = nil
	c.isConnectedFlag = false
	c.contexts = nil
	c.currentWorkspacePath = ""
	c.currentWorkspaceName = ""
	c.targetURL = ""
	c.clearPendingCallsLocked(errors.New("disconnect() was called"))
	c.maxReconnectAttempts = savedMax
	c.mu.Unlock()

	if ws != nil {
		_ = ws.Close()
	}
	return nil
}

func (c *Client) disconnectQuietly() {
	c.mu.Lock()
	c.stopHeartbeatLocked()
	ws := c.ws
	c.ws = nil
	c.isConnectedFlag = false
	c.contexts = nil
	c.targetURL = ""
	c.clearPendingCallsLocked(errors.New("Disconnected for workspace switch"))
	c.mu.Unlock()

	if ws != nil {
		_ = ws.Close()
	}
}

func (c *Client) clearPendingCalls(err error) {
	c.mu.Lock()
	c.clearPendingCallsLocked(err)
	c.mu.Unlock()
}

func (c *Client) clearPendingCallsLocked(err error) {
	for id, pCall := range c.pendingCalls {
		pCall.timer.Stop()
		pCall.resultCh <- rpcResponse{Err: err}
		delete(c.pendingCalls, id)
	}
}

func (c *Client) startHeartbeat() {
	c.stopHeartbeat()

	c.mu.Lock()
	stopCh := make(chan struct{})
	c.heartbeatStopCh = stopCh
	c.lastPongTime = time.Now()
	ws := c.ws
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				c.mu.RLock()
				currentWS := c.ws
				connected := c.isConnectedFlag
				lastPong := c.lastPongTime
				c.mu.RUnlock()

				if currentWS == nil || !connected || currentWS != ws {
					return
				}

				if time.Since(lastPong) > 40*time.Second {
					logger.Error("[CdpService] Heartbeat: no pong in 40s — terminating WebSocket")
					_ = currentWS.Terminate()
					return
				}

				c.writeMu.Lock()
				err := currentWS.Ping()
				c.writeMu.Unlock()
				if err == nil {
					c.mu.Lock()
					c.lastPongTime = time.Now()
					c.mu.Unlock()
				}
			}
		}
	}()
}

func (c *Client) stopHeartbeat() {
	c.mu.Lock()
	c.stopHeartbeatLocked()
	c.mu.Unlock()
}

func (c *Client) stopHeartbeatLocked() {
	if c.heartbeatStopCh != nil {
		close(c.heartbeatStopCh)
		c.heartbeatStopCh = nil
	}
}

// DiscoverAndConnectForWorkspace connects to the workbench page for a workspace.
func (c *Client) DiscoverAndConnectForWorkspace(workspacePath string) (bool, error) {
	projectName := platform.ExtractProjectNameFromPath(workspacePath)
	c.mu.Lock()
	c.currentWorkspacePath = workspacePath
	isConnected := c.isConnectedFlag
	currentName := c.currentWorkspaceName
	c.mu.Unlock()

	if isConnected && currentName == projectName {
		if c.verifyCurrentWorkspace(projectName, workspacePath) {
			return true, nil
		}
		logger.Warn(fmt.Sprintf("[CdpService] Workspace mismatch detected while reusing connection (expected=%q). Reconnecting...", projectName))
	}

	c.mu.Lock()
	c.isSwitchingWorkspace = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.isSwitchingWorkspace = false
		c.mu.Unlock()
	}()

	return c.discoverAndConnectForWorkspaceImpl(workspacePath, projectName)
}

func (c *Client) verifyCurrentWorkspace(projectName, workspacePath string) bool {
	c.mu.RLock()
	ws := c.ws
	connected := c.isConnectedFlag
	c.mu.RUnlock()

	if ws == nil || !connected {
		return false
	}

	titleResult, err := c.Call("Runtime.evaluate", map[string]any{
		"expression":    "document.title",
		"returnByValue": true,
	})
	if err == nil {
		var evalRes struct {
			Result struct {
				Value any `json:"value"`
			} `json:"result"`
		}
		if jsonErr := json.Unmarshal(titleResult, &evalRes); jsonErr == nil {
			liveTitle := fmt.Sprintf("%v", evalRes.Result.Value)
			if strings.Contains(strings.ToLower(liveTitle), strings.ToLower(projectName)) {
				c.mu.Lock()
				c.currentWorkspaceName = projectName
				c.mu.Unlock()
				return true
			}
		}
	}

	return c.probeWorkspaceFolderPath(projectName, workspacePath)
}

func (c *Client) discoverAndConnectForWorkspaceImpl(workspacePath, projectName string) (bool, error) {
	c.mu.RLock()
	ports := make([]int, len(c.ports))
	copy(ports, c.ports)
	c.mu.RUnlock()

	var pages []TargetInfo
	var respondingPort *int

	for _, port := range ports {
		list, err := c.getJSON(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
		if err == nil {
			pages = append(pages, list...)
			hasWorkbench := false
			for _, t := range list {
				if strings.Contains(t.URL, "workbench") {
					hasWorkbench = true
					break
				}
			}
			if hasWorkbench && respondingPort == nil {
				p := port
				respondingPort = &p
			}
		}
	}

	if respondingPort == nil && len(pages) > 0 {
		p := ports[0]
		respondingPort = &p
	}

	if respondingPort == nil {
		return c.launchAndConnectWorkspace(workspacePath, projectName)
	}

	var workbenchPages []TargetInfo
	for _, t := range pages {
		if t.Type == "page" &&
			t.WebSocketDebuggerURL != "" &&
			!strings.Contains(t.Title, "Launchpad") &&
			!strings.Contains(t.URL, "workbench-jetski-agent") &&
			strings.Contains(t.URL, "workbench") {
			workbenchPages = append(workbenchPages, t)
		}
	}

	// 1. Title match (fast path)
	for _, t := range workbenchPages {
		if strings.Contains(t.Title, projectName) {
			return c.connectToPage(t, projectName)
		}
	}

	// 2. CDP probe
	if c.probeWorkbenchPages(workbenchPages, projectName, workspacePath) {
		return true, nil
	}

	// 3. Launch Antigravity
	return c.launchAndConnectWorkspace(workspacePath, projectName)
}

func (c *Client) connectToPage(page TargetInfo, projectName string) (bool, error) {
	c.mu.RLock()
	already := c.isConnectedFlag && c.targetURL == page.WebSocketDebuggerURL
	c.mu.RUnlock()

	if already {
		c.mu.Lock()
		c.currentWorkspaceName = projectName
		c.mu.Unlock()
		return true, nil
	}

	c.disconnectQuietly()
	c.mu.Lock()
	c.targetURL = page.WebSocketDebuggerURL
	c.mu.Unlock()

	if err := c.Connect(); err != nil {
		return false, err
	}

	c.mu.Lock()
	c.currentWorkspaceName = projectName
	c.mu.Unlock()
	return true, nil
}

func (c *Client) probeWorkbenchPages(workbenchPages []TargetInfo, projectName, workspacePath string) bool {
	for _, page := range workbenchPages {
		c.disconnectQuietly()
		c.mu.Lock()
		c.targetURL = page.WebSocketDebuggerURL
		c.mu.Unlock()

		if err := c.Connect(); err != nil {
			continue
		}

		res, err := c.Call("Runtime.evaluate", map[string]any{
			"expression":    "document.title",
			"returnByValue": true,
		})
		if err == nil {
			var evalRes struct {
				Result struct {
					Value any `json:"value"`
				} `json:"result"`
			}
			if jsonErr := json.Unmarshal(res, &evalRes); jsonErr == nil {
				liveTitle := strings.ToLower(fmt.Sprintf("%v", evalRes.Result.Value))
				if strings.Contains(liveTitle, strings.ToLower(projectName)) {
					c.mu.Lock()
					c.currentWorkspaceName = projectName
					c.mu.Unlock()
					return true
				}

				if strings.Contains(liveTitle, "untitled") && workspacePath != "" {
					if c.probeWorkspaceFolderPath(projectName, workspacePath) {
						return true
					}
				}
			}
		}
	}

	c.disconnectQuietly()
	return false
}

func (c *Client) probeWorkspaceFolderPath(projectName, workspacePath string) bool {
	expression := `(() => {
		// Method 1: Check window title data attribute
		const titleEl = document.querySelector('title');
		if (titleEl && titleEl.textContent) {
			const t = titleEl.textContent;
			if (t !== document.title) return { found: true, source: 'title-element', value: t };
		}
		
		// Method 2: Check folder name in explorer view
		const explorerItems = document.querySelectorAll('.explorer-item-label, .monaco-icon-label .label-name');
		const folderNames = Array.from(explorerItems).map(e => (e.textContent || '').trim()).filter(Boolean);
		if (folderNames.length > 0) return { found: true, source: 'explorer', value: folderNames.join(',') };
		
		// Method 3: Get path from tab titles or breadcrumbs
		const breadcrumbs = document.querySelectorAll('.breadcrumbs-view .folder-icon, .tabs-breadcrumbs .label-name');
		const crumbs = Array.from(breadcrumbs).map(e => (e.textContent || '').trim()).filter(Boolean);
		if (crumbs.length > 0) return { found: true, source: 'breadcrumbs', value: crumbs.join(',') };
		
		// Method 4: Check body data-uri attribute, etc.
		const bodyUri = document.body?.getAttribute('data-uri') || '';
		if (bodyUri) return { found: true, source: 'data-uri', value: bodyUri };
		
		return { found: false };
	})()`

	res, err := c.Call("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
	})
	if err == nil {
		var evalRes struct {
			Result struct {
				Value struct {
					Found  bool   `json:"found"`
					Source string `json:"source"`
					Value  string `json:"value"`
				} `json:"value"`
			} `json:"result"`
		}
		if jsonErr := json.Unmarshal(res, &evalRes); jsonErr == nil {
			v := evalRes.Result.Value
			if v.Found && v.Value != "" {
				normDetected := strings.ToLower(v.Value)
				normProject := strings.ToLower(projectName)
				normWorkspace := strings.ToLower(workspacePath)

				if strings.Contains(normDetected, normProject) || strings.Contains(normDetected, normWorkspace) {
					c.mu.Lock()
					c.currentWorkspaceName = projectName
					c.mu.Unlock()
					return true
				}
			}
		}
	}

	urlRes, err := c.Call("Runtime.evaluate", map[string]any{
		"expression":    "window.location.href",
		"returnByValue": true,
	})
	if err == nil {
		var evalRes struct {
			Result struct {
				Value any `json:"value"`
			} `json:"result"`
		}
		if jsonErr := json.Unmarshal(urlRes, &evalRes); jsonErr == nil {
			pageURL := strings.ToLower(fmt.Sprintf("%v", evalRes.Result.Value))
			normWorkspaceURI := strings.ToLower(url.QueryEscape(workspacePath))
			if strings.Contains(pageURL, normWorkspaceURI) || strings.Contains(pageURL, strings.ToLower(projectName)) {
				c.mu.Lock()
				c.currentWorkspaceName = projectName
				c.mu.Unlock()
				return true
			}
		}
	}

	return false
}

func (c *Client) launchAndConnectWorkspace(workspacePath, projectName string) (bool, error) {
	antigravityCli, err := c.getAntigravityCliFunc()
	if err != nil {
		return false, err
	}

	cdpPort, ok := c.findAvailablePortFunc()
	if !ok {
		c.mu.RLock()
		ports := fmt.Sprintf("%v", c.ports)
		c.mu.RUnlock()
		return false, fmt.Errorf("No available CDP ports. All candidate ports are in use: %s", ports)
	}

	launchArgs := []string{
		fmt.Sprintf("--remote-debugging-port=%d", cdpPort),
		"--new-window",
		workspacePath,
	}

	runErr := c.runCommand(antigravityCli, launchArgs)
	if runErr != nil {
		if c.getOS() == "darwin" {
			fallbackArgs := []string{
				"-a", "Antigravity",
				"--args",
				fmt.Sprintf("--remote-debugging-port=%d", cdpPort),
				workspacePath,
			}
			if fbErr := c.runCommand("open", fallbackArgs); fbErr != nil {
				return false, fbErr
			}
		} else {
			return false, runErr
		}
	}

	knownPageIds := make(map[string]struct{})
	c.mu.RLock()
	ports := make([]int, len(c.ports))
	copy(ports, c.ports)
	c.mu.RUnlock()

	for _, port := range ports {
		list, err := c.getJSON(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
		if err == nil {
			for _, p := range list {
				if p.ID != "" {
					knownPageIds[p.ID] = struct{}{}
				}
			}
		}
	}

	maxWait := 30 * time.Second
	pollInterval := c.launchPollInterval
	startTime := time.Now()

	for time.Since(startTime) < maxWait {
		time.Sleep(pollInterval)

		var pages []TargetInfo
		for _, port := range ports {
			list, err := c.getJSON(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
			if err == nil {
				pages = append(pages, list...)
			}
		}

		if len(pages) == 0 {
			continue
		}

		var workbenchPages []TargetInfo
		for _, t := range pages {
			if t.Type == "page" &&
				t.WebSocketDebuggerURL != "" &&
				!strings.Contains(t.Title, "Launchpad") &&
				!strings.Contains(t.URL, "workbench-jetski-agent") &&
				strings.Contains(t.URL, "workbench") {
				workbenchPages = append(workbenchPages, t)
			}
		}

		var titleMatch *TargetInfo
		for i := range workbenchPages {
			if strings.Contains(strings.ToLower(workbenchPages[i].Title), strings.ToLower(projectName)) {
				titleMatch = &workbenchPages[i]
				break
			}
		}
		if titleMatch != nil {
			return c.connectToPage(*titleMatch, projectName)
		}

		if c.probeWorkbenchPages(workbenchPages, projectName, workspacePath) {
			return true, nil
		}

		if time.Since(startTime) > 10*time.Second {
			var newUntitledPages []TargetInfo
			for _, t := range workbenchPages {
				if _, known := knownPageIds[t.ID]; !known && (strings.Contains(t.Title, "Untitled") || t.Title == "") {
					newUntitledPages = append(newUntitledPages, t)
				}
			}
			if len(newUntitledPages) == 1 {
				return c.connectToPage(newUntitledPages[0], projectName)
			}
		}
	}

	return false, fmt.Errorf("Workbench page for workspace %q not found within %d seconds", projectName, int(maxWait.Seconds()))
}

func (c *Client) runCommand(command string, args []string) error {
	if c.runCommandFunc != nil {
		return c.runCommandFunc(command, args)
	}
	return defaultRunCommand(command, args)
}

func defaultRunCommand(command string, args []string) error {
	// SECURITY: Direct execution without shell interpolation (exec.Command)
	cmd := exec.Command(command, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (c *Client) tryReconnect() {
	c.mu.Lock()
	if c.isReconnecting {
		c.mu.Unlock()
		return
	}
	c.isReconnecting = true
	c.reconnectAttemptCnt = 0
	maxAttempts := c.maxReconnectAttempts
	delay := c.reconnectDelay
	wsPath := c.currentWorkspacePath
	c.mu.Unlock()

	for {
		c.mu.Lock()
		if c.reconnectAttemptCnt >= maxAttempts {
			c.isReconnecting = false
			c.mu.Unlock()
			finalErr := fmt.Errorf("CDP reconnection failed %d times. Manual restart required.", maxAttempts)
			logger.Error(fmt.Sprintf("[CdpService] %s", finalErr.Error()))
			c.Emit("reconnectFailed", finalErr)
			return
		}
		c.reconnectAttemptCnt++
		attempt := c.reconnectAttemptCnt
		c.mu.Unlock()

		logger.Error(fmt.Sprintf("[CdpService] Reconnect attempt %d/%d...", attempt, maxAttempts))
		time.Sleep(delay)

		c.mu.Lock()
		c.contexts = nil
		c.mu.Unlock()

		var err error
		if wsPath != "" {
			_, err = c.DiscoverAndConnectForWorkspace(wsPath)
		} else {
			_, err = c.DiscoverTarget()
			if err == nil {
				err = c.Connect()
			}
		}

		if err == nil {
			logger.Error("[CdpService] Reconnect succeeded.")
			c.mu.Lock()
			c.reconnectAttemptCnt = 0
			c.isReconnecting = false
			c.mu.Unlock()
			c.Emit("reconnected", nil)
			return
		}

		logger.Error(fmt.Sprintf("[CdpService] Reconnect failed: %v", err))
	}
}

// WaitForCascadePanelReady waits until a context with 'cascade-panel' in its URL appears.
func (c *Client) WaitForCascadePanelReady(timeout time.Duration, pollInterval time.Duration) bool {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}

	start := time.Now()
	for time.Since(start) < timeout {
		c.mu.RLock()
		found := false
		for _, ctx := range c.contexts {
			if strings.Contains(ctx.URL, "cascade-panel") {
				found = true
				break
			}
		}
		c.mu.RUnlock()

		if found {
			return true
		}
		time.Sleep(pollInterval)
	}
	return false
}

// GetPrimaryContextID resolves the ID of the cascade-panel context, or extension context, or first available context.
func (c *Client) GetPrimaryContextID() *int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, ctx := range c.contexts {
		if strings.Contains(ctx.URL, "cascade-panel") {
			id := ctx.ID
			return &id
		}
	}

	for _, ctx := range c.contexts {
		if strings.Contains(ctx.Name, "Extension") {
			id := ctx.ID
			return &id
		}
	}

	if len(c.contexts) > 0 {
		id := c.contexts[0].ID
		return &id
	}

	return nil
}
