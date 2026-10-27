# Documentação de Engenharia Reversa e Arquitetura Go — `antigravity-telegram-remote`

Bem-vindo à documentação oficial de engenharia reversa, especificação técnica e arquitetura para a reescrita completa do projeto **Antigravity Telegram Remote** (originalmente em TypeScript/Node.js) para **Golang (1.23+)**.

Esta documentação foi auditada linha a linha contra **todos os 68 arquivos reais** (`13.876` linhas no total) do diretório `src/`, sem deduções ou inferências, garantindo fidelidade de 100% ao código-fonte original e separando explicitamente o comportamento real do TypeScript das evoluções arquiteturais planejadas para o binário em Go.

---

## Índice de Documentos

1. **[01 — Engenharia Reversa e Visão Geral](./01-engenharia-reversa-visao-geral.md)**
   Inventário completo dos **68 arquivos** do código-fonte TypeScript (`src/`), dependências reais, arquitetura de alto nível e fluxo de inicialização passo a passo.
2. **[02 — Arquitetura Golang e Estrutura de Pacotes](./02-arquitetura-golang.md)**
   Layout de diretórios idiomático em Go (`cmd/`, `internal/`), bibliotecas recomendadas, modelo de concorrência (Goroutines, Channels, `context.Context`, `sync.Mutex`) e ciclo de vida da aplicação.
3. **[03 — Protocolo CDP, Conexão e Seletores DOM](./03-protocolo-cdp-e-seletores-dom.md)**
   Especificação exata do cliente Chrome DevTools Protocol (`CdpService`, `CdpConnectionPool`, `ChatSessionService`), descoberta de portas (`9222, 9223, 9333, 9444, 9555, 9666`), gerenciamento de contextos V8, injeção de prompts/imagens, troca de modo/modelo e manipulação de sessões via `Input.dispatchMouseEvent`.
4. **[04 — Monitor de Respostas e Extração Estruturada do DOM](./04-monitor-de-respostas-e-extracao-dom.md)**
   Máquina de estados do `ResponseMonitor` (`start`, `startPassive`, `stopGoneConfirmCount = 3`, `consecutiveWsErrors >= 5`), extração estruturada via `AssistantDomExtractor` (`activity`, `thinking`, `body`, `tool-call`, `tool-result`, `feedback`), `ProcessLogBuffer`, `streamMessageFormatter`, `metadataExtractor`, e formatação/chunking para HTML do Telegram (`htmlToTelegramMarkdown`, `telegramFormatter`).
5. **[05 — Detectores em Background e Serviços Auxiliares](./05-detectores-e-servicos-background.md)**
   Detalhamento completo dos 4 detectores por workspace (`ApprovalDetector`, `PlanningDetector`, `ErrorPopupDetector`, `UserMessageDetector`), `CdpBridgeManager`, `QuotaService` (`language_server` gRPC/Connect HTTPS local), `ScheduleService` (`node-cron`), `ScreenshotService`, `TitleGeneratorService`, `AntigravityLauncher`, `UpdateCheckService` e o módulo legado `ProcessManager`.
6. **[06 — Banco de Dados SQLite, Configuração, Utilitários e CLI](./06-banco-de-dados-config-e-cli.md)**
   Esquema SQLite real (`~/.antigravity-telegram-remote/antigravity.db` em modo WAL com as 4 tabelas `chat_sessions`, `workspace_bindings`, `templates`, `schedules`), carregamento de configuração (`config.ts`, `configLoader.ts`), `lockfile.ts` (`.bot.lock` com encerramento automático de instância anterior), `i18n.ts` (`en`, `ja`, `vi`), `voiceHandler.ts` (`nodejs-whisper` / `whisper.cpp` local), ferramenta `tools/doctorSelector.ts` e subcomandos da CLI (`start`, `setup`, `open`, `doctor`, `welcome.ts`).
7. **[07 — Bot do Telegram, Comandos, Callbacks e UI](./07-bot-telegram-comandos-e-ui.md)**
   Mapeamento exato dos 16 comandos registrados em `src/bot/index.ts` (e 15 no menu `setMyCommands`), parser de comandos em texto/voz (`messageParser.ts`, `slashCommandHandler.ts`), tabela completa de `callback_data` reais, teclados inline (`src/ui/*.ts`), `TelegramTopicManager` e roteamento de tópicos de fórum (`<chatId>:<threadId>`).
8. **[08 — ADRs (Architecture Decision Records)](./08-adr-decisoes-de-engenharia.md)**
   Registro das decisões de engenharia para o port em Go, distinguindo claramente o comportamento original em TypeScript (`src/`) das evoluções intencionais da arquitetura Go (suporte multiplataforma em `internal/platform`, exposição do seletor interativo `buildSessionPickerUI` em `/chat` e `/join`, SQLite pure-Go, etc.).

