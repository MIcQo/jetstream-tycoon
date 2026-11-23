import { describe, expect, it } from "vitest";

import { formatMoney, resolveLocale } from "./locale";

describe("resolveLocale", () => {
  it("prefers an explicit ?locale= parameter", () => {
    expect(resolveLocale("?locale=en-XA", ["de-DE"])).toBe("en-XA");
    expect(resolveLocale("?locale=EN-xa", [])).toBe("en-XA");
  });

  it("uses browser languages, exact then by base language", () => {
    expect(resolveLocale("", ["en"])).toBe("en");
    expect(resolveLocale("", ["fr-FR", "en-GB"])).toBe("en");
    expect(resolveLocale("?locale=xx", ["en-US"])).toBe("en");
  });

  it("falls back to English", () => {
    expect(resolveLocale("", ["de-DE", "fr"])).toBe("en");
    expect(resolveLocale("", [])).toBe("en");
  });
});

describe("formatMoney", () => {
  it("formats cents as whole dollars", () => {
    expect(formatMoney(123_456_78, "en")).toBe("$123,457");
    expect(formatMoney(-5000, "en")).toBe("-$50");
    expect(formatMoney(99, "en-XA")).toBe("$1");
  });
});
