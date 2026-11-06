## MODIFIED Requirements

### Requirement: Telegram Bot Lifecycle and Core Commands
The `internal/bot` package SHALL provide `Bot` struct, `StartBot`, channel key helpers, pool connection listeners, and command dispatchers.

#### Scenario: Channel key formatting and decomposition
- **WHEN** `ChannelKey` is called with chat ID and thread ID
- **THEN** it formats as `<chatId>:<threadId>` when thread is non-zero, or `<chatId>` when zero

#### Scenario: Command menu registration
- **WHEN** bot starts up
- **THEN** it registers native bot commands with the Telegram API using standard English descriptions
