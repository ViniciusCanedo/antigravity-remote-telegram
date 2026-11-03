# 15 — Núcleo do Bot do Telegram, Middleware de Autenticação e Comandos

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica descreve de forma 100% autocontida e exaustiva a camada central do Bot do Telegram, o middleware de autenticação, o parser de comandos em texto/voz, os 6 handlers de comando de `src/commands/*.ts` e os 16 comandos nativos registrados em `src/bot/index.ts` (mais o comando `/join` e a integração do seletor de sessões em `/chat` definidos no ADR-005).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript) | Linhas | Arquivo de Teste Original (Jest) | Linhas | Destino no Port Go |
| :--- | :---: | :--- | :---: | :--- |
| [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts) (linhas 1–141, 861–1270, 1899–1938) | 1938 | *(testado via handlers e integração)* | — | `internal/bot/bot.go`<br>`internal/bot/handlers_chat.go`<br>`internal/bot/handlers_project.go`<br>`internal/bot/handlers_session.go`<br>`internal/bot/handlers_template.go`<br>`internal/bot/handlers_cleanup.go` |
| [`src/middleware/auth.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/middleware/auth.ts) | 9 | [`tests/middleware/auth.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/middleware/auth.test.ts) | 21 | `internal/bot/middleware.go`<br>`internal/bot/middleware_test.go` |
| [`src/commands/messageParser.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/messageParser.ts) | 37 | [`tests/commands/messageParser.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/commands/messageParser.test.ts) | 43 | `internal/bot/message_parser.go`<br>`internal/bot/message_parser_test.go` |
| [`src/commands/chatCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/chatCommandHandler.ts) | 40 | [`tests/commands/chatCommandHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/commands/chatCommandHandler.test.ts) | 63 | `internal/bot/handlers_chat.go`<br>`internal/bot/handlers_chat_test.go` |
| [`src/commands/workspaceCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/workspaceCommandHandler.ts) | 32 | [`tests/commands/workspaceCommandHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/commands/workspaceCommandHandler.test.ts) | 41 | `internal/bot/handlers_project.go`<br>`internal/bot/handlers_project_test.go` |
| [`src/commands/joinCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/joinCommandHandler.ts) | 36 | [`tests/commands/joinCommandHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/commands/joinCommandHandler.test.ts) | 28 | `internal/bot/handlers_session.go`<br>`internal/bot/handlers_session_test.go` |
| [`src/commands/slashCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/slashCommandHandler.ts) | 123 | [`tests/commands/slashCommandHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/commands/slashCommandHandler.test.ts) | 106 | `internal/bot/handlers_template.go`<br>`internal/bot/handlers_template_test.go` |
| [`src/commands/cleanupCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/cleanupCommandHandler.ts) | 53 | [`tests/commands/cleanupCommandHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/commands/cleanupCommandHandler.test.ts) | 43 | `internal/bot/handlers_cleanup.go`<br>`internal/bot/handlers_cleanup_test.go` |

### 1.2. Documentação Arquitetural e ADRs Relacionados
- [`docs/07-bot-telegram-comandos-e-ui.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/07-bot-telegram-comandos-e-ui.md)
- [`docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md):
  - **ADR-005**: Exposição do seletor interativo `BuildSessionPickerUI` em `/chat` e `/join`.
  - **ADR-006**: Unificação de `src/middleware/auth.ts` (`withAuth`) com o middleware do bot em `internal/bot/middleware.go`.

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Núcleo do Bot e Estado Global (`internal/bot/bot.go`)

