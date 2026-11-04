package cdp

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"antigravity-telegram-remote/internal/logger"
)

// InjectResult mirrors InjectResult (src/services/cdpService.ts:25-30).
type InjectResult struct {
	OK        bool   `json:"ok"`
	Method    string `json:"method,omitempty"`
	ContextID *int   `json:"contextId,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ExtractedResponseImage mirrors ExtractedResponseImage (src/services/cdpService.ts:32-37).
type ExtractedResponseImage struct {
	Name       string `json:"name"`
	MimeType   string `json:"mimeType"`
	Base64Data string `json:"base64Data,omitempty"`
	URL        string `json:"url,omitempty"`
}

// UISyncResult mirrors UiSyncResult (src/services/cdpService.ts:40-47).
type UISyncResult struct {
	OK    bool   `json:"ok"`
	Mode  string `json:"mode,omitempty"`
	Model string `json:"model,omitempty"`
	Error string `json:"error,omitempty"`
}

// Selectors stores DOM selector constants (src/services/cdpService.ts:50-59).
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

const (
	locateInputScript = `(async () => {
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
        })()`

	getCurrentModeScript = `(() => { const uiNameMap = { fast: "Fast", plan: "Planning" }; const knownModes = Object.values(uiNameMap).map(n => n.toLowerCase()); const reverseMap = {}; Object.entries(uiNameMap).forEach(([k, v]) => { reverseMap[v.toLowerCase()] = k; }); const allBtns = Array.from(document.querySelectorAll("button")); const visibleBtns = allBtns.filter(b => b.offsetParent !== null); const modeToggleBtn = visibleBtns.find(b => {   const text = (b.textContent || "").trim().toLowerCase();   const hasChevron = b.querySelector("svg[class*=\"chevron\"]");   return knownModes.some(m => text === m) && hasChevron; }); if (!modeToggleBtn) return null; const currentModeText = (modeToggleBtn.textContent || "").trim().toLowerCase(); return reverseMap[currentModeText] || null;})()`

	getUIModelsScript = `(async () => {
            return Array.from(document.querySelectorAll('div.cursor-pointer'))
                .map(e => ({text: (e.textContent || '').trim().replace(/New$/, ''), class: e.className}))
                .filter(e => e.class.includes('px-2 py-1 flex items-center justify-between') || e.text.includes('Gemini') || e.text.includes('GPT') || e.text.includes('Claude'))
                .map(e => e.text);
        })()`

	getCurrentModelScript = `(() => {
            return Array.from(document.querySelectorAll('div.cursor-pointer'))
                .find(e => e.className.includes('px-2 py-1 flex items-center justify-between') && e.className.includes('bg-gray-500/20'))
                ?.textContent?.trim().replace(/New$/, '') || null;
        })()`
)

func buildExtractLatestResponseImagesScript(safeMaxImages int) string {
	return fmt.Sprintf(`(async () => {
            const maxImages = %d;
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
        })()`, safeMaxImages)
}

func buildSetUIModeScript(modeName string) string {
	safeMode, _ := json.Marshal(modeName)
	uiNameMap := `{"fast":"Fast","plan":"Planning"}`
	return `(async () => { const targetMode = ` + string(safeMode) + `; const targetModeLower = targetMode.toLowerCase(); const uiNameMap = ` + uiNameMap + `; const targetUiName = uiNameMap[targetModeLower] || targetMode; const targetUiNameLower = targetUiName.toLowerCase(); const allBtns = Array.from(document.querySelectorAll("button")); const visibleBtns = allBtns.filter(b => b.offsetParent !== null); const knownModes = Object.values(uiNameMap).map(n => n.toLowerCase()); const modeToggleBtn = visibleBtns.find(b => {   const text = (b.textContent || "").trim().toLowerCase();   const hasChevron = b.querySelector("svg[class*=\"chevron\"]");   return knownModes.some(m => text === m) && hasChevron; }); if (!modeToggleBtn) {   return { ok: false, error: "Mode toggle button not found" }; } const currentModeText = (modeToggleBtn.textContent || "").trim().toLowerCase(); if (currentModeText === targetUiNameLower) {   return { ok: true, mode: targetUiName, alreadySelected: true }; } modeToggleBtn.click(); await new Promise(r => setTimeout(r, 500)); const dialogs = Array.from(document.querySelectorAll("[role=\"dialog\"]")); const visibleDialog = dialogs.find(d => {   const style = window.getComputedStyle(d);   return style.visibility !== "hidden" && style.display !== "none"; }); let modeOption = null; if (visibleDialog) {   const fontMediumEls = Array.from(visibleDialog.querySelectorAll(".font-medium"));   const matchEl = fontMediumEls.find(el => {     const text = (el.textContent || "").trim().toLowerCase();     return text === targetUiNameLower;   });   if (matchEl) {     modeOption = matchEl.closest("div.cursor-pointer") || matchEl.parentElement;   } } if (!modeOption) {   const fallbackEls = Array.from(document.querySelectorAll(     "div[class*=\"cursor-pointer\"]"   )).filter(el => el.offsetParent !== null);   modeOption = fallbackEls.find(el => {     if (el === modeToggleBtn) return false;     const fm = el.querySelector(".font-medium");     if (fm) {       const text = (fm.textContent || "").trim().toLowerCase();       return text === targetUiNameLower;     }     return false;   }); } if (modeOption) {   modeOption.click();   await new Promise(r => setTimeout(r, 500));   const updBtn = Array.from(document.querySelectorAll("button"))     .filter(b => b.offsetParent !== null)     .find(b => b.querySelector("svg[class*=\"chevron\"]") && knownModes.some(m => (b.textContent || "").trim().toLowerCase() === m));   const newMode = updBtn ? (updBtn.textContent || "").trim() : "unknown";   return { ok: true, mode: newMode }; } document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true })); await new Promise(r => setTimeout(r, 200)); return { ok: false, error: "Mode option " + targetUiName + " not found in dropdown" };})()`
}

func buildSetUIModelScript(modelName string) string {
	safeModel, _ := json.Marshal(modelName)
	return fmt.Sprintf(`(async () => {
            const targetModel = %s;
            
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
        })()`, string(safeModel))
}

// focusChatInput attempts to focus the chat input in one of the registered contexts.
func (c *Client) focusChatInput() (bool, *int, string) {
	focusScript := fmt.Sprintf(`(() => {
            const editors = Array.from(document.querySelectorAll('%s'));
            const visible = editors.filter(el => el.offsetParent !== null);
            const editor = visible[visible.length - 1];
            if (!editor) return { ok: false, error: 'No editor found' };
            editor.focus();
            return { ok: true };
        })()`, Selectors.ChatInput)

	c.mu.RLock()
	contexts := make([]Context, len(c.contexts))
	copy(contexts, c.contexts)
	c.mu.RUnlock()

	for _, ctx := range contexts {
		resRaw, err := c.Call("Runtime.evaluate", map[string]any{
			"expression":    focusScript,
			"returnByValue": true,
			"contextId":     ctx.ID,
		})
		if err != nil {
			continue
		}

		var evalRes struct {
			Result struct {
				Value struct {
					OK    bool   `json:"ok"`
					Error string `json:"error"`
				} `json:"value"`
			} `json:"result"`
		}
		if err := json.Unmarshal(resRaw, &evalRes); err == nil && evalRes.Result.Value.OK {
			ctxID := ctx.ID
			return true, &ctxID, ""
		}
	}

	return false, nil, "Chat input field not found"
}

// clearInputField clears any existing text in the focused input.
func (c *Client) clearInputField() error {
	mod := 2
	if c.getOS() == "darwin" {
		mod = 4
	}

	keyEvents := []map[string]any{
		{
			"type":                  "keyDown",
			"key":                   "a",
			"code":                  "KeyA",
			"modifiers":             mod,
			"windowsVirtualKeyCode": 65,
			"nativeVirtualKeyCode":  65,
		},
		{
			"type":                  "keyUp",
			"key":                   "a",
			"code":                  "KeyA",
			"modifiers":             mod,
			"windowsVirtualKeyCode": 65,
			"nativeVirtualKeyCode":  65,
		},
		{
			"type":                  "keyDown",
			"key":                   "Backspace",
			"code":                  "Backspace",
			"windowsVirtualKeyCode": 8,
			"nativeVirtualKeyCode":  8,
		},
		{
			"type":                  "keyUp",
			"key":                   "Backspace",
			"code":                  "Backspace",
			"windowsVirtualKeyCode": 8,
			"nativeVirtualKeyCode":  8,
		},
	}

	for _, evt := range keyEvents {
		if _, err := c.Call("Input.dispatchKeyEvent", evt); err != nil {
			return err
		}
	}

	time.Sleep(50 * time.Millisecond)
	return nil
}

// pressEnterToSend submits the input with an Enter key event.
func (c *Client) pressEnterToSend() error {
	keyEvents := []map[string]any{
		{
			"type":                  "keyDown",
			"key":                   "Enter",
			"code":                  "Enter",
			"windowsVirtualKeyCode": 13,
			"nativeVirtualKeyCode":  13,
		},
		{
			"type":                  "keyUp",
			"key":                   "Enter",
			"code":                  "Enter",
			"windowsVirtualKeyCode": 13,
			"nativeVirtualKeyCode":  13,
		},
	}

	for _, evt := range keyEvents {
		if _, err := c.Call("Input.dispatchKeyEvent", evt); err != nil {
			return err
		}
	}

	return nil
}

// attachImageFiles locates or triggers the file input element and sets the file paths.
func (c *Client) attachImageFiles(filePaths []string, contextID *int) (bool, string) {
	if len(filePaths) == 0 {
		return true, ""
	}

	if _, err := c.Call("DOM.enable", map[string]any{}); err != nil {
		return false, err.Error()
	}

	callParams := map[string]any{
		"expression":    locateInputScript,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	locateResult, err := c.Call("Runtime.evaluate", callParams)
	if err != nil {
		return false, err.Error()
	}

	var evalRes struct {
		Result struct {
			Value struct {
				OK    bool   `json:"ok"`
				Token string `json:"token"`
				Error string `json:"error"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(locateResult, &evalRes); err != nil {
		return false, err.Error()
	}
	if !evalRes.Result.Value.OK || evalRes.Result.Value.Token == "" {
		errMsg := evalRes.Result.Value.Error
		if errMsg == "" {
			errMsg = "Failed to locate file input"
		}
		return false, errMsg
	}

	token := evalRes.Result.Value.Token
	documentResult, err := c.Call("DOM.getDocument", map[string]any{
		"depth":  1,
		"pierce": true,
	})
	if err != nil {
		return false, err.Error()
	}

	var docRes struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := json.Unmarshal(documentResult, &docRes); err != nil || docRes.Root.NodeID == 0 {
		return false, "Failed to get DOM root"
	}

	selector := fmt.Sprintf(`input[data-agclaw-upload-token="%s"]`, token)
	nodeResult, err := c.Call("DOM.querySelector", map[string]any{
		"nodeId":   docRes.Root.NodeID,
		"selector": selector,
	})
	if err != nil {
		return false, err.Error()
	}

	var nodeRes struct {
		NodeID int `json:"nodeId"`
	}
	if err := json.Unmarshal(nodeResult, &nodeRes); err != nil || nodeRes.NodeID == 0 {
		return false, "Failed to get upload input node"
	}

	if _, err := c.Call("DOM.setFileInputFiles", map[string]any{
		"nodeId": nodeRes.NodeID,
		"files":  filePaths,
	}); err != nil {
		return false, err.Error()
	}

	selectorJSON, _ := json.Marshal(selector)
	notifyScript := fmt.Sprintf(`(() => {
            const input = document.querySelector(%s);
            if (!input) return { ok: false, error: 'Image input not found' };
            input.removeAttribute('data-agclaw-upload-token');
            return { ok: true };
        })()`, string(selectorJSON))

	notifyParams := map[string]any{
		"expression":    notifyScript,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextID != nil {
		notifyParams["contextId"] = *contextID
	}
	_, _ = c.Call("Runtime.evaluate", notifyParams)

	time.Sleep(250 * time.Millisecond)
	return true, ""
}

