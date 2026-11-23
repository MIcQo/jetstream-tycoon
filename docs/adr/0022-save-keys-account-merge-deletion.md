# ADR-0022: Per-save object keys, account merge on sign-in, and outbox-based account deletion

- **Status**: Accepted
- **Date**: 2026-11-23
- **Refines**: ADR-0008, ADR-0010

## Context
- A guest (U1) can sign in with an identity that already belongs to another user (U2). Failing on the `identities` primary key would leave the player stuck.
- Object keys under `saves/{user}/...` would have to be copied when accounts merge.
- Account deletion must reach Postgres, object storage, ClickHouse and Sentry reliably.
- The first upload of a new save (`baseRevision = 0`) was not specified.

## Decision
- **Object keys are per save**: `saves/{save_id}/{revision}`. Save IDs are client-generated UUIDv7.
- **First upload**: `INSERT ... ON CONFLICT (id) DO NOTHING`. Identical retries are idempotent (200); different content → 409; another user's ID → 403.
- **Merge on sign-in**: in one transaction, move U1's saves, devices and tokens to U2; set `users.merged_into` and record `user_aliases`; emit `account_merged`. ClickHouse resolves aliases through a dictionary. No save is overwritten; the player chooses which airline to continue.
- **Deletion**: `DELETE /account` writes a `deletion_jobs` outbox row. A worker deletes the objects of every save of the user and its aliases, the ClickHouse rows, the Sentry user data and finally the Postgres rows, tracking each step and retrying until all are done.

## Consequences
- Merges are cheap (no object copying), and deletion is reliable and auditable.
- Deleting objects needs the list of saves (no single prefix delete), which the outbox job handles.
- Analytics queries must resolve aliases for accurate retention.

## Alternatives considered
- Reject linking when the identity is already used: bad user experience.
- Keep `saves/{user}/...` keys and copy objects on merge: slow, and fails partway on large accounts.
- Synchronous deletion: request timeouts and partial deletes.
