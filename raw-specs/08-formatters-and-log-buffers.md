# 08 — Conversores HTML/Markdown, Formatadores de Streaming, Extrator de Metadados, Buffer de Logs e Emissor de Progresso

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de forma 100% autocontida e exaustiva a conversão de HTML do DOM para HTML compatível com o Telegram, a formatação de Markdown e separação de logs de UI Chrome, o particionamento seguro de mensagens HTML (`splitTelegramHtml`), a formatação de cabeçalhos e filtros de streaming, a extração de metadados de rodapé, o buffer circular de logs de processo (`ProcessLogBuffer`) e o emissor de progresso com *throttle* (`ProgressSender`).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo TypeScript Origem | Linhas | Arquivo Go Destino | Pacote Go | Responsabilidade Principal |
| :--- | :---: | :--- | :---: | :--- |
| [`src/utils/htmlToTelegramMarkdown.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/htmlToTelegramMarkdown.ts) | 275 | `internal/formatter/html_converter.go` | `formatter` | Conversão de HTML bruto do DOM do Antigravity para HTML suportado pelo Telegram (`<b>`, `<i>`, `<code>`, `<pre>`, `<a href="">`, `<s>`, `<u>`, `<blockquote>`), tabelas ASCII, listas aninhadas e decodificação segura de entidades. |
| [`src/utils/telegramFormatter.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/telegramFormatter.ts) | 468 | `internal/formatter/telegram_formatter.go` | `formatter` | Escape HTML, conversão de Markdown para Telegram HTML (`formatForTelegram`), detecção e separação de UI Chrome/logs (`splitOutputAndLogs`, `separateOutputForDelivery`, `sanitizeActivityLines`) e paginação de HTML respeitando pilha de tags abertas (`splitTelegramHtml`). |
| [`src/utils/streamMessageFormatter.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/streamMessageFormatter.ts) | 94 | `internal/formatter/stream_formatter.go` | `formatter` | Construção de linhas de cabeçalho de modo/modelo (`buildModeModelLines`), filtro de ruído de logs de atividade (`shouldSkipActivityLog`) e ajuste/divisão de descrições de embed (`splitForEmbedDescription`, `fitForSingleEmbedDescription`). |
| [`src/utils/metadataExtractor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/metadataExtractor.ts) | 20 | `internal/formatter/metadata.go` | `formatter` | Extração estruturada de `TaskID` e `Dir` a partir de textos de rodapé (`extractMetadataFromFooter`). |
| [`src/utils/processLogBuffer.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/utils/processLogBuffer.ts) | 114 | `internal/monitor/process_log_buffer.go` | `monitor` | Buffer circular deduplicado de entradas de log de processo com classificação automática de emojis (`pickEmoji`) e poda por número de entradas e limite de caracteres. |
| [`src/services/progressSender.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/progressSender.ts) | 77 | `internal/monitor/progress_sender.go` | `monitor` | Acumulador assíncrono com *throttle* temporal (padrão `3000ms`), fatiamento por tamanho máximo (`4000` chars) e envelopamento em `<pre>...</pre>`. |

### 1.2. Mapeamento de Arquivos de Teste Origem $\rightarrow$ Destino Go

| Arquivo de Teste TypeScript Origem | Linhas | Arquivo de Teste Go Destino |
| :--- | :---: | :--- |
| [`tests/utils/htmlToTelegramMarkdown.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/htmlToTelegramMarkdown.test.ts) | 238 | `internal/formatter/html_converter_test.go` |
| [`tests/utils/telegramFormatter.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/telegramFormatter.test.ts) | 208 | `internal/formatter/telegram_formatter_test.go` |
| [`tests/utils/splitTelegramHtml.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/splitTelegramHtml.test.ts) | 119 | `internal/formatter/split_telegram_html_test.go` |
| [`tests/utils/streamMessageFormatter.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/streamMessageFormatter.test.ts) | 83 | `internal/formatter/stream_formatter_test.go` |
| [`tests/utils/metadataExtractor.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/metadataExtractor.test.ts) | 38 | `internal/formatter/metadata_test.go` |
| [`tests/utils/processLogBuffer.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/processLogBuffer.test.ts) | 75 | `internal/monitor/process_log_buffer_test.go` |
| [`tests/services/progressSender.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/progressSender.test.ts) | 88 | `internal/monitor/progress_sender_test.go` |
| [`tests/services/progressSender.bugfix.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/progressSender.bugfix.test.ts) | 127 | `internal/monitor/progress_sender_test.go` |

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pacote `formatter` (`internal/formatter/`)

#### `internal/formatter/html_converter.go`

```go
package formatter

// HtmlToTelegramHtml converte tags HTML comuns extraídas do DOM para formatação HTML
// compatível com o Telegram (<b>, <i>, <code>, <pre>, <a href="">, <s>, <u>, <blockquote>).
// Equivalente a export function htmlToTelegramHtml(html: string): string em src/utils/htmlToTelegramMarkdown.ts.
func HtmlToTelegramHtml(html string) string

// Funções internas do conversor HTML:
func looksLikeFilePath(value string) bool
func stripTags(html string) string
func stripUnsupportedTags(html string) string
func decodeSafeEntities(text string) string
```

#### `internal/formatter/telegram_formatter.go`

```go
package formatter

// SplitOutputResult representa o retorno de SplitOutputAndLogs.
type SplitOutputResult struct {
	Output string `json:"output"`
	Logs   string `json:"logs"`
}

// SeparateOutputOptions representa os parâmetros de SeparateOutputForDelivery.
// DomOutputText usa ponteiro (*string) para distinguir entre string vazia ("") e undefined (nil).
type SeparateOutputOptions struct {
	RawText          string   `json:"rawText"`
	DomSource        string   `json:"domSource"` // "dom-structured" | "legacy-fallback"
	DomOutputText    *string  `json:"domOutputText,omitempty"`
	DomActivityLines []string `json:"domActivityLines,omitempty"`
}

// SeparateOutputResult representa o retorno de SeparateOutputForDelivery.
type SeparateOutputResult struct {
	Source string `json:"source"`
	Output string `json:"output"`
	Logs   string `json:"logs"`
}

// EscapeHtml escapa caracteres especiais HTML (&, <, >) para o modo de parse HTML do Telegram.
func EscapeHtml(text string) string

// FormatForTelegram converte sintaxe Markdown para HTML do Telegram, envolve linhas de
// tabela/árvore em <pre> e referências de caminhos de arquivos em <code>.
func FormatForTelegram(text string) string

// SplitOutputAndLogs separa linhas de texto de saída e linhas de UI chrome / logs de atividade.
func SplitOutputAndLogs(rawText string) SplitOutputResult

// SeparateOutputForDelivery seleciona entre a saída estruturada do DOM e o fallback legado via SplitOutputAndLogs.
func SeparateOutputForDelivery(options SeparateOutputOptions) SeparateOutputResult

// SplitTelegramHtml divide uma string formatada em HTML em múltiplos blocos seguros para o Telegram
// (padrão maxLen = 4096 se maxLen <= 0), fechando tags abertas no final de cada bloco e reabrindo-as no próximo.
func SplitTelegramHtml(html string, maxLen int) []string

// SanitizeActivityLines remove linhas vazias, filtra linhas de UI chrome e deduplica mantendo a ordem de aparição.
func SanitizeActivityLines(raw string) string

// Funções auxiliares internas:
func isUiChromeLine(line string) bool
func formatMarkdownLine(line string) string
func wrapFileReferences(line string) string
func isMcpFormatLine(line string) bool
func isActivityLogLine(line string) bool
func collapseBlankLines(text string) string
```

#### `internal/formatter/stream_formatter.go`

```go
package formatter

// BuildModeModelLines gera linhas de modo/modelo para exibição de status inicial.
// Consolida em uma única linha de modelo se Fast e Plan forem iguais (case-insensitive após trim).
func BuildModeModelLines(modeName string, fastModel string, planModel string) []string

// ShouldSkipActivityLog filtra logs de atividade que tendem a ser ruído na exibição de mensagens.
func ShouldSkipActivityLog(activity string, modeName string, modelName string) bool

// SplitForEmbedDescription divide texto longo em múltiplos blocos para descrição de Embed (padrão maxLength = 3500 se <= 0).
func SplitForEmbedDescription(text string, maxLength int) []string

// FitForSingleEmbedDescription ajusta o texto dentro do limite de uma única descrição de Embed (padrão maxLength = 3500 se <= 0),
// truncando o início e priorizando a cauda mais recente com o prefixo "...(phần đầu được lược bớt)\n".
func FitForSingleEmbedDescription(text string, maxLength int) string
```

