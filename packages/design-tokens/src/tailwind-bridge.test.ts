import { describe, expect, test } from "bun:test";

import {
  COLOR_SCALE_FIELDS,
  SEMANTIC_PALETTE_NAMES,
  buildRootCssVariables,
  buildTailwindColors,
} from "./tailwind-bridge";
import { cssCustomProperties as colorCssProps, safe, neutral } from "./colors";

describe("buildTailwindColors", () => {
  const colors = buildTailwindColors();

  test("includes every semantic palette name", () => {
    for (const name of SEMANTIC_PALETTE_NAMES) {
      expect(colors[name]).toBeDefined();
    }
  });

  test("includes neutral palette which the previous hand-written config omitted", () => {
    expect(colors.neutral).toBeDefined();
    expect(colors.neutral.bg).toBe("var(--color-neutral-bg)");
  });

  test("maps each field to its kebab-case CSS variable reference", () => {
    expect(colors.safe.bg).toBe("var(--color-safe-bg)");
    expect(colors.safe["bg-hover"]).toBe("var(--color-safe-bg-hover)");
    expect(colors.safe.border).toBe("var(--color-safe-border)");
    expect(colors.safe.text).toBe("var(--color-safe-text)");
    expect(colors.safe.solid).toBe("var(--color-safe-solid)");
    expect(colors.safe["solid-hover"]).toBe("var(--color-safe-solid-hover)");
  });

  test("sets DEFAULT to the background tint so bg-safe etc. work as shorthand", () => {
    for (const name of SEMANTIC_PALETTE_NAMES) {
      expect(colors[name].DEFAULT).toBe(`var(--color-${name}-bg)`);
    }
  });
});

describe("buildRootCssVariables", () => {
  const body = buildRootCssVariables();

  test("includes the safe palette in canonical hex form", () => {
    expect(body.toLowerCase()).toContain(`--color-safe-bg: ${safe.bg.toLowerCase()};`);
  });

  test("includes the full neutral palette", () => {
    expect(body.toLowerCase()).toContain(`--color-neutral-bg: ${neutral.bg.toLowerCase()};`);
    expect(body.toLowerCase()).toContain(`--color-neutral-bg-hover:`);
    expect(body.toLowerCase()).toContain(`--color-neutral-border:`);
    expect(body.toLowerCase()).toContain(`--color-neutral-solid-hover:`);
  });

  test("groups output with section comments", () => {
    expect(body).toContain("/* Semantic colors */");
    expect(body).toContain("/* Typography */");
    expect(body).toContain("/* Spacing */");
  });

  test("emits one --name: value; line per color custom property", () => {
    for (const cssVarName of Object.keys(colorCssProps)) {
      expect(body).toContain(`${cssVarName}:`);
    }
  });
});

describe("schema integrity", () => {
  test("COLOR_SCALE_FIELDS lists every ColorScale field exactly once", () => {
    const colorScaleKeys = Object.keys(safe).sort();
    const bridgeKeys = COLOR_SCALE_FIELDS.map(([key]) => key as string).sort();
    expect(bridgeKeys).toEqual(colorScaleKeys);
  });

  test("SEMANTIC_PALETTE_NAMES matches the palettes exported by colors.ts", () => {
    const expected = [
      "safe",
      "warning",
      "blocked",
      "recovered",
      "quarantined",
      "neutral",
      "accent",
    ];
    expect([...SEMANTIC_PALETTE_NAMES].sort()).toEqual([...expected].sort());
  });
});
