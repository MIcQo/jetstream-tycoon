import type { Speed } from "./protocol";

/** One tick is one game hour; a game day lasts 60 s at 1×. */
export const MS_PER_TICK_AT_1X = 2500;

/** Upper bound on ticks run in one pulse, e.g. after the tab was in the background. */
export const MAX_TICKS_PER_PULSE = 24;

/**
 * Converts wall-clock time into whole ticks for the current speed, carrying the
 * remainder to the next pulse so no time is lost.
 */
export class TickClock {
  private carryMs = 0;
  private lastMs: number | undefined;

  /** Returns the number of ticks due since the previous call. */
  due(nowMs: number, speed: Speed): number {
    const elapsed = this.lastMs === undefined ? 0 : Math.max(0, nowMs - this.lastMs);
    this.lastMs = nowMs;
    if (speed === 0) {
      this.carryMs = 0;
      return 0;
    }
    this.carryMs += elapsed * speed;
    const ticks = Math.floor(this.carryMs / MS_PER_TICK_AT_1X);
    this.carryMs -= ticks * MS_PER_TICK_AT_1X;
    if (ticks > MAX_TICKS_PER_PULSE) {
      this.carryMs = 0;
      return MAX_TICKS_PER_PULSE;
    }
    return ticks;
  }
}