#### `internal/formatter/metadata.go`

```go
package formatter

// TaskMetadata representa os metadados extraídos de um rodapé de mensagem.
// Campos vazios ("") equivalem a undefined no TypeScript.
type TaskMetadata struct {
	TaskID    string `json:"taskId,omitempty"`
	Directory string `json:"directory,omitempty"`
}

// ExtractMetadataFromFooter extrai TaskID e Dir de um texto de rodapé.
func ExtractMetadataFromFooter(footerText string) TaskMetadata
```

---

### 2.2. Pacote `monitor` (`internal/monitor/`)

#### `internal/monitor/process_log_buffer.go`

```go
package monitor

import "sync"

// ProcessLogBufferOptions define as opções de configuração do ProcessLogBuffer.
// Valores zero/negativos nos campos numéricos aplicam os padrões:
// MaxChars = 3500, MaxEntries = 120, MaxEntryLength = 260.
type ProcessLogBufferOptions struct {
	MaxChars       int
	MaxEntries     int
	MaxEntryLength int
}

// ProcessLogBuffer mantém um buffer circular deduplicado (case-insensitive) de logs de atividade
// formatados com prefixos de emoji.
type ProcessLogBuffer struct {
	mu             sync.Mutex
	maxChars       int
	maxEntries     int
	maxEntryLength int
	entries        []string
	seen           map[string]struct{}
}

// NewProcessLogBuffer cria uma nova instância de ProcessLogBuffer aplicando os defaults quando omitidos (<= 0).
func NewProcessLogBuffer(options ProcessLogBufferOptions) *ProcessLogBuffer

// Append analisa blocos/linhas do texto bruto, formata com emoji, deduplica, realiza trim e retorna o snapshot atual.
func (b *ProcessLogBuffer) Append(raw string) string

// Snapshot retorna todas as entradas atuais unidas por "\n".
func (b *ProcessLogBuffer) Snapshot() string

// Funções internas:
func collapseWhitespace(text string) string
func parseBlocks(raw string) []string
func pickEmoji(entry string) string
func toDisplayEntry(rawEntry string, maxEntryLength int) string
func (b *ProcessLogBuffer) trim()
func (b *ProcessLogBuffer) dropOldest()
```

#### `internal/monitor/progress_sender.go`

```go
package monitor

import (
	"sync"
	"time"
)

// ProgressSenderOptions configura o ProgressSender.
// Valores default:
// ThrottleMs = 3000 (se <= 0)
// MaxLength = 4000 (se <= 0)
// WrapInCodeBlock = true (se nil)
type ProgressSenderOptions struct {
	Send            func(content string) error
	ThrottleMs      int
	MaxLength       int
	WrapInCodeBlock *bool
}

// ProgressSender acumula pedaços de texto em um buffer e os emite periodicamente com throttle.
type ProgressSender struct {
	mu              sync.Mutex
	throttleMs      time.Duration
	maxLength       int
	wrapInCodeBlock bool
	buffer          string
	timer           *time.Timer
	sendContent     func(content string) error
}

// NewProgressSender cria uma nova instância de ProgressSender com defaults idênticos ao TS.
func NewProgressSender(options ProgressSenderOptions) *ProgressSender

// Append concatena text ao buffer e inicia o timer de throttle caso não esteja ativo.
func (p *ProgressSender) Append(text string)

// ForceEmit força o envio imediato do conteúdo acumulado no buffer e cancela qualquer timer pendente.
func (p *ProgressSender) ForceEmit()

// Dispose cancela qualquer timer pendente e limpa o buffer sem emitir nada.
func (p *ProgressSender) Dispose()

// Métodos internos:
func (p *ProgressSender) emit()
func (p *ProgressSender) splitByLength(text string, maxLength int) []string
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

### 3.1. `src/utils/htmlToTelegramMarkdown.ts` (`internal/formatter/html_converter.go`)

1. **Remoção de elementos invisíveis**:
   - `<style[^>]*>[\s\S]*?<\/style>` (case-insensitive, global)
   - `<script[^>]*>[\s\S]*?<\/script>` (case-insensitive, global)
2. **Quebras de linha e separadores**:
   - `<br\s*\/?>` $\rightarrow$ `"\n"`
   - `<hr\s*\/?>` $\rightarrow$ `"\n---\n"`
3. **Cabeçalhos (`h1` a `h6`)**:
   - `<h[1-6][^>]*>([\s\S]*?)<\/h[1-6]>` $\rightarrow$ `"\n<b>" + strings.TrimSpace(stripTags(content)) + "</b>\n"`
4. **Blocos de código (`<pre><code>`)**:
   - `<pre[^>]*>\s*<code(?:\s+class="language-([^"]*)")?[^>]*>([\s\S]*?)<\/code>\s*<\/pre>`
   - Se `lang != ""` $\rightarrow$ `"\n<pre><code class=\"language-" + lang + "\">" + content + "</code></pre>\n"`
   - Caso contrário $\rightarrow$ `"\n<pre>" + content + "</pre>\n"`
5. **Código inline (`<code>`)**:
   - `<code[^>]*>([\s\S]*?)<\/code>` $\rightarrow$ `"<code>$1</code>"`
6. **Negrito (`<strong>`, `<b>`)**:
   - Regex TS original: `/<(?:strong|b)(?:\s[^>]*)?>((?: |\s|[^<]|<(?!\/(?:strong|b)>))*)<\/(?:strong|b)>/gi` $\rightarrow$ `"<b>$1</b>"`
   - Nota de implementação em Go (`regexp` RE2): `(?is)<(?:strong|b)(?:\s[^>]*)?>(.*?)</(?:strong|b)>`
7. **Itálico (`<em>`, `<i>`)**:
   - Regex TS original: `/<(?:em|i)(?:\s[^>]*)?>((?: |\s|[^<]|<(?!\/(?:em|i)>))*)<\/(?:em|i)>/gi` $\rightarrow$ `"<i>$1</i>"`
   - Nota de implementação em Go (`regexp` RE2): `(?is)<(?:em|i)(?:\s[^>]*)?>(.*?)</(?:em|i)>`
8. **Tachado (`<del>`, `<strike>`, `<s>`)**:
   - `/<(?:del|strike|s)(?:\s[^>]*)?>([\s\S]*?)<\/(?:del|strike|s)>/gi` $\rightarrow$ `"<s>$1</s>"`
9. **Sublinhado (`<ins>`, `<u>`)**:
   - `/<(?:ins|u)(?:\s[^>]*)?>([\s\S]*?)<\/(?:ins|u)>/gi` $\rightarrow$ `"<u>$1</u>"`
10. **Highlight (`<mark>`)**:
    - `/<mark[^>]*>([\s\S]*?)<\/mark>/gi` $\rightarrow$ `"<b>$1</b>"`
11. **Links (`<a href="...">`)**:
    - `/<a\s+[^>]*href="([^"]*)"[^>]*>([\s\S]*?)<\/a>/gi`
    - `linkText = strings.TrimSpace(stripTags(text))`
    - Se `href == ""` ou `strings.HasPrefix(href, "#")` ou `strings.HasPrefix(href, "javascript:")`: retorna `linkText`.
    - Caso contrário: retorna `<a href="` + href + `">` + linkText + `</a>`.
12. **Blockquote (`<blockquote>`)**:
    - `/<blockquote[^>]*>([\s\S]*?)<\/blockquote>/gi` $\rightarrow$ `"\n<blockquote>" + strings.TrimSpace(stripTags(content)) + "</blockquote>\n"`
13. **Context-scope mentions (`.context-scope-mention`)**:
    - `/<span[^>]*class="[^"]*context-scope-mention[^"]*"[^>]*>([\s\S]*?)<\/span>/gi` $\rightarrow$ `"<code>" + strings.TrimSpace(stripTags(text)) + "</code>"`
