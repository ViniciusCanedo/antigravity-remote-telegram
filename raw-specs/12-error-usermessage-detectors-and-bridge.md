# 12 — Especificação Técnica: `ErrorPopupDetector`, `UserMessageDetector` e `CdpBridgeManager` (`internal/bridge`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação define de forma 100% autocontida e exaustiva o port para **Golang (1.23+)** do detector de popups de erro (`ErrorPopupDetector`), do detector de mensagens enviadas diretamente pelo usuário no PC (`UserMessageDetector`) e do gerenciador central de ponte e roteamento de canais/sessões (`CdpBridgeManager`).

### 1.1. Mapeamento de Arquivos Originais (TypeScript) para Destino (Go)

| Componente | Arquivo Original (TypeScript) | Linhas | Arquivo de Teste Original (Jest) | Linhas | Destino no Port Go | Arquivo de Teste Go |
| :--- | :--- | :---: | :--- | :---: | :--- | :--- |
| **Detector de Popup de Erro** | [`src/services/errorPopupDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/errorPopupDetector.ts#L1-L303) | 303 | [`tests/services/errorPopupDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/errorPopupDetector.test.ts#L1-L571) | 571 | `internal/bridge/error_detector.go` | `internal/bridge/error_detector_test.go` |
| **Detector de Mensagem do Usuário** | [`src/services/userMessageDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/userMessageDetector.ts#L1-L249) | 249 | [`tests/services/userMessageDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/userMessageDetector.test.ts#L1-L359) | 359 | `internal/bridge/user_message_detector.go` | `internal/bridge/user_message_detector_test.go` |
| **Gerenciador de Ponte CDP** | [`src/services/cdpBridgeManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpBridgeManager.ts#L1-L439) | 439 | [`tests/services/cdpBridgeManager.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpBridgeManager.test.ts#L1-L107) | 107 | `internal/bridge/manager.go` | `internal/bridge/manager_test.go` |
| **Documentação de Arquitetura** | [`docs/05-detectores-e-servicos-background.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md#L15-L86) | 151 | — | — | Pacote `bridge` (`internal/bridge`) | — |

### 1.2. Responsabilidades no Sistema

1. **`ErrorPopupDetector` ([`src/services/errorPopupDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/errorPopupDetector.ts#L119-L303))**:
   - Realiza polling periódico no DOM do Antigravity (padrão `3000ms`) buscando modais/diálogos (`[role="dialog"], [role="alertdialog"], .modal, .dialog`) ou overlays posicionados (`fixed`/`absolute` com `zIndex > 10` e ao menos um `<button>`) cujo texto contenha padrões de falha do agente (`"agent terminated"`, `"terminated due to error"`, `"unexpected error"`, `"something went wrong"`, `"an error occurred"`).
   - Extrai `title` (com fallback `"Error"`), `body` (via `TreeWalker` excluindo o título e o texto dos botões, truncado em `1000` caracteres) e a lista de `buttons`.
   - Evita notificações duplicadas combinando `lastDetectedKey = "${info.title}::${info.body.slice(0, 100)}"` com uma janela de cooldown de `10000ms` (`ErrorPopupCooldown = 10 * time.Second`).
   - Expõe `ClickDismissButton(ctx)` (`"Dismiss"`), `ClickCopyDebugInfoButton(ctx)` (`"Copy debug info"`), `ClickRetryButton(ctx)` (`"Retry"`) e `ReadClipboard(ctx)` (avaliando `READ_CLIPBOARD_SCRIPT` com `awaitPromise: true`).

2. **`UserMessageDetector` ([`src/services/userMessageDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/userMessageDetector.ts#L94-L249))**:
   - Realiza polling periódico no DOM do Antigravity (padrão `2000ms`) para detectar a última bolha de mensagem do usuário enviada diretamente pela interface desktop do PC.
   - Calcula um hash curto de 16 caracteres hexadecimais (`SHA-256` sobre o texto normalizado `.trim().replace(/\s+/g, ' ').slice(0, 200)`).
   - No primeiro ciclo após `Start()` (`isPriming = true`), apenas registra a mensagem existente no DOM em `lastDetectedHash` e `seenHashes` **sem** invocar `OnUserMessage`.
   - Mantém dois mecanismos de proteção contra eco e re-emissão:
     - `echoHashes`: hashes de mensagens enviadas pelo próprio bot do Telegram via `AddEchoHash(text)`, com expiração automática (TTL) de `60000ms` (60s), preservados mesmo entre ciclos de `Stop()`/`Start()`.
     - `seenHashes`: conjunto FIFO com capacidade máxima de `50` entradas (`MaxSeenHashes = 50`) para evitar re-detecção caso o DOM reverta temporariamente para uma mensagem antiga.

3. **`CdpBridgeManager` ([`src/services/cdpBridgeManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpBridgeManager.ts#L1-L439))**:
   - Gerencia o estado global `CdpBridge` que conecta o `CdpConnectionPool`, `QuotaService`, `AutoAcceptService`, tabelas de roteamento de canais do Telegram (`approvalChannelByWorkspace` e `approvalChannelBySession`) e o cliente da API do Telegram (`botApi`).
   - Resolve dinamicamente para qual tópico/chat do Telegram cada notificação de background deve ser enviada lendo o título da conversa ativa no painel lateral do Antigravity via `GetCurrentChatTitle(ctx, cdp)`.
   - Constrói e faz o parse de `callback_data` dos botões inline de Aprovação, Planejamento e Popup de Erro.
   - Instancia, registra no pool e gerencia o ciclo de vida e as mensagens do Telegram dos 4 detectores (`EnsureApprovalDetector`, `EnsurePlanningDetector`, `EnsureErrorPopupDetector`, `EnsureUserMessageDetector`), incluindo a remoção automática do teclado inline (`EditMessageReplyMarkup` com `reply_markup: nil`) no callback `OnResolved`.

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

```go
package bridge

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// ============================================================================
// 2.1. ErrorPopupDetector (internal/bridge/error_detector.go)
// Origem: src/services/errorPopupDetector.ts
// ============================================================================

// ErrorPopupInfo representa as informações extraídas de um popup de erro na UI do Antigravity.
// Origem: interface ErrorPopupInfo (src/services/errorPopupDetector.ts#L6-L13)
type ErrorPopupInfo struct {
	// Title é o título do popup de erro (ex: "Agent terminated due to error" ou fallback "Error").
	Title string `json:"title"`
	// Body é o texto descritivo do corpo do popup (truncado em até 1000 caracteres pelo script JS).
	Body string `json:"body"`
	// Buttons contém os rótulos visíveis dos botões encontrados no popup (ex: ["Dismiss", "Copy debug info", "Retry"]).
	Buttons []string `json:"buttons"`
}

// ErrorPopupDetectorOptions configura uma instância de ErrorPopupDetector.
// Origem: interface ErrorPopupDetectorOptions (src/services/errorPopupDetector.ts#L15-L24)
type ErrorPopupDetectorOptions struct {
	// CdpService é a instância de conexão CDP com o workspace.
	CdpService CdpEvaluator
	// PollInterval é o intervalo entre ciclos de polling (padrão: 3000ms se <= 0).
	PollInterval time.Duration
	// OnErrorPopup é o callback invocado quando um popup de erro é detectado.
	OnErrorPopup func(info ErrorPopupInfo)
	// OnResolved é o callback opcional invocado quando o popup desaparece do DOM.
	OnResolved func()
	// Logger opcional (usa slog.Default() se nil).
	Logger *slog.Logger
	// NowFunc opcional para injeção de relógio determinístico em testes de cooldown (usa time.Now se nil).
	NowFunc func() time.Time
}

// ErrorPopupDetector detecta diálogos de erro na UI do Antigravity via polling.
// Origem: class ErrorPopupDetector (src/services/errorPopupDetector.ts#L119-L303)
type ErrorPopupDetector struct {
	cdpService   CdpEvaluator
	pollInterval time.Duration
	onErrorPopup func(info ErrorPopupInfo)
	onResolved   func()
	logger       *slog.Logger
	nowFunc      func() time.Time

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedKey  *string
	lastDetectedInfo *ErrorPopupInfo
	lastNotifiedAt   time.Time
}

// Assinaturas públicas e privadas de ErrorPopupDetector:
func NewErrorPopupDetector(options ErrorPopupDetectorOptions) *ErrorPopupDetector
func (d *ErrorPopupDetector) Start()
func (d *ErrorPopupDetector) Stop() error
func (d *ErrorPopupDetector) IsActive() bool
func (d *ErrorPopupDetector) GetLastDetectedInfo() *ErrorPopupInfo
func (d *ErrorPopupDetector) ClickDismissButton(ctx context.Context) (bool, error)
func (d *ErrorPopupDetector) ClickCopyDebugInfoButton(ctx context.Context) (bool, error)
func (d *ErrorPopupDetector) ClickRetryButton(ctx context.Context) (bool, error)
func (d *ErrorPopupDetector) ReadClipboard(ctx context.Context) (*string, error)
func (d *ErrorPopupDetector) PollOnce(ctx context.Context)
func (d *ErrorPopupDetector) clickButton(ctx context.Context, buttonText string) bool
func (d *ErrorPopupDetector) runEvaluateScript(ctx context.Context, expression string, awaitPromise bool) (json.RawMessage, error)

// ============================================================================
// 2.2. UserMessageDetector (internal/bridge/user_message_detector.go)
// Origem: src/services/userMessageDetector.ts
// ============================================================================

// UserMessageInfo representa uma mensagem do usuário detectada diretamente no DOM do Antigravity.
// Origem: interface UserMessageInfo (src/services/userMessageDetector.ts#L6-L9)
type UserMessageInfo struct {
	// Text é o conteúdo textual da mensagem.
	Text string `json:"text"`
}

// UserMessageDetectorOptions configura uma instância de UserMessageDetector.
// Origem: interface UserMessageDetectorOptions (src/services/userMessageDetector.ts#L11-L18)
type UserMessageDetectorOptions struct {
	// CdpService é a instância de conexão CDP com o workspace.
	CdpService CdpEvaluator
	// PollInterval é o intervalo entre ciclos de polling (padrão: 2000ms se <= 0).
	PollInterval time.Duration
	// OnUserMessage é o callback invocado quando uma nova mensagem do usuário (não-eco) é detectada.
	OnUserMessage func(info UserMessageInfo)
	// EchoTTL opcional para testes (padrão: 60s se <= 0).
	EchoTTL time.Duration
	// Logger opcional (usa slog.Default() se nil).
	Logger *slog.Logger
}

// UserMessageDetector detecta mensagens postadas diretamente na UI do Antigravity (ex: via PC).
// Origem: class UserMessageDetector (src/services/userMessageDetector.ts#L94-L249)
type UserMessageDetector struct {
	cdpService    CdpEvaluator
	pollInterval  time.Duration
	onUserMessage func(info UserMessageInfo)
	echoTTL       time.Duration
	logger        *slog.Logger

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedHash *string
	echoHashes       map[string]int // contagem de referências por hash para suportar múltiplos timers de 60s
	seenHashesOrder  []string       // fila FIFO mantendo ordem de inserção até MaxSeenHashes (50)
	seenHashesSet    map[string]struct{}
	isPriming        bool
}

// Assinaturas públicas e privadas de UserMessageDetector:
func NormalizeForHash(text string) string
func ComputeEchoHash(text string) string
func NewUserMessageDetector(options UserMessageDetectorOptions) *UserMessageDetector
func (d *UserMessageDetector) AddEchoHash(text string)
func (d *UserMessageDetector) Start()
func (d *UserMessageDetector) Stop()
func (d *UserMessageDetector) IsActive() bool
func (d *UserMessageDetector) PollOnce(ctx context.Context)
func (d *UserMessageDetector) addToSeenHashesLocked(hash string)

// ============================================================================
// 2.3. CdpBridgeManager (internal/bridge/manager.go)
// Origem: src/services/cdpBridgeManager.ts
// ============================================================================

// TelegramChannel representa um destino de chat no Telegram: chat_id + message_thread_id opcional.
// Origem: interface TelegramChannel (src/services/cdpBridgeManager.ts#L17-L20)
type TelegramChannel struct {
	ChatID   int64
	ThreadID *int
}

// ExecutionContextInfo representa um contexto de execução JS retornado por cdp.getContexts().
type ExecutionContextInfo struct {
	ID int64 `json:"id"`
}

// CdpServiceWithContexts estende CdpEvaluator com a listagem de todos os contextos JS ativos.
type CdpServiceWithContexts interface {
	CdpEvaluator
	GetContexts() []ExecutionContextInfo
}

// BridgeConnectionPool define as operações do CdpConnectionPool consumidas pelo CdpBridgeManager.
type BridgeConnectionPool interface {
	GetConnected(workspaceName string) CdpServiceWithContexts
	GetActiveWorkspaceNames() []string
	GetApprovalDetector(projectName string) *ApprovalDetector
	RegisterApprovalDetector(projectName string, detector *ApprovalDetector)
	GetPlanningDetector(projectName string) *PlanningDetector
	RegisterPlanningDetector(projectName string, detector *PlanningDetector)
	GetErrorPopupDetector(projectName string) *ErrorPopupDetector
	RegisterErrorPopupDetector(projectName string, detector *ErrorPopupDetector)
	GetUserMessageDetector(projectName string) *UserMessageDetector
	RegisterUserMessageDetector(projectName string, detector *UserMessageDetector)
}

// InlineButton representa um botão de teclado inline do Telegram.
type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// InlineKeyboardMarkup representa uma matriz de linhas de botões inline do Telegram.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineButton `json:"inline_keyboard"`
}

// TelegramBotMessenger abstrai o envio e edição de mensagens no Telegram (equivalente a Api do grammy).
type TelegramBotMessenger interface {
	// SendMessage envia uma mensagem com parse_mode="HTML", message_thread_id opcional e reply_markup opcional,
	// retornando o message_id da mensagem enviada.
	SendMessage(ctx context.Context, chatID int64, threadID *int, htmlText string, keyboard *InlineKeyboardMarkup) (int, error)
	// ClearMessageReplyMarkup remove os botões inline de uma mensagem existente (editMessageReplyMarkup com reply_markup: undefined).
	ClearMessageReplyMarkup(ctx context.Context, chatID int64, messageID int) error
}

// AutoAcceptChecker abstrai o AutoAcceptService (src/services/autoAcceptService.ts).
type AutoAcceptChecker interface {
	IsEnabled() bool
	Handle(mode string) (AutoAcceptResult, error)
}

// AutoAcceptResult espelha o retorno de AutoAcceptService.handle().
type AutoAcceptResult struct {
	Success bool
	Enabled bool
	Changed bool
	Message string
}

// QuotaFetcher abstrai o QuotaService (src/services/quotaService.ts).
type QuotaFetcher interface {
	FetchQuota(ctx context.Context) (any, error)
}

// PlanNotificationBuilder constrói o texto HTML e o teclado inline de notificação de plano
// (correspondente a buildPlanNotificationUI em src/ui/planUi.ts).
type PlanNotificationBuilder func(info PlanningInfo, projectName string, targetChannelStr string) (text string, keyboard *InlineKeyboardMarkup)

// CdpBridge mantém o estado central compartilhado de conexões CDP, detectores e roteamento de canais.
// Origem: interface CdpBridge (src/services/cdpBridgeManager.ts#L22-L32)
type CdpBridge struct {
	mu                         sync.RWMutex
	Pool                       BridgeConnectionPool
	Quota                      QuotaFetcher
	AutoAccept                 AutoAcceptChecker
	LastActiveWorkspace        *string
	LastActiveChannel          *TelegramChannel
	ApprovalChannelByWorkspace map[string]TelegramChannel
	ApprovalChannelBySession   map[string]TelegramChannel
	BotApi                     TelegramBotMessenger
	BotToken                   string
	PlanUIBuilder              PlanNotificationBuilder
	TranslateFunc              func(key string) string // equivalente a t() de src/utils/i18n.ts
	Logger                     *slog.Logger
}

// ParsedApprovalCustomID representa o resultado de ParseApprovalCustomID.
type ParsedApprovalCustomID struct {
	Action      string  // "approve" | "always_allow" | "deny"
	ProjectName *string // nil quando ausente
	ChannelID   *string // nil quando ausente
}

// ParsedPlanningCustomID representa o resultado de ParsePlanningCustomID.
type ParsedPlanningCustomID struct {
	Action      string  // "open" | "proceed"
	ProjectName *string // nil quando ausente
	ChannelID   *string // nil quando ausente
}

// ParsedErrorPopupCustomID representa o resultado de ParseErrorPopupCustomID.
type ParsedErrorPopupCustomID struct {
	Action      string  // "dismiss" | "copy_debug" | "retry"
	ProjectName *string // nil quando ausente
	ChannelID   *string // nil quando ausente
}

// Assinaturas públicas de internal/bridge/manager.go:
func InitCdpBridge(autoApproveDefault bool, pool BridgeConnectionPool, quota QuotaFetcher, autoAccept AutoAcceptChecker) *CdpBridge
func GetCurrentCdp(bridge *CdpBridge) CdpServiceWithContexts
func GetCurrentChatTitle(ctx context.Context, cdp CdpServiceWithContexts) *string
func RegisterApprovalWorkspaceChannel(bridge *CdpBridge, projectName string, channel TelegramChannel)
func RegisterApprovalSessionChannel(bridge *CdpBridge, projectName string, sessionTitle string, channel TelegramChannel)
func ResolveApprovalChannelForCurrentChat(bridge *CdpBridge, projectName string, currentChatTitle *string) *TelegramChannel
func BuildApprovalCustomID(action string, projectName string, channelID ...string) string
func ParseApprovalCustomID(customID string) *ParsedApprovalCustomID
func BuildPlanningCustomID(action string, projectName string, channelID ...string) string
func ParsePlanningCustomID(customID string) *ParsedPlanningCustomID
func BuildErrorPopupCustomID(action string, projectName string, channelID ...string) string
func ParseErrorPopupCustomID(customID string) *ParsedErrorPopupCustomID
func EnsureApprovalDetector(bridge *CdpBridge, cdp CdpServiceWithContexts, projectName string)
func EnsurePlanningDetector(bridge *CdpBridge, cdp CdpServiceWithContexts, projectName string)
func EnsureErrorPopupDetector(bridge *CdpBridge, cdp CdpServiceWithContexts, projectName string)
func EnsureUserMessageDetector(bridge *CdpBridge, cdp CdpServiceWithContexts, projectName string, onUserMessage func(info UserMessageInfo))
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

### 3.1. Constantes de Tempo, Limites e Prefixos de `callback_data`

```go
const (
	// ErrorPopupDetector constants (src/services/errorPopupDetector.ts)
	DefaultErrorPopupPollInterval = 3000 * time.Millisecond
	ErrorPopupCooldown            = 10000 * time.Millisecond
	ErrorPopupBodyKeyMaxLen       = 100
	ErrorPopupTelegramBodyMaxLen  = 3800

	// Button texts clicados por ErrorPopupDetector (src/services/errorPopupDetector.ts#L176-L194)
	DismissButtonText       = "Dismiss"
	CopyDebugInfoButtonText = "Copy debug info"
	RetryButtonText         = "Retry"

	// UserMessageDetector constants (src/services/userMessageDetector.ts)
	DefaultUserMessagePollInterval = 2000 * time.Millisecond
	MaxSeenHashes                  = 50
	DefaultEchoHashTTL             = 60000 * time.Millisecond
	NormalizeHashMaxLen            = 200
	EchoHashHexLen                 = 16

	// CdpBridgeManager callback_data prefixes (src/services/cdpBridgeManager.ts#L34-L41)
	ApproveActionPrefix             = "approve_action"
	AlwaysAllowActionPrefix         = "always_allow_action"
	DenyActionPrefix                = "deny_action"
	PlanningOpenActionPrefix        = "planning_open_action"
	PlanningProceedActionPrefix     = "planning_proceed_action"
	ErrorPopupDismissActionPrefix   = "error_popup_dismiss_action"
	ErrorPopupCopyDebugActionPrefix = "error_popup_copy_debug_action"
	ErrorPopupRetryActionPrefix     = "error_popup_retry_action"

	// CdpConnectionPool defaults em initCdpBridge (src/services/cdpBridgeManager.ts#L189-L193)
	DefaultPoolCdpCallTimeout       = 15000 * time.Millisecond
	DefaultPoolMaxReconnectAttempts = 3
	DefaultPoolReconnectDelay       = 3000 * time.Millisecond
)
```

### 3.2. `DETECT_ERROR_POPUP_SCRIPT` ([`src/services/errorPopupDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/errorPopupDetector.ts#L32-L94))

```go
const DetectErrorPopupScript = `(() => {
    const ERROR_PATTERNS = [
        'agent terminated',
        'terminated due to error',
        'unexpected error',
        'something went wrong',
        'an error occurred',
    ];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();

    // Try dialog/modal first
    const dialogs = Array.from(document.querySelectorAll(
        '[role="dialog"], [role="alertdialog"], .modal, .dialog'
    )).filter(el => el.offsetParent !== null || el.getAttribute('aria-modal') === 'true');

    // Fallback: look for fixed/absolute positioned overlays
    if (dialogs.length === 0) {
        const overlays = Array.from(document.querySelectorAll('div[class*="fixed"], div[class*="absolute"]'))
            .filter(el => {
                const style = window.getComputedStyle(el);
                return (style.position === 'fixed' || style.position === 'absolute')
                    && style.zIndex && parseInt(style.zIndex, 10) > 10
                    && el.querySelector('button');
            });
        dialogs.push(...overlays);
    }

    for (const dialog of dialogs) {
        const fullText = normalize(dialog.textContent || '');
        const isError = ERROR_PATTERNS.some(p => fullText.includes(p));
        if (!isError) continue;

        // Extract title from heading elements or first prominent text
        const headingEl = dialog.querySelector('h1, h2, h3, h4, [class*="title"], [class*="heading"]');
        const title = headingEl ? (headingEl.textContent || '').trim() : '';

        // Extract body text (excluding button text and title)
        const allButtons = Array.from(dialog.querySelectorAll('button'))
            .filter(btn => btn.offsetParent !== null);
        const buttonTexts = new Set(allButtons.map(btn => (btn.textContent || '').trim()));

        const bodyParts = [];
        const walker = document.createTreeWalker(dialog, NodeFilter.SHOW_TEXT);
        let node;
        while ((node = walker.nextNode())) {
            const text = (node.textContent || '').trim();
            if (!text) continue;
            if (buttonTexts.has(text)) continue;
            if (text === title) continue;
            bodyParts.push(text);
        }
        const body = bodyParts.join(' ').slice(0, 1000);

        const buttons = allButtons.map(btn => (btn.textContent || '').trim()).filter(t => t.length > 0);

        if (buttons.length === 0) continue;

        return { title: title || 'Error', body, buttons };
    }

    return null;
})()`
```

### 3.3. `READ_CLIPBOARD_SCRIPT` ([`src/services/errorPopupDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/errorPopupDetector.ts#L100-L107))

```go
const ReadClipboardScript = `(async () => {
    try {
        const text = await navigator.clipboard.readText();
        return text || null;
    } catch (e) {
        return null;
    }
})()`
```

### 3.4. `DETECT_USER_MESSAGE_SCRIPT` ([`src/services/userMessageDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/userMessageDetector.ts#L37-L73))

```go
const DetectUserMessageScript = `(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    const scope = panel || document;

    // Strategy A (primary): Query .whitespace-pre-wrap elements directly inside
    // user bubble containers. This avoids the parent-container problem where
    // querySelectorAll matches a wrapper that contains multiple bubbles.
    const textEls = scope.querySelectorAll(
        '[class*="bg-gray-500/15"][class*="select-text"] .whitespace-pre-wrap'
    );

    if (textEls.length > 0) {
        const lastTextEl = textEls[textEls.length - 1];
        const text = (lastTextEl.textContent || '').trim();
        if (text.length > 0) return { text };
    }

    // Strategy B (fallback): Find individual bubble containers, filtering out
    // any element that itself contains nested bubble elements (i.e., a parent wrapper).
    const userBubbles = Array.from(scope.querySelectorAll(
        '[class*="bg-gray-500/15"][class*="rounded-lg"][class*="select-text"]'
    )).filter(el => !el.querySelector('[class*="bg-gray-500/15"][class*="select-text"]'));

    if (userBubbles.length === 0) return null;

    const lastBubble = userBubbles[userBubbles.length - 1];
    const textEl = lastBubble.querySelector('.whitespace-pre-wrap')
        || lastBubble.querySelector('[style*="word-break"]');

    const text = textEl
        ? (textEl.textContent || '').trim()
        : (lastBubble.textContent || '').trim();

    if (!text || text.length < 1) return null;

    return { text };
})()`
```

### 3.5. `GET_CURRENT_CHAT_TITLE_SCRIPT` ([`src/services/cdpBridgeManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpBridgeManager.ts#L51-L60))

```go
const GetCurrentChatTitleScript = `(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    if (!panel) return '';
    const header = panel.querySelector('div[class*="border-b"]');
    if (!header) return '';
    const titleEl = header.querySelector('div[class*="text-ellipsis"]');
    const title = titleEl ? (titleEl.textContent || '').trim() : '';
    if (!title || title === 'Agent') return '';
    return title;
})()`
```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo de `ErrorPopupDetector` (`internal/bridge/error_detector.go`)

1. **Inicialização (`NewErrorPopupDetector`)**:
   - Define `pollInterval = 3000 * time.Millisecond` caso `options.PollInterval <= 0`.
   - Define `nowFunc = time.Now` caso `options.NowFunc == nil`.
2. **Ciclo de Polling (`PollOnce(ctx)`)**:
   - Avalia `DetectErrorPopupScript` com `returnByValue: true`, `awaitPromise: false` e `contextId` opcional (omitido quando `GetPrimaryContextId() == nil`).
   - Em caso de erro CDP: se a mensagem contiver `"WebSocket is not connected"` ou `"WebSocket disconnected"`, retorna silenciosamente sem logar; caso contrário loga `"[ErrorPopupDetector] Error during polling"` e retorna.
   - Se `info != nil`:
     - Extrai `bodyPrefix`: os primeiros `100` caracteres (runes/UTF-16 units) de `info.Body`.
     - Monta `key := fmt.Sprintf("%s::%s", info.Title, bodyPrefix)`.
     - Verifica `withinCooldown := !d.lastNotifiedAt.IsZero() && now.Sub(d.lastNotifiedAt) < 10*time.Second`.
     - Se `(d.lastDetectedKey == nil || *d.lastDetectedKey != key) && !withinCooldown`:
       - Atualiza `d.lastDetectedKey = &key`, `d.lastDetectedInfo = infoCopy`, `d.lastNotifiedAt = now`.
       - Invoca `d.onErrorPopup(*info)`.
     - Senão, se `d.lastDetectedKey != nil && *d.lastDetectedKey == key`:
       - Atualiza silenciosamente `d.lastDetectedInfo = infoCopy`.
   - Se `info == nil`:
     - `wasDetected := d.lastDetectedKey != nil`
     - Reseta `d.lastDetectedKey = nil` e `d.lastDetectedInfo = nil` (mantendo `d.lastNotifiedAt` intacto para preservar o cooldown de 10s).
     - Se `wasDetected && d.onResolved != nil`, invoca `d.onResolved()`.
3. **Ações de Clique e Clipboard**:
   - `ClickDismissButton(ctx)` $\rightarrow$ chama `d.clickButton(ctx, "Dismiss")` usando `BuildClickScript("Dismiss")`.
   - `ClickCopyDebugInfoButton(ctx)` $\rightarrow$ chama `d.clickButton(ctx, "Copy debug info")` usando `BuildClickScript("Copy debug info")`.
   - `ClickRetryButton(ctx)` $\rightarrow$ chama `d.clickButton(ctx, "Retry")` usando `BuildClickScript("Retry")`.
   - `ReadClipboard(ctx)` $\rightarrow$ chama `d.runEvaluateScript(ctx, ReadClipboardScript, true)` (com `awaitPromise: true`). Se o retorno for uma string não-nula, retorna `(&str, nil)`; se for `null` ou ocorrer erro CDP (que é logado em `"[ErrorPopupDetector] Error reading clipboard"`), retorna `(nil, nil)`.

---

### 4.2. Algoritmo de `UserMessageDetector` (`internal/bridge/user_message_detector.go`)

1. **Normalização e Hash de Eco (`NormalizeForHash` e `ComputeEchoHash`)**:
   - `NormalizeForHash(text)`:
     - Aplica `strings.TrimSpace(text)`.
     - Colapsa qualquer sequência de caracteres de espaço em branco (`\s+`) em um único espaço `' '`.
     - Corta nos primeiros `200` caracteres (runes/UTF-16 slice `slice(0, 200)`).
   - `ComputeEchoHash(text)`:
     - Calcula `sha256.Sum256([]byte(NormalizeForHash(text)))`.
     - Codifica em hexadecimal lowercase e retorna os primeiros `16` caracteres (`hexStr[:16]`).
2. **Registro de Eco (`AddEchoHash(text)`)**:
   - Calcula `hash := ComputeEchoHash(text)`.
   - Adquire `d.mu.Lock()`, incrementa `d.echoHashes[hash]++`, libera o lock.
   - Agenda `time.AfterFunc(d.echoTTL, func() { ... })` (onde `d.echoTTL` é `60 * time.Second` por padrão) que decrementa `d.echoHashes[hash]` e remove a chave quando chega a `0`.
3. **Início (`Start()`)**:
   - Se `d.isRunning` for `true`, retorna imediatamente (idempotente).
   - Define `d.isRunning = true`, `d.lastDetectedHash = nil`, limpa `d.seenHashesOrder` e `d.seenHashesSet`, e define `d.isPriming = true`.
   - **Regra Crítica**: `d.echoHashes` **NÃO** é limpo em `Start()` ([`src/services/userMessageDetector.ts#L137-L138`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/userMessageDetector.ts#L137-L138)).
4. **Fila FIFO de Hashes Vistos (`addToSeenHashesLocked(hash)`)**:
   - Se `_, exists := d.seenHashesSet[hash]; exists`, não duplica.
   - Se `len(d.seenHashesOrder) >= MaxSeenHashes` (`50`):
     - Remove o elemento mais antigo `oldest := d.seenHashesOrder[0]`.
     - `d.seenHashesOrder = d.seenHashesOrder[1:]`
     - `delete(d.seenHashesSet, oldest)`
   - Adiciona `hash` ao final de `d.seenHashesOrder` e em `d.seenHashesSet[hash] = struct{}{}`.
5. **Ciclo de Polling (`PollOnce(ctx)`)**:
   - Avalia `DetectUserMessageScript` via `Runtime.evaluate` (`returnByValue: true`, `awaitPromise: false`).
   - Em erro CDP: ignora silenciosamente se contiver `"WebSocket is not connected"` ou `"WebSocket disconnected"`; caso contrário loga `"[UserMessageDetector] Error during polling"` e retorna.
   - **Verificação de Priming com DOM vazio**:
     - Se `d.isPriming && (info == nil || info.Text == "")`:
       - Define `d.isPriming = false`, loga debug `"[UserMessageDetector] Primed with empty DOM"` e retorna.
   - **Quando `info != nil && info.Text != ""`**:
     - Calcula `hash := ComputeEchoHash(info.Text)`.
     - **Passo 1 (Priming com mensagem existente)**:
       - Se `d.isPriming`:
         - Define `d.isPriming = false`, `d.lastDetectedHash = &hash`, chama `d.addToSeenHashesLocked(hash)` e retorna **sem** chamar `onUserMessage`.
     - **Passo 2 (Duplicata consecutiva)**:
       - Se `d.lastDetectedHash != nil && *d.lastDetectedHash == hash`, retorna imediatamente.
     - **Passo 3 (Já visto anteriormente em `seenHashes`)**:
       - Se `_, seen := d.seenHashesSet[hash]; seen`:
         - Atualiza `d.lastDetectedHash = &hash` e retorna sem chamar `onUserMessage`.
     - **Passo 4 (Mensagem de Eco enviada pelo próprio Bot)**:
       - Se `d.echoHashes[hash] > 0`:
         - Atualiza `d.lastDetectedHash = &hash`, chama `d.addToSeenHashesLocked(hash)` e retorna sem chamar `onUserMessage`.
     - **Passo 5 (Nova Mensagem Legítima do PC)**:
       - Atualiza `d.lastDetectedHash = &hash`, chama `d.addToSeenHashesLocked(hash)`, libera o lock e invoca `d.onUserMessage(*info)`.

---

### 4.3. Algoritmo de `CdpBridgeManager` (`internal/bridge/manager.go`)

#### A. Roteamento de Canais por Sessão e Workspace
1. **`normalizeSessionTitle(title string) string`**:
   - Retorna `strings.ToLower(strings.TrimSpace(title))`.
2. **`buildSessionRouteKey(projectName, sessionTitle string) string`**:
   - Retorna `fmt.Sprintf("%s::%s", projectName, normalizeSessionTitle(sessionTitle))`.
3. **`RegisterApprovalWorkspaceChannel(bridge, projectName, channel)`**:
   - Armazena `bridge.ApprovalChannelByWorkspace[projectName] = channel`.
4. **`RegisterApprovalSessionChannel(bridge, projectName, sessionTitle, channel)`**:
   - Se `strings.TrimSpace(sessionTitle) == ""`, retorna sem fazer nada.
   - Armazena `bridge.ApprovalChannelBySession[buildSessionRouteKey(projectName, sessionTitle)] = channel`.
   - Também atualiza `bridge.ApprovalChannelByWorkspace[projectName] = channel`.
5. **`ResolveApprovalChannelForCurrentChat(bridge, projectName, currentChatTitle)`**:
   - Se `currentChatTitle != nil && strings.TrimSpace(*currentChatTitle) != ""`:
     - Procura `key := buildSessionRouteKey(projectName, *currentChatTitle)` em `bridge.ApprovalChannelBySession`.
     - Se encontrar, retorna ponteiro para esse `TelegramChannel`.
   - Caso contrário (ou se não encontrado na sessão), procura `projectName` em `bridge.ApprovalChannelByWorkspace`. Se encontrar, retorna ponteiro para ele; senão retorna `nil`.
6. **`GetCurrentChatTitle(ctx, cdp)`**:
   - Itera sobre `contexts := cdp.GetContexts()`.
   - Para cada `ctxInfo`, chama `cdp.Call(ctx, "Runtime.evaluate", map[string]any{"expression": GetCurrentChatTitleScript, "returnByValue": true, "contextId": ctxInfo.ID})`.
   - Se o valor retornado for uma `string` com `strings.TrimSpace(val) != ""`, retorna ponteiro para `strings.TrimSpace(val)`.
   - Se nenhum contexto retornar título válido, retorna `nil`.
7. **`GetCurrentCdp(bridge)`**:
   - Se `bridge.LastActiveWorkspace != nil && *bridge.LastActiveWorkspace != ""`:
     - Consulta `cdp := bridge.Pool.GetConnected(*bridge.LastActiveWorkspace)`. Se `cdp != nil`, retorna `cdp`.
   - Fallback: consulta `activeNames := bridge.Pool.GetActiveWorkspaceNames()`. Se `len(activeNames) > 0`, retorna `bridge.Pool.GetConnected(activeNames[0])`.
   - Caso contrário, loga warning `"[CdpBridge] No connected workspaces found."` e retorna `nil`.

#### B. Construção e Parsing de `callback_data` (`CustomId`)
1. **`BuildApprovalCustomID(action, projectName, channelID...)`**:
   - Mapeia `action`:
     - `"approve"` $\rightarrow$ `"approve_action"`
     - `"always_allow"` $\rightarrow$ `"always_allow_action"`
     - `"deny"` $\rightarrow$ `"deny_action"`
   - Se `channelID` for informado e `strings.TrimSpace(channelID[0]) != ""`, retorna `fmt.Sprintf("%s:%s:%s", prefix, projectName, channelID[0])`.
   - Caso contrário, retorna `fmt.Sprintf("%s:%s", prefix, projectName)`.
2. **`ParseApprovalCustomID(customID)`**:
   - Verifica na ordem: `("approve", "approve_action")`, `("always_allow", "always_allow_action")`, `("deny", "deny_action")`.
   - Se `customID == prefix`, retorna `&ParsedApprovalCustomID{Action: action, ProjectName: nil, ChannelID: nil}`.
   - Se `strings.HasPrefix(customID, prefix+":")`:
     - Extrai `rest := customID[len(prefix)+1:]` e divide com `strings.Split(rest, ":")`.
     - `projectName` é `parts[0]` (ou `nil` se `""`).
     - `channelID` é `parts[1]` se `len(parts) > 1 && parts[1] != ""` (ou `nil`).
   - Se não corresponder a nenhum prefixo, retorna `nil`.
3. **`BuildPlanningCustomID` / `ParsePlanningCustomID`**:
   - Mesma lógica exata usando `("open", "planning_open_action")` e `("proceed", "planning_proceed_action")`.
4. **`BuildErrorPopupCustomID` / `ParseErrorPopupCustomID`**:
   - Mesma lógica exata usando `("dismiss", "error_popup_dismiss_action")`, `("copy_debug", "error_popup_copy_debug_action")` e `("retry", "error_popup_retry_action")`.

#### C. Orquestração dos 4 Detectores (`Ensure*Detector`)
1. **`EnsureApprovalDetector(bridge, cdp, projectName)`**:
   - Se `existing := bridge.Pool.GetApprovalDetector(projectName); existing != nil && existing.IsActive()`, retorna imediatamente.
   - Mantém estado local closure (`lastMessageID *int`, `lastMessageChatID *int64` protegidos por mutex).
   - Cria `ApprovalDetector` com `PollInterval: 2000 * time.Millisecond`:
     - **`OnResolved`**: se `lastMessageID != nil && lastMessageChatID != nil && bridge.BotApi != nil`, limpa o estado local e chama `bridge.BotApi.ClearMessageReplyMarkup(ctx, *chatID, *msgID)`.
     - **`OnApprovalRequired(info)`**:
       1. Obtém `currentChatTitle := GetCurrentChatTitle(ctx, cdp)` e `targetChannel := ResolveApprovalChannelForCurrentChat(bridge, projectName, currentChatTitle)`.
       2. Se `targetChannel == nil || bridge.BotApi == nil`, loga warning e retorna.
       3. Calcula `targetChannelStr`: se `targetChannel.ThreadID != nil`, usa ` strconv.Itoa(*targetChannel.ThreadID)`; senão usa `strconv.FormatInt(targetChannel.ChatID, 10)`.
       4. **Fluxo Auto-Approve**: se `bridge.AutoAccept != nil && bridge.AutoAccept.IsEnabled()`:
          - Tenta `accepted, _ := detector.AlwaysAllowButton(ctx)`; se `!accepted`, tenta `accepted, _ = detector.ApproveButton(ctx)`.
          - Se `accepted == true`, envia HTML:
            `✅ <b>Auto-approved</b>\nAn action was automatically approved.\n<b>Workspace:</b> ${escapeHtml(projectName)}`
            e **retorna** (não envia teclado manual).
          - Se `accepted == false`, envia HTML:
            `⚠️ <b>Auto-approve failed</b>\nManual approval required.\n<b>Workspace:</b> ${escapeHtml(projectName)}`
            e **prossegue** para enviar a mensagem manual abaixo.
       5. **Fluxo Manual**:
          - Monta HTML exato:
            ```text
            🔔 <b>Approval Required</b>

            {escapeHtml(info.Description) + "\n\n" se info.Description != ""}<b>Allow:</b> {escapeHtml(info.ApproveText)}
            <b>Allow Chat:</b> {escapeHtml(info.AlwaysAllowText ou "In Dropdown")}
            <b>Deny:</b> {escapeHtml(info.DenyText ou "(None)")}
            <b>Workspace:</b> {escapeHtml(projectName)}
            ```
          - Monta teclado inline em 1 linha com 3 botões:
            - `✅ Allow` $\rightarrow$ `BuildApprovalCustomID("approve", projectName, targetChannelStr)`
            - `✅ Allow Chat` $\rightarrow$ `BuildApprovalCustomID("always_allow", projectName, targetChannelStr)`
            - `❌ Deny` $\rightarrow$ `BuildApprovalCustomID("deny", projectName, targetChannelStr)`
          - Envia mensagem e salva `lastMessageID` e `lastMessageChatID`.
   - Inicia `detector.Start()` e registra `bridge.Pool.RegisterApprovalDetector(projectName, detector)`.

2. **`EnsurePlanningDetector(bridge, cdp, projectName)`**:
   - Se já existir detector ativo no pool para `projectName`, retorna.
   - Cria `PlanningDetector` com `PollInterval: 2000 * time.Millisecond`:
     - **`OnResolved`**: remove `reply_markup` da última mensagem enviada via `ClearMessageReplyMarkup`.
     - **`OnPlanningRequired(info)`**:
       - Resolve `targetChannel` via `GetCurrentChatTitle` + `ResolveApprovalChannelForCurrentChat`.
       - Constrói UI via `bridge.PlanUIBuilder(info, projectName, targetChannelStr)` (`buildPlanNotificationUI`).
       - Envia mensagem e armazena `lastMessageID` / `lastMessageChatID`.
   - Inicia `detector.Start()` e registra `bridge.Pool.RegisterPlanningDetector(projectName, detector)`.

3. **`EnsureErrorPopupDetector(bridge, cdp, projectName)`**:
   - Se já existir detector ativo no pool para `projectName`, retorna.
   - Cria `ErrorPopupDetector` com `PollInterval: 3000 * time.Millisecond`:
     - **`OnResolved`**: remove `reply_markup` da última mensagem enviada via `ClearMessageReplyMarkup`.
     - **`OnErrorPopup(info)`**:
       - Resolve `targetChannel` via `GetCurrentChatTitle` + `ResolveApprovalChannelForCurrentChat`.
       - Define `bodyText := info.Body`; se vazio, usa `t("An error occurred in the Antigravity agent.")`.
       - Define `titleText := info.Title`; se vazio, usa `"Agent Error"`.
       - Trunca `bodyText` em `3800` caracteres (`bodyText[:3800]`).
       - Define `buttonsText := strings.Join(info.Buttons, ", ")`; se vazio, usa `"(None)"`.
       - Monta HTML exato:
         ```text
         ❌ <b>{escapeHtml(titleText)}</b>

         {escapeHtml(bodyText[:3800])}

         <b>Buttons:</b> {escapeHtml(buttonsText)}
         <b>Workspace:</b> {escapeHtml(projectName)}
         ```
       - Monta teclado inline em 1 linha com 3 botões:
         - `🔇 Dismiss` $\rightarrow$ `BuildErrorPopupCustomID("dismiss", projectName, targetChannelStr)`
         - `📋 Copy Debug` $\rightarrow$ `BuildErrorPopupCustomID("copy_debug", projectName, targetChannelStr)`
         - `🔄 Retry` $\rightarrow$ `BuildErrorPopupCustomID("retry", projectName, targetChannelStr)`
       - Envia mensagem e armazena `lastMessageID` / `lastMessageChatID`.
   - Inicia `detector.Start()` e registra `bridge.Pool.RegisterErrorPopupDetector(projectName, detector)`.

4. **`EnsureUserMessageDetector(bridge, cdp, projectName, onUserMessage)`**:
   - Se já existir detector ativo no pool para `projectName`, retorna.
   - Cria `UserMessageDetector` com `PollInterval: 2000 * time.Millisecond` e `OnUserMessage: onUserMessage`.
   - Inicia `detector.Start()` e registra `bridge.Pool.RegisterUserMessageDetector(projectName, detector)`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Proteção dos Mapas de Roteamento no `CdpBridge`**:
   - Como múltiplos goroutines de detectores (`ApprovalDetector`, `PlanningDetector`, `ErrorPopupDetector`, `UserMessageDetector`) leem `ApprovalChannelBySession` e `ApprovalChannelByWorkspace` enquanto handlers de comandos/mensagens do Telegram escrevem neles via `RegisterApprovalSessionChannel` e `RegisterApprovalWorkspaceChannel`, todas as funções de leitura/escrita em `CdpBridge` devem adquirir `bridge.mu.RLock()` / `bridge.mu.Lock()`.
2. **Expiração Concorrente de `echoHashes` no `UserMessageDetector`**:
   - `AddEchoHash` pode ser chamado pela goroutine de envio de prompt do Telegram enquanto a goroutine de polling executa `PollOnce`. O acesso a `echoHashes`, `seenHashesOrder`, `seenHashesSet`, `lastDetectedHash` e `isPriming` é protegido por `d.mu.Lock()`.
   - O uso de contagem de referências (`map[string]int`) em `echoHashes` garante que, se o usuário enviar duas mensagens idênticas pelo Telegram em um intervalo de 30s, o timer de 60s da primeira mensagem não apague prematuramente o hash da segunda mensagem.
3. **Estado `lastMessageID` / `lastMessageChatID` nos `Ensure*Detector`**:
   - Cada closure de detector em `EnsureApprovalDetector`, `EnsurePlanningDetector` e `EnsureErrorPopupDetector` protege `lastMessageID` e `lastMessageChatID` com um `sync.Mutex` local para evitar condição de corrida entre `OnApprovalRequired` (gravando o ID após `SendMessage`) e `OnResolved` (lendo e limpando o ID para chamar `ClearMessageReplyMarkup`).

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

### 6.1. Testes de `ErrorPopupDetector` (`internal/bridge/error_detector_test.go`)
Mapeamento 1:1 de todos os 21 casos de teste de [`tests/services/errorPopupDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/errorPopupDetector.test.ts#L19-L571):

| # | Teste Original em `tests/services/errorPopupDetector.test.ts` | Função de Teste em Go (`error_detector_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `calls the onErrorPopup callback when an error popup is detected` (L50) | `TestErrorPopupDetector_CallsOnErrorPopupWhenDetected` | Mock CDP retorna `ErrorPopupInfo{Title: "Agent terminated due to error", Body: "The agent encountered an unexpected error and was terminated.", Buttons: ["Dismiss", "Copy debug info", "Retry"]}`. Após 1 ciclo, `onErrorPopup` é chamado 1 vez com os dados esperados. |
| 2 | `does not call the callback when no error popup exists` (L79) | `TestErrorPopupDetector_DoesNotCallCallbackWhenNull` | Mock CDP retorna `null`. `onErrorPopup` não é chamado. |
| 3 | `does not call the callback multiple times when the same error popup is detected` (L98) | `TestErrorPopupDetector_DeduplicatesConsecutiveDetections` | Mesmo `ErrorPopupInfo` em 3 ciclos seguidos. `onErrorPopup` é chamado apenas 1 vez. |
| 4 | `treats detections with same title and body as duplicate` (L125) | `TestErrorPopupDetector_DeduplicatesByTitleAndBodyPrefix` | 1º ciclo tem `Buttons: ["Dismiss", "Retry"]`, 2º ciclo tem `Buttons: ["Dismiss", "Copy debug info", "Retry"]`, mas mesmo `Title` e `Body`. `onErrorPopup` é chamado apenas 1 vez e `GetLastDetectedInfo().Buttons` é atualizado silenciosamente. |
| 5 | `suppresses re-detection within 10s cooldown even after key reset` (L153) | `TestErrorPopupDetector_SuppressesRedetectionWithin10sCooldown` | Detecta (`t = 500ms`) $\rightarrow$ desaparece (`t = 1000ms`, reseta chave) $\rightarrow$ reaparece (`t = 1500ms`, dentro do cooldown de 10s). `onErrorPopup` permanece com 1 chamada. |
| 6 | `executes a click script via CDP when clickDismissButton() is called` (L183) | `TestErrorPopupDetector_ClickDismissButtonViaCDP` | `ClickDismissButton(ctx)` retorna `true` e envia `Runtime.evaluate` contendo `"Dismiss"`, `returnByValue: true`, `contextId: 42`. |
| 7 | `executes a click script via CDP when clickCopyDebugInfoButton() is called` (L210) | `TestErrorPopupDetector_ClickCopyDebugInfoButtonViaCDP` | `ClickCopyDebugInfoButton(ctx)` retorna `true` e envia `Runtime.evaluate` contendo `"Copy debug info"`, `returnByValue: true`, `contextId: 42`. |
| 8 | `executes a click script via CDP when clickRetryButton() is called` (L237) | `TestErrorPopupDetector_ClickRetryButtonViaCDP` | `ClickRetryButton(ctx)` retorna `true` e envia `Runtime.evaluate` contendo `"Retry"`, `returnByValue: true`, `contextId: 42`. |
| 9 | `readClipboard() returns the clipboard text` (L264) | `TestErrorPopupDetector_ReadClipboardReturnsText` | Mock CDP retorna `"Error: Agent terminated\nStack: at line 42\nVersion: 1.0.0"`. `ReadClipboard(ctx)` retorna essa string e passa `awaitPromise: true` para `Runtime.evaluate`. |
| 10 | `readClipboard() returns null when clipboard content is not available` (L290) | `TestErrorPopupDetector_ReadClipboardReturnsNilWhenEmpty` | Mock CDP retorna `null`. `ReadClipboard(ctx)` retorna `nil`. |
| 11 | `readClipboard() returns null when a CDP error occurs` (L309) | `TestErrorPopupDetector_ReadClipboardReturnsNilOnCDPError` | Mock CDP falha com `"CDP connection lost"`. `ReadClipboard(ctx)` retorna `nil` sem propagar panic. |
| 12 | `stops polling and no longer calls the callback after stop()` (L330) | `TestErrorPopupDetector_StopHaltsPolling` | Após 1ª detecção, chama `Stop()`. Ciclos seguintes não disparam `onErrorPopup`. |
| 13 | `continues monitoring even when a CDP error occurs` (L358) | `TestErrorPopupDetector_ContinuesOnCDPError` | 1º ciclo retorna erro `"CDP error"`; 2º ciclo retorna `Title: "Recovery Error"`. `onErrorPopup` é chamado no 2º ciclo. |
| 14 | `getLastDetectedInfo() returns the detected ErrorPopupInfo` (L388) | `TestErrorPopupDetector_GetLastDetectedInfoReturnsDetected` | Retorna `nil` antes do 1º ciclo e retorna `Title: "Custom Error"`, `Body: "Something went wrong with the agent."` após detecção. |
| 15 | `getLastDetectedInfo() returns null when error popup disappears` (L419) | `TestErrorPopupDetector_GetLastDetectedInfoResetsOnDisappear` | 1º ciclo detecta (`!= nil`); 2º ciclo retorna `null` (`== nil`). |
| 16 | `isActive() returns true while running and false after stop` (L443) | `TestErrorPopupDetector_IsActiveLifecycle` | Verifica `false` $\rightarrow$ `Start()` $\rightarrow$ `true` $\rightarrow$ `Stop()` $\rightarrow$ `false`. |
| 17 | `calls without the contextId parameter when contextId is null` (L462) | `TestErrorPopupDetector_OmitsContextIdWhenNil` | Quando `GetPrimaryContextId()` retorna `nil`, `params` não possui a chave `"contextId"`. |
| 18 | `clickDismissButton() returns false when a CDP error occurs` (L484) | `TestErrorPopupDetector_ClickDismissButtonReturnsFalseOnError` | Mock CDP retorna erro `"CDP error"`. `ClickDismissButton(ctx)` retorna `false`. |
| 19 | `silently ignores WebSocket disconnect errors during polling` (L505) | `TestErrorPopupDetector_SilentlyIgnoresWebSocketDisconnect` | Mock CDP retorna erro `"WebSocket is not connected"`. `onErrorPopup` não é chamado e nenhum log de nível `ERROR` é emitido. |
| 20 | `calls onResolved when error popup disappears after detection` (L530) | `TestErrorPopupDetector_CallsOnResolvedWhenPopupDisappears` | 1º ciclo detecta (`onResolved` não chamado); 2º ciclo retorna `null` (`onResolved` chamado 1 vez). |
| 21 | `does not call onResolved when popup was never detected` (L553) | `TestErrorPopupDetector_DoesNotCallOnResolvedWithoutPriorDetection` | Dois ciclos retornando `null`. `onResolved` nunca é chamado. |

---

### 6.2. Testes de `UserMessageDetector` (`internal/bridge/user_message_detector_test.go`)
Mapeamento 1:1 de todos os 11 casos de teste de [`tests/services/userMessageDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/userMessageDetector.test.ts#L14-L359):

| # | Teste Original em `tests/services/userMessageDetector.test.ts` | Função de Teste em Go (`user_message_detector_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `first poll primes existing DOM state without firing callback` (L27) | `TestUserMessageDetector_FirstPollPrimesWithoutCallback` | Mock CDP retorna `{text: "Existing message"}`. 1º ciclo (priming) não chama `onUserMessage`; 2º ciclo (mesma mensagem) também não chama `onUserMessage`. |
| 2 | `detects a new user message after priming` (L52) | `TestUserMessageDetector_DetectsNewMessageAfterPriming` | 1º ciclo retorna `{text: "Old message"}` (priming, 0 chamadas); 2º ciclo retorna `{text: "Hello from PC"}` (chama `onUserMessage` com `{Text: "Hello from PC"}`). Verifica `IsActive()` durante e após `Stop()`. |
| 3 | `primes with empty DOM and detects first real message` (L85) | `TestUserMessageDetector_PrimesWithEmptyDOMAndDetectsFirstMessage` | 1º ciclo retorna `null` (priming com DOM vazio); 2º ciclo retorna `{text: "First message"}` e dispara `onUserMessage({Text: "First message"})`. |
| 4 | `does not call onUserMessage for duplicate messages` (L115) | `TestUserMessageDetector_DeduplicatesSameMessage` | 1º ciclo `null` (priming); 2º e 3º ciclos retornam `{text: "Same message"}`. `onUserMessage` é chamado apenas 1 vez. |
| 5 | `skips messages matching echo hashes` (L148) | `TestUserMessageDetector_SkipsEchoHashedMessages` | Chama `detector.AddEchoHash("Echoed message")` antes de `Start()`. 1º ciclo `null`; 2º ciclo `{text: "Echoed message"}`. `onUserMessage` não é chamado (`0` vezes). |
| 6 | `handles CDP errors gracefully` (L180) | `TestUserMessageDetector_HandlesCDPErrorsGracefully` | Mock CDP retorna erro `"CDP timeout"`. Detector permanece ativo (`IsActive() == true`) e `onUserMessage` não é chamado. |
| 7 | `handles null result from CDP` (L201) | `TestUserMessageDetector_HandlesNullCDPResult` | Mock CDP retorna `null`. `onUserMessage` não é chamado. |
| 8 | `detects new message after different message` (L222) | `TestUserMessageDetector_DetectsSequentialDifferentMessages` | 1º ciclo `null`; 2º ciclo `"First message"`; 3º ciclo `"Second message"`. `onUserMessage` é chamado 2 vezes na ordem exata. |
| 9 | `start() is idempotent` (L260) | `TestUserMessageDetector_StartIsIdempotent` | Chamar `Start()` duas vezes seguidas não cria goroutines duplicadas de polling e mantém `IsActive() == true`. |
| 10 | `seenHashes prevents re-detection of old messages after a different message appears` (L276) | `TestUserMessageDetector_SeenHashesPreventsOldMessageRedetection` | 1º ciclo `null`; 2º ciclo `"Message A"` (notifica); 3º ciclo `"Message B"` (notifica); 4º ciclo reverte para `"Message A"` (ignorado por `seenHashes`). Total de chamadas a `onUserMessage`: `2`. |
| 11 | `seenHashes are cleared on restart` (L321) | `TestUserMessageDetector_SeenHashesClearedOnRestart` | Sessão 1: prime `null` $\rightarrow$ detecta `"Restart message"` (`1` chamada) $\rightarrow$ `Stop()`. Sessão 2: `Start()` $\rightarrow$ prime `null` $\rightarrow$ detecta `"Restart message"` novamente (`2` chamadas no total). |

---

### 6.3. Testes de `CdpBridgeManager` (`internal/bridge/manager_test.go`)
Mapeamento 1:1 de todos os 14 casos de teste de [`tests/services/cdpBridgeManager.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpBridgeManager.test.ts#L13-L107):

| # | Teste Original em `tests/services/cdpBridgeManager.test.ts` | Função de Teste em Go (`manager_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `initCdpBridge builds the initial state` (L14) | `TestCdpBridgeManager_InitCdpBridgeInitialState` | `InitCdpBridge(false, ...)` inicializa `LastActiveWorkspace == nil`, `LastActiveChannel == nil` e `AutoAccept.IsEnabled() == false`. |
| 2 | `getCurrentCdp returns null when not connected` (L21) | `TestCdpBridgeManager_GetCurrentCdpReturnsNilWhenDisconnected` | Com pool sem workspaces conectados, `GetCurrentCdp(bridge)` retorna `nil`. |
| 3 | `round-trips build/parse of approval action ID` (L26) | `TestCdpBridgeManager_ApprovalCustomIDRoundTrip` | `BuildApprovalCustomID("approve", "my-workspace", "123456")` gera `"approve_action:my-workspace:123456"` e `ParseApprovalCustomID` retorna `{Action: "approve", ProjectName: "my-workspace", ChannelID: "123456"}`. |
| 4 | `supports legacy approval action IDs without channelId` (L32) | `TestCdpBridgeManager_ParseLegacyApprovalCustomIDWithoutChannel` | `ParseApprovalCustomID("approve_action:legacy-workspace")` retorna `{Action: "approve", ProjectName: "legacy-workspace", ChannelID: nil}`. |
| 5 | `routes approval notifications by session title when linked` (L37) | `TestCdpBridgeManager_RoutesBySessionTitleWhenLinked` | Registra `RegisterApprovalSessionChannel(bridge, "ws-a", "Session Alpha", ch)`. `ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptr("Session Alpha"))` retorna `ch` (case-insensitive e com trim). |
| 6 | `falls back to workspace channel when session title does not match` (L45) | `TestCdpBridgeManager_FallsBackToWorkspaceChannelOnMismatch` | Registra `RegisterApprovalWorkspaceChannel(bridge, "ws-a", wsCh)`. `ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptr("Unknown Session"))` retorna `wsCh`. |
| 7 | `falls back to workspace channel when currentChatTitle is null` (L53) | `TestCdpBridgeManager_FallsBackToWorkspaceChannelWhenTitleNil` | Com `currentChatTitle == nil`, `ResolveApprovalChannelForCurrentChat(bridge, "ws-a", nil)` retorna `wsCh`. |
| 8 | `falls back to workspace channel when currentChatTitle is empty` (L61) | `TestCdpBridgeManager_FallsBackToWorkspaceChannelWhenTitleEmpty` | Com `currentChatTitle == ptr("")`, `ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptr(""))` retorna `wsCh`. |
| 9 | `prefers session channel over workspace channel` (L69) | `TestCdpBridgeManager_PrefersSessionChannelOverWorkspaceChannel` | Registra `wsCh` (sem thread) e depois `sessionCh` (`ThreadID: 2`) para `"Session Alpha"`. `ResolveApprovalChannelForCurrentChat(bridge, "ws-a", ptr("Session Alpha"))` retorna `sessionCh`. |
| 10 | `returns null when neither session nor workspace is registered` (L79) | `TestCdpBridgeManager_ReturnsNilWhenUnregistered` | Para `"ws-unknown"`, `ResolveApprovalChannelForCurrentChat` retorna `nil` tanto com título quanto com `nil`. |
| 11 | `round-trips build/parse of planning open action ID` (L86) | `TestCdpBridgeManager_PlanningOpenCustomIDRoundTrip` | `BuildPlanningCustomID("open", "my-workspace", "123456")` $\rightarrow$ `ParsePlanningCustomID` retorna `{Action: "open", ProjectName: "my-workspace", ChannelID: "123456"}`. |
| 12 | `round-trips build/parse of planning proceed action ID` (L92) | `TestCdpBridgeManager_PlanningProceedCustomIDRoundTrip` | `BuildPlanningCustomID("proceed", "my-workspace", "789")` $\rightarrow$ `ParsePlanningCustomID` retorna `{Action: "proceed", ProjectName: "my-workspace", ChannelID: "789"}`. |
| 13 | `supports planning action IDs without channelId` (L98) | `TestCdpBridgeManager_ParseLegacyPlanningCustomIDWithoutChannel` | `ParsePlanningCustomID("planning_open_action:legacy-workspace")` retorna `{Action: "open", ProjectName: "legacy-workspace", ChannelID: nil}`. |
| 14 | `parsePlanningCustomId returns null for non-planning IDs` (L103) | `TestCdpBridgeManager_ParsePlanningCustomIDReturnsNilForInvalid` | `ParsePlanningCustomID("approve_action:ws-a")` e `ParsePlanningCustomID("random_string")` retornam `nil`. |
