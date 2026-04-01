import React, { useState } from "react";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

type EventType = "recipe" | "policy" | "connector" | "member" | "workspace" | "security" | "system";
type EventResult = "success" | "denied" | "error" | "rolled-back";

interface AuditEvent {
  readonly id: string;
  readonly timestamp: string;
  readonly actor: string;
  readonly actorType: "user" | "agent" | "system";
  readonly action: string;
  readonly target: string;
  readonly eventType: EventType;
  readonly workspace: string;
  readonly result: EventResult;
  readonly detail?: string;
}

const AUDIT_EVENTS: AuditEvent[] = [
  { id: "evt-001", timestamp: "2026-04-01T14:32:12Z", actor: "defender", actorType: "agent", action: "block.ssrf_attempt", target: "research-tool", eventType: "security", workspace: "Engineering", result: "success", detail: "Blocked outbound request to 169.254.169.254" },
  { id: "evt-002", timestamp: "2026-04-01T14:30:00Z", actor: "alice@irongolem.local", actorType: "user", action: "recipe.activate", target: "recipe-daily-standup", eventType: "recipe", workspace: "Engineering", result: "success" },
  { id: "evt-003", timestamp: "2026-04-01T14:28:45Z", actor: "connector-github", actorType: "system", action: "connector.sync_failed", target: "conn-004", eventType: "connector", workspace: "Engineering", result: "error", detail: "GitHub API rate limit exceeded (403)" },
  { id: "evt-004", timestamp: "2026-04-01T14:25:30Z", actor: "defender", actorType: "agent", action: "scan.prompt_injection", target: "email-inbound-42", eventType: "security", workspace: "Sales", result: "success", detail: "No injection detected" },
  { id: "evt-005", timestamp: "2026-04-01T14:20:10Z", actor: "bob@irongolem.local", actorType: "user", action: "workspace.list", target: "tenancy-api", eventType: "workspace", workspace: "Engineering", result: "success" },
  { id: "evt-006", timestamp: "2026-04-01T14:15:00Z", actor: "optimizer", actorType: "agent", action: "suggestion.generate", target: "inbox-squad-eng", eventType: "system", workspace: "Engineering", result: "success" },
  { id: "evt-007", timestamp: "2026-04-01T13:50:22Z", actor: "executor", actorType: "agent", action: "plan.execute", target: "plan-triage-batch", eventType: "recipe", workspace: "Engineering", result: "error", detail: "Step 3 failed: connector timeout" },
  { id: "evt-008", timestamp: "2026-04-01T13:45:00Z", actor: "carol@irongolem.local", actorType: "user", action: "policy.update", target: "global-tool-policy", eventType: "policy", workspace: "Sales", result: "success", detail: "Added http_request to denylist for Sales" },
  { id: "evt-009", timestamp: "2026-04-01T13:40:00Z", actor: "researcher", actorType: "agent", action: "research.complete", target: "topic-pricing-analysis", eventType: "recipe", workspace: "Engineering", result: "success" },
  { id: "evt-010", timestamp: "2026-04-01T13:18:00Z", actor: "router", actorType: "agent", action: "block.prompt_injection", target: "email-inbound-37", eventType: "security", workspace: "Sales", result: "denied", detail: "Prompt injection detected in email subject" },
  { id: "evt-011", timestamp: "2026-04-01T12:30:00Z", actor: "healer", actorType: "agent", action: "plan.rollback", target: "plan-calendar-fix", eventType: "recipe", workspace: "HR", result: "rolled-back", detail: "Calendar conflict could not be resolved automatically" },
  { id: "evt-012", timestamp: "2026-04-01T11:45:00Z", actor: "executor", actorType: "agent", action: "cross_tenant.access_attempt", target: "workspace-hr-data", eventType: "security", workspace: "HR", result: "denied", detail: "Quarantined: executor from Engineering attempted HR data access" },
  { id: "evt-013", timestamp: "2026-04-01T11:00:00Z", actor: "alice@irongolem.local", actorType: "user", action: "member.invite", target: "grace@irongolem.local", eventType: "member", workspace: "Engineering", result: "success" },
  { id: "evt-014", timestamp: "2026-04-01T10:22:00Z", actor: "defender", actorType: "agent", action: "block.rate_limit", target: "recipe-data-sync", eventType: "security", workspace: "Engineering", result: "success", detail: "Rate limit exceeded: 500 API calls in 60s" },
  { id: "evt-015", timestamp: "2026-04-01T09:00:00Z", actor: "system", actorType: "system", action: "backup.complete", target: "daily-backup-20260401", eventType: "system", workspace: "Engineering", result: "success" },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const eventTypeStyles: Record<EventType, string> = {
  recipe: "bg-indigo-50 text-indigo-700",
  policy: "bg-violet-50 text-violet-700",
  connector: "bg-sky-50 text-sky-700",
  member: "bg-teal-50 text-teal-700",
  workspace: "bg-neutral-100 text-neutral-700",
  security: "bg-red-50 text-red-700",
  system: "bg-neutral-50 text-neutral-600",
};

const resultStyles: Record<EventResult, string> = {
  success: "text-emerald-600",
  denied: "text-red-600",
  error: "text-amber-600",
  "rolled-back": "text-violet-600",
};

const actorTypeIcon: Record<AuditEvent["actorType"], string> = {
  user: "U",
  agent: "A",
  system: "S",
};

const actorTypeBg: Record<AuditEvent["actorType"], string> = {
  user: "bg-indigo-100 text-indigo-700",
  agent: "bg-amber-100 text-amber-700",
  system: "bg-neutral-100 text-neutral-600",
};

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit",
  });
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function AuditLog() {
  const [filterType, setFilterType] = useState<string>("all");
  const [filterWorkspace, setFilterWorkspace] = useState<string>("all");
  const [filterActor, setFilterActor] = useState<string>("");

  const workspaces = Array.from(new Set(AUDIT_EVENTS.map((e) => e.workspace)));
  const eventTypes: EventType[] = ["recipe", "policy", "connector", "member", "workspace", "security", "system"];

  const filteredEvents = AUDIT_EVENTS.filter((e) => {
    if (filterType !== "all" && e.eventType !== filterType) return false;
    if (filterWorkspace !== "all" && e.workspace !== filterWorkspace) return false;
    if (filterActor && !e.actor.toLowerCase().includes(filterActor.toLowerCase())) return false;
    return true;
  });

  return (
    <div className="page-container">
      <div className="flex items-center justify-between mb-4">
        <h2 className="page-title">Audit Log</h2>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md border border-neutral-300 text-neutral-600 hover:bg-neutral-50 transition-colors"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
          </svg>
          Export
        </button>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3 mb-4">
        <div>
          <label className="block text-[10px] font-medium text-neutral-500 mb-0.5">Event Type</label>
          <select
            value={filterType}
            onChange={(e) => setFilterType(e.target.value)}
            className="text-xs border border-neutral-300 rounded-md px-2 py-1 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          >
            <option value="all">All Types</option>
            {eventTypes.map((t) => (
              <option key={t} value={t} className="capitalize">{t}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-[10px] font-medium text-neutral-500 mb-0.5">Workspace</label>
          <select
            value={filterWorkspace}
            onChange={(e) => setFilterWorkspace(e.target.value)}
            className="text-xs border border-neutral-300 rounded-md px-2 py-1 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          >
            <option value="all">All Workspaces</option>
            {workspaces.map((w) => (
              <option key={w} value={w}>{w}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-[10px] font-medium text-neutral-500 mb-0.5">Actor</label>
          <input
            type="text"
            placeholder="Filter by actor..."
            value={filterActor}
            onChange={(e) => setFilterActor(e.target.value)}
            className="text-xs border border-neutral-300 rounded-md px-2 py-1 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          />
        </div>
        <div className="flex items-end">
          <span className="text-[10px] text-neutral-400 pb-1">
            {filteredEvents.length} of {AUDIT_EVENTS.length} events
          </span>
        </div>
      </div>

      {/* Event list */}
      <div className="card overflow-hidden">
        <table className="w-full text-xs">
          <thead>
            <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
              <th className="px-3 py-2 font-medium w-36">Timestamp</th>
              <th className="px-3 py-2 font-medium">Actor</th>
              <th className="px-3 py-2 font-medium">Action</th>
              <th className="px-3 py-2 font-medium">Target</th>
              <th className="px-3 py-2 font-medium">Type</th>
              <th className="px-3 py-2 font-medium">Workspace</th>
              <th className="px-3 py-2 font-medium">Result</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-100">
            {filteredEvents.map((evt) => (
              <tr key={evt.id} className="hover:bg-neutral-50 transition-colors group">
                <td className="px-3 py-2 text-neutral-500 tabular-nums whitespace-nowrap">{formatTime(evt.timestamp)}</td>
                <td className="px-3 py-2">
                  <span className="inline-flex items-center gap-1.5">
                    <span className={`flex items-center justify-center w-4 h-4 rounded text-[8px] font-bold ${actorTypeBg[evt.actorType]}`}>
                      {actorTypeIcon[evt.actorType]}
                    </span>
                    <span className="text-neutral-800 truncate max-w-[120px]">{evt.actor}</span>
                  </span>
                </td>
                <td className="px-3 py-2 font-mono text-neutral-700">{evt.action}</td>
                <td className="px-3 py-2 text-neutral-600 truncate max-w-[120px]">{evt.target}</td>
                <td className="px-3 py-2">
                  <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold capitalize ${eventTypeStyles[evt.eventType]}`}>
                    {evt.eventType}
                  </span>
                </td>
                <td className="px-3 py-2 text-neutral-600">{evt.workspace}</td>
                <td className="px-3 py-2">
                  <span className={`font-semibold capitalize ${resultStyles[evt.result]}`}>
                    {evt.result}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Detail tooltips appear on hover via group */}
      {filteredEvents.some((e) => e.detail) && (
        <div className="mt-3 card-padded">
          <h3 className="text-xs font-semibold text-neutral-900 mb-2">Event Details</h3>
          <div className="space-y-1.5 text-xs">
            {filteredEvents.filter((e) => e.detail).map((evt) => (
              <div key={evt.id} className="flex items-start gap-2">
                <span className="text-neutral-400 tabular-nums shrink-0 w-16">{evt.id}</span>
                <span className={`font-semibold shrink-0 w-16 capitalize ${resultStyles[evt.result]}`}>{evt.result}</span>
                <span className="text-neutral-600">{evt.detail}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