14. **Elementos com atributo `title` (caminhos de arquivo)**:
    - `/<(?:div|span|a)[^>]*\btitle="([^"]*)"[^>]*>([\s\S]*?)<\/(?:div|span|a)>/gi`
    - Nota importante: em Go, como `<a ...>` com `href` já foi processado no passo 11, se um `<a href="...">` válido já foi convertido para `<a href="...">`, ele não possui atributo `title`. Para `div`, `span` ou `a` com `title="..."`: se `looksLikeFilePath(title)` for `true`, retorna `title + strings.TrimSpace(stripTags(text))`; caso contrário, retorna `stripTags(text)`.
15. **Imagens (`<img>`)**:
    - `/<img[^>]*alt="([^"]*)"[^>]*\/?>/gi` $\rightarrow$ se `alt != ""` retorna `"[" + alt + "]"`, senão `""`.
    - `/<img[^>]*\/?>/gi` $\rightarrow$ `""`
16. **Details/Summary (`<details><summary>`)**:
    - `/<details[^>]*>\s*<summary[^>]*>([\s\S]*?)<\/summary>([\s\S]*?)<\/details>/gi`
    - `summaryText = strings.TrimSpace(stripTags(summary))`
    - `contentText = strings.TrimSpace(stripTags(content))`
    - Se `contentText != ""`: `"\n<b>" + summaryText + "</b>\n" + contentText + "\n"`, senão `"\n<b>" + summaryText + "</b>\n"`.
17. **Figure/Figcaption (`<figure><figcaption>`)**:
    - `/<figure[^>]*>([\s\S]*?)<figcaption[^>]*>([\s\S]*?)<\/figcaption>[\s\S]*?<\/figure>/gi` $\rightarrow$ `strings.TrimSpace(stripTags(content)) + "\n<i>" + strings.TrimSpace(stripTags(caption)) + "</i>\n"`
    - `/<figure[^>]*>([\s\S]*?)<\/figure>/gi` $\rightarrow$ `"$1"`
18. **Tabelas (`<table>`, `<tr>`, `<td>`/`<th>`)**:
    - `/<table[^>]*>([\s\S]*?)<\/table>/gi`
    - `/<tr[^>]*>([\s\S]*?)<\/tr>/gi`
    - `/<(?:td|th)[^>]*>([\s\S]*?)<\/(?:td|th)>/gi`
19. **Listas de definição (`<dl>`, `<dt>`, `<dd>`)**:
    - `/<dl[^>]*>([\s\S]*?)<\/dl>/gi`
    - `/<dt[^>]*>([\s\S]*?)<\/dt>/gi` $\rightarrow$ `"<b>" + strings.TrimSpace(stripTags(text)) + "</b>\n"`
    - `/<dd[^>]*>([\s\S]*?)<\/dd>/gi` $\rightarrow$ `"  " + strings.TrimSpace(stripTags(text)) + "\n"`
20. **Sobrescrito e Subscrito (`<sup>`, `<sub>`)**:
    - `/<sup[^>]*>([\s\S]*?)<\/sup>/gi` $\rightarrow$ `"^" + strings.TrimSpace(stripTags(text))`
    - `/<sub[^>]*>([\s\S]*?)<\/sub>/gi` $\rightarrow$ `"_" + strings.TrimSpace(stripTags(text))`
21. **Elementos de bloco (`<p>`, `<div>`)**:
    - `/<p[^>]*>([\s\S]*?)<\/p>/gi` $\rightarrow$ `"$1\n\n"`
    - `/<div[^>]*>([\s\S]*?)<\/div>/gi` $\rightarrow$ `"$1\n"`
22. **Listas (`<ul>`, `<ol>`, `<li>`) — até 5 iterações**:
    - Teste de parada: `!/<(?:ul|ol)\b/i.test(result)`
    - Regex TS `<ul>`: `/<ul[^>]*>((?:(?!<\/?(?:ul|ol)\b)[\s\S])*?)<\/ul>/gi`
    - Regex TS `<ol>`: `/<ol[^>]*>((?:(?!<\/?(?:ul|ol)\b)[\s\S])*?)<\/ol>/gi`
    - Regex `<li>`: `/<li[^>]*>([\s\S]*?)<\/li>/gi`
23. **`looksLikeFilePath`**:
    - `/^[a-zA-Z0-9._\-/]+\.[a-zA-Z0-9]+$/` e `value.includes('/')`
24. **`stripTags`**:
    - `/<[^>]+>/g` $\rightarrow$ `""`
25. **`stripUnsupportedTags`**:
    - Tags permitidas: `/^\/?(b|i|u|s|code|pre|a|blockquote)(\s|>|\/|$)/i`
    - Varre `/<\/?[^>]+>/g`: remove `^<\/?` e `>$` para testar `inner` contra `allowed`. Se bater, preserva a tag original; senão substitui por `""`.
26. **Normalização final de quebras de linha**:
    - `/\n{3,}/g` $\rightarrow$ `"\n\n"` seguido de `.trim()`.

---

### 3.2. `src/utils/telegramFormatter.ts` (`internal/formatter/telegram_formatter.go`)

Reprodução 100% literal de `UI_CHROME_LITERALS`, `UI_CHROME_REGEXES` e `FILE_REF_REGEX`:

