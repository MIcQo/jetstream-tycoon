import { describe, expect, it } from "vitest";

import { readBuildInfo, releaseName } from "./release";
import { scrubEvent, stripQuery } from "./scrub";

describe("release", () => {
  it("names releases app+content", () => {
    expect(releaseName({ appVersion: "1.2.3", contentVersion: "c42" })).toBe("jetstream-tycoon@1.2.3+c42");
  });

  it("reads build info with defaults for missing or empty values", () => {
    expect(readBuildInfo({})).toEqual({
      appVersion: "0.0.0",
      contentVersion: "dev",
      environment: "development",
      sentryDsn: undefined,
    });
    expect(
      readBuildInfo({
        VITE_APP_VERSION: "0.1.0",
        VITE_CONTENT_VERSION: "",
        MODE: "production",
        VITE_SENTRY_DSN: "https://k@o.ingest.sentry.io/1",
        DEV: false,
      }),
    ).toEqual({
      appVersion: "0.1.0",
      contentVersion: "dev",
      environment: "production",
      sentryDsn: "https://k@o.ingest.sentry.io/1",
    });
  });
});

describe("scrub", () => {
  it("strips query strings and fragments", () => {
    expect(stripQuery("https://x.io/a/b?token=s#frag")).toBe("https://x.io/a/b");
    expect(stripQuery("https://x.io/a#f")).toBe("https://x.io/a");
    expect(stripQuery("/plain")).toBe("/plain");
  });

  it("removes personal data and keeps the pseudonymous id", () => {
    const ev = scrubEvent({
      request: { url: "https://app/x?code=1", headers: { a: "b" }, cookies: "c", query_string: "q", data: "d" },
      user: { id: "u1", ip_address: "1.2.3.4", email: "a@b.c", username: "bob" },
      breadcrumbs: [{ data: { url: "https://api/x?t=1", method: "GET" } }, { data: { status: 200 } }, {}],
    });
    expect(ev).toEqual({
      request: { url: "https://app/x" },
      user: { id: "u1" },
      breadcrumbs: [{ data: { url: "https://api/x", method: "GET" } }, { data: { status: 200 } }, {}],
    });
  });

  it("handles events without url, id or breadcrumbs", () => {
    expect(scrubEvent({ request: { headers: {} }, user: { email: "a@b.c" } })).toEqual({ request: {}, user: {} });
    expect(scrubEvent({})).toEqual({});
  });
});
