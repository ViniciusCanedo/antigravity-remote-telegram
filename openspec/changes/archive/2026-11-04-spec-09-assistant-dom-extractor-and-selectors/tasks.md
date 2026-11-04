## 1. Monitor Package Implementation

- [x] 1.1 Implement `internal/monitor/selectors.go` (`ResponseSelectorsCatalog`, `ResponseSelectors` with all 11 JS selector scripts)
- [x] 1.2 Implement `internal/monitor/dom_extractor.go` (`AssistantSegmentKind`, `AssistantDomSegment`, `AssistantDomSegmentPayload`, `ClassifyDiagnostics`, `ClassifyResult`, `ExtractAssistantSegmentsPayloadScript`, `ClassifyAssistantSegments`, `isValidPayload`)
- [x] 1.3 Implement `internal/monitor/dom_extractor_test.go` matching `tests/services/assistantDomExtractor.test.ts` (11 test cases)

## 2. CLI Package Implementation

- [x] 2.1 Implement `internal/cli/doctor_selectors.go` (`SelectorCheckEntry`, `SelectorEvalResult`, `DoctorSelectorReport`, `RunDoctorSelectors`, `RunDoctorSelectorsWithDeps`, `CdpCaller`)
- [x] 2.2 Implement `internal/cli/doctor_selectors_test.go` (auditing 7 selectors, healthy, warning, broken, CDP call error, connection failure)

## 3. Verification & Quality Gates

- [x] 3.1 Run `go test -v -race ./...` and verify 100% pass rate
- [x] 3.2 Run `go vet ./...`
- [x] 3.3 Run `gofmt -w .` and check `gofmt -l .` is clean
- [x] 3.4 Run `go build ./...`
- [x] 3.5 Sync spec delta to `openspec/specs/dom-extractor/spec.md`
- [x] 3.6 Archive change via `openspec archive -y spec-09-assistant-dom-extractor-and-selectors`
- [x] 3.7 Re-verify quality gates after archiving
