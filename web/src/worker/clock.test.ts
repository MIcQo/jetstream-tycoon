import { describe, expect, it } from "vitest";

import { MAX_TICKS_PER_PULSE, MS_PER_TICK_AT_1X, TickClock } from "./clock";

describe("TickClock", () => {
  it("returns no ticks on the first call (no elapsed time yet)", () => {
    expect(new TickClock().due(1000, 1)).toBe(0);
  });

  it("converts elapsed time into whole ticks and carries the remainder", () => {
    const c = new TickClock();
    c.due(0, 1);
    expect(c.due(MS_PER_TICK_AT_1X - 1, 1)).toBe(0);
    expect(c.due(MS_PER_TICK_AT_1X, 1)).toBe(1); // carry 2499 + 1 = 2500
    expect(c.due(MS_PER_TICK_AT_1X * 3, 1)).toBe(2);
  });

  it("scales with speed", () => {
    const c = new TickClock();
    c.due(0, 4);
    expect(c.due(MS_PER_TICK_AT_1X, 4)).toBe(4);
    expect(c.due(MS_PER_TICK_AT_1X * 2, 2)).toBe(2);
  });

  it("runs nothing while paused and drops the carry", () => {
    const c = new TickClock();
    c.due(0, 1);
    c.due(MS_PER_TICK_AT_1X - 100, 1); // carry 2400
    expect(c.due(MS_PER_TICK_AT_1X * 10, 0)).toBe(0);
    expect(c.due(MS_PER_TICK_AT_1X * 10 + 200, 1)).toBe(0); // carry was reset, 200 ms is not a tick
  });

  it("caps catch-up after a long pause in the background", () => {
    const c = new TickClock();
    c.due(0, 1);
    expect(c.due(MS_PER_TICK_AT_1X * 1000, 1)).toBe(MAX_TICKS_PER_PULSE);
    expect(c.due(MS_PER_TICK_AT_1X * 1000 + 10, 1)).toBe(0); // the excess is dropped
  });

  it("returns exactly the cap when exactly the cap is due", () => {
    const c = new TickClock();
    c.due(0, 1);
    expect(c.due(MS_PER_TICK_AT_1X * MAX_TICKS_PER_PULSE, 1)).toBe(MAX_TICKS_PER_PULSE);
    expect(c.due(MS_PER_TICK_AT_1X * (MAX_TICKS_PER_PULSE + 1), 1)).toBe(1);
  });

  it("ignores clocks that go backwards", () => {
    const c = new TickClock();
    c.due(5000, 1);
    expect(c.due(1000, 1)).toBe(0);
    expect(c.due(1000 + MS_PER_TICK_AT_1X, 1)).toBe(1);
  });
});
