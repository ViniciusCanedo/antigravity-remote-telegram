# Especificação Técnica 01 — Configuração, Logger Estruturado e Internacionalização (i18n)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de forma 100% autocontida e exaustiva a camada de configuração (`AppConfig` / `ConfigLoader`), logging ANSI estruturado (`Logger`) e internacionalização multi-idioma (`i18n` com dicionários embutidos em JSON) para o port em Golang (`1.23+`) do `antigravity-telegram-remote`.

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript / JSON) | Arquivo de Testes Original (Jest) | Destino no Port Go | Pacote Go | Responsabilidade Principal |
| :--- | :--- | :--- | :--- | :--- |
| [`src/utils/config.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/config.ts) | [`tests/config.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/config.test.ts)<br>[`tests/utils/config.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/config.test.ts) | `internal/config/config.go`<br>`internal/config/config_test.go` | `config` | Definição de `AppConfig`, `ExtractionMode`, `ResponseDeliveryMode`, `ResolveResponseDeliveryMode()` e `LoadConfig()`. |
| [`src/utils/configLoader.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/configLoader.ts) | [`tests/utils/configLoader.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/configLoader.test.ts) | `internal/config/loader.go`<br>`internal/config/loader_test.go` | `config` | Carregamento de `.env`, leitura/escrita de `~/.antigravity-telegram-remote/config.json`, expansão de `~`, validação e mesclagem de precedência (`env > config.json > defaults`). |
| [`src/utils/logger.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/logger.ts) | [`tests/utils/logger.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/logger.test.ts) | `internal/logger/logger.go`<br>`internal/logger/logger_test.go` | `logger` | Logger de console com cores ANSI (`COLORS`), timestamp 24h (`HH:MM:SS`), prioridades de nível (`debug`, `info`, `warn`, `error`, `none`) e helpers `Phase`, `Done` e `Divider`. |
| [`src/utils/i18n.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/i18n.ts) | [`tests/utils/i18n.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/i18n.test.ts) | `internal/i18n/i18n.go`<br>`internal/i18n/i18n_test.go` | `i18n` | Inicialização de idioma (`en`, `ja`, `vi`, padrão `'vi'`), carregamento via `//go:embed`, fallback (`currentLanguage -> 'en' -> key`) e interpolação literal `{{var}}`. |
| [`locales/en.json`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/locales/en.json) | [`tests/utils/i18n.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/i18n.test.ts) | `internal/i18n/locales/en.json` | `i18n` | Dicionário de tradução em Inglês (83 chaves). |
| [`locales/ja.json`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/locales/ja.json) | [`tests/utils/i18n.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/i18n.test.ts) | `internal/i18n/locales/ja.json` | `i18n` | Dicionário de tradução em Japonês (107 chaves). |
| [`locales/vi.json`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/locales/vi.json) | [`tests/utils/i18n.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/i18n.test.ts) | `internal/i18n/locales/vi.json` | `i18n` | Dicionário de tradução em Vietnamita (113 chaves). |

### 1.2. Documentação Arquitetural Relacionada
- [`antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md)

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pacote `internal/logger` (`internal/logger/logger.go`)

```go
package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel representa os níveis de severidade suportados pelo Logger.
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	LevelNone  LogLevel = "none"
)

// Colors agrupa os códigos de escape ANSI exatos utilizados na formatação de terminal.
type AnsiColors struct {
	Red     string
	Yellow  string
	Cyan    string
	Green   string
	Magenta string
	Dim     string
	Reset   string
}

var COLORS = AnsiColors{
	Red:     "\x1b[31m",
	Yellow:  "\x1b[33m",
	Cyan:    "\x1b[36m",
	Green:   "\x1b[32m",
	Magenta: "\x1b[35m",
	Dim:     "\x1b[2m",
	Reset:   "\x1b[0m",
}

// Logger define a interface completa equivalente ao Logger em src/utils/logger.ts.
type Logger interface {
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Debug(args ...any)
	Phase(args ...any)
	Done(args ...any)
	Divider(label ...string)
	SetLogLevel(level LogLevel)
	GetLogLevel() LogLevel
}

// ConsoleWriters permite injetar writers customizados em testes unitários
// (equivalente aos spies em console.info, console.warn, console.error, console.debug).
type ConsoleWriters struct {
	InfoOut  io.Writer
	WarnOut  io.Writer
	ErrorOut io.Writer
	DebugOut io.Writer
	NowFunc  func() time.Time
}

type standardLogger struct {
	mu           sync.RWMutex
	currentLevel LogLevel
	writers      ConsoleWriters
}

// CreateLogger cria uma nova instância de Logger com nível inicial (padrão LevelInfo se vazio).
func CreateLogger(initialLevel ...LogLevel) Logger

// CreateLoggerWithWriters cria uma instância de Logger com writers injetáveis para testes.
func CreateLoggerWithWriters(initialLevel LogLevel, writers ConsoleWriters) Logger

// Default é a instância singleton global equivalente a `export const logger = createLogger('info')`.
var Default Logger = CreateLogger(LevelInfo)

// Funções de conveniência no pacote que delegam para Default:
func Info(args ...any)
func Warn(args ...any)
func Error(args ...any)
func Debug(args ...any)
func Phase(args ...any)
func Done(args ...any)
func Divider(label ...string)
func SetLogLevel(level LogLevel)
func GetLogLevel() LogLevel
func IsValidLogLevel(raw string) bool
```

---

### 2.2. Pacote `internal/config` (`internal/config/config.go` e `internal/config/loader.go`)

```go
package config

import (
	"antigravity-telegram-remote/internal/logger"
)

// ExtractionMode define o modo de extração DOM ('legacy' | 'structured').
type ExtractionMode string

const (
	ExtractionModeLegacy     ExtractionMode = "legacy"
	ExtractionModeStructured ExtractionMode = "structured"
)

// ResponseDeliveryMode define o modo de entrega de respostas ('stream').
type ResponseDeliveryMode string

const (
	ResponseDeliveryModeStream ResponseDeliveryMode = "stream"
)

// AppConfig representa a configuração validada em tempo de execução.
type AppConfig struct {
	TelegramBotToken     string                `json:"telegramBotToken"`
	AllowedUserIds       []string              `json:"allowedUserIds"`
	WorkspaceBaseDir     string                `json:"workspaceBaseDir"`
	AutoApproveFileEdits bool                  `json:"autoApproveFileEdits"`
	LogLevel             logger.LogLevel       `json:"logLevel"`
	ExtractionMode       ExtractionMode        `json:"extractionMode"`
	UseTopics            bool                  `json:"useTopics"`
}

// PersistedConfig representa o conteúdo opcional persistido em ~/.antigravity-telegram-remote/config.json.
// Ponteiros são usados para distinguir campos omitidos (undefined/nil) de valores zero (false, "").
type PersistedConfig struct {
	TelegramBotToken     *string          `json:"telegramBotToken,omitempty"`
	AllowedUserIds       []string         `json:"allowedUserIds,omitempty"`
	WorkspaceBaseDir     *string          `json:"workspaceBaseDir,omitempty"`
	AutoApproveFileEdits *bool            `json:"autoApproveFileEdits,omitempty"`
	LogLevel             *logger.LogLevel `json:"logLevel,omitempty"`
	ExtractionMode       *ExtractionMode  `json:"extractionMode,omitempty"`
	UseTopics            *bool            `json:"useTopics,omitempty"`
}

// ResolveResponseDeliveryMode retorna sempre ResponseDeliveryModeStream ("stream").
func ResolveResponseDeliveryMode() ResponseDeliveryMode

// LoadConfig carrega a configuração mesclando variáveis de ambiente, arquivo persistido e defaults.
func LoadConfig() (AppConfig, error)

// Loader encapsula as operações de leitura, validação e persistência de configuração,
// permitindo substituir HomeDir e Env em testes unitários.
type Loader struct {
	HomeDirFunc func() (string, error)
	LookupEnv   func(key string) (string, bool)
	ReadFile    func(path string) ([]byte, error)
	WriteFile   func(path string, data []byte, perm uint32) error
	StatExists  func(path string) bool
	MkdirAll    func(path string, perm uint32) error
}

// DefaultLoader é a instância padrão equivalente ao objeto `ConfigLoader` do TypeScript.
var DefaultLoader *Loader

// Funções de pacote equivalentes aos métodos de `ConfigLoader`:
func GetConfigDir() string
func GetConfigFilePath() string
func GetDefaultDbPath() string
func ConfigExists() bool
func EnsureConfigDirExists() error
func Load(persistedOverride *PersistedConfig) (AppConfig, error)
func Save(partial PersistedConfig) error
func LoadDotEnv(envPath string) error
```

---

### 2.3. Pacote `internal/i18n` (`internal/i18n/i18n.go`)

```go
package i18n

import (
	"embed"
	"sync"
)

// Language representa os códigos de idioma suportados ('en' | 'ja' | 'vi').
type Language string

const (
	LangEN Language = "en"
	LangJA Language = "ja"
	LangVI Language = "vi"
)

//go:embed locales/*.json
var embeddedLocales embed.FS

// Translator gerencia o estado de internacionalização de forma thread-safe.
type Translator struct {
	mu              sync.RWMutex
	currentLanguage Language
	translations    map[Language]map[string]string
	readFileFunc    func(lang Language) ([]byte, error)
}

// InitI18n inicializa o idioma global (padrão LangVI quando nenhum argumento é passado)
// e carrega os dicionários de tradução.
func InitI18n(lang ...Language)

// InitI18nWithReader permite injetar um leitor customizado para testes unitários
// (por exemplo, simulando arquivos ausentes ou vazios como no mock de fs em tests/utils/i18n.test.ts).
func InitI18nWithReader(lang Language, reader func(lang Language) ([]byte, error))

// T traduz uma chave usando o idioma atual (com fallback para 'en' e depois para a própria key)
// e substitui todas as ocorrências de `{{varKey}}` pelos valores fornecidos em variables.
func T(key string, variables ...map[string]any) string

// GetCurrentLanguage retorna o idioma ativo no momento.
func GetCurrentLanguage() Language
```

---

## 3. Constantes, Queries SQL, Expressões Regulares e Scripts Literais

### 3.1. Constantes de `configLoader.ts` e `config.ts`

```go
const (
	ConfigDirName  = ".antigravity-telegram-remote"
	ConfigFileName = "config.json"
	DefaultDbName  = "antigravity.db"
	DefaultCodeDir = "Code"
)

var ValidLogLevels = []logger.LogLevel{
	logger.LevelDebug,
	logger.LevelInfo,
	logger.LevelWarn,
	logger.LevelError,
	logger.LevelNone,
}
```

### 3.2. Expressão Regular de Validação de `ALLOWED_USER_IDS`
No original (`src/utils/configLoader.ts`, linhas 130 e 134):
```regexp
^\d+$
```
- **Regra**: Apenas strings não-vazias compostas exclusivamente por dígitos ASCII (`0-9`) são aceitas como IDs numéricos válidos de usuários do Telegram.

### 3.3. Mensagens de Erro e Logs Literais de `configLoader.ts`
- Erro ao fazer parse de `config.json` malformado:
  ```text
  Failed to parse config file at <filePath>. Check for syntax errors.
  ```
- Erro quando `TELEGRAM_BOT_TOKEN` está ausente ou vazio:
  ```text
  Missing required config: TELEGRAM_BOT_TOKEN
  ```
- Erro quando `ALLOWED_USER_IDS` resulta em lista vazia:
  ```text
  Missing required config: ALLOWED_USER_IDS
  ```

### 3.4. Constantes de Cores e Prioridades de `logger.ts`

```go
var levelPriority = map[LogLevel]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
	LevelNone:  4,
}
```

| Método | Nível de Filtro (`shouldLog`) | Tag Colorida Literal |
| :--- | :--- | :--- |
| `Info` | `info` (prioridade `>= 1`) | `\x1b[2m[HH:MM:SS]\x1b[0m \x1b[36m[INFO]\x1b[0m` |
| `Warn` | `warn` (prioridade `>= 2`) | `\x1b[2m[HH:MM:SS]\x1b[0m \x1b[33m[WARN]\x1b[0m` |
| `Error` | `error` (prioridade `>= 3`) | `\x1b[2m[HH:MM:SS]\x1b[0m \x1b[31m[ERROR]\x1b[0m` |
| `Debug` | `debug` (prioridade `>= 0`) | `\x1b[2m[HH:MM:SS]\x1b[0m \x1b[2m[DEBUG]\x1b[0m` |
| `Phase` | `info` (prioridade `>= 1`) | `\x1b[2m[HH:MM:SS]\x1b[0m \x1b[35m[PHASE]\x1b[0m` |
| `Done` | `info` (prioridade `>= 1`) | `\x1b[2m[HH:MM:SS]\x1b[0m \x1b[32m[DONE]\x1b[0m` |
| `Divider(label)` | `info` (prioridade `>= 1`) | Com label: `\x1b[32m[DONE]\x1b[0m \x1b[2m── <label> <'─'.repeat(pad)>\x1b[0m` onde `pad = max(4, 50 - len(label) - 4)`<br>Sem label: `\x1b[32m[DONE]\x1b[0m \x1b[2m──────────────────────────────────────────────────\x1b[0m` (50 caracteres `'─'`) |

---

### 3.5. Conteúdo Literal Completo dos Arquivos de Tradução (`locales/*.json`)

#### A. `internal/i18n/locales/en.json` ([`locales/en.json`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/locales/en.json))
```json
{
  "Auto-approved": "Auto-approved",
  "Auto-approve failed": "Auto-approve failed",
  "Antigravity is requesting approval for an action": "Antigravity is requesting approval for an action",
  "Auto-approve mode": "Auto-approve mode",
  "ON": "ON",
  "Workspace": "Workspace",
  "Result": "Result",
  "Executed Always Allow/Allow": "Executed Always Allow/Allow",
  "Manual approval required": "Manual approval required",
  "Approval Required": "Approval Required",
  "Allow button": "Allow button",
  "Allow Chat button": "Allow This Conversation button",
  "In Dropdown": "In Dropdown",
  "Deny button": "Deny button",
  "(None)": "(None)",
  "Allow": "Allow",
  "Allow Chat": "Allow This Conversation",
  "Deny": "Deny",
  "🖼️ Detected generated images (${files.length})": "🖼️ Detected generated images (${files.length})",
  "Generating...": "Generating...",
  "🖼️ Attached image fallback": "🖼️ Attached image fallback",
  "Failed to attach image directly, resending via URL reference.": "Failed to attach image directly, resending via URL reference.",
  "⏱️ Elapsed: 0s | Waiting to start": "⏱️ Elapsed: 0s | Waiting to start",
  "⏱️ Elapsed: ${elapsed}s | Generating": "⏱️ Elapsed: ${elapsed}s | Generating",
  "⏱️ Time: ${elapsed}s | Complete": "⏱️ Time: ${elapsed}s | Complete",
  "Failed to extract response. Use `/screenshot` to verify.": "Failed to extract response. Use `/screenshot` to verify.",
  "(Untitled)": "(Untitled)",
  "${timeoutText}\\n\\n[Monitor Ended] Timeout after 5 minutes.": "${timeoutText}\\n\\n[Monitor Ended] Timeout after 5 minutes.",
  "Error occurred during processing: ${e.message}": "Error occurred during processing: ${e.message}",
  "You do not have permission.": "You do not have permission.",
  "Approval detector not found.": "Approval detector not found.",
  "⚠️ This command can only be used in a server.": "⚠️ This command can only be used in a server.",
  "⚠️ Please execute in a text channel.": "⚠️ Please execute in a text channel.",
  "⚠️ Failed to switch project: ${e.message}": "⚠️ Failed to switch project: ${e.message}",
  "⚠️ CDP pool is not initialized or cannot connect to workspace.": "⚠️ CDP pool is not initialized or cannot connect to workspace.",
  "💬 Started a new session": "💬 Started a new session",
  "Created a new chat session\\n→ <#${newChannelId}>": "Created a new chat session\\n→ <#${newChannelId}>",
  "Session": "Session",
  "Project": "Project",
  "(CDP Disconnected)": "(CDP Disconnected)",
  "💬 Chat Session Info": "💬 Chat Session Info",
  "Title": "Title",
  "Status": "Status",
  "🟢 Active": "🟢 Active",
  "⚪ Inactive": "⚪ Inactive",
  " ← **Current**": " ← **Current**",
  "Current session": "Current session",
  "#${session.sessionNumber} — ${session.displayName || '(Unset)'}": "#${session.sessionNumber} — ${session.displayName || '(Unset)'}",
  "Total sessions": "Total sessions",
  "**Sessions:**\\n${sessionList}": "**Sessions:**\\n${sessionList}",
  "⚠️ Unknown command: /${commandName}": "⚠️ Unknown command: /${commandName}",
  "✅ Mode changed to **${result.mode}**.": "✅ Mode changed to **${result.mode}**.",
  "⚠️ Invalid mode.": "⚠️ Invalid mode.",
  "📝 No templates registered.": "📝 No templates registered.",
  "✅ Template **${name}** registered.": "✅ Template **${name}** registered.",
  "⚠️ Failed to register template. Name might be duplicated.": "⚠️ Failed to register template. Name might be duplicated.",
  "🗑️ Template **${name}** deleted.": "🗑️ Template **${name}** deleted.",
  "⚠️ Template **${name}** not found.": "⚠️ Template **${name}** not found.",
  "⚠️ Template **${templateName}** not found.": "⚠️ Template **${templateName}** not found.",
  "📝 Invoked template **${templateName}**.\\nStarting process with this prompt.": "📝 Invoked template **${templateName}**.\\nStarting process with this prompt.",
  "Select a project to auto-create a category and session channel": "Select a project to auto-create a category and session channel",
  "Select a project...": "Select a project...",
  "Select a project... (Showing 25 of ${workspaces.length})": "Select a project... (Showing 25 of ${workspaces.length})",
  "⚠️ Project **${workspacePath}** already exists\\n": "⚠️ Project **${workspacePath}** already exists\\n",
  "Full Path": "Full Path",
  "⏳ **${workspacePath}** is being created. Please wait.": "⏳ **${workspacePath}** is being created. Please wait.",
  "✅ Project **${workspacePath}** created\\n": "✅ Project **${workspacePath}** created\\n",
  "❌ Invalid project name: ${e.message}": "❌ Invalid project name: ${e.message}",
  "⚠️ Project **${name}** already exists → ${channelLinks}": "⚠️ Project **${name}** already exists → ${channelLinks}",
  "⏳ **${name}** is being created.": "⏳ **${name}** is being created.",
  "✅ Project **${name}** created\\n": "✅ Project **${name}** created\\n",
  "⚠️ Invalid argument. Usage: `/autoaccept [on/off/status]`": "⚠️ Invalid argument. Usage: `/autoaccept [on/off/status]`",
  "⚙️ Auto-accept mode: **${this.enabled ? 'ON' : 'OFF'}**": "⚙️ Auto-accept mode: **${this.enabled ? 'ON' : 'OFF'}**",
  "ℹ️ Auto-accept mode is already **ON**.": "ℹ️ Auto-accept mode is already **ON**.",
  "✅ Auto-accept mode turned **ON**. Future dialogs will be auto-allowed.": "✅ Auto-accept mode turned **ON**. Future dialogs will be auto-allowed.",
  "ℹ️ Auto-accept mode is already **OFF**.": "ℹ️ Auto-accept mode is already **OFF**.",
  "✅ Auto-accept mode turned **OFF**. Returned to manual approval.": "✅ Auto-accept mode turned **OFF**. Returned to manual approval.",
  "⚠️ Model name not specified. Available models: ": "⚠️ Model name not specified. Available models: ",
  "⚠️ Invalid model \"${modelName}\". Available models: ${AVAILABLE_MODELS.join(', ')}": "⚠️ Invalid model \"${modelName}\". Available models: ${AVAILABLE_MODELS.join(', ')}",
  "Fast Mode — for simple tasks": "Fast Mode — for simple tasks",
  "Plan Mode — for complex step-by-step tasks": "Plan Mode — for complex step-by-step tasks",
  "⚠️ Mode name not specified. Available modes: ": "⚠️ Mode name not specified. Available modes: ",
  "⚠️ Invalid mode \"${modeName}\". Available modes: ${AVAILABLE_MODES.join(', ')}": "⚠️ Invalid mode \"${modeName}\". Available modes: ${AVAILABLE_MODES.join(', ')}"
}
```

#### B. `internal/i18n/locales/ja.json` ([`locales/ja.json`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/locales/ja.json))
```json
{
  "Display and change execution mode via a dropdown": "実行モードをドロップダウンで表示・変更します",
  "Display and change available LLM models": "利用可能なLLMモデルを表示・変更します",
  "Name of the model to change to": "変更先のモデル名",
  "List, invoke, register, or delete templates": "テンプレートの一覧表示・呼び出し・登録・削除を行います",
  "Display registered template list": "登録済みテンプレート一覧を表示します",
  "Invoke and execute a template": "テンプレートを呼び出して実行します",
  "Template name": "テンプレート名",
  "Register a new template": "新しいテンプレートを登録します",
  "Prompt content of the template": "テンプレートのプロンプト内容",
  "Delete a template": "テンプレートを削除します",
  "Name of the template to delete": "削除するテンプレート名",
  "Interrupt active LLM generation": "動作中のLLM生成を中断します",
  "Capture current Antigravity screen": "Antigravityの現在の画面をキャプチャします",
  "Display overall bot status including connection, model, mode": "Botの接続状態・現在のモデル・モードなどの全体ステータスを表示します",
  "Toggle auto-allow mode for approval dialogs": "承認ダイアログの自動許可モードを切り替えます",
  "on / off / status (default: status)": "on / off / status（省略時はstatus）",
  "List projects, on select auto-create channel and bind": "プロジェクト一覧を表示し、選択するとチャンネルを自動作成してバインドします",
  "Display project list": "プロジェクト一覧を表示します",
  "Create a new project": "新しいプロジェクトを作成します",
  "Name of the project to create": "作成するプロジェクト名",
  "Start a new chat session in the current project": "現在のプロジェクトに新しいチャットセッションを開始します",
  "Display current chat session info and session list": "現在のチャットセッション情報とセッション一覧を表示します",
  "Display list of available commands": "利用可能なコマンド一覧を表示します",
  "❌ Failed to register slash commands:": "❌ スラッシュコマンドの登録に失敗しました:",
  "Auto-approved": "自動承認しました",
  "Auto-approve failed": "自動承認に失敗しました",
  "Antigravity is requesting approval for an action": "Antigravityがアクションの承認を求めています",
  "Auto-approve mode": "自動承認モード",
  "ON": "ON",
  "Workspace": "ワークスペース",
  "Result": "結果",
  "Executed Always Allow/Allow": "常に許可/許可を実行",
  "Manual approval required": "手動承認が必要です",
  "Approval Required": "承認が必要です",
  "Allow button": "許可ボタン",
  "Allow Chat button": "この会話を許可ボタン",
  "In Dropdown": "ドロップダウン内",
  "Deny button": "拒否ボタン",
  "(None)": "(なし)",
  "Allow": "許可",
  "Allow Chat": "この会話を許可",
  "Deny": "拒否",
  "🖼️ Detected generated images (${files.length})": "🖼️ 生成画像を検出しました (${files.length}件)",
  "Generating...": "生成中...",
  "🖼️ Attached image fallback": "🖼️ 添付画像のフォールバック",
  "Failed to attach image directly, resending via URL reference.": "画像の直接添付に失敗したため、URL参照形式で再送します。",
  "⏱️ Elapsed: 0s | Waiting to start": "⏱️ 経過時間: 0秒 | 生成開始待ち",
  "⏱️ Elapsed: ${elapsed}s | Generating": "⏱️ 経過時間: ${elapsed}秒 | 生成中",
  "⏱️ Time: ${elapsed}s | Complete": "⏱️ 所要時間: ${elapsed}秒 | 完了",
  "Failed to extract response. Use `/screenshot` to verify.": "レスポンスの抽出に失敗しました。`/screenshot` で確認してください。",
  "(Untitled)": "(無題)",
  "${timeoutText}\\n\\n[Monitor Ended] Timeout after 5 minutes.": "${timeoutText}\\n\\n[監視終了] 5分経過によりタイムアウトしました。",
  "Error occurred during processing: ${e.message}": "処理中にエラーが発生しました: ${e.message}",
  "You do not have permission.": "権限がありません。",
  "Approval detector not found.": "承認検出器が見つかりません。",
  "⚠️ This command can only be used in a server.": "⚠️ サーバー内でのみ使用できます。",
  "⚠️ Please execute in a text channel.": "⚠️ テキストチャンネルで実行してください。",
  "⚠️ Failed to switch project: ${e.message}": "⚠️ プロジェクト切替に失敗しました: ${e.message}",
  "⚠️ CDP pool is not initialized or cannot connect to workspace.": "⚠️ CDPプールが初期化されていないか、ワークスペースに接続できません。",
  "💬 Started a new session": "💬 新しいセッションを開始しました",
  "Created a new chat session\\n→ <#${newChannelId}>": "新しいチャットセッションを作成しました\\n→ <#${newChannelId}>",
  "Session": "セッション",
  "Project": "プロジェクト",
  "(CDP Disconnected)": "(CDP未接続)",
  "💬 Chat Session Info": "💬 チャットセッション情報",
  "Title": "タイトル",
  "Status": "ステータス",
  "🟢 Active": "🟢 アクティブ",
  "⚪ Inactive": "⚪ 非アクティブ",
  " ← **Current**": " ← **現在**",
  "Current session": "現在のセッション",
  "#${session.sessionNumber} — ${session.displayName || '(Unset)'}": "#${session.sessionNumber} — ${session.displayName || '(未設定)'}",
  "Total sessions": "合計セッション数",
  "**Sessions:**\\n${sessionList}": "**セッション一覧:**\\n${sessionList}",
  "⚠️ Unknown command: /${commandName}": "⚠️ 未知のコマンドです: /${commandName}",
  "✅ Mode changed to **${result.mode}**.": "✅ モードを **${result.mode}** に変更しました。",
  "⚠️ Invalid mode.": "⚠️ 無効なモードです。",
  "📝 No templates registered.": "📝 登録されているテンプレートはありません。",
  "✅ Template **${name}** registered.": "✅ テンプレート「**${name}**」を登録しました。",
  "⚠️ Failed to register template. Name might be duplicated.": "⚠️ テンプレートの登録に失敗しました。名前が重複している可能性があります。",
  "🗑️ Template **${name}** deleted.": "🗑️ テンプレート「**${name}**」を削除しました。",
  "⚠️ Template **${name}** not found.": "⚠️ テンプレート「**${name}**」は見つかりません。",
  "⚠️ Template **${templateName}** not found.": "⚠️ テンプレート「**${templateName}**」は見つかりません。",
  "📝 Invoked template **${templateName}**.\\nStarting process with this prompt.": "📝 テンプレート「**${templateName}**」を呼び出しました。\\nこのプロンプトで処理を開始します。",
  "Select a project to auto-create a category and session channel": "プロジェクトを選択すると、専用カテゴリとセッションチャンネルを自動作成します",
  "Select a project...": "プロジェクトを選択...",
  "Select a project... (Showing 25 of ${workspaces.length})": "プロジェクトを選択... (${workspaces.length}件中25件表示)",
  "⚠️ Project **${workspacePath}** already exists\\n": "⚠️ **${workspacePath}** のプロジェクトは既に作成されています\\n",
  "Full Path": "フルパス",
  "⏳ **${workspacePath}** is being created. Please wait.": "⏳ **${workspacePath}** は現在作成処理中です。しばらくお待ちください。",
  "✅ Project **${workspacePath}** created\\n": "✅ **${workspacePath}** のプロジェクトを作成しました\\n",
  "❌ Invalid project name: ${e.message}": "❌ 無効なプロジェクト名です: ${e.message}",
  "⚠️ Project **${name}** already exists → ${channelLinks}": "⚠️ プロジェクト **${name}** は既に存在します → ${channelLinks}",
  "⏳ **${name}** is being created.": "⏳ **${name}** は現在作成処理中です。",
  "✅ Project **${name}** created\\n": "✅ **${name}** のプロジェクトを作成しました\\n",
  "⚠️ Invalid argument. Usage: `/autoaccept [on/off/status]`": "⚠️ 無効な引数です。使用方法: `/autoaccept [on/off/status]`",
  "⚙️ Auto-accept mode: **${this.enabled ? 'ON' : 'OFF'}**": "⚙️ 自動承認モード: **${this.enabled ? 'ON' : 'OFF'}**",
  "ℹ️ Auto-accept mode is already **ON**.": "ℹ️ 自動承認モードはすでに **ON** です。",
  "✅ Auto-accept mode turned **ON**. Future dialogs will be auto-allowed.": "✅ 自動承認モードを **ON** にしました。以後の承認ダイアログは自動で許可します。",
  "ℹ️ Auto-accept mode is already **OFF**.": "ℹ️ 自動承認モードはすでに **OFF** です。",
  "✅ Auto-accept mode turned **OFF**. Returned to manual approval.": "✅ 自動承認モードを **OFF** にしました。以後は手動承認に戻ります。",
  "⚠️ Model name not specified. Available models: ": "⚠️ モデル名が指定されていません。利用可能なモデル: ",
  "⚠️ Invalid model \"${modelName}\". Available models: ${AVAILABLE_MODELS.join(', ')}": "⚠️ 無効なモデル \"${modelName}\" です。利用可能なモデル: ${AVAILABLE_MODELS.join(', ')}",
  "Fast Mode — for simple tasks": "高速応答モード — シンプルなタスク向け",
  "Plan Mode — for complex step-by-step tasks": "計画モード — 複雑なタスクを段階的に実行",
  "⚠️ Mode name not specified. Available modes: ": "⚠️ モード名が指定されていません。利用可能なモード: ",
  "⚠️ Invalid mode \"${modeName}\". Available modes: ${AVAILABLE_MODES.join(', ')}": "⚠️ 無効なモード \"${modeName}\" です。利用可能なモード: ${AVAILABLE_MODES.join(', ')}"
}
```

#### C. `internal/i18n/locales/vi.json` ([`locales/vi.json`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/locales/vi.json))
```json
{
  "❌ Failed to register slash commands:": "❌ Không thể đăng ký các lệnh slash:",
  "Auto-approved": "Tự động phê duyệt",
  "Auto-approve failed": "Tự động phê duyệt thất bại",
  "Antigravity is requesting approval for an action": "Antigravity đang yêu cầu phê duyệt cho một hành động",
  "Auto-approve mode": "Chế độ tự động phê duyệt",
  "ON": "BẬT",
  "Workspace": "Không gian làm việc",
  "Result": "Kết quả",
  "Executed Always Allow/Allow": "Thực thi Luôn cho phép/Cho phép",
  "Manual approval required": "Cần phê duyệt thủ công",
  "Approval Required": "Cần phê duyệt",
  "Allow button": "Nút Cho phép",
  "Allow Chat button": "Nút Cho phép Cuộc hội thoại này",
  "In Dropdown": "Trong Dropdown",
  "Deny button": "Nút Từ chối",
  "(None)": "(Không)",
  "Allow": "Cho phép",
  "Allow Chat": "Cho phép Cuộc hội thoại này",
  "Deny": "Từ chối",
  "🖼️ Detected generated images (${files.length})": "🖼️ Phát hiện hình ảnh được tạo (${files.length})",
  "Generating...": "Đang tạo...",
  "🖼️ Attached image fallback": "🖼️ Fallback hình ảnh được đính kèm",
  "Failed to attach image directly, resending via URL reference.": "Không thể đính kèm hình ảnh trực tiếp, gửi lại qua tham chiếu URL.",
  "⏱️ Elapsed: 0s | Waiting to start": "⏱️ Đã trôi qua: 0s | Chờ bắt đầu",
  "⏱️ Elapsed: ${elapsed}s | Generating": "⏱️ Đã trôi qua: ${elapsed}s | Đang tạo",
  "⏱️ Time: ${elapsed}s | Complete": "⏱️ Thời gian: ${elapsed}s | Hoàn thành",
  "Failed to extract response. Use `/screenshot` to verify.": "Không thể trích xuất phản hồi. Sử dụng `/screenshot` để xác minh.",
  "No projects found.": "Không tìm thấy dự án nào.",
  "(Untitled)": "(Không có tiêu đề)",
  "${timeoutText}\\n\\n[Monitor Ended] Timeout after 5 minutes.": "${timeoutText}\\n\\n[Giám sát kết thúc] Hết thời gian chờ sau 5 phút.",
  "Error occurred during processing: ${e.message}": "Lỗi xảy ra trong quá trình xử lý: ${e.message}",
  "You do not have permission.": "Bạn không có quyền.",
  "Approval detector not found.": "Không tìm thấy trình phát hiện phê duyệt.",
  "⚠️ This command can only be used in a server.": "⚠️ Lệnh này chỉ có thể được sử dụng trên máy chủ.",
  "⚠️ Please execute in a text channel.": "⚠️ Vui lòng thực thi trong một channel văn bản.",
  "⚠️ Failed to switch project: ${e.message}": "⚠️ Không thể chuyển đổi dự án: ${e.message}",
  "⚠️ CDP pool is not initialized or cannot connect to workspace.": "⚠️ CDP pool chưa được khởi tạo hoặc không thể kết nối tới không gian làm việc.",
  "💬 Started a new session": "💬 Bắt đầu phiên làm việc mới",
  "Created a new chat session\\n→ <#${newChannelId}>": "Tạo một phiên chat mới\\n→ <#${newChannelId}>",
  "Session": "Phiên",
  "Project": "Dự án",
  "(CDP Disconnected)": "(CDP Đã ngắt kết nối)",
  "💬 Chat Session Info": "💬 Thông tin Phiên Chat",
  "Title": "Tiêu đề",
  "Status": "Trạng thái",
  "🟢 Active": "🟢 Hoạt động",
  "⚪ Inactive": "⚪ Không hoạt động",
  " ← **Current**": " ← **Hiện tại**",
  "Current session": "Phiên hiện tại",
  "#${session.sessionNumber} — ${session.displayName || '(Unset)'}": "#${session.sessionNumber} — ${session.displayName || '(Chưa đặt)'}",
  "Total sessions": "Tổng số phiên",
  "**Sessions:**\\n${sessionList}": "**Các phiên:**\\n${sessionList}",
  "⚠️ Unknown command: /${commandName}": "⚠️ Lệnh không xác định: /${commandName}",
  "✅ Mode changed to **${result.mode}**.": "✅ Chế độ đã thay đổi thành **${result.mode}**.",
  "⚠️ Invalid mode.": "⚠️ Chế độ không hợp lệ.",
  "📝 No templates registered.": "📝 Không có mẫu nào được đăng ký.",
  "✅ Template **${name}** registered.": "✅ Mẫu **${name}** đã được đăng ký.",
  "⚠️ Failed to register template. Name might be duplicated.": "⚠️ Không thể đăng ký mẫu. Tên có thể bị trùng lặp.",
  "🗑️ Template **${name}** deleted.": "🗑️ Mẫu **${name}** đã bị xóa.",
  "⚠️ Template **${name}** not found.": "⚠️ Không tìm thấy mẫu **${name}**.",
  "⚠️ Template **${templateName}** not found.": "⚠️ Không tìm thấy mẫu **${templateName}**.",
  "📝 Invoked template **${templateName}**.\\nStarting process with this prompt.": "📝 Gọi mẫu **${templateName}**.\\nBắt đầu quá trình với lời nhắc này.",
  "Select a project...": "Chọn một dự án...",
  "Select a project... (Showing 25 of ${workspaces.length})": "Chọn một dự án... (Hiển thị 25 của ${workspaces.length})",
  "⚠️ Project **${workspacePath}** already exists\\n": "⚠️ Dự án **${workspacePath}** đã tồn tại\\n",
  "Full Path": "Đường dẫn đầy đủ",
  "⏳ **${workspacePath}** is being created. Please wait.": "⏳ **${workspacePath}** đang được tạo. Vui lòng chờ.",
  "✅ Project **${workspacePath}** created\\n": "✅ Dự án **${workspacePath}** đã được tạo\\n",
  "❌ Invalid project name: ${e.message}": "❌ Tên dự án không hợp lệ: ${e.message}",
  "⚠️ Project **${name}** already exists → ${channelLinks}": "⚠️ Dự án **${name}** đã tồn tại → ${channelLinks}",
  "⏳ **${name}** is being created.": "⏳ **${name}** đang được tạo.",
  "✅ Project **${name}** created\\n": "✅ Dự án **${name}** đã được tạo\\n",
  "⚠️ Invalid argument. Usage: `/autoaccept [on/off/status]`": "⚠️ Tham số không hợp lệ. Cách sử dụng: `/autoaccept [on/off/status]`",
  "⚙️ Auto-accept mode: **${this.enabled ? 'ON' : 'OFF'}**": "⚙️ Chế độ tự động chấp nhận: **${this.enabled ? 'BẬT' : 'TẮT'}**",
  "ℹ️ Auto-accept mode is already **ON**.": "ℹ️ Chế độ tự động chấp nhận đã ở chế độ **BẬT**.",
  "✅ Auto-accept mode turned **ON**. Future dialogs will be auto-allowed.": "✅ Chế độ tự động chấp nhận đã được bật **BẬT**. Các hộp thoại trong tương lai sẽ được tự động cho phép.",
  "ℹ️ Auto-accept mode is already **OFF**.": "ℹ️ Chế độ tự động chấp nhận đã ở chế độ **TẮT**.",
  "✅ Auto-accept mode turned **OFF**. Returned to manual approval.": "✅ Chế độ tự động chấp nhận đã được tắt **TẮT**. Quay lại phê duyệt thủ công.",
  "⚠️ Model name not specified. Available models: ": "⚠️ Tên mô hình không được chỉ định. Các mô hình có sẵn: ",
  "⚠️ Invalid model \"${modelName}\". Available models: ${AVAILABLE_MODELS.join(', ')}": "⚠️ Mô hình \"${modelName}\" không hợp lệ. Các mô hình có sẵn: ${AVAILABLE_MODELS.join(', ')}",
  "Fast Mode — for simple tasks": "Chế độ nhanh — cho các tác vụ đơn giản",
  "Plan Mode — for complex step-by-step tasks": "Chế độ kế hoạch — cho các tác vụ phức tạp từng bước",
  "⚠️ Mode name not specified. Available modes: ": "⚠️ Tên chế độ không được chỉ định. Các chế độ có sẵn: ",
  "⚠️ Invalid mode \"${modeName}\". Available modes: ${AVAILABLE_MODES.join(', ')}": "⚠️ Chế độ \"${modeName}\" không hợp lệ. Các chế độ có sẵn: ${AVAILABLE_MODES.join(', ')}",
  "Projects": "Dự án",
  "Select a project to auto-create a topic and session": "Chọn một dự án để tự động tạo topic và phiên làm việc",
  "Page": "Trang",
  "projects total": "tổng số dự án",
  "Prev": "Trước",
  "Next": "Tiếp",
  "Join Session": "Tham gia phiên",
  "No sessions found in the Antigravity side panel.": "Không tìm thấy phiên nào trong bảng điều khiển của Antigravity.",
  "Select a session to join ({{count}} found)": "Chọn một phiên để tham gia (tìm thấy {{count}} phiên)",
  "Welcome message": "Tin nhắn chào mừng",
  "Show all commands": "Hiển thị tất cả lệnh",
  "Select a project": "Chọn một dự án",
  "Start a new chat session": "Bắt đầu phiên chat mới",
  "Current session info": "Thông tin phiên hiện tại",
  "Change execution mode": "Thay đổi chế độ thực thi",
  "Change LLM model": "Thay đổi mô hình LLM",
  "Interrupt active generation": "Dừng việc tạo kết quả",
  "Capture Antigravity screen": "Chụp màn hình Antigravity",
  "Show prompt templates": "Hiển thị mẫu lời nhắc",
  "Register a template": "Đăng ký một mẫu",
  "Delete a template": "Xóa một mẫu",
  "Toggle auto-approve mode": "Bật/tắt chế độ tự động duyệt",
  "Bot status overview": "Tổng quan trạng thái bot",
  "Check latency": "Kiểm tra độ trễ",
  "Current Mode": "Chế độ hiện tại",
  "Model": "Mô hình",
  "Fast Model": "Mô hình Nhanh",
  "Plan Model": "Mô hình Kế hoạch",
  "...(beginning truncated)": "...(phần đầu được lược bớt)"
}
```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo de Resolução de Caminhos e Carregamento de Configuração (`internal/config/loader.go`)

1. **Caminhos Base**:
   - `GetConfigDir()`: obtém o diretório home do usuário (`os.UserHomeDir()`) e concatena `.antigravity-telegram-remote` (`filepath.Join(home, ".antigravity-telegram-remote")`).
   - `GetConfigFilePath()`: `filepath.Join(GetConfigDir(), "config.json")`.
   - `GetDefaultDbPath()`: `filepath.Join(GetConfigDir(), "antigravity.db")`.
   - `ConfigExists()`: retorna `true` se `GetConfigFilePath()` existir no sistema de arquivos.
   - `EnsureConfigDirExists()`: se `GetConfigDir()` não existir, cria com `os.MkdirAll(dir, 0755)`.

2. **Leitura de Arquivo Persistido (`readPersistedConfig(filePath)`)**:
   - Se o arquivo `filePath` não existir (`!StatExists(filePath)`), retorna `PersistedConfig{}` vazio sem erro.
   - Lê os bytes de `filePath` e executa `json.Unmarshal`.
   - Se `json.Unmarshal` falhar, retorna erro com a mensagem exata:
     `fmt.Errorf("Failed to parse config file at %s. Check for syntax errors.", filePath)`.

3. **Expansão de Tilde (`expandTilde(raw string)`)**:
   - Se `raw == "~"`, retorna `homeDir`.
   - Se `strings.HasPrefix(raw, "~/")`, retorna `filepath.Join(homeDir, raw[2:])`.
   - Caso contrário, retorna `raw` inalterado.

4. **Mesclagem e Validação (`mergeConfig(persisted PersistedConfig)`)**:
   - **`TelegramBotToken`**:
     - Verifica se `TELEGRAM_BOT_TOKEN` está definido no ambiente (se a variável existir e não for vazia, ou se definida no `.env`). No TS original: `const token = process.env.TELEGRAM_BOT_TOKEN ?? persisted.telegramBotToken; if (!token) throw new Error('Missing required config: TELEGRAM_BOT_TOKEN')`.
     - Em Go: se `envVal, ok := lookupEnv("TELEGRAM_BOT_TOKEN"); ok`, usa `envVal`; senão se `persisted.TelegramBotToken != nil`, usa `*persisted.TelegramBotToken`. Se a string resultante for `""`, retorna erro `"Missing required config: TELEGRAM_BOT_TOKEN"`.
   - **`AllowedUserIds` (`resolveAllowedUserIds`)**:
     - Se `envVal, ok := lookupEnv("ALLOWED_USER_IDS"); ok && envVal != ""`:
       - Divide `envVal` por `","`.
       - Aplica `strings.TrimSpace(id)` em cada elemento.
       - Filtra apenas elementos que casam com `^\d+$` e `len(id) > 0`.
     - Caso contrário, se `len(persisted.AllowedUserIds) > 0`:
       - Filtra `persisted.AllowedUserIds` mantendo apenas elementos que casam com `^\d+$`.
     - Se a fatia resultante tiver `len == 0`, retorna erro `"Missing required config: ALLOWED_USER_IDS"`.
   - **`WorkspaceBaseDir`**:
     - `defaultDir := filepath.Join(homeDir, "Code")`.
     - Se `WORKSPACE_BASE_DIR` estiver definido no ambiente (`ok`), usa seu valor; senão se `persisted.WorkspaceBaseDir != nil`, usa `*persisted.WorkspaceBaseDir`; senão usa `defaultDir`.
     - Aplica `expandTilde(rawDir)`.
   - **`AutoApproveFileEdits`**:
     - `resolveBoolean(env["AUTO_APPROVE_FILE_EDITS"], persisted.AutoApproveFileEdits, false)`.
     - Regra de `resolveBoolean`: se a variável de ambiente estiver definida (`ok`), retorna `strings.ToLower(envValue) == "true"`; senão se `persistedValue != nil`, retorna `*persistedValue`; senão retorna `defaultValue`.
   - **`LogLevel` (`resolveLogLevel`)**:
     - Se `LOG_LEVEL` estiver definido no ambiente (`ok`), toma `strings.ToLower(envValue)`; senão se `persisted.LogLevel != nil`, toma `string(*persisted.LogLevel)`.
     - Se o valor pertencer a `['debug', 'info', 'warn', 'error', 'none']`, retorna-o como `logger.LogLevel`; caso contrário, retorna `logger.LevelInfo` (`"info"`).
   - **`ExtractionMode` (`resolveExtractionMode`)**:
     - Se `EXTRACTION_MODE` estiver definido no ambiente (`ok`), toma `envValue`; senão se `persisted.ExtractionMode != nil`, toma `string(*persisted.ExtractionMode)`.
     - Se `raw == "legacy"`, retorna `ExtractionModeLegacy` (`"legacy"`); caso contrário, retorna `ExtractionModeStructured` (`"structured"`).
   - **`UseTopics`**:
     - `resolveBoolean(env["USE_TOPICS"], persisted.UseTopics, true)`.

5. **Salvamento Parcial (`Save(partial PersistedConfig)`)**:
   - Garante que `GetConfigDir()` exista (`MkdirAll` recursivo).
   - Lê a configuração existente via `readPersistedConfig(GetConfigFilePath())`.
   - Sobrescreve no objeto `existing` apenas os campos não-nulos de `partial` (merge superficial `...existing, ...config`).
   - Serializa com indentação de 2 espaços (`json.MarshalIndent(merged, "", "  ")`) seguido de `"\n"` no final e grava em `GetConfigFilePath()` em UTF-8.

---

### 4.2. Algoritmo de Formatação e Prioridade do Logger (`internal/logger/logger.go`)

1. **Verificação de Prioridade (`shouldLog(methodLevel)`)**:
   - Cada nível possui peso numérico: `debug=0`, `info=1`, `warn=2`, `error=3`, `none=4`.
   - Um método deve emitir log se e somente se `levelPriority[methodLevel] >= levelPriority[currentLevel]`.
   - Quando `currentLevel == "none"` (`4`), nenhum método emite saída (pois o máximo de `methodLevel` é `error=3`).
2. **Timestamp (`getTimestamp()`)**:
   - Formata o horário local atual em 24 horas no formato `HH:MM:SS` (`now.Format("15:04:05")`, equivalente a `now.toLocaleTimeString('ja-JP', { hour12: false })`).
   - Envolve com a cor `dim`: `fmt.Sprintf("%s[%s]%s", COLORS.Dim, timeString, COLORS.Reset)`.
3. **Formatação das Mensagens**:
   - `Info`, `Phase`, `Done` escrevem em `InfoOut` (`os.Stdout`).
   - `Warn` escreve em `WarnOut` (`os.Stderr` ou writer configurado).
   - `Error` escreve em `ErrorOut` (`os.Stderr`).
   - `Debug` escreve em `DebugOut` (`os.Stdout`).
   - O prefixo de cada linha (exceto `Divider`) é `<timestamp> <color>[<TAG>]<reset>` seguido de espaço e os argumentos unidos por espaço (`fmt.Sprintln`).
4. **Formatação de `Divider(label ...string)`**:
   - Verifica `shouldLog(LevelInfo)`.
   - Se `label` for fornecido e não-vazio:
     - Calcula `pad := 50 - len(label) - 4` (usando contagem de código UTF-16/runes compatível; se `pad < 4`, `pad = 4`).
     - Imprime: `fmt.Fprintf(infoOut, "%s[DONE]%s %s── %s %s%s\n", COLORS.Green, COLORS.Reset, COLORS.Dim, label, strings.Repeat("─", pad), COLORS.Reset)`.
   - Se `label` for omitido ou vazio:
     - Imprime: `fmt.Fprintf(infoOut, "%s[DONE]%s %s%s%s\n", COLORS.Green, COLORS.Reset, COLORS.Dim, strings.Repeat("─", 50), COLORS.Reset)`.

---

### 4.3. Algoritmo de Tradução e Interpolação (`internal/i18n/i18n.go`)

1. **Estado Inicial e `InitI18n`**:
   - Na inicialização do pacote, `currentLanguage` começa como `'vi'`.
   - Nota importante de compatibilidade com os testes unitários originais ([`tests/utils/i18n.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/i18n.test.ts#L73-L84)):
     - Em `src/utils/i18n.ts`: `export function initI18n(lang: Language = 'vi')`.
     - No teste `tests/utils/i18n.test.ts` linha 74, o teste chama `initI18n()` sem argumentos e verifica que `t('test.key')` retorna `'test.key'` sem lançar exceção. Portanto, `InitI18n(lang ...Language)` deve usar `LangVI` (`"vi"`) quando nenhum parâmetro for informado.
   - `loadTranslations()` carrega os 3 JSONs (`en.json`, `ja.json`, `vi.json`). Se um arquivo não existir, inicializa o mapa daquele idioma como vazio `{}`. Se ocorrer erro de leitura/parse, loga com `logger.Error("Failed to load translations:", err)`.

2. **Resolução de Chave e Fallback em `T(key string, variables ...map[string]any)`**:
   - Se `key == ""`, retorna `""`.
   - Obtém `langDict := translations[currentLanguage]`. Se `langDict` for `nil` ou vazio, usa `translations[LangEN]`.
   - Procura `text := langDict[key]`.
   - Se `text == ""` (não encontrado no idioma atual), procura `text = translations[LangEN][key]`.
   - Se ainda não encontrado (`text == ""`), faz fallback para a própria `key`: `text = key`.
   - **Interpolação de Variáveis**:
     - Se `variables` for fornecido e não-vazio:
       - Para cada par `(vKey, vValue)` no mapa:
         - Converte `vValue` para string (`<nil>` / `null` $\rightarrow$ `"null"`, booleano `true` $\rightarrow$ `"true"`, `false` $\rightarrow$ `"false"`, números via `fmt.Sprint(vValue)`).
         - Executa substituição literal (não-regex) de todas as ocorrências:
           `text = strings.ReplaceAll(text, "{{"+vKey+"}}", strVal)`.
         - Usar `strings.ReplaceAll` (e **nunca** `regexp`) garante que chaves com caracteres especiais como `{{file.name}}` sejam substituídas literalmente sem interpretação de metacaracteres regex.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Thread-Safety no `Logger`**:
   - Como goroutines de múltiplos detectores CDP, cron jobs e handlers do Telegram chamam `logger.Info/Debug/Warn/Error` concorrentemente enquanto flags da CLI ou comandos podem chamar `logger.SetLogLevel`, o campo `currentLevel` e a escrita nos streams são protegidos por `sync.RWMutex` (`RLock` em `shouldLog`/`GetLogLevel`, `Lock` em `SetLogLevel` e na escrita de linha atômica).
2. **Thread-Safety no `i18n`**:
   - O mapa `translations` e `currentLanguage` são protegidos por `sync.RWMutex`: `InitI18n` adquire `Lock()`, enquanto chamadas concorrentes a `T()` adquirem `RLock()`.
3. **Ciclo de Vida do `ConfigLoader`**:
   - No startup do binário, `LoadDotEnv(".env")` é chamado (com `override: true`, sobrescrevendo variáveis de ambiente caso `.env` exista no diretório de trabalho atual) antes de `ConfigLoader.Load()`.
   - Durante o wizard `setup`, `ConfigLoader.Save(...)` persiste `~/.antigravity-telegram-remote/config.json`.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os cenários abaixo mapeiam **1 para 1** cada bloco `describe` e `it` dos 5 arquivos de teste originais e devem passar com `go test -v -race ./internal/config/... ./internal/logger/... ./internal/i18n/...`.

### 6.1. `internal/config/config_test.go` (Mapeado de [`tests/config.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/config.test.ts) e [`tests/utils/config.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/config.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestConfig_ThrowsIfTelegramBotTokenMissing` | `Config > throws an error if TELEGRAM_BOT_TOKEN is missing` | `TELEGRAM_BOT_TOKEN` ausente, `ALLOWED_USER_IDS="123456"`, `persisted={}` | `LoadConfig()` retorna erro contendo `"Missing required config: TELEGRAM_BOT_TOKEN"`. |
| `TestConfig_ThrowsIfAllowedUserIdsMissing` | `Config > throws an error if ALLOWED_USER_IDS is missing` | `TELEGRAM_BOT_TOKEN="token"`, `ALLOWED_USER_IDS` ausente, `persisted={}` | `LoadConfig()` retorna erro contendo `"Missing required config: ALLOWED_USER_IDS"`. |
| `TestConfig_ReturnsValidConfigWhenAllSet` | `Config > returns valid config if all required variables are set` | `TELEGRAM_BOT_TOKEN="secret_token"`, `ALLOWED_USER_IDS="123456789,987654321"`, `WORKSPACE_BASE_DIR="/custom/dir"` | `cfg.TelegramBotToken == "secret_token"`, `cfg.AllowedUserIds == []string{"123456789", "987654321"}`, `cfg.WorkspaceBaseDir == "/custom/dir"`. |
| `TestConfig_DefaultWorkspaceBaseDir` | `Config > returns default workspace base dir if not set` | `TELEGRAM_BOT_TOKEN="secret_token"`, `ALLOWED_USER_IDS="123456789"`, sem `WORKSPACE_BASE_DIR` | `cfg.WorkspaceBaseDir` não é vazio e é `filepath.Join(home, "Code")`. |
| `TestConfig_DefaultsUseTopicsToTrue` | `Config > defaults useTopics to true when not set` | Sem `USE_TOPICS` | `cfg.UseTopics == true`. |
| `TestConfig_DisablesUseTopicsWhenFalse` | `Config > disables useTopics when set to false` | `USE_TOPICS="false"` | `cfg.UseTopics == false`. |
| `TestConfig_DefaultsAutoApproveFileEditsToFalse` | `Config > defaults AUTO_APPROVE_FILE_EDITS to false when not set` | Sem `AUTO_APPROVE_FILE_EDITS` | `cfg.AutoApproveFileEdits == false`. |
| `TestConfig_EnablesAutoApproveFileEditsWhenTrue` | `Config > enables AUTO_APPROVE_FILE_EDITS when set to true` | `AUTO_APPROVE_FILE_EDITS="true"` | `cfg.AutoApproveFileEdits == true`. |
| `TestConfig_ResolveResponseDeliveryMode` | `Config > returns stream as the only supported delivery mode` & `config > resolveResponseDeliveryMode() > always returns "stream"` | Chamada direta a `ResolveResponseDeliveryMode()` | Retorna `ResponseDeliveryModeStream` (`"stream"`). |

---

### 6.2. `internal/config/loader_test.go` (Mapeado de [`tests/utils/configLoader.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/configLoader.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestConfigLoader_GetConfigDir` | `ConfigLoader > getConfigDir() > returns ~/.antigravity-telegram-remote` | `home = os.UserHomeDir()` | Retorna `filepath.Join(home, ".antigravity-telegram-remote")`. |
| `TestConfigLoader_GetConfigFilePath` | `ConfigLoader > getConfigFilePath() > returns ~/.antigravity-telegram-remote/config.json` | `home = os.UserHomeDir()` | Retorna `filepath.Join(home, ".antigravity-telegram-remote", "config.json")`. |
| `TestConfigLoader_GetDefaultDbPath` | `ConfigLoader > getDefaultDbPath() > returns ~/.antigravity-telegram-remote/antigravity.db` | `home = os.UserHomeDir()` | Retorna `filepath.Join(home, ".antigravity-telegram-remote", "antigravity.db")`. |
| `TestConfigLoader_ConfigExists_True` | `ConfigLoader > configExists() > returns true when config file exists` | Mock `StatExists` retornando `true` | `ConfigExists() == true`. |
| `TestConfigLoader_ConfigExists_False` | `ConfigLoader > configExists() > returns false when config file does not exist` | Mock `StatExists` retornando `false` | `ConfigExists() == false`. |
| `TestConfigLoader_Load_FromPersistedOverride` | `ConfigLoader > load() > loads config from persisted override` | `PersistedConfig{TelegramBotToken: ptr("test-token"), AllowedUserIds: []string{"111", "222"}}` | `cfg.TelegramBotToken == "test-token"`, `cfg.AllowedUserIds == []string{"111", "222"}`. |
| `TestConfigLoader_Load_EnvOverridesPersisted` | `ConfigLoader > load() > env vars override persisted config` | Env `TELEGRAM_BOT_TOKEN="env-token"`, `ALLOWED_USER_IDS="999"`; Persisted `file-token`, `["111"]` | `cfg.TelegramBotToken == "env-token"`, `cfg.AllowedUserIds == []string{"999"}`. |
| `TestConfigLoader_Load_ThrowsWhenTokenMissing` | `ConfigLoader > load() > throws when telegramBotToken is missing` | `Load(&PersistedConfig{})` | Erro `"Missing required config: TELEGRAM_BOT_TOKEN"`. |
| `TestConfigLoader_Load_ThrowsWhenAllowedUserIdsMissing` | `ConfigLoader > load() > throws when allowedUserIds is missing` | `Load(&PersistedConfig{TelegramBotToken: ptr("tok")})` | Erro `"Missing required config: ALLOWED_USER_IDS"`. |
| `TestConfigLoader_Load_DefaultWorkspaceBaseDir` | `ConfigLoader > load() > uses default workspaceBaseDir when not specified` | Token `"tok"`, IDs `["1"]` | `cfg.WorkspaceBaseDir == filepath.Join(home, "Code")`. |
| `TestConfigLoader_Load_ExpandsTildeInWorkspaceBaseDir` | `ConfigLoader > load() > expands tilde in workspaceBaseDir` | `WorkspaceBaseDir: ptr("~/projects")` | `cfg.WorkspaceBaseDir == filepath.Join(home, "projects")`. |
| `TestConfigLoader_Load_ResolvesAutoApproveFromEnv` | `ConfigLoader > load() > resolves autoApproveFileEdits from env` | Env `AUTO_APPROVE_FILE_EDITS="true"` | `cfg.AutoApproveFileEdits == true`. |
| `TestConfigLoader_Load_DefaultsAutoApproveToFalse` | `ConfigLoader > load() > defaults autoApproveFileEdits to false` | Sem env/persisted | `cfg.AutoApproveFileEdits == false`. |
| `TestConfigLoader_Load_ResolvesLogLevelFromEnv` | `ConfigLoader > load() > resolves logLevel from env` | Env `LOG_LEVEL="debug"` | `cfg.LogLevel == logger.LevelDebug` (`"debug"`). |
| `TestConfigLoader_Load_DefaultsLogLevelToInfo` | `ConfigLoader > load() > defaults logLevel to info` | Sem env/persisted | `cfg.LogLevel == logger.LevelInfo` (`"info"`). |
| `TestConfigLoader_Load_ResolvesExtractionModeFromEnv` | `ConfigLoader > load() > resolves extractionMode from env` | Env `EXTRACTION_MODE="legacy"` | `cfg.ExtractionMode == ExtractionModeLegacy` (`"legacy"`). |
| `TestConfigLoader_Load_DefaultsExtractionModeToStructured` | `ConfigLoader > load() > defaults extractionMode to structured` | Sem env/persisted | `cfg.ExtractionMode == ExtractionModeStructured` (`"structured"`). |
| `TestConfigLoader_Load_ResolvesUseTopicsFromEnv` | `ConfigLoader > load() > resolves useTopics from env` | Env `USE_TOPICS="false"` | `cfg.UseTopics == false`. |
| `TestConfigLoader_Load_DefaultsUseTopicsToTrue` | `ConfigLoader > load() > defaults useTopics to true` | Sem env/persisted | `cfg.UseTopics == true`. |
| `TestConfigLoader_Load_CommaSeparatedAllowedUserIdsWithWhitespace` | `ConfigLoader > load() > handles comma-separated ALLOWED_USER_IDS with whitespace` | Env `ALLOWED_USER_IDS=" 111 , 222 , 333 "` | `cfg.AllowedUserIds == []string{"111", "222", "333"}`. |
| `TestConfigLoader_Load_FiltersEmptyStringsFromAllowedUserIds` | `ConfigLoader > load() > filters empty strings from ALLOWED_USER_IDS` | Env `ALLOWED_USER_IDS="111,,222,"` | `cfg.AllowedUserIds == []string{"111", "222"}`. |
| `TestConfigLoader_Load_ThrowsClearErrorForMalformedJSON` | `ConfigLoader > load() > throws a clear error for malformed JSON config` | Mock `StatExists=true`, `ReadFile="{ invalid json }"` | Retorna erro contendo `"Failed to parse config file"`. |
| `TestConfigLoader_Load_EmptyPersistedWhenFileDoesNotExist` | `ConfigLoader > load() > returns empty persisted config when file does not exist` | Mock `StatExists=false`, Env `TELEGRAM_BOT_TOKEN="tok"`, `ALLOWED_USER_IDS="1"` | `cfg.TelegramBotToken == "tok"`. |
| `TestConfigLoader_Load_IgnoresInvalidLogLevelValues` | `ConfigLoader > load() > ignores invalid logLevel values` | Env `LOG_LEVEL="INVALID"` | `cfg.LogLevel == logger.LevelInfo` (`"info"`). |
| `TestConfigLoader_Save_CreatesDirAndWritesMergedConfig` | `ConfigLoader > save() > creates directory and writes merged config` | Mock `StatExists=false`, chama `Save(PersistedConfig{TelegramBotToken: ptr("new-token")})` | `MkdirAll` chamado para `.antigravity-telegram-remote`; `WriteFile` chamado para `config.json` contendo `"new-token"`. |

---

### 6.3. `internal/logger/logger_test.go` (Mapeado de [`tests/utils/logger.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/logger.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestLogger_DefaultLevelInfo` | `createLogger > default level (info) > outputs info, warn, error but not debug` | `CreateLogger(LevelInfo)` + chamadas `Info`, `Warn`, `Error`, `Debug` | `InfoOut`, `WarnOut`, `ErrorOut` recebem 1 chamada cada; `DebugOut` recebe 0 chamadas. |
| `TestLogger_DebugLevel` | `createLogger > debug level > outputs all log levels including debug` | `CreateLogger(LevelDebug)` + chamadas `Debug`, `Info`, `Warn`, `Error` | Todos os 4 writers recebem 1 chamada cada. |
| `TestLogger_WarnLevel` | `createLogger > warn level > outputs only warn and error` | `CreateLogger(LevelWarn)` | Apenas `WarnOut` e `ErrorOut` recebem 1 chamada; `DebugOut` e `InfoOut` vazios. |
| `TestLogger_ErrorLevel` | `createLogger > error level > outputs only error` | `CreateLogger(LevelError)` | Apenas `ErrorOut` recebe 1 chamada; demais vazios. |
| `TestLogger_NoneLevel` | `createLogger > none level > suppresses all output` | `CreateLogger(LevelNone)` + `Debug`, `Info`, `Warn`, `Error`, `Phase`, `Done`, `Divider("section")` | Todos os writers permanecem vazios (0 chamadas). |
| `TestLogger_SetLogLevel_RuntimeChange` | `createLogger > setLogLevel > changes the log level at runtime` | Inicia em `info`, chama `Debug` (0 saídas), `SetLogLevel(LevelDebug)`, chama `Debug` | `DebugOut` passa a ter 1 saída. |
| `TestLogger_SetLogLevel_EscalateDebugToError` | `createLogger > setLogLevel > can escalate from debug to error` | Inicia em `debug`, chama `Info` (1 saída), `SetLogLevel(LevelError)`, chama `Info` | `InfoOut` continua com apenas 1 saída. |
| `TestLogger_GetLogLevel` | `createLogger > getLogLevel > returns the current log level` | Inicia em `warn`, depois `SetLogLevel(LevelDebug)` | `GetLogLevel()` retorna `"warn"` e depois `"debug"`. |
| `TestLogger_Helpers_PhaseAtInfo` | `createLogger > phase, done, divider > phase outputs at info level` | `CreateLogger(LevelInfo)` $\rightarrow$ `Phase("starting phase")` | `InfoOut` recebe 1 chamada. |
| `TestLogger_Helpers_PhaseSuppressedAtWarn` | `createLogger > phase, done, divider > phase is suppressed at warn level` | `CreateLogger(LevelWarn)` $\rightarrow$ `Phase("starting phase")` | `InfoOut` vazio. |
| `TestLogger_Helpers_DoneAtInfo` | `createLogger > phase, done, divider > done outputs at info level` | `CreateLogger(LevelInfo)` $\rightarrow$ `Done("completed")` | `InfoOut` recebe 1 chamada. |
| `TestLogger_Helpers_DoneSuppressedAtWarn` | `createLogger > phase, done, divider > done is suppressed at warn level` | `CreateLogger(LevelWarn)` $\rightarrow$ `Done("completed")` | `InfoOut` vazio. |
| `TestLogger_Helpers_DividerWithLabelAtInfo` | `createLogger > phase, done, divider > divider with label outputs at info level` | `CreateLogger(LevelInfo)` $\rightarrow$ `Divider("Section")` | `InfoOut` recebe 1 chamada. |
| `TestLogger_Helpers_DividerWithoutLabelAtInfo` | `createLogger > phase, done, divider > divider without label outputs at info level` | `CreateLogger(LevelInfo)` $\rightarrow$ `Divider()` | `InfoOut` recebe 1 chamada. |
| `TestLogger_Helpers_DividerSuppressedAtError` | `createLogger > phase, done, divider > divider is suppressed at error level` | `CreateLogger(LevelError)` $\rightarrow$ `Divider("Section")` | `InfoOut` vazio. |
| `TestLogger_Format_InfoTagAndColor` | `createLogger > console output format > info includes [INFO] tag with cyan color` | `Info("test message")` | Saída contém `"[INFO]"` e `COLORS.Cyan` (`"\x1b[36m"`). |
| `TestLogger_Format_WarnTagAndColor` | `createLogger > console output format > warn includes [WARN] tag with yellow color` | `Warn("test message")` | Saída contém `"[WARN]"` e `COLORS.Yellow` (`"\x1b[33m"`). |
| `TestLogger_Format_ErrorTagAndColor` | `createLogger > console output format > error includes [ERROR] tag with red color` | `Error("test message")` | Saída contém `"[ERROR]"` e `COLORS.Red` (`"\x1b[31m"`). |
| `TestLogger_Format_DebugTagAndColor` | `createLogger > console output format > debug includes [DEBUG] tag with dim color` | `Debug("test message")` | Saída contém `"[DEBUG]"` e `COLORS.Dim` (`"\x1b[2m"`). |
| `TestLogger_Format_PhaseTagAndColor` | `createLogger > console output format > phase includes [PHASE] tag with magenta color` | `Phase("test message")` | Saída contém `"[PHASE]"` e `COLORS.Magenta` (`"\x1b[35m"`). |
| `TestLogger_Format_DoneTagAndColor` | `createLogger > console output format > done includes [DONE] tag with green color` | `Done("test message")` | Saída contém `"[DONE]"` e `COLORS.Green` (`"\x1b[32m"`). |
| `TestLogger_DefaultArgument` | `createLogger > createLogger default > defaults to info level when no argument provided` | `CreateLogger()` | `GetLogLevel() == LevelInfo` (`"info"`). |

---

### 6.4. `internal/i18n/i18n_test.go` (Mapeado de [`tests/utils/i18n.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/i18n.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestI18n_ReturnsKeyWhenNoTranslationExists` | `i18n > t() > returns the key as-is when no translation exists` | `InitI18n(LangEN)` (com dicionário vazio ou chave inexistente) $\rightarrow$ `T("hello.world")` | Retorna `"hello.world"`. |
| `TestI18n_ReturnsKeyWhenTranslationsAreEmpty` | `i18n > t() > returns the key when translations are empty` | `T("some.key")` | Retorna `"some.key"`. |
| `TestI18n_SubstitutesVariablesUsingDoubleBraces` | `i18n > t() > substitutes variables using {{key}} syntax` | `T("Hello {{name}}, you have {{count}} messages", map[string]any{"name": "Alice", "count": 5})` | Retorna `"Hello Alice, you have 5 messages"`. |
| `TestI18n_ReplacesAllOccurrencesOfSameVariable` | `i18n > t() > replaces all occurrences of the same variable` | `T("{{x}} and {{x}} again", map[string]any{"x": "test"})` | Retorna `"test and test again"`. |
| `TestI18n_HandlesNumericVariableValues` | `i18n > t() > handles numeric variable values` | `T("Count: {{n}}", map[string]any{"n": 42})` | Retorna `"Count: 42"`. |
| `TestI18n_HandlesBooleanVariableValues` | `i18n > t() > handles boolean variable values` | `T("Active: {{val}}", map[string]any{"val": true})` | Retorna `"Active: true"`. |
| `TestI18n_HandlesNilVariableValues` | `i18n > t() > handles null/undefined variable values` | `T("Value: {{v}}", map[string]any{"v": nil})` | Retorna `"Value: null"`. |
| `TestI18n_DoesNotSubstituteWhenNoVariablesProvided` | `i18n > t() > does not substitute when no variables provided` | `T("Hello {{name}}")` | Retorna `"Hello {{name}}"`. |
| `TestI18n_HandlesEmptyVariablesMap` | `i18n > t() > handles empty variables object` | `T("Hello {{name}}", map[string]any{})` | Retorna `"Hello {{name}}"`. |
| `TestI18n_HandlesVariableKeysWithDots` | `i18n > t() > handles variable keys with dots (no regex interpretation)` | `T("Path: {{file.name}}", map[string]any{"file.name": "test.ts"})` | Retorna `"Path: test.ts"`. |
| `TestI18n_HandlesEmptyStringKey` | `i18n > t() > handles empty string key` | `T("")` | Retorna `""`. |
| `TestI18n_InitI18nDefaultsWithoutThrowing` | `i18n > initI18n() > defaults to English` *(nota: no código TS original o default do parâmetro é `'vi'`, e o teste valida que `initI18n()` sem argumentos retorna `t('test.key') === 'test.key'`)* | `InitI18n()` $\rightarrow$ `T("test.key")` | Não entra em pânico e retorna `"test.key"`. |
| `TestI18n_InitI18nAcceptsJaLanguage` | `i18n > initI18n() > accepts ja language` | `InitI18n(LangJA)` $\rightarrow$ `T("test.key")` | Retorna `"test.key"` (e traduz chaves reais de `ja.json` quando carregadas via `embed.FS`). |
