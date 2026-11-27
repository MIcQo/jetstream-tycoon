import { describe, expect, it } from "vitest";

import { Recent } from "./recent";

describe("Recent", () => {
  it("keeps the newest items up to capacity, oldest first", () => {
    const r = new Recent<number>(3);
    expect(r.toArray()).toEqual([]);
    [1, 2, 3].forEach((n) => r.push(n));
    expect(r.toArray()).toEqual([1, 2, 3]);
    r.push(4);
    expect(r.toArray()).toEqual([2, 3, 4]);
  });

  it("returns a copy", () => {
    const r = new Recent<number>(2);
    r.push(1);
    r.toArray().push(99);
    expect(r.toArray()).toEqual([1]);
  });
});
