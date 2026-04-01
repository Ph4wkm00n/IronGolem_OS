import type { Config } from "tailwindcss";

export default {
  content: [
    "./index.html",
    "./src/**/*.{ts,tsx}",
    "../../packages/ui/src/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: [
          "Inter",
          "-apple-system",
          "BlinkMacSystemFont",
          "Segoe UI",
          "Roboto",
          "sans-serif",
        ],
        mono: [
          "JetBrains Mono",
          "Fira Code",
          "SF Mono",
          "Menlo",
          "Consolas",
          "monospace",
        ],
      },
      colors: {
        safe: {
          bg: "var(--color-safe-bg)",
          "bg-hover": "var(--color-safe-bg-hover)",
          border: "var(--color-safe-border)",
          text: "var(--color-safe-text)",
          solid: "var(--color-safe-solid)",
          "solid-hover": "var(--color-safe-solid-hover)",
        },
        warning: {
          bg: "var(--color-warning-bg)",
          "bg-hover": "var(--color-warning-bg-hover)",
          border: "var(--color-warning-border)",
          text: "var(--color-warning-text)",
          solid: "var(--color-warning-solid)",
          "solid-hover": "var(--color-warning-solid-hover)",
        },
        blocked: {
          bg: "var(--color-blocked-bg)",
          "bg-hover": "var(--color-blocked-bg-hover)",
          border: "var(--color-blocked-border)",
          text: "var(--color-blocked-text)",
          solid: "var(--color-blocked-solid)",
          "solid-hover": "var(--color-blocked-solid-hover)",
        },
        recovered: {
          bg: "var(--color-recovered-bg)",
          "bg-hover": "var(--color-recovered-bg-hover)",
          border: "var(--color-recovered-border)",
          text: "var(--color-recovered-text)",
          solid: "var(--color-recovered-solid)",
          "solid-hover": "var(--color-recovered-solid-hover)",
        },
        quarantined: {
          bg: "var(--color-quarantined-bg)",
          "bg-hover": "var(--color-quarantined-bg-hover)",
          border: "var(--color-quarantined-border)",
          text: "var(--color-quarantined-text)",
          solid: "var(--color-quarantined-solid)",
          "solid-hover": "var(--color-quarantined-solid-hover)",
        },
        accent: {
          bg: "var(--color-accent-bg)",
          "bg-hover": "var(--color-accent-bg-hover)",
          border: "var(--color-accent-border)",
          text: "var(--color-accent-text)",
          solid: "var(--color-accent-solid)",
          "solid-hover": "var(--color-accent-solid-hover)",
        },
      },
    },
  },
  plugins: [],
} satisfies Config;
