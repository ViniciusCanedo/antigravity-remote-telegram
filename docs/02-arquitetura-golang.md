# 02 — Arquitetura Golang e Estrutura de Pacotes

## 1. Visão Geral da Arquitetura em Go

A reescrita para **Go 1.23+** preserva integralmente todas as funcionalidades auditadas nos **68 arquivos** do projeto original em TypeScript (`src/`), substituindo o runtime Node.js e módulos nativos C++ por um **binário estático único** (CGO-free para o banco SQLite via `modernc.org/sqlite`), com concorrência estruturada baseada em `context.Context`, Goroutines, `sync.Mutex` e Channels.

---

## 2. Estrutura de Diretórios e Mapeamento Completo (`src/` $\rightarrow$ Go)

```text
antigravity-telegram-remote-golang/
├── cmd/
│   └── antigravity-bot/
│       └── main.go                       # Entrypoint único (equiv. src/index.ts + src/bin/cli.ts)
├── internal/
│   ├── cli/
│   │   ├── root.go                       # Subcomandos Cobra e flags --verbose / --quiet (src/bin/cli.ts)
│   │   ├── welcome.go                    # Box ASCII de boas-vindas (src/bin/welcome.ts)
│   │   ├── logo.go                       # Constante ASCII LOGO (src/utils/logo.ts)
│   │   ├── start.go                      # Subcomando start (src/bin/commands/start.ts)
│   │   ├── setup.go                      # Wizard interativo em 3 passos (src/bin/commands/setup.ts)
│   │   ├── open.go                       # Abertura do Antigravity com porta livre (src/bin/commands/open.ts)
│   │   ├── doctor.go                     # Diagnóstico de ambiente em 7 passos (src/bin/commands/doctor.ts)
│   │   ├── doctor_selectors.go           # Auditoria dos 7 seletores DOM (src/tools/doctorSelector.ts)
│   │   └── update_check.go               # Verificador de atualizações com cache 24h (src/services/updateCheckService.ts)
│   ├── config/
│   │   ├── config.go                     # Struct AppConfig e modos de extração/entrega (src/utils/config.ts)
│   │   └── loader.go                     # Merge .env + ~/.antigravity-telegram-remote/config.json (src/utils/configLoader.ts)
│   ├── logger/
│   │   └── logger.go                     # Logger ANSI com níveis, [PHASE], [DONE] e divider (src/utils/logger.ts)
│   ├── i18n/
│   │   ├── i18n.go                       # Suporte a en, ja, vi com interpolação {{var}} (src/utils/i18n.ts)
│   │   └── locales/                      # Arquivos JSON embutidos via //go:embed (locales/{en,ja,vi}.json)
│   ├── platform/
│   │   ├── lockfile.go                   # Lock ~/.antigravity-telegram-remote/.bot.lock com SIGTERM/SIGKILL (src/utils/lockfile.ts)
│   │   ├── paths.go                      # getAntigravityCliPath, extractProjectNameFromPath, hints (src/utils/pathUtils.ts)
│   │   ├── launcher.go                   # ensureAntigravityRunning verificando CDP_PORTS (src/services/antigravityLauncher.ts)
│   │   ├── proc_unix.go                  # Descoberta de PID/CSRF e portas do language_server em macOS/Linux
│   │   └── proc_windows.go               # Descoberta de PID/CSRF e portas do language_server no Windows
│   ├── database/
│   │   ├── sqlite.go                     # Abertura de antigravity.db com PRAGMA journal_mode = WAL
│   │   ├── session_repo.go               # ChatSessionRepository (src/database/chatSessionRepository.ts)
│   │   ├── binding_repo.go               # WorkspaceBindingRepository (src/database/workspaceBindingRepository.ts)
│   │   ├── template_repo.go              # TemplateRepository (src/database/templateRepository.ts)
│   │   └── schedule_repo.go              # ScheduleRepository (src/database/scheduleRepository.ts)
│   ├── workspace/
│   │   ├── sanitize.go                   # resolveSafePath com EvalSymlinks (src/middleware/sanitize.ts)
│   │   └── service.go                    # WorkspaceService: scanWorkspaces, validatePath, exists (src/services/workspaceService.ts)
│   ├── cdp/
│   │   ├── ports.go                      # CDP_PORTS = [9222, 9223, 9333, 9444, 9555, 9666] (src/utils/cdpPorts.ts)
│   │   ├── client.go                     # Cliente WebSocket JSON-RPC, contextos V8 e reconexão (src/services/cdpService.ts)
│   │   ├── operations.go                 # Injeção de texto/imagens, modos, modelos e extração de imagens (src/services/cdpService.ts)
│   │   ├── session.go                    # ChatSessionService: startNewChat, listAllSessions, activateSessionByTitle (src/services/chatSessionService.ts)
│   │   ├── screenshot.go                 # ScreenshotService via Page.captureScreenshot (src/services/screenshotService.ts)
│   │   └── pool.go                       # CdpConnectionPool + eventos disconnected/reconnected/reconnectFailed (src/services/cdpConnectionPool.ts)
│   ├── monitor/
│   │   ├── dom_extractor.go              # AssistantDomExtractor estruturado (src/services/assistantDomExtractor.ts)
│   │   ├── response_monitor.go           # ResponseMonitor ativo/passivo + detecção de cota (src/services/responseMonitor.ts)
│   │   ├── process_log_buffer.go         # ProcessLogBuffer e pickEmoji (src/utils/processLogBuffer.ts)
│   │   └── progress_sender.go            # ProgressSender com throttle de 3000ms (src/services/progressSender.ts)
│   ├── bridge/
│   │   ├── manager.go                    # CdpBridgeManager e roteamento de canais/sessões (src/services/cdpBridgeManager.ts)
│   │   ├── dispatcher.go                 # PromptDispatcher com locks por workspace e canal (src/services/promptDispatcher.ts)
│   │   ├── approval_detector.go          # ApprovalDetector (src/services/approvalDetector.ts)
│   │   ├── planning_detector.go          # PlanningDetector + extractPlanContent (src/services/planningDetector.ts)
│   │   ├── error_detector.go             # ErrorPopupDetector + readClipboard (src/services/errorPopupDetector.ts)
│   │   ├── user_message_detector.go      # UserMessageDetector com hash SHA-256 anti-eco (src/services/userMessageDetector.ts)
│   │   ├── auto_accept.go                # AutoAcceptService (src/services/autoAcceptService.ts)
│   │   ├── mode_service.go               # ModeService (src/services/modeService.ts)
│   │   ├── model_service.go              # ModelService (src/services/modelService.ts)
│   │   └── title_generator.go            # TitleGeneratorService (src/services/titleGeneratorService.ts)
│   ├── quota/
│   │   └── service.go                    # QuotaService: cliente HTTPS local GetUserStatus com cache (src/services/quotaService.ts)
│   ├── scheduler/
│   │   └── service.go                    # ScheduleService baseado em robfig/cron/v3 (src/services/scheduleService.ts)
│   ├── formatter/
│   │   ├── html_converter.go             # htmlToTelegramHtml (src/utils/htmlToTelegramMarkdown.ts)
│   │   ├── telegram_formatter.go         # formatForTelegram, splitOutputAndLogs, splitTelegramHtml (src/utils/telegramFormatter.ts)
│   │   ├── stream_formatter.go           # buildModeModelLines, shouldSkipActivityLog, splitForEmbedDescription (src/utils/streamMessageFormatter.ts)
│   │   └── metadata.go                   # extractMetadataFromFooter (src/utils/metadataExtractor.ts)
│   ├── voice/
│   │   └── transcriber.go                # Integração com whisper-cli / ffmpeg (src/utils/voiceHandler.ts)
│   └── bot/
│       ├── bot.go                        # Inicialização do bot, setMyCommands e ciclo de vida (src/bot/index.ts)
│       ├── middleware.go                 # Middleware de autorização por ALLOWED_USER_IDS (src/bot/index.ts + src/middleware/auth.ts)
│       ├── topic_manager.go              # TelegramTopicManager (src/services/telegramTopicManager.ts)
│       ├── message_parser.go             # parseMessageContent com suporte a aspas (src/commands/messageParser.ts)
│       ├── image_handler.go              # Download, conversão e fallback de imagens (src/utils/imageHandler.ts)
│       ├── streaming.go                  # sendPromptToAntigravity: ciclo completo de streaming (src/bot/index.ts)
│       ├── handlers_chat.go              # /start, /help, /new, /chat, /stop, /ping, /status (src/bot/index.ts + src/commands/chatCommandHandler.ts)
│       ├── handlers_project.go           # /project e resolução de workspaces (src/bot/index.ts + src/commands/workspaceCommandHandler.ts)
│       ├── handlers_session.go           # /join e vinculação a sessões existentes (src/commands/joinCommandHandler.ts)
│       ├── handlers_template.go          # /template, /template_add, /template_delete (src/commands/slashCommandHandler.ts)
│       ├── handlers_cleanup.go           # /cleanup [days] (src/commands/cleanupCommandHandler.ts)
│       ├── callbacks.go                  # Roteador completo de callback_query:data (src/bot/index.ts)
│       └── ui/
│           ├── autoaccept_ui.go          # Painel Auto-accept (src/ui/autoAcceptUi.ts)
│           ├── mode_ui.go                # Painel Mode Management (src/ui/modeUi.ts)
│           ├── models_ui.go              # Painel Model Management + Cotas (src/ui/modelsUi.ts)
│           ├── plan_ui.go                # Notificação e paginação de Plan Mode (src/ui/planUi.ts)
│           ├── project_ui.go             # Lista paginada de projetos (src/ui/projectListUi.ts)
│           ├── screenshot_ui.go          # Handler de captura e envio de screenshot (src/ui/screenshotUi.ts)
│           ├── session_ui.go             # Seletor interativo de sessões (src/ui/sessionPickerUi.ts)
│           └── template_ui.go            # Lista e botões de templates (src/ui/templateUi.ts)
```

