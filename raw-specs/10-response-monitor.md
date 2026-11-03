# 10 — Monitor de Respostas via CDP (`ResponseMonitor`) e Seletores DOM (`RESPONSE_SELECTORS`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de forma 100% autocontida e exaustiva o monitor de respostas da IA (`ResponseMonitor`) e o catálogo completo de seletores/scripts DOM (`RESPONSE_SELECTORS`) avaliados via Chrome DevTools Protocol (`Runtime.evaluate`), incluindo detecção de botão de parada, detecção de diálogo de planejamento (`Open`/`Proceed`), detecção de erro de cota, captura de baseline, deduplicação de logs de processo/thinking, injeção de `MutationObserver` com debounce e modos de operação ativo (`Start`) e passivo (`StartPassive`).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo TypeScript Origem | Linhas | Arquivo Go Destino | Pacote Go | Responsabilidade Principal |
| :--- | :---: | :--- | :---: | :--- |
| [`src/services/responseMonitor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/responseMonitor.ts) | 1206 | `internal/monitor/response_monitor.go` | `monitor` | Definição de `RESPONSE_SELECTORS` (11 scripts JS/IIFE), tipo `ResponsePhase`, struct `ResponseMonitor` e máquina de estados de monitoramento híbrida (polling CDP de `2000ms` + eventos push `Runtime.bindingCalled` via `MutationObserver`). |

### 1.2. Mapeamento de Arquivos de Teste Origem $\rightarrow$ Destino Go

| Arquivo de Teste TypeScript Origem | Linhas | Arquivo de Teste Go Destino |
| :--- | :---: | :--- |
| [`tests/services/responseMonitor.lean.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.lean.test.ts) | 523 | `internal/monitor/response_monitor_lean_test.go` |
| [`tests/services/responseMonitor.selectors.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.selectors.test.ts) | 258 | `internal/monitor/response_monitor_selectors_test.go` |
| [`tests/services/responseMonitor.stopButtonSelector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.stopButtonSelector.test.ts) | 329 | `internal/monitor/response_monitor_stop_button_test.go` |
| [`tests/services/responseMonitor.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.test.ts) | 1251 | `internal/monitor/response_monitor_test.go` |

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Tipos, Interfaces e Structs (`internal/monitor/response_monitor.go`)

```go
package monitor

import (
	"context"
	"sync"
	"time"
)

// ResponsePhase representa as fases do ciclo de vida da geração de resposta.
type ResponsePhase string

const (
	PhaseWaiting      ResponsePhase = "waiting"
	PhaseThinking     ResponsePhase = "thinking"
	PhaseGenerating   ResponsePhase = "generating"
	PhaseComplete     ResponsePhase = "complete"
	PhaseTimeout      ResponsePhase = "timeout"
	PhaseQuotaReached ResponsePhase = "quotaReached"
)

// ExtractionMode define o modo de extração do DOM ("structured" | "legacy").
type ExtractionMode string

const (
	ExtractionModeStructured ExtractionMode = "structured"
	ExtractionModeLegacy     ExtractionMode = "legacy"
)

// ExtractionSource indica se o texto final veio da extração estruturada (HTML) ou legada (plain text).
type ExtractionSource string

const (
	SourceStructured ExtractionSource = "structured"
	SourceLegacy     ExtractionSource = "legacy"
)

// CompleteMeta contém metadados entregues no callback OnComplete.
type CompleteMeta struct {
	Source ExtractionSource `json:"source"`
}

// ClickStopButtonResult representa o retorno de ClickStopButton().
type ClickStopButtonResult struct {
	Ok     bool   `json:"ok"`
	Method string `json:"method,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CombinedPollPayload mapeia o objeto retornado por RESPONSE_SELECTORS.COMBINED_POLL.
type CombinedPollPayload struct {
	IsGenerating   bool     `json:"isGenerating"`
	QuotaError     bool     `json:"quotaError"`
	PlanningActive bool     `json:"planningActive"`
	ResponseText   *string  `json:"responseText"`
	ProcessLogs    []string `json:"processLogs"`
}

// CdpClient define a interface mínima do CdpService utilizada pelo ResponseMonitor,
// permitindo injeção de mocks determinísticos nos testes unitários.
type CdpClient interface {
	Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)
	GetPrimaryContextID() *int
	On(event string, listener func(params map[string]any))
	Off(event string, listener func(params map[string]any))
}

// ResponseSelectorsCatalog agrupa todos os 11 scripts JS/IIFE de RESPONSE_SELECTORS.
type ResponseSelectorsCatalog struct {
	ResponseText       string
	StopButton         string
	PlanningActive     string
	ClickStopButton    string
	DumpAllTexts       string
	ProcessLogs        string
	CombinedPoll       string
	QuotaError         string
	ResponseStructured string
	DomDiagnostic      string
	DomObserver        string
}

// ResponseSelectors é a instância global equivalente a export const RESPONSE_SELECTORS.
var ResponseSelectors = ResponseSelectorsCatalog{
	// Inicializado com os scripts literais da Seção 3.
}

// ResponseMonitorOptions configura uma instância de ResponseMonitor.
type ResponseMonitorOptions struct {
	CdpService           CdpClient
	PollIntervalMs       int                                             // default: 2000
	MaxDurationMs        int                                             // default: 300000 (se 0 e não especificado, usa 300000; permita desativar se < 0)
	StopGoneConfirmCount int                                             // default: 3
	ExtractionMode       ExtractionMode                                  // default: os.Getenv("EXTRACTION_MODE") == "legacy" ? "legacy" : "structured"
	OnProgress           func(text string)                               // opcional
	OnComplete           func(finalText string, meta *CompleteMeta)      // opcional
	OnTimeout            func(lastText string)                           // opcional
	OnPhaseChange        func(phase ResponsePhase, text *string)         // opcional
	OnProcessLog         func(text string)                               // opcional
	OnThinkingLog        func(text string)                               // opcional
}

// ResponseMonitor implementa a máquina de estados de monitoramento de respostas via CDP.
type ResponseMonitor struct {
	mu                   sync.Mutex
	pollMu               sync.Mutex
	cdpService           CdpClient
	pollIntervalMs       time.Duration
	maxDurationMs        time.Duration
	stopGoneConfirmCount int
	extractionMode       ExtractionMode
	onProgress           func(text string)
	onComplete           func(finalText string, meta *CompleteMeta)
	onTimeout            func(lastText string)
	onPhaseChange        func(phase ResponsePhase, text *string)
	onProcessLog         func(text string)
	onThinkingLog        func(text string)

	pollTimer               *time.Timer
	timeoutTimer            *time.Timer
	domChangedDebounceTimer *time.Timer
	isRunning               bool
	lastText                *string
	baselineText            *string
	generationStarted       bool
	currentPhase            ResponsePhase
	stopGoneCount           int
	quotaDetected           bool
	seenProcessLogKeys      map[string]struct{}
	seenThinkingLogKeys     map[string]struct{}
	structuredDiagLogged    bool
	lastExtractionSource    *ExtractionSource
	consecutiveWsErrors     int
	bindingListener         func(params map[string]any)
}

// NewResponseMonitor cria uma nova instância de ResponseMonitor e registra o listener em Runtime.bindingCalled.
func NewResponseMonitor(options ResponseMonitorOptions) *ResponseMonitor

// Start inicia o monitoramento em modo ativo (generationStarted = false, currentPhase = "waiting").
func (m *ResponseMonitor) Start(ctx context.Context) error

// StartPassive inicia o monitoramento em modo passivo (generationStarted = true, currentPhase = "generating").
func (m *ResponseMonitor) StartPassive(ctx context.Context) error

// Stop interrompe o monitoramento, cancela todos os timers e remove o listener de Runtime.bindingCalled.
func (m *ResponseMonitor) Stop() error

// GetPhase retorna a fase atual do monitor.
func (m *ResponseMonitor) GetPhase() ResponsePhase

// GetQuotaDetected informa se erro de cota foi detectado durante a geração com texto parcial.
func (m *ResponseMonitor) GetQuotaDetected() bool

// IsActive informa se o monitor está em execução (isRunning).
func (m *ResponseMonitor) IsActive() bool

// GetLastText retorna o último texto extraído (ou nil se nenhum texto novo foi capturado).
func (m *ResponseMonitor) GetLastText() *string

// GetLastExtractionSource retorna a última fonte de extração ("structured" | "legacy" | nil).
func (m *ResponseMonitor) GetLastExtractionSource() *ExtractionSource

// ClickStopButton clica no botão Stop no DOM via CDP e, se o monitor estiver rodando, chama Stop().
func (m *ResponseMonitor) ClickStopButton(ctx context.Context) ClickStopButtonResult

// Métodos privados internos:
func (m *ResponseMonitor) initMonitoring(ctx context.Context, passive bool) error
func (m *ResponseMonitor) handleDomChangedEvent()
func (m *ResponseMonitor) setPhase(phase ResponsePhase, text *string)
func (m *ResponseMonitor) schedulePoll()
func (m *ResponseMonitor) buildEvaluateParams(expression string) map[string]any
func (m *ResponseMonitor) emitNewProcessLogs(entries []string)
func (m *ResponseMonitor) emitNewThinkingLogs(entries []string)
func (m *ResponseMonitor) poll(ctx context.Context)
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

Todos os 11 seletores de `RESPONSE_SELECTORS` em [`src/services/responseMonitor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/responseMonitor.ts#L10-L636) **DEVEM** ser preservados 100% literalmente conforme abaixo:

### 3.1. `RESPONSE_SELECTORS.RESPONSE_TEXT`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];

        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };

        const looksLikeFeedbackFooter = (text) => {
            const normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
            if (!normalized) return false;
            return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
        };

        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };

        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };

        const looksLikeQuotaPopup = (text) => {
            var lower = (text || '').trim().toLowerCase();
            // Inline error: "Error You have exhausted your quota on this model."
            if (lower.includes('exhausted your quota') || lower.includes('exhausted quota')) return true;
            // Popup: quota keyword + dismiss/upgrade button text
            if (!lower.includes('model quota reached') && !lower.includes('quota exceeded') && !lower.includes('rate limit')) return false;
            return lower.includes('dismiss') || lower.includes('upgrade');
        };

        const combinedSelector = selectors.map((s) => s.sel).join(', ');
        const seen = new Set();

        for (const scope of scopes) {
            const nodes = scope.querySelectorAll(combinedSelector);
            for (let i = nodes.length - 1; i >= 0; i--) {
                const node = nodes[i];
                if (!node || seen.has(node)) continue;
                seen.add(node);
                if (isInsideExcludedContainer(node)) continue;
                const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                if (!text || text.length < 2) continue;
                if (looksLikeActivityLog(text)) continue;
                if (looksLikeFeedbackFooter(text)) continue;
                if (looksLikeToolOutput(text)) continue;
                if (looksLikeQuotaPopup(text)) continue;
                // Prefer recency first: return the newest acceptable node.
                return text;
            }
        }

        return null;
    })()
