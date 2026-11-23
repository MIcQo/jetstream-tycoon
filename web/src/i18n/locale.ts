export const SUPPORTED_LOCALES = ["en", "en-XA"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];

/**
 * Picks the UI locale: an explicit `?locale=` query parameter wins (used for the
 * en-XA pseudo-locale in development and CI), then the browser languages, then English.
 */
export function resolveLocale(search: string, languages: readonly string[]): Locale {
  const requested = new URLSearchParams(search).get("locale");
  for (const candidate of [requested, ...languages]) {
    const match = SUPPORTED_LOCALES.find((l) => l.toLowerCase() === candidate?.toLowerCase());
    if (match) {
      return match;
    }
  }
  for (const lang of languages) {
    const base = SUPPORTED_LOCALES.find((l) => l === lang.split("-")[0]);
    if (base) {
      return base;
    }
  }
  return "en";
}

/** Formats in-game money (int64 cents) for the locale. Pseudo-locale formats as English. */
export function formatMoney(cents: number, locale: Locale): string {
  const intlLocale = locale === "en-XA" ? "en" : locale;
  return new Intl.NumberFormat(intlLocale, {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(cents / 100);
}
