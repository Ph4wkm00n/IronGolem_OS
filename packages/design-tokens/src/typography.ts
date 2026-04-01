/**
 * Typography scale for IronGolem OS.
 *
 * Uses a limited set of named stops so every screen stays visually
 * consistent without ad-hoc font sizes.
 */

export interface TypographyToken {
  /** CSS font-size value */
  readonly fontSize: string;
  /** CSS line-height value */
  readonly lineHeight: string;
  /** CSS font-weight value */
  readonly fontWeight: number;
  /** CSS letter-spacing value */
  readonly letterSpacing: string;
  /** Font family stack */
  readonly fontFamily: string;
}

const sansStack =
  "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif";
const monoStack =
  "'JetBrains Mono', 'Fira Code', 'SF Mono', Menlo, Consolas, monospace";

/** Large page headings */
export const pageTitle: TypographyToken = {
  fontSize: "1.875rem",    // 30px
  lineHeight: "2.25rem",   // 36px
  fontWeight: 700,
  letterSpacing: "-0.025em",
  fontFamily: sansStack,
};

/** Section headings within a page */
export const sectionTitle: TypographyToken = {
  fontSize: "1.25rem",     // 20px
  lineHeight: "1.75rem",   // 28px
  fontWeight: 600,
  letterSpacing: "-0.01em",
  fontFamily: sansStack,
};

/** Default body text */
export const body: TypographyToken = {
  fontSize: "0.9375rem",   // 15px
  lineHeight: "1.5rem",    // 24px
  fontWeight: 400,
  letterSpacing: "0",
  fontFamily: sansStack,
};

/** Small supporting text */
export const caption: TypographyToken = {
  fontSize: "0.8125rem",   // 13px
  lineHeight: "1.125rem",  // 18px
  fontWeight: 400,
  letterSpacing: "0.01em",
  fontFamily: sansStack,
};

/** Labels for form fields, badges, etc. */
export const label: TypographyToken = {
  fontSize: "0.8125rem",   // 13px
  lineHeight: "1rem",      // 16px
  fontWeight: 500,
  letterSpacing: "0.02em",
  fontFamily: sansStack,
};

/** Monospaced text for code, IDs, logs */
export const code: TypographyToken = {
  fontSize: "0.8125rem",   // 13px
  lineHeight: "1.25rem",   // 20px
  fontWeight: 400,
  letterSpacing: "0",
  fontFamily: monoStack,
};

/** All named typography tokens for iteration. */
export const typographyTokens = {
  pageTitle,
  sectionTitle,
  body,
  caption,
  label,
  code,
} as const;

/** CSS custom properties for typography. */
export const cssCustomProperties: Record<string, string> = Object.fromEntries(
  Object.entries(typographyTokens).flatMap(([name, token]) => [
    [`--type-${name}-size`, token.fontSize],
    [`--type-${name}-line`, token.lineHeight],
    [`--type-${name}-weight`, String(token.fontWeight)],
    [`--type-${name}-tracking`, token.letterSpacing],
    [`--type-${name}-family`, token.fontFamily],
  ]),
);
