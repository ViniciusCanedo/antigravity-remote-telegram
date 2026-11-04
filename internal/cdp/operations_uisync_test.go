package cdp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type uiSyncServer struct {
	server           *httptest.Server
	wsURL            string
	port             int
	mu               sync.Mutex
	receivedMessages []recordedMessage
	contexts         []Context
	evalResponder    func(req recordedMessage) (any, string) // (value, errMsg)
}

func newUiSyncServer(t *testing.T) *uiSyncServer {
	t.Helper()
	uss := &uiSyncServer{
		contexts: []Context{
			{ID: 2, Name: "cascade", URL: "file:///workbench/cascade-panel.html"},
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []TargetInfo{
				{
					Type:                 "page",
					Title:                "Antigravity Workspace",
					URL:                  "file:///workbench/index.html",
					WebSocketDebuggerURL: uss.wsURL,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
			return
		}

		if r.URL.Path == "/devtools/page/test-id" {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer conn.CloseNow()

			ctx := r.Context()
			for {
				_, data, err := conn.Read(ctx)
				if err != nil {
					return
				}

				var req recordedMessage
				if err := json.Unmarshal(data, &req); err != nil {
					continue
				}

				uss.mu.Lock()
				uss.receivedMessages = append(uss.receivedMessages, req)
				evalFunc := uss.evalResponder
				currContexts := uss.contexts
				uss.mu.Unlock()

				switch req.Method {
				case "Runtime.enable":
					for _, c := range currContexts {
						evt := map[string]any{
							"method": "Runtime.executionContextCreated",
							"params": map[string]any{
								"context": c,
							},
						}
						evtBytes, _ := json.Marshal(evt)
						_ = conn.Write(ctx, websocket.MessageText, evtBytes)
					}
					resp := map[string]any{"id": req.ID, "result": map[string]any{}}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "Network.enable", "Runtime.addBinding":
					resp := map[string]any{"id": req.ID, "result": map[string]any{}}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "Runtime.evaluate":
					if evalFunc != nil {
						val, errMsg := evalFunc(req)
						if errMsg != "" {
							errResp := map[string]any{
								"id": req.ID,
								"error": map[string]any{
									"code":    -32000,
									"message": errMsg,
								},
							}
							respBytes, _ := json.Marshal(errResp)
							_ = conn.Write(ctx, websocket.MessageText, respBytes)
							continue
						}
						resp := map[string]any{
							"id": req.ID,
							"result": map[string]any{
								"result": map[string]any{
									"value": val,
								},
							},
						}
						respBytes, _ := json.Marshal(resp)
						_ = conn.Write(ctx, websocket.MessageText, respBytes)
					} else {
						resp := map[string]any{
							"id": req.ID,
							"result": map[string]any{
								"result": map[string]any{
									"value": map[string]any{"ok": true},
								},
							},
						}
						respBytes, _ := json.Marshal(resp)
						_ = conn.Write(ctx, websocket.MessageText, respBytes)
					}
				}
			}
		}

		http.NotFound(w, r)
	})

	server := httptest.NewServer(handler)
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	uss.server = server
	uss.port = port
	uss.wsURL = "ws://" + u.Host + "/devtools/page/test-id"

	return uss
}

func (uss *uiSyncServer) close() {
	uss.server.Close()
}

func (uss *uiSyncServer) setEvalResponder(fn func(req recordedMessage) (any, string)) {
	uss.mu.Lock()
	defer uss.mu.Unlock()
	uss.evalResponder = fn
}

func (uss *uiSyncServer) setContexts(contexts []Context) {
	uss.mu.Lock()
	defer uss.mu.Unlock()
	uss.contexts = contexts
}

func (uss *uiSyncServer) getReceivedMessages() []recordedMessage {
	uss.mu.Lock()
	defer uss.mu.Unlock()
	copied := make([]recordedMessage, len(uss.receivedMessages))
	copy(copied, uss.receivedMessages)
	return copied
}

// ========== setUiMode tests ==========

func TestClient_SetUIMode_ErrorWhenDisconnected(t *testing.T) {
	zero := 0
	client := NewClient(ClientOptions{
		PortsToScan:          []int{19999},
		MaxReconnectAttempts: &zero,
	})

	_, err := client.SetUIMode("plan")
	if err == nil {
		t.Fatalf("expected error when SetUIMode called while disconnected, got nil")
	}
	if !strings.Contains(err.Error(), "Not connected to CDP") {
		t.Errorf("expected error containing 'Not connected to CDP', got %v", err)
	}
}

func TestClient_SetUIMode_ExecutesEvaluateWhenConnected(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": true, "mode": "Planning"}, ""
	})

	result, err := client.SetUIMode("plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Errorf("expected OK == true, got false, error: %s", result.Error)
	}

	messages := srv.getReceivedMessages()
	var evalCall *recordedMessage
	for _, m := range messages {
		if m.Method == "Runtime.evaluate" {
			evalCall = &m
			break
		}
	}
	if evalCall == nil {
		t.Fatalf("expected Runtime.evaluate call")
	}

	expr, _ := evalCall.Params["expression"].(string)
	if !strings.Contains(expr, "plan") {
		t.Errorf("expected expression to contain 'plan', got %s", expr)
	}
	rbv, _ := evalCall.Params["returnByValue"].(bool)
	if !rbv {
		t.Errorf("expected returnByValue to be true")
	}
	awp, _ := evalCall.Params["awaitPromise"].(bool)
	if !awp {
		t.Errorf("expected awaitPromise to be true")
	}
}

