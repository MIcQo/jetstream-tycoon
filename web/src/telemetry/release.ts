/** Build information injected by Vite (see vite.config.ts and .env files). */
export interface BuildInfo {
  appVersion: string;
  contentVersion: string;
  environment: string;
  sentryDsn: string | undefined;
}

/**
 * Sentry release name: `jetstream-tycoon@<appVersion>+<contentVersion>`, the same
 * scheme CI uses when uploading source maps (ADR-0014).
 */
export function releaseName(info: Pick<BuildInfo, "appVersion" | "contentVersion">): string {
  return `jetstream-tycoon@${info.appVersion}+${info.contentVersion}`;
}

/** Reads build information from Vite's env object. Empty values count as missing. */
export function readBuildInfo(env: Record<string, string | boolean | undefined>): BuildInfo {
  const str = (key: string): string | undefined => {
    const v = env[key];
    return typeof v === "string" && v !== "" ? v : undefined;
  };
  return {
    appVersion: str("VITE_APP_VERSION") ?? "0.0.0",
    contentVersion: str("VITE_CONTENT_VERSION") ?? "dev",
    environment: str("MODE") ?? "development",
    sentryDsn: str("VITE_SENTRY_DSN"),
  };
}
