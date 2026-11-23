# ADR-0010: Guest-first accounts with Apple/Google/email linking; JWT + rotating refresh tokens

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
Players must be able to start instantly and offline. Cross-device sync needs an identity. iOS requires Sign in with Apple when Google sign-in is offered, and an account deletion feature.

## Decision
- The device creates a local guest identity and registers it as an anonymous user when first online.
- Players can link Apple, Google or an email magic link.
- Access tokens are JWTs (15 min). Refresh tokens rotate and are stored hashed. Tokens are kept in Keychain/Keystore.
- Account deletion removes the user's data in Postgres, object storage and ClickHouse.

## Consequences
- No friction for new players; meets App Store requirements.
- Guest progress is lost if the device is lost before linking, so the UI prompts players to link.

## Alternatives considered
Mandatory sign-in (hurts onboarding); a third-party auth service (cost, vendor lock-in; may be revisited).
