import React, { useState } from "react";
import type { PolicyLayer, PolicyEffect, AgentRole } from "@irongolem/schema";
import { policyLayerLabel, policyLayerDescription, agentRoleLabel } from "@irongolem/schema";

/* ------------------------------------------------------------------ */
/*  Sample data                                                        */
/* ------------------------------------------------------------------ */

const LAYER_ORDER: readonly PolicyLayer[] = [
  "gateway-identity",
  "global-tool-policy",
  "per-agent-permissions",
  "per-channel-restrictions",
  "admin-only-controls",
];

const AGENT_ROLES: readonly AgentRole[] = [
  "planner", "executor", "verifier", "researcher",
  "defender", "healer", "optimizer", "narrator", "router",
];

type Capability = "read" | "write" | "execute" | "delete" | "approve";
const CAPABILITIES: readonly Capability[] = ["read", "write", "execute", "delete", "approve"];

const PERMISSION_MATRIX: Record<AgentRole, Record<Capability, PolicyEffect>> = {
  planner:    { read: "allow", write: "allow", execute: "deny", delete: "deny", approve: "deny" },
  executor:   { read: "allow", write: "allow", execute: "allow", delete: "require-approval", approve: "deny" },
  verifier:   { read: "allow", write: "deny", execute: "deny", delete: "deny", approve: "allow" },
  researcher: { read: "allow", write: "allow", execute: "deny", delete: "deny", approve: "deny" },
  defender:   { read: "allow", write: "deny", execute: "allow", delete: "deny", approve: "require-approval" },
  healer:     { read: "allow", write: "allow", execute: "allow", delete: "deny", approve: "deny" },
  optimizer:  { read: "allow", write: "require-approval", execute: "require-approval", delete: "deny", approve: "deny" },
  narrator:   { read: "allow", write: "deny", execute: "deny", delete: "deny", approve: "deny" },
  router:     { read: "allow", write: "deny", execute: "deny", delete: "deny", approve: "deny" },
};

interface ToolRule {
  readonly tool: string;
  readonly effect: "allow" | "deny";
  readonly scope: "global" | string;
}

const TOOL_RULES: ToolRule[] = [
  { tool: "file_read", effect: "allow", scope: "global" },
  { tool: "file_write", effect: "allow", scope: "global" },
  { tool: "file_delete", effect: "deny", scope: "global" },
  { tool: "http_request", effect: "allow", scope: "Engineering" },
  { tool: "http_request", effect: "deny", scope: "HR" },
  { tool: "email_send", effect: "allow", scope: "global" },
  { tool: "shell_exec", effect: "deny", scope: "global" },
  { tool: "db_query", effect: "allow", scope: "Engineering" },
  { tool: "db_query", effect: "deny", scope: "Sales" },
  { tool: "calendar_write", effect: "allow", scope: "global" },
];

