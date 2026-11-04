package cdp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type recordedMessage struct {
	ID     int64          `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

type injectionServer struct {
	server           *httptest.Server
	wsURL            string
	port             int
	mu               sync.Mutex
	receivedMessages []recordedMessage
	evalResponder    func(req recordedMessage) any
}

func newInjectionServer(t *testing.T) *injectionServer {
	t.Helper()
	is := &injectionServer{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []TargetInfo{
				{
					Type:                 "page",
					Title:                "Antigravity Workspace",
					URL:                  "file:///workbench/index.html",
					WebSocketDebuggerURL: is.wsURL,
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

				is.mu.Lock()
				is.receivedMessages = append(is.receivedMessages, req)
				evalFunc := is.evalResponder
				is.mu.Unlock()

				switch req.Method {
				case "Runtime.enable":
					contexts := []Context{
						{ID: 1, Name: "top", URL: "file:///workbench/index.html"},
						{ID: 2, Name: "cascade", URL: "file:///workbench/cascade-panel.html"},
						{ID: 3, Name: "Extension", URL: "file:///workbench/extension.html"},
					}
					for _, c := range contexts {
						evt := map[string]any{
							"method": "Runtime.executionContextCreated",
							"params": map[string]any{
								"context": c,
							},
						}
						evtBytes, _ := json.Marshal(evt)
						_ = conn.Write(ctx, websocket.MessageText, evtBytes)
					}
					resp := map[string]any{
						"id":     req.ID,
						"result": map[string]any{},
					}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "Network.enable", "Runtime.addBinding":
					resp := map[string]any{
						"id":     req.ID,
						"result": map[string]any{},
					}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)

				case "Runtime.evaluate":
					var val any
					if evalFunc != nil {
						val = evalFunc(req)
					} else {
						val = map[string]any{"ok": false, "error": "No responder configured"}
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

				case "Input.insertText", "Input.dispatchKeyEvent":
					resp := map[string]any{
						"id":     req.ID,
						"result": map[string]any{},
					}
					respBytes, _ := json.Marshal(resp)
					_ = conn.Write(ctx, websocket.MessageText, respBytes)
				}
			}
		}

		http.NotFound(w, r)
	})

	server := httptest.NewServer(handler)
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	is.server = server
	is.port = port
	is.wsURL = "ws://" + u.Host + "/devtools/page/test-id"

	return is
}

func (is *injectionServer) close() {
	is.server.Close()
}

func (is *injectionServer) setEvalResponder(fn func(req recordedMessage) any) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.evalResponder = fn
}

func (is *injectionServer) getReceivedMessages() []recordedMessage {
	is.mu.Lock()
	defer is.mu.Unlock()
	copied := make([]recordedMessage, len(is.receivedMessages))
	copy(copied, is.receivedMessages)
	return copied
}

func (is *injectionServer) clearReceivedMessages() {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.receivedMessages = nil
}

func newTestClient(port int) *Client {
	zero := 0
	return NewClient(ClientOptions{
		PortsToScan:          []int{port},
		MaxReconnectAttempts: &zero,
		CallTimeout:          2 * time.Second,
	})
}

func TestClient_InjectMessage_SucceedsInCascadePanelContext(t *testing.T) {
	srv := newInjectionServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) any {
		if cid, ok := req.Params["contextId"].(float64); ok && int(cid) == 2 {
			return map[string]any{"ok": true, "method": "focus"}
		}
		return map[string]any{"ok": false, "error": "No editor found"}
	})

	result, err := client.InjectMessage("テストメッセージ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.OK {
		t.Errorf("expected OK == true, got false, error: %s", result.Error)
	}
	if result.ContextID == nil || *result.ContextID != 2 {
		t.Errorf("expected ContextID == 2, got %v", result.ContextID)
	}
	if result.Method != "enter" {
		t.Errorf("expected Method == 'enter', got %q", result.Method)
	}
}

func TestClient_InjectMessage_FallsBackToAnotherContext(t *testing.T) {
	srv := newInjectionServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) any {
		if cid, ok := req.Params["contextId"].(float64); ok && int(cid) == 3 {
			return map[string]any{"ok": true, "method": "focus"}
		}
		return map[string]any{"ok": false, "error": "No editor found"}
	})

	result, err := client.InjectMessage("フォールバックテスト")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.OK {
		t.Errorf("expected OK == true, got false, error: %s", result.Error)
	}
	if result.ContextID == nil || *result.ContextID != 3 {
		t.Errorf("expected ContextID == 3, got %v", result.ContextID)
	}
}

func TestClient_InjectMessage_ReturnsFalseWhenAllContextsFail(t *testing.T) {
	srv := newInjectionServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)

	srv.setEvalResponder(func(req recordedMessage) any {
		return map[string]any{"ok": false, "error": "No editor found"}
	})

	result, err := client.InjectMessage("失敗するメッセージ")
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

func TestClient_InjectMessage_DispatchesExactCDPSequence(t *testing.T) {
	srv := newInjectionServer(t)
	defer srv.close()

	client := newTestClient(srv.port)
	if err := client.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect() }()

	time.Sleep(100 * time.Millisecond)
	srv.clearReceivedMessages()

	targetText := `注入テキスト<script>alert("xss")</script>`
	srv.setEvalResponder(func(req recordedMessage) any {
		if cid, ok := req.Params["contextId"].(float64); ok && int(cid) == 2 {
			return map[string]any{"ok": true, "method": "focus"}
		}
		return map[string]any{"ok": false, "error": "No editor found"}
	})

	result, err := client.InjectMessage(targetText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK {
		t.Fatalf("inject failed: %s", result.Error)
	}

	messages := srv.getReceivedMessages()

	// Verify that Runtime.evaluate for focus was called
	var evalCalls []recordedMessage
	for _, m := range messages {
		if m.Method == "Runtime.evaluate" {
			evalCalls = append(evalCalls, m)
		}
	}
	if len(evalCalls) == 0 {
		t.Fatalf("expected at least one Runtime.evaluate call")
	}

	firstCall := evalCalls[0]
	expr, _ := firstCall.Params["expression"].(string)
	if !strings.Contains(expr, "editor.focus()") {
		t.Errorf("expected focus expression to contain 'editor.focus()', got %s", expr)
	}
	rbv, _ := firstCall.Params["returnByValue"].(bool)
	if !rbv {
		t.Errorf("expected returnByValue to be true")
	}

	// Verify that text is sent via Input.insertText
	var insertTextCalls []recordedMessage
	for _, m := range messages {
		if m.Method == "Input.insertText" {
			insertTextCalls = append(insertTextCalls, m)
		}
	}
	if len(insertTextCalls) != 1 {
		t.Fatalf("expected 1 Input.insertText call, got %d", len(insertTextCalls))
	}
	txt, _ := insertTextCalls[0].Params["text"].(string)
	if txt != targetText {
		t.Errorf("expected text %q, got %q", targetText, txt)
	}

	// Verify that key events are dispatched:
	//   clearInputField: Meta+A (keyDown/keyUp) + Backspace (keyDown/keyUp) = 4 events
	//   pressEnterToSend: Enter (keyDown/keyUp) = 2 events
	var keyCalls []recordedMessage
	for _, m := range messages {
		if m.Method == "Input.dispatchKeyEvent" {
			keyCalls = append(keyCalls, m)
		}
	}
	if len(keyCalls) != 6 {
		t.Fatalf("expected 6 Input.dispatchKeyEvent calls, got %d", len(keyCalls))
	}

	expectedMod := 2
	if runtime.GOOS == "darwin" {
		expectedMod = 4
	}

	// clearInputField: Meta+A select all
	if k, _ := keyCalls[0].Params["key"].(string); k != "a" {
		t.Errorf("keyCalls[0] key expected 'a', got %v", k)
	}
	if m, _ := keyCalls[0].Params["modifiers"].(float64); int(m) != expectedMod {
		t.Errorf("keyCalls[0] modifiers expected %d, got %v", expectedMod, m)
	}
	if k, _ := keyCalls[1].Params["key"].(string); k != "a" {
		t.Errorf("keyCalls[1] key expected 'a', got %v", k)
	}

	// clearInputField: Backspace delete
	if k, _ := keyCalls[2].Params["key"].(string); k != "Backspace" {
		t.Errorf("keyCalls[2] key expected 'Backspace', got %v", k)
	}
	if k, _ := keyCalls[3].Params["key"].(string); k != "Backspace" {
		t.Errorf("keyCalls[3] key expected 'Backspace', got %v", k)
	}

	// pressEnterToSend: Enter
	if k, _ := keyCalls[4].Params["key"].(string); k != "Enter" {
		t.Errorf("keyCalls[4] key expected 'Enter', got %v", k)
	}
	if typ, _ := keyCalls[4].Params["type"].(string); typ != "keyDown" {
		t.Errorf("keyCalls[4] type expected 'keyDown', got %v", typ)
	}
	if k, _ := keyCalls[5].Params["key"].(string); k != "Enter" {
		t.Errorf("keyCalls[5] key expected 'Enter', got %v", k)
	}
	if typ, _ := keyCalls[5].Params["type"].(string); typ != "keyUp" {
		t.Errorf("keyCalls[5] type expected 'keyUp', got %v", typ)
	}
}

func TestClient_InjectMessage_ErrorWhenDisconnected(t *testing.T) {
	zero := 0
	client := NewClient(ClientOptions{
		PortsToScan:          []int{19999},
		MaxReconnectAttempts: &zero,
	})

	_, err := client.InjectMessage("test")
	if err == nil {
		t.Fatalf("expected error when injectMessage called while disconnected, got nil")
	}
	if !strings.Contains(err.Error(), "Not connected to CDP") {
		t.Errorf("expected error containing 'Not connected to CDP', got %v", err)
	}
}
