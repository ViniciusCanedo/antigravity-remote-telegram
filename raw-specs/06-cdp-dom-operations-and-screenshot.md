# Especificação Técnica 06 — Operações DOM no Painel Cascade, Transferência de Imagens, Sincronização de UI e Captura de Screenshot (`internal/cdp`)

## 1. Escopo, Mapeamento e Rastreabilidade

Esta especificação define de forma 100% autocontida e exaustiva todas as operações de manipulação de DOM via CDP (`Runtime.evaluate`, `Input.*`, `DOM.*`, `Page.captureScreenshot`) executadas sobre o painel Cascade da janela do Antigravity: foco e limpeza do campo de chat, injeção de prompts de texto, anexação de arquivos de imagem locais, extração de imagens geradas pelo assistente, leitura e troca de modo de execução (`fast` / `plan`), listagem, leitura e troca de modelo LLM, e captura de screenshots da janela.

### 1.1. Mapeamento de Arquivos Originais (TypeScript) $\rightarrow$ Destino (Go)

| Arquivo Original (TypeScript) | Linhas | Arquivo Destino (Go) | Responsabilidade Principal |
| :--- | :---: | :--- | :--- |
| [`src/services/cdpService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/cdpService.ts) (Linhas 25–59, 945–1712) | 768 | `internal/cdp/operations.go` | Métodos de `Client` para foco no editor (`focusChatInput`), limpeza via teclas (`clearInputField`), envio via Enter (`pressEnterToSend`), upload de imagens (`attachImageFiles`), injeção de mensagem (`InjectMessage`, `InjectMessageWithImageFiles`), extração de imagens da resposta (`ExtractLatestResponseImages`) e sincronização de modo/modelo (`GetCurrentMode`, `SetUIMode`, `GetUIModels`, `GetCurrentModel`, `SetUIModel`). |
| [`src/services/screenshotService.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/src/services/screenshotService.ts) | 128 | `internal/cdp/screenshot.go` | Serviço `ScreenshotService` para captura de tela via comando CDP `Page.captureScreenshot`, suportando formatos `png`, `jpeg` e `webp`, qualidade JPEG, região de recorte (`clip`) e `captureBeyondViewport`, retornando `[]byte` (`Capture`) ou string Base64 (`GetBase64`). |

### 1.2. Mapeamento de Arquivos de Teste Originais $\rightarrow$ Suítes de Teste em Go

| Arquivo de Teste Original (Jest) | Linhas | Arquivo de Teste Destino (Go) | Escopo de Verificação |
| :--- | :---: | :--- | :--- |
| [`tests/services/cdpService.injection.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.injection.test.ts) | 221 | `internal/cdp/operations_injection_test.go` | Injeção de texto no contexto `cascade-panel`, fallback entre múltiplos contextos, falha quando nenhum contexto possui editor visível, sequência exata de comandos CDP (`Runtime.evaluate` $\rightarrow$ 4 eventos `Input.dispatchKeyEvent` de limpeza $\rightarrow$ `Input.insertText` $\rightarrow$ 2 eventos `Input.dispatchKeyEvent` de Enter) e erro quando desconectado. |
| [`tests/services/cdpService.imageTransfer.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.imageTransfer.test.ts) | 206 | `internal/cdp/operations_images_test.go` | Fluxo completo de `InjectMessageWithImageFiles` (`DOM.enable`, token `data-agclaw-upload-token`, `DOM.getDocument`, `DOM.querySelector`, `DOM.setFileInputFiles`), garantia de não disparar eventos `input`/`change` duplicados após `DOM.setFileInputFiles` e extração de imagens geradas com `ExtractLatestResponseImages`. |
| [`tests/services/cdpService.uiSync.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.uiSync.test.ts) | 255 | `internal/cdp/operations_uisync_test.go` | Operação de `SetUIMode` (mapeamento `'fast'` $\rightarrow$ `'Fast'`, `'plan'` $\rightarrow$ `'Planning'`, seletores baseados em `[role="dialog"]` e `.font-medium`, erro quando desconectado, tratamento gracioso de falha DOM ou erro CDP) e `SetUIModel` (seleção de modelo, uso do `primaryContextId`, tratamento de erros). |
| [`tests/services/screenshotService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/screenshotService.test.ts) | 137 | `internal/cdp/screenshot_test.go` | Captura de screenshot retornando buffer decodificado de Base64, parâmetros padrão (`format: 'png'`), suporte a `jpeg` + `quality: 80`, suporte a `clip`, retorno de `success: false` quando `data` é vazio ou ocorre erro CDP, e retorno direto da string Base64 em `GetBase64()`. |