func TestClient_SetUIMode_MapsInternalNamesToUIDisplayNames(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": true, "mode": "Planning"}, ""
	})

	result, err := client.SetUIMode("plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := srv.getReceivedMessages()
	var evalCall *recordedMessage
	for _, m := range messages {
		if m.Method == "Runtime.evaluate" {
			evalCall = &m
			break
		}
	}
	if evalCall == nil {
		t.Fatalf("expected Runtime.evaluate call")
	}

	expr, _ := evalCall.Params["expression"].(string)
	if !strings.Contains(expr, "Planning") {
		t.Errorf("expected expression to contain 'Planning', got %s", expr)
	}
	if !strings.Contains(expr, "Fast") {
		t.Errorf("expected expression to contain 'Fast', got %s", expr)
	}

	if !result.OK {
		t.Errorf("expected OK == true")
	}
	if result.Mode != "Planning" {
		t.Errorf("expected Mode == 'Planning', got %q", result.Mode)
	}
}

func TestClient_SetUIMode_ReturnsFastModeName(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": true, "mode": "Fast"}, ""
	})

	result, err := client.SetUIMode("fast")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.OK {
		t.Errorf("expected OK == true")
	}
	if result.Mode != "Fast" {
		t.Errorf("expected Mode == 'Fast', got %q", result.Mode)
	}
}

func TestClient_SetUIMode_UsesDialogSelectors(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": true, "mode": "Planning"}, ""
	})

	_, err := client.SetUIMode("plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := srv.getReceivedMessages()
	var evalCall *recordedMessage
	for _, m := range messages {
		if m.Method == "Runtime.evaluate" {
			evalCall = &m
			break
		}
	}
	if evalCall == nil {
		t.Fatalf("expected Runtime.evaluate call")
	}

	expr, _ := evalCall.Params["expression"].(string)
	if !strings.Contains(expr, `role=\"dialog\"`) {
		t.Errorf("expected expression to contain 'role=\\\"dialog\\\"', got %s", expr)
	}
	if !strings.Contains(expr, ".font-medium") {
		t.Errorf("expected expression to contain '.font-medium', got %s", expr)
	}
	if !strings.Contains(expr, "cursor-pointer") {
		t.Errorf("expected expression to contain 'cursor-pointer', got %s", expr)
	}
}

func TestClient_SetUIMode_ReturnsFalseWhenDOMNotFound(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": false, "error": "Mode toggle button not found"}, ""
	})

	result, err := client.SetUIMode("plan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OK {
		t.Errorf("expected OK == false, got true")
	}
	if result.Error == "" {
		t.Errorf("expected non-empty Error")
	}
}