> [!NOTE]
> O arquivo `src/services/processManager.ts` (78 linhas) do projeto original é um gerenciador legado de subprocessos que não é importado nem chamado por nenhum módulo do fluxo CDP. Conforme decidido no ADR-006, ele é documentado na engenharia reversa (Doc 01 e Doc 05), mas omitido da compilação do binário Go.

---

## 3. Bibliotecas Go Recomendadas

| Domínio | Biblioteca Go | Equivalente no TypeScript (`src/`) |
| :--- | :--- | :--- |
| Bot Telegram | `github.com/go-telegram/bot` | `grammy` |
| WebSocket CDP | `github.com/coder/websocket` | `ws` |
| Banco SQLite (Pure Go) | `modernc.org/sqlite` + `database/sql` | `better-sqlite3` |
| CLI & Subcomandos | `github.com/spf13/cobra` | `commander` |
| Prompts Interativos CLI | `golang.org/x/term` + `bufio.Reader` | `readline` + `stdin.setRawMode(true)` |
| Agendador Cron | `github.com/robfig/cron/v3` | `node-cron` |
| Carregamento `.env` | `github.com/joho/godotenv` | `dotenv` |

---

## 4. Modelo de Concorrência e Sincronização

### 4.1.Cliente CDP (`internal/cdp/client.go`)
- **Correlação Assíncrona de Chamadas JSON-RPC**:
  - `nextID atomic.Int64` incrementa o `id` de cada comando CDP.
  - `pendingCalls map[int64]chan cdpResponse` protegido por `sync.Mutex` armazena o canal de resposta de cada chamada em voo.
  - Uma única Goroutine `readLoop(ctx)` lê frames do WebSocket:
    - Mensagens com campo `id` resolvem o canal em `pendingCalls[id]`.
    - Eventos sem `id` (`Runtime.executionContextCreated`, `Runtime.executionContextDestroyed`, `Runtime.executionContextsCleared`) atualizam a lista `contexts []ExecutionContext` protegida por `sync.RWMutex`.
  - Se o socket fechar inesperadamente (`!intentionalClose`), `readLoop` falha todas as `pendingCalls` abertas e aciona a reconexão automática exponencial (até `maxReconnectAttempts = 3`, delay base `3000ms` $\rightarrow$ `3s, 6s, 12s`).

