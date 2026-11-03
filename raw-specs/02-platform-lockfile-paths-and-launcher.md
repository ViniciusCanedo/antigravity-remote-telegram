# Especificação Técnica 02 — Camada de Plataforma: Lockfile, Caminhos, Launcher CDP e Descoberta de Processos

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de forma 100% autocontida e exaustiva o pacote `internal/platform` do port em Golang (`1.23+`) do `antigravity-telegram-remote`, abrangendo o controle de instância única via `.bot.lock` (`lockfile.go`), resolução segura de caminhos multiplataforma (`paths.go`), verificação paralela de portas CDP (`launcher.go`) e descoberta de processos `language_server` / portas TCP em escuta (`proc_unix.go` e `proc_windows.go`, conforme ADR-001 e ADR-003).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript) | Arquivo de Testes Original (Jest) | Destino no Port Go | Pacote Go | Responsabilidade Principal |
| :--- | :--- | :--- | :--- | :--- |
| [`src/utils/lockfile.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/lockfile.ts) | [`tests/utils/lockfile.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/lockfile.test.ts) | `internal/platform/lockfile.go`<br>`internal/platform/lockfile_test.go` | `platform` | Lockfile `~/.antigravity-telegram-remote/.bot.lock`, detecção de PID ativo, encerramento gracioso (`SIGTERM` $\rightarrow$ polling de `50ms` por `5000ms` $\rightarrow$ `SIGKILL`) e liberação no encerramento do processo. |
| [`src/utils/pathUtils.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/pathUtils.ts) | [`tests/utils/pathUtils.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/pathUtils.test.ts) | `internal/platform/paths.go`<br>`internal/platform/paths_test.go` | `platform` | `GetAntigravityCliPath()` (com validação de arquivo contra command injection em `ANTIGRAVITY_PATH`), `ExtractProjectNameFromPath()` e `GetAntigravityCdpHint()`. |
| [`src/services/antigravityLauncher.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/antigravityLauncher.ts)<br>[`src/utils/cdpPorts.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/cdpPorts.ts) | [`tests/services/antigravityLauncher.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/antigravityLauncher.test.ts) | `internal/platform/launcher.go`<br>`internal/platform/launcher_test.go` | `platform` | `EnsureAntigravityRunning()`: sondagem HTTP paralela de `CDP_PORTS` (`9222, 9223, 9333, 9444, 9555, 9666`) em `http://127.0.0.1:<port>/json/list` (timeout `2000ms`) e emissão do banner de aviso de 70 colunas `'='`. |
| [`src/services/quotaService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/quotaService.ts) *(linhas 23–67 e 89–174)* | *(Testado em `proc_test.go`)* | `internal/platform/proc_unix.go`<br>`internal/platform/proc_windows.go`<br>`internal/platform/proc_test.go` | `platform` | Descoberta do PID e `--csrf_token` do processo `language_server` e descoberta das portas TCP em estado `LISTEN` daquele PID em macOS/Linux (`//go:build darwin || linux`) e Windows (`//go:build windows`), além de `QuotaService.FetchQuota()`. |

### 1.2. Documentação Arquitetural e ADRs Relacionados
- [`antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md)
- [`antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md)
- [`antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md) (ADR-001 e ADR-003)

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. `internal/platform/lockfile.go`

```go
package platform

import (
	"os"
	"syscall"
	"time"
)

// LockfileDeps permite injetar dependências de sistema de arquivos, processos e sinais
// para testes determinísticos equivalentes a tests/utils/lockfile.test.ts.
type LockfileDeps struct {
	LockFilePath      string
	CurrentPID        int
	FileExists        func(path string) bool
	ReadFile          func(path string) ([]byte, error)
	WriteFile         func(path string, data []byte, perm os.FileMode) error
	RemoveFile        func(path string) error
	MkdirAll          func(path string, perm os.FileMode) error
	KillProcess       func(pid int, sig syscall.Signal) error
	Sleep             func(d time.Duration)
	Now               func() time.Time
	RegisterExitHooks func(release func())
}

// DefaultLockFilePath retorna ~/.antigravity-telegram-remote/.bot.lock.
func DefaultLockFilePath() string

// AcquireLock adquire o lockfile ~/.antigravity-telegram-remote/.bot.lock,
// encerrando eventual instância anterior ativa, e retorna a função releaseLock().
func AcquireLock() func()

// AcquireLockWithDeps executa a lógica de AcquireLock com dependências injetadas para testes.
func AcquireLockWithDeps(deps LockfileDeps) func()
```

---

### 2.2. `internal/platform/paths.go`

```go
package platform

import "os"

// PathDeps permite injetar plataforma (GOOS), variáveis de ambiente e os.Stat em testes.
type PathDeps struct {
	GOOS      string
	Getenv    func(key string) string
	Stat      func(name string) (os.FileInfo, error)
}

// GetAntigravityCliPath resolve o caminho do executável CLI do Antigravity.
// Precedência:
// 1. Variável de ambiente ANTIGRAVITY_PATH (validando via os.Stat que existe e !info.IsDir()).
// 2. Caminho padrão por sistema operacional (darwin, windows, linux).
func GetAntigravityCliPath() (string, error)

// GetAntigravityCliPathWithDeps permite testar todos os cenários de SO e ANTIGRAVITY_PATH.
func GetAntigravityCliPathWithDeps(deps PathDeps) (string, error)

// ExtractProjectNameFromPath extrai o último segmento não-vazio de um caminho POSIX ou Windows.
func ExtractProjectNameFromPath(workspacePath string) string

// GetAntigravityCdpHint retorna a instrução de linha de comando específica do SO para iniciar
// o Antigravity com --remote-debugging-port=<port> (porta padrão: 9222 quando port <= 0).
func GetAntigravityCdpHint(port ...int) string

// GetAntigravityCdpHintForOS retorna a dica de comando CDP para a string de plataforma informada
// ("darwin", "win32"/"windows", "linux").
func GetAntigravityCdpHintForOS(goos string, port ...int) string
```

---

### 2.3. `internal/platform/launcher.go`

```go
package platform

import (
	"context"
	"net/http"
)

// CDPPorts contém a lista exata de portas CDP escaneadas para conexões com o Antigravity.
var CDPPorts = []int{9222, 9223, 9333, 9444, 9555, 9666}

// HTTPGetFunc abstrai a chamada HTTP GET para permitir mocks nos testes unitários.
type HTTPGetFunc func(ctx context.Context, url string) (statusCode int, body []byte, err error)

// CheckPort verifica se o endpoint http://127.0.0.1:<port>/json/list responde em até 2000ms
// com um JSON que seja um array válido (`[...]`).
func CheckPort(port int, httpGet ...HTTPGetFunc) bool

// EnsureAntigravityRunning verifica todas as portas de CDPPorts em paralelo.
// Se ao menos uma responder com array JSON válido, loga em nível DEBUG e retorna.
// Caso todas falhem, emite o banner de aviso completo via logger.Warn.
func EnsureAntigravityRunning(httpGet ...HTTPGetFunc)
```

---

### 2.4. `internal/platform/proc_unix.go`, `internal/platform/proc_windows.go` e `QuotaService`

```go
package platform

import (
	"context"
	"sync"
)

// LanguageServerProcessInfo representa o PID e o CSRF token descobertos do processo language_server.
type LanguageServerProcessInfo struct {
	PID       int
	CSRFToken string
}

// QuotaInfo representa a fração restante e o horário de reset da cota de um modelo.
type QuotaInfo struct {
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

// ModelQuota representa a configuração e cota de um modelo retornada pelo LanguageServerService.
type ModelQuota struct {
	Label     string     `json:"label"`
	Model     string     `json:"model"`
	QuotaInfo *QuotaInfo `json:"quotaInfo,omitempty"`
}

// UserStatusData encapsula a lista de configurações de modelos do usuário.
type UserStatusData struct {
	ClientModelConfigs []ModelQuota `json:"clientModelConfigs,omitempty"`
}

// CommandRunner abstrai a execução de comandos externos (pgrep, lsof, ss, powershell, netstat) para testes.
type CommandRunner func(ctx context.Context, name string, args ...string) (stdout string, err error)

// ProcessFinder define o contrato multiplataforma implementado por proc_unix.go e proc_windows.go.
type ProcessFinder interface {
	GetProcessInfo(ctx context.Context) (*LanguageServerProcessInfo, error)
	GetListeningPorts(ctx context.Context, pid int) ([]int, error)
}

// GetLanguageServerProcessInfo descobre o PID e o --csrf_token do processo language_server ativo.
func GetLanguageServerProcessInfo(ctx context.Context, runner ...CommandRunner) (*LanguageServerProcessInfo, error)

// GetListeningPorts descobre todas as portas TCP em estado LISTEN pertencentes ao PID informado.
func GetListeningPorts(ctx context.Context, pid int, runner ...CommandRunner) ([]int, error)

// ParseUnixProcessInfo extrai (pid, csrf_token) da saída textual de `pgrep -fl language_server`
// ou linhas equivalentes de `/proc/<pid>/cmdline`.
func ParseUnixProcessInfo(stdout string) *LanguageServerProcessInfo

// ParseLsofListeningPorts extrai a lista deduplicada de portas TCP (LISTEN) da saída do `lsof`.
func ParseLsofListeningPorts(stdout string, pid int) []int

// ParseSSListeningPorts extrai a lista deduplicada de portas TCP LISTEN da saída de `ss -tlnp` para o PID.
func ParseSSListeningPorts(stdout string, pid int) []int

// ParseWindowsProcessInfo extrai (pid, csrf_token) da saída do PowerShell/WMIC no Windows.
func ParseWindowsProcessInfo(stdout string) *LanguageServerProcessInfo

// ParseNetstatListeningPorts extrai a lista deduplicada de portas TCP LISTENING da saída de `netstat -ano` para o PID.
func ParseNetstatListeningPorts(stdout string, pid int) []int

// QuotaService consulta o endpoint HTTPS local do language_server com cache de PID/Token/Porta e 1 retry.
type QuotaService struct {
	mu              sync.Mutex
	cachedPort      *int
	cachedCsrfToken string
	cachedPid       int
	finder          ProcessFinder
	requestAPI      func(ctx context.Context, port int, csrfToken string) (*UserStatusData, error)
}

func NewQuotaService(finder ...ProcessFinder) *QuotaService
func (s *QuotaService) FetchQuota(ctx context.Context) []ModelQuota
```

---

## 3. Constantes, Queries SQL, Expressões Regulares e Scripts Literais

### 3.1. Constantes e Strings Literais de `lockfile.ts`

- **Caminho do arquivo de lock**:
  ```text
  ~/.antigravity-telegram-remote/.bot.lock
  ```
- **Timeouts e Intervalos**:
  - `KillGracePeriod = 5000 * time.Millisecond` (`5000ms`)
  - `KillPollInterval = 50 * time.Millisecond` (`50ms`)
- **Mensagens de Log Literais**:
  - Ao iniciar encerramento de processo existente:
    `🔄 Stopping existing Bot process (PID: <pid>)...` (`logger.Info`)
  - Ao confirmar que o processo existente encerrou dentro de 5s:
    `✅ Existing process (PID: <pid>) stopped` (`logger.Info`)
  - Ao atingir timeout de 5s e enviar `SIGKILL`:
    `⚠️  Process did not exit with SIGTERM, force killing (SIGKILL)` (`logger.Warn`)
  - Ao detectar lockfile órfão (PID já não existe):
    `⚠️  Stale lock file detected (PID: <existingPid> has exited). Cleaning up.` (`logger.Warn`)
  - Ao adquirir o lockfile:
    `🔒 Lock acquired (PID: <process.pid>)` (`logger.Info`)
  - Ao liberar o lockfile:
    `🔓 Lock released (PID: <process.pid>)` (`logger.Info`)
  - Ao capturar exceção não tratada:
    `Uncaught exception:` (`logger.Error`)

---

### 3.2. Constantes, Caminhos e Expressões Regulares de `pathUtils.ts`

- **Caminhos Padrão do Executável Antigravity por Plataforma**:
  - **macOS (`darwin`)**:
    ```text
    /Applications/Antigravity.app/Contents/Resources/app/bin/antigravity
    ```
  - **Windows (`win32` / `windows`)**:
    - Com `LOCALAPPDATA` definido:
      ```text
      <LOCALAPPDATA>\Programs\Antigravity\Antigravity.exe
      ```
    - Sem `LOCALAPPDATA` definido (fallback):
      ```text
      Antigravity.exe
      ```
  - **Linux / Outros (`linux` / default)**:
    ```text
    antigravity
    ```
- **Mensagens de Erro Literais de `getAntigravityCliPath()`**:
  - Quando `ANTIGRAVITY_PATH` aponta para um diretório (não é arquivo regular):
    `ANTIGRAVITY_PATH validation failed: ANTIGRAVITY_PATH must be a file, got: <path>`
  - Quando `fs.statSync(ANTIGRAVITY_PATH)` lança erro (ex.: `ENOENT`):
    `ANTIGRAVITY_PATH validation failed: <err.message>`
- **Expressão Regular de `extractProjectNameFromPath(workspacePath)`**:
  ```regexp
  [/\\]
  ```
  Divide a string tanto por `/` (POSIX) quanto por `\` (Windows), remove segmentos vazios (`filter(Boolean)`) e retorna o último segmento (ou `""` se vazio).
- **Strings Literais de `getAntigravityCdpHint(port = 9222)`**:
  - `darwin`: `open -a Antigravity --args --remote-debugging-port=<port>`
  - `win32` / `windows`: `Antigravity.exe --remote-debugging-port=<port>`
  - `linux` / default: `antigravity --remote-debugging-port=<port>`

---

### 3.3. Constantes e Banner Literal de `antigravityLauncher.ts` e `cdpPorts.ts`

- **Lista de Portas CDP (`CDP_PORTS`)**:
  ```go
  []int{9222, 9223, 9333, 9444, 9555, 9666}
  ```
- **Endpoint e Timeout de Sondagem**:
  - URL: `http://127.0.0.1:<port>/json/list`
  - Timeout por requisição: `2000ms` (`2 * time.Second`)
- **Logs de Debug e Banner de Warning Literais**:
  - Antes de escanear:
    `[AntigravityLauncher] Checking CDP ports...` (`logger.Debug`)
  - Quando ao menos uma porta responde:
    `[AntigravityLauncher] OK — Port <CDP_PORTS[foundIndex]> responding` (`logger.Debug`)
  - Quando nenhuma porta responde (14 chamadas exatas a `logger.Warn`):
    ```text
    logger.Warn("")
    logger.Warn("======================================================================")
    logger.Warn("  Antigravity CDP ports are not responding")
    logger.Warn("")
    logger.Warn("  Launch Antigravity with CDP enabled:")
    logger.Warn("    antigravity-telegram-remote open")
    logger.Warn("")
    logger.Warn("  Or manually:")
    logger.Warn("    " + GetAntigravityCdpHint(9222))
    logger.Warn("")
    logger.Warn("  The bot will auto-connect once Antigravity is running.")
    logger.Warn("======================================================================")
    logger.Warn("")
    ```

---

### 3.4. Comandos, Expressões Regulares e Payload HTTP de `quotaService.ts` (`proc_unix.go` / `proc_windows.go`)

1. **Descoberta de Processo e CSRF Token**:
   - Comando primário Unix (`darwin` / `linux`):
     ```sh
     pgrep -fl language_server
     ```
   - Regex literal de extração do token CSRF (`src/services/quotaService.ts`, linha 38):
     ```regexp
     --csrf_token[=\s]+([a-zA-Z0-9\-]+)
     ```
   - Fallback Linux (`proc_unix.go` conforme ADR-001): varredura de `/proc/<pid>/cmdline` (bytes separados por `\x00` convertidos em espaços) procurando processos cujo comando contenha `language_server` e `--csrf_token`.
   - Comando Windows (`proc_windows.go` conforme ADR-001):
     ```powershell
     powershell -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { $_.Name -like '*language_server*' -or $_.CommandLine -like '*language_server*' } | Select-Object ProcessId, CommandLine | ConvertTo-Json -Compress"
     ```
     (Com fallback para `wmic process where "name like '%language_server%'" get ProcessId,CommandLine /format:csv`), aplicando a mesma regex `--csrf_token[=\s]+([a-zA-Z0-9\-]+)`.

2. **Descoberta de Portas TCP em `LISTEN`**:
   - Comando primário Unix (`darwin` / `linux`):
     ```sh
     lsof -nP -a -iTCP -sTCP:LISTEN -p <pid>
     ```
   - Regex literal de extração de portas do `lsof` (`src/services/quotaService.ts`, linha 55, flags `gim`):
     ```regexp
     (?im)^\S+\s+<pid>\s+.*?(?:TCP|UDP)\s+(?:\*|[\d.]+|\[[\da-f:]+\]):(\d+)\s+\(LISTEN\)
     ```
   - Fallback Linux (`proc_unix.go` conforme ADR-001):
     ```sh
     ss -tlnp
     ```
     Filtrando linhas com `pid=<pid>,` e extraindo a porta local `:(\d+)`.
   - Comando Windows (`proc_windows.go` conforme ADR-001):
     ```cmd
     netstat -ano -p tcp
     ```
     Regex de extração para o PID:
     ```regexp
     (?im)^\s*TCP\s+(?:[\d.]+|\[[\da-f:]+\]):(\d+)\s+\S+\s+LISTENING\s+<pid>\s*$
     ```

3. **Requisição HTTPS Local para `LanguageServerService/GetUserStatus`**:
   - **Método e URL**:
     `POST https://127.0.0.1:<port>/exa.language_server_pb.LanguageServerService/GetUserStatus`
   - **Configuração TLS e Timeout**:
     `InsecureSkipVerify: true` (loopback `127.0.0.1` com certificado autoassinado do Antigravity), `Timeout: 2000 * time.Millisecond`.
   - **Headers HTTP Literais**:
     ```http
     Content-Type: application/json
     Connect-Protocol-Version: 1
     X-Codeium-Csrf-Token: <csrfToken>
     ```
   - **Body JSON Literal**:
     ```json
     {"metadata":{"ideName":"antigravity","extensionName":"antigravity","locale":"en"}}
     ```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo de `AcquireLock()` (`internal/platform/lockfile.go`)

1. **Verificação de Lockfile Existente**:
   - Verifica se `LOCK_FILE` (`~/.antigravity-telegram-remote/.bot.lock`) existe.
   - Se existir:
     - Lê o conteúdo em UTF-8, aplica `strings.TrimSpace(content)` e converte para inteiro base 10 (`strconv.Atoi`).
     - Se `err == nil` (PID numérico válido):
       - Se `existingPid != currentPID` e `isProcessRunning(existingPid)` for `true` (`kill(existingPid, 0) == nil`):
         - Executa `killExistingProcess(existingPid)`:
           1. Loga `logger.Info(fmt.Sprintf("🔄 Stopping existing Bot process (PID: %d)...", existingPid))`.
           2. Envia `SIGTERM` (`kill(existingPid, syscall.SIGTERM)`). Se retornar erro, retorna imediatamente (já terminou).
           3. Define `deadline = now.Add(5000 * time.Millisecond)`.
           4. Enquanto `now.Before(deadline)`:
              - Se `!isProcessRunning(existingPid)`:
                - Loga `logger.Info(fmt.Sprintf("✅ Existing process (PID: %d) stopped", existingPid))`.
                - Retorna de `killExistingProcess`.
              - Dorme `50ms` (`Sleep(50 * time.Millisecond)`).
           5. Se o loop expirar após 5s com o processo ainda vivo:
              - Loga `logger.Warn("⚠️  Process did not exit with SIGTERM, force killing (SIGKILL)")`.
              - Envia `SIGKILL` (`kill(existingPid, syscall.SIGKILL)`), ignorando eventual erro.
       - Senão, se `!isProcessRunning(existingPid)`:
         - Loga `logger.Warn(fmt.Sprintf("⚠️  Stale lock file detected (PID: %d has exited). Cleaning up.", existingPid))`.
     - Se o conteúdo não for numérico (`NaN`), não loga erro nem lança pânico; apenas prossegue para remover o arquivo.
     - Remove `LOCK_FILE` (`RemoveFile(LOCK_FILE)`), ignorando erros.
2. **Criação do Novo Lockfile**:
   - Garante que o diretório pai exista: `MkdirAll(filepath.Dir(LOCK_FILE), 0755)`.
   - Escreve `strconv.Itoa(currentPID)` em `LOCK_FILE`.
   - Loga `logger.Info(fmt.Sprintf("🔒 Lock acquired (PID: %d)", currentPID))`.
3. **Função `releaseLock()`**:
   - Verifica se `LOCK_FILE` existe.
   - Se existir, lê o conteúdo, aplica `strings.TrimSpace` e faz parse do inteiro.
   - **Somente se** o PID lido for exatamente igual a `currentPID`:
     - Remove `LOCK_FILE`.
     - Loga `logger.Info(fmt.Sprintf("🔓 Lock released (PID: %d)", currentPID))`.
   - Qualquer erro durante `releaseLock()` é silenciosamente ignorado.
4. **Registro de Hooks de Encerramento**:
   - Registra captura de sinais (`SIGINT`, `SIGTERM`) e encerramento do processo para invocar `releaseLock()`.

---

### 4.2. Algoritmo de `EnsureAntigravityRunning()` (`internal/platform/launcher.go`)

1. Loga `logger.Debug("[AntigravityLauncher] Checking CDP ports...")`.
2. Dispara 6 goroutines em paralelo (uma para cada porta em `CDPPorts = [9222, 9223, 9333, 9444, 9555, 9666]`), armazenando o resultado booleano no índice correspondente `results[i]`.
3. Em cada `CheckPort(port)`:
   - Faz `GET http://127.0.0.1:<port>/json/list` com timeout de `2000ms`.
   - Se houver erro de rede/timeout, retorna `false`.
   - Faz `json.Unmarshal(body, &parsed)` onde `var parsed []any`.
   - Retorna `true` se e somente se o JSON for um array JSON válido (`err == nil && parsed != nil`).
4. Aguarda todas as 6 verificações terminarem (`wg.Wait()`).
5. Procura o primeiro índice `foundIndex` na ordem de `CDPPorts` cujo `results[foundIndex] == true`.
6. Se `foundIndex != -1`:
   - Loga `logger.Debug(fmt.Sprintf("[AntigravityLauncher] OK — Port %d responding", CDPPorts[foundIndex]))`.
   - Retorna imediatamente sem emitir warnings.
7. Se nenhuma porta respondeu (`foundIndex == -1`):
   - Emite o banner de aviso com `logger.Warn` (exatamente conforme Seção 3.3).

---

### 4.3. Algoritmo de `QuotaService.FetchQuota()` (`internal/platform/proc_*.go`)

1. Chama `GetProcessInfo(ctx)` para localizar o PID e o `csrf_token` do `language_server`.
   - Se não encontrar (`nil`), loga `logger.Error("No language_server process found.")` e retorna `[]ModelQuota{}`.
2. **Validação de Cache**:
   - Se `cachedPid != pid || cachedCsrfToken != csrf_token`:
     - Invalida `cachedPort = nil`.
     - Atualiza `cachedPid = pid` e `cachedCsrfToken = csrf_token`.
3. **Descoberta ou Uso de Porta em Cache**:
   - Se `cachedPort == nil`:
     - Obtém `ports := GetListeningPorts(ctx, pid)`.
     - Itera em ordem sobre `port` em `ports`:
       - Chama `requestApi(ctx, port, csrf_token)`.
       - Se tiver sucesso (`HTTP 200` e JSON válido), salva `cachedPort = &port` e retorna `data.ClientModelConfigs`.
       - Se falhar, continua para a próxima porta.
   - Se `cachedPort != nil`:
     - Chama `requestApi(ctx, *cachedPort, csrf_token)`.
     - Se tiver sucesso, retorna `data.ClientModelConfigs`.
     - Se falhar:
       - Define `cachedPort = nil`.
       - Se `retryCount < 1`, chama recursivamente `fetchQuota(ctx, retryCount + 1)`.
       - Se `retryCount >= 1`, loga `logger.Error("fetchQuota failed after retry:", err)` e retorna `[]ModelQuota{}`.
4. **Mapeamento de Resposta em `requestApi`**:
   - Lê `parsed.userStatus.cascadeModelConfigData.clientModelConfigs`.
   - Para cada item `c`:
     - `label = coalesce(c.label, c.displayName, c.modelName, "")`
     - `model = coalesce(c.modelOrAlias.model, c.model, c.modelId, "")`
     - `qi = coalesce(c.quotaInfo, c.quota, c.usageInfo)`
     - Se `qi` existir, `QuotaInfo = &QuotaInfo{ RemainingFraction: parseFloat(qi.remainingFraction ?? qi.remaining ?? 0), ResetTime: coalesce(qi.resetTime, qi.resetAt, "") }`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Sondagem Paralela em `EnsureAntigravityRunning`**:
   - Utiliza `sync.WaitGroup` e um slice pré-alocado `results := make([]bool, len(CDPPorts))` onde cada goroutine escreve exclusivamente em sua posição `results[i]`, eliminando data races e preservando a ordem determinística de avaliação (`indexOf(true)`).
2. **Sincronização em `QuotaService`**:
   - Os campos `cachedPort`, `cachedPid` e `cachedCsrfToken` são protegidos por `sync.Mutex` para evitar condições de corrida caso dois usuários/tópicos executem `/status` simultaneamente.
3. **Ciclo de Vida do Lockfile**:
   - `AcquireLock()` é invocado no início do comando `start` (antes de inicializar o banco SQLite e o bot do Telegram).
   - A função `releaseLock` retornada é registrada tanto em `defer releaseLock()` na `main`/`startAction` quanto no handler de `os/signal` (`syscall.SIGINT`, `syscall.SIGTERM`) com `sync.Once` ou verificação idempotente de PID no arquivo.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os testes abaixo mapeiam **1 para 1** os casos dos arquivos originais em `tests/` e devem passar com `go test -v -race ./internal/platform/...`.

### 6.1. `internal/platform/lockfile_test.go` (Mapeado de [`tests/utils/lockfile.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/lockfile.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestLockfile_CreatesWithCurrentPIDWhenNoExistingLock` | `lockfile > creates lock file with current PID when no existing lock` | `FileExists` retorna `false` | `WriteFile` é chamado com `expectedLockPath` e `strconv.Itoa(currentPID)`. |
| `TestLockfile_EnsuresDirectoryIsCreated` | `lockfile > ensures ~/.antigravity-telegram-remote directory is created` | `FileExists` retorna `false` | `MkdirAll` é chamado com `filepath.Dir(expectedLockPath)`. |
| `TestLockfile_CleansUpStaleLockWhenPIDNotRunning` | `lockfile > cleans up stale lock file when PID is not running` | `FileExists=true`, `ReadFile="99999"`, `KillProcess(99999, 0)` retorna erro `ESRCH` | `RemoveFile` é chamado com `expectedLockPath` antes de gravar o novo PID. |
| `TestLockfile_HandlesNaNPIDGracefully` | `lockfile > handles NaN PID in lock file gracefully` | `FileExists=true`, `ReadFile="not-a-number"` | `AcquireLockWithDeps` não entra em pânico, remove o arquivo inválido e grava o novo PID. |
| `TestLockfile_KillsExistingRunningProcess` | *(Cobertura adicional da função `killExistingProcess` em `lockfile.ts`)* | `FileExists=true`, `ReadFile="88888"`, `KillProcess(88888, 0)` retorna `nil` 2 vezes e depois `ESRCH` | Envia `SIGTERM` para `88888`, aguarda com `Sleep(50ms)` e confirma parada sem precisar de `SIGKILL` (e testa também o fallback para `SIGKILL` após 5s). |
| `TestLockfile_ReturnsReleaseFunction` | `lockfile > returns a release function` | `FileExists=false` | `release := AcquireLockWithDeps(...)` retorna função não-nula. |
| `TestLockfile_ReleaseDeletesLockFileWhenPIDMatches` | `lockfile > release function deletes lock file when PID matches` | Durante `release()`, `FileExists=true` e `ReadFile=strconv.Itoa(currentPID)` | `RemoveFile` é chamado com `expectedLockPath`. |
| `TestLockfile_ReleaseDoesNotDeleteWhenPIDDoesNotMatch` | `lockfile > release function does not delete lock file when PID does not match` | Durante `release()`, `FileExists=true` e `ReadFile="99999"` (outro PID) | `RemoveFile` **não** é chamado. |
| `TestLockfile_RegistersProcessExitHandlers` | `lockfile > registers process exit handlers` | Spy em `RegisterExitHooks` | Confirma que o hook de saída/sinais (`exit`, `SIGINT`, `SIGTERM`, `uncaughtException`) é registrado em `AcquireLock`. |

---

### 6.2. `internal/platform/paths_test.go` (Mapeado de [`tests/utils/pathUtils.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/pathUtils.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestPathUtils_ExtractProjectName_POSIX` | `pathUtils > extractProjectNameFromPath() > extracts name from POSIX path` | `"/home/user/Code/MyProject"` | Retorna `"MyProject"`. |
| `TestPathUtils_ExtractProjectName_Windows` | `pathUtils > extractProjectNameFromPath() > extracts name from Windows path` | `"D:\\Code\\MyProject"` | Retorna `"MyProject"`. |
| `TestPathUtils_ExtractProjectName_WindowsDriveRoot` | `pathUtils > extractProjectNameFromPath() > extracts name from Windows drive root` | `"D:\\categorizer"` | Retorna `"categorizer"`. |
| `TestPathUtils_ExtractProjectName_TrailingSlash` | `pathUtils > extractProjectNameFromPath() > handles trailing slash` | `"/home/user/Code/MyProject/"` | Retorna `"MyProject"`. |
| `TestPathUtils_ExtractProjectName_TrailingBackslash` | `pathUtils > extractProjectNameFromPath() > handles trailing backslash` | `"C:\\Code\\MyProject\\"` | Retorna `"MyProject"`. |
| `TestPathUtils_ExtractProjectName_MixedSeparators` | `pathUtils > extractProjectNameFromPath() > handles mixed separators` | `"C:\\Users\\test/Code/MyProject"` | Retorna `"MyProject"`. |
| `TestPathUtils_ExtractProjectName_EmptyInput` | `pathUtils > extractProjectNameFromPath() > returns empty string for empty input` | `""` | Retorna `""`. |
| `TestPathUtils_ExtractProjectName_SimpleName` | `pathUtils > extractProjectNameFromPath() > returns name as-is for simple name` | `"MyProject"` | Retorna `"MyProject"`. |
| `TestPathUtils_GetAntigravityCdpHint_MacOS` | `pathUtils > getAntigravityCdpHint() > returns open -a hint on macOS` | `GOOS="darwin"`, `port=9222` | Retorna `"open -a Antigravity --args --remote-debugging-port=9222"`. |
| `TestPathUtils_GetAntigravityCdpHint_Windows` | `pathUtils > getAntigravityCdpHint() > returns exe hint on Windows` | `GOOS="windows"` (ou `"win32"`), `port=9222` | Retorna `"Antigravity.exe --remote-debugging-port=9222"`. |
| `TestPathUtils_GetAntigravityCdpHint_Linux` | `pathUtils > getAntigravityCdpHint() > returns lowercase hint on Linux` | `GOOS="linux"`, `port=9222` | Retorna `"antigravity --remote-debugging-port=9222"`. |
| `TestPathUtils_GetAntigravityCdpHint_DefaultPort9222` | `pathUtils > getAntigravityCdpHint() > uses default port 9222` | `GOOS="darwin"`, sem argumento de porta | Resultado contém `"9222"`. |
| `TestPathUtils_GetAntigravityCdpHint_CustomPort` | `pathUtils > getAntigravityCdpHint() > uses custom port` | `GOOS="darwin"`, `port=9333` | Resultado contém `"9333"`. |
| `TestPathUtils_GetAntigravityCliPath_AllPlatformsAndOverride` | *(Cobertura de `getAntigravityCliPath` em `pathUtils.ts`)* | Testa `ANTIGRAVITY_PATH` (arquivo válido, diretório inválido, arquivo inexistente) e defaults para `darwin`, `windows` e `linux` | Retorna o caminho validado ou erro `"ANTIGRAVITY_PATH validation failed: ..."`. |

---

### 6.3. `internal/platform/launcher_test.go` (Mapeado de [`tests/services/antigravityLauncher.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/antigravityLauncher.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestEnsureAntigravityRunning_ReturnsImmediatelyWhenPortResponds` | `ensureAntigravityRunning > returns immediately when a port responds (parallel scan)` | `logger.SetLogLevel(LevelDebug)`; mock HTTP retorna `200` com `"[]"` na porta `9222` e erro de conexão nas outras 5 portas | Todas as 6 portas (`9222, 9223, 9333, 9444, 9555, 9666`) são consultadas em paralelo (`http://127.0.0.1:<port>/json/list`); `DebugOut` contém `"\x1b[2m[DEBUG]\x1b[0m"` e `"[AntigravityLauncher] OK — Port 9222 responding"`; `WarnOut` **não** contém `"Antigravity CDP ports are not responding"`. |
| `TestEnsureAntigravityRunning_OutputsWarningWhenAllPortsFail` | `ensureAntigravityRunning > outputs a warning log when all ports fail` | Mock HTTP falha em todas as 6 portas | Todas as 6 portas são consultadas; `WarnOut` contém `"\x1b[33m[WARN]\x1b[0m"` e `"  Antigravity CDP ports are not responding"`. |
