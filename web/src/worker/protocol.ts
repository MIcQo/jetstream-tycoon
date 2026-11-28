// Messages between the UI thread and the simulation Worker (TECH_DESIGN §3).
// M0 sends a small full view; M2 switches updates to the delta envelope (see delta.ts).

export type Speed = 0 | 1 | 2 | 4;

export interface SimView {
  tick: number;
  day: number;
  speed: Speed;
  cashCents: number;
  /** Version of the content compiled into sim.wasm (e.g. "c79fa5ed651d9"). */
  contentVersion?: string;
}

export interface SimNotification {
  key: string;
  params?: Record<string, string>;
}

/** A problem the simulation recovered from (e.g. a clamped NaN in a release build). */
export interface SimReport {
  kind: string;
  context: string;
  detail: string;
  tick: number;
}

/**
 * Context attached to a simulation failure. The simulation is deterministic, so
 * the seed plus the recent commands are enough to reproduce most bugs (ADR-0014).
 */
export interface SimErrorReport {
  message: string;
  seed: number | undefined;
  tick: number | undefined;
  recentCommands: string[];
}

export type ToWorker = { kind: "start"; seed: number } | { kind: "setSpeed"; speed: Speed };

export type FromWorker =
  | { kind: "init"; seq: number; view: SimView }
  | { kind: "update"; seq: number; view: SimView; notifications: SimNotification[] }
  | { kind: "error"; errorKey: string; report?: SimErrorReport }
  | { kind: "simReports"; reports: SimReport[] };
