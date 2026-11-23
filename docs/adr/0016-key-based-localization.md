# ADR-0016: Key-based localization with ICU MessageFormat

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
The game targets a global, mobile audience with real country and city names.

## Decision
- No hardcoded strings. The UI and content reference keys, and the simulation emits `{key, params}`.
- `i18next` with ICU MessageFormat; `Intl` for numbers, currency and dates.
- Localized country and city names come from the data sources.
- CI checks for missing keys and builds an en-XA pseudo-locale.
- Logical CSS properties keep RTL possible later.

## Consequences
- Adding a language is a translation task, not a code change.
- Some discipline from day one.

## Alternatives considered
Adding translations later (costly to retrofit).