### 4.2. Pool de Conexões e Eventos de Ciclo de Vida (`internal/cdp/pool.go`)
- O `CdpConnectionPool` mantém:
  - `connections map[string]*Client` (por nome de projeto).
  - `connecting map[string]chan struct{}` (equivalente a `connectingPromises` em `src/services/cdpConnectionPool.ts`, evitando que duas mensagens simultâneas no mesmo projeto abram duas conexões WebSocket duplicadas).
  - Mapas de detectores por projeto: `approvalDetectors`, `planningDetectors`, `errorPopupDetectors` e `userMessageDetectors`.
  - Canal/callbacks de eventos `workspace:disconnected`, `workspace:reconnected` e `workspace:reconnectFailed`, consumidos pelo bot para enviar notificações HTML no último canal ativo (`bridge.lastActiveChannel`).

### 4.3. Serialização de Prompts (`internal/bridge/dispatcher.go`)
- Reproduzindo fielmente `PromptDispatcher` (`src/services/promptDispatcher.ts`):
  - Dois mapas de mutexes por chave: `workspaceLocks` (`ws:<workspaceName>`, primário) e `channelLocks` (`<chatId>:<threadId>` ou `<chatId>`, fallback).
  - Dois tópicos do Telegram vinculados ao mesmo workspace são serializados pelo lock do workspace para nunca fazerem polling ou injeção concorrente no mesmo DOM do Antigravity.

