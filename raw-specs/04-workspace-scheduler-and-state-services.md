# Especificação Técnica 04 — Workspaces, Sanitização de Caminhos, Agendador Cron e Serviços de Estado do Bridge

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de forma 100% autocontida e exaustiva os pacotes `internal/workspace`, `internal/scheduler` e os serviços de estado/despacho de `internal/bridge` (`mode_service.go`, `model_service.go`, `auto_accept.go`, `title_generator.go`, `dispatcher.go`) do port em Golang (`1.23+`) do `antigravity-telegram-remote`, além do registro completo do módulo legado `processManager.ts` (conforme ADR-006).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript) | Arquivo(s) de Testes Original(is) (Jest) | Destino no Port Go | Pacote Go | Responsabilidade Principal |
| :--- | :--- | :--- | :--- | :--- |
| [`src/middleware/sanitize.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/middleware/sanitize.ts) | [`tests/middleware/sanitize.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/middleware/sanitize.test.ts)<br>[`tests/middleware/sanitize.bugfix.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/middleware/sanitize.bugfix.test.ts) | `internal/workspace/sanitize.go`<br>`internal/workspace/sanitize_test.go` | `workspace` | `ResolveSafePath(inputPath, baseDir)`: prevenção de path traversal (`..`, caminhos absolutos externos e escapes via symlink com `filepath.EvalSymlinks`). |
| [`src/services/workspaceService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/workspaceService.ts) | [`tests/services/workspaceService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/workspaceService.test.ts) | `internal/workspace/service.go`<br>`internal/workspace/service_test.go` | `workspace` | `WorkspaceService`: criação de `baseDir`, listagem ordenada de subdiretórios não-ocultos (`ScanWorkspaces`), validação de caminhos e checagem de existência (`Exists`). |
| [`src/services/scheduleService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/scheduleService.ts) | [`tests/services/scheduleService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/scheduleService.test.ts) | `internal/scheduler/service.go`<br>`internal/scheduler/service_test.go` | `scheduler` | `ScheduleService`: restauração de agendamentos ativos na inicialização (`RestoreAll`), validação de expressões cron de 5/6 campos, adição (`AddSchedule`), remoção (`RemoveSchedule`), listagem (`ListSchedules`) e parada em massa (`StopAll`). |
| [`src/services/modeService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/modeService.ts) | [`tests/services/modeService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/modeService.test.ts) | `internal/bridge/mode_service.go`<br>`internal/bridge/mode_service_test.go` | `bridge` | `ModeService`: gerenciamento do modo de execução (`fast` / `plan`), mapeamentos de exibição e UI (`Fast` / `Planning`). |
| [`src/services/modelService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/modelService.ts) | [`tests/services/modelService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/modelService.test.ts) | `internal/bridge/model_service.go`<br>`internal/bridge/model_service_test.go` | `bridge` | `ModelService`: gerenciamento dos 6 modelos LLM disponíveis no Antigravity (padrão `gemini-3-flash`). |
| [`src/services/autoAcceptService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/autoAcceptService.ts) | [`tests/services/autoAcceptService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/autoAcceptService.test.ts) | `internal/bridge/auto_accept.go`<br>`internal/bridge/auto_accept_test.go` | `bridge` | `AutoAcceptService`: máquina de estados booleana para auto-aprovação de diálogos (`on`, `off`, `status` e aliases). |
| [`src/services/titleGeneratorService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/titleGeneratorService.ts) | [`tests/services/titleGeneratorService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/titleGeneratorService.test.ts) | `internal/bridge/title_generator.go`<br>`internal/bridge/title_generator_test.go` | `bridge` | `TitleGeneratorService`: geração de título de sessão via CDP (`GENERATE_TITLE_SCRIPT`) com fallback de extração de 40 caracteres e sanitização CJK/slug (`SanitizeForChannelName`). |
| [`src/services/promptDispatcher.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/promptDispatcher.ts) | [`tests/services/promptDispatcher.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/promptDispatcher.test.ts)<br>[`tests/services/promptDispatcher.concurrency.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/promptDispatcher.concurrency.test.ts) | `internal/bridge/dispatcher.go`<br>`internal/bridge/dispatcher_test.go` | `bridge` | `PromptDispatcher`: fila serializada por workspace (`ws:<workspaceName>`, primário) e por canal (`<chatId>:<threadId>` ou `<chatId>`, secundário). |
| [`src/services/processManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/processManager.ts) *(Legado — ADR-006)* | [`tests/services/processManager.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/processManager.test.ts) | *(Omitido do binário Go conforme ADR-006; documentado na Seção 4.8 e 6.8)* | N/A | Fila legada de processos filhos (`child_process.spawn`) pré-CDP não referenciada pelo bot ativo. |

### 1.2. Documentação Arquitetural e ADRs Relacionados
- [`antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md)
- [`antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md) (ADR-006)

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pacote `internal/workspace` (`sanitize.go` e `service.go`)

```go
package workspace

// ResolveSafePath resolve inputPath em relação a baseDir e bloqueia tentativas de path traversal
// (tanto via ".." ou caminhos absolutos externos quanto via symlinks).
// Lança erro contendo "Path traversal detected" ou "Path traversal detected (symlink bypass attempt)".
func ResolveSafePath(inputPath string, baseDir string) (string, error)

// WorkspaceService gerencia diretórios de projetos dentro de WORKSPACE_BASE_DIR.
type WorkspaceService struct {
	baseDir string
}

func NewWorkspaceService(baseDir string) *WorkspaceService
func (s *WorkspaceService) EnsureBaseDir() error
func (s *WorkspaceService) ScanWorkspaces() []string
func (s *WorkspaceService) ValidatePath(relativePath string) (string, error)
func (s *WorkspaceService) GetBaseDir() string
func (s *WorkspaceService) GetWorkspacePath(workspaceName string) (string, error)
func (s *WorkspaceService) Exists(workspaceName string) (bool, error)
```

---

### 2.2. Pacote `internal/scheduler` (`internal/scheduler/service.go`)

```go
package scheduler

import (
	"sync"

	"antigravity-telegram-remote/internal/database"
)

// JobCallback é a função executada quando um agendamento cron dispara.
type JobCallback func(schedule database.ScheduleRecord)

// ScheduledTask representa uma tarefa agendada ativa que pode ser interrompida.
type ScheduledTask interface {
	Stop()
	Start()
}

// CronEngine abstrai o motor de agendamento cron (ex.: robfig/cron/v3) para permitir
// injeção de mocks determinísticos equivalentes a jest.mock('node-cron') em testes unitários.
type CronEngine interface {
	Validate(cronExpression string) bool
	Schedule(cronExpression string, fn func()) (ScheduledTask, error)
}

// ScheduleRepositoryInterface permite usar tanto *database.ScheduleRepository real quanto mocks em testes.
type ScheduleRepositoryInterface interface {
	Create(input database.CreateScheduleInput) (database.ScheduleRecord, error)
	FindAll() ([]database.ScheduleRecord, error)
	FindById(id int64) (*database.ScheduleRecord, error)
	FindEnabled() ([]database.ScheduleRecord, error)
	Delete(id int64) (bool, error)
	Update(id int64, input database.UpdateScheduleInput) (bool, error)
}

// ScheduleService gerencia o ciclo de vida das tarefas agendadas.
type ScheduleService struct {
	mu          sync.Mutex
	repo        ScheduleRepositoryInterface
	engine      CronEngine
	activeTasks map[int64]ScheduledTask
}

func NewScheduleService(repo ScheduleRepositoryInterface, engine ...CronEngine) *ScheduleService
func (s *ScheduleService) RestoreAll(jobCallback JobCallback) (int, error)
func (s *ScheduleService) AddSchedule(cronExpression string, prompt string, workspacePath string, jobCallback JobCallback) (database.ScheduleRecord, error)
func (s *ScheduleService) RemoveSchedule(scheduleId int64) (bool, error)
func (s *ScheduleService) StopAll()
func (s *ScheduleService) ListSchedules() ([]database.ScheduleRecord, error)
```

