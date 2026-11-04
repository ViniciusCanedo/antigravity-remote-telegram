package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/cdp"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type mockTelegramSender struct {
	mu            sync.Mutex
	sentMessages  []*tgbot.SendMessageParams
	editedTexts   []*tgbot.EditMessageTextParams
	editedMarkup  []*tgbot.EditMessageReplyMarkupParams
	sentPhotos    []*tgbot.SendPhotoParams
	sentDocs      []*tgbot.SendDocumentParams
	answeredCbs   []*tgbot.AnswerCallbackQueryParams
	deletedTopics []*tgbot.DeleteForumTopicParams
	closedTopics  []*tgbot.CloseForumTopicParams

	nextMsgID        int
	sendMessageErr   error
	editMessageErr   error
	failAttemptsHTML int
	attemptCountHTML int
}

func newMockTelegramSender() *mockTelegramSender {
	return &mockTelegramSender{nextMsgID: 100}
}

func toChatID(v any) int64 {
	switch id := v.(type) {
	case int64:
		return id
	case int:
		return int64(id)
	default:
		return 0
	}
}

func (m *mockTelegramSender) SendMessage(ctx context.Context, params *tgbot.SendMessageParams) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failAttemptsHTML > 0 && m.attemptCountHTML < m.failAttemptsHTML {
		m.attemptCountHTML++
		return nil, errors.New("Bad Request: can't parse entities in message text")
	}

	if m.sendMessageErr != nil {
		return nil, m.sendMessageErr
	}

	m.nextMsgID++
	m.sentMessages = append(m.sentMessages, params)
	return &models.Message{
		ID:   m.nextMsgID,
		Text: params.Text,
		Chat: models.Chat{ID: toChatID(params.ChatID)},
	}, nil
}

func (m *mockTelegramSender) EditMessageText(ctx context.Context, params *tgbot.EditMessageTextParams) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.editMessageErr != nil {
		return nil, m.editMessageErr
	}
	m.editedTexts = append(m.editedTexts, params)
	return &models.Message{
		ID:   params.MessageID,
		Text: params.Text,
		Chat: models.Chat{ID: toChatID(params.ChatID)},
	}, nil
}

func (m *mockTelegramSender) EditMessageReplyMarkup(ctx context.Context, params *tgbot.EditMessageReplyMarkupParams) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.editedMarkup = append(m.editedMarkup, params)
	return &models.Message{ID: params.MessageID, Chat: models.Chat{ID: toChatID(params.ChatID)}}, nil
}

func (m *mockTelegramSender) SendPhoto(ctx context.Context, params *tgbot.SendPhotoParams) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentPhotos = append(m.sentPhotos, params)
	m.nextMsgID++
	return &models.Message{ID: m.nextMsgID, Chat: models.Chat{ID: toChatID(params.ChatID)}}, nil
}

func (m *mockTelegramSender) SendDocument(ctx context.Context, params *tgbot.SendDocumentParams) (*models.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentDocs = append(m.sentDocs, params)
	m.nextMsgID++
	return &models.Message{ID: m.nextMsgID, Chat: models.Chat{ID: toChatID(params.ChatID)}}, nil
}

func (m *mockTelegramSender) AnswerCallbackQuery(ctx context.Context, params *tgbot.AnswerCallbackQueryParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answeredCbs = append(m.answeredCbs, params)
	return true, nil
}

func (m *mockTelegramSender) DeleteForumTopic(ctx context.Context, params *tgbot.DeleteForumTopicParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedTopics = append(m.deletedTopics, params)
	return true, nil
}

func (m *mockTelegramSender) CloseForumTopic(ctx context.Context, params *tgbot.CloseForumTopicParams) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closedTopics = append(m.closedTopics, params)
	return true, nil
}

type mockCdpServiceClient struct {
	connected     bool
	model         string
	workspaceName string
	primaryCtxID  *int
	injectResult  cdp.InjectResult
	injectErr     error
	images        []cdp.ExtractedResponseImage

	callResponses map[string]json.RawMessage
	mu            sync.Mutex
	calls         []string
}

func newMockCdpServiceClient() *mockCdpServiceClient {
	id := 1
	return &mockCdpServiceClient{
		connected:     true,
		model:         "claude-3-5-sonnet",
		workspaceName: "test-workspace",
		primaryCtxID:  &id,
		injectResult:  cdp.InjectResult{OK: true},
		callResponses: make(map[string]json.RawMessage),
	}
}

func (c *mockCdpServiceClient) IsConnected() bool {
	return c.connected
}

func (c *mockCdpServiceClient) GetCurrentModel() *string {
	return &c.model
}

func (c *mockCdpServiceClient) InjectMessage(text string) (cdp.InjectResult, error) {
	return c.injectResult, c.injectErr
}

func (c *mockCdpServiceClient) InjectMessageWithImageFiles(text string, imageFilePaths []string) (cdp.InjectResult, error) {
	return c.injectResult, c.injectErr
}

func (c *mockCdpServiceClient) GetPrimaryContextID() *int {
	return c.primaryCtxID
}

