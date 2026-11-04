## Context

The Antigravity agent panel renders complex UI trees containing conversation messages, code blocks with copy bars and styles, collapsible `<details>` thinking and tool execution summaries, non-selector leaf activity text nodes, and feedback buttons.

In TypeScript, this was split into:
1. `src/services/assistantDomExtractor.ts`:
   - Browser-side IIFE string evaluated by CDP `Runtime.evaluate`.
   - Node-side classifier `classifyAssistantSegments` converting raw segments into Telegram HTML and distinct message channels.
2. `src/tools/doctorSelector.ts`:
   - Standalone CLI utility auditing 7 key selectors against active CDP targets.

In Go, these capabilities map to:
- `internal/monitor/dom_extractor.go`: Stateless DOM extractor script generator and classifier in package `monitor`.
- `internal/monitor/selectors.go`: Catalog of all 11 response selectors shared between `ResponseMonitor` and diagnostic tools.
- `internal/cli/doctor_selectors.go`: Doctor selectors runner in package `cli` with dependency-injected CDP client and logging interfaces.

## Goals / Non-Goals

**Goals:**
- Provide full 1:1 behavioral parity with `assistantDomExtractor.ts` and `doctorSelector.ts`.
- Pure functions and thread-safe execution without shared mutable state in `dom_extractor.go`.
- Robust payload handling accepting `*AssistantDomSegmentPayload`, `AssistantDomSegmentPayload`, `map[string]any`, `json.RawMessage`, `[]byte`, `string`, and `nil`.
- Testable `RunDoctorSelectorsWithDeps` supporting mock CDP connections, verifying healthy, partial, broken, warning, and connection failure states.
- 100% test coverage matching the original test suite.

**Non-Goals:**
- Direct network/WebSocket communication inside `dom_extractor.go` (it only generates the script string and parses the resulting payload).
- Mutating browser DOM state (the IIFE only reads and cleans cloned DOM nodes, restoring collapsed details state).

## Decisions

- **Polymorphic Payload Parsing in `ClassifyAssistantSegments`**:
  - *Context*: CDP `Runtime.evaluate` returns result values either as decoded JSON `map[string]any` or raw JSON messages depending on client wrappers.
  - *Decision*: `isValidPayload` handles strongly typed structs, raw bytes, strings, and generic `map[string]any`. Invalid payloads return a `legacy-fallback` diagnostics result with `fallbackReason: "invalid-payload"`.
- **Pre Block Normalization in IIFE**:
  - *Context*: Antigravity embeds `<style>` tags, header bars with language labels and copy buttons, and `.code-line` divs instead of standard `<pre><code>`.
  - *Decision*: The JS IIFE clones the candidate body node, extracts language class, strips `<style>` and header bars, extracts code lines or innerText, strips trailing copy text, and replaces with `<pre><code class="language-...">`.
- **Preservation of Non-Selector Activity in Pass 2.5**:
  - *Context*: Some activity messages occur in simple `<div>` elements outside standard content containers.
  - *Decision*: Broad scan with leaf-node filtering (`children.length <= 3`), excluded container checks (`role="dialog"`, `.notify-user-container`), activity verb checks, up-to-3-level ancestor expansion, and ancestor deduplication.
- **Dependency-Injected Doctor Selector Runner**:
  - *Context*: `doctorSelector.ts` used `CdpService` directly, which makes unit testing difficult without a running browser.
  - *Decision*: Introduce `CdpCaller` interface (`Connect`, `Disconnect`, `IsConnected`, `Call`) and `RunDoctorSelectorsWithDeps` to allow injecting mock clients, writers, and loggers.

## Risks / Trade-offs

- [Risk] Antigravity UI updates altering DOM structures or class names.
  → [Mitigation] `doctorSelector` audits all 7 core selectors on demand with clear health status reports (`Score: X/7 Healthy`).
- [Risk] Large DOM trees causing slow IIFE evaluation.
  → [Mitigation] Scoped querying to `.antigravity-agent-side-panel`, reverse iteration on recency-first body search, and leaf-node limits.
