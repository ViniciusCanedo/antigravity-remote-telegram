# 14 — Componentes de UI do Telegram (Teclados Inline) e Gerenciador de Tópicos de Fórum

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define, de forma 100% autocontida e exaustiva, a implementação em **Golang (1.23+)** de todos os 8 construtores de interface de usuário do Telegram (`src/ui/*.ts`) e do gerenciador de tópicos de fórum (`src/services/telegramTopicManager.ts`).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript) | Linhas | Arquivo de Teste Original (Jest) | Linhas | Destino no Port Go |
| :--- | :---: | :--- | :---: | :--- |
| [`src/ui/autoAcceptUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/autoAcceptUi.ts) | 27 | [`tests/ui/autoAcceptUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/autoAcceptUi.test.ts) | 33 | `internal/bot/ui/autoaccept_ui.go`<br>`internal/bot/ui/autoaccept_ui_test.go` |
| [`src/ui/modeUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/modeUi.ts) | 65 | [`tests/ui/modeUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/modeUi.test.ts) | 58 | `internal/bot/ui/mode_ui.go`<br>`internal/bot/ui/mode_ui_test.go` |
| [`src/ui/modelsUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/modelsUi.ts) | 120 | [`tests/ui/modelsUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/modelsUi.test.ts) | 143 | `internal/bot/ui/models_ui.go`<br>`internal/bot/ui/models_ui_test.go` |
| [`src/ui/planUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/planUi.ts) | 177 | [`tests/ui/planUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/planUi.test.ts) | 139 | `internal/bot/ui/plan_ui.go`<br>`internal/bot/ui/plan_ui_test.go` |
| [`src/ui/projectListUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/projectListUi.ts) | 71 | [`tests/ui/projectListUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/projectListUi.test.ts) | 119 | `internal/bot/ui/project_ui.go`<br>`internal/bot/ui/project_ui_test.go` |
| [`src/ui/screenshotUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/screenshotUi.ts) | 29 | [`tests/ui/screenshotUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/screenshotUi.test.ts) | 58 | `internal/bot/ui/screenshot_ui.go`<br>`internal/bot/ui/screenshot_ui_test.go` |
| [`src/ui/sessionPickerUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/sessionPickerUi.ts) | 37 | [`tests/ui/sessionPickerUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/sessionPickerUi.test.ts) | 56 | `internal/bot/ui/session_ui.go`<br>`internal/bot/ui/session_ui_test.go` |
| [`src/ui/templateUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/templateUi.ts) | 55 | [`tests/ui/templateUi.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/ui/templateUi.test.ts) | 77 | `internal/bot/ui/template_ui.go`<br>`internal/bot/ui/template_ui_test.go` |
| [`src/services/telegramTopicManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/telegramTopicManager.ts) | 112 | *(coberto pela suíte de tópicos)* | — | `internal/bot/topic_manager.go`<br>`internal/bot/topic_manager_test.go` |

### 1.2. Documentação Arquitetural Relacionada
- [`docs/07-bot-telegram-comandos-e-ui.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/07-bot-telegram-comandos-e-ui.md)
- [`docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md) (ADR-005: Exposição do Seletor Interativo de Sessões em `/chat` e `/join`)

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Estruturas Comuns de Teclado Inline (`internal/bot/ui/types.go`)

Para desacoplar os construtores de UI de implementações específicas de clientes HTTP do Telegram e manter 100% de paridade com `InlineKeyboard` do `grammy`, o pacote `internal/bot/ui` define:

```go
package ui

import "context"

// InlineKeyboardButton representa um botão individual de um teclado inline do Telegram.
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

// InlineKeyboardMarkup representa a matriz de botões inline enviada em reply_markup.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// NewInlineKeyboard cria um teclado inline vazio (InlineKeyboard: [][]InlineKeyboardButton{}).
func NewInlineKeyboard() *InlineKeyboardMarkup

// Text adiciona um botão na linha atual (criando a primeira linha se necessário).
func (k *InlineKeyboardMarkup) Text(label, callbackData string) *InlineKeyboardMarkup

// Row finaliza a linha atual e prepara uma nova linha para os próximos botões.
func (k *InlineKeyboardMarkup) Row() *InlineKeyboardMarkup

// UIPayload encapsula o texto HTML formatado e o teclado inline resultante.
type UIPayload struct {
	Text     string
	Keyboard *InlineKeyboardMarkup
}

// SendUIFunc é o callback de envio/edição usado por SendAutoAcceptUI, SendModeUI, SendModelsUI e SendTemplateUI.
type SendUIFunc func(ctx context.Context, text string, keyboard *InlineKeyboardMarkup) error
```

---

### 2.2. Auto-Accept UI (`internal/bot/ui/autoaccept_ui.go`)

```go
package ui

import "context"

const (
	AutoAcceptBtnOn      = "autoaccept_btn_on"
	AutoAcceptBtnOff     = "autoaccept_btn_off"
	AutoAcceptBtnRefresh = "autoaccept_btn_refresh"
)

// AutoAcceptStateProvider abstrai AutoAcceptService.
type AutoAcceptStateProvider interface {
	IsEnabled() bool
}

// BuildAutoAcceptUI constrói o texto HTML e o teclado inline de gerenciamento de Auto-accept.
func BuildAutoAcceptUI(autoAcceptService AutoAcceptStateProvider) UIPayload

// SendAutoAcceptUI constrói e envia a interface de gerenciamento de Auto-accept via sendFn.
func SendAutoAcceptUI(
	ctx context.Context,
	sendFn SendUIFunc,
	autoAcceptService AutoAcceptStateProvider,
) error
```

---

### 2.3. Mode UI (`internal/bot/ui/mode_ui.go`)

```go
package ui

import "context"

// ModeCdpProvider abstrai a leitura do modo atual diretamente da UI via CDP.
type ModeCdpProvider interface {
	GetCurrentMode(ctx context.Context) (string, error)
}

// ModeServiceProvider abstrai ModeService.
type ModeServiceProvider interface {
	GetCurrentMode() string
	SetMode(mode string)
}

// ModeUIDeps representa as dependências opcionais para sincronização ao vivo do modo com o CDP.
type ModeUIDeps struct {
	GetCurrentCdp func() ModeCdpProvider
}

// BuildModeUI sincroniza (se deps.GetCurrentCdp retornar instância não-nula) e constrói o payload de Mode Management.
func BuildModeUI(
	ctx context.Context,
	modeService ModeServiceProvider,
	deps *ModeUIDeps,
) (UIPayload, error)

// SendModeUI constrói o payload via BuildModeUI e o envia através de sendFn.
func SendModeUI(
	ctx context.Context,
	sendFn SendUIFunc,
	modeService ModeServiceProvider,
	deps *ModeUIDeps,
) error
```

---

### 2.4. Models UI (`internal/bot/ui/models_ui.go`)

```go
package ui

import (
	"context"
	"time"
)

const (
	ModelBtnPrefix       = "model_btn_"
	ModelExhaustedPrefix = "model_exhausted_"
	ModelRefreshBtn      = "model_refresh_btn"
	MaxModelButtons      = 24
)

// QuotaInfo representa os dados de fração restante e horário de reset de cota de um modelo.
type QuotaInfo struct {
	RemainingFraction *float64 // nil ou NaN quando indisponível
	ResetTime         string   // ISO-8601 timestamp ou vazio
}

// ModelQuotaEntry representa um item retornado por QuotaService.FetchQuota().
type ModelQuotaEntry struct {
	Label     string
	Model     string
	QuotaInfo *QuotaInfo
}

// ModelsCdpProvider abstrai os métodos de modelos do CdpService.
type ModelsCdpProvider interface {
	GetUiModels(ctx context.Context) ([]string, error)
	GetCurrentModel(ctx context.Context) (string, error)
}

// ModelsUIDeps encapsula os provedores de CDP e de cotas para SendModelsUI.
type ModelsUIDeps struct {
	GetCurrentCdp func() ModelsCdpProvider
	FetchQuota    func(ctx context.Context) ([]ModelQuotaEntry, error)
	NowFunc       func() time.Time // opcional; padrão time.Now
}

// BuildModelsUI consulta modelos da UI, modelo atual e cotas, retornando nil se a lista de modelos estiver vazia.
func BuildModelsUI(
	ctx context.Context,
	cdp ModelsCdpProvider,
	fetchQuota func(ctx context.Context) ([]ModelQuotaEntry, error),
	nowFunc ...func() time.Time,
) (*UIPayload, error)

// SendModelsUI valida a conexão CDP, invoca BuildModelsUI e envia o resultado (ou mensagem de erro) via sendFn.
func SendModelsUI(
	ctx context.Context,
	sendFn SendUIFunc,
	deps ModelsUIDeps,
) error
```

---

### 2.5. Plan UI (`internal/bot/ui/plan_ui.go`)

```go
package ui

const (
	PlanViewBtn     = "plan_view_btn"
	PlanProceedBtn  = "plan_proceed_btn"
	PlanEditBtn     = "plan_edit_btn"
	PlanRefreshBtn  = "plan_refresh_btn"
	PlanPagePrefix  = "plan_page"
	DefaultPageSize = 3500
)

// PlanningInfo espelha os metadados extraídos pelo PlanningDetector.
type PlanningInfo struct {
	OpenText    string
	ProceedText string
	PlanTitle   string
	PlanSummary string
	Description string
}

// MarkdownToTelegramHTML converte Markdown de planos em HTML compatível com o Telegram.
func MarkdownToTelegramHTML(md string) string

// BuildPlanNotificationUI constrói o card inicial de notificação de Planning Mode com 4 botões de ação.
func BuildPlanNotificationUI(
	info PlanningInfo,
	projectName string,
	targetChannelStr string,
) UIPayload

// PaginatePlanContent divide o conteúdo do plano em páginas respeitando quebras de linha (\n) até pageSize.
// Se pageSize <= 0, assume DefaultPageSize (3500).
func PaginatePlanContent(content string, pageSize int) []string

// BuildPlanContentUI constrói uma página específica do plano convertido em HTML com botões de paginação e de ação.
func BuildPlanContentUI(
	pages []string,
	currentPage int,
	projectName string,
	targetChannelStr string,
) UIPayload
```

---

### 2.6. Project List UI (`internal/bot/ui/project_ui.go`)

```go
package ui

const (
	ProjectSelectID    = "project_select"
	WorkspaceSelectID  = "workspace_select"
	ProjectPagePrefix  = "project_page"
	ProjectItemsPerPage = 100
)

// ParseProjectPageID extrai o índice numérico da página de "project_page:<page>".
// Retorna (page, true) se válido, ou (0, false) se inválido (equivalente a NaN no TypeScript).
func ParseProjectPageID(customID string) (int, bool)

// IsProjectSelectID verifica se customID corresponde a project_select, workspace_select ou project_select:<path>.
func IsProjectSelectID(customID string) bool

// BuildProjectListUI constrói a mensagem HTML e o teclado inline paginado para a lista de workspaces.
func BuildProjectListUI(workspaces []string, page int) UIPayload
```

---

### 2.7. Screenshot UI (`internal/bot/ui/screenshot_ui.go`)

```go
package ui

import "context"

// ScreenshotResult representa o retorno de ScreenshotService.Capture.
type ScreenshotResult struct {
	Success bool
	Buffer  []byte
	Error   string
}

// ScreenshotCapturer abstrai a captura de tela via CDP.
type ScreenshotCapturer interface {
	Capture(ctx context.Context, format string) (ScreenshotResult, error)
}

// SendPhotoFunc envia um arquivo de imagem (nome + bytes) com legenda opcional.
type SendPhotoFunc func(ctx context.Context, filename string, data []byte, caption string) error

// SendTextFunc envia uma mensagem de texto simples.
type SendTextFunc func(ctx context.Context, text string) error

// HandleScreenshot captura um screenshot PNG do Antigravity e o envia via sendPhoto, ou envia erro via sendText.
// Se cdpIsNil for true (ou capturer == nil), envia imediatamente "Not connected to Antigravity.".
func HandleScreenshot(
	ctx context.Context,
	sendPhoto SendPhotoFunc,
	sendText SendTextFunc,
	capturer ScreenshotCapturer,
) error
```

---

### 2.8. Session Picker UI (`internal/bot/ui/session_ui.go`)

```go
package ui

const (
	SessionSelectID         = "session_select"
	MaxSessionSelectOptions = 25
)

// SessionListItem representa uma sessão retornada por ChatSessionService.ListAllSessions.
type SessionListItem struct {
	Title    string
	IsActive bool
}

// IsSessionSelectID verifica se customID inicia com "session_select:" ou é exatamente "session_select".
func IsSessionSelectID(customID string) bool

// BuildSessionPickerUI constrói o seletor interativo de sessões (máximo de 25 botões).
func BuildSessionPickerUI(sessions []SessionListItem) UIPayload
```

---

### 2.9. Template UI (`internal/bot/ui/template_ui.go`)

```go
package ui

import "context"

const (
	TemplateBtnPrefix      = "template_btn_"
	MaxPromptPreviewLen    = 60
	MaxTemplateButtons     = 25
)

// TemplateItem representa um registro de template vindo do TemplateRepository.
type TemplateItem struct {
	ID        int
	Name      string
	Prompt    string
	CreatedAt string
}

// ParseTemplateButtonID extrai o ID numérico de "template_btn_<id>".
// Retorna (id, true) se válido, ou (0, false) caso contrário (equivalente a NaN no TypeScript).
func ParseTemplateButtonID(customID string) (int, bool)

// BuildTemplateUI constrói o texto HTML e o teclado inline para a lista de templates.
func BuildTemplateUI(templates []TemplateItem) UIPayload

// SendTemplateUI constrói o payload com BuildTemplateUI e o envia via sendFn.
func SendTemplateUI(
	ctx context.Context,
	sendFn SendUIFunc,
	templates []TemplateItem,
) error
```

---

### 2.10. Telegram Topic Manager (`internal/bot/topic_manager.go`)

```go
package bot

import (
	"context"
	"sync"
)

// EnsureTopicResult representa o resultado de EnsureTopic.
type EnsureTopicResult struct {
	TopicID int
	Created bool
}

// ForumTopicAPI abstrai as chamadas de Forum Topic da Bot API do Telegram.
type ForumTopicAPI interface {
	CreateForumTopic(ctx context.Context, chatID int64, name string) (messageThreadID int, err error)
	EditForumTopic(ctx context.Context, chatID int64, messageThreadID int, name string) error
	CloseForumTopic(ctx context.Context, chatID int64, messageThreadID int) error
	DeleteForumTopic(ctx context.Context, chatID int64, messageThreadID int) error
}

// TelegramTopicManager gerencia tópicos de fórum do Telegram para isolamento de workspaces e sessões.
type TelegramTopicManager struct {
	mu         sync.RWMutex
	chatID     int64
	api        ForumTopicAPI
	topicCache map[string]int
}

// NewTelegramTopicManager cria uma nova instância de TelegramTopicManager.
func NewTelegramTopicManager(api ForumTopicAPI, chatID int64) *TelegramTopicManager

// EnsureTopic garante que um tópico de fórum exista para workspaceName ("🗂️ " + workspaceName).
func (m *TelegramTopicManager) EnsureTopic(ctx context.Context, workspaceName string) (EnsureTopicResult, error)

// CreateSessionTopic cria um tópico de sessão sob o contexto atual ("💬 " + sessionName).
func (m *TelegramTopicManager) CreateSessionTopic(ctx context.Context, sessionName string) (int, error)

// RenameTopic renomeia um tópico de fórum existente (erros são logados e suprimidos, sem propagar panic/erro).
func (m *TelegramTopicManager) RenameTopic(ctx context.Context, topicID int, newName string)

// CloseTopic fecha (arquiva) um tópico de fórum (erros são logados e suprimidos).
func (m *TelegramTopicManager) CloseTopic(ctx context.Context, topicID int)

// DeleteTopic exclui um tópico de fórum (erros são logados e suprimidos).
func (m *TelegramTopicManager) DeleteTopic(ctx context.Context, topicID int)

// SetChatID atualiza o chatID alvo do gerenciador de tópicos.
func (m *TelegramTopicManager) SetChatID(chatID int64)

// RegisterTopic registra manualmente um par (workspaceName, topicID) no cache em memória.
func (m *TelegramTopicManager) RegisterTopic(workspaceName string, topicID int)

// GetTopicID consulta o topicID associado a workspaceName no cache em memória.
func (m *TelegramTopicManager) GetTopicID(workspaceName string) (int, bool)

// SanitizeName higieniza um nome de workspace/tópico conforme as regras de sanitização.
func (m *TelegramTopicManager) SanitizeName(name string) string
```

---

## 3. Constantes, Templates HTML/ASCII, Prefixos de Callback e Expressões Regulares Literais

### 3.1. Tabela Completa de Prefixos e Constantes de `callback_data`

| Constante | Valor Literal Exato | Limite / Regra de Truncamento | Arquivo Origem |
| :--- | :--- | :--- | :--- |
| `AUTOACCEPT_BTN_ON` | `'autoaccept_btn_on'` | Fixo (17 bytes) | `src/ui/autoAcceptUi.ts` |
| `AUTOACCEPT_BTN_OFF` | `'autoaccept_btn_off'` | Fixo (18 bytes) | `src/ui/autoAcceptUi.ts` |
| `AUTOACCEPT_BTN_REFRESH` | `'autoaccept_btn_refresh'` | Fixo (22 bytes) | `src/ui/autoAcceptUi.ts` |
| Mode Select | `'mode_select:' + m` | `mode_select:fast` / `mode_select:plan` | `src/ui/modeUi.ts` |
| `model_btn_` | `'model_btn_' + cbName` | `MAX_BUTTONS = 24`; `maxNameLen = 64 - 'model_exhausted_'.length` (`48`); `cbName = mName.slice(0, 48)` | `src/ui/modelsUi.ts` |
| `model_exhausted_` | `'model_exhausted_' + cbName` | `MAX_BUTTONS = 24`; `cbName = mName.slice(0, 48)` (máximo 64 bytes) | `src/ui/modelsUi.ts` |
| `model_refresh_btn` | `'model_refresh_btn'` | Fixo (17 bytes) | `src/ui/modelsUi.ts` |
| `PLAN_VIEW_BTN` | `'plan_view_btn'` | `${PLAN_VIEW_BTN}:${projectName}:${targetChannelStr}` | `src/ui/planUi.ts` |
| `PLAN_PROCEED_BTN` | `'plan_proceed_btn'` | `${PLAN_PROCEED_BTN}:${projectName}:${targetChannelStr}` | `src/ui/planUi.ts` |
| `PLAN_EDIT_BTN` | `'plan_edit_btn'` | `${PLAN_EDIT_BTN}:${projectName}:${targetChannelStr}` | `src/ui/planUi.ts` |
| `PLAN_REFRESH_BTN` | `'plan_refresh_btn'` | `${PLAN_REFRESH_BTN}:${projectName}:${targetChannelStr}` | `src/ui/planUi.ts` |
| `PLAN_PAGE_PREFIX` | `'plan_page'` | `${PLAN_PAGE_PREFIX}:${page}:${projectName}:${targetChannelStr}` | `src/ui/planUi.ts` |
| `PROJECT_SELECT_ID` | `'project_select'` | `${PROJECT_SELECT_ID}:${ws}`; `ITEMS_PER_PAGE = 100` | `src/ui/projectListUi.ts` |
| `WORKSPACE_SELECT_ID` | `'workspace_select'` | Alias aceito em `isProjectSelectId` | `src/ui/projectListUi.ts` |
| `PROJECT_PAGE_PREFIX` | `'project_page'` | `${PROJECT_PAGE_PREFIX}:${page}` | `src/ui/projectListUi.ts` |
| `SESSION_SELECT_ID` | `'session_select'` | `${SESSION_SELECT_ID}:${session.title.slice(0, 49)}`; `MAX_SELECT_OPTIONS = 25` | `src/ui/sessionPickerUi.ts` |
| `TEMPLATE_BTN_PREFIX` | `'template_btn_'` | `${TEMPLATE_BTN_PREFIX}${tpl.id}`; `MAX_BUTTONS = 25`; `MAX_PROMPT_PREVIEW_LEN = 60` | `src/ui/templateUi.ts` |

---

### 3.2. Templates HTML e Rótulos de Botões Literais por Componente

#### A. `autoAcceptUi.ts`
- **Texto HTML**:
  ```html
  <b>Auto-accept Management</b>

  <b>Current Status:</b> {enabled ? '🟢 ON' : '⚪ OFF'}

  ON: approval dialogs are automatically allowed.
  OFF: approval dialogs require manual action.
  ```
- **Teclado Inline** (2 linhas):
  - Linha 1, Botão 1: Rótulo `enabled ? '✅ Turn ON' : 'Turn ON'`, callback `autoaccept_btn_on`
  - Linha 1, Botão 2: Rótulo `!enabled ? '🔴 Turn OFF' : 'Turn OFF'`, callback `autoaccept_btn_off`
  - Linha 2, Botão 1: Rótulo `'🔄 Refresh'`, callback `autoaccept_btn_refresh`

#### B. `modeUi.ts`
- **Constantes de `modeService.ts` utilizadas**:
  - `AVAILABLE_MODES = ['fast', 'plan']`
  - `MODE_DISPLAY_NAMES = { fast: 'Fast', plan: 'Planning' }`
  - `MODE_DESCRIPTIONS = { fast: 'Agent will execute tasks directly. Use for simple tasks that can be completed faster', plan: 'Agent can plan before executing tasks. Use for deep research, complex tasks, or collaborative work' }`
- **Formato de cada linha em `modeLines`** (unidas por `'\n'`):
  ```html
  {m === currentMode ? '✅' : '⬜'} <b>{escapeHtml(MODE_DISPLAY_NAMES[m] || m)}</b> — {escapeHtml(MODE_DESCRIPTIONS[m] || '')}
  ```
- **Texto HTML Completo**:
  ```html
  <b>Mode Management</b>

  <b>Current Mode:</b> {escapeHtml(MODE_DISPLAY_NAMES[currentMode] || currentMode)}
  {escapeHtml(MODE_DESCRIPTIONS[currentMode] || '')}

  <b>Available Modes ({AVAILABLE_MODES.length})</b>
  {modeLines}
  ```
- **Teclado Inline** (1 botão por linha para cada `m` em `AVAILABLE_MODES`):
  - Rótulo: `m === currentMode ? "✅ " + (MODE_DISPLAY_NAMES[m] || m) : (MODE_DISPLAY_NAMES[m] || m)`
  - Callback: `mode_select:${m}`

#### C. `modelsUi.ts`
- **Mensagens de Erro em `sendModelsUI`**:
  - Quando `cdp == null`: `'Not connected to CDP.'` (com teclado vazio `new InlineKeyboard()`)
  - Quando `payload == null` (`models.length === 0`): `'Failed to retrieve model list from Antigravity.'` (com teclado vazio `new InlineKeyboard()`)
- **Formatação de cada modelo (`formatQuota(mName, current)`)**:
  - Se `mName == ""`: `${current ? '✅' : '⬜'} Unknown`
  - Se não houver entrada correspondente em `quotaData` ou `!q.quotaInfo`: `${current ? '✅' : '⬜'} ${mName}`
  - Cálculo de `timeStr`:
    - `diffMs = resetTime ? resetTime.getTime() - Date.now() : 0`
    - Se `diffMs <= 0`: `timeStr = 'Ready'`
    - Se `diffMs > 0`: `mins = Math.ceil(diffMs / 60000)`
      - Se `mins < 60`: `timeStr = "${mins}m"`
      - Caso contrário: `timeStr = "${Math.floor(mins / 60)}h ${mins % 60}m"`
  - Se `rem !== undefined && rem !== null && !isNaN(rem)`:
    - `percent = Math.round(rem * 100)`
    - Ícone (`icon`):
      - `percent <= 0`: `'⛔'`
      - `percent <= 20`: `'🔴'`
      - `percent <= 50`: `'🟡'`
      - `percent > 50`: `'🟢'`
    - `quotaStr = percent <= 0 ? 'Exhausted' : "${percent}%"`
    - Retorno: `${current ? '✅' : '⬜'} ${mName} ${icon} ${quotaStr} (⏱️ ${timeStr})`
  - Caso `rem` seja `undefined`, `null` ou `NaN`:
    - Retorno: `${current ? '✅' : '⬜'} ${mName} ❓ N/A (⏱️ ${timeStr})`
- **Texto HTML Completo**:
  ```html
  <b>Model Management</b>

  <b>Current Model:</b>
  {escapeHtml(currentModel ? formatQuota(currentModel, true) : 'Unknown')}

  <b>Available Models ({models.length})</b>
  {models.map(m => escapeHtml(formatQuota(m, m === currentModel))).join('\n')}
  ```
- **Teclado Inline**:
  - Para cada `mName` em `models.slice(0, 24)` (1 botão por linha):
    - `safeName = mName.length > 40 ? mName.substring(0, 37) + '...' : mName`
    - `label = exhausted ? "⛔ " + safeName : safeName`
    - `maxNameLen = 64 - 'model_exhausted_'.length` (`48`)
    - `cbName = mName.length > maxNameLen ? mName.substring(0, maxNameLen) : mName`
    - `cbData = exhausted ? "model_exhausted_" + cbName : "model_btn_" + cbName`
  - Última linha: Rótulo `'🔄 Refresh'`, callback `'model_refresh_btn'`

#### D. `planUi.ts`
- **Card de Notificação (`buildPlanNotificationUI`)**:
  - `description = info.description || info.planSummary || 'A plan has been generated and is awaiting your review.'`
  - `planTitle = info.planTitle || 'Implementation Plan'`
  - **Texto HTML**:
    ```html
    📋 <b>Planning Mode</b>

    {escapeHtml(description)}

    <b>Plan:</b> {escapeHtml(planTitle)}
    <b>Workspace:</b> {escapeHtml(projectName)}
    ```
  - **Teclado Inline** (`suffix = "${projectName}:${targetChannelStr}"`):
    - Linha 1: `[📖 View Full Plan -> plan_view_btn:${suffix}]`, `[▶ Proceed -> plan_proceed_btn:${suffix}]`
    - Linha 2: `[✏️ Edit Plan -> plan_edit_btn:${suffix}]`, `[🔄 Refresh -> plan_refresh_btn:${suffix}]`
- **Card de Conteúdo Paginado (`buildPlanContentUI`)**:
  - `page = pages[currentPage] || '(Page not found)'`
  - **Texto HTML**:
    - Se `totalPages > 1`: `<b>📋 Plan Content</b> (${currentPage + 1}/${totalPages})\n\n${markdownToTelegramHtml(page)}`
    - Se `totalPages <= 1`: `<b>📋 Plan Content</b>\n\n${markdownToTelegramHtml(page)}`
  - **Teclado Inline** (`suffix = "${projectName}:${targetChannelStr}"`):
    - Se `totalPages > 1`:
      - Se `currentPage > 0`: adiciona botão `'◀ Prev'` com callback `plan_page:${currentPage - 1}:${suffix}`
      - Se `currentPage < totalPages - 1`: adiciona botão `'Next ▶'` com callback `plan_page:${currentPage + 1}:${suffix}`
      - Finaliza linha (`row()`)
    - Linha de ações 1: `[▶ Proceed -> plan_proceed_btn:${suffix}]`, `[✏️ Edit -> plan_edit_btn:${suffix}]`
    - Linha de ações 2: `[🔄 Refresh -> plan_refresh_btn:${suffix}]`, `[📖 View Full -> plan_view_btn:${suffix}]`

#### E. `projectListUi.ts`
- **Estado Vazio (`workspaces.length === 0`)**:
  - **Texto HTML**:
    ```html
    <b>📁 Dự án</b>

    Không tìm thấy dự án nào.
    ```
  - **Teclado Inline**: vazio (`new InlineKeyboard()`).
- **Estado com Projetos**:
  - Linhas numeradas (`start = safePage * 100`): `${start + i + 1}. ${escapeHtml(ws)}`
  - **Texto HTML Base**:
    ```html
    <b>📁 Dự án</b>

    Chọn một dự án để tự động tạo topic và phiên làm việc

    {lines.join('\n')}
    ```
  - **Sufixo de Paginação** (adicionado apenas quando `totalPages > 1`):
    ```html


    <i>Trang {safePage + 1} / {totalPages} ({workspaces.length} tổng số dự án)</i>
    ```
  - **Teclado Inline**:
    - Para cada `ws` em `pageItems` (1 por linha):
      - `label = ws.length > 40 ? ws.substring(0, 37) + '...' : ws`
      - Callback: `project_select:${ws}`
    - Se `totalPages > 1`:
      - Se `safePage > 0`: botão `'◀ Trước'` com callback `project_page:${safePage - 1}`
      - Se `safePage < totalPages - 1`: botão `'Tiếp ▶'` com callback `project_page:${safePage + 1}`
      - Finaliza linha (`row()`)

#### F. `screenshotUi.ts`
- Quando `cdp === null`: envia texto `'Not connected to Antigravity.'`
- Quando `result.success && result.buffer`: envia foto com nome de arquivo `'screenshot.png'` e caption `'📸 Screenshot'`
- Quando `!result.success || !result.buffer`: envia texto `` `Screenshot failed: ${result.error}` ``
- Quando ocorre exceção: envia texto `` `Screenshot error: ${e.message}` ``

#### G. `sessionPickerUi.ts`
- **Estado Vazio (`sessions.length === 0`)**:
  - **Texto HTML**:
    ```html
    <b>🔗 Tham gia phiên</b>

    Không tìm thấy phiên nào trong bảng điều khiển của Antigravity.
    ```
  - **Teclado Inline**: vazio.
- **Estado com Sessões (`sessions.length > 0`)**:
  - **Texto HTML**:
    ```html
    <b>🔗 Tham gia phiên</b>

    Chọn một phiên để tham gia (tìm thấy {sessions.length} phiên)
    ```
  - **Teclado Inline** (`sessions.slice(0, 25)`, 1 botão por linha):
    - `label = session.isActive ? "✅ " + session.title.slice(0, 40) : session.title.slice(0, 40)`
    - Callback: `session_select:${session.title.slice(0, 49)}`

#### H. `templateUi.ts`
- **Estado Vazio (`templates.length === 0`)**:
  - **Texto HTML**:
    ```html
    <b>Template Management</b>

    No templates registered.

    Use /template_add to add one.
    ```
  - **Teclado Inline**: vazio.
- **Estado com Templates (`templates.length > 0`)**:
  - `truncate(text, 60)`: se `text.length > 60`, retorna `${text.substring(0, 57)}...`; caso contrário `text`.
  - `displayTemplates = templates.slice(0, 25)`
  - `hasMore = templates.length > 25`
  - `description`: cada item formatado como `<b>${i + 1}. ${escapeHtml(tpl.name)}</b>\n  ${escapeHtml(truncate(tpl.prompt, 60))}` e unido por `'\n\n'`.
  - `footerText`:
    - Se `hasMore`: `\n\n${templates.length - 25} templates are hidden.`
    - Caso contrário: `\n\nTap a button to execute the template.`
  - **Texto HTML Completo**:
    ```html
    <b>Template Management</b>

    <b>Registered Templates ({templates.length})</b>

    {description}{footerText}
    ```
  - **Teclado Inline** (para cada `tpl` em `displayTemplates`, 1 por linha):
    - `safeLabel = tpl.name.length > 40 ? "${tpl.name.substring(0, 37)}..." : tpl.name`
    - Rótulo: `▶ ${safeLabel}`
    - Callback: `template_btn_${tpl.id}`

#### I. `telegramTopicManager.ts`
- Prefixo de tópico de workspace em `ensureTopic`: `` `🗂️ ${workspaceName}` ``
- Prefixo de tópico de sessão em `createSessionTopic`: `` `💬 ${sessionName}` ``

---

### 3.3. Expressões Regulares Literais

1. **Normalização de Nome de Modelo (`src/ui/modelsUi.ts`, linhas 29 e 73)**:
   - Regex JS: `/[\s\-_]/g` (após `.toLowerCase()`)
   - Em Go: remover espaços em branco, `-` e `_` após `strings.ToLower(s)`.
2. **Conversor `markdownToTelegramHtml` (`src/ui/planUi.ts`, linhas 18–69)**:
   - Cabeçalhos (linha 44): `/^(#{1,4})\s+(.*)/` $\rightarrow$ `<b>$2</b>`
   - Negrito (linha 51): `/\*\*(.+?)\*\*/g` $\rightarrow$ `<b>$1</b>`
   - Itálico (linha 53): `/(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)/g` $\rightarrow$ `<i>$1</i>` (em Go, como `regexp` RE2 não suporta lookbehind/lookahead, implementar função equivalente que converte `*texto*` simples após os `**texto**` já terem sido substituídos por `<b>texto</b>`)
   - Código inline (linha 55): `` /`([^`]+)`/g `` $\rightarrow$ `<code>$1</code>`
   - Item de lista (linha 57): `/^(\s*)[-*]\s+/` $\rightarrow$ `$1• `
3. **Sanitização de Tópico (`src/services/telegramTopicManager.ts`, linhas 99–110)**:
   - Remover barras finais: `/\/+$/` $\rightarrow$ `""`
   - Trocar barras por hífens: `/\//g` $\rightarrow$ `"-"`
   - Colapsar múltiplos hífens: `/-{2,}/g` $\rightarrow$ `"-"`
   - Remover hífens nas extremidades: `/^-+|-+$/g` $\rightarrow$ `""`
   - Truncar em `128` caracteres (`sanitized.substring(0, 128)`).

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Sincronização Ao Vivo em `BuildModeUI` (`mode_ui.go`)
1. Se `deps != nil && deps.GetCurrentCdp != nil`:
   - Obtém `cdp := deps.GetCurrentCdp()`.
   - Se `cdp != nil`, chama `liveMode, err := cdp.GetCurrentMode(ctx)`.
   - Se `err == nil && liveMode != ""`, atualiza o serviço local com `modeService.SetMode(liveMode)`.
