// route: /commitments
// purpose: user-facing future-obligation tracker. v0.3 Step 7 of
// Plans/modular-puzzling-blum.md. The backend lives at
// services/gateway/internal/commitments/; this page renders pending /
// sent / dismissed / snoozed / expired commitments, scoped by filter
// chips, with dismiss + snooze affordances.

import React, { useMemo, useState } from "react";

import { RouteError } from "../../components/RouteError";
import { RouteSkeleton } from "../../components/RouteSkeleton";
import { useRouteData } from "../../lib/route-data";
import {
  api,
  type Commitment,
  type CommitmentKind,
  type CommitmentSensitivity,
  type CommitmentStatus,
} from "../../lib/api";
import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

const STATUS_FILTERS: readonly CommitmentStatus[] = [
  "pending",
  "sent",
  "snoozed",
  "dismissed",
  "expired",
];

const KIND_LABEL: Record<CommitmentKind, string> = {
  event_check_in: "Event check-in",
  deadline_check: "Deadline",
  care_check_in: "Care check-in",
  open_loop: "Open loop",
};

const SENSITIVITY_TONE: Record<CommitmentSensitivity, string> = {
  routine: "bg-neutral text-neutral-700",
  personal: "bg-accent text-accent",
  care: "bg-warning text-warning",
};

const STATUS_TONE: Record<CommitmentStatus, string> = {
  pending: "bg-accent text-accent",
  sent: "bg-safe text-safe",
  snoozed: "bg-neutral text-neutral-700",
  dismissed: "bg-neutral text-neutral-500",
  expired: "bg-blocked text-blocked",
};

function relTimeMs(deltaMs: number): string {
  const abs = Math.abs(deltaMs);
  const sign = deltaMs >= 0 ? "in" : "ago";
  if (abs < 60_000) return sign === "in" ? "in <1min" : "just now";
  const minutes = Math.round(abs / 60_000);
  if (minutes < 60) {
    return sign === "in" ? `in ${minutes}min` : `${minutes}min ago`;
  }
  const hours = Math.round(minutes / 60);
  if (hours < 24) {
    return sign === "in" ? `in ${hours}h` : `${hours}h ago`;
  }
  const days = Math.round(hours / 24);
  return sign === "in" ? `in ${days}d` : `${days}d ago`;
}

