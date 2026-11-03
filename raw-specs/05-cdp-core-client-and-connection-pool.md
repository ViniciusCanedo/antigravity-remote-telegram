# Especificação Técnica 05 — Cliente Core CDP, Descoberta de Alvos, Reconexão e Pool de Conexões (`internal/cdp`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação define de forma 100% autocontida e exaustiva a camada de protocolo Chrome DevTools Protocol (CDP), descoberta de alvos HTTP (`/json/list`), lançamento multiplataforma do binário Antigravity, transporte WebSocket JSON-RPC 2.0, rastreamento de contextos de execução V8, heartbeat ping/pong, reconexão automática e o pool de conexões e detectores por workspace.

### 1.1. Mapeamento de Arquivos Originais (TypeScript) $\rightarrow$ Destino (Go)

| Arquivo Original (TypeScript) | Linhas | Arquivo Destino (Go) | Responsabilidade Principal |
| :--- | :---: | :--- | :--- |
| [`src/utils/cdpPorts.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/cdpPorts.ts) | 2 | `internal/cdp/ports.go` | Lista imutável de portas TCP candidatas (`CDP_PORTS`) e verificação de disponibilidade de porta local (`127.0.0.1`). |
| [`src/services/cdpService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpService.ts) (Linhas 1–944) | 944 | `internal/cdp/client.go` | Cliente WebSocket JSON-RPC (`Call`), varredura de portas (`/json/list`), descoberta de janela de workspace (`DiscoverTarget`, `DiscoverAndConnectForWorkspace`, `probeWorkbenchPages`, `probeWorkspaceFolderPath`), lançamento do processo Antigravity (`launchAndConnectWorkspace`), rastreamento de contextos V8 (`Runtime.executionContextCreated`/`Destroyed`), heartbeat (`30s` ping / `40s` timeout) e reconexão automática (`tryReconnect`). |
| [`src/services/cdpConnectionPool.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpConnectionPool.ts) | 283 | `internal/cdp/pool.go` | Gerenciamento de instâncias `Client` independentes por workspace (`projectName`), prevenção de race conditions na conexão concorrente (`connectingPromises`), revalidação de janelas abertas, registro/parada dos 4 detectores em background e emissão de eventos de ciclo de vida (`workspace:disconnected`, `workspace:reconnected`, `workspace:reconnectFailed`). |
| [`src/utils/pathUtils.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/pathUtils.ts) | 70 | `internal/platform/paths.go` (consumido por `internal/cdp`) | Funções utilitárias `getAntigravityCliPath()` e `extractProjectNameFromPath(workspacePath)`. |

### 1.2. Mapeamento de Arquivos de Teste Originais $\rightarrow$ Suítes de Teste em Go

| Arquivo de Teste Original (Jest) | Linhas | Arquivo de Teste Destino (Go) | Escopo de Verificação |
| :--- | :---: | :--- | :--- |
| [`tests/services/cdpService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.test.ts) | 125 | `internal/cdp/client_test.go` | Descoberta de alvo (`workbench.html` vs `Launchpad`), conexão WebSocket, `Runtime.enable`, recebimento de `Runtime.executionContextCreated` (`cascade-panel` id=2) e emissão do evento `disconnected`. |
| [`tests/services/cdpService.workspace.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.workspace.test.ts) | 285 | `internal/cdp/client_workspace_test.go` | Lançamento multiplataforma (`darwin` com fallback `open -a Antigravity`, `win32` com `LOCALAPPDATA`, `linux` com `ANTIGRAVITY_PATH`), polling pós-lançamento e extração de `projectName` com barras POSIX, Windows e trailing slashes. |
| [`tests/services/cdpService.errorHandling.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.errorHandling.test.ts) | 223 | `internal/cdp/client_error_test.go` | Limpeza e rejeição de `pendingCalls` na desconexão (prevenção de vazamento de memória), reconexão determinística ao `currentWorkspacePath`, respeito a `maxReconnectAttempts` (`0`, `1`, `2`, default `3`), `reconnectDelayMs` (default `2000ms`) e emissão de `reconnectFailed`. |
| [`tests/services/cdpService.security.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.security.test.ts) | 11 | `internal/cdp/client_security_test.go` | Garantia de execução de comandos externos sem interpolação de shell (`exec.Command` direto, equivalente a `spawn(..., { shell: false })`, nunca `sh -c` / `cmd /c`). |
| [`tests/services/cdpConnectionPool.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpConnectionPool.test.ts) | 283 | `internal/cdp/pool_test.go` | `ExtractProjectName`, `GetOrConnect` (cache, revalidação, múltiplos workspaces, singleflight/lock de conexões concorrentes), `GetConnected`, `DisconnectWorkspace`, `DisconnectAll`, gerenciamento de detectores e `GetActiveWorkspaceNames`. |
| [`tests/services/cdpConnectionPool.bugfix.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpConnectionPool.bugfix.test.ts) | 145 | `internal/cdp/pool_bugfix_test.go` | Desconexão e remoção de entrada obsoleta quando a revalidação falha, limpeza de entrada já desconectada sem chamar `Disconnect()` redundante, e limpeza completa via `DisconnectWorkspace` (incluindo parada de detectores) quando ocorre `reconnectFailed`. |

### 1.3. Documentação Arquitetural Relacionada
- [`docs/03-protocolo-cdp-e-seletores-dom.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/03-protocolo-cdp-e-seletores-dom.md)

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pacote `internal/cdp` — `ports.go`

```go
package cdp

import (
	"fmt"
	"net"
)

// DefaultCDPPorts corresponde exatamente a CDP_PORTS em src/utils/cdpPorts.ts:
// export const CDP_PORTS = [9222, 9223, 9333, 9444, 9555, 9666] as const;
var DefaultCDPPorts = []int{9222, 9223, 9333, 9444, 9555, 9666}

// IsPortAvailable verifica se uma porta TCP local em 127.0.0.1 está livre para uso,
// realizando bind temporário via net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)).
func IsPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// FindAvailablePort retorna a primeira porta livre da lista fornecida (ou 0 e false se todas estiverem ocupadas).
func FindAvailablePort(ports []int) (int, bool) {
	for _, port := range ports {
		if IsPortAvailable(port) {
			return port, true
		}
	}
	return 0, false
}
```

### 2.2. Pacote `internal/cdp` — `client.go`

```go
package cdp

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// ClientOptions espelha CdpServiceOptions (src/services/cdpService.ts:10-17).
type ClientOptions struct {
	// PortsToScan define as portas TCP locais a escanear. Default: DefaultCDPPorts.
	PortsToScan []int
	// CallTimeout define o timeout por chamada RPC CDP. Default: 30000ms (30s).
	CallTimeout time.Duration
	// MaxReconnectAttempts define o número de tentativas de reconexão após queda.
	// Ponteiro ou flag opcional para distinguir 0 explícito (sem reconexão) do default (3).
	MaxReconnectAttempts *int
	// ReconnectDelay define o intervalo entre tentativas de reconexão. Default: 2000ms (2s).
	ReconnectDelay time.Duration

	// Hooks injetáveis para testes unitários determinísticos:
	HTTPGetJSONFunc       func(url string) ([]TargetInfo, error)
	RunCommandFunc        func(command string, args []string) error
	FindAvailablePortFunc func() (int, bool)
	GetAntigravityCliFunc func() (string, error)
	DialWebSocketFunc     func(ctx context.Context, url string) (WebSocketConn, error)
}

// Context espelha CdpContext (src/services/cdpService.ts:19-23).
type Context struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// TargetInfo representa um item retornado por GET http://127.0.0.1:<port>/json/list.
type TargetInfo struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// WebSocketConn abstrai a conexão WebSocket para permitir mock em testes unitários.
type WebSocketConn interface {
	WriteMessage(data []byte) error
	ReadMessage() ([]byte, error)
	Ping() error
	SetPongHandler(h func(appData string) error)
	Close() error
	Terminate() error
}

// EventHandler é o callback para eventos emitidos pelo Client (ex: "disconnected",
// "reconnected", "reconnectFailed", "Runtime.executionContextCreated", "Network.*").
type EventHandler func(payload any)

// pendingCall armazena o canal de resposta e o timer de timeout de uma requisição JSON-RPC.
type pendingCall struct {
	resultCh chan rpcResponse
	timer    *time.Timer
}

type rpcRequest struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage
	Err    error
}

type rpcErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type incomingMessage struct {
	ID     *int64          `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcErrorPayload `json:"error,omitempty"`
}

// Client implementa toda a funcionalidade de CdpService (src/services/cdpService.ts).
type Client struct {
	mu                   sync.RWMutex
	ports                []int
	isConnectedFlag      bool
	ws                   WebSocketConn
	contexts             []Context
	pendingCalls         map[int64]*pendingCall
	idCounter            int64
	cdpCallTimeout       time.Duration
	targetURL            string
	maxReconnectAttempts int
	reconnectDelay       time.Duration
	reconnectAttemptCnt  int
	isReconnecting       bool
	currentWorkspaceName string
	currentWorkspacePath string
	isSwitchingWorkspace bool
	heartbeatStopCh      chan struct{}
	lastPongTime         time.Time
	listeners            map[string][]EventHandler

	// Funções injetáveis (com defaults de produção)
	getJSONFunc           func(url string) ([]TargetInfo, error)
	runCommandFunc        func(command string, args []string) error
	findAvailablePortFunc func() (int, bool)
	getAntigravityCliFunc func() (string, error)
	dialWebSocketFunc     func(ctx context.Context, url string) (WebSocketConn, error)
}

// NewClient constrói uma instância de Client com os mesmos defaults do construtor TypeScript.
func NewClient(opts ClientOptions) *Client

// Métodos públicos de ciclo de vida, descoberta e RPC (Parte 1):
func (c *Client) DiscoverTarget() (string, error)
func (c *Client) Connect() error
func (c *Client) Call(method string, params any) (json.RawMessage, error)
func (c *Client) Disconnect() error
func (c *Client) GetCurrentWorkspaceName() string
func (c *Client) DiscoverAndConnectForWorkspace(workspacePath string) (bool, error)
func (c *Client) IsConnected() bool
func (c *Client) GetContexts() []Context
func (c *Client) WaitForCascadePanelReady(timeout time.Duration, pollInterval time.Duration) bool
func (c *Client) GetPrimaryContextID() *int
func (c *Client) On(event string, handler EventHandler)
func (c *Client) Emit(event string, payload any)

// Métodos internos do protocolo (Parte 1):
func (c *Client) getJSON(url string) ([]TargetInfo, error)
func (c *Client) scanAllPorts() []TargetInfo
func (c *Client) verifyCurrentWorkspace(projectName, workspacePath string) bool
func (c *Client) discoverAndConnectForWorkspaceImpl(workspacePath, projectName string) (bool, error)
func (c *Client) connectToPage(page TargetInfo, projectName string) (bool, error)
func (c *Client) probeWorkbenchPages(workbenchPages []TargetInfo, projectName, workspacePath string) bool
func (c *Client) probeWorkspaceFolderPath(projectName, workspacePath string) bool
func (c *Client) launchAndConnectWorkspace(workspacePath, projectName string) (bool, error)
func (c *Client) runCommand(command string, args []string) error
func (c *Client) isPortAvailable(port int) bool
func (c *Client) findAvailableCdpPort() (int, bool)
func (c *Client) disconnectQuietly()
func (c *Client) startHeartbeat()
func (c *Client) stopHeartbeat()
func (c *Client) clearPendingCalls(err error)
func (c *Client) tryReconnect()
```

### 2.3. Pacote `internal/cdp` — `pool.go`

```go
package cdp

import (
	"sync"
)

// BackgroundDetector representa o contrato comum aos 4 detectores registrados no pool:
// ApprovalDetector, ErrorPopupDetector, PlanningDetector e UserMessageDetector.
type BackgroundDetector interface {
	IsActive() bool
	Start()
	Stop()
}

// ClientFactory permite injetar mocks de Client nos testes unitários do ConnectionPool.
type ClientFactory func(opts ClientOptions) PoolClient

// PoolClient define a interface que o ConnectionPool exige de cada instância CdpService.
type PoolClient interface {
	IsConnected() bool
	DiscoverAndConnectForWorkspace(workspacePath string) (bool, error)
	Disconnect() error
	On(event string, handler EventHandler)
	Call(method string, params any) (json.RawMessage, error)
	GetContexts() []Context
	GetPrimaryContextID() *int
	WaitForCascadePanelReady(timeout, pollInterval time.Duration) bool
}

// ConnectionPool espelha CdpConnectionPool (src/services/cdpConnectionPool.ts:23-283).
type ConnectionPool struct {
	mu                   sync.Mutex
	connections          map[string]PoolClient
	approvalDetectors    map[string]BackgroundDetector
	errorPopupDetectors  map[string]BackgroundDetector
	planningDetectors    map[string]BackgroundDetector
	userMessageDetectors map[string]BackgroundDetector
	connectingCalls      map[string]*inflightConnect
	cdpOptions           ClientOptions
	clientFactory        ClientFactory
	listeners            map[string][]func(projectName string)
}

type inflightConnect struct {
	done chan struct{}
	cdp  PoolClient
	err  error
}

func NewConnectionPool(opts ClientOptions, factory ...ClientFactory) *ConnectionPool

func (p *ConnectionPool) GetOrConnect(workspacePath string) (PoolClient, error)
func (p *ConnectionPool) GetConnected(projectName string) PoolClient
func (p *ConnectionPool) DisconnectWorkspace(projectName string)
func (p *ConnectionPool) DisconnectAll()

func (p *ConnectionPool) RegisterApprovalDetector(projectName string, detector BackgroundDetector)
func (p *ConnectionPool) GetApprovalDetector(projectName string) BackgroundDetector

func (p *ConnectionPool) RegisterErrorPopupDetector(projectName string, detector BackgroundDetector)
func (p *ConnectionPool) GetErrorPopupDetector(projectName string) BackgroundDetector

func (p *ConnectionPool) RegisterPlanningDetector(projectName string, detector BackgroundDetector)
func (p *ConnectionPool) GetPlanningDetector(projectName string) BackgroundDetector

func (p *ConnectionPool) RegisterUserMessageDetector(projectName string, detector BackgroundDetector)
func (p *ConnectionPool) GetUserMessageDetector(projectName string) BackgroundDetector

func (p *ConnectionPool) GetActiveWorkspaceNames() []string
func (p *ConnectionPool) ExtractProjectName(workspacePath string) string
func (p *ConnectionPool) On(event string, handler func(projectName string))
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

### 3.1. Constantes de Portas e Timeouts

| Nome Original | Valor | Uso |
| :--- | :--- | :--- |
| `CDP_PORTS` | `[9222, 9223, 9333, 9444, 9555, 9666]` | Portas escaneadas em `127.0.0.1` para `/json/list` e alocação de nova janela. |
| `cdpCallTimeout` | `30000` (30s) | Timeout padrão de uma chamada `call(method, params)`. |
| `maxReconnectAttempts` | `3` | Número padrão de tentativas de reconexão automática após queda de WebSocket. |
| `reconnectDelayMs` | `2000` (2s) | Intervalo fixo (`setTimeout(r, this.reconnectDelayMs)`) entre tentativas de reconexão em `tryReconnect()`. |
| `getJson` HTTP timeout | `5000` (5s) | Timeout de socket HTTP em `http.get("http://127.0.0.1:<port>/json/list")`. |
| `connect` WebSocket timeout | `10000` (10s) | Timeout máximo para o evento `open` do WebSocket ao conectar em `targetUrl`. |
| `heartbeatInterval` | `30000` (30s) | Intervalo de envio de `ws.ping()`. |
| `heartbeatPongTimeout` | `40000` (40s) | Tempo máximo sem receber `pong` (`Date.now() - this.lastPongTime > 40_000`) antes de chamar `ws.terminate()`. |
| `launchAndConnectWorkspace` `maxWaitMs` | `30000` (30s) | Tempo máximo de polling esperando uma nova janela de workbench aparecer após lançar o Antigravity. |
| `launchAndConnectWorkspace` `pollIntervalMs` | `1000` (1s) | Intervalo entre ciclos de polling após o lançamento do binário. |
| `launchAndConnectWorkspace` `untitledFallbackAfterMs` | `10000` (10s) | Tempo decorrido após o qual uma única página nova `"Untitled"` (ou título vazio) não presente em `knownPageIds` é aceita como fallback. |
| `waitForCascadePanelReady` `timeoutMs` | `10000` (10s) | Timeout padrão aguardando contexto cujo `.url` contenha `'cascade-panel'`. |
| `waitForCascadePanelReady` `pollIntervalMs` | `500` (500ms) | Intervalo de verificação da lista `this.contexts`. |

### 3.2. Constantes `SELECTORS` (`src/services/cdpService.ts:50-59`)

```ts
const SELECTORS = {
    /** Chat input box: textbox excluding xterm */
    CHAT_INPUT: 'div[role="textbox"]:not(.xterm-helper-textarea)',
    /** Submit button search target tag */
    SUBMIT_BUTTON_CONTAINER: 'button',
    /** Submit icon SVG class candidates */
    SUBMIT_BUTTON_SVG_CLASSES: ['lucide-arrow-right', 'lucide-arrow-up', 'lucide-send'],
    /** Keyword to identify message injection target context */
    CONTEXT_URL_KEYWORD: 'cascade-panel',
};
```

### 3.3. Expressões Regulares Utilizadas

1. **Extração do Nome do Workspace a partir do Título da Janela (`discoverTarget`, linha 184)**:
   - Regex: `/\s[—–-]\s/` (espaço seguido de travessão longo `—` U+2014, meia-risca `–` U+2013 ou hífen `-` U+002D, seguido de espaço).
   - Uso: `const titleParts = target.title.split(/\s[—–-]\s/); if (titleParts.length > 0) this.currentWorkspaceName = titleParts[0].trim();`
2. **Extração do Nome do Projeto a partir do Caminho (`extractProjectNameFromPath`, `src/utils/pathUtils.ts:52`)**:
   - Regex: `/[/\\]/`
   - Uso: `workspacePath.split(/[/\\]/).filter(Boolean).pop() || ''`
   - Em `CdpConnectionPool.extractProjectName(workspacePath)`: retorna `extractProjectNameFromPath(workspacePath) || workspacePath`.

### 3.4. Scripts JS/DOM Literais Avaliados via `Runtime.evaluate` (Parte 1)

#### 3.4.1. Verificação de Título Vivo da Página (`verifyCurrentWorkspace` e `probeWorkbenchPages`, linhas 376–379 e 502–505)
```javascript
document.title
```
*(Avaliado com `{ expression: 'document.title', returnByValue: true }`)*

#### 3.4.2. Sonda de Caminho de Pasta do Workspace no DOM (`probeWorkspaceFolderPath`, linhas 551–574)
Copiado 100% literalmente de `src/services/cdpService.ts`:
```javascript
(() => {
                // Method 1: Check window title data attribute
                const titleEl = document.querySelector('title');
                if (titleEl && titleEl.textContent) {
                    const t = titleEl.textContent;
                    if (t !== document.title) return { found: true, source: 'title-element', value: t };
                }
                
                // Method 2: Check folder name in explorer view
                const explorerItems = document.querySelectorAll('.explorer-item-label, .monaco-icon-label .label-name');
                const folderNames = Array.from(explorerItems).map(e => (e.textContent || '').trim()).filter(Boolean);
                if (folderNames.length > 0) return { found: true, source: 'explorer', value: folderNames.join(',') };
                
                // Method 3: Get path from tab titles or breadcrumbs
                const breadcrumbs = document.querySelectorAll('.breadcrumbs-view .folder-icon, .tabs-breadcrumbs .label-name');
                const crumbs = Array.from(breadcrumbs).map(e => (e.textContent || '').trim()).filter(Boolean);
                if (crumbs.length > 0) return { found: true, source: 'breadcrumbs', value: crumbs.join(',') };
                
                // Method 4: Check body data-uri attribute, etc.
                const bodyUri = document.body?.getAttribute('data-uri') || '';
                if (bodyUri) return { found: true, source: 'data-uri', value: bodyUri };
                
                return { found: false };
            })()
```
*(Avaliado com `{ expression, returnByValue: true }`)*

#### 3.4.3. Sonda de Fallback de URL da Página (`probeWorkspaceFolderPath`, linhas 601–604)
```javascript
window.location.href
```
*(Avaliado com `{ expression: 'window.location.href', returnByValue: true }`)*

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Construtor e Inicialização (`CdpService.constructor`)
1. Recebe `options: CdpServiceOptions = {}`.
2. `this.ports = options.portsToScan || [...CDP_PORTS]` (`[9222, 9223, 9333, 9444, 9555, 9666]`).
3. `this.cdpCallTimeout = options.cdpCallTimeout || 30000`.
4. `this.maxReconnectAttempts = options.maxReconnectAttempts ?? 3` (importante: se `0` for passado, deve ser mantido como `0`, não substituído por `3`).
5. `this.reconnectDelayMs = options.reconnectDelayMs ?? 2000`.

### 4.2. Consulta HTTP `/json/list` e Varredura de Portas (`getJson` e `scanAllPorts`)
1. `getJson(url)` faz `GET` HTTP com timeout de **5000ms**:
   - Se `statusCode < 200 || statusCode >= 300`: drena o body e rejeita com `Error("HTTP <statusCode> from <url>")`.
   - Lê o body completo e faz `JSON.parse(data)`. Se falhar, rejeita o erro de parse.
   - Se ocorrer timeout (5000ms), destrói a requisição e rejeita com `Error("Timeout fetching <url>")`.
2. `scanAllPorts()` itera sequencialmente sobre `this.ports`:
   - Chama `this.getJson("http://127.0.0.1:" + port + "/json/list")`.
   - Se retornar um array, concatena os elementos em `allPages`.
   - Erros de portas que não respondem são silenciosamente ignorados (`catch {}`).

### 4.3. Descoberta Genérica de Alvo (`discoverTarget`, linhas 149–193)
1. Chama `allPages = await this.scanAllPorts()`.
2. Aplica **3 níveis de prioridade** (usando `.find`) para localizar o alvo:
   - **Nível 1 (Estrito)**:
     - `t.type === 'page'`
     - `Boolean(t.webSocketDebuggerUrl)`
     - `!t.title?.includes('Launchpad')`
     - `!t.url?.includes('workbench-jetski-agent')`
     - `(t.url?.includes('workbench') || t.title?.includes('Antigravity') || t.title?.includes('Cascade'))`
   - **Nível 2 (Fallback sem restrição de `type === 'page'` e `workbench-jetski-agent`, mas ainda excluindo `Launchpad`)**:
     - `Boolean(t.webSocketDebuggerUrl)`
     - `(t.url?.includes('workbench') || t.title?.includes('Antigravity') || t.title?.includes('Cascade'))`
     - `!t.title?.includes('Launchpad')`
   - **Nível 3 (Fallback amplo incluindo `Launchpad`)**:
     - `Boolean(t.webSocketDebuggerUrl)`
     - `(t.url?.includes('workbench') || t.title?.includes('Antigravity') || t.title?.includes('Cascade') || t.title?.includes('Launchpad'))`
3. Se encontrar `target` com `target.webSocketDebuggerUrl`:
   - Define `this.targetUrl = target.webSocketDebuggerUrl`.
   - Se `target.title` existir e `!this.currentWorkspaceName`: divide `target.title` por `/\s[—–-]\s/` e atribui `this.currentWorkspaceName = titleParts[0].trim()`.
   - Retorna `target.webSocketDebuggerUrl`.
4. Caso contrário, retorna erro `"CDP target not found on any port."`.

### 4.4. Conexão WebSocket e Inicialização de Domínios CDP (`connect`, linhas 195–291)
1. Se `this.targetUrl` estiver vazio, chama `await this.discoverTarget()`. Se continuar vazio, retorna erro `"Target URL not established."`.
2. Abre conexão WebSocket para `this.targetUrl` com timeout de **10000ms**:
   - Se o timeout de 10s expirar antes do evento `open`: remove listeners, termina o socket (`terminate()`), limpa `this.ws = null` e retorna erro `"CDP connection timeout for <targetUrl>"`.
   - No evento `open`: cancela o timeout e define `this.isConnectedFlag = true`.
   - No evento `error` durante o handshake: cancela o timeout e retorna o erro.
3. Registra o handler de mensagens (`message`):
   - Faz parse JSON da mensagem recebida.
   - Se `data.id !== undefined` e `this.pendingCalls.has(data.id)`:
     - Cancela `timeoutId` e remove `data.id` de `this.pendingCalls`.
     - Se `data.error` existir: rejeita a chamada com `new Error(data.error.message || JSON.stringify(data.error))`.
     - Caso contrário: resolve a chamada com `data.result`.
   - Se `data.method === 'Runtime.executionContextCreated'`:
     - Adiciona `data.params.context` ao slice `this.contexts`.
   - Se `data.method === 'Runtime.executionContextDestroyed'`:
     - Localiza o índice onde `c.id === data.params.executionContextId` e remove de `this.contexts`.
   - Se `data.method` não for vazio:
     - Emite o evento `this.emit(data.method, data.params)` (permitindo que consumidores escutem `Runtime.bindingCalled`, `Network.*`, etc.).
4. Registra o handler de fechamento (`close`):
   - Chama `this.stopHeartbeat()`.
   - Define `this.isConnectedFlag = false`.
   - Chama `this.clearPendingCalls(new Error('WebSocket disconnected'))`.
   - Define `this.ws = null` e `this.targetUrl = null`.
   - Se `this.isSwitchingWorkspace` for `true`: retorna imediatamente (suprime o evento `disconnected` e não inicia reconexão).
   - Emite `this.emit('disconnected')`.
   - Se `this.maxReconnectAttempts > 0 && !this.isReconnecting`: dispara `this.tryReconnect()` assincronamente.
5. Após abrir o WebSocket, envia em sequência:
   - `await this.call('Runtime.enable', {})` (obrigatório; se falhar, propaga erro).
   - `await this.call('Runtime.addBinding', { name: 'onantigravity-telegram-remoteDOMChange' })` (não-fatal: envolto em `try/catch`, apenas loga warning se falhar).
   - `await this.call('Network.enable', {})` (não-fatal: envolto em `try/catch`, apenas loga warning se falhar).
6. Inicia o heartbeat chamando `this.startHeartbeat()`.

### 4.5. Transporte RPC (`call`, linhas 293–310) e Desconexão (`disconnect`, linhas 312–328)
1. **`call(method, params = {})`**:
   - Verifica se `this.ws` existe e está aberto (`WebSocket.OPEN`). Se não estiver, retorna erro imediato `"WebSocket is not connected"`.
   - Incrementa atomicamente `id = this.idCounter++` (iniciando em `1`).
   - Agenda timer de `this.cdpCallTimeout` (padrão `30000ms`): ao disparar, se `pendingCalls` ainda contiver `id`, remove-o e rejeita com `"Timeout calling CDP method " + method`.
   - Armazena em `this.pendingCalls` e envia `JSON.stringify({ id, method, params })` pelo WebSocket.
2. **`disconnect()`**:
   - Chama `this.stopHeartbeat()`.
   - Salva `savedMaxReconnectAttempts = this.maxReconnectAttempts` e define `this.maxReconnectAttempts = 0` para impedir que o evento `close` dispare `tryReconnect()`.
   - Se `this.ws` existir, fecha o WebSocket (`this.ws.close()`) e define `this.ws = null`.
   - Define `this.isConnectedFlag = false`, `this.contexts = []`, `this.currentWorkspacePath = null`, `this.currentWorkspaceName = null`.
   - Chama `this.clearPendingCalls(new Error('disconnect() was called'))`.
   - Restaura `this.maxReconnectAttempts = savedMaxReconnectAttempts`.

### 4.6. Descoberta e Conexão por Workspace (`discoverAndConnectForWorkspace`, linhas 344–618)
1. Extrai `projectName = extractProjectNameFromPath(workspacePath)` e salva `this.currentWorkspacePath = workspacePath`.
2. **Revalidação de Conexão Existente**:
   - Se `this.isConnectedFlag && this.currentWorkspaceName === projectName`:
     - Chama `stillMatched = await this.verifyCurrentWorkspace(projectName, workspacePath)`.
     - Se `stillMatched` for `true`, retorna `true` sem reconectar.
     - Se for `false`, loga warning de mismatch e prossegue para reconexão.
3. **Sonda `verifyCurrentWorkspace(projectName, workspacePath)`**:
   - Se o WebSocket não estiver conectado/aberto, retorna `false`.
   - Avalia `document.title` via `Runtime.evaluate`. Se `liveTitle.toLowerCase().includes(projectName.toLowerCase())`, define `this.currentWorkspaceName = projectName` e retorna `true`.
   - Caso contrário (ou se `Runtime.evaluate` falhar), delega para `this.probeWorkspaceFolderPath(projectName, workspacePath)`.
4. **Fluxo `_discoverAndConnectForWorkspaceImpl(workspacePath, projectName)`** (executado com `this.isSwitchingWorkspace = true` protegido por `try/finally`):
   - Escaneia todas as portas de `this.ports` via `getJson`:
     - Acumula todos os alvos em `pages`.
     - Registra `respondingPort` priorizando a primeira porta que contenha ao menos um alvo com `t.url?.includes('workbench')` (ou `this.ports[0]` se alguma porta respondeu sem workbench).
   - Se nenhuma porta respondeu (`respondingPort === null`):
     - Chama e retorna `this.launchAndConnectWorkspace(workspacePath, projectName)`.
   - Filtra `workbenchPages`:
     ```ts
     t.type === 'page' &&
     t.webSocketDebuggerUrl &&
     !t.title?.includes('Launchpad') &&
     !t.url?.includes('workbench-jetski-agent') &&
     t.url?.includes('workbench')
     ```
   - **Passo 1 — Match rápido por título (`titleMatch`)**:
     - Procura em `workbenchPages` o primeiro alvo onde `t.title?.includes(projectName)` (case-sensitive no fast path inicial, conforme linha 445).
     - Se encontrar, chama `this.connectToPage(titleMatch, projectName)`.
   - **Passo 2 — Sonda ativa via CDP (`probeWorkbenchPages`)**:
     - Para cada `page` em `workbenchPages`:
       1. Chama `this.disconnectQuietly()`, define `this.targetUrl = page.webSocketDebuggerUrl` e chama `await this.connect()`.
       2. Avalia `document.title`. Se `liveTitle.toLowerCase().includes(projectName.toLowerCase())`, define `this.currentWorkspaceName = projectName` e retorna `true` (mantendo a conexão aberta nessa página).
       3. Se `liveTitle.toLowerCase().includes('untitled') && workspacePath`: chama `await this.probeWorkspaceFolderPath(projectName, workspacePath)`. Se retornar `true`, mantém a conexão e retorna `true`.
     - Se nenhuma página corresponder após iterar todas: chama `this.disconnectQuietly()` e retorna `false`.
   - **Passo 3 — Sonda de Caminho de Pasta (`probeWorkspaceFolderPath`)**:
     - Avalia a IIFE de 4 métodos (Seção 3.4.2). Se `value?.found && value?.value`:
       - Converte `detectedValue`, `projectName` e `workspacePath` para minúsculas.
       - Se `normalizedDetected.includes(normalizedProject) || normalizedDetected.includes(normalizedWorkspace)`, define `this.currentWorkspaceName = projectName` e retorna `true`.
     - Fallback adicional de URL (Seção 3.4.3): avalia `window.location.href`. Se `pageUrl.toLowerCase()` contiver `encodeURIComponent(workspacePath).toLowerCase()` ou `projectName.toLowerCase()`, define `this.currentWorkspaceName = projectName` e retorna `true`.
   - **Passo 4 — Lançamento de Nova Janela**:
     - Se nem o `titleMatch` nem o `probeWorkbenchPages` encontrarem a janela, chama `this.launchAndConnectWorkspace(workspacePath, projectName)`.

### 4.7. Lançamento Multiplataforma de Workspace (`launchAndConnectWorkspace`, linhas 623–758)
1. Resolve o caminho do executável via `antigravityCli = getAntigravityCliPath()` (`src/utils/pathUtils.ts`):
   - Se `process.env.ANTIGRAVITY_PATH` estiver definido: verifica com `fs.statSync(path)` se existe e se `stat.isFile()` é `true` (lançando erro caso contrário).
   - Se `darwin`: retorna `'/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity'`.
   - Se `win32`: se `process.env.LOCALAPPDATA` existir, retorna `${LOCALAPPDATA}\Programs\Antigravity\Antigravity.exe`; caso contrário, `'Antigravity.exe'`.
   - Demais plataformas (`linux`, etc.): retorna `'antigravity'`.
2. Obtém uma porta livre chamando `cdpPort = await this.findAvailableCdpPort()`.
   - Se `cdpPort === null`, lança erro: `"No available CDP ports. All candidate ports are in use: " + this.ports.join(', ')`.
3. Executa o binário via `runCommand(antigravityCli, ['--remote-debugging-port=' + cdpPort, '--new-window', workspacePath])`:
   - **Regra de Segurança (testada em `cdpService.security.test.ts`)**: `runCommand` DEVE executar o processo filho diretamente sem shell (`shell: false` no Node / `exec.Command` no Go), com `stdio: 'ignore'` e `detached: true`, aguardando o evento `close` com código `0`.
   - **Fallback macOS (`darwin`)**: se `runCommand` falhar e a plataforma for `darwin`, executa `runCommand('open', ['-a', 'Antigravity', '--args', '--remote-debugging-port=' + cdpPort, workspacePath])`. Em outras plataformas, propaga o erro imediatamente.
4. Coleta os IDs de páginas já existentes antes/logo após o comando (`knownPageIds: Set<string>`) consultando `/json/list` em todas as portas de `this.ports`.
5. Inicia loop de polling enquanto `Date.now() - startTime < 30000` (`maxWaitMs = 30000`), aguardando `1000ms` (`pollIntervalMs = 1000`) no início de cada iteração:
   - Consulta `/json/list` em todas as portas. Se `pages.length === 0`, continua para a próxima iteração.
   - Filtra `workbenchPages` (mesmo filtro de workbench).
   - Tenta match case-insensitive no título: `workbenchPages.find(t => t.title?.toLowerCase().includes(projectName.toLowerCase()))`. Se encontrar, retorna `this.connectToPage(titleMatch, projectName)`.
   - Tenta sonda ativa: `await this.probeWorkbenchPages(workbenchPages, projectName, workspacePath)`. Se retornar `true`, retorna `true`.
   - **Fallback de Página Untitled após 10s**: se `Date.now() - startTime > 10000`, filtra `newUntitledPages = workbenchPages.filter(t => !knownPageIds.has(t.id) && (t.title?.includes('Untitled') || t.title === ''))`. Se `newUntitledPages.length === 1`, conecta nela via `this.connectToPage(newUntitledPages[0], projectName)`.
6. Se o loop de 30s terminar sem encontrar a janela, lança erro:
   `"Workbench page for workspace \"" + projectName + "\" not found within 30 seconds"`.

### 4.8. Seleção de Contextos V8 (`waitForCascadePanelReady` e `getPrimaryContextId`, linhas 919–943)
1. **`waitForCascadePanelReady(timeoutMs = 10000, pollIntervalMs = 500)`**:
   - Enquanto `Date.now() - start < timeoutMs`:
     - Procura em `this.contexts` um contexto onde `c.url && c.url.includes('cascade-panel')`.
     - Se encontrar, retorna `true`.
     - Caso contrário, dorme `pollIntervalMs` (500ms).
   - Retorna `false` ao atingir o timeout.
2. **`getPrimaryContextId()`**:
   - Prioridade 1: primeiro contexto em `this.contexts` onde `c.url && c.url.includes('cascade-panel')`.
   - Prioridade 2: primeiro contexto em `this.contexts` onde `c.name && c.name.includes('Extension')`.
   - Prioridade 3: `this.contexts[0].id` se `this.contexts.length > 0`, ou `null` se vazio.

### 4.9. Reconexão Automática (`tryReconnect`, linhas 862–900)
1. Se `this.isReconnecting` já for `true`, retorna imediatamente (evita loops de reconexão concorrentes).
2. Define `this.isReconnecting = true` e `this.reconnectAttemptCount = 0`.
3. Enquanto `this.reconnectAttemptCount < this.maxReconnectAttempts`:
   - Incrementa `this.reconnectAttemptCount++`.
   - Aguarda `this.reconnectDelayMs` (padrão `2000ms`).
   - Limpa `this.contexts = []`.
   - Se `this.currentWorkspacePath` estiver preenchido: chama `await this.discoverAndConnectForWorkspace(this.currentWorkspacePath)`.
   - Caso contrário: chama `await this.discoverTarget()` seguido de `await this.connect()`.
   - Se a conexão suceder sem erro:
     - Reseta `this.reconnectAttemptCount = 0` e `this.isReconnecting = false`.
     - Emite `this.emit('reconnected')` e retorna.
   - Se lançar erro: captura, loga o erro e avança para a próxima tentativa.
4. Se esgotar todas as `this.maxReconnectAttempts` tentativas:
   - Define `this.isReconnecting = false`.
   - Cria `finalError = new Error("CDP reconnection failed " + this.maxReconnectAttempts + " times. Manual restart required.")`.
   - Emite `this.emit('reconnectFailed', finalError)`.

### 4.10. Regras do `CdpConnectionPool` (`src/services/cdpConnectionPool.ts`)
1. **`extractProjectName(workspacePath)`**:
   - Retorna `extractProjectNameFromPath(workspacePath) || workspacePath`.
2. **`getOrConnect(workspacePath)`**:
   - Calcula `projectName = this.extractProjectName(workspacePath)`.
   - Verifica se já existe entrada `existing = this.connections.get(projectName)`:
     - Se `existing.isConnected()` for `true`:
       - Executa `await existing.discoverAndConnectForWorkspace(workspacePath)` para revalidar que a janela ainda pertence ao workspace.
       - Se suceder, retorna `existing`.
       - Se falhar (lançar exceção): chama `existing.disconnect().catch(() => {})` e remove `projectName` de `this.connections` (correção de bug verificada em `cdpConnectionPool.bugfix.test.ts`).
     - Se `existing.isConnected()` for `false`:
       - Apenas remove a entrada obsoleta `this.connections.delete(projectName)` **sem** chamar `existing.disconnect()` (verificado em `cdpConnectionPool.bugfix.test.ts`).
   - Verifica se há uma conexão em andamento em `this.connectingPromises.get(projectName)`:
     - Se existir, aguarda e retorna a mesma Promise/chamada in-flight.
   - Caso contrário, inicia `createAndConnect(workspacePath, projectName)`, registra em `connectingPromises` e garante sua remoção no `finally`.
3. **`createAndConnect(workspacePath, projectName)`**:
   - Se houver conexão antiga residual em `this.connections.get(projectName)`, desconecta e remove.
   - Instancia `cdp = new CdpService(this.cdpOptions)`.
   - Registra 3 listeners de ciclo de vida na instância `cdp`:
     - `'disconnected'` $\rightarrow$ emite `'workspace:disconnected'` com `projectName` (não remove do mapa ainda pois `CdpService` tentará reconectar internamente).
     - `'reconnected'` $\rightarrow$ emite `'workspace:reconnected'` com `projectName`.
     - `'reconnectFailed'` $\rightarrow$ emite `'workspace:reconnectFailed'` com `projectName` e chama `this.disconnectWorkspace(projectName)` (que desconecta o cliente e para todos os 4 detectores).
   - Chama `await cdp.discoverAndConnectForWorkspace(workspacePath)`.
   - Armazena `this.connections.set(projectName, cdp)` e retorna `cdp`.
4. **`disconnectWorkspace(projectName)`**:
   - Se `this.connections.get(projectName)` existir: chama `cdp.disconnect()` e remove de `this.connections`.
   - Para cada um dos 4 mapas de detectores (`approvalDetectors`, `errorPopupDetectors`, `planningDetectors`, `userMessageDetectors`): se houver detector registrado para `projectName`, chama `detector.stop()` e remove do mapa.
5. **`register<Type>Detector(projectName, detector)`**:
   - Se já existir um detector ativo (`existing && existing.isActive()`) no mapa correspondente para `projectName`, chama `existing.stop()` antes de substituí-lo pelo novo `detector`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Proteção de Estado Interno em `Client` (`sync.RWMutex`)**:
   - Em Go, o loop de leitura do WebSocket roda em uma goroutine dedicada (`readLoop`), enquanto o heartbeat roda em outra goroutine (`heartbeatLoop`) e chamadas `Call()` vêm de múltiplas goroutines (detectores em background, comandos do Telegram, `ResponseMonitor`).
   - O mapa `pendingCalls`, o contador `idCounter`, a fatia `contexts` e as flags `isConnectedFlag`, `isSwitchingWorkspace` e `isReconnecting` devem ser protegidos por mutex (`c.mu`).
   - Escritas no WebSocket (`ws.WriteMessage` e `ws.Ping`) devem ser serializadas por um mutex de escrita (`writeMu sync.Mutex`), pois `gorilla/websocket` / `nhooyr.io/websocket` não permitem escritas concorrentes sem sincronização.
2. **Desconexão Silenciosa durante Troca/Sonda de Workspace (`disconnectQuietly`)**:
   - Quando `probeWorkbenchPages` ou `connectToPage` fecha um WebSocket anterior para abrir outro, deve remover/desativar os handlers de `close` daquela conexão antes de fechá-la (`disconnectQuietly`), rejeitando `pendingCalls` com `Error("Disconnected for workspace switch")` sem disparar o evento `disconnected` nem `tryReconnect()`.
3. **Singleflight por Workspace no `ConnectionPool` (`connectingCalls`)**:
   - Quando duas goroutines solicitam `GetOrConnect("/path/to/ProjectX")` simultaneamente, a primeira cria uma struct `inflightConnect` com um canal `done chan struct{}` em `p.connectingCalls["ProjectX"]` e libera o lock do pool enquanto executa `createAndConnect`. A segunda goroutine encontra `inflightConnect` no mapa, libera o lock e faz `<-inflight.done`, recebendo exatamente a mesma instância `PoolClient` e `error`.
4. **Prevenção de Vazamento de Goroutines e Timers**:
   - `clearPendingCalls(err)` interrompe todos os `*time.Timer` de chamadas em andamento e envia `err` nos respectivos canais `resultCh` antes de limpar o mapa.
   - `stopHeartbeat()` fecha `heartbeatStopCh` garantindo o encerramento imediato do `time.Ticker` de 30s.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Cada caso de teste abaixo mapeia 1:1 um bloco `it(...)` dos 6 arquivos de teste originais em `tests/services/` e deve ser implementado em Go com 100% de cobertura comportamental.

### 6.1. `internal/cdp/client_test.go` (Mapeado de [`tests/services/cdpService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `CdpService - Target Detection & Connection` $\rightarrow$ `scans ports and discovers the correct target (workbench) to obtain the WebSocket URL` | `TestClient_DiscoverTarget_SkipsLaunchpadAndSelectsWorkbench(t *testing.T)` | Dado um servidor HTTP mock em `/json/list` retornando 2 páginas (`Launchpad` e `Antigravity Workspace` com `workbench.html`), `DiscoverTarget()` deve ignorar `Launchpad` e retornar exatamente a `webSocketDebuggerUrl` do workbench. |
| `CdpService - Target Detection & Connection` $\rightarrow$ `establishes a WebSocket connection and recognizes the context (cascade-panel)` | `TestClient_Connect_RecognizesCascadePanelContext(t *testing.T)` | Ao chamar `Connect()`, o cliente envia `Runtime.enable`, recebe dois eventos `Runtime.executionContextCreated` (`id: 1` top `workbench.html` e `id: 2` `cascade-panel.html`), `IsConnected()` retorna `true`, `GetContexts()` possui tamanho $\ge 1$ e `GetPrimaryContextID()` retorna `2`. |
| `CdpService - Target Detection & Connection` $\rightarrow$ `triggers the auto-reconnect listener when disconnected` | `TestClient_DisconnectEvent_EmittedOnServerClose(t *testing.T)` | Com conexão ativa (`IsConnected() == true`), quando o servidor WebSocket encerra a conexão abruptamente, o evento `'disconnected'` é emitido e `IsConnected()` passa a retornar `false`. |

### 6.2. `internal/cdp/client_workspace_test.go` (Mapeado de [`tests/services/cdpService.workspace.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.workspace.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `launchAndConnectWorkspace (Mac)` $\rightarrow$ `should launch Antigravity using the Mac application path` | `TestClient_LaunchWorkspace_DarwinPrimaryCLI(t *testing.T)` | Em `darwin`, quando nenhuma porta responde inicialmente, executa `/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity` com args `["--remote-debugging-port=9999", "--new-window", "/Users/test/Documents/MyProject"]` e conecta na página detectada no poll. |
| `launchAndConnectWorkspace (Mac)` $\rightarrow$ `should fallback to open -a Antigravity if the CLI launch fails on Mac` | `TestClient_LaunchWorkspace_DarwinFallbackToOpenApp(t *testing.T)` | Em `darwin`, se a primeira chamada a `runCommand` falhar, executa uma segunda chamada com comando `"open"` e args `["-a", "Antigravity", "--args", "--remote-debugging-port=9999", "/Users/test/Documents/MyProject"]`. |
| `launchAndConnectWorkspace (Windows)` $\rightarrow$ `should launch Antigravity using LOCALAPPDATA environment variable` | `TestClient_LaunchWorkspace_WindowsWithLocalAppData(t *testing.T)` | Em `windows` com `LOCALAPPDATA=C:\Users\TestUser\AppData\Local`, executa `C:\Users\TestUser\AppData\Local\Programs\Antigravity\Antigravity.exe` com `["--remote-debugging-port=9999", "--new-window", "C:\\Source\\MyProject"]`. |
| `launchAndConnectWorkspace (Windows)` $\rightarrow$ `should fallback to Antigravity.exe if LOCALAPPDATA is missing on Windows` | `TestClient_LaunchWorkspace_WindowsWithoutLocalAppData(t *testing.T)` | Em `windows` sem `LOCALAPPDATA`, executa `"Antigravity.exe"` com `["--remote-debugging-port=9999", "--new-window", "C:\\Source\\MyProject"]`. |
| `launchAndConnectWorkspace (Linux / Unknown)` $\rightarrow$ `should default to antigravity command if ANTIGRAVITY_PATH is not set` | `TestClient_LaunchWorkspace_LinuxDefaultCommand(t *testing.T)` | Em `linux` sem `ANTIGRAVITY_PATH`, executa `"antigravity"` com `["--remote-debugging-port=9999", "--new-window", "/home/user/MyProject"]`. |
| `launchAndConnectWorkspace (Linux / Unknown)` $\rightarrow$ `should use ANTIGRAVITY_PATH if it is set` | `TestClient_LaunchWorkspace_LinuxCustomEnvPath(t *testing.T)` | Em `linux` com `ANTIGRAVITY_PATH=/opt/custom/antigravity.AppImage`, executa `/opt/custom/antigravity.AppImage` com `["--remote-debugging-port=9999", "--new-window", "/home/user/MyProject"]`. |
| `Project Name Extraction` $\rightarrow$ `should extract the project name from a Windows path with backslashes` | `TestClient_ProjectNameExtraction_WindowsBackslashes(t *testing.T)` | Para `"C:\\Source\\MyProject"`, `GetCurrentWorkspaceName()` retorna `"MyProject"` após conectar. |
| `Project Name Extraction` $\rightarrow$ `should extract the project name from a Mac/Linux path with forward slashes` | `TestClient_ProjectNameExtraction_PosixSlashes(t *testing.T)` | Para `"/Users/test/Documents/my-cool-project"`, `GetCurrentWorkspaceName()` retorna `"my-cool-project"`. |
| `Project Name Extraction` $\rightarrow$ `should extract the project name from a path with trailing slashes` | `TestClient_ProjectNameExtraction_TrailingSlashes(t *testing.T)` | Para `"/home/user/trailing-slash-proj/"`, `GetCurrentWorkspaceName()` retorna `"trailing-slash-proj"`. |

### 6.3. `internal/cdp/client_error_test.go` (Mapeado de [`tests/services/cdpService.errorHandling.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.errorHandling.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `pendingCalls cleanup on disconnection` $\rightarrow$ `rejects all unresolved pendingCalls when WebSocket disconnects` | `TestClient_ClearPendingCalls_RejectsAllWithDisconnectError(t *testing.T)` | Chamadas pendentes em `pendingCalls` devem receber imediatamente o erro passado a `clearPendingCalls` quando o WebSocket desconecta. |
| `pendingCalls cleanup on disconnection` $\rightarrow$ `pendingCalls is empty after disconnection` | `TestClient_ClearPendingCalls_EmptiesMap(t *testing.T)` | Após adicionar 2 entradas em `pendingCalls` e invocar `clearPendingCalls`, `len(pendingCalls)` deve ser `0`. |
| `auto-reconnection feature` $\rightarrow$ `reconnects to the same workspace when workspace path is known` | `TestClient_TryReconnect_UsesCurrentWorkspacePath(t *testing.T)` | Quando `currentWorkspacePath == "/tmp/my-workspace"`, `tryReconnect()` chama `DiscoverAndConnectForWorkspace("/tmp/my-workspace")`, não chama `DiscoverTarget()` genérico e emite `'reconnected'`. |
| `auto-reconnection feature` $\rightarrow$ `does not reconnect when maxReconnectAttempts is 0` | `TestClient_AutoReconnect_DisabledWhenMaxAttemptsZero(t *testing.T)` | Com `MaxReconnectAttempts = 0`, a queda da conexão emite `'disconnected'` mas nunca invoca `tryReconnect()`. |
| `auto-reconnection feature` $\rightarrow$ `fires a reconnectFailed event after exhausting maxReconnectAttempts` | `TestClient_TryReconnect_EmitsReconnectFailedOnExhaustion(t *testing.T)` | Com `MaxReconnectAttempts = 2` e falha persistente na descoberta, emite `'reconnectFailed'` contendo erro não-nulo com mensagem definida. |
| `auto-reconnection feature` $\rightarrow$ `does not exceed maxReconnectAttempts for reconnection attempts` | `TestClient_TryReconnect_DoesNotExceedMaxAttempts(t *testing.T)` | Com `MaxReconnectAttempts = 2`, o contador de tentativas de reconexão é $\le 2$ até emitir `'reconnectFailed'`. |
| `CdpServiceOptions extension` $\rightarrow$ `accepts the maxReconnectAttempts option` / `accepts the reconnectDelayMs option` / `defaults maxReconnectAttempts to 3` / `defaults reconnectDelayMs to 2000` | `TestClient_Options_DefaultsAndOverrides(t *testing.T)` | Verifica que `NewClient(ClientOptions{})` configura `maxReconnectAttempts = 3` e `reconnectDelay = 2000ms`, e aceita sobrescritas explícitas (`5` e `3000ms`). |

### 6.4. `internal/cdp/client_security_test.go` (Mapeado de [`tests/services/cdpService.security.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.security.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `CdpService security hardening` $\rightarrow$ `does not use child_process.exec` | `TestClient_Security_DoesNotInvokeShell(t *testing.T)` | Inspeciona `internal/cdp/client.go` (e testa `runCommand`) garantindo que comandos externos são executados diretamente via `exec.Command(command, args...)` sem invocar `sh -c`, `bash -c` ou `cmd /c`. |

### 6.5. `internal/cdp/pool_test.go` (Mapeado de [`tests/services/cdpConnectionPool.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpConnectionPool.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `extractProjectName()` (6 casos) | `TestConnectionPool_ExtractProjectName(t *testing.T)` | Table-driven test validando: `"/home/user/Code/MyProject"` $\rightarrow$ `"MyProject"`, `"/home/user/Code/MyProject/"` $\rightarrow$ `"MyProject"`, `"MyProject"` $\rightarrow$ `"MyProject"`, `"D:\\Code\\MyProject"` $\rightarrow$ `"MyProject"`, `"D:\\categorizer"` $\rightarrow$ `"categorizer"`, `"C:\\Users\\test/Code/MyProject"` $\rightarrow$ `"MyProject"`. |
| `getOrConnect()` $\rightarrow$ `creates a new CdpService and connects` | `TestConnectionPool_GetOrConnect_CreatesAndConnects(t *testing.T)` | Cria instância via factory, chama `DiscoverAndConnectForWorkspace("/path/to/ProjectA")` e inclui `"ProjectA"` em `GetActiveWorkspaceNames()`. |
| `getOrConnect()` $\rightarrow$ `returns the cached instance on the second call to the same workspace` | `TestConnectionPool_GetOrConnect_ReusesAndRevalidatesCachedInstance(t *testing.T)` | Duas chamadas sequenciais para `"/path/to/ProjectA"` retornam a mesma instância `cdp1 == cdp2` e chamam `DiscoverAndConnectForWorkspace` 2 vezes (conexão inicial + revalidação). |
| `getOrConnect()` $\rightarrow$ `creates separate instances for different workspaces` | `TestConnectionPool_GetOrConnect_SeparateInstancesPerWorkspace(t *testing.T)` | Chamadas para `"/path/to/ProjectA"` e `"/path/to/ProjectB"` retornam instâncias distintas e ambas constam em `GetActiveWorkspaceNames()`. |
| `getOrConnect()` $\rightarrow$ `prevents concurrent connections with a Promise lock` | `TestConnectionPool_GetOrConnect_PreventsConcurrentConnections(t *testing.T)` | Duas goroutines chamando `GetOrConnect("/path/to/ProjectX")` simultaneamente enquanto a conexão está bloqueada instanciam apenas 1 cliente na factory e ambas recebem a mesma referência ao concluir. |
| `getConnected()` $\rightarrow$ `returns the CdpService when connected` / `returns null when not connected` | `TestConnectionPool_GetConnected(t *testing.T)` | Retorna a instância conectada para `"ProjectA"` e `nil` para `"NonExistent"`. |
| `disconnectWorkspace()` $\rightarrow$ `disconnects the specified workspace and removes it from the map` / `also stops the approval detector` | `TestConnectionPool_DisconnectWorkspace_StopsClientAndDetectors(t *testing.T)` | Chama `Disconnect()` no cliente, remove do pool (`GetConnected("ProjectA") == nil`), chama `Stop()` no detector registrado e remove o detector (`GetApprovalDetector("ProjectA") == nil`). |
| `disconnectAll()` $\rightarrow$ `disconnects all workspace connections` | `TestConnectionPool_DisconnectAll(t *testing.T)` | Desconecta `"A"` e `"B"` e deixa `GetActiveWorkspaceNames()` com tamanho `0`. |
| `ApprovalDetector management` $\rightarrow$ `can register... and retrieve...` / `stops the old detector when replacing an existing one` | `TestConnectionPool_DetectorRegistrationAndReplacement(t *testing.T)` | Registrar um segundo detector para o mesmo `projectName` chama `Stop()` no detector anterior (se `IsActive() == true`) e substitui pelo novo. |
| `getActiveWorkspaceNames()` $\rightarrow$ `returns only connected workspace names` | `TestConnectionPool_GetActiveWorkspaceNames_FiltersDisconnected(t *testing.T)` | Se `"A"` retorna `IsConnected() == true` e `"B"` retorna `IsConnected() == false`, `GetActiveWorkspaceNames()` contém `"A"` e não contém `"B"`. |

### 6.6. `internal/cdp/pool_bugfix_test.go` (Mapeado de [`tests/services/cdpConnectionPool.bugfix.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpConnectionPool.bugfix.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `disconnect stale connection on re-validation failure` $\rightarrow$ `disconnects and removes the stale entry when re-validation throws` | `TestConnectionPool_Bugfix_DisconnectsStaleOnRevalidationError(t *testing.T)` | Se a 2ª chamada a `GetOrConnect` falhar na revalidação (`DiscoverAndConnectForWorkspace` retorna erro `"tab closed"`), o pool chama `Disconnect()` na instância antiga, remove-a e cria uma nova conexão `freshCdp`. |
| `disconnect stale connection on re-validation failure` $\rightarrow$ `cleans up disconnected entries without calling disconnect` | `TestConnectionPool_Bugfix_CleansUpDisconnectedEntryWithoutCallingDisconnect(t *testing.T)` | Se a entrada existente no mapa já retornar `IsConnected() == false` no início de `GetOrConnect`, o pool remove a entrada e cria `freshCdp` **sem** invocar `Disconnect()` na entrada já desconectada. |
| `reconnectFailed uses disconnectWorkspace` $\rightarrow$ `removes connection and stops detectors on reconnectFailed` | `TestConnectionPool_Bugfix_ReconnectFailedCleansUpWorkspaceAndDetectors(t *testing.T)` | Quando o cliente emite `'reconnectFailed'`, o pool invoca `DisconnectWorkspace(projectName)`, chamando `Disconnect()` no cliente, `Stop()` nos detectores e removendo ambos dos mapas. |
| `reconnectFailed uses disconnectWorkspace` $\rightarrow$ `registers disconnected event handler` | `TestConnectionPool_Bugfix_RegistersLifecycleEventHandlers(t *testing.T)` | Verifica que `createAndConnect` registra handlers para `'disconnected'`, `'reconnected'` e `'reconnectFailed'`. |
