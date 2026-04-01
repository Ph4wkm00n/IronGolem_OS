/**
 * @irongolem/design-tokens
 *
 * Canonical visual language for IronGolem OS.
 */

export * from "./colors";
export * from "./typography";
export * from "./spacing";

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
