# 07 — Bot do Telegram, Comandos, Callbacks e UI

## 1. Visão Geral da Camada do Bot e UI

A camada de interação com o Telegram é implementada em [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts) (1938 linhas), apoiada por:
- **6 módulos em `src/commands/`**: [`chatCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/chatCommandHandler.ts) (40 linhas), [`cleanupCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/cleanupCommandHandler.ts) (53 linhas), [`joinCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/joinCommandHandler.ts) (36 linhas), [`messageParser.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/messageParser.ts) (37 linhas), [`slashCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/slashCommandHandler.ts) (123 linhas) e [`workspaceCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/workspaceCommandHandler.ts) (32 linhas).
- **8 módulos em `src/ui/`**: [`autoAcceptUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/autoAcceptUi.ts) (27 linhas), [`modeUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/modeUi.ts) (65 linhas), [`modelsUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/modelsUi.ts) (120 linhas), [`planUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/planUi.ts) (177 linhas), [`projectListUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/projectListUi.ts) (71 linhas), [`screenshotUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/screenshotUi.ts) (29 linhas), [`sessionPickerUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/sessionPickerUi.ts) (37 linhas) e [`templateUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/templateUi.ts) (55 linhas).
- **Gerenciador de Tópicos**: [`src/services/telegramTopicManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/telegramTopicManager.ts) (112 linhas).

---

## 2. Identificação de Canais, Tópicos de Fórum e Autorização

### 2.1. Chave Composta `channelKey` (`<chatId>:<threadId>`)
O bot identifica cada conversa ou tópico através de `TelegramChannel { chatId: number | string, threadId?: number }`:
```ts
function channelKey(ch: TelegramChannel): string {
    return ch.threadId ? `${ch.chatId}:${ch.threadId}` : String(ch.chatId);
}
```
Essa string é usada como chave `channel_id` nas tabelas `workspace_bindings` e `chat_sessions` do SQLite.

### 2.2. `TelegramTopicManager` (`src/services/telegramTopicManager.ts`)
Quando `config.useTopics === true` e o chat é um supergrupo com Fórum ativo (`ctx.chat.type === 'supergroup' && ctx.chat.is_forum === true`):
- `ensureTopic(workspaceName)`: cria um tópico via `api.createForumTopic(chatId, "🗂️ " + workspaceName)` e armazena o `message_thread_id` no `topicCache`.
- `createSessionTopic(sessionName)`: cria um tópico via `api.createForumTopic(chatId, "💬 " + sessionName)`.
- `renameTopic(topicId, newName)`: chama `api.editForumTopic(chatId, topicId, { name: newName })`.
- `closeTopic(topicId)`: chama `api.closeForumTopic(chatId, topicId)`.
- `deleteTopic(topicId)`: chama `api.deleteForumTopic(chatId, topicId)`.
- `sanitizeName(name)`: remove barras finais, troca `/` por `-`, colapsa hífens repetidos e trunca em `128` caracteres.

### 2.3. Middleware de Autorização
Todas as atualizações passam pelo middleware global em `src/bot/index.ts` (linhas 929–939):
- Extrai `userId = String(ctx.from?.id ?? '')`.
- Se `!config.allowedUserIds.includes(userId)`:
  - Se for um `callback_query`, responde `ctx.answerCallbackQuery({ text: 'You do not have permission.' })`.
  - Bloqueia silenciosamente mensagens de usuários não autorizados.

---

## 3. Comandos Registrados no Bot (`src/bot/index.ts`)

O bot registra **16 comandos** via `bot.command(...)` (dos quais 15 são enviados ao menu do Telegram via `bot.api.setMyCommands` no `onStart`; `/cleanup` funciona via `bot.command('cleanup')` e aparece em `/help`):

| Comando | No `setMyCommands`? | Comportamento Exato em `src/bot/index.ts` |
| :--- | :---: | :--- |
| `/start` | Sim | Exibe mensagem de boas-vindas HTML (`antigravity-telegram-remote Online`). |
| `/help` | Sim | Lista os comandos divididos em 6 seções (`Chat`, `Control`, `Settings`, `Projects`, `Templates`, `System`). |
| `/project` | Sim | Escaneia subdiretórios de `WORKSPACE_BASE_DIR` via `workspaceService.scanWorkspaces()` e exibe `buildProjectListUI(workspaces, 0)` (`ITEMS_PER_PAGE = 100`). |
| `/new` | Sim | Localiza o workspace vinculado ao canal/tópico atual, conecta via `bridge.pool.getOrConnect(workspacePath)` e chama `chatSessionService.startNewChat(cdp)` (clique via `Input.dispatchMouseEvent` no ícone de nova conversa). |
| `/chat` | Sim | No TS original: se o canal possui sessão no SQLite, lista `chatSessionRepo.findByCategoryId(session.categoryId)` e destaca a sessão atual (`← Current`); se não possui sessão no SQLite, consulta `chatSessionService.getCurrentSessionInfo(anyCdp)` e mostra título e status (`🟢 Active` / `⚪ Inactive`). **No port Go (ADR-005)**: além do resumo da sessão, expõe o seletor interativo `buildSessionPickerUI` alimentado por `chatSessionService.listAllSessions(cdp)` (também acessível via `/join`). |
| `/mode` | Sim | Chama `sendModeUI` (`src/ui/modeUi.ts`), que sincroniza `cdp.getCurrentMode()` ao vivo e exibe botões `mode_select:fast` e `mode_select:plan`. |
| `/model [nome]` | Sim | Se receber `<nome>`, chama diretamente `cdp.setUiModel(modelName)`. Sem argumentos, chama `sendModelsUI` (`src/ui/modelsUi.ts`), combinando `cdp.getUiModels()`, `cdp.getCurrentModel()` e `bridge.quota.fetchQuota()`. |
| `/stop` | Sim | Avalia `RESPONSE_SELECTORS.CLICK_STOP_BUTTON` via `Runtime.evaluate`, adiciona a chave do canal em `userStopRequestedChannels` e responde `⏹️ Generation Interrupted`. |
| `/screenshot` | Sim | Chama `handleScreenshot` (`src/ui/screenshotUi.ts`), que captura PNG via `ScreenshotService.capture({ format: 'png' })` e envia com `ctx.replyWithPhoto(new InputFile(buffer, 'screenshot.png'), { caption: '📸 Screenshot' })`. |
| `/template` | Sim | Lista até 25 templates do `TemplateRepository` via `sendTemplateUI` (`src/ui/templateUi.ts`) com botões `template_btn_<id>`. |
| `/template_add` | Sim | Sintaxe `/template_add <name> <prompt>`: delega para `slashCommandHandler.handleCommand('template', ['add', name, prompt])`. |
| `/template_delete` | Sim | Sintaxe `/template_delete <name>`: delega para `slashCommandHandler.handleCommand('template', ['delete', name])`. |
| `/autoaccept [on\|off]` | Sim | Se receber `on` ou `off`, altera diretamente via `bridge.autoAccept.handle(requestedMode)`. Sem argumento, exibe `sendAutoAcceptUI` (`src/ui/autoAcceptUi.ts`). |
| `/cleanup [days]` | Não (está em `/help`) | Padrão `days = 7`. Busca sessões/vínculos cujo `createdAt < cutoffIso` via `cleanupHandler.findInactiveSessions(guildId, days)` e exibe botões `cleanup_archive:<days>`, `cleanup_delete:<days>` e `cleanup_cancel`. |
| `/status` | Sim | Exibe status de conexão do `CdpConnectionPool` (projetos conectados e contagem de contextos V8 de cada um), modo atual e estado do Auto Approve (`🟢 ON` / `⚪ OFF`). |
| `/ping` | Sim | Envia `🏓 Pong!` e edita a mensagem com a latência medida em milissegundos (`Latency: <b>Xms</b>`). |

### 3.1. Parser de Comandos em Texto e Voz (`messageParser.ts` & `slashCommandHandler.ts`)
No handler `bot.on('message:text')` (e após a transcrição em `bot.on('message:voice')`), `parseMessageContent(text)` verifica se o texto começa com `/` e separa tokens preservando argumentos entre aspas duplas (`/(?:[^\s"]+|"[^"]*")+/g`). Isso permite executar via texto ou voz:
- `/template` (lista templates em texto)
- `/template add "nome" "prompt"`
- `/template delete "nome"`
- `/template <nome>` (invoca imediatamente o template enviando seu `prompt` pelo `PromptDispatcher`!)
- `/autoaccept [on|off|status]`, `/screenshot` e `/status`.

---

## 4. Tabela Completa de `callback_data` (`src/bot/index.ts` & `src/ui/`)

Todos os botões de teclado inline respeitam o limite de 64 bytes da API do Telegram e utilizam os seguintes formatos exatos em `src/`:

| Categoria | Formato Exato de `callback_data` | Arquivo Gerador | Ação Executada em `bot.on('callback_query:data')` |
| :--- | :--- | :--- | :--- |
| **Modo** | `mode_select:fast`<br>`mode_select:plan` | `src/ui/modeUi.ts` | Altera o modo em `ModeService` e na UI (`cdp.setUiMode`), atualiza o teclado `buildModeUI` e responde ao callback. |
| **Modelos** | `model_btn_<cbName>`<br>`model_exhausted_<cbName>`<br>`model_refresh_btn` | `src/ui/modelsUi.ts` (`MAX_BUTTONS = 24`, `cbName` truncado para caber em 64 bytes) | `model_btn_`: chama `cdp.setUiModel(modelName)` e atualiza `buildModelsUI`.<br>`model_exhausted_`: exibe alerta modal (`show_alert: true`) informando que a cota do modelo está esgotada.<br>`model_refresh_btn`: reconsulta modelos e cotas (`fetchQuota`). |
| **Auto-Accept** | `autoaccept_btn_on`<br>`autoaccept_btn_off`<br>`autoaccept_btn_refresh` | `src/ui/autoAcceptUi.ts` | Altera o estado em `AutoAcceptService` e atualiza a mensagem com `sendAutoAcceptUI`. |
| **Projetos** | `project_select:<workspacePath>`<br>`project_page:<page>` | `src/ui/projectListUi.ts` (`ITEMS_PER_PAGE = 100`) | `project_select:`: se `useTopics && isForum && !ch.threadId`, reutiliza ou cria tópico `🗂️ <sanitized>` via `topicManager.ensureTopic`, envia mensagem de boas-vindas no tópico e faz `workspaceBindingRepo.upsert`. Caso contrário, vincula ao canal atual.<br>`project_page:`: edita a mensagem com a página solicitada. |
| **Templates** | `template_btn_<id>` | `src/ui/templateUi.ts` (`MAX_BUTTONS = 25`) | Busca o template por `id` (`templateRepo.findById`) e despacha `template.prompt` via `promptDispatcher.send`. |
| **Sessões** | `session_select:<title.slice(0, 49)>` | `src/ui/sessionPickerUi.ts` (`MAX_SELECT_OPTIONS = 25`) | Conecta ao workspace vinculado ao canal e chama `chatSessionService.activateSessionByTitle(cdp, selectedTitle)`, editando a mensagem para `🔗 Joined Session`. |
| **Aprovação** | `approve_action:<project>:<ch>`<br>`always_allow_action:<project>:<ch>`<br>`deny_action:<project>:<ch>` | `src/services/cdpBridgeManager.ts` | Aciona `detector.approveButton()`, `detector.alwaysAllowButton()` ou `detector.denyButton()` no `ApprovalDetector` do projeto e remove o teclado inline. |
| **Plano (Detector)** | `planning_open_action:<project>:<ch>`<br>`planning_proceed_action:<project>:<ch>` | `src/services/cdpBridgeManager.ts` | Handlers de compatibilidade para abrir o plano (`clickOpenButton` + `extractPlanContent`) ou prosseguir (`clickProceedButton`). |
| **Plano (UI Rica)** | `plan_view_btn:<project>:<ch>`<br>`plan_proceed_btn:<project>:<ch>`<br>`plan_edit_btn:<project>:<ch>`<br>`plan_refresh_btn:<project>:<ch>`<br>`plan_page:<page>:<project>:<ch>` | `src/ui/planUi.ts` (`PAGE_SIZE = 3500`) | `plan_view_btn`: clica em `Open`, extrai o Markdown (`extractPlanContent` com até 3 tentativas de 500ms), pagina em blocos de 3500 chars (`planContentCache`) e envia `buildPlanContentUI`.<br>`plan_proceed_btn`: clica em `Proceed` e limpa `planEditPendingChannels`.<br>`plan_edit_btn`: registra o canal em `planEditPendingChannels` aguardando o próximo texto do usuário (ou `/cancel`).<br>`plan_refresh_btn`: atualiza o card com `detector.getLastDetectedInfo()`.<br>`plan_page`: navega entre as páginas em cache do plano. |
| **Popup de Erro** | `error_popup_dismiss_action:<project>:<ch>`<br>`error_popup_copy_debug_action:<project>:<ch>`<br>`error_popup_retry_action:<project>:<ch>` | `src/services/cdpBridgeManager.ts` | `dismiss`: clica em Dismiss.<br>`copy_debug`: clica em `Copy debug info`, aguarda `300ms`, lê `detector.readClipboard()` e envia até `3800` chars em `<pre>`.<br>`retry`: clica em Retry. |
| **Cleanup** | `cleanup_archive:<days>`<br>`cleanup_delete:<days>`<br>`cleanup_cancel` | `src/commands/cleanupCommandHandler.ts` | Para cada sessão inativa: fecha o tópico (`closeForumTopic` no archive) ou exclui o tópico (`deleteForumTopic` no delete) e remove os registros de `chat_sessions` e `workspace_bindings` via `cleanupByChannelId`. |

---

## 5. Fluxo de Mensagens de Texto, Fotos e Voz (`src/bot/index.ts`)

### 5.1. Mensagens de Texto (`bot.on('message:text')`)
1. **Interceptação de Edição de Plano (`planEditPendingChannels`)**:
   - Se o canal estiver aguardando feedback de edição de plano (`plan_edit_btn`):
     - Se o texto for `/cancel`, cancela o modo de edição.
     - Caso contrário, remove o estado pendente e envia ao Antigravity o prompt:
       `Please revise the plan based on the following feedback:\n\n<texto>`
2. **Interceptação de Slash Commands em Texto**:
   - Processa via `parseMessageContent` + `SlashCommandHandler` (`/autoaccept`, `/screenshot`, `/status`, `/template ...`).
3. **Roteamento de Sessão e Prevenção de Eco**:
   - Resolve o workspace e conexão CDP (`resolveWorkspaceAndCdp`).
   - Se a sessão já foi renomeada (`session?.isRenamed && session.displayName`), garante que a conversa correta esteja ativa na UI chamando `chatSessionService.activateSessionByTitle(cdp, session.displayName)`. Se a sessão existe mas ainda não foi renomeada (`!session.isRenamed`), inicia um chat limpo via `chatSessionService.startNewChat(cdp)`.
   - Registra o hash do texto no `UserMessageDetector` (`userMsgDetector.addEchoHash(text)`) para que o próprio prompt não seja detectado como mensagem vinda do PC.
   - Envia pelo `promptDispatcher.send(...)`.

### 5.2. Mensagens de Foto (`bot.on('message:photo')`)
1. Seleciona a maior variante da foto (`photos[photos.length - 1]`) e usa `ctx.message.caption?.trim() || 'Please review the attached images and respond accordingly.'`.
2. Baixa a imagem para `/tmp/antigravity-telegram-remote-inbound/` via `downloadTelegramImages`.
3. Despacha via `promptDispatcher.send(...)` (que anexa via `DOM.setFileInputFiles` ou faz fallback para `buildPromptWithAttachmentUrls`) e remove o arquivo temporário no `finally` (`cleanupInboundImageAttachments`).

### 5.3. Mensagens de Voz (`bot.on('message:voice')`)
1. Verifica `checkWhisperAvailability()` (`src/utils/voiceHandler.ts`). Se o `nodejs-whisper` / `whisper-cli` / modelo `base.en` não estiver pronto, envia as instruções de instalação.
2. Envia `🎙️ Transcribing voice message...`, baixa o arquivo `.ogg` via `downloadTelegramVoice` e executa `transcribeVoice(voicePath)`.
3. Se a transcrição começar com `/` (`parseMessageContent(transcript).isCommand`), executa como comando via `SlashCommandHandler` (e dispara o prompt se for uma invocação de `/template <nome>`).
4. Caso contrário, responde com o texto transcrito (`📝 "<transcript>"`), registra `userMsgDetector.addEchoHash(transcript)` e despacha o texto transcrito para o Antigravity via `promptDispatcher.send(...)`.