const WORKSPACE_POLICIES: { workspace: string; autoApproveBelow: string; maxConcurrent: number; requireMfa: boolean }[] = [
  { workspace: "Engineering", autoApproveBelow: "medium", maxConcurrent: 10, requireMfa: true },
  { workspace: "Sales", autoApproveBelow: "low", maxConcurrent: 5, requireMfa: false },
  { workspace: "HR", autoApproveBelow: "low", maxConcurrent: 3, requireMfa: true },
  { workspace: "Finance", autoApproveBelow: "low", maxConcurrent: 3, requireMfa: true },
  { workspace: "Marketing", autoApproveBelow: "medium", maxConcurrent: 8, requireMfa: false },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const effectStyles: Record<PolicyEffect, string> = {
  allow: "bg-emerald-50 text-emerald-700",
  deny: "bg-red-50 text-red-700",
  "require-approval": "bg-amber-50 text-amber-700",
};

const effectLabel: Record<PolicyEffect, string> = {
  allow: "Allow",
  deny: "Deny",
  "require-approval": "Approval",
};

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export function Policies() {
  const [activeTab, setActiveTab] = useState<"layers" | "tools" | "agents" | "workspaces">("layers");

  const tabs = [
    { key: "layers" as const, label: "Five-Layer Model" },
    { key: "tools" as const, label: "Tool Allowlist" },
    { key: "agents" as const, label: "Agent Permissions" },
    { key: "workspaces" as const, label: "Workspace Policies" },
  ];

  return (
    <div className="page-container">
      <h2 className="page-title mb-4">Policies</h2>

      {/* Tabs */}
      <div className="flex gap-1 mb-4 border-b border-neutral-200">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => setActiveTab(tab.key)}
            className={`px-3 py-1.5 text-xs font-medium rounded-t-md transition-colors -mb-px ${
              activeTab === tab.key
                ? "bg-white border border-neutral-200 border-b-white text-indigo-700"
                : "text-neutral-500 hover:text-neutral-700"
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Five-Layer Model */}
      {activeTab === "layers" && (
        <div className="card overflow-hidden">
          <ol className="divide-y divide-neutral-100" role="list">
            {LAYER_ORDER.map((layer, idx) => (
              <li key={layer} className="px-4 py-3">
                <div className="flex items-start gap-3">
                  <span className="flex-shrink-0 flex items-center justify-center w-6 h-6 rounded-full bg-indigo-100 text-xs font-bold text-indigo-700">
                    {idx + 1}
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium text-neutral-900">{policyLayerLabel[layer]}</p>
                    <p className="text-xs text-neutral-500 mt-0.5">{policyLayerDescription[layer]}</p>
                    <p className="text-[10px] text-neutral-400 mt-1 font-mono">{layer}</p>
                  </div>
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-50 text-emerald-700">
                    Active
                  </span>
                </div>
              </li>
            ))}
          </ol>
        </div>
      )}

      {/* Tool Allowlist / Denylist */}
      {activeTab === "tools" && (
        <div className="card overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
                <th className="px-3 py-2 font-medium">Tool</th>
                <th className="px-3 py-2 font-medium">Effect</th>
                <th className="px-3 py-2 font-medium">Scope</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {TOOL_RULES.map((rule, idx) => (
                <tr key={`${rule.tool}-${rule.scope}-${idx}`} className="hover:bg-neutral-50 transition-colors">
                  <td className="px-3 py-2 font-mono text-neutral-800">{rule.tool}</td>
                  <td className="px-3 py-2">
                    <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold ${effectStyles[rule.effect]}`}>
                      {rule.effect === "allow" ? "Allow" : "Deny"}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-neutral-600">
                    {rule.scope === "global" ? (
                      <span className="text-neutral-400 italic">Global</span>
                    ) : (
                      rule.scope
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Agent Permission Matrix */}
      {activeTab === "agents" && (
        <div className="card overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
                <th className="px-3 py-2 font-medium sticky left-0 bg-neutral-50">Agent Role</th>
                {CAPABILITIES.map((cap) => (
                  <th key={cap} className="px-3 py-2 font-medium capitalize text-center">{cap}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {AGENT_ROLES.map((role) => (
                <tr key={role} className="hover:bg-neutral-50 transition-colors">
                  <td className="px-3 py-2 font-medium text-neutral-900 sticky left-0 bg-white">
                    {agentRoleLabel[role]}
                    <span className="block text-[10px] text-neutral-400 font-mono">{role}</span>
                  </td>
                  {CAPABILITIES.map((cap) => {
                    const effect = PERMISSION_MATRIX[role][cap];
                    return (
                      <td key={cap} className="px-3 py-2 text-center">
                        <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold ${effectStyles[effect]}`}>
                          {effectLabel[effect]}
                        </span>
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Workspace Policies */}
      {activeTab === "workspaces" && (
        <div className="card overflow-hidden">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-neutral-500 bg-neutral-50 border-b border-neutral-200">
                <th className="px-3 py-2 font-medium">Workspace</th>
                <th className="px-3 py-2 font-medium">Auto-Approve Below</th>
                <th className="px-3 py-2 font-medium">Max Concurrent</th>
                <th className="px-3 py-2 font-medium">Require MFA</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {WORKSPACE_POLICIES.map((wp) => (
                <tr key={wp.workspace} className="hover:bg-neutral-50 transition-colors">
                  <td className="px-3 py-2 font-medium text-neutral-900">{wp.workspace}</td>
                  <td className="px-3 py-2">
                    <span className="capitalize text-neutral-600">{wp.autoApproveBelow} risk</span>
                  </td>
                  <td className="px-3 py-2 text-neutral-600">{wp.maxConcurrent}</td>
                  <td className="px-3 py-2">
                    {wp.requireMfa ? (
                      <span className="text-emerald-600 font-semibold">Yes</span>
                    ) : (
                      <span className="text-neutral-400">No</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
