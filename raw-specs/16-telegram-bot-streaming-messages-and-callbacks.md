# 16 — Streaming de Respostas, Handlers de Mensagens (Texto/Foto/Voz) e Roteador de Callbacks

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de maneira 100% autocontida e exaustiva o pipeline de envio de prompts e streaming de progresso/respostas (`sendPromptToAntigravity`), o espelhamento passivo PC $\rightarrow$ Telegram (`UserMessageDetector` + `ResponseMonitor.startPassive()`), o roteador completo de `callback_query:data` (cobrindo todas as 11 famílias de `callback_data`) e os handlers de mensagens de texto, foto e voz de [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript) | Intervalo de Linhas | Responsabilidade | Destino no Port Go |
| :--- | :---: | :--- | :--- |
| [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts#L68-L855) | 68–855 | Constantes de streaming, `stripHtmlForFile`, `createSerialTaskQueue`, `sendPromptToAntigravity`, `balanceHtmlTags`, `sendChunkedResponse`, `onProcessLog`, `onThinkingLog`, `onComplete`, `onTimeout`, extração de `<blockquote expandable>`, imagens geradas e auto-rename de tópico/sessão | `internal/bot/streaming.go`<br>`internal/bot/streaming_test.go` |
| [`src/services/cdpBridgeManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpBridgeManager.ts#L34-L439) | 34–439 | `parseApprovalCustomId`, `parsePlanningCustomId`, `parseErrorPopupCustomId`, `ensureUserMessageDetector` e espelhamento PC $\rightarrow$ Telegram | `internal/bot/streaming.go`<br>`internal/bot/callbacks.go` |
| [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts#L1275-L1674) | 1275–1674 | Roteador `bot.on('callback_query:data')` para todas as 11 famílias de botões inline | `internal/bot/callbacks.go`<br>`internal/bot/callbacks_test.go` |
| [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts#L1680-L1897) | 1680–1897 | Handlers `bot.on('message:text')`, `bot.on('message:photo')` e `bot.on('message:voice')` | `internal/bot/handlers_messages.go`<br>`internal/bot/handlers_messages_test.go` |

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pipeline de Streaming e Filas Seriais (`internal/bot/streaming.go`)

```go
package bot

import (
	"context"
	"sync"

	"antigravity-telegram-remote/internal/bot/ui"
	"antigravity-telegram-remote/internal/cdp"
	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/services"
	"antigravity-telegram-remote/internal/utils"
)

const (
	MaxOutboundGeneratedImages = 4
	TelegramMsgLimit           = 4096
	MaxInlineChunks            = 5
	LiveResponseMaxLen         = 3800
	MaxProgressBody            = 3500
	MaxProgressEntries         = 60
	ElapsedTimerIntervalMs     = 5000
	MonitorPollIntervalMs      = 2000
	MonitorMaxDurationMs       = 1800000 // 30 minutos
	MonitorStopGoneConfirm     = 3
)

// PhaseIcons mapeia cada fase de execução ao seu emoji correspondente.
var PhaseIcons = map[string]string{
	"sending":    "📡",
	"thinking":   "🧠",
	"generating": "✍️",
	"complete":   "✅",
	"timeout":    "⏰",
	"error":      "❌",
}

// ProgressEntryKind identifica o tipo de evento no log de progresso ordenado.
type ProgressEntryKind string

const (
	ProgressKindThought        ProgressEntryKind = "thought"
	ProgressKindThoughtContent ProgressEntryKind = "thought-content"
	ProgressKindActivity       ProgressEntryKind = "activity"
)

// ProgressEntry representa um item no fluxo ordenado de eventos da mensagem de progresso ao vivo.
type ProgressEntry struct {
	Kind ProgressEntryKind
	Text string
}

// ThinkingBlock representa um bloco <details><summary> extraído do DOM ao final da geração.
type ThinkingBlock struct {
	Label string `json:"label"`
	Body  string `json:"body"`
}

// PromptSendOptions reúne os serviços opcionais usados para renomeação automática de sessão/tópico após conclusão.
type PromptSendOptions struct {
	ChatSessionService *services.ChatSessionService
	ChatSessionRepo    *database.ChatSessionRepository
	TopicManager       *TelegramTopicManager
	TitleGenerator     *services.TitleGeneratorService
}

// SerialTaskQueue executa tarefas assíncronas estritamente em série (FIFO) para uma mesma categoria/canal.
type SerialTaskQueue struct {
	mu      sync.Mutex
	name    string
	traceID string
	seq     int
}

// NewSerialTaskQueue cria uma fila de execução serial equivalente a createSerialTaskQueue.
func NewSerialTaskQueue(queueName, traceID string) *SerialTaskQueue

// Enqueue executa fn de forma serializada, capturando e logando erros sem interromper a fila.
func (q *SerialTaskQueue) Enqueue(ctx context.Context, label string, fn func(ctx context.Context) error) error

// StripHTMLForFile converte HTML do Telegram de volta para Markdown legível para anexos .md.
func StripHTMLForFile(htmlStr string) string

// BalanceHTMLTags fecha tags HTML abertas na ordem inversa e descarta tags de fechamento órfãs.
func BalanceHTMLTags(htmlStr string) string

// StripHTML remove todas as tags HTML e decodifica entidades básicas para fallback em texto puro.
func StripHTML(htmlStr string) string

// IsJunkEntry verifica se uma linha de atividade deve ser descartada (curta demais, apenas números ou 1 palavra < 8 chars).
func IsJunkEntry(text string) bool

// FormatActivityLine colapsa quebras de linha/espaços múltiplos, filtra junk e aplica EscapeHTML nos primeiros 120 caracteres.
func FormatActivityLine(raw string) string

// SendPromptToAntigravity executa o ciclo completo de injeção de prompt/imagem e monitoramento de resposta em tempo real.
func (b *Bot) SendPromptToAntigravity(
	ctx context.Context,
	channel TelegramChannel,
	prompt string,
	cdpSvc *cdp.CdpService,
	inboundImages []utils.InboundImageAttachment,
	options *PromptSendOptions,
) error

// StartPassiveMirrorForUserMessage envia a notificação da mensagem digitada no PC e inicia o monitoramento passivo (startPassive) para espelhar a resposta no Telegram.
func (b *Bot) StartPassiveMirrorForUserMessage(
	ctx context.Context,
	projectName string,
	cdpSvc *cdp.CdpService,
	info services.UserMessageInfo,
)
```

---

### 2.2. Roteador de Callbacks (`internal/bot/callbacks.go`)

```go
package bot

import "context"

// CallbackContext abstrai os dados e operações de resposta de um callback_query do Telegram.
type CallbackContext interface {
	Data() string
	Channel() TelegramChannel
	ChatType() string
	IsForum() bool
	EditMessageText(ctx context.Context, text string, keyboard *ui.InlineKeyboardMarkup) error
	EditMessagePlainText(ctx context.Context, text string) error
	RemoveInlineKeyboard(ctx context.Context) error
	AnswerCallbackQuery(ctx context.Context, text string, showAlert bool) error
}

// HandleCallbackQuery processa todas as 11 famílias de callback_data.
func (b *Bot) HandleCallbackQuery(ctx context.Context, cb CallbackContext) error
```

---

### 2.3. Handlers de Mensagens de Texto, Foto e Voz (`internal/bot/handlers_messages.go`)

```go
package bot

import (
	"context"

	"antigravity-telegram-remote/internal/utils"
)

// HandleTextMessage processa mensagens de texto recebidas (edição de plano pendente, slash commands em texto e envio ao Antigravity).
func (b *Bot) HandleTextMessage(ctx context.Context, ch TelegramChannel, rawText string) error

// HandlePhotoMessage processa mensagens com foto (seleciona maior variante, baixa para /tmp, envia ao Antigravity e limpa no defer).
func (b *Bot) HandlePhotoMessage(
	ctx context.Context,
	ch TelegramChannel,
	messageID int,
	photos []utils.TelegramPhotoSize,
	caption string,
) error

// HandleVoiceMessage processa mensagens de voz (valida Whisper local, baixa .ogg, transcreve com base.en e executa comando ou envia prompt).
func (b *Bot) HandleVoiceMessage(
	ctx context.Context,
	ch TelegramChannel,
	voice utils.TelegramVoice,
) error
```

---

## 3. Constantes, Templates HTML/ASCII, Prefixos de Callback e Expressões Regulares Literais

### 3.1. Constantes e Limites de Streaming (`src/bot/index.ts`, linhas 68–80 e 350–353)
- `PHASE_ICONS`:
  - `sending`: `'📡'`
  - `thinking`: `'🧠'`
  - `generating`: `'✍️'`
  - `complete`: `'✅'`
  - `timeout`: `'⏰'`
  - `error`: `'❌'`
- `MAX_OUTBOUND_GENERATED_IMAGES = 4`
- `TELEGRAM_MSG_LIMIT = 4096`
- `MAX_INLINE_CHUNKS = 5`
- `LIVE_RESPONSE_MAX_LEN = 3800`
- `MAX_PROGRESS_BODY = 3500`
- `MAX_PROGRESS_ENTRIES = 60`

---

### 3.2. Expressões Regulares e Scripts DOM Literais em `sendPromptToAntigravity`

1. **`stripHtmlForFile(html)` (`src/bot/index.ts`, linhas 82–112)**:
   - `<pre>\s*<code\s+class="language-([^"]*)">([\s\S]*?)<\/code>\s*<\/pre>/gi` $\rightarrow$ `` `\n\`\`\`${lang}\n${content}\n\`\`\`\n` ``
   - `/<pre[^>]*>([\s\S]*?)<\/pre>/gi` $\rightarrow$ `` `\n\`\`\`\n${content}\n\`\`\`\n` ``
   - `/<code>([\s\S]*?)<\/code>/gi` $\rightarrow$ `` `\`$1\`` ``
   - `/<b>([\s\S]*?)<\/b>/gi` $\rightarrow$ `'**$1**'`
   - `/<i>([\s\S]*?)<\/i>/gi` $\rightarrow$ `'*$1*'`
   - `/<s>([\s\S]*?)<\/s>/gi` $\rightarrow$ `'~~$1~~'`
   - `/<a\s+href="([^"]*)">([\s\S]*?)<\/a>/gi` $\rightarrow$ `'[$2]($1)'`
   - `/<blockquote[^>]*>([\s\S]*?)<\/blockquote>/gi` $\rightarrow$ cada linha prefixada com `'> '`
   - `/<[^>]+>/g` $\rightarrow$ `''`
   - Decodificação de `&amp;`, `&lt;`, `&gt;`, `&quot;` e colapso `/\n{3,}/g` $\rightarrow$ `'\n\n'`.