func (c *mockCdpServiceClient) Call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, method)

	if resp, ok := c.callResponses[method]; ok {
		return resp, nil
	}
	// Return default empty Runtime.evaluate value
	return json.RawMessage(`{"result":{"value":""}}`), nil
}

func (c *mockCdpServiceClient) ExtractLatestResponseImages(maxImages int) []cdp.ExtractedResponseImage {
	return c.images
}

func (c *mockCdpServiceClient) GetCurrentWorkspaceName() string {
	return c.workspaceName
}

func (c *mockCdpServiceClient) DiscoverAndConnectForWorkspace(workspacePath string) (bool, error) {
	return true, nil
}

func (c *mockCdpServiceClient) Disconnect() error {
	c.connected = false
	return nil
}

func (c *mockCdpServiceClient) On(event string, handler cdp.EventHandler) {}

func (c *mockCdpServiceClient) GetContexts() []cdp.Context {
	return []cdp.Context{{ID: 1}}
}

func (c *mockCdpServiceClient) WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool {
	return true
}

func TestBalanceHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Closes tags in reverse order",
			input:    "<b><i>hello",
			expected: "<b><i>hello</i></b>",
		},
		{
			name:     "Removes orphan closing tags",
			input:    "hello</i></b> world",
			expected: "hello world",
		},
		{
			name:     "Preserves self-closing tags",
			input:    "line 1<br/>line 2<hr>line 3<img src='x'/>",
			expected: "line 1<br/>line 2<hr>line 3<img src='x'/>",
		},
		{
			name:     "Complex unclosed tags",
			input:    "<div><p><b>content",
			expected: "<div><p><b>content</b></p></div>",
		},
		{
			name:     "Already balanced",
			input:    "<b>hello</b> <i>world</i>",
			expected: "<b>hello</b> <i>world</i>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BalanceHTMLTags(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestStripHTMLForFile(t *testing.T) {
	input := `<pre><code class="language-go">fmt.Println("hi")</code></pre>
<b>bold</b>, <i>italic</i>, <s>strike</s>, <a href="https://example.com">link</a>
<blockquote>quote line 1
quote line 2</blockquote>`

	got := StripHTMLForFile(input)

	if !strings.Contains(got, "```go\nfmt.Println(\"hi\")\n```") {
		t.Errorf("expected fenced code block, got: %s", got)
	}
	if !strings.Contains(got, "**bold**") {
		t.Errorf("expected bold in markdown, got: %s", got)
	}
	if !strings.Contains(got, "*italic*") {
		t.Errorf("expected italic in markdown, got: %s", got)
	}
	if !strings.Contains(got, "~~strike~~") {
		t.Errorf("expected strikethrough in markdown, got: %s", got)
	}
	if !strings.Contains(got, "[link](https://example.com)") {
		t.Errorf("expected markdown link, got: %s", got)
	}
	if !strings.Contains(got, "> quote line 1\n> quote line 2") {
		t.Errorf("expected blockquote lines, got: %s", got)
	}
}

func TestIsJunkEntryAndFormatActivityLine(t *testing.T) {
	// IsJunkEntry tests
	if !IsJunkEntry("123") {
		t.Error("expected '123' to be junk (too short and digits)")
	}
	if !IsJunkEntry("1234567") {
		t.Error("expected all digits to be junk")
	}
	if !IsJunkEntry("Read") {
		t.Error("expected single word < 8 chars to be junk")
	}
	if IsJunkEntry("Reading configuration file") {
		t.Error("expected full sentence not to be junk")
	}

	// FormatActivityLine tests
	formatted := FormatActivityLine("Analyzed\npackage.json#L1-75")
	if formatted != "Analyzed package.json#L1-75" {
		t.Errorf("expected collapsed line, got %q", formatted)
	}

	longLine := strings.Repeat("A", 150)
	truncated := FormatActivityLine(longLine)
	if len(truncated) > 120 {
		t.Errorf("expected at most 120 chars, got length %d", len(truncated))
	}
}

func TestSendMsg_ThreeTierFallback(t *testing.T) {
	b := NewBot(BotDeps{})
	mockSender := newMockTelegramSender()
	b.SetTelegramSender(mockSender)
	channel := TelegramChannel{ChatID: 1001, ThreadID: 5}

	// Case 1: First attempt succeeds
	err := b.sendMsg(context.Background(), channel, "<b>Valid HTML</b>", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockSender.sentMessages) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(mockSender.sentMessages))
	}

	// Case 2: Attempt 1 fails, Attempt 2 succeeds with balanced tags
	mockSender.attemptCountHTML = 0
	mockSender.failAttemptsHTML = 1
	mockSender.sentMessages = nil

	err = b.sendMsg(context.Background(), channel, "<b>Unclosed tag", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockSender.sentMessages) != 1 {
		t.Fatalf("expected 1 message sent on auto-fix, got %d", len(mockSender.sentMessages))
	}
	if mockSender.sentMessages[0].Text != "<b>Unclosed tag</b>" {
		t.Errorf("expected auto-balanced tags, got %q", mockSender.sentMessages[0].Text)
	}

	// Case 3: Attempt 1 & 2 fail, Attempt 3 succeeds as plain text
	mockSender.attemptCountHTML = 0
	mockSender.failAttemptsHTML = 2
	mockSender.sentMessages = nil

	err = b.sendMsg(context.Background(), channel, "<b>Broken <invalid entities", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockSender.sentMessages) != 1 {
		t.Fatalf("expected 1 message sent on fallback, got %d", len(mockSender.sentMessages))
	}
	if mockSender.sentMessages[0].ParseMode != "" {
		t.Errorf("expected plain text (no parse_mode), got %q", mockSender.sentMessages[0].ParseMode)
	}
}

