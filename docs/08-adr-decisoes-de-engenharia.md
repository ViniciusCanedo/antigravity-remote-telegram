# 08 — ADRs (Architecture Decision Records)

Este documento registra as decisões arquiteturais para a reescrita do `antigravity-telegram-remote` em **Golang (1.23+)**, distinguindo explicitamente o que é o **comportamento original auditado em `src/` (TypeScript)** do que são **evoluções intencionais do port para Go**.

---

## ADR-001: Abstração Multiplataforma de Descoberta de Processos (`internal/platform`)

- **Status**: Aprovado
- **Estado Real no TypeScript (`src/`)**:
  - Em [`src/services/quotaService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/quotaService.ts), a descoberta do processo `language_server` e de suas portas TCP utiliza exclusivamente `pgrep -fl language_server` e `lsof -nP -a -iTCP -sTCP:LISTEN -p ${pid}` (voltado para macOS).
  - Em [`src/bin/commands/doctor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bin/commands/doctor.ts), as checagens de sistema operacional rodam apenas dentro de `if (platform === 'darwin')` (`xcode-select -p` e `/Applications/Antigravity.app`).
- **Decisão para o Port Go**:
  - Isolar todas as operações dependentes de SO no pacote `internal/platform` usando build tags (`//go:build darwin || linux` e `//go:build windows`):
    - **macOS (`darwin`)**: manter `pgrep -fl language_server` e `lsof -nP -a -iTCP -sTCP:LISTEN -p <pid>`.
    - **Linux (`linux`)**: suportar leitura direta de `/proc/<pid>/cmdline` (ou `pgrep -fa language_server`) e `ss -tlnp` (com fallback para `lsof`).
    - **Windows (`windows`)**: suportar `Get-CimInstance Win32_Process` / `wmic` para extrair `--csrf_token` e `netstat -ano` para listar portas TCP do PID.
  - No comando `doctor` em Go, validar o executável do Antigravity nos 3 sistemas operacionais (`darwin`, `linux`, `windows`) usando as mesmas regras de resolução de `getAntigravityCliPath()` (`src/utils/pathUtils.ts`).

---

## ADR-002: Driver SQLite Pure-Go (`modernc.org/sqlite`) em Modo WAL