```

### 3.2. `RESPONSE_SELECTORS.STOP_BUTTON`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        for (const scope of scopes) {
            const el = scope.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
            if (el) return { isGenerating: true };
        }

        const normalize = (value) => (value || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const STOP_PATTERNS = [
            /^stop$/,
            /^stop generating$/,
            /^stop response$/,
            /^停止$/,
            /^生成を停止$/,
            /^応答を停止$/,
        ];
        const isStopLabel = (value) => {
            const normalized = normalize(value);
            if (!normalized) return false;
            return STOP_PATTERNS.some((re) => re.test(normalized));
        };
        for (const scope of scopes) {
            const buttons = scope.querySelectorAll('button, [role="button"]');
            for (let i = 0; i < buttons.length; i++) {
                const btn = buttons[i];
                const labels = [
                    btn.textContent || '',
                    btn.getAttribute('aria-label') || '',
                    btn.getAttribute('title') || '',
                ];
                if (labels.some(isStopLabel)) {
                    return { isGenerating: true };
                }
            }
        }

        return { isGenerating: false };
    })()
```

### 3.3. `RESPONSE_SELECTORS.PLANNING_ACTIVE`

```javascript
(() => {
        var container = document.querySelector('.notify-user-container');
        if (!container) return false;
        var buttons = Array.from(container.querySelectorAll('button')).filter(function(btn) { return btn.offsetParent !== null; });
        var hasOpen = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'open'; });
        var hasProceed = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'proceed'; });
        return hasOpen && hasProceed;
    })()
```

