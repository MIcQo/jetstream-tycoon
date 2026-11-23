---
name: add-adr
description: Write a new Architecture Decision Record when making or changing a significant technical decision.
---

# Add an ADR

1. Copy `docs/adr/template.md` to `docs/adr/NNNN-<kebab-title>.md`, using the next free number.
2. Fill in Context, Decision, Consequences and Alternatives considered. Be concrete: name the components and numbers.
3. Status: `Proposed`, then `Accepted` once agreed. If it replaces an older ADR, set the old one's status to `Superseded by ADR-NNNN` (the only edit allowed to an old ADR).
4. Add a row to `docs/adr/README.md`.
5. Update `docs/TECH_DESIGN.md`, `docs/DIAGRAMS.md` and `CLAUDE.md` (golden rules) if they're affected.