2. **Detecção de intenção/URL de imagem gerada (`src/bot/index.ts`, linhas 489–490)**:
   - `imageIntentPattern = /(image|images|png|jpg|jpeg|gif|webp|illustration|diagram|render)/i`
   - `imageUrlPattern = /https?:\/\/\S+\.(png|jpg|jpeg|gif|webp)/i`

3. **Classificação de `onThinkingLog` (`src/bot/index.ts`, linhas 587–592)**:
   - `stripped = trimmed.replace(/^[^a-zA-Z]+/, '')`
   - `thinking...`: `/^thinking\.{0,3}$/i`
   - `thought for ...`: `/^thought for\s/i`

4. **Script DOM de Extração de Emergência (`tryEmergencyExtractText`, linhas 512–522)**:
   ```javascript
   (() => {
       const panel = document.querySelector('.antigravity-agent-side-panel');
       const scope = panel || document;
       const candidateSelectors = ['.rendered-markdown', '.leading-relaxed.select-text', '.flex.flex-col.gap-y-3', '[data-message-author-role="assistant"]', '[data-message-role="assistant"]', '[class*="assistant-message"]', '[class*="message-content"]', '[class*="markdown-body"]', '.prose'];
       const looksLikeActivity = (text) => { const n = (text || '').trim().toLowerCase(); if (!n) return true; return /^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|analyzed|read|wrote|ran)/i.test(n) && n.length <= 220; };
       const clean = (text) => (text || '').replace(/\r/g, '').replace(/\n{3,}/g, '\n\n').trim();
       const candidates = []; const seen = new Set();
       for (const selector of candidateSelectors) { const nodes = scope.querySelectorAll(selector); for (const node of nodes) { if (!node || seen.has(node)) continue; seen.add(node); candidates.push(node); } }
       for (let i = candidates.length - 1; i >= 0; i--) { const node = candidates[i]; const text = clean(node.innerText || node.textContent || ''); if (!text || text.length < 20) continue; if (looksLikeActivity(text)) continue; if (/^(good|bad)$/i.test(text)) continue; return text; }
       return '';
   })()
   ```

