import { render } from "preact";

import { initI18n } from "./i18n";
import { resolveLocale } from "./i18n/locale";
import { createPlatform } from "./platform";
import { App } from "./ui/App";

const locale = resolveLocale(window.location.search, navigator.languages);
const t = await initI18n(locale);
document.documentElement.lang = locale;
const root = document.getElementById("app");
if (root) {
  render(<App t={t} locale={locale} lifecycle={createPlatform().lifecycle} />, root);
}