### 3.4. `RESPONSE_SELECTORS.CLICK_STOP_BUTTON`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        for (const scope of scopes) {
            const el = scope.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
            if (el && typeof el.click === 'function') {
                el.click();
                return { ok: true, method: 'tooltip-id' };
            }
        }

        const normalize = (value) => (value || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const STOP_PATTERNS = [
            /^stop$/,
            /^stop generating$/,
            /^stop response$/,
            /^停止$/,
            /^生成を停止$/,
            /^応答を停止$/,
        ];
        const isStopLabel = (value) => {
            const normalized = normalize(value);
            if (!normalized) return false;
            return STOP_PATTERNS.some((re) => re.test(normalized));
        };
        for (const scope of scopes) {
            const buttons = scope.querySelectorAll('button, [role="button"]');
            for (let i = 0; i < buttons.length; i++) {
                const btn = buttons[i];
                const labels = [
                    btn.textContent || '',
                    btn.getAttribute('aria-label') || '',
                    btn.getAttribute('title') || '',
                ];
                if (labels.some(isStopLabel) && typeof btn.click === 'function') {
                    btn.click();
                    return { ok: true, method: 'text-fallback' };
                }
            }
        }

        return { ok: false, error: 'Stop button not found' };
    })()
```

### 3.5. `RESPONSE_SELECTORS.DUMP_ALL_TEXTS`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];

        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };
        const looksLikeFeedbackFooter = (text) => {
            const normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
            if (!normalized) return false;
            return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
        };
        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };
        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };

        const results = [];
        const seen = new Set();

        for (const scope of scopes) {
            for (const { sel, score } of selectors) {
                const nodes = scope.querySelectorAll(sel);
                for (let i = nodes.length - 1; i >= 0; i--) {
                    const node = nodes[i];
                    if (!node || seen.has(node)) continue;
                    seen.add(node);
                    const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                    let skip = null;
                    if (!text || text.length < 2) skip = 'too-short';
                    else if (isInsideExcludedContainer(node)) skip = 'excluded-container';
                    else if (looksLikeActivityLog(text)) skip = 'activity-log';
                    else if (looksLikeFeedbackFooter(text)) skip = 'feedback-footer';
                    else if (looksLikeToolOutput(text)) skip = 'tool-output';
                    else {
                        var qlower = (text || '').trim().toLowerCase();
                        if (qlower.includes('exhausted your quota') || qlower.includes('exhausted quota')) skip = 'quota-popup';
                        else if ((qlower.includes('model quota reached') || qlower.includes('quota exceeded') || qlower.includes('rate limit'))
                            && (qlower.includes('dismiss') || qlower.includes('upgrade'))) skip = 'quota-popup';
                    }
                    const classes = (node.className || '').toString().slice(0, 80);
                    results.push({
                        sel,
                        score,
                        skip,
                        len: text.length,
                        classes,
                        preview: text.slice(0, 120),
                    });
                }
            }
        }
        return results;
    })()
```

### 3.6. `RESPONSE_SELECTORS.PROCESS_LOGS`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];

        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };

        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };

        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };

        const results = [];
        const seen = new Set();

        for (const scope of scopes) {
            for (const { sel } of selectors) {
                const nodes = scope.querySelectorAll(sel);
                for (let i = 0; i < nodes.length; i++) {
                    const node = nodes[i];
                    if (!node || seen.has(node)) continue;
                    seen.add(node);
                    if (isInsideExcludedContainer(node)) continue;
                    const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                    if (!text || text.length < 4) continue;
                    if (looksLikeActivityLog(text) || looksLikeToolOutput(text)) {
                        results.push(text.slice(0, 300));
                    }
                }
            }
        }

        return results;
    })()
```

### 3.7. `RESPONSE_SELECTORS.COMBINED_POLL`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scopes = [panel, document].filter(Boolean);

        // --- Stop button ---
        let isGenerating = false;
        for (const scope of scopes) {
            const el = scope.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
            if (el) { isGenerating = true; break; }
        }
        if (!isGenerating) {
            const normalize = (value) => (value || '').toLowerCase().replace(/\s+/g, ' ').trim();
            const STOP_PATTERNS = [/^stop$/, /^stop generating$/, /^stop response$/, /^停止$/, /^生成を停止$/, /^応答を停止$/];
            const isStopLabel = (value) => { const n = normalize(value); return n ? STOP_PATTERNS.some((re) => re.test(n)) : false; };
            outer: for (const scope of scopes) {
                const buttons = scope.querySelectorAll('button, [role="button"]');
                for (let i = 0; i < buttons.length; i++) {
                    const btn = buttons[i];
                    if ([btn.textContent || '', btn.getAttribute('aria-label') || '', btn.getAttribute('title') || ''].some(isStopLabel)) {
                        isGenerating = true; break outer;
                    }
                }
            }
        }

        // --- Quota error ---
        let quotaError = false;
        const scope = panel || document;
        const QUOTA_KEYWORDS = ['model quota reached', 'rate limit', 'quota exceeded', 'exhausted your quota', 'exhausted quota'];
        const isInsideResponse = (node) =>
            node.closest('.rendered-markdown, .prose, pre, code, [data-message-author-role="assistant"], [data-message-role="assistant"], [class*="message-content"]');
        const headings = scope.querySelectorAll('h3 span, h3');
        for (const el of headings) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) { quotaError = true; break; }
        }
        if (!quotaError) {
            const inlineSpans = scope.querySelectorAll('span');
            for (const el of inlineSpans) {
                if (isInsideResponse(el)) continue;
                const text = (el.textContent || '').trim().toLowerCase();
                if (text.includes('exhausted your quota') || text.includes('exhausted quota')) { quotaError = true; break; }
            }
        }
        if (!quotaError) {
            const errorSelectors = ['[role="alert"]','[class*="error"]','[class*="warning"]','[class*="toast"]','[class*="banner"]','[class*="notification"]','[class*="alert"]','[class*="quota"]','[class*="rate-limit"]'];
            const errorElements = scope.querySelectorAll(errorSelectors.join(', '));
            for (const el of errorElements) {
                if (isInsideResponse(el)) continue;
                const text = (el.textContent || '').trim().toLowerCase();
                if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) { quotaError = true; break; }
            }
        }

        // --- Planning active ---
        let planningActive = false;
        const container = document.querySelector('.notify-user-container');
        if (container) {
            const buttons = Array.from(container.querySelectorAll('button')).filter(function(btn) { return btn.offsetParent !== null; });
            const hasOpen = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'open'; });
            const hasProceed = buttons.some(function(btn) { return (btn.textContent || '').toLowerCase().trim() === 'proceed'; });
            planningActive = hasOpen && hasProceed;
        }

        // --- Legacy text extraction ---
        const selectors = [
            { sel: '.rendered-markdown', score: 10 },
            { sel: '.leading-relaxed.select-text', score: 9 },
            { sel: '.flex.flex-col.gap-y-3', score: 8 },
            { sel: '[data-message-author-role="assistant"]', score: 7 },
            { sel: '[data-message-role="assistant"]', score: 6 },
            { sel: '[class*="assistant-message"]', score: 5 },
            { sel: '[class*="message-content"]', score: 4 },
            { sel: '[class*="markdown-body"]', score: 3 },
            { sel: '.prose', score: 2 },
        ];
        const looksLikeActivityLog = (text) => {
            const normalized = (text || '').trim().toLowerCase();
            if (!normalized) return false;
            const stripped = normalized.replace(/^[^a-z]+/i, '');
            const activityPattern = /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i;
            if (activityPattern.test(stripped) && normalized.length <= 220) return true;
            if (/^initiating\s/i.test(stripped) && normalized.length <= 500) return true;
            if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
            return false;
        };
        const looksLikeFeedbackFooter = (text) => {
            const normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
            if (!normalized) return false;
            return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
        };
        const isInsideExcludedContainer = (node) => {
            if (node.closest('details')) return true;
            if (node.closest('[class*="feedback"], footer')) return true;
            if (node.closest('.notify-user-container')) return true;
            if (node.closest('[role="dialog"]')) return true;
            return false;
        };
        const looksLikeToolOutput = (text) => {
            const first = (text || '').trim().split('\n')[0] || '';
            if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
            if (/^full output written to\b/i.test(first)) return true;
            if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
            var lower = (text || '').trim().toLowerCase();
            if (/^title:\s/.test(lower) && /\surl:\s/.test(lower) && /\ssnippet:\s/.test(lower)) return true;
            if (/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log|ruby|go|rust|java|c|cpp|csharp|php|swift|kotlin)$/i.test(first)) return true;
            return false;
        };
        const looksLikeQuotaPopup = (text) => {
            var lower = (text || '').trim().toLowerCase();
            if (lower.includes('exhausted your quota') || lower.includes('exhausted quota')) return true;
            if (!lower.includes('model quota reached') && !lower.includes('quota exceeded') && !lower.includes('rate limit')) return false;
            return lower.includes('dismiss') || lower.includes('upgrade');
        };
        const combinedSelector = selectors.map((s) => s.sel).join(', ');
        const seen = new Set();
        let responseText = null;
        for (const s of scopes) {
            const nodes = s.querySelectorAll(combinedSelector);
            for (let i = nodes.length - 1; i >= 0; i--) {
                const node = nodes[i];
                if (!node || seen.has(node)) continue;
                seen.add(node);
                if (isInsideExcludedContainer(node)) continue;
                const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                if (!text || text.length < 2) continue;
                if (looksLikeActivityLog(text)) continue;
                if (looksLikeFeedbackFooter(text)) continue;
                if (looksLikeToolOutput(text)) continue;
                if (looksLikeQuotaPopup(text)) continue;
                responseText = text;
                break;
            }
            if (responseText !== null) break;
        }

        // --- Process logs ---
        const logSeen = new Set();
        const processLogs = [];
        for (const s of scopes) {
            for (const { sel } of selectors) {
                const nodes = s.querySelectorAll(sel);
                for (let i = 0; i < nodes.length; i++) {
                    const node = nodes[i];
                    if (!node || logSeen.has(node)) continue;
                    logSeen.add(node);
                    if (isInsideExcludedContainer(node)) continue;
                    const text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
                    if (!text || text.length < 4) continue;
                    if (looksLikeActivityLog(text) || looksLikeToolOutput(text)) {
                        processLogs.push(text.slice(0, 300));
                    }
                }
            }
        }

        return { isGenerating, quotaError, planningActive, responseText, processLogs };
    })()
```

