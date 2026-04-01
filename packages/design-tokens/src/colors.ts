/**
 * Semantic color tokens for IronGolem OS.
 *
 * Every status-bearing surface uses a semantic color so the meaning is
 * immediately clear.  The palette intentionally stays calm — bright hues
 * are reserved for states that genuinely need attention.
 */

export interface ColorScale {
  /** Lightest tint — backgrounds, cards */
  readonly bg: string;
  /** Slightly darker — hover states, subtle fills */
  readonly bgHover: string;
  /** Borders and dividers */
  readonly border: string;
  /** Primary foreground text on a neutral background */
  readonly text: string;
  /** Solid fill — badges, indicators */
  readonly solid: string;
  /** Solid fill hover */
  readonly solidHover: string;
}

/** Safe / healthy / approved */
export const safe: ColorScale = {
  bg: "#ECFDF5",
  bgHover: "#D1FAE5",
  border: "#6EE7B7",
  text: "#065F46",
  solid: "#10B981",
  solidHover: "#059669",
};

/** Warning / needs attention */
export const warning: ColorScale = {
  bg: "#FFFBEB",
  bgHover: "#FEF3C7",
  border: "#FCD34D",
  text: "#92400E",
  solid: "#F59E0B",
  solidHover: "#D97706",
};

/** Blocked / denied / error */
export const blocked: ColorScale = {
  bg: "#FEF2F2",
  bgHover: "#FEE2E2",
  border: "#FCA5A5",
  text: "#991B1B",
  solid: "#EF4444",
  solidHover: "#DC2626",
};

/** Recovered / healed */
export const recovered: ColorScale = {
  bg: "#EFF6FF",
  bgHover: "#DBEAFE",
  border: "#93C5FD",
  text: "#1E40AF",
  solid: "#3B82F6",
  solidHover: "#2563EB",
};

/** Quarantined / isolated */
export const quarantined: ColorScale = {
  bg: "#FDF4FF",
  bgHover: "#FAE8FF",
  border: "#D8B4FE",
  text: "#6B21A8",
  solid: "#A855F7",
  solidHover: "#9333EA",
};

/** Neutral / default / informational */
export const neutral: ColorScale = {
  bg: "#F9FAFB",
  bgHover: "#F3F4F6",
  border: "#D1D5DB",
  text: "#111827",
  solid: "#6B7280",
  solidHover: "#4B5563",
};

/** Accent — primary brand interaction color */
export const accent: ColorScale = {
  bg: "#EEF2FF",
  bgHover: "#E0E7FF",
  border: "#A5B4FC",
  text: "#3730A3",
  solid: "#6366F1",
  solidHover: "#4F46E5",
};

/**
 * Flat map of every semantic color as a CSS custom-property name/value pair.
 * Useful for injecting into `:root`.
 */
export const cssCustomProperties: Record<string, string> = {
  "--color-safe-bg": safe.bg,
  "--color-safe-bg-hover": safe.bgHover,
  "--color-safe-border": safe.border,
  "--color-safe-text": safe.text,
  "--color-safe-solid": safe.solid,
  "--color-safe-solid-hover": safe.solidHover,

  "--color-warning-bg": warning.bg,
  "--color-warning-bg-hover": warning.bgHover,
  "--color-warning-border": warning.border,
  "--color-warning-text": warning.text,
  "--color-warning-solid": warning.solid,
  "--color-warning-solid-hover": warning.solidHover,

  "--color-blocked-bg": blocked.bg,
  "--color-blocked-bg-hover": blocked.bgHover,
  "--color-blocked-border": blocked.border,
  "--color-blocked-text": blocked.text,
  "--color-blocked-solid": blocked.solid,
  "--color-blocked-solid-hover": blocked.solidHover,

  "--color-recovered-bg": recovered.bg,
  "--color-recovered-bg-hover": recovered.bgHover,
  "--color-recovered-border": recovered.border,
  "--color-recovered-text": recovered.text,
  "--color-recovered-solid": recovered.solid,
  "--color-recovered-solid-hover": recovered.solidHover,

  "--color-quarantined-bg": quarantined.bg,
  "--color-quarantined-bg-hover": quarantined.bgHover,
  "--color-quarantined-border": quarantined.border,
  "--color-quarantined-text": quarantined.text,
  "--color-quarantined-solid": quarantined.solid,
  "--color-quarantined-solid-hover": quarantined.solidHover,

  "--color-neutral-bg": neutral.bg,
  "--color-neutral-bg-hover": neutral.bgHover,
  "--color-neutral-border": neutral.border,
  "--color-neutral-text": neutral.text,
  "--color-neutral-solid": neutral.solid,
  "--color-neutral-solid-hover": neutral.solidHover,

  "--color-accent-bg": accent.bg,
  "--color-accent-bg-hover": accent.bgHover,
  "--color-accent-border": accent.border,
  "--color-accent-text": accent.text,
  "--color-accent-solid": accent.solid,
  "--color-accent-solid-hover": accent.solidHover,
};
