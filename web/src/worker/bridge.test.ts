import { describe, expect, it } from "vitest";

import { jsonBridge, type RawSimApi } from "./bridge";

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
        ? JSON.stringify({ view, notifications: [{ key: "notifications.day.started", params: { day: "1" } }] })
        : JSON.stringify({ view });
    },
    command: (cmd) => {
      calls.push(`command ${cmd}`);
      return JSON.stringify({ ok: false, errorKey: "errors.cmd.invalid_speed" });
    },
  };
}

describe("jsonBridge", () => {
  it("serialises requests and parses responses", () => {
    const calls: string[] = [];
    const b = jsonBridge(fakeRaw(calls));
    expect(b.newGame(7)).toEqual(view);
    expect(b.advance(3).notifications).toEqual([{ key: "notifications.day.started", params: { day: "1" } }]);
    expect(b.advance(0)).toEqual({ view, notifications: [] });
    expect(b.command({ type: "setSpeed", speed: 3 })).toEqual({ ok: false, errorKey: "errors.cmd.invalid_speed" });
    expect(calls).toEqual([
      'newGame {"seed":7}',
      "advance 3",
      "advance 0",
      'command {"type":"setSpeed","speed":3}',
    ]);
  });
});
