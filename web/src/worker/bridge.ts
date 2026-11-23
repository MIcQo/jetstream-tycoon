import type { SimNotification, SimView } from "./protocol";

/** The JSON string API that sim.wasm registers as `globalThis.jetstreamSim` (M0.6). */
export interface RawSimApi {
  newGame(requestJson: string): string;
  advance(ticks: number): string;
  command(commandJson: string): string;
}

export interface AdvanceResult {
  view: SimView;
  notifications: SimNotification[];
}

export interface CommandResult {
  ok: boolean;
  errorKey?: string;
}

/** Typed simulation API used by the Worker controller. */
export interface SimBridge {
  newGame(seed: number): SimView;
  advance(ticks: number): AdvanceResult;
  command(cmd: { type: string } & Record<string, unknown>): CommandResult;
}

/** Wraps the raw JSON API with parsing and typing. */
export function jsonBridge(raw: RawSimApi): SimBridge {
  return {
    newGame: (seed) => JSON.parse(raw.newGame(JSON.stringify({ seed }))) as SimView,
    advance: (ticks) => {
      const res = JSON.parse(raw.advance(ticks)) as Partial<AdvanceResult> & { view: SimView };
      return { view: res.view, notifications: res.notifications ?? [] };
    },
    command: (cmd) => JSON.parse(raw.command(JSON.stringify(cmd))) as CommandResult,
  };
}