```go
package bot

import (
	"context"
	"database/sql"
	"sync"

	"antigravity-telegram-remote/internal/bot/ui"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/config"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/services"
	"antigravity-telegram-remote/internal/utils"
)

// TelegramChannel identifica univocamente um chat ou tópico de fórum no Telegram.
type TelegramChannel struct {
	ChatID   int64
	ThreadID int // 0 quando não está em um tópico de fórum
}

// PlanEditPendingState armazena o estado de um canal aguardando instruções de edição de plano.
type PlanEditPendingState struct {
	ProjectName string
}

// BotCommandEntry representa um item enviado para setMyCommands.
type BotCommandEntry struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ResolvedWorkspaceCdp encapsula o retorno de ResolveWorkspaceAndCdp.
type ResolvedWorkspaceCdp struct {
	Cdp           *cdp.CdpService
	ProjectName   string
	WorkspacePath string
}

// Bot encapsula todas as dependências, repositórios, serviços e estados em memória do bot Telegram.
type Bot struct {
	Config               *config.AppConfig
	DB                   *sql.DB
	Bridge               *services.CdpBridge
	ModeService          *services.ModeService
	ModelService         *services.ModelService
	TemplateRepo         *database.TemplateRepository
	WorkspaceBindingRepo *database.WorkspaceBindingRepository
	ChatSessionRepo      *database.ChatSessionRepository
	WorkspaceService     *services.WorkspaceService
	ChatSessionService   *services.ChatSessionService
	TitleGenerator       *services.TitleGeneratorService
	PromptDispatcher     *services.PromptDispatcher
	TopicManager         *TelegramTopicManager
	SlashCommandHandler  *SlashCommandHandler
	CleanupHandler       *CleanupCommandHandler
	ChatHandler          *ChatCommandHandler
	WorkspaceHandler     *WorkspaceCommandHandler
	JoinHandler          *JoinCommandHandler

	// Estados concorrentes em memória (equivalentes aos Sets/Maps de nível de módulo em src/bot/index.ts)
	stateMu                  sync.RWMutex
	userStopRequested        map[string]struct{}             // Set<channelKey>
	planEditPendingChannels  map[string]PlanEditPendingState // Map<channelKey, { projectName }>
	planContentCache         map[string][]string             // Map<channelKey, pages[]>
}

// StartBot inicializa configuração, SQLite WAL, launcher do Antigravity, CdpBridge, handlers e inicia o long-polling.
func StartBot(ctx context.Context, cliLogLevel *utils.LogLevel) error

// ChannelKey formata a chave composta "<chatId>:<threadId>" (se ThreadID != 0) ou "<chatId>".
func ChannelKey(ch TelegramChannel) string

// ParseChannelKey decompõe "<chatId>:<threadId>" ou "<chatId>" de volta em TelegramChannel.
func ParseChannelKey(key string) (TelegramChannel, error)

// GetChannelFromMessage extrai TelegramChannel de uma mensagem recebida.
func GetChannelFromMessage(chatID int64, messageThreadID int) TelegramChannel

// GetChannelFromCallback extrai TelegramChannel de um callback_query recebido.
func GetChannelFromCallback(chatID int64, messageThreadID int) TelegramChannel

// ResolveWorkspaceAndCdp localiza o vínculo do canal no SQLite, conecta ao CDP do projeto e inicializa os detectores.
func (b *Bot) ResolveWorkspaceAndCdp(ctx context.Context, ch TelegramChannel) *ResolvedWorkspaceCdp

// ReplyHTML envia uma resposta em modo HTML no mesmo chat/tópico da mensagem original.
func (b *Bot) ReplyHTML(ctx context.Context, ch TelegramChannel, text string, keyboard *ui.InlineKeyboardMarkup) error
```

---

### 2.2. Middleware de Autorização (`internal/bot/middleware.go`)

```go
package bot

import "context"

// WithAuth replica a função pura de src/middleware/auth.ts para compatibilidade de testes unitários.
func WithAuth(userID string, allowedUserIDs []string, next func())

// IsUserAllowed verifica se userID pertence à whitelist allowedUserIDs.
func IsUserAllowed(userID string, allowedUserIDs []string) bool

// AuthMiddleware intercepta todas as atualizações do Telegram.
// Se o usuário não estiver em allowedUserIDs:
//   - Se for callback_query, responde AnswerCallbackQuery com "You do not have permission."
//   - Caso contrário, ignora silenciosamente sem chamar next.
func (b *Bot) AuthMiddleware(
	ctx context.Context,
	userID string,
	isCallbackQuery bool,
	answerCallback func(text string) error,
	next func(ctx context.Context) error,
) error
```

---

### 2.3. Parser de Mensagens e Comandos (`internal/bot/message_parser.go`)

```go
package bot

// ParsedMessage representa o resultado da análise sintática de uma mensagem de texto ou transcrição de voz.
type ParsedMessage struct {
	IsCommand   bool
	CommandName string
	Args        []string
	Text        string
}

// ParseMessageContent analisa se content é um comando iniciado por '/' e extrai nome e argumentos (preservando segmentos entre aspas duplas).
func ParseMessageContent(content string) ParsedMessage
```

---

### 2.4. Handlers de Chat, Workspace, Join, Slash Commands e Cleanup

#### A. `internal/bot/handlers_chat.go` (`chatCommandHandler.ts` + comandos `/start`, `/help`, `/new`, `/chat`, `/mode`, `/model`, `/stop`, `/screenshot`, `/status`, `/ping`, `/autoaccept`)
```go
package bot

import (
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/services"
)

type ChatCommandHandler struct {
	chatSessionService *services.ChatSessionService
	chatSessionRepo    *database.ChatSessionRepository
	bindingRepo        *database.WorkspaceBindingRepository
	pool               *cdp.CdpConnectionPool
	workspaceService   *services.WorkspaceService
}

func NewChatCommandHandler(
	chatSessionService *services.ChatSessionService,
	chatSessionRepo *database.ChatSessionRepository,
	bindingRepo *database.WorkspaceBindingRepository,
	workspaceService *services.WorkspaceService,
	pool *cdp.CdpConnectionPool,
) *ChatCommandHandler

func (h *ChatCommandHandler) GetSessionService() *services.ChatSessionService
func (h *ChatCommandHandler) GetSessionRepo() *database.ChatSessionRepository
```

