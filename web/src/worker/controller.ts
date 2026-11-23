import type { SimBridge } from "./bridge";
import { TickClock } from "./clock";
import type { FromWorker, SimView, ToWorker } from "./protocol";

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
  let view: SimView | undefined;

  // Any failure inside the simulation is reported to the UI instead of throwing
  // out of a message handler or timer.
  const guarded =
    <A extends unknown[]>(fn: (...args: A) => void) =>
    (...args: A) => {
      try {
        fn(...args);
      } catch {
        post({ kind: "error", errorKey: "errors.sim.internal" });
      }
    };

  const controller: Controller = {
    handle(msg) {
      if (msg.kind === "start") {
        view = bridge.newGame(msg.seed);
        seq += 1;
        post({ kind: "init", seq, view });
        return;
      }
      if (view === undefined) {
        post({ kind: "error", errorKey: "errors.sim.not_started" });
        return;
      }
      const res = bridge.command({ type: "setSpeed", speed: msg.speed });
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
    },
  };
  return { handle: guarded(controller.handle), pulse: guarded(controller.pulse) };
}
