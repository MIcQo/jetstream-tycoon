import { describe, expect, it } from "vitest";

import { jsonBridge, type RawSimApi, SimError } from "./bridge";
import type { NewGameParams } from "./protocol";

const game = (seed: number): NewGameParams => ({ seed, airlineName: "A", homeCountry: "GB", difficulty: "normal" });

const view = { tick: 1, day: 0, speed: 1, cashCents: 500 };

function fakeRaw(calls: string[]): RawSimApi {
  return {
    newGame: (req) => {
      calls.push(`newGame ${req}`);
      return JSON.stringify(view);
    },
    advance: (ticks) => {
      calls.push(`advance ${ticks}`);
      return ticks > 0
        ? JSON.stringify({
            view,
            notifications: [{ key: "notifications.day.started", params: { day: "1" } }],
            reports: [{ kind: "nonFinite", context: "c", detail: "NaN", tick: 1 }],
          })
        : JSON.stringify({ view });
    },
    command: (cmd) => {
      calls.push(`command ${cmd}`);
      return JSON.stringify({ ok: false, errorKey: "errors.cmd.invalid_speed" });
    },
    save: () => new Uint8Array([1, 2, 3]),
    load: (data) => {
      calls.push(`load ${data.length}`);
      return JSON.stringify(view);
    },
  };
}

describe("jsonBridge", () => {
  it("serialises requests and parses responses", () => {
    const calls: string[] = [];
    const b = jsonBridge(fakeRaw(calls));
    expect(b.newGame(game(7))).toEqual(view);
    const adv = b.advance(3);
    expect(adv.notifications).toEqual([{ key: "notifications.day.started", params: { day: "1" } }]);
    expect(adv.reports).toEqual([{ kind: "nonFinite", context: "c", detail: "NaN", tick: 1 }]);
    expect(b.advance(0)).toEqual({ view, notifications: [], reports: [] });
    expect(b.command({ type: "setSpeed", speed: 3 })).toEqual({ ok: false, errorKey: "errors.cmd.invalid_speed" });
    expect(b.save()).toEqual(new Uint8Array([1, 2, 3]));
    expect(b.load(new Uint8Array([9, 9]))).toEqual(view);
    expect(calls).toEqual([
      'newGame {"seed":7,"airlineName":"A","homeCountry":"GB","difficulty":"normal"}',
      "advance 3",
      "advance 0",
      'command {"type":"setSpeed","speed":3}',
      "load 2",
    ]);
  });
});

describe("jsonBridge errors", () => {
  it("throws SimError for {error} responses", () => {
    const raw: RawSimApi = {
      newGame: () => JSON.stringify({ error: "newGame request: bad" }),
      advance: () => JSON.stringify({ error: "errors.sim.not_started" }),
      command: () => JSON.stringify({ ok: true }),
      save: () => JSON.stringify({ error: "errors.sim.not_started" }),
      load: () => JSON.stringify({ error: "save: not a save" }),
    };
    const b = jsonBridge(raw);
    expect(() => b.newGame(game(1))).toThrow(SimError);
    expect(() => b.advance(1)).toThrow("errors.sim.not_started");
    expect(b.command({ type: "setSpeed", speed: 1 })).toEqual({ ok: true });
    expect(() => b.save()).toThrow("errors.sim.not_started");
    expect(() => b.load(new Uint8Array())).toThrow("save: not a save");
  });

  it("rejects a save response that is neither bytes nor an error", () => {
    const raw: RawSimApi = { newGame: () => "{}", advance: () => "{}", command: () => "{}", save: () => "{}", load: () => "{}" };
    expect(() => jsonBridge(raw).save()).toThrow("save: unexpected response");
  });

  it("does not treat JSON null or primitives as errors", () => {
    const raw: RawSimApi = { newGame: () => "null", advance: () => "1", command: () => '"x"', save: () => new Uint8Array(), load: () => "null" };
    const b = jsonBridge(raw);
    expect(b.newGame(game(1))).toBeNull();
    expect(b.command({ type: "t" })).toBe("x");
  });
});