```typescript
const UI_CHROME_LITERALS = new Set([
    'analyzed', 'analyzing', 'reading', 'writing', 'running',
    'searching', 'planning', 'thinking', 'thinking...', 'processing',
    'loading', 'executing', 'testing', 'debugging', 'read', 'wrote',
    'ran', 'good', 'bad', 'good bad', 'show details',
    'json', 'css', 'html', 'xml', 'yaml', 'toml', 'sql', 'graphql',
]);

const UI_CHROME_REGEXES: RegExp[] = [
    /^[+-]\d+$/,
    /^\d+\s*chars?$/i,
    /^line\s+\d+/i,
    /^col\s+\d+/i,
    /^tool call:/i,
    /^tool result:/i,
    /^calling tool\b/i,
    /^tool response\b/i,
    /^mcp\b/i,
    /^thought for\s*<?\d+/i,
    /^show details$/i,
    /^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i,
    /^full output written to\b/i,
    /^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i,
    /^\s*\{\s*$/,
    /^\s*\}\s*$/,
    /^\s*"[^"]*"\s*:\s*/,
];

const FILE_REF_REGEX = /(?<![`/\\])(\b[a-zA-Z][\w.-]*(?:\/[\w.-]+)+(?::\d+(?:-\d+)?)?)\s?(?!`)/g;
```

Outras expressões regulares literais de `telegramFormatter.ts`:
- `isTableLine`: `(trimmed.startsWith('|') && trimmed.length > 2) || /^\|[\s\-:]+\|/.test(trimmed)`
- `isTreeLine`: `/[├└│┌┐┘┤┬┴┼]/.test(line) || /^\s*[│├└]\s*──/.test(line) || /^\s*\|.*──/.test(line)`
- `quoteContent`: `trimmed.replace(/^>\s?/, '')`
- `headingMatch`: `result.match(/^(#{1,6})\s+(.+)$/)`
- `horizontalRule`: `/^(?:---+|\*\*\*+|___+)\s*$/.test(result.trim())`
- `inlineCode`: ``/`([^`]+)`/g`` $\rightarrow$ `"<code>$1</code>"`
- `links`: `/\[([^\]]+)\]\(([^)]+)\)/g` $\rightarrow$ `"<a href=\"$2\">$1</a>"`
- `bold`: `/\*\*(.+?)\*\*/g` $\rightarrow$ `"<b>$1</b>"` e `/__(.+?)__/g` $\rightarrow$ `"<b>$1</b>"`
- `italic`: `/(?<!\w)\*([^*]+)\*(?!\w)/g` $\rightarrow$ `"<i>$1</i>"` e `/(?<!\w)_([^_]+)_(?!\w)/g` $\rightarrow$ `"<i>$1</i>"`
- `strikethrough`: `/~~(.+?)~~/g` $\rightarrow$ `"<s>$1</s>"`
- `isMcpFormatLine`: `/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i`
- `isActivityLogLine`: `/^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|analyzed|read|wrote|ran)\s+.+/i.test(trimmed) && trimmed.length <= 220`
- `collapseBlankLines`: `text.replace(/\n{3,}/g, '\n\n').trim()`
- `splitTelegramHtml` tag tracking: `/<\/?(\w+)(?:\s[^>]*)?\s*\/?>/g` e `tag.match(/^<(\w+)/)`

---

### 3.3. `src/utils/streamMessageFormatter.ts` (`internal/formatter/stream_formatter.go`)

Strings e expressões regulares literais:
- Linhas de `buildModeModelLines`:
  - `"Chế độ hiện tại: " + modeName`
  - Se `fastModel.trim().toLowerCase() === planModel.trim().toLowerCase()`:
    - `"Mô hình: " + fastModel`
  - Caso contrário:
    - `"Mô hình Nhanh: " + fastModel`
    - `"Mô hình Kế hoạch: " + planModel`
- Expressões regulares de `shouldSkipActivityLog`:
  - Status genérico: `/^(?:fast|planning|plan|generating\.*|thinking\.*|processing\.*|working\.*)$/`
  - Palavra única ruidosa: `/^[a-z][a-z0-9_-]{1,24}[.!…]?$/`
  - Trace detalhado de leitura: `/^analyzed/`
- Prefixo de truncamento em `fitForSingleEmbedDescription`:
  - `"...(phần đầu được lược bớt)\n"`
  - Limite padrão: `maxLength = 3500`

---

### 3.4. `src/utils/metadataExtractor.ts` (`internal/formatter/metadata.go`)

Expressões regulares literais:
- `TaskID`: `/TaskID:\s*([^\s|]+)/i`
- `Dir`: `/Dir:\s*([^\s|]+)/i`

---

### 3.5. `src/utils/processLogBuffer.ts` (`internal/monitor/process_log_buffer.go`)

Constantes e expressões regulares literais:
- `DEFAULT_MAX_CHARS = 3500`
- `DEFAULT_MAX_ENTRIES = 120`
- `DEFAULT_MAX_ENTRY_LENGTH = 260`
- Regras de `pickEmoji(entry)` (avaliadas exatamente nesta ordem):
  1. `/^thought for\b/.test(lower) || /^thinking\b/.test(lower)` $\rightarrow$ `'🧠'`
  2. `/^initiating\b/.test(lower) || /^starting\b/.test(lower)` $\rightarrow$ `'🚀'`
  3. `/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(entry)` $\rightarrow$ `'🛠️'`
  4. `/^(?:analy[sz]ed|read|wrote|created|updated|deleted|built|compiled|installed|resolved|downloaded|connected|fetched)\b/i.test(entry)` $\rightarrow$ `'📄'`
  5. `/^(?:analy[sz]ing|reading|writing|running|searching|fetching|checking|scanning|creating|updating|deleting|building|compiling|deploying|parsing|resolving|downloading|uploading|connecting|installing|executing|testing|debugging|processing|loading)\b/i.test(entry)` $\rightarrow$ `'🔍'`
  6. `/^title:\s/.test(lower) && /\surl:\s/.test(lower)` $\rightarrow$ `'🔎'`
  7. `/^(json|javascript|typescript|python|bash|sh|html|css|xml|yaml|yml|toml|sql|graphql|markdown|text|plaintext|log)$/i.test(entry)` $\rightarrow$ `'📦'`
  8. Default $\rightarrow$ `'•'`

---

### 3.6. `src/services/progressSender.ts` (`internal/monitor/progress_sender.go`)

Constantes padrão literais:
- `throttleMs`: `3000`
- `maxLength`: `4000`
- `wrapInCodeBlock`: `true`

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Conversor HTML para Telegram HTML (`HtmlToTelegramHtml`)

O algoritmo executa uma sequência estritamente ordenada de transformações sobre a string `html`:
1. **Guarda de entrada vazia**: se `html == ""`, retorna `""`.
2. **Remoção de tags invisíveis**: remove blocos `<style...>...</style>` e `<script...>...</script>` (incluindo quebras de linha internas).
3. **Quebras e separadores**: substitui `<br>` / `<br/>` por `\n` e `<hr>` / `<hr/>` por `\n---\n`.
4. **Headings**: converte `<h1..h6>content</h1..h6>` aplicando `stripTags(content)` e `strings.TrimSpace`, envolvendo em `\n<b>...</b>\n`.
5. **Blocos `<pre><code>`**: deve rodar **antes** do `<code>` inline. Se houver `class="language-X"`, gera `\n<pre><code class="language-X">content</code></pre>\n`; se não houver classe de linguagem (ex: `<pre><code>plain</code></pre>`), gera `\n<pre>content</pre>\n`.
6. **Código inline**: converte `<code...>content</code>` em `<code>content</code>`.
7. **Formatação inline**:
   - `<strong>` / `<b>` $\rightarrow$ `<b>$1</b>`
   - `<em>` / `<i>` $\rightarrow$ `<i>$1</i>`
   - `<del>` / `<strike>` / `<s>` $\rightarrow$ `<s>$1</s>`
   - `<ins>` / `<u>` $\rightarrow$ `<u>$1</u>`
   - `<mark>` $\rightarrow$ `<b>$1</b>`
8. **Links `<a href="...">`**: extrai `href` e `linkText = strings.TrimSpace(stripTags(text))`. Se `href` for vazio, começar com `#` ou começar com `javascript:`, descarta a tag e retorna apenas `linkText`. Caso contrário, retorna `<a href="${href}">${linkText}</a>`.
9. **Blockquotes**: converte `<blockquote...>content</blockquote>` em `\n<blockquote>${strings.TrimSpace(stripTags(content))}</blockquote>\n`.
10. **Menções de contexto (`.context-scope-mention`)**: qualquer `<span ... class="...context-scope-mention...">text</span>` vira `<code>${strings.TrimSpace(stripTags(text))}</code>`.
11. **Atributos `title` com caminho de arquivo**: qualquer `<div|span|a ... title="TITLE"...>TEXT</div|span|a>` (que ainda possua atributo `title`): se `looksLikeFilePath(title)` for verdadeiro (contém `/` e casa com `^[a-zA-Z0-9._\-/]+\.[a-zA-Z0-9]+$`), retorna `title + strings.TrimSpace(stripTags(text))` (ex: `<div title="src/bot/index.ts">:54</div>` $\rightarrow$ `src/bot/index.ts:54`). Caso contrário, retorna `stripTags(text)`.
12. **Imagens**: `<img ... alt="ALT" ...>` vira `[ALT]` se `ALT` não for vazio, ou `""` se vazio. Qualquer `<img ...>` restante sem `alt` é removido (`""`).
13. **Details / Summary**: `<details...><summary...>S</summary>C</details>` extrai `summaryText` e `contentText` (ambos com `stripTags` + `TrimSpace`). Se `contentText != ""`, retorna `\n<b>${summaryText}</b>\n${contentText}\n`; senão `\n<b>${summaryText}</b>\n`.
14. **Figure / Figcaption**: `<figure...>C<figcaption...>CAP</figcaption>...</figure>` vira `${stripTags(C).trim()}\n<i>${stripTags(CAP).trim()}</i>\n`. `<figure...>C</figure>` sem `figcaption` vira `C`.
15. **Tabelas HTML (`<table>`)**:
    - Extrai todas as linhas `<tr>`. Para cada `<tr>`, extrai todas as células `<td>` ou `<th>`.
    - Para cada célula: aplica `stripTags(cell).trim()` e escapa `&` $\rightarrow$ `&amp;`, `<` $\rightarrow$ `&lt;`, `>` $\rightarrow$ `&gt;`.
    - Se a linha tiver `cells.length > 0`, adiciona à matriz `rows`.
    - Se `rows.length == 0`, retorna `""`.
    - Calcula `colCount = max(len(r))` e, para cada coluna `c`, `colWidths[c] = max(len(r[c]), 1)`.
    - Formata cada linha como `| cell0.padEnd(w0) | cell1.padEnd(w1) |`.
    - Logo após a primeira linha (`ri == 0`), insere a linha separadora `'|-'.repeat(w + 2)...|'` (ex: `|` + `strings.Repeat("-", w+2)` unido por `|` + `|`).
    - Retorna `\n<pre>${lines.join('\n')}</pre>\n`.
