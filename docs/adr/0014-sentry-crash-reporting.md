# ADR-0014: Sentry for crash and error reporting

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
We need crash reporting for JS, native iOS/Android, Go WASM panics and the server, tied to releases.

## Decision
- `@sentry/capacitor` in the app and `sentry-go` on the server.
- The WASM bridge recovers panics and reports seed, tick, versions, flags and the last 200 commands, so the bug can be reproduced deterministically.
- Releases are tagged `appVersion+contentVersion`, with source maps and dSYM/ProGuard mappings uploaded in CI.
- Consent is respected and personal data scrubbed.

## Consequences
- Fast diagnosis and release-health gating for staged rollouts.
- An external dependency and cost, with a self-hosted option.

## Alternatives considered
Firebase Crashlytics (no web or server coverage); building our own (not worth it).
