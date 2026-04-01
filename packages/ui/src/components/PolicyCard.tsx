import React from "react";
import type {
  PolicyLayer,
  PolicyDecision,
  PolicyEffect,
  PolicyEvaluation,
} from "@irongolem/schema";
import { policyLayerLabel, policyLayerDescription } from "@irongolem/schema";

export interface PolicyCardProps {
  /** Full policy evaluation result across all five layers. */
  readonly evaluation: PolicyEvaluation;
  /** Whether to show detailed descriptions per layer. */
  readonly showDescriptions?: boolean;
}

const LAYER_ORDER: readonly PolicyLayer[] = [
  "gateway-identity",
  "global-tool-policy",
  "per-agent-permissions",
  "per-channel-restrictions",
  "admin-only-controls",
];

const effectConfig: Record<PolicyEffect, { icon: React.ReactNode; color: string; label: string }> = {
  allow: {
    icon: (
      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
      </svg>
    ),
    color: "text-emerald-600 bg-emerald-50 border-emerald-200",
    label: "Allowed",
  },
  deny: {
    icon: (
      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
      </svg>
    ),
    color: "text-red-600 bg-red-50 border-red-200",
    label: "Blocked",
  },
  "require-approval": {
    icon: (
      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v4m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z" />
      </svg>
    ),
    color: "text-amber-600 bg-amber-50 border-amber-200",
    label: "Needs approval",
  },
};

const finalEffectBanner: Record<PolicyEffect, { bg: string; text: string; label: string }> = {
  allow: { bg: "bg-emerald-50", text: "text-emerald-800", label: "Allowed" },
  deny: { bg: "bg-red-50", text: "text-red-800", label: "Blocked" },
  "require-approval": { bg: "bg-amber-50", text: "text-amber-800", label: "Needs approval" },
};

/**
 * Displays the five-layer policy evaluation result.
 *
 * Each layer is shown in order with its decision. The final combined
 * effect is prominently displayed at the top.
 */
export function PolicyCard({ evaluation, showDescriptions = false }: PolicyCardProps) {
  const decisionByLayer = new Map(
    evaluation.decisions.map((d) => [d.layer, d])
  );

  const banner = finalEffectBanner[evaluation.finalEffect];

  return (
    <article
      className="rounded-xl border border-neutral-200 bg-white shadow-sm overflow-hidden"
      aria-label="Safety rules evaluation"
    >
      {/* Final result banner */}
      <header className={`px-4 py-3 ${banner.bg} border-b border-neutral-100`}>
        <div className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-neutral-900">Safety rules</h3>
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold ${banner.text}`}>
            {banner.label}
          </span>
        </div>
      </header>

      {/* Layer-by-layer breakdown */}
      <ol className="divide-y divide-neutral-100" role="list" aria-label="Safety rule layers">
        {LAYER_ORDER.map((layer, idx) => {
          const decision = decisionByLayer.get(layer);
          const config = decision ? effectConfig[decision.effect] : null;

          return (
            <li key={layer} className="px-4 py-3">
              <div className="flex items-start gap-3">
                {/* Step number */}
                <span className="flex-shrink-0 flex items-center justify-center w-6 h-6 rounded-full bg-neutral-100 text-xs font-medium text-neutral-600">
                  {idx + 1}
                </span>

                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-neutral-900">
                      {policyLayerLabel[layer]}
                    </span>
                    {config && (
                      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium border ${config.color}`}>
                        {config.icon}
                        {config.label}
                      </span>
                    )}
                  </div>

                  {showDescriptions && (
                    <p className="mt-0.5 text-xs text-neutral-500">
                      {policyLayerDescription[layer]}
                    </p>
                  )}

                  {decision && decision.reason && (
                    <p className="mt-1 text-xs text-neutral-600">
                      {decision.reason}
                    </p>
                  )}
                </div>
              </div>
            </li>
          );
        })}
      </ol>
    </article>
  );
}
