import React, { useState } from "react";
import type { AgentRole } from "@irongolem/schema";
import { agentRoleLabel } from "@irongolem/schema";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

interface SquadTemplate {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly roles: readonly AgentRole[];
  readonly defaultTrustLevel: "low" | "medium" | "high";
}

interface ActiveSquad {
  readonly id: string;
  readonly templateId: string;
  readonly templateName: string;
  readonly workspace: string;
  readonly status: "active" | "idle" | "paused";
  readonly trustLevel: "low" | "medium" | "high";
  readonly runsToday: number;
  readonly lastRun: string;
  readonly tools: readonly string[];
}

interface RunHistoryEntry {
  readonly id: string;
  readonly startedAt: string;
  readonly duration: string;
  readonly status: "completed" | "failed" | "rolled-back";
  readonly trigger: string;
}

const SQUAD_TEMPLATES: SquadTemplate[] = [
  { id: "tpl-inbox", name: "Inbox Squad", description: "Triage, route, and auto-respond to incoming messages", roles: ["router", "executor", "narrator"], defaultTrustLevel: "medium" },
  { id: "tpl-research", name: "Research Squad", description: "Deep research with contradiction detection and freshness tracking", roles: ["researcher", "verifier", "narrator"], defaultTrustLevel: "medium" },
  { id: "tpl-ops", name: "Ops Squad", description: "Operational automation, scheduling, and monitoring", roles: ["planner", "executor", "healer", "verifier"], defaultTrustLevel: "high" },
  { id: "tpl-security", name: "Security Squad", description: "Threat detection, defense, and incident response", roles: ["defender", "verifier", "narrator"], defaultTrustLevel: "low" },
  { id: "tpl-ea", name: "Executive Assistant Squad", description: "Calendar management, meeting prep, and follow-ups", roles: ["planner", "executor", "narrator", "router"], defaultTrustLevel: "medium" },
];

const ACTIVE_SQUADS: ActiveSquad[] = [
  { id: "sq-001", templateId: "tpl-inbox", templateName: "Inbox Squad", workspace: "Engineering", status: "active", trustLevel: "medium", runsToday: 47, lastRun: "2026-04-01T14:28:00Z", tools: ["email_read", "email_send", "slack_post", "classify"] },
  { id: "sq-002", templateId: "tpl-inbox", templateName: "Inbox Squad", workspace: "Sales", status: "active", trustLevel: "medium", runsToday: 32, lastRun: "2026-04-01T14:25:00Z", tools: ["email_read", "email_send", "crm_update"] },
  { id: "sq-003", templateId: "tpl-research", templateName: "Research Squad", workspace: "Engineering", status: "active", trustLevel: "medium", runsToday: 8, lastRun: "2026-04-01T13:40:00Z", tools: ["web_search", "file_read", "file_write", "summarize"] },
  { id: "sq-004", templateId: "tpl-ops", templateName: "Ops Squad", workspace: "Engineering", status: "idle", trustLevel: "high", runsToday: 3, lastRun: "2026-04-01T11:00:00Z", tools: ["scheduler", "monitor", "deploy", "rollback"] },
  { id: "sq-005", templateId: "tpl-security", templateName: "Security Squad", workspace: "Engineering", status: "active", trustLevel: "low", runsToday: 156, lastRun: "2026-04-01T14:32:00Z", tools: ["scan", "block", "quarantine", "alert"] },
  { id: "sq-006", templateId: "tpl-ea", templateName: "EA Squad", workspace: "HR", status: "paused", trustLevel: "medium", runsToday: 0, lastRun: "2026-03-31T17:00:00Z", tools: ["calendar_read", "calendar_write", "email_send"] },
];

