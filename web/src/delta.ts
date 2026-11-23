export interface EntityMap<T> {
  [id: string]: T;
}

export interface UIStateView<T> {
  seq: number;
  tick: number;
  entities: EntityMap<T>;
}

export interface DeltaEnvelope<T> {
  seq: number;
  tick: number;
  changed: EntityMap<T>;
  deleted: string[];
}

export type DeltaApplyResult<T> =
  | { ok: true; state: UIStateView<T> }
  | { ok: false; reason: "sequence_gap"; expectedSeq: number; actualSeq: number };

/**
 * Applies a Worker delta envelope to the UI state view (TECH_DESIGN §3).
 * Detects sequence gaps so the UI can request a fresh `{kind: "init"}` snapshot.
 */
export function applyDelta<T>(
  current: UIStateView<T>,
  delta: DeltaEnvelope<T>,
): DeltaApplyResult<T> {
  const expectedSeq = current.seq + 1;
  if (delta.seq !== expectedSeq) {
    return {
      ok: false,
      reason: "sequence_gap",
      expectedSeq,
      actualSeq: delta.seq,
    };
  }

  const nextEntities: EntityMap<T> = { ...current.entities };
  for (const [id, value] of Object.entries(delta.changed)) {
    nextEntities[id] = value;
  }
  for (const id of delta.deleted) {
    delete nextEntities[id];
  }

  return {
    ok: true,
    state: {
      seq: delta.seq,
      tick: delta.tick,
      entities: nextEntities,
    },
  };
}
