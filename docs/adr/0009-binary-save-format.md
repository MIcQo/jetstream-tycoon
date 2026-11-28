# ADR-0009: MessagePack + zstd as the save format

- **Status**: Superseded by ADR-0024 (compression only: gzip instead of zstd)
- **Date**: 2026-11-23

## Context
Saves are produced inside WASM on phones and uploaded over mobile networks. gzip JSON is large and slow to encode.

## Decision
- Snapshots are encoded as MessagePack and compressed with zstd.
- A JSON export is available for debugging and bug reports.
- Snapshots carry `schemaVersion`, and versioned Go migrators upgrade old saves when they load.

## Consequences
- About 3–5x smaller with faster encode and decode.
- Not human-readable without a tool, which the JSON export covers.

## Alternatives considered
gzip JSON (simple, but larger and slower); protobuf (needs schema files, and the extra rigidity isn't needed).
