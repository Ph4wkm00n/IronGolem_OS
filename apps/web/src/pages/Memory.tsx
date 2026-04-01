import React, { useEffect, useState } from "react";
import type { MemoryEntry } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

type ViewMode = "list" | "graph";

interface MemoryState {
  entries: readonly MemoryEntry[];
  loading: boolean;
  viewMode: ViewMode;
  selectedId: string | null;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Memory() {
  const [state, setState] = useState<MemoryState>({
    entries: [],
    loading: true,
    viewMode: "list",
    selectedId: null,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.memory.list({ pageSize: 100 });
        if (!cancelled) {
          setState((prev) => ({ ...prev, loading: false, entries: res.items }));
        }
      } catch {
        if (!cancelled) setState((prev) => ({ ...prev, loading: false }));
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  const selectedEntry = state.entries.find((e) => e.id === state.selectedId) ?? null;

  return (
    <div className="page-container">
      <div className="flex items-center justify-between mb-6">
        <h1 className="page-title">Memory</h1>

        {/* View toggle */}
        <div className="flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
          <ViewToggleButton
            label="List"
            active={state.viewMode === "list"}
            onClick={() => setState((prev) => ({ ...prev, viewMode: "list" }))}
            icon={
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zM3.75 12h.007v.008H3.75V12zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zm-.375 5.25h.007v.008H3.75v-.008zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z" />
              </svg>
            }
          />
          <ViewToggleButton
            label="Graph"
            active={state.viewMode === "graph"}
            onClick={() => setState((prev) => ({ ...prev, viewMode: "graph" }))}
            icon={
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 14.25v2.25m3-4.5v4.5m3-6.75v6.75m3-9v9M6 20.25h12A2.25 2.25 0 0020.25 18V6A2.25 2.25 0 0018 3.75H6A2.25 2.25 0 003.75 6v12A2.25 2.25 0 006 20.25z" />
              </svg>
            }
          />
        </div>
      </div>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : state.entries.length === 0 ? (
        <div className="card-padded text-center py-12">
          <p className="text-neutral-500">No memories stored yet.</p>
          <p className="text-sm text-neutral-400 mt-1">
            As your assistants learn from your preferences, memories will appear here.
          </p>
        </div>
      ) : state.viewMode === "list" ? (
        <div className="grid gap-6 lg:grid-cols-3">
          {/* List view */}
          <div className="lg:col-span-2 space-y-2">
            {state.entries.map((entry) => (
              <MemoryListItem
                key={entry.id}
                entry={entry}
                isSelected={entry.id === state.selectedId}
                onSelect={() =>
                  setState((prev) => ({
                    ...prev,
                    selectedId: prev.selectedId === entry.id ? null : entry.id,
                  }))
                }
              />
            ))}
          </div>

          {/* Detail panel */}
          <div className="lg:col-span-1">
            {selectedEntry ? (
              <MemoryDetail entry={selectedEntry} allEntries={state.entries} />
            ) : (
              <div className="card-padded text-center text-sm text-neutral-500 sticky top-6">
                Select a memory to see details and connections.
              </div>
            )}
          </div>
        </div>
      ) : (
        /* Graph view placeholder */
        <GraphView entries={state.entries} />
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function ViewToggleButton({
  label,
  active,
  onClick,
  icon,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
        active
          ? "bg-indigo-100 text-indigo-700"
          : "text-neutral-500 hover:text-neutral-700"
      }`}
      aria-pressed={active}
    >
      {icon}
      {label}
    </button>
  );
}

function MemoryListItem({
  entry,
  isSelected,
  onSelect,
}: {
  entry: MemoryEntry;
  isSelected: boolean;
  onSelect: () => void;
}) {
  const confidencePercent = Math.round(entry.confidence * 100);

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left rounded-xl border p-4 transition-all ${
        isSelected
          ? "border-indigo-300 bg-indigo-50/50 ring-1 ring-indigo-200"
          : "border-neutral-200 bg-white hover:border-neutral-300 hover:shadow-sm"
      }`}
    >
      <p className="text-sm font-medium text-neutral-900 line-clamp-2">
        {entry.content}
      </p>
      <div className="mt-2 flex items-center gap-3 flex-wrap">
        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-neutral-100 text-neutral-600">
          {entry.category}
        </span>
        <span className="text-xs text-neutral-400">{confidencePercent}% confidence</span>
        {entry.connections.length > 0 && (
          <span className="text-xs text-neutral-400">
            {entry.connections.length} {entry.connections.length === 1 ? "connection" : "connections"}
          </span>
        )}
        <time className="text-xs text-neutral-400 ml-auto" dateTime={entry.updatedAt}>
          {new Date(entry.updatedAt).toLocaleDateString()}
        </time>
      </div>
    </button>
  );
}

function MemoryDetail({
  entry,
  allEntries,
}: {
  entry: MemoryEntry;
  allEntries: readonly MemoryEntry[];
}) {
  const connectedEntries = allEntries.filter((e) =>
    entry.connections.includes(e.id),
  );

  return (
    <div className="sticky top-6 space-y-4">
      <div className="card-padded">
        <h3 className="text-sm font-semibold text-neutral-900 mb-2">Memory detail</h3>
        <p className="text-sm text-neutral-700">{entry.content}</p>

        <dl className="mt-4 space-y-2">
          <div>
            <dt className="text-xs font-medium text-neutral-500">Category</dt>
            <dd className="text-sm text-neutral-900">{entry.category}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-neutral-500">Source</dt>
            <dd className="text-sm text-neutral-900">{entry.source}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-neutral-500">Confidence</dt>
            <dd className="text-sm text-neutral-900">
              {Math.round(entry.confidence * 100)}%
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-neutral-500">Created</dt>
            <dd className="text-sm text-neutral-900">
              {new Date(entry.createdAt).toLocaleDateString()}
            </dd>
          </div>
        </dl>
      </div>

      {connectedEntries.length > 0 && (
        <div className="card-padded">
          <h4 className="text-sm font-semibold text-neutral-900 mb-2">
            Connected memories
          </h4>
          <ul className="space-y-2" role="list">
            {connectedEntries.map((c) => (
              <li
                key={c.id}
                className="rounded-lg border border-neutral-100 px-3 py-2"
              >
                <p className="text-sm text-neutral-700 line-clamp-1">
                  {c.content}
                </p>
                <span className="text-xs text-neutral-400">{c.category}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function GraphView({ entries }: { entries: readonly MemoryEntry[] }) {
  const nodeCount = entries.length;
  const edgeCount = entries.reduce((sum, e) => sum + e.connections.length, 0) / 2;

  return (
    <div className="card-padded">
      <div className="flex flex-col items-center justify-center py-16 text-center">
        <svg
          className="w-24 h-24 text-neutral-300 mb-4"
          fill="none"
          viewBox="0 0 96 96"
          stroke="currentColor"
          strokeWidth={1}
        >
          {/* Nodes */}
          <circle cx="48" cy="20" r="6" className="fill-indigo-200 stroke-indigo-400" />
          <circle cx="24" cy="52" r="6" className="fill-emerald-200 stroke-emerald-400" />
          <circle cx="72" cy="52" r="6" className="fill-amber-200 stroke-amber-400" />
          <circle cx="36" cy="76" r="6" className="fill-blue-200 stroke-blue-400" />
          <circle cx="60" cy="76" r="6" className="fill-purple-200 stroke-purple-400" />
          {/* Edges */}
          <line x1="48" y1="26" x2="24" y2="46" className="stroke-neutral-300" />
          <line x1="48" y1="26" x2="72" y2="46" className="stroke-neutral-300" />
          <line x1="24" y1="58" x2="36" y2="70" className="stroke-neutral-300" />
          <line x1="72" y1="58" x2="60" y2="70" className="stroke-neutral-300" />
          <line x1="36" y1="76" x2="60" y2="76" className="stroke-neutral-300" />
        </svg>

        <h3 className="text-lg font-semibold text-neutral-900">Knowledge graph</h3>
        <p className="mt-1 text-sm text-neutral-500 max-w-md">
          {nodeCount} memories with {edgeCount} connections.
          The interactive graph view requires a canvas renderer and will be available in a future update.
        </p>

        {/* Summary table of categories */}
        <div className="mt-6 w-full max-w-sm">
          <CategorySummary entries={entries} />
        </div>
      </div>
    </div>
  );
}

function CategorySummary({ entries }: { entries: readonly MemoryEntry[] }) {
  const categoryCounts = entries.reduce<Record<string, number>>((acc, e) => {
    acc[e.category] = (acc[e.category] ?? 0) + 1;
    return acc;
  }, {});

  const sorted = Object.entries(categoryCounts).sort(([, a], [, b]) => b - a);

  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b border-neutral-200">
          <th className="py-2 text-left font-medium text-neutral-600">Category</th>
          <th className="py-2 text-right font-medium text-neutral-600">Count</th>
        </tr>
      </thead>
      <tbody>
        {sorted.map(([category, count]) => (
          <tr key={category} className="border-b border-neutral-100">
            <td className="py-2 text-neutral-900">{category}</td>
            <td className="py-2 text-right text-neutral-500">{count}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