// InjectMessage injects and sends text into Antigravity's chat input.
func (c *Client) InjectMessage(text string) (InjectResult, error) {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return InjectResult{}, errors.New("Not connected to CDP. Call connect() first.")
	}

	logger.Debug(fmt.Sprintf("[CDP] Injecting message to LLM (%d chars)", len(text)))
	if len(text) > 0 && len(text) < 500 {
		preview := text
		if len(preview) > 100 {
			preview = preview[:100]
		}
		logger.Debug(fmt.Sprintf("[CDP] Message preview: %s", preview))
	}

	ok, contextID, errMsg := c.focusChatInput()
	if !ok {
		if errMsg == "" {
			errMsg = "Chat input field not found"
		}
		return InjectResult{OK: false, Error: errMsg}, nil
	}

	if err := c.clearInputField(); err != nil {
		return InjectResult{OK: false, Error: err.Error()}, nil
	}

	if _, err := c.Call("Input.insertText", map[string]any{"text": text}); err != nil {
		return InjectResult{OK: false, Error: err.Error()}, nil
	}

	time.Sleep(200 * time.Millisecond)

	if err := c.pressEnterToSend(); err != nil {
		return InjectResult{OK: false, Error: err.Error()}, nil
	}

	return InjectResult{OK: true, Method: "enter", ContextID: contextID}, nil
}