#### B. `internal/bot/handlers_project.go` (`workspaceCommandHandler.ts` + comando `/project`)
```go
package bot

import (
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/services"
)

type WorkspaceCommandHandler struct {
	bindingRepo      *database.WorkspaceBindingRepository
	chatSessionRepo  *database.ChatSessionRepository
	workspaceService *services.WorkspaceService
}

func NewWorkspaceCommandHandler(
	bindingRepo *database.WorkspaceBindingRepository,
	chatSessionRepo *database.ChatSessionRepository,
	workspaceService *services.WorkspaceService,
) *WorkspaceCommandHandler

// GetWorkspaceForChannel retorna o caminho absoluto resolvido do workspace vinculado ao channelID, ou ("", false) se não houver vínculo.
func (h *WorkspaceCommandHandler) GetWorkspaceForChannel(channelID string) (string, bool)
```

#### C. `internal/bot/handlers_session.go` (`joinCommandHandler.ts` + comando `/join` e seletor em `/chat` — ADR-005)
```go
package bot

import (
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/services"
)

type JoinCommandHandler struct {
	chatSessionService *services.ChatSessionService
	chatSessionRepo    *database.ChatSessionRepository
	bindingRepo        *database.WorkspaceBindingRepository
	pool               *cdp.CdpConnectionPool
	workspaceService   *services.WorkspaceService
}

func NewJoinCommandHandler(
	chatSessionService *services.ChatSessionService,
	chatSessionRepo *database.ChatSessionRepository,
	bindingRepo *database.WorkspaceBindingRepository,
	pool *cdp.CdpConnectionPool,
	workspaceService *services.WorkspaceService,
) *JoinCommandHandler

func (h *JoinCommandHandler) ResolveProjectPath(projectName string) (string, error)
```

#### D. `internal/bot/handlers_template.go` (`slashCommandHandler.ts` + comandos `/template`, `/template_add`, `/template_delete`)
```go
package bot

import "antigravity-telegram-remote/internal/database"

// CommandResult representa o resultado da execução de um comando processado por SlashCommandHandler.
type CommandResult struct {
	Success bool
	Message string
	Prompt  string // preenchido quando um template existente é invocado via /template <name>
}

type SlashCommandHandler struct {
	templateRepo *database.TemplateRepository
}

func NewSlashCommandHandler(templateRepo *database.TemplateRepository) *SlashCommandHandler

func (h *SlashCommandHandler) HandleCommand(commandName string, args []string) CommandResult
```

#### E. `internal/bot/handlers_cleanup.go` (`cleanupCommandHandler.ts` + comando `/cleanup`)
```go
package bot

import "antigravity-telegram-remote/internal/database"

const (
	CleanupArchiveBtn = "cleanup_archive"
	CleanupDeleteBtn  = "cleanup_delete"
	CleanupCancelBtn  = "cleanup_cancel"
)

type InactiveSession struct {
	Binding database.WorkspaceBindingRecord
	Session *database.ChatSessionRecord
}

type CleanupCommandHandler struct {
	chatSessionRepo *database.ChatSessionRepository
	bindingRepo     *database.WorkspaceBindingRepository
}

func NewCleanupCommandHandler(
	chatSessionRepo *database.ChatSessionRepository,
	bindingRepo *database.WorkspaceBindingRepository,
) *CleanupCommandHandler

func (h *CleanupCommandHandler) CleanupByChannelID(channelID string) error
func (h *CleanupCommandHandler) FindInactiveSessions(guildID string, days int) ([]InactiveSession, error)
```

---

## 3. Constantes, Templates HTML/ASCII, Prefixos de Callback e Expressões Regulares Literais

### 3.1. Expressões Regulares em `messageParser.ts` e `cleanupCommandHandler.ts`
1. **Tokenização de argumentos com suporte a aspas duplas (`src/commands/messageParser.ts`, linha 19)**:
   - Regex literal: `/(?:[^\s"]+|"[^"]*")+/g`
   - Remoção de aspas externas de cada argumento (linha 30): `/^"(.*)"$/` $\rightarrow$ `$1`
2. **Formatação de timestamp de corte ISO para SQLite (`src/commands/cleanupCommandHandler.ts`, linha 38)**:
   - `cutoff.toISOString().replace('T', ' ').replace(/\.\d{3}Z$/, '')`
   - Formato exato resultante em UTC: `"YYYY-MM-DD HH:MM:SS"` (compatível com `datetime('now')` do SQLite).

---

### 3.2. Notificações de Ciclo de Vida WebSocket (`src/bot/index.ts`, linhas 899–924)

Enviadas para `bridge.lastActiveChannel` (se não nulo):
- Evento `workspace:disconnected`:
  ```html
  ⚠️ <b>{escapeHtml(projectName)}</b>: Connection lost. Reconnecting…
  ```
- Evento `workspace:reconnected`:
  ```html
  ✅ <b>{escapeHtml(projectName)}</b>: Reconnected.
  ```
- Evento `workspace:reconnectFailed`:
  ```html
  ❌ <b>{escapeHtml(projectName)}</b>: Reconnection failed. Send a message to retry.
  ```

