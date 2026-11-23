---
name: add-i18n-strings
description: Add or change user-facing text with i18n keys (UI or content), ICU MessageFormat, pseudo-locale check.
---

# Add i18n strings

1. Never hardcode user-facing text. UI strings go in `web/src/i18n/en.json`; content strings go in `content/i18n/en.json`.
2. Key naming: `area.object.detail` (e.g. `route.panel.load_factor`).
3. Use ICU MessageFormat for plurals and selects:
   ```
   {count, plural, one {# plane} other {# planes}}
   ```
   Format money, numbers and dates with `Intl` helpers, never by string concatenation.
4. Simulation notifications pass `{key, params}`. Params are raw values; the UI formats them.
5. Run `make content-check`, which covers missing and unused keys. Check the en-XA pseudo-locale build for overflow.
6. Don't edit other locales' files by hand; they come from the translation platform.
