package bot

import (
	"context"
	"testing"

	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/config"
	"github.com/go-telegram/bot/models"
)

func TestChannelKey_Formatting(t *testing.T) {
	ch1 := TelegramChannel{ChatID: 12345, ThreadID: 0}
	if got := ChannelKey(ch1); got != "12345" {
		t.Fatalf("expected '12345', got %q", got)
	}

	ch2 := TelegramChannel{ChatID: -100123456789, ThreadID: 42}
	if got := ChannelKey(ch2); got != "-100123456789:42" {
		t.Fatalf("expected '-100123456789:42', got %q", got)
	}
}

func TestParseChannelKey_Success(t *testing.T) {
	ch1, err := ParseChannelKey("98765")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch1.ChatID != 98765 || ch1.ThreadID != 0 {
		t.Fatalf("unexpected channel: %+v", ch1)
	}

	ch2, err := ParseChannelKey("-100999:77")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch2.ChatID != -100999 || ch2.ThreadID != 77 {
		t.Fatalf("unexpected channel: %+v", ch2)
	}
}

func TestParseChannelKey_Errors(t *testing.T) {
	tests := []string{
		"invalid",
		"123:abc",
		"abc:123",
		"1:2:3",
		"",
	}
	for _, tc := range tests {
		_, err := ParseChannelKey(tc)
		if err == nil {
			t.Fatalf("expected error for input %q, got nil", tc)
		}
	}
}

func TestGetChannelFromMessage_And_Callback(t *testing.T) {
	msgCh := GetChannelFromMessage(100, 20)
	if msgCh.ChatID != 100 || msgCh.ThreadID != 20 {
		t.Fatalf("unexpected message channel: %+v", msgCh)
	}

	cbCh := GetChannelFromCallback(200, 0)
	if cbCh.ChatID != 200 || cbCh.ThreadID != 0 {
		t.Fatalf("unexpected callback channel: %+v", cbCh)
	}
}

func TestBot_UserStopRequest(t *testing.T) {
	b := NewBot(BotDeps{})
	key := "test-channel-key"

	if b.HasUserStopRequest(key) {
		t.Fatal("expected stop request not to be present initially")
	}

	b.AddUserStopRequest(key)
	if !b.HasUserStopRequest(key) {
		t.Fatal("expected stop request to be present after Add")
	}

	consumed := b.ConsumeUserStopRequest(key)
	if !consumed {
		t.Fatal("expected Consume to return true")
	}
	if b.HasUserStopRequest(key) {
		t.Fatal("expected stop request to be cleared after Consume")
	}

	secondConsume := b.ConsumeUserStopRequest(key)
	if secondConsume {
		t.Fatal("expected subsequent Consume to return false")
	}
}

func TestBot_PlanEditPending(t *testing.T) {
	b := NewBot(BotDeps{})
	key := "channel-1"

	_, ok := b.GetPlanEditPending(key)
	if ok {
		t.Fatal("expected plan edit pending not to be set initially")
	}

	b.SetPlanEditPending(key, "my-project")
	state, ok := b.GetPlanEditPending(key)
	if !ok || state.ProjectName != "my-project" {
		t.Fatalf("expected project 'my-project', got %+v (ok=%v)", state, ok)
	}

	b.ClearPlanEditPending(key)
	_, ok = b.GetPlanEditPending(key)
	if ok {
		t.Fatal("expected plan edit pending to be cleared")
	}
}

func TestBot_PlanContentCache(t *testing.T) {
	b := NewBot(BotDeps{})
	key := "channel-cache"

	_, ok := b.GetPlanContentCache(key)
	if ok {
		t.Fatal("expected cache to be empty initially")
	}

	pages := []string{"page 1 content", "page 2 content"}
	b.SetPlanContentCache(key, pages)
	cached, ok := b.GetPlanContentCache(key)
	if !ok || len(cached) != 2 || cached[0] != "page 1 content" {
		t.Fatalf("unexpected cached content: %+v (ok=%v)", cached, ok)
	}

	b.ClearPlanContentCache(key)
	_, ok = b.GetPlanContentCache(key)
	if ok {
		t.Fatal("expected cache to be cleared")
	}
}

