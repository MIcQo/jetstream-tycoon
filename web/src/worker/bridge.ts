import type { NewGameParams, SimNotification, SimReport, SimView } from "./protocol";

/** The JSON string API that sim.wasm registers as `globalThis.jetstreamSim` (M0.6). */
export interface RawSimApi {
  newGame(requestJson: string): string;
  advance(ticks: number): string;
  command(commandJson: string): string;
  /** gzip(MessagePack) save bytes, or a `{"error"}` JSON string. */
  save(): Uint8Array | string;
  /** Replaces the running game with a save; returns the view JSON or `{"error"}`. */
  load(data: Uint8Array): string;
}

export interface AdvanceResult {
  view: SimView;
  notifications: SimNotification[];
  reports: SimReport[];
}

export interface CommandResult {
  ok: boolean;
  errorKey?: string;
}

/** Typed simulation API used by the Worker controller. */
export interface SimBridge {
  newGame(params: NewGameParams): SimView;
  advance(ticks: number): AdvanceResult;
  command(cmd: { type: string } & Record<string, unknown>): CommandResult;
  save(): Uint8Array;
  load(data: Uint8Array): SimView;
}

/** Thrown when sim.wasm answers `{"error": "..."}` (bad request, or a recovered panic). */
export class SimError extends Error {
  override name = "SimError";
}

function parse<T>(json: string): T {
  const value = JSON.parse(json) as T | { error: string };
  if (typeof value === "object" && value !== null && "error" in value) {
    throw new SimError(value.error);
  }
  return value as T;
}

/** Wraps the raw JSON API with parsing and typing. */
export function jsonBridge(raw: RawSimApi): SimBridge {
  return {
    newGame: (params) => parse<SimView>(raw.newGame(JSON.stringify(params))),
    advance: (ticks) => {
      const res = parse<Partial<AdvanceResult> & { view: SimView }>(raw.advance(ticks));
      return { view: res.view, notifications: res.notifications ?? [], reports: res.reports ?? [] };
    },
    command: (cmd) => parse<CommandResult>(raw.command(JSON.stringify(cmd))),
    save: () => {
      const out = raw.save();
      if (typeof out === "string") {
        parse<never>(out);
        throw new SimError("save: unexpected response");
      }
      return out;
    },
    load: (data) => parse<SimView>(raw.load(data)),
  };
}