---

## Matriz de Rastreabilidade Completa (68 Arquivos `src/` $\rightarrow$ Go)

A tabela abaixo lista **todos os 68 arquivos reais** existentes em `src/` (`13.876` linhas no total), seu destino na arquitetura Go e o documento onde cada um é especificado:

| # | Arquivo Original (`src/`) | Linhas | Pacote / Arquivo Go Correspondente | Doc de Referência |
| :- | :--- | :---: | :--- | :--- |
| **1** | **Entrypoint Raiz (`src/`)** | | | |
| 1 | `src/index.ts` | 11 | `cmd/antigravity-bot/main.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| **2** | **CLI e Binários (`src/bin/`)** | | | |
| 2 | `src/bin/cli.ts` | 54 | `internal/cli/root.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 3 | `src/bin/welcome.ts` | 53 | `internal/cli/welcome.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 4 | `src/bin/commands/doctor.ts` | 178 | `internal/cli/doctor.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 5 | `src/bin/commands/open.ts` | 157 | `internal/cli/open.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 6 | `src/bin/commands/setup.ts` | 281 | `internal/cli/setup.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 7 | `src/bin/commands/start.ts` | 42 | `internal/cli/start.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| **3** | **Bot Principal (`src/bot/`)** | | | |
| 8 | `src/bot/index.ts` | 1938 | `internal/bot/bot.go`, `handlers_*.go`, `callbacks.go`, `streaming.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| **4** | **Comandos (`src/commands/`)** | | | |
| 9 | `src/commands/chatCommandHandler.ts` | 40 | `internal/bot/handlers_chat.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 10 | `src/commands/cleanupCommandHandler.ts` | 53 | `internal/bot/handlers_cleanup.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 11 | `src/commands/joinCommandHandler.ts` | 36 | `internal/bot/handlers_session.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 12 | `src/commands/messageParser.ts` | 37 | `internal/bot/message_parser.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 13 | `src/commands/slashCommandHandler.ts` | 123 | `internal/bot/handlers_template.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 14 | `src/commands/workspaceCommandHandler.ts` | 32 | `internal/bot/handlers_project.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| **5** | **Banco de Dados SQLite (`src/database/`)** | | | |
| 15 | `src/database/chatSessionRepository.ts` | 164 | `internal/database/session_repo.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 16 | `src/database/scheduleRepository.ts` | 169 | `internal/database/schedule_repo.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 17 | `src/database/templateRepository.ts` | 131 | `internal/database/template_repo.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 18 | `src/database/workspaceBindingRepository.ts` | 137 | `internal/database/binding_repo.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| **6** | **Middleware (`src/middleware/`)** | | | |
| 19 | `src/middleware/auth.ts` | 9 | `internal/bot/middleware.go` | [Doc 02](./02-arquitetura-golang.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 20 | `src/middleware/sanitize.ts` | 34 | `internal/workspace/sanitize.go` | [Doc 02](./02-arquitetura-golang.md), [Doc 05](./05-detectores-e-servicos-background.md) |
| **7** | **Serviços Core e CDP (`src/services/`)** | | | |
| 21 | `src/services/antigravityLauncher.ts` | 60 | `internal/platform/launcher.go` | [Doc 05](./05-detectores-e-servicos-background.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 22 | `src/services/approvalDetector.ts` | 428 | `internal/bridge/approval_detector.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 23 | `src/services/assistantDomExtractor.ts` | 444 | `internal/monitor/dom_extractor.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 24 | `src/services/autoAcceptService.ts` | 94 | `internal/bridge/auto_accept.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 25 | `src/services/cdpBridgeManager.ts` | 439 | `internal/bridge/manager.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 26 | `src/services/cdpConnectionPool.ts` | 283 | `internal/cdp/pool.go` | [Doc 03](./03-protocolo-cdp-e-seletores-dom.md) |
| 27 | `src/services/cdpService.ts` | 1712 | `internal/cdp/client.go`, `operations.go` | [Doc 03](./03-protocolo-cdp-e-seletores-dom.md) |
| 28 | `src/services/chatSessionService.ts` | 820 | `internal/cdp/session.go` | [Doc 03](./03-protocolo-cdp-e-seletores-dom.md) |
| 29 | `src/services/errorPopupDetector.ts` | 303 | `internal/bridge/error_detector.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 30 | `src/services/modeService.ts` | 94 | `internal/bridge/mode_service.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 31 | `src/services/modelService.ts` | 78 | `internal/bridge/model_service.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 32 | `src/services/planningDetector.ts` | 353 | `internal/bridge/planning_detector.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 33 | `src/services/processManager.ts` | 78 | *(Legado não utilizado no fluxo CDP — omitido no binário Go)* | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 05](./05-detectores-e-servicos-background.md), [Doc 08](./08-adr-decisoes-de-engenharia.md) |
| 34 | `src/services/progressSender.ts` | 77 | `internal/monitor/progress_sender.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 35 | `src/services/promptDispatcher.ts` | 91 | `internal/bridge/dispatcher.go` | [Doc 02](./02-arquitetura-golang.md), [Doc 05](./05-detectores-e-servicos-background.md) |
| 36 | `src/services/quotaService.ts` | 175 | `internal/quota/service.go`, `internal/platform/proc_*.go` | [Doc 05](./05-detectores-e-servicos-background.md), [Doc 08](./08-adr-decisoes-de-engenharia.md) |
| 37 | `src/services/responseMonitor.ts` | 1206 | `internal/monitor/response_monitor.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 38 | `src/services/scheduleService.ts` | 127 | `internal/scheduler/service.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 39 | `src/services/screenshotService.ts` | 128 | `internal/cdp/screenshot.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 40 | `src/services/telegramTopicManager.ts` | 112 | `internal/bot/topic_manager.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 41 | `src/services/titleGeneratorService.ts` | 119 | `internal/bridge/title_generator.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 42 | `src/services/updateCheckService.ts` | 118 | `internal/cli/update_check.go` | [Doc 05](./05-detectores-e-servicos-background.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 43 | `src/services/userMessageDetector.ts` | 249 | `internal/bridge/user_message_detector.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| 44 | `src/services/workspaceService.ts` | 80 | `internal/workspace/service.go` | [Doc 05](./05-detectores-e-servicos-background.md) |
| **8** | **Ferramentas de Diagnóstico (`src/tools/`)** | | | |
| 45 | `src/tools/doctorSelector.ts` | 95 | `internal/cli/doctor_selectors.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| **9** | **Interfaces Visuais / Teclados Inline (`src/ui/`)** | | | |
| 46 | `src/ui/autoAcceptUi.ts` | 27 | `internal/bot/ui/autoaccept_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 47 | `src/ui/modeUi.ts` | 65 | `internal/bot/ui/mode_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 48 | `src/ui/modelsUi.ts` | 120 | `internal/bot/ui/models_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 49 | `src/ui/planUi.ts` | 177 | `internal/bot/ui/plan_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 50 | `src/ui/projectListUi.ts` | 71 | `internal/bot/ui/project_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md), [Doc 08](./08-adr-decisoes-de-engenharia.md) |
| 51 | `src/ui/screenshotUi.ts` | 29 | `internal/bot/ui/screenshot_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 52 | `src/ui/sessionPickerUi.ts` | 37 | `internal/bot/ui/session_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md), [Doc 08](./08-adr-decisoes-de-engenharia.md) |
| 53 | `src/ui/templateUi.ts` | 55 | `internal/bot/ui/template_ui.go` | [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| **10** | **Utilitários (`src/utils/`)** | | | |
| 54 | `src/utils/cdpPorts.ts` | 2 | `internal/cdp/ports.go` | [Doc 03](./03-protocolo-cdp-e-seletores-dom.md) |
| 55 | `src/utils/config.ts` | 24 | `internal/config/config.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 56 | `src/utils/configLoader.ts` | 203 | `internal/config/loader.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 57 | `src/utils/htmlToTelegramMarkdown.ts` | 275 | `internal/formatter/html_converter.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 58 | `src/utils/i18n.ts` | 54 | `internal/i18n/i18n.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 59 | `src/utils/imageHandler.ts` | 141 | `internal/bot/image_handler.go` | [Doc 03](./03-protocolo-cdp-e-seletores-dom.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
| 60 | `src/utils/lockfile.ts` | 115 | `internal/platform/lockfile.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 61 | `src/utils/logger.ts` | 99 | `internal/logger/logger.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 62 | `src/utils/logo.ts` | 9 | `internal/cli/logo.go` | [Doc 01](./01-engenharia-reversa-visao-geral.md), [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 63 | `src/utils/metadataExtractor.ts` | 20 | `internal/formatter/metadata.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 64 | `src/utils/pathUtils.ts` | 70 | `internal/platform/paths.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md) |
| 65 | `src/utils/processLogBuffer.ts` | 114 | `internal/monitor/process_log_buffer.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 66 | `src/utils/streamMessageFormatter.ts` | 94 | `internal/formatter/stream_formatter.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 67 | `src/utils/telegramFormatter.ts` | 468 | `internal/formatter/telegram_formatter.go` | [Doc 04](./04-monitor-de-respostas-e-extracao-dom.md) |
| 68 | `src/utils/voiceHandler.ts` | 165 | `internal/voice/transcriber.go` | [Doc 06](./06-banco-de-dados-config-e-cli.md), [Doc 07](./07-bot-telegram-comandos-e-ui.md) |
