// Messages between the UI thread and the simulation Worker (TECH_DESIGN §3).
// `init` and `update` carry a small JSON view plus a MessagePack buffer of entity
// state (a full snapshot or a delta, see src/world.ts), transferred without a copy.

export type Speed = 0 | 1 | 2 | 4;

export interface SimView {
  tick: number;
  day: number;
  speed: Speed;
  cashCents: number;
  airlineName?: string;
  homeCountry?: string;
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

export type Difficulty = "easy" | "normal" | "hard";

/** The player's choices for a new game (sim.NewGameParams). */
export interface NewGameParams {
  seed: number;
  airlineName: string;
  homeCountry: string;
  difficulty: Difficulty;
}

export type ToWorker =
  | ({ kind: "start" } & NewGameParams)
  | { kind: "setSpeed"; speed: Speed }
  /** The UI saw a sequence gap and needs a fresh full snapshot. */
  | { kind: "resync" };

export type FromWorker =
  | { kind: "init"; seq: number; view: SimView; buf: Uint8Array }
  | { kind: "update"; seq: number; view: SimView; notifications: SimNotification[]; buf: Uint8Array }
  | { kind: "error"; errorKey: string; report?: SimErrorReport }
  | { kind: "simReports"; reports: SimReport[] };
