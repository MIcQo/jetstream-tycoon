import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { applyDelta, type UIStateView } from "./delta";

describe("applyDelta fuzz / property tests", () => {
  it("preserves non-deleted changed entities and removes all tombstoned keys", () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: 10000 }),
        fc.integer({ min: 0, max: 100000 }),
        fc.dictionary(fc.string({ minLength: 1, maxLength: 6 }), fc.integer()),
        fc.dictionary(fc.string({ minLength: 1, maxLength: 6 }), fc.integer()),
        fc.array(fc.string({ minLength: 1, maxLength: 6 }), { maxLength: 10 }),
        (seq, tick, initialEntities, changed, deleted) => {
          const current: UIStateView<number> = {
            seq,
            tick,
            entities: initialEntities,
          };

          const result = applyDelta(current, {
            seq: seq + 1,
            tick: tick + 1,
            changed,
            deleted,
          });

          expect(result.ok).toBe(true);
          if (!result.ok) return;

          const deletedSet = new Set(deleted);
          for (const delId of deletedSet) {
            expect(delId in result.state.entities).toBe(false);
          }
          for (const [k, v] of Object.entries(changed)) {
            if (!deletedSet.has(k)) {
              expect(result.state.entities[k]).toBe(v);
            }
          }
        },
      ),
      { numRuns: 200 },
    );
  });
});
