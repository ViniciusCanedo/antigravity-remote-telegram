package ui

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

type mockModelsCdp struct {
	models       []string
	currentModel string
	err          error
}

func (m *mockModelsCdp) GetUiModels(ctx context.Context) ([]string, error) {
	return m.models, m.err
}

func (m *mockModelsCdp) GetCurrentModel(ctx context.Context) (string, error) {
	return m.currentModel, m.err
}

func TestSendModelsUI_NotConnected(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	var sentText string
	var sentKb *InlineKeyboardMarkup

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		callCount++
		sentText = text
		sentKb = keyboard
		return nil
	}

	err := SendModelsUI(ctx, sendFn, ModelsUIDeps{
		GetCurrentCdp: func() ModelsCdpProvider { return nil },
		FetchQuota:    func(ctx context.Context) ([]ModelQuotaEntry, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if sentText != "Not connected to CDP." {
		t.Errorf("expected 'Not connected to CDP.', got %q", sentText)
	}
	if sentKb == nil || len(sentKb.InlineKeyboard) != 0 {
		t.Errorf("expected empty keyboard, got %+v", sentKb)
	}
}

func TestSendModelsUI_ModelsAvailable(t *testing.T) {
	ctx := context.Background()
	cdp := &mockModelsCdp{
		models:       []string{"Model A", "Model B"},
		currentModel: "Model A",
	}

	var sentText string
	var sentKb *InlineKeyboardMarkup

	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		sentText = text
		sentKb = keyboard
		return nil
	}

	err := SendModelsUI(ctx, sendFn, ModelsUIDeps{
		GetCurrentCdp: func() ModelsCdpProvider { return cdp },
		FetchQuota:    func(ctx context.Context) ([]ModelQuotaEntry, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(sentText, "Model Management") {
		t.Errorf("expected 'Model Management', got: %s", sentText)
	}
	if !strings.Contains(sentText, "Model A") {
		t.Errorf("expected 'Model A', got: %s", sentText)
	}
	if !strings.Contains(sentText, "Model B") {
		t.Errorf("expected 'Model B', got: %s", sentText)
	}
	if sentKb == nil {
		t.Fatal("expected non-nil keyboard")
	}
}

func TestBuildModelsUI_EmptyModels(t *testing.T) {
	ctx := context.Background()
	cdp := &mockModelsCdp{
		models:       []string{},
		currentModel: "",
	}

	res, err := BuildModelsUI(ctx, cdp, func(ctx context.Context) ([]ModelQuotaEntry, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result for empty models, got: %+v", res)
	}

	// Also verify SendModelsUI sends failure message
	var sentText string
	sendFn := func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error {
		sentText = text
		return nil
	}
	err = SendModelsUI(ctx, sendFn, ModelsUIDeps{
		GetCurrentCdp: func() ModelsCdpProvider { return cdp },
		FetchQuota:    func(ctx context.Context) ([]ModelQuotaEntry, error) { return nil, nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentText != "Failed to retrieve model list from Antigravity." {
		t.Errorf("expected 'Failed to retrieve model list from Antigravity.', got %q", sentText)
	}
}

func TestBuildModelsUI_ExhaustedStatus(t *testing.T) {
	ctx := context.Background()
	cdp := &mockModelsCdp{
		models:       []string{"Model A"},
		currentModel: "Model A",
	}

	frac := 0.0
	quota := []ModelQuotaEntry{
		{
			Label: "Model A",
			Model: "model_a",
			QuotaInfo: &QuotaInfo{
				RemainingFraction: &frac,
				ResetTime:         time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	res, err := BuildModelsUI(ctx, cdp, func(ctx context.Context) ([]ModelQuotaEntry, error) {
		return quota, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	if !strings.Contains(res.Text, "Exhausted") {
		t.Errorf("expected 'Exhausted' in text, got: %s", res.Text)
	}
	if !strings.Contains(res.Text, "⛔") {
		t.Errorf("expected '⛔' in text, got: %s", res.Text)
	}
	if strings.Contains(res.Text, "100%") {
		t.Errorf("did not expect '100%%' in text, got: %s", res.Text)
	}
}

func TestBuildModelsUI_PercentageStatus(t *testing.T) {
	ctx := context.Background()
	cdp := &mockModelsCdp{
		models:       []string{"Model A"},
		currentModel: "Model A",
	}

	frac := 0.6
	quota := []ModelQuotaEntry{
		{
			Label: "Model A",
			Model: "model_a",
			QuotaInfo: &QuotaInfo{
				RemainingFraction: &frac,
				ResetTime:         time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	res, err := BuildModelsUI(ctx, cdp, func(ctx context.Context) ([]ModelQuotaEntry, error) {
		return quota, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	if !strings.Contains(res.Text, "60%") {
		t.Errorf("expected '60%%' in text, got: %s", res.Text)
	}
	if !strings.Contains(res.Text, "🟢") {
		t.Errorf("expected '🟢' in text, got: %s", res.Text)
	}
}

func TestBuildModelsUI_NaNRemainingFraction(t *testing.T) {
	ctx := context.Background()
	cdp := &mockModelsCdp{
		models:       []string{"Model A"},
		currentModel: "Model A",
	}

	nanVal := math.NaN()
	quota := []ModelQuotaEntry{
		{
			Label: "Model A",
			Model: "model_a",
			QuotaInfo: &QuotaInfo{
				RemainingFraction: &nanVal,
				ResetTime:         "",
			},
		},
	}

	res, err := BuildModelsUI(ctx, cdp, func(ctx context.Context) ([]ModelQuotaEntry, error) {
		return quota, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	if !strings.Contains(res.Text, "N/A") {
		t.Errorf("expected 'N/A' in text, got: %s", res.Text)
	}
	if !strings.Contains(res.Text, "❓") {
		t.Errorf("expected '❓' in text, got: %s", res.Text)
	}
}

func TestBuildModelsUI_ExhaustedCallbackPrefix(t *testing.T) {
	ctx := context.Background()
	cdp := &mockModelsCdp{
		models:       []string{"Healthy Model", "Dead Model"},
		currentModel: "Healthy Model",
	}

	fracHealthy := 0.8
	fracDead := 0.0
	quota := []ModelQuotaEntry{
		{
			Label: "Healthy Model",
			Model: "healthy",
			QuotaInfo: &QuotaInfo{
				RemainingFraction: &fracHealthy,
				ResetTime:         "",
			},
		},
		{
			Label: "Dead Model",
			Model: "dead",
			QuotaInfo: &QuotaInfo{
				RemainingFraction: &fracDead,
				ResetTime:         time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	res, err := BuildModelsUI(ctx, cdp, func(ctx context.Context) ([]ModelQuotaEntry, error) {
		return quota, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}

	data, _ := json.Marshal(res.Keyboard.InlineKeyboard)
	kbData := string(data)

	if !strings.Contains(kbData, "model_btn_Healthy Model") {
		t.Errorf("expected callback 'model_btn_Healthy Model', got: %s", kbData)
	}
	if !strings.Contains(kbData, "model_exhausted_Dead Model") {
		t.Errorf("expected callback 'model_exhausted_Dead Model', got: %s", kbData)
	}
	if !strings.Contains(kbData, "⛔ Dead Model") {
		t.Errorf("expected label '⛔ Dead Model', got: %s", kbData)
	}
}