export function Commitments(): React.JSX.Element {
  const load = useRouteData<readonly Commitment[]>({
    initialData: api.v2.commitments.getMock(),
    load: () => api.v2.commitments.list(),
  });
  const [filter, setFilter] = useState<CommitmentStatus | "all">("all");
  const [pendingAction, setPendingAction] = useState<string | null>(null);

  if (load.status === "loading" && load.data == null) {
    return (
      <div className="min-h-screen bg-neutral-50">
        <WorkspaceTopbar />
        <RouteSkeleton variant="list" count={4} />
      </div>
    );
  }

  if (load.status === "error") {
    return (
      <div className="min-h-screen bg-neutral-50">
        <WorkspaceTopbar />
        <RouteError
          route="Commitments"
          error={load.error}
          onRetry={load.reload}
        />
      </div>
    );
  }

  const items = load.data ?? [];
  const visible =
    filter === "all" ? items : items.filter((c) => c.status === filter);
  const counts: Record<CommitmentStatus | "all", number> = {
    all: items.length,
    pending: items.filter((c) => c.status === "pending").length,
    sent: items.filter((c) => c.status === "sent").length,
    snoozed: items.filter((c) => c.status === "snoozed").length,
    dismissed: items.filter((c) => c.status === "dismissed").length,
    expired: items.filter((c) => c.status === "expired").length,
  };

  const handleDismiss = async (id: string) => {
    setPendingAction(id);
    try {
      await api.v2.commitments.dismiss(id);
    } finally {
      setPendingAction(null);
      load.reload();
    }
  };

  const handleSnooze = async (id: string) => {
    // Default snooze: 4 hours from now. v0.4 will add a picker.
    const untilMs = Date.now() + 4 * 60 * 60 * 1000;
    setPendingAction(id);
    try {
      await api.v2.commitments.snooze(id, untilMs);
    } finally {
      setPendingAction(null);
      load.reload();
    }
  };

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar showHeartbeatPill={false} />

      <main className="page-container max-w-[78rem] flex flex-col gap-4">
        <header className="mt-4">
          <h1 className="page-title">Commitments</h1>
          <p className="text-neutral-600 mt-1 max-w-2xl">
            Time-bound promises the assistant made. Pending items fire when
            their window opens. You can dismiss or snooze any of them.
          </p>
        </header>

        <div role="tablist" aria-label="Filter by status" className="flex gap-2 flex-wrap">
          {(["all", ...STATUS_FILTERS] as const).map((s) => {
            const isActive = filter === s;
            return (
              <button
                key={s}
                role="tab"
                aria-selected={isActive}
                onClick={() => setFilter(s)}
                className={
                  "rounded-full px-3 py-1 text-sm font-medium border transition " +
                  (isActive
                    ? "bg-accent-solid text-text-solid border-accent-solid"
                    : "bg-white text-neutral-700 border-neutral-200 hover:border-neutral-300")
                }
              >
                {s === "all" ? "All" : s.charAt(0).toUpperCase() + s.slice(1)}
                <span className="ml-2 text-xs text-neutral-500">
                  {counts[s]}
                </span>
              </button>
            );
          })}
        </div>

        {visible.length === 0 ? (
          <EmptyState filter={filter} />
        ) : (
          <ul className="space-y-2" role="list">
            {visible.map((c) => (
              <CommitmentCard
                key={c.id}
                commitment={c}
                pending={pendingAction === c.id}
                onDismiss={() => handleDismiss(c.id)}
                onSnooze={() => handleSnooze(c.id)}
              />
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}

interface CommitmentCardProps {
  readonly commitment: Commitment;
  readonly pending: boolean;
  readonly onDismiss: () => void;
  readonly onSnooze: () => void;
}

function CommitmentCard({
  commitment: c,
  pending,
  onDismiss,
  onSnooze,
}: CommitmentCardProps): React.JSX.Element {
  const now = Date.now();
  const targetMs = c.due_window.earliest_ms;
  const dueLabel = useMemo(() => relTimeMs(targetMs - now), [targetMs, now]);
  const canAct = c.status === "pending";

  return (
    <li className="card card-padded">
      <div className="flex items-start gap-4">
        <div className="flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span
              className={
                "rounded-full px-2 py-0.5 text-xs font-medium " +
                STATUS_TONE[c.status]
              }
            >
              {c.status}
            </span>
            <span className="text-xs font-medium text-neutral-500">
              {KIND_LABEL[c.kind]}
            </span>
            <span
              className={
                "rounded-full px-2 py-0.5 text-xs " +
                SENSITIVITY_TONE[c.sensitivity]
              }
              title={`Sensitivity: ${c.sensitivity}`}
            >
              {c.sensitivity}
            </span>
            <span className="text-xs text-neutral-500">·</span>
            <span className="text-xs text-neutral-500">{dueLabel}</span>
          </div>
          <p className="mt-2 text-sm text-neutral-900">{c.suggested_text}</p>
          <p className="mt-1 text-xs text-neutral-500">{c.reason}</p>
          <p className="mt-2 text-xs text-neutral-400">
            via {c.connector_id ?? "?"} · channel {c.channel_id ?? "?"}
          </p>
        </div>
        {canAct ? (
          <div className="flex flex-col gap-2 shrink-0">
            <button
              type="button"
              disabled={pending}
              onClick={onSnooze}
              className="rounded-md border border-neutral-200 bg-white px-3 py-1 text-sm font-medium text-neutral-700 hover:border-neutral-300 disabled:opacity-50"
            >
              Snooze 4h
            </button>
            <button
              type="button"
              disabled={pending}
              onClick={onDismiss}
              className="rounded-md border border-neutral-200 bg-white px-3 py-1 text-sm font-medium text-neutral-700 hover:border-neutral-300 disabled:opacity-50"
            >
              Dismiss
            </button>
          </div>
        ) : null}
      </div>
    </li>
  );
}

function EmptyState({
  filter,
}: {
  readonly filter: CommitmentStatus | "all";
}): React.JSX.Element {
  return (
    <div className="card card-padded text-center text-neutral-500">
      <p className="text-sm">
        No {filter === "all" ? "" : filter + " "}commitments to show.
      </p>
      <p className="mt-1 text-xs text-neutral-400">
        Commitments are extracted automatically when the assistant promises
        a follow-up.
      </p>
    </div>
  );
}
