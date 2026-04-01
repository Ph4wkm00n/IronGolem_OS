import React, { useState } from "react";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

interface Span {
  readonly spanId: string;
  readonly service: string;
  readonly operation: string;
  readonly startMs: number;
  readonly durationMs: number;
  readonly status: "ok" | "error";
}

interface Trace {
  readonly traceId: string;
  readonly rootService: string;
  readonly rootOperation: string;
  readonly durationMs: number;
  readonly status: "ok" | "error";
  readonly spanCount: number;
  readonly startedAt: string;
  readonly spans: readonly Span[];
}

const TRACES: Trace[] = [
  {
    traceId: "t-7a3f1b2c", rootService: "gateway", rootOperation: "POST /api/recipes/activate",
    durationMs: 342, status: "ok", spanCount: 6, startedAt: "2026-04-01T14:32:12Z",
    spans: [
      { spanId: "s-001", service: "gateway", operation: "auth.verify", startMs: 0, durationMs: 12, status: "ok" },
      { spanId: "s-002", service: "gateway", operation: "policy.evaluate", startMs: 12, durationMs: 28, status: "ok" },
      { spanId: "s-003", service: "scheduler", operation: "recipe.activate", startMs: 40, durationMs: 85, status: "ok" },
      { spanId: "s-004", service: "runtime", operation: "plan.create", startMs: 125, durationMs: 120, status: "ok" },
      { spanId: "s-005", service: "runtime", operation: "checkpoint.save", startMs: 245, durationMs: 65, status: "ok" },
      { spanId: "s-006", service: "gateway", operation: "response.send", startMs: 310, durationMs: 32, status: "ok" },
    ],
  },
  {
    traceId: "t-9e4d2a8f", rootService: "scheduler", rootOperation: "heartbeat.check",
    durationMs: 89, status: "ok", spanCount: 3, startedAt: "2026-04-01T14:31:00Z",
    spans: [
      { spanId: "s-010", service: "scheduler", operation: "agents.poll", startMs: 0, durationMs: 45, status: "ok" },
      { spanId: "s-011", service: "health", operation: "status.aggregate", startMs: 45, durationMs: 32, status: "ok" },
      { spanId: "s-012", service: "scheduler", operation: "heartbeat.report", startMs: 77, durationMs: 12, status: "ok" },
    ],
  },
  {
    traceId: "t-2c8b5e1d", rootService: "gateway", rootOperation: "POST /api/connectors/sync",
    durationMs: 1247, status: "error", spanCount: 5, startedAt: "2026-04-01T14:28:45Z",
    spans: [
      { spanId: "s-020", service: "gateway", operation: "auth.verify", startMs: 0, durationMs: 8, status: "ok" },
      { spanId: "s-021", service: "gateway", operation: "policy.evaluate", startMs: 8, durationMs: 15, status: "ok" },
      { spanId: "s-022", service: "connector-github", operation: "api.fetch", startMs: 23, durationMs: 1100, status: "error" },
      { spanId: "s-023", service: "health", operation: "connector.mark_degraded", startMs: 1123, durationMs: 45, status: "ok" },
      { spanId: "s-024", service: "gateway", operation: "error.respond", startMs: 1168, durationMs: 79, status: "ok" },
    ],
  },
  {
    traceId: "t-5f1a7c3e", rootService: "defense", rootOperation: "scan.prompt_injection",
    durationMs: 67, status: "ok", spanCount: 2, startedAt: "2026-04-01T14:25:30Z",
    spans: [
      { spanId: "s-030", service: "defense", operation: "input.analyze", startMs: 0, durationMs: 52, status: "ok" },
      { spanId: "s-031", service: "defense", operation: "result.log", startMs: 52, durationMs: 15, status: "ok" },
    ],
  },
  {
    traceId: "t-8d3e9b2a", rootService: "gateway", rootOperation: "GET /api/workspaces",
    durationMs: 156, status: "ok", spanCount: 3, startedAt: "2026-04-01T14:20:10Z",
    spans: [
      { spanId: "s-040", service: "gateway", operation: "auth.verify", startMs: 0, durationMs: 10, status: "ok" },
      { spanId: "s-041", service: "tenancy", operation: "workspaces.list", startMs: 10, durationMs: 128, status: "ok" },
      { spanId: "s-042", service: "gateway", operation: "response.send", startMs: 138, durationMs: 18, status: "ok" },
    ],
  },
  {
    traceId: "t-4b2c8f1e", rootService: "optimizer", rootOperation: "suggestions.generate",
    durationMs: 2340, status: "ok", spanCount: 4, startedAt: "2026-04-01T14:15:00Z",
    spans: [
      { spanId: "s-050", service: "optimizer", operation: "metrics.collect", startMs: 0, durationMs: 450, status: "ok" },
      { spanId: "s-051", service: "optimizer", operation: "patterns.analyze", startMs: 450, durationMs: 1200, status: "ok" },
      { spanId: "s-052", service: "optimizer", operation: "suggestions.rank", startMs: 1650, durationMs: 580, status: "ok" },
      { spanId: "s-053", service: "optimizer", operation: "result.store", startMs: 2230, durationMs: 110, status: "ok" },
    ],
  },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    hour: "2-digit", minute: "2-digit", second: "2-digit",
  });
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function Traces() {
  const [selectedTrace, setSelectedTrace] = useState<Trace | null>(null);
  const [filterService, setFilterService] = useState<string>("all");
  const [filterStatus, setFilterStatus] = useState<string>("all");

  const services = Array.from(new Set(TRACES.map((t) => t.rootService)));

  const filteredTraces = TRACES.filter((t) => {
    if (filterService !== "all" && t.rootService !== filterService) return false;
    if (filterStatus !== "all" && t.status !== filterStatus) return false;
    return true;
  });

  return (
    <div className="page-container">
      <h2 className="page-title mb-4">Traces</h2>

      {/* Filters */}
      <div className="flex gap-3 mb-4">
        <div>
          <label className="block text-[10px] font-medium text-neutral-500 mb-0.5">Service</label>
          <select
            value={filterService}
            onChange={(e) => setFilterService(e.target.value)}
            className="text-xs border border-neutral-300 rounded-md px-2 py-1 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          >
            <option value="all">All Services</option>
            {services.map((s) => (
              <option key={s} value={s}>{s}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-[10px] font-medium text-neutral-500 mb-0.5">Status</label>
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
            className="text-xs border border-neutral-300 rounded-md px-2 py-1 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          >
            <option value="all">All</option>
            <option value="ok">OK</option>
            <option value="error">Error</option>
          </select>
        </div>
      </div>

      {/* Trace list */}
      <div className="card overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
              <th className="px-3 py-2 font-medium">Trace ID</th>
              <th className="px-3 py-2 font-medium">Service</th>
              <th className="px-3 py-2 font-medium">Operation</th>
              <th className="px-3 py-2 font-medium">Duration</th>
              <th className="px-3 py-2 font-medium">Spans</th>
              <th className="px-3 py-2 font-medium">Status</th>
              <th className="px-3 py-2 font-medium">Time</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-100">
            {filteredTraces.map((trace) => (
              <tr
                key={trace.traceId}
                className={`hover:bg-neutral-50 cursor-pointer transition-colors ${selectedTrace?.traceId === trace.traceId ? "bg-indigo-50" : ""}`}
                onClick={() => setSelectedTrace(selectedTrace?.traceId === trace.traceId ? null : trace)}
              >
                <td className="px-3 py-2 font-mono text-neutral-700">{trace.traceId}</td>
                <td className="px-3 py-2 text-neutral-600">{trace.rootService}</td>
                <td className="px-3 py-2 font-mono text-neutral-800">{trace.rootOperation}</td>
                <td className="px-3 py-2 tabular-nums text-neutral-600">{formatDuration(trace.durationMs)}</td>
                <td className="px-3 py-2 text-neutral-600">{trace.spanCount}</td>
                <td className="px-3 py-2">
                  <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold ${
                    trace.status === "ok" ? "bg-emerald-50 text-emerald-700" : "bg-red-50 text-red-700"
                  }`}>
                    {trace.status === "ok" ? "OK" : "Error"}
                  </span>
                </td>
                <td className="px-3 py-2 text-neutral-500">{formatTime(trace.startedAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Trace detail — waterfall view */}
      {selectedTrace && (
        <div className="card-padded mt-3">
          <div className="flex items-center justify-between mb-3">
            <div>
              <h3 className="text-sm font-semibold text-neutral-900">
                Trace {selectedTrace.traceId}
              </h3>
              <p className="text-xs text-neutral-500">
                {selectedTrace.rootOperation} -- {formatDuration(selectedTrace.durationMs)} total
              </p>
            </div>
            <button
              type="button"
              onClick={() => setSelectedTrace(null)}
              className="text-neutral-400 hover:text-neutral-600"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* Waterfall timeline */}
          <div className="space-y-1">
            {selectedTrace.spans.map((span) => {
              const leftPct = (span.startMs / selectedTrace.durationMs) * 100;
              const widthPct = Math.max((span.durationMs / selectedTrace.durationMs) * 100, 1);
              return (
                <div key={span.spanId} className="flex items-center gap-2 text-xs">
                  <div className="w-32 shrink-0 truncate text-neutral-500">{span.service}</div>
                  <div className="w-40 shrink-0 truncate font-mono text-neutral-700">{span.operation}</div>
                  <div className="flex-1 relative h-5 bg-neutral-50 rounded overflow-hidden">
                    <div
                      className={`absolute top-0.5 bottom-0.5 rounded ${
                        span.status === "ok" ? "bg-indigo-400" : "bg-red-400"
                      }`}
                      style={{ left: `${leftPct}%`, width: `${widthPct}%`, minWidth: "2px" }}
                    />
                  </div>
                  <div className="w-16 shrink-0 text-right tabular-nums text-neutral-500">
                    {formatDuration(span.durationMs)}
                  </div>
                </div>
              );
            })}
          </div>

          {/* Timeline scale */}
          <div className="flex items-center justify-between mt-1 text-[10px] text-neutral-400 pl-[calc(8rem+10rem+0.5rem)]">
            <span>0ms</span>
            <span>{formatDuration(Math.round(selectedTrace.durationMs / 2))}</span>
            <span>{formatDuration(selectedTrace.durationMs)}</span>
          </div>
        </div>
      )}
    </div>
  );
}
