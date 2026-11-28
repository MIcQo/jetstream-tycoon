import { decode } from "@msgpack/msgpack";

/** An entity as the simulation sends it (fields as in sim/state, JSON names). */
export type Entity = Record<string, unknown>;

/** A plane in the air, for map interpolation. */
export interface Flight {
  planeId: string;
  from: string;
  to: string;
  departTick: number;
  arriveTick: number;
}

/** The MessagePack envelope from sim.snapshot() / sim.delta() (sim/bridge/delta.go). */
export interface WireDelta {
  full: boolean;
  tick: number;
  changed: Record<string, Record<string, Entity>>;
  deleted: Record<string, string[]>;
  flights: Flight[];
}

/** The UI's copy of the simulation state: collection → ID → entity. */
export interface World {
  seq: number;
  tick: number;
  collections: Record<string, Record<string, Entity>>;
  /** Flights by plane, updated from every message. */
  flights: Record<string, Flight>;
}

export type WorldResult = { ok: true; world: World } | { ok: false; reason: "sequence_gap" | "not_initialised" };

export function decodeDelta(buf: Uint8Array): WireDelta {
  const d = decode(buf) as Partial<WireDelta>;
  return {
    full: d.full === true,
    tick: d.tick ?? 0,
    changed: d.changed ?? {},
    deleted: d.deleted ?? {},
    flights: d.flights ?? [],
  };
}

function merge(base: World, seq: number, d: WireDelta): World {
  const collections = { ...base.collections };
  for (const [name, items] of Object.entries(d.changed)) {
    collections[name] = { ...collections[name], ...items };
  }
  for (const [name, ids] of Object.entries(d.deleted)) {
    const next = { ...collections[name] };
    for (const id of ids) {
      delete next[id];
    }
    collections[name] = next;
  }
  const flights = { ...base.flights };
  for (const id of d.deleted.planes ?? []) {
    delete flights[id];
  }
  for (const [id, plane] of Object.entries(d.changed.planes ?? {})) {
    if (plane.state !== "flying") {
      delete flights[id];
    }
  }
  for (const f of d.flights) {
    flights[f.planeId] = f;
  }
  return { seq, tick: d.tick, collections, flights };
}

const EMPTY: World = { seq: 0, tick: 0, collections: {}, flights: {} };

/**
 * Applies an `init` or `update` message to the UI world. A full snapshot
 * replaces it; a delta must follow the previous message's seq, otherwise the UI
 * asks the Worker for a resync (TECH_DESIGN §3).
 */
export function applyWorld(world: World | undefined, msg: { kind: "init" | "update"; seq: number; buf: Uint8Array }): WorldResult {
  const d = decodeDelta(msg.buf);
  if (msg.kind === "init" || d.full) {
    return { ok: true, world: merge(EMPTY, msg.seq, d) };
  }
  if (world === undefined) {
    return { ok: false, reason: "not_initialised" };
  }
  if (msg.seq !== world.seq + 1) {
    return { ok: false, reason: "sequence_gap" };
  }
  return { ok: true, world: merge(world, msg.seq, d) };
}
