package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"antigravity-telegram-remote/internal/bot/ui"
	"antigravity-telegram-remote/internal/bridge"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/formatter"
	"antigravity-telegram-remote/internal/logger"
	"antigravity-telegram-remote/internal/monitor"
	"antigravity-telegram-remote/internal/platform"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	MaxOutboundGeneratedImages = 4
	TelegramMsgLimit           = 4096
	MaxInlineChunks            = 5
	LiveResponseMaxLen         = 3800
	MaxProgressBody            = 3500
	MaxProgressEntries         = 60
	ElapsedTimerIntervalMs     = 5000
	MonitorPollIntervalMs      = 2000
	MonitorMaxDurationMs       = 1800000 // 30 minutes
	MonitorStopGoneConfirm     = 3
)

// PhaseIcons maps each execution phase to its corresponding emoji icon.
var PhaseIcons = map[string]string{
	"sending":    "📡",
	"thinking":   "🧠",
	"generating": "✍️",
	"complete":   "✅",
	"timeout":    "⏰",
	"error":      "❌",
}

var (
	tagRegex            = regexp.MustCompile(`<\/?(\w+)(?:\s[^>]*)?\s*\/?>`)
	allTagsRegex        = regexp.MustCompile(`<\/?[^>]+>`)
	nonAlphaRegex       = regexp.MustCompile(`^[^a-zA-Z]+`)
	thinkingRegex       = regexp.MustCompile(`(?i)^thinking\.{0,3}$`)
	thoughtForRegex     = regexp.MustCompile(`(?i)^thought for\s`)
	multipleSpacesRegex = regexp.MustCompile(`\s{2,}`)
	excessNewlinesRegex = regexp.MustCompile(`\n{3,}`)
	imageIntentRegex    = regexp.MustCompile(`(?i)(image|images|png|jpg|jpeg|gif|webp|illustration|diagram|render)`)
	imageURLRegex       = regexp.MustCompile(`(?i)https?://\S+\.(png|jpg|jpeg|gif|webp)`)
	codeBlockWithLangRe = regexp.MustCompile(`(?i)<pre>\s*<code\s+class="language-([^"]*)">([\s\S]*?)<\/code>\s*<\/pre>`)
	codeBlockGenericRe  = regexp.MustCompile(`(?i)<pre[^>]*>([\s\S]*?)<\/pre>`)
	inlineCodeRe        = regexp.MustCompile(`(?i)<code>([\s\S]*?)<\/code>`)
	boldRe              = regexp.MustCompile(`(?i)<b>([\s\S]*?)<\/b>`)
	italicRe            = regexp.MustCompile(`(?i)<i>([\s\S]*?)<\/i>`)
	strikeRe            = regexp.MustCompile(`(?i)<s>([\s\S]*?)<\/s>`)
	linkRe              = regexp.MustCompile(`(?i)<a\s+href="([^"]*)">([\s\S]*?)<\/a>`)
	blockquoteRe        = regexp.MustCompile(`(?i)<blockquote[^>]*>([\s\S]*?)<\/blockquote>`)
)

// EmergencyExtractTextScript extracts raw response text from DOM when normal methods fail.
const EmergencyExtractTextScript = `(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    const scope = panel || document;
    const candidateSelectors = ['.rendered-markdown', '.leading-relaxed.select-text', '.flex.flex-col.gap-y-3', '[data-message-author-role="assistant"]', '[data-message-role="assistant"]', '[class*="assistant-message"]', '[class*="message-content"]', '[class*="markdown-body"]', '.prose'];
    const looksLikeActivity = (text) => { const n = (text || '').trim().toLowerCase(); if (!n) return true; return /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|analyzed|read|wrote|ran)/i.test(n) && n.length <= 220; };
    const clean = (text) => (text || '').replace(/\r/g, '').replace(/\n{3,}/g, '\n\n').trim();
    const candidates = []; const seen = new Set();
    for (const selector of candidateSelectors) { const nodes = scope.querySelectorAll(selector); for (const node of nodes) { if (!node || seen.has(node)) continue; seen.add(node); candidates.push(node); } }
    for (let i = candidates.length - 1; i >= 0; i--) { const node = candidates[i]; const text = clean(node.innerText || node.textContent || ''); if (!text || text.length < 20) continue; if (looksLikeActivity(text)) continue; if (/^(good|bad)$/i.test(text)) continue; return text; }
    return '';
})()`

// ExtractThinkingBlocksScript extracts <details><summary> thinking blocks from Antigravity agent panel DOM.
const ExtractThinkingBlocksScript = `(function() {
    var panel = document.querySelector('.antigravity-agent-side-panel');
    var scope = panel || document;
    var details = scope.querySelectorAll('details');
    var blocks = [];
    for (var i = 0; i < details.length; i++) {
        var d = details[i];
        var summary = d.querySelector('summary');
        if (!summary) continue;
        var rawLabel = (summary.textContent || '').trim();
        var stripped = rawLabel.replace(/^[^a-zA-Z]+/, '');
        if (!/^(?:thought for|thinking)\b/i.test(stripped)) continue;
        var wasOpen = d.open;
        if (!wasOpen) d.open = true;
        var children = d.children;
        var parts = [];
        for (var c = 0; c < children.length; c++) {
            if (children[c].tagName === 'SUMMARY' || children[c].tagName === 'STYLE') continue;
            var t = (children[c].innerText || children[c].textContent || '').trim();
            if (t && t.length >= 5) parts.push(t);
        }
        if (parts.length === 0) {
            var fullText = (d.innerText || d.textContent || '').trim();
            var bodyText = fullText.replace(rawLabel, '').trim();
            if (bodyText && bodyText.length >= 5) parts.push(bodyText);
        }
        if (!wasOpen) d.open = false;
        blocks.push({ label: rawLabel, body: parts.join('\n\n') });
    }
    return blocks;
})()`

// ProgressEntryKind identifies kind of progress entry in the event stream.
type ProgressEntryKind string

const (
	ProgressKindThought        ProgressEntryKind = "thought"
	ProgressKindThoughtContent ProgressEntryKind = "thought-content"
	ProgressKindActivity       ProgressEntryKind = "activity"
)

// ProgressEntry represents a progress log item.
type ProgressEntry struct {
	Kind ProgressEntryKind
	Text string
}

// ThinkingBlock represents an extracted thinking block from Antigravity DOM.
type ThinkingBlock struct {
	Label string `json:"label"`
	Body  string `json:"body"`
}

