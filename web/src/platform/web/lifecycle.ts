import { Listeners } from "../listeners";
import type { Lifecycle } from "../types";

/** The part of `document` the web lifecycle needs (injectable for tests). */
export interface VisibilitySource extends EventTarget {
  readonly visibilityState: DocumentVisibilityState;
}

/** Browser lifecycle based on the Page Visibility API (tab hidden = background). */
export function createWebLifecycle(doc: VisibilitySource): Lifecycle {
  const background = new Listeners();
  const foreground = new Listeners();
  doc.addEventListener("visibilitychange", () => {
    if (doc.visibilityState === "hidden") {
      background.emit();
    } else {
      foreground.emit();
    }
  });
  return {
    onBackground: (cb) => background.add(cb),
    onForeground: (cb) => foreground.add(cb),
  };
}