func TestBot_Adapters(t *testing.T) {
	modeSvc := bridge.NewModeService()
	modeAdapter := modeServiceUIAdapter{svc: modeSvc}
	if modeAdapter.GetCurrentMode() != "fast" {
		t.Fatalf("expected default mode 'fast', got %q", modeAdapter.GetCurrentMode())
	}
	modeAdapter.SetMode("plan")
	if modeAdapter.GetCurrentMode() != "plan" {
		t.Fatalf("expected updated mode 'plan', got %q", modeAdapter.GetCurrentMode())
	}

	nilModeAdapter := modeServiceUIAdapter{svc: nil}
	if nilModeAdapter.GetCurrentMode() != "fast" {
		t.Fatalf("expected fallback 'fast', got %q", nilModeAdapter.GetCurrentMode())
	}
	nilModeAdapter.SetMode("fast") // should not panic

	autoAcceptSvc := bridge.NewAutoAcceptService(false)
	aaAdapter := autoAcceptCheckerAdapter{svc: autoAcceptSvc}
	if aaAdapter.IsEnabled() {
		t.Fatal("expected auto-accept to be disabled initially")
	}
	res, err := aaAdapter.Handle("on")
	if err != nil || !res.Enabled {
		t.Fatalf("expected handle 'on' to enable auto-accept, got res=%+v err=%v", res, err)
	}

	nilAaAdapter := autoAcceptCheckerAdapter{svc: nil}
	if nilAaAdapter.IsEnabled() {
		t.Fatal("expected nil auto-accept adapter to return false")
	}
	_, err = nilAaAdapter.Handle("on")
	if err == nil {
		t.Fatal("expected error for nil auto-accept service")
	}

	detectorAdapter := backgroundDetectorAdapter{
		isActive: func() bool { return true },
		stop:     func() {},
	}
	if !detectorAdapter.IsActive() {
		t.Fatal("expected IsActive to return true")
	}
	detectorAdapter.Start() // should not panic
	detectorAdapter.Stop()  // should not panic
}

func TestBot_SetupPoolEventListeners(t *testing.T) {
	b := NewBot(BotDeps{
		Config: &config.AppConfig{
			AllowedUserIds: []string{"123"},
		},
	})
	pool := cdp.NewConnectionPool(cdp.ClientOptions{})

	// Should not panic with nil or non-nil pool
	b.SetupPoolEventListeners(nil)
	b.SetupPoolEventListeners(pool)

	// Triggering pool events when TgBot is nil should not crash
	pool.Emit("workspace:disconnected", "my-project")
	pool.Emit("workspace:reconnected", "my-project")
	pool.Emit("workspace:reconnectFailed", "my-project")
}

func TestBot_RegisterCommands_NilTgBot(t *testing.T) {
	b := NewBot(BotDeps{})
	b.RegisterCommands() // should not panic

	err := b.RegisterBotCommands(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when TgBot is nil, got %v", err)
	}
}

func TestBot_NilMessageHandlers(t *testing.T) {
	b := NewBot(BotDeps{})
	ctx := context.Background()
	emptyUpdate := &models.Update{}

	b.handleStart(ctx, nil, emptyUpdate)
	b.handleHelp(ctx, nil, emptyUpdate)
	b.handlePing(ctx, nil, emptyUpdate)
	b.handleStop(ctx, nil, emptyUpdate)
	b.handleChat(ctx, nil, emptyUpdate)
	b.handleProject(ctx, nil, emptyUpdate)
	b.handleNew(ctx, nil, emptyUpdate)
	b.handleJoin(ctx, nil, emptyUpdate)
	b.handleMode(ctx, nil, emptyUpdate)
	b.handleModel(ctx, nil, emptyUpdate)
	b.handleScreenshot(ctx, nil, emptyUpdate)
	b.handleTemplate(ctx, nil, emptyUpdate)
	b.handleTemplateAdd(ctx, nil, emptyUpdate)
	b.handleTemplateDelete(ctx, nil, emptyUpdate)
	b.handleAutoAccept(ctx, nil, emptyUpdate)
	b.handleStatus(ctx, nil, emptyUpdate)
	b.handleCleanup(ctx, nil, emptyUpdate)
}