2. Lê `currentMode := modeService.GetCurrentMode()`.
3. Itera sobre `AVAILABLE_MODES` (`["fast", "plan"]`) construindo `modeLines` e os botões `mode_select:fast` e `mode_select:plan`.

### 4.2. Matching Fuzzy de Cota e Exaustão em `BuildModelsUI` (`models_ui.go`)
1. Chama `models := cdp.GetUiModels()`, `currentModel := cdp.GetCurrentModel()` e `quotaData := fetchQuota()`.
2. Se `len(models) == 0`, retorna `nil`.
3. Para associar um modelo `mName` a um item `q` de `quotaData`:
   - Normaliza strings com `normalize(s) = strings.ToLower(re.ReplaceAllString(s, ""))` onde `re = [\s\-_]`.
   - Encontra o primeiro item `q` em `quotaData` que satisfaça qualquer uma das condições:
     - `nLabel == nName`
     - `nModel == nName`
     - `nLabel != ""` e (`strings.Contains(nName, nLabel)` ou `strings.Contains(nLabel, nName)`)
     - `nModel != ""` e (`strings.Contains(nName, nModel)` ou `strings.Contains(nModel, nName)`)
4. Um modelo é considerado esgotado (`isExhausted(mName) == true`) se, e somente se, houver item `q` correspondente com `q.QuotaInfo != nil`, `q.QuotaInfo.RemainingFraction != nil`, `!math.IsNaN(*q.QuotaInfo.RemainingFraction)` e `*q.QuotaInfo.RemainingFraction <= 0`.
5. Na construção do teclado:
   - Limita a lista a no máximo `MAX_BUTTONS = 24` modelos (`models[:min(len(models), 24)]`).
   - Trunca o texto visível do botão (`safeName`) se `len(mName) > 40` para `mName[:37] + "..."`.
   - Se `exhausted == true`, prefixa o rótulo com `"⛔ "` e usa o callback `"model_exhausted_" + cbName`; caso contrário, usa `safeName` e `"model_btn_" + cbName`, onde `cbName` é truncado em `48` caracteres (`64 - len("model_exhausted_")`).
   - Adiciona ao final uma linha extra com o botão `'🔄 Refresh'` (`callback_data = "model_refresh_btn"`).

