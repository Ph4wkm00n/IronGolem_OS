import React from "react";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

const STATS = [
  { label: "Workspaces", value: "12", change: "+2 this month", color: "text-indigo-600" },
  { label: "Members", value: "47", change: "+5 this month", color: "text-emerald-600" },
  { label: "Active Recipes", value: "83", change: "6 pending approval", color: "text-amber-600" },
  { label: "Connectors", value: "28", change: "2 degraded", color: "text-violet-600" },
];

const SERVICE_STATUS: { name: string; status: "healthy" | "degraded" | "down" }[] = [
  { name: "Gateway", status: "healthy" },
  { name: "Scheduler", status: "healthy" },
  { name: "Health Monitor", status: "healthy" },
  { name: "Defense Service", status: "healthy" },
  { name: "Research Service", status: "degraded" },
  { name: "Optimizer", status: "healthy" },
  { name: "Tenancy API", status: "healthy" },
];

const RECENT_SECURITY_EVENTS: {
  id: string;
  action: string;
  agent: string;
  workspace: string;
  result: "blocked" | "quarantined";
  timestamp: string;
}[] = [
  { id: "sec-001", action: "Attempted SSRF via research tool", agent: "researcher", workspace: "Engineering", result: "blocked", timestamp: "2026-04-01T14:32:00Z" },
  { id: "sec-002", action: "Prompt injection detected in email", agent: "router", workspace: "Sales", result: "blocked", timestamp: "2026-04-01T13:18:00Z" },
  { id: "sec-003", action: "Unauthorized cross-tenant data access", agent: "executor", workspace: "HR", result: "quarantined", timestamp: "2026-04-01T11:45:00Z" },
  { id: "sec-004", action: "Excessive API call rate from recipe", agent: "executor", workspace: "Engineering", result: "blocked", timestamp: "2026-04-01T10:22:00Z" },
  { id: "sec-005", action: "Suspicious file write attempt", agent: "executor", workspace: "Finance", result: "quarantined", timestamp: "2026-03-31T22:10:00Z" },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const statusDot: Record<string, string> = {
  healthy: "bg-emerald-400",
  degraded: "bg-amber-400",
  down: "bg-red-400",
};

const resultBadge: Record<string, string> = {
  blocked: "bg-red-50 text-red-700 border-red-200",
  quarantined: "bg-purple-50 text-purple-700 border-purple-200",
};

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function Dashboard() {
  return (
    <div className="page-container">
      <h2 className="page-title mb-4">Dashboard</h2>

      {/* Quick stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
        {STATS.map((s) => (
          <div key={s.label} className="card-padded">
            <p className="text-xs font-medium text-neutral-500 uppercase tracking-wide">{s.label}</p>
            <p className={`text-2xl font-bold mt-1 ${s.color}`}>{s.value}</p>
            <p className="text-xs text-neutral-400 mt-0.5">{s.change}</p>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
        {/* Service health */}
        <div className="card-padded">
          <h3 className="section-title mb-2">Service Health</h3>
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-neutral-500 border-b border-neutral-100">
                <th className="pb-1.5 font-medium">Service</th>
                <th className="pb-1.5 font-medium">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-50">
              {SERVICE_STATUS.map((svc) => (
                <tr key={svc.name}>
                  <td className="py-1.5 text-neutral-800">{svc.name}</td>
                  <td className="py-1.5">
                    <span className="inline-flex items-center gap-1.5">
                      <span className={`w-2 h-2 rounded-full ${statusDot[svc.status]}`} />
                      <span className="capitalize text-neutral-600">{svc.status}</span>
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Recent security events */}
        <div className="card-padded">
          <h3 className="section-title mb-2">Recent Security Events</h3>
          <div className="space-y-2">
            {RECENT_SECURITY_EVENTS.map((evt) => (
              <div key={evt.id} className="flex items-start gap-2 text-xs">
                <span className={`mt-0.5 inline-flex shrink-0 items-center px-1.5 py-0.5 rounded border text-[10px] font-semibold ${resultBadge[evt.result]}`}>
                  {evt.result}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-neutral-800 truncate">{evt.action}</p>
                  <p className="text-neutral-400">
                    {evt.workspace} &middot; {evt.agent} &middot; {formatTime(evt.timestamp)}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
