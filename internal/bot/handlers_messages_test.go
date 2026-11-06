package bot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/voice"
)

type mockCdpEvaluatorImpl struct {
	primaryID *int64
	wsName    string
}

func (m *mockCdpEvaluatorImpl) Call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockCdpEvaluatorImpl) GetPrimaryContextId() *int64 {
	return m.primaryID
}

func (m *mockCdpEvaluatorImpl) GetCurrentWorkspaceName() string {
	return m.wsName
}

func TestHandleTextMessage_PlanEditInterception(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	sender := newMockTelegramSender()
	bot.SetTelegramSender(sender)

	ch := TelegramChannel{ChatID: 200, ThreadID: 10}
	key := ChannelKey(ch)

	var dispatchedPrompt string
	bot.PromptDispatcher = bridge.NewPromptDispatcher(bridge.PromptDispatcherDeps{
		SendPromptImpl: func(
			ctx context.Context,
			b any,
			channel bridge.TelegramChannel,
			prompt string,
			cdp bridge.CdpEvaluator,
			modeService *bridge.ModeService,
			modelService *bridge.ModelService,
			inboundImages []bridge.InboundImageAttachment,
			options *bridge.PromptDispatchOptions,
		) error {
			dispatchedPrompt = prompt
			return nil
		},
	})

	// Scenario 1: /cancel should cancel plan edit
	bot.SetPlanEditPending(key, "test-project")
	err := bot.HandleTextMessage(context.Background(), ch, "/cancel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := bot.GetPlanEditPending(key); ok {
		t.Errorf("expected plan edit pending state to be cleared after /cancel")
	}
	if len(sender.sentMessages) == 0 || sender.sentMessages[len(sender.sentMessages)-1].Text != "Plan edit cancelled." {
		t.Errorf("expected 'Plan edit cancelled.', got %v", sender.sentMessages)
	}
	if dispatchedPrompt != "" {
		t.Errorf("expected no prompt dispatched after cancel, got %q", dispatchedPrompt)
	}

	// Scenario 2: Edit feedback when CDP is disconnected
	bot.SetPlanEditPending(key, "test-project")
	err = bot.HandleTextMessage(context.Background(), ch, "Change SQLite to PostgreSQL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := bot.GetPlanEditPending(key); ok {
		t.Errorf("expected plan edit pending state to be cleared")
	}
	lastMsg := sender.sentMessages[len(sender.sentMessages)-1].Text
	if lastMsg != "Not connected to CDP." {
		t.Errorf("expected 'Not connected to CDP.', got %q", lastMsg)
	}

	// Scenario 3: Edit feedback with CDP connected
	mockClient := newMockCdpServiceClient()
	bot.SetWorkspaceResolver(func(ctx context.Context, c TelegramChannel) *ResolvedWorkspaceCdp {
		return &ResolvedWorkspaceCdp{
			Cdp:           mockClient,
			ProjectName:   "test-project",
			WorkspacePath: "/tmp/test-project",
		}
	})

	bot.SetPlanEditPending(key, "test-project")
	err = bot.HandleTextMessage(context.Background(), ch, "Change SQLite to PostgreSQL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrefix := "Please revise the plan based on the following feedback:\n\nChange SQLite to PostgreSQL"
	if dispatchedPrompt != expectedPrefix {
		t.Errorf("expected dispatched prompt %q, got %q", expectedPrefix, dispatchedPrompt)
	}
}

func TestHandleTextMessage_SlashCommandsAndRouting(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	sender := newMockTelegramSender()
	bot.SetTelegramSender(sender)

	ch := TelegramChannel{ChatID: 300}

	// 1. /autoaccept command
	err := bot.HandleTextMessage(context.Background(), ch, "/autoaccept on")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sentMessages) == 0 {
		t.Fatalf("expected reply for /autoaccept")
	}
	msgText := sender.sentMessages[len(sender.sentMessages)-1].Text
	if !strings.Contains(msgText, "ON") {
		t.Errorf("expected autoaccept reply to mention ON, got %q", msgText)
	}

	// 2. /status command
	err = bot.HandleTextMessage(context.Background(), ch, "/status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	statusMsg := sender.sentMessages[len(sender.sentMessages)-1].Text
	if !strings.Contains(statusMsg, "Bot Status") || !strings.Contains(statusMsg, "Auto Approve") {
		t.Errorf("expected status HTML message, got %q", statusMsg)
	}

	// 3. /template command via SlashCommandHandler
	var dispatchedPrompt string
	bot.PromptDispatcher = bridge.NewPromptDispatcher(bridge.PromptDispatcherDeps{
		SendPromptImpl: func(
			ctx context.Context,
			b any,
			channel bridge.TelegramChannel,
			prompt string,
			cdp bridge.CdpEvaluator,
			modeService *bridge.ModeService,
			modelService *bridge.ModelService,
			inboundImages []bridge.InboundImageAttachment,
			options *bridge.PromptDispatchOptions,
		) error {
			dispatchedPrompt = prompt
			return nil
		},
	})

	// Add template
	_, err = bot.TemplateRepo.Create(database.CreateTemplateInput{
		Name:   "PRReview",
		Prompt: "Review this pull request carefully.",
	})
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	mockClient := newMockCdpServiceClient()
	bot.SetWorkspaceResolver(func(ctx context.Context, c TelegramChannel) *ResolvedWorkspaceCdp {
		return &ResolvedWorkspaceCdp{
			Cdp:           mockClient,
			ProjectName:   "my-app",
			WorkspacePath: "/tmp/my-app",
		}
	})

	err = bot.HandleTextMessage(context.Background(), ch, "/template PRReview")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dispatchedPrompt != "Review this pull request carefully." {
		t.Errorf("expected dispatched template prompt, got %q", dispatchedPrompt)
	}
}

func TestHandleTextMessage_RegularMessageRouting(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	sender := newMockTelegramSender()
	bot.SetTelegramSender(sender)

	ch := TelegramChannel{ChatID: 400}
	key := ChannelKey(ch)

	// Scenario 1: No project bound
	err := bot.HandleTextMessage(context.Background(), ch, "Hello Antigravity")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lastMsg := sender.sentMessages[len(sender.sentMessages)-1].Text
	if lastMsg != "No project is configured for this chat. Use /project to select one." {
		t.Errorf("expected no project message, got %q", lastMsg)
	}

	// Scenario 2: Project bound + Chat Session
	mockClient := newMockCdpServiceClient()
	bot.SetWorkspaceResolver(func(ctx context.Context, c TelegramChannel) *ResolvedWorkspaceCdp {
		return &ResolvedWorkspaceCdp{
			Cdp:           mockClient,
			ProjectName:   "my-web",
			WorkspacePath: "/tmp/my-web",
		}
	})

	var dispatchedPrompt string
	bot.PromptDispatcher = bridge.NewPromptDispatcher(bridge.PromptDispatcherDeps{
		SendPromptImpl: func(
			ctx context.Context,
			b any,
			channel bridge.TelegramChannel,
			prompt string,
			cdp bridge.CdpEvaluator,
			modeService *bridge.ModeService,
			modelService *bridge.ModelService,
			inboundImages []bridge.InboundImageAttachment,
			options *bridge.PromptDispatchOptions,
		) error {
			dispatchedPrompt = prompt
			return nil
		},
	})

	// Create session with display name
	sessionRecord, err := bot.ChatSessionRepo.Create(database.CreateChatSessionInput{
		ChannelID:     key,
		CategoryID:    "cat-1",
		WorkspacePath: "/tmp/my-web",
		SessionNumber: 1,
		GuildID:       "400",
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	_, _ = bot.ChatSessionRepo.UpdateDisplayName(key, "Feature Design")

	evaluator := &mockCdpEvaluatorImpl{wsName: "my-web"}
	detector := bridge.NewUserMessageDetector(bridge.UserMessageDetectorOptions{
		CdpService: evaluator,
	})
	bot.Bridge.Pool.RegisterUserMessageDetector("my-web", detector)

	err = bot.HandleTextMessage(context.Background(), ch, "Please build the login page.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dispatchedPrompt != "Please build the login page." {
		t.Errorf("expected dispatched prompt 'Please build the login page.', got %q", dispatchedPrompt)
	}

	// Verify echo hash was added to detector
	normHash := bridge.ComputeEchoHash("Please build the login page.")
	if !detector.HasEchoHash(normHash) {
		t.Errorf("expected detector to contain echo hash for message")
	}

	_ = sessionRecord
}

func TestHandlePhotoMessage(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	sender := newMockTelegramSender()
	bot.SetTelegramSender(sender)

	ch := TelegramChannel{ChatID: 500}

	// 1. Empty photos slice
	err := bot.HandlePhotoMessage(context.Background(), ch, 1, nil, "Look at this")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2. No project configured
	photos := []TelegramPhotoInfo{
		{FileID: "file-sm", Width: 100, Height: 100},
		{FileID: "file-lg", Width: 800, Height: 800},
	}
	err = bot.HandlePhotoMessage(context.Background(), ch, 2, photos, "Analyze this image")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sentMessages) == 0 || sender.sentMessages[len(sender.sentMessages)-1].Text != "No project configured. Use /project first." {
		t.Errorf("expected 'No project configured. Use /project first.', got %v", sender.sentMessages)
	}

	// 3. Project configured and photo download success
	mockClient := newMockCdpServiceClient()
	bot.SetWorkspaceResolver(func(ctx context.Context, c TelegramChannel) *ResolvedWorkspaceCdp {
		return &ResolvedWorkspaceCdp{
			Cdp:           mockClient,
			ProjectName:   "photo-proj",
			WorkspacePath: "/tmp/photo-proj",
		}
	})

	var dispatchedPrompt string
	var dispatchedImages []bridge.InboundImageAttachment
	bot.PromptDispatcher = bridge.NewPromptDispatcher(bridge.PromptDispatcherDeps{
		SendPromptImpl: func(
			ctx context.Context,
			b any,
			channel bridge.TelegramChannel,
			prompt string,
			cdp bridge.CdpEvaluator,
			modeService *bridge.ModeService,
			modelService *bridge.ModelService,
			inboundImages []bridge.InboundImageAttachment,
			options *bridge.PromptDispatchOptions,
		) error {
			dispatchedPrompt = prompt
			dispatchedImages = inboundImages
			return nil
		},
	})

	// Setup local test server for image download
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("mock-image-binary-data"))
	}))
	defer ts.Close()

	resolver := &mockFileResolver{
		paths: map[string]string{
			"file-lg": "photos/file-lg.png",
		},
	}
	bot.SetFileResolver(resolver)

	// With empty caption, should use default caption
	err = bot.HandlePhotoMessage(context.Background(), ch, 3, photos, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dispatchedPrompt != "Please review the attached images and respond accordingly." {
		t.Errorf("expected default caption, got %q", dispatchedPrompt)
	}
	_ = dispatchedImages
}

