package monitor

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

// mockCdpClient provides a deterministic mock of CdpClient for unit testing.
type mockCdpClient struct {
	callFunc func(ctx context.Context, method string, params map[string]any) (map[string]any, error)
	onFunc   func(event string, listener func(params map[string]any))
	offFunc  func(event string, listener func(params map[string]any))
	ctxID    *int
	calls    []struct {
		Method string
		Params map[string]any
	}
}

func (m *mockCdpClient) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	m.calls = append(m.calls, struct {
		Method string
		Params map[string]any
	}{Method: method, Params: params})
	if m.callFunc != nil {
		return m.callFunc(ctx, method, params)
	}
	return map[string]any{"result": map[string]any{"value": nil}}, nil
}

func (m *mockCdpClient) GetPrimaryContextID() *int {
	return m.ctxID
}

func (m *mockCdpClient) On(event string, listener func(params map[string]any)) {
	if m.onFunc != nil {
		m.onFunc(event, listener)
	}
}

func (m *mockCdpClient) Off(event string, listener func(params map[string]any)) {
	if m.offFunc != nil {
		m.offFunc(event, listener)
	}
}

func TestStopButtonSelector_DetectsAndClicksFixedPositionAriaLabel(t *testing.T) {
	// Verify script does not check offsetParent (which fails on fixed-position elements)
	if strings.Contains(ResponseSelectors.StopButton, "offsetParent") {
		t.Errorf("StopButton script should not check offsetParent")
	}
	if strings.Contains(ResponseSelectors.ClickStopButton, "offsetParent") {
		t.Errorf("ClickStopButton script should not check offsetParent")
	}

	// Verify aria-label support in STOP_PATTERNS
	stopPatterns := []string{
		"^stop$",
		"^stop generating$",
		"^stop response$",
		"^停止$",
		"^生成を停止$",
		"^応答を停止$",
	}
	for _, p := range stopPatterns {
		if !strings.Contains(ResponseSelectors.StopButton, p) {
			t.Errorf("expected StopButton to contain regex pattern %s", p)
		}
		if !strings.Contains(ResponseSelectors.ClickStopButton, p) {
			t.Errorf("expected ClickStopButton to contain regex pattern %s", p)
		}
	}

	// Verify ClickStopButton with text-fallback result
	mockClient := &mockCdpClient{
		callFunc: func(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
			return map[string]any{
				"result": map[string]any{
					"value": map[string]any{
						"ok":     true,
						"method": "text-fallback",
					},
				},
			}, nil
		},
	}

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService: mockClient,
	})

	res := monitor.ClickStopButton(context.Background())
	if !res.Ok {
		t.Errorf("expected Ok to be true, got false")
	}
	if res.Method != "text-fallback" {
		t.Errorf("expected method to be 'text-fallback', got '%s'", res.Method)
	}
}

func TestStopButtonSelector_DetectsCancelTooltipId(t *testing.T) {
	// Verify tooltip-id presence
	tooltipID := "input-send-button-cancel-tooltip"
	if !strings.Contains(ResponseSelectors.StopButton, tooltipID) {
		t.Errorf("expected StopButton to contain tooltip-id: %s", tooltipID)
	}
	if !strings.Contains(ResponseSelectors.ClickStopButton, tooltipID) {
		t.Errorf("expected ClickStopButton to contain tooltip-id: %s", tooltipID)
	}

	// Verify ClickStopButton with tooltip-id result
	mockClient := &mockCdpClient{
		callFunc: func(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
			return map[string]any{
				"result": map[string]any{
					"value": map[string]any{
						"ok":     true,
						"method": "tooltip-id",
					},
				},
			}, nil
		},
	}

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService: mockClient,
	})

	res := monitor.ClickStopButton(context.Background())
	if !res.Ok {
		t.Errorf("expected Ok to be true, got false")
	}
	if res.Method != "tooltip-id" {
		t.Errorf("expected method to be 'tooltip-id', got '%s'", res.Method)
	}
}

func TestStopButtonSelector_IgnoresNonStopButtons(t *testing.T) {
	// Neither script should contain SVG, rect, or microphone logic
	for _, scriptName := range []string{"StopButton", "ClickStopButton"} {
		var script string
		if scriptName == "StopButton" {
			script = ResponseSelectors.StopButton
		} else {
			script = ResponseSelectors.ClickStopButton
		}
		lower := strings.ToLower(script)
		if strings.Contains(lower, "svg") {
			t.Errorf("%s script should not contain svg", scriptName)
		}
		if strings.Contains(lower, "rect") {
			t.Errorf("%s script should not contain rect", scriptName)
		}
		if strings.Contains(lower, "path") {
			t.Errorf("%s script should not contain path", scriptName)
		}
		if strings.Contains(lower, "microphone") {
			t.Errorf("%s script should not contain microphone", scriptName)
		}
	}

	// Verify that non-stop button labels do not match STOP_PATTERNS
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^stop$`),
		regexp.MustCompile(`(?i)^stop generating$`),
		regexp.MustCompile(`(?i)^stop response$`),
		regexp.MustCompile(`^停止$`),
		regexp.MustCompile(`^生成を停止$`),
		regexp.MustCompile(`^応答を停止$`),
	}

	nonStopLabels := []string{
		"Send",
		"Send message",
		"Microphone",
		"Attach file",
		"Submit",
		"Settings",
		"Clear",
		"Red square",
	}

	for _, label := range nonStopLabels {
		for _, re := range patterns {
			if re.MatchString(strings.TrimSpace(label)) {
				t.Errorf("non-stop label '%s' should not match pattern %s", label, re.String())
			}
		}
	}

	// Verify ClickStopButton returning failure when stop button is not found
	mockClient := &mockCdpClient{
		callFunc: func(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
			return map[string]any{
				"result": map[string]any{
					"value": map[string]any{
						"ok":    false,
						"error": "Stop button not found",
					},
				},
			}, nil
		},
	}

	monitor := NewResponseMonitor(ResponseMonitorOptions{
		CdpService: mockClient,
	})

	res := monitor.ClickStopButton(context.Background())
	if res.Ok {
		t.Errorf("expected Ok to be false, got true")
	}
	if res.Error != "Stop button not found" {
		t.Errorf("expected Error to be 'Stop button not found', got '%s'", res.Error)
	}
}
