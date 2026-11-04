## ADDED Requirements

### Requirement: Pure-Go SQLite Database Initialization
The persistence engine SHALL initialize SQLite databases using the pure-Go driver `modernc.org/sqlite` without requiring CGO, supporting file paths and `:memory:`, ensuring parent directories exist, and applying WAL mode and busy timeout.

#### Scenario: Open SQLite database from file path
- **WHEN** `Open(dbPath)` is invoked with a file path
- **THEN** parent directories are created if missing, SQLite connection is opened, and `PRAGMA journal_mode = WAL;` is applied

#### Scenario: Open isolated in-memory SQLite database
- **WHEN** `OpenInMemory()` is invoked
- **THEN** an isolated in-memory SQLite connection is returned with `MaxOpenConns` set to 1

### Requirement: Chat Session Table Initialization
The `ChatSessionRepository` SHALL automatically initialize the `chat_sessions` table schema if it does not already exist upon repository creation.

#### Scenario: Table creation on repository instantiation
- **WHEN** `NewChatSessionRepository(db)` is called
- **THEN** table `chat_sessions` exists in `sqlite_master` with columns `id`, `channel_id`, `category_id`, `workspace_path`, `session_number`, `display_name`, `is_renamed`, `guild_id`, and `created_at`

### Requirement: Chat Session Creation and Retrieval
The `ChatSessionRepository` SHALL allow creating session records with auto-incrementing IDs and looking them up by channel ID.

#### Scenario: Create session record with auto-increment ID and defaults
- **WHEN** `Create` is called with valid `CreateChatSessionInput`
- **THEN** a new row is inserted, returning a `ChatSessionRecord` with positive `ID`, `DisplayName == nil`, `IsRenamed == false`, and matching input fields

#### Scenario: Disallow duplicate channel IDs
- **WHEN** `Create` is called with a `channel_id` that already exists
- **THEN** an error is returned due to the unique constraint violation

#### Scenario: Retrieve session by channel ID
- **WHEN** `FindByChannelId` is called with an existing `channel_id`
- **THEN** the corresponding `ChatSessionRecord` pointer is returned

#### Scenario: Handle non-existent channel ID lookup
- **WHEN** `FindByChannelId` is called with an unknown `channel_id`
- **THEN** `nil, nil` is returned without error

### Requirement: Chat Session Categorization and Numbering
The `ChatSessionRepository` SHALL list sessions by category ordered by session number and compute the next sequential session number scoped to that category.

#### Scenario: Retrieve sessions by category ordered by session number
- **WHEN** `FindByCategoryId` is called with a `category_id`
- **THEN** all matching sessions are returned in ascending order of `session_number`, or an empty slice if none exist

#### Scenario: Calculate next session number per category
- **WHEN** `GetNextSessionNumber` is called for a category with no sessions
- **THEN** `1` is returned

#### Scenario: Increment next session number per category
- **WHEN** `GetNextSessionNumber` is called for a category containing existing sessions
- **THEN** the maximum `session_number` in that category plus 1 is returned

### Requirement: Chat Session Modification and Deletion
The `ChatSessionRepository` SHALL support updating display names, querying by display name within a workspace, and deleting sessions by channel ID.

#### Scenario: Update display name and mark session as renamed
- **WHEN** `UpdateDisplayName` is called with an existing `channel_id` and new title
- **THEN** `true` is returned, `display_name` is updated, and `is_renamed` is set to `true`

#### Scenario: Find session by display name and workspace path
- **WHEN** `FindByDisplayName` is called with matching `workspace_path` and `display_name`
- **THEN** the most recently created matching `ChatSessionRecord` is returned

#### Scenario: Delete session by channel ID
- **WHEN** `DeleteByChannelId` is called
- **THEN** the row is deleted and `true` is returned if it existed, or `false` if not found

### Requirement: Workspace Binding Table Initialization
The `WorkspaceBindingRepository` SHALL automatically initialize the `workspace_bindings` table schema upon repository creation.

#### Scenario: Workspace bindings table creation
- **WHEN** `NewWorkspaceBindingRepository(db)` is called
- **THEN** table `workspace_bindings` exists with columns `id`, `channel_id` (UNIQUE), `workspace_path`, `guild_id`, and `created_at`

### Requirement: Workspace Binding Operations
The `WorkspaceBindingRepository` SHALL allow creating, querying, deleting, and upserting channel-to-workspace bindings.

#### Scenario: Create workspace binding
- **WHEN** `Create` is called with unique `channel_id`, `workspace_path`, and `guild_id`
- **THEN** a record with positive `ID` and matching fields is returned

