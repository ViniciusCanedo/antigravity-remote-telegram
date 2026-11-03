# 13 — Especificação Técnica: `QuotaService`, `VoiceHandler` e `ImageHandler` (`internal/quota`, `internal/voice`, `internal/bot`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação define de forma 100% autocontida e exaustiva o port para **Golang (1.23+)** do serviço de consulta de cotas de modelos junto ao `language_server` local do Antigravity (`QuotaService`), do pipeline de download e transcrição local de mensagens de voz via `whisper.cpp` (`VoiceHandler`) e do pipeline bidirecional de anexos de imagem (`ImageHandler`).

### 1.1. Mapeamento de Arquivos Originais (TypeScript) para Destino (Go)

| Componente | Arquivo Original (TypeScript) | Linhas | Arquivo de Teste Original (Jest) | Linhas | Destino no Port Go | Arquivo de Teste Go |
| :--- | :--- | :---: | :--- | :---: | :--- | :--- |
| **Serviço de Cota (`QuotaService`)** | [`src/services/quotaService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/quotaService.ts#L1-L175) | 175 | [`tests/services/quotaService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/quotaService.test.ts#L1-L120) | 120 | `internal/quota/service.go`<br>`internal/platform/proc_unix.go`<br>`internal/platform/proc_windows.go` | `internal/quota/service_test.go` |
| **Handler de Voz (`voiceHandler`)** | [`src/utils/voiceHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/voiceHandler.ts#L1-L165) | 165 | [`tests/utils/voiceHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/voiceHandler.test.ts#L1-L114) | 114 | `internal/voice/transcriber.go` | `internal/voice/transcriber_test.go` |
| **Handler de Imagem (`imageHandler`)** | [`src/utils/imageHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/imageHandler.ts#L1-L141) | 141 | [`tests/utils/imageHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/imageHandler.test.ts#L1-L31) | 31 | `internal/bot/image_handler.go` | `internal/bot/image_handler_test.go` |
| **Documentação e ADRs** | [`docs/05-detectores-e-servicos-background.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md#L89-L110)<br>[`docs/06-banco-de-dados-config-e-cli.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/06-banco-de-dados-config-e-cli.md#L134-L147)<br>[`docs/08-adr-decisoes-de-engenharia.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/08-adr-decisoes-de-engenharia.md#L7-L51) | — | — | — | ADR-001 (`internal/platform`)<br>ADR-004 (`internal/voice`) | — |

### 1.2. Responsabilidades no Sistema

1. **`QuotaService` ([`src/services/quotaService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/quotaService.ts#L23-L175) + ADR-001)**:
   - Localiza o processo em execução `language_server` do Antigravity, extraindo seu `PID` e o argumento `--csrf_token`.
   - Descobre as portas TCP em estado `LISTEN` abertas por esse `PID`.
   - Consulta o endpoint Connect/gRPC local `POST https://127.0.0.1:<port>/exa.language_server_pb.LanguageServerService/GetUserStatus` (com TLS `InsecureSkipVerify: true` e timeout de `2000ms`) enviando o header `X-Codeium-Csrf-Token`.
   - Faz cache de `cachedPid`, `cachedCsrfToken` e `cachedPort`, invalidando `cachedPort` e realizando exatamente 1 retry (`retryCount < 1`) com re-escanemento de portas caso a chamada na porta em cache falhe.

2. **`VoiceHandler` ([`src/utils/voiceHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/voiceHandler.ts#L1-L165) + ADR-004)**:
   - Verifica a disponibilidade do ambiente local de transcrição Whisper (`CheckWhisperAvailability`), retornando mensagens de instrução idênticas às originais caso o pacote/binário `whisper-cli` ou o modelo `base.en` não estejam presentes.
   - Faz o download de arquivos de voz `.ogg`/`.oga` do Telegram para `/tmp/antigravity-telegram-remote-voice/<timestamp>-<file_unique_id><ext>` (`DownloadTelegramVoice`).
   - Converte o áudio para WAV 16kHz mono via `ffmpeg` e executa a transcrição local com `whisper-cli` (`base.en`), removendo marcações de tempo `[00:00:00.000 --> 00:00:02.000]` e garantindo a exclusão do arquivo de áudio temporário no bloco `defer` (`TranscribeVoice`).

3. **`ImageHandler` ([`src/utils/imageHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/imageHandler.ts#L1-L141))**:
   - Valida tipos MIME e extensões de imagem (`IsImageAttachment`), converte MIME types em extensões (`MimeTypeToExtension`) e higieniza nomes de arquivos (`SanitizeFileName`).
   - Formata prompts enriquecidos com metadados de imagens anexadas (`BuildPromptWithAttachmentUrls`).
   - Baixa até `4` fotos por mensagem do Telegram para `/tmp/antigravity-telegram-remote-images/` (`DownloadTelegramImages`) e remove os arquivos temporários após o envio ao CDP (`CleanupInboundImageAttachments`).
   - Converte imagens extraídas da resposta do Antigravity (via Base64 ou URL HTTP/HTTPS) em buffers nomeados prontos para envio ao Telegram (`ToTelegramInputFile`).

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pacote `internal/quota` e `internal/platform` (`internal/quota/service.go`)

```go
package quota

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
)

// QuotaInfo representa a fração restante e o horário de reset da cota de um modelo.
// Origem: interface QuotaInfo (src/services/quotaService.ts#L8-L11)
type QuotaInfo struct {
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

// ModelQuota representa a configuração e o estado de cota de um modelo retornado pelo language_server.
// Origem: interface ModelQuota (src/services/quotaService.ts#L13-L17)
type ModelQuota struct {
	Label     string     `json:"label"`
	Model     string     `json:"model"`
	QuotaInfo *QuotaInfo `json:"quotaInfo,omitempty"`
}

// UserStatusData encapsula a lista de configurações de modelos do usuário.
// Origem: interface UserStatusData (src/services/quotaService.ts#L19-L21)
type UserStatusData struct {
	ClientModelConfigs []ModelQuota `json:"clientModelConfigs,omitempty"`
}

// LanguageServerProcessInfo representa o PID e o CSRF token extraídos do processo language_server.
type LanguageServerProcessInfo struct {
	PID       int
	CSRFToken string
}

// CommandExecutor abstrai a execução de comandos de sistema (pgrep, lsof, ss, netstat, etc.)
// permitindo testes unitários 100% determinísticos (equivalente ao mock de child_process.exec).
type CommandExecutor func(ctx context.Context, command string) (stdout string, stderr string, err error)

// ProcessDiscoverer abstrai a descoberta multiplataforma do processo language_server e suas portas (ADR-001).
type ProcessDiscoverer interface {
	GetProcessInfo(ctx context.Context) (*LanguageServerProcessInfo, error)
	GetListeningPorts(ctx context.Context, pid int) ([]int, error)
}

// Service gerencia a descoberta de porta/token e consulta de cotas do Antigravity.
// Origem: class QuotaService (src/services/quotaService.ts#L23-L175)
type Service struct {
	mu              sync.Mutex
	cachedPort      *int
	cachedCsrfToken *string
	cachedPid       *int

	execCmd    CommandExecutor
	discoverer ProcessDiscoverer
	httpClient *http.Client
	logger     *slog.Logger
}

// Assinaturas públicas e privadas de internal/quota/service.go:
func NewService(opts ...ServiceOption) *Service
func (s *Service) FetchQuota(ctx context.Context) []ModelQuota
func (s *Service) fetchQuotaWithRetry(ctx context.Context, retryCount int) []ModelQuota
func (s *Service) getUnixProcessInfo(ctx context.Context) *LanguageServerProcessInfo
func (s *Service) getListeningPorts(ctx context.Context, pid int) []int
func (s *Service) requestAPI(ctx context.Context, port int, csrfToken string) (*UserStatusData, error)
func ParseUnixProcessInfoOutput(stdout string) *LanguageServerProcessInfo
func ParseLsofListeningPortsOutput(stdout string, pid int) []int
func ParseUserStatusResponseBody(body []byte) (*UserStatusData, error)
```

### 2.2. Pacote `internal/voice` (`internal/voice/transcriber.go`)

```go
package voice

import (
	"context"
	"log/slog"
	"net/http"
)

// TelegramVoiceInfo representa os metadados de uma mensagem de voz do Telegram.
// Origem: interface TelegramVoiceInfo (src/utils/voiceHandler.ts#L63-L69)
type TelegramVoiceInfo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// TelegramFileInfo representa o retorno de bot.api.getFile(fileId).
type TelegramFileInfo struct {
	FilePath string `json:"file_path"`
}

// TelegramFileGetter abstrai botApi.getFile(fileId) (src/utils/voiceHandler.ts#L75).
type TelegramFileGetter interface {
	GetFile(ctx context.Context, fileID string) (*TelegramFileInfo, error)
}

// WhisperInspector abstrai a verificação de arquivos/pacotes do Whisper para testes determinísticos.
type WhisperInspector struct {
	// BaseDir é o diretório raiz do projeto ou de instalação onde node_modules/nodejs-whisper ou modelos residem.
	BaseDir string
	// WhisperBinaryPath permite sobrescrever o caminho do executável whisper-cli via env/config (ADR-004).
	WhisperBinaryPath string
	// WhisperModelDir permite sobrescrever o diretório onde base.en está armazenado (ADR-004).
	WhisperModelDir string
}

// WhisperRunner abstrai a execução do motor de transcrição sobre um arquivo de áudio local.
type WhisperRunner func(ctx context.Context, voicePath string) (string, error)

// Assinaturas públicas de internal/voice/transcriber.go:
func TempVoiceDir() string
func CheckWhisperAvailability(inspector ...WhisperInspector) *string
func DownloadTelegramVoice(ctx context.Context, botApi TelegramFileGetter, botToken string, voice TelegramVoiceInfo, httpClient ...*http.Client) (string, error)
func TranscribeVoice(ctx context.Context, voicePath string, runner ...WhisperRunner) *string
func StripWhisperTimestamps(raw string) string
```

### 2.3. Pacote `internal/bot` (`internal/bot/image_handler.go`)

```go
package bot

import (
	"context"
	"net/http"
)

// InboundImageAttachment representa uma imagem baixada do Telegram pronta para envio ao Antigravity via CDP.
// Origem: interface InboundImageAttachment (src/utils/imageHandler.ts#L13-L18)
type InboundImageAttachment struct {
	LocalPath string `json:"localPath"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
}

// TelegramPhotoInfo representa um item do array photo de uma mensagem do Telegram.
// Origem: interface TelegramPhotoInfo (src/utils/imageHandler.ts#L20-L26)
type TelegramPhotoInfo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// ExtractedResponseImage representa uma imagem extraída do DOM de resposta do Antigravity.
// Origem: interface ExtractedResponseImage (src/services/cdpService.ts)
type ExtractedResponseImage struct {
	Name       string `json:"name,omitempty"`
	MimeType   string `json:"mimeType,omitempty"`
	Base64Data string `json:"base64Data,omitempty"`
	URL        string `json:"url,omitempty"`
}

// TelegramInputFileResult representa o buffer binário e o nome final de arquivo para envio ao Telegram.
// Origem: retorno de toTelegramInputFile (src/utils/imageHandler.ts#L118)
type TelegramInputFileResult struct {
	Buffer []byte
	Name   string
}

// TelegramFileResolver abstrai botApi.getFile(fileId) para download de imagens.
type TelegramFileResolver interface {
	GetFile(ctx context.Context, fileID string) (filePath string, err error)
}

// Assinaturas públicas de internal/bot/image_handler.go:
func TempImageDir() string
func IsImageAttachment(contentType string, fileName string) bool
func MimeTypeToExtension(mimeType string) string
func SanitizeFileName(fileName string) string
func BuildPromptWithAttachmentUrls(prompt string, attachments []InboundImageAttachment) string
func DownloadTelegramImages(ctx context.Context, botApi TelegramFileResolver, botToken string, photos []TelegramPhotoInfo, messageID string, httpClient ...*http.Client) ([]InboundImageAttachment, error)
func CleanupInboundImageAttachments(attachments []InboundImageAttachment)
func ToTelegramInputFile(ctx context.Context, image ExtractedResponseImage, index int, httpClient ...*http.Client) *TelegramInputFileResult
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

### 3.1. Constantes, Endpoints, Headers e Expressões Regulares de `QuotaService` ([`src/services/quotaService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/quotaService.ts#L28-L127))

```go
package quota

import (
	"fmt"
	"regexp"
	"time"
)

const (
	// Comandos Unix/macOS originais (src/services/quotaService.ts#L31, L54)
	PgrepLanguageServerCmd = "pgrep -fl language_server"
	LsofListeningPortsFmt  = "lsof -nP -a -iTCP -sTCP:LISTEN -p %d"

	// Endpoint e configuração HTTPS local (src/services/quotaService.ts#L71-L88)
	QuotaAPIHost                   = "127.0.0.1"
	QuotaAPIPath                   = "/exa.language_server_pb.LanguageServerService/GetUserStatus"
	QuotaAPIMethod                 = "POST"
	QuotaAPITimeout                = 2000 * time.Millisecond
	QuotaHeaderContentType         = "application/json"
	QuotaHeaderConnectProtocolVer  = "1"
	QuotaHeaderCodeiumCsrfTokenKey = "X-Codeium-Csrf-Token"
	QuotaHeaderConnectProtocolKey  = "Connect-Protocol-Version"

	// Payload JSON literal enviado ao GetUserStatus (src/services/quotaService.ts#L71-L73)
	QuotaRequestBodyJSON = `{"metadata":{"ideName":"antigravity","extensionName":"antigravity","locale":"en"}}`

	// Limite de retries quando a porta em cache falha (src/services/quotaService.ts#L166)
	MaxQuotaCachedPortRetries = 1
)

// CsrfTokenRegex extrai o token CSRF da linha de comando do language_server.
// Origem: /--csrf_token[=\s]+([a-zA-Z0-9\-]+)/ (src/services/quotaService.ts#L38)
var CsrfTokenRegex = regexp.MustCompile(`--csrf_token[=\s]+([a-zA-Z0-9\-]+)`)

// BuildLsofPortRegex constrói a expressão regular que extrai portas TCP/UDP em estado (LISTEN) para um PID específico.
// Origem: new RegExp(`^\\S+\\s+${pid}\\s+.*?(?:TCP|UDP)\\s+(?:\\*|[\\d.]+|\\[[\\da-f:]+\\]):(\\d+)\\s+\\(LISTEN\\)`, 'gim')
// (src/services/quotaService.ts#L55)
func BuildLsofPortRegex(pid int) *regexp.Regexp {
	pattern := fmt.Sprintf(`(?im)^\S+\s+%d\s+.*?(?:TCP|UDP)\s+(?:\*|[\d.]+|\[[\da-f:]+\]):(\d+)\s+\(LISTEN\)`, pid)
	return regexp.MustCompile(pattern)
}
```

### 3.2. Constantes, Mensagens de Setup e Expressões Regulares de `VoiceHandler` ([`src/utils/voiceHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/voiceHandler.ts#L7-L150))

```go
package voice

import "regexp"

const (
	// Subdiretório em os.TempDir() (src/utils/voiceHandler.ts#L7)
	TempVoiceSubdir = "antigravity-telegram-remote-voice"

	// Extensão padrão quando file_path do Telegram não possui extensão (src/utils/voiceHandler.ts#L98)
	DefaultVoiceExt = ".ogg"

	// Modelo padrão do Whisper (src/utils/voiceHandler.ts#L51, L129)
	DefaultWhisperModelName = "base.en"

	// Mensagens literais retornadas por checkWhisperAvailability() (src/utils/voiceHandler.ts#L18-L58)
	MsgWhisperNotInstalled = "🔇 Voice transcription is not set up.\n\n" +
		"To enable it, run:\n" +
		"  npm install nodejs-whisper ffmpeg-static\n" +
		"  npx nodejs-whisper download"

	MsgWhisperNotCompiled = "🔇 Whisper is installed but not compiled.\n\n" +
		"Run this to build it:\n" +
		"  cd node_modules/nodejs-whisper/cpp/whisper.cpp\n" +
		"  cmake -B build && cmake --build build --config Release\n\n" +
		"Requires cmake (brew install cmake / apt install cmake)."

	MsgWhisperModelMissing = "🔇 Whisper model not downloaded.\n\n" +
		"Run this once to fetch the base.en model (~140 MB):\n" +
		"  npx nodejs-whisper download"
)

// WhisperTimestampRegex remove prefixos de timestamp como "[00:00:00.000 --> 00:00:02.000] ".
// Origem: /\[[\d:.]+\s*-->\s*[\d:.]+\]\s*/g (src/utils/voiceHandler.ts#L148)
var WhisperTimestampRegex = regexp.MustCompile(`\[[\d:.]+\s*-->\s*[\d:.]+\]\s*`)
```

### 3.3. Constantes e Expressões Regulares de `ImageHandler` ([`src/utils/imageHandler.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/imageHandler.ts#L9-L56))

```go
package bot

import "regexp"

const (
	// Limite máximo de imagens baixadas por mensagem (src/utils/imageHandler.ts#L9)
	MaxInboundImageAttachments = 4

	// Subdiretório em os.TempDir() (src/utils/imageHandler.ts#L11)
	TempImageSubdir = "antigravity-telegram-remote-images"

	// Prompt padrão quando o usuário envia imagem sem legenda/texto (src/utils/imageHandler.ts#L48)
	DefaultImagePromptBase = "Please review the attached images and respond accordingly."

	// Rodapé anexado em buildPromptWithAttachmentUrls (src/utils/imageHandler.ts#L55)
	ImagePromptFooter = "Please refer to the attached images above in your response."
)

// ImageExtPattern valida se um nome de arquivo termina com extensão de imagem suportada.
// Origem: /\.(png|jpe?g|webp|gif|bmp)$/i (src/utils/imageHandler.ts#L10)
var ImageExtPattern = regexp.MustCompile(`(?i)\.(png|jpe?g|webp|gif|bmp)$`)

// InvalidFileNameCharsRegex substitui caracteres fora de [a-zA-Z0-9._-] por '-'.
// Origem: /[^a-zA-Z0-9._-]/g (src/utils/imageHandler.ts#L43)
var InvalidFileNameCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// MultipleHyphensRegex colapsa múltiplos hífens consecutivos em um único '-'.
// Origem: /-+/g (src/utils/imageHandler.ts#L43)
var MultipleHyphensRegex = regexp.MustCompile(`-+`)

// HTTPURLPrefixRegex verifica se a URL da imagem extraída começa com http:// ou https://.
// Origem: /^https?:\/\//i (src/utils/imageHandler.ts#L125)
var HTTPURLPrefixRegex = regexp.MustCompile(`(?i)^https?://`)
```

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo de `QuotaService` (`internal/quota/service.go` e `internal/platform`)

#### A. Descoberta do Processo `language_server` (`getUnixProcessInfo` e `internal/platform`)
1. **Comportamento Padrão (`pgrep` — macOS/Unix, idêntico a `src/services/quotaService.ts#L28-L48`)**:
   - Executa `pgrep -fl language_server` via `s.execCmd(ctx, "pgrep -fl language_server")`.
   - Se ocorrer erro, loga `s.logger.Error("Failed to get process info", "error", err)` e retorna `nil`.
   - Divide `stdout` por `"\n"`. Para cada `line`:
     - Verifica se `strings.Contains(line, "--csrf_token")`.
     - Divide `strings.TrimSpace(line)` por espaços (`strings.Fields`):
       - `pid, err := strconv.Atoi(parts[0])`
       - `cmd := strings.TrimSpace(line[len(parts[0]):])`
       - Aplica `CsrfTokenRegex.FindStringSubmatch(cmd)`.
       - Se `pid > 0 && len(match) > 1 && match[1] != ""`, retorna `&LanguageServerProcessInfo{PID: pid, CSRFToken: match[1]}`.
2. **Extensão Multiplataforma em `internal/platform` (ADR-001)**:
   - Em **Linux** (`//go:build linux`): tenta primeiro `pgrep -fl language_server` (ou `pgrep -fa language_server`); se falhar, varre `/proc/<pid>/cmdline` procurando entradas contendo `language_server` e `--csrf_token`.
   - Em **Windows** (`//go:build windows`): executa PowerShell `Get-CimInstance Win32_Process -Filter "Name LIKE '%language_server%'" | Select-Object ProcessId,CommandLine` (ou `wmic process where "name like '%language_server%'" get ProcessId,CommandLine`) e aplica `CsrfTokenRegex`.

#### B. Descoberta de Portas TCP em Escuta (`getListeningPorts`)
1. **Comportamento Padrão (`lsof` — macOS/Unix, idêntico a `src/services/quotaService.ts#L50-L67`)**:
   - Executa `fmt.Sprintf("lsof -nP -a -iTCP -sTCP:LISTEN -p %d", pid)`.
   - Se ocorrer erro, loga `s.logger.Error(fmt.Sprintf("Failed to get ports for pid %d", pid), "error", err)` e retorna `[]int{}`.
   - Aplica `BuildLsofPortRegex(pid).FindAllStringSubmatch(stdout, -1)`.
   - Para cada match, converte `match[1]` com `strconv.Atoi` e adiciona na slice `ports` **sem duplicatas** (preservando a ordem de aparição).
2. **Extensão Multiplataforma em `internal/platform` (ADR-001)**:
   - Em **Linux**: se `lsof` não estiver instalado ou não retornar portas, faz fallback para `ss -tlnp` filtrando por `pid=<pid>`.
   - Em **Windows**: executa `netstat -ano -p tcp` filtrando linhas `LISTENING` cujo último campo seja `<pid>`.

#### C. Chamada HTTPS ao `GetUserStatus` (`requestAPI`)
1. Serializa `QuotaRequestBodyJSON` (`{"metadata":{"ideName":"antigravity","extensionName":"antigravity","locale":"en"}}`).
2. Constrói requisição `POST https://127.0.0.1:<port>/exa.language_server_pb.LanguageServerService/GetUserStatus` com:
   - Header `Content-Type: application/json`
   - Header `Content-Length: <len(data)>`
   - Header `Connect-Protocol-Version: 1`
   - Header `X-Codeium-Csrf-Token: <csrfToken>`
   - Cliente HTTP configurado com `tls.Config{InsecureSkipVerify: true}` e `Timeout: 2000 * time.Millisecond`.
3. Se `res.StatusCode != 200`, retorna erro `fmt.Errorf("HTTP %d", res.StatusCode)`.
4. Faz o parse do JSON de resposta suportando todos os campos de fallback do TypeScript original ([`src/services/quotaService.ts#L98-L111`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/quotaService.ts#L98-L111)):
   - Navega até `parsed.userStatus.cascadeModelConfigData.clientModelConfigs` (se ausente, usa slice vazia `[]`).
   - Para cada item `c` em `clientModelConfigs`:
     - `label := firstNonEmpty(c.Label, c.DisplayName, c.ModelName, "")`
     - `model := firstNonEmpty(c.ModelOrAlias.Model, c.Model, c.ModelID, "")`
     - `qi := firstNonNil(c.QuotaInfo, c.Quota, c.UsageInfo)`
     - Se `qi != nil`:
       - `remainingFraction`: converte `qi.RemainingFraction` (ou fallback `qi.Remaining`, ou `0`) para `float64` (aceitando tanto número JSON quanto string numérica, igual a `parseFloat(...)` em JS).
       - `resetTime := firstNonEmpty(qi.ResetTime, qi.ResetAt, "")`
       - Popula `QuotaInfo: &QuotaInfo{RemainingFraction: remainingFraction, ResetTime: resetTime}`.
   - Se o JSON for inválido, retorna erro `errors.New("Invalid JSON response")`.

#### D. Fluxo de Cache e Retry (`FetchQuota` / `fetchQuotaWithRetry`)
1. Chama `processInfo := s.getUnixProcessInfo(ctx)`.
2. Se `processInfo == nil`, loga `"No language_server process found."` e retorna `[]ModelQuota{}`.
3. Compara com o cache:
   - Se `s.cachedPid == nil || *s.cachedPid != processInfo.PID || s.cachedCsrfToken == nil || *s.cachedCsrfToken != processInfo.CSRFToken`:
     - Invalida `s.cachedPort = nil`.
     - Atualiza `s.cachedPid = &processInfo.PID` e `s.cachedCsrfToken = &processInfo.CSRFToken`.
4. **Se `s.cachedPort == nil` (Descoberta de Porta)**:
   - Obtém `ports := s.getListeningPorts(ctx, processInfo.PID)`.
   - Para cada `port` em `ports`:
     - Chama `data, err := s.requestAPI(ctx, port, processInfo.CSRFToken)`.
     - Se `err == nil`:
       - Salva `s.cachedPort = &port`.
       - Retorna `data.ClientModelConfigs` (ou `[]ModelQuota{}` se nil).
     - Se `err != nil`, continua para a próxima porta (`continue`).
   - Se nenhuma porta responder com sucesso, retorna `[]ModelQuota{}`.
5. **Se `s.cachedPort != nil` (Uso de Porta em Cache)**:
   - Chama `data, err := s.requestAPI(ctx, *s.cachedPort, processInfo.CSRFToken)`.
   - Se `err == nil`, retorna `data.ClientModelConfigs`.
   - Se `err != nil`:
     - Invalida `s.cachedPort = nil`.
     - Se `retryCount < 1`: chama recursivamente `return s.fetchQuotaWithRetry(ctx, retryCount+1)` (que fará um novo `getListeningPorts`).
     - Se `retryCount >= 1`: loga `"fetchQuota failed after retry:"` e retorna `[]ModelQuota{}`.

---

### 4.2. Algoritmo de `VoiceHandler` (`internal/voice/transcriber.go`)

1. **`CheckWhisperAvailability(inspector ...WhisperInspector) *string`**:
   - Verifica na ordem exata de [`src/utils/voiceHandler.ts#L13-L61`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/voiceHandler.ts#L13-L61) (suportando tanto a árvore `node_modules/nodejs-whisper` quanto caminhos nativos configurados em Go conforme ADR-004):
     1. Verifica se o diretório do pacote `nodejs-whisper` (ou configuração de Whisper local) existe. Se não existir e nenhum binário nativo estiver configurado, retorna ponteiro para `MsgWhisperNotInstalled`.
     2. Verifica se o binário `whisper-cli` existe e é um arquivo regular (`stat.Mode().IsRegular()`, ex: em `<pkgDir>/cpp/whisper.cpp/build/bin/whisper-cli` ou `WhisperBinaryPath`). Se não existir, retorna ponteiro para `MsgWhisperNotCompiled`.
     3. Lê o diretório `models` (`<pkgDir>/models` ou `WhisperModelDir`) e verifica se algum arquivo contém `"base.en"` no nome (`strings.Contains(f.Name(), "base.en")`). Se não encontrar, retorna ponteiro para `MsgWhisperModelMissing`.
   - Se todas as 3 checagens passarem, retorna `nil`.

2. **`DownloadTelegramVoice(ctx, botApi, botToken, voice, httpClient...) (string, error)`**:
   - Cria o diretório `TempVoiceDir()` (`filepath.Join(os.TempDir(), "antigravity-telegram-remote-voice")`) com `os.MkdirAll(..., 0o755)`.
   - Chama `file, err := botApi.GetFile(ctx, voice.FileID)`.
   - Se `err != nil`, propaga o erro.
   - Se `file == nil || file.FilePath == ""`, retorna erro `errors.New("Telegram returned no file_path for voice message")`.
   - Constrói a URL: `fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, file.FilePath)`.
   - Executa `GET` HTTP na URL.
   - Se `resp.StatusCode < 200 || resp.StatusCode >= 300`, retorna erro `fmt.Errorf("Voice download failed (status=%d)", resp.StatusCode)`.
   - Lê todos os bytes do body (`io.ReadAll`).
   - Se `len(bytes) == 0`, retorna erro `errors.New("Voice download returned empty file")`.
   - Determina a extensão: `ext := filepath.Ext(file.FilePath)`; se `ext == ""`, usa `".ogg"`.
   - Constrói o caminho local: `filepath.Join(TempVoiceDir(), fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), voice.FileUniqueID, ext))`.
   - Escreve o arquivo no disco (`os.WriteFile(localPath, bytes, 0o600)`), loga `"[VoiceHandler] Downloaded voice message to ..."` e retorna `localPath`.

3. **`TranscribeVoice(ctx, voicePath, runner...) *string`**:
   - Registra imediatamente `defer func() { _ = os.Remove(voicePath) }()` para garantir que o arquivo temporário `.ogg`/`.oga` seja sempre removido ao final, independentemente de sucesso ou erro (idêntico ao bloco `finally { await fs.unlink(voicePath).catch(() => {}); }` em [`src/utils/voiceHandler.ts#L161-L164`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/voiceHandler.ts#L161-L164)).
   - Executa o `WhisperRunner` (por padrão: converte `voicePath` para `<voicePath>.wav` 16kHz 16-bit mono via `ffmpeg -y -i <voicePath> -ar 16000 -ac 1 -c:a pcm_s16le <wavPath>`, remove `<wavPath>` via `defer os.Remove(wavPath)`, e invoca `whisper-cli -m <modelPath> -f <wavPath> -nt` capturando a saída textual).
   - Se a execução retornar erro, loga `"[VoiceHandler] Transcription failed:"` e retorna `nil`.
   - Aplica `StripWhisperTimestamps(raw)`:
     - `strings.TrimSpace(WhisperTimestampRegex.ReplaceAllString( strings.TrimSpace(raw), "" ))`
   - Se o texto resultante for vazio (`transcript == ""`), loga warning `"[VoiceHandler] Whisper returned empty transcript"` e retorna `nil`.
   - Caso contrário, loga info com os primeiros 100 caracteres e retorna `&transcript`.

---

### 4.3. Algoritmo de `ImageHandler` (`internal/bot/image_handler.go`)

1. **`IsImageAttachment(contentType, fileName string) bool`**:
   - Se `strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/")`, retorna `true`.
   - Caso contrário, retorna `ImageExtPattern.MatchString(fileName)` (`/\.(png|jpe?g|webp|gif|bmp)$/i`).
2. **`MimeTypeToExtension(mimeType string) string`**:
   - `normalized := strings.ToLower(mimeType)`
   - Se contém `"jpeg"` ou `"jpg"` $\rightarrow$ retorna `"jpg"`.
   - Se contém `"webp"` $\rightarrow$ retorna `"webp"`.
   - Se contém `"gif"` $\rightarrow$ retorna `"gif"`.
   - Se contém `"bmp"` $\rightarrow$ retorna `"bmp"`.
   - Caso contrário $\rightarrow$ retorna `"png"`.
3. **`SanitizeFileName(fileName string) string`**:
   - Substitui `InvalidFileNameCharsRegex` (`[^a-zA-Z0-9._-]`) por `"-"`.
   - Substitui `MultipleHyphensRegex` (`-+`) por `"-"`.
   - Remove hífens no início e no fim com `strings.Trim(s, "-")`.
   - Se o resultado for `""`, retorna `fmt.Sprintf("image-%d.png", time.Now().UnixMilli())`.
   - **Nota de fidelidade com o teste original**: Para `"bad name/with*chars?.png"`, `InvalidFileNameCharsRegex` produz `"bad-name-with-chars-.png"`, `MultipleHyphensRegex` mantém `"bad-name-with-chars-.png"`, e `strings.Trim(..., "-")` mantém `"bad-name-with-chars-.png"` (pois termina em `.png`, não em `-`).
4. **`BuildPromptWithAttachmentUrls(prompt string, attachments []InboundImageAttachment) string`**:
   - `base := strings.TrimSpace(prompt)`
   - Se `base == ""`, define `base = "Please review the attached images and respond accordingly."`.
   - Se `len(attachments) == 0`, retorna `base`.
   - Para cada `image` com índice `i` (`0..len-1`), formata `fmt.Sprintf("%d. %s\nURL: %s", i+1, image.Name, image.URL)`.
   - Junta os itens com `"\n\n"` e retorna:
     `fmt.Sprintf("%s\n\n[Telegram Attached Images]\n%s\n\nPlease refer to the attached images above in your response.", base, strings.Join(lines, "\n\n"))`.
5. **`DownloadTelegramImages(ctx, botApi, botToken, photos, messageID, httpClient...)`**:
   - Limita `photos` aos primeiros `4` elementos (`MaxInboundImageAttachments = 4`). Se vazio, retorna `[]InboundImageAttachment{}, nil`.
   - Cria `TempImageDir()` (`os.MkdirAll(..., 0o755)`).
   - Inicializa `index := 0`. Para cada `photo`:
     - Chama `filePath, err := botApi.GetFile(ctx, photo.FileID)`. Se erro ou `filePath == ""`, faz `continue`.
     - Faz `GET https://api.telegram.org/file/bot<botToken>/<filePath>`. Se `!ok` ou `len(bytes) == 0`, faz `continue`.
     - `ext := filepath.Ext(filePath)`; se `ext == ""`, usa `".png"`.
     - `name := SanitizeFileName(fmt.Sprintf("telegram-image-%d%s", index+1, ext))`
     - `localPath := filepath.Join(TempImageDir(), fmt.Sprintf("%d-%s-%d-%s", time.Now().UnixMilli(), messageID, index, name))`
     - Salva `bytes` em `localPath`.
     - Calcula `mimeExt := strings.ToLower(strings.TrimPrefix(ext, "."))`; se `mimeExt == "jpg"`, muda para `"jpeg"`.
     - Adiciona `InboundImageAttachment{LocalPath: localPath, URL: fmt.Sprintf("[local file: %s]", name), Name: name, MimeType: "image/" + mimeExt}` e incrementa `index++`.
6. **`CleanupInboundImageAttachments(attachments)`**:
   - Para cada `image` em `attachments`, executa `_ = os.Remove(image.LocalPath)`.
7. **`ToTelegramInputFile(ctx, image, index, httpClient...) *TelegramInputFileResult`**:
   - Define `mimeType := image.MimeType`; se vazio, usa `"image/png"`.
   - Se `image.Base64Data != ""`, decodifica com `base64.StdEncoding.DecodeString(image.Base64Data)`.
   - Senão, se `image.URL != ""` e `HTTPURLPrefixRegex.MatchString(image.URL)`:
     - Faz `GET image.URL`; se `StatusCode == 200`, lê o body em `buffer` e, se `resp.Header.Get("Content-Type") != ""`, atualiza `mimeType`.
   - Se `len(buffer) == 0`, retorna `nil`.
   - `fallbackExt := MimeTypeToExtension(mimeType)`
   - `rawName := image.Name`; se vazio, usa `fmt.Sprintf("generated-image-%d.%s", index+1, fallbackExt)`.
   - `baseName := SanitizeFileName(rawName)`
   - `finalName := baseName`; se `!ImageExtPattern.MatchString(baseName)`, concatena `fmt.Sprintf("%s.%s", baseName, fallbackExt)`.
   - Retorna `&TelegramInputFileResult{Buffer: buffer, Name: finalName}`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Thread-Safety do Cache em `QuotaService`**:
   - O `QuotaService` é compartilhado no `CdpBridge` e pode ser chamado concorrentemente por múltiplos comandos `/status` ou `/models`. Os campos `cachedPid`, `cachedCsrfToken` e `cachedPort` são protegidos por `s.mu sync.Mutex`.
2. **Timeouts e Cancelamento via `context.Context`**:
   - Todas as chamadas externas (`execCmd` para `pgrep`/`lsof`, requisições HTTPS para `127.0.0.1:<port>` com timeout de `2000ms`, downloads de arquivos do Telegram e execução de `ffmpeg`/`whisper-cli`) recebem `ctx context.Context`, garantindo cancelamento imediato no shutdown do bot sem deixar processos órfãos.
3. **Limpeza Garantida de Arquivos Temporários**:
   - Em `TranscribeVoice`, a remoção de `voicePath` (e do `.wav` intermediário) ocorre via `defer os.Remove(...)`, garantindo limpeza mesmo em caso de falha do `whisper-cli` ou cancelamento de contexto.
   - Em `DownloadTelegramImages` / `CleanupInboundImageAttachments`, cada arquivo inclui `UnixMilli()`, `messageId` e `index` no nome para evitar colisão entre mensagens simultâneas em diferentes tópicos.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

### 6.1. Testes de `QuotaService` (`internal/quota/service_test.go`)
Mapeamento 1:1 dos casos de teste de [`tests/services/quotaService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/quotaService.test.ts#L29-L120) + cobertura complementar de retry e invalidação de cache:

| # | Teste Original em `tests/services/quotaService.test.ts` | Função de Teste em Go (`service_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `returns an empty array when language_server process is not found` (L43) | `TestQuotaService_ReturnsEmptySliceWhenProcessNotFound` | Mock `CommandExecutor` falha com erro `"not found"` ao rodar `pgrep -fl language_server`. `FetchQuota(ctx)` retorna slice vazia `[]ModelQuota{}` e nenhuma requisição HTTPS é disparada. |
| 2 | `fetches quota info and caches port discovery for the same PID/Token` (L56) | `TestQuotaService_FetchesQuotaAndCachesPortForSamePIDAndToken` | Mock `CommandExecutor` retorna `"123 language_server --csrf_token abc-123\n"` para `pgrep` e `"language_server 123 user 10u IPv4 0x0 0t0 TCP *:4444 (LISTEN)\n"` para `lsof -nP -a -iTCP -sTCP:LISTEN -p 123`. Servidor/mock HTTPS retorna `200` com `label: "Gemini"`, `model: "gemini-pro"`, `remainingFraction: 0.6`, `resetTime: "2026-02-23T12:00:00.000Z"`. Executa `FetchQuota(ctx)` 2 vezes seguidas: ambas retornam 1 item com `Label == "Gemini"`, o comando `lsof` é executado **apenas 1 vez** (na 1ª chamada), e a 2ª chamada reutiliza a porta `4444` em cache. |
| 3 | *(Complementar da regra de negócio L160-L171)* | `TestQuotaService_RetriesOnceWithFreshPortScanWhenCachedPortFails` | Após cachear a porta `4444`, na 2ª chamada a porta `4444` falha e `lsof` retorna a nova porta `5555`. `FetchQuota(ctx)` invalida `cachedPort`, re-executa `lsof` (1 retry) e obtém sucesso na porta `5555`. |

---

### 6.2. Testes de `VoiceHandler` (`internal/voice/transcriber_test.go`)
Mapeamento 1:1 de todos os 6 casos de teste de [`tests/utils/voiceHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/voiceHandler.test.ts#L29-L114) + testes de `CheckWhisperAvailability` e `StripWhisperTimestamps`:

| # | Teste Original em `tests/utils/voiceHandler.test.ts` | Função de Teste em Go (`transcriber_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `downloadTelegramVoice > downloads voice file to temp directory` (L46) | `TestDownloadTelegramVoice_DownloadsToTempDir` | Mock `GetFile("voice-file-id-123")` retorna `file_path: "voice/file_0.oga"`; mock HTTP retorna `200 OK` com 100 bytes para `https://api.telegram.org/file/bottest-bot-token/voice/file_0.oga`. Verifica criação do diretório `TempVoiceDir()`, gravação do arquivo e que o caminho retornado casa com `/unique-456\.oga$/`. |
| 2 | `downloadTelegramVoice > throws when Telegram returns no file_path` (L64) | `TestDownloadTelegramVoice_ErrorWhenNoFilePath` | Mock `GetFile` retorna `FilePath: ""`. Retorna erro contendo `"Telegram returned no file_path"`. |
| 3 | `downloadTelegramVoice > throws when download HTTP status is not ok` (L72) | `TestDownloadTelegramVoice_ErrorWhenHTTPNotOK` | Mock HTTP retorna status `404`. Retorna erro contendo `"Voice download failed (status=404)"`. |
| 4 | `downloadTelegramVoice > throws when downloaded file is empty` (L80) | `TestDownloadTelegramVoice_ErrorWhenFileEmpty` | Mock HTTP retorna `200 OK` com `0` bytes. Retorna erro contendo `"Voice download returned empty file"`. |
| 5 | `downloadTelegramVoice > uses .ogg extension when file_path has no extension` (L93) | `TestDownloadTelegramVoice_DefaultsToOggExtension` | Mock `GetFile` retorna `file_path: "voice/file_0"` (sem extensão). O caminho salvo termina com `.ogg`. |
| 6 | `transcribeVoice > returns null and cleans up file on transcription failure` (L106) | `TestTranscribeVoice_ReturnsNilAndCleansUpFileOnFailure` | Cria arquivo temporário `/tmp/.../test.ogg` e executa `TranscribeVoice` com falha no runner/whisper ausente. Verifica retorno `nil` e que o arquivo temporário foi removido do disco (`os.Remove`). |
| 7 | *(Complementar de `src/utils/voiceHandler.ts#L147-L150`)* | `TestStripWhisperTimestamps_RemovesTimestampPrefixes` | Dada a saída `"[00:00:00.000 --> 00:00:02.000]   Hello world \n[00:00:02.000 --> 00:00:04.000] test"`, `StripWhisperTimestamps` retorna `"Hello world \ntest"`. |

---

### 6.3. Testes de `ImageHandler` (`internal/bot/image_handler_test.go`)
Mapeamento 1:1 de todos os 5 casos de teste de [`tests/utils/imageHandler.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/imageHandler.test.ts#L3-L31) + cobertura das funções de prompt e conversão:

| # | Teste Original em `tests/utils/imageHandler.test.ts` | Função de Teste em Go (`image_handler_test.go`) | Cenário, Asserções e Critérios de Aceite |
| :-: | :--- | :--- | :--- |
| 1 | `isImageAttachment > returns true when contentType is image/*` (L5) | `TestIsImageAttachment_TrueWhenContentTypeImage` | `IsImageAttachment("image/png", "any.bin")` retorna `true`. |
| 2 | `isImageAttachment > returns true when the file extension is an image format` (L9) | `TestIsImageAttachment_TrueWhenExtensionIsImage` | `IsImageAttachment("", "photo.jpeg")` retorna `true`. |
| 3 | `isImageAttachment > returns false for non-image files` (L13) | `TestIsImageAttachment_FalseForNonImage` | `IsImageAttachment("application/pdf", "doc.pdf")` retorna `false`. |
| 4 | `mimeTypeToExtension > converts common mimeTypes to file extensions` (L19) | `TestMimeTypeToExtension_ConvertsCommonMimeTypes` | Verifica `MimeTypeToExtension("image/png") == "png"`, `MimeTypeToExtension("image/jpeg") == "jpg"` e `MimeTypeToExtension("image/webp") == "webp"`. |
| 5 | `sanitizeFileName > replaces invalid characters with hyphens` (L27) | `TestSanitizeFileName_ReplacesInvalidCharactersWithHyphens` | `SanitizeFileName("bad name/with*chars?.png")` retorna exatamente `"bad-name-with-chars-.png"`. |
| 6 | *(Complementar de `src/utils/imageHandler.ts#L47-L56`)* | `TestBuildPromptWithAttachmentUrls_FormatsPromptAndAttachments` | Verifica fallback `"Please review the attached images and respond accordingly."` quando prompt é vazio e formatação da seção `[Telegram Attached Images]` quando há anexos. |
| 7 | *(Complementar de `src/utils/imageHandler.ts#L118-L141`)* | `TestToTelegramInputFile_DecodesBase64AndAppendsExtension` | Verifica decodificação de `Base64Data` e adição automática da extensão `.jpg`/`.png` quando `image.Name` não possui extensão de imagem. |
