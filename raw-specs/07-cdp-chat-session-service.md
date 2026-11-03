# Especificação Técnica 07 — Gerenciamento de Sessões de Chat via CDP (`ChatSessionService` / `internal/cdp/session.go`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação define de forma 100% autocontida e exaustiva o serviço `ChatSessionService`, responsável por iniciar uma nova conversa (`startNewChat`), ler o título e o estado da conversa ativa no cabeçalho do painel Cascade (`getCurrentSessionInfo`), listar até 20 conversas recentes do workspace atual abrindo e fazendo scrape do modal "Past Conversations" (`listAllSessions`), e ativar uma conversa existente por título tanto diretamente no painel lateral quanto via busca/seleção no histórico (`activateSessionByTitle`).

### 1.1. Mapeamento de Arquivos Originais (TypeScript) $\rightarrow$ Destino (Go)

| Arquivo Original (TypeScript) | Linhas | Arquivo Destino (Go) | Responsabilidade Principal |
| :--- | :---: | :--- | :--- |
| [`src/services/chatSessionService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/chatSessionService.ts) | 820 | `internal/cdp/session.go` | Structs `SessionListItem` e `ChatSessionInfo`, os 5 scripts estáticos de inspeção/scrape DOM, os 2 builders dinâmicos de scripts de ativação (`buildActivateChatByTitleScript`, `buildActivateViaPastConversationsScript`) e a struct `ChatSessionService` com todos os métodos públicos e privados. |

### 1.2. Mapeamento de Arquivos de Teste Originais $\rightarrow$ Suítes de Teste em Go

| Arquivo de Teste Original (Jest) | Linhas | Arquivo de Teste Destino (Go) | Escopo de Verificação |
| :--- | :---: | :--- | :--- |
| [`tests/services/chatSessionService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/chatSessionService.test.ts) | 391 | `internal/cdp/session_test.go` | Testes unitários cobrindo todos os fluxos de `StartNewChat` (6 testes), `GetCurrentSessionInfo` (4 testes), `ActivateSessionByTitle` (6 testes) e `ListAllSessions` (5 testes). |

### 1.3. Documentação Arquitetural Relacionada e Decisão de Engenharia (ADR-005)
- [`docs/03-protocolo-cdp-e-seletores-dom.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/03-protocolo-cdp-e-seletores-dom.md) (Seção 6 — Gerenciamento de Sessões de Chat)
- [`docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md) (**ADR-005 — Exposição do Seletor Interativo de Sessões em `/chat` e `/join`**):
  - No código TypeScript original, `ChatSessionService.listAllSessions(cdp)` e `ChatSessionService.activateSessionByTitle(cdp, title)` estão 100% implementados e testados, assim como `buildSessionPickerUI` (`src/ui/sessionPickerUi.ts`) e `JoinCommandHandler` (`src/commands/joinCommandHandler.ts`), porém `/chat` em `src/bot/index.ts` exibia apenas o resumo do SQLite sem anexar o seletor de sessões do CDP.
  - Conforme **ADR-005**, no port para Go o `ChatSessionService` especificado neste documento é consumido integralmente tanto por `/new` (`StartNewChat` + `GetCurrentSessionInfo`) e pelo roteamento de tópicos (`ActivateSessionByTitle` + `GetCurrentSessionInfo`) quanto pelos comandos `/chat` e `/join` (`ListAllSessions` + callback `session_select:<title>` $\rightarrow$ `ActivateSessionByTitle`).

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pacote `internal/cdp` — `session.go`

```go
package cdp

import (
	"encoding/json"
	"time"
)

// SessionCDPClient define o subconjunto de métodos de Client utilizados por ChatSessionService.
// Recebido como argumento em cada método (compatível com ConnectionPool e mocks de teste).
type SessionCDPClient interface {
	Call(method string, params any) (json.RawMessage, error)
	GetContexts() []Context
	WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool
}

// SessionListItem espelha SessionListItem (src/services/chatSessionService.ts:4-9).
type SessionListItem struct {
	// Title é o título da conversa extraído do painel Past Conversations.
	Title string `json:"title"`
	// IsActive indica se a linha possui a classe focusBackground (sessão atualmente ativa).
	IsActive bool `json:"isActive"`
}

// ChatSessionInfo espelha ChatSessionInfo (src/services/chatSessionService.ts:12-17).
type ChatSessionInfo struct {
	// Title é o título atual do cabeçalho (ou "(Untitled)" / "(Failed to retrieve)").
	Title string `json:"title"`
	// HasActiveChat é true quando title != "" e title != "Agent".
	HasActiveChat bool `json:"hasActiveChat"`
}

// SessionActionResult representa o retorno { ok: boolean; error?: string } de startNewChat e activateSessionByTitle.
type SessionActionResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ActivateSessionOptions espelha o parâmetro opcional options de activateSessionByTitle (linhas 665-668).
type ActivateSessionOptions struct {
	MaxWait       *time.Duration
	RetryInterval *time.Duration
}

// newChatButtonState representa o retorno interno de getNewChatButtonState (linhas 801-804).
type newChatButtonState struct {
	Found   bool   `json:"found"`
	Enabled bool   `json:"enabled"`
	Cursor  string `json:"cursor,omitempty"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
}

// coordsState representa o retorno { found: boolean, x: number, y: number } dos scripts de busca de botão.
type coordsState struct {
	Found bool `json:"found"`
	X     int  `json:"x"`
	Y     int  `json:"y"`
}

// Constantes da classe ChatSessionService (src/services/chatSessionService.ts:442-444)
const (
	ActivateSessionMaxWait       = 30000 * time.Millisecond
	ActivateSessionRetryInterval = 800 * time.Millisecond
	ListSessionsTarget           = 20
)

// ChatSessionService espelha ChatSessionService (src/services/chatSessionService.ts:441-820).
type ChatSessionService struct {
	// Hooks de sleep opcionais para acelerar testes unitários quando necessário,
	// usando time.Sleep por padrão em produção.
	SleepFunc func(d time.Duration)
}

func NewChatSessionService() *ChatSessionService