---

### 2.3. Pacote `internal/bridge` — Serviços de Estado (`mode_service.go`, `model_service.go`, `auto_accept.go`)

```go
package bridge

import "sync"

// --- mode_service.go ---

type Mode string

const (
	ModeFast Mode = "fast"
	ModePlan Mode = "plan"

	DefaultMode Mode = ModeFast
)

var AvailableModes = []Mode{ModeFast, ModePlan}

var ModeDisplayNames = map[Mode]string{
	ModeFast: "⚡ Fast",
	ModePlan: "📋 Plan",
}

var ModeUINames = map[Mode]string{
	ModeFast: "Fast",
	ModePlan: "Planning",
}

var ModeUINameReverse = map[string]Mode{
	"fast":     ModeFast,
	"planning": ModePlan,
}

// ModeDescription retorna a descrição traduzida via i18n.T no idioma ativo.
func GetModeDescription(mode Mode) string

type ModeSetResult struct {
	Success bool
	Mode    Mode
	Error   string
}

type ModeService struct {
	mu          sync.RWMutex
	currentMode Mode
}

func NewModeService() *ModeService
func (s *ModeService) GetCurrentMode() Mode
func (s *ModeService) SetMode(modeName string) ModeSetResult
func (s *ModeService) GetAvailableModes() []string

// --- model_service.go ---

type Model string

const (
	ModelGemini31ProHigh      Model = "gemini-3.1-pro-high"
	ModelGemini31ProLow       Model = "gemini-3.1-pro-low"
	ModelGemini3Flash         Model = "gemini-3-flash"
	ModelClaudeSonnet46Think  Model = "claude-sonnet-4.6-thinking"
	ModelClaudeOpus46Think    Model = "claude-opus-4.6-thinking"
	ModelGptOss120bMedium     Model = "gpt-oss-120b-medium"

	DefaultModel Model = ModelGemini3Flash
)

var AvailableModels = []Model{
	ModelGemini31ProHigh,
	ModelGemini31ProLow,
	ModelGemini3Flash,
	ModelClaudeSonnet46Think,
	ModelClaudeOpus46Think,
	ModelGptOss120bMedium,
}

type ModelSetResult struct {
	Success bool
	Model   Model
	Error   string
}

type ModelService struct {
	mu           sync.RWMutex
	currentModel Model
}

func NewModelService() *ModelService
func (s *ModelService) GetCurrentModel() Model
func (s *ModelService) SetModel(modelName string) ModelSetResult
func (s *ModelService) GetAvailableModels() []string

// --- auto_accept.go ---

type AutoAcceptAction string

const (
	AutoAcceptOn     AutoAcceptAction = "on"
	AutoAcceptOff    AutoAcceptAction = "off"
	AutoAcceptStatus AutoAcceptAction = "status"
)

type AutoAcceptCommandResult struct {
	Success bool
	Enabled bool
	Changed bool
	Message string
}

type AutoAcceptService struct {
	mu      sync.RWMutex
	enabled bool
}

func NewAutoAcceptService(initialEnabled ...bool) *AutoAcceptService
func (s *AutoAcceptService) IsEnabled() bool
func (s *AutoAcceptService) Handle(rawAction ...string) AutoAcceptCommandResult
```

---

### 2.4. Pacote `internal/bridge` — `title_generator.go` e `dispatcher.go`

```go
package bridge

import (
	"context"
	"sync"

	"antigravity-telegram-remote/internal/database"
)

// CdpEvaluator define a interface mínima de CdpService usada pelo TitleGeneratorService e PromptDispatcher.
type CdpEvaluator interface {
	GetPrimaryContextId() *int
	Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)
	GetCurrentWorkspaceName() string
}

// TitleGeneratorService gera títulos curtos para sessões/tópicos a partir do prompt do usuário.
type TitleGeneratorService struct{}

func NewTitleGeneratorService() *TitleGeneratorService
func (s *TitleGeneratorService) GenerateTitle(ctx context.Context, prompt string, cdpService ...CdpEvaluator) string
func (s *TitleGeneratorService) SanitizeForChannelName(text string) string

// --- dispatcher.go ---

type TelegramChannel struct {
	ChatID   int64
	ThreadID *int
}

type InboundImageAttachment struct {
	FileName string
	MimeType string
	Base64   string
}

type PromptDispatchOptions struct {
	ChatSessionService any
	ChatSessionRepo    *database.ChatSessionRepository
	TopicManager       any
	TitleGenerator     *TitleGeneratorService
}

type PromptDispatchRequest struct {
	Channel       TelegramChannel
	Prompt        string
	Cdp           CdpEvaluator
	InboundImages []InboundImageAttachment
	Options       *PromptDispatchOptions
}

type SendPromptFunc func(
	ctx context.Context,
	bridge any,
	channel TelegramChannel,
	prompt string,
	cdp CdpEvaluator,
	modeService *ModeService,
	modelService *ModelService,
	inboundImages []InboundImageAttachment,
	options *PromptDispatchOptions,
) error

type PromptDispatcherDeps struct {
	Bridge         any
	ModeService    *ModeService
	ModelService   *ModelService
	SendPromptImpl SendPromptFunc
}

type lockEntry struct {
	done chan struct{}
}

type PromptDispatcher struct {
	mu             sync.Mutex
	deps           PromptDispatcherDeps
	channelLocks   map[string]*lockEntry
	workspaceLocks map[string]*lockEntry
}

func NewPromptDispatcher(deps PromptDispatcherDeps) *PromptDispatcher
func (d *PromptDispatcher) Send(ctx context.Context, req PromptDispatchRequest) error
```

---

## 3. Constantes, Queries SQL, Expressões Regulares e Scripts Literais

### 3.1. Mensagens de Erro Literais de `sanitize.ts` e `scheduleService.ts`

- **`sanitize.ts`**:
  - Violação de caminho relativo (`..` ou fora de `baseDir`):
    ```text
    Path traversal detected
    ```
  - Violação via resolução de symlink (`realpathSync(resolvedPath)` não inicia com `realpathSync(normalizedBaseDir)`):
    ```text
    Path traversal detected (symlink bypass attempt)
    ```
