import { describe, expect, it } from "vitest";

import { initI18n } from "./index";

describe("initI18n", () => {
  it("formats ICU plurals in English", async () => {
    const t = await initI18n("en");
    expect(t("fleet.summary.count", { count: 1 })).toBe("1 plane");
    expect(t("fleet.summary.count", { count: 3 })).toBe("3 planes");
    expect(t("hud.day.current", { day: 4 })).toBe("Day 4");
  });

  it("serves the en-XA pseudo-locale", async () => {
    const t = await initI18n("en-XA");
    expect(t("fleet.summary.count", { count: 3 })).toBe("⟦3 plánés⟧");
  });
});