---

### 3.3. Resposta de Permissão Negada no Middleware (`src/bot/index.ts`, linha 934)
- Texto exato em `ctx.answerCallbackQuery`:
  ```text
  You do not have permission.
  ```

---

### 3.4. Textos Literais Exatos de Todos os Comandos (`src/bot/index.ts` & `src/commands/slashCommandHandler.ts`)

#### 1. `/start` (`src/bot/index.ts`, linhas 980–987)
```html
<b>antigravity-telegram-remote Online</b>

Use /help for available commands.
Send any text message to forward it to Antigravity.
```

#### 2. `/help` (`src/bot/index.ts`, linhas 989–1015)
```html
<b>📖 antigravity-telegram-remote Commands</b>

<b>💬 Chat</b>
/new — Start a new chat session
/chat — Show current session info

<b>⏹️ Control</b>
/stop — Interrupt active LLM generation
/screenshot — Capture Antigravity screen

<b>⚙️ Settings</b>
/mode — Display and change execution mode
/model — Display and change LLM model

<b>📁 Projects</b>
/project — Display project list

<b>📝 Templates</b>
/template — Show templates
/template_add — Register a template
/template_delete — Delete a template

<b>🔧 System</b>
/status — Display overall bot status
/autoaccept — Toggle auto-approve mode
/cleanup [days] — Clean up inactive sessions
/ping — Check latency

<i>Text messages are sent directly to Antigravity</i>
```

#### 3. `/mode` (`src/bot/index.ts`, linhas 1017–1024)
- Delega integralmente para `sendModeUI` (ver Spec 14, Seção 3.2.B).

#### 4. `/model [nome]` (`src/bot/index.ts`, linhas 1026–1044)
- Se `modelName` for fornecido (`/model <modelName>`):
  - Sem conexão CDP: `'Not connected to CDP. Send a message first to connect.'` (texto simples)
  - Sucesso (`res.ok == true`): `Model changed to <b>${escapeHtml(res.model || modelName)}</b>.` (HTML)
  - Falha (`res.ok == false`): `res.error || 'Failed to change model.'` (texto simples)
- Sem argumentos: delega para `sendModelsUI` (ver Spec 14, Seção 3.2.C).

#### 5. `/template` (`src/bot/index.ts`, linhas 1046–1053)
- Como comando registrado em `bot.command('template')`: delega para `sendTemplateUI(replyHtml, templateRepo.findAll())` (ver Spec 14, Seção 3.2.H).
- Quando invocado via `SlashCommandHandler.handleCommand` (`src/commands/slashCommandHandler.ts`, linhas 26–122):
  - Comando desconhecido: `` `⚠️ Unknown command: /${commandName}` ``
  - `/template` sem argumentos e lista vazia: `'📝 No templates registered.'`
  - `/template` sem argumentos e com itens (`list = templates.map(t => "- **" + t.name + "**").join('\n')`):
    ```text
    📝 Registered Templates:
    {list}

    To use: `/template [name]`
    ```
  - `/template add` com `args.length < 3`:
    ```text
    ⚠️ Missing arguments.
    Usage: `/template add "name" "prompt"`
    ```
  - `/template add` com sucesso: `` `✅ Template **${name}** registered.` ``
  - `/template add` com erro (ex.: nome duplicado no SQLite): `'⚠️ Failed to register template. Name might be duplicated.'`
  - `/template delete` com `args.length < 2`:
    ```text
    ⚠️ Specify a template name to delete.
    Usage: `/template delete "name"`
    ```
  - `/template delete` com sucesso (`deleted == true`): `` `🗑️ Template **${name}** deleted.` ``
  - `/template delete` quando não encontrado (`deleted == false`): `` `⚠️ Template **${name}** not found.` ``
  - `/template <templateName>` quando não encontrado: `` `⚠️ Template **${templateName}** not found.` ``
  - `/template <templateName>` quando encontrado (retorna também `prompt: template.prompt`):
    ```text
    📝 Invoked template **{templateName}**.
    Starting process with this prompt.
    ```

#### 6. `/template_add` (`src/bot/index.ts`, linhas 1055–1067)
- Se `parts.length < 2` (após `args.split(/\s+/)` sobre `ctx.match.trim()`):
  ```text
  Usage: /template_add <name> <prompt>
  ```
- Caso contrário, delega para `slashCommandHandler.handleCommand('template', ['add', name, prompt])` e envia `result.message`.

#### 7. `/template_delete` (`src/bot/index.ts`, linhas 1069–1075)
- Se `!name` (`ctx.match.trim()` vazio):
  ```text
  Usage: /template_delete <name>
  ```
- Caso contrário, delega para `slashCommandHandler.handleCommand('template', ['delete', name])` e envia `result.message`.