### 4.3. Conversão Markdown $\rightarrow$ HTML e Paginação de Planos (`plan_ui.go`)
1. **`MarkdownToTelegramHTML(md)`**:
   - Divide `md` por `'\n'` e mantém uma flag booleana `inCodeBlock = false`.
   - Para cada linha `line`:
     - Se `strings.HasPrefix(strings.TrimLeft(line, " \t"), "```")`:
       - Se `inCodeBlock == true`: adiciona `"</pre>"` à saída e define `inCodeBlock = false`.
       - Se `inCodeBlock == false`: adiciona `"<pre>"` à saída e define `inCodeBlock = true`.
       - Continua para a próxima linha.
     - Se `inCodeBlock == true`: adiciona `EscapeHTML(line)` sem processar formatação inline e continua.
     - Caso contrário (`inCodeBlock == false`):
       - Faz `converted := EscapeHTML(line)`.
       - Se casar com `^(#{1,4})\s+(.*)`: substitui toda a linha por `<b>` + grupo 2 + `</b>` e continua.
       - Aplica substituição de negrito `**(.+?)**` $\rightarrow$ `<b>$1</b>`.
       - Aplica substituição de itálico `*([^*\n]+)*` $\rightarrow$ `<i>$1</i>`.
       - Aplica substituição de código inline `` `([^`]+)` `` $\rightarrow$ `<code>$1</code>`.
       - Se o início da linha casar com `^(\s*)[-*]\s+`: substitui o marcador por `$1• `.
       - Adiciona `converted` à saída.
   - Ao terminar o loop, se `inCodeBlock == true`, adiciona `"</pre>"` para fechar bloco pendente.
   - Une todas as linhas com `"\n"`.
2. **`PaginatePlanContent(content, pageSize)`**:
   - Se `strings.TrimSpace(content) == ""`, retorna `[]string{"(Empty plan)"}`.
   - Se `len(content) <= pageSize`, retorna `[]string{content}`.
   - Enquanto `len(remaining) > 0`:
     - Se `len(remaining) <= pageSize`, adiciona `remaining` em `pages` e encerra.
     - Busca o último `'\n'` dentro de `remaining[:pageSize+1]` (`strings.LastIndexByte(remaining[:pageSize+1], '\n')`).
     - Se `splitAt <= 0`, faz corte rígido em `splitAt = pageSize`.
     - Adiciona `remaining[:splitAt]` em `pages`.
     - Atualiza `remaining = strings.TrimPrefix(remaining[splitAt:], "\n")`.

### 4.4. Paginação e Clamping em `BuildProjectListUI` (`project_ui.go`)
1. Calcula `totalPages = max(1, int(math.Ceil(float64(len(workspaces)) / 100.0)))`.
2. Calcula `safePage = max(0, min(page, totalPages - 1))`.
3. Se `len(workspaces) == 0`, retorna o texto vietnamita de lista vazia e teclado vazio.
4. Fatia `pageItems = workspaces[start:end]` onde `start = safePage * 100` e `end = min(start + 100, len(workspaces))`.
5. Numera cada item como `${start + i + 1}. ${escapeHtml(ws)}`.
6. Se `totalPages > 1`, anexa `\n\n<i>Trang ${safePage + 1} / ${totalPages} (${workspaces.length} tổng số dự án)</i>` e adiciona na última linha do teclado os botões `◀ Trước` (se `safePage > 0`) e `Tiếp ▶` (se `safePage < totalPages - 1`).

### 4.5. Gerenciamento de Tópicos de Fórum (`topic_manager.go`)
1. **`EnsureTopic(ctx, workspaceName)`**:
   - Consulta `topicCache[workspaceName]`. Se existir e for `!= 0`, retorna `EnsureTopicResult{TopicID: cached, Created: false}`.
   - Caso contrário, invoca `api.CreateForumTopic(ctx, chatID, "🗂️ " + workspaceName)`.
   - Em caso de sucesso, grava `topicCache[workspaceName] = topic.MessageThreadID` e retorna `EnsureTopicResult{TopicID: topic.MessageThreadID, Created: true}`.
   - Em caso de erro, registra log `ERROR` (`[TopicManager] Failed to create topic for "<workspaceName>": ...`) e retorna o erro.
2. **`CreateSessionTopic(ctx, sessionName)`**:
   - Invoca `api.CreateForumTopic(ctx, chatID, "💬 " + sessionName)` (não armazena em `topicCache`).
   - Em caso de erro, registra log `ERROR` e retorna o erro.
3. **`RenameTopic`, `CloseTopic`, `DeleteTopic`**:
   - Invocam respectivamente `EditForumTopic`, `CloseForumTopic` e `DeleteForumTopic`.
   - Qualquer erro retornado pela API do Telegram é capturado e registrado via `logger.Error(...)`, sem interromper o fluxo chamador.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Funções Puras de Construção de UI (`internal/bot/ui`)**:
   - `BuildAutoAcceptUI`, `BuildPlanNotificationUI`, `PaginatePlanContent`, `BuildPlanContentUI`, `BuildProjectListUI`, `BuildSessionPickerUI` e `BuildTemplateUI` são funções puras, determinísticas e sem estado compartilhado, podendo ser invocadas concorrentemente por múltiplas goroutines sem necessidade de locks.
2. **Thread-Safety em `TelegramTopicManager` (`internal/bot/topic_manager.go`)**:
   - No TypeScript (single-threaded no event loop), `topicCache` (`Map<string, number>`) e `chatId` eram acessados sem mutex.
   - Em Go, como múltiplos callbacks e mensagens podem acionar `EnsureTopic`, `SetChatID`, `RegisterTopic` e `GetTopicID` concorrentemente, `TelegramTopicManager` protege `chatID` e `topicCache` com `sync.RWMutex` (`mu`).
   - Leituras de cache (`GetTopicID` e verificação inicial em `EnsureTopic`) usam `RLock`/`RUnlock`; mutações (`SetChatID`, `RegisterTopic` e atualização do cache após `CreateForumTopic`) usam `Lock`/`Unlock`. Nota: a chamada de rede `api.CreateForumTopic` deve ocorrer fora do lock para não bloquear outras goroutines durante I/O HTTP.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os cenários dos 8 arquivos de teste originais em `tests/ui/*.test.ts` mais os testes de `TelegramTopicManager` devem ser implementados e passar com `go test -race ./internal/bot/...`:

### 6.1. `internal/bot/ui/autoaccept_ui_test.go` (espelhando `tests/ui/autoAcceptUi.test.ts`)
- `TestSendAutoAcceptUI_Disabled`: verifica que quando `IsEnabled() == false`, `sendFn` é chamado 1 vez, o texto contém `"Auto-accept Management"` e `"⚪ OFF"`, o botão ON tem texto `"Turn ON"` e o botão OFF tem texto `"🔴 Turn OFF"`.
- `TestSendAutoAcceptUI_Enabled`: verifica que quando `IsEnabled() == true`, o texto contém `"🟢 ON"`, o botão ON tem texto `"✅ Turn ON"` e o botão OFF tem texto `"Turn OFF"`.

### 6.2. `internal/bot/ui/mode_ui_test.go` (espelhando `tests/ui/modeUi.test.ts`)
- `TestSendModeUI_DefaultFast`: verifica envio de texto contendo `"Mode Management"` e teclado com `mode_select:fast` (`✅ Fast`) e `mode_select:plan` (`Planning`).
- `TestSendModeUI_SyncsFromCdp`: quando `deps.GetCurrentCdp` retorna mock cujo `GetCurrentMode` retorna `"plan"`, verifica que `modeService.GetCurrentMode()` passa a ser `"plan"`.
- `TestSendModeUI_DoesNotSyncWhenCdpReturnsEmpty`: quando `GetCurrentMode` retorna `""`, mantém `"fast"`.
- `TestSendModeUI_NilDeps`: funciona sem `deps` (`deps == nil`).
- `TestSendModeUI_NilCdpInstance`: funciona quando `deps.GetCurrentCdp()` retorna `nil`.

### 6.3. `internal/bot/ui/models_ui_test.go` (espelhando `tests/ui/modelsUi.test.ts`)
- `TestSendModelsUI_NotConnected`: quando `GetCurrentCdp()` retorna `nil`, chama `sendFn` com `"Not connected to CDP."` e teclado vazio.
- `TestSendModelsUI_ModelsAvailable`: envia texto contendo `"Model Management"`, `"Model A"`, `"Model B"` e teclado inline.
- `TestBuildModelsUI_EmptyModels`: retorna `nil` quando `GetUiModels()` retorna slice vazio, e `SendModelsUI` envia `"Failed to retrieve model list from Antigravity."`.
- `TestBuildModelsUI_ExhaustedStatus`: quando `RemainingFraction == 0`, exibe `"Exhausted"`, `"⛔"` e não exibe `"100%"`.
- `TestBuildModelsUI_PercentageStatus`: quando `RemainingFraction == 0.6`, exibe `"60%"` e `"🟢"`; testa também faixas `<= 20%` (`🔴`) e `<= 50%` (`🟡`).
- `TestBuildModelsUI_NaNRemainingFraction`: quando `RemainingFraction` é `NaN` ou `nil`, exibe `"N/A"` e `"❓"`.
- `TestBuildModelsUI_ExhaustedCallbackPrefix`: modelos saudáveis geram callback `"model_btn_<name>"`, modelos esgotados geram `"model_exhausted_<name>"` e rótulo `"⛔ <name>"`.

### 6.4. `internal/bot/ui/plan_ui_test.go` (espelhando `tests/ui/planUi.test.ts`)
- `TestBuildPlanNotificationUI_AllFourButtons`: verifica presença de `"Planning Mode"`, `planTitle`, `projectName`, `description` e 2 linhas de 2 botões (`plan_view_btn`, `plan_proceed_btn`, `plan_edit_btn`, `plan_refresh_btn`).
- `TestBuildPlanNotificationUI_Fallbacks`: fallback para `planSummary` quando `description` é vazio; fallback para `"A plan has been generated and is awaiting your review."` quando ambos são vazios.
- `TestPaginatePlanContent_Short`: retorna 1 página para conteúdo curto.
- `TestPaginatePlanContent_Empty`: retorna `["(Empty plan)"]` para string vazia ou só espaços.
- `TestPaginatePlanContent_NewlineBoundaries`: divide conteúdo longo respeitando `'\n'` com cada página `<= pageSize`.
- `TestPaginatePlanContent_HardSplit`: divide exatamente em `pageSize` quando não há `'\n'`.
- `TestBuildPlanContentUI_SinglePage`: não exibe `(1/1)` no título e não inclui botões `Prev`/`Next`, exibindo apenas os 4 botões de ação.
- `TestBuildPlanContentUI_MultiPage`: página 0 exibe `(1/3)` e apenas `Next ▶`; página 1 exibe `(2/3)`, `◀ Prev` e `Next ▶`; página 2 exibe `(3/3)` e apenas `◀ Prev`.

### 6.5. `internal/bot/ui/project_ui_test.go` (espelhando `tests/ui/projectListUi.test.ts`)
- `TestParseProjectPageID`: testa `"project_page:0"` $\rightarrow$ `(0, true)`, `"project_page:3"` $\rightarrow$ `(3, true)`, `"project_page:99"` $\rightarrow$ `(99, true)`, e entradas inválidas (`"other_id"`, `"project_page"`, `""`, `"project_page:abc"`) $\rightarrow$ `(_, false)`.
- `TestIsProjectSelectID`: reconhece `"project_select"`, `"workspace_select"`, `"project_select:my-project"`, e rejeita `"mode_select"`, `"project_page:0"`, `""`.
- `TestBuildProjectListUI`: testa lista vazia (`"Không tìm thấy dự án nào"`), página única sem `"Trang"`, múltiplas páginas (`ITEMS_PER_PAGE + 5` itens $\rightarrow$ `"Trang 1 / 2"` e `"Trang 2 / 2"`), clamping de página acima do limite (`100` $\rightarrow$ página 2) e página negativa (`-5` $\rightarrow$ página 1), e exatamente `ITEMS_PER_PAGE` itens (página única).

### 6.6. `internal/bot/ui/screenshot_ui_test.go` (espelhando `tests/ui/screenshotUi.test.ts`)
- `TestHandleScreenshot_NilCdp`: envia `"Not connected to Antigravity."` via `sendText`.
- `TestHandleScreenshot_Success`: envia `screenshot.png` com caption `"📸 Screenshot"` via `sendPhoto`.
- `TestHandleScreenshot_CaptureFailed`: envia `"Screenshot failed: No page found"` via `sendText`.
- `TestHandleScreenshot_CaptureError`: envia `"Screenshot error: CDP timeout"` via `sendText`.

### 6.7. `internal/bot/ui/session_ui_test.go` (espelhando `tests/ui/sessionPickerUi.test.ts`)
- `TestIsSessionSelectID`: reconhece `"session_select:some_session"` e `"session_select"`; rejeita `"project_select"`, `"mode_select"`, `""`.
- `TestBuildSessionPickerUI`: testa lista com 3 sessões (verifica texto contendo `"3"` e prefixo `"✅ "` na sessão ativa), lista vazia (`"Không tìm thấy phiên nào"`), e truncamento em 25 itens quando recebe 30 sessões.

### 6.8. `internal/bot/ui/template_ui_test.go` (espelhando `tests/ui/templateUi.test.ts`)
- `TestSendTemplateUI_Empty`: exibe `"No templates registered"` e `"/template_add"`.
- `TestSendTemplateUI_WithTemplates`: exibe `"Template Management"`, nomes dos templates, `"(2)"` e botões `"▶ <name>"`.
- `TestSendTemplateUI_TruncatesLongPrompts`: trunca prompts com mais de 60 caracteres adicionando `"..."`.
- `TestSendTemplateUI_CapsAt25Buttons`: para 30 templates, gera 25 botões e exibe `"5 templates are hidden."`.
- `TestParseTemplateButtonID`: `"template_btn_42"` $\rightarrow$ `(42, true)`, `"model_btn_foo"` $\rightarrow$ `(_, false)`, `"template_btn_abc"` $\rightarrow$ `(_, false)`.

### 6.9. `internal/bot/topic_manager_test.go`
- `TestSanitizeName`: remove `/` finais, converte `/` internos em `-`, colapsa `---` em `-`, remove `-` nas extremidades e trunca nomes maiores que 128 caracteres.
- `TestEnsureTopic_CacheHitAndMiss`: cria tópico `"🗂️ my-repo"` na primeira chamada (`Created: true`) e retorna do cache na segunda (`Created: false`).
- `TestCreateSessionTopic`: cria tópico `"💬 1-fix-bug"` e retorna `message_thread_id`.
