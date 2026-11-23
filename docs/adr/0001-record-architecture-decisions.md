# ADR-0001: Record architecture decisions

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
The project will grow over a long time (new mechanics, platforms, live ops). Decisions and the reasons behind them need to stay discoverable for future contributors and agents.

## Decision
We record each significant architecture decision as an ADR in `docs/adr/`, using the format in `template.md` (Nygard style).

- ADRs are numbered sequentially and never deleted.
- A changed decision gets a new ADR that marks the old one `Superseded by ADR-XXXX`.
- `docs/TECH_DESIGN.md` describes the current design and links to the ADRs.

## Consequences
- There is a short history of why the system looks the way it does.
- A small writing overhead per decision.

## Alternatives considered
Wiki pages, which drift from the code and aren't versioned with it; no record at all.