#### 8. `/status` (`src/bot/index.ts`, linhas 1077–1100)
- **Quando `activeNames.length > 0`**:
  ```html
  <b>🔧 Bot Status</b>

  <b>CDP:</b> 🟢 {activeNames.length} project(s) connected
  <b>Mode:</b> {escapeHtml(MODE_DISPLAY_NAMES[currentMode] || currentMode)}
  <b>Auto Approve:</b> {bridge.autoAccept.isEnabled() ? '🟢 ON' : '⚪ OFF'}

  <b>Connected Projects:</b>
  • <b>{escapeHtml(name)}</b> — Contexts: {contexts}
  ```
- **Quando `activeNames.length === 0`**:
  ```html
  <b>🔧 Bot Status</b>

  <b>CDP:</b> ⚪ Disconnected
  <b>Mode:</b> {escapeHtml(MODE_DISPLAY_NAMES[currentMode] || currentMode)}
  <b>Auto Approve:</b> {bridge.autoAccept.isEnabled() ? '🟢 ON' : '⚪ OFF'}

  Send a message to auto-connect to a project.
  ```

#### 9. `/autoaccept [on|off]` (`src/bot/index.ts`, linhas 1102–1114)
- Se `requestedMode === 'on' || requestedMode === 'off'`: chama `bridge.autoAccept.handle(requestedMode)` e responde com `result.message`:
  - Quando `'on'`: `'🟢 Auto-accept mode turned ON. Approval dialogs will be automatically allowed.'` (ou mensagem traduzida pelo `AutoAcceptService`)
  - Quando `'off'`: `'⚪ Auto-accept mode turned OFF. Approval dialogs require manual action.'`
- Caso contrário: delega para `sendAutoAcceptUI` (ver Spec 14, Seção 3.2.A).

#### 10. `/cleanup [days]` (`src/bot/index.ts`, linhas 1116–1145)
- Cálculo de `days`: `Math.max(1, parseInt((ctx.match || '').trim(), 10) || 7)` (se não numérico ou omitido, `days = 7`; se `<= 0`, força `1`).
- **Quando `inactive.length === 0`**:
  ```html
  No inactive sessions older than <b>{days}</b> day(s).
  ```
- **Quando `inactive.length > 0`**:
  - `list`: até 20 itens formatados como `• ${escapeHtml(session?.displayName ?? binding.workspacePath)}` unidos por `'\n'`.
  - `extra`: se `inactive.length > 20`, `\n…and ${inactive.length - 20} more`; caso contrário `''`.
  - **Texto HTML**:
    ```html
    <b>🧹 Cleanup</b>

    Found <b>{inactive.length}</b> session(s) older than <b>{days}</b> day(s):

    {list}{extra}

    Choose an action:
    ```
  - **Teclado Inline** (1 linha com 3 botões):
    - `['📦 Archive' -> 'cleanup_archive:' + days]`
    - `['🗑 Delete' -> 'cleanup_delete:' + days]`
    - `['❌ Cancel' -> 'cleanup_cancel']`

#### 11. `/screenshot` (`src/bot/index.ts`, linhas 1147–1154)
- Delega para `handleScreenshot` passando `getCurrentCdp(bridge)` (ver Spec 14, Seção 3.2.F).

#### 12. `/stop` (`src/bot/index.ts`, linhas 1156–1180)
- Sem conexão CDP: `'⚠️ Not connected to CDP.'` (texto simples)
- Quando `RESPONSE_SELECTORS.CLICK_STOP_BUTTON` retorna `value?.ok === true`:
  - Adiciona `channelKey(ch)` em `userStopRequestedChannels` e envia HTML:
    ```html
    <b>⏹️ Generation Interrupted</b>
    AI response generation was safely stopped.
    ```
- Quando `value?.ok` é falso:
  ```html
  <b>⚠️ Could Not Stop</b>
  {escapeHtml(value?.error || 'Stop button not found.')}
  ```
- Quando ocorre exceção: `` `❌ Error during stop: ${e.message}` `` (texto simples).

#### 13. `/project` (`src/bot/index.ts`, linhas 1182–1188)
- Escaneia `workspaces = workspaceService.scanWorkspaces()` e envia `buildProjectListUI(workspaces, 0)` (ver Spec 14, Seção 3.2.E).

#### 14. `/new` (`src/bot/index.ts`, linhas 1190–1218)
- Sem projeto vinculado ao canal (`!workspaceName`):
  ```text
  ⚠️ No project is bound to this chat. Use /project to select one.
  ```
- Falha ao conectar ao pool CDP: `` `⚠️ Failed to connect: ${e.message}` ``
- Sucesso em `chatSessionService.startNewChat(cdp)` (`chatResult.ok === true`):
  ```html
  <b>💬 New Chat Started</b>
  Send your message now.
  ```
- Falha em `startNewChat` (`chatResult.ok === false`): `` `⚠️ Could not start new chat: ${chatResult.error}` ``
- Exceção em `startNewChat`: `` `⚠️ Error: ${e.message}` ``

