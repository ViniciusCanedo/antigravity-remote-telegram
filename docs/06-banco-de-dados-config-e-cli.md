# 06 — Banco de Dados SQLite, Configuração, Utilitários e CLI

## 1. Persistência SQLite (`src/database/`)

### 1.1. Inicialização e Arquivo de Banco de Dados
No código original (`src/bot/index.ts`, linhas 865–870, e `src/utils/configLoader.ts`, linha 28):
- O caminho padrão do arquivo SQLite é **`~/.antigravity-telegram-remote/antigravity.db`** (`DEFAULT_DB_NAME = 'antigravity.db'`, ou `:memory:` quando `NODE_ENV === 'test'`).
- A conexão é aberta diretamente em `startBot` com `new Database(dbPath)` e configurada imediatamente com:
  ```sql
  PRAGMA journal_mode = WAL;
  ```
- Não existem arquivos `src/database/index.ts`, `src/database/schema.ts` ou `localUsageRepository.ts` em `src/database/`: cada um dos **4 repositórios** executa seu próprio `CREATE TABLE IF NOT EXISTS` no método `initialize()` chamado pelo construtor e prepara seus `Database.Statement` em cache.

---

### 1.2. Esquema Exato e Queries dos 4 Repositórios

#### A. `ChatSessionRepository` (`src/database/chatSessionRepository.ts` — 164 linhas)
```sql
CREATE TABLE IF NOT EXISTS chat_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL UNIQUE,
    category_id TEXT NOT NULL,
    workspace_path TEXT NOT NULL,
    session_number INTEGER NOT NULL,
    display_name TEXT,
    is_renamed INTEGER NOT NULL DEFAULT 0,
    guild_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```
- **Prepared Statements e Métodos**:
  - `create(input)`: `INSERT INTO chat_sessions (channel_id, category_id, workspace_path, session_number, guild_id) VALUES (?, ?, ?, ?, ?)`
  - `findByChannelId(channelId)`: `SELECT * FROM chat_sessions WHERE channel_id = ?`
  - `findByCategoryId(categoryId)`: `SELECT * FROM chat_sessions WHERE category_id = ? ORDER BY session_number ASC`
  - `getNextSessionNumber(categoryId)`: `SELECT MAX(session_number) as max_num FROM chat_sessions WHERE category_id = ?` (retorna `(row?.max_num ?? 0) + 1`)
  - `updateDisplayName(channelId, displayName)`: `UPDATE chat_sessions SET display_name = ?, is_renamed = 1 WHERE channel_id = ?`
  - `findByDisplayName(workspacePath, displayName)`: `SELECT * FROM chat_sessions WHERE workspace_path = ? AND display_name = ? ORDER BY id DESC LIMIT 1`
  - `deleteByChannelId(channelId)`: `DELETE FROM chat_sessions WHERE channel_id = ?`

#### B. `WorkspaceBindingRepository` (`src/database/workspaceBindingRepository.ts` — 137 linhas)
```sql
CREATE TABLE IF NOT EXISTS workspace_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL UNIQUE,
    workspace_path TEXT NOT NULL,
    guild_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```
- **Prepared Statements e Métodos**:
  - `create(input)`: `INSERT INTO workspace_bindings (channel_id, workspace_path, guild_id) VALUES (?, ?, ?)`
  - `findByChannelId(channelId)`: `SELECT * FROM workspace_bindings WHERE channel_id = ?`
  - `findByWorkspacePathAndGuildId(workspacePath, guildId)`: `SELECT * FROM workspace_bindings WHERE workspace_path = ? AND guild_id = ? ORDER BY id ASC`
  - `findByGuildId(guildId)`: `SELECT * FROM workspace_bindings WHERE guild_id = ? ORDER BY id ASC`
  - `findAll()`: `SELECT * FROM workspace_bindings ORDER BY id ASC`
  - `deleteByChannelId(channelId)`: `DELETE FROM workspace_bindings WHERE channel_id = ?`
  - `upsert(input)`:
    ```sql
    INSERT INTO workspace_bindings (channel_id, workspace_path, guild_id)
    VALUES (?, ?, ?)
    ON CONFLICT(channel_id) DO UPDATE SET
        workspace_path = excluded.workspace_path,
        guild_id = excluded.guild_id
    ```