// Métodos públicos:
func (s *ChatSessionService) ListAllSessions(cdpService SessionCDPClient) []SessionListItem
func (s *ChatSessionService) StartNewChat(cdpService SessionCDPClient) SessionActionResult
func (s *ChatSessionService) GetCurrentSessionInfo(cdpService SessionCDPClient) ChatSessionInfo
func (s *ChatSessionService) ActivateSessionByTitle(cdpService SessionCDPClient, title string, opts ...ActivateSessionOptions) SessionActionResult

// Métodos privados auxiliares:
func (s *ChatSessionService) evaluateOnAnyContext(cdpService SessionCDPClient, expression string, awaitPromise bool) (json.RawMessage, bool)
func (s *ChatSessionService) cdpMouseClick(cdpService SessionCDPClient, x, y int) error
func (s *ChatSessionService) tryActivateByDirectSidePanel(cdpService SessionCDPClient, title string) SessionActionResult
func (s *ChatSessionService) tryActivateByPastConversations(cdpService SessionCDPClient, title string) SessionActionResult
func (s *ChatSessionService) tryActivateWithScript(cdpService SessionCDPClient, script string, awaitPromise bool) SessionActionResult
func (s *ChatSessionService) getNewChatButtonState(cdpService SessionCDPClient, contexts []Context) newChatButtonState

// Builders de scripts JS dinâmicos:
func BuildActivateChatByTitleScript(title string) string
func BuildActivateViaPastConversationsScript(title string) string
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

### 3.1. Tabela de Constantes e Temporizações (`chatSessionService.ts`)

| Constante / Parâmetro | Valor Exato | Local de Uso |
| :--- | :--- | :--- |
| `ACTIVATE_SESSION_MAX_WAIT_MS` | `30000` (30s) | Timeout padrão do loop de tentativas em `activateSessionByTitle`. |
| `ACTIVATE_SESSION_RETRY_INTERVAL_MS` | `800` (800ms) | Intervalo entre tentativas no loop de `activateSessionByTitle`. |
| `LIST_SESSIONS_TARGET` | `20` | Quantidade alvo máxima de sessões retornadas por `listAllSessions`. |
| `listAllSessions` wait após clicar em Past Conversations | `500ms` | Pausa para renderização do painel antes do primeiro scrape (linha 475). |
| `listAllSessions` wait após clicar em `"Show N more..."` | `500ms` | Pausa antes do segundo scrape (linha 490). |
| `startNewChat` `waitForCascadePanelReady` | `timeout = 10000ms`, `pollInterval = 500ms` | Aguarda criação de contexto caso `getContexts()` esteja vazio (linha 570). |
| `startNewChat` retries de busca do botão | `maxRetries = 5`, intervalo `1000ms` (1s) | Retries caso `btnState.found` seja `false` inicialmente (linhas 581–586). |
| `startNewChat` wait após clique no botão | `1500ms` (1.5s) | Aguarda transição da UI antes de verificar se `enabled` mudou para `false` (linha 612). |
| `activateSessionByTitle` wait pós-clique antes de verificar título | `500ms` | Pausa antes de chamar `getCurrentSessionInfo` (linhas 721 e 731). |
| `Escape` keyCode | `key: 'Escape'`, `code: 'Escape'`, `windowsVirtualKeyCode: 27`, `nativeVirtualKeyCode: 27` | Fecha o modal Past Conversations ao final de `listAllSessions` (linhas 501–508). |

### 3.2. Expressões Regulares dentro dos Scripts JS

1. **Fronteira de Outros Projetos (`SCRAPE_PAST_CONVERSATIONS_SCRIPT`, linha 113)**:
   - `/^Other\s+Conversations?$/i` — Identifica o cabeçalho `"Other Conversation"` / `"Other Conversations"` para excluir sessões cujo `getBoundingClientRect().top >= boundaryTop`.
2. **Filtro de Labels de Tempo Relativo (`SCRAPE_PAST_CONVERSATIONS_SCRIPT`, linha 131)**:
   - `/^\d+\s+(min|hr|hour|day|sec|week|month|year)s?\s+ago$/i` — Ignora spans de timestamp como `"1 hr ago"`, `"7 mins ago"`.
3. **Filtro de Labels de Ação/UI (`SCRAPE_PAST_CONVERSATIONS_SCRIPT`, linha 134)**:
   - `/^(show\s+\d+\s+more|new|past|history|settings|close|menu)\b/i` — Ignora textos de botões/controles internos.
4. **Detecção de Sessão Ativa (`SCRAPE_PAST_CONVERSATIONS_SCRIPT`, linha 141)**:
   - `/focusBackground/i` — Testa `row.className` para definir `isActive`.
5. **Botão `"Show N more..."` (`FIND_SHOW_MORE_BUTTON_SCRIPT`, linha 157)**:
   - `/^Show\s+\d+\s+more/i` — Localiza o link de expansão da lista de conversas.
