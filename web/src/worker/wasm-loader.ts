import { jsonBridge, type RawSimApi, type SimBridge } from "./bridge";

interface GoRuntime {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

declare global {
  // Set by wasm_exec.js (Go runtime glue) and by sim.wasm's main.
  var Go: (new () => GoRuntime) | undefined;
  var jetstreamSim: RawSimApi | undefined;
}

/** Loads Go's wasm_exec.js and sim.wasm, and waits until the bridge API is registered. */
export async function loadWasmBridge(wasmExecUrl: string, wasmUrl: string): Promise<SimBridge> {
  await import(/* @vite-ignore */ wasmExecUrl);
  const GoCtor = globalThis.Go;
  if (!GoCtor) {
    throw new Error("wasm_exec.js did not define Go");
  }
  const go = new GoCtor();
  const { instance } = await WebAssembly.instantiateStreaming(fetch(wasmUrl), go.importObject);
  void go.run(instance);
  const api = globalThis.jetstreamSim;
  if (!api) {
    throw new Error("sim.wasm did not register jetstreamSim");
  }
  return jsonBridge(api);
}
