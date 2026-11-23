import i18next, { type TFunction } from "i18next";
import ICU from "i18next-icu";

import en from "./en.json";
import type { Locale } from "./locale";
import { pseudoCatalog } from "./pseudo";

/** Initialises i18next with ICU MessageFormat and returns the translate function (ADR-0016). */
export async function initI18n(locale: Locale): Promise<TFunction> {
  return i18next.use(ICU).init({
    lng: locale,
    fallbackLng: "en",
    resources: {
      en: { translation: en },
      "en-XA": { translation: pseudoCatalog(en) },
    },
    interpolation: { escapeValue: false }, // Preact escapes output
  });
}