// InjectMessageWithImageFiles attaches image files to the UI and sends text into the chat.
func (c *Client) InjectMessageWithImageFiles(text string, imageFilePaths []string) (InjectResult, error) {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return InjectResult{}, errors.New("Not connected to CDP. Call connect() first.")
	}

	ok, contextID, errMsg := c.focusChatInput()
	if !ok {
		if errMsg == "" {
			errMsg = "Chat input field not found"
		}
		return InjectResult{OK: false, Error: errMsg}, nil
	}

	if err := c.clearInputField(); err != nil {
		return InjectResult{OK: false, Error: err.Error()}, nil
	}

	attachOK, attachErr := c.attachImageFiles(imageFilePaths, contextID)
	if !attachOK {
		if attachErr == "" {
			attachErr = "Failed to attach images"
		}
		return InjectResult{OK: false, Error: attachErr}, nil
	}

	if _, err := c.Call("Input.insertText", map[string]any{"text": text}); err != nil {
		return InjectResult{OK: false, Error: err.Error()}, nil
	}

	time.Sleep(200 * time.Millisecond)

	if err := c.pressEnterToSend(); err != nil {
		return InjectResult{OK: false, Error: err.Error()}, nil
	}

	return InjectResult{OK: true, Method: "enter", ContextID: contextID}, nil
}

