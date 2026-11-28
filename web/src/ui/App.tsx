import type { TFunction } from "i18next";
import { useEffect, useMemo, useState } from "preact/hooks";

import { formatMoney, type Locale } from "../i18n/locale";
import type { ErrorReporter } from "../platform/sentry";
import type { Lifecycle } from "../platform/types";
import type { FromWorker, SimView, Speed, ToWorker } from "../worker/protocol";

const SPEEDS: Speed[] = [0, 1, 2, 4];

export interface AppProps {
  t: TFunction;
  locale: Locale;
  lifecycle: Lifecycle;
  reporter: ErrorReporter;
}

export function App({ t, locale, lifecycle, reporter }: AppProps) {
  const [view, setView] = useState<SimView | undefined>();
  const [errorKey, setErrorKey] = useState<string | undefined>();
  const worker = useMemo(
    () => new Worker(new URL("../worker/sim.worker.ts", import.meta.url), { type: "module" }),
    [],
  );

  useEffect(() => {
    worker.onmessage = (e: MessageEvent<FromWorker>) => {
      const msg = e.data;
      if (msg.kind === "error") {
        setErrorKey(msg.errorKey);
        if (msg.report) {
          reporter.simError(msg.report);
        }
      } else if (msg.kind === "simReports") {
        reporter.simReports(msg.reports);
      } else {
        setView(msg.view);
      }
    };
    // Until the new-game screen exists (M2.7), start with default choices.
    worker.postMessage({
      kind: "start",
      seed: 42,
      airlineName: "Jetstream Air",
      homeCountry: "GB",
      difficulty: "normal",
    } satisfies ToWorker);
    // The game pauses when the app goes to the background; there is no idle progress.
    const offBackground = lifecycle.onBackground(() =>
      worker.postMessage({ kind: "setSpeed", speed: 0 } satisfies ToWorker),
    );
    return () => {
      offBackground();
      worker.terminate();
    };
  }, [worker, lifecycle, reporter]);

  const send = (msg: ToWorker) => worker.postMessage(msg);

  return (
    <main>
      <h1>{t("app.title.main")}</h1>
      {errorKey && <p role="alert">{t(errorKey)}</p>}
      {!view && !errorKey && <p>{t("app.loading.sim")}</p>}
      {view && (
        <section>
          <p>{t("hud.day.current", { day: view.day })}</p>
          <p>{t("hud.cash.current", { cash: formatMoney(view.cashCents, locale) })}</p>
          <p>{t("hud.speed.current", { speed: view.speed })}</p>
          <div role="group" aria-label={t("hud.speed.label")}>
            {SPEEDS.map((s) => (
              <button
                key={s}
                type="button"
                aria-pressed={view.speed === s}
                onClick={() => send({ kind: "setSpeed", speed: s })}
              >
                {s === 0 ? t("hud.speed.pause") : `${s}×`}
              </button>
            ))}
          </div>
        </section>
      )}
    </main>
  );
}
