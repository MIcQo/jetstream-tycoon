# ADR-0004: Data-driven content with one shared schema for events and investments

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
The game must be easy to extend: new planes, events, investments, missions, seasons and tutorial steps should not need code changes. Many mechanics have the same shape (condition → choice → probabilistic outcomes → effects).

## Decision
- All game content lives in YAML under `content/`, is validated in CI, and is compiled to `content.json` at build time. WASM has no YAML parser.
- Incidents, news and investments share **one event schema**: each event has choices, each choice has a cost and outcomes, and each outcome has a chance, effects and an optional delayed reveal.
- Conditions and formulas use `expr-lang/expr`, a sandboxed expression language.
- New condition or effect kinds are added through small Go registries.
- Every content build has a `contentVersion`, which is stored in saves and analytics.

## Consequences
- Designers can add most content without engineers, and content can later ship remotely without an app release.
- A content validator and tests are needed to catch broken references and probabilities.
- The expression language adds a dependency and must stay deterministic.

## Alternatives considered
Hardcoded Go content (every addition needs code); a full scripting language such as Lua (more powerful, larger attack surface and WASM size).
