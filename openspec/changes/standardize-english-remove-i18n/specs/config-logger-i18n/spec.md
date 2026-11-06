## REMOVED Requirements

### Requirement: Embedded Multi-Language Internationalization (i18n)
**Reason**: The internationalization module is unused and removed; all user-facing strings are standardized directly in English.
**Migration**: Remove `internal/i18n/` package and use static English string literals directly in code.
