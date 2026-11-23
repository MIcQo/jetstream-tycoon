// Platform adapter interfaces (ADR-0003, TECH_DESIGN §14). UI code depends only on
// these; web and Capacitor (iOS/Android) provide implementations. Capacitor plugins
// may be imported only inside src/platform (enforced by ESLint).

export type PlatformKind = "web" | "ios" | "android";

/** App lifecycle: the game pauses and autosaves when the app goes to the background. */
export interface Lifecycle {
  /** Registers a callback; returns a function that unregisters it. */
  onBackground(cb: () => void): () => void;
  onForeground(cb: () => void): () => void;
}

export type HapticStyle = "light" | "medium" | "heavy";

export interface Haptics {
  impact(style: HapticStyle): Promise<void>;
}

/** Ads (ADR-0018). Rewards are applied as simulation commands, never directly. */
export interface Ads {
  isAvailable(placement: string): boolean;
  showRewarded(placement: string): Promise<{ rewarded: boolean }>;
}

// Declared now, implemented in later milestones:
// - SaveStore (IndexedDB / Capacitor Filesystem) — M2
// - SecureTokens (Keychain / Keystore), SignIn (Apple / Google / email) — M9
// - LocalNotifications, AdMob — M11

export interface Platform {
  kind: PlatformKind;
  lifecycle: Lifecycle;
  haptics: Haptics;
  ads: Ads;
}