5. **Script DOM de Extração dos Blocos `<details>` de Thinking na Conclusão (`src/bot/index.ts`, linhas 705–737)**:
   ```javascript
   (function() {
       var panel = document.querySelector('.antigravity-agent-side-panel');
       var scope = panel || document;
       var details = scope.querySelectorAll('details');
       var blocks = [];
       for (var i = 0; i < details.length; i++) {
           var d = details[i];
           var summary = d.querySelector('summary');
           if (!summary) continue;
           var rawLabel = (summary.textContent || '').trim();
           var stripped = rawLabel.replace(/^[^a-zA-Z]+/, '');
           if (!/^(?:thought for|thinking)\b/i.test(stripped)) continue;
           var wasOpen = d.open;
           if (!wasOpen) d.open = true;
           // Try children first, then fall back to full textContent minus summary
           var children = d.children;
           var parts = [];
           for (var c = 0; c < children.length; c++) {
               if (children[c].tagName === 'SUMMARY' || children[c].tagName === 'STYLE') continue;
               var t = (children[c].innerText || children[c].textContent || '').trim();
               if (t && t.length >= 5) parts.push(t);
           }
           // Fallback: use detail's full text minus the summary text
           if (parts.length === 0) {
               var fullText = (d.innerText || d.textContent || '').trim();
               var bodyText = fullText.replace(rawLabel, '').trim();
               if (bodyText && bodyText.length >= 5) parts.push(bodyText);
           }
           if (!wasOpen) d.open = false;
           blocks.push({ label: rawLabel, body: parts.join('\n\n') });
       }
       return blocks;
   })()
   ```

---

### 3.3. Templates HTML Literais do Fluxo de Streaming (`sendPromptToAntigravity`)

- **Erro de Conexão Inicial (`!cdp.isConnected()`, linhas 327–330)**:
  - Título: `❌ Connection Error`
  - Descrição: `` `Not connected to Antigravity.\nStart with \`${getAntigravityCdpHint(9223)}\`, then send a message to auto-connect.` ``
  - Enviado como: `<b>${escapeHtml(title)}</b>\n\n${escapeHtml(description)}`
- **Mensagem Inicial de Progresso (`liveActivityMsgId`, linha 342)**:
  ```html
  <b>📡 {escapeHtml(modeName)} · {escapeHtml(modelLabel)}</b>

  <i>Sending...</i>
  ```
