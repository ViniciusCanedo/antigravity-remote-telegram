package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"antigravity-telegram-remote/internal/logger"
)

type mockCdpCaller struct {
	connectErr   error
	connected    bool
	disconnectFn func() error
	callFn       func(method string, params any) (json.RawMessage, error)
}

func (m *mockCdpCaller) Connect() error {
	return m.connectErr
}

func (m *mockCdpCaller) Disconnect() error {
	if m.disconnectFn != nil {
		return m.disconnectFn()
	}
	m.connected = false
	return nil
}

func (m *mockCdpCaller) IsConnected() bool {
	return m.connected
}

func (m *mockCdpCaller) Call(method string, params any) (json.RawMessage, error) {
	if m.callFn != nil {
		return m.callFn(method, params)
	}
	return nil, nil
}

func TestDoctorSelectorChecks_List(t *testing.T) {
	if len(DoctorSelectorChecks) != 7 {
		t.Fatalf("expected 7 selector checks, got %d", len(DoctorSelectorChecks))
	}

	expectedNames := []string{
		"Chat Message (Text)",
		"Stop Button (State)",
		"Planning Active (Dialog)",
		"Process Logs (Activity)",
		"Combined Poll (Logic)",
		"Quota Error (Detection)",
		"DOM Observer (Event-driven)",
	}

	for i, name := range expectedNames {
		if DoctorSelectorChecks[i].Name != name {
			t.Errorf("selector %d name mismatch: expected %q, got %q", i, name, DoctorSelectorChecks[i].Name)
		}
		if DoctorSelectorChecks[i].Expression == "" {
			t.Errorf("selector %d expression is empty", i)
		}
	}
}

