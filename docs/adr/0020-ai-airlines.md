# ADR-0020: AI airlines as full simulation entities driven by utility AI through the player's command path

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
Virtual per-route competitors make pricing meaningful, but they don't compete for countries, can't be acquired and have no strategy. We want real rival airlines with personalities and difficulty levels, exclusive country ownership with buyouts, and takeovers in both directions. All of it must stay deterministic and offline (ADR-0002, ADR-0006), and the design must leave room for multiplayer later.

## Decision
- **Multi-airline state.** The player is one `Airline` among several. Ownership is stored on world entities (`Country.ownerId`, `Route.airlineId`, ...), and every system loops over airlines in sorted ID order.
- **Controller interface** (`human`, `ai`, later `remote`). AI airlines act **only by issuing the same commands** as the player, through the same validation. There are no hidden bonuses except an explicit, content-defined Hard-difficulty cash multiplier.
- **Utility AI**:
  - Each AI decides once per game day at a staggered hour, plus urgent triggers.
  - Capped candidate generators (expansion, network, fleet, pricing, finance, corporate) feed a score = expected profit × personality weights − risk.
  - Forecast noise, mistake chance, action caps and reaction delay come from `difficulty.yaml`. Personalities are content (`ai_personalities.yaml`).
- **Determinism**: each AI has its own RNG stream, `hash(seed, "ai", airlineID, tick)`, and runs inside the simulation.
- **Markets**: market share per airport pair and class is split among all real airlines plus background carriers (the former virtual competitors).
- **Corporate actions** use one `Offer` entity (country buyout, airline takeover) between any two airlines. Acceptance rules depend on valuation, importance and financial distress.
- **Network candidates** are precomputed in datagen (top 30 destinations per airport), which keeps AI search cheap enough for WASM (≤ 5 ms per AI per day).
- `tools/balancebot` runs AI-only games as the balancing harness.

## Consequences
- The game is much deeper and replayable. Multiplayer later means swapping in a `remote` controller.
- The AI is fair by construction (same rules). AI decision quality is a real ongoing tuning cost.
- Every system must handle multiple airlines, and the save size grows by about the number of airlines.
- More simulation CPU on phones, kept in check by the staggered schedule, candidate caps and the `Markets` index.
- New risks to balance: snowballing (the leader keeps growing). Mitigations: background carriers, the monopoly cap on takeovers, and price-war events.

## Alternatives considered
- **Keep only virtual competitors**: simpler, but no strategic rivals.
- **Scripted / cheating AI** (bonus income, perfect information): cheaper to build, but feels unfair and breaks the balance-bot use.
- **Goal-oriented planning / MCTS AI**: stronger play, but too expensive in WASM and harder to tune than utility scoring.
- **AI running on the server**: breaks offline play.