- **Fallback de Anexo de Imagem (`!injectResult.ok` com `inboundImages`, linha 538)**:
  - Título: `🖼️ Attached image fallback`
  - Descrição: `Failed to attach image directly, resending via URL reference.`
- **Falha de Injeção de Mensagem (`!injectResult.ok`, linha 547)**:
  - Título: `❌ Message Injection Failed`
  - Descrição: `Failed to send message: ${injectResult.error}`
- **Mensagem de Progresso Durante Execução (`buildProgressMessage`, linhas 391–423)**:
  - Título: `🧠 ${modelLabel}`
  - Corpo (`buildProgressBody()`): entradas separadas por `\n\n`:
    - `thought`: `💭 <i>${escapeHtml(e.text)}</i>`
    - `thought-content`: `<i>${escapeHtml(e.text)}</i>`
    - `activity`: `${e.text}` (já escapado e truncado em 120 chars)
    - Se `thinkingActive == true`: adiciona linha `💭 <i>Thinking...</i>`
    - Se o corpo exceder `3500` chars: `'...\n\n' + body.slice(-3495)`
    - Se o corpo estiver vazio: `'<i>Generating...</i>'`
  - Rodapé: `⏱️ ${elapsed}s`
  - Formato completo: `<b>${escapeHtml(title)}</b>\n\n${body}\n\n<i>${escapeHtml(footer)}</i>`
- **Interrupção pelo Usuário (`wasStoppedByUser`, linha 636)**:
  ```text
  ⏹️ Generation stopped.
  ```
- **Cota Esgotada (`isQuotaError`, linhas 647–656)**:
  - Atualiza `liveActivityMsgId`:
    ```html
    <b>⚠️ {escapeHtml(modelLabel)} · Quota Reached</b>

    {buildProgressBody()}

    <i>⏱️ {elapsed}s</i>
    ```
  - Atualiza/cria `liveResponseMsgId`:
    - Título: `⚠️ Quota Reached`
    - Corpo: `Model quota limit reached. Please wait or switch to a different model.`
    - Rodapé: `⏱️ ${elapsed}s`
  - Em seguida, constrói e envia automaticamente o seletor de modelos `buildModelsUI(cdp, () => bridge.quota.fetchQuota())`.
- **Bloco Colapsável de Thinking (`thinkBlocks.length > 0`, linhas 745–760)**:
  - Cada seção formatada como:
    - Com `body`: `  💭 <b>${escapeHtml(label)}</b>\n\n<i>${escapeHtml(body)}</i>`
    - Sem `body`: `  💭 <b>${escapeHtml(label)}</b>`
  - Seções unidas por `\n\n`, truncadas em `TELEGRAM_MSG_LIMIT - 100` (`3996` chars, adicionando `'...'` se truncado) e envolvidas em:
    ```html
    <blockquote expandable>{trimmed}</blockquote>
    ```
- **Mensagem de Progresso Concluída (linha 776)**:
  ```html
  <b>✅ {escapeHtml(modelLabel)} · {elapsed}s</b>

  {completedBody}
  ```
- **Resposta Final Vazia (linha 783)**:
  - Título: `✅ Complete`
  - Corpo: `Failed to extract response. Use /screenshot to verify.`
  - Rodapé: `⏱️ ${elapsed}s`
- **Anexo de Arquivo `.md` para Respostas Longas (`bodyChunks.length > 5`, linhas 318–321)**:
  - Nome do arquivo: `response-${timestamp}.md` (onde `timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)`)
  - Caption: `📄 Full response (${rawBody.length} chars)`
- **Legenda de Imagens Geradas (`sendGeneratedImages`, linha 501)**:
  - Caption: `🖼️ Generated image (${i + 1}/${extracted.length})`
- **Timeout de 30 Minutos (`onTimeout`, linhas 828–836)**:
  - Se houve texto parcial: `${separated.output}\n\n[Monitor Ended] Timeout after 30 minutes.`
  - Sem texto parcial: `'Monitor ended after 30 minutes. No text was retrieved.'`
  - Título da resposta: `⏰ Timeout`, Rodapé: `⏱️ ${elapsed}s`
  - Mensagem de progresso: `<b>⏰ ${escapeHtml(modelLabel)} · ${elapsed}s</b>\n\n${buildProgressBody()}`
- **Exceção Geral no Processamento (linha 853)**:
  - Título: `❌ Error`
  - Descrição: `Error occurred during processing: ${e.message}`

---

### 3.4. Textos Literais do Roteador `callback_query:data` e Handlers de Mensagens (`src/bot/index.ts`, linhas 1275–1897)

#### A. Respostas das 11 Famílias de Callbacks (`src/bot/index.ts`, linhas 1275–1674)
1. **`mode_select:<mode>`**:
   - `answerCallbackQuery`: `Mode: ${MODE_DISPLAY_NAMES[selectedMode] || selectedMode}`
2. **`model_exhausted_<modelName>`**:
   - `answerCallbackQuery` (`show_alert: true`): `⛔ ${modelName} is exhausted. Wait for quota reset or pick another model.`
3. **`model_btn_<modelName>`**:
   - Sem CDP: `Not connected to CDP.`
   - Sucesso: `Model: ${res.model}`
   - Falha: `res.error || 'Failed to change model.'`