6. **Normalização de Aspas e Espaços (`buildActivateViaPastConversationsScript`, linhas 226–229)**:
   - `/[\u2018\u2019\u201C\u201D'"`]/g` e `/[^a-z0-9\u3040-\u30ff\u4e00-\u9faf\s]/g` e `/\s+/g`.

---

### 3.3. Scripts JS/DOM Literais Avaliados via `Runtime.evaluate` (Cópia 100% Integral)

#### 3.3.1. `GET_NEW_CHAT_BUTTON_SCRIPT` (`src/services/chatSessionService.ts:20-32`)
```javascript
(() => {
    const btn = document.querySelector('[data-tooltip-id="new-conversation-tooltip"]');
    if (!btn) return { found: false };
    const cursor = window.getComputedStyle(btn).cursor;
    const rect = btn.getBoundingClientRect();
    return {
        found: true,
        enabled: cursor === 'pointer',
        cursor,
        x: Math.round(rect.x + rect.width / 2),
        y: Math.round(rect.y + rect.height / 2),
    };
})()
```

#### 3.3.2. `GET_CHAT_TITLE_SCRIPT` (`src/services/chatSessionService.ts:38-48`)
```javascript
(() => {
    const panel = document.querySelector('.antigravity-agent-side-panel');
    if (!panel) return { title: '', hasActiveChat: false };
    const header = panel.querySelector('div[class*="border-b"]');
    if (!header) return { title: '', hasActiveChat: false };
    const titleEl = header.querySelector('div[class*="text-ellipsis"]');
    const title = titleEl ? (titleEl.textContent || '').trim() : '';
    // "Agent" is the default empty chat title
    const hasActiveChat = title.length > 0 && title !== 'Agent';
    return { title: title || '(Untitled)', hasActiveChat };
})()
```

#### 3.3.3. `FIND_PAST_CONVERSATIONS_BUTTON_SCRIPT` (`src/services/chatSessionService.ts:57-87`)
```javascript
(() => {
    const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
    const getRect = (el) => {
        const rect = el.getBoundingClientRect();
        return { found: true, x: Math.round(rect.x + rect.width / 2), y: Math.round(rect.y + rect.height / 2) };
    };

    // Strategy 1 (primary): data-past-conversations-toggle attribute
    const toggle = document.querySelector('[data-past-conversations-toggle]');
    if (toggle && isVisible(toggle)) return getRect(toggle);

    // Strategy 2: data-tooltip-id containing "history"
    const tooltipEls = Array.from(document.querySelectorAll('[data-tooltip-id]'));
    for (const el of tooltipEls) {
        if (!isVisible(el)) continue;
        const tid = (el.getAttribute('data-tooltip-id') || '').toLowerCase();
        if (tid.includes('history') || tid.includes('past-conversations')) {
            return getRect(el);
        }
    }

    // Strategy 3: SVG with lucide-history class
    const icons = Array.from(document.querySelectorAll('svg.lucide-history, svg[class*="lucide-history"]'));
    for (const icon of icons) {
        const parent = icon.closest('a, button, [role="button"], div[class*="cursor-pointer"]');
        const target = parent instanceof HTMLElement && isVisible(parent) ? parent : icon;
        if (isVisible(target)) return getRect(target);
    }

    return { found: false, x: 0, y: 0 };
})()
```

#### 3.3.4. `SCRAPE_PAST_CONVERSATIONS_SCRIPT` (`src/services/chatSessionService.ts:95-145`)
```javascript
(() => {
    const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
    const normalize = (text) => (text || '').trim();

    const items = [];
    const seen = new Set();

    // Find the scrollable conversation list container
    const containers = Array.from(document.querySelectorAll('div[class*="overflow-auto"], div[class*="overflow-y-scroll"]'));
    const container = containers.find((c) => isVisible(c) && c.querySelectorAll('div[class*="cursor-pointer"]').length > 0) || document;

    // Detect the "Other Conversations" section boundary.
    // Sessions below this header belong to other projects and must be excluded.
    let boundaryTop = Infinity;
    const headerCandidates = container.querySelectorAll('div[class*="text-xs"][class*="opacity"]');
    for (const el of headerCandidates) {
        if (!isVisible(el)) continue;
        const t = normalize(el.textContent || '');
        if (/^Other\s+Conversations?$/i.test(t)) {
            boundaryTop = el.getBoundingClientRect().top;
            break;
        }
    }

    // Each session row is a div with cursor-pointer
    const rows = Array.from(container.querySelectorAll('div[class*="cursor-pointer"]'));
    for (const row of rows) {
        if (!isVisible(row)) continue;
        // Skip rows that are below the "Other Conversations" boundary
        if (row.getBoundingClientRect().top >= boundaryTop) continue;
        // Find the session title — nested span within the row
        const spans = Array.from(row.querySelectorAll('span.text-sm span, span.text-sm'));
        let title = '';
        for (const span of spans) {
            const t = normalize(span.textContent || '');
            // Skip timestamp labels like "1 hr ago", "7 mins ago"
            if (/^\d+\s+(min|hr|hour|day|sec|week|month|year)s?\s+ago$/i.test(t)) continue;
            // Skip very short or action-like labels
            if (t.length < 2 || t.length > 200) continue;
            if (/^(show\s+\d+\s+more|new|past|history|settings|close|menu)\b/i.test(t)) continue;
            title = t;
            break;
        }
        if (!title || seen.has(title)) continue;
        seen.add(title);
        // Detect if this is the active/current session (has focusBackground class)
        const isActive = /focusBackground/i.test(row.className || '');
        items.push({ title, isActive });
    }
    return { sessions: items };
})()
```

#### 3.3.5. `FIND_SHOW_MORE_BUTTON_SCRIPT` (`src/services/chatSessionService.ts:151-163`)
```javascript
(() => {
    const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
    const els = Array.from(document.querySelectorAll('div, span'));
    for (const el of els) {
        if (!isVisible(el)) continue;
        const text = (el.textContent || '').trim();
        if (/^Show\s+\d+\s+more/i.test(text)) {
            const rect = el.getBoundingClientRect();
            return { found: true, x: Math.round(rect.x + rect.width / 2), y: Math.round(rect.y + rect.height / 2) };
        }
    }
    return { found: false, x: 0, y: 0 };
})()
```

#### 3.3.6. `buildActivateChatByTitleScript(title)` (`src/services/chatSessionService.ts:169-212`)
Onde `${safeTitle}` é `JSON.stringify(title)`:
```javascript
(() => {
        const wantedRaw = ${safeTitle};
        const wanted = (wantedRaw || '').toLowerCase().replace(/\s+/g, ' ').trim();
        if (!wanted) return { ok: false, error: 'Empty target title' };

        const panel = document.querySelector('.antigravity-agent-side-panel') || document;
        const normalize = (text) => (text || '').toLowerCase().replace(/\s+/g, ' ').trim();
        const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
        const clickTarget = (el) => {
            const clickable = el.closest('button, [role="button"], a, li, [data-testid*="conversation"]') || el;
            if (!(clickable instanceof HTMLElement)) return false;
            clickable.click();
            return true;
        };

        const nodes = Array.from(panel.querySelectorAll('button, [role="button"], a, li, div, span'))
            .filter(isVisible);

        const exact = [];
        const includes = [];
        for (const node of nodes) {
            const text = normalize(node.textContent || '');
            if (!text) continue;
            if (text === wanted) {
                exact.push({ node, textLength: text.length });
            } else if (text.includes(wanted)) {
                includes.push({ node, textLength: text.length });
            }
        }

        const pick = (list) => {
            if (list.length === 0) return null;
            list.sort((a, b) => a.textLength - b.textLength);
            return list[0].node;
        };

        const target = pick(exact) || pick(includes);
        if (!target) return { ok: false, error: 'Chat title not found in side panel' };
        if (!clickTarget(target)) return { ok: false, error: 'Matched element is not clickable' };
        return { ok: true };
    })()