// PromptSendOptions contains optional session and topic management dependencies.
type PromptSendOptions struct {
	ChatSessionService *cdp.ChatSessionService
	ChatSessionRepo    *database.ChatSessionRepository
	TopicManager       *TelegramTopicManager
	TitleGenerator     *bridge.TitleGeneratorService
}

// TelegramSender defines Telegram message operations required by streaming and callbacks.
type TelegramSender interface {
	SendMessage(ctx context.Context, params *tgbot.SendMessageParams) (*models.Message, error)
	EditMessageText(ctx context.Context, params *tgbot.EditMessageTextParams) (*models.Message, error)
	EditMessageReplyMarkup(ctx context.Context, params *tgbot.EditMessageReplyMarkupParams) (*models.Message, error)
	SendPhoto(ctx context.Context, params *tgbot.SendPhotoParams) (*models.Message, error)
	SendDocument(ctx context.Context, params *tgbot.SendDocumentParams) (*models.Message, error)
	AnswerCallbackQuery(ctx context.Context, params *tgbot.AnswerCallbackQueryParams) (bool, error)
	DeleteForumTopic(ctx context.Context, params *tgbot.DeleteForumTopicParams) (bool, error)
	CloseForumTopic(ctx context.Context, params *tgbot.CloseForumTopicParams) (bool, error)
}

// CdpServiceClient defines CDP methods needed for prompt injection and DOM querying.
type CdpServiceClient interface {
	IsConnected() bool
	GetCurrentModel() *string
	InjectMessage(text string) (cdp.InjectResult, error)
	InjectMessageWithImageFiles(text string, imageFilePaths []string) (cdp.InjectResult, error)
	GetPrimaryContextID() *int
	Call(method string, params any) (json.RawMessage, error)
	ExtractLatestResponseImages(maxImages int) []cdp.ExtractedResponseImage
	GetCurrentWorkspaceName() string
	GetContexts() []cdp.Context
	WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool
}

// SerialTaskQueue executes asynchronous tasks in strict FIFO order per category name.
type SerialTaskQueue struct {
	mu      sync.Mutex
	name    string
	traceID string
	seq     int
	tail    chan struct{}
}

// NewSerialTaskQueue creates a new SerialTaskQueue instance.
func NewSerialTaskQueue(queueName, traceID string) *SerialTaskQueue {
	done := make(chan struct{})
	close(done)
	return &SerialTaskQueue{
		name:    queueName,
		traceID: traceID,
		tail:    done,
	}
}

// Enqueue enqueues and runs fn sequentially, waiting for completion or cancellation.
func (q *SerialTaskQueue) Enqueue(ctx context.Context, label string, fn func(ctx context.Context) error) error {
	q.mu.Lock()
	q.seq++
	seq := q.seq
	prev := q.tail
	curr := make(chan struct{})
	var taskErr error
	q.tail = curr
	q.mu.Unlock()

	go func() {
		defer close(curr)
		if prev != nil {
			<-prev
		}
		if ctx.Err() != nil {
			taskErr = ctx.Err()
			return
		}
		if err := fn(ctx); err != nil {
			logger.Error(fmt.Sprintf("[sendQueue:%s:%s] error #%d label=%s: %v", q.traceID, q.name, seq, label, err))
			taskErr = err
		}
	}()

	select {
	case <-curr:
		return taskErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// StripHTMLForFile converts Telegram HTML to readable Markdown for .md file attachments.
func StripHTMLForFile(htmlStr string) string {
	text := htmlStr
	text = codeBlockWithLangRe.ReplaceAllString(text, "\n```$1\n$2\n```\n")
	text = codeBlockGenericRe.ReplaceAllString(text, "\n```\n$1\n```\n")
	text = inlineCodeRe.ReplaceAllString(text, "`$1`")
	text = boldRe.ReplaceAllString(text, "**$1**")
	text = italicRe.ReplaceAllString(text, "*$1*")
	text = strikeRe.ReplaceAllString(text, "~~$1~~")
	text = linkRe.ReplaceAllString(text, "[$2]($1)")

	text = blockquoteRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := blockquoteRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		content := sub[1]
		lines := strings.Split(content, "\n")
		var out []string
		for _, l := range lines {
			out = append(out, "> "+l)
		}
		return strings.Join(out, "\n")
	})

	text = allTagsRegex.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = excessNewlinesRegex.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// BalanceHTMLTags ensures all opened HTML tags are closed in reverse order, and stray closing tags discarded.
func BalanceHTMLTags(htmlStr string) string {
	matches := tagRegex.FindAllStringSubmatchIndex(htmlStr, -1)
	if len(matches) == 0 {
		return htmlStr
	}

	var openTags []string
	type part struct {
		content string
	}
	var parts []part
	lastIdx := 0

	for _, loc := range matches {
		start := loc[0]
		end := loc[1]
		if start > lastIdx {
			parts = append(parts, part{content: htmlStr[lastIdx:start]})
		}
		fullTag := htmlStr[start:end]
		tagSub := htmlStr[loc[2]:loc[3]]
		tagName := strings.ToLower(tagSub)
		isStart := !strings.HasPrefix(fullTag, "</") && !strings.HasSuffix(fullTag, "/>")
		isSelfClosing := strings.HasSuffix(fullTag, "/>") || tagName == "br" || tagName == "hr" || tagName == "img"

		if isSelfClosing {
			parts = append(parts, part{content: fullTag})
		} else if isStart {
			openTags = append(openTags, tagName)
			parts = append(parts, part{content: fullTag})
		} else {
			// Closing tag: find last matching open tag
			idx := -1
			for i := len(openTags) - 1; i >= 0; i-- {
				if openTags[i] == tagName {
					idx = i
					break
				}
			}
			if idx != -1 {
				for len(openTags) > idx {
					last := openTags[len(openTags)-1]
					openTags = openTags[:len(openTags)-1]
					parts = append(parts, part{content: "</" + last + ">"})
				}
			}
		}
		lastIdx = end
	}

	if lastIdx < len(htmlStr) {
		parts = append(parts, part{content: htmlStr[lastIdx:]})
	}

	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p.content)
	}
	for i := len(openTags) - 1; i >= 0; i-- {
		sb.WriteString("</" + openTags[i] + ">")
	}

	return sb.String()
}

