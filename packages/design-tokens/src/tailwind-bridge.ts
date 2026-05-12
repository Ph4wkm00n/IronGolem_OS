/**
 * Tailwind ↔ design-tokens bridge.
 *
 * Single source of truth for two things consumers used to hand-duplicate:
 *
 *   1. The `theme.extend.colors` block in `apps/web/tailwind.config.ts` —
 *      built programmatically from the semantic palettes in `colors.ts` so
 *      adding a new palette is one edit, not three.
 *
 *   2. The `:root { --color-...: ... }` block in `apps/web/src/styles/globals.css` —
 *      buildable from `allCssCustomProperties` for use by the
 *      `scripts/dump-tokens-css.ts` generator.
 *
 * Ergonomic alias: every palette gets a `DEFAULT` value pointing at its
 * background tint, so `bg-safe`, `bg-warning`, etc. work as shorthand for
 * the most common visual intent. Suffixed forms (`bg-safe-bg-hover`,
 * `text-safe-text`, etc.) remain available for explicit usage.
 */

import {
  accent,
  blocked,
  neutral,
  quarantined,
  recovered,
  safe,
  warning,
  cssCustomProperties as colorCustomProperties,
  type ColorScale,
} from "./colors";
import { cssCustomProperties as spacingCustomProperties } from "./spacing";
import { cssCustomProperties as typographyCustomProperties } from "./typography";

/** Every semantic palette name exposed by the bridge. */
export const SEMANTIC_PALETTE_NAMES = [
  "safe",
  "warning",
  "blocked",
  "recovered",
  "quarantined",
  "neutral",
  "accent",
] as const;

export type SemanticPaletteName = (typeof SEMANTIC_PALETTE_NAMES)[number];

/** Every field on a {@link ColorScale}, with its kebab-case Tailwind suffix. */
export const COLOR_SCALE_FIELDS: ReadonlyArray<readonly [keyof ColorScale, string]> = [
  ["bg", "bg"],
  ["bgHover", "bg-hover"],
  ["border", "border"],
  ["text", "text"],
  ["solid", "solid"],
  ["solidHover", "solid-hover"],
] as const;

/**
 * The shape of one palette entry in Tailwind's `theme.extend.colors`.
 *
 * `DEFAULT` is set to the background tint so `bg-safe` works as shorthand;
 * suffixed keys remain for explicit usage (`bg-safe-solid-hover`, etc.).
 */
export interface SemanticTailwindColor {
  readonly DEFAULT: string;
  readonly bg: string;
  readonly "bg-hover": string;
  readonly border: string;
  readonly text: string;
  readonly solid: string;
  readonly "solid-hover": string;
}

export type TailwindSemanticColors = Readonly<Record<SemanticPaletteName, SemanticTailwindColor>>;

const PALETTES: Readonly<Record<SemanticPaletteName, ColorScale>> = {
  safe,
  warning,
  blocked,
  recovered,
  quarantined,
  neutral,
  accent,
};

/**
 * Build the `theme.extend.colors` object for `tailwind.config.ts`.
 *
 * Every value is a `var(--color-NAME-FIELD)` reference; the CSS custom
 * properties themselves live in `globals.css` (or whatever consumes
 * {@link buildRootCssVariables}). This indirection is what lets a future
 * theme swap (dark mode, brand variants) happen by editing CSS only.
 */
export function buildTailwindColors(): TailwindSemanticColors {
  const result = {} as Record<SemanticPaletteName, SemanticTailwindColor>;
  for (const name of SEMANTIC_PALETTE_NAMES) {
    result[name] = {
      DEFAULT: `var(--color-${name}-bg)`,
      bg: `var(--color-${name}-bg)`,
      "bg-hover": `var(--color-${name}-bg-hover)`,
      border: `var(--color-${name}-border)`,
      text: `var(--color-${name}-text)`,
      solid: `var(--color-${name}-solid)`,
      "solid-hover": `var(--color-${name}-solid-hover)`,
    };
  }
  return result;
}

/**
 * Build the body of a `:root { ... }` block: every design-token CSS
 * custom property as a `  --name: value;` line, sorted by section.
 *
 * Output is deterministic so re-running the dump script produces a clean
 * diff. Section comments delimit color, typography, spacing groups.
 */
export function buildRootCssVariables(): string {
  const lines: string[] = [];
  lines.push("  /* Semantic colors */");
  for (const [key, value] of orderedEntries(colorCustomProperties)) {
    lines.push(`  ${key}: ${value};`);
  }
  lines.push("");
  lines.push("  /* Typography */");
  for (const [key, value] of orderedEntries(typographyCustomProperties)) {
    lines.push(`  ${key}: ${value};`);
  }
  lines.push("");
  lines.push("  /* Spacing */");
  for (const [key, value] of orderedEntries(spacingCustomProperties)) {
    lines.push(`  ${key}: ${value};`);
  }
  return lines.join("\n");
}

function orderedEntries(record: Record<string, string>): Array<[string, string]> {
  return Object.entries(record).sort(([a], [b]) => a.localeCompare(b));
}
