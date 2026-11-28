# ADR-0024: gzip instead of zstd for save compression

- **Status**: Accepted
- **Date**: 2026-11-28

## Context
ADR-0009 chose MessagePack + zstd for saves. Building the save format (M2.4) showed the cost of each library in `sim.wasm` (`-trimpath -ldflags="-s -w"`, gzip-9 as the brotli proxy):

| Build | Raw | gzip-9 |
|---|---|---|
| Before saves (content bundle already uses `compress/gzip`) | 4.65 MB | 1.27 MB |
| + MessagePack (`vmihailenco/msgpack/v5`) | 5.15 MB | 1.39 MB |
| + zstd (`klauspost/compress/zstd`) | 5.98 MB | 1.63 MB |

zstd adds about 0.8 MB raw and 0.24 MB compressed on top of MessagePack. The budget is 10 MB raw and 2.5 MB brotli **including `expr`**, which is still to come (about +3.6 MB raw, +0.8 MB compressed). With zstd the budget would be exceeded. `compress/gzip` is already linked for the content bundle, so it costs nothing extra.

## Decision
- Saves are **gzip (level 9) of MessagePack**, behind the `JTSV` header with format version 1 (`sim/save`).
- Everything else in ADR-0009 stands: MessagePack encoding, `schemaVersion` with Go migrators, JSON debug export.
- ADR-0009 is superseded only in its compression choice.

## Consequences
- `sim.wasm` stays at about 6.6 MB raw and 2.0 MB gzip with saves, leaving room for `expr`.
- gzip compresses MessagePack saves somewhat worse and encodes more slowly than zstd. Saves are small (tens to hundreds of KB) and autosave is already deferred off the tick path, so this is acceptable.
- The gzip writer sets no name or timestamp, so encoding stays byte-identical across platforms.
- Switching to zstd later needs a new header format version; old gzip saves must still load.

## Alternatives considered
- **Keep zstd**: breaks the WASM budget once `expr` lands.
- **zstd in JS (Worker side)**: adds a JS dependency and a second codec path; the Go side would emit raw MessagePack. Rejected for complexity.
- **No compression**: saves several times larger for upload and storage.
