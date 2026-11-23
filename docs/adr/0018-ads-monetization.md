# ADR-0018: Free-to-play monetization with AdMob rewarded ads; rewards as simulation commands

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
Monetization is free with ads. Ads must not hurt the experience, and rewards must not break determinism.

## Decision
- Rewarded ads (opt-in) are the main revenue: revenue boosts, cash grants, faster overhauls, investment odds, mission rerolls.
- Interstitials are rare (at most one per ~10 min, only at natural breaks, never during the tutorial or a decision).
- No banners on the map.
- Placements and pacing are set by remote config.
- Rewards enter the simulation as `adReward` commands.
- UMP/GDPR consent and iOS ATT; offline means no ads.

## Consequences
- Fair, player-friendly monetization that is fully replayable.
- Lower ad revenue than aggressive placement. A 'remove ads' in-app purchase may follow.

## Alternatives considered
Premium paid app; in-app purchases for currency (pay-to-win risk).
