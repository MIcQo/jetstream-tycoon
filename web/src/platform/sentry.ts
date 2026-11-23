import { type BuildInfo, releaseName } from "../telemetry/release";
import { scrubEvent } from "../telemetry/scrub";
import type { SimErrorReport, SimReport } from "../worker/protocol";
import type { PlatformKind } from "./types";

/** Error reporting used by the UI. A no-op when no DSN is configured. */
export interface ErrorReporter {
  simError(report: SimErrorReport): void;
  simReports(reports: SimReport[]): void;
}

const noop: ErrorReporter = { simError: () => {}, simReports: () => {} };

/**
 * Initialises Sentry (ADR-0014): @sentry/capacitor (with @sentry/browser as its
 * sibling SDK) on iOS/Android for native crashes, @sentry/browser on the web.
 * Personal data is never sent: sendDefaultPii is off and every event is scrubbed.
 * Analytics/crash consent (UMP/ATT) gates this call from M11.
 *
 * The SDKs are loaded lazily, so builds without a DSN don't ship them in the main bundle.
 */
export async function initErrorReporting(kind: PlatformKind, info: BuildInfo): Promise<ErrorReporter> {
  if (!info.sentryDsn) {
    return noop;
  }
  const SentryBrowser = await import("@sentry/browser");
  const options = {
    dsn: info.sentryDsn,
    release: releaseName(info),
    environment: info.environment,
    sendDefaultPii: false,
    beforeSend: scrubEvent,
    beforeSendTransaction: scrubEvent,
  };
  if (kind === "web") {
    SentryBrowser.init(options);
  } else {
    const SentryCapacitor = await import("@sentry/capacitor");
    SentryCapacitor.init(options, SentryBrowser.init);
  }
  return {
    simError: (report) =>
      SentryBrowser.captureException(new Error(report.message), {
        tags: { area: "sim" },
        contexts: { sim: { seed: report.seed, tick: report.tick } },
        extra: { recentCommands: report.recentCommands },
      }),
    simReports: (reports) => {
      for (const r of reports) {
        SentryBrowser.captureMessage(`sim ${r.kind}: ${r.context}`, {
          level: "warning",
          tags: { area: "sim", kind: r.kind },
          contexts: { sim: { tick: r.tick, detail: r.detail } },
        });
      }
    },
  };
}