- **`scheduleService.ts`**:
  - Expressão cron inválida:
    ```text
    Invalid cron expression: <cronExpression>
    ```

---

### 3.2. Constantes e Mensagens Literais de `modeService.ts`, `modelService.ts` e `autoAcceptService.ts`

1. **`modeService.ts`**:
   - `AVAILABLE_MODES = ['fast', 'plan']`
   - `DEFAULT_MODE = 'fast'`
   - `MODE_DISPLAY_NAMES = { fast: '⚡ Fast', plan: '📋 Plan' }`
   - `MODE_DESCRIPTIONS`:
     - `fast`: `t('Fast Mode — for simple tasks')`
     - `plan`: `t('Plan Mode — for complex step-by-step tasks')`
   - `MODE_UI_NAMES = { fast: 'Fast', plan: 'Planning' }`
   - `MODE_UI_NAME_REVERSE = { fast: 'fast', planning: 'plan' }`
   - Mensagem de erro quando `modeName` é vazio/espaços:
     `t('⚠️ Mode name not specified. Available modes: ') + AVAILABLE_MODES.join(', ')`
   - Mensagem de erro quando `modeName` é inválido:
     `t('⚠️ Invalid mode "${modeName}". Available modes: ${AVAILABLE_MODES.join(\', \')}')`

2. **`modelService.ts`**:
   - `AVAILABLE_MODELS`:
     ```json
     [
       "gemini-3.1-pro-high",
       "gemini-3.1-pro-low",
       "gemini-3-flash",
       "claude-sonnet-4.6-thinking",
       "claude-opus-4.6-thinking",
       "gpt-oss-120b-medium"
     ]
     ```
   - `DEFAULT_MODEL = 'gemini-3-flash'`
   - Mensagem de erro quando `modelName` é vazio/espaços:
     `t('⚠️ Model name not specified. Available models: ') + AVAILABLE_MODELS.join(', ')`
   - Mensagem de erro quando `modelName` é inválido:
     `t('⚠️ Invalid model "${modelName}". Available models: ${AVAILABLE_MODELS.join(\', \')}')`

3. **`autoAcceptService.ts`**:
   - Aliases aceitos em `normalizeAction(rawAction)` (após `.trim().toLowerCase()`):
     - String vazia ou ausente: `'status'`
     - Para `'on'`: `['on', 'enable', 'enabled', 'true', '1']`
     - Para `'off'`: `['off', 'disable', 'disabled', 'false', '0']`
     - Para `'status'`: `['status', 'state', 'show']`
     - Qualquer outro valor: `null` (inválido)
   - Mensagens traduzidas via `t(...)`:
     - Inválido: `t('⚠️ Invalid argument. Usage: `/autoaccept [on/off/status]`')`
     - Status ON: `t('⚙️ Auto-accept mode: **ON**')`
     - Status OFF: `t('⚙️ Auto-accept mode: **OFF**')`
     - Já estava ON: `t('ℹ️ Auto-accept mode is already **ON**.')`
     - Ligado agora: `t('✅ Auto-accept mode turned **ON**. Future dialogs will be auto-allowed.')`
     - Já estava OFF: `t('ℹ️ Auto-accept mode is already **OFF**.')`
     - Desligado agora: `t('✅ Auto-accept mode turned **OFF**. Returned to manual approval.')`

---

### 3.3. Script JavaScript Literal e Expressões Regulares de `titleGeneratorService.ts`

1. **`GENERATE_TITLE_SCRIPT` (`src/services/titleGeneratorService.ts`, linhas 8–28)**:
   ```javascript
   (async (userPrompt) => {
       try {
           // Generate title using Antigravity's internal fetch API
           // Look for configuration from __NEXT_DATA__ or window.__remixContext
           const configs = [
               window.__NEXT_DATA__,
               window.__remixContext,
               window.__APP_CONFIG__,
           ].filter(Boolean);

           // Fallback: extract leading text from prompt
           const fallbackTitle = userPrompt
               .replace(/^\[.*?\]\n?/, '')
               .substring(0, 40)
               .trim();

           return { ok: true, title: fallbackTitle, method: 'text-extract' };
       } catch (e) {
           return { ok: false, error: e.message };
       }
   })
   ```
   - Expressão avaliada em `Runtime.evaluate`:
     `${GENERATE_TITLE_SCRIPT}(${JSON.stringify(cleanPrompt)})` com `returnByValue: true, awaitPromise: true` (e `contextId` se `cdpService.getPrimaryContextId() !== null`).

2. **Expressões Regulares de `titleGeneratorService.ts`**:
   - Remoção de prefixo de workspace (`stripWorkspacePrefix`, linha 101):
     ```regexp
     ^\[ワークスペース:.*?\]\n?
     ```
   - Substituição de espaços por hífen em `sanitizeForChannelName` (linha 110):
     ```regexp
     \s+
     ```
   - Caracteres permitidos em `sanitizeForChannelName` (linha 112 — negação substituída por `'-'`):
     ```regexp
     [^a-z0-9\-_\x{3000}-\x{303f}\x{3040}-\x{309f}\x{30a0}-\x{30ff}\x{ff00}-\x{ff9f}\x{4e00}-\x{9faf}]
     ```
     *(No TS original: `/[^a-z0-9\-_\u3000-\u303f\u3040-\u309f\u30a0-\u30ff\uff00-\uff9f\u4e00-\u9faf]/g`)*.
   - Colapso de múltiplos hífens consecutivos (linha 113):
     ```regexp
     -{2,}
     ```
   - Remoção de hífens no início e no fim (linha 114):
     ```regexp
     ^-+|-+$
     ```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo de `ResolveSafePath(inputPath, baseDir)` (`internal/workspace/sanitize.go`)

1. Calcula `normalizedBaseDir`:
   - `filepath.Abs(baseDir)` (equivalente a `path.resolve(baseDir)`).
2. Calcula `resolvedPath`:
   - Se `filepath.IsAbs(inputPath)`, `resolvedPath = filepath.Clean(inputPath)` (como `path.resolve(baseDir, inputPath)` no Node.js quando `inputPath` é absoluto).
   - Caso contrário, `resolvedPath = filepath.Clean(filepath.Join(normalizedBaseDir, inputPath))`.
3. Calcula o caminho relativo `relative, err := filepath.Rel(normalizedBaseDir, resolvedPath)`:
   - Se `err != nil` ou `relative == ".."` ou `strings.HasPrefix(relative, ".."+string(filepath.Separator))`:
     - Retorna erro `"Path traversal detected"`.
4. **Verificação Anti-Symlink Bypass**:
   - Tenta resolver `realPath, err1 := filepath.EvalSymlinks(resolvedPath)` e `realBaseDir, err2 := filepath.EvalSymlinks(normalizedBaseDir)`.
   - Se ambos tiverem sucesso (`err1 == nil && err2 == nil`):
     - Verifica se `realPath` está contido em `realBaseDir` (`!strings.HasPrefix(realPath, realBaseDir)`).
     - Se não estiver, retorna erro `"Path traversal detected (symlink bypass attempt)"`.
     - Se estiver, retorna `(realPath, nil)`.
   - Se `EvalSymlinks` falhar porque o arquivo/diretório ainda não existe em disco:
     - Retorna `(resolvedPath, nil)` (já validado na etapa 3 contra `..`).