16. **Listas de definição (`<dl>`)**: substitui `<dt>` por `<b>${stripTags(text).trim()}</b>\n` e `<dd>` por `  ${stripTags(text).trim()}\n`, prefixando o bloco com `\n`.
17. **Superscript / Subscript**: `<sup>T</sup>` $\rightarrow$ `^${stripTags(T).trim()}`, `<sub>T</sub>` $\rightarrow$ `_${stripTags(T).trim()}`.
18. **Parágrafos e Divs**: `<p...>C</p>` $\rightarrow$ `C\n\n`; `<div...>C</div>` $\rightarrow$ `C\n`.
19. **Listas `<ul>` e `<ol>` (até 5 iterações)**:
    - Em cada iteração (0 a 4), se não houver `<ul` nem `<ol`, interrompe.
    - Substitui os blocos `<ul>` mais internos (que não contenham `<ul` ou `<ol` aninhados) convertendo cada `<li>T</li>` em `- ${stripTags(T).trim()}\n` e prefixando com `\n`.
    - Substitui os blocos `<ol>` mais internos convertendo cada `<li>T</li>` em `${counter}. ${stripTags(T).trim()}\n` (com `counter` iniciando em 1 para cada `<ol>`) e prefixando com `\n`.
20. **Limpeza final**:
    - `stripUnsupportedTags`: remove qualquer tag HTML cujo nome não seja `b`, `i`, `u`, `s`, `code`, `pre`, `a` ou `blockquote`.
    - `decodeSafeEntities`: decodifica `&quot;` ($\rightarrow$ `"`), `&#39;` e `&#x27;` ($\rightarrow$ `'`), `&nbsp;` ($\rightarrow$ `' '`), entidades hexadecimais `&#xHEX;` e decimais `&#DEC;`. Se o codepoint decodificado de uma entidade numérica for `&`, `<` ou `>`, converte para `&amp;`, `&lt;` ou `&gt;` respectivamente! Entidades literais `&amp;`, `&lt;` e `&gt;` já presentes permanecem intocadas.
    - Colapsa 3 ou mais quebras de linha (`\n{3,}`) em `\n\n` e aplica `strings.TrimSpace`.

---

### 4.2. Formatador Markdown e Paginador HTML (`telegram_formatter.go`)

#### 4.2.1. `FormatForTelegram(text string) string`
1. Divide `text` por `\n`. Mantém estados booleanos: `inSpecialBlock`, `inCodeBlock`, `codeBlockHasLang`, `inBlockquote` e slice `blockquoteLines`.
2. Função auxiliar `flushBlockquote()`: se `len(blockquoteLines) > 0`, adiciona `<blockquote>` + `strings.Join(blockquoteLines, "\n")` + `</blockquote>` a `result`, limpa `blockquoteLines` e define `inBlockquote = false`.
3. Para cada `line`:
   - `trimmed = strings.TrimSpace(line)`
   - Se `strings.HasPrefix(trimmed, "```")`:
     - Se `inBlockquote`, chama `flushBlockquote()`.
     - Inverte `inCodeBlock = !inCodeBlock`.
     - Se entrou em `inCodeBlock`: extrai `lang = strings.TrimSpace(trimmed[3:])`. Se `lang != ""`, `codeBlockHasLang = true` e adiciona `<pre><code class="language-${EscapeHtml(lang)}">`; senão adiciona `<pre>`.
     - Se saiu de `inCodeBlock`: adiciona `</code></pre>` se `codeBlockHasLang`, senão `</pre>`, e reseta `codeBlockHasLang = false`.
     - Continua para a próxima linha.
   - Se `inCodeBlock` for `true`: adiciona `EscapeHtml(line)` e continua.
   - Avalia `isTableLine` e `isTreeLine`: `isSpecialLine = isTableLine || isTreeLine`.
   - Se `strings.HasPrefix(trimmed, ">") && !isSpecialLine`:
     - Se `inSpecialBlock`, adiciona `</pre>` e `inSpecialBlock = false`.
     - Define `inBlockquote = true`, remove `^>\s?` de `trimmed`, aplica `formatMarkdownLine(EscapeHtml(quoteContent))` e adiciona a `blockquoteLines`. Continua.
   - Senão, se `inBlockquote` for `true`: chama `flushBlockquote()`.
   - Tratamento de `isSpecialLine`:
     - Se `isSpecialLine && !inSpecialBlock`: adiciona `<pre>`, define `inSpecialBlock = true`, adiciona `EscapeHtml(line)`.
     - Se `isSpecialLine && inSpecialBlock`: adiciona `EscapeHtml(line)`.
     - Se `!isSpecialLine && inSpecialBlock`: adiciona `</pre>`, define `inSpecialBlock = false`, e adiciona `formatMarkdownLine(wrapFileReferences(EscapeHtml(line)))`.
     - Caso contrário: adiciona `formatMarkdownLine(wrapFileReferences(EscapeHtml(line)))`.
4. Ao final do loop:
   - Se `inBlockquote`, chama `flushBlockquote()`.
   - Se `inSpecialBlock`, adiciona `</pre>`.
   - Se `inCodeBlock`, adiciona `</code></pre>` (se `codeBlockHasLang`) ou `</pre>`.
5. Retorna `strings.Join(result, "\n")`.

