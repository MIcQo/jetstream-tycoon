import type { AppPlugin } from "@capacitor/app";
import { describe, expect, it, vi } from "vitest";

import { createCapacitorLifecycle } from "./capacitor/lifecycle";
import { toPlatformKind } from "./index";
import { Listeners } from "./listeners";
import { createWebLifecycle, type VisibilitySource } from "./web/lifecycle";
import { webAds, webHaptics } from "./web/stubs";

class FakeDocument extends EventTarget implements VisibilitySource {
  visibilityState: DocumentVisibilityState = "visible";
  set(state: DocumentVisibilityState) {
    this.visibilityState = state;
    this.dispatchEvent(new Event("visibilitychange"));
  }
}

describe("Listeners", () => {
  it("emits to all callbacks and supports unsubscribe", () => {
    const l = new Listeners();
    const a = vi.fn();
    const b = vi.fn();
    const offA = l.add(a);
    l.add(b);
    l.emit();
    offA();
    l.emit();
    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenCalledTimes(2);
  });

  it("tolerates a callback that unsubscribes during emit", () => {
    const l = new Listeners();
    const b = vi.fn();
    const off = l.add(() => off());
    l.add(b);
    l.emit();
    expect(b).toHaveBeenCalledTimes(1);
  });
});

describe("web lifecycle", () => {
  it("maps tab visibility to background/foreground", () => {
    const doc = new FakeDocument();
    const lc = createWebLifecycle(doc);
    const bg = vi.fn();
    const fg = vi.fn();
    lc.onBackground(bg);
    const offFg = lc.onForeground(fg);

    doc.set("hidden");
    expect(bg).toHaveBeenCalledTimes(1);
    expect(fg).not.toHaveBeenCalled();

    doc.set("visible");
    expect(fg).toHaveBeenCalledTimes(1);

    offFg();
    doc.set("hidden");
    doc.set("visible");
    expect(bg).toHaveBeenCalledTimes(2);
    expect(fg).toHaveBeenCalledTimes(1);
  });
});

describe("capacitor lifecycle", () => {
  it("maps appStateChange to background/foreground", () => {
    let handler: ((s: { isActive: boolean }) => void) | undefined;
    const app = {
      addListener: (event: string, cb: (s: { isActive: boolean }) => void) => {
        expect(event).toBe("appStateChange");
        handler = cb;
        return Promise.resolve({ remove: async () => {} });
      },
    } as unknown as Pick<AppPlugin, "addListener">;
    const lc = createCapacitorLifecycle(app);
    const bg = vi.fn();
    const fg = vi.fn();
    lc.onBackground(bg);
    lc.onForeground(fg);

    handler?.({ isActive: false });
    expect(bg).toHaveBeenCalledTimes(1);
    expect(fg).not.toHaveBeenCalled();
    handler?.({ isActive: true });
    expect(fg).toHaveBeenCalledTimes(1);
  });
});

describe("platform detection and web stubs", () => {
  it("maps Capacitor platform names", () => {
    expect(toPlatformKind("ios")).toBe("ios");
    expect(toPlatformKind("android")).toBe("android");
    expect(toPlatformKind("web")).toBe("web");
    expect(toPlatformKind("electron")).toBe("web");
  });

  it("web has no ads and no-op haptics", async () => {
    expect(webAds.isAvailable("double_revenue")).toBe(false);
    await expect(webAds.showRewarded("double_revenue")).resolves.toEqual({ rewarded: false });
    await expect(webHaptics.impact("light")).resolves.toBeUndefined();
  });
});