### 1.3. Documentação Arquitetural Relacionada
- [`docs/03-protocolo-cdp-e-seletores-dom.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/03-protocolo-cdp-e-seletores-dom.md)
- [`docs/05-detectores-e-servicos-background.md`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/antigravity-telegram-remote-golang/docs/05-detectores-e-servicos-background.md)

---

## 2. Contratos, Structs, Interfaces e Assinaturas em Go

### 2.1. Pacote `internal/cdp` — `operations.go`

```go
package cdp

// InjectResult espelha InjectResult (src/services/cdpService.ts:25-30).
type InjectResult struct {
	OK        bool   `json:"ok"`
	Method    string `json:"method,omitempty"`
	ContextID *int   `json:"contextId,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ExtractedResponseImage espelha ExtractedResponseImage (src/services/cdpService.ts:32-37).
type ExtractedResponseImage struct {
	Name       string `json:"name"`
	MimeType   string `json:"mimeType"`
	Base64Data string `json:"base64Data,omitempty"`
	URL        string `json:"url,omitempty"`
}

// UISyncResult espelha UiSyncResult (src/services/cdpService.ts:40-47).
type UISyncResult struct {
	OK    bool   `json:"ok"`
	Mode  string `json:"mode,omitempty"`
	Model string `json:"model,omitempty"`
	Error string `json:"error,omitempty"`
}

// Selectors armazena as constantes de seletores DOM (src/services/cdpService.ts:50-59).
var Selectors = struct {
	ChatInput              string
	SubmitButtonContainer  string
	SubmitButtonSVGClasses []string
	ContextURLKeyword      string
}{
	ChatInput:              `div[role="textbox"]:not(.xterm-helper-textarea)`,
	SubmitButtonContainer:  "button",
	SubmitButtonSVGClasses: []string{"lucide-arrow-right", "lucide-arrow-up", "lucide-send"},
	ContextURLKeyword:      "cascade-panel",
}

// Métodos públicos de operações DOM em Client:
func (c *Client) InjectMessage(text string) (InjectResult, error)
func (c *Client) InjectMessageWithImageFiles(text string, imageFilePaths []string) (InjectResult, error)
func (c *Client) ExtractLatestResponseImages(maxImages int) []ExtractedResponseImage
func (c *Client) GetCurrentMode() *string
func (c *Client) SetUIMode(modeName string) (UISyncResult, error)
func (c *Client) GetUIModels() ([]string, error)
func (c *Client) GetCurrentModel() *string
func (c *Client) SetUIModel(modelName string) (UISyncResult, error)

// Métodos auxiliares privados de Client:
func (c *Client) focusChatInput() (ok bool, contextID *int, errMsg string)
func (c *Client) clearInputField() error
func (c *Client) pressEnterToSend() error
func (c *Client) attachImageFiles(filePaths []string, contextID *int) (ok bool, errMsg string)
```

### 2.2. Pacote `internal/cdp` — `screenshot.go`

```go
package cdp

import "encoding/json"

// CDPCaller define a interface mínima consumida por ScreenshotService (permitindo mock direto em testes).
type CDPCaller interface {
	Call(method string, params any) (json.RawMessage, error)
}

// ClipRegion representa o retângulo de recorte da captura (src/services/screenshotService.ts:11-17).
type ClipRegion struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Scale  float64 `json:"scale"`
}

// CaptureOptions espelha CaptureOptions (src/services/screenshotService.ts:5-20).
type CaptureOptions struct {
	// Format: "png" (default), "jpeg", ou "webp"
	Format string
	// Quality: 0-100 (apenas para JPEG; omitido se nil)
	Quality *int
	// Clip: região de recorte (omitido se nil)
	Clip *ClipRegion
	// CaptureBeyondViewport: captura além do viewport (omitido se nil)
	CaptureBeyondViewport *bool
}

// CaptureResult espelha CaptureResult (src/services/screenshotService.ts:23-30).
type CaptureResult struct {
	Success bool
	Buffer  []byte
	Error   string
}

// ScreenshotServiceOptions espelha ScreenshotServiceOptions (src/services/screenshotService.ts:32-35).
type ScreenshotServiceOptions struct {
	CDPService CDPCaller
}

// ScreenshotService espelha ScreenshotService (src/services/screenshotService.ts:43-128).
type ScreenshotService struct {
	cdpService CDPCaller
}

func NewScreenshotService(opts ScreenshotServiceOptions) *ScreenshotService
func (s *ScreenshotService) Capture(opts ...CaptureOptions) CaptureResult
func (s *ScreenshotService) GetBase64(opts ...CaptureOptions) *string
```

---

## 3. Constantes, Seletores, Expressões Regulares e Scripts JS/DOM Literais

### 3.1. Seletores, Atributos e Constantes de Temporização

| Constante / Parâmetro | Valor Literal | Local de Uso |
| :--- | :--- | :--- |
| `SELECTORS.CHAT_INPUT` | `'div[role="textbox"]:not(.xterm-helper-textarea)'` | `focusChatInput` |
| `SELECTORS.CONTEXT_URL_KEYWORD` | `'cascade-panel'` | `waitForCascadePanelReady`, `getPrimaryContextId` |
| `clearInputField` settle delay | `50ms` | Pausa após `Meta+A` / `Ctrl+A` + `Backspace`. |
| `clearInputField` modifier | `4` (macOS `darwin` — `Meta`) ou `2` (Windows/Linux — `Ctrl`) | `Input.dispatchKeyEvent` para tecla `'a'` (`code: 'KeyA'`, `windowsVirtualKeyCode: 65`). |
| `injectMessage` post-insert delay | `200ms` | Pausa entre `Input.insertText` e `pressEnterToSend()`. |
| `attachImageFiles` post-cleanup delay | `250ms` | Pausa final após remover `data-agclaw-upload-token`. |
| `attachImageFiles` trigger click delay | `150ms` | Pausa após clicar em botão de upload (`attach`, `upload`, `image`, `file`, `paperclip`, `plus`). |
| `extractLatestResponseImages` default/bounds | `default = 4`, `min = 1`, `max = 8` (`Math.max(1, Math.min(8, Math.floor(maxImages)))`) | Limite de imagens extraídas da última resposta. |
| `extractLatestResponseImages` filtro de tamanho | `w < 96 \|\| h < 96 \|\| (w * h) < 12000` | Descarta ícones, avatares, emojis e miniaturas. |
| `setUiMode` mapeamento `uiNameMap` | `{ fast: "Fast", plan: "Planning" }` | Mapeamento entre código interno do modo e texto exibido na UI. |
| `setUiMode` delays de UI | `500ms` (após abrir dropdown), `500ms` (após clicar na opção), `200ms` (após enviar `Escape` em caso de falha) | `setUiMode` |
| `setUiModel` delay pós-clique | `500ms` | Pausa após `targetItem.click()` antes de verificar a classe `'bg-gray-500/20'`. |

---

### 3.2. Scripts JS/DOM Literais Avaliados via `Runtime.evaluate` (Cópia 100% Integral)

#### 3.2.1. `focusScript` em `focusChatInput()` (`src/services/cdpService.ts:949-956`)
```javascript
(() => {
            const editors = Array.from(document.querySelectorAll('div[role="textbox"]:not(.xterm-helper-textarea)'));
            const visible = editors.filter(el => el.offsetParent !== null);
            const editor = visible[visible.length - 1];
            if (!editor) return { ok: false, error: 'No editor found' };
            editor.focus();
            return { ok: true };
        })()
```
*(Avaliado em cada contexto de `this.contexts` com `{ expression: focusScript, returnByValue: true, contextId: ctx.id }`)*

#### 3.2.2. `locateInputScript` em `attachImageFiles()` (`src/services/cdpService.ts:1045-1100`)
```javascript
(async () => {
            const wait = (ms) => new Promise(resolve => setTimeout(resolve, ms));
            const visible = (el) => {
                if (!el) return false;
                if (el.offsetParent !== null) return true;
                const style = window.getComputedStyle(el);
                if (!style) return false;
                if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return false;
                const rect = typeof el.getBoundingClientRect === 'function' ? el.getBoundingClientRect() : null;
                return !!rect && rect.width > 0 && rect.height > 0;
            };
            const normalize = (v) => (v || '').toLowerCase();
            const hasImageAccept = (input) => {
                const accept = normalize(input.getAttribute('accept'));
                return !accept || accept.includes('image') || accept.includes('*/*');
            };
            const findInput = () => {
                const inputs = Array.from(document.querySelectorAll('input[type="file"]'));
                const visibleInput = inputs.find(i => visible(i) && hasImageAccept(i));
                if (visibleInput) return visibleInput;
                return inputs.find(hasImageAccept) || null;
            };

            let input = findInput();
            if (!input) {
                const triggerKeywords = ['attach', 'upload', 'image', 'file', 'paperclip', 'plus'];
                const triggers = Array.from(document.querySelectorAll('button, [role="button"]'))
                    .filter(visible)
                    .filter((el) => {
                        const text = normalize(el.textContent);
                        const aria = normalize(el.getAttribute('aria-label'));
                        const title = normalize(el.getAttribute('title'));
                        const cls = normalize(el.getAttribute('class'));
                        const all = [text, aria, title, cls].join(' ');
                        return triggerKeywords.some(k => all.includes(k));
                    })
                    .slice(-8);

                for (const trigger of triggers) {
                    if (typeof trigger.click === 'function') {
                        trigger.click();
                        await wait(150);
                        input = findInput();
                        if (input) break;
                    }
                }
            }

            if (!input) {
                return { ok: false, error: 'Image upload input not found' };
            }

            const token = 'agclaw-upload-' + Math.random().toString(36).slice(2, 10);
            input.setAttribute('data-agclaw-upload-token', token);
            return { ok: true, token };
        })()
```
*(Avaliado com `{ expression: locateInputScript, returnByValue: true, awaitPromise: true, contextId }`)*

#### 3.2.3. `notifyScript` em `attachImageFiles()` (`src/services/cdpService.ts:1139-1144`)
Onde `selector` é `'input[data-agclaw-upload-token="' + token + '"]'` e interpolado via `JSON.stringify(selector)`:
```javascript
(() => {
            const input = document.querySelector("input[data-agclaw-upload-token=\"<TOKEN>\"]");
            if (!input) return { ok: false, error: 'Image input not found' };
            input.removeAttribute('data-agclaw-upload-token');
            return { ok: true };
        })()
```
> [!IMPORTANT]
> Este script **NÃO DEVE** conter `dispatchEvent` (`new Event('input')` ou `new Event('change')`), pois `DOM.setFileInputFiles` já dispara o evento `change` nativamente no Chromium/Electron. A ausência da substring `'dispatchEvent'` é explicitamente verificada no teste `does not dispatch duplicate input/change events after DOM.setFileInputFiles` em `tests/services/cdpService.imageTransfer.test.ts`.

#### 3.2.4. Script de `extractLatestResponseImages(maxImages)` (`src/services/cdpService.ts:1237-1383`)
Onde `${safeMaxImages}` é um inteiro no intervalo `[1, 8]`:
```javascript
(async () => {
            const maxImages = ${safeMaxImages};
            const panel = document.querySelector('.antigravity-agent-side-panel');
            const scope = panel || document;

            const candidateSelectors = [
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

            const responseNodes = [];
            const seenNodes = new Set();
            for (const selector of candidateSelectors) {
                const nodes = scope.querySelectorAll(selector);
                for (const node of nodes) {
                    if (!node || seenNodes.has(node)) continue;
                    seenNodes.add(node);
                    responseNodes.push(node);
                }
            }

            // Skip image extraction when no response nodes found (prevent UI icon false positives)
            if (responseNodes.length === 0) return [];

            const normalize = (value) => (value || '').toLowerCase();
            const isLikelyUiImage = (img) => {
                if (!img) return true;
                const src = normalize(img.currentSrc || img.src || img.getAttribute('src') || '');
                const alt = normalize(img.getAttribute('alt') || '');
                const title = normalize(img.getAttribute('title') || '');
                const cls = normalize(img.getAttribute('class') || '');
                const blob = [src, alt, title, cls].join(' ');

                if (blob.includes('icon') || blob.includes('avatar') || blob.includes('emoji')) return true;
                if (blob.includes('thumb') || blob.includes('good') || blob.includes('bad')) return true;
                if (src.startsWith('data:image/svg+xml')) return true;
                if (img.closest('button, [role="button"], nav, header, footer, [class*="toolbar"], [class*="reaction"]')) return true;

                const rect = typeof img.getBoundingClientRect === 'function' ? img.getBoundingClientRect() : null;
                const w = Number(img.naturalWidth || img.width || rect?.width || 0);
                const h = Number(img.naturalHeight || img.height || rect?.height || 0);
                if (w < 96 || h < 96) return true;
                if ((w * h) < 12000) return true;

                return false;
            };

            const dedup = new Set();
            const images = [];
            for (let i = responseNodes.length - 1; i >= 0; i--) {
                const node = responseNodes[i];
                const nodeImages = Array.from(node.querySelectorAll('img'));
                for (const img of nodeImages) {
                    if (isLikelyUiImage(img)) continue;
                    const key = (img.currentSrc || img.src || img.getAttribute('src') || '') + '|' + (img.getAttribute('alt') || '');
                    if (!key || dedup.has(key)) continue;
                    dedup.add(key);
                    images.push(img);
                }
                if (images.length >= maxImages) break;
            }

            if (images.length === 0) return [];
            const picked = images.slice(-maxImages);

            const normalizeFileName = (value, idx) => {
                const raw = (value || '').trim();
                const safe = raw.replace(/[^a-zA-Z0-9._-]/g, '-').replace(/-+/g, '-').replace(/^-+|-+$/g, '');
                return safe || ('generated-image-' + (idx + 1));
            };

            const guessMimeType = (src) => {
                if (!src) return 'image/png';
                if (src.startsWith('data:')) {
                    const match = src.match(/^data:([^;]+);/);
                    return (match && match[1]) || 'image/png';
                }
                const lower = src.toLowerCase();
                if (lower.includes('.jpg') || lower.includes('.jpeg')) return 'image/jpeg';
                if (lower.includes('.webp')) return 'image/webp';
                if (lower.includes('.gif')) return 'image/gif';
                return 'image/png';
            };

            const blobToBase64 = (blob) => new Promise((resolve, reject) => {
                const reader = new FileReader();
                reader.onload = () => {
                    const value = typeof reader.result === 'string' ? reader.result : '';
                    const commaIndex = value.indexOf(',');
                    resolve(commaIndex >= 0 ? value.slice(commaIndex + 1) : value);
                };
                reader.onerror = () => reject(reader.error || new Error('read failed'));
                reader.readAsDataURL(blob);
            });

            const result = [];
            for (let i = 0; i < picked.length; i++) {
                const img = picked[i];
                const src = img.currentSrc || img.src || img.getAttribute('src') || '';
                if (!src) continue;

                const baseName = normalizeFileName(img.getAttribute('alt') || img.getAttribute('title'), i);
                const mimeType = guessMimeType(src);
                const extensionMap = { 'image/png': 'png', 'image/jpeg': 'jpg', 'image/webp': 'webp', 'image/gif': 'gif' };
                const ext = extensionMap[mimeType] || 'png';
                const name = baseName.includes('.') ? baseName : (baseName + '.' + ext);

                if (src.startsWith('data:')) {
                    const commaIndex = src.indexOf(',');
                    if (commaIndex > 0) {
                        result.push({
                            name,
                            mimeType,
                            base64Data: src.slice(commaIndex + 1),
                        });
                    }
                    continue;
                }

                try {
                    const response = await fetch(src);
                    if (!response.ok) throw new Error('fetch failed');
                    const blob = await response.blob();
                    const base64Data = await blobToBase64(blob);
                    result.push({
                        name,
                        mimeType: blob.type || mimeType,
                        base64Data,
                    });
                } catch {
                    result.push({
                        name,
                        mimeType,
                        url: src,
                    });
                }
            }

            return result;
        })()
```
*(Avaliado com `{ expression, returnByValue: true, awaitPromise: true, contextId }` onde `contextId = this.getPrimaryContextId()`)*

#### 3.2.5. Script de `getCurrentMode()` (`src/services/cdpService.ts:1424-1439`)
```javascript
(() => { const uiNameMap = { fast: "Fast", plan: "Planning" }; const knownModes = Object.values(uiNameMap).map(n => n.toLowerCase()); const reverseMap = {}; Object.entries(uiNameMap).forEach(([k, v]) => { reverseMap[v.toLowerCase()] = k; }); const allBtns = Array.from(document.querySelectorAll("button")); const visibleBtns = allBtns.filter(b => b.offsetParent !== null); const modeToggleBtn = visibleBtns.find(b => {   const text = (b.textContent || "").trim().toLowerCase();   const hasChevron = b.querySelector("svg[class*=\"chevron\"]");   return knownModes.some(m => text === m) && hasChevron; }); if (!modeToggleBtn) return null; const currentModeText = (modeToggleBtn.textContent || "").trim().toLowerCase(); return reverseMap[currentModeText] || null;})()
```
*(Avaliado com `{ expression, returnByValue: true, awaitPromise: false, contextId }`)*

#### 3.2.6. Script de `setUiMode(modeName)` (`src/services/cdpService.ts:1474-1547`)
Construído exatamente por concatenação de strings (onde `safeMode = JSON.stringify(modeName)` e `uiNameMap = '{"fast":"Fast","plan":"Planning"}'`):
```javascript
(async () => { const targetMode = "plan"; const targetModeLower = targetMode.toLowerCase(); const uiNameMap = {"fast":"Fast","plan":"Planning"}; const targetUiName = uiNameMap[targetModeLower] || targetMode; const targetUiNameLower = targetUiName.toLowerCase(); const allBtns = Array.from(document.querySelectorAll("button")); const visibleBtns = allBtns.filter(b => b.offsetParent !== null); const knownModes = Object.values(uiNameMap).map(n => n.toLowerCase()); const modeToggleBtn = visibleBtns.find(b => {   const text = (b.textContent || "").trim().toLowerCase();   const hasChevron = b.querySelector("svg[class*=\"chevron\"]");   return knownModes.some(m => text === m) && hasChevron; }); if (!modeToggleBtn) {   return { ok: false, error: "Mode toggle button not found" }; } const currentModeText = (modeToggleBtn.textContent || "").trim().toLowerCase(); if (currentModeText === targetUiNameLower) {   return { ok: true, mode: targetUiName, alreadySelected: true }; } modeToggleBtn.click(); await new Promise(r => setTimeout(r, 500)); const dialogs = Array.from(document.querySelectorAll("[role=\"dialog\"]")); const visibleDialog = dialogs.find(d => {   const style = window.getComputedStyle(d);   return style.visibility !== "hidden" && style.display !== "none"; }); let modeOption = null; if (visibleDialog) {   const fontMediumEls = Array.from(visibleDialog.querySelectorAll(".font-medium"));   const matchEl = fontMediumEls.find(el => {     const text = (el.textContent || "").trim().toLowerCase();     return text === targetUiNameLower;   });   if (matchEl) {     modeOption = matchEl.closest("div.cursor-pointer") || matchEl.parentElement;   } } if (!modeOption) {   const fallbackEls = Array.from(document.querySelectorAll(     "div[class*=\"cursor-pointer\"]"   )).filter(el => el.offsetParent !== null);   modeOption = fallbackEls.find(el => {     if (el === modeToggleBtn) return false;     const fm = el.querySelector(".font-medium");     if (fm) {       const text = (fm.textContent || "").trim().toLowerCase();       return text === targetUiNameLower;     }     return false;   }); } if (modeOption) {   modeOption.click();   await new Promise(r => setTimeout(r, 500));   const updBtn = Array.from(document.querySelectorAll("button"))     .filter(b => b.offsetParent !== null)     .find(b => b.querySelector("svg[class*=\"chevron\"]") && knownModes.some(m => (b.textContent || "").trim().toLowerCase() === m));   const newMode = updBtn ? (updBtn.textContent || "").trim() : "unknown";   return { ok: true, mode: newMode }; } document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true })); await new Promise(r => setTimeout(r, 200)); return { ok: false, error: "Mode option " + targetUiName + " not found in dropdown" };})()
```
> [!NOTE]
> Observe que no código TypeScript original (`src/services/cdpService.ts:1501`), a string literal usa `"[role=\\"dialog\\"]"` dentro de aspas simples (`' const dialogs = Array.from(document.querySelectorAll("[role=\\"dialog\\"]"));'`), o que produz `\"` literal na string JS resultante. O teste `uses dialog-based selectors in the expression` em `tests/services/cdpService.uiSync.test.ts:131` verifica `expect(callArgs.expression).toContain('role=\\"dialog\\"')` ou os seletores `.font-medium` e `cursor-pointer`.

#### 3.2.7. Script de `getUiModels()` (`src/services/cdpService.ts:1577-1582`)
```javascript
(async () => {
            return Array.from(document.querySelectorAll('div.cursor-pointer'))
                .map(e => ({text: (e.textContent || '').trim().replace(/New$/, ''), class: e.className}))
                .filter(e => e.class.includes('px-2 py-1 flex items-center justify-between') || e.text.includes('Gemini') || e.text.includes('GPT') || e.text.includes('Claude'))
                .map(e => e.text);
        })()
```
*(Avaliado com `{ expression, returnByValue: true, awaitPromise: true, contextId }`)*

#### 3.2.8. Script de `getCurrentModel()` (`src/services/cdpService.ts:1613-1617`)
```javascript
(() => {
            return Array.from(document.querySelectorAll('div.cursor-pointer'))
                .find(e => e.className.includes('px-2 py-1 flex items-center justify-between') && e.className.includes('bg-gray-500/20'))
                ?.textContent?.trim().replace(/New$/, '') || null;
        })()
```
*(Avaliado com `{ expression, returnByValue: true, awaitPromise: true, contextId: contextId || undefined }`)*

#### 3.2.9. Script de `setUiModel(modelName)` (`src/services/cdpService.ts:1646-1691`)
Onde `${safeModel}` é `JSON.stringify(modelName)`:
```javascript
(async () => {
            const targetModel = ${safeModel};
            
            // Get all items in the model list
            const modelItems = Array.from(document.querySelectorAll('div.cursor-pointer'))
                .filter(e => e.className.includes('px-2 py-1 flex items-center justify-between'));
            
            if (modelItems.length === 0) {
                return { ok: false, error: 'Model list not found. The dropdown may not be open.' };
            }
            
            // Match target model by name (compare after removing New suffix)
            const targetItem = modelItems.find(el => {
                const text = (el.textContent || '').trim().replace(/New$/, '').trim();
                return text === targetModel || text.toLowerCase() === targetModel.toLowerCase();
            });
            
            if (!targetItem) {
                const available = modelItems.map(el => (el.textContent || '').trim().replace(/New$/, '').trim()).join(', ');
                return { ok: false, error: 'Model "' + targetModel + '" not found. Available: ' + available };
            }
            
            // Check if already selected
            if (targetItem.className.includes('bg-gray-500/20') && !targetItem.className.includes('hover:bg-gray-500/20')) {
                return { ok: true, model: targetModel, alreadySelected: true };
            }
            
            // Click to select model
            targetItem.click();
            await new Promise(r => setTimeout(r, 500));
            
            // Verify selection was applied
            const updatedItems = Array.from(document.querySelectorAll('div.cursor-pointer'))
                .filter(e => e.className.includes('px-2 py-1 flex items-center justify-between'));
            const selectedItem = updatedItems.find(el => {
                const text = (el.textContent || '').trim().replace(/New$/, '').trim();
                return text === targetModel || text.toLowerCase() === targetModel.toLowerCase();
            });
            
            if (selectedItem && selectedItem.className.includes('bg-gray-500/20') && !selectedItem.className.includes('hover:bg-gray-500/20')) {
                return { ok: true, model: targetModel, verified: true };
            }
            
            // Click succeeded but verification failed
            return { ok: true, model: targetModel, verified: false };
        })()
```
*(Avaliado com `{ expression, returnByValue: true, awaitPromise: true, contextId }`)*

---

## 4. Regras de Negócio e Algoritmos Passo a Passo

### 4.1. Foco, Limpeza e Envio no Editor de Chat (`focusChatInput`, `clearInputField`, `pressEnterToSend`)

1. **`focusChatInput()`**:
   - Itera sequencialmente sobre `this.contexts` na ordem em que foram registrados.
   - Para cada `ctx`, chama `Runtime.evaluate` com `focusScript` (Seção 3.2.1), `returnByValue: true` e `contextId: ctx.id`.
   - O script busca todos os elementos `div[role="textbox"]:not(.xterm-helper-textarea)` visíveis (`el.offsetParent !== null`), seleciona o **último** (`visible[visible.length - 1]`) e chama `editor.focus()`.
   - Se `res?.result?.value?.ok === true`, interrompe a busca e retorna `{ ok: true, contextId: ctx.id }`.
   - Se todos os contextos falharem ou retornarem `ok: false`, retorna `{ ok: false, error: 'Chat input field not found' }`.

2. **`clearInputField()`**:
   - Determina o modificador de teclado: `4` (Meta) se `runtime.GOOS == "darwin"`, ou `2` (Ctrl) nos demais sistemas operacionais.
   - Envia **exatamente 4 chamadas** sequenciais a `Input.dispatchKeyEvent`:
     1. `{ type: 'keyDown', key: 'a', code: 'KeyA', modifiers: mod, windowsVirtualKeyCode: 65, nativeVirtualKeyCode: 65 }`
     2. `{ type: 'keyUp', key: 'a', code: 'KeyA', modifiers: mod, windowsVirtualKeyCode: 65, nativeVirtualKeyCode: 65 }`
     3. `{ type: 'keyDown', key: 'Backspace', code: 'Backspace', windowsVirtualKeyCode: 8, nativeVirtualKeyCode: 8 }`
     4. `{ type: 'keyUp', key: 'Backspace', code: 'Backspace', windowsVirtualKeyCode: 8, nativeVirtualKeyCode: 8 }`
   - Aguarda **50ms** para estabilização do DOM.

3. **`pressEnterToSend()`**:
   - Envia **exatamente 2 chamadas** sequenciais a `Input.dispatchKeyEvent`:
     1. `{ type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13 }`
     2. `{ type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13 }`

### 4.2. Injeção de Prompt de Texto (`injectMessage`, linhas 1171–1198)
1. **Pré-condição**: se `!this.isConnectedFlag || !this.ws`, lança erro/retorna `error`: `"Not connected to CDP. Call connect() first."`.
2. Registra log de debug de auditoria: `"[CDP] Injecting message to LLM (<len> chars)"` e, se `0 < len < 500`, `"[CDP] Message preview: <primeiros 100 chars>"`.
3. Chama `focusResult = await this.focusChatInput()`. Se `!focusResult.ok`, retorna `{ ok: false, error: focusResult.error || 'Chat input field not found' }`.
4. Chama `await this.clearInputField()`.
5. Chama `await this.call('Input.insertText', { text })`.
6. Aguarda **200ms**.
7. Chama `await this.pressEnterToSend()`.
8. Retorna `{ ok: true, method: 'enter', contextId: focusResult.contextId }`.

### 4.3. Anexação de Imagens e Injeção Combinada (`attachImageFiles` e `injectMessageWithImageFiles`, linhas 1040–1155 e 1203–1226)
1. **`attachImageFiles(filePaths, contextId)`**:
   - Se `filePaths.length === 0`, retorna `{ ok: true }` imediatamente.
   - Chama `await this.call('DOM.enable', {})`.
   - Avalia `locateInputScript` (Seção 3.2.2) via `Runtime.evaluate` (`returnByValue: true`, `awaitPromise: true`, e `contextId` se definido):
     - O script procura um `<input type="file">` que aceite imagens (`!accept || accept.includes('image') || accept.includes('*/*')`), priorizando um visível.
     - Se não encontrar, procura até os últimos 8 botões visíveis cujos atributos (`textContent`, `aria-label`, `title`, `class`) contenham qualquer palavra-chave em `['attach', 'upload', 'image', 'file', 'paperclip', 'plus']`, clica em cada um com intervalo de `150ms` e verifica novamente se o `<input type="file">` surgiu no DOM.
     - Quando encontra o `<input type="file">`, gera `token = 'agclaw-upload-' + Math.random().toString(36).slice(2, 10)`, aplica `input.setAttribute('data-agclaw-upload-token', token)` e retorna `{ ok: true, token }`.
   - Se `!locateValue?.ok || !locateValue?.token`, retorna `{ ok: false, error: locateValue?.error || 'Failed to locate file input' }`.
   - Chama `documentResult = await this.call('DOM.getDocument', { depth: 1, pierce: true })`.
   - Extrai `rootNodeId = documentResult?.root?.nodeId`. Se ausente/zero, retorna `{ ok: false, error: 'Failed to get DOM root' }`.
   - Monta `selector = 'input[data-agclaw-upload-token="' + token + '"]'` e chama `nodeResult = await this.call('DOM.querySelector', { nodeId: rootNodeId, selector })`.
   - Extrai `nodeId = nodeResult?.nodeId`. Se ausente/zero, retorna `{ ok: false, error: 'Failed to get upload input node' }`.
   - Chama `await this.call('DOM.setFileInputFiles', { nodeId, files: filePaths })`.
   - Avalia `notifyScript` (Seção 3.2.3) para remover o atributo `data-agclaw-upload-token` do elemento `<input>`.
   - Aguarda **250ms** e retorna `{ ok: true }`.
2. **`injectMessageWithImageFiles(text, imageFilePaths)`**:
   - Se `!this.isConnectedFlag || !this.ws`, lança erro `"Not connected to CDP. Call connect() first."`.
   - Chama `focusResult = await this.focusChatInput()`. Se `!focusResult.ok`, retorna `{ ok: false, error: focusResult.error || 'Chat input field not found' }`.
   - Chama `await this.clearInputField()`.
   - Chama `attachResult = await this.attachImageFiles(imageFilePaths, focusResult.contextId)`. Se `!attachResult.ok`, retorna `{ ok: false, error: attachResult.error || 'Failed to attach images' }`.
   - Chama `await this.call('Input.insertText', { text })`, aguarda **200ms** e chama `await this.pressEnterToSend()`.
   - Retorna `{ ok: true, method: 'enter', contextId: focusResult.contextId }`.

### 4.4. Extração de Imagens Geradas na Resposta (`extractLatestResponseImages`, linhas 1231–1412)
1. Se `!this.isConnectedFlag || !this.ws`, retorna `[]` (não lança exceção).
2. Limita `safeMaxImages = Math.max(1, Math.min(8, Math.floor(maxImages)))` (default `4`).
3. Obtém `contextId = this.getPrimaryContextId()` e avalia o script da Seção 3.2.4 com `returnByValue: true, awaitPromise: true`:
   - Coleta nós de resposta usando os 9 seletores candidatos dentro de `.antigravity-agent-side-panel` (ou `document`). Se `responseNodes.length === 0`, retorna `[]`.
   - Percorre `responseNodes` de trás para frente (`i = responseNodes.length - 1` até `0`), filtrando `<img>` que **não** sejam `isLikelyUiImage(img)` (exclui palavras `icon`, `avatar`, `emoji`, `thumb`, `good`, `bad`, SVGs `data:image/svg+xml`, imagens dentro de `button, [role="button"], nav, header, footer, [class*="toolbar"], [class*="reaction"]`, e imagens com `w < 96 || h < 96 || (w * h) < 12000`).
   - Deduplica por `src + '|' + alt` e interrompe ao atingir `maxImages`.
   - Para cada imagem selecionada:
     - Normaliza o nome de arquivo a partir de `alt` ou `title` (ou `'generated-image-' + (idx + 1)`), adicionando extensão (`.png`, `.jpg`, `.webp`, `.gif`).
     - Se `src.startsWith('data:')`: fatia após a primeira vírgula e retorna `{ name, mimeType, base64Data }`.
     - Caso contrário: executa `fetch(src)` $\rightarrow$ `response.blob()` $\rightarrow$ `FileReader.readAsDataURL(blob)` para extrair `base64Data`. Se o `fetch` falhar, retorna `{ name, mimeType, url: src }`.
4. Valida e mapeia o array retornado garantindo que `name` e `mimeType` sejam strings válidas. Se ocorrer qualquer erro CDP, captura e retorna `[]`.

### 4.5. Sincronização de Modo (`getCurrentMode` e `setUiMode`, linhas 1420–1567)
1. **`getCurrentMode()`**:
   - Se desconectado, retorna `null`.
   - Avalia o script da Seção 3.2.5 no `primaryContextId`: procura um `<button>` visível (`offsetParent !== null`) que contenha um `svg[class*="chevron"]` e cujo texto (`.trim().toLowerCase()`) seja `"fast"` ou `"planning"`, mapeando de volta para `"fast"` ou `"plan"`.
   - Em caso de erro ou não encontrado, retorna `null`.
2. **`setUiMode(modeName)`**:
   - Se `!this.isConnectedFlag || !this.ws`, **lança erro** `"Not connected to CDP. Call connect() first."`.
   - Avalia o script da Seção 3.2.6 no `primaryContextId` com `awaitPromise: true`:
     - Mapeia `'fast'` $\rightarrow$ `'Fast'` e `'plan'` $\rightarrow$ `'Planning'`.
     - Localiza o botão `modeToggleBtn`. Se não encontrar, retorna `{ ok: false, error: "Mode toggle button not found" }`.
     - Se o texto atual já for igual ao modo desejado, retorna `{ ok: true, mode: targetUiName, alreadySelected: true }`.
     - Clica em `modeToggleBtn.click()`, aguarda `500ms`.
     - Procura dentro de um `[role="dialog"]` visível um elemento `.font-medium` cujo texto seja igual a `targetUiNameLower` e sobe para `matchEl.closest("div.cursor-pointer") || matchEl.parentElement` (com fallback para `div[class*="cursor-pointer"]` visíveis no documento).
     - Se encontrar `modeOption`, clica nele, aguarda `500ms`, relê o texto do botão e retorna `{ ok: true, mode: newMode }`.
     - Se não encontrar, despacha `KeyboardEvent("keydown", { key: "Escape", bubbles: true })`, aguarda `200ms` e retorna `{ ok: false, error: "Mode option " + targetUiName + " not found in dropdown" }`.
   - Se `call('Runtime.evaluate', ...)` lançar erro CDP, captura e retorna `{ ok: false, error: error.message }` (não propaga exceção quando conectado).

### 4.6. Sincronização de Modelos LLM (`getUiModels`, `getCurrentModel`, `setUiModel`, linhas 1572–1711)
1. **`getUiModels()`**:
   - Se desconectado, **lança erro** `"Not connected to CDP."`.
   - Avalia o script da Seção 3.2.7 no `primaryContextId`: coleta todos os `div.cursor-pointer`, remove sufixo `/New$/` do `textContent.trim()`, filtra itens cuja classe contenha `'px-2 py-1 flex items-center justify-between'` ou cujo texto contenha `'Gemini'`, `'GPT'` ou `'Claude'`, e retorna a lista deduplicada (`Array.from(new Set(value))`).
   - Em caso de erro no `Runtime.evaluate`, loga erro e retorna `[]`.
2. **`getCurrentModel()`**:
   - Se desconectado, retorna `null`.
   - Avalia o script da Seção 3.2.8 no `primaryContextId`: procura `div.cursor-pointer` cuja classe contenha simultaneamente `'px-2 py-1 flex items-center justify-between'` e `'bg-gray-500/20'`, retornando seu texto sem o sufixo `/New$/` (ou `null`).
3. **`setUiModel(modelName)`**:
   - Se desconectado, **lança erro** `"Not connected to CDP. Call connect() first."`.
   - Avalia o script da Seção 3.2.9 no `primaryContextId`:
     - Filtra `div.cursor-pointer` com `'px-2 py-1 flex items-center justify-between'`. Se vazio, retorna `{ ok: false, error: 'Model list not found. The dropdown may not be open.' }`.
     - Procura item cujo texto (sem `/New$/`) seja igual a `targetModel` (exato ou case-insensitive). Se não achar, retorna `{ ok: false, error: 'Model "' + targetModel + '" not found. Available: ' + available }`.
     - Se já tiver `'bg-gray-500/20'` (e não `'hover:bg-gray-500/20'`), retorna `{ ok: true, model: targetModel, alreadySelected: true }`.
     - Clica em `targetItem.click()`, aguarda `500ms` e retorna `{ ok: true, model: targetModel }`.
   - Se `call()` lançar erro CDP, captura e retorna `{ ok: false, error: error.message }`.

### 4.7. Captura de Screenshot (`ScreenshotService`, `src/services/screenshotService.ts:43-128`)
1. **`capture(options: CaptureOptions = {})`**:
   - Monta `params`:
     - `format`: `options.format ?? 'png'`.
     - `quality`: incluído somente se `options.quality !== undefined`.
     - `clip`: incluído somente se `options.clip` estiver definido.
     - `captureBeyondViewport`: incluído somente se `options.captureBeyondViewport !== undefined`.
   - Chama `result = await this.cdpService.call('Page.captureScreenshot', params)`.
   - Extrai `base64Data = result?.data ?? ''`.
   - Se `base64Data === ''`: retorna `{ success: false, error: 'Screenshot data was empty.' }`.
   - Decodifica `base64Data` de Base64 para bytes (`Buffer.from(base64Data, 'base64')`) e retorna `{ success: true, buffer }`.
   - Se `this.cdpService.call` lançar erro: loga o erro e retorna `{ success: false, error: err.message }`.
2. **`getBase64(options: CaptureOptions = {})`**:
   - Monta `params` com `format` (default `'png'`), `quality` (se definido) e `clip` (se definido).
   - Chama `result = await this.cdpService.call('Page.captureScreenshot', params)` e retorna `result?.data ?? null` (ou `null` em caso de exceção).

---

## 5. Modelo de Concorrência, Sincronização e Ciclo de Vida

1. **Serialização de Operações de Input/DOM por Workspace**:
   - `InjectMessage` e `InjectMessageWithImageFiles` alteram o foco do DOM da janela (`editor.focus()`), despacham teclas globais na página (`Meta+A`, `Backspace`, `Enter`) e interagem com `<input type="file">`. Na arquitetura geral do bot, o `PromptDispatcher` garante que apenas um prompt por workspace seja enviado por vez; ainda assim, os métodos de `Client` são thread-safe no transporte JSON-RPC (`Call`).
2. **Distinção Estrita entre Retorno de Erro (Exceção) vs. Struct `{ OK: false }` / `nil`**:
   - Métodos que exigem conexão prévia e lançam exceção no TypeScript quando desconectados (`InjectMessage`, `InjectMessageWithImageFiles`, `SetUIMode`, `GetUIModels`, `SetUIModel`) devem retornar `error` não-nulo em Go quando `!c.IsConnected()`.
   - Já falhas de avaliação DOM ou erros retornados pelo `Call('Runtime.evaluate', ...)` **enquanto conectado** em `SetUIMode` e `SetUIModel` são capturados e retornados dentro de `UISyncResult{OK: false, Error: err.Error()}` com `err == nil`, exatamente como verificado em `tests/services/cdpService.uiSync.test.ts`.
   - Métodos de leitura passiva (`ExtractLatestResponseImages`, `GetCurrentMode`, `GetCurrentModel`, `ScreenshotService.Capture`, `ScreenshotService.GetBase64`) nunca propagam pânico/erro não tratado: retornam slice vazio, `nil` ou `CaptureResult{Success: false, Error: ...}`.

---

## 6. Matriz Completa de Testes Unitários (`*_test.go`) e Critérios de Aceite

### 6.1. `internal/cdp/operations_injection_test.go` (Mapeado de [`tests/services/cdpService.injection.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.injection.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `CdpService - Message Injection (Step 5)` $\rightarrow$ `successfully injects a message in the cascade-panel context` | `TestClient_InjectMessage_SucceedsInCascadePanelContext(t *testing.T)` | Com 3 contextos registrados (`id: 1` top, `id: 2` cascade-panel, `id: 3` Extension), quando `contextId == 2` responde `{ ok: true, method: 'focus' }`, `InjectMessage("テストメッセージ")` retorna `OK: true`, `ContextID: 2`, `Method: "enter"`. |
| `CdpService - Message Injection (Step 5)` $\rightarrow$ `falls back to another context when cascade-panel fails` | `TestClient_InjectMessage_FallsBackToAnotherContext(t *testing.T)` | Quando `contextId == 2` falha (`{ ok: false, error: 'No editor found' }`) e `contextId == 3` sucede, `InjectMessage("フォールバックテスト")` retorna `OK: true` e `ContextID: 3`. |
| `CdpService - Message Injection (Step 5)` $\rightarrow$ `returns ok:false when all contexts fail` | `TestClient_InjectMessage_ReturnsFalseWhenAllContextsFail(t *testing.T)` | Quando todos os contextos retornam `{ ok: false, error: 'No editor found' }`, `InjectMessage` retorna `OK: false` e `Error != ""`. |
| `CdpService - Message Injection (Step 5)` $\rightarrow$ `calls Runtime.evaluate with the correct parameters during injectMessage` | `TestClient_InjectMessage_DispatchesExactCDPSequence(t *testing.T)` | Ao injetar `'注入テキスト<script>alert("xss")</script>'`, verifica: (1) `Runtime.evaluate` contém `'editor.focus()'` e `returnByValue: true`; (2) 1 chamada a `Input.insertText` com o texto exato; (3) 6 chamadas a `Input.dispatchKeyEvent`: `[0]` `keyDown` `'a'`, `[1]` `keyUp` `'a'`, `[2]` `keyDown` `'Backspace'`, `[3]` `keyUp` `'Backspace'`, `[4]` `keyDown` `'Enter'`, `[5]` `keyUp` `'Enter'`. |
| `CdpService - Message Injection (Step 5)` $\rightarrow$ `throws an error when injectMessage is called while not connected` | `TestClient_InjectMessage_ErrorWhenDisconnected(t *testing.T)` | Chamar `InjectMessage("test")` sem `Connect()` retorna erro não-nulo (`"Not connected to CDP. Call connect() first."`). |

### 6.2. `internal/cdp/operations_images_test.go` (Mapeado de [`tests/services/cdpService.imageTransfer.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.imageTransfer.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `CdpService - Image Transfer` $\rightarrow$ `sends a message with attached image files` | `TestClient_InjectMessageWithImageFiles_AttachesAndSends(t *testing.T)` | Chama `InjectMessageWithImageFiles("画像付きプロンプト", []string{"/tmp/agclaw-a.png", "/tmp/agclaw-b.jpg"})`, retorna `OK: true, ContextID: 2` e envia comando `DOM.setFileInputFiles` com `files: ["/tmp/agclaw-a.png", "/tmp/agclaw-b.jpg"]` e `nodeId: 101`. |
| `CdpService - Image Transfer` $\rightarrow$ `does not dispatch duplicate input/change events after DOM.setFileInputFiles` | `TestClient_InjectMessageWithImageFiles_NoDuplicateDispatchEvent(t *testing.T)` | Inspeciona todas as chamadas `Runtime.evaluate` que contêm `'data-agclaw-upload-token'` e verifica que nenhuma delas contém a substring `'dispatchEvent'`. |
| `CdpService - Image Transfer` $\rightarrow$ `extracts image data from the latest response` | `TestClient_ExtractLatestResponseImages_ReturnsExtractedImages(t *testing.T)` | `ExtractLatestResponseImages(4)` retorna 1 imagem `{ Name: "generated-1.png", MimeType: "image/png", Base64Data: <base64> }`. |

### 6.3. `internal/cdp/operations_uisync_test.go` (Mapeado de [`tests/services/cdpService.uiSync.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/cdpService.uiSync.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `setUiMode` $\rightarrow$ `throws an error when not connected` | `TestClient_SetUIMode_ErrorWhenDisconnected(t *testing.T)` | Quando desconectado, `SetUIMode("plan")` retorna erro contendo `"Not connected to CDP"`. |
| `setUiMode` $\rightarrow$ `executes a UI manipulation script via CDP when connected` | `TestClient_SetUIMode_ExecutesEvaluateWhenConnected(t *testing.T)` | Quando conectado, invoca `Runtime.evaluate` com `expression` contendo `"plan"`, `returnByValue: true`, `awaitPromise: true` e retorna `OK: true`. |
| `setUiMode` $\rightarrow$ `maps internal mode names to UI display names` | `TestClient_SetUIMode_MapsInternalNamesToUIDisplayNames(t *testing.T)` | A `expression` enviada em `SetUIMode("plan")` contém `"Planning"` e `"Fast"`, e o resultado retorna `OK: true, Mode: "Planning"`. |
| `setUiMode` $\rightarrow$ `returns the mode name on successful UI operation` | `TestClient_SetUIMode_ReturnsFastModeName(t *testing.T)` | `SetUIMode("fast")` retorna `OK: true, Mode: "Fast"`. |
| `setUiMode` $\rightarrow$ `uses dialog-based selectors in the expression` | `TestClient_SetUIMode_UsesDialogSelectors(t *testing.T)` | A `expression` contém `role=\"dialog\"`, `.font-medium` e `cursor-pointer`. |
| `setUiMode` $\rightarrow$ `returns ok: false when DOM elements are not found` | `TestClient_SetUIMode_ReturnsFalseWhenDOMNotFound(t *testing.T)` | Quando o script retorna `{ ok: false, error: 'Mode toggle button not found' }`, `SetUIMode` retorna `OK: false` e `Error != ""`. |
| `setUiMode` $\rightarrow$ `returns ok: false without crashing when a CDP error occurs` | `TestClient_SetUIMode_HandlesCDPErrorGracefully(t *testing.T)` | Quando `Call` falha com `"CDP通信エラー"`, `SetUIMode` não entra em pânico nem retorna erro fatal, mas sim `UISyncResult{OK: false, Error: "CDP通信エラー"}`. |
| `setUiModel` $\rightarrow$ `throws an error when not connected` | `TestClient_SetUIModel_ErrorWhenDisconnected(t *testing.T)` | Quando desconectado, `SetUIModel("gpt-4o")` retorna erro contendo `"Not connected to CDP"`. |
| `setUiModel` $\rightarrow$ `executes a UI manipulation script via CDP when connected` | `TestClient_SetUIModel_ExecutesEvaluateWhenConnected(t *testing.T)` | Invoca `Runtime.evaluate` com `expression` contendo `"gpt-4o"`, `returnByValue: true`, `awaitPromise: true` e retorna `OK: true`. |
| `setUiModel` $\rightarrow$ `returns the model name on successful UI operation` | `TestClient_SetUIModel_ReturnsModelName(t *testing.T)` | `SetUIModel("claude-3-opus")` retorna `OK: true, Model: "claude-3-opus"`. |
| `setUiModel` $\rightarrow$ `returns ok: false when DOM elements are not found` | `TestClient_SetUIModel_ReturnsFalseWhenDOMNotFound(t *testing.T)` | Quando o script retorna `{ ok: false, error: 'モデルセレクターが見つかりませんでした' }`, retorna `OK: false` e `Error != ""`. |
| `setUiModel` $\rightarrow$ `returns ok: false without crashing when a CDP error occurs` | `TestClient_SetUIModel_HandlesCDPErrorGracefully(t *testing.T)` | Quando `Call` falha com `"タイムアウト"`, retorna `UISyncResult{OK: false, Error: "タイムアウト"}`. |
| `setUiModel` $\rightarrow$ `uses the primary context ID when a context exists` | `TestClient_SetUIModel_UsesPrimaryContextID(t *testing.T)` | Com contexto `{ ID: 42, Name: "cascade-panel", URL: "vscode-webview://cascade-panel" }`, `SetUIModel("gemini-1.5-pro")` passa `contextId: 42` em `Runtime.evaluate`. |

### 6.4. `internal/cdp/screenshot_test.go` (Mapeado de [`tests/services/screenshotService.test.ts`](file:///Users/vinicius.canedo/Documents/trae_projects/antigravity-telegram-remote/tests/services/screenshotService.test.ts))

| Teste Original (`describe` / `it`) | Função de Teste em Go | Critério de Aceite Exato |
| :--- | :--- | :--- |
| `ScreenshotService` $\rightarrow$ `captures a screenshot and returns a Buffer` | `TestScreenshotService_Capture_ReturnsDecodedBuffer(t *testing.T)` | Dado `dummyBase64` (`1x1` PNG), `Capture()` retorna `Success: true` e `len(Buffer) > 0` com os bytes PNG decodificados. |
| `ScreenshotService` $\rightarrow$ `calls captureScreenshot with the correct CDP command and parameters` | `TestScreenshotService_Capture_DefaultPNGFormat(t *testing.T)` | Chama `Page.captureScreenshot` com `{ format: "png" }`. |
| `ScreenshotService` $\rightarrow$ `captures with JPEG format specified` | `TestScreenshotService_Capture_JPEGFormatWithQuality(t *testing.T)` | `Capture(CaptureOptions{Format: "jpeg", Quality: ptr(80)})` envia `{ format: "jpeg", quality: 80 }` para `Page.captureScreenshot`. |
| `ScreenshotService` $\rightarrow$ `returns success:false with error message on CDP error` | `TestScreenshotService_Capture_ReturnsErrorOnCDPFailure(t *testing.T)` | Quando `Call` retorna erro `"CDP接続エラー"`, `Capture()` retorna `Success: false` e `Error` contendo `"CDP接続エラー"`. |
| `ScreenshotService` $\rightarrow$ `returns success:false when CDP returns empty data` | `TestScreenshotService_Capture_ReturnsErrorOnEmptyData(t *testing.T)` | Quando `Page.captureScreenshot` retorna `{ data: "" }`, `Capture()` retorna `Success: false` e `Error: "Screenshot data was empty."`. |
| `ScreenshotService` $\rightarrow$ `retrieves base64-encoded image string via getBase64()` | `TestScreenshotService_GetBase64_ReturnsRawBase64String(t *testing.T)` | `GetBase64()` retorna ponteiro para string igual a `dummyBase64`. |
| `ScreenshotService` $\rightarrow$ `captures with a specified clip region` | `TestScreenshotService_Capture_WithClipRegion(t *testing.T)` | `Capture(CaptureOptions{Clip: &ClipRegion{X: 0, Y: 0, Width: 800, Height: 600, Scale: 1}})` repassa o objeto `clip` exato em `Page.captureScreenshot`. |
