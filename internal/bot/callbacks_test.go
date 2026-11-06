package bot

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"

	"antigravity-telegram-remote/internal/bot/ui"
	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/config"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/workspace"
	_ "modernc.org/sqlite"
)

type mockCallbackContext struct {
	mu           sync.Mutex
	data         string
	channel      TelegramChannel
	chatType     string
	isForum      bool
	editedTexts  []string
	editedPlains []string
	kbRemoved    bool
	answeredText string
	showAlert    bool
}

func newMockCallbackContext(data string, channel TelegramChannel) *mockCallbackContext {
	return &mockCallbackContext{
		data:     data,
		channel:  channel,
		chatType: "private",
	}
}

func (m *mockCallbackContext) Data() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data
}

func (m *mockCallbackContext) Channel() TelegramChannel {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.channel
}

func (m *mockCallbackContext) ChatType() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chatType
}

func (m *mockCallbackContext) IsForum() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isForum
}

func (m *mockCallbackContext) EditMessageText(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.editedTexts = append(m.editedTexts, text)
	return nil
}

func (m *mockCallbackContext) EditMessagePlainText(ctx context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.editedPlains = append(m.editedPlains, text)
	return nil
}

func (m *mockCallbackContext) RemoveInlineKeyboard(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kbRemoved = true
	return nil
}

func (m *mockCallbackContext) AnswerCallbackQuery(ctx context.Context, text string, showAlert bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answeredText = text
	m.showAlert = showAlert
	return nil
}

func setupTestBotWithDB(t *testing.T) (*Bot, *sql.DB) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in memory: %v", err)
	}

	templateRepo, err := database.NewTemplateRepository(db)
	if err != nil {
		t.Fatalf("failed to create template repo: %v", err)
	}
	bindingRepo, err := database.NewWorkspaceBindingRepository(db)
	if err != nil {
		t.Fatalf("failed to create binding repo: %v", err)
	}
	chatSessionRepo, err := database.NewChatSessionRepository(db)
	if err != nil {
		t.Fatalf("failed to create chat session repo: %v", err)
	}

	modeService := bridge.NewModeService()
	modelService := bridge.NewModelService()
	pool := cdp.NewConnectionPool(cdp.ClientOptions{})
	bridgePool := newBridgePoolAdapter(pool)
	autoAccept := autoAcceptCheckerAdapter{svc: bridge.NewAutoAcceptService(false)}
	bridgeInst := bridge.InitCdpBridge(false, bridgePool, nil, autoAccept)
	wsService := workspace.NewWorkspaceService(t.TempDir())

	bot := NewBot(BotDeps{
		Config: &config.AppConfig{
			UseTopics: false,
		},
		DB:                   db,
		Bridge:               bridgeInst,
		Pool:                 pool,
		ModeService:          modeService,
		ModelService:         modelService,
		TemplateRepo:         templateRepo,
		WorkspaceBindingRepo: bindingRepo,
		ChatSessionRepo:      chatSessionRepo,
		WorkspaceService:     wsService,
		SlashCommandHandler:  NewSlashCommandHandler(templateRepo),
		CleanupHandler:       NewCleanupCommandHandler(chatSessionRepo, bindingRepo),
	})

	return bot, db
}

