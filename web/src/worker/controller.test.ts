import { describe, expect, it } from "vitest";

import type { AdvanceResult, CommandResult, SimBridge } from "./bridge";
import { MS_PER_TICK_AT_1X, TickClock } from "./clock";
import { createController } from "./controller";
import type { FromWorker, SimView } from "./protocol";

function fakeBridge(commandResult: CommandResult = { ok: true }) {
  const advanced: number[] = [];
  const commands: unknown[] = [];
  let view: SimView = { tick: 0, day: 0, speed: 1, cashCents: 0 };
  const bridge: SimBridge = {
    newGame: (seed) => {
      view = { ...view, cashCents: seed };
      return view;
    },
    advance: (ticks): AdvanceResult => {
      advanced.push(ticks);
      view = { ...view, tick: view.tick + ticks };
      return { view, notifications: [] };
    },
    command: (cmd) => {
      commands.push(cmd);
      return commandResult;
    },
  };
  return { bridge, advanced, commands };
}

describe("createController", () => {
  it("starts a game and numbers messages", () => {
    const { bridge, advanced } = fakeBridge();
    const posted: FromWorker[] = [];
    const c = createController(bridge, (m) => posted.push(m));

    c.pulse(0); // not started yet: nothing happens
    expect(posted).toEqual([]);

    c.handle({ kind: "start", seed: 99 });
    c.pulse(0);
    c.pulse(MS_PER_TICK_AT_1X * 2);

    expect(posted.map((m) => m.kind)).toEqual(["init", "update", "update"]);
    expect(posted.map((m) => ("seq" in m ? m.seq : -1))).toEqual([1, 2, 3]);
    expect(advanced).toEqual([0, 2]);
    const last = posted.at(-1);
    expect(last?.kind === "update" && last.view.tick).toBe(2);
  });

  it("sends speed changes as commands and reports rejections", () => {
    const ok = fakeBridge();
    const posted: FromWorker[] = [];
    const c = createController(ok.bridge, (m) => posted.push(m), new TickClock());
    c.handle({ kind: "start", seed: 1 });
    c.handle({ kind: "setSpeed", speed: 4 });
    expect(ok.commands).toEqual([{ type: "setSpeed", speed: 4 }]);
    expect(posted.map((m) => m.kind)).toEqual(["init"]);

    const rejected = fakeBridge({ ok: false, errorKey: "errors.cmd.invalid_speed" });
    const posted2: FromWorker[] = [];
    const c2 = createController(rejected.bridge, (m) => posted2.push(m));
    c2.handle({ kind: "start", seed: 1 });
    c2.handle({ kind: "setSpeed", speed: 2 });
    expect(posted2.at(-1)).toEqual({ kind: "error", errorKey: "errors.cmd.invalid_speed" });

    const noKey = fakeBridge({ ok: false });
    const posted3: FromWorker[] = [];
    const c3 = createController(noKey.bridge, (m) => posted3.push(m));
    c3.handle({ kind: "start", seed: 1 });
    c3.handle({ kind: "setSpeed", speed: 2 });
    expect(posted3.at(-1)).toEqual({ kind: "error", errorKey: "errors.cmd.unknown" });
  });

  it("rejects commands before the game starts", () => {
    const { bridge, commands } = fakeBridge();
    const posted: FromWorker[] = [];
    createController(bridge, (m) => posted.push(m)).handle({ kind: "setSpeed", speed: 2 });
    expect(commands).toEqual([]);
    expect(posted).toEqual([{ kind: "error", errorKey: "errors.sim.not_started" }]);
  });
});

describe("createController failures", () => {
  it("reports simulation errors instead of throwing", () => {
    const { bridge } = fakeBridge();
    const failing = {
      ...bridge,
      advance: () => {
        throw new Error("boom");
      },
      newGame: () => {
        throw new Error("boom");
      },
    };
    const posted: FromWorker[] = [];
    const c = createController(failing, (m) => posted.push(m));
    expect(() => c.handle({ kind: "start", seed: 1 })).not.toThrow();
    expect(posted).toEqual([{ kind: "error", errorKey: "errors.sim.internal" }]);

    const ok = createController({ ...bridge, advance: failing.advance }, (m) => posted.push(m));
    ok.handle({ kind: "start", seed: 1 });
    expect(() => ok.pulse(10)).not.toThrow();
    expect(posted.at(-1)).toEqual({ kind: "error", errorKey: "errors.sim.internal" });
  });
});
