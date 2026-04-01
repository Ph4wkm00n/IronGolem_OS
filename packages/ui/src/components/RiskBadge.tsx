import React from "react";
import type { RiskLevel } from "@irongolem/schema";
import { riskLevelLabel } from "@irongolem/schema";

export interface RiskBadgeProps {
  readonly level: RiskLevel;
  /** Size variant. */
  readonly size?: "sm" | "md";
  /** Override label text. */
  readonly label?: string;
}

const styles: Record<RiskLevel, string> = {
  low: "bg-emerald-100 text-emerald-800 border-emerald-200",
  medium: "bg-amber-100 text-amber-800 border-amber-200",
  high: "bg-orange-100 text-orange-800 border-orange-200",
  critical: "bg-red-100 text-red-800 border-red-200",
};

const icons: Record<RiskLevel, React.ReactNode> = {
  low: (
    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
    </svg>
  ),
  medium: (
    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01" />
    </svg>
  ),
  high: (
    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01M12 3l9.09 16.91H2.91L12 3z" />
    </svg>
  ),
  critical: (
    <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728L5.636 5.636" />
    </svg>
  ),
};

/**
 * Compact badge showing the risk level of an action or recipe.
 */
export function RiskBadge({ level, size = "md", label }: RiskBadgeProps) {
  const sizeClass = size === "sm" ? "px-1.5 py-0.5 text-[11px]" : "px-2.5 py-0.5 text-xs";

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full border font-medium ${styles[level]} ${sizeClass}`}
      role="status"
      aria-label={`Risk level: ${riskLevelLabel[level]}`}
    >
      {icons[level]}
      {label ?? riskLevelLabel[level]}
    </span>
  );
}