#### 15. `/chat` e `/join` (`src/bot/index.ts`, linhas 1220–1261 + ADR-005)
- **Quando não há `session` registrada para `channelKey(ch)` no SQLite**:
  - Se houver workspace conectado no pool (`activeNames.length > 0`), consulta `chatSessionService.getCurrentSessionInfo(anyCdp)`; caso contrário usa `{ title: '(CDP Disconnected)', hasActiveChat: false }`.
  - **Texto HTML**:
    ```html
    <b>💬 Chat Session Info</b>

    <b>Title:</b> {escapeHtml(info.title)}
    <b>Status:</b> {info.hasActiveChat ? '🟢 Active' : '⚪ Inactive'}

    <i>No project bound to this chat thread. Use /project to select one.</i>
    <code>Key: {key}</code>
    ```
- **Quando há `session` registrada no SQLite**:
  - Busca `allSessions = chatSessionRepo.findByCategoryId(session.categoryId)`.
  - Formata cada item em `sessionList` como:
    `• ${s.displayName || "session-" + s.sessionNumber}${s.channelId === key ? ' ← Current' : ''}`
  - **Texto HTML**:
    ```html
    <b>💬 Chat Session Info</b>

    <b>Current:</b> #{session.sessionNumber} — {escapeHtml(session.displayName || '(Unset)')}
    <b>Project:</b> {escapeHtml(session.workspacePath)}
    <b>Total sessions:</b> {allSessions.length}

    <b>Sessions:</b>
    {escapeHtml(sessionList)}
    ```
- **Evolução ADR-005 (`/chat` e `/join`)**:
  - Quando o canal possui um workspace vinculado (`binding` ou `session`) e conexão CDP ativa:
    - Em `/join`: consulta `sessions = await chatSessionService.listAllSessions(cdp)` e responde com `buildSessionPickerUI(sessions)` (`<b>🔗 Tham gia phiên</b>...` + botões `session_select:<title>`). Se não houver projeto vinculado, responde `'⚠️ No project is bound to this chat. Use /project to select one.'`.
    - Em `/chat`: se conectado ao CDP e `listAllSessions(cdp)` retornar sessões disponíveis no painel do Antigravity, anexa o teclado inline de `buildSessionPickerUI(sessions)` à resposta de `/chat` para permitir troca imediata de sessão.

#### 16. `/ping` (`src/bot/index.ts`, linhas 1263–1269)
- Mensagem inicial enviada via `ctx.reply`:
  ```text
  🏓 Pong!
  ```
- Mensagem editada via `editMessageText` com `parse_mode: 'HTML'`:
  ```html
  🏓 Pong! Latency: <b>{latency}ms</b>
  ```

---

### 3.5. Menu de Comandos Registrado em `bot.api.setMyCommands` (`src/bot/index.ts`, linhas 1915–1931)

Registrado no callback `onStart` do bot exatamente nesta ordem e com estas descrições em vietnamita:

```json
[
  { "command": "start", "description": "Tin nhắn chào mừng" },
  { "command": "help", "description": "Hiển thị tất cả lệnh" },
  { "command": "project", "description": "Chọn một dự án" },
  { "command": "new", "description": "Bắt đầu phiên chat mới" },
  { "command": "chat", "description": "Thông tin phiên hiện tại" },
  { "command": "mode", "description": "Thay đổi chế độ thực thi" },
  { "command": "model", "description": "Thay đổi mô hình LLM" },
  { "command": "stop", "description": "Dừng việc tạo kết quả" },
  { "command": "screenshot", "description": "Chụp màn hình Antigravity" },
  { "command": "template", "description": "Hiển thị mẫu lời nhắc" },
  { "command": "template_add", "description": "Đăng ký một mẫu" },
  { "command": "template_delete", "description": "Xóa một mẫu" },
  { "command": "autoaccept", "description": "Bật/tắt chế độ tự động duyệt" },
  { "command": "status", "description": "Tổng quan trạng thái bot" },
  { "command": "ping", "description": "Kiểm tra độ trễ" }
]
```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Sequência de Inicialização em `StartBot` (`internal/bot/bot.go`)
1. Carrega `config := config.LoadConfig()` e define o nível de log (`cliLogLevel` tem prioridade sobre `config.LogLevel`).
2. Determina o caminho do banco SQLite: `:memory:` quando em ambiente de teste (`NODE_ENV == "test"`), ou `~/.antigravity-telegram-remote/antigravity.db` (garantindo antes a criação do diretório `~/.antigravity-telegram-remote` com `EnsureConfigDirExists()`).
3. Abre a conexão SQLite e executa imediatamente `PRAGMA journal_mode = WAL;`.
4. Instancia `ModeService`, `ModelService`, `TemplateRepository(db)`, `WorkspaceBindingRepository(db)`, `ChatSessionRepository(db)` e `WorkspaceService(config.WorkspaceBaseDir)`.
5. Executa `EnsureAntigravityRunning()` para verificar se o Antigravity já está escutando em alguma das portas CDP.
6. Inicializa o bridge via `InitCdpBridge(config.AutoApproveFileEdits)` e atribui `bridge.BotToken = config.TelegramBotToken`.
7. Instancia `ChatSessionService`, `TitleGeneratorService`, `PromptDispatcher` (injetando `SendPromptToAntigravity`), `SlashCommandHandler(templateRepo)` e `CleanupCommandHandler(chatSessionRepo, workspaceBindingRepo)`.
8. Cria a instância do cliente do Bot Telegram, atribui `bridge.BotAPI = bot.API` e registra os 3 listeners de eventos do `bridge.Pool`:
   - `workspace:disconnected`
   - `workspace:reconnected`
   - `workspace:reconnectFailed`