#### C. `TemplateRepository` (`src/database/templateRepository.ts` — 131 linhas)
```sql
CREATE TABLE IF NOT EXISTS templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    prompt TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```
- **Prepared Statements e Métodos**:
  - `create({ name, prompt })`: `INSERT INTO templates (name, prompt) VALUES (?, ?)`
  - `findAll()`: `SELECT * FROM templates ORDER BY id ASC`
  - `findById(id)`: `SELECT * FROM templates WHERE id = ?`
  - `findByName(name)`: `SELECT * FROM templates WHERE name = ?`
  - `deleteByName(name)`: `DELETE FROM templates WHERE name = ?`
  - `updateByName(name, { prompt })`: `UPDATE templates SET prompt = ? WHERE name = ?`

#### D. `ScheduleRepository` (`src/database/scheduleRepository.ts` — 169 linhas)
```sql
CREATE TABLE IF NOT EXISTS schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cron_expression TEXT NOT NULL,
    prompt TEXT NOT NULL,
    workspace_path TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```
- **Prepared Statements e Métodos**:
  - `create(input)`: `INSERT INTO schedules (cron_expression, prompt, workspace_path, enabled) VALUES (?, ?, ?, ?)`
  - `findAll()`: `SELECT * FROM schedules ORDER BY id ASC`
  - `findById(id)`: `SELECT * FROM schedules WHERE id = ?`
  - `findEnabled()`: `SELECT * FROM schedules WHERE enabled = 1 ORDER BY id ASC`
  - `delete(id)`: `DELETE FROM schedules WHERE id = ?`
  - `update(id, input)`: atualiza dinamicamente os campos informados (`cron_expression`, `prompt`, `workspace_path`, `enabled`).

---

## 2. Configuração (`src/utils/config.ts` & `src/utils/configLoader.ts`)

### 2.1. Estrutura `AppConfig` e Precedência
`ConfigLoader.load()` mescla configurações na ordem de precedência **`process.env` (incluindo `.env` carregado via `dotenv`) $>$ `~/.antigravity-telegram-remote/config.json` $>$ Valores Padrão**:

| Campo em `AppConfig` | Variável `.env` | Chave em `config.json` | Valor Padrão | Validação / Regra |
| :--- | :--- | :--- | :--- | :--- |
| `telegramBotToken` | `TELEGRAM_BOT_TOKEN` | `telegramBotToken` | *(Obrigatório)* | Lança erro se ausente. |
| `allowedUserIds` | `ALLOWED_USER_IDS` | `allowedUserIds` | *(Obrigatório)* | Filtra apenas strings numéricas (`/^\d+$/`); lança erro se lista resultante for vazia. |
| `workspaceBaseDir` | `WORKSPACE_BASE_DIR` | `workspaceBaseDir` | `~/Code` | Expande `~` / `~/` para `os.homedir()`. |
| `autoApproveFileEdits` | `AUTO_APPROVE_FILE_EDITS` | `autoApproveFileEdits` | `false` | Booleano (`'true'` case-insensitive). |
| `logLevel` | `LOG_LEVEL` | `logLevel` | `'info'` | `'debug' \| 'info' \| 'warn' \| 'error' \| 'none'`. |
| `extractionMode` | `EXTRACTION_MODE` | `extractionMode` | `'structured'` | `'legacy'` ou `'structured'`. |
| `useTopics` | `USE_TOPICS` | `useTopics` | `true` | Booleano (`true` cria tópicos em supergrupos com Fórum). |

Além disso, `src/utils/config.ts` exporta `resolveResponseDeliveryMode(): ResponseDeliveryMode`, que retorna `'stream'`.

---

## 3. Utilitários de Sistema, Áudio e Internacionalização (`src/utils/`)

