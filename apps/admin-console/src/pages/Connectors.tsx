import React, { useState } from "react";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

interface Connector {
  readonly id: string;
  readonly name: string;
  readonly type: "email" | "calendar" | "slack" | "telegram" | "github" | "jira" | "notion";
  readonly workspace: string;
  readonly status: "healthy" | "degraded" | "disconnected";
  readonly lastActivity: string;
  readonly credentials: string;
  readonly config: Record<string, string>;
  readonly healthHistory: { date: string; status: "healthy" | "degraded" | "disconnected" }[];
}

const CONNECTORS: Connector[] = [
  {
    id: "conn-001", name: "Gmail - Engineering", type: "email", workspace: "Engineering",
    status: "healthy", lastActivity: "2026-04-01T14:30:00Z", credentials: "oauth2:****...3kf9",
    config: { provider: "gmail", polling_interval: "30s", max_fetch: "50" },
    healthHistory: [
      { date: "2026-04-01", status: "healthy" }, { date: "2026-03-31", status: "healthy" },
      { date: "2026-03-30", status: "healthy" }, { date: "2026-03-29", status: "degraded" },
    ],
  },
  {
    id: "conn-002", name: "Slack - Sales", type: "slack", workspace: "Sales",
    status: "healthy", lastActivity: "2026-04-01T14:28:00Z", credentials: "bot-token:****...x8m2",
    config: { workspace_id: "T0123SALES", channels: "#general,#deals", event_types: "message,reaction" },
    healthHistory: [
      { date: "2026-04-01", status: "healthy" }, { date: "2026-03-31", status: "healthy" },
    ],
  },
  {
    id: "conn-003", name: "Google Calendar - HR", type: "calendar", workspace: "HR",
    status: "healthy", lastActivity: "2026-04-01T13:00:00Z", credentials: "oauth2:****...7bnq",
    config: { provider: "google", sync_direction: "bidirectional", lookahead_days: "14" },
    healthHistory: [
      { date: "2026-04-01", status: "healthy" }, { date: "2026-03-31", status: "healthy" },
    ],
  },
  {
    id: "conn-004", name: "GitHub - Engineering", type: "github", workspace: "Engineering",
    status: "degraded", lastActivity: "2026-04-01T12:15:00Z", credentials: "pat:****...p4kz",
    config: { org: "irongolem-dev", repos: "*", events: "push,pr,issue" },
    healthHistory: [
      { date: "2026-04-01", status: "degraded" }, { date: "2026-03-31", status: "healthy" },
      { date: "2026-03-30", status: "healthy" },
    ],
  },
  {
    id: "conn-005", name: "Telegram - Support", type: "telegram", workspace: "Marketing",
    status: "disconnected", lastActivity: "2026-03-28T09:00:00Z", credentials: "bot-token:****...disabled",
    config: { bot_username: "@irongolem_support_bot", allowed_chats: "restricted" },
    healthHistory: [
      { date: "2026-04-01", status: "disconnected" }, { date: "2026-03-31", status: "disconnected" },
      { date: "2026-03-30", status: "degraded" }, { date: "2026-03-29", status: "healthy" },
    ],
  },
  {
    id: "conn-006", name: "Jira - Engineering", type: "jira", workspace: "Engineering",
    status: "healthy", lastActivity: "2026-04-01T14:10:00Z", credentials: "api-key:****...j2w1",
    config: { instance: "irongolem.atlassian.net", project_keys: "IG,CORE", sync: "issues,comments" },
    healthHistory: [
      { date: "2026-04-01", status: "healthy" }, { date: "2026-03-31", status: "healthy" },
    ],
  },
  {
    id: "conn-007", name: "Notion - All", type: "notion", workspace: "Engineering",
    status: "healthy", lastActivity: "2026-04-01T13:55:00Z", credentials: "integration:****...n9q3",
    config: { databases: "3", pages: "auto-sync", polling: "60s" },
    healthHistory: [
      { date: "2026-04-01", status: "healthy" }, { date: "2026-03-31", status: "healthy" },
    ],
  },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const statusDot: Record<Connector["status"], string> = {
  healthy: "bg-emerald-400",
  degraded: "bg-amber-400",
  disconnected: "bg-red-400",
};

const statusLabel: Record<Connector["status"], string> = {
  healthy: "Healthy",
  degraded: "Degraded",
  disconnected: "Disconnected",
};

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function Connectors() {
  const [selectedConnector, setSelectedConnector] = useState<Connector | null>(null);

  return (
    <div className="page-container">
      <h2 className="page-title mb-4">Connectors</h2>

      {/* Connector table */}
      <div className="card overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
              <th className="px-3 py-2 font-medium">Name</th>
              <th className="px-3 py-2 font-medium">Type</th>
              <th className="px-3 py-2 font-medium">Workspace</th>
              <th className="px-3 py-2 font-medium">Status</th>
              <th className="px-3 py-2 font-medium">Last Activity</th>
              <th className="px-3 py-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-100">
            {CONNECTORS.map((conn) => (
              <tr
                key={conn.id}
                className={`hover:bg-neutral-50 cursor-pointer transition-colors ${selectedConnector?.id === conn.id ? "bg-indigo-50" : ""}`}
                onClick={() => setSelectedConnector(selectedConnector?.id === conn.id ? null : conn)}
              >
                <td className="px-3 py-2 font-medium text-neutral-900">{conn.name}</td>
                <td className="px-3 py-2">
                  <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold bg-neutral-100 text-neutral-600 capitalize">
                    {conn.type}
                  </span>
                </td>
                <td className="px-3 py-2 text-neutral-600">{conn.workspace}</td>
                <td className="px-3 py-2">
                  <span className="inline-flex items-center gap-1.5">
                    <span className={`w-2 h-2 rounded-full ${statusDot[conn.status]}`} />
                    <span className="text-neutral-600">{statusLabel[conn.status]}</span>
                  </span>
                </td>
                <td className="px-3 py-2 text-neutral-500">{formatTime(conn.lastActivity)}</td>
                <td className="px-3 py-2">
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); }}
                    className={`text-[10px] font-semibold px-2 py-0.5 rounded border transition-colors ${
                      conn.status === "disconnected"
                        ? "bg-emerald-50 text-emerald-700 border-emerald-200 hover:bg-emerald-100"
                        : "bg-red-50 text-red-700 border-red-200 hover:bg-red-100"
                    }`}
                  >
                    {conn.status === "disconnected" ? "Enable" : "Disable"}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Connector detail panel */}
      {selectedConnector && (
        <div className="card-padded mt-3">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold text-neutral-900">
              {selectedConnector.name}
            </h3>
            <button
              type="button"
              onClick={() => setSelectedConnector(null)}
              className="text-neutral-400 hover:text-neutral-600"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
            {/* Credentials */}
            <div>
              <p className="text-neutral-500 font-medium mb-1">Credentials</p>
              <p className="font-mono text-neutral-700 bg-neutral-50 rounded px-2 py-1">
                {selectedConnector.credentials}
              </p>
            </div>

            {/* Configuration */}
            <div>
              <p className="text-neutral-500 font-medium mb-1">Configuration</p>
              <div className="bg-neutral-50 rounded px-2 py-1 space-y-0.5">
                {Object.entries(selectedConnector.config).map(([key, value]) => (
                  <p key={key} className="font-mono text-neutral-700">
                    <span className="text-neutral-400">{key}:</span> {value}
                  </p>
                ))}
              </div>
            </div>

            {/* Health History */}
            <div>
              <p className="text-neutral-500 font-medium mb-1">Health History</p>
              <div className="space-y-1">
                {selectedConnector.healthHistory.map((h, idx) => (
                  <div key={idx} className="flex items-center gap-2">
                    <span className={`w-2 h-2 rounded-full ${statusDot[h.status]}`} />
                    <span className="text-neutral-600">{h.date}</span>
                    <span className="text-neutral-400 capitalize">{h.status}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
