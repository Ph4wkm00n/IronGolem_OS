import React, { useEffect, useState } from "react";
import { ResearchCard } from "@irongolem/ui";
import type { ResearchTopic } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

type FreshnessFilter = "all" | "fresh" | "aging" | "stale";

interface ResearchState {
  topics: readonly ResearchTopic[];
  loading: boolean;
  filter: FreshnessFilter;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Research() {
  const [state, setState] = useState<ResearchState>({
    topics: [],
    loading: true,
    filter: "all",
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.research.listTopics({ pageSize: 50 });
        if (!cancelled) {
          setState((prev) => ({ ...prev, loading: false, topics: res.items }));
        }
      } catch {
        if (!cancelled) setState((prev) => ({ ...prev, loading: false }));
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  const filtered =
    state.filter === "all"
      ? state.topics
      : state.topics.filter((t) => t.freshness === state.filter);

  const contradictionCount = state.topics.filter((t) => t.hasContradictions).length;

  async function handleRefresh(topicId: string) {
    try {
      await api.research.refresh(topicId);
      // Reload the topic
      const updated = await api.research.getTopic(topicId);
      setState((prev) => ({
        ...prev,
        topics: prev.topics.map((t) => (t.id === topicId ? updated : t)),
      }));
    } catch {
      // Would use toast in production
    }
  }

  return (
    <div className="page-container">
      <h1 className="page-title mb-6">Research center</h1>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : (
        <div className="space-y-6">
          {/* Summary bar */}
          <div className="flex items-center gap-4 flex-wrap">
            <span className="text-sm text-neutral-600">
              {state.topics.length} tracked {state.topics.length === 1 ? "topic" : "topics"}
            </span>
            {contradictionCount > 0 && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-orange-100 text-orange-700">
                {contradictionCount} with conflicting sources
              </span>
            )}
          </div>

          {/* Filter tabs */}
          <div className="flex gap-1 border-b border-neutral-200">
            {(["all", "fresh", "aging", "stale"] as const).map((tab) => {
              const labels: Record<FreshnessFilter, string> = {
                all: "All",
                fresh: "Up to date",
                aging: "Getting older",
                stale: "Needs refresh",
              };
              return (
                <button
                  key={tab}
                  type="button"
                  onClick={() => setState((prev) => ({ ...prev, filter: tab }))}
                  className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                    state.filter === tab
                      ? "border-indigo-600 text-indigo-700"
                      : "border-transparent text-neutral-500 hover:text-neutral-700"
                  }`}
                >
                  {labels[tab]}
                </button>
              );
            })}
          </div>

          {/* Topic grid */}
          {filtered.length === 0 ? (
            <div className="card-padded text-center py-12">
              <p className="text-neutral-500">
                {state.filter === "all"
                  ? "No research topics are being tracked yet."
                  : "No topics match this filter."}
              </p>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {filtered.map((topic) => (
                <ResearchCard
                  key={topic.id}
                  topic={topic}
                  onViewDetails={() => handleRefresh(topic.id)}
                />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
