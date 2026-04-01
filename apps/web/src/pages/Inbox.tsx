import React, { useEffect, useState } from "react";
import { RiskBadge } from "@irongolem/ui";
import type { ApprovalRequest, RiskLevel } from "@irongolem/schema";
import { agentRoleLabel } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

type FilterStatus = "all" | "pending" | "approved" | "denied";

interface InboxState {
  requests: readonly ApprovalRequest[];
  loading: boolean;
  filter: FilterStatus;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Inbox() {
  const [state, setState] = useState<InboxState>({
    requests: [],
    loading: true,
    filter: "pending",
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.approvals.listPending({ pageSize: 50 });
        if (!cancelled) {
          setState((prev) => ({ ...prev, loading: false, requests: res.items }));
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
      ? state.requests
      : state.requests.filter((r) => r.status === state.filter);

  const pendingCount = state.requests.filter((r) => r.status === "pending").length;

  async function handleApprove(id: string) {
    try {
      const updated = await api.approvals.approve(id);
      setState((prev) => ({
        ...prev,
        requests: prev.requests.map((r) => (r.id === id ? updated : r)),
      }));
    } catch {
      // Would use toast in production
    }
  }

  async function handleDeny(id: string) {
    try {
      const updated = await api.approvals.deny(id);
      setState((prev) => ({
        ...prev,
        requests: prev.requests.map((r) => (r.id === id ? updated : r)),
      }));
    } catch {
      // Would use toast in production
    }
  }

  return (
    <div className="page-container">
      <div className="flex items-center gap-3 mb-6">
        <h1 className="page-title">Inbox</h1>
        {pendingCount > 0 && (
          <span className="inline-flex items-center justify-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-100 text-indigo-700">
            {pendingCount}
          </span>
        )}
      </div>

      {/* Filter tabs */}
      <div className="flex gap-1 mb-6 border-b border-neutral-200">
        {(["pending", "approved", "denied", "all"] as const).map((tab) => (
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
            {tab === "all" ? "All" : tab.charAt(0).toUpperCase() + tab.slice(1)}
          </button>
        ))}
      </div>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : filtered.length === 0 ? (
        <div className="card-padded text-center py-12">
          <p className="text-neutral-500">
            {state.filter === "pending"
              ? "No items waiting for your approval."
              : "No items match this filter."}
          </p>
        </div>
      ) : (
        <ul className="space-y-3" role="list">
          {filtered.map((req) => (
            <ApprovalCard
              key={req.id}
              request={req}
              onApprove={() => handleApprove(req.id)}
              onDeny={() => handleDeny(req.id)}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Approval card                                                      */
/* ------------------------------------------------------------------ */

function ApprovalCard({
  request,
  onApprove,
  onDeny,
}: {
  request: ApprovalRequest;
  onApprove: () => void;
  onDeny: () => void;
}) {
  const statusStyles: Record<ApprovalRequest["status"], string> = {
    pending: "border-amber-200 bg-amber-50/30",
    approved: "border-emerald-200 bg-emerald-50/30",
    denied: "border-red-200 bg-red-50/30",
  };

  const roleLabel = agentRoleLabel[request.agentRole] ?? request.agentRole;

  return (
    <li className={`rounded-xl border p-4 ${statusStyles[request.status]}`}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="text-sm font-semibold text-neutral-900">{request.summary}</h3>
            <RiskBadge level={request.riskLevel} size="sm" />
          </div>
          <p className="mt-1 text-xs text-neutral-500">
            Requested by {roleLabel}
          </p>
          <time className="text-xs text-neutral-400" dateTime={request.requestedAt}>
            {new Date(request.requestedAt).toLocaleString()}
          </time>
        </div>

        {request.status === "pending" && (
          <div className="flex gap-2 flex-shrink-0">
            <button
              type="button"
              onClick={onApprove}
              className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-emerald-500 transition-colors"
            >
              Approve
            </button>
            <button
              type="button"
              onClick={onDeny}
              className="rounded-lg bg-neutral-100 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-200 transition-colors"
            >
              Deny
            </button>
          </div>
        )}

        {request.status !== "pending" && (
          <span
            className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
              request.status === "approved"
                ? "bg-emerald-100 text-emerald-700"
                : "bg-red-100 text-red-700"
            }`}
          >
            {request.status === "approved" ? "Approved" : "Denied"}
          </span>
        )}
      </div>
    </li>
  );
}
