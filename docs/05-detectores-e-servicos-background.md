# 05 — Detectores em Background e Serviços Auxiliares

## 1. Visão Geral dos Detectores e Serviços

Para cada workspace conectado no `CdpConnectionPool`, o `CdpBridgeManager` (`src/services/cdpBridgeManager.ts`) mantém **4 detectores assíncronos em background** que inspecionam continuamente o DOM da janela do Antigravity via `Runtime.evaluate`:
1. **`ApprovalDetector`** (`src/services/approvalDetector.ts` — 428 linhas)
2. **`PlanningDetector`** (`src/services/planningDetector.ts` — 353 linhas)
3. **`ErrorPopupDetector`** (`src/services/errorPopupDetector.ts` — 303 linhas)
4. **`UserMessageDetector`** (`src/services/userMessageDetector.ts` — 249 linhas)

Além dos 4 detectores, esta camada inclui os serviços `QuotaService`, `PromptDispatcher`, `AutoAcceptService`, `ModeService`, `ModelService`, `WorkspaceService`, `ScheduleService`, `ScreenshotService`, `TitleGeneratorService`, `AntigravityLauncher`, `UpdateCheckService` e o módulo legado `ProcessManager`.

---

## 2. Roteamento de Canais e `CdpBridgeManager` (`src/services/cdpBridgeManager.ts`)

### 2.1. Estado `CdpBridge` e Roteamento por Sessão/Workspace
Como múltiplos tópicos do Telegram podem interagir com o mesmo workspace (em sessões de chat distintas), o `CdpBridge` mantém dois mapas de roteamento:
- `approvalChannelBySession: Map<string, TelegramChannel>` — indexado por `${projectName}::${sessionTitle.trim().toLowerCase()}`.
- `approvalChannelByWorkspace: Map<string, TelegramChannel>` — indexado por `projectName` (fallback).

Quando qualquer detector dispara um evento (`onApprovalRequired`, `onPlanningRequired`, `onErrorPopup`), o bridge executa `getCurrentChatTitle(cdp)` (lendo `.antigravity-agent-side-panel div[class*="border-b"] div[class*="text-ellipsis"]`) e resolve o canal de destino via `resolveApprovalChannelForCurrentChat(bridge, projectName, currentChatTitle)`, garantindo que a notificação chegue ao tópico exato daquela sessão.

### 2.2. Prefixos Reais de `callback_data` no Bridge
- **Aprovação**:
  - `approve_action:<projectName>:<channelId>`
  - `always_allow_action:<projectName>:<channelId>`
  - `deny_action:<projectName>:<channelId>`
- **Planejamento (legado/direto)**:
  - `planning_open_action:<projectName>:<channelId>`
  - `planning_proceed_action:<projectName>:<channelId>`
- **Popup de Erro**:
  - `error_popup_dismiss_action:<projectName>:<channelId>`
  - `error_popup_copy_debug_action:<projectName>:<channelId>`
  - `error_popup_retry_action:<projectName>:<channelId>`

---

## 3. Os 4 Detectores em Background

### 3.1. `ApprovalDetector` (`src/services/approvalDetector.ts`)
- **Intervalo de Polling**: `1500ms` por padrão na classe (`2000ms` quando instanciado por `ensureApprovalDetector` em `cdpBridgeManager.ts`).
- **Detecção (`DETECT_APPROVAL_SCRIPT`)**:
  - Procura dentro de `.antigravity-agent-side-panel` (ou `document`) por botões visíveis cujos textos correspondam a:
    - `APPROVAL_PATTERNS`: `['allow', 'approve', 'accept', 'yes', 'run', 'confirm', 'save', 'apply', 'create', 'delete', 'modify']` (excluindo `'allow chat'`, `'always allow'`, `'allow this conversation'`).
    - `ALWAYS_ALLOW_PATTERNS`: `['allow chat', 'always allow', 'allow this conversation', 'allow all']`.
    - `DENY_PATTERNS`: `['deny', 'reject', 'cancel', 'no', 'skip', 'abort', 'decline']`.
  - Um diálogo é válido quando possui simultaneamente um botão de aprovação e um botão de negação no mesmo container ancestral.
  - Extrai `approveText`, `alwaysAllowText`, `denyText` e `description` (texto de contexto do comando/arquivo no card).
