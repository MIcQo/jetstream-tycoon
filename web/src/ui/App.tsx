import type { TFunction } from "i18next";
import { useEffect, useMemo, useState } from "preact/hooks";

import { formatMoney, type Locale } from "../i18n/locale";
import type { FromWorker, SimView, Speed, ToWorker } from "../worker/protocol";

const SPEEDS: Speed[] = [0, 1, 2, 4];

export function App({ t, locale }: { t: TFunction; locale: Locale }) {
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
      } else {
        setView(msg.view);
      }
    };
    worker.postMessage({ kind: "start", seed: 42 } satisfies ToWorker);
    return () => worker.terminate();
  }, [worker]);

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
