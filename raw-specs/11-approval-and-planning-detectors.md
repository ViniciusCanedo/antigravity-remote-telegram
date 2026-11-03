# 11 — Especificação Técnica: `ApprovalDetector` e `PlanningDetector` (`internal/bridge`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação define de forma 100% autocontida e exaustiva o port para **Golang (1.23+)** dos detectores de aprovação de ações/ferramentas (`ApprovalDetector`) e do modo de planejamento (`PlanningDetector`) que monitoram em background o DOM da janela do Antigravity via Chrome DevTools Protocol (`Runtime.evaluate`).

### 1.1. Mapeamento de Arquivos Originais (TypeScript) para Destino (Go)

| Componente | Arquivo Original (TypeScript) | Linhas | Arquivo de Teste Original (Jest) | Linhas | Destino no Port Go | Arquivo de Teste Go |
| :--- | :--- | :---: | :--- | :---: | :--- | :--- |
| **Detector de Aprovação** | [`src/services/approvalDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/approvalDetector.ts#L1-L428) | 428 | [`tests/services/approvalDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/approvalDetector.test.ts#L1-L480) | 480 | `internal/bridge/approval_detector.go` | `internal/bridge/approval_detector_test.go` |
| **Detector de Planejamento** | [`src/services/planningDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/planningDetector.ts#L1-L353) | 353 | [`tests/services/planningDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/planningDetector.test.ts#L1-L572) | 572 | `internal/bridge/planning_detector.go` | `internal/bridge/planning_detector_test.go` |
| **Documentação de Arquitetura** | [`docs/05-detectores-e-servicos-background.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md#L39-L64) | 151 | — | — | Pacote `bridge` (`internal/bridge`) | — |

### 1.2. Responsabilidades no Sistema

1. **`ApprovalDetector` ([`src/services/approvalDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/approvalDetector.ts#L229-L428))**:
   - Realiza polling periódico no DOM do Antigravity (padrão `1500ms` na classe, configurável para `2000ms` via `ensureApprovalDetector`) buscando pares de botões visíveis de permissão (`Allow` / `Allow Once` + `Deny`, e opcionalmente `Allow This Conversation` / `Always Allow`).
   - Extrai os rótulos exatos dos botões (`approveText`, `alwaysAllowText`, `denyText`) e a descrição da ação pendente (`description`) com 3 níveis de fallback.
   - Evita notificações duplicadas através da chave composta `lastDetectedKey = "${info.approveText}::${info.description}"`.
   - Dispara `OnApprovalRequired(info)` em novas detecções e `OnResolved()` quando o diálogo desaparece do DOM após ter sido detectado.
   - Expõe métodos de ação remota via CDP: `ApproveButton(ctx, buttonText)`, `AlwaysAllowButton(ctx)` (com clique direto ou expansão automática do menu dropdown ao lado do botão `Allow Once` seguida de até 5 tentativas espaçadas em `120ms`) e `DenyButton(ctx, buttonText)`.
   - Exporta a função utilitária compartilhada `BuildClickScript(buttonText string) string` (originalmente [`buildClickScript`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/approvalDetector.ts#L201-L221)), reutilizada por `PlanningDetector` e `ErrorPopupDetector`.

2. **`PlanningDetector` ([`src/services/planningDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/planningDetector.ts#L174-L353))**:
   - Realiza polling periódico no DOM do Antigravity (padrão `2000ms`) buscando containers `.notify-user-container` que contenham simultaneamente botões visíveis `Open` e `Proceed`.
   - Extrai metadados do card de plano (`openText`, `proceedText`, `planTitle`, `planSummary`, `description` sem blocos `PRE`/`CODE`/`STYLE`/`SCRIPT` limitada a 500 caracteres).
   - Evita notificações duplicadas combinando a chave estável `lastDetectedKey = "${info.openText}::${info.proceedText}"` com uma janela de cooldown de `5000ms` (`PlanningCooldown = 5 * time.Second`) baseada em `lastNotifiedAt`.
   - Dispara `OnPlanningRequired(info)` em novas detecções fora do cooldown, atualiza silenciosamente `lastDetectedInfo` quando a chave permanece igual e dispara `OnResolved()` quando os botões desaparecem do DOM.
   - Expõe `ClickOpenButton(ctx, buttonText)`, `ClickProceedButton(ctx, buttonText)` e `ExtractPlanContent(ctx)` (que converte o HTML renderizado do plano aberto em Markdown limpo via `EXTRACT_PLAN_CONTENT_SCRIPT`).

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

O código abaixo define integralmente os tipos, interfaces de desacoplamento CDP, structs de configuração/estado e assinaturas de métodos que devem ser implementados em `internal/bridge/approval_detector.go` e `internal/bridge/planning_detector.go`.

```go
package bridge

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// CdpEvaluator abstrai as operações do CdpService necessárias para todos os detectores em background.
// Corresponde aos métodos call('Runtime.evaluate', ...) e getPrimaryContextId() de src/services/cdpService.ts.
type CdpEvaluator interface {
	// Call executa um comando CDP arbitrário (ex: "Runtime.evaluate") e retorna o payload JSON bruto.
	Call(ctx context.Context, method string, params map[string]any) (json.RawMessage, error)
	// GetPrimaryContextId retorna o ID do contexto de execução principal da janela do Antigravity,
	// ou nil caso nenhum contexto primário esteja registrado (quando contextId deve ser omitido da chamada).
	GetPrimaryContextId() *int64
}

// runtimeEvaluateResponse representa o envelope padrão de resposta do Chrome DevTools Protocol
// para chamadas Runtime.evaluate com returnByValue: true.
type runtimeEvaluateResponse struct {
	Result struct {
		Value json.RawMessage `json:"value"`
	} `json:"result"`
}

// clickScriptResult representa o retorno dos scripts buildClickScript e EXPAND_ALWAYS_ALLOW_MENU_SCRIPT.
type clickScriptResult struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ============================================================================
// 2.1. ApprovalDetector (internal/bridge/approval_detector.go)
// Origem: src/services/approvalDetector.ts
// ============================================================================

// ApprovalInfo representa as informações dos botões de aprovação extraídas do DOM.
// Origem: interface ApprovalInfo (src/services/approvalDetector.ts#L5-L14)
type ApprovalInfo struct {
	// ApproveText é o texto do botão de aprovação (ex: "Allow", "Allow Once", "許可").
	ApproveText string `json:"approveText"`
	// AlwaysAllowText é o texto do botão de permissão para toda a conversa (ex: "Allow This Conversation").
	// No script JS retorna string vazia "" quando não está diretamente visível.
	AlwaysAllowText string `json:"alwaysAllowText,omitempty"`
	// DenyText é o texto do botão de negação (ex: "Deny", "拒否").
	DenyText string `json:"denyText"`
	// Description é a descrição da ação solicitada (ex: "write to file.ts").
	Description string `json:"description"`
}

// ApprovalDetectorOptions configura uma instância de ApprovalDetector.
// Origem: interface ApprovalDetectorOptions (src/services/approvalDetector.ts#L16-L25)
type ApprovalDetectorOptions struct {
	// CdpService é a instância de conexão CDP com o workspace.
	CdpService CdpEvaluator
	// PollInterval é o intervalo entre ciclos de polling (padrão: 1500ms se <= 0).
	PollInterval time.Duration
	// OnApprovalRequired é o callback invocado quando um novo botão de aprovação é detectado.
	OnApprovalRequired func(info ApprovalInfo)
	// OnResolved é o callback opcional invocado quando uma aprovação previamente detectada desaparece do DOM.
	OnResolved func()
	// Logger opcional (usa slog.Default() se nil).
	Logger *slog.Logger
	// NowFunc opcional para injeção de relógio determinístico em testes (usa time.Now se nil).
	NowFunc func() time.Time
	// SleepFunc opcional para injeção de espera determinística no dropdown de AlwaysAllowButton (usa time.Sleep/timer se nil).
	SleepFunc func(ctx context.Context, d time.Duration) error
}

// ApprovalDetector detecta botões de aprovação na UI do Antigravity via polling.
// Origem: class ApprovalDetector (src/services/approvalDetector.ts#L229-L428)
type ApprovalDetector struct {
	cdpService         CdpEvaluator
	pollInterval       time.Duration
	onApprovalRequired func(info ApprovalInfo)
	onResolved         func()
	logger             *slog.Logger
	sleepFunc          func(ctx context.Context, d time.Duration) error

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedKey  *string
	lastDetectedInfo *ApprovalInfo
}

// Assinaturas públicas e privadas de ApprovalDetector:
func BuildClickScript(buttonText string) string
func NewApprovalDetector(options ApprovalDetectorOptions) *ApprovalDetector
func (d *ApprovalDetector) Start()
func (d *ApprovalDetector) Stop() error
func (d *ApprovalDetector) IsActive() bool
func (d *ApprovalDetector) GetLastDetectedInfo() *ApprovalInfo
func (d *ApprovalDetector) ApproveButton(ctx context.Context, buttonText ...string) (bool, error)
func (d *ApprovalDetector) AlwaysAllowButton(ctx context.Context) (bool, error)
func (d *ApprovalDetector) DenyButton(ctx context.Context, buttonText ...string) (bool, error)
func (d *ApprovalDetector) PollOnce(ctx context.Context) // Exportado/usado internamente e para testes determinísticos
func (d *ApprovalDetector) clickButton(ctx context.Context, buttonText string) bool
func (d *ApprovalDetector) runEvaluateScript(ctx context.Context, expression string) (json.RawMessage, error)

// ============================================================================
// 2.2. PlanningDetector (internal/bridge/planning_detector.go)
// Origem: src/services/planningDetector.ts
// ============================================================================

// PlanningInfo representa as informações dos botões e metadados do modo de planejamento extraídos do DOM.
// Origem: interface PlanningInfo (src/services/planningDetector.ts#L6-L17)
type PlanningInfo struct {
	// OpenText é o texto do botão Open (ex: "Open").
	OpenText string `json:"openText"`
	// ProceedText é o texto do botão Proceed (ex: "Proceed").
	ProceedText string `json:"proceedText"`
	// PlanTitle é o título do plano (nome do arquivo exibido no card).
	PlanTitle string `json:"planTitle"`
	// PlanSummary é o texto de resumo do plano extraído de span.text-sm.
	PlanSummary string `json:"planSummary"`
	// Description é a descrição do plano (texto extraído de .leading-relaxed.select-text sem tags de código/estilo).
	Description string `json:"description"`
}

// PlanningDetectorOptions configura uma instância de PlanningDetector.
// Origem: interface PlanningDetectorOptions (src/services/planningDetector.ts#L19-L28)
type PlanningDetectorOptions struct {
	// CdpService é a instância de conexão CDP com o workspace.
	CdpService CdpEvaluator
	// PollInterval é o intervalo entre ciclos de polling (padrão: 2000ms se <= 0).
	PollInterval time.Duration
	// OnPlanningRequired é o callback invocado quando botões de planejamento são detectados.
	OnPlanningRequired func(info PlanningInfo)
	// OnResolved é o callback opcional invocado quando os botões de planejamento desaparecem do DOM.
	OnResolved func()
	// Logger opcional (usa slog.Default() se nil).
	Logger *slog.Logger
	// NowFunc opcional para injeção de relógio determinístico em testes de cooldown (usa time.Now se nil).
	NowFunc func() time.Time
}

// PlanningDetector detecta botões do modo de planejamento (Open/Proceed) na UI do Antigravity via polling.
// Origem: class PlanningDetector (src/services/planningDetector.ts#L174-L353)
type PlanningDetector struct {
	cdpService         CdpEvaluator
	pollInterval       time.Duration
	onPlanningRequired func(info PlanningInfo)
	onResolved         func()
	logger             *slog.Logger
	nowFunc            func() time.Time

	mu               sync.RWMutex
	isRunning        bool
	cancelPoll       context.CancelFunc
	pollDone         chan struct{}
	lastDetectedKey  *string
	lastDetectedInfo *PlanningInfo
	lastNotifiedAt   time.Time
}

// Assinaturas públicas e privadas de PlanningDetector:
func NewPlanningDetector(options PlanningDetectorOptions) *PlanningDetector
func (d *PlanningDetector) Start()
func (d *PlanningDetector) Stop() error
func (d *PlanningDetector) IsActive() bool
func (d *PlanningDetector) GetLastDetectedInfo() *PlanningInfo
func (d *PlanningDetector) ClickOpenButton(ctx context.Context, buttonText ...string) (bool, error)
func (d *PlanningDetector) ClickProceedButton(ctx context.Context, buttonText ...string) (bool, error)
func (d *PlanningDetector) ExtractPlanContent(ctx context.Context) (*string, error)
func (d *PlanningDetector) PollOnce(ctx context.Context) // Exportado/usado internamente e para testes determinísticos
func (d *PlanningDetector) clickButton(ctx context.Context, buttonText string) bool
func (d *PlanningDetector) runEvaluateScript(ctx context.Context, expression string) (json.RawMessage, error)
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

> [!IMPORTANT]
> Todos os scripts JavaScript abaixo são avaliados diretamente no navegador Chromium/Electron do Antigravity via `Runtime.evaluate`. Eles **DEVEM** ser mantidos em Go exatamente como constantes `string` literais (sem alterações de seletores, padrões de idioma ou lógica de árvore DOM).

### 3.1. Constantes de Tempo, Tentativas e Padrões (`ApprovalDetector` e `PlanningDetector`)

```go
const (
	// DefaultApprovalPollInterval é o intervalo padrão do ApprovalDetector (src/services/approvalDetector.ts#L244).
	DefaultApprovalPollInterval = 1500 * time.Millisecond

	// DefaultBridgeApprovalPollInterval é o intervalo usado quando instanciado pelo CdpBridgeManager (src/services/cdpBridgeManager.ts#L259).
	DefaultBridgeApprovalPollInterval = 2000 * time.Millisecond

	// AlwaysAllowExpandMaxRetries é o número de iterações após expandir o menu dropdown (src/services/approvalDetector.ts#L373).
	AlwaysAllowExpandMaxRetries = 5

	// AlwaysAllowExpandRetryDelay é o intervalo de espera entre tentativas após expandir o dropdown (src/services/approvalDetector.ts#L377).
	AlwaysAllowExpandRetryDelay = 120 * time.Millisecond

	// DefaultApproveFallbackText é o texto padrão para ApproveButton quando nenhum argumento ou detecção prévia existe (src/services/approvalDetector.ts#L346).
	DefaultApproveFallbackText = "Allow"

	// DefaultDenyFallbackText é o texto padrão para DenyButton quando nenhum argumento ou detecção prévia existe (src/services/approvalDetector.ts#L389).
	DefaultDenyFallbackText = "Deny"

	// DefaultPlanningPollInterval é o intervalo padrão do PlanningDetector (src/services/planningDetector.ts#L193).
	DefaultPlanningPollInterval = 2000 * time.Millisecond

	// PlanningCooldown é o período de supressão de re-notificação no PlanningDetector (src/services/planningDetector.ts#L189).
	PlanningCooldown = 5000 * time.Millisecond

	// DefaultOpenFallbackText é o texto padrão para ClickOpenButton (src/services/planningDetector.ts#L233).
	DefaultOpenFallbackText = "Open"

	// DefaultProceedFallbackText é o texto padrão para ClickProceedButton (src/services/planningDetector.ts#L243).
	DefaultProceedFallbackText = "Proceed"
)

// DefaultAlwaysAllowCandidates define os rótulos candidatos tentados em ordem por AlwaysAllowButton()
// após lastDetectedInfo.AlwaysAllowText (src/services/approvalDetector.ts#L355-L362).
var DefaultAlwaysAllowCandidates = []string{
	"Allow This Conversation",
	"Allow This Chat",
	"この会話を許可",
	"Always Allow",
	"常に許可",
}
```

### 3.2. `DETECT_APPROVAL_SCRIPT` ([`src/services/approvalDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/approvalDetector.ts#L32-L121))

```go
const DetectApprovalScript = `(() => {
    const ALLOW_ONCE_PATTERNS = ['allow once', 'allow one time', '今回のみ許可', '1回のみ許可', '一度許可'];
    const ALWAYS_ALLOW_PATTERNS = [
        'allow this conversation',
        'allow this chat',
        'always allow',
        '常に許可',
        'この会話を許可',
    ];
    const ALLOW_PATTERNS = ['allow', 'permit', '許可', '承認', '確認'];
    const DENY_PATTERNS = ['deny', '拒否', 'decline'];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();

    const allButtons = Array.from(document.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    let approveBtn = allButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALLOW_ONCE_PATTERNS.some(p => t.includes(p));
    }) || null;

    if (!approveBtn) {
        approveBtn = allButtons.find(btn => {
            const t = normalize(btn.textContent || '');
            const isAlways = ALWAYS_ALLOW_PATTERNS.some(p => t.includes(p));
            return !isAlways && ALLOW_PATTERNS.some(p => t.includes(p));
        }) || null;
    }

    if (!approveBtn) return null;

    const container = approveBtn.closest('[role="dialog"], .modal, .dialog, .approval-container, .permission-dialog')
        || approveBtn.parentElement?.parentElement
        || approveBtn.parentElement
        || document.body;

    const containerButtons = Array.from(container.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const denyBtn = containerButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return DENY_PATTERNS.some(p => t.includes(p));
    }) || null;

    if (!denyBtn) return null;

    const alwaysAllowBtn = containerButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALWAYS_ALLOW_PATTERNS.some(p => t.includes(p));
    }) || null;

    const approveText = (approveBtn.textContent || '').trim();
    const alwaysAllowText = alwaysAllowBtn ? (alwaysAllowBtn.textContent || '').trim() : '';
    const denyText = (denyBtn.textContent || '').trim();

    // Description extraction (multiple fallbacks)
    let description = '';

    // 1. p or .description inside dialog/modal
    const dialog = container;
    if (dialog) {
        const descEl = dialog.querySelector('p, .description, [data-testid="description"]');
        if (descEl) {
            description = (descEl.textContent || '').trim();
        }
    }

    // 2. Parent element text (excluding button text)
    if (!description) {
        const parent = approveBtn.parentElement?.parentElement || approveBtn.parentElement;
        if (parent) {
            const clone = parent.cloneNode(true);
            const buttons = clone.querySelectorAll('button');
            buttons.forEach(b => b.remove());
            const parentText = (clone.textContent || '').trim();
            if (parentText.length > 5 && parentText.length < 500) {
                description = parentText;
            }
        }
    }

    // 3. aria-label fallback
    if (!description) {
        const ariaLabel = approveBtn.getAttribute('aria-label') || '';
        if (ariaLabel) description = ariaLabel;
    }

    return { approveText, alwaysAllowText, denyText, description };
})()`
```

### 3.3. `EXPAND_ALWAYS_ALLOW_MENU_SCRIPT` ([`src/services/approvalDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/approvalDetector.ts#L126-L194))

```go
const ExpandAlwaysAllowMenuScript = `(() => {
    const ALLOW_ONCE_PATTERNS = ['allow once', 'allow one time', '今回のみ許可', '1回のみ許可', '一度許可'];
    const ALWAYS_ALLOW_PATTERNS = [
        'allow this conversation',
        'allow this chat',
        'always allow',
        '常に許可',
        'この会話を許可',
    ];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();
    const visibleButtons = Array.from(document.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const directAlways = visibleButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALWAYS_ALLOW_PATTERNS.some(p => t.includes(p));
    });
    if (directAlways) return { ok: true, reason: 'already-visible' };

    const allowOnceBtn = visibleButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return ALLOW_ONCE_PATTERNS.some(p => t.includes(p));
    });
    if (!allowOnceBtn) return { ok: false, error: 'allow-once button not found' };

    const container = allowOnceBtn.closest('[role="dialog"], .modal, .dialog, .approval-container, .permission-dialog')
        || allowOnceBtn.parentElement?.parentElement
        || allowOnceBtn.parentElement
        || document.body;

    const containerButtons = Array.from(container.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const toggleBtn = containerButtons.find(btn => {
        if (btn === allowOnceBtn) return false;
        const text = normalize(btn.textContent || '');
        const aria = normalize(btn.getAttribute('aria-label') || '');
        const hasPopup = btn.getAttribute('aria-haspopup');
        if (hasPopup === 'menu' || hasPopup === 'listbox') return true;
        if (text === '') return true;
        return /menu|more|expand|options|dropdown|chevron|arrow/.test(aria);
    });

    if (toggleBtn) {
        toggleBtn.click();
        return { ok: true, reason: 'toggle-button' };
    }

    const rect = allowOnceBtn.getBoundingClientRect();
    if (!rect || rect.width <= 0 || rect.height <= 0) {
        return { ok: false, error: 'allow-once button rect unavailable' };
    }

    const clickX = rect.right - Math.max(4, Math.min(12, rect.width * 0.15));
    const clickY = rect.top + rect.height / 2;

    const events = ['pointerdown', 'mousedown', 'mouseup', 'click'];
    for (const type of events) {
        allowOnceBtn.dispatchEvent(new MouseEvent(type, {
            bubbles: true,
            cancelable: true,
            view: window,
            clientX: clickX,
            clientY: clickY,
        }));
    }
    return { ok: true, reason: 'allow-once-right-edge' };
})()`
```

### 3.4. `BuildClickScript` ([`src/services/approvalDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/approvalDetector.ts#L201-L221))

```go
// BuildClickScript gera um script CDP IIFE que localiza um botão visível pelo texto ou aria-label e o clica.
// Usa json.Marshal para escapar buttonText de forma idêntica a JSON.stringify(buttonText).
func BuildClickScript(buttonText string) string {
	safeBytes, _ := json.Marshal(buttonText)
	safeText := string(safeBytes)
	return `(() => {
        const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const text = ` + safeText + `;
        const wanted = normalize(text);
        const allButtons = Array.from(document.querySelectorAll('button'));
        const target = allButtons.find(btn => {
            if (!btn.offsetParent) return false;
            const buttonText = normalize(btn.textContent || '');
            const ariaLabel = normalize(btn.getAttribute('aria-label') || '');
            return buttonText === wanted ||
                ariaLabel === wanted ||
                buttonText.includes(wanted) ||
                ariaLabel.includes(wanted);
        });
        if (!target) return { ok: false, error: 'Button not found: ' + text };
        target.click();
        return { ok: true };
    })()`
}
```

### 3.5. `DETECT_PLANNING_SCRIPT` ([`src/services/planningDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/planningDetector.ts#L36-L95))

```go
const DetectPlanningScript = `(() => {
    const OPEN_PATTERNS = ['open'];
    const PROCEED_PATTERNS = ['proceed'];

    const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();

    // Find the notify container that holds planning UI
    const container = document.querySelector('.notify-user-container');
    if (!container) return null;

    const allButtons = Array.from(container.querySelectorAll('button'))
        .filter(btn => btn.offsetParent !== null);

    const openBtn = allButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return OPEN_PATTERNS.some(p => t === p || t.includes(p));
    }) || null;

    const proceedBtn = allButtons.find(btn => {
        const t = normalize(btn.textContent || '');
        return PROCEED_PATTERNS.some(p => t === p || t.includes(p));
    }) || null;

    // Both buttons must exist for this to be a planning UI
    if (!openBtn || !proceedBtn) return null;

    const openText = (openBtn.textContent || '').trim();
    const proceedText = (proceedBtn.textContent || '').trim();

    // Extract plan title from .inline-flex.break-all
    const titleEl = container.querySelector('span.inline-flex.break-all, .inline-flex.break-all');
    const planTitle = titleEl ? (titleEl.textContent || '').trim() : '';

    // Extract plan summary from span.text-sm (excluding buttons text)
    const summaryEls = Array.from(container.querySelectorAll('span.text-sm'));
    const planSummary = summaryEls
        .map(el => (el.textContent || '').trim())
        .filter(text => text.length > 0 && text !== openText && text !== proceedText)
        .join(' ');

    // Extract description from leading-relaxed container, skipping code/style blocks
    const descEl = container.querySelector('.leading-relaxed.select-text');
    let description = '';
    if (descEl) {
        const SKIP_TAGS = new Set(['PRE', 'CODE', 'STYLE', 'SCRIPT']);
        const parts = [];
        const walk = (node) => {
            if (node.nodeType === 3) {
                const t = node.textContent || '';
                if (t.trim()) parts.push(t.trim());
            } else if (node.nodeType === 1 && !SKIP_TAGS.has(node.tagName)) {
                for (const child of node.childNodes) walk(child);
            }
        };
        walk(descEl);
        description = parts.join(' ').slice(0, 500);
    }

    return { openText, proceedText, planTitle, planSummary, description };
})()`
```

