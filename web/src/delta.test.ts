import { describe, expect, it } from "vitest";
import { applyDelta, type UIStateView } from "./delta";

describe("applyDelta", () => {
  it("applies changed entities and tombstones when seq is contiguous", () => {
    const initial: UIStateView<number> = {
      seq: 1,
      tick: 10,
      entities: { a: 100, b: 200 },
    };

    const res = applyDelta(initial, {
      seq: 2,
      tick: 11,
      changed: { b: 250, c: 300 },
      deleted: ["a"],
    });

    expect(res).toEqual({
      ok: true,
      state: {
        seq: 2,
        tick: 11,
        entities: { b: 250, c: 300 },
      },
    });
    expect(initial.entities).toEqual({ a: 100, b: 200 });
  });

  it("detects a sequence gap when delta.seq !== current.seq + 1", () => {
    const initial: UIStateView<number> = {
      seq: 5,
      tick: 20,
      entities: { a: 1 },
    };

    const res = applyDelta(initial, {
      seq: 7,
      tick: 22,
      changed: {},
      deleted: [],
    });

    expect(res).toEqual({
      ok: false,
      reason: "sequence_gap",
      expectedSeq: 6,
      actualSeq: 7,
    });
  });
});