```

#### 3.3.7. `buildActivateViaPastConversationsScript(title)` (`src/services/chatSessionService.ts:218-434`)
Onde `${safeTitle}` é `JSON.stringify(title)`:
```javascript
(() => {
        const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
        const wantedRaw = ${safeTitle};
        const normalize = (text) => (text || '')
            .normalize('NFKC')
            .toLowerCase()
            .replace(/[\u2018\u2019\u201C\u201D'"`]/g, '')
            .replace(/\s+/g, ' ')
            .trim();
        const normalizeLoose = (text) => normalize(text).replace(/[^a-z0-9\u3040-\u30ff\u4e00-\u9faf\s]/g, '').replace(/\s+/g, ' ').trim();

        const wanted = normalize(wantedRaw || '');
        const wantedLoose = normalizeLoose(wantedRaw || '');
        if (!wanted) return { ok: false, error: 'Empty target title' };

        const isVisible = (el) => !!el && el instanceof HTMLElement && el.offsetParent !== null;
        const asArray = (nodeList) => Array.from(nodeList || []);
        const getLabelText = (el) => {
            if (!el || !(el instanceof Element)) return '';
            const parts = [
                el.textContent || '',
                el.getAttribute('aria-label') || '',
                el.getAttribute('title') || '',
                el.getAttribute('placeholder') || '',
                el.getAttribute('data-tooltip-content') || '',
                el.getAttribute('data-testid') || '',
            ];
            return parts.filter(Boolean).join(' ');
        };
        const getClickable = (el) => {
            if (!el || !(el instanceof Element)) return null;
            const clickable = el.closest('button, [role="button"], a, li, [role="option"], [data-testid*="conversation"]');
            return clickable instanceof HTMLElement ? clickable : (el instanceof HTMLElement ? el : null);
        };
        const pickBest = (elements, patterns) => {
            const matched = [];
            for (const el of elements) {
                if (!isVisible(el)) continue;
                const text = normalize(getLabelText(el));
                const textLoose = normalizeLoose(getLabelText(el));
                if (!text) continue;
                for (const pattern of patterns) {
                    if (!pattern) continue;
                    const p = normalize(pattern);
                    const pLoose = normalizeLoose(pattern);
                    if (
                        text === p ||
                        text.includes(p) ||
                        (pLoose && (textLoose === pLoose || textLoose.includes(pLoose)))
                    ) {
                        matched.push({ el, score: Math.abs(text.length - pattern.length) });
                        break;
                    }
                }
            }
            if (matched.length === 0) return null;
            matched.sort((a, b) => a.score - b.score);
            return matched[0].el;
        };
        const clickByPatterns = (patterns, selector) => {
            const nodes = asArray(document.querySelectorAll('button, [role="button"], a, li, div, span'));
            const scopedNodes = selector ? asArray(document.querySelectorAll(selector)) : [];
            const source = scopedNodes.length > 0 ? scopedNodes : nodes;
            const target = pickBest(source, patterns);
            const clickable = getClickable(target);
            if (!clickable) return false;
            clickable.click();
            return true;
        };
        const setInputValue = (el, value) => {
            if (!el) return false;
            if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) {
                el.focus();
                el.value = value;
                el.dispatchEvent(new Event('input', { bubbles: true }));
                el.dispatchEvent(new Event('change', { bubbles: true }));
                return true;
            }
            if (el instanceof HTMLElement) {
                el.focus();
                if (el.isContentEditable) {
                    el.textContent = value;
                    el.dispatchEvent(new Event('input', { bubbles: true }));
                    return true;
                }
            }
            return false;
        };
        const clickIconHistoryButton = () => {
            const iconTargets = asArray(document.querySelectorAll('svg, i, span, div'));
            const patterns = ['history', 'clock', 'conversation', 'past'];
            for (const icon of iconTargets) {
                const descriptor = normalize([
                    icon.getAttribute?.('class') || '',
                    icon.getAttribute?.('data-testid') || '',
                    icon.getAttribute?.('data-icon') || '',
                    icon.getAttribute?.('aria-label') || '',
                    icon.getAttribute?.('title') || '',
                    icon.getAttribute?.('data-tooltip-id') || '',
                ].join(' '));
                if (!descriptor) continue;
                if (!patterns.some((p) => descriptor.includes(p))) continue;
                const clickable = getClickable(icon);
                if (clickable && isVisible(clickable)) {
                    clickable.click();
                    return true;
                }
            }
            return false;
        };
        const openMenuThenClickPast = async () => {
            const openedMenu = clickByPatterns(
                ['more', 'options', 'menu', 'actions', '...', 'ellipsis', '設定', '操作'],
                'button[aria-haspopup], [role="button"][aria-haspopup], button, [role="button"]',
            );
            if (!openedMenu) return false;
            await wait(180);
            return clickByPatterns([
                'past conversations',
                'past conversation',
                'conversation history',
                'past chats',
                '過去の会話',
                'chat history',
            ], '[role="menuitem"], [role="option"], button, [role="button"], li, div, span');
        };
        const pressEnter = (el) => {
            if (!(el instanceof HTMLElement)) return;
            el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', bubbles: true }));
            el.dispatchEvent(new KeyboardEvent('keyup', { key: 'Enter', code: 'Enter', bubbles: true }));
        };
        const findSearchInput = () => {
            const inputs = asArray(document.querySelectorAll('input, textarea, [role="combobox"], [role="searchbox"], [contenteditable="true"]'));
            const strongPatterns = ['select a conversation', 'search conversation', 'search chats', 'search'];
            const placeholders = [];
            for (const el of inputs) {
                if (!isVisible(el)) continue;
                const placeholder = normalize(el.getAttribute('placeholder') || '');
                const ariaLabel = normalize(el.getAttribute('aria-label') || '');
                const text = normalize(getLabelText(el));
                const combined = [placeholder, ariaLabel, text].filter(Boolean).join(' ');
                placeholders.push({ el, combined });
            }
            for (const p of strongPatterns) {
                const found = placeholders.find((x) => x.combined.includes(p));
                if (found) return found.el;
            }
            return placeholders[0]?.el || null;
        };

        return (async () => {
            // Primary: click via data-past-conversations-toggle attribute
            let opened = false;
            const toggleBtn = document.querySelector('[data-past-conversations-toggle]');
            if (toggleBtn && isVisible(toggleBtn)) {
                const clickable = getClickable(toggleBtn);
                if (clickable) { clickable.click(); opened = true; }
            }
            if (!opened) {
                // Fallback: data-tooltip-id containing "history"
                const tooltipEls = asArray(document.querySelectorAll('[data-tooltip-id]'));
                for (const el of tooltipEls) {
                    if (!isVisible(el)) continue;
                    const tid = normalize(el.getAttribute('data-tooltip-id') || '');
                    if (tid.includes('history') || tid.includes('past-conversations')) {
                        const cl = getClickable(el);
                        if (cl) { cl.click(); opened = true; break; }
                    }
                }
            }
            if (!opened) {
                opened = clickByPatterns([
                    'past conversations',
                    'past conversation',
                    'conversation history',
                    'past chats',
                    '過去の会話',
                    'chat history',
                ]);
            }
            if (!opened) {
                opened = clickIconHistoryButton();
            }
            if (!opened) {
                opened = await openMenuThenClickPast();
            }
            if (!opened) {
                return { ok: false, error: 'Past Conversations button not found' };
            }

            await wait(320);

            // In some UI states "Select a conversation" itself is a trigger.
            clickByPatterns(['select a conversation', 'select conversation', 'conversation'], '[role="button"], button, [aria-haspopup], [data-testid*="conversation"]');
            await wait(220);

            const input = findSearchInput();
            if (input) {
                setInputValue(input, wantedRaw);
                await wait(260);
            }

            let selected = clickByPatterns([wanted, wantedLoose], '[role="option"], li, button, [data-testid*="conversation"]');
            if (!selected && input) {
                pressEnter(input);
                await wait(220);
                selected = true;
            }
            if (!selected) {
                return { ok: false, error: 'Conversation not found in Past Conversations' };
            }
            return { ok: true };
        })();
    })()
```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Métodos Auxiliares (`evaluateOnAnyContext`, `cdpMouseClick`, `getNewChatButtonState`)

1. **`evaluateOnAnyContext(cdpService, expression, awaitPromise)`**:
   - Obtém `contexts = cdpService.getContexts()`.
   - Para cada `ctx` em `contexts`:
     - Chama `cdpService.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise, contextId: ctx.id })`.
     - Extrai `value = result?.result?.value`.
     - Se `value` for truthy (não-nulo/não-vazio), retorna `value` imediatamente.
     - Se lançar exceção, ignora e tenta o próximo contexto.
   - Se nenhum contexto retornar valor truthy, retorna `null`.

2. **`cdpMouseClick(cdpService, x, y)`**:
   - Envia **3 chamadas sequenciais** a `Input.dispatchMouseEvent`:
     1. `{ type: 'mouseMoved', x, y }`
     2. `{ type: 'mousePressed', x, y, button: 'left', clickCount: 1 }`
     3. `{ type: 'mouseReleased', x, y, button: 'left', clickCount: 1 }`

3. **`getNewChatButtonState(cdpService, contexts)`**:
   - Itera sobre `contexts`, avaliando `GET_NEW_CHAT_BUTTON_SCRIPT` (Seção 3.3.1) com `returnByValue: true, contextId: ctx.id`.
   - Se `value?.found === true`, retorna `{ found: true, enabled: value.enabled, x: value.x, y: value.y }`.
   - Se nenhum contexto encontrar o botão, retorna `{ found: false, enabled: false, x: 0, y: 0 }`.

---

### 4.2. Listar Sessões do Workspace Atual (`listAllSessions`, linhas 461–514)

Todo o corpo de `listAllSessions(cdpService)` é protegido por `try / catch` que retorna `[]` em caso de qualquer erro:

1. **Passo 1 — Localizar botão Past Conversations**:
   - Chama `btnState = await this.evaluateOnAnyContext(cdpService, FIND_PAST_CONVERSATIONS_BUTTON_SCRIPT, false)`.
   - Se `!btnState?.found`, retorna `[]`.
2. **Passo 2 — Clicar via eventos nativos de mouse**:
   - Chama `await this.cdpMouseClick(cdpService, btnState.x, btnState.y)`.
3. **Passo 3 — Aguardar renderização do modal**:
   - Aguarda **500ms**.
4. **Passo 4 — Primeiro Scrape**:
   - Chama `scrapeResult = await this.evaluateOnAnyContext(cdpService, SCRAPE_PAST_CONVERSATIONS_SCRIPT, false)`.
   - Extrai `sessions = scrapeResult?.sessions ?? []`.
   - O script `SCRAPE_PAST_CONVERSATIONS_SCRIPT` (Seção 3.3.4) localiza o container rolável, identifica a posição vertical `boundaryTop` do cabeçalho `"Other Conversations"` e coleta apenas as linhas `div[class*="cursor-pointer"]` visíveis cujo `getBoundingClientRect().top < boundaryTop` (excluindo sessões de outros projetos).
5. **Passo 5 e 6 — Expansão via `"Show N more..."` se houver menos de 20 sessões**:
   - Se `sessions.length < 20` (`ChatSessionService.LIST_SESSIONS_TARGET`):
     - Chama `showMoreState = await this.evaluateOnAnyContext(cdpService, FIND_SHOW_MORE_BUTTON_SCRIPT, false)`.
     - Se `showMoreState?.found` for `true`:
       - Clica nas coordenadas `(showMoreState.x, showMoreState.y)` via `this.cdpMouseClick(cdpService, showMoreState.x, showMoreState.y)`.
       - Aguarda **500ms**.
       - Executa o scrape novamente (`SCRAPE_PAST_CONVERSATIONS_SCRIPT`) e atualiza `sessions = scrapeResult?.sessions ?? []`.
6. **Passo 7 — Fechar o painel com tecla `Escape`**:
   - Envia 2 eventos `Input.dispatchKeyEvent`:
     1. `{ type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27, nativeVirtualKeyCode: 27 }`
     2. `{ type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27, nativeVirtualKeyCode: 27 }`
7. **Passo 8 — Retorno limitado**:
   - Retorna `sessions.slice(0, 20)`.

---

### 4.3. Iniciar Novo Chat (`startNewChat`, linhas 564–626)

1. **Verificação e Espera de Contextos**:
   - Obtém `contexts = cdpService.getContexts()`.
   - Se `contexts.length === 0`:
     - Chama `ready = await cdpService.waitForCascadePanelReady(10000, 500)`.
     - Se `!ready`, retorna `{ ok: false, error: 'No contexts available (timed out)' }`.
     - Atualiza `contexts = cdpService.getContexts()`.
2. **Obtenção do Estado do Botão Novo Chat (com até 5 retries)**:
   - Chama `btnState = await this.getNewChatButtonState(cdpService, contexts)`.
   - Se `!btnState.found`:
     - Executa um loop de até `maxRetries = 5` vezes enquanto `!btnState.found`:
       - Aguarda **1000ms**.
       - Atualiza `contexts = cdpService.getContexts()`.
       - Chama `btnState = await this.getNewChatButtonState(cdpService, contexts)`.
   - Se após os retries `!btnState.found` continuar verdadeiro:
     - Retorna `{ ok: false, error: 'New chat button not found' }`.
3. **Verificação de Chat Já Limpo (`cursor !== 'pointer'`)**:
   - Se `!btnState.enabled` (ou seja, `cursor` é `'not-allowed'` porque já está em uma conversa nova vazia):
     - Retorna `{ ok: true }` imediatamente sem despachar eventos de mouse.
4. **Clique por Coordenadas e Verificação Pós-Clique**:
   - Despacha os 3 eventos `Input.dispatchMouseEvent` (`mouseMoved`, `mousePressed`, `mouseReleased` com `button: 'left', clickCount: 1`) em `(btnState.x, btnState.y)`.
   - Aguarda **1500ms**.
   - Chama `afterState = await this.getNewChatButtonState(cdpService, contexts)`.
   - Se `afterState.found && !afterState.enabled`:
     - Retorna `{ ok: true }`.
   - Caso contrário (o botão continuou habilitado):
     - Retorna `{ ok: false, error: 'Clicked new chat button but state did not change' }`.
5. Se qualquer erro não tratado ocorrer no bloco `try`, captura e retorna `{ ok: false, error: message }`.

---

### 4.4. Obter Informações da Sessão Atual (`getCurrentSessionInfo`, linhas 633–655)

1. Obtém `contexts = cdpService.getContexts()`.
2. Para cada `ctx` em `contexts`:
   - Avalia `GET_CHAT_TITLE_SCRIPT` (Seção 3.3.2) via `Runtime.evaluate` com `returnByValue: true, contextId: ctx.id`.
   - Se `value && value.title` for uma string não-vazia:
     - Retorna `{ title: value.title, hasActiveChat: value.hasActiveChat ?? false }`.
     - Nota: dentro de `GET_CHAT_TITLE_SCRIPT`, se o painel e o header existirem mas o `titleEl` estiver vazio, o script retorna `{ title: '(Untitled)', hasActiveChat: false }`; se o texto for `'Agent'`, retorna `{ title: 'Agent', hasActiveChat: false }`.
3. Se todos os contextos retornarem `title: ''`, `null` ou lançarem exceção:
   - Retorna `{ title: '(Failed to retrieve)', hasActiveChat: false }`.

---

### 4.5. Ativar Sessão por Título (`activateSessionByTitle`, linhas 662–796)

1. **Validação de Entrada**:
   - Se `!title || title.trim().length === 0`, retorna `{ ok: false, error: 'Session title is empty' }`.
2. **Short-circuit se já estiver na sessão alvo**:
   - Chama `current = await this.getCurrentSessionInfo(cdpService)`.
   - Se `current.title.trim().toLowerCase() === title.trim().toLowerCase()`, retorna `{ ok: true }`.
3. **Configuração do Loop de Retry**:
   - `maxWaitMs = options?.maxWaitMs ?? 30000` (`ACTIVATE_SESSION_MAX_WAIT_MS`).
   - `retryIntervalMs = options?.retryIntervalMs ?? 800` (`ACTIVATE_SESSION_RETRY_INTERVAL_MS`).
   - Inicializa `usedPastConversations = false`, `directResult = { ok: false, error: 'not attempted' }`, `pastResult = null`, `clicked = false`, `startedAt = Date.now()`, `attempts = 0`.
4. **Loop de Tentativas (`while (Date.now() - startedAt <= maxWaitMs)`)**:
   - Incrementa `attempts += 1`.
   - **Tentativa Direta no Painel Lateral**:
     - Chama `directResult = await this.tryActivateByDirectSidePanel(cdpService, title)` (que avalia `buildActivateChatByTitleScript(title)` com `awaitPromise: false` em todos os contextos até encontrar `value?.ok === true`).
     - `clicked = directResult.ok`.
   - **Fallback via Past Conversations**:
     - Se `!clicked`:
       - Chama `pastResult = await this.tryActivateByPastConversations(cdpService, title)` (que avalia `buildActivateViaPastConversationsScript(title)` com `awaitPromise: true`).
       - `clicked = pastResult.ok`.
       - `usedPastConversations = pastResult.ok`.
   - Se `clicked` for `true`, sai do loop (`break`).
   - Se `Date.now() - startedAt <= maxWaitMs`, aguarda `retryIntervalMs` antes da próxima tentativa.
5. **Falha após esgotar todas as tentativas**:
   - Se `!clicked`:
     - Retorna:
       ```ts
       {
           ok: false,
           error: `Failed to activate session "${title}" after ${attempts} attempt(s) (direct: ${directResult.error || 'direct search failed'}; past: ${pastResult?.error || 'past conversations search failed'})`
       }
       ```
6. **Verificação Pós-Clique**:
   - Aguarda **500ms**.
   - Chama `after = await this.getCurrentSessionInfo(cdpService)`.
   - Se `after.title.trim().toLowerCase() === title.trim().toLowerCase()`, retorna `{ ok: true }`.
7. **Fallback Secundário para Past Conversations (se a busca direta clicou na linha errada)**:
   - Se `!usedPastConversations`:
     - Chama `viaPast = await this.tryActivateByPastConversations(cdpService, title)`.
     - Se `viaPast.ok`:
       - Aguarda **500ms** e chama `afterPast = await this.getCurrentSessionInfo(cdpService)`.
       - Se `afterPast.title.trim().toLowerCase() === title.trim().toLowerCase()`, retorna `{ ok: true }`.
       - Caso contrário, retorna:
         `{ ok: false, error: 'Past Conversations selected a different chat (expected="' + title + '", actual="' + afterPast.title + '")' }`.
     - Se `!viaPast.ok`, retorna:
       `{ ok: false, error: 'Activated chat did not match target title (expected="' + title + '", actual="' + after.title + '") and Past Conversations fallback failed (' + (viaPast.error || 'unknown') + ')' }`.
8. **Falha de Verificação quando `usedPastConversations` já era `true`**:
   - Retorna `{ ok: false, error: 'Activated chat did not match target title (expected="' + title + '", actual="' + after.title + '")' }`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Design Stateless e Compatível com Pool**:
   - `ChatSessionService` não armazena estado mutável por workspace nem referências permanentes a `Client`. O `SessionCDPClient` é injetado em cada chamada de método (`ListAllSessions(cdp)`, `StartNewChat(cdp)`, `GetCurrentSessionInfo(cdp)`, `ActivateSessionByTitle(cdp, title)`), tornando uma única instância de `ChatSessionService` 100% segura para uso concorrente entre diferentes workspaces do `ConnectionPool`.
2. **Garantia de Fechamento do Modal `Past Conversations`**:
   - Em `ListAllSessions`, o envio de `Escape` (`keyDown` + `keyUp`) ocorre sempre após o scrape (e eventual expansão `"Show N more..."`) antes de retornar os itens, garantindo que o modal de histórico não permaneça aberto bloqueando cliques subsequentes no painel Cascade.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Cada caso de teste abaixo mapeia 1:1 os **21 blocos `it(...)`** de [`tests/services/chatSessionService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/chatSessionService.test.ts) para funções de teste em `internal/cdp/session_test.go`.