// StripHTML removes all tags and decodes common HTML entities for plain text fallback.
func StripHTML(htmlStr string) string {
	res := allTagsRegex.ReplaceAllString(htmlStr, "")
	res = strings.ReplaceAll(res, "&amp;", "&")
	res = strings.ReplaceAll(res, "&lt;", "<")
	res = strings.ReplaceAll(res, "&gt;", ">")
	res = strings.ReplaceAll(res, "&quot;", "\"")
	res = strings.ReplaceAll(res, "&#39;", "'")
	return res
}

// IsJunkEntry returns true if text is too short, numeric, or a single short word.
func IsJunkEntry(text string) bool {
	t := strings.TrimSpace(text)
	if len(t) < 5 {
		return true
	}
	isAllDigits := true
	for _, r := range t {
		if r < '0' || r > '9' {
			isAllDigits = false
			break
		}
	}
	if isAllDigits {
		return true
	}
	if !strings.ContainsAny(t, " \t\r\n") && len(t) < 8 {
		return true
	}
	return false
}

// FormatActivityLine collapses spaces/newlines and applies HTML escaping to at most 120 chars.
func FormatActivityLine(raw string) string {
	collapsed := strings.ReplaceAll(raw, "\r", "")
	collapsed = strings.ReplaceAll(collapsed, "\n", " ")
	collapsed = multipleSpacesRegex.ReplaceAllString(collapsed, " ")
	collapsed = strings.TrimSpace(collapsed)
	if collapsed == "" || IsJunkEntry(collapsed) {
		return ""
	}
	runes := []rune(collapsed)
	if len(runes) > 120 {
		runes = runes[:120]
	}
	return ui.EscapeHTML(string(runes))
}

func (b *Bot) getSender() TelegramSender {
	b.stateMu.RLock()
	defer b.stateMu.RUnlock()
	if b.telegramSender != nil {
		return b.telegramSender
	}
	return b.TgBot
}

// SetTelegramSender overrides the TelegramSender implementation (useful for unit tests).
func (b *Bot) SetTelegramSender(sender TelegramSender) {
	b.stateMu.Lock()
	defer b.stateMu.Unlock()
	b.telegramSender = sender
}

func (b *Bot) sendEmbed(ctx context.Context, channel TelegramChannel, title, description string) error {
	text := fmt.Sprintf("<b>%s</b>\n\n%s", ui.EscapeHTML(title), ui.EscapeHTML(description))
	return b.sendMsg(ctx, channel, text, nil, nil)
}