#### 4.2.2. Compatibilidade RE2 para `wrapFileReferences` e `formatMarkdownLine` em Go
Como Go usa RE2 (sem lookbehind/lookahead):
- **`wrapFileReferences(line string) string`**:
  - No TS: `/(?<![`/\\])(\b[a-zA-Z][\w.-]*(?:\/[\w.-]+)+(?::\d+(?:-\d+)?)?)\s?(?!`)/g` substitui por `'<code>$1</code>'`. (Note que o `\s?` opcional após o caminho é consumido pela regex no TS se não for seguido de `` ` ``).
  - Em Go, usa-se a regex base `reFileRef = regexp.MustCompile(`\b[a-zA-Z][\w.-]*(?:/[\w.-]+)+(?::\d+(?:-\d+)?)?`)` encontrando os índices `FindAllStringIndex(line, -1)`:
    - Verifica o caractere imediatamente anterior a `start`: se `start > 0` e `line[start-1]` for `` ` ``, `'/'` ou `'\\'`, ignora o match.
    - Verifica os caracteres após `end`: se `end < len(line)` e `line[end] == '`'`, ignora o match. Se `end < len(line)` e for espaço (`' '` ou `'\t'`), verifica se o caractere seguinte ao espaço não é `` ` ``: se não for `` ` ``, consome esse 1 caractere de espaço (igual ao `\s?(?!`)` do TS); se for `` ` ``, não consome o espaço e envolve apenas o caminho!
- **Itálico em `formatMarkdownLine(line string) string`**:
  - No TS: `/(?<!\w)\*([^*]+)\*(?!\w)/g` e `/(?<!\w)_([^_]+)_(?!\w)/g`.
  - Em Go, varre matches de `\*([^*]+)\*` e `_([^_]+)_` verificando que o byte/runa anterior a `start` (se existir) não é word char (`[a-zA-Z0-9_]`) e o byte/runa em `end` (se existir) não é word char (`[a-zA-Z0-9_]`).

#### 4.2.3. `SplitOutputAndLogs(rawText string) SplitOutputResult`
1. Normaliza `\r` removendo-o. Se `strings.TrimSpace(normalized) == ""`, retorna `{Output: "", Logs: ""}`.
2. Classifica cada linha em `'output' | 'chrome' | 'blank' | 'code'`:
   - Linhas iniciadas com ` ``` ` alternam `inCodeBlock` e recebem classe `'code'`.
   - Enquanto `inCodeBlock == true`, todas as linhas recebem `'code'`.
   - Linhas vazias após trim recebem `'blank'`.
   - Se `isUiChromeLine(trimmed) || isActivityLogLine(trimmed)`, recebe `'chrome'`; senão `'output'`.
3. Verifica `hasMcpCalls`: existe alguma linha `i` com `classes[i] == 'chrome'` e `isMcpFormatLine(lines[i].trim())`?
4. **Caminho A (`!hasMcpCalls`)**:
   - Todas as linhas `'chrome'` (com `.trim()`) vão para `logLines`; todas as demais vão para `outputLines`.
   - Retorna `{Output: collapseBlankLines(outputLines), Logs: collapseBlankLines(logLines)}`.
5. **Caminho B (`hasMcpCalls == true`)**:
   - Procura de trás para frente o último índice `lastOutputEnd` onde `classes[i]` é `'output'` ou `'code'`.
   - Se `lastOutputEnd == -1`: retorna `{Output: "", Logs: collapseBlankLines(todas as linhas chrome)}`.
   - Recua de `lastOutputEnd - 1` até `0` buscando `lastOutputStart`: para no primeiro `'blank'` ou `'chrome'`.
   - Linhas no intervalo `[lastOutputStart, lastOutputEnd]` que não sejam `'chrome'` vão para `outputLines`; todas as outras linhas `'chrome'` ou `'output'` fora desse bloco final vão para `logLines` (com `.trim()`).

#### 4.2.4. `SplitTelegramHtml(html string, maxLen int) []string`
1. Se `maxLen <= 0`, usa `4096`. Se `len(html) <= maxLen`, retorna `[]string{html}`.
2. Mantém uma pilha `openTags []string` com as tags de abertura brutas (ex: `<pre>`, `<code class="language-ts">`, `<b>`).
3. `closingFor(tag)` extrai o nome da tag (`^<(\w+)`) e retorna `</name>`.
4. `trackTags(line)` percorre `/<\/?(\w+)(?:\s[^>]*)?\s*\/?>/g`:
   - Ignora self-closing (`strings.HasSuffix(full, "/>")`) e `<br>` (`tagName == "br"`).
   - Se for fechamento (`</...`), busca de trás para frente em `openTags` a última tag com o mesmo `tagName` (case-insensitive) e a remove da pilha.
   - Se for abertura, empilha `full` em `openTags`.
5. Para cada `line` de `strings.Split(html, "\n")`:
   - Simula a adição da linha em `tempTags` para calcular `newClosingLen` (soma dos tamanhos das tags de fechamento que ficarão abertas após a linha).
   - Se `len(candidate) + newClosingLen <= maxLen`: acumula `current = candidate`, atualiza `trackTags(line)` e continua.
   - Se não couber: chama `flush()` (que fecha as tags abertas de `current` em ordem reversa, adiciona ao slice `chunks` e reinicializa `current = strings.Join(openTags, "")`).
   - Se `reopenLen + len(line) + newClosingLen <= maxLen`: define `current = strings.Join(openTags, "") + line`, chama `trackTags(line)` e continua.
   - Caso a única linha seja maior que o limite (*hard-split*): fatia a linha em pedaços de tamanho `available = maxLen - reopenLen - newClosingLen` (garantindo `available >= 1` nas iterações seguintes), chamando `trackTags(slice)` e `flush()` a cada fatia intermediária.
6. Ao final, se `current != ""`, fecha as tags abertas restantes e adiciona a `chunks`.

---

### 4.3. `ProcessLogBuffer` (`internal/monitor/process_log_buffer.go`)

1. **`parseBlocks(raw)`**:
   - Remove `\r` e aplica `TrimSpace`. Se vazio, retorna `[]`.
   - Divide por `\n{2,}`, aplica `collapseWhitespace` em cada pedaço e filtra vazios.
   - Se produzir ao menos 1 bloco, retorna esses blocos; caso contrário, divide por `\n`.
   - *Nota importante*: como ` normalized.split(/\n{2,}/)` em uma string não-vazia sempre retorna pelo menos 1 elemento, um texto separado apenas por `\n` simples sem linhas em branco duplas terá suas quebras simples colapsadas por `collapseWhitespace` em um único bloco, enquanto textos separados por `\n\n` geram múltiplos blocos independentes.
2. **`toDisplayEntry(rawEntry, maxEntryLength)`**:
   - Aplica `collapseWhitespace`. Se vazio, retorna `""`.
   - Se `len(trimmed) > maxEntryLength`: trunca para `trimmed[:max(0, maxEntryLength-3)] + "..."`.
   - Prefixa com `pickEmoji(clipped) + " " + clipped`.
3. **`Append(raw)`**:
   - Para cada bloco de `parseBlocks(raw)`: gera `display = toDisplayEntry(block, b.maxEntryLength)`.
   - Calcula `key = strings.ToLower(display)`. Se já estiver em `b.seen`, ignora.
   - Adiciona `display` a `b.entries` e `key` a `b.seen`.
   - Chama `b.trim()` e retorna `b.Snapshot()`.
4. **`trim()`**:
   - Enquanto `len(b.entries) > b.maxEntries`, chama `b.dropOldest()`.
   - Calcula `totalLen` = soma dos tamanhos de `b.entries` + `max(0, len(b.entries)-1)` (separadores `\n`).
   - Enquanto `len(b.entries) > 1 && totalLen > b.maxChars`: remove o mais antigo (`b.dropOldest()`) e subtrai `len(removed) + 1` de `totalLen`.
   - Se restar `len(b.entries) == 1 && len(b.entries[0]) > b.maxChars`: trunca `b.entries[0]` para `only[:max(0, b.maxChars-3)] + "..."`, limpa `b.seen` e insere `strings.ToLower(b.entries[0])`.

---

### 4.4. `ProgressSender` (`internal/monitor/progress_sender.go`)

1. **`Append(text)`**:
   - Concatena `p.buffer += text`.
   - Se `p.timer == nil`, agenda um timer para daqui a `p.throttleMs` que invocará `p.emit()`.
2. **`ForceEmit()`**:
   - Invoca `p.emit()` imediatamente.
3. **`Dispose()`**:
   - Se `p.timer != nil`, cancela o timer e define `p.timer = nil`.
   - Limpa `p.buffer = ""`.
4. **`emit()`**:
   - Se `p.timer != nil`, cancela o timer e define `p.timer = nil`.
   - Se `p.buffer == ""` retorna imediatamente sem chamar `sendContent`.
   - Copia `payload := p.buffer` e limpa `p.buffer = ""`.
   - Divide `payload` em fatias de até `p.maxLength` caracteres (`splitByLength`).
   - Para cada `chunk`:
     - Escapa `&` $\rightarrow$ `&amp;`, `<` $\rightarrow$ `&lt;`, `>` $\rightarrow$ `&gt;`.
     - Se `p.wrapInCodeBlock` for `true`, envia `<pre>` + escaped + `</pre>`.
     - Se `p.wrapInCodeBlock` for `false`, envia `chunk` (sem `<pre>` e sem escape HTML, idêntico ao TS: `const content = this.wrapInCodeBlock ? '<pre>${escaped}</pre>' : chunk`).
     - Invoca `p.sendContent(content)` ignorando erros silenciosamente (`_ = p.sendContent(content)`).

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Funções Puras (`formatter`)**:
   - Todas as funções do pacote `internal/formatter` (`HtmlToTelegramHtml`, `EscapeHtml`, `FormatForTelegram`, `SplitOutputAndLogs`, `SeparateOutputForDelivery`, `SplitTelegramHtml`, `SanitizeActivityLines`, `BuildModeModelLines`, `ShouldSkipActivityLog`, `SplitForEmbedDescription`, `FitForSingleEmbedDescription`, `ExtractMetadataFromFooter`) são 100% *stateless* e seguras para uso concorrente por múltiplas goroutines. As expressões regulares devem ser pré-compiladas em variáveis de nível de pacote (`var reXxx = regexp.MustCompile(...)`).
2. **`ProcessLogBuffer`**:
   - Protegido por `sync.Mutex` (`b.mu.Lock()` / `defer b.mu.Unlock()`) nos métodos públicos `Append` e `Snapshot`, pois pode ser acessado concorrentemente pela goroutine de polling do `ResponseMonitor` e pelo ticker de atualização de status do bot (`elapsedTimer` a cada `5000ms`).
3. **`ProgressSender`**:
   - Protegido por `sync.Mutex` (`p.mu`).
   - Quando o `time.AfterFunc(p.throttleMs, ...)` dispara em uma goroutine do runtime Go, ele adquire `p.mu.Lock()`, extrai os chunks a serem enviados, zera `p.buffer` e `p.timer`, libera o lock (ou executa o callback de forma que não haja deadlock caso `Dispose()` seja chamado) e invoca `sendContent`.
   - `Dispose()` pode ser chamado múltiplas vezes com segurança (idempotente) e permite que novos `Append()` posteriores reiniciem o ciclo normalmente.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os casos abaixo mapeiam 1:1 cada bloco `describe`/`it` dos 8 arquivos de teste originais e devem passar com `go test ./internal/formatter/... ./internal/monitor/...`.

### 6.1. `internal/formatter/html_converter_test.go` (Origem: [`tests/utils/htmlToTelegramMarkdown.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/htmlToTelegramMarkdown.test.ts))