- **Deduplicação e Resolução**:
  - Chave `lastDetectedKey = "${info.approveText}::${info.denyText}::${info.description}"`.
  - Quando o diálogo desaparece do DOM (`info === null` após estar ativo), dispara `onResolved()`, que remove o teclado inline da mensagem do Telegram (`editMessageReplyMarkup({ reply_markup: undefined })`).
- **Fluxo de Auto-Accept (`ensureApprovalDetector`)**:
  - Quando `bridge.autoAccept.isEnabled()` é `true`, o bridge tenta **primeiro** `await detector.alwaysAllowButton()` (que expande o dropdown de chevron ao lado do botão Allow via `EXPAND_ALLOW_OPTIONS_SCRIPT`, aguarda `300ms` e clica em `Allow This Conversation` / `Always Allow`) e, caso retorne `false`, faz fallback para `await detector.approveButton()`.
  - Se a aprovação automática tiver sucesso, envia `✅ <b>Auto-approved</b>` e encerra; se falhar, envia `⚠️ <b>Auto-approve failed</b>` e envia normalmente os 3 botões manuais (`✅ Allow`, `✅ Allow Chat`, `❌ Deny`).

### 3.2. `PlanningDetector` (`src/services/planningDetector.ts`)
- **Intervalo de Polling**: `2000ms`, com **`COOLDOWN_MS = 5000` (5s)** após clicar em `Open` ou `Proceed` para evitar re-detecção imediata enquanto o DOM transiciona.
- **Detecção (`DETECT_PLANNING_SCRIPT`)**:
  - Localiza cards no `.antigravity-agent-side-panel` que contenham simultaneamente botões visíveis `Open` e `Proceed` (ou `Approve`/`Start`), extraindo `openText`, `proceedText`, `planTitle`, `planSummary` e `description`.
- **Extração do Conteúdo do Plano (`extractPlanContent` / `EXTRACT_PLAN_CONTENT_SCRIPT`)**:
  - Contido diretamente dentro de `src/services/planningDetector.ts` (linhas 95–218).
  - Após clicar no botão `Open` (`clickOpenButton()`), aguarda `500ms` e faz até 3 tentativas (intervalo `500ms`) para extrair o documento renderizado no painel de edição/visualização (convertendo cabeçalhos `h1`-`h6`, listas `ul`/`ol`, blocos `<pre><code>` e parágrafos em Markdown limpo), que é então paginado por `paginatePlanContent` (`src/ui/planUi.ts`).

### 3.3. `ErrorPopupDetector` (`src/services/errorPopupDetector.ts`)
- **Intervalo de Polling**: `3000ms`, com **`COOLDOWN_MS = 10000` (10s)** após ações de `Dismiss` ou `Retry`.
- **Detecção (`DETECT_ERROR_POPUP_SCRIPT`)**:
  - Busca diálogos `[role="dialog"]`, `[role="alertdialog"]`, `.monaco-dialog-box` ou cards de erro contendo títulos como `'Agent terminated due to error'`, `'Error'`, `'Failed'` acompanhados de botões `Dismiss`/`Close`, `Copy debug info` e/ou `Retry`/`Try again`.
- **Ações Suportadas**:
  - `clickDismissButton()`: clica em `Dismiss` / `Close` / `OK` e inicia cooldown de 10s.
  - `clickCopyDebugInfoButton()` + `readClipboard()`: clica no botão `Copy debug info`, aguarda `300ms` e avalia `navigator.clipboard.readText()` (`awaitPromise: true`) para enviar o conteúdo de debug (truncado em `3800` chars dentro de `<pre>`) diretamente no Telegram.
  - `clickRetryButton()`: clica em `Retry` / `Try again` e inicia cooldown de 10s.

### 3.4. `UserMessageDetector` (`src/services/userMessageDetector.ts`)
- **Propósito**: Detectar mensagens digitadas diretamente pelo usuário no teclado do PC dentro da interface do Antigravity, espelhando-as no tópico correspondente do Telegram e iniciando o monitoramento passivo da resposta (`ResponseMonitor.startPassive()`).
- **Intervalo de Polling**: `2000ms`.
- **Seletor DOM (`DETECT_USER_MESSAGE_SCRIPT`)**:
  - **Estratégia A (primária)**: busca todos os elementos `[class*="bg-gray-500/15"][class*="select-text"] .whitespace-pre-wrap` dentro de `.antigravity-agent-side-panel` e retorna o texto do último elemento (`textEls[textEls.length - 1]`).
  - **Estratégia B (fallback)**: busca containers folhas `[class*="bg-gray-500/15"][class*="rounded-lg"][class*="select-text"]` que não contenham outra bolha aninhada e extrai `.whitespace-pre-wrap` ou `[style*="word-break"]`.