func TestSerialTaskQueue(t *testing.T) {
	q := NewSerialTaskQueue("test-queue", "trace-1")
	var executionOrder []int
	var mu sync.Mutex

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 1; i <= 3; i++ {
		taskNum := i
		go func() {
			defer wg.Done()
			_ = q.Enqueue(ctx, fmt.Sprintf("task-%d", taskNum), func(c context.Context) error {
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				executionOrder = append(executionOrder, taskNum)
				mu.Unlock()
				return nil
			})
		}()
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(executionOrder) != 3 {
		t.Fatalf("expected 3 tasks executed, got %d", len(executionOrder))
	}
}

func TestSendPromptToAntigravity_Disconnected(t *testing.T) {
	b := NewBot(BotDeps{})
	mockSender := newMockTelegramSender()
	b.SetTelegramSender(mockSender)
	mockCdp := newMockCdpServiceClient()
	mockCdp.connected = false

	channel := TelegramChannel{ChatID: 2002}
	err := b.SendPromptToAntigravity(context.Background(), channel, "Hello", mockCdp, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockSender.sentMessages) != 1 {
		t.Fatalf("expected 1 connection error message, got %d", len(mockSender.sentMessages))
	}
	if !strings.Contains(mockSender.sentMessages[0].Text, "Connection Error") {
		t.Errorf("expected Connection Error in message, got %q", mockSender.sentMessages[0].Text)
	}
}

func TestSendPromptToAntigravity_InjectionFailure(t *testing.T) {
	b := NewBot(BotDeps{})
	mockSender := newMockTelegramSender()
	b.SetTelegramSender(mockSender)
	mockCdp := newMockCdpServiceClient()
	mockCdp.injectResult = cdp.InjectResult{OK: false, Error: "Target input missing"}

	channel := TelegramChannel{ChatID: 2003}
	err := b.SendPromptToAntigravity(context.Background(), channel, "Hello", mockCdp, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have sent initial status message, then injection failed error embed
	if len(mockSender.sentMessages) < 2 {
		t.Fatalf("expected at least 2 messages (status + error), got %d", len(mockSender.sentMessages))
	}
	lastMsg := mockSender.sentMessages[len(mockSender.sentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Message Injection Failed") {
		t.Errorf("expected Message Injection Failed, got %q", lastMsg.Text)
	}
}

func TestSendChunkedResponse(t *testing.T) {
	b := NewBot(BotDeps{})
	mockSender := newMockTelegramSender()
	b.SetTelegramSender(mockSender)
	channel := TelegramChannel{ChatID: 3001}

	// 1. Short message (<= 4096)
	shortBody := "This is a concise final response."
	mockCdp := newMockCdpServiceClient()
	// Mock response monitor completion
	mockCdp.callResponses["Runtime.evaluate"] = json.RawMessage(`{"result":{"value":""}}`)

	// Let's test sendChunkedResponse directly through SendPromptToAntigravity
	// Or we can invoke SendPromptToAntigravity with immediate stop
	b.AddUserStopRequest(ChannelKey(channel))
	err := b.SendPromptToAntigravity(context.Background(), channel, "Test short", mockCdp, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = shortBody
}

func TestStartPassiveMirrorForUserMessage(t *testing.T) {
	bridgeInst := bridge.InitCdpBridge(false, nil, nil, nil)
	b := NewBot(BotDeps{
		Bridge: bridgeInst,
	})
	mockSender := newMockTelegramSender()
	b.SetTelegramSender(mockSender)

	ch := bridge.TelegramChannel{ChatID: 4004}
	bridge.RegisterApprovalWorkspaceChannel(bridgeInst, "my-ws", ch)

	mockCdp := newMockCdpServiceClient()
	mockCdp.workspaceName = "my-ws"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.StartPassiveMirrorForUserMessage(ctx, "my-ws", mockCdp, bridge.UserMessageInfo{
		Text: "Hello from PC",
	})

	time.Sleep(50 * time.Millisecond)

	mockSender.mu.Lock()
	defer mockSender.mu.Unlock()

	found := false
	for _, msg := range mockSender.sentMessages {
		if strings.Contains(msg.Text, "Sent from PC:") && strings.Contains(msg.Text, "Hello from PC") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'Sent from PC:' notification, got messages: %+v", mockSender.sentMessages)
	}
}
