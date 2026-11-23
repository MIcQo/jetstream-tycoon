// Copies Go's wasm_exec.js (the runtime glue for sim.wasm) into public/, matching
// the Go toolchain that builds sim.wasm. The copy is git-ignored.
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const goroot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf8" }).trim();
const candidates = [path.join(goroot, "lib/wasm/wasm_exec.js"), path.join(goroot, "misc/wasm/wasm_exec.js")];
const src = candidates.find((p) => fs.existsSync(p));
if (!src) {
  throw new Error(`wasm_exec.js not found under ${goroot}`);
}
const dest = path.resolve(import.meta.dirname, "../public/wasm_exec.js");
fs.mkdirSync(path.dirname(dest), { recursive: true });
fs.copyFileSync(src, dest);
console.log(`copied ${src} -> ${dest}`);