func (b *Bot) sendMsg(ctx context.Context, channel TelegramChannel, text string, keyboard *ui.InlineKeyboardMarkup, outMsgID *int) error {
	sender := b.getSender()
	if sender == nil {
		return nil
	}

	params := &tgbot.SendMessageParams{
		ChatID:    channel.ChatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if channel.ThreadID != 0 {
		params.MessageThreadID = channel.ThreadID
	}
	if keyboard != nil {
		params.ReplyMarkup = keyboard.ToTelegramModel()
	}

	// Attempt 1: Default HTML
	msg, err := sender.SendMessage(ctx, params)
	if err == nil {
		if outMsgID != nil && msg != nil {
			*outMsgID = msg.ID
		}
		return nil
	}

	logger.Warn(fmt.Sprintf("[sendMsg] HTML attempt 1 failed (%v). Attempting auto-fix...", err))

	// Attempt 2: Balanced tags
	fixed := BalanceHTMLTags(text)
	params.Text = fixed
	msg, err = sender.SendMessage(ctx, params)
	if err == nil {
		if outMsgID != nil && msg != nil {
			*outMsgID = msg.ID
		}
		return nil
	}

	logger.Error(fmt.Sprintf("[sendMsg] HTML attempt 2 (fixed) failed: %v. Falling back to plain text.", err))

	// Attempt 3: Plain text fallback
	plain := StripHTML(text)
	params.Text = plain
	params.ParseMode = ""
	msg, err = sender.SendMessage(ctx, params)
	if err == nil {
		if outMsgID != nil && msg != nil {
			*outMsgID = msg.ID
		}
		return nil
	}

	logger.Error(fmt.Sprintf("[sendMsg] Critical failure in fallback: %v", err))
	return err
}

func (b *Bot) editMsg(ctx context.Context, channel TelegramChannel, msgID int, text string) error {
	sender := b.getSender()
	if sender == nil {
		return nil
	}
	truncated := text
	if len(truncated) > TelegramMsgLimit {
		truncated = truncated[:TelegramMsgLimit-20] + "\n...(truncated)"
	}
	params := &tgbot.EditMessageTextParams{
		ChatID:    channel.ChatID,
		MessageID: msgID,
		Text:      truncated,
		ParseMode: models.ParseModeHTML,
	}
	_, err := sender.EditMessageText(ctx, params)
	if err != nil && !strings.Contains(err.Error(), "message is not modified") {
		logger.Error(fmt.Sprintf("[editMsg] Failed: %v", err))
	}
	return err
}

func (b *Bot) tryEmergencyExtractText(ctx context.Context, cdpSvc CdpServiceClient) string {
	if cdpSvc == nil {
		return ""
	}
	evalParams := map[string]any{
		"expression":    EmergencyExtractTextScript,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if ctxID := cdpSvc.GetPrimaryContextID(); ctxID != nil {
		evalParams["contextId"] = *ctxID
	}
	resRaw, err := cdpSvc.Call("Runtime.evaluate", evalParams)
	if err != nil {
		return ""
	}
	var resp struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &resp); err == nil {
		return strings.TrimSpace(resp.Result.Value)
	}
	return ""
}

type responseMonitorCdpAdapter struct {
	client CdpServiceClient
}

func (a *responseMonitorCdpAdapter) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	if a.client == nil {
		return nil, fmt.Errorf("cdp client is nil")
	}
	resRaw, err := a.client.Call(method, params)
	if err != nil {
		return nil, err
	}
	var res map[string]any
	if err := json.Unmarshal(resRaw, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (a *responseMonitorCdpAdapter) GetPrimaryContextID() *int {
	if a.client == nil {
		return nil
	}
	return a.client.GetPrimaryContextID()
}

func (a *responseMonitorCdpAdapter) On(event string, listener func(params map[string]any)) {
	if emitter, ok := a.client.(interface {
		On(event string, handler cdp.EventHandler)
	}); ok {
		emitter.On(event, func(data any) {
			if m, ok := data.(map[string]any); ok {
				listener(m)
			}
		})
	}
}

func (a *responseMonitorCdpAdapter) Off(event string, listener func(params map[string]any)) {}

// SendPromptToAntigravity sends a prompt to Antigravity and streams real-time progress.
func (b *Bot) SendPromptToAntigravity(
	ctx context.Context,
	channel TelegramChannel,
	prompt string,
	cdpSvc CdpServiceClient,
	inboundImages []InboundImageAttachment,
	options *PromptSendOptions,
) error {
	monitorTraceID := ChannelKey(channel)
	enqueueGeneral := NewSerialTaskQueue("general", monitorTraceID)
	enqueueResponse := NewSerialTaskQueue("response", monitorTraceID)
	enqueueActivity := NewSerialTaskQueue("activity", monitorTraceID)

	if cdpSvc == nil || !cdpSvc.IsConnected() {
		_ = enqueueGeneral.Enqueue(ctx, "send-embed", func(c context.Context) error {
			return b.sendEmbed(c, channel,
				PhaseIcons["error"]+" Connection Error",
				fmt.Sprintf("Not connected to Antigravity.\nStart with `%s`, then send a message to auto-connect.", platform.GetAntigravityCdpHint(9223)),
			)
		})
		return nil
	}

	localMode := "fast"
	if b.ModeService != nil {
		localMode = string(b.ModeService.GetCurrentMode())
	}
	modeName := ui.ModeDisplayNames[localMode]
	if modeName == "" {
		modeName = localMode
	}

	modelLabel := "default"
	if cur := cdpSvc.GetCurrentModel(); cur != nil && *cur != "" {
		modelLabel = *cur
	} else if b.ModelService != nil && b.ModelService.GetCurrentModel() != "" {
		modelLabel = string(b.ModelService.GetCurrentModel())
	}

	var liveActivityMsgID int
	sendingText := fmt.Sprintf("<b>%s %s · %s</b>\n\n<i>Sending...</i>",
		PhaseIcons["sending"], ui.EscapeHTML(modeName), ui.EscapeHTML(modelLabel))
	_ = b.sendMsg(ctx, channel, sendingText, nil, &liveActivityMsgID)

	var (
		mu                        sync.Mutex
		isFinalized               bool
		liveResponseMsgID         int
		lastLiveResponseKey       string
		lastLiveActivityKey       string
		liveResponseUpdateVersion int
		liveActivityUpdateVersion int
		lastProgressText          string
		progressLog               []ProgressEntry
		thinkingActive            bool
		thinkingContentParts      []string
		lastThoughtLabel          string
	)

	trimProgressLog := func() {
		if len(progressLog) > MaxProgressEntries {
			progressLog = progressLog[len(progressLog)-MaxProgressEntries:]
		}
	}

	buildProgressBody := func() string {
		var lines []string
		for _, e := range progressLog {
			switch e.Kind {
			case ProgressKindThought:
				lines = append(lines, fmt.Sprintf("💭 <i>%s</i>", ui.EscapeHTML(e.Text)))
			case ProgressKindThoughtContent:
				lines = append(lines, fmt.Sprintf("<i>%s</i>", ui.EscapeHTML(e.Text)))
			case ProgressKindActivity:
				lines = append(lines, e.Text)
			}
		}
		if thinkingActive {
			lines = append(lines, "💭 <i>Thinking...</i>")
		}
		body := strings.Join(lines, "\n\n")
		if len(body) > MaxProgressBody {
			body = "...\n\n" + body[len(body)-MaxProgressBody+5:]
		}
		if body == "" {
			return "<i>Generating...</i>"
		}
		return body
	}

	buildProgressMessage := func(title, footer string) string {
		body := buildProgressBody()
		footerLine := ""
		if footer != "" {
			footerLine = fmt.Sprintf("\n\n<i>%s</i>", ui.EscapeHTML(footer))
		}
		return fmt.Sprintf("<b>%s</b>\n\n%s%s", ui.EscapeHTML(title), body, footerLine)
	}

	buildLiveResponseText := func(title, rawText, footer string, isAlreadyHTML, skipTruncation bool) string {
		normalized := strings.TrimSpace(rawText)
		var body string
		if normalized != "" {
			if isAlreadyHTML {
				body = normalized
			} else {
				body = formatter.FormatForTelegram(normalized)
			}
		} else {
			body = "Generating..."
		}

		truncated := body
		if !skipTruncation && len(body) > LiveResponseMaxLen {
			rawSlice := body[len(body)-LiveResponseMaxLen+30:]
			if isAlreadyHTML || strings.Contains(body, "<") {
				truncated = "...(beginning truncated)\n" + BalanceHTMLTags(rawSlice)
			} else {
				truncated = "...(beginning truncated)\n" + rawSlice
			}
		}

		titleLine := ""
		if title != "" {
			titleLine = fmt.Sprintf("<b>%s</b>\n\n", ui.EscapeHTML(title))
		}
		footerLine := ""
		if footer != "" {
			footerLine = fmt.Sprintf("\n\n<i>%s</i>", ui.EscapeHTML(footer))
		}
		return fmt.Sprintf("%s%s%s", titleLine, truncated, footerLine)
	}

	type upsertOpts struct {
		expectedVersion   *int
		skipWhenFinalized bool
		isAlreadyHTML     bool
		skipTruncation    bool
	}

	upsertLiveResponse := func(title, rawText, footer string, opts *upsertOpts) error {
		return enqueueResponse.Enqueue(ctx, "upsert-response", func(c context.Context) error {
			mu.Lock()
			if opts != nil && opts.skipWhenFinalized && isFinalized {
				mu.Unlock()
				return nil
			}
			if opts != nil && opts.expectedVersion != nil && *opts.expectedVersion != liveResponseUpdateVersion {
				mu.Unlock()
				return nil
			}
			mu.Unlock()

			isAlreadyHTML := opts != nil && opts.isAlreadyHTML
			skipTruncation := opts != nil && opts.skipTruncation
			text := buildLiveResponseText(title, rawText, footer, isAlreadyHTML, skipTruncation)

			prefix200 := rawText
			if len(prefix200) > 200 {
				prefix200 = prefix200[:200]
			}
			renderKey := fmt.Sprintf("%s|%s|%s", title, prefix200, footer)

			mu.Lock()
			if renderKey == lastLiveResponseKey && liveResponseMsgID != 0 {
				mu.Unlock()
				return nil
			}
			lastLiveResponseKey = renderKey
			msgID := liveResponseMsgID
			mu.Unlock()

			if msgID != 0 {
				return b.editMsg(c, channel, msgID, text)
			}
			var newID int
			err := b.sendMsg(c, channel, text, nil, &newID)
			if err == nil && newID != 0 {
				mu.Lock()
				liveResponseMsgID = newID
				mu.Unlock()
			}
			return err
		})
	}

	refreshProgress := func(title, footer string, expectedVersion *int, skipWhenFinalized bool) error {
		return enqueueActivity.Enqueue(ctx, "upsert-activity", func(c context.Context) error {
			mu.Lock()
			if skipWhenFinalized && isFinalized {
				mu.Unlock()
				return nil
			}
			if expectedVersion != nil && *expectedVersion != liveActivityUpdateVersion {
				mu.Unlock()
				return nil
			}
			text := buildProgressMessage(title, footer)
			bodySnap := fmt.Sprintf("%d|%v|%s|%s", len(progressLog), thinkingActive, title, footer)
			if bodySnap == lastLiveActivityKey && liveActivityMsgID != 0 {
				mu.Unlock()
				return nil
			}
			lastLiveActivityKey = bodySnap
			msgID := liveActivityMsgID
			mu.Unlock()

			if msgID != 0 {
				return b.editMsg(c, channel, msgID, text)
			}
			var newID int
			err := b.sendMsg(c, channel, text, nil, &newID)
			if err == nil && newID != 0 {
				mu.Lock()
				liveActivityMsgID = newID
				mu.Unlock()
			}
			return err
		})
	}

	setProgressMessage := func(htmlContent string, expectedVersion int) error {
		return enqueueActivity.Enqueue(ctx, "upsert-activity", func(c context.Context) error {
			mu.Lock()
			if expectedVersion != liveActivityUpdateVersion {
				mu.Unlock()
				return nil
			}
			preview := htmlContent
			if len(preview) > 200 {
				preview = preview[:200]
			}
			lastLiveActivityKey = preview
			msgID := liveActivityMsgID
			mu.Unlock()

			if msgID != 0 {
				return b.editMsg(c, channel, msgID, htmlContent)
			}
			var newID int
			err := b.sendMsg(c, channel, htmlContent, nil, &newID)
			if err == nil && newID != 0 {
				mu.Lock()
				liveActivityMsgID = newID
				mu.Unlock()
			}
			return err
		})
	}

	sendChunkedResponse := func(title, footer, rawBody string, isAlreadyHTML bool) error {
		var formattedBody string
		if isAlreadyHTML {
			formattedBody = rawBody
		} else {
			formattedBody = formatter.FormatForTelegram(rawBody)
		}

		titleLine := ""
		if title != "" {
			titleLine = fmt.Sprintf("<b>%s</b>\n\n", ui.EscapeHTML(title))
		}
		footerLine := ""
		if footer != "" {
			footerLine = fmt.Sprintf("\n\n<i>%s</i>", ui.EscapeHTML(footer))
		}
		fullMsg := fmt.Sprintf("%s%s%s", titleLine, formattedBody, footerLine)

		if len(fullMsg) <= TelegramMsgLimit {
			mu.Lock()
			v := liveResponseUpdateVersion
			mu.Unlock()
			return upsertLiveResponse(title, rawBody, footer, &upsertOpts{
				expectedVersion: &v,
				isAlreadyHTML:   isAlreadyHTML,
				skipTruncation:  true,
			})
		}

		bodyChunks := formatter.SplitTelegramHtml(formattedBody, TelegramMsgLimit-200)
		inlineCount := len(bodyChunks)
		if inlineCount > MaxInlineChunks {
			inlineCount = MaxInlineChunks
		}
		hasFile := len(bodyChunks) > MaxInlineChunks
		total := len(bodyChunks)
		if hasFile {
			total = inlineCount
		}

		for pi := 0; pi < inlineCount; pi++ {
			var partLabel string
			if hasFile {
				partLabel = fmt.Sprintf("(%d/%d+file)", pi+1, inlineCount)
			} else {
				partLabel = fmt.Sprintf("(%d/%d)", pi+1, total)
			}

			if pi == 0 {
				firstTitle := partLabel
				if title != "" {
					firstTitle = fmt.Sprintf("%s %s", title, partLabel)
				}
				mu.Lock()
				v := liveResponseUpdateVersion
				mu.Unlock()
				_ = upsertLiveResponse(firstTitle, bodyChunks[pi], footer, &upsertOpts{
					expectedVersion: &v,
					isAlreadyHTML:   true,
					skipTruncation:  true,
				})
			} else {
				partFooter := partLabel
				if footer != "" {
					partFooter = fmt.Sprintf("%s %s", ui.EscapeHTML(footer), partLabel)
				}
				_ = b.sendMsg(ctx, channel, fmt.Sprintf("%s\n\n<i>%s</i>", bodyChunks[pi], partFooter), nil, nil)
			}
		}

		if hasFile {
			fileContent := StripHTMLForFile(formattedBody)
			buf := []byte(fileContent)
			timestamp := time.Now().UTC().Format("2006-01-02T15-04-05")
			filename := fmt.Sprintf("response-%s.md", timestamp)
			sender := b.getSender()
			if sender != nil {
				params := &tgbot.SendDocumentParams{
					ChatID:  channel.ChatID,
					Caption: fmt.Sprintf("📄 Full response (%d chars)", len(rawBody)),
					Document: &models.InputFileUpload{
						Filename: filename,
						Data:     bytes.NewReader(buf),
					},
				}
				if channel.ThreadID != 0 {
					params.MessageThreadID = channel.ThreadID
				}
				_, err := sender.SendDocument(ctx, params)
				if err != nil {
					logger.Error(fmt.Sprintf("[sendPrompt] Failed to send response file: %v", err))
				}
			}
		}

		return nil
	}

	sendGeneratedImages := func(responseText string) {
		if !imageIntentRegex.MatchString(prompt) && !strings.Contains(responseText, "![") && !imageURLRegex.MatchString(responseText) {
			return
		}
		extracted := cdpSvc.ExtractLatestResponseImages(MaxOutboundGeneratedImages)
		if len(extracted) == 0 {
			return
		}
		sender := b.getSender()
		if sender == nil {
			return
		}

		for i, img := range extracted {
			botImg := ExtractedResponseImage{
				Name:       img.Name,
				MimeType:   img.MimeType,
				Base64Data: img.Base64Data,
				URL:        img.URL,
			}
			file := ToTelegramInputFile(ctx, botImg, i)
			if file != nil {
				caption := fmt.Sprintf("🖼️ Generated image (%d/%d)", i+1, len(extracted))
				params := &tgbot.SendPhotoParams{
					ChatID:  channel.ChatID,
					Caption: caption,
					Photo: &models.InputFileUpload{
						Filename: file.Name,
						Data:     bytes.NewReader(file.Buffer),
					},
				}
				if channel.ThreadID != 0 {
					params.MessageThreadID = channel.ThreadID
				}
				_, err := sender.SendPhoto(ctx, params)
				if err != nil {
					logger.Error(fmt.Sprintf("[sendGeneratedImages] Failed: %v", err))
				}
			}
		}
	}

	// Step 3: Message injection
	var injectResult cdp.InjectResult
	var injectErr error

	if len(inboundImages) > 0 {
		var localPaths []string
		for _, img := range inboundImages {
			if img.LocalPath != "" {
				localPaths = append(localPaths, img.LocalPath)
			}
		}
		injectResult, injectErr = cdpSvc.InjectMessageWithImageFiles(prompt, localPaths)
		if injectErr != nil || !injectResult.OK {
			_ = enqueueGeneral.Enqueue(ctx, "send-embed", func(c context.Context) error {
				return b.sendEmbed(c, channel, "🖼️ Attached image fallback", "Failed to attach image directly, resending via URL reference.")
			})
			fallbackPrompt := BuildPromptWithAttachmentUrls(prompt, inboundImages)
			injectResult, injectErr = cdpSvc.InjectMessage(fallbackPrompt)
		}
	} else {
		injectResult, injectErr = cdpSvc.InjectMessage(prompt)
	}

	if injectErr != nil || !injectResult.OK {
		mu.Lock()
		isFinalized = true
		mu.Unlock()
		errMsg := "Unknown error"
		if injectErr != nil {
			errMsg = injectErr.Error()
		} else if injectResult.Error != "" {
			errMsg = injectResult.Error
		}
		_ = enqueueGeneral.Enqueue(ctx, "send-embed", func(c context.Context) error {
			return b.sendEmbed(c, channel, fmt.Sprintf("%s Message Injection Failed", PhaseIcons["error"]), fmt.Sprintf("Failed to send message: %s", errMsg))
		})
		return nil
	}

	startTime := time.Now()
	progressTitle := func() string {
		return fmt.Sprintf("%s %s", PhaseIcons["thinking"], modelLabel)
	}
	progressFooter := func() string {
		return fmt.Sprintf("⏱️ %ds", int(time.Since(startTime).Seconds()))
	}

	triggerProgressRefresh := func() {
		mu.Lock()
		liveActivityUpdateVersion++
		v := liveActivityUpdateVersion
		mu.Unlock()
		go func() {
			_ = refreshProgress(progressTitle(), progressFooter(), &v, true)
		}()
	}

	_ = refreshProgress(progressTitle(), progressFooter(), nil, false)

	monitorAdapter := &responseMonitorCdpAdapter{client: cdpSvc}
	tickerDone := make(chan struct{})

	var respMonitor *monitor.ResponseMonitor
	respMonitor = monitor.NewResponseMonitor(monitor.ResponseMonitorOptions{
		CdpService:           monitorAdapter,
		PollIntervalMs:       MonitorPollIntervalMs,
		MaxDurationMs:        MonitorMaxDurationMs,
		StopGoneConfirmCount: MonitorStopGoneConfirm,
		OnProcessLog: func(logText string) {
			mu.Lock()
			if isFinalized {
				mu.Unlock()
				return
			}
			mu.Unlock()
			trimmed := strings.TrimSpace(logText)
			if trimmed == "" || IsJunkEntry(trimmed) {
				return
			}
			formatted := FormatActivityLine(trimmed)
			if formatted != "" {
				mu.Lock()
				progressLog = append(progressLog, ProgressEntry{Kind: ProgressKindActivity, Text: formatted})
				trimProgressLog()
				mu.Unlock()
				triggerProgressRefresh()
			}
		},
		OnThinkingLog: func(thinkingText string) {
			mu.Lock()
			if isFinalized {
				mu.Unlock()
				return
			}
			mu.Unlock()
			trimmed := strings.TrimSpace(thinkingText)
			if trimmed == "" {
				return
			}
			logger.Debug(fmt.Sprintf("[Bot] onThinkingLog received: %.100s", trimmed))
			stripped := nonAlphaRegex.ReplaceAllString(trimmed, "")

			mu.Lock()
			if thinkingRegex.MatchString(stripped) {
				thinkingActive = true
			} else if thoughtForRegex.MatchString(stripped) {
				thinkingActive = false
				lastThoughtLabel = trimmed
				progressLog = append(progressLog, ProgressEntry{Kind: ProgressKindThought, Text: trimmed})
				trimProgressLog()
			} else {
				thinkingContentParts = append(thinkingContentParts, trimmed)
				firstLine := strings.TrimSpace(strings.Split(trimmed, "\n")[0])
				heading := firstLine
				if len(heading) > 60 {
					heading = heading[:57] + "..."
				}
				merged := false
				for i := len(progressLog) - 1; i >= 0; i-- {
					if progressLog[i].Kind == ProgressKindThought {
						if !strings.Contains(progressLog[i].Text, " — ") {
							progressLog[i].Text += " — " + heading
							merged = true
						}
						break
					}
				}
				if !merged && len(heading) > 10 {
					progressLog = append(progressLog, ProgressEntry{Kind: ProgressKindThoughtContent, Text: heading})
					trimProgressLog()
				}
			}
			mu.Unlock()
			triggerProgressRefresh()
		},
		OnProgress: func(text string) {
			mu.Lock()
			if isFinalized {
				mu.Unlock()
				return
			}
			mu.Unlock()
			src := respMonitor.GetLastExtractionSource()
			isStructured := src != nil && *src == monitor.SourceStructured
			var output string
			if isStructured {
				output = text
			} else {
				sep := formatter.SplitOutputAndLogs(text)
				output = sep.Output
			}
			if strings.TrimSpace(output) != "" {
				mu.Lock()
				lastProgressText = output
				mu.Unlock()
			}
		},
		OnComplete: func(finalText string, meta *monitor.CompleteMeta) {
			mu.Lock()
			if isFinalized {
				mu.Unlock()
				return
			}
			isFinalized = true
			mu.Unlock()

			close(tickerDone)

			wasStopped := b.ConsumeUserStopRequest(ChannelKey(channel))
			if wasStopped {
				logger.Info(fmt.Sprintf("[sendPrompt:%s] Stopped by user", monitorTraceID))
				_ = b.sendMsg(ctx, channel, "⏹️ Generation stopped.", nil, nil)
				return
			}

			elapsed := int(time.Since(startTime).Seconds())
			isQuotaError := respMonitor.GetPhase() == monitor.PhaseQuotaReached || respMonitor.GetQuotaDetected()

			if isQuotaError {
				mu.Lock()
				liveActivityUpdateVersion++
				vAct := liveActivityUpdateVersion
				thinkingActive = false
				quotaBody := buildProgressBody()
				mu.Unlock()

				quotaActivityMsg := fmt.Sprintf("<b>⚠️ %s · Quota Reached</b>\n\n%s\n\n<i>⏱️ %ds</i>",
					ui.EscapeHTML(modelLabel), quotaBody, elapsed)
				_ = setProgressMessage(quotaActivityMsg, vAct)

				mu.Lock()
				liveResponseUpdateVersion++
				vResp := liveResponseUpdateVersion
				mu.Unlock()

				_ = upsertLiveResponse("⚠️ Quota Reached",
					"Model quota limit reached. Please wait or switch to a different model.",
					fmt.Sprintf("⏱️ %ds", elapsed), &upsertOpts{expectedVersion: &vResp})

				if poolClient, ok := cdpSvc.(cdp.PoolClient); ok {
					var fetchQuotaFn func(ctx context.Context) ([]ui.ModelQuotaEntry, error)
					if b.Bridge != nil && b.Bridge.Quota != nil {
						fetchQuotaFn = func(ctx context.Context) ([]ui.ModelQuotaEntry, error) {
							res, err := b.Bridge.Quota.FetchQuota(ctx)
							if err != nil {
								return nil, err
							}
							if list, ok := res.([]ui.ModelQuotaEntry); ok {
								return list, nil
							}
							return nil, nil
						}
					}
					modelsUI, err := ui.BuildModelsUI(ctx, modelsCdpUIAdapter{client: poolClient}, fetchQuotaFn)
					if err == nil && modelsUI != nil {
						_ = b.sendMsg(ctx, channel, modelsUI.Text, modelsUI.Keyboard, nil)
					}
				}
				return
			}

			freshText := ""
			freshIsHTML := false
			ctxID := cdpSvc.GetPrimaryContextID()
			evalParams := map[string]any{
				"expression":    monitor.ExtractAssistantSegmentsPayloadScript(),
				"returnByValue": true,
				"awaitPromise":  true,
			}
			if ctxID != nil {
				evalParams["contextId"] = *ctxID
			}
			resRaw, err := cdpSvc.Call("Runtime.evaluate", evalParams)
			if err == nil {
				var evalResp struct {
					Result struct {
						Value any `json:"value"`
					} `json:"result"`
				}
				if err := json.Unmarshal(resRaw, &evalResp); err == nil {
					classified := monitor.ClassifyAssistantSegments(evalResp.Result.Value)
					if classified.Diagnostics.Source == "dom-structured" && strings.TrimSpace(classified.FinalOutputText) != "" {
						freshText = strings.TrimSpace(classified.FinalOutputText)
						freshIsHTML = true
					}
				}
			}

			mu.Lock()
			polledText := finalText
			if strings.TrimSpace(polledText) == "" {
				polledText = lastProgressText
			}
			bestPolled := strings.TrimSpace(polledText)
			mu.Unlock()

			var finalResponseText string
			var isAlreadyHTML bool

			if freshText != "" && len(freshText) >= len(bestPolled) {
				finalResponseText = freshText
				isAlreadyHTML = freshIsHTML
			} else if bestPolled != "" {
				finalResponseText = bestPolled
				if meta != nil && meta.Source == monitor.SourceStructured {
					isAlreadyHTML = true
				}
			} else {
				finalResponseText = b.tryEmergencyExtractText(ctx, cdpSvc)
				isAlreadyHTML = false
			}

			var finalOutputText string
			if isAlreadyHTML {
				finalOutputText = finalResponseText
			} else {
				sep := formatter.SplitOutputAndLogs(finalResponseText)
				if sep.Output != "" {
					finalOutputText = sep.Output
				} else {
					finalOutputText = finalResponseText
				}
			}

			// Collapsible thinking block
			thinkParams := map[string]any{
				"expression":    ExtractThinkingBlocksScript,
				"returnByValue": true,
			}
			if ctxID != nil {
				thinkParams["contextId"] = *ctxID
			}
			thinkResRaw, err := cdpSvc.Call("Runtime.evaluate", thinkParams)
			if err == nil {
				var thinkResp struct {
					Result struct {
						Value []ThinkingBlock `json:"value"`
					} `json:"result"`
				}
				if err := json.Unmarshal(thinkResRaw, &thinkResp); err == nil && len(thinkResp.Result.Value) > 0 {
					mu.Lock()
					accumulatedBody := strings.Join(thinkingContentParts, "\n\n")
					labelFallback := lastThoughtLabel
					if labelFallback == "" {
						labelFallback = "Thinking"
					}
					mu.Unlock()

					var sections []string
					for _, block := range thinkResp.Result.Value {
						label := block.Label
						if label == "" {
							label = labelFallback
						}
						body := block.Body
						if body == "" {
							body = accumulatedBody
						}
						if body != "" {
							sections = append(sections, fmt.Sprintf("  💭 <b>%s</b>\n\n<i>%s</i>", ui.EscapeHTML(label), ui.EscapeHTML(body)))
						} else {
							sections = append(sections, fmt.Sprintf("  💭 <b>%s</b>", ui.EscapeHTML(label)))
						}
					}

					combined := strings.Join(sections, "\n\n")
					maxThinkLen := TelegramMsgLimit - 100
					trimmed := combined
					if len(trimmed) > maxThinkLen {
						trimmed = trimmed[:maxThinkLen] + "..."
					}
					thinkMsg := fmt.Sprintf("<blockquote expandable>%s</blockquote>", trimmed)
					_ = b.sendMsg(ctx, channel, thinkMsg, nil, nil)
				}
			}

			mu.Lock()
			liveActivityUpdateVersion++
			vAct := liveActivityUpdateVersion
			thinkingActive = false
			completedBody := buildProgressBody()
			mu.Unlock()

			completedProgressMsg := fmt.Sprintf("<b>%s %s · %ds</b>\n\n%s",
				PhaseIcons["complete"], ui.EscapeHTML(modelLabel), elapsed, completedBody)
			_ = setProgressMessage(completedProgressMsg, vAct)

			mu.Lock()
			liveResponseUpdateVersion++
			mu.Unlock()

			footer := fmt.Sprintf("⏱️ %ds", elapsed)
			if strings.TrimSpace(finalOutputText) != "" {
				_ = sendChunkedResponse("", footer, finalOutputText, isAlreadyHTML)
			} else {
				mu.Lock()
				vResp := liveResponseUpdateVersion
				mu.Unlock()
				_ = upsertLiveResponse(PhaseIcons["complete"]+" Complete",
					"Failed to extract response. Use /screenshot to verify.", footer, &upsertOpts{
						expectedVersion: &vResp,
					})
			}

			if options != nil && options.ChatSessionService != nil && options.ChatSessionRepo != nil {
				sessionInfo := options.ChatSessionService.GetCurrentSessionInfo(cdpSvc)
				if sessionInfo.HasActiveChat && sessionInfo.Title != "" && sessionInfo.Title != "(Untitled)" {
					chKey := ChannelKey(channel)
					session, _ := options.ChatSessionRepo.FindByChannelId(chKey)
					var projectName string
					if session != nil && b.Pool != nil {
						projectName = b.Pool.ExtractProjectName(session.WorkspacePath)
					} else {
						projectName = cdpSvc.GetCurrentWorkspaceName()
					}
					if projectName != "" && b.Bridge != nil {
						var th *int
						if channel.ThreadID != 0 {
							t := channel.ThreadID
							th = &t
						}
						bridge.RegisterApprovalSessionChannel(b.Bridge, projectName, sessionInfo.Title, bridge.TelegramChannel{
							ChatID:   channel.ChatID,
							ThreadID: th,
						})
					}

					if session != nil && (session.DisplayName == nil || *session.DisplayName != sessionInfo.Title) {
						if options.TitleGenerator != nil && options.TopicManager != nil && channel.ThreadID != 0 {
							newName := options.TitleGenerator.SanitizeForChannelName(sessionInfo.Title)
							formattedName := fmt.Sprintf("%d-%s", session.SessionNumber, newName)
							options.TopicManager.SetChatID(channel.ChatID)
							options.TopicManager.RenameTopic(ctx, channel.ThreadID, formattedName)
						}
						_, _ = options.ChatSessionRepo.UpdateDisplayName(chKey, sessionInfo.Title)
					}
				}
			}

			sendGeneratedImages(finalOutputText)
		},
		OnTimeout: func(lastText string) {
			mu.Lock()
			isFinalized = true
			mu.Unlock()

			close(tickerDone)
			b.ConsumeUserStopRequest(ChannelKey(channel))

			elapsed := int(time.Since(startTime).Seconds())
			mu.Lock()
			timeoutText := lastText
			if strings.TrimSpace(timeoutText) == "" {
				timeoutText = lastProgressText
			}
			mu.Unlock()

			src := respMonitor.GetLastExtractionSource()
			timeoutIsHTML := src != nil && *src == monitor.SourceStructured
			var payload string
			if timeoutIsHTML {
				if strings.TrimSpace(timeoutText) != "" {
					payload = fmt.Sprintf("%s\n\n[Monitor Ended] Timeout after 30 minutes.", timeoutText)
				} else {
					payload = "Monitor ended after 30 minutes. No text was retrieved."
				}
			} else {
				sep := formatter.SplitOutputAndLogs(timeoutText)
				if strings.TrimSpace(sep.Output) != "" {
					payload = fmt.Sprintf("%s\n\n[Monitor Ended] Timeout after 30 minutes.", sep.Output)
				} else {
					payload = "Monitor ended after 30 minutes. No text was retrieved."
				}
			}

			mu.Lock()
			liveResponseUpdateVersion++
			mu.Unlock()
			_ = sendChunkedResponse(PhaseIcons["timeout"]+" Timeout", fmt.Sprintf("⏱️ %ds", elapsed), payload, timeoutIsHTML)

			mu.Lock()
			liveActivityUpdateVersion++
			vAct := liveActivityUpdateVersion
			thinkingActive = false
			timeoutBody := buildProgressBody()
			mu.Unlock()

			timeoutMsg := fmt.Sprintf("<b>%s %s · %ds</b>\n\n%s",
				PhaseIcons["timeout"], ui.EscapeHTML(modelLabel), elapsed, timeoutBody)
			_ = setProgressMessage(timeoutMsg, vAct)
		},
	})

	if err := respMonitor.Start(ctx); err != nil {
		mu.Lock()
		isFinalized = true
		mu.Unlock()
		return err
	}

	ticker := time.NewTicker(ElapsedTimerIntervalMs * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-tickerDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				done := isFinalized
				mu.Unlock()
				if done {
					return
				}
				triggerProgressRefresh()
			}
		}
	}()

	return nil
}

