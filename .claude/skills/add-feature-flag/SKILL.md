---
name: add-feature-flag
description: Add a remote config parameter or feature flag / A/B test, including client defaults and determinism handling.
---

# Add a feature flag or remote config parameter

1. Add a Postgres migration inserting the row into `feature_flags` (with `variants`, `rules`, `sim_affecting`) or `config_params`. Use skill `add-migration`.
2. Add the default to the bundled `web/src/config/defaults.json`, so the app works offline on first launch.
3. Read it on the client through `config.flag(key)` / `config.param(key)`. The bucket is `hash(flagKey + userId) % 100`, computed on the client.
4. **If it changes game rules** (`sim_affecting: true`):
   - Apply it only through the `setConfig` command at a new game or day boundary.
   - It is stored in `State.ConfigSnapshot`.
   - Never read remote config directly inside `sim/`.
5. Emit `flag_exposure{flag, variant}` once per session when the flag is first evaluated.
6. Plan its cleanup: note in the PR when the flag will be removed.