4. **`model_refresh_btn`**:
   - Sem CDP: `Not connected.`
   - Sucesso: `Refreshed`
5. **`autoaccept_btn_on` / `autoaccept_btn_off` / `autoaccept_btn_refresh`**:
   - ON/OFF: `Auto-accept: ${action.toUpperCase()}` (`Auto-accept: ON` / `Auto-accept: OFF`)
   - Refresh: `Refreshed`
6. **`project_select:<workspacePath>` e `project_page:<page>`**:
   - Projeto inexistente (`!workspaceService.exists(workspacePath)`): `Project "${workspacePath}" not found.`
   - Mensagem HTML de projeto selecionado (enviada no novo tópico ou editada na mensagem atual):
     ```html
     <b>📁 Project Selected</b>

     ✅ <b>{escapeHtml(workspacePath)}</b>
     <code>{escapeHtml(fullPath)}</code>

     Send messages here to interact with this project.
     ```
   - `answerCallbackQuery` com tópico criado: `Topic created for: ${workspacePath}`
   - `answerCallbackQuery` sem tópico: `Selected: ${workspacePath}`
7. **`template_btn_<id>`**:
   - ID inválido (`isNaN`): `Invalid template.`
   - Não encontrado: `Template not found.`
   - Sem CDP: `Not connected.`
   - Sucesso: `Running: ${template.name}`
8. **`session_select:<title>`**:
   - Sem projeto vinculado: `No project bound.`
   - Sucesso (edita a mensagem com HTML):
     ```html
     <b>🔗 Joined Session</b>

     <b>{escapeHtml(selectedTitle)}</b>
     ```
   - Falha na ativação: `Failed: ${activateResult.error}`
   - Exceção: `Error: ${e.message}`
9. **Aprovação (`approve_action`, `always_allow_action`, `deny_action`)**:
   - Detector não encontrado: `Approval detector not found.`
   - Sucesso (`actionLabel` = `'Allow'`, `'Allow Chat'` ou `'Deny'`): remove teclado inline e responde `${actionLabel} executed.`
   - Botão não encontrado no DOM: `Button not found.`
10. **Plano (`planning_open_action`, `planning_proceed_action`, `plan_view_btn`, `plan_proceed_btn`, `plan_edit_btn`, `plan_refresh_btn`, `plan_page`)**:
    - Detector não encontrado: `Planning detector not found.`
    - `open` / `plan_view_btn`: `Opened` (se clicou) ou `Open button not found.`
    - `proceed` / `plan_proceed_btn`: `Proceeding...` (se clicou) ou `Proceed button not found.`
    - `plan_edit_btn`:
      - `answerCallbackQuery`: `Type your edit instructions (or /cancel).`
      - Envia mensagem HTML no canal:
        ```html
        <b>Edit Plan</b>

        Type your plan edit instructions below.
        Send <code>/cancel</code> to cancel.
        ```
    - `plan_refresh_btn`: `Refreshed`
    - `plan_page`: `Page not found.` (se ausente do cache) ou `Page ${page + 1}/${pages.length}`
11. **Popup de Erro (`error_popup_dismiss_action`, `error_popup_copy_debug_action`, `error_popup_retry_action`)**:
    - Detector não encontrado: `Error popup detector not found.`
    - `dismiss`: `Dismissed` ou `Button not found.`
    - `copy_debug`:
      - Envia mensagem HTML se leu o clipboard (truncado em `3800` chars com `'\n(truncated)'`):
        ```html
        <b>Debug Info</b>

        <pre>{escapeHtml(truncated)}</pre>
        ```
      - `answerCallbackQuery`: `'Button not found.'` (se não clicou), `'Copied'` (se leu clipboard) ou `'Could not read clipboard.'`
    - `retry`: `Retrying...` ou `Button not found.`
12. **Cleanup (`cleanup_archive:<days>`, `cleanup_delete:<days>`, `cleanup_cancel`)**:
    - `cleanup_cancel`: edita mensagem para `'Cleanup cancelled.'` e responde callback `'Cancelled'`.
    - `cleanup_archive` / `cleanup_delete` (`action` = `'archived'` ou `'deleted'`):
      - Edita mensagem para: `✅ Cleanup complete — ${processed} session(s) ${action}.`
      - Responde callback: `${processed} session(s) ${action}`

#### B. Mensagens em `message:text`, `message:photo` e `message:voice`
- **Intercepção de Edição de Plano (`planEditPendingChannels`)**:
  - Se `text === '/cancel'`: `'Plan edit cancelled.'`
  - Se texto normal:
    - Prompt enviado ao Antigravity: `` `Please revise the plan based on the following feedback:\n\n${text}` ``
    - Sem CDP: `'Not connected to CDP.'`
    - Antes de despachar: `'Sending plan edit...'`
