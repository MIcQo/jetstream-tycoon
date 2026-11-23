---
name: run-balance
description: Run headless AI-only balance simulations and compare against a baseline.
---

# Run a balance check

1. Baseline from main:
   ```bash
   make balance ARGS="--seeds 50 --years 10 --out out/base.json"
   ```
2. Candidate with your content change:
   ```bash
   make balance ARGS="--seeds 50 --years 10 --out out/cand.json"
   ```
3. Compare the two:
   ```bash
   go run ./tools/balancebot compare out/base.json out/cand.json
   ```
4. Report a before/after table:
   - Bankruptcy rate by personality and difficulty.
   - Median net worth at years 1/3/10.
   - Leader capacity share.
   - Time to first profit.
   - Personality win rates.
   - Event EV and accept rates.
5. Guardrails (Normal difficulty):
   - Bankruptcy 10–25%.
   - No personality wins more than 35%.
   - Leader share below 45% at year 10.
   - First profitable route within 3 game days for a sensible start.
6. Use the same seeds for both runs. Changes that don't fit within the noise (compare runs with different seeds) are not conclusive.
