# ADR-0002: Run the game simulation on the device as deterministic Go compiled to WebAssembly

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
The game must be fully playable offline on web, iOS and Android. The preferred stack is Go. A server-authoritative simulation would need a constant connection. Writing the simulation separately for client and server would duplicate the game logic and let the two versions drift apart.

## Decision
- The simulation is a pure Go package (`sim/`), compiled to WebAssembly and run in a Web Worker on every platform.
- The server never runs the game loop.
- The same Go code runs natively for tests, the balance bot and (later) server-side replay verification.
- The JS bridge exchanges compact messages: commands in, deltas, notifications and analytics out.

## Consequences
- Offline play everywhere, with one codebase for the game rules.
- Cheating is possible locally. This is acceptable for single player, and leaderboards will need replay verification (ADR-0006).
- WASM size (about 2 MB compressed) and load time must be managed (build flags, wasm-opt, CDN, bundled in apps).
- Go WASM is single-threaded, so the simulation must stay efficient (demand only for existing routes).

## Alternatives considered
Server-authoritative simulation (no offline play); TypeScript simulation (gives up Go, and a server replay would need a second implementation); gomobile native libraries (no web support, two bridges).