### 3.8. `RESPONSE_SELECTORS.QUOTA_ERROR`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel');
        const scope = panel || document;
        const QUOTA_KEYWORDS = ['model quota reached', 'rate limit', 'quota exceeded', 'exhausted your quota', 'exhausted quota'];
        const isInsideResponse = (node) =>
            node.closest('.rendered-markdown, .prose, pre, code, [data-message-author-role="assistant"], [data-message-role="assistant"], [class*="message-content"]');

        // Primary: text-based detection via h3 span (Tailwind-only popup)
        const headings = scope.querySelectorAll('h3 span, h3');
        for (const el of headings) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) return true;
        }

        // Inline error: "Error You have exhausted your quota on this model."
        // Appears in process log area as a span inside flex containers
        const inlineSpans = scope.querySelectorAll('span');
        for (const el of inlineSpans) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (text.includes('exhausted your quota') || text.includes('exhausted quota')) return true;
        }

        // Fallback: semantic class-based detection
        const errorSelectors = [
            '[role="alert"]',
            '[class*="error"]',
            '[class*="warning"]',
            '[class*="toast"]',
            '[class*="banner"]',
            '[class*="notification"]',
            '[class*="alert"]',
            '[class*="quota"]',
            '[class*="rate-limit"]',
        ];
        const errorElements = scope.querySelectorAll(errorSelectors.join(', '));
        for (const el of errorElements) {
            if (isInsideResponse(el)) continue;
            const text = (el.textContent || '').trim().toLowerCase();
            if (QUOTA_KEYWORDS.some(kw => text.includes(kw))) return true;
        }
        return false;
    })()
```

### 3.9. `RESPONSE_SELECTORS.RESPONSE_STRUCTURED`

Inicializado diretamente via `ExtractAssistantSegmentsPayloadScript()` (cujo código integral consta na Especificação `09-assistant-dom-extractor-and-selectors.md`, Seção 3.1).

### 3.10. `RESPONSE_SELECTORS.DOM_DIAGNOSTIC`

```javascript
(() => {
        var panel = document.querySelector('.antigravity-agent-side-panel');
        var scope = panel || document;
        var diag = { detailsCount: 0, detailsDump: [], activityNodes: [], allTextNodes: [] };

        // 1. Dump all <details> elements
        var details = scope.querySelectorAll('details');
        diag.detailsCount = details.length;
        for (var i = 0; i < Math.min(details.length, 5); i++) {
            diag.detailsDump.push({
                outerHTML: details[i].outerHTML.slice(0, 500),
                summaryText: (details[i].querySelector('summary') || {}).textContent || '(no summary)',
                childCount: details[i].children.length
            });
        }

        // 2. Find all text nodes that look like activity
        var selectors = '.rendered-markdown, .leading-relaxed.select-text, .flex.flex-col.gap-y-3, [data-message-author-role="assistant"], [data-message-role="assistant"], [class*="assistant-message"], [class*="message-content"], [class*="markdown-body"], .prose';
        var nodes = scope.querySelectorAll(selectors);
        for (var j = 0; j < nodes.length; j++) {
            var text = (nodes[j].innerText || nodes[j].textContent || '').trim();
            if (!text || text.length < 2) continue;
            diag.allTextNodes.push({
                tag: nodes[j].tagName,
                className: (nodes[j].className || '').toString().slice(0, 100),
                text: text.slice(0, 200),
                insideDetails: !!nodes[j].closest('details'),
                length: text.length
            });
        }

        // 3. Broader scan: any element with activity-like text
        var allEls = scope.querySelectorAll('*');
        for (var k = 0; k < allEls.length; k++) {
            var el = allEls[k];
            if (el.children.length > 2) continue; // only leaf-ish nodes
            var t = (el.textContent || '').trim();
            if (!t || t.length < 5 || t.length > 300) continue;
            var lower = t.toLowerCase();
            if (/^(?:analy[sz]|read|writ|run|search|think|process|execut|debug|test)/i.test(lower) || /\//.test(t)) {
                diag.activityNodes.push({
                    tag: el.tagName,
                    className: (el.className || '').toString().slice(0, 100),
                    text: t.slice(0, 200),
                    parentTag: el.parentElement ? el.parentElement.tagName : null,
                    parentClass: el.parentElement ? (el.parentElement.className || '').toString().slice(0, 100) : null,
                    insideDetails: !!el.closest('details')
                });
            }
        }
        return diag;
    })()
