# 03 — Protocolo CDP, Conexão e Seletores DOM

## 1. Visão Geral da Camada CDP

A comunicação com o IDE Antigravity é realizada diretamente via **Chrome DevTools Protocol (CDP)** sobre WebSocket local (`127.0.0.1`). No código-fonte original (`src/`), essa camada é composta por 4 arquivos principais:
1. [`src/utils/cdpPorts.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/cdpPorts.ts) (2 linhas): define as portas candidatas `CDP_PORTS`.
2. [`src/services/cdpService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpService.ts) (1712 linhas): cliente WebSocket JSON-RPC, descoberta de janelas/contextos V8, injeção de prompts e imagens, troca de modo/modelo e extração de imagens geradas.
3. [`src/services/cdpConnectionPool.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpConnectionPool.ts) (283 linhas): pool de instâncias `CdpService` e detectores por workspace.
4. [`src/services/chatSessionService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/chatSessionService.ts) (820 linhas): criação de novo chat, leitura de sessão ativa, listagem do histórico ("Past Conversations") e ativação de sessão por título.

---

## 2. Portas Candidatas e Descoberta de Alvos (`cdpPorts.ts` & `cdpService.ts`)

### 2.1. Constante `CDP_PORTS`
```ts
export const CDP_PORTS = [9222, 9223, 9333, 9444, 9555, 9666] as const;
```

### 2.2. Descoberta de Páginas Workbench (`/json/list`)
Para cada porta em `CDP_PORTS`, `CdpService` faz `GET http://127.0.0.1:<port>/json/list` e filtra a lista de alvos (`CdpTarget`):
1. `t.type === 'page'` e `t.webSocketDebuggerUrl` presente.
2. Exclui ferramentas DevTools: `!t.url.startsWith('devtools://') && !t.url.startsWith('chrome-devtools://')`.
3. Prioriza páginas do Workbench/Agent: `t.url.includes('workbench.html') || t.url.includes('jetski-agent') || t.title.includes('Antigravity')`.
4. **Busca por Workspace (`discoverWorkspaceTarget`)**:
   - Extrai o nome do projeto (`extractProjectNameFromPath(workspacePath)`).
   - Compara o título da janela (`t.title`) usando expressão regular com fronteira de palavra (`new RegExp(\`\\\\b\${escapedName}\\\\b\`, 'i')`) para evitar falsos positivos entre projetos com prefixos parecidos.

### 2.3. Abertura Automática de Workspace (`launchAndConnectWorkspace`)
Se nenhuma janela aberta corresponder ao workspace solicitado:
1. Chama `findAvailableCdpPort()`, que testa cada porta de `CDP_PORTS` fazendo bind local via `net.createServer().listen(port, '127.0.0.1')` para encontrar uma porta **livre** (ou reutiliza uma porta CDP já ativa se todas estiverem ocupadas).
2. Executa o binário do Antigravity (`getAntigravityCliPath()`) passando `[workspacePath, '--remote-debugging-port=' + port]` (ou `open -a Antigravity --args ...` no macOS).
3. Faz polling a cada **`1000ms`** (`pollIntervalMs = 1000`) durante até **`30000ms`** (`waitForWindowTimeoutMs = 30000`) chamando `discoverWorkspaceTarget` até que a nova janela exponha seu `webSocketDebuggerUrl`.

---

## 3. Gerenciamento de Conexão WebSocket, Contextos V8 e Pool

### 3.1. Rastreamento de Contextos V8 (`Runtime.enable`)
Ao conectar no `webSocketDebuggerUrl`:
1. Envia `Runtime.enable` para receber eventos de ciclo de vida de contextos V8:
   - `Runtime.executionContextCreated`: adiciona `params.context` à lista `contexts` (ignorando duplicatas).
   - `Runtime.executionContextDestroyed`: remove o `executionContextId` correspondente.
   - `Runtime.executionContextsCleared`: limpa a lista `contexts`.
