# Especificação Técnica 03 — Banco de Dados SQLite (Modo WAL) e Os 4 Repositórios

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de forma 100% autocontida e exaustiva a camada de persistência SQLite (`internal/database`) do port em Golang (`1.23+`) do `antigravity-telegram-remote` utilizando o driver Pure-Go `modernc.org/sqlite` (sem `CGO_ENABLED`, conforme ADR-002), preservando integralmente o esquema DDL, os prepared statements e a semântica de cada um dos 4 repositórios originais.

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript) | Arquivo de Testes Original (Jest) | Destino no Port Go | Pacote Go | Responsabilidade Principal |
| :--- | :--- | :--- | :--- | :--- |
| [`src/bot/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bot/index.ts#L865-L870) *(inicialização do SQLite)* | *(Testado em todos os `*_repo_test.go`)* | `internal/database/sqlite.go` | `database` | Abertura da conexão SQLite (`~/.antigravity-telegram-remote/antigravity.db` ou `:memory:`), criação do diretório pai e execução imediata de `PRAGMA journal_mode = WAL;`. |
| [`src/database/chatSessionRepository.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/database/chatSessionRepository.ts) | [`tests/database/chatSessionRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/chatSessionRepository.test.ts) | `internal/database/session_repo.go`<br>`internal/database/session_repo_test.go` | `database` | Repositório `ChatSessionRepository` (tabela `chat_sessions`): mapeamento `channel_id` $\leftrightarrow$ sessão numerada por `category_id` e título (`display_name`). |
| [`src/database/workspaceBindingRepository.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/database/workspaceBindingRepository.ts) | [`tests/database/workspaceBindingRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/workspaceBindingRepository.test.ts) | `internal/database/binding_repo.go`<br>`internal/database/binding_repo_test.go` | `database` | Repositório `WorkspaceBindingRepository` (tabela `workspace_bindings`): vínculo `channel_id` (UNIQUE) $\leftrightarrow$ `workspace_path` e `guild_id` com suporte a `upsert`. |
| [`src/database/templateRepository.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/database/templateRepository.ts) | [`tests/database/templateRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/templateRepository.test.ts) | `internal/database/template_repo.go`<br>`internal/database/template_repo_test.go` | `database` | Repositório `TemplateRepository` (tabela `templates`): CRUD de templates de prompt reutilizáveis indexados por `name` (UNIQUE). |
| [`src/database/scheduleRepository.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/database/scheduleRepository.ts) | [`tests/database/scheduleRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/scheduleRepository.test.ts) | `internal/database/schedule_repo.go`<br>`internal/database/schedule_repo_test.go` | `database` | Repositório `ScheduleRepository` (tabela `schedules`): persistência de agendamentos cron (`cron_expression`, `prompt`, `workspace_path`, `enabled`). |

### 1.2. Documentação Arquitetural e ADRs Relacionados
- [`antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md)
- [`antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md) (ADR-002)

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. `internal/database/sqlite.go`

```go
package database

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

// Open abre ou cria o banco de dados SQLite no caminho especificado (ou ":memory:"),
// garantindo que o diretório pai exista (quando dbPath != ":memory:") e aplicando
// imediatamente `PRAGMA journal_mode = WAL;`.
func Open(dbPath string) (*sql.DB, error)

// OpenInMemory abre uma instância isolada em memória para testes unitários.
func OpenInMemory() (*sql.DB, error)
```

---

### 2.2. `internal/database/session_repo.go` (`ChatSessionRepository`)

```go
package database

import "database/sql"

// ChatSessionRecord corresponde a ChatSessionRecord em src/database/chatSessionRepository.ts.
type ChatSessionRecord struct {
	ID            int64   `json:"id"`
	ChannelID     string  `json:"channelId"`
	CategoryID    string  `json:"categoryId"`
	WorkspacePath string  `json:"workspacePath"`
	SessionNumber int     `json:"sessionNumber"`
	DisplayName   *string `json:"displayName"`
	IsRenamed     bool    `json:"isRenamed"`
	GuildID       string  `json:"guildId"`
	CreatedAt     string  `json:"createdAt,omitempty"`
}

// CreateChatSessionInput corresponde a CreateChatSessionInput em src/database/chatSessionRepository.ts.
type CreateChatSessionInput struct {
	ChannelID     string
	CategoryID    string
	WorkspacePath string
	SessionNumber int
	GuildID       string
}

// ChatSessionRepository gerencia a persistência de sessões de chat por canal/tópico.
type ChatSessionRepository struct {
	db                       *sql.DB
	stmtCreate               *sql.Stmt
	stmtFindByChannelID      *sql.Stmt
	stmtFindByCategoryID     *sql.Stmt
	stmtGetNextSessionNumber *sql.Stmt
	stmtUpdateDisplayName    *sql.Stmt
	stmtFindByDisplayName    *sql.Stmt
	stmtDeleteByChannelID    *sql.Stmt
}

// NewChatSessionRepository executa CREATE TABLE IF NOT EXISTS chat_sessions e prepara todos os statements.
func NewChatSessionRepository(db *sql.DB) (*ChatSessionRepository, error)

func (r *ChatSessionRepository) Create(input CreateChatSessionInput) (ChatSessionRecord, error)
func (r *ChatSessionRepository) FindByChannelId(channelId string) (*ChatSessionRecord, error)
func (r *ChatSessionRepository) FindByCategoryId(categoryId string) ([]ChatSessionRecord, error)
func (r *ChatSessionRepository) GetNextSessionNumber(categoryId string) (int, error)
func (r *ChatSessionRepository) UpdateDisplayName(channelId string, displayName string) (bool, error)
func (r *ChatSessionRepository) FindByDisplayName(workspacePath string, displayName string) (*ChatSessionRecord, error)
func (r *ChatSessionRepository) DeleteByChannelId(channelId string) (bool, error)
func (r *ChatSessionRepository) Close() error
```

---

### 2.3. `internal/database/binding_repo.go` (`WorkspaceBindingRepository`)

```go
package database

import "database/sql"

// WorkspaceBindingRecord corresponde a WorkspaceBindingRecord em src/database/workspaceBindingRepository.ts.
type WorkspaceBindingRecord struct {
	ID            int64  `json:"id"`
	ChannelID     string `json:"channelId"`
	WorkspacePath string `json:"workspacePath"`
	GuildID       string `json:"guildId"`
	CreatedAt     string `json:"createdAt,omitempty"`
}

// CreateWorkspaceBindingInput corresponde a CreateWorkspaceBindingInput em src/database/workspaceBindingRepository.ts.
type CreateWorkspaceBindingInput struct {
	ChannelID     string
	WorkspacePath string
	GuildID       string
}

// WorkspaceBindingRepository gerencia a persistência de vínculos canal/tópico <-> workspace.
type WorkspaceBindingRepository struct {
	db                                 *sql.DB
	stmtCreate                         *sql.Stmt
	stmtFindByChannelID                *sql.Stmt
	stmtFindByWorkspacePathAndGuildID  *sql.Stmt
	stmtFindByGuildID                  *sql.Stmt
	stmtFindAll                        *sql.Stmt
	stmtDeleteByChannelID              *sql.Stmt
	stmtUpsert                         *sql.Stmt
}

// NewWorkspaceBindingRepository executa CREATE TABLE IF NOT EXISTS workspace_bindings e prepara todos os statements.
func NewWorkspaceBindingRepository(db *sql.DB) (*WorkspaceBindingRepository, error)

func (r *WorkspaceBindingRepository) Create(input CreateWorkspaceBindingInput) (WorkspaceBindingRecord, error)
func (r *WorkspaceBindingRepository) FindByChannelId(channelId string) (*WorkspaceBindingRecord, error)
func (r *WorkspaceBindingRepository) FindByWorkspacePathAndGuildId(workspacePath string, guildId string) ([]WorkspaceBindingRecord, error)
func (r *WorkspaceBindingRepository) FindByGuildId(guildId string) ([]WorkspaceBindingRecord, error)
func (r *WorkspaceBindingRepository) FindAll() ([]WorkspaceBindingRecord, error)
func (r *WorkspaceBindingRepository) DeleteByChannelId(channelId string) (bool, error)
func (r *WorkspaceBindingRepository) Upsert(input CreateWorkspaceBindingInput) (WorkspaceBindingRecord, error)
func (r *WorkspaceBindingRepository) Close() error
```

---

### 2.4. `internal/database/template_repo.go` (`TemplateRepository`)

```go
package database

import "database/sql"

// TemplateRecord corresponde a TemplateRecord em src/database/templateRepository.ts.
type TemplateRecord struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Prompt    string `json:"prompt"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// CreateTemplateInput corresponde a CreateTemplateInput em src/database/templateRepository.ts.
type CreateTemplateInput struct {
	Name   string
	Prompt string
}

// UpdateTemplateInput corresponde a UpdateTemplateInput em src/database/templateRepository.ts.
type UpdateTemplateInput struct {
	Prompt *string
}

// TemplateRepository gerencia a persistência de templates de prompt em SQLite.
type TemplateRepository struct {
	db               *sql.DB
	stmtCreate       *sql.Stmt
	stmtFindAll      *sql.Stmt
	stmtFindByID     *sql.Stmt
	stmtFindByName   *sql.Stmt
	stmtDeleteByName *sql.Stmt
	stmtUpdateByName *sql.Stmt
}

// NewTemplateRepository executa CREATE TABLE IF NOT EXISTS templates e prepara todos os statements.
func NewTemplateRepository(db *sql.DB) (*TemplateRepository, error)

func (r *TemplateRepository) Create(input CreateTemplateInput) (TemplateRecord, error)
func (r *TemplateRepository) FindAll() ([]TemplateRecord, error)
func (r *TemplateRepository) FindById(id int64) (*TemplateRecord, error)
func (r *TemplateRepository) FindByName(name string) (*TemplateRecord, error)
func (r *TemplateRepository) DeleteByName(name string) (bool, error)
func (r *TemplateRepository) UpdateByName(name string, input UpdateTemplateInput) (bool, error)
func (r *TemplateRepository) Close() error
```

---

### 2.5. `internal/database/schedule_repo.go` (`ScheduleRepository`)

```go
package database

import "database/sql"

// ScheduleRecord corresponde a ScheduleRecord em src/database/scheduleRepository.ts.
type ScheduleRecord struct {
	ID             int64  `json:"id"`
	CronExpression string `json:"cronExpression"`
	Prompt         string `json:"prompt"`
	WorkspacePath  string `json:"workspacePath"`
	Enabled        bool   `json:"enabled"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// CreateScheduleInput corresponde a CreateScheduleInput em src/database/scheduleRepository.ts.
type CreateScheduleInput struct {
	CronExpression string
	Prompt         string
	WorkspacePath  string
	Enabled        bool
}

// UpdateScheduleInput corresponde a UpdateScheduleInput em src/database/scheduleRepository.ts.
// Campos ponteiro permitem distinguir campos não informados (nil) de valores zero.
type UpdateScheduleInput struct {
	CronExpression *string
	Prompt         *string
	WorkspacePath  *string
	Enabled        *bool
}

// ScheduleRepository gerencia a persistência de tarefas agendadas em SQLite.
type ScheduleRepository struct {
	db              *sql.DB
	stmtCreate      *sql.Stmt
	stmtFindAll     *sql.Stmt
	stmtFindByID    *sql.Stmt
	stmtFindEnabled *sql.Stmt
	stmtDelete      *sql.Stmt
}

// NewScheduleRepository executa CREATE TABLE IF NOT EXISTS schedules e prepara todos os statements.
func NewScheduleRepository(db *sql.DB) (*ScheduleRepository, error)

func (r *ScheduleRepository) Create(input CreateScheduleInput) (ScheduleRecord, error)
func (r *ScheduleRepository) FindAll() ([]ScheduleRecord, error)
func (r *ScheduleRepository) FindById(id int64) (*ScheduleRecord, error)
func (r *ScheduleRepository) FindEnabled() ([]ScheduleRecord, error)
func (r *ScheduleRepository) Delete(id int64) (bool, error)
func (r *ScheduleRepository) Update(id int64, input UpdateScheduleInput) (bool, error)
func (r *ScheduleRepository) Close() error
```

---

## 3. Constantes, Queries SQL, Expressões Regulares e Scripts Literais

### 3.1. Pragma de Inicialização (`sqlite.go`)
```sql
PRAGMA journal_mode = WAL;
```

---

### 3.2. DDL e Queries SQL Literais de `ChatSessionRepository` (`session_repo.go`)

1. **DDL (`initialize`)**:
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
   )
   ```
2. **`stmtCreate`**:
   ```sql
   INSERT INTO chat_sessions (channel_id, category_id, workspace_path, session_number, guild_id) VALUES (?, ?, ?, ?, ?)
   ```
3. **`stmtFindByChannelId`**:
   ```sql
   SELECT id, channel_id, category_id, workspace_path, session_number, display_name, is_renamed, guild_id, created_at FROM chat_sessions WHERE channel_id = ?
   ```
   *(Equivalente a `SELECT * FROM chat_sessions WHERE channel_id = ?` com ordem de colunas explícita).*
4. **`stmtFindByCategoryId`**:
   ```sql
   SELECT id, channel_id, category_id, workspace_path, session_number, display_name, is_renamed, guild_id, created_at FROM chat_sessions WHERE category_id = ? ORDER BY session_number ASC
   ```
5. **`stmtGetNextSessionNumber`**:
   ```sql
   SELECT MAX(session_number) as max_num FROM chat_sessions WHERE category_id = ?
   ```
6. **`stmtUpdateDisplayName`**:
   ```sql
   UPDATE chat_sessions SET display_name = ?, is_renamed = 1 WHERE channel_id = ?
   ```
7. **`stmtFindByDisplayName`**:
   ```sql
   SELECT id, channel_id, category_id, workspace_path, session_number, display_name, is_renamed, guild_id, created_at FROM chat_sessions WHERE workspace_path = ? AND display_name = ? ORDER BY id DESC LIMIT 1
   ```
8. **`stmtDeleteByChannelId`**:
   ```sql
   DELETE FROM chat_sessions WHERE channel_id = ?
   ```

---

### 3.3. DDL e Queries SQL Literais de `WorkspaceBindingRepository` (`binding_repo.go`)

1. **DDL (`initialize`)**:
   ```sql
   CREATE TABLE IF NOT EXISTS workspace_bindings (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       channel_id TEXT NOT NULL UNIQUE,
       workspace_path TEXT NOT NULL,
       guild_id TEXT NOT NULL,
       created_at TEXT NOT NULL DEFAULT (datetime('now'))
   )
   ```
2. **`stmtCreate`**:
   ```sql
   INSERT INTO workspace_bindings (channel_id, workspace_path, guild_id) VALUES (?, ?, ?)
   ```
3. **`stmtFindByChannelId`**:
   ```sql
   SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings WHERE channel_id = ?
   ```
4. **`stmtFindByWorkspacePathAndGuildId`**:
   ```sql
   SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings WHERE workspace_path = ? AND guild_id = ? ORDER BY id ASC
   ```
5. **`stmtFindByGuildId`**:
   ```sql
   SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings WHERE guild_id = ? ORDER BY id ASC
   ```
6. **`stmtFindAll`**:
   ```sql
   SELECT id, channel_id, workspace_path, guild_id, created_at FROM workspace_bindings ORDER BY id ASC
   ```
7. **`stmtDeleteByChannelId`**:
   ```sql
   DELETE FROM workspace_bindings WHERE channel_id = ?
   ```
8. **`stmtUpsert`**:
   ```sql
   INSERT INTO workspace_bindings (channel_id, workspace_path, guild_id)
   VALUES (?, ?, ?)
   ON CONFLICT(channel_id) DO UPDATE SET
       workspace_path = excluded.workspace_path,
       guild_id = excluded.guild_id
   ```

---

### 3.4. DDL e Queries SQL Literais de `TemplateRepository` (`template_repo.go`)

1. **DDL (`initialize`)**:
   ```sql
   CREATE TABLE IF NOT EXISTS templates (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       name TEXT NOT NULL UNIQUE,
       prompt TEXT NOT NULL,
       created_at TEXT NOT NULL DEFAULT (datetime('now'))
   )
   ```
2. **`stmtCreate`**:
   ```sql
   INSERT INTO templates (name, prompt) VALUES (?, ?)
   ```
3. **`stmtFindAll`**:
   ```sql
   SELECT id, name, prompt, created_at FROM templates ORDER BY id ASC
   ```
4. **`stmtFindById`**:
   ```sql
   SELECT id, name, prompt, created_at FROM templates WHERE id = ?
   ```
5. **`stmtFindByName`**:
   ```sql
   SELECT id, name, prompt, created_at FROM templates WHERE name = ?
   ```
6. **`stmtDeleteByName`**:
   ```sql
   DELETE FROM templates WHERE name = ?
   ```
7. **`stmtUpdateByName`**:
   ```sql
   UPDATE templates SET prompt = ? WHERE name = ?
   ```

---

### 3.5. DDL e Queries SQL Literais de `ScheduleRepository` (`schedule_repo.go`)

1. **DDL (`initialize`)**:
   ```sql
   CREATE TABLE IF NOT EXISTS schedules (
       id INTEGER PRIMARY KEY AUTOINCREMENT,
       cron_expression TEXT NOT NULL,
       prompt TEXT NOT NULL,
       workspace_path TEXT NOT NULL,
       enabled INTEGER NOT NULL DEFAULT 1,
       created_at TEXT NOT NULL DEFAULT (datetime('now'))
   )
   ```
2. **`stmtCreate`**:
   ```sql
   INSERT INTO schedules (cron_expression, prompt, workspace_path, enabled) VALUES (?, ?, ?, ?)
   ```
3. **`stmtFindAll`**:
   ```sql
   SELECT id, cron_expression, prompt, workspace_path, enabled, created_at FROM schedules ORDER BY id ASC
   ```
4. **`stmtFindById`**:
   ```sql
   SELECT id, cron_expression, prompt, workspace_path, enabled, created_at FROM schedules WHERE id = ?
   ```
5. **`stmtFindEnabled`**:
   ```sql
   SELECT id, cron_expression, prompt, workspace_path, enabled, created_at FROM schedules WHERE enabled = 1 ORDER BY id ASC
   ```
6. **`stmtDelete`**:
   ```sql
   DELETE FROM schedules WHERE id = ?
   ```
7. **Query Dinâmica de `Update(id, input)`**:
   ```sql
   UPDATE schedules SET <sets.join(', ')> WHERE id = ?
   ```
   Onde `<sets>` pode conter, na ordem exata:
   - `cron_expression = ?` (se `input.CronExpression != nil`)
   - `prompt = ?` (se `input.Prompt != nil`)
   - `workspace_path = ?` (se `input.WorkspacePath != nil`)
   - `enabled = ?` (se `input.Enabled != nil`, mapeando `true -> 1`, `false -> 0`)

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Regras de `ChatSessionRepository` (`internal/database/session_repo.go`)
1. **`Create(input)`**:
   - Executa `stmtCreate.Exec(input.ChannelID, input.CategoryID, input.WorkspacePath, input.SessionNumber, input.GuildID)`.
   - Se violar a constraint `UNIQUE` de `channel_id`, retorna erro imediatamente.
   - Obtém `id, _ := result.LastInsertId()`.
   - Retorna `ChatSessionRecord{ID: id, ChannelID: input.ChannelID, CategoryID: input.CategoryID, WorkspacePath: input.WorkspacePath, SessionNumber: input.SessionNumber, DisplayName: nil, IsRenamed: false, GuildID: input.GuildID}`.
2. **`FindByChannelId(channelId)`**:
   - Executa `stmtFindByChannelID.QueryRow(channelId)`.
   - Se `err == sql.ErrNoRows`, retorna `(nil, nil)` (equivalente a `undefined` em TypeScript).
   - Mapeia `display_name` (`sql.NullString` $\rightarrow$ `*string`, `nil` quando `Valid == false`) e `is_renamed` (`int == 1` $\rightarrow$ `bool`).
3. **`FindByCategoryId(categoryId)`**:
   - Executa `stmtFindByCategoryID.Query(categoryId)`.
   - Se não houver linhas, retorna slice vazio não-nulo `[]ChatSessionRecord{}` (`len == 0`).
4. **`GetNextSessionNumber(categoryId)`**:
   - Executa `stmtGetNextSessionNumber.QueryRow(categoryId)` lendo `var maxNum sql.NullInt64`.
   - Se `!maxNum.Valid`, retorna `1` (`0 + 1`).
   - Se `maxNum.Valid`, retorna `int(maxNum.Int64) + 1`.
5. **`UpdateDisplayName(channelId, displayName)`**:
   - Executa `stmtUpdateDisplayName.Exec(displayName, channelId)`.
   - Retorna `rowsAffected > 0`.
6. **`FindByDisplayName(workspacePath, displayName)`**:
   - Executa `stmtFindByDisplayName.QueryRow(workspacePath, displayName)` (`ORDER BY id DESC LIMIT 1`, retornando a sessão mais recente com aquele título no workspace).
   - Se `sql.ErrNoRows`, retorna `(nil, nil)`.
7. **`DeleteByChannelId(channelId)`**:
   - Executa `stmtDeleteByChannelID.Exec(channelId)` e retorna `rowsAffected > 0`.

---

### 4.2. Regras de `WorkspaceBindingRepository` (`internal/database/binding_repo.go`)
1. **`Create(input)`**:
   - Executa `stmtCreate.Exec(input.ChannelID, input.WorkspacePath, input.GuildID)`.
   - Se `channel_id` já existir, retorna erro de constraint `UNIQUE`.
   - Retorna `WorkspaceBindingRecord` com `ID = LastInsertId()`.
2. **`FindByChannelId(channelId)`**:
   - Retorna `(*WorkspaceBindingRecord, nil)` ou `(nil, nil)` quando `sql.ErrNoRows`.
3. **`FindByWorkspacePathAndGuildId(workspacePath, guildId)`**, **`FindByGuildId(guildId)`** e **`FindAll()`**:
   - Retornam sempre slice inicializado `[]WorkspaceBindingRecord{}` (nunca `nil` quando vazio) ordenado por `id ASC`.
4. **`DeleteByChannelId(channelId)`**:
   - Retorna `rowsAffected > 0`.
5. **`Upsert(input)`**:
   - Executa `stmtUpsert.Exec(input.ChannelID, input.WorkspacePath, input.GuildID)`.
   - Em seguida chama `FindByChannelId(input.ChannelID)` e retorna o registro atualizado/inserido completo.

---

### 4.3. Regras de `TemplateRepository` (`internal/database/template_repo.go`)
1. **`Create(input)`**:
   - Executa `stmtCreate.Exec(input.Name, input.Prompt)`.
   - Se `name` já existir, retorna erro de constraint `UNIQUE`.
2. **`FindAll()`**:
   - Retorna todos os templates em `ORDER BY id ASC` (slice vazio `[]TemplateRecord{}` quando não houver registros).
3. **`FindById(id)`** e **`FindByName(name)`**:
   - Retornam `(nil, nil)` quando `sql.ErrNoRows`.
4. **`DeleteByName(name)`**:
   - Executa `stmtDeleteByName.Exec(name)` e retorna `rowsAffected > 0`.
5. **`UpdateByName(name, input)`**:
   - Se `input.Prompt == nil`, retorna `(false, nil)` sem executar query.
   - Se `input.Prompt != nil`, executa `UPDATE templates SET prompt = ? WHERE name = ?` e retorna `rowsAffected > 0`.

---

### 4.4. Regras de `ScheduleRepository` (`internal/database/schedule_repo.go`)
1. **`Create(input)`**:
   - Converte `input.Enabled` em `enabledInt := 0` (`1` se `true`).
   - Executa `stmtCreate.Exec(input.CronExpression, input.Prompt, input.WorkspacePath, enabledInt)`.
   - Retorna `ScheduleRecord` com `ID = LastInsertId()`.
2. **`FindAll()`** e **`FindEnabled()`**:
   - `FindAll()` retorna todos os registros em `ORDER BY id ASC`.
   - `FindEnabled()` retorna apenas registros com `enabled = 1` em `ORDER BY id ASC`.
   - Ambos convertem `enabled == 1` para `bool` e retornam `[]ScheduleRecord{}` quando vazios.
3. **`FindById(id)`**:
   - Retorna `(nil, nil)` quando `sql.ErrNoRows`.
4. **`Delete(id)`**:
   - Retorna `rowsAffected > 0`.
5. **`Update(id, input)`**:
   - Constrói `sets []string` e `values []any` apenas para os campos não-nulos de `input`.
   - Se `len(sets) == 0`, retorna `(false, nil)` imediatamente.
   - Adiciona `id` ao final de `values`, executa `UPDATE schedules SET <sets> WHERE id = ?` e retorna `rowsAffected > 0`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Configuração de Pool do `database/sql` com SQLite (`modernc.org/sqlite`)**:
   - Para conexões `:memory:` em testes unitários, configurar `db.SetMaxOpenConns(1)` para garantir que todas as operações do teste compartilhem o mesmo banco em memória.
   - Para conexões em arquivo (`antigravity.db`), `PRAGMA journal_mode = WAL;` e `PRAGMA busy_timeout = 5000;` garantem leituras concorrentes sem bloqueio e serialização segura de escritas.
2. **Ciclo de Vida dos Prepared Statements**:
   - Cada repositório prepara seus `*sql.Stmt` no construtor (`New...Repository(db)`) após executar `initialize()`, e expõe `Close() error` para fechar os statements antes de `db.Close()` no encerramento gracioso do bot.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os cenários abaixo mapeiam **1 para 1** os blocos `describe` e `it` dos 4 arquivos de teste originais e devem passar com `go test -v -race ./internal/database/...`.

### 6.1. `internal/database/session_repo_test.go` (Mapeado de [`tests/database/chatSessionRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/chatSessionRepository.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestChatSessionRepo_TableInitialization` | `ChatSessionRepository > table initialization > creates the chat_sessions table on initialization` | `OpenInMemory()` + `NewChatSessionRepository(db)` | Query em `sqlite_master WHERE type='table' AND name='chat_sessions'` retorna 1 linha. |
| `TestChatSessionRepo_Create_ReturnsIDAndDefaults` | `ChatSessionRepository > create - session creation > creates a session and returns the ID` | `Create({ChannelID: "ch-1", CategoryID: "cat-1", WorkspacePath: "my-project", SessionNumber: 1, GuildID: "guild-1"})` | `result.ID > 0`, `ChannelID == "ch-1"`, `CategoryID == "cat-1"`, `WorkspacePath == "my-project"`, `SessionNumber == 1`, `DisplayName == nil`, `IsRenamed == false`, `GuildID == "guild-1"`. |
| `TestChatSessionRepo_Create_DuplicateChannelIDThrows` | `ChatSessionRepository > create - session creation > throws an error on duplicate creation with the same channel ID` | Duas chamadas `Create` com `ChannelID: "ch-1"` | A segunda chamada retorna erro (`err != nil`). |
| `TestChatSessionRepo_FindByChannelId_Existing` | `ChatSessionRepository > findByChannelId > retrieves an existing session` | Cria `"ch-1"` com `WorkspacePath: "proj"` $\rightarrow$ `FindByChannelId("ch-1")` | Retorna registro não-nulo com `WorkspacePath == "proj"` e `IsRenamed == false`. |
| `TestChatSessionRepo_FindByChannelId_NonExistent` | `ChatSessionRepository > findByChannelId > returns undefined for a non-existent channel ID` | `FindByChannelId("nonexistent")` | Retorna `nil, nil`. |
| `TestChatSessionRepo_FindByCategoryId_SortedBySessionNumber` | `ChatSessionRepository > findByCategoryId > retrieves sessions in the same category sorted by session number` | Cria `("ch-1", "cat-1", num=1)`, `("ch-2", "cat-1", num=2)`, `("ch-3", "cat-2", num=1)` $\rightarrow$ `FindByCategoryId("cat-1")` | Retorna 2 itens com `SessionNumber` `1` e `2` respectivamente. |
| `TestChatSessionRepo_FindByCategoryId_Empty` | `ChatSessionRepository > findByCategoryId > returns an empty array when there are no sessions` | `FindByCategoryId("nonexistent")` | Retorna slice vazio `[]ChatSessionRecord{}` (`len == 0`). |
| `TestChatSessionRepo_GetNextSessionNumber_EmptyReturns1` | `ChatSessionRepository > getNextSessionNumber > returns 1 when there are no sessions` | `GetNextSessionNumber("cat-1")` | Retorna `1`. |
| `TestChatSessionRepo_GetNextSessionNumber_ReturnsMaxPlus1` | `ChatSessionRepository > getNextSessionNumber > returns MAX+1 when existing sessions exist` | Cria sessões `1` e `3` em `"cat-1"` $\rightarrow$ `GetNextSessionNumber("cat-1")` | Retorna `4`. |
| `TestChatSessionRepo_GetNextSessionNumber_IsolatedPerCategory` | `ChatSessionRepository > getNextSessionNumber > sessions in different categories do not affect each other` | Cria sessão `5` em `"cat-1"` $\rightarrow$ `GetNextSessionNumber("cat-2")` | Retorna `1`. |
| `TestChatSessionRepo_UpdateDisplayName_Existing` | `ChatSessionRepository > updateDisplayName > updates the display name and sets is_renamed to true` | Cria `"ch-1"` $\rightarrow$ `UpdateDisplayName("ch-1", "React認証バグ修正")` | Retorna `true`; `FindByChannelId("ch-1")` retorna `*DisplayName == "React認証バグ修正"` e `IsRenamed == true`. |
| `TestChatSessionRepo_UpdateDisplayName_NonExistent` | `ChatSessionRepository > updateDisplayName > returns false for a non-existent channel ID` | `UpdateDisplayName("nonexistent", "title")` | Retorna `false`. |
| `TestChatSessionRepo_DeleteByChannelId_Existing` | `ChatSessionRepository > deleteByChannelId > deletes a session` | Cria `"ch-1"` $\rightarrow$ `DeleteByChannelId("ch-1")` | Retorna `true` e `FindByChannelId("ch-1")` passa a retornar `nil`. |
| `TestChatSessionRepo_DeleteByChannelId_NonExistent` | `ChatSessionRepository > deleteByChannelId > returns false when deleting a non-existent channel ID` | `DeleteByChannelId("nonexistent")` | Retorna `false`. |

---

### 6.2. `internal/database/binding_repo_test.go` (Mapeado de [`tests/database/workspaceBindingRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/workspaceBindingRepository.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestWorkspaceBindingRepo_TableInitialization` | `WorkspaceBindingRepository > table initialization > creates the workspace_bindings table on initialization` | `OpenInMemory()` + `NewWorkspaceBindingRepository(db)` | Tabela `workspace_bindings` existe em `sqlite_master`. |
| `TestWorkspaceBindingRepo_Create_ReturnsID` | `WorkspaceBindingRepository > create - binding creation > saves channel ID and workspace path, and returns the ID` | `Create({ChannelID: "123456", WorkspacePath: "my-project", GuildID: "guild-1"})` | `ID > 0`, `ChannelID == "123456"`, `WorkspacePath == "my-project"`, `GuildID == "guild-1"`. |
| `TestWorkspaceBindingRepo_Create_DuplicateChannelIDThrows` | `WorkspaceBindingRepository > create - binding creation > throws an error on duplicate creation with the same channel ID` | Duas chamadas `Create` com `ChannelID: "123456"` | Segunda chamada retorna erro (`err != nil`). |
| `TestWorkspaceBindingRepo_FindByChannelId_Existing` | `WorkspaceBindingRepository > findByChannelId > retrieves an existing binding` | Cria `"123456"` $\rightarrow$ `FindByChannelId("123456")` | Retorna registro com `WorkspacePath == "my-project"`. |
| `TestWorkspaceBindingRepo_FindByChannelId_NonExistent` | `WorkspaceBindingRepository > findByChannelId > returns undefined for a non-existent channel ID` | `FindByChannelId("nonexistent")` | Retorna `nil, nil`. |
| `TestWorkspaceBindingRepo_FindByGuildId_RetrievesAllForGuild` | `WorkspaceBindingRepository > findByGuildId > retrieves all bindings for the same guild` | Cria 2 em `"guild-1"` (`"proj-a"`, `"proj-b"`) e 1 em `"guild-2"` (`"proj-c"`) $\rightarrow$ `FindByGuildId("guild-1")` | Retorna 2 registros: `"proj-a"` e `"proj-b"`. |
| `TestWorkspaceBindingRepo_FindByGuildId_Empty` | `WorkspaceBindingRepository > findByGuildId > returns an empty array when there are no bindings` | `FindByGuildId("nonexistent")` | Retorna slice vazio `[]WorkspaceBindingRecord{}`. |
| `TestWorkspaceBindingRepo_FindAll_RetrievesAll` | `WorkspaceBindingRepository > findAll > retrieves all bindings` | Cria `"111"` e `"222"` $\rightarrow$ `FindAll()` | Retorna 2 registros. |
| `TestWorkspaceBindingRepo_FindAll_Empty` | `WorkspaceBindingRepository > findAll > returns an empty array when there are no bindings` | `FindAll()` | Retorna slice vazio `[]WorkspaceBindingRecord{}`. |
| `TestWorkspaceBindingRepo_DeleteByChannelId_Existing` | `WorkspaceBindingRepository > deleteByChannelId > deletes the binding for the specified channel ID` | Cria `"123456"` $\rightarrow$ `DeleteByChannelId("123456")` | Retorna `true` e `FindByChannelId("123456")` retorna `nil`. |
| `TestWorkspaceBindingRepo_DeleteByChannelId_NonExistent` | `WorkspaceBindingRepository > deleteByChannelId > returns false when deleting a non-existent channel ID` | `DeleteByChannelId("nonexistent")` | Retorna `false`. |
| `TestWorkspaceBindingRepo_Upsert_CreatesWhenNotExist` | `WorkspaceBindingRepository > upsert > creates a new binding when it does not exist` | `Upsert({ChannelID: "123456", WorkspacePath: "proj-a", GuildID: "guild-1"})` | Retorna registro com `ChannelID == "123456"` e `WorkspacePath == "proj-a"`. |
| `TestWorkspaceBindingRepo_Upsert_UpdatesWhenExists` | `WorkspaceBindingRepository > upsert > updates the workspace path when the binding already exists` | `Create("123456", "proj-a")` $\rightarrow$ `Upsert("123456", "proj-b")` | Retorna `WorkspacePath == "proj-b"` e `FindAll()` possui comprimento `1`. |

---

### 6.3. `internal/database/template_repo_test.go` (Mapeado de [`tests/database/templateRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/templateRepository.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestTemplateRepo_TableInitialization` | `TemplateRepository > table initialization > creates the templates table on initialization` | `OpenInMemory()` + `NewTemplateRepository(db)` | Tabela `templates` existe em `sqlite_master`. |
| `TestTemplateRepo_Create_SavesNameAndPrompt` | `TemplateRepository > create - save template > saves the template name and prompt, and returns the ID` | `Create({Name: "PR作成", Prompt: "PRを作成してください。変更点をまとめてください。"})` | `ID > 0`, `Name == "PR作成"`, `Prompt == "PRを作成してください。変更点をまとめてください。"`. |
| `TestTemplateRepo_Create_MultipleTemplates` | `TemplateRepository > create - save template > can save multiple templates` | Cria `"PR作成"` e `"エラー調査"` | `FindAll()` retorna 2 registros. |
| `TestTemplateRepo_Create_DuplicateNameThrows` | `TemplateRepository > create - save template > cannot save duplicate templates with the same name` | Duas chamadas `Create` com `Name: "PR作成"` | Segunda chamada retorna erro (`err != nil`). |
| `TestTemplateRepo_FindAll_RetrievesAll` | `TemplateRepository > findAll > retrieves all saved templates` | Cria `"PR作成"` e `"エラー調査"` $\rightarrow$ `FindAll()` | Retorna 2 itens na ordem `"PR作成"`, `"エラー調査"`. |
| `TestTemplateRepo_FindAll_Empty` | `TemplateRepository > findAll > returns an empty array when there are no templates` | `FindAll()` | Retorna slice vazio `[]TemplateRecord{}`. |
| `TestTemplateRepo_FindByName_Existing` | `TemplateRepository > findByName > retrieves the template with the specified name` | Cria `"PR作成"` $\rightarrow$ `FindByName("PR作成")` | Retorna registro não-nulo com `Name == "PR作成"`. |
| `TestTemplateRepo_FindByName_NonExistent` | `TemplateRepository > findByName > returns undefined for a non-existent name` | `FindByName("存在しないテンプレート")` | Retorna `nil, nil`. |
| `TestTemplateRepo_DeleteByName_Existing` | `TemplateRepository > delete > deletes the template with the specified name` | Cria `"PR作成"` $\rightarrow$ `DeleteByName("PR作成")` | Retorna `true` e `FindByName("PR作成")` retorna `nil`. |
| `TestTemplateRepo_DeleteByName_NonExistent` | `TemplateRepository > delete > returns false when deleting a non-existent name` | `DeleteByName("存在しない")` | Retorna `false`. |
| `TestTemplateRepo_UpdateByName_UpdatesPrompt` | `TemplateRepository > update > can update the prompt` | Cria `"PR作成"` $\rightarrow$ `UpdateByName("PR作成", {Prompt: ptr("新しいPR作成プロンプト")})` | Retorna `true` e `FindByName("PR作成").Prompt == "新しいPR作成プロンプト"`. |
| `TestTemplateRepo_UpdateByName_NonExistent` | `TemplateRepository > update > returns false when updating a non-existent name` | `UpdateByName("存在しない", {Prompt: ptr("新しいプロンプト")})` | Retorna `false`. |

---

### 6.4. `internal/database/schedule_repo_test.go` (Mapeado de [`tests/database/scheduleRepository.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/database/scheduleRepository.test.ts))

| Função de Teste em Go | Caso Original (`describe` / `it`) | Preparação / Entrada | Asserção / Critério de Aceite Exato |
| :--- | :--- | :--- | :--- |
| `TestScheduleRepo_TableInitialization` | `ScheduleRepository > table initialization > creates the schedules table on initialization` | `OpenInMemory()` + `NewScheduleRepository(db)` | Tabela `schedules` existe em `sqlite_master`. |
| `TestScheduleRepo_Create_SavesScheduleAndReturnsID` | `ScheduleRepository > create - save schedule > saves the cron expression and prompt, and returns the ID` | `Create({CronExpression: "0 9 * * *", Prompt: "テストプロンプト: ビルドを実行", WorkspacePath: "/home/user/project", Enabled: true})` | `ID > 0`, `CronExpression == "0 9 * * *"`, `Prompt == "テストプロンプト: ビルドを実行"`, `WorkspacePath == "/home/user/project"`, `Enabled == true`. |
| `TestScheduleRepo_Create_MultipleSchedules` | `ScheduleRepository > create - save schedule > can save multiple schedules` | Cria 2 schedules | `FindAll()` retorna 2 registros. |
| `TestScheduleRepo_FindAll_RetrievesAll` | `ScheduleRepository > findAll > retrieves all saved schedules` | Cria 1 com `Enabled: true` e 1 com `Enabled: false` $\rightarrow$ `FindAll()` | Retorna 2 registros preservando `Prompt` e `Enabled` (`true` e `false`). |
| `TestScheduleRepo_FindAll_Empty` | `ScheduleRepository > findAll > returns an empty array when there are no schedules` | `FindAll()` | Retorna slice vazio `[]ScheduleRecord{}`. |
| `TestScheduleRepo_FindById_Existing` | `ScheduleRepository > findById > retrieves the schedule with the specified ID` | Cria schedule $\rightarrow$ `FindById(created.ID)` | Retorna registro com `CronExpression == "30 12 * * *"` e `Prompt == "お昼のリマインダー"`. |
| `TestScheduleRepo_FindById_NonExistent` | `ScheduleRepository > findById > returns undefined for a non-existent ID` | `FindById(9999)` | Retorna `nil, nil`. |
| `TestScheduleRepo_FindEnabled_RetrievesOnlyEnabled` | `ScheduleRepository > findEnabled > retrieves only schedules with enabled=true` | Cria `"有効なジョブ"` (`Enabled: true`) e `"無効なジョブ"` (`Enabled: false`) $\rightarrow$ `FindEnabled()` | Retorna exatamente 1 registro (`"有効なジョブ"`). |
| `TestScheduleRepo_Delete_Existing` | `ScheduleRepository > delete > deletes the schedule with the specified ID` | Cria schedule $\rightarrow$ `Delete(created.ID)` | Retorna `true` e `FindById(created.ID)` retorna `nil`. |
| `TestScheduleRepo_Delete_NonExistent` | `ScheduleRepository > delete > returns false when deleting a non-existent ID` | `Delete(9999)` | Retorna `false`. |
| `TestScheduleRepo_Update_ToggleEnabled` | `ScheduleRepository > update > can toggle enabled/disabled state` | Cria com `Enabled: true` $\rightarrow$ `Update(created.ID, {Enabled: ptr(false)})` | Retorna `true` e `FindById(created.ID).Enabled == false`. |
| `TestScheduleRepo_Update_CronExpression` | `ScheduleRepository > update > can update the cron expression` | Cria com `"0 9 * * *"` $\rightarrow$ `Update(created.ID, {CronExpression: ptr("0 12 * * *")})` | Retorna `true` e `FindById(created.ID).CronExpression == "0 12 * * *"`. |