### 6.1. `startNewChat()` (Linhas 20–106 de `chatSessionService.test.ts`)

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `startNewChat()` $\rightarrow$ `opens a new chat via coordinate click when the button is enabled` | `TestChatSessionService_StartNewChat_ClicksWhenEnabledAndVerifiesDisabled(t *testing.T)` | Na 1ª chamada a `Runtime.evaluate`, o botão retorna `{ found: true, enabled: true, x: 100, y: 50 }`; na 2ª chamada (após o clique), retorna `{ found: true, enabled: false, x: 100, y: 50 }`. Verifica que `Input.dispatchMouseEvent` foi chamado com `{ type: "mousePressed", x: 100, y: 50, button: "left", clickCount: 1 }` e `StartNewChat` retorna `OK: true`. |
| `startNewChat()` $\rightarrow$ `returns success without action when the button is disabled (already empty chat)` | `TestChatSessionService_StartNewChat_NoOpWhenAlreadyDisabled(t *testing.T)` | Quando `Runtime.evaluate` retorna `{ found: true, enabled: false, cursor: "not-allowed", x: 100, y: 50 }`, `StartNewChat` retorna `OK: true` e **não** invoca `Input.dispatchMouseEvent`. |
| `startNewChat()` $\rightarrow$ `returns ok: false when the button is not found` | `TestChatSessionService_StartNewChat_FailsWhenButtonNotFound(t *testing.T)` | Quando `Runtime.evaluate` retorna `{ found: false }` em todas as tentativas, retorna `OK: false` e `Error` contendo `"not found"`. |
| `startNewChat()` $\rightarrow$ `returns ok: false when contexts are empty` | `TestChatSessionService_StartNewChat_FailsWhenNoContextsAvailable(t *testing.T)` | Quando `GetContexts()` retorna `[]` e `WaitForCascadePanelReady` retorna `false`, retorna `OK: false` e `Error` contendo `"No contexts available"`. |
| `startNewChat()` $\rightarrow$ `returns ok: false when a CDP call throws an exception` | `TestChatSessionService_StartNewChat_FailsOnCDPException(t *testing.T)` | Quando `Call` retorna erro `"WebSocket切断"`, retorna `OK: false` e `Error != ""`. |
| `startNewChat()` $\rightarrow$ `returns ok: false when button state does not change after click` | `TestChatSessionService_StartNewChat_FailsWhenStateDoesNotChangeAfterClick(t *testing.T)` | Quando o botão permanece `{ found: true, enabled: true, x: 100, y: 50 }` mesmo após o clique, retorna `OK: false` e `Error` contendo `"state did not change"`. |

