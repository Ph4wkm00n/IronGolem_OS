import React, { useEffect, useState } from "react";
import { HeartbeatStatus as HeartbeatIndicator, Timeline } from "@irongolem/ui";
import type { TimelineEntry } from "@irongolem/ui";
import type { HeartbeatStatus as HBStatus, Event, EventKind } from "@irongolem/schema";
import api from "../lib/api";

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const EVENT_TO_TIMELINE_STATE: Partial<Record<EventKind, TimelineEntry["state"]>> = {
  "action-taken": "taken",
  "action-proposed": "proposed",
  "action-blocked": "blocked",
  "action-healed": "healed",
  "action-quarantined": "quarantined",
  "research-update": "research-update",
  "squad-handoff": "squad-handoff",
};

function eventToTimelineEntry(evt: Event): TimelineEntry {
  const payload = evt.payload as Record<string, unknown>;
  return {
    id: evt.id,
    state: EVENT_TO_TIMELINE_STATE[evt.kind] ?? "taken",
    title: (payload?.summary as string) ?? evt.kind,
    description: (payload?.message as string) ?? (payload?.reason as string) ?? "",
    timestamp: evt.timestamp,
    agentRole: evt.agentRole,
  };
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

interface HealthState {
  status: HBStatus;
  message: string;
  uptimeSeconds: number;
  timeline: readonly TimelineEntry[];
  loading: boolean;
}

export function Health() {
  const [state, setState] = useState<HealthState>({
    status: "healthy",
    message: "",
    uptimeSeconds: 0,
    timeline: [],
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const [healthRes, timelineRes] = await Promise.allSettled([
          api.health.getStatus(),
          api.health.getTimeline({ pageSize: 50 }),
        ]);

        if (cancelled) return;

        setState({
          loading: false,
          status: healthRes.status === "fulfilled" ? healthRes.value.status : "healthy",
          message: healthRes.status === "fulfilled" ? healthRes.value.message : "",
          uptimeSeconds: healthRes.status === "fulfilled" ? healthRes.value.uptimeSeconds : 0,
          timeline:
            timelineRes.status === "fulfilled"
              ? timelineRes.value.items.map(eventToTimelineEntry)
              : [],
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
      <h1 className="page-title mb-6">Health center</h1>

      {state.loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-neutral-200 border-t-indigo-600" />
        </div>
      ) : (
        <div className="space-y-8">
          {/* Current status */}
          <section className="card-padded" aria-label="Current health status">
            <h2 className="section-title mb-4">Current status</h2>
            <HeartbeatIndicator
              status={state.status}
              message={state.message}
              uptimeSeconds={state.uptimeSeconds}
              size="lg"
            />
          </section>

          {/* Status legend */}
          <section className="card-padded" aria-label="Status descriptions">
            <h2 className="section-title mb-4">What each status means</h2>
            <dl className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <StatusDescription
                status="healthy"
                title="All good"
                description="Everything is running normally. No action needed."
              />
              <StatusDescription
                status="quietly-recovering"
                title="Fixing itself"
                description="A minor issue was detected and is being resolved automatically."
              />
              <StatusDescription
                status="needs-attention"
                title="Needs your attention"
                description="Something requires your input or approval to proceed."
              />
              <StatusDescription
                status="paused"
                title="Paused"
                description="Operations have been paused, either by you or automatically."
              />
              <StatusDescription
                status="quarantined"
                title="Isolated for safety"
                description="A component has been isolated to prevent potential issues."
              />
            </dl>
          </section>

          {/* Recovery timeline */}
          <section className="card-padded" aria-label="Recovery timeline">
            <h2 className="section-title mb-4">Activity timeline</h2>
            <Timeline entries={state.timeline as TimelineEntry[]} maxVisible={20} />
          </section>
        </div>
      )}
    </div>
  );
}

function StatusDescription({
  status,
  title,
  description,
}: {
  status: HBStatus;
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-lg border border-neutral-100 p-3">
      <dt className="mb-1">
        <HeartbeatIndicator status={status} size="sm" />
      </dt>
      <dd>
        <p className="text-sm font-medium text-neutral-900 mt-2">{title}</p>
        <p className="text-xs text-neutral-500 mt-0.5">{description}</p>
      </dd>
    </div>
  );
}