### 3.1. Lockfile de Instância Única (`src/utils/lockfile.ts` — 115 linhas)
- Arquivo de lock: **`~/.antigravity-telegram-remote/.bot.lock`**.
- Ao chamar `acquireLock()`:
  1. Se `.bot.lock` existir, lê o `existingPid`.
  2. Se `existingPid !== process.pid` e o processo estiver ativo (`process.kill(existingPid, 0)`):
     - Chama `killExistingProcess(existingPid)`: envia `SIGTERM`, aguarda até **`5000ms`** verificando a cada `50ms` (`Atomics.wait`) se o processo encerrou e, caso persista após 5s, força `SIGKILL`.
  3. Remove o `.bot.lock` antigo, grava `String(process.pid)` e registra cleanup automático em `exit`, `SIGINT`, `SIGTERM` e `uncaughtException`.

### 3.2. Transcrição de Voz Local (`src/utils/voiceHandler.ts` — 165 linhas)
O suporte a mensagens de voz no código original é **100% local**, baseado em `nodejs-whisper` (`whisper.cpp`) e `ffmpeg-static`:
1. **`checkWhisperAvailability()`**:
   - Verifica se o pacote `nodejs-whisper` está instalado (`require.resolve('nodejs-whisper')`).
   - Verifica se o binário compilado `node_modules/nodejs-whisper/cpp/whisper.cpp/build/bin/whisper-cli` existe.
   - Verifica se o modelo `base.en` existe em `node_modules/nodejs-whisper/models/` (`files.some(f => f.includes('base.en'))`).
   - Se qualquer etapa falhar, retorna instruções passo a passo de instalação/compilação para o usuário no Telegram.
2. **`downloadTelegramVoice(botApi, botToken, voice)`**:
   - Obtém `file_path` via `botApi.getFile(voice.file_id)`, baixa o arquivo `.ogg` de `https://api.telegram.org/file/bot<token>/<file_path>` e salva em `/tmp/antigravity-telegram-remote-voice/<timestamp>-<file_unique_id>.ogg`.
3. **`transcribeVoice(voicePath)`**:
   - Define `process.env.FFMPEG_PATH` usando `ffmpeg-static` se não estiver definido.
   - Chama `nodewhisper(voicePath, { modelName: 'base.en', autoDownloadModelName: 'base.en', removeWavFileAfterTranscription: true, whisperOptions: { outputInText: true, ... } })`.
   - Remove prefixos de timestamp (`/[[\d:.]+\s*-->\s*[\d:.]+\]\s*/g`), retorna o texto transcrito e apaga o `.ogg` temporário no bloco `finally`.

### 3.3. Internacionalização (`src/utils/i18n.ts` — 54 linhas)
- Suporta `Language = 'en' | 'ja' | 'vi'` (idioma padrão inicial: `'vi'`).
- Carrega os dicionários `locales/en.json`, `locales/ja.json` e `locales/vi.json`, fazendo fallback para `'en'` e depois para a própria `key`, substituindo variáveis `{{varName}}`.

### 3.4. Logger (`src/utils/logger.ts` — 99 linhas) e Caminhos (`src/utils/pathUtils.ts` — 70 linhas)
- `logger.ts`: formata logs no console com timestamp 24h (`toLocaleTimeString('ja-JP', { hour12: false })`), cores ANSI e tags `[INFO]`, `[WARN]`, `[ERROR]`, `[DEBUG]`, `[PHASE]` (magenta), `[DONE]` (verde) e divisórias `divider(label?)`.
- `pathUtils.ts`:
  - `getAntigravityCliPath()`: valida `process.env.ANTIGRAVITY_PATH` via `fs.statSync(path).isFile()` (prevenindo command injection); caso não definido, retorna `/Applications/Antigravity.app/Contents/Resources/app/bin/antigravity` (macOS), `%LOCALAPPDATA%\Programs\Antigravity\Antigravity.exe` (Windows) ou `'antigravity'` (Linux).
  - `extractProjectNameFromPath(workspacePath)`: retorna o último segmento do caminho (`workspacePath.split(/[/\\]/).filter(Boolean).pop()`).
  - `getAntigravityCdpHint(port)`: retorna o comando de abertura com `--remote-debugging-port=<port>` apropriado para o SO atual.

