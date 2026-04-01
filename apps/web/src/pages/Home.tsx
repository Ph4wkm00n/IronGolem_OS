import React, { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { HeartbeatStatus } from "@irongolem/ui";
import { RiskBadge } from "@irongolem/ui";
import type {
  HeartbeatStatus as HBStatus,
  Recipe,
  Squad,
  Event,
} from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  State                                                              */
/* ------------------------------------------------------------------ */

interface DashboardState {
  healthStatus: HBStatus;
  healthMessage: string;
  uptimeSeconds: number;
  activeRecipes: readonly Recipe[];
  squads: readonly Squad[];
  recentAlerts: readonly Event[];
  loading: boolean;
}

const INITIAL_STATE: DashboardState = {
  healthStatus: "healthy",
  healthMessage: "All systems operational",
  uptimeSeconds: 0,
  activeRecipes: [],
  squads: [],
  recentAlerts: [],
  loading: true,
};

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function HealthWidget({
  status,
  message,
  uptimeSeconds,
}: {
  status: HBStatus;
  message: string;
  uptimeSeconds: number;
}) {
  return (
    <section className="card-padded" aria-label="System health">
      <h2 className="section-title mb-3">System health</h2>
      <HeartbeatStatus
        status={status}
        message={message}
        uptimeSeconds={uptimeSeconds}
        size="lg"
      />
    </section>
  );
}

function ActiveRecipesWidget({ recipes }: { recipes: readonly Recipe[] }) {
  return (
    <section className="card-padded" aria-label="Active recipes">
      <div className="flex items-center justify-between mb-3">
        <h2 className="section-title">Active recipes</h2>
        <Link to="/recipes" className="text-sm text-indigo-600 hover:text-indigo-500 font-medium">
          View all
        </Link>
      </div>

      {recipes.length === 0 ? (
        <p className="text-sm text-neutral-500">No active recipes.</p>
      ) : (
        <ul className="space-y-2" role="list">
          {recipes.map((r) => (
            <li
              key={r.id}
              className="flex items-center justify-between rounded-lg border border-neutral-100 px-3 py-2"
            >
              <div>
                <p className="text-sm font-medium text-neutral-900">{r.title}</p>
                <p className="text-xs text-neutral-500">{r.category}</p>
              </div>
              <RiskBadge level={r.riskLevel} size="sm" />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function SquadStatusWidget({ squads }: { squads: readonly Squad[] }) {
  const statusColors: Record<Squad["status"], string> = {
    active: "bg-emerald-500",
    idle: "bg-neutral-400",
    paused: "bg-amber-500",
  };

  return (
    <section className="card-padded" aria-label="Assistant teams">
      <h2 className="section-title mb-3">Assistant teams</h2>

      {squads.length === 0 ? (
        <p className="text-sm text-neutral-500">No teams configured.</p>
      ) : (
        <ul className="space-y-2" role="list">
          {squads.map((s) => (
            <li
              key={s.id}
              className="flex items-center gap-3 rounded-lg border border-neutral-100 px-3 py-2"
            >
              <span className={`h-2 w-2 rounded-full ${statusColors[s.status]}`} />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-neutral-900">{s.displayName}</p>
                <p className="text-xs text-neutral-500">
                  {s.activeRecipeCount} active {s.activeRecipeCount === 1 ? "recipe" : "recipes"}
                </p>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function AlertsWidget({ alerts }: { alerts: readonly Event[] }) {
  return (
    <section className="card-padded" aria-label="Recent alerts">
      <div className="flex items-center justify-between mb-3">
        <h2 className="section-title">Recent alerts</h2>
        <Link to="/security" className="text-sm text-indigo-600 hover:text-indigo-500 font-medium">
          Security center
        </Link>
      </div>

      {alerts.length === 0 ? (
        <p className="text-sm text-neutral-500">No recent alerts. Everything looks good.</p>
      ) : (
        <ul className="space-y-2" role="list">
          {alerts.map((evt) => (
            <li
              key={evt.id}
              className="flex items-start gap-2 rounded-lg border border-neutral-100 px-3 py-2"
            >
              <span className="mt-0.5 h-2 w-2 flex-shrink-0 rounded-full bg-red-500" />
              <div className="min-w-0">
                <p className="text-sm text-neutral-900">{evt.kind}</p>
                <time className="text-xs text-neutral-400" dateTime={evt.timestamp}>
                  {new Date(evt.timestamp).toLocaleString()}
                </time>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Home() {
  const [state, setState] = useState<DashboardState>(INITIAL_STATE);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [healthRes, recipesRes, squadsRes, alertsRes] = await Promise.allSettled([
          api.health.getStatus(),
          api.recipes.list({ pageSize: 5 }),
          api.squads.list(),
          api.security.getBlockedActions({ pageSize: 5 }),
        ]);

        if (cancelled) return;

        setState((prev) => ({
          ...prev,
          loading: false,
          healthStatus:
            healthRes.status === "fulfilled" ? healthRes.value.status : prev.healthStatus,
          healthMessage:
            healthRes.status === "fulfilled" ? healthRes.value.message : prev.healthMessage,
          uptimeSeconds:
            healthRes.status === "fulfilled" ? healthRes.value.uptimeSeconds : prev.uptimeSeconds,
          activeRecipes:
            recipesRes.status === "fulfilled"
              ? recipesRes.value.items.filter((r) => r.isActive)
              : prev.activeRecipes,
          squads: squadsRes.status === "fulfilled" ? squadsRes.value : prev.squads,
          recentAlerts:
            alertsRes.status === "fulfilled" ? alertsRes.value.items : prev.recentAlerts,
        }));
      } catch {
        if (!cancelled) setState((prev) => ({ ...prev, loading: false }));
      }
    }

    load();
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="page-container">
      <h1 className="page-title mb-6">Home</h1>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : (
        <div className="grid gap-6 lg:grid-cols-2">
          <HealthWidget
            status={state.healthStatus}
            message={state.healthMessage}
            uptimeSeconds={state.uptimeSeconds}
          />
          <AlertsWidget alerts={state.recentAlerts} />
          <ActiveRecipesWidget recipes={state.activeRecipes} />
          <SquadStatusWidget squads={state.squads} />
        </div>
      )}
    </div>
  );
}
