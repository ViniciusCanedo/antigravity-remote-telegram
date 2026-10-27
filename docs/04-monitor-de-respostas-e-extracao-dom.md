# 04 — Monitor de Respostas, Extração Estruturada do DOM e Formatação Telegram

## 1. Visão Geral do Pipeline de Monitoramento e Formatação

Quando um prompt é enviado ao Antigravity (ou quando o `UserMessageDetector` detecta uma mensagem enviada diretamente pelo PC), o sistema monitora a geração da resposta no DOM do painel `.antigravity-agent-side-panel` e formata a saída para o Telegram.

Os arquivos de `src/` que compõem este subsistema são:
1. [`src/services/responseMonitor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/responseMonitor.ts) (1206 linhas): máquina de estados de polling CDP, detecção de início/fim de geração, logs de processo/thinking e detecção de erro de cota.
2. [`src/services/assistantDomExtractor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/assistantDomExtractor.ts) (444 linhas): extração estruturada dos segmentos do último turno do assistente no DOM.
3. [`src/utils/processLogBuffer.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/processLogBuffer.ts) (114 linhas): buffer circular de logs de atividades com classificação por emojis (`pickEmoji`).
4. [`src/utils/streamMessageFormatter.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/streamMessageFormatter.ts) (94 linhas): formatação de cabeçalhos de modo/modelo e filtragem de ruído de logs de atividade (`shouldSkipActivityLog`).
5. [`src/utils/metadataExtractor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/metadataExtractor.ts) (20 linhas): extração de metadados de rodapé (`extractMetadataFromFooter`).
6. [`src/services/progressSender.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/progressSender.ts) (77 linhas): emissor com throttle de `3000ms` para blocos `<pre>`.
7. [`src/utils/htmlToTelegramMarkdown.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/htmlToTelegramMarkdown.ts) (275 linhas): conversor de HTML do DOM para HTML suportado pelo Telegram.
8. [`src/utils/telegramFormatter.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/telegramFormatter.ts) (468 linhas): conversor de Markdown para Telegram HTML, separador `splitOutputAndLogs` e paginador seguro de tags `splitTelegramHtml` (`4096` chars).
9. [`src/tools/doctorSelector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/tools/doctorSelector.ts) (95 linhas): ferramenta de auditoria de saúde dos seletores de `RESPONSE_SELECTORS`.

---

## 2. Máquina de Estados do `ResponseMonitor` (`src/services/responseMonitor.ts`)

### 2.1. Parâmetros e Constantes Reais
| Parâmetro | Valor Padrão | Descrição Exata |
| :--- | :---: | :--- |
| `pollIntervalMs` | `2000` (2s) | Intervalo entre avaliações de `poll()` via `Runtime.evaluate`. |
| `maxDurationMs` | `300000` (5 min) | Tempo máximo total da sessão de monitoramento antes de disparar `onTimeout`. |
| `networkTimeoutMs` | `300000` (`maxDurationMs`) | Timeout de inatividade caso o texto pare de mudar após o início da geração. |
| `stopGoneConfirmCount` | `3` | Número de polls consecutivos (`3 x 2s = 6s`) sem botão Stop ativo (`!isGenerating && generationStarted && !planningActive`) necessários para confirmar conclusão (`complete`). |
| `extractionMode` | `'structured'` | Modo de extração: `'structured'` (usa `AssistantDomExtractor`) ou `'legacy'` (usa `RESPONSE_TEXT`). |
| `consecutiveWsErrors` | Limite `5` | Após `5` erros consecutivos de WebSocket (`WebSocket is not connected`), encerra o monitor e aciona `onTimeout`. |

### 2.2. Fases (`MonitorPhase`) e Modos de Início (`start` vs `startPassive`)
As fases emitidas via `onPhaseChange(phase, text)` são:
`waiting` $\rightarrow$ `thinking` $\rightarrow$ `generating` $\rightarrow$ (`complete` | `timeout` | `quotaReached`).

- **`start()` (Fluxo Ativo — Mensagem enviada pelo Telegram)**:
  1. Captura o baseline inicial do DOM (`baselineText`, `baselineProcessLogKeys`, `baselineThinkingText`) **antes** de iniciar o loop de polling, para não confundir a resposta do turno anterior com a nova resposta.
  2. Inicia na fase `'waiting'` com `generationStarted = false`.
- **`startPassive()` (Fluxo Passivo — Mensagem enviada pelo PC detectada pelo `UserMessageDetector`)**:
  1. Captura o baseline atual, mas já inicia diretamente na fase `'generating'` com `generationStarted = true`, permitindo acompanhar uma geração que já começou na UI desktop.

### 2.3. Seletores DOM (`RESPONSE_SELECTORS`)
- **`STOP_BUTTON`**: verifica se há geração ativa procurando dentro de `.antigravity-agent-side-panel` por `[data-tooltip-id="input-send-button-cancel-tooltip"]` ou botões visíveis com texto/aria-label `'Stop'` ou `'Cancel'`.
- **`CLICK_STOP_BUTTON`**: localiza o botão de parada ativo e executa `.click()`, retornando `{ ok: true }` (usado por `ResponseMonitor.clickStopButton()` e pelo comando `/stop`).
- **`PLANNING_ACTIVE`**: detecta se há um card de plano aguardando ação (botões `Open` e `Proceed` visíveis), pausando a contagem de conclusão (`stopGoneCount = 0`) enquanto o plano estiver pendente.
- **`QUOTA_ERROR` / `EXTRACT_QUOTA_ERROR_SCRIPT`**: detecta mensagens de limite de cota atingido (`Quota exceeded`, `rate limit`, `exhausted`, `You've reached your usage limit`, `upgrade your plan`) em alertas/banners ou na resposta curta do assistente.
- **`PROCESS_LOGS`** e **`RESPONSE_TEXT`**: seletores de fallback do modo `'legacy'`.

### 2.4. Critério Real de Conclusão (`poll()`)
A cada ciclo de `2000ms`:
1. Quando `isGenerating` se torna `true`, ou quando um novo log de atividade/thinking ou novo texto diferente do `baselineText` surge, define `generationStarted = true`.
2. Enquanto `isGenerating` for `true`:
   - Se houver blocos de thinking/logs mas ainda sem corpo de resposta final (`!text`), transiciona para `'thinking'`.
   - Assim que houver corpo de resposta (`text`), transiciona para `'generating'` e invoca `onUpdate(text)` a cada mudança.
   - Emite novos logs de ferramentas via `onProcessLog(newLogText)` e blocos de raciocínio via `onThinkingLog(thinkingText)`.
3. Quando `!isGenerating && this.generationStarted && !planningActive`:
   - Incrementa `this.stopGoneCount++`.
   - Ao atingir `this.stopGoneCount >= this.stopGoneConfirmCount` (`3` polls consecutivos):
     - Verifica `detectQuotaError`. Se detectado, emite fase `'quotaReached'` e conclui.
     - Caso contrário, emite fase `'complete'` e chama `onComplete(lastText, meta)`.

---

## 3. Extração Estruturada do DOM (`src/services/assistantDomExtractor.ts`)

No modo padrão (`extractionMode = 'structured'`), `extractAssistantSegmentsPayloadScript()` inspeciona o último turno do assistente em `.antigravity-agent-side-panel` e classifica os elementos filhos em 6 tipos de segmento (`AssistantSegmentKind`):

| Tipo de Segmento (`kind`) | Como é Identificado no DOM do Antigravity | Tratamento no Bot |
| :--- | :--- | :--- |
| `thinking` | `<details>` ou containers colapsáveis cujo cabeçalho/sumário contenha `Thought for`, `Thinking` ou ícone de raciocínio. | Enviado via callback `onThinkingLog` e renderizado no Telegram dentro de `<blockquote expandable>🧠 ...</blockquote>`. |
| `activity` | Linhas curtas de status de ação (`Analyzing...`, `Reading...`, `Edited...`, `Searched...`, etc.). | Adicionado ao `ProcessLogBuffer` com emoji semântico (`pickEmoji`). |
| `tool-call` | Blocos de invocação de ferramentas/terminal ou cabeçalhos MCP (`server / tool`). | Adicionado ao `ProcessLogBuffer`. |
| `tool-result` | Saídas colapsadas de execução de ferramentas (`output.txt`, `Full output written to...`). | Filtrado do corpo principal e registrado como atividade. |
| `feedback` | Botões de avaliação da UI (`Good`, `Bad`, `Copy`, rodapés de telemetria). | Descartado completamente para não poluir a resposta no Telegram. |
| `body` | Parágrafos, cabeçalhos (`h1`-`h6`), listas (`ul`/`ol`), tabelas (`table`), blocos `<pre><code>` (excluindo barras de cabeçalho do bloco de código com botões Copy/Run) e menções `.context-scope-mention`. | Convertido em Markdown/HTML estruturado (`domOutputText`) e entregue como resposta final ao usuário. |

---

## 4. Orquestração do Streaming no Bot (`sendPromptToAntigravity` em `src/bot/index.ts`)

O fluxo exato executado em `src/bot/index.ts` (linhas 490–855) durante o processamento de um prompt é:

1. **Mensagem Inicial de Status**:
   - Envia imediatamente um card HTML inicial com fase `⏳ Sending...`, modo e modelo atuais (`buildModeModelLines`) e rodapé `⏱️ 0s`.
2. **Injeção antes do Monitor**:
   - Se houver fotos anexadas (`inboundImages.length > 0`), chama `cdp.injectMessageWithImageFiles(prompt, filePaths)`. Se falhar, faz fallback para `cdp.injectMessage(buildPromptWithAttachmentUrls(prompt, inboundImages))`.
   - Se for apenas texto, chama `cdp.injectMessage(prompt)`.
3. **Timer de Tempo Decorrido (`elapsedTimer` — `5000ms`)**:
   - Inicia um `setInterval` a cada **`5000ms`** que atualiza a mensagem de progresso ao vivo no Telegram com os logs acumulados no `ProcessLogBuffer` (`maxChars = 3500`, `maxEntries = 120`, `maxEntryLength = 220`) e o tempo decorrido `⏱️ Xs`.
4. **Callbacks do `ResponseMonitor`**:
   - `onProcessLog`: filtra ruídos via `shouldSkipActivityLog`, adiciona ao `ProcessLogBuffer` (que prefixa cada linha com `pickEmoji`: `🧠` thinking/planning, `📄` reading, `✏️` writing/editing, `▶️` running/executing, `🔍` searching, `🗂️` directory/list, `🌐` web/fetch, `⚙️` default) e agenda atualização da mensagem de status.
   - `onThinkingLog`: armazena o texto de raciocínio mais recente (`latestThinkingText`) e atualiza o status ao vivo.
5. **Conclusão (`onComplete`)**:
   - Para o `elapsedTimer`.
   - Edita a mensagem de progresso finalizando com `✅ Complete` (ou `⛔ Quota Reached` / `⏹️ Stopped`) mantendo o resumo de atividades do `ProcessLogBuffer`.
   - **Bloco de Raciocínio (`thinking`)**: se `latestThinkingText` não for vazio, envia uma **mensagem separada** antes da resposta final contendo:
     ```html
     <blockquote expandable>🧠 <b>Thought Process</b>
     ...texto escapado...</blockquote>
     ```
   - **Resposta Principal (`body`)**:
     - Separa texto final e logs residuais via `separateOutputForDelivery` (`src/utils/telegramFormatter.ts`).
     - Formata o texto para HTML do Telegram (`formatForTelegram` / `htmlToTelegramHtml`) e divide em blocos seguros de até **`4096` caracteres** via `splitTelegramHtml` (que rastreia a pilha de tags abertas `<pre>`, `<code>`, `<b>`, `<i>`, `<blockquote>`, fechando-as no fim de cada chunk e reabrindo-as no início do próximo).
   - **Extração e Envio de Imagens Geradas**:
     - Chama `cdp.extractLatestResponseImages()` e envia cada imagem extraída via `bot.api.sendPhoto` (convertida por `toTelegramInputFile`).
   - **Renomeação Automática da Sessão e do Tópico**:
     - Consulta `chatSessionService.getCurrentSessionInfo(cdp)`. Se a sessão no SQLite ainda não tiver o título atualizado (`session.displayName !== sessionInfo.title`), sanitiza o título via `titleGenerator.sanitizeForChannelName(sessionInfo.title)`, renomeia o Tópico do Fórum para `<sessionNumber>-<newName>` via `topicManager.renameTopic` e atualiza `chatSessionRepo.updateDisplayName`.
   - **Abertura Automática do Seletor de Modelos em Erro de Cota**:
     - Se `isQuotaError` for `true`, chama automaticamente `buildModelsUI(cdp, () => bridge.quota.fetchQuota())` e envia o teclado inline de troca de modelo no canal para que o usuário possa escolher outro modelo imediatamente.

---

## 5. Formatação HTML e Limpeza de Ruído (`htmlToTelegramMarkdown.ts` & `telegramFormatter.ts`)

### 5.1. Conversão de HTML (`htmlToTelegramHtml` em `src/utils/htmlToTelegramMarkdown.ts`)
- Remove `<style>` e `<script>`.
- Converte `<h1..h6>` em `<b>...</b>`.
- Preserva `<pre><code class="language-X">...</code></pre>` e `<code>...</code>`.
- Converte `<strong>`/`<b>`, `<em>`/`<i>`, `<del>`/`<strike>`/`<s>`, `<ins>`/`<u>`, `<mark>` ($\rightarrow$ `<b>`), `<blockquote>`.
- Converte spans `.context-scope-mention` em `<code>...</code>` e elementos com atributo `title` que pareçam caminhos de arquivo (`looksLikeFilePath`) em texto com o caminho completo.
- Converte `<details><summary>` em título em negrito seguido do conteúdo.
- **Tabelas HTML (`<table>`)**: calcula a largura máxima de cada coluna e renderiza uma tabela ASCII alinhada com pipes (`| col1 | col2 |`) dentro de um bloco `<pre>...</pre>`.
- Resolve listas `<ul>` e `<ol>` aninhadas em até 5 iterações.
- Remove qualquer tag HTML residual fora da whitelist do Telegram (`b, i, u, s, code, pre, a, blockquote`) via `stripUnsupportedTags` e decodifica entidades HTML seguras (`decodeSafeEntities`), mantendo `&amp;`, `&lt;` e `&gt;` escapados.

### 5.2. Formatação Markdown e Filtro de UI Chrome (`src/utils/telegramFormatter.ts`)
- `formatForTelegram(text)`:
  - Converte blocos cercados por ```` ```lang ```` em `<pre><code class="language-lang">`.
  - Detecta linhas de tabelas Markdown (`| ... |`) e árvores de diretórios (`├`, `└`, `│`, `──`) envolvendo-as automaticamente em `<pre>...</pre>`.
  - Envolve referências a caminhos de arquivos (`FILE_REF_REGEX`) em `<code>...</code>`.
- `splitOutputAndLogs(rawText)`:
  - No modo legado, separa linhas de interface (`UI_CHROME_LITERALS` como `'thinking'`, `'good bad'`, `'show details'` e `UI_CHROME_REGEXES` como `/^[+-]\d+$/`, `/^thought for\s*<?\d+/i`, `/^full output written to\b/i`) do texto de saída real.

---

## 6. Ferramenta de Diagnóstico de Seletores (`src/tools/doctorSelector.ts`)

O arquivo `src/tools/doctorSelector.ts` (95 linhas) provê um diagnóstico dedicado para validar se uma atualização do Antigravity alterou a estrutura do DOM:
- Conecta via `new CdpService({ portsToScan: [...CDP_PORTS] })`.
- Avalia via `Runtime.evaluate` os 7 seletores de `RESPONSE_SELECTORS`:
  1. `Chat Message (Text)` (`RESPONSE_SELECTORS.RESPONSE_TEXT`)
  2. `Stop Button (State)` (`RESPONSE_SELECTORS.STOP_BUTTON`)
  3. `Planning Active (Dialog)` (`RESPONSE_SELECTORS.PLANNING_ACTIVE`)
  4. `Process Logs (Activity)` (`RESPONSE_SELECTORS.PROCESS_LOGS`)
  5. `Combined Poll (Logic)` (`RESPONSE_SELECTORS.COMBINED_POLL`)
  6. `Quota Error (Detection)` (`RESPONSE_SELECTORS.QUOTA_ERROR`)
  7. `DOM Observer (Event-driven)` (`RESPONSE_SELECTORS.DOM_OBSERVER`)
- Imprime para cada seletor `✅ Healthy`, `⚠️ Warning` (retornou `null`/`undefined`) ou `❌ Broken` e exibe o placar final `Score: X/7 Healthy`.
