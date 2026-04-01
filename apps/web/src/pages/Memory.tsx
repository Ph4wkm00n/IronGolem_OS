import React, { useEffect, useState } from "react";
import type { MemoryEntry } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  Placeholder data                                                   */
/* ------------------------------------------------------------------ */

const SAMPLE_ENTRIES: readonly MemoryEntry[] = [
  {
    id: "mem-1",
    content: "User prefers morning meetings before 11 AM",
    category: "preference",
    source: "Calendar Manager recipe",
    confidence: 0.95,
    createdAt: "2026-03-10T09:00:00Z",
    updatedAt: "2026-03-28T10:15:00Z",
    connections: ["mem-2", "mem-5"],
  },
  {
    id: "mem-2",
    content: "Weekly team standup is every Tuesday at 9:30 AM",
    category: "fact",
    source: "Calendar sync",
    confidence: 1.0,
    createdAt: "2026-03-05T08:00:00Z",
    updatedAt: "2026-03-29T09:30:00Z",
    connections: ["mem-1"],
  },
  {
    id: "mem-3",
    content: "User's primary email for work is on the corporate domain",
    category: "fact",
    source: "Email Triage recipe",
    confidence: 0.99,
    createdAt: "2026-03-01T12:00:00Z",
    updatedAt: "2026-03-25T14:00:00Z",
    connections: [],
  },
  {
    id: "mem-4",
    content: "User is interested in sustainable technology and green computing",
    category: "interest",
    source: "Research Monitor recipe",
    confidence: 0.78,
    createdAt: "2026-03-15T16:00:00Z",
    updatedAt: "2026-03-30T11:00:00Z",
    connections: ["mem-6"],
  },
  {
    id: "mem-5",
    content: "User avoids scheduling on Friday afternoons",
    category: "preference",
    source: "Calendar Manager recipe",
    confidence: 0.88,
    createdAt: "2026-03-12T13:00:00Z",
    updatedAt: "2026-03-27T15:30:00Z",
    connections: ["mem-1", "mem-2"],
  },
  {
    id: "mem-6",
    content: "Quarterly report is due on the last business day of each quarter",
    category: "fact",
    source: "Email Triage recipe",
    confidence: 0.92,
    createdAt: "2026-03-20T10:00:00Z",
    updatedAt: "2026-03-31T08:00:00Z",
    connections: ["mem-4"],
  },
];

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

type ViewMode = "list" | "graph";

interface MemoryState {
  entries: readonly MemoryEntry[];
  viewMode: ViewMode;
  loading: boolean;
}

/* ------------------------------------------------------------------ */
/*  Category styling                                                   */
/* ------------------------------------------------------------------ */

const CATEGORY_STYLES: Record<string, { label: string; color: string }> = {
  preference: { label: "Preference", color: "bg-indigo-100 text-indigo-800" },
  fact: { label: "Fact", color: "bg-emerald-100 text-emerald-800" },
  interest: { label: "Interest", color: "bg-amber-100 text-amber-800" },
};