// StartPassiveMirrorForUserMessage sends PC user message notification and initiates passive response monitoring.
func (b *Bot) StartPassiveMirrorForUserMessage(
	ctx context.Context,
	projectName string,
	cdpSvc CdpServiceClient,
	info bridge.UserMessageInfo,
) {
	var currentChatTitle *string
	if b.ChatSessionService != nil && cdpSvc != nil {
		sessionInfo := b.ChatSessionService.GetCurrentSessionInfo(cdpSvc)
		if sessionInfo.Title != "" {
			t := sessionInfo.Title
			currentChatTitle = &t
		}
	}
	targetChannel := bridge.ResolveApprovalChannelForCurrentChat(b.Bridge, projectName, currentChatTitle)
	if targetChannel == nil {
		return
	}

	botChannel := TelegramChannel{ChatID: targetChannel.ChatID}
	if targetChannel.ThreadID != nil {
		botChannel.ThreadID = *targetChannel.ThreadID
	}

	text := fmt.Sprintf("🖥️ <b>Sent from PC:</b>\n\n%s", ui.EscapeHTML(info.Text))
	_ = b.ReplyHTML(ctx, botChannel, text, nil)

	var options *PromptSendOptions
	if b.ChatSessionService != nil || b.ChatSessionRepo != nil || b.TopicManager != nil || b.TitleGenerator != nil {
		options = &PromptSendOptions{
			ChatSessionService: b.ChatSessionService,
			ChatSessionRepo:    b.ChatSessionRepo,
			TopicManager:       b.TopicManager,
			TitleGenerator:     b.TitleGenerator,
		}
	}

	go func() {
		_ = b.SendPromptToAntigravity(ctx, botChannel, "", cdpSvc, nil, options)
	}()
}
