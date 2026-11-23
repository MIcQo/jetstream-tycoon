import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["src/**/*.test.ts"],
    exclude: ["src/**/*.fuzz.test.ts", "src/**/*.integration.test.ts"],
  },
});
