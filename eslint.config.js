// Shared ESLint flat config for the whole pnpm workspace.
// Each package's `lint` script runs `eslint src/` from its own directory;
// ESLint 9 resolves this root config by walking upward from the CWD.
import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default tseslint.config(
  {
    ignores: [
      "**/dist/**",
      "**/node_modules/**",
      "**/*.d.ts",
      "**/target/**",
      // Design-drop staging copies (imported artifacts, not maintained source).
      "**/_design-inbox/**",
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx,jsx}"],
    plugins: { "react-hooks": reactHooks },
    rules: {
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
  {
    rules: {
      // Interop shims and event payloads legitimately carry unknown shapes;
      // strict mode (`no any in production`) is enforced by tsc, reviews, and
      // the explicit annotations rule below — not by banning the keyword.
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },
);