---

### 4.2. Algoritmo de `WorkspaceService` (`internal/workspace/service.go`)

1. **`EnsureBaseDir()`**:
   - Se `s.baseDir` não existir, cria com `os.MkdirAll(s.baseDir, 0755)`.
2. **`ScanWorkspaces()`**:
   - Chama `EnsureBaseDir()`.
   - Lê as entradas de `s.baseDir` via `os.ReadDir(s.baseDir)`. Se falhar, loga o erro e retorna `[]string{}`.
   - Filtra apenas entradas onde `entry.IsDir() == true` e `!strings.HasPrefix(entry.Name(), ".")`.
   - Ordena os nomes em ordem lexicográfica ascendente (`sort.Strings(workspaces)`).
   - Retorna `workspaces` (slice vazio `[]string{}` se nenhum diretório for encontrado).
3. **`ValidatePath(relativePath)`** e **`GetWorkspacePath(workspaceName)`**:
   - Delegam diretamente para `ResolveSafePath(relativePath, s.baseDir)`.
4. **`Exists(workspaceName)`**:
   - Chama `fullPath, err := s.ValidatePath(workspaceName)`. Se `err != nil`, propaga o erro.
   - Executa `info, err := os.Stat(fullPath)`.
   - Retorna `(err == nil && info.IsDir(), nil)`.

---

### 4.3. Algoritmo de `ScheduleService` (`internal/scheduler/service.go`)

1. **`RestoreAll(jobCallback)`**:
   - Obtém `enabledSchedules, err := s.repo.FindEnabled()`.
   - Para cada `schedule` em `enabledSchedules`, chama `s.registerCronTask(schedule, jobCallback)`.
   - Retorna `len(enabledSchedules)`.
2. **`AddSchedule(cronExpression, prompt, workspacePath, jobCallback)`**:
   - Valida a expressão com `s.engine.Validate(cronExpression)`.
   - Se inválida (`false`), retorna erro `fmt.Errorf("Invalid cron expression: %s", cronExpression)` **sem** tocar no banco de dados nem agendar tarefa.
   - Persiste no banco: `record, err := s.repo.Create(database.CreateScheduleInput{CronExpression: cronExpression, Prompt: prompt, WorkspacePath: workspacePath, Enabled: true})`.
   - Registra a tarefa cron em `s.activeTasks[record.ID]` via `s.registerCronTask(record, jobCallback)`.
   - Retorna `record`.
3. **`RemoveSchedule(scheduleId)`**:
   - Se existir `task, ok := s.activeTasks[scheduleId]`:
     - Chama `task.Stop()` e remove `scheduleId` de `s.activeTasks`.
   - Chama e retorna `s.repo.Delete(scheduleId)`.
4. **`StopAll()`**:
   - Para cada `task` em `s.activeTasks`, chama `task.Stop()`.
   - Limpa o mapa `s.activeTasks`.
5. **`ListSchedules()`**:
   - Retorna `s.repo.FindAll()`.

---

### 4.4. Algoritmos de `ModeService`, `ModelService` e `AutoAcceptService` (`internal/bridge/`)

1. **`ModeService.SetMode(modeName)`**:
   - Se `strings.TrimSpace(modeName) == ""`:
     - Retorna `ModeSetResult{Success: false, Error: i18n.T("⚠️ Mode name not specified. Available modes: ") + "fast, plan"}`.
   - `normalized := Mode(strings.ToLower(strings.TrimSpace(modeName)))`.
   - Se `normalized` não for `"fast"` nem `"plan"`:
     - Retorna `ModeSetResult{Success: false, Error: i18n.T(fmt.Sprintf("⚠️ Invalid mode %q. Available modes: fast, plan", modeName))}` sem alterar `currentMode`.
   - Atualiza `s.currentMode = normalized` e retorna `ModeSetResult{Success: true, Mode: normalized}`.

2. **`ModelService.SetModel(modelName)`**:
   - Se `strings.TrimSpace(modelName) == ""`:
     - Retorna `ModelSetResult{Success: false, Error: i18n.T("⚠️ Model name not specified. Available models: ") + strings.Join(s.GetAvailableModels(), ", ")}`.
   - `normalized := Model(strings.ToLower(strings.TrimSpace(modelName)))`.
   - Se `normalized` não estiver em `AvailableModels`:
     - Retorna `ModelSetResult{Success: false, Error: i18n.T(fmt.Sprintf("⚠️ Invalid model %q. Available models: %s", modelName, strings.Join(s.GetAvailableModels(), ", ")))}` sem alterar `currentModel`.
   - Atualiza `s.currentModel = normalized` e retorna `ModelSetResult{Success: true, Model: normalized}`.

3. **`AutoAcceptService.Handle(rawAction)`**:
   - Normaliza `rawAction` conforme a tabela da Seção 3.2.3.
   - Se inválido, retorna `{Success: false, Enabled: s.enabled, Changed: false, Message: ...}`.
   - Se `'status'`, retorna `{Success: true, Enabled: s.enabled, Changed: false, Message: ...}`.
   - Se `'on'`:
     - Se `s.enabled == true`, retorna `{Success: true, Enabled: true, Changed: false, Message: ...}`.
     - Caso contrário, define `s.enabled = true` e retorna `{Success: true, Enabled: true, Changed: true, Message: ...}`.
   - Se `'off'`:
     - Se `s.enabled == false`, retorna `{Success: true, Enabled: false, Changed: false, Message: ...}`.
     - Caso contrário, define `s.enabled = false` e retorna `{Success: true, Enabled: false, Changed: true, Message: ...}`.

---

### 4.5. Algoritmo de `TitleGeneratorService` (`internal/bridge/title_generator.go`)

1. **`GenerateTitle(ctx, prompt, cdpService...)`**:
   - Se `cdpService` não for nulo:
     - Tenta `generateViaCdp(ctx, prompt, cdp)`. Se retornar título não-vazio, retorna-o.
   - Fallback (`extractTitleFromText(prompt)`):
     - Remove prefixo `[ワークスペース:...\n?]` (`stripWorkspacePrefix`).
     - Trunca nos primeiros **40 caracteres** (runes/UTF-16 code units: `runes[:min(40, len(runes))]`) e aplica `strings.TrimSpace`.
     - Retorna `SanitizeForChannelName(truncated)` (que garante retorno `"untitled"` se vazio).
2. **`SanitizeForChannelName(text)`**:
   - Converte para minúsculas (`strings.ToLower(text)`).
   - Substitui sequências de espaços `\s+` por `"-"`.
   - Substitui qualquer caractere fora de `[a-z0-9\-_\u3000-\u303f\u3040-\u309f\u30a0-\u30ff\uff00-\uff9f\u4e00-\u9faf]` por `"-"`.
   - Colapsa 2 ou mais hífens seguidos (`-{2,}`) em um único `"-"`.
   - Remove hífens iniciais e finais (`^-+|-+$`).
   - Trunca em no máximo **80 caracteres** (runes).
   - Se a string resultante for `""`, retorna `"untitled"`.

