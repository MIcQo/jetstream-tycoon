/**
 * The parts of a Sentry event that can carry personal data. Typed structurally so
 * the scrubber is testable without the SDK.
 */
export interface ScrubbableEvent {
  request?: { url?: string; headers?: unknown; cookies?: unknown; query_string?: unknown; data?: unknown };
  user?: { id?: string | number; ip_address?: string | null; email?: string; username?: string };
  breadcrumbs?: { data?: Record<string, unknown> }[];
}

/** Removes the query string and fragment, which may contain tokens. */
export function stripQuery(url: string): string {
  const cut = url.search(/[?#]/);
  return cut === -1 ? url : url.slice(0, cut);
}

/**
 * Removes personal data before an event leaves the device (golden rule 7): only
 * the pseudonymous user ID is kept, request data is dropped except the URL path,
 * and breadcrumb URLs lose their query strings.
 */
export function scrubEvent<E extends ScrubbableEvent>(event: E): E {
  if (event.request) {
    event.request = event.request.url === undefined ? {} : { url: stripQuery(event.request.url) };
  }
  if (event.user) {
    event.user = event.user.id === undefined ? {} : { id: event.user.id };
  }
  for (const crumb of event.breadcrumbs ?? []) {
    const url = crumb.data?.["url"];
    if (crumb.data && typeof url === "string") {
      crumb.data["url"] = stripQuery(url);
    }
  }
  return event;
}