const SAMPLE_RUN_HISTORY: RunHistoryEntry[] = [
  { id: "run-001", startedAt: "2026-04-01T14:28:00Z", duration: "2.3s", status: "completed", trigger: "New email from client@example.com" },
  { id: "run-002", startedAt: "2026-04-01T14:15:00Z", duration: "1.1s", status: "completed", trigger: "Slack message in #engineering" },
  { id: "run-003", startedAt: "2026-04-01T13:50:00Z", duration: "4.7s", status: "failed", trigger: "Auto-triage batch" },
  { id: "run-004", startedAt: "2026-04-01T13:40:00Z", duration: "12.4s", status: "completed", trigger: "Research request: pricing analysis" },
  { id: "run-005", startedAt: "2026-04-01T12:30:00Z", duration: "0.8s", status: "rolled-back", trigger: "Calendar conflict resolution" },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const statusStyles: Record<ActiveSquad["status"], string> = {
  active: "bg-emerald-50 text-emerald-700 border-emerald-200",
  idle: "bg-neutral-100 text-neutral-600 border-neutral-200",
  paused: "bg-amber-50 text-amber-700 border-amber-200",
};

const trustStyles: Record<ActiveSquad["trustLevel"], string> = {
  low: "text-red-600",
  medium: "text-amber-600",
  high: "text-emerald-600",
};

const runStatusStyles: Record<RunHistoryEntry["status"], string> = {
  completed: "bg-emerald-50 text-emerald-700",
  failed: "bg-red-50 text-red-700",
  "rolled-back": "bg-amber-50 text-amber-700",
};

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString("en-US", {
    month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function Squads() {
  const [selectedSquad, setSelectedSquad] = useState<ActiveSquad | null>(null);
  const [activeTab, setActiveTab] = useState<"active" | "templates">("active");

  return (
    <div className="page-container">
      <h2 className="page-title mb-4">Squads</h2>

      {/* Tabs */}
      <div className="flex gap-1 mb-4 border-b border-neutral-200">
        <button
          type="button"
          onClick={() => setActiveTab("active")}
          className={`px-3 py-1.5 text-xs font-medium rounded-t-md transition-colors -mb-px ${
            activeTab === "active"
              ? "bg-white border border-neutral-200 border-b-white text-indigo-700"
              : "text-neutral-500 hover:text-neutral-700"
          }`}
        >
          Active Squads ({ACTIVE_SQUADS.length})
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("templates")}
          className={`px-3 py-1.5 text-xs font-medium rounded-t-md transition-colors -mb-px ${
            activeTab === "templates"
              ? "bg-white border border-neutral-200 border-b-white text-indigo-700"
              : "text-neutral-500 hover:text-neutral-700"
          }`}
        >
          Templates ({SQUAD_TEMPLATES.length})
        </button>
      </div>

      {/* Active squads */}
      {activeTab === "active" && (
        <>
          <div className="card overflow-hidden">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
                  <th className="px-3 py-2 font-medium">Squad</th>
                  <th className="px-3 py-2 font-medium">Workspace</th>
                  <th className="px-3 py-2 font-medium">Status</th>
                  <th className="px-3 py-2 font-medium">Trust</th>
                  <th className="px-3 py-2 font-medium">Runs Today</th>
                  <th className="px-3 py-2 font-medium">Last Run</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-100">
                {ACTIVE_SQUADS.map((squad) => (
                  <tr
                    key={squad.id}
                    className={`hover:bg-neutral-50 cursor-pointer transition-colors ${selectedSquad?.id === squad.id ? "bg-indigo-50" : ""}`}
                    onClick={() => setSelectedSquad(selectedSquad?.id === squad.id ? null : squad)}
                  >
                    <td className="px-3 py-2 font-medium text-neutral-900">{squad.templateName}</td>
                    <td className="px-3 py-2 text-neutral-600">{squad.workspace}</td>
                    <td className="px-3 py-2">
                      <span className={`inline-flex items-center px-1.5 py-0.5 rounded border text-[10px] font-semibold capitalize ${statusStyles[squad.status]}`}>
                        {squad.status}
                      </span>
                    </td>
                    <td className="px-3 py-2">
                      <span className={`font-semibold capitalize ${trustStyles[squad.trustLevel]}`}>
                        {squad.trustLevel}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-neutral-600">{squad.runsToday}</td>
                    <td className="px-3 py-2 text-neutral-500">{formatTime(squad.lastRun)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Squad detail */}
          {selectedSquad && (
            <div className="card-padded mt-3">
              <div className="flex items-center justify-between mb-3">
                <h3 className="text-sm font-semibold text-neutral-900">
                  {selectedSquad.templateName} — {selectedSquad.workspace}
                </h3>
                <button
                  type="button"
                  onClick={() => setSelectedSquad(null)}
                  className="text-neutral-400 hover:text-neutral-600"
                >
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs mb-4">
                {/* Member roles */}
                <div>
                  <p className="text-neutral-500 font-medium mb-1">Member Roles</p>
                  <div className="flex flex-wrap gap-1">
                    {SQUAD_TEMPLATES.find((t) => t.id === selectedSquad.templateId)?.roles.map((role) => (
                      <span key={role} className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold bg-indigo-50 text-indigo-700">
                        {agentRoleLabel[role]}
                      </span>
                    ))}
                  </div>
                </div>

                {/* Tools */}
                <div>
                  <p className="text-neutral-500 font-medium mb-1">Tools</p>
                  <div className="flex flex-wrap gap-1">
                    {selectedSquad.tools.map((tool) => (
                      <span key={tool} className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-mono bg-neutral-100 text-neutral-600">
                        {tool}
                      </span>
                    ))}
                  </div>
                </div>

                {/* Trust level */}
                <div>
                  <p className="text-neutral-500 font-medium mb-1">Trust Level</p>
                  <p className={`font-semibold capitalize ${trustStyles[selectedSquad.trustLevel]}`}>
                    {selectedSquad.trustLevel}
                  </p>
                </div>
              </div>

              {/* Run history */}
              <div>
                <p className="text-neutral-500 font-medium text-xs mb-2">Recent Runs</p>
                <div className="space-y-1.5">
                  {SAMPLE_RUN_HISTORY.map((run) => (
                    <div key={run.id} className="flex items-center gap-2 text-xs">
                      <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold ${runStatusStyles[run.status]}`}>
                        {run.status}
                      </span>
                      <span className="text-neutral-500 tabular-nums">{run.duration}</span>
                      <span className="text-neutral-700 truncate flex-1">{run.trigger}</span>
                      <span className="text-neutral-400 shrink-0">{formatTime(run.startedAt)}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </>
      )}

      {/* Squad templates */}
      {activeTab === "templates" && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {SQUAD_TEMPLATES.map((tpl) => (
            <div key={tpl.id} className="card-padded">
              <h4 className="text-sm font-semibold text-neutral-900 mb-1">{tpl.name}</h4>
              <p className="text-xs text-neutral-500 mb-3">{tpl.description}</p>
              <div className="mb-2">
                <p className="text-[10px] text-neutral-400 font-medium mb-1">ROLES</p>
                <div className="flex flex-wrap gap-1">
                  {tpl.roles.map((role) => (
                    <span key={role} className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold bg-indigo-50 text-indigo-700">
                      {agentRoleLabel[role]}
                    </span>
                  ))}
                </div>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[10px] text-neutral-400">
                  Default trust: <span className={`font-semibold capitalize ${trustStyles[tpl.defaultTrustLevel]}`}>{tpl.defaultTrustLevel}</span>
                </span>
                <span className="text-[10px] text-neutral-400">
                  {ACTIVE_SQUADS.filter((s) => s.templateId === tpl.id).length} active
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