---

### 4.6. Algoritmo de `PromptDispatcher.Send(ctx, req)` (`internal/bridge/dispatcher.go`)

1. Calcula a chave do canal (`channelKey`):
   - Se `req.Channel.ThreadID != nil && *req.Channel.ThreadID != 0`: `fmt.Sprintf("%d:%d", req.Channel.ChatID, *req.Channel.ThreadID)`.
   - Caso contrário: `fmt.Sprintf("%d", req.Channel.ChatID)`.
2. Calcula a chave do workspace (`wsKey`):
   - `wsName := req.Cdp.GetCurrentWorkspaceName()`.
   - Se `wsName != ""`, `lockKey = "ws:" + wsName`; caso contrário, `lockKey = chKey`.
3. **Encadeamento FIFO de Locks (Equivalente à cadeia de Promises em TypeScript)**:
   - Sob `d.mu.Lock()`:
     - Obtém `prev := d.workspaceLocks[lockKey]`.
     - Cria `current := &lockEntry{done: make(chan struct{})}`.
     - Registra `d.workspaceLocks[lockKey] = current` e `d.channelLocks[chKey] = current`.
   - Fora do mutex:
     - Se `prev != nil`, aguarda `<-prev.done`.
     - Executa `d.deps.SendPromptImpl(ctx, d.deps.Bridge, req.Channel, req.Prompt, req.Cdp, d.deps.ModeService, d.deps.ModelService, inboundImages, req.Options)`.
     - Eventual erro retornado por `SendPromptImpl` é capturado/ignorado para nunca quebrar a fila subsequente (`catch(() => {})` no TS original).
   - No bloco `defer`:
     - Fecha `close(current.done)` para liberar a próxima chamada na fila.
     - Sob `d.mu.Lock()`:
       - Se `d.workspaceLocks[lockKey] == current`, remove `delete(d.workspaceLocks, lockKey)`.
       - Se `d.channelLocks[chKey] == current`, remove `delete(d.channelLocks, chKey)`.

---

### 4.7. Documentação Completa do Módulo Legado `ProcessManager` (`src/services/processManager.ts` — ADR-006)

Embora `ProcessManager` seja código legado pré-CDP não importado pelo bot e omitido do binário Go (conforme ADR-006), seu funcionamento exato no TypeScript é:
- Mantém `maxConcurrentTasks` (padrão `1`), uma fila FIFO `queue: TaskOptions[]` e um mapa `runningProcesses: Map<string, ChildProcess>`.
- `submitTask(options)` adiciona a tarefa em `queue` e chama `runNext()`.
- `runNext()` verifica se `runningProcesses.size < maxConcurrentTasks`, retira o primeiro item com `queue.shift()`, executa `spawn(command, args, { cwd })`, encaminha eventos `stdout.on('data')` para `onStdout(data.toString())`, `stderr.on('data')` para `onStderr(data.toString())` e, no evento `'close'`, remove o `id` de `runningProcesses`, invoca `onClose(code ?? 0)` e chama `runNext()` recursivamente.
- `stopTask(taskId)`: se estiver em `runningProcesses`, chama `child.kill()`, remove do mapa e retorna `true`; se estiver em `queue`, remove com `splice` e retorna `true`; caso contrário retorna `false`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **`PromptDispatcher` Sem Deadlocks e Sem Condições de Corrida**:
   - O padrão de encadeamento de canais `done chan struct{}` protegido por `sync.Mutex` reproduz exatamente a semântica de encadeamento de Promises (`previous.then(...)`) do Node.js: duas mensagens enviadas para o mesmo canal ou para dois tópicos distintos vinculados ao mesmo workspace entram na fila na ordem exata de chegada e executam estritamente uma após a outra, enquanto mensagens para workspaces distintos executam em paralelo.
2. **Proteção de Estado em `ModeService`, `ModelService` e `AutoAcceptService`**:
   - Todos os três serviços utilizam `sync.RWMutex` para leituras e escritas atômicas de estado.
3. **Encerramento Limpo do `ScheduleService`**:
   - No shutdown do bot, `scheduleService.StopAll()` é chamado sob `sync.Mutex`, parando todos os timers ativos e esvaziando `activeTasks`.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os cenários abaixo mapeiam **1 para 1** os blocos `describe` e `it` dos arquivos de teste originais e devem passar com `go test -v -race ./internal/workspace/... ./internal/scheduler/... ./internal/bridge/...`.

### 6.1. `internal/workspace/sanitize_test.go` (Mapeado de [`tests/middleware/sanitize.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/middleware/sanitize.test.ts) e [`tests/middleware/sanitize.bugfix.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/middleware/sanitize.bugfix.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestSanitize_ResolvesValidPathWithinBaseDir` | `Sanitize Middleware > resolves a valid path within the base directory` | `baseDir="/home/user/workspace"`, `input="project/src/index.ts"` | Retorna `"/home/user/workspace/project/src/index.ts"`, `err == nil`. |
| `TestSanitize_ThrowsOnTraverseOutsideBaseDir` | `Sanitize Middleware > throws an error if the path attempts to traverse outside the base directory` | `input="../config/secret.json"` | Retorna erro contendo `"Path traversal detected"`. |
| `TestSanitize_ThrowsOnAbsolutePathOutsideBaseDir` | `Sanitize Middleware > throws an error if the path uses absolute path outside the base directory` | `input="/etc/passwd"` | Retorna erro contendo `"Path traversal detected"`. |
| `TestSanitize_ResolvesAbsolutePathInsideBaseDir` | `Sanitize Middleware > resolves an absolute path if it is within the base directory` | `input="/home/user/workspace/project/README.md"` | Retorna `"/home/user/workspace/project/README.md"`, `err == nil`. |
| `TestSanitizeBugfix_AllowsSubdirectoryPaths` | `resolveSafePath — bug fix coverage > allows subdirectory paths` | `input="a/b/c"` | Retorna `filepath.Join(baseDir, "a/b/c")`. |
| `TestSanitizeBugfix_AllowsExactBaseDirectory` | `resolveSafePath — bug fix coverage > allows exact base directory` | `input="."` | Retorna `filepath.Clean(baseDir)`. |
| `TestSanitizeBugfix_BlocksParentTraversalWithDotDotSlash` | `resolveSafePath — bug fix coverage > blocks parent traversal with ../` | `input="../outside"` | Erro `"Path traversal detected"`. |
| `TestSanitizeBugfix_BlocksExactDotDotPath` | `resolveSafePath — bug fix coverage > blocks exact ".." path` | `input=".."` | Erro `"Path traversal detected"`. |
| `TestSanitizeBugfix_BlocksMultiLevelTraversal` | `resolveSafePath — bug fix coverage > blocks multi-level traversal` | `input="../../etc/passwd"` | Erro `"Path traversal detected"`. |
| `TestSanitizeBugfix_BlocksAbsolutePathsOutsideBase` | `resolveSafePath — bug fix coverage > blocks absolute paths outside base` | `input="/etc/passwd"` | Erro `"Path traversal detected"`. |
| `TestSanitizeBugfix_AllowsAbsolutePathsInsideBase` | `resolveSafePath — bug fix coverage > allows absolute paths inside base` | `input=filepath.Join(baseDir, "project", "file.txt")` | Retorna `input`, `err == nil`. |
| `TestSanitizeBugfix_BlocksTraversalTmpEvil` | `resolveSafePath — bug fix coverage > still blocks traversal after dead code removal` | `input="/tmp/evil"` | Erro `"Path traversal detected"`. |
| `TestSanitizeBugfix_HandlesEmbeddedDotDotInsideBase` | `resolveSafePath — bug fix coverage > handles embedded ../ that resolves inside base` | `input="a/../b"` | Retorna `filepath.Join(baseDir, "b")`. |
| `TestSanitizeBugfix_BlocksEmbeddedDotDotOutsideBase` | `resolveSafePath — bug fix coverage > blocks embedded ../ that resolves outside base` | `input="a/../../outside"` | Erro `"Path traversal detected"`. |

