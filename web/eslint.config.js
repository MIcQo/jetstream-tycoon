import eslint from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["src/**/*.ts"],
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@capacitor/*"],
              message:
                "Capacitor plugins may only be imported inside web/src/platform/ adapters (ADR-0003).",
            },
          ],
        },
      ],
    },
  },
  {
    files: ["src/platform/**/*.ts"],
    rules: {
      "no-restricted-imports": "off",
    },
  },
);
