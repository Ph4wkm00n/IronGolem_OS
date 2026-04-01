/**
 * @irongolem/design-tokens
 *
 * Canonical visual language for IronGolem OS.
 */

export {
  safe, warning, blocked, recovered, quarantined, neutral, accent,
  cssCustomProperties as colorCssProperties,
} from "./colors";
export type { ColorScale } from "./colors";

export {
  pageTitle, sectionTitle, body, caption, label, code, typographyTokens,
  cssCustomProperties as typographyCssProperties,
} from "./typography";
export type { TypographyToken } from "./typography";

export {
  BASE_UNIT, spacing, px, rem,
  cssCustomProperties as spacingCssProperties,
} from "./spacing";
export type { SpacingKey } from "./spacing";

import { cssCustomProperties as colorProps } from "./colors";
import { cssCustomProperties as typographyProps } from "./typography";
import { cssCustomProperties as spacingProps } from "./spacing";

/**
 * Every design-token CSS custom property in a single map.
 * Inject into `:root` at app boot to make tokens available globally.
 */
export const allCssCustomProperties: Record<string, string> = {
  ...colorProps,
  ...typographyProps,
  ...spacingProps,
};