func TestHandleVoiceMessage(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	sender := newMockTelegramSender()
	bot.SetTelegramSender(sender)

	ch := TelegramChannel{ChatID: 600}
	voiceInfo := voice.TelegramVoiceInfo{
		FileID:       "voice-123",
		FileUniqueID: "unique-123",
		Duration:     5,
		MimeType:     "audio/ogg",
	}

	// 1. Whisper unavailable
	// Since nodejs-whisper is not installed in the workspace temp dir, CheckWhisperAvailability returns an issue message
	err := bot.HandleVoiceMessage(context.Background(), ch, voiceInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sentMessages) == 0 {
		t.Fatalf("expected message from CheckWhisperAvailability")
	}
	if !strings.Contains(sender.sentMessages[len(sender.sentMessages)-1].Text, "Voice transcription is not set up") {
		t.Errorf("expected setup instruction, got %q", sender.sentMessages[len(sender.sentMessages)-1].Text)
	}

	// 2. Whisper mock setup (fake binary + fake model directory)
	tmpDir := t.TempDir()
	whisperDir := filepath.Join(tmpDir, "node_modules", "nodejs-whisper")
	_ = os.MkdirAll(whisperDir, 0o755)
	binDir := filepath.Join(whisperDir, "cpp", "whisper.cpp", "build", "bin")
	_ = os.MkdirAll(binDir, 0o755)
	mockBin := filepath.Join(binDir, "whisper-cli")
	_ = os.WriteFile(mockBin, []byte("#!/bin/sh\necho 'hello world'\n"), 0o755)
	modelDir := filepath.Join(whisperDir, "models")
	_ = os.MkdirAll(modelDir, 0o755)
	_ = os.WriteFile(filepath.Join(modelDir, "ggml-base.en.bin"), []byte("model"), 0o644)

	insp := &voice.WhisperInspector{
		BaseDir:           tmpDir,
		WhisperBinaryPath: mockBin,
		WhisperModelDir:   modelDir,
	}
	bot.SetWhisperInspector(insp)

	// 3. No project configured when whisper is available
	bot.SetWorkspaceResolver(func(ctx context.Context, c TelegramChannel) *ResolvedWorkspaceCdp {
		return nil
	})
	err = bot.HandleVoiceMessage(context.Background(), ch, voiceInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lastMsg := sender.sentMessages[len(sender.sentMessages)-1].Text
	if lastMsg != "No project configured. Use /project first." {
		t.Errorf("expected 'No project configured. Use /project first.', got %q", lastMsg)
	}

	// 4. Project configured: test download error
	mockClient := newMockCdpServiceClient()
	bot.SetWorkspaceResolver(func(ctx context.Context, c TelegramChannel) *ResolvedWorkspaceCdp {
		return &ResolvedWorkspaceCdp{
			Cdp:           mockClient,
			ProjectName:   "voice-project",
			WorkspacePath: "/tmp/voice-project",
		}
	})

	errResolver := &mockFileResolver{
		err: errors.New("telegram download failed"),
	}
	bot.SetFileResolver(errResolver)

	err = bot.HandleVoiceMessage(context.Background(), ch, voiceInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lastMsg = sender.sentMessages[len(sender.sentMessages)-1].Text
	if lastMsg != "❌ Could not download voice message. Please try again." {
		t.Errorf("expected download failure error, got %q", lastMsg)
	}

	// 5. Test transcription error
	// Create mock file on disk for successful download
	fakeVoiceFile := filepath.Join(tmpDir, "audio.ogg")
	_ = os.WriteFile(fakeVoiceFile, []byte("fake-ogg-content"), 0o644)

	// Setup local test server for voice download
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/ogg")
		_, _ = w.Write([]byte("fake-ogg-content"))
	}))
	defer ts.Close()

	testVoiceClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			newReq, _ := http.NewRequest(req.Method, ts.URL, req.Body)
			return http.DefaultClient.Do(newReq)
		}),
	}
	bot.SetHTTPClient(testVoiceClient)

	okResolver := &mockFileResolver{
		paths: map[string]string{
			"voice-123": "voice/voice-123.ogg",
		},
	}
	bot.SetFileResolver(okResolver)

	// Runner returns error / empty transcript
	bot.SetWhisperRunner(func(ctx context.Context, voicePath string) (string, error) {
		return "", errors.New("whisper error")
	})

	err = bot.HandleVoiceMessage(context.Background(), ch, voiceInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lastMsg = sender.sentMessages[len(sender.sentMessages)-1].Text
	if lastMsg != "❌ Could not transcribe voice message. Please try again or type your prompt." {
		t.Errorf("expected transcription failure error, got %q", lastMsg)
	}

	// 6. Test successful voice transcription as regular prompt
	var dispatchedPrompt string
	bot.PromptDispatcher = bridge.NewPromptDispatcher(bridge.PromptDispatcherDeps{
		SendPromptImpl: func(
			ctx context.Context,
			b any,
			channel bridge.TelegramChannel,
			prompt string,
			cdp bridge.CdpEvaluator,
			modeService *bridge.ModeService,
			modelService *bridge.ModelService,
			inboundImages []bridge.InboundImageAttachment,
			options *bridge.PromptDispatchOptions,
		) error {
			dispatchedPrompt = prompt
			return nil
		},
	})

	bot.SetWhisperRunner(func(ctx context.Context, voicePath string) (string, error) {
		return "Deploy the latest release to production", nil
	})

	detector := bridge.NewUserMessageDetector(bridge.UserMessageDetectorOptions{
		CdpService: &mockCdpEvaluatorImpl{wsName: "voice-project"},
	})
	bot.Bridge.Pool.RegisterUserMessageDetector("voice-project", detector)

	err = bot.HandleVoiceMessage(context.Background(), ch, voiceInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dispatchedPrompt != "Deploy the latest release to production" {
		t.Errorf("expected dispatched prompt from voice transcript, got %q", dispatchedPrompt)
	}

	// 7. Test voice transcription as a slash command (e.g. /autoaccept)
	bot.SetWhisperRunner(func(ctx context.Context, voicePath string) (string, error) {
		return "/autoaccept off", nil
	})

	err = bot.HandleVoiceMessage(context.Background(), ch, voiceInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lastMsg = sender.sentMessages[len(sender.sentMessages)-1].Text
	if !strings.Contains(lastMsg, "/autoaccept off") {
		t.Errorf("expected command output in reply, got %q", lastMsg)
	}
}

func TestRegisterMessageHandlers(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	// Calling RegisterMessageHandlers with nil TgBot does not panic
	bot.TgBot = nil
	bot.RegisterMessageHandlers()
}
