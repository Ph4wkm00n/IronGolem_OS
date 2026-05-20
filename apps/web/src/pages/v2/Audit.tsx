// route: /audit
// purpose: audit-probe findings feed for the v2 frontend. v0.3 Step 7 of
// Plans/modular-puzzling-blum.md. Surfaces the gateway's continuous-
// security probe results (services/gateway/internal/audit/) so an
// operator can see at a glance whether the trust model, channel rules,
// and connector readiness are intact.

import React, { useMemo, useState } from "react";

import { RouteError } from "../../components/RouteError";
import { RouteSkeleton } from "../../components/RouteSkeleton";
import { useRouteData } from "../../lib/route-data";
import { api, type AuditFinding, type AuditSeverity } from "../../lib/api";
import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

const SEVERITY_FILTERS: readonly AuditSeverity[] = ["info", "warning", "critical"];

const SEVERITY_TONE: Record<AuditSeverity, string> = {
  info: "bg-safe text-safe border-safe",
  warning: "bg-warning text-warning border-warning",
  critical: "bg-blocked text-blocked border-blocked",
};

const PROBE_LABEL: Record<string, string> = {
  workspace_skill_escape: "Workspace skill escape",
  trust_model: "Trust model",
  channel_dm_policy: "Channel DM policy",
  connector_health_drift: "Connector health drift",
};

export function Audit(): React.JSX.Element {
  const load = useRouteData<readonly AuditFinding[]>({
    initialData: api.v2.audit.getMock(),
    load: () => api.v2.audit.findings(),
  });
  const [filter, setFilter] = useState<AuditSeverity | "all">("all");
  const [expanded, setExpanded] = useState<string | null>(null);

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
          route="Audit"
          error={load.error}
          onRetry={load.reload}
        />
      </div>
    );
  }

  const items = load.data ?? [];
  const visible =
    filter === "all" ? items : items.filter((f) => f.severity === filter);
  const counts: Record<AuditSeverity | "all", number> = {
    all: items.length,
    info: items.filter((f) => f.severity === "info").length,
    warning: items.filter((f) => f.severity === "warning").length,
    critical: items.filter((f) => f.severity === "critical").length,
  };

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar showHeartbeatPill={false} />

      <main className="page-container max-w-[78rem] flex flex-col gap-4">
        <header className="mt-4">
          <h1 className="page-title">Audit findings</h1>
          <p className="text-neutral-600 mt-1 max-w-2xl">
            Continuous-security probes run on the gateway. Info findings are
            confirmations; warnings flag drift; critical findings indicate a
            broken trust invariant.
          </p>
        </header>

        <div role="tablist" aria-label="Filter by severity" className="flex gap-2 flex-wrap">
          {(["all", ...SEVERITY_FILTERS] as const).map((s) => {
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
          <div className="card card-padded text-center text-neutral-500">
            <p className="text-sm">No findings to show.</p>
            <p className="mt-1 text-xs text-neutral-400">
              The audit runtime ticks every 5 minutes; new findings appear here.
            </p>
          </div>
        ) : (
          <ul className="space-y-2" role="list">
            {visible.map((f) => (
              <FindingCard
                key={f.id}
                finding={f}
                expanded={expanded === f.id}
                onToggle={() => setExpanded(expanded === f.id ? null : f.id)}
              />
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}

interface FindingCardProps {
  readonly finding: AuditFinding;
  readonly expanded: boolean;
  readonly onToggle: () => void;
}

function FindingCard({
  finding: f,
  expanded,
  onToggle,
}: FindingCardProps): React.JSX.Element {
  const evidenceEntries = useMemo(
    () => (f.evidence ? Object.entries(f.evidence) : []),
    [f.evidence],
  );
  const probeLabel = PROBE_LABEL[f.probe_id] ?? f.probe_id;
  const hasEvidence = evidenceEntries.length > 0;

  return (
    <li className={"card card-padded border-l-4 " + SEVERITY_TONE[f.severity]}>
      <div className="flex items-start gap-4">
        <div className="flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span
              className={
                "rounded-full px-2 py-0.5 text-xs font-medium uppercase " +
                SEVERITY_TONE[f.severity]
              }
            >
              {f.severity}
            </span>
            <span className="text-xs font-medium text-neutral-600">
              {probeLabel}
            </span>
            <span className="text-xs text-neutral-400">·</span>
            <span className="text-xs text-neutral-500">
              {new Date(f.stored_at).toLocaleString()}
            </span>
          </div>
          <p className="mt-2 text-sm text-neutral-900">{f.reason}</p>
          {hasEvidence ? (
            <button
              type="button"
              onClick={onToggle}
              className="mt-2 text-xs text-accent hover:text-accent-solid"
              aria-expanded={expanded}
            >
              {expanded ? "Hide evidence" : `Show ${evidenceEntries.length} evidence keys`}
            </button>
          ) : null}
          {expanded && hasEvidence ? (
            <dl className="mt-3 rounded-md bg-neutral-100 p-3 text-xs space-y-1">
              {evidenceEntries.map(([key, value]) => (
                <div key={key} className="grid grid-cols-[8rem_1fr] gap-2">
                  <dt className="font-mono text-neutral-500">{key}</dt>
                  <dd className="font-mono text-neutral-700 break-all">
                    {typeof value === "string"
                      ? value
                      : JSON.stringify(value)}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}
        </div>
      </div>
    </li>
  );
}
