import { type SimBridge, SimError } from "./bridge";
import { TickClock } from "./clock";
import type { FromWorker, SimView, ToWorker } from "./protocol";
import { Recent } from "./recent";

/** How many recent commands are attached to an error report (TECH_DESIGN §20). */
export const RECENT_COMMANDS = 200;

/** Error keys that describe a player mistake rather than a bug (not sent to Sentry). */
const USER_ERROR = /^errors\.(newgame|cmd)\.[a-z_]+$/;

export interface Controller {
  handle(msg: ToWorker): void;
  /** Called on a timer (~4×/s); advances due ticks and posts one update. */
  pulse(nowMs: number): void;
}

/** Owns the simulation inside the Worker and numbers every message (seq). */
export function createController(
  bridge: SimBridge,
  post: (msg: FromWorker) => void,
  clock: TickClock = new TickClock(),
): Controller {
  let seq = 0;
  let seed: number | undefined;
  let view: SimView | undefined;
  const recent = new Recent<string>(RECENT_COMMANDS);

  // Any failure inside the simulation is reported to the UI, with the context
  // needed to reproduce it, instead of throwing out of a message handler or timer.
  const guarded =
    <A extends unknown[]>(fn: (...args: A) => void) =>
    (...args: A) => {
      try {
        fn(...args);
      } catch (err) {
        // Validation errors from sim.wasm carry their own i18n key (e.g. errors.newgame.*).
        if (err instanceof SimError && USER_ERROR.test(err.message)) {
          post({ kind: "error", errorKey: err.message });
          return;
        }
        post({
          kind: "error",
          errorKey: "errors.sim.internal",
          report: {
            message: err instanceof Error ? err.message : String(err),
            seed,
            tick: view?.tick,
            recentCommands: recent.toArray(),
          },
        });
      }
    };

  const sendCommand = (cmd: { type: string } & Record<string, unknown>) => {
    recent.push(JSON.stringify({ atTick: view?.tick, ...cmd }));
    return bridge.command(cmd);
  };

  const controller: Controller = {
    handle(msg) {
      if (msg.kind === "start") {
        seed = msg.seed;
        view = bridge.newGame({
          seed: msg.seed,
          airlineName: msg.airlineName,
          homeCountry: msg.homeCountry,
          difficulty: msg.difficulty,
        });
        seq += 1;
        post({ kind: "init", seq, view });
        return;
      }
      if (view === undefined) {
        post({ kind: "error", errorKey: "errors.sim.not_started" });
        return;
      }
      const res = sendCommand({ type: "setSpeed", speed: msg.speed });
      if (!res.ok) {
        post({ kind: "error", errorKey: res.errorKey ?? "errors.cmd.unknown" });
      }
    },
    pulse(nowMs) {
      if (view === undefined) {
        return;
      }
      // advance(0) still applies queued commands, so a paused game reacts to the player.
      const res = bridge.advance(clock.due(nowMs, view.speed));
      view = res.view;
      seq += 1;
      post({ kind: "update", seq, view, notifications: res.notifications });
      if (res.reports.length > 0) {
        post({ kind: "simReports", reports: res.reports });
      }
    },
  };
  return { handle: guarded(controller.handle), pulse: guarded(controller.pulse) };
}
