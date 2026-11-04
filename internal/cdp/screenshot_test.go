package cdp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type mockCDPCaller struct {
	callFunc   func(method string, params any) (json.RawMessage, error)
	lastMethod string
	lastParams any
}

func (m *mockCDPCaller) Call(method string, params any) (json.RawMessage, error) {
	m.lastMethod = method
	m.lastParams = params
	if m.callFunc != nil {
		return m.callFunc(method, params)
	}
	return nil, nil
}

const dummyBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

func TestScreenshotService_Capture_ReturnsDecodedBuffer(t *testing.T) {
	mockCDP := &mockCDPCaller{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return json.Marshal(map[string]string{"data": dummyBase64})
		},
	}

	service := NewScreenshotService(ScreenshotServiceOptions{CDPService: mockCDP})
	result := service.Capture()

	if !result.Success {
		t.Fatalf("expected success == true, got false, error: %s", result.Error)
	}
	if len(result.Buffer) == 0 {
		t.Errorf("expected non-empty buffer")
	}
}

func TestScreenshotService_Capture_DefaultPNGFormat(t *testing.T) {
	mockCDP := &mockCDPCaller{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return json.Marshal(map[string]string{"data": dummyBase64})
		},
	}

	service := NewScreenshotService(ScreenshotServiceOptions{CDPService: mockCDP})
	service.Capture()

	if mockCDP.lastMethod != "Page.captureScreenshot" {
		t.Errorf("expected method 'Page.captureScreenshot', got %q", mockCDP.lastMethod)
	}
	params, ok := mockCDP.lastParams.(map[string]any)
	if !ok {
		t.Fatalf("expected params map[string]any, got %T", mockCDP.lastParams)
	}
	if params["format"] != "png" {
		t.Errorf("expected format == 'png', got %v", params["format"])
	}
}

func TestScreenshotService_Capture_JPEGFormatWithQuality(t *testing.T) {
	mockCDP := &mockCDPCaller{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return json.Marshal(map[string]string{"data": dummyBase64})
		},
	}

	service := NewScreenshotService(ScreenshotServiceOptions{CDPService: mockCDP})
	quality := 80
	service.Capture(CaptureOptions{
		Format:  "jpeg",
		Quality: &quality,
	})

	if mockCDP.lastMethod != "Page.captureScreenshot" {
		t.Errorf("expected method 'Page.captureScreenshot', got %q", mockCDP.lastMethod)
	}
	params, ok := mockCDP.lastParams.(map[string]any)
	if !ok {
		t.Fatalf("expected params map[string]any, got %T", mockCDP.lastParams)
	}
	if params["format"] != "jpeg" {
		t.Errorf("expected format == 'jpeg', got %v", params["format"])
	}
	if params["quality"] != 80 {
		t.Errorf("expected quality == 80, got %v", params["quality"])
	}
}

func TestScreenshotService_Capture_ReturnsErrorOnCDPFailure(t *testing.T) {
	mockCDP := &mockCDPCaller{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return nil, errors.New("CDP接続エラー")
		},
	}

	service := NewScreenshotService(ScreenshotServiceOptions{CDPService: mockCDP})
	result := service.Capture()

	if result.Success {
		t.Errorf("expected success == false, got true")
	}
	if !strings.Contains(result.Error, "CDP接続エラー") {
		t.Errorf("expected Error to contain 'CDP接続エラー', got %q", result.Error)
	}
}

func TestScreenshotService_Capture_ReturnsErrorOnEmptyData(t *testing.T) {
	mockCDP := &mockCDPCaller{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return json.Marshal(map[string]string{"data": ""})
		},
	}

	service := NewScreenshotService(ScreenshotServiceOptions{CDPService: mockCDP})
	result := service.Capture()

	if result.Success {
		t.Errorf("expected success == false, got true")
	}
	if result.Error != "Screenshot data was empty." {
		t.Errorf("expected Error == 'Screenshot data was empty.', got %q", result.Error)
	}
}

func TestScreenshotService_GetBase64_ReturnsRawBase64String(t *testing.T) {
	mockCDP := &mockCDPCaller{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return json.Marshal(map[string]string{"data": dummyBase64})
		},
	}

	service := NewScreenshotService(ScreenshotServiceOptions{CDPService: mockCDP})
	result := service.GetBase64()

	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if *result != dummyBase64 {
		t.Errorf("expected %q, got %q", dummyBase64, *result)
	}
}

func TestScreenshotService_Capture_WithClipRegion(t *testing.T) {
	mockCDP := &mockCDPCaller{
		callFunc: func(method string, params any) (json.RawMessage, error) {
			return json.Marshal(map[string]string{"data": dummyBase64})
		},
	}

	service := NewScreenshotService(ScreenshotServiceOptions{CDPService: mockCDP})
	clip := &ClipRegion{X: 0, Y: 0, Width: 800, Height: 600, Scale: 1}
	service.Capture(CaptureOptions{Clip: clip})

	if mockCDP.lastMethod != "Page.captureScreenshot" {
		t.Errorf("expected method 'Page.captureScreenshot', got %q", mockCDP.lastMethod)
	}
	params, ok := mockCDP.lastParams.(map[string]any)
	if !ok {
		t.Fatalf("expected params map[string]any, got %T", mockCDP.lastParams)
	}
	c, ok := params["clip"].(*ClipRegion)
	if !ok || c.Width != 800 || c.Height != 600 {
		t.Errorf("expected clip with 800x600, got %v", params["clip"])
	}
}