---

## 4. Interface de Linha de Comando (`src/bin/`)

### 4.1. Entrypoint e Boas-Vindas (`src/bin/cli.ts` & `src/bin/welcome.ts`)
- Opções globais: `--verbose` (define `logLevel = 'debug'`) e `--quiet` (define `logLevel = 'error'`).
- Quando executado sem subcomando (`antigravity-telegram-remote`):
  - Verifica `ConfigLoader.configExists()` e `fs.existsSync(path.resolve(process.cwd(), '.env'))`.
  - Se nenhum dos dois existir, chama `printWelcome()` (`src/bin/welcome.ts`, que desenha o quadro ASCII da coruja com os comandos `setup`, `open`, `start`, `doctor`) e inicia `setupAction()`.
  - Caso contrário, inicia `startAction()`.

### 4.2. Subcomando `setup` (`src/bin/commands/setup.ts` — 281 linhas)
Wizard interativo de 3 etapas:
1. **Passo 1/3 — `Telegram Bot Token`**:
   - Lê o token com eco mascarado por asteriscos (`*`) via `askSecret` (`stdin.setRawMode(true)`).
   - Valida online via `GET https://api.telegram.org/bot<token>/getMe` (timeout `10000ms`). Se retornar `200 OK`, exibe `Đã xác thực! Bot: @<username> (<first_name>)`. Se falhar por rede/offline, exibe aviso `Không thể xác thực trực tuyến — sử dụng token như hiện tại.` e aceita o token informado.
2. **Passo 2/3 — `Allowed User IDs`**:
   - Valida que ao menos um ID foi informado e que todos são estritamente numéricos (`/^\d+$/`).
3. **Passo 3/3 — `Workspace BaseDir`**:
   - Oferece valor padrão `~/Code` e autocompletar de diretórios com `Tab` (`directoryCompleter`). Se o diretório não existir, pergunta se deseja criá-lo (`y/n`) e executa `fs.mkdirSync(resolved, { recursive: true })`.
- Salva os 3 campos em `~/.antigravity-telegram-remote/config.json` via `ConfigLoader.save()` e imprime os próximos passos (`open`, `start`).

### 4.3. Subcomando `open` (`src/bin/commands/open.ts` — 157 linhas)
- Não recebe argumentos de linha de comando.
- Percorre `CDP_PORTS` (`9222, 9223, 9333, 9444, 9555, 9666`) testando se a porta está livre via `net.createServer().listen(port, '127.0.0.1')`.
- Abre o Antigravity na primeira porta livre encontrada (`openMacOS`, `openWindows` ou `openLinux`).

### 4.4. Subcomando `doctor` (`src/bin/commands/doctor.ts` — 178 linhas)
Executa exatamente 7 checagens sequenciais:
1. Existência do diretório `~/.antigravity-telegram-remote`.
2. Existência do arquivo `~/.antigravity-telegram-remote/config.json`.
3. Existência do arquivo `.env` no diretório atual (se existir, carrega com `dotenv`).
4. Definição de `TELEGRAM_BOT_TOKEN` e `ALLOWED_USER_IDS` (em `process.env` ou `config.json`).
5. Versão do Node.js `>= 18` (no port Go, substituído pela verificação do binário/ambiente Go).
6. Checagens de plataforma (no TS original, executadas quando `os.platform() === 'darwin'`): `xcode-select -p` e existência de `/Applications/Antigravity.app` ou `ANTIGRAVITY_PATH`. *(No port Go, expandido para validar o executável do Antigravity em macOS, Linux e Windows)*.
7. Sondagem HTTP (`http://127.0.0.1:<port>/json/list`, timeout `2000ms`) em todas as portas de `CDP_PORTS`.
Se qualquer verificação obrigatória falhar, define `process.exitCode = 1`.
