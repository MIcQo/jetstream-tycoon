import { encode } from "@msgpack/msgpack";
import { describe, expect, it } from "vitest";

import { applyWorld, decodeDelta, type World } from "./world";

const flight = { planeId: "pl1", from: "LHR", to: "CDG", departTick: 1, arriveTick: 3 };

const snapshot = encode({
  full: true,
  tick: 1,
  changed: {
    airlines: { P: { id: "P", cashCents: 100 } },
    planes: { pl1: { id: "pl1", state: "flying" }, pl2: { id: "pl2", state: "idle" } },
  },
  deleted: {},
  flights: [flight],
});

function init(): World {
  const res = applyWorld(undefined, { kind: "init", seq: 1, buf: snapshot });
  if (!res.ok) {
    throw new Error(res.reason);
  }
  return res.world;
}

describe("applyWorld", () => {
  it("builds the world from a snapshot", () => {
    const w = init();
    expect(w.seq).toBe(1);
    expect(w.tick).toBe(1);
    expect(Object.keys(w.collections.planes ?? {})).toEqual(["pl1", "pl2"]);
    expect(w.flights).toEqual({ pl1: flight });
  });

  it("merges changed entities, tombstones and landed flights", () => {
    const w = init();
    const buf = encode({
      full: false,
      tick: 2,
      changed: { airlines: { P: { id: "P", cashCents: 250 } }, planes: { pl1: { id: "pl1", state: "idle" } } },
      deleted: { planes: ["pl2"] },
      flights: [],
    });
    const res = applyWorld(w, { kind: "update", seq: 2, buf });
    expect(res.ok).toBe(true);
    if (!res.ok) return;
    expect(res.world.tick).toBe(2);
    expect(res.world.collections.airlines?.P).toEqual({ id: "P", cashCents: 250 });
    expect(Object.keys(res.world.collections.planes ?? {})).toEqual(["pl1"]);
    expect(res.world.flights).toEqual({});
    // The previous world is not mutated.
    expect(w.collections.airlines?.P).toEqual({ id: "P", cashCents: 100 });
    expect(Object.keys(w.collections.planes ?? {})).toEqual(["pl1", "pl2"]);
  });

  it("drops flights of deleted planes and adds new ones", () => {
    const w = init();
    const next = { ...flight, planeId: "pl3" };
    const buf = encode({ full: false, tick: 2, changed: { planes: { pl3: { id: "pl3", state: "flying" } } }, deleted: { planes: ["pl1"] }, flights: [next] });
    const res = applyWorld(w, { kind: "update", seq: 2, buf });
    expect(res.ok && res.world.flights).toEqual({ pl3: next });
  });

  it("asks for a resync on a sequence gap or before init", () => {
    const empty = encode({ full: false, tick: 2, changed: {}, deleted: {}, flights: [] });
    expect(applyWorld(init(), { kind: "update", seq: 3, buf: empty })).toEqual({ ok: false, reason: "sequence_gap" });
    expect(applyWorld(undefined, { kind: "update", seq: 2, buf: empty })).toEqual({ ok: false, reason: "not_initialised" });
  });

  it("treats a full delta as a snapshot", () => {
    const res = applyWorld(undefined, { kind: "update", seq: 9, buf: snapshot });
    expect(res.ok && res.world.seq).toBe(9);
  });
});

describe("decodeDelta", () => {
  it("fills missing fields", () => {
    expect(decodeDelta(encode({}))).toEqual({ full: false, tick: 0, changed: {}, deleted: {}, flights: [] });
  });
});