### 6.2. `getCurrentSessionInfo()` (Linhas 108–150 de `chatSessionService.test.ts`)

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `getCurrentSessionInfo()` $\rightarrow$ `retrieves the chat title from the Cascade panel header` | `TestChatSessionService_GetCurrentSessionInfo_ReturnsActiveChatTitle(t *testing.T)` | Quando `Runtime.evaluate` retorna `{ title: "テストチャット", hasActiveChat: true }`, retorna `Title: "テストチャット", HasActiveChat: true`. |
| `getCurrentSessionInfo()` $\rightarrow$ `returns hasActiveChat: false when the title is "Agent" (default)` | `TestChatSessionService_GetCurrentSessionInfo_DefaultAgentTitleIsInactive(t *testing.T)` | Quando `Runtime.evaluate` retorna `{ title: "Agent", hasActiveChat: false }`, retorna `Title: "Agent", HasActiveChat: false`. |
| `getCurrentSessionInfo()` $\rightarrow$ `returns fallback values when a CDP call throws an exception` | `TestChatSessionService_GetCurrentSessionInfo_FallbackOnCDPError(t *testing.T)` | Quando `Call` retorna erro `"CDPエラー"`, retorna `Title: "(Failed to retrieve)", HasActiveChat: false`. |
| `getCurrentSessionInfo()` $\rightarrow$ `returns fallback values when the result is null` | `TestChatSessionService_GetCurrentSessionInfo_FallbackOnNullValue(t *testing.T)` | Quando `Runtime.evaluate` retorna `{ result: { value: null } }`, retorna `Title: "(Failed to retrieve)", HasActiveChat: false`. |

