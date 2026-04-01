import React, { useEffect, useState } from "react";
import { PolicyCard, RiskBadge } from "@irongolem/ui";
import type { Event, PolicyEvaluation } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

interface SecurityState {
  blockedActions: readonly Event[];
  quarantinedItems: readonly Event[];
  policyCoverage: readonly PolicyEvaluation[];
  loading: boolean;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Security() {
  const [state, setState] = useState<SecurityState>({
    blockedActions: [],
    quarantinedItems: [],
    policyCoverage: [],
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [blockedRes, quarantinedRes, coverageRes] = await Promise.allSettled([
          api.security.getBlockedActions({ pageSize: 20 }),
          api.security.getQuarantinedItems({ pageSize: 20 }),
          api.security.getPolicyCoverage(),
        ]);

        if (cancelled) return;

        setState({
          loading: false,
          blockedActions:
            blockedRes.status === "fulfilled" ? blockedRes.value.items : [],
          quarantinedItems:
            quarantinedRes.status === "fulfilled" ? quarantinedRes.value.items : [],
          policyCoverage:
            coverageRes.status === "fulfilled" ? coverageRes.value : [],
        });
      } catch {
        if (!cancelled) setState((prev) => ({ ...prev, loading: false }));
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="page-container">
      <h1 className="page-title mb-6">Security center</h1>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : (
        <div className="space-y-8">
          {/* Summary stats */}
          <div className="grid gap-4 sm:grid-cols-3">
            <StatCard
              label="Blocked actions"
              value={state.blockedActions.length}
              color="text-red-700"
              bg="bg-red-50"
            />
            <StatCard
              label="Quarantined items"
              value={state.quarantinedItems.length}
              color="text-purple-700"
              bg="bg-purple-50"
            />
            <StatCard
              label="Active safety rules"
              value={state.policyCoverage.length}
              color="text-emerald-700"
              bg="bg-emerald-50"
            />
          </div>

          {/* Blocked actions */}
          <section className="card-padded" aria-label="Blocked actions">
            <h2 className="section-title mb-4">Blocked actions</h2>
            {state.blockedActions.length === 0 ? (
              <p className="text-sm text-neutral-500">
                No actions have been blocked. Your safety rules are keeping things on track.
              </p>
            ) : (
              <ul className="space-y-2" role="list">
                {state.blockedActions.map((evt) => {
                  const payload = evt.payload as Record<string, unknown>;
                  return (
                    <li
                      key={evt.id}
                      className="flex items-start gap-3 rounded-lg border border-red-100 bg-red-50/50 px-4 py-3"
                    >
                      <span className="mt-1 h-2 w-2 flex-shrink-0 rounded-full bg-red-500" />
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-neutral-900">
                          {(payload?.reason as string) ?? "Action blocked"}
                        </p>
                        <p className="text-xs text-neutral-500 mt-0.5">
                          Layer: {(payload?.policyLayer as string) ?? "Unknown"}
                        </p>
                        <time className="text-xs text-neutral-400" dateTime={evt.timestamp}>
                          {new Date(evt.timestamp).toLocaleString()}
                        </time>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          {/* Quarantined items */}
          <section className="card-padded" aria-label="Quarantined items">
            <h2 className="section-title mb-4">Isolated for safety</h2>
            {state.quarantinedItems.length === 0 ? (
              <p className="text-sm text-neutral-500">Nothing is currently isolated.</p>
            ) : (
              <ul className="space-y-2" role="list">
                {state.quarantinedItems.map((evt) => {
                  const payload = evt.payload as Record<string, unknown>;
                  return (
                    <li
                      key={evt.id}
                      className="flex items-start gap-3 rounded-lg border border-purple-100 bg-purple-50/50 px-4 py-3"
                    >
                      <span className="mt-1 h-2 w-2 flex-shrink-0 rounded-full bg-purple-500" />
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-neutral-900">
                          {(payload?.summary as string) ?? "Item quarantined"}
                        </p>
                        <time className="text-xs text-neutral-400" dateTime={evt.timestamp}>
                          {new Date(evt.timestamp).toLocaleString()}
                        </time>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          {/* Policy coverage */}
          <section aria-label="Safety rules coverage">
            <h2 className="section-title mb-4">Safety rules coverage</h2>
            {state.policyCoverage.length === 0 ? (
              <div className="card-padded">
                <p className="text-sm text-neutral-500">
                  Safety rules will appear here once configured.
                </p>
              </div>
            ) : (
              <div className="grid gap-4 lg:grid-cols-2">
                {state.policyCoverage.map((evaluation) => (
                  <PolicyCard
                    key={evaluation.requestId}
                    evaluation={evaluation}
                    showDescriptions
                  />
                ))}
              </div>
            )}
          </section>
        </div>
      )}
    </div>
  );
}

function StatCard({
  label,
  value,
  color,
  bg,
}: {
  label: string;
  value: number;
  color: string;
  bg: string;
}) {
  return (
    <div className={`rounded-xl border border-neutral-200 ${bg} px-4 py-5`}>
      <p className="text-sm font-medium text-neutral-600">{label}</p>
      <p className={`mt-1 text-3xl font-bold ${color}`}>{value}</p>
    </div>
  );
}