2. Caso `contexts` ainda esteja vazio após `Runtime.enable`, executa `discoverContexts()` sondando IDs de `1` a `50` com `Runtime.evaluate({ expression: '1+1', contextId })`.
3. `getPrimaryContextId()` prioriza o contexto onde `.antigravity-agent-side-panel` está presente no `document`.

### 3.2. Reconexão Exponencial e Eventos do Pool
- Quando o WebSocket fecha sem `intentionalClose`:
  - `CdpService` tenta reconectar até `maxReconnectAttempts = 3` vezes, com backoff exponencial:
    $$\text{delay} = \text{reconnectDelayMs} \times 2^{\text{attempt}-1} \quad (\text{padrão: } 3000\text{ms}, 6000\text{ms}, 12000\text{ms})$$
  - O `CdpConnectionPool` escuta os eventos de cada `CdpService` e reemite no nível do pool:
    - `workspace:disconnected (projectName)`: o bot envia `⚠️ <b><projectName></b>: Connection lost. Reconnecting…` ao `lastActiveChannel`.
    - `workspace:reconnected (projectName)`: o bot envia `✅ <b><projectName></b>: Reconnected.`.
    - `workspace:reconnectFailed (projectName)`: remove o cliente do pool, para os 4 detectores daquele projeto e o bot envia `❌ <b><projectName></b>: Reconnection failed. Send a message to retry.`.
- **Prevenção de Race Condition (`connectingPromises`)**:
  - Em `CdpConnectionPool.getOrConnect(workspacePath)`, se já houver uma conexão em andamento para o mesmo `projectName`, aguarda a mesma Promise/canal em vez de abrir um segundo WebSocket concorrente.

---

## 4. Injeção de Prompts e Anexação de Imagens (`cdpService.ts` & `imageHandler.ts`)

### 4.1. Injeção de Texto (`injectMessage`)
1. Avalia `FOCUS_EDITOR_SCRIPT` nos contextos V8 para localizar e focar o editor de chat do painel lateral:
   - Prioriza elementos editáveis dentro de `.antigravity-agent-side-panel`: `[contenteditable="true"][role="textbox"]`, `[contenteditable="true"]` ou `textarea` visíveis (`getBoundingClientRect().width > 0`), excluindo o editor principal de código (`.monaco-editor`).
   - Chama `.focus()` no elemento encontrado e limpa eventual texto residual.
2. Envia o comando CDP nativo `Input.insertText({ text: prompt })`.
3. Aguarda **`200ms`**.
4. Dispara a tecla `Enter` via `Input.dispatchKeyEvent`:
   - `type: 'keyDown'`, `key: 'Enter'`, `code: 'Enter'`, `windowsVirtualKeyCode: 13`, `nativeVirtualKeyCode: 13`.
   - `type: 'keyUp'`, `key: 'Enter'`, `code: 'Enter'`, `windowsVirtualKeyCode: 13`, `nativeVirtualKeyCode: 13`.

### 4.2. Anexação de Imagens via CDP (`attachImageFiles` & `injectMessageWithImageFiles`)
Quando o usuário envia uma foto no Telegram (`bot.on('message:photo')`):
1. `downloadTelegramImages` (`src/utils/imageHandler.ts`) baixa a maior resolução da foto (`photos[photos.length - 1]`) via `https://api.telegram.org/file/bot<token>/<file_path>` para `/tmp/antigravity-telegram-remote-inbound/`.
2. `CdpService.attachImageFiles(filePaths)` executa:
   - Gera um token único: `token = "agclaw-upload-" + Date.now() + "-" + Math.random().toString(36).slice(2, 8)`.
   - Avalia um script no contexto do painel lateral que localiza o `<input type="file">` dentro de `.antigravity-agent-side-panel` (ou no documento) e define o atributo `input.setAttribute('data-agclaw-upload-token', token)`.
   - Chama `DOM.enable` e `DOM.getDocument({ depth: 1, pierce: true })` para obter o `root.nodeId`.
   - Chama `DOM.querySelector({ nodeId: rootNodeId, selector: '[data-agclaw-upload-token="' + token + '"]' })`.
   - Chama `DOM.setFileInputFiles({ nodeId: inputNodeId, files: filePaths })`.
   - Remove o atributo temporário `data-agclaw-upload-token` e aguarda **`400ms`** antes de injetar o texto da legenda (`injectMessage`).
