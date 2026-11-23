# ADR-0023: Server-side save revision retention, staged uploads and upload abuse limits

- **Status**: Accepted
- **Date**: 2026-11-23
- **Refines**: ADR-0008, ADR-0022

## Context
ADR-0008 said "a lifecycle rule keeps the last 3 revisions". S3/MinIO lifecycle rules can only expire objects by age, or by noncurrent versions of the same key. They can't keep the highest N revision numbers under a prefix.

Presigned PUT URLs that don't sign the size and checksum also let a client upload far more data than it declared, before the commit step's HEAD check ever runs.

## Decision
- **Staging**:
  - Presigned uploads go to `staging/{save_id}/{uuid}`, and a lifecycle rule expires `staging/` after 24 h. This cleans up abandoned uploads.
  - On commit, the server copies the staged object to `saves/{save_id}/{R}` (server-side copy).
- **Retention**: after committing revision *R*, the server enqueues `saves/{save_id}/{R−3}` (and anything lower) in a `blob_gc` table. A background worker deletes those objects, so the last 3 committed revisions are kept.
- **Upload limits**:
  - `size ≤ maxSaveSizeBytes` (remote config, default 4 MB), otherwise 413.
  - The presigned `PutObject` **signs `Content-Length` and `x-amz-checksum-sha256`**, so storage rejects any upload that doesn't match.
  - URLs expire after 5 minutes, and `upload-url` is rate-limited per user (Redis).
- **Quotas**: at most 10 saves per user (`save_quotas.max_saves`), otherwise 429 `quota.saves_exceeded`.

## Consequences
- Storage per user is bounded and predictable, with abuse stopped at the storage edge.
- Each commit costs one extra server-side copy plus a GC queue to operate (with a `blob_gc` backlog metric).
- Clients must send the signed headers exactly as declared.

## Alternatives considered
- **S3 bucket versioning with a single key per save + `NoncurrentVersionExpiration`** (keeping 3 newer noncurrent versions): workable, but it couples us to versioning semantics that differ between providers and MinIO setups, and it complicates per-revision presigned downloads.
- **Proxying uploads through the API**: full control, but it puts save bytes back on the API (rejected in ADR-0008).
