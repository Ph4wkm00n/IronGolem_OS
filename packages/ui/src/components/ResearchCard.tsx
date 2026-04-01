import React from "react";
import type { ResearchTopic } from "@irongolem/schema";

export interface ResearchCardProps {
  readonly topic: ResearchTopic;
  /** Called when the user clicks for more detail. */
  readonly onViewDetails?: (topicId: string) => void;
}

const freshnessConfig: Record<ResearchTopic["freshness"], { label: string; color: string }> = {
  fresh: { label: "Up to date", color: "text-emerald-700 bg-emerald-50" },
  aging: { label: "Getting older", color: "text-amber-700 bg-amber-50" },
  stale: { label: "Needs refresh", color: "text-red-700 bg-red-50" },
};

/**
 * Card displaying a research finding with confidence, freshness,
 * source count, and an optional contradiction marker.
 */
export function ResearchCard({ topic, onViewDetails }: ResearchCardProps) {
  const freshness = freshnessConfig[topic.freshness];
  const confidencePercent = Math.round(topic.confidence * 100);

  return (
    <article
      className="rounded-xl border border-neutral-200 bg-white shadow-sm overflow-hidden hover:shadow-md transition-shadow"
      aria-label={`Research: ${topic.title}`}
    >
      <div className="px-4 py-4">
        {/* Header */}
        <div className="flex items-start justify-between gap-2">
          <h3 className="text-sm font-semibold text-neutral-900 leading-snug">
            {topic.title}
          </h3>
          {topic.hasContradictions && (
            <span
              className="flex-shrink-0 inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-orange-100 text-orange-800 border border-orange-200"
              title="Sources disagree on some findings"
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01M12 3l9.09 16.91H2.91L12 3z" />
              </svg>
              Conflicting
            </span>
          )}
        </div>

        <p className="mt-1.5 text-sm text-neutral-600 line-clamp-2">
          {topic.description}
        </p>

        {/* Metrics row */}
        <div className="mt-3 flex items-center gap-3 flex-wrap">
          {/* Confidence */}
          <div className="flex items-center gap-1.5">
            <div className="w-16 h-1.5 rounded-full bg-neutral-200 overflow-hidden">
              <div
                className={`h-full rounded-full ${
                  confidencePercent >= 70
                    ? "bg-emerald-500"
                    : confidencePercent >= 40
                      ? "bg-amber-500"
                      : "bg-red-500"
                }`}
                style={{ width: `${confidencePercent}%` }}
              />
            </div>
            <span className="text-xs text-neutral-600">{confidencePercent}% confidence</span>
          </div>

          {/* Freshness */}
          <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${freshness.color}`}>
            {freshness.label}
          </span>

          {/* Source count */}
          <span className="text-xs text-neutral-500">
            {topic.sourceCount} {topic.sourceCount === 1 ? "source" : "sources"}
          </span>
        </div>

        {/* Last updated */}
        <div className="mt-3 flex items-center justify-between">
          <time className="text-xs text-neutral-400" dateTime={topic.lastUpdated}>
            Updated {formatRelative(topic.lastUpdated)}
          </time>
          {onViewDetails && (
            <button
              type="button"
              className="text-xs font-medium text-indigo-600 hover:text-indigo-500"
              onClick={() => onViewDetails(topic.id)}
            >
              View details
            </button>
          )}
        </div>
      </div>
    </article>
  );
}

function formatRelative(iso: string): string {
  try {
    const then = new Date(iso).getTime();
    const now = Date.now();
    const diffMs = now - then;
    const diffMin = Math.floor(diffMs / 60_000);

    if (diffMin < 1) return "just now";
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    const diffDays = Math.floor(diffHr / 24);
    if (diffDays < 7) return `${diffDays}d ago`;
    return new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric" });
  } catch {
    return iso;
  }
}
