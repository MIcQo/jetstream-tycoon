import type { Ads, Haptics } from "../types";

/** Browsers have no haptics; impact is a no-op. */
export const webHaptics: Haptics = {
  impact: async () => {},
};

/** No ads on the web in the MVP (ADR-0018), so rewarded buttons stay hidden. */
export const webAds: Ads = {
  isAvailable: () => false,
  showRewarded: async () => ({ rewarded: false }),
};
