## MODIFIED Requirements

### Requirement: Project and Workspace Selection UI
The `internal/bot/ui` package SHALL render paginated workspace selection menus, clamp out-of-range pages, and validate project callback IDs.

#### Scenario: Empty project list
- **WHEN** workspaces slice is empty
- **THEN** it returns the empty state message and an empty keyboard

#### Scenario: Paginated workspace list and clamping
- **WHEN** workspaces count exceeds 100
- **THEN** it displays page info, clamps negative or overflow pages, and provides `◀ Prev` and `Next ▶` navigation buttons