---

### 6.2. `internal/workspace/service_test.go` (Mapeado de [`tests/services/workspaceService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/workspaceService.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestWorkspaceService_EnsureBaseDir_CreatesWhenNotExist` | `WorkspaceService > ensureBaseDir > creates the base directory when it does not exist` | `newDir = filepath.Join(tmpDir, "nested", "dir")` $\rightarrow$ `EnsureBaseDir()` | Diretório existe em disco. |
| `TestWorkspaceService_EnsureBaseDir_NoErrorWhenExists` | `WorkspaceService > ensureBaseDir > does not throw an error when the directory already exists` | `EnsureBaseDir()` em `tmpDir` existente | Retorna `err == nil`. |
| `TestWorkspaceService_ScanWorkspaces_SortedOrder` | `WorkspaceService > scanWorkspaces > returns subdirectory list in sorted order` | Cria `project-b`, `project-a`, `project-c` | Retorna `[]string{"project-a", "project-b", "project-c"}`. |
| `TestWorkspaceService_ScanWorkspaces_ExcludesFiles` | `WorkspaceService > scanWorkspaces > does not include files` | Cria dir `project-a` e arquivo `readme.txt` | Retorna `[]string{"project-a"}`. |
| `TestWorkspaceService_ScanWorkspaces_ExcludesDotDirectories` | `WorkspaceService > scanWorkspaces > does not include dotfiles or dot directories` | Cria `.hidden` e `visible` | Retorna `[]string{"visible"}`. |
| `TestWorkspaceService_ScanWorkspaces_EmptyDir` | `WorkspaceService > scanWorkspaces > returns an empty array for an empty directory` | `tmpDir` vazio | Retorna `[]string{}` (`len == 0`). |
| `TestWorkspaceService_ValidatePath_ValidRelative` | `WorkspaceService > validatePath > resolves a valid relative path to an absolute path` | `ValidatePath("my-project")` | Retorna `filepath.Join(tmpDir, "my-project")`. |
| `TestWorkspaceService_ValidatePath_RejectsTraversal` | `WorkspaceService > validatePath > rejects path traversal` | `ValidatePath("../etc/passwd")` | Erro `"Path traversal detected"`. |
| `TestWorkspaceService_ValidatePath_RejectsNestedTraversal` | `WorkspaceService > validatePath > rejects nested path traversal` | `ValidatePath("a/../../etc")` | Erro `"Path traversal detected"`. |
| `TestWorkspaceService_Exists_TrueForDirectory` | `WorkspaceService > exists > returns true for an existing directory` | Cria dir `my-project` $\rightarrow$ `Exists("my-project")` | Retorna `true, nil`. |
| `TestWorkspaceService_Exists_FalseForNonExistent` | `WorkspaceService > exists > returns false for a non-existent directory` | `Exists("nonexistent")` | Retorna `false, nil`. |
| `TestWorkspaceService_Exists_FalseForFile` | `WorkspaceService > exists > returns false for a file` | Cria arquivo `file.txt` $\rightarrow$ `Exists("file.txt")` | Retorna `false, nil`. |
| `TestWorkspaceService_GetBaseDir` | `WorkspaceService > getBaseDir > returns the base directory` | `GetBaseDir()` | Retorna `tmpDir`. |
| `TestWorkspaceService_GetWorkspacePath_Valid` | `WorkspaceService > getWorkspacePath > returns the absolute path of the workspace` | `GetWorkspacePath("proj")` | Retorna `filepath.Join(tmpDir, "proj")`. |
| `TestWorkspaceService_GetWorkspacePath_RejectsTraversal` | `WorkspaceService > getWorkspacePath > rejects path traversal` | `GetWorkspacePath("../outside")` | Erro `"Path traversal detected"`. |

---

### 6.3. `internal/scheduler/service_test.go` (Mapeado de [`tests/services/scheduleService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/scheduleService.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestScheduleService_RestoreAll_RegistersAllEnabled` | `ScheduleService > restoreAll > registers all enabled schedules with node-cron` | Mock repo retorna 2 schedules habilitados (`"0 9 * * *"`, `"0 18 * * *"`) | `FindEnabled` chamado 1 vez; `engine.Schedule` chamado 2 vezes; `RestoreAll` retorna `2`. |
| `TestScheduleService_RestoreAll_ReturnsZeroWhenNoneEnabled` | `ScheduleService > restoreAll > returns 0 when there are no enabled schedules` | Mock repo retorna `[]` | `RestoreAll` retorna `0` e `engine.Schedule` não é chamado. |
| `TestScheduleService_AddSchedule_ValidatesSavesAndSchedules` | `ScheduleService > addSchedule > passes validation, saves to DB, and registers with cron` | `Validate` retorna `true`; `AddSchedule("*/10 * * * *", "10分毎のチェック", "/service", cb)` | `Validate("*/10 * * * *")` chamado; `repo.Create` chamado com `Enabled: true`; `engine.Schedule` chamado 1 vez; retorna o registro criado. |
| `TestScheduleService_AddSchedule_ThrowsOnInvalidCron` | `ScheduleService > addSchedule > throws an error for an invalid cron expression` | `Validate` retorna `false`; `AddSchedule("invalid-cron", ...)` | Retorna erro `"Invalid cron expression: invalid-cron"`; `repo.Create` e `engine.Schedule` **não** são chamados. |
| `TestScheduleService_RemoveSchedule_StopsTaskAndDeletesFromDB` | `ScheduleService > removeSchedule > stops the running cron job and deletes from DB` | Agenda ID `5` e chama `RemoveSchedule(5)` | Retorna `true`; `mockTask.Stop()` é chamado; `repo.Delete(5)` é chamado. |
| `TestScheduleService_RemoveSchedule_ReturnsFalseForNonExistent` | `ScheduleService > removeSchedule > returns false for a non-existent schedule ID` | `repo.Delete(999)` retorna `false` | `RemoveSchedule(999)` retorna `false`. |
| `TestScheduleService_StopAll_StopsAllRunningJobs` | `ScheduleService > stopAll > stops all running cron jobs` | Registra 2 schedules (`mockTask1`, `mockTask2`) $\rightarrow$ `StopAll()` | `mockTask1.Stop()` e `mockTask2.Stop()` são chamados. |
| `TestScheduleService_ListSchedules_RetrievesAllFromDB` | `ScheduleService > listSchedules > retrieves all schedules from DB` | `repo.FindAll()` retorna 2 schedules | `ListSchedules()` retorna os 2 schedules e chama `repo.FindAll()` 1 vez. |