- **Slash command `/status` via texto (`src/bot/index.ts`, linhas 1733–1742)**:
  ```html
  <b>🔧 Bot Status</b>

  <b>CDP:</b> {activeNames.length > 0 ? '🟢 ' + activeNames.length + ' project(s)' : '⚪ Disconnected'}
  <b>Mode:</b> {escapeHtml(MODE_DISPLAY_NAMES[currentMode] || currentMode)}
  <b>Auto Approve:</b> {bridge.autoAccept.isEnabled() ? '🟢 ON' : '⚪ OFF'}
  ```
- **Mensagem de texto sem projeto vinculado**:
  ```text
  No project is configured for this chat. Use /project to select one.
  ```
- **Falha ao rotear para sessão existente renomeada**:
  ```text
  ⚠️ Could not route to session ({session.displayName}).
  ```
- **Mensagem de foto**:
  - Caption padrão (quando `ctx.message.caption` está vazio): `'Please review the attached images and respond accordingly.'`
  - Sem projeto vinculado: `'No project configured. Use /project first.'`
- **Mensagem de voz**:
  - Sem projeto vinculado: `'No project configured. Use /project first.'`
  - Status inicial: `'🎙️ Transcribing voice message...'`
  - Falha no download: `'❌ Could not download voice message. Please try again.'`
  - Falha na transcrição (vazia): `'❌ Could not transcribe voice message. Please try again or type your prompt.'`
  - Comando transcrito por voz: `` `🎙️ "${transcript}"\n\n${result.message}` ``
  - Prompt transcrito por voz: `` `📝 "${transcript}"` ``

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo Completo de `SendPromptToAntigravity` (`internal/bot/streaming.go`)

1. **Verificação Inicial de Conexão**:
   - Se `!cdp.IsConnected()`, envia embed `❌ Connection Error` com a dica `getAntigravityCdpHint(9223)` e retorna.
2. **Mensagem Inicial de Status**:
   - Obtém `modeName = MODE_UI_NAMES[modeService.GetCurrentMode()]` e `currentModel = cdp.GetCurrentModel() || modelService.GetCurrentModel()`.
   - Envia imediatamente `<b>📡 ${modeName} · ${modelLabel}</b>\n\n<i>Sending...</i>` e guarda o `message_id` em `liveActivityMsgId`.
3. **Injeção de Prompt / Imagens**:
   - Se `len(inboundImages) > 0`:
     - Tenta `cdp.InjectMessageWithImageFiles(prompt, localPaths)`.
     - Se `!injectResult.Ok`, envia embed `🖼️ Attached image fallback` e tenta `cdp.InjectMessage(BuildPromptWithAttachmentUrls(prompt, inboundImages))`.
   - Caso contrário, chama `cdp.InjectMessage(prompt)`.
   - Se `!injectResult.Ok`, marca `isFinalized = true`, envia embed `❌ Message Injection Failed` e retorna.
4. **Inicialização do Progresso e `ResponseMonitor`**:
   - Registra `startTime = time.Now()`.
   - Atualiza a mensagem de progresso com título `🧠 ${modelLabel}`, corpo `'<i>Generating...</i>'` e rodapé `'⏱️ 0s'`.
   - Instancia `ResponseMonitor` com `PollIntervalMs: 2000`, `MaxDurationMs: 1800000`, `StopGoneConfirmCount: 3`.
   - Inicia `monitor.Start()` e o ticker `elapsedTimer` de `5000ms` que invoca `triggerProgressRefresh()`.
5. **Processamento de Eventos de Streaming**:
   - `onProcessLog(logText)`: descarta se `isFinalized` ou `IsJunkEntry(trimmed)`. Formata via `FormatActivityLine` (colapsa quebras de linha, trunca em 120 chars, aplica `EscapeHTML`), adiciona `{ Kind: "activity", Text: formatted }` em `progressLog` (mantendo no máximo 60 itens via FIFO) e chama `triggerProgressRefresh()`.
   - `onThinkingLog(thinkingText)`:
     - Se casar com `/^thinking\.{0,3}$/i`: apenas define `thinkingActive = true`.
     - Se casar com `/^thought for\s/i`: define `thinkingActive = false`, `lastThoughtLabel = trimmed`, e adiciona `{ Kind: "thought", Text: trimmed }` em `progressLog`.
     - Caso contrário (conteúdo do pensamento): armazena em `thinkingContentParts`, extrai a primeira linha truncada em 60 chars (`heading`) e tenta anexar `" — " + heading` à entrada `thought` mais recente que ainda não possua `" — "`. Se não houver entrada `thought` para mesclar e `len(heading) > 10`, adiciona `{ Kind: "thought-content", Text: heading }`.
     - Chama `triggerProgressRefresh()`.
   - `onProgress(text)`: separa logs se `legacy` (`splitOutputAndLogs`) e atualiza `lastProgressText`.