### 6.3. `activateSessionByTitle()` (Linhas 152–279 de `chatSessionService.test.ts`)

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `activateSessionByTitle()` $\rightarrow$ `returns ok when already on the target session title` | `TestChatSessionService_ActivateSessionByTitle_NoOpWhenAlreadyActive(t *testing.T)` | Se `GetCurrentSessionInfo` já retorna `title: "target-session"`, `ActivateSessionByTitle(cdp, "target-session")` retorna `OK: true` imediatamente. |
| `activateSessionByTitle()` $\rightarrow$ `returns ok:false when switching succeeded but verification title mismatches` | `TestChatSessionService_ActivateSessionByTitle_FailsOnVerificationTitleMismatch(t *testing.T)` | Se o clique direto retorna `{ ok: true }` mas a verificação subsequente de título retorna `"different-session"` e o fallback falha, retorna `OK: false` com `Error` contendo `"did not match target title"`. |
| `activateSessionByTitle()` $\rightarrow$ `falls back to Past Conversations flow when direct side-panel search cannot find the chat` | `TestChatSessionService_ActivateSessionByTitle_FallsBackToPastConversations(t *testing.T)` | Quando o script direto (`Chat title not found in side panel`) retorna `{ ok: false }` e o script de Past Conversations (`Past Conversations button not found`) retorna `{ ok: true }`, seguido do título `"target-session"`, retorna `OK: true`. |
| `activateSessionByTitle()` $\rightarrow$ `retries activation while UI is still loading and eventually succeeds` | `TestChatSessionService_ActivateSessionByTitle_RetriesUntilSuccess(t *testing.T)` | Quando as 2 primeiras tentativas diretas falham com `'side panel still loading'` e a 3ª tentativa retorna `{ ok: true }`, `ActivateSessionByTitle` com `MaxWait: 100ms, RetryInterval: 1ms` retorna `OK: true` após exatamente 3 tentativas diretas. |
| `activateSessionByTitle()` $\rightarrow$ `returns ok:false for empty title` | `TestChatSessionService_ActivateSessionByTitle_FailsOnEmptyTitle(t *testing.T)` | Passar `""` retorna `OK: false` com `Error` contendo `"empty"`. |
| `activateSessionByTitle()` $\rightarrow$ `returns direct and past errors when both activation paths fail` | `TestChatSessionService_ActivateSessionByTitle_ReportsBothDirectAndPastErrors(t *testing.T)` | Quando o caminho direto falha com `"direct miss"` e o caminho Past Conversations falha com `"past miss"`, o erro retornado contém `"direct: direct miss"`, `"past: past miss"` e `"after"`. |