```

### 3.11. `RESPONSE_SELECTORS.DOM_OBSERVER`

```javascript
(() => {
        const panel = document.querySelector('.antigravity-agent-side-panel') || document.body;
        // Clean up previous observer if exists
        if (window._antigravity-telegram-remoteObserver) window._antigravity-telegram-remoteObserver.disconnect();
        
        let debounceTimer;
        const notify = () => {
            if (typeof window.onantigravity-telegram-remoteDOMChange === 'function') {
                window.onantigravity-telegram-remoteDOMChange('changed');
            }
        };

        window._antigravity-telegram-remoteObserver = new MutationObserver((mutations) => {
            clearTimeout(debounceTimer);
            // Throttle to 100ms to avoid flooding CDP during rapid streaming
            debounceTimer = setTimeout(notify, 100);
        });

        window._antigravity-telegram-remoteObserver.observe(panel, {
            childList: true,
            subtree: true,
            characterData: true
        });
        return { ok: true, observing: panel.className || 'body' };
    })()
```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Construção (`NewResponseMonitor`)

1. Aplica os valores padrão caso omitidos:
   - `pollIntervalMs`: `2000` ms.
   - `maxDurationMs`: `300000` ms (`5` minutos).
   - `stopGoneConfirmCount`: `3`.
   - `extractionMode`: se `options.ExtractionMode != ""` usa o valor informado; caso contrário, verifica `os.Getenv("EXTRACTION_MODE") == "legacy"` $\rightarrow$ `"legacy"`, senão `"structured"`.
2. Registra imediatamente no construtor um listener no evento `"Runtime.bindingCalled"` do `cdpService`:
   - Se `params["name"] == "onantigravity-telegram-remoteDOMChange"` e `m.isRunning == true`, aciona `m.handleDomChangedEvent()`.

---

### 4.2. Inicialização (`Start` vs `StartPassive` $\rightarrow$ `initMonitoring(passive)`)

1. Se `m.isRunning` já for `true`, retorna imediatamente (idempotente).
2. Inicializa o estado interno:
   - `m.isRunning = true`
   - `m.lastText = nil`
   - `m.baselineText = nil`
   - `m.generationStarted = passive` (`false` em `Start()`, `true` em `StartPassive()`)
   - `m.currentPhase = "waiting"` (se `!passive`) ou `"generating"` (se `passive`)
   - `m.stopGoneCount = 0`
   - `m.quotaDetected = false`
   - `m.seenProcessLogKeys = make(map[string]struct{})`
   - `m.seenThinkingLogKeys = make(map[string]struct{})`
   - `m.consecutiveWsErrors = 0`
3. Invoca `m.onPhaseChange(m.currentPhase, nil)` se configurado.
4. **Captura Paralela de Baseline**:
   - Dispara em paralelo (`errgroup` ou `sync.WaitGroup`):
     1. `Runtime.evaluate` com `RESPONSE_SELECTORS.RESPONSE_TEXT` (ignorando erros).
     2. `Runtime.evaluate` com `RESPONSE_SELECTORS.PROCESS_LOGS` (ignorando erros).
     3. Se `m.extractionMode == ExtractionModeStructured`: `Runtime.evaluate` com `RESPONSE_SELECTORS.RESPONSE_STRUCTURED` (ignorando erros).
   - Define `m.baselineText`: se o retorno de `RESPONSE_TEXT` for uma string cujo `.trim()` não seja vazio, armazena o ponteiro; senão `nil`.
   - Popula `m.seenProcessLogKeys`: para cada string retornada em `PROCESS_LOGS`, remove `\r`, aplica `TrimSpace`, ignora vazias e adiciona `s[:min(len(s), 200)]` ao conjunto.
   - Se `structuredBaseline` retornou um payload válido (`ClassifyAssistantSegments` com `diagnostics.source == "dom-structured"`):
     - Para cada linha em `activityLines`: normaliza (`replace("\r", "").trim()[:200]`) e adiciona a `m.seenProcessLogKeys`.
     - Para cada linha em `thinkingLines`: normaliza (`replace("\r", "").trim()[:200]`) e adiciona a `m.seenThinkingLogKeys`.
5. **Timer de Timeout Global (`maxDurationMs`)**:
   - Se `m.maxDurationMs > 0`, inicia `m.timeoutTimer`:
     - Ao disparar, verifica se `m.currentPhase == PhaseComplete || m.currentPhase == PhaseQuotaReached`; se sim, retorna sem fazer nada.
     - Obtém `lastText := ""` (ou `*m.lastText` se não-nil).
     - Chama `m.setPhase(PhaseTimeout, &lastText)`.
     - Chama `m.Stop()`.
     - Invoca `m.onTimeout(lastText)` protegendo contra erros/panics.
6. **Injeção do `MutationObserver`**:
   - Avalia `RESPONSE_SELECTORS.DOM_OBSERVER` via `Runtime.evaluate`. Se falhar, apenas registra `logger.Warn` e segue com polling puro.
7. Agenda o primeiro ciclo de polling chamando `m.schedulePoll()`.

---

### 4.3. Ciclo de Polling (`poll()`)

A cada disparo do timer (ou do debounce de evento DOM):

1. **Avaliação CDP conforme `extractionMode`**:
   - **Modo `'structured'`**:
     - Executa em paralelo 2 chamadas `Runtime.evaluate`:
       1. `RESPONSE_SELECTORS.COMBINED_POLL`
       2. `RESPONSE_SELECTORS.RESPONSE_STRUCTURED` (com `.catch(() => null)` — se falhar individualmente, não aborta o `poll`, permitindo fallback para o resultado do `COMBINED_POLL`).
     - Extrai `isGenerating`, `quotaDetected` (`combined.quotaError`) e `planningActive` de `COMBINED_POLL`.
     - Se `structuredResult` retornou valor válido e `ClassifyAssistantSegments(payload)` produziu `diagnostics.source == "dom-structured"`:
       - `currentText` recebe `strings.TrimSpace(classified.FinalOutputText)` (ou `nil` se vazio).
       - `m.lastExtractionSource = SourceStructured`.
       - `structuredHandledLogs = true`.
       - Se `len(classified.ActivityLines) > 0`, chama `m.emitNewProcessLogs(classified.ActivityLines)`.
       - Se `len(classified.ThinkingLines) > 0`, chama `m.emitNewThinkingLogs(classified.ThinkingLines)`.
     - **Fallback dentro do modo `'structured'`**:
       - Se `currentText == nil`, usa `combined.responseText` (se string não-vazia após trim) e define `m.lastExtractionSource = SourceLegacy`.
       - Se `!structuredHandledLogs` e `combined.processLogs` for um array, chama `m.emitNewProcessLogs(combined.processLogs)`.
   - **Modo `'legacy'`**:
     - Executa **exatamente 1 chamada CDP** (`Runtime.evaluate` com `RESPONSE_SELECTORS.COMBINED_POLL`).
     - Extrai `isGenerating`, `quotaDetected`, `planningActive`, `currentText` (de `combined.responseText`) e define `m.lastExtractionSource = SourceLegacy`.
     - Se `combined.processLogs` for um array, chama `m.emitNewProcessLogs(combined.processLogs)`.

2. **Reset de Erros de WebSocket**:
   - Como a chamada CDP teve sucesso, zera `m.consecutiveWsErrors = 0`.

3. **Detecção de Botão Stop (`isGenerating == true`)**:
   - Se `!m.generationStarted`:
     - Define `m.generationStarted = true`.
     - Chama `m.setPhase(PhaseThinking, nil)`.
   - Zera `m.stopGoneCount = 0`.

4. **Detecção de Erro de Cota (`quotaDetected == true`)**:
   - Verifica `hasText := m.lastText != nil && strings.TrimSpace(*m.lastText) != ""`.
   - Se `hasText` for `true`:
     - Define apenas a flag `m.quotaDetected = true` (não interrompe imediatamente; deixa o fluxo de conclusão normal entregar o texto parcial já gerado).
   - Se `hasText` for `false`:
     - Muda a fase para `m.setPhase(PhaseQuotaReached, ptr(""))`.
     - Chama `m.Stop()`.
     - Invoca `m.onComplete("", &CompleteMeta{Source: SourceLegacy})` e retorna imediatamente.

5. **Supressão de Baseline (`effectiveText`)**:
   - Se `currentText != nil && m.baselineText != nil && *currentText == *m.baselineText && m.lastText == nil`:
     - `effectiveText = nil` (suprime o texto pré-existente do turno anterior).
   - Caso contrário:
     - `effectiveText = currentText`.
   - **Regra Crítica**: a supressão de baseline **NÃO** faz `return` antecipado! A lógica de conclusão (passo 7) deve obrigatoriamente ser executada mesmo quando `effectiveText == nil`.

6. **Mudança de Texto (`textChanged`)**:
   - Se `effectiveText != nil && (m.lastText == nil || *effectiveText != *m.lastText)`:
     - Atualiza `m.lastText = effectiveText`.
     - Se `m.currentPhase == PhaseWaiting || m.currentPhase == PhaseThinking`:
       - Chama `m.setPhase(PhaseGenerating, effectiveText)`.
       - Se `!m.generationStarted`, define `m.generationStarted = true`.
     - Invoca `m.onProgress(*effectiveText)`.

7. **Verificação de Conclusão (`!isGenerating && m.generationStarted`)**:
   - Se `planningActive == true`:
     - Zera `m.stopGoneCount = 0` e registra `logger.Info("[ResponseMonitor] Planning dialog active — deferring completion")`.
   - Se `planningActive == false`:
     - Incrementa `m.stopGoneCount++`.
     - Se `m.stopGoneCount >= m.stopGoneConfirmCount && m.isRunning`:
       - `finalText := ""` (ou `*m.lastText` se não-nil).
       - Chama `m.setPhase(PhaseComplete, &finalText)`.
       - Chama `m.Stop()`.
       - Determina `source := SourceLegacy` (ou `*m.lastExtractionSource` se não-nil).
       - Invoca `m.onComplete(finalText, &CompleteMeta{Source: source})` e retorna.

8. **Tratamento de Erros de WebSocket no `catch` do `poll()`**:
   - Se o erro retornado por `cdpService.Call` contiver `"WebSocket is not connected"` ou `"WebSocket disconnected"`:
     - Incrementa `m.consecutiveWsErrors++`.
     - Se `m.consecutiveWsErrors >= 5`:
       - Registra erro, captura `lastText := ""` (ou `*m.lastText`), chama `m.Stop()` e invoca `m.onTimeout(lastText)`.
     - Retorna.
   - Para outros erros de CDP, apenas registra `logger.Error("[ResponseMonitor] poll error:", err)` e mantém o monitor vivo para o próximo ciclo agendado.

---

### 4.4. Deduplicação de Logs de Processo e Thinking

1. **`emitNewProcessLogs(entries []string)`**:
   - Para cada `line` em `entries`:
     - `normalized = strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))`. Se vazio, ignora.
     - `key = normalized[:min(len(normalized), 200)]`.
     - Se `key` já existir em `m.seenProcessLogKeys`, ignora.
     - Adiciona `key` a `m.seenProcessLogKeys`.
     - Invoca `m.onProcessLog(normalized[:min(len(normalized), 300)])`.
2. **`emitNewThinkingLogs(entries []string)`**:
   - Para cada `line` em `entries`:
     - `normalized = strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))`. Se vazio, ignora.
     - `key = normalized[:min(len(normalized), 200)]`.
     - Se `key` já existir em `m.seenThinkingLogKeys`, ignora.
     - Adiciona `key` a `m.seenThinkingLogKeys` **e também** a `m.seenProcessLogKeys` (para impedir contaminação cruzada entre thinking e process logs).
     - Invoca `m.onThinkingLog(normalized[:min(len(normalized), 2000)])`.

---

### 4.5. Eventos Push do DOM (`handleDomChangedEvent`)

1. Quando o binding `"onantigravity-telegram-remoteDOMChange"` é chamado pelo navegador enquanto `m.isRunning == true`:
   - Calcula `debounceMs`: `200ms` se `m.currentPhase == PhaseGenerating`, ou `500ms` nas demais fases.
   - Reinicia `m.domChangedDebounceTimer` para `debounceMs`.
   - Quando o timer de debounce dispara:
     - Se `!m.isRunning`, aborta.
     - Cancela `m.pollTimer` se estiver pendente.
     - Executa `m.poll(ctx)` imediatamente.
     - Se ainda estiver rodando (`m.isRunning`), reagenda o polling regular via `m.schedulePoll()`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Exclusão Mútua de `poll()` (`pollMu`) e Estado (`mu`)**:
   - Em Go, como timers (`time.AfterFunc`) disparam em goroutines próprias e `handleDomChangedEvent()` ou `Stop()` podem ocorrer concorrentemente, `poll()` deve ser serializado por `m.pollMu.Lock()` / `defer m.pollMu.Unlock()`, e as transições de estado (`isRunning`, `currentPhase`, `lastText`, timers) protegidas por `m.mu`.
   - Callbacks externos (`onProgress`, `onComplete`, `onTimeout`, `onPhaseChange`, `onProcessLog`, `onThinkingLog`) devem ser invocados fora do lock de estado (`m.mu`) ou com lock reentrante seguro, pois o callback `onComplete` / `onTimeout` ou o próprio fluxo interno chama `m.Stop()`.
2. **Desregistro Limpo (`Stop()`)**:
   - `Stop()` define `m.isRunning = false`, para `pollTimer`, `timeoutTimer` e `domChangedDebounceTimer`, e chama `m.cdpService.Off("Runtime.bindingCalled", m.bindingListener)`.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

### 6.1. `internal/monitor/response_monitor_lean_test.go` (Origem: [`tests/services/responseMonitor.lean.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.lean.test.ts))