func TestRunDoctorSelectors_AllHealthy(t *testing.T) {
	disconnected := false
	mock := &mockCdpCaller{
		connected: true,
		disconnectFn: func() error {
			disconnected = true
			return nil
		},
		callFn: func(method string, params any) (json.RawMessage, error) {
			resp := map[string]any{
				"result": map[string]any{
					"value": map[string]any{
						"ok":   true,
						"data": map[string]any{"element": "present"},
					},
				},
			}
			return json.Marshal(resp)
		},
	}

	var outBuf bytes.Buffer
	var logBuf bytes.Buffer
	testLogger := logger.CreateLoggerWithWriters(logger.LevelDebug, logger.ConsoleWriters{
		InfoOut:  &logBuf,
		WarnOut:  &logBuf,
		ErrorOut: &logBuf,
		DebugOut: &logBuf,
	})

	report, err := RunDoctorSelectorsWithDeps(context.Background(), mock, testLogger, &outBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !disconnected {
		t.Error("expected client Disconnect to be called")
	}

	if report.HealthCount != 7 || report.TotalCount != 7 {
		t.Fatalf("expected 7/7, got %d/%d", report.HealthCount, report.TotalCount)
	}

	logOutput := logBuf.String()
	consoleOutput := outBuf.String()

	if !strings.Contains(consoleOutput, "✅ Healthy: Found and extracted data.") {
		t.Error("console output missing healthy message")
	}
	if !strings.Contains(consoleOutput, "Data: {\"element\":\"present\"}") {
		t.Error("console output missing object data serialization")
	}
	if !strings.Contains(logOutput, "✨ Score: 7/7 Healthy") {
		t.Errorf("log output missing score 7/7: %s", logOutput)
	}
	if !strings.Contains(logOutput, "All core selectors are healthy") {
		t.Error("log output missing all healthy summary message")
	}
}

func TestRunDoctorSelectors_MixedHealth(t *testing.T) {
	callIdx := 0
	mock := &mockCdpCaller{
		connected: true,
		callFn: func(method string, params any) (json.RawMessage, error) {
			callIdx++
			switch callIdx {
			case 1:
				// Healthy string data
				resp := map[string]any{
					"result": map[string]any{
						"value": map[string]any{
							"ok":   true,
							"data": "some message text",
						},
					},
				}
				return json.Marshal(resp)
			case 2:
				// Warning (data is null)
				resp := map[string]any{
					"result": map[string]any{
						"value": map[string]any{
							"ok":   true,
							"data": nil,
						},
					},
				}
				return json.Marshal(resp)
			case 3:
				// Broken (ok is false)
				resp := map[string]any{
					"result": map[string]any{
						"value": map[string]any{
							"ok":    false,
							"error": "SyntaxError: Unexpected token",
						},
					},
				}
				return json.Marshal(resp)
			case 4:
				// CDP call network failure
				return nil, errors.New("timeout waiting for CDP response")
			default:
				// Remaining are healthy slices
				resp := map[string]any{
					"result": map[string]any{
						"value": map[string]any{
							"ok":   true,
							"data": []any{"item1"},
						},
					},
				}
				return json.Marshal(resp)
			}
		},
	}

	var outBuf bytes.Buffer
	var logBuf bytes.Buffer
	testLogger := logger.CreateLoggerWithWriters(logger.LevelDebug, logger.ConsoleWriters{
		InfoOut:  &logBuf,
		WarnOut:  &logBuf,
		ErrorOut: &logBuf,
		DebugOut: &logBuf,
	})

	report, err := RunDoctorSelectorsWithDeps(context.Background(), mock, testLogger, &outBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Healthy calls: #1 and #5, #6, #7 = 4 healthy
	if report.HealthCount != 4 || report.TotalCount != 7 {
		t.Fatalf("expected 4/7, got %d/%d", report.HealthCount, report.TotalCount)
	}

	consoleOutput := outBuf.String()
	logOutput := logBuf.String()

	if !strings.Contains(consoleOutput, "⚠️  Warning: Found but returned null/undefined.") {
		t.Error("console output missing warning message")
	}
	if !strings.Contains(consoleOutput, "❌ Broken: SyntaxError: Unexpected token") {
		t.Error("console output missing broken error message")
	}
	if !strings.Contains(consoleOutput, "❌ CDP Call Error: timeout waiting for CDP response") {
		t.Error("console output missing CDP call error message")
	}
	if !strings.Contains(logOutput, "⚠️ Score: 4/7 Healthy") {
		t.Errorf("log output missing score 4/7: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Some selectors failed. Compatibility might be compromised.") {
		t.Error("log output missing partial failure warning message")
	}
}

func TestRunDoctorSelectors_AllFailed(t *testing.T) {
	mock := &mockCdpCaller{
		connected: true,
		callFn: func(method string, params any) (json.RawMessage, error) {
			resp := map[string]any{
				"result": map[string]any{
					"value": map[string]any{
						"ok":    false,
						"error": "Element not found",
					},
				},
			}
			return json.Marshal(resp)
		},
	}

	var outBuf bytes.Buffer
	var logBuf bytes.Buffer
	testLogger := logger.CreateLoggerWithWriters(logger.LevelDebug, logger.ConsoleWriters{
		InfoOut:  &logBuf,
		WarnOut:  &logBuf,
		ErrorOut: &logBuf,
		DebugOut: &logBuf,
	})

	report, err := RunDoctorSelectorsWithDeps(context.Background(), mock, testLogger, &outBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.HealthCount != 0 || report.TotalCount != 7 {
		t.Fatalf("expected 0/7, got %d/%d", report.HealthCount, report.TotalCount)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "💀 All selectors failed! The UI has likely changed significantly.") {
		t.Errorf("log output missing all failed message: %s", logOutput)
	}
}

func TestRunDoctorSelectors_ConnectError(t *testing.T) {
	mock := &mockCdpCaller{
		connectErr: errors.New("connection refused on port 9222"),
		connected:  false,
	}

	var logBuf bytes.Buffer
	testLogger := logger.CreateLoggerWithWriters(logger.LevelDebug, logger.ConsoleWriters{
		ErrorOut: &logBuf,
	})

	report, err := RunDoctorSelectorsWithDeps(context.Background(), mock, testLogger, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if report != nil {
		t.Errorf("expected nil report, got %v", report)
	}
	if !strings.Contains(logBuf.String(), "❌ Could not connect to any Antigravity tab: connection refused on port 9222") {
		t.Errorf("unexpected error log: %s", logBuf.String())
	}
}

func TestRunDoctorSelectors_NotConnected(t *testing.T) {
	mock := &mockCdpCaller{
		connectErr: nil,
		connected:  false,
	}

	var logBuf bytes.Buffer
	testLogger := logger.CreateLoggerWithWriters(logger.LevelDebug, logger.ConsoleWriters{
		ErrorOut: &logBuf,
	})

	report, err := RunDoctorSelectorsWithDeps(context.Background(), mock, testLogger, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if report != nil {
		t.Errorf("expected nil report, got %v", report)
	}
	if !strings.Contains(logBuf.String(), "❌ Connection failed unexpectedly.") {
		t.Errorf("unexpected error log: %s", logBuf.String())
	}
}

func TestRunDoctorSelectors_NilClient(t *testing.T) {
	report, err := RunDoctorSelectorsWithDeps(context.Background(), nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if report != nil {
		t.Errorf("expected nil report, got %v", report)
	}
}
