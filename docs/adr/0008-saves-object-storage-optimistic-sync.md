# ADR-0008: Save files in object storage, synced with optimistic locking and presigned URLs

- **Status**: Accepted (refined by ADR-0022, ADR-0023)
- **Date**: 2026-11-23

## Context
Saves are written locally first (offline) and synced every few minutes. Storing large binary rows in Postgres that are rewritten often causes TOAST bloat and makes the API carry the save bytes. Single player means conflicts are rare but must never silently lose progress.

## Decision
- Save files are stored in S3-compatible object storage (MinIO locally) at `saves/{user}/{save}/{revision}`. Postgres stores metadata, `revision`, size and sha256.
- Upload: request a presigned URL (checks `baseRevision`), PUT directly to storage, then commit with a conditional `UPDATE ... WHERE revision = baseRevision`.
- On a mismatch the API returns **409** and the client asks the player which copy to keep.
- Nothing is uploaded when the hash hasn't changed. A lifecycle rule keeps the last 3 revisions.

## Consequences
- The API stays light, Postgres stays small, and every conflict is visible to the player.
- Adds an object-storage dependency and a 3-step upload with an orphaned-object cleanup (lifecycle rule).

## Alternatives considered
Save files in Postgres BYTEA (simpler, but bloats at scale); last-writer-wins (silent data loss); CRDT/merge (overkill for single player).