- **Prevenção de Eco e Duplicatas**:
  - `normalizeForHash(text)`: aplica `.trim().replace(/\s+/g, ' ').slice(0, 200)`.
  - `computeEchoHash(text)`: calcula `SHA-256` do texto normalizado e retorna os primeiros **16 caracteres hexadecimais**.
  - `isPriming`: no primeiro poll após `start()`, apenas registra a mensagem já existente no DOM em `lastDetectedHash` e `seenHashes` **sem disparar** `onUserMessage`.
  - `addEchoHash(text)`: chamado pelo bot sempre que uma mensagem de texto ou voz vinda do Telegram é enviada ao Antigravity; adiciona o hash em `echoHashes` com limpeza automática após **`60000ms` (60s)**.
  - `seenHashes`: conjunto FIFO limitado a `MAX_SEEN_HASHES = 50` entradas para evitar re-emissão caso mensagens antigas reapareçam.

---

## 4. `QuotaService` (`src/services/quotaService.ts`)

### 4.1. Implementação Original em TypeScript (`src/services/quotaService.ts`)
No código TypeScript original (175 linhas), a descoberta do servidor local de linguagem do Antigravity executa:
1. **`getUnixProcessInfo()`**:
   - Executa `pgrep -fl language_server`.
   - Procura a linha que contém `--csrf_token`, extraindo o `pid` e o token via regex `/--csrf_token[=\s]+([a-zA-Z0-9\-]+)/`.
2. **`getListeningPorts(pid)`**:
   - Executa `lsof -nP -a -iTCP -sTCP:LISTEN -p ${pid}` e extrai todas as portas TCP em estado `(LISTEN)`.
3. **`requestApi(port, csrfToken)`**:
   - Faz uma requisição `POST https://127.0.0.1:${port}/exa.language_server_pb.LanguageServerService/GetUserStatus` (`rejectUnauthorized: false`, `timeout: 2000ms`) com headers:
     - `Content-Type: application/json`
     - `Connect-Protocol-Version: 1`
     - `X-Codeium-Csrf-Token: <csrfToken>`
   - Body JSON: `{"metadata":{"ideName":"antigravity","extensionName":"antigravity","locale":"en"}}`.
   - Extrai `parsed?.userStatus?.cascadeModelConfigData?.clientModelConfigs` mapeando `{ label, model, quotaInfo: { remainingFraction, resetTime } }`.
4. **Cache e Retry**:
   - Faz cache de `cachedPid`, `cachedCsrfToken` e `cachedPort`. Se a chamada na `cachedPort` falhar, invalida `cachedPort = null` e faz 1 retry (`retryCount < 1`) reescaneando as portas do PID.

> [!TIP]
> **Evolução Intencional na Arquitetura Go (`internal/platform`)**: Enquanto o `quotaService.ts` original suporta apenas `pgrep` e `lsof` (macOS/Unix), o port para Go (conforme ADR-001) isola a descoberta de processo/portas em `internal/platform/proc_unix.go` (adicionando fallback para `/proc` e `ss -tlnp` no Linux) e `internal/platform/proc_windows.go` (usando `wmic` / PowerShell `Get-CimInstance Win32_Process` e `netstat -ano` no Windows).

---

## 5. Demais Serviços de Background e Inicialização

### 5.1. `PromptDispatcher` (`src/services/promptDispatcher.ts` — 91 linhas)
- Serializa o envio de prompts através de dois mapas de Promises:
  - `workspaceLocks: Map<string, Promise<void>>` (chave `ws:<workspaceName>`, primário).
  - `channelLocks: Map<string, Promise<void>>` (chave `<chatId>:<threadId>` ou `<chatId>`, secundário).
- Garante que dois tópicos do Telegram vinculados ao mesmo workspace nunca executem `sendPromptToAntigravity` simultaneamente.