function categoryStyle(category: string) {
  return (
    CATEGORY_STYLES[category] ?? {
      label: category,
      color: "bg-neutral-100 text-neutral-700",
    }
  );
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Memory() {
  const [state, setState] = useState<MemoryState>({
    entries: [],
    viewMode: "list",
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.memory.list({ pageSize: 100 });
        if (!cancelled) {
          setState((prev) => ({
            ...prev,
            entries: res.items.length > 0 ? res.items : SAMPLE_ENTRIES,
            loading: false,
          }));
        }
      } catch {
        if (!cancelled) {
          setState((prev) => ({
            ...prev,
            entries: SAMPLE_ENTRIES,
            loading: false,
          }));
        }
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  function toggleView(mode: ViewMode) {
    setState((prev) => ({ ...prev, viewMode: mode }));
  }

  return (
    <div className="page-container">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="page-title">Memory</h1>
          <p className="mt-1 text-sm text-neutral-500">
            What your assistant has learned and remembers about your preferences and routines.
          </p>
        </div>

        {/* View toggle */}
        <div className="flex rounded-lg border border-neutral-200 bg-white p-0.5">
          <button
            type="button"
            onClick={() => toggleView("list")}
            className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
              state.viewMode === "list"
                ? "bg-indigo-50 text-indigo-700"
                : "text-neutral-600 hover:text-neutral-900"
            }`}
          >
            List
          </button>
          <button
            type="button"
            onClick={() => toggleView("graph")}
            className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
              state.viewMode === "graph"
                ? "bg-indigo-50 text-indigo-700"
                : "text-neutral-600 hover:text-neutral-900"
            }`}
          >
            Graph
          </button>
        </div>
      </div>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : state.viewMode === "list" ? (
        <ListView entries={state.entries} />
      ) : (
        <GraphView entries={state.entries} />
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  List view                                                          */
/* ------------------------------------------------------------------ */

function ListView({ entries }: { entries: readonly MemoryEntry[] }) {
  return (
    <div className="space-y-3">
      {entries.map((entry) => {
        const cat = categoryStyle(entry.category);
        const confidencePercent = Math.round(entry.confidence * 100);

        return (
          <article
            key={entry.id}
            className="card-padded"
            aria-label={`Memory: ${entry.content}`}
          >
            <div className="flex items-start gap-3">
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-neutral-900">
                  {entry.content}
                </p>

                <div className="mt-2 flex items-center gap-3 flex-wrap">
                  {/* Category */}
                  <span
                    className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cat.color}`}
                  >
                    {cat.label}
                  </span>

                  {/* Confidence */}
                  <div className="flex items-center gap-1.5">
                    <div className="w-12 h-1.5 rounded-full bg-neutral-200 overflow-hidden">
                      <div
                        className={`h-full rounded-full ${
                          confidencePercent >= 80
                            ? "bg-emerald-500"
                            : confidencePercent >= 50
                              ? "bg-amber-500"
                              : "bg-red-500"
                        }`}
                        style={{ width: `${confidencePercent}%` }}
                      />
                    </div>
                    <span className="text-xs text-neutral-500">
                      {confidencePercent}%
                    </span>
                  </div>

                  {/* Source */}
                  <span className="text-xs text-neutral-400">
                    from {entry.source}
                  </span>

                  {/* Connections */}
                  {entry.connections.length > 0 && (
                    <span className="text-xs text-neutral-400">
                      {entry.connections.length}{" "}
                      {entry.connections.length === 1 ? "connection" : "connections"}
                    </span>
                  )}
                </div>
              </div>

              {/* Updated timestamp */}
              <time
                className="text-xs text-neutral-400 flex-shrink-0"
                dateTime={entry.updatedAt}
              >
                {formatDate(entry.updatedAt)}
              </time>
            </div>
          </article>
        );
      })}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Graph view (visual representation)                                 */
/* ------------------------------------------------------------------ */

function GraphView({ entries }: { entries: readonly MemoryEntry[] }) {
  const nodePositions = entries.map((_, i) => {
    const angle = (2 * Math.PI * i) / entries.length - Math.PI / 2;
    const radiusX = 280;
    const radiusY = 200;
    return {
      x: 400 + radiusX * Math.cos(angle),
      y: 250 + radiusY * Math.sin(angle),
    };
  });

  const idToIndex = new Map(entries.map((e, i) => [e.id, i]));

  return (
    <div className="card-padded">
      <div className="flex items-center justify-center">
        <svg
          viewBox="0 0 800 500"
          className="w-full max-w-3xl"
          aria-label="Memory connections graph"
        >
          {/* Connection lines */}
          {entries.map((entry, i) =>
            entry.connections.map((connId) => {
              const j = idToIndex.get(connId);
              if (j === undefined || j <= i) return null;
              return (
                <line
                  key={`${entry.id}-${connId}`}
                  x1={nodePositions[i].x}
                  y1={nodePositions[i].y}
                  x2={nodePositions[j].x}
                  y2={nodePositions[j].y}
                  stroke="#d1d5db"
                  strokeWidth={1.5}
                  strokeDasharray="4 4"
                />
              );
            }),
          )}

          {/* Nodes */}
          {entries.map((entry, i) => {
            const cat = categoryStyle(entry.category);
            const pos = nodePositions[i];
            const radius = 20 + entry.confidence * 15;

            return (
              <g key={entry.id}>
                <circle
                  cx={pos.x}
                  cy={pos.y}
                  r={radius}
                  className={
                    entry.category === "preference"
                      ? "fill-indigo-100 stroke-indigo-300"
                      : entry.category === "fact"
                        ? "fill-emerald-100 stroke-emerald-300"
                        : "fill-amber-100 stroke-amber-300"
                  }
                  strokeWidth={2}
                />
                <text
                  x={pos.x}
                  y={pos.y + radius + 16}
                  textAnchor="middle"
                  className="text-[11px] fill-neutral-600"
                >
                  {truncate(entry.content, 30)}
                </text>
                <text
                  x={pos.x}
                  y={pos.y + 4}
                  textAnchor="middle"
                  className="text-[10px] fill-neutral-500 font-medium"
                >
                  {cat.label}
                </text>
              </g>
            );
          })}
        </svg>
      </div>

      <p className="text-center text-xs text-neutral-400 mt-4">
        Nodes represent what your assistant knows. Lines show related memories.
        Larger nodes indicate higher confidence.
      </p>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
    });
  } catch {
    return iso;
  }
}

function truncate(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  return text.slice(0, maxLen - 1) + "\u2026";
}