3. **Fallback de URLs (`buildPromptWithAttachmentUrls`)**:
   - Em `src/bot/index.ts`, se `injectMessageWithImageFiles` retornar `{ ok: false }`, o bot faz fallback automático chamando `cdp.injectMessage(buildPromptWithAttachmentUrls(prompt, inboundImages))`, que anexa ao final do prompt a seção `Attached images (Telegram URLs):` com os links diretos.
4. Ao término do envio (`finally`), `cleanupInboundImageAttachments` remove os arquivos temporários de `/tmp/antigravity-telegram-remote-inbound/`.

---

## 5. Modos de Execução, Modelos LLM e Imagens de Resposta (`cdpService.ts`)

### 5.1. Leitura e Troca de Modo (`getCurrentMode` / `setUiMode`)
- Mapeamento interno $\leftrightarrow$ UI (`src/services/modeService.ts`):
  - `'fast'` $\leftrightarrow$ `'Fast'` (`⚡ Fast`)
  - `'plan'` $\leftrightarrow$ `'Planning'` (`📋 Plan`)
- `getCurrentMode()` procura no rodapé do `.antigravity-agent-side-panel` botões/spans cujo texto seja exatamente `'Fast'` ou `'Planning'`.
- `setUiMode(mode)` clica no seletor de modo atual para abrir o popover, aguarda `300ms`, e clica na opção correspondente a `'Fast'` ou `'Planning'`.

### 5.2. Leitura e Troca de Modelo LLM (`getUiModels` / `getCurrentModel` / `setUiModel`)
- `getUiModels()` e `setUiModel(modelName)`:
  1. Clicam no botão do seletor de modelo na barra inferior do `.antigravity-agent-side-panel` para abrir o menu dropdown, aguardando `400ms`.
  2. No menu aberto, inspecionam todos os elementos `div.cursor-pointer` cuja classe contenha `'px-2 py-1 flex items-center justify-between'`.
  3. O texto de cada item tem o sufixo `/New$/` removido (`.replace(/New$/, '').trim()`).
  4. O modelo atualmente selecionado é identificado pela presença da classe `'bg-gray-500/20'` no item (ou lido diretamente do botão do seletor fechado em `getCurrentModel()`).
  5. Para apenas listar modelos (`getUiModels`), fecha o menu clicando novamente no gatilho ou enviando `Escape`. Para trocar de modelo (`setUiModel`), faz match case-insensitive (exato ou substring) e executa `.click()` no `div.cursor-pointer` correspondente.

### 5.3. Extração de Imagens Geradas na Resposta (`extractLatestResponseImages`)
Após a conclusão de uma resposta do assistente, `extractLatestResponseImages` busca tags `<img>` dentro do último turno do assistente em `.antigravity-agent-side-panel`:
1. Ignora avatares, ícones de UI e imagens pequenas aplicando o filtro de dimensões:
   ```js
   const w = img.naturalWidth || img.width || rect.width || 0;
   const h = img.naturalHeight || img.height || rect.height || 0;
   if (w < 96 || h < 96 || (w * h) < 12000) continue;
   ```
2. Se `img.src` já for um `data:image/...`, retorna diretamente.
3. Caso contrário (`blob:`, `http://`, `https://`, `vscode-file://`), converte a imagem dentro do contexto do navegador usando `fetch(src)` $\rightarrow$ `res.blob()` $\rightarrow$ `new FileReader()` $\rightarrow$ `reader.readAsDataURL(blob)`, retornando Data URLs Base64 prontas para conversão em `InputFile` pelo `toTelegramInputFile` (`src/utils/imageHandler.ts`).