### 5.2. `AntigravityLauncher` (`src/services/antigravityLauncher.ts` — 60 linhas)
- Exporta `ensureAntigravityRunning()`, chamada no início de `startBot()`.
- Verifica em paralelo (`Promise.all`) todas as portas de `CDP_PORTS` fazendo `GET http://127.0.0.1:<port>/json/list` com timeout de `2000ms`.
- Se nenhuma porta responder com um array JSON válido, imprime um banner de aviso delimitado por 70 sinais de `=` com instruções para iniciar o Antigravity com `--remote-debugging-port=9222` ou executar `antigravity-telegram-remote open`.

### 5.3. `UpdateCheckService` (`src/services/updateCheckService.ts` — 118 linhas)
- Chamado de forma não-bloqueante em `src/bin/commands/start.ts` (`checkForUpdates(version)`).
- Lê `~/.antigravity-telegram-remote/update-check.json` (`{ lastCheck: number, latestVersion: string }`). Se `Date.now() - lastCheck < 86400000` (24 horas), usa o valor em cache sem fazer requisição de rede.
- Caso contrário, faz `GET https://registry.npmjs.org/antigravity-telegram-remote/latest` (timeout `5000ms`), salva o resultado em `update-check.json` e, se `isNewerVersion(currentVersion, latestVersion)` for verdadeiro, imprime um box ANSI notificando a atualização disponível.

### 5.4. `WorkspaceService` (`src/services/workspaceService.ts` — 80 linhas) e `sanitize.ts` (34 linhas)
- `WorkspaceService` gerencia subdiretórios dentro de `WORKSPACE_BASE_DIR`:
  - `ensureBaseDir()`: cria `baseDir` recursivamente se não existir.
  - `scanWorkspaces()`: lê `fs.readdirSync(baseDir, { withFileTypes: true })`, filtra entradas onde `entry.isDirectory() && !entry.name.startsWith('.')` e ordena alfabeticamente (`.sort()`).
  - `validatePath(relativePath)` / `getWorkspacePath(name)` / `exists(name)`: delegam a validação de segurança para `resolveSafePath(inputPath, baseDir)` (`src/middleware/sanitize.ts`), que bloqueia tanto `..` (`path.relative`) quanto escapes por symlink (`fs.realpathSync`).

### 5.5. `ScheduleService` (`src/services/scheduleService.ts` — 127 linhas)
- Gerencia tarefas agendadas persistidas no `ScheduleRepository` usando `node-cron`:
  - `restoreAll(jobCallback)`: carrega todos os registros com `enabled = 1` na inicialização.
  - `addSchedule(cronExpression, prompt, workspacePath, jobCallback)`: valida a expressão via `cron.validate()`, persiste no SQLite e registra a tarefa ativa em `activeTasks`.
  - `removeSchedule(scheduleId)`, `stopAll()` e `listSchedules()`.

### 5.6. `ScreenshotService` (`src/services/screenshotService.ts` — 128 linhas)
- Executa `cdpService.call('Page.captureScreenshot', { format: 'png' | 'jpeg' | 'webp', quality, clip, captureBeyondViewport })` e retorna `Buffer.from(base64Data, 'base64')` (`capture()`) ou a string Base64 crua (`getBase64()`).

### 5.7. `TitleGeneratorService` (`src/services/titleGeneratorService.ts` — 119 linhas)
- Gera títulos curtos para renomeação automática de tópicos de sessão:
  - Remove eventual prefixo `[ワークスペース:...]` (`stripWorkspacePrefix`), extrai os primeiros `40` caracteres do prompt (`GENERATE_TITLE_SCRIPT` / `extractTitleFromText`) e aplica `sanitizeForChannelName(text)` (minúsculas, espaços $\rightarrow$ `-`, mantém apenas `[a-z0-9\-_]` e caracteres CJK, colapsa múltiplos `-`, trunca em `80` caracteres, ou retorna `'untitled'`).

### 5.8. `ProcessManager` (`src/services/processManager.ts` — 78 linhas — Legado)
- Implementa uma fila de execução de processos filhos (`child_process.spawn`, `submitTask`, `stopTask`, `maxConcurrentTasks = 1`). Trata-se de código legado da arquitetura pré-CDP que não é importado por nenhum módulo ativo do bot e será omitido no binário Go (ADR-006).
