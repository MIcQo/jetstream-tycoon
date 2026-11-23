---
name: release-mobile
description: Prepare and ship an iOS/Android release (versioning, content version, Sentry release, staged rollout).
---

# Release the mobile apps

1. Bump `appVersion` (semver) and the build numbers in `mobile/`. Note the `contentVersion` and `schemaVersion`.
2. Pre-flight:
   - `make test`, `make determinism` and `make content-check` are green.
   - WASM size is within budget.
   - The save migration tests from every past `schemaVersion` pass.
3. Build: `npm run build`, then `npx cap sync`, then the native builds in CI.
4. Sentry: create the release `appVersion+contentVersion` and upload source maps, dSYM and ProGuard mappings.
5. Check that the consent flows work (UMP, ATT), that ads use **test IDs in debug** and production IDs in release, and that offline mode works.
6. Ship to TestFlight and Play internal testing first, then a staged rollout (5% → 20% → 50% → 100%).
   - Gate each step on the Sentry crash-free rate (≥ 99.5% sessions) and on API error rates.
7. If a client bug needs blocking, raise `minAppVersion` in remote config.
