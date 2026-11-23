import { describe, expect, it } from "vitest";

import { pseudoCatalog, pseudoLocalize } from "./pseudo";

describe("pseudoLocalize", () => {
  it("accents literal text and brackets the message", () => {
    expect(pseudoLocalize("Load factor")).toBe("⟦Lóád fáctór⟧");
    expect(pseudoLocalize("AEIOU")).toBe("⟦ÁÉÍÓÚ⟧");
  });

  it("keeps simple placeholders intact", () => {
    expect(pseudoLocalize("Speed: {speed}x")).toBe("⟦Spééd: {speed}x⟧");
  });

  it("keeps plural syntax but accents nested message bodies", () => {
    expect(pseudoLocalize("{count, plural, one {# plane} other {# planes}}")).toBe(
      "⟦{count, plural, one {# pláné} other {# plánés}}⟧",
    );
  });

  it("handles a placeholder inside a nested body", () => {
    expect(pseudoLocalize("{n, select, a {at {city}} other {}}")).toBe("⟦{n, select, a {át {city}} other {}}⟧");
  });

  it("maps a whole catalog", () => {
    expect(pseudoCatalog({ "a.b": "hi", "c.d": "{x}" })).toEqual({ "a.b": "⟦hí⟧", "c.d": "⟦{x}⟧" });
  });
});
