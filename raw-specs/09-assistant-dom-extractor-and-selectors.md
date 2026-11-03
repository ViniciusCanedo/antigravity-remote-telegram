# 09 — Extrator Estruturado do DOM do Assistente e Diagnóstico de Seletores (`doctorSelector`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação técnica define de forma 100% autocontida e exaustiva o extrator estruturado de segmentos do DOM do Antigravity (`assistantDomExtractor.ts`) e a ferramenta de auditoria/diagnóstico de saúde dos seletores via CDP (`doctorSelector.ts`).

### 1.1. Mapeamento de Arquivos (TypeScript $\rightarrow$ Go)

| Arquivo TypeScript Origem | Linhas | Arquivo Go Destino | Pacote Go | Responsabilidade Principal |
| :--- | :---: | :--- | :---: | :--- |
| [`src/services/assistantDomExtractor.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/assistantDomExtractor.ts) | 444 | `internal/monitor/dom_extractor.go` | `monitor` | Geração do script IIFE `ExtractAssistantSegmentsPayloadScript()` avaliado via CDP `Runtime.evaluate` (4 passes de varredura DOM) e classificador Go-side `ClassifyAssistantSegments(payload)` que converte os segmentos em `FinalOutputText` (via `formatter.HtmlToTelegramHtml`), `ActivityLines`, `ThinkingLines`, `Feedback` e `Diagnostics`. |
| [`src/tools/doctorSelector.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/tools/doctorSelector.ts) | 95 | `internal/cli/doctor_selectors.go` | `cli` | Ferramenta CLI de auditoria de saúde dos 7 seletores principais de `RESPONSE_SELECTORS` contra uma aba ativa do Antigravity via CDP, exibindo relatório individual (`✅ Healthy`, `⚠️  Warning`, `❌ Broken`) e placar final (`Score: X/7 Healthy`). |

### 1.2. Mapeamento de Arquivos de Teste Origem $\rightarrow$ Destino Go

