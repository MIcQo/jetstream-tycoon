import type { AppPlugin } from "@capacitor/app";

import { Listeners } from "../listeners";
import type { Lifecycle } from "../types";

/** iOS/Android lifecycle from the Capacitor App plugin's appStateChange event. */
export function createCapacitorLifecycle(app: Pick<AppPlugin, "addListener">): Lifecycle {
  const background = new Listeners();
  const foreground = new Listeners();
  void app.addListener("appStateChange", ({ isActive }) => {
    if (isActive) {
      foreground.emit();
    } else {
      background.emit();
    }
  });
  return {
    onBackground: (cb) => background.add(cb),
    onForeground: (cb) => foreground.add(cb),
  };
}