#### Scenario: Disallow duplicate binding creation
- **WHEN** `Create` is called with an existing `channel_id`
- **THEN** an error is returned due to unique constraint

#### Scenario: Find binding by channel ID
- **WHEN** `FindByChannelId` is called
- **THEN** the matching record pointer is returned, or `nil, nil` if not found

#### Scenario: Find bindings by guild ID
- **WHEN** `FindByGuildId` is called
- **THEN** all bindings matching the `guild_id` are returned sorted by `id ASC`, or an empty slice if none

#### Scenario: Find bindings by workspace path and guild ID
- **WHEN** `FindByWorkspacePathAndGuildId` is called
- **THEN** all matching records are returned sorted by `id ASC`

#### Scenario: Find all workspace bindings
- **WHEN** `FindAll` is called
- **THEN** all records in `workspace_bindings` are returned ordered by `id ASC`

#### Scenario: Delete binding by channel ID
- **WHEN** `DeleteByChannelId` is called
- **THEN** `true` is returned if deleted, `false` if non-existent

#### Scenario: Atomic upsert binding for channel ID
- **WHEN** `Upsert` is called with an existing or new `channel_id`
- **THEN** the binding is inserted or updated on conflict and the full current record is returned

### Requirement: Prompt Template Table Initialization
The `TemplateRepository` SHALL automatically initialize the `templates` table schema upon repository creation.

#### Scenario: Templates table creation
- **WHEN** `NewTemplateRepository(db)` is called
- **THEN** table `templates` exists with columns `id`, `name` (UNIQUE), `prompt`, and `created_at`

### Requirement: Prompt Template CRUD Operations
The `TemplateRepository` SHALL provide CRUD operations for prompt templates identified by unique names.

#### Scenario: Create template with unique name
- **WHEN** `Create` is called with `name` and `prompt`
- **THEN** the template is stored and returned with a positive `ID`

#### Scenario: Disallow duplicate template names
- **WHEN** `Create` is called with an already existing `name`
- **THEN** a unique constraint error is returned

#### Scenario: Find template by ID
- **WHEN** `FindById` is called
- **THEN** the matching record is returned, or `nil, nil` if not found

#### Scenario: Find template by name
- **WHEN** `FindByName` is called
- **THEN** the matching record is returned, or `nil, nil` if not found

#### Scenario: Find all templates ordered by ID
- **WHEN** `FindAll` is called
- **THEN** all templates are returned ordered by `id ASC`

#### Scenario: Update template prompt partially
- **WHEN** `UpdateByName` is called with `input.Prompt != nil`
- **THEN** the prompt is updated and `true` is returned, or `false` if not found / empty input

#### Scenario: Delete template by name
- **WHEN** `DeleteByName` is called
- **THEN** `true` is returned if deleted, or `false` if non-existent

### Requirement: Schedule Table Initialization
The `ScheduleRepository` SHALL automatically initialize the `schedules` table schema upon repository creation.

#### Scenario: Schedules table creation
- **WHEN** `NewScheduleRepository(db)` is called
- **THEN** table `schedules` exists with columns `id`, `cron_expression`, `prompt`, `workspace_path`, `enabled`, and `created_at`

### Requirement: Schedule Management and Queries
The `ScheduleRepository` SHALL support creating, listing, filtering, dynamically updating, and deleting cron-based schedules.

#### Scenario: Create schedule with cron expression and enabled status
- **WHEN** `Create` is called with valid schedule input
- **THEN** the schedule is inserted and returned with positive `ID` and correct boolean `Enabled` status

#### Scenario: Find schedule by ID
- **WHEN** `FindById` is called
- **THEN** the record is returned, or `nil, nil` if not found

#### Scenario: Find all schedules
- **WHEN** `FindAll` is called
- **THEN** all schedules are returned ordered by `id ASC`

#### Scenario: Filter only enabled schedules
- **WHEN** `FindEnabled` is called
- **THEN** only schedules where `enabled = 1` are returned ordered by `id ASC`

#### Scenario: Dynamically update schedule fields
- **WHEN** `Update` is called with any combination of non-nil fields (`CronExpression`, `Prompt`, `WorkspacePath`, `Enabled`)
- **THEN** dynamic SQL update applies only specified fields and returns `true`

#### Scenario: Delete schedule by ID
- **WHEN** `Delete` is called
- **THEN** `true` is returned if deleted, or `false` if not found

### Requirement: Prepared Statement Resource Management
Repositories SHALL manage their prepared statements cleanly and expose a `Close() error` method.

#### Scenario: Close repository statements on shutdown
- **WHEN** `Close()` is called on any repository
- **THEN** all cached prepared statements are closed cleanly without errors