// ExtractLatestResponseImages extracts images generated in the assistant's latest response.
func (c *Client) ExtractLatestResponseImages(maxImages int) []ExtractedResponseImage {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return []ExtractedResponseImage{}
	}

	if maxImages <= 0 {
		maxImages = 4
	}
	safeMaxImages := maxImages
	if safeMaxImages < 1 {
		safeMaxImages = 1
	} else if safeMaxImages > 8 {
		safeMaxImages = 8
	}

	expression := buildExtractLatestResponseImagesScript(safeMaxImages)
	contextID := c.GetPrimaryContextID()

	callParams := map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	resRaw, err := c.Call("Runtime.evaluate", callParams)
	if err != nil {
		return []ExtractedResponseImage{}
	}

	var evalRes struct {
		Result struct {
			Value []struct {
				Name       string  `json:"name"`
				MimeType   *string `json:"mimeType"`
				Base64Data *string `json:"base64Data"`
				URL        *string `json:"url"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &evalRes); err != nil {
		return []ExtractedResponseImage{}
	}

	var images []ExtractedResponseImage
	for _, item := range evalRes.Result.Value {
		if item.Name == "" {
			continue
		}
		mime := "image/png"
		if item.MimeType != nil && *item.MimeType != "" {
			mime = *item.MimeType
		}
		img := ExtractedResponseImage{
			Name:     item.Name,
			MimeType: mime,
		}
		if item.Base64Data != nil {
			img.Base64Data = *item.Base64Data
		}
		if item.URL != nil {
			img.URL = *item.URL
		}
		images = append(images, img)
	}

	if images == nil {
		return []ExtractedResponseImage{}
	}
	return images
}

// GetCurrentMode reads the active execution mode ('fast' or 'plan') from the Antigravity UI.
func (c *Client) GetCurrentMode() *string {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return nil
	}

	contextID := c.GetPrimaryContextID()
	callParams := map[string]any{
		"expression":    getCurrentModeScript,
		"returnByValue": true,
		"awaitPromise":  false,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	resRaw, err := c.Call("Runtime.evaluate", callParams)
	if err != nil {
		return nil
	}

	var evalRes struct {
		Result struct {
			Value *string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &evalRes); err != nil {
		return nil
	}

	return evalRes.Result.Value
}

// SetUIMode changes the execution mode in the Antigravity UI.
func (c *Client) SetUIMode(modeName string) (UISyncResult, error) {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return UISyncResult{}, errors.New("Not connected to CDP. Call connect() first.")
	}

	expression := buildSetUIModeScript(modeName)
	contextID := c.GetPrimaryContextID()

	callParams := map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	resRaw, err := c.Call("Runtime.evaluate", callParams)
	if err != nil {
		return UISyncResult{OK: false, Error: err.Error()}, nil
	}

	var evalRes struct {
		Result struct {
			Value struct {
				OK    bool   `json:"ok"`
				Mode  string `json:"mode"`
				Error string `json:"error"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &evalRes); err != nil {
		return UISyncResult{OK: false, Error: err.Error()}, nil
	}

	if evalRes.Result.Value.OK {
		return UISyncResult{OK: true, Mode: evalRes.Result.Value.Mode}, nil
	}

	errMsg := evalRes.Result.Value.Error
	if errMsg == "" {
		errMsg = "UI operation failed (setUiMode)"
	}
	return UISyncResult{OK: false, Error: errMsg}, nil
}

// GetUIModels queries available models from the Antigravity UI dropdown.
func (c *Client) GetUIModels() ([]string, error) {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return nil, errors.New("Not connected to CDP.")
	}

	contextID := c.GetPrimaryContextID()
	callParams := map[string]any{
		"expression":    getUIModelsScript,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	resRaw, err := c.Call("Runtime.evaluate", callParams)
	if err != nil {
		logger.Error("Failed to get UI models:", err)
		return []string{}, nil
	}

	var evalRes struct {
		Result struct {
			Value []string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &evalRes); err != nil {
		return []string{}, nil
	}

	seen := make(map[string]struct{})
	var unique []string
	for _, m := range evalRes.Result.Value {
		if _, ok := seen[m]; !ok {
			seen[m] = struct{}{}
			unique = append(unique, m)
		}
	}
	if unique == nil {
		return []string{}, nil
	}
	return unique, nil
}

// GetCurrentModel reads the currently selected model from the Antigravity UI.
func (c *Client) GetCurrentModel() *string {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return nil
	}

	contextID := c.GetPrimaryContextID()
	callParams := map[string]any{
		"expression":    getCurrentModelScript,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	resRaw, err := c.Call("Runtime.evaluate", callParams)
	if err != nil {
		return nil
	}

	var evalRes struct {
		Result struct {
			Value *string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &evalRes); err != nil {
		return nil
	}

	return evalRes.Result.Value
}

// SetUIModel selects a model in the Antigravity UI dropdown.
func (c *Client) SetUIModel(modelName string) (UISyncResult, error) {
	c.mu.RLock()
	connected := c.isConnectedFlag && c.ws != nil
	c.mu.RUnlock()
	if !connected {
		return UISyncResult{}, errors.New("Not connected to CDP. Call connect() first.")
	}

	expression := buildSetUIModelScript(modelName)
	contextID := c.GetPrimaryContextID()

	callParams := map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if contextID != nil {
		callParams["contextId"] = *contextID
	}

	resRaw, err := c.Call("Runtime.evaluate", callParams)
	if err != nil {
		return UISyncResult{OK: false, Error: err.Error()}, nil
	}

	var evalRes struct {
		Result struct {
			Value struct {
				OK    bool   `json:"ok"`
				Model string `json:"model"`
				Error string `json:"error"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resRaw, &evalRes); err != nil {
		return UISyncResult{OK: false, Error: err.Error()}, nil
	}

	if evalRes.Result.Value.OK {
		return UISyncResult{OK: true, Model: evalRes.Result.Value.Model}, nil
	}

	errMsg := evalRes.Result.Value.Error
	if errMsg == "" {
		errMsg = "UI operation failed (setUiModel)"
	}
	return UISyncResult{OK: false, Error: errMsg}, nil
}