func TestClient_SetUIMode_HandlesCDPErrorGracefully(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return nil, "CDP通信エラー"
	})

	result, err := client.SetUIMode("plan")
	if err != nil {
		t.Fatalf("expected err == nil when CDP error is handled gracefully, got %v", err)
	}

	if result.OK {
		t.Errorf("expected OK == false")
	}
	if !strings.Contains(result.Error, "CDP通信エラー") {
		t.Errorf("expected Error to contain 'CDP通信エラー', got %q", result.Error)
	}
}

// ========== setUiModel tests ==========

func TestClient_SetUIModel_ErrorWhenDisconnected(t *testing.T) {
	zero := 0
	client := NewClient(ClientOptions{
		PortsToScan:          []int{19999},
		MaxReconnectAttempts: &zero,
	})

	_, err := client.SetUIModel("gpt-4o")
	if err == nil {
		t.Fatalf("expected error when SetUIModel called while disconnected, got nil")
	}
	if !strings.Contains(err.Error(), "Not connected to CDP") {
		t.Errorf("expected error containing 'Not connected to CDP', got %v", err)
	}
}

func TestClient_SetUIModel_ExecutesEvaluateWhenConnected(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": true, "model": "gpt-4o"}, ""
	})

	result, err := client.SetUIModel("gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.OK {
		t.Errorf("expected OK == true")
	}

	messages := srv.getReceivedMessages()
	var evalCall *recordedMessage
	for _, m := range messages {
		if m.Method == "Runtime.evaluate" {
			evalCall = &m
			break
		}
	}
	if evalCall == nil {
		t.Fatalf("expected Runtime.evaluate call")
	}

	expr, _ := evalCall.Params["expression"].(string)
	if !strings.Contains(expr, "gpt-4o") {
		t.Errorf("expected expression to contain 'gpt-4o', got %s", expr)
	}
	rbv, _ := evalCall.Params["returnByValue"].(bool)
	if !rbv {
		t.Errorf("expected returnByValue to be true")
	}
	awp, _ := evalCall.Params["awaitPromise"].(bool)
	if !awp {
		t.Errorf("expected awaitPromise to be true")
	}
}

func TestClient_SetUIModel_ReturnsModelName(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": true, "model": "claude-3-opus"}, ""
	})

	result, err := client.SetUIModel("claude-3-opus")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.OK {
		t.Errorf("expected OK == true")
	}
	if result.Model != "claude-3-opus" {
		t.Errorf("expected Model == 'claude-3-opus', got %q", result.Model)
	}
}

func TestClient_SetUIModel_ReturnsFalseWhenDOMNotFound(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": false, "error": "モデルセレクターが見つかりませんでした"}, ""
	})

	result, err := client.SetUIModel("gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OK {
		t.Errorf("expected OK == false")
	}
	if result.Error == "" {
		t.Errorf("expected non-empty Error")
	}
}

func TestClient_SetUIModel_HandlesCDPErrorGracefully(t *testing.T) {
	srv := newUiSyncServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return nil, "タイムアウト"
	})

	result, err := client.SetUIModel("gpt-4o")
	if err != nil {
		t.Fatalf("expected err == nil when CDP error is handled gracefully, got %v", err)
	}

	if result.OK {
		t.Errorf("expected OK == false")
	}
	if !strings.Contains(result.Error, "タイムアウト") {
		t.Errorf("expected Error to contain 'タイムアウト', got %q", result.Error)
	}
}

func TestClient_SetUIModel_UsesPrimaryContextID(t *testing.T) {
	srv := newUiSyncServer(t)
	srv.setContexts([]Context{
		{ID: 42, Name: "cascade-panel", URL: "vscode-webview://cascade-panel"},
	})
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) (any, string) {
		return map[string]any{"ok": true, "model": "gemini-1.5-pro"}, ""
	})

	_, err := client.SetUIModel("gemini-1.5-pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	messages := srv.getReceivedMessages()
	var evalCall *recordedMessage
	for _, m := range messages {
		if m.Method == "Runtime.evaluate" {
			evalCall = &m
			break
		}
	}
	if evalCall == nil {
		t.Fatalf("expected Runtime.evaluate call")
	}

	cid, ok := evalCall.Params["contextId"].(float64)
	if !ok || int(cid) != 42 {
		t.Errorf("expected contextId == 42, got %v", evalCall.Params["contextId"])
	}
}