6. **Finalização em `onComplete(finalText, meta)`**:
   - Garante execução única (`if isFinalized { return }; isFinalized = true`) e para `elapsedTimer`.
   - Se o canal estiver em `userStopRequestedChannels`, remove-o do conjunto, envia `'⏹️ Generation stopped.'` e retorna.
   - Se `monitor.GetPhase() == "quotaReached" || monitor.GetQuotaDetected()`: atualiza progresso e resposta com `⚠️ Quota Reached` e envia `BuildModelsUI`.
   - **Re-extração DOM Fresca**: avalia `extractAssistantSegmentsPayloadScript()` + `classifyAssistantSegments`. Escolhe o texto final pela prioridade: `freshText` (se `len(freshText) >= len(bestPolled)`) $>$ `bestPolled` (`finalText` ou `lastProgressText`) $>$ `tryEmergencyExtractText()`.
   - **Bloco de Thinking Expandível**: avalia o script de extração de `<details><summary>` no DOM; se encontrar blocos `thought for` / `thinking`, monta `<blockquote expandable>...</blockquote>` e envia via `sendMsg` antes da resposta final.
   - Atualiza `liveActivityMsgId` para o estado concluído (`<b>✅ ${modelLabel} · ${elapsed}s</b>\n\n${completedBody}`).
   - Envia a resposta final via `sendChunkedResponse`:
     - Se couber em `4096` chars, edita/cria `liveResponseMsgId`.
     - Se exceder `4096` chars, divide com `splitTelegramHtml(formattedBody, 3896)`, envia até `MAX_INLINE_CHUNKS = 5` partes numeradas `(1/N)` (ou `(1/5+file)`) e, se `len(bodyChunks) > 5`, anexa o Markdown completo em `response-<timestamp>.md`.
   - **Auto-Rename de Sessão e Tópico**: consulta `chatSessionService.GetCurrentSessionInfo(cdp)`. Se houver título válido (`!= "(Untitled)"`), registra o canal de aprovação por sessão (`registerApprovalSessionChannel`) e, se `session.DisplayName != sessionInfo.Title`, sanitiza o nome (`${session.SessionNumber}-${sanitized}`), renomeia o tópico do fórum (`topicManager.RenameTopic`) e atualiza o SQLite (`chatSessionRepo.UpdateDisplayName`).
   - **Imagens Geradas**: se o prompt ou a resposta contiver intenção/URL/markdown de imagem, chama `cdp.ExtractLatestResponseImages(4)` e envia cada imagem via `sendPhoto` com caption `🖼️ Generated image (i/N)`.

---

### 4.2. Espelhamento PC $\rightarrow$ Telegram (`UserMessageDetector` + `ResponseMonitor.startPassive()`)

1. Ao resolver um workspace em `ResolveWorkspaceAndCdp(ch)`, o bot garante que `EnsureUserMessageDetector(bridge, cdp, projectName, onUserMessage)` esteja ativo.
2. Quando uma mensagem é enviada pelo próprio bot Telegram (em `HandleTextMessage` ou `HandleVoiceMessage`), o bot calcula e registra `userMsgDetector.AddEchoHash(text)` (SHA-256 dos primeiros 200 caracteres normalizados, com TTL de 60s) antes de injetar no CDP.
3. Quando o usuário digita uma mensagem diretamente no PC:
   - O `UserMessageDetector` detecta a nova bolha de usuário no DOM (`.bg-gray-500/15.select-text .whitespace-pre-wrap`), verifica que não é duplicata (`seenHashes`) nem eco do Telegram (`echoHashes`) e dispara `onUserMessage(info)`.
   - O callback `StartPassiveMirrorForUserMessage`:
     1. Descobre o canal/tópico correspondente à conversa ativa chamando `currentTitle := GetCurrentChatTitle(cdp)` e `targetChannel := ResolveApprovalChannelForCurrentChat(bridge, projectName, currentTitle)`.
     2. Se o workspace não estiver com um `PromptDispatcher` ativo para uma requisição originada do Telegram, envia no `targetChannel`:
        `🖥️ <b>Sent from PC:</b>\n\n${escapeHtml(info.Text)}`
     3. Inicia o monitoramento passivo da resposta do assistente instanciando `ResponseMonitor` e chamando `monitor.StartPassive()` (que inicia diretamente na fase `'generating'` com `generationStarted = true` sem aguardar o aparecimento inicial do botão de stop), reutilizando o mesmo pipeline de progresso, `<blockquote expandable>`, paginação e imagens de `SendPromptToAntigravity`.

---

### 4.3. Recuperação Automática de HTML Inválido em `sendMsg` (`internal/bot/streaming.go`)

Toda chamada de envio de mensagem em `SendPromptToAntigravity` executa até **3 tentativas**:
1. **Tentativa 1**: Envia `text` com `parse_mode: "HTML"`.
2. **Tentativa 2 (Auto-fix)**: Se a API do Telegram rejeitar o HTML, aplica ` fixed := BalanceHTMLTags(text)` e reenvia com `parse_mode: "HTML"`.
3. **Tentativa 3 (Plain text fallback)**: Se ainda falhar, aplica `plain := StripHTML(text)` e envia sem `parse_mode` (`parse_mode: ""`).

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Três Filas Seriais Independentes por Execução de Prompt**:
   - Cada invocação de `SendPromptToAntigravity` cria 3 instâncias de `SerialTaskQueue`:
     - `enqueueGeneral` (`"general"`): serializa embeds de aviso/erro.
     - `enqueueResponse` (`"response"`): serializa `upsertLiveResponse` (criação/edição de `liveResponseMsgId`).
     - `enqueueActivity` (`"activity"`): serializa `refreshProgress` e `setProgressMessage` (criação/edição de `liveActivityMsgId`).
