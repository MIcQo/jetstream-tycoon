import { describe, expect, it } from "vitest";
import { applyDelta, type DeltaEnvelope, type UIStateView } from "./delta";

describe("Worker -> UI delta synchronization stream (integration)", () => {
  it("processes a multi-tick stream and recovers on sequence gap", () => {
    let state: UIStateView<{ cashCents: number }> = {
      seq: 0,
      tick: 0,
      entities: { P: { cashCents: 10_000_00 } },
    };

    const stream: DeltaEnvelope<{ cashCents: number }>[] = [
      {
        seq: 1,
        tick: 1,
        changed: { P: { cashCents: 12_000_00 }, A1: { cashCents: 9_500_00 } },
        deleted: [],
      },
      {
        seq: 2,
        tick: 2,
        changed: { P: { cashCents: 15_000_00 } },
        deleted: ["A1"],
      },
    ];

    for (const msg of stream) {
      const res = applyDelta(state, msg);
      expect(res.ok).toBe(true);
      if (res.ok) {
        state = res.state;
      }
    }

    expect(state).toEqual({
      seq: 2,
      tick: 2,
      entities: { P: { cashCents: 15_000_00 } },
    });

    const gapResult = applyDelta(state, {
      seq: 5,
      tick: 5,
      changed: {},
      deleted: [],
    });
    expect(gapResult.ok).toBe(false);
  });
});