### 3.6. `EXTRACT_PLAN_CONTENT_SCRIPT` ([`src/services/planningDetector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/planningDetector.ts#L103-L164))

```go
const ExtractPlanContentScript = `(() => {
    // Simple HTML-to-Markdown converter for plan content
    const htmlToMd = (el) => {
        const parts = [];
        const process = (node) => {
            if (node.nodeType === 3) {
                parts.push(node.textContent || '');
                return;
            }
            if (node.nodeType !== 1) return;
            const tag = node.tagName;
            if (tag === 'H1') { parts.push('\n# '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'H2') { parts.push('\n## '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'H3') { parts.push('\n### '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'H4') { parts.push('\n#### '); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'STRONG' || tag === 'B') { parts.push('**'); node.childNodes.forEach(process); parts.push('**'); return; }
            if (tag === 'EM' || tag === 'I') { parts.push('*'); node.childNodes.forEach(process); parts.push('*'); return; }
            if (tag === 'PRE') {
                const code = node.querySelector('code');
                const text = code ? (code.textContent || '') : (node.textContent || '');
                parts.push('\n` + "```" + `\n' + text + '\n` + "```" + `\n');
                return;
            }
            if (tag === 'CODE') { parts.push('` + "`" + `' + (node.textContent || '') + '` + "`" + `'); return; }
            if (tag === 'A') {
                const href = node.getAttribute('href') || '';
                parts.push('['); node.childNodes.forEach(process); parts.push('](' + href + ')');
                return;
            }
            if (tag === 'LI') { parts.push('\n- '); node.childNodes.forEach(process); return; }
            if (tag === 'BR') { parts.push('\n'); return; }
            if (tag === 'P') { parts.push('\n\n'); node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'UL' || tag === 'OL') { node.childNodes.forEach(process); parts.push('\n'); return; }
            if (tag === 'STYLE' || tag === 'SCRIPT') return;
            node.childNodes.forEach(process);
        };
        process(el);
        return parts.join('').replace(/\n{3,}/g, '\n\n').trim();
    };

    // Primary selector: plan content container
    const contentContainer = document.querySelector(
        'div.relative.pl-4.pr-4.py-1, div.relative.pl-4.pr-4'
    );
    if (contentContainer) {
        const textEl = contentContainer.querySelector('.leading-relaxed.select-text');
        if (textEl) {
            return htmlToMd(textEl);
        }
    }

    // Fallback: any leading-relaxed.select-text with significant content
    const allLeading = Array.from(document.querySelectorAll('.leading-relaxed.select-text'));
    for (const el of allLeading) {
        const md = htmlToMd(el);
        if (md.length > 100) {
            return md;
        }
    }

    return null;
})()`
```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo de `ApprovalDetector` (`internal/bridge/approval_detector.go`)

#### A. Inicialização (`NewApprovalDetector`)
1. Recebe `ApprovalDetectorOptions`.
2. Se `options.PollInterval <= 0`, define `pollInterval = 1500 * time.Millisecond` (`DefaultApprovalPollInterval`).
3. Se `options.Logger == nil`, define `logger = slog.Default()`.
4. Se `options.SleepFunc == nil`, define `sleepFunc` padrão que aguarda `d` respeitando `ctx.Done()`.

#### B. Ciclo de Polling (`PollOnce(ctx)`)
1. Obtém `contextID := d.cdpService.GetPrimaryContextId()`.
2. Monta `callParams`:
   ```go
   callParams := map[string]any{
       "expression":    DetectApprovalScript,
       "returnByValue": true,
       "awaitPromise":  false,
   }
   if contextID != nil {
       callParams["contextId"] = *contextID
   }
   ```
3. Executa `raw, err := d.cdpService.Call(ctx, "Runtime.evaluate", callParams)`.
4. **Tratamento de Erro CDP**:
   - Se `err != nil`:
     - Verifica se `err.Error()` contém `"WebSocket is not connected"` ou `"WebSocket disconnected"`.
     - Se contiver qualquer uma dessas substrings, **retorna silenciosamente** sem registrar log de erro.
     - Caso contrário, registra `d.logger.Error("[ApprovalDetector] Error during polling", "error", err)` e retorna (mantendo o loop de monitoramento vivo).
5. **Decodificação do Resultado**:
   - Decodifica `raw` em `runtimeEvaluateResponse`.
   - Se `resp.Result.Value` estiver vazio ou for `"null"`, considera `info = nil`.
   - Caso contrário, faz `json.Unmarshal(resp.Result.Value, &info)`.
6. **Caso `info != nil` (Botões Detectados)**:
   - Constrói a chave de deduplicação exata conforme [`src/services/approvalDetector.ts#L313`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/approvalDetector.ts#L313):
     `key := fmt.Sprintf("%s::%s", info.ApproveText, info.Description)`
   - Sob lock (`d.mu.Lock()`):
     - Compara `d.lastDetectedKey == nil || *d.lastDetectedKey != key`.
     - Se for uma **nova detecção**:
       - Atualiza `d.lastDetectedKey = &key`.
       - Armazena cópia de `info` em `d.lastDetectedInfo`.
       - Libera o lock e invoca `d.onApprovalRequired(*info)` (capturando eventuais panics/erros sem interromper o loop, equivalente ao `.catch(err => logger.error(...))` em TS).
     - Se a chave for igual à anterior (`*d.lastDetectedKey == key`), ignora a notificação duplicada.
7. **Caso `info == nil` (Botões Desapareceram ou Ausentes)**:
   - Sob lock (`d.mu.Lock()`):
     - `wasDetected := d.lastDetectedKey != nil`
     - `d.lastDetectedKey = nil`
     - `d.lastDetectedInfo = nil`
   - Libera o lock.
   - Se `wasDetected && d.onResolved != nil`, invoca `d.onResolved()`.

#### C. Cliques Remotos (`ApproveButton`, `DenyButton`, `AlwaysAllowButton`)
1. **`ApproveButton(ctx, buttonText ...string)`**:
   - Determina `text`:
     - Se `len(buttonText) > 0 && buttonText[0] != ""` $\rightarrow$ usa `buttonText[0]`.
     - Senão, se `last := d.GetLastDetectedInfo(); last != nil && last.ApproveText != ""` $\rightarrow$ usa `last.ApproveText`.
     - Senão $\rightarrow$ usa `"Allow"` (`DefaultApproveFallbackText`).
   - Chama `d.clickButton(ctx, text)`.
2. **`DenyButton(ctx, buttonText ...string)`**:
   - Determina `text`:
     - Se `len(buttonText) > 0 && buttonText[0] != ""` $\rightarrow$ usa `buttonText[0]`.
     - Senão, se `last := d.GetLastDetectedInfo(); last != nil && last.DenyText != ""` $\rightarrow$ usa `last.DenyText`.
     - Senão $\rightarrow$ usa `"Deny"` (`DefaultDenyFallbackText`).
   - Chama `d.clickButton(ctx, text)`.
3. **`AlwaysAllowButton(ctx)`**:
   - Constrói a lista `directCandidates`:
     - Se `last := d.GetLastDetectedInfo(); last != nil && strings.TrimSpace(last.AlwaysAllowText) != ""` $\rightarrow$ adiciona `last.AlwaysAllowText` no início da slice.
     - Concatena `DefaultAlwaysAllowCandidates`: `["Allow This Conversation", "Allow This Chat", "この会話を許可", "Always Allow", "常に許可"]`.
   - **Fase 1 (Tentativa Direta)**:
     - Itera sobre `candidate` em `directCandidates`:
       - Se `d.clickButton(ctx, candidate)` retornar `true`, retorna `(true, nil)` imediatamente.
   - **Fase 2 (Expansão do Dropdown de `Allow Once`)**:
     - Avalia `ExpandAlwaysAllowMenuScript` via `d.runEvaluateScript(ctx, ExpandAlwaysAllowMenuScript)`.
     - Faz unmarshal do retorno em `clickScriptResult`. Se houver erro ou `!expanded.OK`, retorna `(false, nil)`.
   - **Fase 3 (Polling de Clique Pós-Expansão)**:
     - Executa até `5` iterações (`for i := 0; i < 5; i++`):
       - Para cada `candidate` em `directCandidates`:
         - Se `d.clickButton(ctx, candidate)` retornar `true`, retorna `(true, nil)`.
       - Aguarda `120 * time.Millisecond` (`d.sleepFunc(ctx, 120*time.Millisecond)`).
     - Se todas as 5 iterações falharem, retorna `(false, nil)`.
4. **`clickButton(ctx, buttonText string) bool`**:
   - Gera o script com `BuildClickScript(buttonText)`.
   - Executa `val, err := d.runEvaluateScript(ctx, script)`.
   - Se `err != nil`, loga `d.logger.Error("[ApprovalDetector] Error while clicking button", "error", err)` e retorna `false`.
   - Decodifica `val` em `clickScriptResult` e retorna `res.OK`.

---

### 4.2. Algoritmo de `PlanningDetector` (`internal/bridge/planning_detector.go`)

#### A. Inicialização (`NewPlanningDetector`)
1. Recebe `PlanningDetectorOptions`.
2. Se `options.PollInterval <= 0`, define `pollInterval = 2000 * time.Millisecond` (`DefaultPlanningPollInterval`).
3. Se `options.NowFunc == nil`, define `nowFunc = time.Now`.
4. Se `options.Logger == nil`, define `logger = slog.Default()`.

#### B. Ciclo de Polling (`PollOnce(ctx)`)
1. Obtém `contextID := d.cdpService.GetPrimaryContextId()`.
2. Monta `callParams` com `expression: DetectPlanningScript`, `returnByValue: true`, `awaitPromise: false` (e `contextId: *contextID` somente quando `contextID != nil`).
3. Executa `raw, err := d.cdpService.Call(ctx, "Runtime.evaluate", callParams)`.
4. **Tratamento de Erro CDP**:
   - Se `err != nil`:
     - Se `err.Error()` contiver `"WebSocket is not connected"` ou `"WebSocket disconnected"`, retorna silenciosamente sem logar erro.
     - Caso contrário, loga `d.logger.Error("[PlanningDetector] Error during polling", "error", err)` e retorna.
5. **Caso `info != nil` (Botões `Open` e `Proceed` Detectados)**:
   - Constrói a chave de deduplicação baseada exclusivamente no par de rótulos dos botões ([`src/services/planningDetector.ts#L295`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/planningDetector.ts#L295)):
     `key := fmt.Sprintf("%s::%s", info.OpenText, info.ProceedText)`
   - Obtém o instante atual: `now := d.nowFunc()`.
   - Sob lock (`d.mu.Lock()`):
     - Calcula `withinCooldown := !d.lastNotifiedAt.IsZero() && now.Sub(d.lastNotifiedAt) < PlanningCooldown` (`5000ms`).
     - Se `(d.lastDetectedKey == nil || *d.lastDetectedKey != key) && !withinCooldown`:
       - `d.lastDetectedKey = &key`
       - `infoCopy := *info; d.lastDetectedInfo = &infoCopy`
       - `d.lastNotifiedAt = now`
       - Libera o lock e invoca `d.onPlanningRequired(*info)`.
     - Senão, se `d.lastDetectedKey != nil && *d.lastDetectedKey == key`:
       - Atualiza silenciosamente `infoCopy := *info; d.lastDetectedInfo = &infoCopy` e libera o lock.
     - Senão (ex: chave diferente ou chave recém-resetada mas `withinCooldown == true`):
       - Libera o lock e suprime a notificação.
6. **Caso `info == nil` (Botões Desapareceram ou Ausentes)**:
   - Sob lock (`d.mu.Lock()`):
     - `wasDetected := d.lastDetectedKey != nil`
     - `d.lastDetectedKey = nil`
     - `d.lastDetectedInfo = nil`
     - **Atenção**: `d.lastNotifiedAt` **NÃO** é zerado aqui! Ele permanece com o timestamp da última notificação para garantir que, se o container piscar no DOM (`detect -> null -> detect` em menos de 5s), o cooldown de 5s suprima a re-emissão (conforme `tests/services/planningDetector.test.ts#L156-L181`). `d.lastNotifiedAt` só é zerado em `Start()`.
   - Libera o lock.
   - Se `wasDetected && d.onResolved != nil`, invoca `d.onResolved()`.

#### C. Cliques Remotos e Extração do Plano (`ClickOpenButton`, `ClickProceedButton`, `ExtractPlanContent`)
1. **`ClickOpenButton(ctx, buttonText ...string)`**:
   - Usa `buttonText[0]` se fornecido e não-vazio; senão `d.GetLastDetectedInfo().OpenText` se disponível; senão `"Open"`.
   - Chama `d.clickButton(ctx, text)` (que usa `BuildClickScript(text)`).
2. **`ClickProceedButton(ctx, buttonText ...string)`**:
   - Usa `buttonText[0]` se fornecido e não-vazio; senão `d.GetLastDetectedInfo().ProceedText` se disponível; senão `"Proceed"`.
   - Chama `d.clickButton(ctx, text)` (que usa `BuildClickScript(text)`).
3. **`ExtractPlanContent(ctx)`**:
   - Chama `raw, err := d.runEvaluateScript(ctx, ExtractPlanContentScript)`.
   - Se `err != nil`, loga `d.logger.Error("[PlanningDetector] Error extracting plan content", "error", err)` e retorna `(nil, nil)`.
   - Se `raw` for vazio, `"null"` ou não for uma string JSON válida, retorna `(nil, nil)`.
   - Faz unmarshal para `var content string` e retorna `(&content, nil)`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

Em TypeScript (single-threaded event loop), o agendamento usa `setTimeout` recursivo (`schedulePoll`) onde o próximo timer só é agendado após o término de `await this.poll()`. Em Go, cada detector deve replicar essa semântica exata de **polling sequencial não-sobreposto** usando uma goroutine dedicada controlada por `context.Context`:

1. **`Start()`**:
   - Adquire `d.mu.Lock()`.
   - Se `d.isRunning` já for `true`, libera o lock e retorna imediatamente (**idempotente**).
   - Define `d.isRunning = true`, `d.lastDetectedKey = nil`, `d.lastDetectedInfo = nil` (e `d.lastNotifiedAt = time.Time{}` no `PlanningDetector`).
   - Cria `ctx, cancel := context.WithCancel(context.Background())`, armazena `d.cancelPoll = cancel` e inicializa `done := make(chan struct{})`, `d.pollDone = done`.
   - Libera o lock e inicia a goroutine de loop:
     ```go
     go func() {
         defer close(done)
         timer := time.NewTimer(d.pollInterval)
         defer timer.Stop()
         for {
             select {
             case <-ctx.Done():
                 return
             case <-timer.C:
                 if !d.IsActive() {
                     return
                 }
                 d.PollOnce(ctx)
                 if !d.IsActive() {
                     return
                 }
                 timer.Reset(d.pollInterval)
             }
         }
     }()
     ```
   - **Nota sobre o primeiro tick**: exatamente como `setTimeout(..., this.pollIntervalMs)` em TypeScript, o primeiro `PollOnce` ocorre **após** decorrer `1 * pollInterval`, e cada ciclo subsequente aguarda `pollInterval` após a conclusão de `PollOnce`.

2. **`Stop()`**:
   - Adquire `d.mu.Lock()`.
   - Se `!d.isRunning`, libera o lock e retorna `nil`.
   - Define `d.isRunning = false`, captura `cancel := d.cancelPoll` e `done := d.pollDone`, limpa `d.cancelPoll = nil`.
   - Libera o lock.
   - Chama `if cancel != nil { cancel() }` e aguarda `if done != nil { <-done }` para garantir que nenhuma execução concorrente de `PollOnce` continue após o retorno de `Stop()`.

3. **Thread-Safety (`sync.RWMutex`)**:
   - Métodos de leitura (`IsActive()`, `GetLastDetectedInfo()`) usam `d.mu.RLock()` / `d.mu.RUnlock()` e retornam uma cópia segura de `ApprovalInfo` / `PlanningInfo` (evitando data races com o loop de polling).
   - Callbacks (`onApprovalRequired`, `onPlanningRequired`, `onResolved`) são sempre chamados **fora** da seção crítica do mutex para evitar deadlocks caso o callback invoque métodos do próprio detector (como `ApproveButton()`, `AlwaysAllowButton()` ou `GetLastDetectedInfo()`).

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

### 6.1. Testes de `ApprovalDetector` (`internal/bridge/approval_detector_test.go`)
Mapeamento 1:1 de todos os 16 casos de teste de [`tests/services/approvalDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/approvalDetector.test.ts#L20-L480):

| # | Teste Original em `tests/services/approvalDetector.test.ts` | Função de Teste em Go (`approval_detector_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `calls the onApprovalRequired callback when an approval button is detected` (L51) | `TestApprovalDetector_CallsOnApprovalRequiredWhenDetected` | Mock CDP retorna `ApprovalInfo{ApproveText: "許可", DenyText: "拒否", Description: "ファイルへの書き込みを許可しますか？"}`. Após 1 ciclo de poll, `onApprovalRequired` é chamado exatamente 1 vez com os campos esperados. |
| 2 | `does not call the callback when no approval button exists` (L81) | `TestApprovalDetector_DoesNotCallCallbackWhenNull` | Mock CDP retorna `{"result": {"value": null}}`. Após 1 ciclo de poll, `onApprovalRequired` não é chamado (`0` chamadas). |
| 3 | `does not call the callback multiple times when the same approval button is detected consecutively` (L100) | `TestApprovalDetector_DeduplicatesConsecutiveDetections` | Mock CDP retorna o mesmo `ApprovalInfo` (`Description: "重複テスト: run command"`) por 3 ciclos consecutivos. `onApprovalRequired` é chamado apenas 1 vez. |
| 4 | `executes a click script via CDP when approveButton() is called` (L127) | `TestApprovalDetector_ApproveButtonClicksViaCDP` | Chama `detector.ApproveButton(ctx, "Allow")` com `contextId = 42`. Verifica retorno `true` e chamada a `Runtime.evaluate` contendo `"Allow"`, `returnByValue: true` e `contextId: 42`. |
| 5 | `executes a deny click script via CDP when denyButton() is called` (L153) | `TestApprovalDetector_DenyButtonClicksViaCDP` | Chama `detector.DenyButton(ctx, "Deny")` com `contextId = 42`. Verifica retorno `true` e chamada a `Runtime.evaluate` contendo `"Deny"`, `returnByValue: true` e `contextId: 42`. |
| 6 | `alwaysAllowButton() can directly click Allow This Conversation` (L178) | `TestApprovalDetector_AlwaysAllowButtonDirectClick` | Mock CDP retorna `{ok: true}` na primeira chamada. `detector.AlwaysAllowButton(ctx)` retorna `true` e a expressão avaliada contém `"Allow This Conversation"`, `returnByValue: true` e `contextId: 42`. |
| 7 | `alwaysAllowButton() can click the conversation allow button after expanding the Allow Once dropdown` (L202) | `TestApprovalDetector_AlwaysAllowButtonExpandsDropdownThenClicks` | Mock CDP retorna `{ok: false}` para cliques diretos até receber a expressão que contém `"ALLOW_ONCE_PATTERNS"`, quando passa `expanded = true` e retorna `{ok: true, reason: "toggle-button"}`, e na tentativa subsequente com `"Allow This Conversation"` retorna `{ok: true}`. Verifica que `AlwaysAllowButton(ctx)` retorna `true` e que ambas as expressões foram avaliadas. |
| 8 | `stops polling and no longer calls the callback after stop()` (L239) | `TestApprovalDetector_StopHaltsPolling` | Detecta no 1º ciclo (`1` chamada), executa `detector.Stop()`, avança o tempo/ciclos e verifica que `onApprovalRequired` permanece com `1` chamada e `IsActive() == false`. |
| 9 | `continues monitoring even when a CDP error occurs` (L267) | `TestApprovalDetector_ContinuesOnCDPError` | 1ª chamada CDP retorna erro `"CDP error"`; 2ª chamada retorna `ApprovalInfo{ApproveText: "許可", Description: "エラー後リカバリ"}`. Verifica que no 2º ciclo `onApprovalRequired` é chamado normalmente. |
| 10 | `getLastDetectedInfo() returns the detected ApprovalInfo` (L293) | `TestApprovalDetector_GetLastDetectedInfoReturnsDetected` | Antes do `Start()`, `GetLastDetectedInfo()` é `nil`. Após 1 ciclo retornando `ApproveText: "Accept"`, `DenyText: "Decline"`, `Description: "テストアクション"`, `GetLastDetectedInfo()` retorna struct preenchida com esses exatos valores. |
| 11 | `getLastDetectedInfo() returns null when the button disappears` (L326) | `TestApprovalDetector_GetLastDetectedInfoResetsOnDisappear` | 1º ciclo retorna `ApprovalInfo` (`GetLastDetectedInfo() != nil`); 2º ciclo retorna `null` (`GetLastDetectedInfo() == nil`). |
| 12 | `approveButton() without arguments uses the detected approveText` (L350) | `TestApprovalDetector_ApproveButtonUsesDetectedText` | 1º ciclo detecta `ApproveText: "承認する"`. Chamada `detector.ApproveButton(ctx)` sem argumentos avalia script contendo `"承認する"` e retorna `true`. |
| 13 | `denyButton() without arguments uses the detected denyText` (L384) | `TestApprovalDetector_DenyButtonUsesDetectedText` | 1º ciclo detecta `DenyText: "キャンセル"`. Chamada `detector.DenyButton(ctx)` sem argumentos avalia script contendo `"キャンセル"` e retorna `true`. |
| 14 | `calls without the contextId parameter when contextId is null` (L414) | `TestApprovalDetector_OmitsContextIdWhenNil` | Mock `GetPrimaryContextId()` retorna `nil`. Verifica que o mapa `params` passado a `Runtime.evaluate` **não** contém a chave `"contextId"`. |
| 15 | `calls onResolved when buttons disappear after detection` (L436) | `TestApprovalDetector_CallsOnResolvedWhenButtonsDisappear` | 1º ciclo detecta `ApprovalInfo` (`onResolved` não chamado); 2º ciclo retorna `null` (`onResolved` chamado exatamente 1 vez). |
| 16 | `does not call onResolved when buttons were never detected` (L462) | `TestApprovalDetector_DoesNotCallOnResolvedWithoutPriorDetection` | Dois ciclos consecutivos retornando `null`. `onResolved` nunca é chamado (`0` chamadas). |

---

### 6.2. Testes de `PlanningDetector` (`internal/bridge/planning_detector_test.go`)
Mapeamento 1:1 de todos os 22 casos de teste de [`tests/services/planningDetector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/planningDetector.test.ts#L19-L572):

| # | Teste Original em `tests/services/planningDetector.test.ts` | Função de Teste em Go (`planning_detector_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `calls the onPlanningRequired callback when planning buttons are detected` (L52) | `TestPlanningDetector_CallsOnPlanningRequiredWhenDetected` | Mock CDP retorna `PlanningInfo{OpenText: "Open", ProceedText: "Proceed", PlanTitle: "Implementation Plan", ...}`. Após 1 ciclo, `onPlanningRequired` é chamado 1 vez com os campos esperados. |
| 2 | `does not call the callback when no planning buttons exist` (L82) | `TestPlanningDetector_DoesNotCallCallbackWhenNull` | Mock CDP retorna `null`. Após 1 ciclo, `onPlanningRequired` não é chamado. |
| 3 | `does not call the callback multiple times when the same planning buttons are detected` (L101) | `TestPlanningDetector_DeduplicatesConsecutiveDetections` | Mesmo `PlanningInfo` retornado em 3 ciclos seguidos. `onPlanningRequired` é chamado apenas 1 vez. |
| 4 | `treats detections with different planTitle but same button texts as duplicate` (L128) | `TestPlanningDetector_DeduplicatesByButtonTextPair` | 1º ciclo retorna `PlanTitle: "Plan A"`, 2º ciclo retorna `PlanTitle: "Plan B"`, ambos com `OpenText: "Open"` e `ProceedText: "Proceed"`. `onPlanningRequired` é chamado apenas 1 vez e `GetLastDetectedInfo().PlanTitle` é atualizado silenciosamente para `"Plan B"`. |
| 5 | `suppresses re-detection within 5s cooldown even after key reset` (L156) | `TestPlanningDetector_SuppressesRedetectionWithin5sCooldown` | 1º ciclo detecta (`t = 500ms`, 1ª notificação) $\rightarrow$ 2º ciclo retorna `null` (`t = 1000ms`, reseta chave) $\rightarrow$ 3º ciclo detecta novamente (`t = 1500ms`, dentro do cooldown de `5000ms`). `onPlanningRequired` permanece com apenas `1` chamada. |
| 6 | `executes a click script via CDP when clickOpenButton() is called` (L186) | `TestPlanningDetector_ClickOpenButtonViaCDP` | `detector.ClickOpenButton(ctx, "Open")` retorna `true` e envia `Runtime.evaluate` com `"Open"`, `returnByValue: true` e `contextId: 42`. |
| 7 | `executes a click script via CDP when clickProceedButton() is called` (L213) | `TestPlanningDetector_ClickProceedButtonViaCDP` | `detector.ClickProceedButton(ctx, "Proceed")` retorna `true` e envia `Runtime.evaluate` com `"Proceed"`, `returnByValue: true` e `contextId: 42`. |
| 8 | `extractPlanContent() returns the plan text from the DOM` (L240) | `TestPlanningDetector_ExtractPlanContentReturnsMarkdown` | Mock CDP retorna `"# Implementation Plan\n\n## Step 1\nDo something\n\n## Step 2\nDo something else"`. `ExtractPlanContent(ctx)` retorna ponteiro para essa string exata. |
| 9 | `extractPlanContent() returns null when no plan content is found` (L260) | `TestPlanningDetector_ExtractPlanContentReturnsNilWhenNull` | Mock CDP retorna `null`. `ExtractPlanContent(ctx)` retorna `nil`. |
| 10 | `stops polling and no longer calls the callback after stop()` (L279) | `TestPlanningDetector_StopHaltsPolling` | Após 1ª detecção, chama `detector.Stop()`. Ciclos seguintes não disparam `onPlanningRequired`. |
| 11 | `continues monitoring even when a CDP error occurs` (L307) | `TestPlanningDetector_ContinuesOnCDPError` | 1º ciclo falha com erro `"CDP error"`; 2º ciclo retorna `PlanningInfo{PlanTitle: "Recovery Plan"}`. `onPlanningRequired` é chamado no 2º ciclo. |
| 12 | `getLastDetectedInfo() returns the detected PlanningInfo` (L333) | `TestPlanningDetector_GetLastDetectedInfoReturnsDetected` | Retorna `nil` antes da detecção e retorna `PlanTitle: "Auth Feature"`, `PlanSummary: "Add OAuth2"` após o 1º ciclo. |
| 13 | `getLastDetectedInfo() returns null when planning buttons disappear` (L364) | `TestPlanningDetector_GetLastDetectedInfoResetsOnDisappear` | 1º ciclo detecta (`!= nil`); 2º ciclo retorna `null` (`== nil`). |
| 14 | `clickOpenButton() without arguments uses the detected openText` (L388) | `TestPlanningDetector_ClickOpenButtonUsesDetectedText` | 1º ciclo detecta `OpenText: "Open Plan"`. `detector.ClickOpenButton(ctx)` sem argumentos avalia script contendo `"Open Plan"`. |
| 15 | `clickProceedButton() without arguments uses the detected proceedText` (L418) | `TestPlanningDetector_ClickProceedButtonUsesDetectedText` | 1º ciclo detecta `ProceedText: "Start Implementation"`. `detector.ClickProceedButton(ctx)` sem argumentos avalia script contendo `"Start Implementation"`. |
| 16 | `calls without the contextId parameter when contextId is null` (L448) | `TestPlanningDetector_OmitsContextIdWhenNil` | Quando `GetPrimaryContextId()` retorna `nil`, `params` de `Runtime.evaluate` não contém a chave `"contextId"`. |
| 17 | `isActive() returns true while running and false after stop` (L470) | `TestPlanningDetector_IsActiveLifecycle` | Verifica `IsActive() == false` antes de `Start()`, `true` após `Start()` e `false` após `Stop()`. |
| 18 | `DETECT_PLANNING_SCRIPT filters code/style from description` (L489) | `TestPlanningDetector_DetectScriptContainsSkipTags` | Verifica que a constante `DetectPlanningScript` contém `SKIP_TAGS` com `'PRE'`, `'CODE'`, `'STYLE'`, `'SCRIPT'` e `.leading-relaxed.select-text`. |
| 19 | `EXTRACT_PLAN_CONTENT_SCRIPT uses HTML-to-Markdown conversion` (L500) | `TestPlanningDetector_ExtractScriptContainsHtmlToMd` | Verifica que a constante `ExtractPlanContentScript` contém o conversor `htmlToMd` e os seletores `div.relative.pl-4.pr-4.py-1, div.relative.pl-4.pr-4`. |
| 20 | `extractPlanContent() returns null on CDP error` (L510) | `TestPlanningDetector_ExtractPlanContentReturnsNilOnCDPError` | Mock CDP retorna erro `"CDP connection lost"`. `ExtractPlanContent(ctx)` captura o erro, loga e retorna `nil`. |
| 21 | `calls onResolved when planning buttons disappear after detection` (L531) | `TestPlanningDetector_CallsOnResolvedWhenButtonsDisappear` | 1º ciclo detecta (`onResolved` não chamado); 2º ciclo retorna `null` (`onResolved` chamado 1 vez). |
| 22 | `does not call onResolved when buttons were never detected` (L554) | `TestPlanningDetector_DoesNotCallOnResolvedWithoutPriorDetection` | Dois ciclos retornando `null`. `onResolved` nunca é chamado. |