func TestHandleCallbackQuery_ModeSelect(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	ch := TelegramChannel{ChatID: 100}
	cb := newMockCallbackContext("mode_select:plan", ch)

	err := bot.HandleCallbackQuery(context.Background(), cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bot.ModeService.GetCurrentMode() != "plan" {
		t.Errorf("expected current mode 'plan', got %q", bot.ModeService.GetCurrentMode())
	}
	if !strings.Contains(cb.answeredText, "Mode: Planning") {
		t.Errorf("expected answer to contain 'Mode: Planning', got %q", cb.answeredText)
	}
}

func TestHandleCallbackQuery_ModelExhausted(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	ch := TelegramChannel{ChatID: 100}
	cb := newMockCallbackContext(ui.ModelExhaustedPrefix+"Claude", ch)

	err := bot.HandleCallbackQuery(context.Background(), cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cb.showAlert {
		t.Errorf("expected showAlert to be true for exhausted model callback")
	}
	if !strings.Contains(cb.answeredText, "Claude is exhausted") {
		t.Errorf("expected alert message about Claude exhausted, got %q", cb.answeredText)
	}
}

func TestHandleCallbackQuery_AutoAccept(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	ch := TelegramChannel{ChatID: 100}

	// Turn ON
	cbOn := newMockCallbackContext(ui.AutoAcceptBtnOn, ch)
	err := bot.HandleCallbackQuery(context.Background(), cbOn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bot.Bridge.AutoAccept.IsEnabled() {
		t.Errorf("expected autoaccept to be enabled")
	}
	if cbOn.answeredText != "Auto-accept: ON" {
		t.Errorf("expected answer 'Auto-accept: ON', got %q", cbOn.answeredText)
	}

	// Turn OFF
	cbOff := newMockCallbackContext(ui.AutoAcceptBtnOff, ch)
	err = bot.HandleCallbackQuery(context.Background(), cbOff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bot.Bridge.AutoAccept.IsEnabled() {
		t.Errorf("expected autoaccept to be disabled")
	}
	if cbOff.answeredText != "Auto-accept: OFF" {
		t.Errorf("expected answer 'Auto-accept: OFF', got %q", cbOff.answeredText)
	}

	// Refresh
	cbRefresh := newMockCallbackContext(ui.AutoAcceptBtnRefresh, ch)
	err = bot.HandleCallbackQuery(context.Background(), cbRefresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cbRefresh.answeredText != "Refreshed" {
		t.Errorf("expected answer 'Refreshed', got %q", cbRefresh.answeredText)
	}
}

func TestHandleCallbackQuery_ProjectSelect(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	// Non-existent project
	ch := TelegramChannel{ChatID: 100}
	cbNonExistent := newMockCallbackContext(ui.ProjectSelectID+":nonexistent", ch)
	err := bot.HandleCallbackQuery(context.Background(), cbNonExistent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cbNonExistent.answeredText, `Project "nonexistent" not found.`) {
		t.Errorf("expected not found response, got %q", cbNonExistent.answeredText)
	}

	// Project page pagination
	cbPage := newMockCallbackContext(ui.ProjectPagePrefix+":0", ch)
	err = bot.HandleCallbackQuery(context.Background(), cbPage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cbPage.editedTexts) != 1 {
		t.Fatalf("expected 1 edited text for project page, got %d", len(cbPage.editedTexts))
	}
}

func TestHandleCallbackQuery_TemplateButton(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	ch := TelegramChannel{ChatID: 100}

	// Invalid template ID
	cbInvalid := newMockCallbackContext(ui.TemplateBtnPrefix+"abc", ch)
	err := bot.HandleCallbackQuery(context.Background(), cbInvalid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cbInvalid.answeredText != "Invalid template." {
		t.Errorf("expected 'Invalid template.', got %q", cbInvalid.answeredText)
	}

	// Non-existent template
	cbMissing := newMockCallbackContext(ui.TemplateBtnPrefix+"999", ch)
	err = bot.HandleCallbackQuery(context.Background(), cbMissing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cbMissing.answeredText != "Template not found." {
		t.Errorf("expected 'Template not found.', got %q", cbMissing.answeredText)
	}
}

func TestHandleCallbackQuery_SessionSelect_NoProjectBound(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	ch := TelegramChannel{ChatID: 555}
	cb := newMockCallbackContext(ui.SessionSelectID+":My Session", ch)

	err := bot.HandleCallbackQuery(context.Background(), cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb.answeredText != "No project bound." {
		t.Errorf("expected 'No project bound.', got %q", cb.answeredText)
	}
}

func TestHandleCallbackQuery_PlanEditButton(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()
	mockSender := newMockTelegramSender()
	bot.SetTelegramSender(mockSender)

	ch := TelegramChannel{ChatID: 600}
	cb := newMockCallbackContext(ui.PlanEditBtn+":test-project", ch)

	err := bot.HandleCallbackQuery(context.Background(), cb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cb.answeredText != "Type your edit instructions (or /cancel)." {
		t.Errorf("expected answer prompt, got %q", cb.answeredText)
	}
	state, ok := bot.GetPlanEditPending(ChannelKey(ch))
	if !ok || state.ProjectName != "test-project" {
		t.Errorf("expected pending state for 'test-project', got %+v (ok=%v)", state, ok)
	}
}

func TestHandleCallbackQuery_CleanupButtons(t *testing.T) {
	bot, db := setupTestBotWithDB(t)
	defer db.Close()

	ch := TelegramChannel{ChatID: 700}

	// Cancel button
	cbCancel := newMockCallbackContext(CleanupCancelBtn, ch)
	err := bot.HandleCallbackQuery(context.Background(), cbCancel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cbCancel.answeredText != "Cancelled" {
		t.Errorf("expected 'Cancelled', got %q", cbCancel.answeredText)
	}
	if len(cbCancel.editedPlains) != 1 || cbCancel.editedPlains[0] != "Cleanup cancelled." {
		t.Errorf("expected 'Cleanup cancelled.' edit, got %+v", cbCancel.editedPlains)
	}

	// Archive with 0 sessions
	cbArchive := newMockCallbackContext(CleanupArchiveBtn+":7", ch)
	err = bot.HandleCallbackQuery(context.Background(), cbArchive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cbArchive.answeredText, "0 session(s) archived") {
		t.Errorf("expected '0 session(s) archived', got %q", cbArchive.answeredText)
	}

	// Delete with 0 sessions
	cbDelete := newMockCallbackContext(CleanupDeleteBtn+":7", ch)
	err = bot.HandleCallbackQuery(context.Background(), cbDelete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(cbDelete.answeredText, "0 session(s) deleted") {
		t.Errorf("expected '0 session(s) deleted', got %q", cbDelete.answeredText)
	}
}