---

### 6.4. `internal/bridge/mode_service_test.go` (Mapeado de [`tests/services/modeService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/modeService.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestModeService_GetCurrentMode_DefaultFast` | `ModeService > getCurrentMode > returns the default mode ("fast") in the initial state` | `NewModeService()` | `GetCurrentMode() == DefaultMode` (`"fast"`). |
| `TestModeService_SetMode_SwitchesValidMode` | `ModeService > setMode > switches the mode when a valid mode name is specified` | `SetMode("plan")` | `Success == true`, `Mode == "plan"`, `GetCurrentMode() == "plan"`. |
| `TestModeService_SetMode_RetainsLastModeAfterMultipleSwitches` | `ModeService > setMode > retains the last set mode after multiple switches` | `SetMode("plan")` $\rightarrow$ `SetMode("fast")` | `GetCurrentMode() == "fast"`. |
| `TestModeService_SetMode_InvalidModeReturnsError` | `ModeService > setMode > returns an error and does not change the mode for an invalid mode name` | `SetMode("invalid_mode")` | `Success == false`, `Error != ""`, `GetCurrentMode() == DefaultMode`. |
| `TestModeService_SetMode_CaseInsensitive` | `ModeService > setMode > sets the mode case-insensitively` | `SetMode("FAST")` | `Success == true`, `Mode == "fast"`. |
| `TestModeService_SetMode_EmptyStringReturnsError` | `ModeService > setMode > returns an error when an empty string is specified` | `SetMode("")` | `Success == false`, `Error != ""`. |
| `TestModeService_GetAvailableModes` | `ModeService > getAvailableModes > returns the list of available modes` | `GetAvailableModes()` | Retorna `[]string{"fast", "plan"}` (`len == 2`). |

---

### 6.5. `internal/bridge/model_service_test.go` (Mapeado de [`tests/services/modelService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/modelService.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestModelService_GetCurrentModel_Default` | `ModelService > getCurrentModel > returns the default model in the initial state` | `NewModelService()` | `GetCurrentModel() == DefaultModel` (`"gemini-3-flash"`). |
| `TestModelService_SetModel_SwitchesValidModel` | `ModelService > setModel > switches the model when a valid model name is specified` | `SetModel("claude-sonnet-4.6-thinking")` | `Success == true`, `Model == "claude-sonnet-4.6-thinking"`, `GetCurrentModel() == "claude-sonnet-4.6-thinking"`. |
| `TestModelService_SetModel_RetainsLastSetModel` | `ModelService > setModel > retains the last set model after multiple switches` | `SetModel("claude-sonnet-4.6-thinking")` $\rightarrow$ `SetModel("gemini-3.1-pro-high")` | `GetCurrentModel() == "gemini-3.1-pro-high"`. |
| `TestModelService_SetModel_InvalidModelReturnsError` | `ModelService > setModel > returns an error and does not change the model for an invalid model name` | `SetModel("invalid_model")` | `Success == false`, `Error != ""`, `GetCurrentModel() == DefaultModel`. |
| `TestModelService_SetModel_CaseInsensitive` | `ModelService > setModel > sets the model case-insensitively` | `SetModel("GEMINI-3-FLASH")` | `Success == true`, `Model == "gemini-3-flash"`. |
| `TestModelService_SetModel_EmptyStringReturnsError` | `ModelService > setModel > returns an error when an empty string is specified` | `SetModel("")` | `Success == false`, `Error != ""`. |
| `TestModelService_SetModel_ClaudeOpus46Thinking` | `ModelService > setModel > can set the claude-opus-4.6-thinking model` | `SetModel("claude-opus-4.6-thinking")` | `Success == true`, `Model == "claude-opus-4.6-thinking"`. |
| `TestModelService_SetModel_GptOss120bMedium` | `ModelService > setModel > can set the gpt-oss-120b-medium model` | `SetModel("gpt-oss-120b-medium")` | `Success == true`, `Model == "gpt-oss-120b-medium"`. |
| `TestModelService_GetAvailableModels_ReturnsList` | `ModelService > getAvailableModels > returns the list of available models` | `GetAvailableModels()` | Igual a `AvailableModels`, `len == 6`. |
| `TestModelService_GetAvailableModels_IncludesSonnetGptProHigh` | `ModelService > getAvailableModels > includes claude-sonnet-4.6-thinking, gpt-oss-120b-medium, and gemini-3.1-pro-high in the list` | `GetAvailableModels()` | Contém os 3 modelos. |
| `TestModelService_GetAvailableModels_IncludesOpusAndFlash` | `ModelService > getAvailableModels > includes claude-opus-4.6-thinking and gemini-3-flash in the list` | `GetAvailableModels()` | Contém os 2 modelos. |

---

### 6.6. `internal/bridge/auto_accept_test.go` (Mapeado de [`tests/services/autoAcceptService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/autoAcceptService.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestAutoAcceptService_InitialStateMatchesConstructor` | `AutoAcceptService > initial state matches the constructor argument` | `NewAutoAcceptService()` e `NewAutoAcceptService(true)` | `false` e `true` respectivamente. |
| `TestAutoAcceptService_EnablesWithOn` | `AutoAcceptService > enables the service with "on"` | Inicia `false` $\rightarrow$ `Handle("on")` | `Success == true`, `Enabled == true`, `Changed == true`, `IsEnabled() == true`. |
| `TestAutoAcceptService_DisablesWithOff` | `AutoAcceptService > disables the service with "off"` | Inicia `true` $\rightarrow$ `Handle("off")` | `Success == true`, `Enabled == false`, `Changed == false`? Não: `Changed == true`, `IsEnabled() == false`. |
| `TestAutoAcceptService_StatusDoesNotChangeState` | `AutoAcceptService > does not change state with "status"` | Inicia `true` $\rightarrow$ `Handle("status")` | `Success == true`, `Enabled == true`, `Changed == false`, `IsEnabled() == true`. |
| `TestAutoAcceptService_InvalidArgumentReturnsError` | `AutoAcceptService > returns an error for invalid arguments without changing state` | Inicia `false` $\rightarrow$ `Handle("invalid")` | `Success == false`, `Enabled == false`, `Changed == false`, `IsEnabled() == false`, `strings.Contains(Message, "on/off/status")`. |

---

