// Simulation Worker entry: loads sim.wasm and drives it ~4 times per second.
import { createController } from "./controller";
import type { FromWorker, ToWorker } from "./protocol";
import { loadWasmBridge } from "./wasm-loader";

const PULSE_MS = 250;

const ready = loadWasmBridge("/wasm_exec.js", "/sim.wasm").then((bridge) =>
  createController(bridge, (msg: FromWorker) => self.postMessage(msg)),
);
ready.catch(() => self.postMessage({ kind: "error", errorKey: "errors.sim.load_failed" } satisfies FromWorker));

self.onmessage = (e: MessageEvent<ToWorker>) => {
  void ready.then((c) => c.handle(e.data));
};
setInterval(() => {
  void ready.then((c) => c.pulse(performance.now()));
}, PULSE_MS);