9. Instancia `TelegramTopicManager(bot.API, 0)`.
10. Registra o middleware global de autenticação (`AuthMiddleware`), todos os handlers de comando, o handler `callback_query:data` e os handlers `message:text`, `message:photo` e `message:voice`.
11. Registra captura de sinais (`SIGINT`, `SIGTERM`) para fechar o banco SQLite graciosamente (`db.Close()`).
12. Inicia o long-polling e, no callback `onStart`, registra `setMyCommands`.

### 4.2. Resolução de Workspace e Inicialização de Detectores (`ResolveWorkspaceAndCdp`)
1. Calcula `key := ChannelKey(ch)`.
2. Busca `binding := workspaceBindingRepo.FindByChannelID(key)`. Se `binding == nil`, retorna `nil`.
3. Resolve o caminho absoluto seguro com `workspacePath := workspaceService.GetWorkspacePath(binding.WorkspacePath)`.
4. Obtém ou estabelece conexão CDP via `cdp, err := bridge.Pool.GetOrConnect(ctx, workspacePath)`. Se falhar, loga `[resolveWorkspaceAndCdp] Connection failed:` e retorna `nil`.
5. Extrai `projectName := bridge.Pool.ExtractProjectName(workspacePath)`.
6. Atualiza `bridge.LastActiveWorkspace = projectName` e `bridge.LastActiveChannel = ch`.
7. Registra o canal no mapa de aprovações por workspace: `RegisterApprovalWorkspaceChannel(bridge, projectName, ch)`.
8. Garante que os detectores em background estejam ativos para o projeto:
   - `EnsureApprovalDetector(bridge, cdp, projectName)`
   - `EnsureErrorPopupDetector(bridge, cdp, projectName)`
   - `EnsurePlanningDetector(bridge, cdp, projectName)`
   - `EnsureUserMessageDetector(bridge, cdp, projectName, onUserMessage)` (para espelhamento PC $\rightarrow$ Telegram detalhado na Spec 16).
9. Retorna `&ResolvedWorkspaceCdp{Cdp: cdp, ProjectName: projectName, WorkspacePath: workspacePath}`.

### 4.3. Algoritmo de `ParseMessageContent` (`internal/bot/message_parser.go`)
1. Calcula `trimmed := strings.TrimSpace(content)`.
2. Se `!strings.HasPrefix(trimmed, "/") || trimmed == "/"`:
   - Retorna `ParsedMessage{IsCommand: false, Text: content}` (preservando o `content` original sem `trim`).
3. Remove a barra inicial (`trimmed[1:]`) e extrai todos os tokens casando a expressão `[^\s"]+|"[^"]*"`.
4. Se nenhum token for encontrado (`len(parts) == 0`), retorna `ParsedMessage{IsCommand: false, Text: content}`.
5. Define `commandName := parts[0]`.
6. Para cada token em `parts[1:]`, se começar com `"` e terminar com `"` (e `len >= 2`), remove as aspas externas (`arg[1 : len(arg)-1]`).
7. Retorna `ParsedMessage{IsCommand: true, CommandName: commandName, Args: args}`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Proteção de Mapas de Estado no `Bot`**:
   - `userStopRequested`, `planEditPendingChannels` e `planContentCache` são protegidos por `stateMu sync.RWMutex` na struct `Bot`.
   - Operações como `/stop` (`AddUserStopRequest(channelKey)`) e `onComplete` (`ConsumeUserStopRequest(channelKey)`) são atômicas sob `stateMu.Lock()`.
2. **Encerramento Gracioso (`SIGINT` / `SIGTERM`)**:
   - O contexto principal passado a `StartBot` é cancelado via `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`.
   - No `defer` de `StartBot`, o polling do Telegram é interrompido, todos os detectores e conexões WebSocket do `bridge.Pool` são fechados e `db.Close()` é executado para fazer checkpoint do WAL do SQLite.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os testes unitários originais de `tests/middleware/auth.test.ts` e `tests/commands/*.test.ts` devem ser implementados e passar com `go test -race ./internal/bot/...`:

### 6.1. `internal/bot/middleware_test.go` (espelhando `tests/middleware/auth.test.ts`)
- `TestWithAuth_AllowedUser`: quando `userID = "123"` e `allowedUserIDs = ["123", "456"]`, chama `next` exatamente 1 vez.
- `TestWithAuth_DeniedUser`: quando `userID = "789"` e `allowedUserIDs = ["123", "456"]`, não chama `next`.
- `TestAuthMiddleware_DeniedCallbackQuery`: quando não autorizado e `isCallbackQuery == true`, chama `answerCallback("You do not have permission.")` e não chama `next`.