### 6.7. `internal/bridge/title_generator_test.go` (Mapeado de [`tests/services/titleGeneratorService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/titleGeneratorService.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestTitleGenerator_Fallback_GeneratesShortTitle` | `TitleGeneratorService > generateTitle - fallback > generates a short title from the prompt` | `GenerateTitle(ctx, "React認証バグを修正したい")` | Não vazio e `len(runes) <= 80`. |
| `TestTitleGenerator_Fallback_RemovesWorkspacePrefix` | `TitleGeneratorService > generateTitle - fallback > removes workspace prefix` | `GenerateTitle(ctx, "[ワークスペース: /Users/test/Code/project]\nReactの認証バグ")` | Resultado não contém `"ワークスペース"` e não é vazio. |
| `TestTitleGenerator_Fallback_EmptyPromptReturnsUntitled` | `TitleGeneratorService > generateTitle - fallback > returns untitled for an empty prompt` | `GenerateTitle(ctx, "")` | Retorna `"untitled"`. |
| `TestTitleGenerator_Fallback_SpecialCharsOnlyReturnsUntitled` | `TitleGeneratorService > generateTitle - fallback > returns untitled for a prompt containing only special characters` | `GenerateTitle(ctx, "!!!@@@###")` | Retorna `"untitled"`. |
| `TestTitleGenerator_Fallback_TruncatesLongPrompt` | `TitleGeneratorService > generateTitle - fallback > truncates a long prompt to 80 characters or less` | `GenerateTitle(ctx, strings.Repeat("a", 200))` | `len(title) <= 80`. |
| `TestTitleGenerator_Sanitize_SpacesToHyphens` | `TitleGeneratorService > sanitizeForChannelName > converts spaces to hyphens` | `SanitizeForChannelName("hello world")` | Retorna `"hello-world"`. |
| `TestTitleGenerator_Sanitize_UppercaseToLowercase` | `TitleGeneratorService > sanitizeForChannelName > converts uppercase to lowercase` | `SanitizeForChannelName("Hello World")` | Retorna `"hello-world"`. |
| `TestTitleGenerator_Sanitize_PreservesJapaneseCharacters` | `TitleGeneratorService > sanitizeForChannelName > preserves Japanese characters` | `SanitizeForChannelName("認証バグ修正")` | Retorna `"認証バグ修正"`. |
| `TestTitleGenerator_Sanitize_SpecialCharsToHyphens` | `TitleGeneratorService > sanitizeForChannelName > converts special characters to hyphens` | `SanitizeForChannelName("test@#$%ing")` | Não contém `"@"` nem `"#"` (`"test-ing"`). |
| `TestTitleGenerator_Sanitize_CollapsesConsecutiveHyphens` | `TitleGeneratorService > sanitizeForChannelName > collapses consecutive hyphens into one` | `SanitizeForChannelName("a---b")` | Retorna `"a-b"`. |
| `TestTitleGenerator_Sanitize_RemovesLeadingAndTrailingHyphens` | `TitleGeneratorService > sanitizeForChannelName > removes leading and trailing hyphens` | `SanitizeForChannelName("-test-")` | Retorna `"test"`. |
| `TestTitleGenerator_Sanitize_EmptyStringReturnsUntitled` | `TitleGeneratorService > sanitizeForChannelName > returns untitled for an empty string` | `SanitizeForChannelName("")` | Retorna `"untitled"`. |
| `TestTitleGenerator_ViaCDP_FallsBackOnCDPError` | `TitleGeneratorService > generateTitle - via CDP > falls back when CDP call fails` | Mock CDP `Call` retorna erro `"CDP error"` para `"テスト用プロンプト"` | Retorna `"テスト用プロンプト"` (não vazio e `!= "untitled"`). |
| `TestTitleGenerator_ViaCDP_FallsBackOnOkFalse` | `TitleGeneratorService > generateTitle - via CDP > falls back when CDP returns a null value` | Mock CDP retorna `{result: {value: {ok: false}}}` | Retorna `"テスト用プロンプト"` via fallback. |

---

### 6.8. `internal/bridge/dispatcher_test.go` (Mapeado de [`tests/services/promptDispatcher.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/promptDispatcher.test.ts) e [`tests/services/promptDispatcher.concurrency.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/promptDispatcher.concurrency.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestPromptDispatcher_InvokesSendPromptImpl` | `PromptDispatcher > invokes the send implementation via send()` | `Send(ctx, req)` com `ChatID: 12345, ThreadID: 1, Prompt: "hello", Workspace: "TestProject"` | `SendPromptImpl` é chamado com os argumentos exatos (`bridge`, `channel`, `"hello"`, `cdp`, `modeService`, `modelService`, `[]`, `options`). |
| `TestPromptDispatcher_SerializesSendsToSameChannel` | `PromptDispatcher — per-channel concurrency > serializes sends to the same channel` | Dispara `p1` (`"first"`) bloqueado em canal e `p2` (`"second"`) para o mesmo `(100, 1, "TestProject")` | Enquanto `p1` não libera, `SendPromptImpl` foi chamado apenas 1 vez (`order == [1]`); após liberar `p1`, ambos concluem na ordem `[1, 2]`. |
| `TestPromptDispatcher_AllowsParallelSendsToDifferentChannelsAndWorkspaces` | `PromptDispatcher — per-channel concurrency > allows parallel sends to different channels` | Dispara em paralelo `(100, 1, "channel-A", "WorkspaceA")` e `(200, 2, "channel-B", "WorkspaceB")` | Ambos executam concorrentemente (`SendPromptImpl` chamado 2 vezes). |
| `TestPromptDispatcher_GeneratesDifferentKeysForDifferentThreadIDs` | `PromptDispatcher — per-channel concurrency > generates different keys for different threadIds` | `(100, 1, "thread-1", "WorkspaceA")` e `(100, 2, "thread-2", "WorkspaceB")` | Ambos executam independentemente (`len(calls) == 2`). |
| `TestPromptDispatcher_UsesChatIDOnlyWhenNoThreadID` | `PromptDispatcher — per-channel concurrency > uses chatId only when no threadId` | `(100, nil, "no-thread")` | `SendPromptImpl` chamado 1 vez sem erro. |
| `TestPromptDispatcher_CleansUpLockAfterSendCompletes` | `PromptDispatcher — per-channel concurrency > cleans up lock after send completes` | Duas chamadas sequenciais `Send` para `(100, 1)` | Segunda chamada não bloqueia e `SendPromptImpl` é chamado 2 vezes; mapas `workspaceLocks` e `channelLocks` ficam vazios ao final. |
| `TestPromptDispatcher_HandlesErrorsWithoutBreakingSerialization` | `PromptDispatcher — per-channel concurrency > handles errors in sendPromptImpl without breaking serialization` | Primeira chamada a `SendPromptImpl` retorna erro `"fail"`, segunda retorna `nil` | Ambas as chamadas `Send` completam e a segunda executa normalmente (`SendPromptImpl` chamado 2 vezes). |

> **Nota sobre [`tests/services/processManager.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/processManager.test.ts) (ADR-006)**: Os 4 testes de `ProcessManager` (`should spawn a process with correct arguments`, `should pass stdout and stderr to callbacks`, `should limit concurrent executions and queue tasks`, `should kill a running process on stopTask`) testavam a fila `child_process.spawn` legada pré-CDP. No port Go, a serialização de tarefas é exercitada diretamente por `TestPromptDispatcher_*` acima.
