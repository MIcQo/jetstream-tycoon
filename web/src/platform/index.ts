import { App } from "@capacitor/app";
import { Capacitor } from "@capacitor/core";

import { createCapacitorLifecycle } from "./capacitor/lifecycle";
import type { Platform, PlatformKind } from "./types";
import { createWebLifecycle } from "./web/lifecycle";
import { webAds, webHaptics } from "./web/stubs";

export type { Platform, PlatformKind } from "./types";

/** Maps Capacitor's platform name to ours; anything unknown is treated as web. */
export function toPlatformKind(name: string): PlatformKind {
  return name === "ios" || name === "android" ? name : "web";
}

/** Creates the adapters for the platform the app is running on. */
export function createPlatform(): Platform {
  const kind = toPlatformKind(Capacitor.getPlatform());
  const lifecycle = kind === "web" ? createWebLifecycle(document) : createCapacitorLifecycle(App);
  // Native haptics and AdMob arrive with their milestones; until then the web stubs apply.
  return { kind, lifecycle, haptics: webHaptics, ads: webAds };
}