Esta é a suíte principal que valida o contrato atual (Lean API) do `ResponseMonitor`:

| Função de Teste Go | Caso Original (`it`) | Comportamento e Asserção Exata |
| :--- | :--- | :--- |
| `TestLeanResponseMonitor_StartCapturesBaselineAndSetsWaiting` | `start() captures baseline, sets phase to waiting, starts polling` | `Start()` emite fase `'waiting'`, faz exatamente 3 chamadas CDP iniciais (`RESPONSE_TEXT`, `PROCESS_LOGS`, `DOM_OBSERVER`) e mantém `GetPhase() == "waiting"` |
| `TestLeanResponseMonitor_StopButtonSetsPhaseThinking` | `stop button appearing sets phase to thinking` | Quando `COMBINED_POLL` retorna `isGenerating: true` sem texto, transiciona para `'thinking'` |
| `TestLeanResponseMonitor_TextUpdateTriggersProgressAndGenerating` | `text update triggers onProgress and sets phase to generating` | Quando `COMBINED_POLL` retorna `isGenerating: true, responseText: "Hello"`, transiciona para `'generating'` e chama `OnProgress("Hello")` |
| `TestLeanResponseMonitor_StopGone3TimesTriggersComplete` | `stop button disappearing 3 consecutive times triggers onComplete` | Após `isGenerating: true` com `"response"`, 3 polls consecutivos com `isGenerating: false` disparam `OnComplete("response", ...)` apenas no 3º poll |
| `TestLeanResponseMonitor_StopReappearingResetsCounter` | `stop button reappearing resets gone counter` | `isGenerating: false` (2x) seguido de `isGenerating: true` (1x) e `isGenerating: false` (1x) zera o contador e não dispara `OnComplete` |
| `TestLeanResponseMonitor_TextChangeDoesNotResetStopGoneCounter` | `text change does NOT reset stop gone counter — completion still fires` | Mudança de texto de `"first"` para `"first updated"` durante os polls com `isGenerating: false` não reseta `stopGoneCount`; completa no 3º poll com `"first updated"` |
| `TestLeanResponseMonitor_ContinuousTextUpdatesAfterStopGone` | `continuous text updates after stop button disappears do NOT block completion` | Mesmo que o texto mude em todos os 3 polls após `isGenerating: false`, completa no 3º poll com `"token1 token2 token3 final"` |
| `TestLeanResponseMonitor_BaselineTextSuppressed` | `baseline text is suppressed (same text as before is not treated as new)` | Se o baseline capturou `"old response"` e o poll retorna `"old response"`, `OnProgress` não é chamado |
| `TestLeanResponseMonitor_BaselineSuppressionDoesNotBlockCompletion` | `baseline suppression does not block completion when stop button disappears` | Se a geração inicia (`isGenerating: true`) mas o texto continua igual ao baseline (`"old response"`), após 3 polls com `isGenerating: false` completa com `""` |
| `TestLeanResponseMonitor_TimeoutTriggersOnTimeout` | `timeout triggers onTimeout after maxDurationMs` | Ao atingir `MaxDurationMs: 10000`, chama `OnTimeout` |
| `TestLeanResponseMonitor_QuotaDetectionNoTextImmediateComplete` | `quota detection with no text triggers immediate complete with empty string` | `COMBINED_POLL` com `quotaError: true` e sem texto chama imediatamente `OnComplete("", ...)` e define fase `'quotaReached'` |
| `TestLeanResponseMonitor_ClickStopButtonSuccess` | `clickStopButton returns { ok: true, method: tooltip-id } on success` | `ClickStopButton()` retorna `{Ok: true, Method: "tooltip-id"}` e para o monitor |
| `TestLeanResponseMonitor_SubscribesToBindingCalled` | `subscribes to push notifications (Runtime.bindingCalled)` | Registra listener em `On("Runtime.bindingCalled", ...)` e remove em `Off("Runtime.bindingCalled", ...)` no `Stop()` |
| `TestLeanResponseMonitor_DefaultPollIntervalIs2000ms` | `default poll interval is 2000ms` | Sem especificar `PollIntervalMs`, após `1000ms` nenhum poll ocorre; após `2000ms` o 1º poll é executado |
| `TestLeanResponseMonitor_HasGetLastExtractionSource` | `has getLastExtractionSource but NOT getLastDomActivityLines` | Método `GetLastExtractionSource()` existe e retorna a fonte correta |
| `TestLeanResponseMonitor_SingleCdpCallInLegacyMode` | `poll makes exactly 1 CDP call (COMBINED_POLL) in legacy mode` | No modo `'legacy'`, cada ciclo de `poll()` faz exatamente 1 chamada `Runtime.evaluate` (`COMBINED_POLL`) |

