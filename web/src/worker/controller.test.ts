import { describe, expect, it } from "vitest";

import { type AdvanceResult, type CommandResult, type SimBridge, SimError } from "./bridge";
import { MS_PER_TICK_AT_1X, TickClock } from "./clock";
import { createController, RECENT_COMMANDS } from "./controller";
import type { FromWorker, SimView, ToWorker } from "./protocol";

const start = (seed: number): ToWorker => ({
  kind: "start",
  seed,
  airlineName: "Test Air",
  homeCountry: "GB",
  difficulty: "normal",
});

function fakeBridge(commandResult: CommandResult = { ok: true }) {
  const advanced: number[] = [];
  const commands: unknown[] = [];
  let view: SimView = { tick: 0, day: 0, speed: 1, cashCents: 0 };
  const bridge: SimBridge = {
    newGame: (params) => {
      view = { ...view, cashCents: params.seed };
      return view;
    },
    advance: (ticks): AdvanceResult => {
      advanced.push(ticks);
      view = { ...view, tick: view.tick + ticks };
      return { view, notifications: [], reports: [] };
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

    c.handle(start(99));
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
    c.handle(start(1));
    c.handle({ kind: "setSpeed", speed: 4 });
    expect(ok.commands).toEqual([{ type: "setSpeed", speed: 4 }]);
    expect(posted.map((m) => m.kind)).toEqual(["init"]);

    const rejected = fakeBridge({ ok: false, errorKey: "errors.cmd.invalid_speed" });
    const posted2: FromWorker[] = [];
    const c2 = createController(rejected.bridge, (m) => posted2.push(m));
    c2.handle(start(1));
    c2.handle({ kind: "setSpeed", speed: 2 });
    expect(posted2.at(-1)).toEqual({ kind: "error", errorKey: "errors.cmd.invalid_speed" });

    const noKey = fakeBridge({ ok: false });
    const posted3: FromWorker[] = [];
    const c3 = createController(noKey.bridge, (m) => posted3.push(m));
    c3.handle(start(1));
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
    expect(() => c.handle(start(1))).not.toThrow();
    expect(posted).toEqual([
      {
        kind: "error",
        errorKey: "errors.sim.internal",
        report: { message: "boom", seed: 1, tick: undefined, recentCommands: [] },
      },
    ]);

    const ok = createController({ ...bridge, advance: failing.advance }, (m) => posted.push(m));
    ok.handle(start(7));
    ok.handle({ kind: "setSpeed", speed: 2 });
    expect(() => ok.pulse(10)).not.toThrow();
    expect(posted.at(-1)).toEqual({
      kind: "error",
      errorKey: "errors.sim.internal",
      report: {
        message: "boom",
        seed: 7,
        tick: 0,
        recentCommands: ['{"atTick":0,"type":"setSpeed","speed":2}'],
      },
    });
  });

  it("reports non-Error throws as strings", () => {
    const { bridge } = fakeBridge();
    const posted: FromWorker[] = [];
    const c = createController(
      {
        ...bridge,
        newGame: () => {
          throw "plain";
        },
      },
      (m) => posted.push(m),
    );
    c.handle(start(1));
    expect(posted[0]?.kind === "error" && posted[0].report?.message).toBe("plain");
  });

  it("keeps only the last RECENT_COMMANDS commands", () => {
    const { bridge } = fakeBridge();
    const posted: FromWorker[] = [];
    const failingAdvance = {
      ...bridge,
      advance: () => {
        throw new Error("x");
      },
    };
    const c = createController(failingAdvance, (m) => posted.push(m));
    c.handle(start(1));
    for (let i = 0; i < RECENT_COMMANDS + 5; i++) {
      c.handle({ kind: "setSpeed", speed: 1 });
    }
    c.pulse(0);
    const last = posted.at(-1);
    expect(last?.kind === "error" && last.report?.recentCommands.length).toBe(RECENT_COMMANDS);
  });
});

describe("simulation reports", () => {
  it("forwards recovered-problem reports after the update", () => {
    const { bridge } = fakeBridge();
    const report = { kind: "nonFinite", context: "fuel_price", detail: "+Inf", tick: 3 };
    let withReport = true;
    const b = {
      ...bridge,
      advance: (ticks: number) => {
        const r = bridge.advance(ticks);
        const reports = withReport ? [report] : [];
        withReport = false;
        return { ...r, reports };
      },
    };
    const posted: FromWorker[] = [];
    const c = createController(b, (m) => posted.push(m));
    c.handle(start(1));
    c.pulse(0);
    c.pulse(0);
    expect(posted.map((m) => m.kind)).toEqual(["init", "update", "simReports", "update"]);
    expect(posted[2]).toEqual({ kind: "simReports", reports: [report] });
  });
});

describe("validation errors from sim.wasm", () => {
  it("passes player-facing error keys through without a crash report", () => {
    const { bridge } = fakeBridge();
    const posted: FromWorker[] = [];
    const c = createController(
      {
        ...bridge,
        newGame: () => {
          throw new SimError("errors.newgame.home_country");
        },
      },
      (m) => posted.push(m),
    );
    c.handle(start(1));
    expect(posted).toEqual([{ kind: "error", errorKey: "errors.newgame.home_country" }]);
  });

  it("treats other SimErrors as internal failures", () => {
    const { bridge } = fakeBridge();
    const posted: FromWorker[] = [];
    const c = createController(
      {
        ...bridge,
        newGame: () => {
          throw new SimError("panic: index out of range");
        },
      },
      (m) => posted.push(m),
    );
    c.handle(start(1));
    expect(posted[0]?.kind === "error" && posted[0].errorKey).toBe("errors.sim.internal");
  });
});
