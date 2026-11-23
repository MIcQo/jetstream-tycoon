import preact from "@preact/preset-vite";
import { defineConfig } from "vite";

import pkg from "./package.json" with { type: "json" };

export default defineConfig({
  plugins: [preact()],
  worker: { format: "es" },
  build: { target: "es2022", sourcemap: "hidden" }, // source maps are uploaded to Sentry, not served
  define: {
    "import.meta.env.VITE_APP_VERSION": JSON.stringify(pkg.version),
  },
});
