# ADR-0005: A single effects/modifier system for all bonuses and penalties

- **Status**: Accepted (refined by ADR-0021)
- **Date**: 2026-11-23

## Context
Events, investments, upgrades, seasons, ad rewards and missions all change game values temporarily or permanently. Separate code paths for each would be hard to extend and to balance.

## Decision
- Every change is either a **Modifier** `{scope, target, stat, op(mul|add|set), value, expiresTick}` or an instant **Action** (cash, reputation, damage, give plane).
- Systems read values only through `effects.Apply(stat, base, target)`.
- Seasons and holidays are generated as modifiers.
- Ad rewards are applied as simulation commands that produce modifiers or actions.

## Consequences
- New features reuse one well-tested mechanism.
- Stacking order must be defined and documented.
- Modifier counts must be bounded (they expire and are pruned) for performance.

## Alternatives considered
Ad-hoc flags per feature.
