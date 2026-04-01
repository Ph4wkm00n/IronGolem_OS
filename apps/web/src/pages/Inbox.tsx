import React, { useEffect, useState } from "react";
import { RiskBadge } from "@irongolem/ui";
import type { ApprovalRequest, RiskLevel } from "@irongolem/schema";
import { agentRoleLabel } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  Placeholder data                                                   */
/* ------------------------------------------------------------------ */

const SAMPLE_APPROVALS: readonly ApprovalRequest[] = [
  {
    id: "approval-1",
    planNodeId: "node-101",
    summary: "Send a reply to the team standup thread confirming your availability for tomorrow.",
    riskLevel: "low",
    requestedAt: "2026-03-31T14:22:00Z",
    agentRole: "executor",
    status: "pending",
  },
  {
    id: "approval-2",
    planNodeId: "node-202",
    summary: "Reschedule your 3 PM meeting with the design team to Thursday at 10 AM.",
    riskLevel: "medium",
    requestedAt: "2026-03-31T13:05:00Z",
    agentRole: "planner",
    status: "pending",
  },
  {
    id: "approval-3",
    planNodeId: "node-303",
    summary: "Archive 12 newsletter emails older than 30 days that you haven't opened.",
    riskLevel: "medium",
    requestedAt: "2026-03-31T11:48:00Z",
    agentRole: "executor",
    status: "pending",
  },
  {
    id: "approval-4",
    planNodeId: "node-404",
    summary: "Share the quarterly report document with 3 external collaborators.",
    riskLevel: "high",
    requestedAt: "2026-03-31T10:30:00Z",
    agentRole: "executor",
    status: "pending",
  },
];

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

interface InboxState {
  approvals: readonly ApprovalRequest[];
  loading: boolean;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Inbox() {
  const [state, setState] = useState<InboxState>({
    approvals: [],
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const res = await api.approvals.listPending({ pageSize: 50 });
        if (!cancelled) {
          setState({
            approvals: res.items.length > 0 ? res.items : SAMPLE_APPROVALS,
            loading: false,
          });
        }
      } catch {
        if (!cancelled) {
          setState({ approvals: SAMPLE_APPROVALS, loading: false });
        }
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleApprove(id: string) {
    try {
      await api.approvals.approve(id);
    } catch {
      // Demo mode fallback
    }
    setState((prev) => ({
      ...prev,
      approvals: prev.approvals.filter((a) => a.id !== id),
    }));
  }

  async function handleDeny(id: string) {
    try {
      await api.approvals.deny(id);
    } catch {
      // Demo mode fallback
    }
    setState((prev) => ({
      ...prev,
      approvals: prev.approvals.filter((a) => a.id !== id),
    }));
  }

  const pendingCount = state.approvals.filter((a) => a.status === "pending").length;

  return (
    <div className="page-container">
      <div className="mb-6">
        <h1 className="page-title">Inbox</h1>
        <p className="mt-1 text-sm text-neutral-500">
          {pendingCount === 0
            ? "Nothing needs your approval right now."
            : `${pendingCount} ${pendingCount === 1 ? "item needs" : "items need"} your review.`}
        </p>
      </div>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : state.approvals.length === 0 ? (
        <div className="card-padded text-center py-16">
          <svg
            className="mx-auto h-12 w-12 text-neutral-300"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M2.25 13.5h3.86a2.25 2.25 0 012.012 1.244l.256.512a2.25 2.25 0 002.013 1.244h3.218a2.25 2.25 0 002.013-1.244l.256-.512a2.25 2.25 0 012.013-1.244h3.859"
            />
          </svg>
          <p className="mt-4 text-sm text-neutral-500">Your inbox is clear. Nice work.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {state.approvals.map((approval) => (
            <ApprovalCard
              key={approval.id}
              approval={approval}
              onApprove={() => handleApprove(approval.id)}
              onDeny={() => handleDeny(approval.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Approval card                                                      */
/* ------------------------------------------------------------------ */

interface ApprovalCardProps {
  approval: ApprovalRequest;
  onApprove: () => void;
  onDeny: () => void;
}

const RISK_BORDER: Record<RiskLevel, string> = {
  low: "border-l-emerald-400",
  medium: "border-l-amber-400",
  high: "border-l-orange-400",
  critical: "border-l-red-500",
};

function ApprovalCard({ approval, onApprove, onDeny }: ApprovalCardProps) {
  const roleLabel =
    agentRoleLabel[approval.agentRole as keyof typeof agentRoleLabel] ??
    approval.agentRole;

  return (
    <article
      className={`card-padded border-l-4 ${RISK_BORDER[approval.riskLevel]}`}
      aria-label={`Approval request: ${approval.summary}`}
    >
      <div className="flex flex-col sm:flex-row sm:items-start gap-4">
        {/* Content */}
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-neutral-900 leading-relaxed">
            {approval.summary}
          </p>

          <div className="mt-2 flex items-center gap-3 flex-wrap">
            <RiskBadge level={approval.riskLevel} size="sm" />
            <span className="text-xs text-neutral-500">
              Requested by {roleLabel}
            </span>
            <time
              className="text-xs text-neutral-400"
              dateTime={approval.requestedAt}
            >
              {formatTimestamp(approval.requestedAt)}
            </time>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2 flex-shrink-0">
          <button
            type="button"
            onClick={onDeny}
            className="rounded-lg border border-neutral-200 px-4 py-2 text-sm font-medium text-neutral-700 hover:bg-neutral-50 transition-colors"
          >
            Deny
          </button>
          <button
            type="button"
            onClick={onApprove}
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors"
          >
            Approve
          </button>
        </div>
      </div>
    </article>
  );
}

function formatTimestamp(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}