### 6.2. `internal/bot/message_parser_test.go` (espelhando `tests/commands/messageParser.test.ts`)
- `TestParseMessageContent_PlainText`: `"こんにちは、システムを再起動してください。"` $\rightarrow$ `IsCommand: false`, `Text` idêntico.
- `TestParseMessageContent_CommandWithArg`: `"/mode gpt-4"` $\rightarrow$ `IsCommand: true`, `CommandName: "mode"`, `Args: ["gpt-4"]`.
- `TestParseMessageContent_DoubleQuotedArgs`: `'/template add "朝会" "おはよう"'` $\rightarrow$ `IsCommand: true`, `CommandName: "template"`, `Args: ["add", "朝会", "おはよう"]`.
- `TestParseMessageContent_UnquotedArgs`: `"/template PR作成"` $\rightarrow$ `IsCommand: true`, `CommandName: "template"`, `Args: ["PR作成"]`.
- `TestParseMessageContent_SurroundingWhitespace`: `"  /stop  "` $\rightarrow$ `IsCommand: true`, `CommandName: "stop"`, `Args: []`.
- `TestParseMessageContent_LoneSlash`: `"/"` $\rightarrow$ `IsCommand: false`, `Text: "/"`.

### 6.3. `internal/bot/handlers_chat_test.go` (espelhando `tests/commands/chatCommandHandler.test.ts`)
- `TestChatCommandHandler_GettersAndOptionalPool`: valida `GetSessionService()`, `GetSessionRepo()`, construção com `pool` opcional e `pool == nil`.

### 6.4. `internal/bot/handlers_project_test.go` (espelhando `tests/commands/workspaceCommandHandler.test.ts`)
- `TestWorkspaceCommandHandler_GetWorkspaceForChannel_Found`: quando `bindingRepo.FindByChannelID("channel-1")` retorna `{ WorkspacePath: "my-project" }`, delega para `workspaceService.GetWorkspacePath("my-project")` e retorna `("/home/user/my-project", true)`.
- `TestWorkspaceCommandHandler_GetWorkspaceForChannel_NotFound`: quando não há binding, retorna `("", false)` sem chamar `workspaceService`.

### 6.5. `internal/bot/handlers_session_test.go` (espelhando `tests/commands/joinCommandHandler.test.ts`)
- `TestJoinCommandHandler_ResolveProjectPath`: delega para `workspaceService.GetWorkspacePath("my-project")` e retorna `"/home/user/code/my-project"`.

### 6.6. `internal/bot/handlers_template_test.go` (espelhando `tests/commands/slashCommandHandler.test.ts`)
- `TestSlashCommandHandler_UnknownCommand`: `"unknown"` $\rightarrow$ `Success: false`, contém `"Unknown command"`.
- `TestSlashCommandHandler_RejectsPluralAlias`: `"templates"` $\rightarrow$ `Success: false`, contém `"Unknown command"`.
- `TestSlashCommandHandler_ListTemplates`: sem argumentos com 2 templates $\rightarrow$ `Success: true`, contém ambos os nomes.
- `TestSlashCommandHandler_ListEmpty`: sem argumentos e sem templates $\rightarrow$ `Success: true`, contém `"No templates registered"`.
- `TestSlashCommandHandler_InvokeTemplate`: `["PR作成"]` existente $\rightarrow$ `Success: true`, `Prompt: "PRを作成してください。"`.
- `TestSlashCommandHandler_InvokeNonExistent`: `["存在しない"]` $\rightarrow$ `Success: false`, contém `"not found"`.
- `TestSlashCommandHandler_AddTemplate`: `["add", "テスト実行", "テストを実行して"]` $\rightarrow$ `Success: true`, chama `templateRepo.Create`.
- `TestSlashCommandHandler_AddMissingArgs`: `["add"]` $\rightarrow$ `Success: false`, contém `"Missing arguments"`.
- `TestSlashCommandHandler_DeleteTemplate`: `["delete", "テスト実行"]` $\rightarrow$ `Success: true` quando existe, `Success: false` quando não existe.

### 6.7. `internal/bot/handlers_cleanup_test.go` (espelhando `tests/commands/cleanupCommandHandler.test.ts`)
- `TestCleanupButtonConstants`: verifica `CleanupArchiveBtn == "cleanup_archive"`, `CleanupDeleteBtn == "cleanup_delete"`, `CleanupCancelBtn == "cleanup_cancel"`.
- `TestCleanupByChannelID_SuccessAndError`: chama `chatSessionRepo.DeleteByChannelID` seguido de `bindingRepo.DeleteByChannelID`; se o primeiro retornar erro, aborta sem chamar o segundo.
- `TestFindInactiveSessions`: filtra corretamente sessões/bindings com `created_at < cutoffIso`.
