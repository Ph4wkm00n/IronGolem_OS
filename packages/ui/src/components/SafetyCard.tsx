import React from "react";
import type { Recipe, RiskLevel } from "@irongolem/schema";
import { safe, blocked, warning, accent } from "@irongolem/design-tokens";

export interface SafetyCardProps {
  /** Items the recipe can access. */
  readonly canAccess: readonly string[];
  /** Items the recipe cannot access. */
  readonly cannotAccess: readonly string[];
  /** Actions requiring explicit approval. */
  readonly needsApprovalFor: readonly string[];
  /** Conditions that trigger an automatic stop. */
  readonly stopsIf: readonly string[];
  /** Overall risk level shown in the card header. */
  readonly riskLevel?: RiskLevel;
  /** Optional title override. */
  readonly title?: string;
}

interface SectionProps {
  readonly heading: string;
  readonly items: readonly string[];
  readonly icon: React.ReactNode;
  readonly colorClass: string;
}

function Section({ heading, items, icon, colorClass }: SectionProps) {
  if (items.length === 0) return null;

  return (
    <div className="py-3 first:pt-0 last:pb-0">
      <h4 className={`text-sm font-medium mb-2 flex items-center gap-1.5 ${colorClass}`}>
        {icon}
        {heading}
      </h4>
      <ul className="space-y-1 pl-5" role="list">
        {items.map((item) => (
          <li key={item} className="text-sm text-neutral-700 list-disc">
            {item}
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Safety card summarising what a recipe or action can and cannot do.
 *
 * Designed for progressive disclosure: each section only renders when
 * it has items, so the card never shows empty lists.
 */
export function SafetyCard({
  canAccess,
  cannotAccess,
  needsApprovalFor,
  stopsIf,
  riskLevel,
  title = "Safety summary",
}: SafetyCardProps) {
  return (
    <article
      className="rounded-xl border border-neutral-200 bg-white shadow-sm overflow-hidden"
      aria-label={title}
    >
      <header className="px-4 py-3 border-b border-neutral-100 flex items-center justify-between">
        <h3 className="text-base font-semibold text-neutral-900">{title}</h3>
        {riskLevel && <RiskIndicator level={riskLevel} />}
      </header>

      <div className="px-4 py-3 divide-y divide-neutral-100">
        <Section
          heading="Can access"
          items={canAccess}
          colorClass="text-emerald-700"
          icon={
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
            </svg>
          }
        />

        <Section
          heading="Cannot access"
          items={cannotAccess}
          colorClass="text-red-700"
          icon={
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728L5.636 5.636" />
            </svg>
          }
        />

        <Section
          heading="Needs approval for"
          items={needsApprovalFor}
          colorClass="text-amber-700"
          icon={
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z" />
            </svg>
          }
        />

        <Section
          heading="Stops automatically if"
          items={stopsIf}
          colorClass="text-indigo-700"
          icon={
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          }
        />
      </div>
    </article>
  );
}

function RiskIndicator({ level }: { readonly level: RiskLevel }) {
  const styles: Record<RiskLevel, string> = {
    low: "bg-emerald-100 text-emerald-800",
    medium: "bg-amber-100 text-amber-800",
    high: "bg-orange-100 text-orange-800",
    critical: "bg-red-100 text-red-800",
  };

  const labels: Record<RiskLevel, string> = {
    low: "Low risk",
    medium: "Medium risk",
    high: "High risk",
    critical: "Critical",
  };

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${styles[level]}`}>
      {labels[level]}
    </span>
  );
}