2. **Controle de Versão Monotônico (`liveResponseUpdateVersion` e `liveActivityUpdateVersion`)**:
   - Antes de enfileirar uma atualização de progresso ou de resposta final, o contador de versão correspondente é incrementado.
   - Quando a tarefa entra em execução na fila serial, se `expectedVersion != currentVersion` ou se `skipWhenFinalized && isFinalized`, a edição obsoleta é descartada imediatamente, impedindo que um tick atrasado do `elapsedTimer` sobrescreva a mensagem final de conclusão (`✅`) ou de cota (`⚠️`).
3. **Serialização por Workspace no `PromptDispatcher`**:
   - Dois tópicos vinculados ao mesmo workspace nunca injetam prompts nem fazem polling do DOM simultaneamente (`workspaceLocks` indexado por `"ws:" + wsName`, com fallback para `channelKey`).

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

### 6.1. `internal/bot/streaming_test.go`
- `TestBalanceHTMLTags`:
  - Fecha tags abertas não fechadas na ordem inversa (`"<b><i>hello"` $\rightarrow$ `"<b><i>hello</i></b>"`).
  - Remove tags de fechamento órfãs (`"hello</i></b> world"` $\rightarrow$ `"hello world"`).
  - Preserva tags auto-fechadas (`<br/>`, `<hr>`, `<img>`).
- `TestStripHTMLForFile`:
  - Converte `<pre><code class="language-go">...</code></pre>` em blocos cercados ```` ```go ````, `<b>` em `**`, `<i>` em `*`, `<s>` em `~~`, `<a href="...">` em `[texto](url)`, `<blockquote>` em `> ` e decodifica entidades HTML.
- `TestIsJunkEntryAndFormatActivityLine`:
  - Rejeita strings com `< 5` chars, apenas dígitos (`"12345"`) ou palavra única `< 8` chars (`"Read"`).
  - Colapsa `"Analyzed\npackage.json#L1-75"` em `"Analyzed package.json#L1-75"` e trunca em 120 chars.
- `TestSendMsg_ThreeTierFallback`:
  - Simula falha de HTML na Tentativa 1 $\rightarrow$ sucesso com `BalanceHTMLTags` na Tentativa 2; simula falha nas Tentativas 1 e 2 $\rightarrow$ sucesso com `StripHTML` sem `parse_mode` na Tentativa 3.
- `TestSendChunkedResponse`:
  - Resposta `<= 4096` chars atualiza mensagem única.
  - Resposta entre 2 e 5 chunks envia mensagens numeradas `(1/N)`.
  - Resposta `> 5` chunks envia 5 mensagens `(1/5+file)` e anexa `response-<timestamp>.md` via `sendDocument`.

### 6.2. `internal/bot/callbacks_test.go`
- Testa cada uma das 11 famílias de `callback_data`:
  1. `mode_select:plan` e `mode_select:fast`
  2. `model_exhausted_Claude` (verifica `show_alert: true`)
  3. `model_btn_Claude` e `model_refresh_btn`
  4. `autoaccept_btn_on`, `autoaccept_btn_off`, `autoaccept_btn_refresh`
  5. `project_select:<ws>` (com e sem criação automática de tópico de fórum) e `project_page:<n>`
  6. `template_btn_<id>` (despacho do prompt do template)
  7. `session_select:<title>` (ativação de sessão e edição para `🔗 Joined Session`)
  8. `approve_action`, `always_allow_action`, `deny_action`
  9. `planning_open_action`, `planning_proceed_action`, `plan_view_btn`, `plan_proceed_btn`, `plan_edit_btn`, `plan_refresh_btn`, `plan_page`
  10. `error_popup_dismiss_action`, `error_popup_copy_debug_action` (incluindo leitura de clipboard após 300ms e truncamento em 3800 chars), `error_popup_retry_action`
  11. `cleanup_archive:<days>`, `cleanup_delete:<days>`, `cleanup_cancel`

### 6.3. `internal/bot/handlers_messages_test.go`
- `TestHandleTextMessage_PlanEditInterception`:
  - Quando canal está em `planEditPendingChannels` e recebe `"/cancel"`, remove estado e responde `"Plan edit cancelled."`.
  - Quando recebe feedback, envia `"Sending plan edit..."` e despacha `"Please revise the plan based on the following feedback:\n\n<feedback>"`.
- `TestHandleTextMessage_SlashCommandsAndRouting`:
  - Intercepta `/autoaccept`, `/screenshot`, `/status` e `/template <name>`.
  - Registra `addEchoHash(text)` no `UserMessageDetector` antes de despachar mensagens regulares.
- `TestHandlePhotoMessage`:
  - Seleciona a última foto do array (maior resolução), usa caption padrão quando ausente, baixa imagem, despacha e remove arquivo temporário no `defer`/`finally`.
- `TestHandleVoiceMessage`:
  - Verifica disponibilidade do Whisper, baixa `.ogg`, transcreve, executa slash command se começar com `/` ou envia `📝 "<transcript>"` e despacha ao Antigravity.
