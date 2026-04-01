import React, { useEffect, useState } from "react";
import { ResearchCard } from "@irongolem/ui";
import type { ResearchTopic } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  Placeholder data                                                   */
/* ------------------------------------------------------------------ */

const SAMPLE_TOPICS: readonly ResearchTopic[] = [
  {
    id: "topic-1",
    title: "Remote work productivity trends in 2026",
    description:
      "Analysis of how distributed teams are adapting their workflows, including async communication tools, meeting cadence changes, and output metrics.",
    confidence: 0.82,
    freshness: "fresh",
    sourceCount: 14,
    hasContradictions: false,
    lastUpdated: "2026-03-31T09:00:00Z",
  },
  {
    id: "topic-2",
    title: "AI-assisted code review best practices",
    description:
      "Comparison of automated code review approaches, including static analysis, LLM-based suggestions, and hybrid workflows. Covers accuracy, developer trust, and adoption patterns.",
    confidence: 0.68,
    freshness: "aging",
    sourceCount: 9,
    hasContradictions: true,
    lastUpdated: "2026-03-25T15:30:00Z",
  },
  {
    id: "topic-3",
    title: "Privacy regulations update (EU, US, APAC)",
    description:
      "Tracking changes to data privacy frameworks across major regions, with focus on how they affect SaaS products handling personal data.",
    confidence: 0.91,
    freshness: "fresh",
    sourceCount: 22,
    hasContradictions: false,
    lastUpdated: "2026-03-30T18:00:00Z",
  },
  {
    id: "topic-4",
    title: "Sustainable cloud infrastructure",
    description:
      "Research on carbon-aware computing, green hosting providers, and energy-efficient architecture patterns for cloud-native applications.",
    confidence: 0.45,
    freshness: "stale",
    sourceCount: 5,
    hasContradictions: true,
    lastUpdated: "2026-03-10T12:00:00Z",
  },
];

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

interface ResearchState {
  topics: readonly ResearchTopic[];
  loading: boolean;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Research() {
  const [state, setState] = useState<ResearchState>({
    topics: [],
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.research.listTopics({ pageSize: 50 });
        if (!cancelled) {
          setState({
            topics: res.items.length > 0 ? res.items : SAMPLE_TOPICS,
            loading: false,
          });
        }
      } catch {
        if (!cancelled) {
          setState({ topics: SAMPLE_TOPICS, loading: false });
        }
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleRefresh(topicId: string) {
    try {
      await api.research.refresh(topicId);
      setState((prev) => ({
        ...prev,
        topics: prev.topics.map((t) =>
          t.id === topicId ? { ...t, freshness: "fresh" as const } : t,
        ),
      }));
    } catch {
      // Silently fail in demo mode
    }
  }

  const freshCount = state.topics.filter((t) => t.freshness === "fresh").length;
  const staleCount = state.topics.filter((t) => t.freshness === "stale").length;

  return (
    <div className="page-container">
      <div className="mb-6">
        <h1 className="page-title">Research</h1>
        <p className="mt-1 text-sm text-neutral-500">
          Topics being tracked and researched on your behalf.
        </p>
      </div>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : (
        <div className="space-y-8">
          {/* Summary */}
          <div className="grid gap-4 sm:grid-cols-3">
            <div className="card-padded">
              <p className="text-sm font-medium text-neutral-600">Tracked topics</p>
              <p className="mt-1 text-3xl font-bold text-neutral-900">
                {state.topics.length}
              </p>
            </div>
            <div className="card-padded">
              <p className="text-sm font-medium text-neutral-600">Up to date</p>
              <p className="mt-1 text-3xl font-bold text-emerald-700">{freshCount}</p>
            </div>
            <div className="card-padded">
              <p className="text-sm font-medium text-neutral-600">Needs refresh</p>
              <p className="mt-1 text-3xl font-bold text-red-700">{staleCount}</p>
            </div>
          </div>

          {/* Topic cards */}
          <section aria-label="Research topics">
            <h2 className="section-title mb-4">Research briefs</h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-2">
              {state.topics.map((topic) => (
                <div key={topic.id} className="flex flex-col">
                  <ResearchCard
                    topic={topic}
                    onViewDetails={() => handleRefresh(topic.id)}
                  />
                </div>
              ))}
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