| Arquivo de Teste TypeScript Origem | Linhas | Arquivo de Teste Go Destino |
| :--- | :---: | :--- |
| [`tests/services/assistantDomExtractor.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/assistantDomExtractor.test.ts) | 392 | `internal/monitor/dom_extractor_test.go` |

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Extrator Estruturado do DOM (`internal/monitor/dom_extractor.go`)

```go
package monitor

// AssistantSegmentKind enumera os 6 tipos de segmento extraídos do DOM.
type AssistantSegmentKind string

const (
	SegmentKindAssistantBody   AssistantSegmentKind = "assistant-body"
	SegmentKindThinking        AssistantSegmentKind = "thinking"
	SegmentKindThinkingContent AssistantSegmentKind = "thinking-content"
	SegmentKindToolCall        AssistantSegmentKind = "tool-call"
	SegmentKindToolResult      AssistantSegmentKind = "tool-result"
	SegmentKindFeedback        AssistantSegmentKind = "feedback"
)

// AssistantDomSegment representa um segmento individual extraído do turno do assistente no DOM.
type AssistantDomSegment struct {
	Kind         AssistantSegmentKind `json:"kind"`
	Text         string               `json:"text"`
	Role         string               `json:"role"` // sempre "assistant"
	MessageIndex int                  `json:"messageIndex"`
	DomPath      string               `json:"domPath"`
}

// AssistantDomSegmentPayload representa o payload retornado pela IIFE de extração DOM via CDP.
type AssistantDomSegmentPayload struct {
	Source      string                `json:"source"` // deve ser "dom-structured"
	ExtractedAt int64                 `json:"extractedAt"`
	Segments    []AssistantDomSegment `json:"segments"`
}

// ClassifyDiagnostics contém metadados de diagnóstico sobre a classificação dos segmentos.
type ClassifyDiagnostics struct {
	Source         string         `json:"source"` // "dom-structured" | "legacy-fallback"
	SegmentCounts  map[string]int `json:"segmentCounts"`
	FallbackReason string         `json:"fallbackReason,omitempty"`
}

// ClassifyResult representa o resultado final de ClassifyAssistantSegments.
type ClassifyResult struct {
	FinalOutputText string              `json:"finalOutputText"`
	ActivityLines   []string            `json:"activityLines"`
	ThinkingLines   []string            `json:"thinkingLines"`
	Feedback        []string            `json:"feedback"`
	Diagnostics     ClassifyDiagnostics `json:"diagnostics"`
}

// ClassifyAssistantSegments classifica um payload (pode ser *AssistantDomSegmentPayload,
// AssistantDomSegmentPayload, map[string]any vindo de json.Unmarshal ou nil) em texto de saída
// formatado para Telegram HTML, linhas de atividade, linhas de raciocínio, feedback e diagnósticos.
// Se o payload for inválido, retorna source="legacy-fallback" e fallbackReason="invalid-payload".
func ClassifyAssistantSegments(payload any) ClassifyResult

// ExtractAssistantSegmentsPayloadScript retorna a string literal da IIFE JavaScript avaliada no
// contexto do navegador via CDP Runtime.evaluate.
func ExtractAssistantSegmentsPayloadScript() string

// isValidPayload valida e converte um valor arbitrário para *AssistantDomSegmentPayload.
func isValidPayload(value any) (*AssistantDomSegmentPayload, bool)
```

---

### 2.2. Ferramenta `doctorSelector` (`internal/cli/doctor_selectors.go`)

```go
package cli

import (
	"context"
)

// SelectorCheckEntry define um seletor nomeado a ser auditado pelo DoctorSelector.
type SelectorCheckEntry struct {
	Name       string
	Expression string
}

// SelectorEvalResult mapeia o envelope JSON retornado pela IIFE wrapper do DoctorSelector:
// { ok: true, data: result } ou { ok: false, error: e.message }.
type SelectorEvalResult struct {
	Ok    bool   `json:"ok"`
	Data  any    `json:"data"`
	Error string `json:"error,omitempty"`
}

// DoctorSelectorReport sumariza o resultado da execução do DoctorSelector.
type DoctorSelectorReport struct {
	HealthCount int
	TotalCount  int
}

// RunDoctorSelectors conecta ao Antigravity via CdpService nas portas CDP_PORTS,
// avalia os 7 seletores de RESPONSE_SELECTORS em ordem determinística e imprime o relatório de saúde.
// Retorna erro (ou chama os.Exit(1) no entrypoint CLI) caso a conexão inicial falhe.
func RunDoctorSelectors(ctx context.Context) (*DoctorSelectorReport, error)
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

### 3.1. Script JS/IIFE Literal de `extractAssistantSegmentsPayloadScript()` (`src/services/assistantDomExtractor.ts`)

O método `ExtractAssistantSegmentsPayloadScript()` em `internal/monitor/dom_extractor.go` **DEVE** retornar exatamente a string IIFE abaixo, sem qualquer omissão ou abreviação:

```javascript
(() => {
    var panel = document.querySelector('.antigravity-agent-side-panel');
    var scope = panel || document;

    // Same selectors as RESPONSE_TEXT — ordered by specificity
    var selectors = [
        '.rendered-markdown',
        '.leading-relaxed.select-text',
        '.flex.flex-col.gap-y-3',
        '[data-message-author-role="assistant"]',
        '[data-message-role="assistant"]',
        '[class*="assistant-message"]',
        '[class*="message-content"]',
        '[class*="markdown-body"]',
        '.prose',
    ];

    var looksLikeActivityLog = function(text) {
        var normalized = (text || '').trim().toLowerCase();
        if (!normalized) return false;
        // Strip leading emoji/non-letter chars for matching
        var stripped = normalized.replace(/^[^a-z]+/i, '');
        if (/^(?:analy[sz]ing|reading|writing|running|searching|planning|thinking|processing|loading|executing|testing|debugging|fetching|connecting|creating|updating|deleting|installing|building|compiling|deploying|checking|scanning|parsing|resolving|downloading|uploading|analyzed|read|wrote|ran|created|updated|deleted|fetched|built|compiled|installed|resolved|downloaded|connected)\b/i.test(stripped) && normalized.length <= 220) return true;
        // "Initiating..." removed — too broad; catches response body text like
        // "Initiating Task Execution I'm now ready to..." causing false activity entries.
        if (/^thought for\s/i.test(stripped) && normalized.length <= 500) return true;
        return false;
    };

    var looksLikeFeedbackFooter = function(text) {
        var normalized = (text || '').trim().toLowerCase().replace(/\s+/g, ' ');
        return normalized === 'good bad' || normalized === 'good' || normalized === 'bad';
    };

    var looksLikeToolOutput = function(text) {
        var first = (text || '').trim().split('\n')[0] || '';
        if (/^[a-z0-9._-]+\s*\/\s*[a-z0-9._-]+$/i.test(first)) return true;
        if (/^full output written to\b/i.test(first)) return true;
        if (/^output\.[a-z0-9._-]+(?:#l\d+(?:-\d+)?)?$/i.test(first)) return true;
        return false;
    };

    var looksLikeThinking = function(text) {
        var stripped = (text || '').trim().replace(/^[^a-zA-Z]+/, '').toLowerCase();
        if (!stripped) return false;
        return /^(?:thought for|thinking)\b/i.test(stripped);
    };

    var isInsideExcludedContainer = function(node) {
        if (node.closest('details')) return true;
        if (node.closest('[class*="feedback"], footer')) return true;
        if (node.closest('.notify-user-container')) return true;
        if (node.closest('[role="dialog"]')) return true;
        return false;
    };

    var segments = [];
    var seen = new Set();
    var bodyFound = false;

    // Pass 1: Find assistant body — last non-excluded content node (recency first)
    var combinedSelector = selectors.join(', ');
    var nodes = scope.querySelectorAll(combinedSelector);

    for (var i = nodes.length - 1; i >= 0; i--) {
        var node = nodes[i];
        if (!node || seen.has(node)) continue;
        seen.add(node);
        if (isInsideExcludedContainer(node)) continue;

        var text = (node.innerText || node.textContent || '').replace(/\r/g, '').trim();
        if (!text || text.length < 2) continue;
        if (looksLikeActivityLog(text)) continue;
        if (looksLikeFeedbackFooter(text)) continue;
        if (looksLikeToolOutput(text)) continue;

        // This is the assistant body — normalize code blocks then extract innerHTML
        // AG wraps code in <pre><div class="..."> with a header div for language label,
        // embedded <style> tags, and code-line divs — NOT standard <pre><code>.
        // We normalize to <pre><code> for htmlToTelegramHtml.
        var clone = node.cloneNode(true);
        var pres = clone.querySelectorAll('pre');
        for (var pi = 0; pi < pres.length; pi++) {
            var pre = pres[pi];
            // Extract language from header div (font-sans text-sm class)
            var langDiv = pre.querySelector('.font-sans.text-sm, [class*="text-sm"][class*="opacity"]');
            var lang = langDiv ? (langDiv.textContent || '').trim() : '';

            // Remove <style> tags before extracting text (AG injects code-block CSS)
            var styles = pre.querySelectorAll('style');
            for (var si = 0; si < styles.length; si++) {
                styles[si].parentNode.removeChild(styles[si]);
            }

            // Remove the header bar (language label + copy button)
            var headerBar = pre.querySelector('[class*="rounded-t"][class*="border-b"]');
            if (headerBar) headerBar.parentNode.removeChild(headerBar);

            // Extract code text: prefer code-line elements for reliable newlines
            var codeLines = pre.querySelectorAll('.code-line, [class*="code-line"]');
            var codeText;
            if (codeLines.length > 0) {
                var lineTexts = [];
                for (var cli = 0; cli < codeLines.length; cli++) {
                    lineTexts.push(codeLines[cli].textContent || '');
                }
                codeText = lineTexts.join('\n');
            } else {
                // Fallback: use innerText of cleaned pre
                codeText = (pre.innerText || '').trim();
                if (lang && codeText.startsWith(lang)) {
                    codeText = codeText.slice(lang.length).trim();
                }
            }
            // Remove trailing "Copy" or clipboard button text
            codeText = codeText.replace(/\nCopy$/i, '').replace(/\ncopy code$/i, '').trim();
            // Replace with simple <pre><code>
            var newPre = document.createElement('pre');
            var newCode = document.createElement('code');
            if (lang) newCode.setAttribute('class', 'language-' + lang);
            newCode.textContent = codeText;
            newPre.appendChild(newCode);
            pre.parentNode.replaceChild(newPre, pre);
        }
        // Also remove any top-level <style> tags from the clone
        var topStyles = clone.querySelectorAll('style');
        for (var tsi = 0; tsi < topStyles.length; tsi++) {
            topStyles[tsi].parentNode.removeChild(topStyles[tsi]);
        }
        var bodyHtml = clone.innerHTML;
        if (bodyHtml && bodyHtml.trim()) {
            segments.push({
                kind: 'assistant-body',
                text: bodyHtml,
                role: 'assistant',
                messageIndex: 0,
                domPath: 'multi-selector'
            });
            bodyFound = true;
            break; // Only take the last (most recent) output node
        }
    }

    // Pass 2: Extract thinking + tool segments from <details>
    var details = scope.querySelectorAll('details');
    for (var di = 0; di < details.length; di++) {
        var detail = details[di];
        var summary = detail.querySelector('summary');
        var isThinkingBlock = false;
        if (summary) {
            var summaryText = (summary.textContent || '').trim();
            // Strip leading emoji/non-letter chars for matching
            var summaryStripped = summaryText.replace(/^[^a-zA-Z]+/, '');
            isThinkingBlock = /^(?:thought for|thinking)\b/i.test(summaryStripped);
            if (summaryText) {
                segments.push({
                    kind: isThinkingBlock ? 'thinking' : 'tool-call',
                    text: summaryText,
                    role: 'assistant',
                    messageIndex: 0,
                    domPath: 'details:nth(' + di + ') summary'
                });
            }
        }
        // Force-open collapsed details to access hidden content
        var wasOpen = detail.open;
        if (!wasOpen) {
            detail.open = true;
        }
        // Extract child content inside <details>
        var children = detail.children;
        for (var ci = 0; ci < children.length; ci++) {
            var child = children[ci];
            if (child.tagName === 'SUMMARY' || child.tagName === 'STYLE') continue;
            var childText = (child.innerText || child.textContent || '').trim();
            if (!childText || childText.length < 5) continue;
            // Skip junk: pure numbers, single words under 8 chars
            if (/^\d+$/.test(childText)) continue;
            if (isThinkingBlock) {
                // Thinking body content — preserve more text for display
                segments.push({
                    kind: 'thinking-content',
                    text: childText.slice(0, 2000),
                    role: 'assistant',
                    messageIndex: 0,
                    domPath: 'details:nth(' + di + ') child:nth(' + ci + ')'
                });
            } else {
                var childKind = looksLikeToolOutput(childText) ? 'tool-result' : 'tool-call';
                segments.push({
                    kind: childKind,
                    text: childText.slice(0, 300),
                    role: 'assistant',
                    messageIndex: 0,
                    domPath: 'details:nth(' + di + ') child:nth(' + ci + ')'
                });
            }
        }
        // Restore original collapsed state
        if (!wasOpen) {
            detail.open = false;
        }
    }

    // Pass 2.5: Broad activity scan - leaf-ish elements with activity-like text
    // Uses querySelectorAll('*') to find activity nodes that don't match content
    // selectors (e.g. "Analyzed package.json#L1-75" inside <div class="flex flex-row">).
    // Excludes nodes inside response body containers to avoid false positives.
    // Uses ancestor dedup to prevent capturing both parent and child activity nodes.
    var actSeen = new Set();
    var allEls = scope.querySelectorAll('*');
    for (var ai = 0; ai < allEls.length; ai++) {
        var el = allEls[ai];
        if (el.children.length > 3) continue;       // leaf-ish only
        if (seen.has(el)) continue;                  // already captured in Pass 1/2
        if (isInsideExcludedContainer(el)) continue;
        // Skip nodes inside response body containers (prevents capturing inline words)
        if (el.closest('.leading-relaxed, .rendered-markdown, .prose, .animate-markdown, [data-message-role], [data-message-author-role]')) continue;
        var aText = (el.innerText || el.textContent || '').replace(/\r/g, '').trim();
        if (!aText || aText.length < 5 || aText.length > 300) continue;
        // Skip junk: pure numbers, single short words
        if (/^\d+$/.test(aText)) continue;
        // Skip model selector / dropdown UI text (contains multiple model names)
        var aLower = aText.toLowerCase();
        if ((aLower.includes('gemini') || aLower.includes('claude') || aLower.includes('gpt')) && aLower.includes('send')) continue;
        var isThinking = looksLikeThinking(aText);
        if (isThinking || looksLikeActivityLog(aText) || looksLikeToolOutput(aText)) {
            // When we find a short activity verb (e.g. "Analyzed" alone), the file
            // reference may be in a sibling element.  Walk up to the nearest ancestor
            // (max 3 levels) whose innerText contains the verb AND more context
            // (e.g. "Analyzed package.json#L1-75").
            var bestText = aText;
            if (!isThinking && looksLikeActivityLog(aText) && aText.length < 60) {
                var ancestor = el.parentElement;
                for (var up = 0; up < 3 && ancestor && ancestor !== scope; up++) {
                    // Don't cross into excluded containers
                    if (ancestor.closest && ancestor.closest('.leading-relaxed, .rendered-markdown, .prose, .animate-markdown, [data-message-role], [data-message-author-role]')) break;
                    var ancestorText = (ancestor.innerText || ancestor.textContent || '').replace(/\r/g, '').trim();
                    // Accept if it still looks like activity, is longer, and not too long
                    if (ancestorText && ancestorText.length > bestText.length && ancestorText.length <= 300 && looksLikeActivityLog(ancestorText)) {
                        bestText = ancestorText;
                        // Mark ancestor as seen to prevent child duplication
                        actSeen.add(ancestor);
                    }
                    ancestor = ancestor.parentElement;
                }
            }
            // Ancestor dedup: skip if a parent was already captured as activity
            var dup = false;
            var p = el.parentElement;
            while (p && p !== scope) {
                if (actSeen.has(p)) { dup = true; break; }
                p = p.parentElement;
            }
            if (dup) continue;
            actSeen.add(el);
            var aKind = isThinking ? 'thinking' : (looksLikeToolOutput(bestText) ? 'tool-result' : 'tool-call');
            segments.push({
                kind: aKind,
                text: bestText.slice(0, isThinking ? 2000 : 300),
                role: 'assistant',
                messageIndex: 0,
                domPath: 'activity-scan:nth(' + ai + ')'
            });
        }
    }

    // Pass 3: Extract feedback buttons
    var feedbackBtns = scope.querySelectorAll('[class*="feedback"] button, footer button');
    for (var fi = 0; fi < feedbackBtns.length; fi++) {
        var btnText = (feedbackBtns[fi].textContent || '').trim();
        if (btnText === 'Good' || btnText === 'Bad') {
            segments.push({
                kind: 'feedback',
                text: btnText,
                role: 'assistant',
                messageIndex: 0,
                domPath: 'feedback button.' + btnText.toLowerCase()
            });
        }
    }

    if (!bodyFound && segments.length === 0) return null;

    return {
        source: 'dom-structured',
        extractedAt: Date.now(),
        segments: segments
    };
})()
```

---

### 3.2. Constantes, Lista de Seletores e Wrapper JS de `doctorSelector.ts` (`internal/cli/doctor_selectors.go`)

Ordem exata dos 7 seletores auditados por `doctorSelector.ts`:

```go
var doctorSelectorChecks = []SelectorCheckEntry{
	{Name: "Chat Message (Text)", Expression: monitor.ResponseSelectors.ResponseText},
	{Name: "Stop Button (State)", Expression: monitor.ResponseSelectors.StopButton},
	{Name: "Planning Active (Dialog)", Expression: monitor.ResponseSelectors.PlanningActive},
	{Name: "Process Logs (Activity)", Expression: monitor.ResponseSelectors.ProcessLogs},
	{Name: "Combined Poll (Logic)", Expression: monitor.ResponseSelectors.CombinedPoll},
	{Name: "Quota Error (Detection)", Expression: monitor.ResponseSelectors.QuotaError},
	{Name: "DOM Observer (Event-driven)", Expression: monitor.ResponseSelectors.DomObserver},
}
```

Expressão wrapper avaliada via `Runtime.evaluate` (`returnByValue: true`) para cada seletor:

```javascript
(function() { 
    try { 
        const result = ${expression}; 
        return { ok: true, data: result }; 
    } catch(e) { 
        return { ok: false, error: e.message }; 
    } 
})()
```

Strings de log/console literais de `doctorSelector.ts`:
- Início: `🩺 Start Doctor Selector - Auditing Antigravity UI...`
- Erro de conexão: `❌ Could not connect to any Antigravity tab: ${e.message}`
- Falha inesperada de conexão: `❌ Connection failed unexpectedly.`
- Antes de cada seletor: `Checking: [${name}]...`
- Resultado saudável (`value.ok && data !== null && data !== undefined`):
  - `  ✅ Healthy: Found and extracted data.`
  - Se `typeof data === 'object'` (em Go: mapa ou slice/struct não-nulo): `    Data: ${JSON.stringify(data)}`
- Resultado nulo (`value.ok && (data === null || data === undefined)`):
  - `  ⚠️  Warning: Found but returned null/undefined.`
- Resultado quebrado (`!value.ok`):
  - `  ❌ Broken: ${value?.error || 'Unknown evaluation error'}`
- Erro na chamada CDP:
  - `  ❌ CDP Call Error: ${err.message}`
- Relatório final:
  - `\n--- Final Selector Health Report ---`
  - `scoreColor = healthCount === totalCount ? '✨' : '⚠️'`
  - `${scoreColor} Score: ${healthCount}/${totalCount} Healthy`
  - Se `healthCount === totalCount`: `✅ All core selectors are healthy. The bot is in top shape!`
  - Senão, se `healthCount > 0`: `🚨 Some selectors failed. Compatibility might be compromised.`
  - Senão (`healthCount === 0`): `💀 All selectors failed! The UI has likely changed significantly.`
- Erro fatal no bloco externo: `💥 Doctor collapsed:`

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Algoritmo de Extração no Browser (`ExtractAssistantSegmentsPayloadScript`)

O script IIFE executa 4 etapas sequenciais dentro do escopo `.antigravity-agent-side-panel` (ou `document` como fallback):

1. **Funções Predicadas Internas**:
   - `looksLikeActivityLog(text)`: remove prefixos não-alfabéticos (`^[^a-z]+/i`) e verifica se começa com um dos 40 verbos de atividade no presente/passado (ex: `analyzing`, `reading`, `writing`, `fetching`, `building`, `analyzed`, `read`, `wrote`, etc.) com `normalized.length <= 220`, ou se começa com `thought for\s` com `normalized.length <= 500`. Note que `"Initiating..."` é deliberadamente excluído neste extrator para evitar capturar texto de resposta real.
   - `looksLikeFeedbackFooter(text)`: verifica se o texto normalizado é `'good bad'`, `'good'` ou `'bad'`.
   - `looksLikeToolOutput(text)`: verifica a primeira linha contra formato MCP (`server / tool`), `'full output written to'` ou `'output.<ext>#l<num>'`.
   - `looksLikeThinking(text)`: remove prefixos não-alfabéticos e verifica `/^(?:thought for|thinking)\b/i`.
   - `isInsideExcludedContainer(node)`: retorna `true` se `node.closest('details')`, `node.closest('[class*="feedback"], footer')`, `node.closest('.notify-user-container')` ou `node.closest('[role="dialog"]')` existir.

2. **Pass 1 — Localização do Corpo Principal da Resposta (`assistant-body`)**:
   - Consulta todos os elementos que casam com os 9 seletores (`selectors.join(', ')`) e itera **de trás para frente** (`nodes.length - 1` até `0`) para obter o nó mais recente.
   - Ignora nós já vistos em `seen`, nós dentro de containers excluídos (`isInsideExcludedContainer`), textos com menos de 2 caracteres, ou textos que pareçam `looksLikeActivityLog`, `looksLikeFeedbackFooter` ou `looksLikeToolOutput`.
   - Ao encontrar o primeiro nó válido:
     - Clona a subárvore (`node.cloneNode(true)`).
     - Normaliza todos os blocos `<pre>` do clone:
       - Extrai a linguagem de `.font-sans.text-sm, [class*="text-sm"][class*="opacity"]`.
       - Remove todas as tags `<style>` internas ao `<pre>`.
       - Remove a barra de cabeçalho `[class*="rounded-t"][class*="border-b"]`.
       - Se existirem elementos `.code-line, [class*="code-line"]`, junta os `textContent` deles com `\n`; caso contrário, usa `pre.innerText.trim()` (removendo o prefixo `lang` se presente).
       - Remove sufixos `\nCopy` ou `\ncopy code` (case-insensitive) do final do código.
       - Substitui o `<pre>` original por um `<pre><code class="language-X">...</code></pre>` limpo.
     - Remove quaisquer tags `<style>` de nível superior restantes no clone.
     - Se `clone.innerHTML.trim()` não for vazio, adiciona um segmento `kind: 'assistant-body'`, `role: 'assistant'`, `messageIndex: 0`, `domPath: 'multi-selector'`, define `bodyFound = true` e encerra o Pass 1 (`break`).

3. **Pass 2 — Extração de `<details>` (`thinking`, `thinking-content`, `tool-call`, `tool-result`)**:
   - Itera sobre todos os elementos `<details>` na ordem do DOM (`0` a `details.length - 1`).
   - Se houver `<summary>`, lê `summaryText`. Verifica se `summaryStripped` casa com `/^(?:thought for|thinking)\b/i` (`isThinkingBlock`).
   - Adiciona o sumário como segmento `kind: isThinkingBlock ? 'thinking' : 'tool-call'`, `domPath: 'details:nth(' + di + ') summary'`.
   - Salva `wasOpen = detail.open`. Se estiver fechado (`!wasOpen`), força `detail.open = true` temporariamente para ler o texto dos filhos ocultos.
   - Para cada elemento filho direto (`detail.children`) que não seja `SUMMARY` nem `STYLE`:
     - Ignora textos com menos de 5 caracteres ou compostos apenas por dígitos (`/^\d+$/`).
     - Se `isThinkingBlock == true`, adiciona segmento `kind: 'thinking-content'` com `childText.slice(0, 2000)`.
     - Se `isThinkingBlock == false`, classifica como `'tool-result'` (se `looksLikeToolOutput(childText)`) ou `'tool-call'`, truncando em `childText.slice(0, 300)`.
   - Restaura `detail.open = false` se `!wasOpen`.

4. **Pass 2.5 — Varredura Ampla de Atividades (`activity-scan`)**:
   - Percorre `scope.querySelectorAll('*')` buscando elementos "quase-folha" (`el.children.length <= 3`) que não estejam em `seen`, não estejam em `isInsideExcludedContainer(el)` e não estejam dentro de containers de corpo de mensagem (`.leading-relaxed, .rendered-markdown, .prose, .animate-markdown, [data-message-role], [data-message-author-role]`).
   - Filtra textos fora da faixa `[5, 300]` caracteres, números puros (`/^\d+$/`) e textos de dropdown de seleção de modelos (que contenham `'gemini'`/`'claude'`/`'gpt'` **e** `'send'`).
   - Se o texto casar com `looksLikeThinking`, `looksLikeActivityLog` ou `looksLikeToolOutput`:
     - **Expansão de contexto em ancestrais**: se não for `thinking`, casar com `looksLikeActivityLog(aText)` e for curto (`aText.length < 60`), sobe até 3 níveis de `parentElement` (sem cruzar para containers de corpo de resposta) procurando um `ancestorText` mais longo (`<= 300` chars) que ainda case com `looksLikeActivityLog`. Se encontrar, atualiza `bestText = ancestorText` e marca o ancestral em `actSeen`.
     - **Deduplicação por ancestral**: verifica se algum ancestral até `scope` já está em `actSeen`. Se estiver, ignora o elemento (`continue`).
     - Marca `el` em `actSeen` e adiciona o segmento com `kind: isThinking ? 'thinking' : (looksLikeToolOutput(bestText) ? 'tool-result' : 'tool-call')`, truncando em `2000` (se thinking) ou `300` chars, com `domPath: 'activity-scan:nth(' + ai + ')'`.

5. **Pass 3 — Botões de Feedback (`feedback`)**:
   - Percorre `scope.querySelectorAll('[class*="feedback"] button, footer button')`.
   - Se `btnText === 'Good'` ou `btnText === 'Bad'`, adiciona segmento `kind: 'feedback'`, `domPath: 'feedback button.' + btnText.toLowerCase()`.

6. **Retorno da IIFE**:
   - Se `!bodyFound && segments.length === 0`, retorna `null`.
   - Caso contrário, retorna `{ source: 'dom-structured', extractedAt: Date.now(), segments: segments }`.

---

### 4.2. Classificador Go-Side (`ClassifyAssistantSegments`)

1. **Validação (`isValidPayload`)**:
   - Se `payload == nil` ou não puder ser interpretado como um objeto contendo `source == "dom-structured"` e um slice/array `segments`, retorna imediatamente:
     ```go
     ClassifyResult{
         FinalOutputText: "",
         ActivityLines:   []string{},
         ThinkingLines:   []string{},
         Feedback:        []string{},
         Diagnostics: ClassifyDiagnostics{
             Source:         "legacy-fallback",
             SegmentCounts:  map[string]int{},
             FallbackReason: "invalid-payload",
         },
     }
     ```
2. **Iteração e Contagem**:
   - Inicializa slices vazios (não-nil, para serialização JSON `[]`) `bodyTexts`, `activityLines`, `thinkingLines`, `feedbackTexts` e mapa `segmentCounts`.
   - Para cada segmento `seg` em `segments`:
     - Incrementa `segmentCounts[string(seg.Kind)]++`.
     - `assistant-body`: se `strings.TrimSpace(seg.Text) != ""`, adiciona `seg.Text` (sem trim externo no push, preservando estrutura para join) a `bodyTexts`.
     - `thinking` ou `thinking-content`: se `strings.TrimSpace(seg.Text) != ""`, adiciona `strings.TrimSpace(seg.Text)` a `thinkingLines`.
     - `tool-call` ou `tool-result`: se `strings.TrimSpace(seg.Text) != ""`, adiciona `strings.TrimSpace(seg.Text)` a `activityLines`.
     - `feedback`: se `strings.TrimSpace(seg.Text) != ""`, adiciona `strings.TrimSpace(seg.Text)` a `feedbackTexts`.
3. **Conversão do Corpo Final**:
   - Une `rawBody := strings.Join(bodyTexts, "\n\n")`.
   - Aplica `finalOutputText := formatter.HtmlToTelegramHtml(rawBody)`.
4. **Retorno**:
   - Retorna `ClassifyResult` com `Diagnostics.Source = "dom-structured"` e `Diagnostics.SegmentCounts = segmentCounts`.

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **`ExtractAssistantSegmentsPayloadScript` e `ClassifyAssistantSegments`**:
   - São funções puras e *stateless*, invocáveis concorrentemente sem necessidade de locks.
   - `ClassifyAssistantSegments` aceita tanto structs tipadas (`AssistantDomSegmentPayload` / `*AssistantDomSegmentPayload`) quanto `map[string]any` / `json.RawMessage` provenientes da desserialização genérica de respostas CDP (`result.value`).
2. **`RunDoctorSelectors`**:
   - Executa sequencialmente a conexão `cdpService.Connect(ctx)` e o loop pelos 7 seletores.
   - Garante `defer cdpService.Disconnect()` (equivalente ao bloco `finally { await service.disconnect(); }` em TypeScript) antes de retornar.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

Todos os testes abaixo mapeiam 1:1 os casos de [`tests/services/assistantDomExtractor.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/assistantDomExtractor.test.ts) para `internal/monitor/dom_extractor_test.go`.

> **Nota sobre testes que avaliam a IIFE contra um DOM simulado (Pass 2.5 e Tasks 1-3)**:
> No TypeScript, o Jest usa `@jest-environment jsdom` para avaliar o script IIFE gerado por `extractAssistantSegmentsPayloadScript()`. Em Go, os testes unitários do pacote `internal/monitor/dom_extractor_test.go` devem testar:
> 1. Toda a lógica de `ClassifyAssistantSegments` e `HtmlToTelegramHtml` diretamente em Go;
> 2. A estrutura e o conteúdo exato do script retornado por `ExtractAssistantSegmentsPayloadScript()`;
> 3. A execução real da IIFE `ExtractAssistantSegmentsPayloadScript()` em um motor JS/DOM leve em teste (ou verificando a equivalência de extração + classificação dos payloads gerados para cada cenário DOM dos testes Pass 2.5 e Tasks 1-3).

| Função de Teste Go | Caso Original (`describe` / `it`) | Entrada e Asserção Exata |
| :--- | :--- | :--- |
| `TestClassifyAssistantSegments_SeparatesBodyActivityFeedback` | `separates body, activity, and feedback based on DOM structure` | Payload com 6 segmentos (`assistant-body`, `thinking`, `tool-call`, `tool-result`, 2x `feedback`): `FinalOutputText == "最終回答: 1ドル=154.8円です。"`, `ActivityLines == ["jina-mcp-server / search_web", "Full output written to output.txt#L1-10"]`, `ThinkingLines == ["Analyzing current exchange data..."]`, `Feedback == ["Good", "Bad"]`, `Diagnostics.Source == "dom-structured"`, `SegmentCounts["assistant-body"] == 1`, `SegmentCounts["feedback"] == 2` |
| `TestClassifyAssistantSegments_ConcatenatesMultipleBodySegments` | `concatenates body text when split across multiple segments` | 2 segmentos `assistant-body` (`"前半です。"` e `"後半です。"`) + 1 `tool-call` (`"mcp.search"`) $\rightarrow$ `FinalOutputText == "前半です。\n\n後半です。"`, `ActivityLines == ["mcp.search"]`, `Feedback == []` |
| `TestClassifyAssistantSegments_InvalidPayloadFallback` | `treats invalid payload as legacy-fallback` | `ClassifyAssistantSegments(nil)` $\rightarrow$ `FinalOutputText == ""`, slices vazios, `Diagnostics.Source == "legacy-fallback"`, `Diagnostics.FallbackReason == "invalid-payload"` |
| `TestClassifyAssistantSegments_RoutesThinkingAndThinkingContent` | `routes thinking and thinking-content segments to thinkingLines` | Segmentos `thinking` (`"Thought for 12 seconds"`) e `thinking-content` (`"Let me analyze the user request carefully..."`) vão ambos para `ThinkingLines` na ordem; `SegmentCounts["thinking-content"] == 1` |
| `TestExtractAssistantSegmentsPayloadScript_Structure` | `returns the extraction script as a DOM structure extraction function` | Verifica que o script retornado contém `"data-message-role"`, `"assistant-body"`, `"details"` e `"feedback"` |
| `TestExtractAssistantSegments_Pass25_LeafActivity` | `Pass 2.5 > captures activity text from leaf-ish elements outside content selectors` | Nó `<div class="flex flex-row">Analyzed package.json#L1-75</div>` dentro de `.antigravity-agent-side-panel` resulta em `ActivityLines` contendo `"Analyzed package.json#L1-75"` |
| `TestExtractAssistantSegments_Pass25_McpToolOutput` | `Pass 2.5 > captures MCP tool output from non-selector-matched nodes` | Nó `<div class="flex flex-row">jina-mcp-server / search_web</div>` resulta em `ActivityLines` contendo `"jina-mcp-server / search_web"` |
| `TestExtractAssistantSegments_Pass25_NewActivityVerbs` | `Pass 2.5 > captures new activity verbs (fetching, creating, building, etc.)` | Nós com `'Fetching data from API'`, `'Creating new file'`, `'Building project'`, `'Connected to server'` aparecem todos em `ActivityLines` |
| `TestExtractAssistantSegments_Pass25_SkipsRoleDialog` | `Pass 2.5 > skips mode description inside role="dialog" container` | Elementos com texto `"Planning"` dentro de `<div role="dialog">` são ignorados (`ActivityLines == []`) |
| `TestExtractAssistantSegments_Pass25_SkipsContainerOver3Children` | `Pass 2.5 > skips container elements with more than 3 children` | Container `<div class="flex flex-col">` com 5 `<span>` filhos não tem seu texto concatenado capturado como uma única entrada em `ActivityLines` |
| `TestExtractAssistantSegments_PreservesListAndFileReferences` | `Task 1 & Task 2 & Task 3 > preserves list structure and file references in extraction script output` | Corpo com `<p>改行付き\nの段落</p>`, `<ol><li>最初の項目</li><li>次の項目</li></ol>`, `<ul><li>箇条書き1</li></ul>` e `<div title="src/bot/index.ts">:54</div>` produz `FinalOutputText` contendo `"1. 最初の項目"`, `"2. 次の項目"`, `"- 箇条書き1"`, `"改行付き\nの段落"` e `"src/bot/index.ts:54"` |