---

### 6.2. `internal/monitor/response_monitor_selectors_test.go` (Origem: [`tests/services/responseMonitor.selectors.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.selectors.test.ts))

Verifica todas as 29 asserções estruturais sobre as strings de `ResponseSelectors`:

| Função de Teste Go | Casos Originais (`it`) | Asserções Exatas sobre `ResponseSelectors` |
| :--- | :--- | :--- |
| `TestResponseSelectors_StopButtonAndClickStopButton` | Testes 1 a 8c (10 casos) | `StopButton` e `ClickStopButton` contêm `"input-send-button-cancel-tooltip"`, não contêm `"svg"`, `"getBoundingClientRect"`, `"getComputedStyle"`, `"composer"`, `"heuristic"`, `"blob.includes"`, `"includes(w)"`, e `StopButton` contém `"停止"` |
| `TestResponseSelectors_ResponseTextAndProcessLogsAndDumpAllTexts` | Testes 12 a 15, 19b a 22, 25, 26 (11 casos) | `ResponseText` contém `"score"`, `"closest('details')"`, `"feedback"`, `"footer"`, `"looksliketooloutput"`, `".notify-user-container"`, `"lookslikequotapopup"`, `"dismiss"`, `"upgrade"`, `"exhausted your quota"`; `DumpAllTexts` e `ProcessLogs` contêm `".notify-user-container"`; `DumpAllTexts` contém `"quota-popup"` e `"exhausted your quota"` |
| `TestResponseSelectors_QuotaError` | Testes 16 a 19, 23, 24 (6 casos) | `QuotaError` contém `"h3 span"`, `"h3"`, `"model quota reached"`, `"rate limit"`, `"quota exceeded"`, `".rendered-markdown"`, `".prose"`, `'[role="alert"]'`, `'[class*="error"]'`, `"exhausted your quota"`, `"querySelectorAll('span')"` |

