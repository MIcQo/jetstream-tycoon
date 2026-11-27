import { render } from "preact";

import { initI18n } from "./i18n";
import { resolveLocale } from "./i18n/locale";
import { createPlatform } from "./platform";
import { initErrorReporting } from "./platform/sentry";
import { readBuildInfo } from "./telemetry/release";
import { App } from "./ui/App";

const platform = createPlatform();
// Initialise error reporting first so that failures during start-up are captured.
const reporter = await initErrorReporting(platform.kind, readBuildInfo(import.meta.env));
const locale = resolveLocale(window.location.search, navigator.languages);
const t = await initI18n(locale);
document.documentElement.lang = locale;
const root = document.getElementById("app");
if (root) {
  render(<App t={t} locale={locale} lifecycle={platform.lifecycle} reporter={reporter} />, root);
}