| Função de Teste Go | Caso Original (`describe` / `it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestHtmlToTelegramHtml_BasicConversion` | `basic conversion > returns empty string for empty input` / `preserves plain text` | `HtmlToTelegramHtml("") == ""` e `HtmlToTelegramHtml("Hello world") == "Hello world"` |
| `TestHtmlToTelegramHtml_FormattingTags` | `formatting tags > converts <strong>, <em>, <del>, <strike>, <ins>, <mark>` | `<strong>bold</strong>` $\rightarrow$ `<b>bold</b>`; `<em>italic</em>` $\rightarrow$ `<i>italic</i>`; `<del>deleted</del>` $\rightarrow$ `<s>deleted</s>`; `<strike>struck</strike>` $\rightarrow$ `<s>struck</s>`; `<ins>underline</ins>` $\rightarrow$ `<u>underline</u>`; `<mark>highlighted</mark>` $\rightarrow$ `<b>highlighted</b>` |
| `TestHtmlToTelegramHtml_Headings` | `headings > converts h1, h3 to bold, strips nested tags` | `<h1>Title</h1>` $\rightarrow$ `<b>Title</b>`; `<h3>Subtitle</h3>` $\rightarrow$ `<b>Subtitle</b>`; `<h2><span>Nested</span> heading</h2>` $\rightarrow$ `<b>Nested heading</b>` |
| `TestHtmlToTelegramHtml_CodeBlocks` | `code blocks > inline code, pre/code with lang, pre/code without lang` | `<code>const x = 1</code>` $\rightarrow$ `<code>const x = 1</code>`; `<pre><code class="language-js">console.log("hi")</code></pre>` contém `<pre>`, `<code` e `console.log("hi")`; `<pre><code>plain code</code></pre>` contém `<pre>` e `plain code` |
| `TestHtmlToTelegramHtml_Links` | `links > preserves href, strips # anchor, strips javascript:` | `<a href="https://example.com">click here</a>` $\rightarrow$ `<a href="https://example.com">click here</a>`; `<a href="#section">jump</a>` contém `jump` e não contém `href="#section"`; `<a href="javascript:alert(1)">xss</a>` contém `xss` e não contém `javascript:` |
| `TestHtmlToTelegramHtml_LineBreaksAndSeparators` | `line breaks and separators > <br>, <br/>, <hr>` | `a<br>b` $\rightarrow$ `a\nb`; `a<br/>b` $\rightarrow$ `a\nb`; `a<hr>b` contém `---` |
| `TestHtmlToTelegramHtml_Blockquotes` | `blockquotes > preserves blockquote tags` | `<blockquote>quoted text</blockquote>` $\rightarrow$ `<blockquote>quoted text</blockquote>` |
| `TestHtmlToTelegramHtml_Lists` | `lists > converts ul to dashes and ol to numbers` | `<ul><li>item 1</li><li>item 2</li></ul>` contém `- item 1` e `- item 2`; `<ol><li>first</li><li>second</li></ol>` contém `1. first` e `2. second` |
| `TestHtmlToTelegramHtml_Images` | `images > shows alt text, removes images without alt` | `<img alt="diagram" src="img.png">` contém `[diagram]`; `before<img src="img.png">after` contém `beforeafter` |
| `TestHtmlToTelegramHtml_Tables` | `tables > converts HTML table to ASCII table with pipes, empty table` | `<table><tr><th>Name</th><th>Value</th></tr><tr><td>foo</td><td>42</td></tr></table>` contém `Name`, `foo`, `|`; `<table></table>` $\rightarrow$ `""` |
| `TestHtmlToTelegramHtml_EntityDecoding` | `entity decoding > &amp;, &lt;, &gt;, &nbsp;, &#65;, &#x41;, &quot;, &#39;` | Preserva `A &amp; B` e `&lt;tag&gt;`; decodifica `a&nbsp;b` $\rightarrow$ `a b`, `&#65;` $\rightarrow$ `A`, `&#x41;` $\rightarrow$ `A`, `&quot;hello&quot;` $\rightarrow$ `"hello"`, `it&#39;s` $\rightarrow$ `it's` |
| `TestHtmlToTelegramHtml_TagStripping` | `tag stripping > strips <span>, removes <style> and <script>, strips <div> and <p>` | `<span>text</span>` $\rightarrow$ `text`; `<style>.cls { color: red; }</style>Hello` $\rightarrow$ `Hello`; `<script>alert(1)</script>Hello` $\rightarrow$ `Hello`; `<div>content</div>` $\rightarrow$ `content`; `<p>paragraph</p>` $\rightarrow$ `paragraph` |
| `TestHtmlToTelegramHtml_DetailsSummary` | `details/summary > shows summary as bold with content` | `<details><summary>Click me</summary>Hidden content</details>` contém `<b>Click me</b>` e `Hidden content` |
| `TestHtmlToTelegramHtml_SupSub` | `superscript/subscript > ^ and _ prefixes` | `x<sup>2</sup>` contém `x^2`; `H<sub>2</sub>O` contém `H_2O` |

### 6.2. `internal/formatter/telegram_formatter_test.go` (Origem: [`tests/utils/telegramFormatter.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/telegramFormatter.test.ts))

| Função de Teste Go | Caso Original (`describe` / `it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestEscapeHtml` | `escapeHtml() > escapes &, <, >, empty string, no special chars, multiple` | `a & b < c > d` $\rightarrow$ `a &amp; b &lt; c &gt; d`; `""` $\rightarrow$ `""`; `hello world` $\rightarrow$ `hello world`; `<<>>` $\rightarrow$ `&lt;&lt;&gt;&gt;` |
| `TestFormatForTelegram` | `formatForTelegram() > code blocks, lang, escape in code, headings, bold, italic, strike, inline code, links, tables, trees, blockquotes, unclosed code, hr, file refs` | Verifica todos os 15 subcasos: `<pre>`, `language-typescript`, `&lt;div&gt;`, `<b>Heading</b>`, `<b>bold</b>`, `<i>italic</i>`, `<s>deleted</s>`, `<code>npm install</code>`, `<a href="https://example.com">click</a>`, tabelas e árvores em `<pre>`, `<blockquote>`, fechamento automático de `</pre>`, `---`, e `<code>src/utils/config.ts</code>` |
| `TestSplitOutputAndLogs` | `splitOutputAndLogs() > empty, UI chrome lines, activity log lines, code blocks, only UI chrome, no chrome lines` | Verifica todos os subcasos: entrada vazia retorna `{Output: "", Logs: ""}`; `analyzed\nHello world\nreading files` separa `Hello world` em `Output` e `analyzed`/`reading files` em `Logs`; `reading src/index.ts` vai para `Logs`; blocos de código ficam em `Output`; só chrome resulta em `Output == ""` |
| `TestSeparateOutputForDelivery` | `separateOutputForDelivery() > dom-structured, legacy-fallback, undefined domOutputText` | Quando `DomSource == "dom-structured"` e `DomOutputText != nil`, retorna `Source: "dom-structured"`, `Output: *DomOutputText`, `Logs: strings.Join(DomActivityLines, "\n")`; quando `"legacy-fallback"` ou `DomOutputText == nil`, retorna `Source: "legacy-fallback"` usando `SplitOutputAndLogs` |
| `TestSanitizeActivityLines` | `sanitizeActivityLines() > removes UI chrome, deduplicates, trims/filters empty, handles empty` | `analyzed\nreal content\nthinking` $\rightarrow$ `real content`; `foo\nfoo\nbar` $\rightarrow$ `foo\nbar`; `  hello  \n\n  world  ` $\rightarrow$ `hello\nworld`; `""` $\rightarrow$ `""` |

### 6.3. `internal/formatter/split_telegram_html_test.go` (Origem: [`tests/utils/splitTelegramHtml.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/splitTelegramHtml.test.ts))

| Função de Teste Go | Caso Original (`it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestSplitTelegramHtml_UnderAndExactLimit` | `returns single chunk when text is under limit` / `when exactly at limit` | `"Hello world"` (limit 100) e `strings.Repeat("a", 100)` (limit 100) retornam 1 único chunk idêntico |
| `TestSplitTelegramHtml_LineBoundaries` | `splits at line boundaries` | 3 linhas de 40 chars com limite 85 $\rightarrow$ 2 chunks: `line1 + "\n" + line2` e `line3` |
| `TestSplitTelegramHtml_ReopensBoldTags` | `closes and reopens <b> tags across chunks` | `<b>` com 2 linhas de 50 `'x'` e limite 60 $\rightarrow$ 2 chunks, `chunks[0]` termina com `</b>`, `chunks[1]` começa com `<b>` e fecha `</b>` |
| `TestSplitTelegramHtml_ReopensPreCodeTags` | `closes and reopens <pre><code> across chunks` | `<pre><code>` com 3 linhas e limite 50 $\rightarrow$ cada chunk tem o mesmo número de `<pre>` e `</pre>` |
| `TestSplitTelegramHtml_NestedPreCodeWithClass` | `handles nested <pre><code class="language-ts"> tags` | 10 linhas dentro de `<pre><code class="language-ts">` com limite 60 $\rightarrow$ todos os chunks fecham `</code>` e `</pre>` |
| `TestSplitTelegramHtml_HardSplitLongLine` | `hard-splits a single very long line` | 200 `'x'` com limite 80 $\rightarrow$ 3 chunks cuja concatenação é idêntica ao original |
| `TestSplitTelegramHtml_EmptyAndItalicAndMaxLen` | `handles empty string`, `handles text with <i> tags`, `preserves content`, `does not produce chunks exceeding maxLen`, `handles mixed tags` | Verifica `""` $\rightarrow$ `[""]`, balanceamento de `<i>...</i>`, preservação de todas as 20 linhas, `len(chunk) <= 300` para 50 linhas e tags mistas |

### 6.4. `internal/formatter/stream_formatter_test.go` (Origem: [`tests/utils/streamMessageFormatter.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/streamMessageFormatter.test.ts))

| Função de Teste Go | Caso Original (`describe` / `it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestBuildModeModelLines` | `consolidates into a single line when both models are the same` / `shows both Fast and Plan models when they differ` | Modelos iguais: `["Chế độ hiện tại: Fast", "Mô hình: Gemini 3.1 Pro (High)"]`; diferentes: `["Chế độ hiện tại: Planning", "Mô hình Nhanh: Claude Opus 4.6", "Mô hình Kế hoạch: Gemini 3.1 Pro"]` |
| `TestShouldSkipActivityLog` | `filters out duplicate mode/model, generic status, single-word logs, analyzed logs, keeps actual execution` | `"Fast"`, `"Gemini 3.1 Pro"`, `"Generating."`, `"Thinking..."`, `"create"`, `"ready"`, `"pull."`, `"Analyzed.gemini/..."` retornam `true`; `"Analyzing Name Preference"` retorna `false` |
| `TestSplitForEmbedDescription` | `splits long text at the limit while preserving content` | `1200 'a' + \n + 1200 'b' + \n + 1200 'c'` com limite 1000 $\rightarrow$ `len(parts) > 1`, todos `<= 1000` |
| `TestFitForSingleEmbedDescription` | `returns text as-is when within the limit` / `truncates the beginning to fit within the limit` | `"短いテキスト"` (limite 50) retorna intacto; texto de 401 chars com limite 120 começa com `"...(phần đầu được lược bớt)"` e tem tamanho `<= 120` |

### 6.5. `internal/formatter/metadata_test.go` (Origem: [`tests/utils/metadataExtractor.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/metadataExtractor.test.ts))

| Função de Teste Go | Caso Original (`it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestExtractMetadataFromFooter` | Todos os 5 casos (`TaskID and Dir`, `only TaskID`, `only Dir`, `no metadata`, `whitespace variations`) | `"TaskID: abc-123 \| Dir: /workspace/projectA"` $\rightarrow$ `TaskID: "abc-123", Directory: "/workspace/projectA"`; apenas um presente deixa o outro `""`; sem metadados retorna struct zerada; espaços extras são ignorados |

### 6.6. `internal/monitor/process_log_buffer_test.go` (Origem: [`tests/utils/processLogBuffer.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/utils/processLogBuffer.test.ts))

| Função de Teste Go | Caso Original (`it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestProcessLogBuffer_EmojiPrefixes` | `formats entries with emoji prefixes for readability` | `'Initiating Project Setup'` $\rightarrow$ `'🚀 Initiating Project Setup'`; `'Thought for 6s'` $\rightarrow$ `'🧠 Thought for 6s'`; `'jina-mcp-server / search_web'` $\rightarrow$ `'🛠️ jina-mcp-server / search_web'` |
| `TestProcessLogBuffer_PastTenseFileEmoji` | `uses 📄 emoji for past-tense file operations` | `'Analyzed package.json#L1-75'`, `'Read src/index.ts'`, `'Created new-file.ts'`, `'Built project successfully'` recebem prefixo `'📄 '` |
| `TestProcessLogBuffer_PresentTenseActivityEmoji` | `uses 🔍 emoji for present-tense activity operations` | `'Fetching data from API'`, `'Scanning directory for files'`, `'Building project'`, `'Creating test fixtures'` recebem prefixo `'🔍 '` |
| `TestProcessLogBuffer_DropsOldestFirst` | `drops oldest entries first when exceeding maxChars` | Com `MaxChars: 45, MaxEntries: 10`, após inserir `'Initiating Step A'`, `'Initiating Step B'` e `'Initiating Step C'`, o resultado não contém `'Step A'` e contém `'Step B'` e `'Step C'` |

### 6.7. `internal/monitor/progress_sender_test.go` (Origem: [`tests/services/progressSender.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/progressSender.test.ts) & [`tests/services/progressSender.bugfix.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/progressSender.bugfix.test.ts))

| Função de Teste Go | Caso Original (`describe` / `it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestProgressSender_ThrottleSendCalls` | `should throttle send calls` | 3 chamadas `Append` seguidas emitem 1 única chamada após `ThrottleMs` contendo `"chunk 1\nchunk 2\nchunk 3\n"` |
| `TestProgressSender_ForceEmit` | `should send immediately if forced` | `Append("chunk 1\n")` + `ForceEmit()` emite imediatamente 1 vez |
| `TestProgressSender_SplitByMaxLength` | `should split if max length is exceeded` | String de 100 chars com `MaxLength: 50` emite `ceil(len/50)` vezes preservando todo o conteúdo |
| `TestProgressSender_WrapInCodeBlockOptions` | `should use send function without code block wrapping` / `wraps in <pre> tags by default` | Com `WrapInCodeBlock: false`, envia `"line 1\n"`; por padrão (`true`), envia `"<pre>code output</pre>"` |
| `TestProgressSender_Dispose` | `dispose() > clears pending timer, clears buffer, safe multiple times, can append new data after dispose` | `Dispose()` impede emissão agendada, esvazia o buffer para `ForceEmit()`, aceita chamadas repetidas e permite novo ciclo de `Append("new")` $\rightarrow$ `"<pre>new</pre>"` |
| `TestProgressSender_EmitEdgeCases` | `emit edge cases > empty buffer, swallows send errors, single timer for multiple appends` | `ForceEmit()` com buffer vazio não chama `Send`; erro retornado por `Send` não causa panic; múltiplos `Append("a")`, `Append("b")`, `Append("c")` resultam em 1 única emissão `"<pre>abc</pre>"` |
| `TestProgressSender_DefaultOptions` | `default options > defaults throttleMs to 3000, maxLength to 4000, wrapInCodeBlock to true` | Verifica os 3 defaults quando omitidos em `ProgressSenderOptions` |