### 4.4. Detectores em Background e Monitor de Resposta
- Cada workspace conectado executa 4 Goroutines leves de polling controladas por `context.WithCancel`:
  1. `ApprovalDetector` (a cada `2000ms` quando registrado via `ensureApprovalDetector`).
  2. `PlanningDetector` (a cada `2000ms`, com cooldown de `5000ms` após clique em `Open`/`Proceed`).
  3. `ErrorPopupDetector` (a cada `3000ms`, com cooldown de `10000ms` após `Dismiss`/`Retry`).
  4. `UserMessageDetector` (a cada `2000ms`, com `isPriming` no primeiro ciclo e filtro anti-eco SHA-256 com TTL de `60s`).
- Quando um prompt é enviado (ou uma mensagem de PC é detectada pelo `UserMessageDetector`), o `ResponseMonitor` roda sua própria Goroutine com `time.NewTicker(2000 * time.Millisecond)` e um segundo ticker em `sendPromptToAntigravity` (`elapsedTimer` de `5000ms`) atualiza o cabeçalho de progresso no Telegram (`⏱️ Xs`).

### 4.5. Graceful Shutdown (`SIGINT` / `SIGTERM`)
1. Cancela o `ctx` raiz criado por `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`.
2. Para o Long Polling do Telegram Bot.
3. Para todos os jobs ativos do `ScheduleService` (`stopAll()`).
4. Para todos os detectores e fecha todas as conexões WebSocket no `CdpConnectionPool` (`disconnectAll()`).
5. Fecha a conexão SQLite (`db.Close()`).
6. Remove `~/.antigravity-telegram-remote/.bot.lock` se o PID gravado pertencer ao processo atual (`releaseLock()`).