---

### 6.3. `internal/monitor/response_monitor_stop_button_test.go` (Origem: [`tests/services/responseMonitor.stopButtonSelector.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.stopButtonSelector.test.ts))

| Função de Teste Go | Casos Originais (`it`) | Comportamento e Asserção Exata |
| :--- | :--- | :--- |
| `TestStopButtonSelector_DetectsAndClicksFixedPositionAriaLabel` | `detects/clicks the stop button even when offsetParent is null for fixed-position elements` | Botão com `offsetParent: null` e `aria-label="Stop generating"` é detectado (`isGenerating: true`) e clicado (`ok: true, method: "text-fallback"`) |
| `TestStopButtonSelector_DetectsCancelTooltipId` | `detects the cancel button by data-tooltip-id as generating` | Botão com `data-tooltip-id="input-send-button-cancel-tooltip"` retorna `isGenerating: true` e no click retorna `ok: true, method: "tooltip-id"` |
| `TestStopButtonSelector_IgnoresNonStopButtons` | `does not use a generic red square icon button` / `arrow icon` / `microphone button` | Botões sem o tooltip-id e sem os textos de `STOP_PATTERNS` retornam `isGenerating: false` e `ok: false` |

---

### 6.4. `internal/monitor/response_monitor_test.go` (Origem: [`tests/services/responseMonitor.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/responseMonitor.test.ts))

> **Nota de Rastreabilidade**: O arquivo `tests/services/responseMonitor.test.ts` contém testes históricos da versão pré-Lean (que usava 4 chamadas CDP separadas por poll e parâmetros removidos como `textStabilityCompleteMs`, `noUpdateTimeoutMs`, `networkCompleteDelayMs`, conforme documentado em `responseMonitor.lean.test.ts` linhas 15-32), além de cenários fundamentais que continuam válidos na arquitetura atual e o bloco `describe('startPassive()')` (linhas 1196-1250). Em Go, `internal/monitor/response_monitor_test.go` deve cobrir todos os cenários comportamentais adaptados para o formato `COMBINED_POLL` + `RESPONSE_STRUCTURED`:

| Função de Teste Go | Caso Original (`it`) | Comportamento e Asserção Exata |
| :--- | :--- | :--- |
| `TestResponseMonitor_IncrementalProgressAndNoDuplicate` | Testes 1 e 2 (`incrementally extracts in-progress text`, `does not redundantly call onProgress`) | Mudanças `"Hello"` $\rightarrow$ `"Hello World"` acionam `OnProgress` 2 vezes; texto repetido `"Same text"` aciona `OnProgress` apenas 1 vez |
| `TestResponseMonitor_Stop stopsPollingAndCallbacks` | Testes 4, 5 e 6 (`stops polling after stop()`, `auto-stops on maxDurationMs`, `continues monitoring on CDP error`) | Após `Stop()`, nenhum poll adicional ocorre; erro transitório de CDP não interrompe os polls subsequentes; 5 erros consecutivos de `"WebSocket is not connected"` encerram o monitor e acionam `OnTimeout` |
| `TestResponseMonitor_PhaseTransitions` | Teste 7 (`calls onPhaseChange when phase changes`) | Sequência completa `'waiting'` $\rightarrow$ `'thinking'` $\rightarrow$ `'generating'` $\rightarrow$ `'complete'` |
| `TestResponseMonitor_StructuredPayloadBodyProcessAndThinking` | Testes 8b e 8c (`notifies onProgress with body only...`, `does not send duplicate notifications...`) | No modo `'structured'`, extrai `assistant-body` para `OnProgress`, `tool-call`/`tool-result` para `OnProcessLog`, `thinking`/`thinking-content` para `OnThinkingLog`, ignora `feedback` e deduplica chaves já vistas |
| `TestResponseMonitor_PlanningDialogDefersCompletion` | Verifica `planningActive: true` durante `!isGenerating && generationStarted` | Enquanto `planningActive == true`, `stopGoneCount` permanece `0` e `OnComplete` é adiado até o diálogo fechar |
| `TestResponseMonitor_AttachesPrimaryContextId` | Teste 19 (`attaches contextId to Runtime.evaluate when primary context exists`) | Quando `GetPrimaryContextID()` retorna `42`, todas as chamadas `Runtime.evaluate` incluem `"contextId": 42` nos parâmetros |
| `TestResponseMonitor_StartPassive` | `describe('startPassive()') > starts with generationStarted=true and detects text changes immediately` | `StartPassive()` inicia na fase `'generating'` (não `'waiting'`), detecta imediatamente `"Updated response text"` no 1º poll chamando `OnProgress` e completa assim que o botão Stop desaparece por `StopGoneConfirmCount` polls |
