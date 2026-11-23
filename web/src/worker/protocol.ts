// Messages between the UI thread and the simulation Worker (TECH_DESIGN §3).
// M0 sends a small full view; M2 switches updates to the delta envelope (see delta.ts).

export type Speed = 0 | 1 | 2 | 4;

export interface SimView {
  tick: number;
  day: number;
  speed: Speed;
  cashCents: number;
}

export interface SimNotification {
  key: string;
  params?: Record<string, string>;
}

export type ToWorker = { kind: "start"; seed: number } | { kind: "setSpeed"; speed: Speed };

export type FromWorker =
  | { kind: "init"; seq: number; view: SimView }
  | { kind: "update"; seq: number; view: SimView; notifications: SimNotification[] }
  | { kind: "error"; errorKey: string };