- **Status**: Aprovado
- **Estado Real no TypeScript (`src/`)**:
  - O projeto original usa `better-sqlite3` (addon nativo C++ que exige compilador C/C++ e `xcode-select` no macOS), abrindo `~/.antigravity-telegram-remote/antigravity.db` com `db.pragma('journal_mode = WAL')` em [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts#L865-L870), e cada repositório em `src/database/` executa seu próprio `CREATE TABLE IF NOT EXISTS`.
- **Decisão para o Port Go**:
  - Utilizar `modernc.org/sqlite` (100% Go, sem `CGO_ENABLED`), preservando o caminho padrão `~/.antigravity-telegram-remote/antigravity.db`, `PRAGMA journal_mode = WAL`, e mantendo exatamente o mesmo esquema das 4 tabelas (`chat_sessions`, `workspace_bindings`, `templates`, `schedules`) e a semântica exata das queries SQL (incluindo `WHERE category_id = ?` em `getNextSessionNumber` e `ON CONFLICT(channel_id) DO UPDATE` em `workspace_bindings`).

---

## ADR-003: CLI Unificada, Banners e Verificação de Atualizações (`internal/cli`)

- **Status**: Aprovado
- **Estado Real no TypeScript (`src/`)**:
  - Possui dois entrypoints (`src/index.ts` e `src/bin/cli.ts`), exibe `printWelcome()` (`src/bin/welcome.ts`) quando executado sem configuração, imprime `LOGO` (`src/utils/logo.ts`), adquire `~/.antigravity-telegram-remote/.bot.lock` matando instâncias anteriores via `SIGTERM`/`SIGKILL` (`src/utils/lockfile.ts`), verifica portas ativas em `ensureAntigravityRunning()` (`src/services/antigravityLauncher.ts`) e consulta atualizações com cache de 24h em `~/.antigravity-telegram-remote/update-check.json` (`src/services/updateCheckService.ts`).
- **Decisão para o Port Go**:
  - Unificar o entrypoint em `cmd/antigravity-bot/main.go` usando `spf13/cobra`.
  - Preservar `printWelcome()`, `LOGO`, o comportamento exato do `.bot.lock` (`SIGTERM` $\rightarrow$ espera de 5s $\rightarrow$ `SIGKILL`), `ensureAntigravityRunning()` e o verificador não-bloqueante de atualizações com cache de 24h em `~/.antigravity-telegram-remote/update-check.json`.
  - Incluir o diagnóstico de seletores DOM de [`src/tools/doctorSelector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/tools/doctorSelector.ts) como parte da CLI (`antigravity-telegram-remote doctor --selectors` ou subcomando dedicado).

---

## ADR-004: Transcrição de Voz Local via `whisper.cpp` (`internal/voice`)

- **Status**: Aprovado
- **Estado Real no TypeScript (`src/`)**:
  - [`src/utils/voiceHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/voiceHandler.ts) realiza transcrição **exclusivamente local** invocando `nodejs-whisper` (`whisper-cli` compilado do `whisper.cpp` com o modelo `base.en`) e `ffmpeg-static` para converter `.ogg` em `.wav`, removendo marcações de tempo `[00:00:00.000 --> ...]`.
- **Decisão para o Port Go**:
  - Manter a transcrição local invocando diretamente os binários `ffmpeg` e `whisper-cli` (com modelo `base.en` configurável ou localizado nos diretórios padrão), preservando `checkWhisperAvailability()`, o download temporário em `/tmp/antigravity-telegram-remote-voice/`, a remoção de timestamps por regex e a execução de slash commands transcritos por voz.

---

## ADR-005: Exposição do Seletor Interativo de Sessões em `/chat` e `/join`

- **Status**: Aprovado
- **Estado Real no TypeScript (`src/`)**:
  - [`src/services/chatSessionService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/chatSessionService.ts) implementa `listAllSessions(cdp)` (que abre o modal "Past Conversations", coleta até 20 sessões do projeto atual acima de `"Other Conversations"` e fecha com `Escape`) e `activateSessionByTitle(cdp, title)`.
  - [`src/ui/sessionPickerUi.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/ui/sessionPickerUi.ts) implementa `buildSessionPickerUI(sessions)` gerando botões `session_select:<title.slice(0, 49)>`, [`src/commands/joinCommandHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/commands/joinCommandHandler.ts) implementa `JoinCommandHandler`, e [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts#L1437-L1455) trata o callback `session_select:<title>`.
  - Contudo, no TypeScript original, o comando `/chat` apenas exibe o texto das sessões salvas no SQLite (`chatSessionRepo.findByCategoryId`) e nunca chama `buildSessionPickerUI`, nem há um comando `/join` registrado em `bot.command(...)`.
- **Decisão para o Port Go**:
  - Conectar o fluxo completo no port Go: além de exibir as informações da sessão atual e do SQLite em `/chat`, consultar `chatSessionService.listAllSessions(cdp)` e anexar o teclado interativo `buildSessionPickerUI` (`session_select:<title>`) tanto no comando `/chat` quanto no comando `/join`, permitindo ao usuário alternar entre conversas passadas do Antigravity diretamente pelo Telegram.

---

## ADR-006: Descarte de Código Morto Legado (`processManager.ts` e `middleware/auth.ts`)

- **Status**: Aprovado
- **Estado Real no TypeScript (`src/`)**:
  - [`src/services/processManager.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/processManager.ts) (78 linhas) é uma fila de processos filhos (`child_process.spawn`) remanescente de uma versão anterior que não usava CDP; não é importada por nenhum arquivo de `src/`.
  - [`src/middleware/auth.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/middleware/auth.ts) (9 linhas) está marcado com `@deprecated Not used by the bot (grammy middleware handles auth)`.
- **Decisão para o Port Go**:
  - Documentar ambos no inventário de engenharia reversa para rastreabilidade de 100% dos 68 arquivos de `src/`, mas não portar `ProcessManager` para o binário Go e manter a autenticação diretamente no middleware do bot (`internal/bot/middleware.go`).