### 6.4. `listAllSessions()` (Linhas 281–390 de `chatSessionService.test.ts`)

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `listAllSessions()` $\rightarrow$ `opens Past Conversations via CDP mouse click and returns scraped sessions` | `TestChatSessionService_ListAllSessions_ScrapesViaCDPMouseClick(t *testing.T)` | Quando a 1ª avaliação retorna `{ found: true, x: 200, y: 30 }` e a 2ª retorna 2 sessões (`Fix login bug` ativa, `Refactor auth` inativa), verifica que `Input.dispatchMouseEvent` foi chamado e retorna os 2 itens. |
| `listAllSessions()` $\rightarrow$ `clicks "Show more" when fewer than 10 sessions found initially` | `TestChatSessionService_ListAllSessions_ClicksShowMoreWhenBelowTarget(t *testing.T)` | Quando o 1º scrape retorna 3 sessões (`< 20`), busca o botão `"Show more"` (`{ found: true, x: 150, y: 300 }`), clica via `Input.dispatchMouseEvent`, faz o 2º scrape retornando 5 sessões e realiza 4 chamadas `Runtime.evaluate` no total. |
| `listAllSessions()` $\rightarrow$ `returns empty array when Past Conversations button not found` | `TestChatSessionService_ListAllSessions_EmptyWhenButtonNotFound(t *testing.T)` | Quando `FIND_PAST_CONVERSATIONS_BUTTON_SCRIPT` retorna `{ found: false, x: 0, y: 0 }`, retorna `[]SessionListItem{}` vazio. |
| `listAllSessions()` $\rightarrow$ `returns empty array when CDP call throws` | `TestChatSessionService_ListAllSessions_EmptyOnCDPError(t *testing.T)` | Quando `Call` falha com `"WebSocket disconnected"`, retorna `[]SessionListItem{}` vazio sem pânico. |
| `listAllSessions()` $\rightarrow$ `closes panel with Escape key after scraping` | `TestChatSessionService_ListAllSessions_ClosesPanelWithEscapeKey(t *testing.T)` | Verifica que após o scrape é enviada chamada `Input.dispatchKeyEvent` com `key: "Escape"`. |