---

## 6. Gerenciamento de Sessões de Chat (`chatSessionService.ts`)

Todos os métodos de manipulação de sessões de conversa na UI do Antigravity pertencem à classe `ChatSessionService` (`src/services/chatSessionService.ts`):

### 6.1. Iniciar Novo Chat (`startNewChat`)
1. Avalia `GET_NEW_CHAT_BUTTON_SCRIPT` procurando `[data-tooltip-id="new-conversation-tooltip"]`.
2. Verifica o estilo computado `window.getComputedStyle(btn).cursor`:
   - Se `cursor !== 'pointer'` (por exemplo, `'not-allowed'`), significa que o chat atual já está vazio/novo, retornando `{ ok: true, alreadyNew: true }` sem precisar clicar.
3. Se `cursor === 'pointer'`, obtém as coordenadas centrais `(x, y)` via `getBoundingClientRect()` e despacha 3 eventos nativos de mouse via CDP (`Input.dispatchMouseEvent`):
   - `type: 'mouseMoved'`, `x`, `y`
   - `type: 'mousePressed'`, `x`, `y`, `button: 'left'`, `clickCount: 1`
   - `type: 'mouseReleased'`, `x`, `y`, `button: 'left'`, `clickCount: 1`
4. Aguarda **`1500ms`** e reavalia o botão para confirmar que `cursor` mudou para `'not-allowed'` (confirmando que a conversa limpa foi aberta).

### 6.2. Informações da Sessão Atual (`getCurrentSessionInfo`)
Inspeciona `.antigravity-agent-side-panel`:
- Busca o cabeçalho `div[class*="border-b"]` $\rightarrow$ `div[class*="text-ellipsis"]`.
- Se o texto for vazio ou igual a `'Agent'`, `'Antigravity'` ou `'New Chat'`, retorna `{ title: '(New Chat)', hasActiveChat: false }`; caso contrário, retorna `{ title, hasActiveChat: true }`.

### 6.3. Listar Sessões do Workspace Atual (`listAllSessions`)
1. Localiza o botão de histórico (`[data-tooltip-id="history-tooltip"]` ou `[data-tooltip-id*="past-conversations"]`) e clica nele via `Input.dispatchMouseEvent` nas coordenadas `(x, y)`.
2. Aguarda **`800ms`** para o modal "Past Conversations" renderizar.
3. Executa `SCRAPE_SESSIONS_SCRIPT`:
   - Percorre as linhas do modal de histórico **parando imediatamente ao encontrar o cabeçalho divisório `"Other Conversations"`**, garantindo que apenas as sessões pertencentes ao workspace atual sejam coletadas.
   - Se encontrar menos de **`20` sessões** (`LIST_SESSIONS_TARGET = 20`) e existir um botão `"Show N more..."` acima de `"Other Conversations"`, clica em `"Show N more..."`, aguarda **`600ms`** e executa o scrape novamente.
4. Fecha o modal enviando `Input.dispatchKeyEvent` com `key: 'Escape'`.

### 6.4. Ativar Sessão por Título (`activateSessionByTitle`)
Executa um loop de retry por até **`30000ms`** (`ACTIVATE_TIMEOUT_MS = 30000`) com intervalo de **`800ms`** (`ACTIVATE_RETRY_INTERVAL_MS = 800`):
1. Verifica via `getCurrentSessionInfo` se a sessão desejada já está ativa (comparação case-insensitive do prefixo/título).
2. Tenta ativação direta no painel lateral via `buildActivateChatByTitleScript(targetTitle)`.
3. Se não encontrar no painel lateral, abre o modal "Past Conversations", digita/filtra ou clica no item correspondente via `buildActivateViaPastConversationsScript(targetTitle)` (expandindo `"Show N more..."` se necessário).
4. Confirma a mudança de título via `getCurrentSessionInfo` antes de retornar `{ ok: true }`.
