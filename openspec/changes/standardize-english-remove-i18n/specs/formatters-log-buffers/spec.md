## MODIFIED Requirements

### Requirement: Streaming Message Formatting
The `formatter` package SHALL provide `BuildModeModelLines`, `ShouldSkipActivityLog`, `SplitForEmbedDescription`, and `FitForSingleEmbedDescription`.

#### Scenario: Build mode and model lines
- **WHEN** fast and plan models are identical (case-insensitive)
- **THEN** a single consolidated model line is generated with English labels ("Current mode: ...", "Model: ...")
- **WHEN** models differ
- **THEN** separate lines for Fast and Plan models are generated with English labels ("Current mode: ...", "Fast Model: ...", "Plan Model: ...")

#### Scenario: Activity log noise filtering
- **WHEN** activity logs contain generic status words, single-word tokens, or analyzed trace prefixes
- **THEN** `ShouldSkipActivityLog` returns `true`

#### Scenario: Truncation prefix formatting
- **WHEN** text length exceeds `maxLength` in `FitForSingleEmbedDescription`
- **THEN** the text is truncated from the head and prefixed with `"...(earlier output truncated)\n"`
