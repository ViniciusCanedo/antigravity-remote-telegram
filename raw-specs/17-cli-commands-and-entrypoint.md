# 17 — Interface de Linha de Comando (CLI), Entrypoint, Banners ASCII e Verificador de Atualizações

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define, de forma 100% autocontida e exaustiva, o entrypoint unificado do binário Go (`cmd/antigravity-bot/main.go`), a árvore de subcomandos CLI (`internal/cli/`), os banners ASCII e quadros de boas-vindas (`LOGO`, `printWelcome`, `SETUP_LOGO`), os 4 subcomandos (`start`, `setup`, `open`, `doctor` — incluindo o modo de auditoria de seletores DOM `--selectors` do ADR-003) e o serviço não-bloqueante de verificação de atualizações (`updateCheckService.ts`).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo Original (TypeScript) | Linhas | Arquivo de Teste Original (Jest) | Linhas | Destino no Port Go |
| :--- | :---: | :--- | :---: | :--- |
| [`src/index.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/index.ts) | 11 | — | — | `cmd/antigravity-bot/main.go` |
| [`src/bin/cli.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bin/cli.ts) | 54 | — | — | `cmd/antigravity-bot/main.go`<br>`internal/cli/root.go`<br>`internal/cli/root_test.go` |
| [`src/bin/welcome.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bin/welcome.ts) | 53 | — | — | `internal/cli/welcome.go`<br>`internal/cli/welcome_test.go` |
| [`src/utils/logo.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/logo.ts) | 9 | — | — | `internal/cli/logo.go` |
| [`src/bin/commands/start.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bin/commands/start.ts) | 42 | — | — | `internal/cli/start.go`<br>`internal/cli/start_test.go` |
| [`src/bin/commands/setup.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bin/commands/setup.ts) | 281 | — | — | `internal/cli/setup.go`<br>`internal/cli/setup_test.go` |
| [`src/bin/commands/open.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bin/commands/open.ts) | 157 | — | — | `internal/cli/open.go`<br>`internal/cli/open_test.go` |
| [`src/bin/commands/doctor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/bin/commands/doctor.ts)<br>[`src/tools/doctorSelector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/tools/doctorSelector.ts) | 178<br>95 | — | — | `internal/cli/doctor.go`<br>`internal/cli/doctor_test.go` |
| [`src/services/updateCheckService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/updateCheckService.ts) | 118 | [`tests/services/updateCheckService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/updateCheckService.test.ts) | 276 | `internal/cli/update_check.go`<br>`internal/cli/update_check_test.go` |

### 1.2. Documentação Arquitetural e ADRs Relacionados
- [`docs/06-banco-de-dados-config-e-cli.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md)
- [`docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md):
  - **ADR-001**: Validação multiplataforma (`darwin`, `linux`, `windows`) do executável do Antigravity no subcomando `doctor`.
  - **ADR-003**: Unificação dos entrypoints `src/index.ts` e `src/bin/cli.ts` em `cmd/antigravity-bot/main.go` usando `spf13/cobra`, preservando `printWelcome()`, `LOGO`, `.bot.lock`, `checkForUpdates()` e incorporando `doctor --selectors`.

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Entrypoint Principal (`cmd/antigravity-bot/main.go`)

```go
package main

import (
	"os"

	"antigravity-telegram-remote/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
```

---

### 2.2. Comando Raiz e Flags Globais (`internal/cli/root.go`)

```go
package cli

import (
	"github.com/spf13/cobra"
	"antigravity-telegram-remote/internal/utils"
)

// Version armazena a versão semântica do pacote (padrão "0.2.12", injetável via -ldflags "-X ...").
var Version = "0.2.12"

// RootOptions representa as flags globais da CLI (--verbose e --quiet).
type RootOptions struct {
	Verbose bool
	Quiet   bool
}

// NewRootCmd constrói o comando raiz "antigravity-telegram-remote" e registra start, setup, open e doctor.
func NewRootCmd() *cobra.Command

// Execute executa o comando raiz.
func Execute() error

// ResolveCliLogLevel resolve a precedência de nível de log: --verbose ("debug") > --quiet ("error") > nil.
func ResolveCliLogLevel(opts RootOptions) *utils.LogLevel
```

---

### 2.3. Banners ASCII (`internal/cli/logo.go` & `internal/cli/welcome.go`)

```go
package cli

import "io"

// Logo é a constante ASCII art exibida antes de adquirir o lockfile no comando start.
const Logo = `
    ___     _   _  _____  ___  ____  ___     _  _ 
   / _ \   | \ | ||_   _||_ _||  _ \|_ _|   | \/ |
  / /_\ \  |  \| |  | |   | | | |_) || |    |    |
 / /   \ \ | |\  |  | |   | | |  _ < | |    | || |
/_/     \_\|_| \_|  |_|  |___||_| \_\|___|  |_||_|

           QUAN NGUYEN - TNM
`

// VisibleLength retorna o comprimento em caracteres visíveis ignorando sequências de escape ANSI (\x1b\[[0-9]+m).
func VisibleLength(s string) int

// FormatWelcomeBox constrói a string completa do quadro de boas-vindas de largura 50 para version.
func FormatWelcomeBox(version string) string

// PrintWelcome imprime FormatWelcomeBox(Version) em stdout (ou no io.Writer configurado).
func PrintWelcome(w ...io.Writer)
```

---

### 2.4. Subcomando `start` (`internal/cli/start.go`)

```go
package cli

import "context"

// StartDeps permite injetar mocks em testes unitários para o fluxo de inicialização.
type StartDeps struct {
	AcquireLock     func() error
	CheckForUpdates func(ctx context.Context, currentVersion string)
	StartBot        func(ctx context.Context, cliLogLevel *utils.LogLevel) error
}

// StartAction executa a resolução de log level, imprime LOGO, adquire .bot.lock, dispara CheckForUpdates em goroutine e inicia StartBot.
func StartAction(ctx context.Context, opts RootOptions, deps *StartDeps) error
```

---

### 2.5. Subcomando `setup` (`internal/cli/setup.go`)

```go
package cli

import "context"

const TotalSetupSteps = 3

// BotInfo representa os dados retornados por GET https://api.telegram.org/bot<token>/getMe.
type BotInfo struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// SetupResult contém os 3 valores coletados pelo wizard interativo e salvos em config.json.
type SetupResult struct {
	TelegramBotToken string
	AllowedUserIDs   []string
	WorkspaceBaseDir string
}

// IsNonEmpty verifica se strings.TrimSpace(value) != "".
func IsNonEmpty(value string) bool

// IsNumericString verifica se strings.TrimSpace(value) casa com ^\d+$.
func IsNumericString(value string) bool

// ParseAllowedUserIDs divide por vírgula, aplica TrimSpace e remove entradas vazias.
func ParseAllowedUserIDs(raw string) []string

// ValidateAllowedUserIDs valida se há ao menos um ID e se todos são numéricos, retornando a mensagem de erro em vietnamita ou "".
func ValidateAllowedUserIDs(raw string) string

// ExpandTilde expande "~" ou "~/..." para os.UserHomeDir().
func ExpandTilde(raw string) string

// VerifyTelegramToken consulta https://api.telegram.org/bot<token>/getMe com timeout de 10000ms.
func VerifyTelegramToken(ctx context.Context, token string) *BotInfo

// DirectoryCompleter retorna os subdiretórios candidatos para autocompletar com Tab no Passo 3/3.
func DirectoryCompleter(line string) (candidates []string, origLine string)

// SetupAction executa o wizard interativo de 3 etapas e persiste o resultado via ConfigLoader.Save.
func SetupAction(ctx context.Context) error
```

---

### 2.6. Subcomando `open` (`internal/cli/open.go`)

```go
package cli

import "context"

const AppName = "Antigravity"

// IsPortAvailable verifica se a porta TCP em 127.0.0.1 está livre tentando abrir um net.Listen("tcp", "127.0.0.1:<port>").
func IsPortAvailable(port int) bool

// FindAvailablePort percorre CDP_PORTS (9222, 9223, 9333, 9444, 9555, 9666) e retorna a primeira porta livre (ou 0 se todas ocupadas).
func FindAvailablePort(ports []int) int

// OpenMacOS abre o Antigravity no macOS via ANTIGRAVITY_PATH ou `open -a Antigravity --args --remote-debugging-port=<port>`.
func OpenMacOS(port int) error

// OpenWindows abre o Antigravity no Windows com `--remote-debugging-port=<port>`.
func OpenWindows(port int) error

// OpenLinux abre o Antigravity no Linux com `--remote-debugging-port=<port>`.
func OpenLinux(port int) error

// OpenAction executa a busca de porta livre e abre o aplicativo Antigravity de acordo com o SO.
func OpenAction(ctx context.Context) error
```

---

### 2.7. Subcomando `doctor` (`internal/cli/doctor.go`)

```go
package cli

import "context"

// DoctorOptions controla as flags do subcomando doctor (incluindo --selectors do ADR-003).
type DoctorOptions struct {
	Selectors bool
}

// EnvVarCheckStatus representa o estado de uma variável obrigatória.
type EnvVarCheckStatus struct {
	Name string
	Set  bool
}

// CheckPort faz GET http://127.0.0.1:<port>/json/list com timeout de 2000ms e verifica se o corpo é um JSON Array válido.
func CheckPort(ctx context.Context, port int) bool

// CheckEnvFile verifica se o arquivo .env existe no diretório de trabalho atual (os.Getwd()).
func CheckEnvFile() (exists bool, envPath string)

// CheckRequiredEnvVars verifica TELEGRAM_BOT_TOKEN e ALLOWED_USER_IDS tanto no ambiente quanto em config.json.
func CheckRequiredEnvVars() []EnvVarCheckStatus

// DoctorAction executa as 7 verificações de ambiente (ou RunDoctorSelectors se opts.Selectors == true).
func DoctorAction(ctx context.Context, opts DoctorOptions) error

// RunDoctorSelectors audita os 7 seletores DOM em RESPONSE_SELECTORS via CDP (portado de src/tools/doctorSelector.ts).
func RunDoctorSelectors(ctx context.Context) error
```

---

### 2.8. Verificador de Atualizações (`internal/cli/update_check.go`)

```go
package cli

import (
	"context"
	"time"
)

const (
	ConfigDirName        = ".antigravity-telegram-remote"
	UpdateCheckFile      = "update-check.json"
	CooldownMs           = 24 * 60 * 60 * 1000 // 86400000 ms (24 horas)
	CooldownDuration     = 24 * time.Hour
	RegistryURL          = "https://registry.npmjs.org/antigravity-telegram-remote/latest"
	RequestTimeoutMs     = 5000
	RequestTimeout       = 5 * time.Second
)

// UpdateCheckCache modela o JSON salvo em ~/.antigravity-telegram-remote/update-check.json.
type UpdateCheckCache struct {
	LastCheck int64 `json:"lastCheck"` // Unix timestamp em milissegundos (Date.now())
}

// ShouldCheckForUpdates retorna true se o arquivo de cache não existir, estiver corrompido ou se Date.now() - lastCheck >= CooldownMs.
func ShouldCheckForUpdates(homeDirFunc ...func() string) bool

// FetchLatestVersion consulta RegistryURL com timeout de 5000ms e extrai o campo "version" do JSON retornado.
func FetchLatestVersion(ctx context.Context, registryURL ...string) (string, error)

// CompareSemver compara duas strings semver "X.Y.Z" retornando -1 se a < b, 0 se a == b, e 1 se a > b.
func CompareSemver(a, b string) int

// CheckForUpdates executa a verificação não-bloqueante respeitando o cooldown de 24h e imprime o aviso se houver versão mais nova.
func CheckForUpdates(ctx context.Context, currentVersion string)
```

---

## 3. Constantes, Templates HTML/ASCII, Prefixos de Callback e Expressões Regulares Literais

### 3.1. Códigos de Escape ANSI (`welcome.ts`, `setup.ts`, `open.ts`, `doctor.ts`)
- `reset`: `"\x1b[0m"`
- `bold`: `"\x1b[1m"`
- `dim`: `"\x1b[2m"`
- `red`: `"\x1b[31m"`
- `green`: `"\x1b[32m"`
- `yellow`: `"\x1b[33m"`
- `magenta`: `"\x1b[35m"`
- `cyan`: `"\x1b[36m"`

---

### 3.2. Banner `LOGO` Literal (`src/utils/logo.ts`)

```text

    ___     _   _  _____  ___  ____  ___     _  _ 
   / _ \   | \ | ||_   _||_ _||  _ \|_ _|   | \/ |
  / /_\ \  |  \| |  | |   | | | |_) || |    |    |
 / /   \ \ | |\  |  | |   | | |  _ < | |    | || |
/_/     \_\|_| \_|  |_|  |___||_| \_\|___|  |_||_|

           QUAN NGUYEN - TNM

```

---

### 3.3. Quadro `printWelcome()` Literal (`src/bin/welcome.ts`, linhas 17–53)

- Largura interna: `width = 50`
- Topo: `╭──────────────────────────────────────────────────╮` (`╭` + 50 `─` + `╮`)
- Base: `╰──────────────────────────────────────────────────╯` (`╰` + 50 `─` + `╯`)
- Linha vazia: `│                                                  │` (`│` + 50 espaços + `│`)
- Função de preenchimento: ``pad(s) = `│  ${s}${''.padEnd(50 - visibleLength(s) - 4)}  │` ``
- Linhas exatas (incluindo linha em branco inicial e final):
  1. `""`
  2. `borderTop`
  3. `empty`
  4. `pad("\x1b[36m            ,_, \x1b[0m")` *(nota: sem espaço após `_,`: `"\x1b[36m            ,_,\x1b[0m"`)*
  5. `pad("\x1b[36m           (O,O)\x1b[0m")`
  6. `pad("\x1b[36m           (   )\x1b[0m")`
  7. `pad("\x1b[36m           -\"-\"-\x1b[0m")`
  8. `empty`
  9. `pad("\x1b[1mAntigravity Telegram Remote\x1b[0m v" + version)`
  10. `pad("\x1b[2mControl Antigravity from your phone\x1b[0m")`
  11. `empty`
  12. `pad("\x1b[33mQuick start:\x1b[0m")`
  13. `pad("  \x1b[1m1.\x1b[0m \x1b[32mantigravity-telegram-remote setup\x1b[0m   \x1b[2m— configure Telegram bot\x1b[0m")`
  14. `pad("  \x1b[1m2.\x1b[0m \x1b[32mantigravity-telegram-remote open\x1b[0m    \x1b[2m— launch Antigravity + CDP\x1b[0m")`
  15. `pad("  \x1b[1m3.\x1b[0m \x1b[32mantigravity-telegram-remote start\x1b[0m   \x1b[2m— start the bot\x1b[0m")`
  16. `empty`
  17. `pad("\x1b[2mTroubleshoot: \x1b[0m\x1b[32mantigravity-telegram-remote doctor\x1b[0m")`
  18. `pad("\x1b[2mGitHub: \x1b[0m\x1b[36mgithub.com/optimistengineer/remoat\x1b[0m")`
  19. `empty`
  20. `borderBottom`
  21. `""`

---

### 3.4. Descrições dos Comandos e Flags em `src/bin/cli.ts`

- Nome do programa: `'antigravity-telegram-remote'`
- Descrição raiz: `'Điều khiển trợ lý lập trình AI của bạn từ Telegram'`
- Flag `--verbose`: `'Hiển thị nhật ký cấp độ debug'`
- Flag `--quiet`: `'Chỉ hiển thị lỗi'`
- Subcomando `start`: `'Bắt đầu bot Telegram'`
- Subcomando `doctor`: `'Kiểm tra môi trường và các phụ thuộc'`
- Subcomando `setup`: `'Trình hướng dẫn thiết lập tương tác'`
- Subcomando `open`: `'Mở Antigravity với CDP được bật (tự động chọn cổng khả dụng)'`

---

### 3.5. Textos e Prompts Literais do Wizard `setup` (`src/bin/commands/setup.ts`)

- **`SETUP_LOGO` (linhas 20–27)**:
  ```text

  \x1b[36m            ,_,\x1b[0m
  \x1b[36m           (O,O)\x1b[0m
  \x1b[36m           (   )\x1b[0m
  \x1b[36m           -"-"-\x1b[0m

       \x1b[1m~ Cài đặt Antigravity Telegram Remote ~\x1b[0m
  ```
- **Título do Wizard**: `  \x1b[1mHướng dẫn cài đặt — 3 bước\x1b[0m\n`
- **Passo 1/3**:
  - Cabeçalho: `  \x1b[36m[Bước 1/3]\x1b[0m \x1b[1mTelegram Bot Token\x1b[0m`
  - Hints:
    - `  \x1b[2m1. Mở Telegram và tìm kiếm @BotFather\x1b[0m`
    - `  \x1b[2m2. Gửi lệnh /newbot và làm theo hướng dẫn để tạo bot mới\x1b[0m`
    - `  \x1b[2m3. Sao chép mã bot token mà BotFather cung cấp cho bạn\x1b[0m`
  - Erro de token vazio: `  \x1b[31mToken không được để trống.\x1b[0m\n`
  - Status validando: `  \x1b[2mĐang xác thực token...\x1b[0m`
  - Sucesso na validação: `\r  \x1b[32mĐã xác thực!\x1b[0m Bot: \x1b[1m@${botInfo.username}\x1b[0m (${botInfo.first_name})\n`
  - Falha/Offline na validação: `\r  \x1b[33mKhông thể xác thực trực tuyến\x1b[0m — sử dụng token như hiện tại.\n`
- **Passo 2/3**:
  - Cabeçalho: `  \x1b[36m[Bước 2/3]\x1b[0m \x1b[1mID người dùng Telegram được phép\x1b[0m`
  - Hints:
    - `  \x1b[2mChỉ những người dùng này mới có thể gửi lệnh cho bot.\x1b[0m`
    - `  \x1b[2m1. Mở Telegram và tìm kiếm @raw_data_bot\x1b[0m`
    - `  \x1b[2m2. Gửi bất kỳ tin nhắn nào — nó sẽ trả lời bằng ID người dùng (số) của bạn\x1b[0m`
    - `  \x1b[2mNhiều ID: phân tách bằng dấu phẩy (ví dụ: 123456,789012)\x1b[0m`
  - Mensagens de validação (`validateAllowedUserIds`):
    - Lista vazia: `'Vui lòng nhập ít nhất một ID người dùng.'`
    - ID não numérico: `` `ID người dùng không hợp lệ: "${invalid}" — phải là một chuỗi số.` ``
- **Passo 3/3**:
  - Cabeçalho: `  \x1b[36m[Bước 3/3]\x1b[0m \x1b[1mThư mục gốc của Workspace\x1b[0m`
  - Hints:
    - `  \x1b[2mThư mục cha chứa các dự án code của bạn.\x1b[0m`
    - `  \x1b[2mMỗi thư mục con sẽ trở thành một dự án có thể chọn trên Telegram qua lệnh /project.\x1b[0m`
    - `  \x1b[2mBạn có thể thay đổi mục này sau tại ~/.antigravity-telegram-remote/config.json hoặc chạy lại lệnh setup.\x1b[0m`
    - `  \x1b[2mNhấn Tab để tự động hoàn thành đường dẫn thư mục.\x1b[0m`
  - Prompt com diretório padrão: `  \x1b[33m>\x1b[0m [\x1b[2m${defaultDir}\x1b[0m] `
  - Confirmação de criação de diretório inexistente: `  \x1b[33m"${resolved}" không tồn tại. Tạo thư mục này? (y/n):\x1b[0m `
  - Erro caso recuse criar diretório: `  \x1b[31mVui lòng nhập một thư mục hợp lệ.\x1b[0m\n`
- **Resumo Final (`setupAction`, linhas 273–280)**:
  ```text
    \x1b[32mCài đặt hoàn tất!\x1b[0m

    \x1b[2mĐã lưu tại\x1b[0m {configPath}

    \x1b[36mCác bước tiếp theo:\x1b[0m
    \x1b[1m1.\x1b[0m Mở Antigravity với CDP được bật:
       \x1b[32mantigravity-telegram-remote open\x1b[0m
       \x1b[2m(tự động chọn một cổng khả dụng từ: 9222, 9223, 9333, 9444, 9555, 9666)\x1b[0m

    \x1b[1m2.\x1b[0m Chạy lệnh: \x1b[32mantigravity-telegram-remote start\x1b[0m

    \x1b[1m3.\x1b[0m Mở Telegram và gửi tin nhắn cho bot của bạn!
  ```

---

### 3.6. Textos Literais de `open`, `doctor` e `updateCheckService`

- **`open` (`src/bin/commands/open.ts`, linhas 118–155)**:
  - Procurando porta: `\n  \x1b[36mSearching for an available CDP port...\x1b[0m`
  - Nenhuma porta livre:
    ```text
      \x1b[31mNo available CDP ports found.\x1b[0m
      \x1b[2mAll candidate ports are in use: 9222, 9223, 9333, 9444, 9555, 9666\x1b[0m
      \x1b[2mClose an application using one of these ports and try again.\x1b[0m
    ```
  - Porta encontrada:
    ```text
      \x1b[32mFound available port: {port}\x1b[0m
      \x1b[2mOpening Antigravity with --remote-debugging-port={port}...\x1b[0m
    ```
  - Sucesso ao abrir:
    ```text
      \x1b[32mAntigravity opened on CDP port {port}\x1b[0m
      \x1b[2mRun \x1b[0m\x1b[36mantigravity-telegram-remote start\x1b[0m\x1b[2m to connect the bot.\x1b[0m
    ```
  - Dicas de erro por SO:
    - Geral: `  \x1b[2mMake sure Antigravity is installed on your system.\x1b[0m`
    - `darwin`: `  \x1b[2mAntigravity must be in /Applications, or set ANTIGRAVITY_PATH in your .env file.\x1b[0m`
    - `linux`: `  \x1b[2mSet ANTIGRAVITY_PATH in your .env file (e.g. /opt/applications/antigravity.AppImage).\x1b[0m`
    - `windows`: `  \x1b[2mMake sure Antigravity.exe is in your PATH, or set ANTIGRAVITY_PATH in your .env file.\x1b[0m`
- **`doctor` (`src/bin/commands/doctor.ts`, linhas 11–177)**:
  - Prefixos de linha:
    - `ok`: `  \x1b[32m[OK]\x1b[0m ${msg}`
    - `warn`: `  \x1b[33m[--]\x1b[0m ${msg}`
    - `fail`: `  \x1b[31m[!!]\x1b[0m ${msg}`
    - `hint`: `       \x1b[2m${msg}\x1b[0m`
  - Cabeçalho: `\n\x1b[36mantigravity-telegram-remote doctor\x1b[0m\n`
  - Rodapé de sucesso: `  \x1b[32mAll checks passed!\x1b[0m`
  - Rodapé de falha: `  \x1b[31mSome checks failed. Please fix the issues above.\x1b[0m`
- **`checkForUpdates` (`src/services/updateCheckService.ts`, linhas 111–113)**:
  ```text

    Update available: {currentVersion} → {latest} — run \x1b[36mnpm i -g antigravity-telegram-remote\x1b[0m

  ```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Comportamento Padrão sem Subcomando (`internal/cli/root.go`)
Quando o binário `antigravity-telegram-remote` é invocado sem subcomando:
1. Verifica `hasConfig := config.ConfigExists()` (`~/.antigravity-telegram-remote/config.json`) e `hasEnv := fileExists( filepath.Join(cwd, ".env") )`.
2. Se `!hasConfig && !hasEnv`:
   - Chama `PrintWelcome()`.
   - Executa `SetupAction(ctx)` (entrando diretamente no wizard interativo).
3. Caso contrário (`hasConfig || hasEnv`):
   - Executa `StartAction(ctx, rootOpts, nil)`.

### 4.2. Subcomando `start` (`internal/cli/start.go`)
1. Calcula o nível de log da CLI (`ResolveCliLogLevel`): `--verbose` $\rightarrow$ `"debug"`, `--quiet` $\rightarrow$ `"error"`, caso contrário `nil` (delega ao `.env`/`config.json`).
2. Se definido, aplica `logger.SetLogLevel(*cliLevel)`.
3. Imprime `Logo` em `stdout`.
4. Chama `lockfile.AcquireLock()` (`~/.antigravity-telegram-remote/.bot.lock`), que encerra instâncias anteriores via `SIGTERM` $\rightarrow$ espera de até 5000ms $\rightarrow$ `SIGKILL`.
5. Dispara `go CheckForUpdates(ctx, Version)` de forma assíncrona e não-bloqueante (fire-and-forget, ignorando qualquer erro).
6. Chama `bot.StartBot(ctx, cliLevel)`. Se retornar erro, loga `'Failed to start bot:'` e encerra com código de saída `1`.

### 4.3. Subcomando `setup` (`internal/cli/setup.go`)
1. **Leitura Mascarada de Token (`askSecret`)**:
   - Se `os.Stdin` não for um terminal TTY (`!term.IsTerminal`), lê uma linha normalmente.
   - Se for TTY, coloca o terminal em modo raw (`term.MakeRaw`), exibe o prompt `  \x1b[33m>\x1b[0m ` e lê caractere a caractere:
     - `'\r'` ou `'\n'`: restaura o terminal, imprime `'\n'` e retorna o `input` acumulado.
     - Backspace (`code == 127` ou `code == 8`): se `len(input) > 0`, remove o último byte/runa e escreve `"\b \b"` em `stdout`.
     - `Ctrl+C` (`code == 3`): restaura o terminal, imprime `'\n'` e encerra o processo com `os.Exit(0)`.
     - Caracteres imprimíveis (`code >= 32`): concatena em `input` e escreve `'*'` em `stdout`.
2. **Validação Online do Token (`VerifyTelegramToken`)**:
   - Faz `GET https://api.telegram.org/bot<token>/getMe` com timeout de `10000ms`.
   - Se `statusCode == 200` e `json.ok == true`, retorna `&BotInfo{ID, Username, FirstName}` e imprime `Đã xác thực! Bot: @<username> (<first_name>)`.
   - Se falhar por erro de rede, timeout ou status `!= 200`, imprime `Không thể xác thực trực tuyến — sử dụng token như hiện tại.` e aceita o token informado.
3. **Autocompletar de Diretórios (`DirectoryCompleter`)**:
   - Expande `~` / `~/` e resolve o caminho absoluto.
   - Se o caminho existir e for diretório, lista seus subdiretórios; caso contrário, lista os subdiretórios de `filepath.Dir(resolved)` cujo nome comece (case-insensitive) com `filepath.Base(resolved)`.
   - Formata cada candidato terminando com `'/'` (e preservando o prefixo `'~/'` se o usuário digitou `'~/'`).

### 4.4. Subcomando `open` (`internal/cli/open.go`)
1. Itera sobre `CDP_PORTS = [9222, 9223, 9333, 9444, 9555, 9666]` testando `IsPortAvailable(port)` (abre um listener TCP em `127.0.0.1:<port>` e fecha imediatamente).
2. Se nenhuma porta estiver livre, imprime a mensagem de erro em vermelho e define código de saída `1`.
3. Na primeira porta livre encontrada:
   - **macOS (`darwin`)**: se `ANTIGRAVITY_PATH` estiver definido, inicia o binário em processo destacado (`--remote-debugging-port=<port>`) e aguarda `500ms`; caso contrário executa `open -a Antigravity --args --remote-debugging-port=<port>`.
   - **Windows (`windows`)**: resolve `GetAntigravityCliPath()`, inicia em processo destacado com `--remote-debugging-port=<port>` e aguarda `500ms`.
   - **Linux (`linux`)**: usa `ANTIGRAVITY_PATH` ou `"antigravity"`, inicia em processo destacado com `--remote-debugging-port=<port>` e aguarda `500ms`.

### 4.5. Subcomando `doctor` (`internal/cli/doctor.go`)
1. Se `--selectors` for informado (ADR-003), conecta ao CDP nas portas de `CDP_PORTS` e avalia os 7 seletores de `RESPONSE_SELECTORS`:
   - `'Chat Message (Text)'`: `RESPONSE_SELECTORS.RESPONSE_TEXT`
   - `'Stop Button (State)'`: `RESPONSE_SELECTORS.STOP_BUTTON`
   - `'Planning Active (Dialog)'`: `RESPONSE_SELECTORS.PLANNING_ACTIVE`
   - `'Process Logs (Activity)'`: `RESPONSE_SELECTORS.PROCESS_LOGS`
   - `'Combined Poll (Logic)'`: `RESPONSE_SELECTORS.COMBINED_POLL`
   - `'Quota Error (Detection)'`: `RESPONSE_SELECTORS.QUOTA_ERROR`
   - `'DOM Observer (Event-driven)'`: `RESPONSE_SELECTORS.DOM_OBSERVER`
   Imprime o relatório `Score: X/7 Healthy`.
2. Sem `--selectors`, executa sequencialmente as **7 checagens**:
   - **Checagem 1**: Existência de `~/.antigravity-telegram-remote` (`[OK]` ou `[--]` com hint `Run: antigravity-telegram-remote setup  (optional if using .env)`).
   - **Checagem 2**: Existência de `~/.antigravity-telegram-remote/config.json` (`[OK]` ou `[--]`).
   - **Checagem 3**: Existência de `.env` no diretório atual. Se existir, carrega suas variáveis no ambiente e imprime `[OK]`. Se não existir e `config.json` também não existir, imprime `[!!]` e marca `allOk = false`; se `config.json` existir, imprime `[--]`.
   - **Checagem 4**: Verifica se `TELEGRAM_BOT_TOKEN` e `ALLOWED_USER_IDS` estão definidos (no ambiente ou em `config.json`). Qualquer um ausente imprime `[!!] <NAME> is NOT set` e marca `allOk = false`.
   - **Checagem 5**: Exibe a versão do runtime (`[OK] Go <runtime.Version()>` / paridade com a checagem de runtime).
   - **Checagem 6 (Plataforma — ADR-001)**:
     - Se `ANTIGRAVITY_PATH` estiver definido: verifica se o arquivo existe (`[OK] Antigravity found: <path>` ou `[!!] ANTIGRAVITY_PATH set but not found: <path>` com `allOk = false`).
     - Se `ANTIGRAVITY_PATH` não estiver definido: verifica a instalação padrão do SO (`/Applications/Antigravity.app` no macOS, `%LOCALAPPDATA%\Programs\Antigravity\Antigravity.exe` no Windows, ou `exec.LookPath("antigravity")` no Linux).
   - **Checagem 7 (Portas CDP)**: sonda `http://127.0.0.1:<port>/json/list` (timeout `2000ms`) para cada porta em `CDP_PORTS`. Se ao menos uma responder com JSON array, imprime `[OK] CDP port <port> is responding`. Se nenhuma responder, imprime `[!!] No CDP ports responding`, as duas hints (`Run: antigravity-telegram-remote open` e `Or manually: <getAntigravityCdpHint(9222)>`) e marca `allOk = false`.
3. Se `!allOk`, retorna erro / define código de saída `1`.

### 4.6. Algoritmo de `CheckForUpdates` (`internal/cli/update_check.go`)
1. `ShouldCheckForUpdates()`:
   - Caminho do cache: `filepath.Join(homeDir, ".antigravity-telegram-remote", "update-check.json")`.
   - Se o arquivo não existir, falhar na leitura ou contiver JSON inválido, retorna `true`.
   - Caso contrário, retorna `time.Now().UnixMilli() - cache.LastCheck >= CooldownMs` (`86400000`).
2. `FetchLatestVersion()`:
   - Faz `GET https://registry.npmjs.org/antigravity-telegram-remote/latest` com timeout de `5000ms`.
   - Se `statusCode != 200`, retorna erro `"HTTP <statusCode>"`.
   - Se der timeout, retorna erro `"Request timed out"`.
   - Decodifica o JSON e retorna `data.Version`.
3. `CheckForUpdates(currentVersion)`:
   - Se `!ShouldCheckForUpdates()`, retorna imediatamente.
   - Chama `latest, err := FetchLatestVersion()`. Se houver erro, retorna silenciosamente sem gravar o cache.
   - Grava o cache `{ "lastCheck": time.Now().UnixMilli() }` em `~/.antigravity-telegram-remote/update-check.json` (criando o diretório pai se necessário e ignorando falhas de escrita).
   - Se `CompareSemver(currentVersion, latest) < 0`, imprime o aviso de atualização.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Verificação de Atualizações Não-Bloqueante**:
   - Em `StartAction`, `CheckForUpdates` roda em uma goroutine independente (`go CheckForUpdates(ctx, Version)`) com timeout HTTP estrito de `5000ms`, garantindo que lentidão ou ausência de rede nunca atrase a inicialização do bot Telegram.
2. **Garantia de Instância Única via Lockfile**:
   - `AcquireLock()` é invocado antes de `StartBot` e registra limpeza automática do arquivo `~/.antigravity-telegram-remote/.bot.lock` no encerramento do processo.
3. **Restauração Segura do Terminal em `setup`**:
   - Durante `askSecret`, o estado anterior do descritor de arquivo `os.Stdin` (`term.GetState`) é salvo e sempre restaurado via `defer term.Restore(...)` bem como no tratamento de `Ctrl+C` (`0x03`), prevenindo que o terminal do usuário permaneça em modo raw após saída abrupta.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os testes de `tests/services/updateCheckService.test.ts` e os testes unitários dos comandos da CLI devem ser implementados e passar com `go test -race ./internal/cli/...`:

### 6.1. `internal/cli/update_check_test.go` (espelhando `tests/services/updateCheckService.test.ts`)
- **Suíte `ShouldCheckForUpdates`**:
  - `TestShouldCheckForUpdates_NoCacheFile`: retorna `true` quando `update-check.json` não existe.
  - `TestShouldCheckForUpdates_OldTimestamp`: retorna `true` quando `lastCheck` é `now - CooldownMs - 1000`.
  - `TestShouldCheckForUpdates_RecentTimestamp`: retorna `false` quando `lastCheck` é `now - 1000`.
  - `TestShouldCheckForUpdates_CorruptedJson`: retorna `true` quando o arquivo contém `"not-json"`.
  - `TestShouldCheckForUpdates_ReadError`: retorna `true` quando a leitura falha por permissão (`EACCES`).
- **Suíte `FetchLatestVersion`**:
  - `TestFetchLatestVersion_Success`: retorna `"1.2.3"` quando o servidor HTTP de teste responde `200 OK` com `{"version":"1.2.3"}`.
  - `TestFetchLatestVersion_Non200`: retorna erro contendo `"HTTP 404"` quando status é `404`.
  - `TestFetchLatestVersion_InvalidJSON`: retorna erro quando o corpo 200 não é JSON válido.
  - `TestFetchLatestVersion_NetworkError`: retorna erro quando a conexão falha (`ECONNREFUSED`).
  - `TestFetchLatestVersion_Timeout`: retorna erro contendo `"timed out"` quando excede o timeout.
- **Suíte `CheckForUpdates`**:
  - `TestCheckForUpdates_NewerAvailable`: quando `currentVersion = "0.1.0"` e registry retorna `"99.0.0"`, imprime aviso contendo `"Update available"` e `"99.0.0"`, e grava `lastCheck` no cache.
  - `TestCheckForUpdates_SameVersion`: quando ambos são `"0.1.0"`, não imprime aviso, mas grava o arquivo de cache (`TestCheckForUpdates_WritesCacheEvenWhenVersionsMatch`).
  - `TestCheckForUpdates_LocalNewer`: quando `currentVersion = "0.1.0"` e registry retorna `"0.0.9"`, não imprime aviso.
  - `TestCheckForUpdates_SkipsWithinCooldown`: quando o cache é recente, não realiza requisição HTTP nem imprime aviso.
  - `TestCheckForUpdates_SwallowsNetworkErrors`: quando a requisição falha, não propaga erro nem imprime aviso.

### 6.2. `internal/cli/welcome_test.go` & `internal/cli/root_test.go`
- `TestVisibleLength`: calcula comprimento exato removendo sequências ANSI `\x1b[...m`.
- `TestFormatWelcomeBox`: verifica que todas as linhas com borda possuem exatamente a mesma largura visível (`52` runas contando `╭`/`│`/`╰` e `╮`/`│`/`╯`) e contêm a versão atual e os 4 comandos (`setup`, `open`, `start`, `doctor`).
- `TestResolveCliLogLevel`: verifica precedência `--verbose` (`"debug"`) $>$ `--quiet` (`"error"`) $>$ `nil`.

### 6.3. `internal/cli/setup_test.go`, `internal/cli/open_test.go` & `internal/cli/doctor_test.go`
- `TestValidateAllowedUserIDs`: rejeita string vazia (`"Vui lòng nhập ít nhất một ID người dùng."`), rejeita IDs alfanuméricos (`'ID người dùng không hợp lệ: "abc" — phải là một chuỗi số.'`) e aceita `"123456, 789012"`.
- `TestDirectoryCompleter`: autocompleta subdiretórios reais em diretório temporário e preserva prefixo `~/`.
- `TestFindAvailablePort`: detecta portas ocupadas por um `net.Listener` local e retorna a primeira porta livre de `CDP_PORTS`.
- `TestDoctorChecks`: valida detecção de variáveis obrigatórias via `.env` e `config.json`, sondagem HTTP `/json/list` nas portas CDP e código de saída `1` quando qualquer verificação obrigatória falha.
