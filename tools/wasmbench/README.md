# wasmbench

Measures `sim.wasm` compile, instantiate, `newGame` and per-day cost in headless
Chromium, with CPU throttling (1×, 4×, 6×) as a stand-in for low-end phones.

```bash
make wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/public/sim.wasm tools/wasmbench/
node tools/wasmbench/bench.cjs tools/wasmbench
```

Needs Playwright with Chromium. Results are recorded in `docs/perf/`.
