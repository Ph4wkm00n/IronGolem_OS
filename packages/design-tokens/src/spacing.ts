/**
 * Spacing scale for IronGolem OS.
 *
 * Based on a 4 px base unit.  Named stops keep layouts consistent and
 * make it easy to adjust density globally.
 */

/** Base unit in pixels. */
export const BASE_UNIT = 4;

/** Named spacing stops in pixels. */
export const spacing = {
  /** 0 px */
  none: 0,
  /** 2 px — hairline gaps */
  "0.5": BASE_UNIT * 0.5,
  /** 4 px — tightest useful gap */
  "1": BASE_UNIT,
  /** 8 px — compact inner padding */
  "2": BASE_UNIT * 2,
  /** 12 px — default inner padding */
  "3": BASE_UNIT * 3,
  /** 16 px — standard gap / card padding */
  "4": BASE_UNIT * 4,
  /** 20 px */
  "5": BASE_UNIT * 5,
  /** 24 px — comfortable card padding */
  "6": BASE_UNIT * 6,
  /** 32 px — section padding */
  "8": BASE_UNIT * 8,
  /** 40 px */
  "10": BASE_UNIT * 10,
  /** 48 px — page-level padding */
  "12": BASE_UNIT * 12,
  /** 64 px — large section gaps */
  "16": BASE_UNIT * 16,
  /** 80 px */
  "20": BASE_UNIT * 20,
  /** 96 px — maximum breathing room */
  "24": BASE_UNIT * 24,
} as const;

export type SpacingKey = keyof typeof spacing;

/** Returns the spacing value as a CSS `px` string. */
export function px(key: SpacingKey): string {
  return `${spacing[key]}px`;
}

/** Returns the spacing value as a CSS `rem` string (base 16). */
export function rem(key: SpacingKey): string {
  return `${spacing[key] / 16}rem`;
}

/** CSS custom properties for spacing. */
export const cssCustomProperties: Record<string, string> = Object.fromEntries(
  Object.entries(spacing).map(([key, val]) => [
    `--space-${key}`,
    `${val}px`,
  ]),
);
