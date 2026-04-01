import React from "react";
import type { EventKind } from "@irongolem/schema";

/** Visual states for timeline entries. */
export type TimelineState =
  | "taken"
  | "proposed"
  | "blocked"
  | "healed"
  | "quarantined"
  | "research-update"
  | "squad-handoff";

export interface TimelineEntry {
  readonly id: string;
  readonly state: TimelineState;
  readonly title: string;
  readonly description: string;
  readonly timestamp: string;
  /** Optional agent role that produced this entry. */
  readonly agentRole?: string;
}

export interface TimelineProps {
  readonly entries: readonly TimelineEntry[];
  /** Maximum entries to display before showing "show more". */
  readonly maxVisible?: number;
}

const stateConfig: Record<TimelineState, { dot: string; label: string; bg: string }> = {
  taken: {
    dot: "bg-emerald-500",
    label: "Completed",
    bg: "bg-emerald-50",
  },
  proposed: {
    dot: "bg-indigo-500",
    label: "Proposed",
    bg: "bg-indigo-50",
  },
  blocked: {
    dot: "bg-red-500",
    label: "Blocked",
    bg: "bg-red-50",
  },
  healed: {
    dot: "bg-blue-500",
    label: "Auto-fixed",
    bg: "bg-blue-50",
  },
  quarantined: {
    dot: "bg-purple-500",
    label: "Isolated",
    bg: "bg-purple-50",
  },
  "research-update": {
    dot: "bg-cyan-500",
    label: "Research update",
    bg: "bg-cyan-50",
  },
  "squad-handoff": {
    dot: "bg-amber-500",
    label: "Team handoff",
    bg: "bg-amber-50",
  },
};

function formatTimestamp(iso: string): string {
  try {
    const date = new Date(iso);
    return date.toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

/**
 * Chronological timeline showing system events with semantic state colours.
 */
export function Timeline({ entries, maxVisible = 50 }: TimelineProps) {
  const [expanded, setExpanded] = React.useState(false);
  const visible = expanded ? entries : entries.slice(0, maxVisible);
  const hasMore = entries.length > maxVisible;

  if (entries.length === 0) {
    return (
      <div className="text-center py-8 text-neutral-500 text-sm">
        No activity yet.
      </div>
    );
  }

  return (
    <div className="flow-root">
      <ul role="list" className="-mb-8">
        {visible.map((entry, idx) => {
          const config = stateConfig[entry.state];
          const isLast = idx === visible.length - 1;

          return (
            <li key={entry.id}>
              <div className="relative pb-8">
                {/* Connector line */}
                {!isLast && (
                  <span
                    className="absolute left-3 top-6 -ml-px h-full w-0.5 bg-neutral-200"
                    aria-hidden="true"
                  />
                )}

                <div className="relative flex items-start gap-3">
                  {/* State dot */}
                  <span
                    className={`flex h-6 w-6 items-center justify-center rounded-full ring-4 ring-white ${config.dot}`}
                    aria-label={config.label}
                  >
                    <span className="h-2 w-2 rounded-full bg-white" />
                  </span>

                  {/* Content */}
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${config.bg} text-neutral-800`}>
                        {config.label}
                      </span>
                      <time className="text-xs text-neutral-500" dateTime={entry.timestamp}>
                        {formatTimestamp(entry.timestamp)}
                      </time>
                      {entry.agentRole && (
                        <span className="text-xs text-neutral-400">
                          by {entry.agentRole}
                        </span>
                      )}
                    </div>
                    <p className="mt-1 text-sm font-medium text-neutral-900">
                      {entry.title}
                    </p>
                    <p className="mt-0.5 text-sm text-neutral-600">
                      {entry.description}
                    </p>
                  </div>
                </div>
              </div>
            </li>
          );
        })}
      </ul>

      {hasMore && !expanded && (
        <div className="pt-4 text-center">
          <button
            type="button"
            className="text-sm font-medium text-indigo-600 hover:text-indigo-500"
            onClick={() => setExpanded(true)}
          >
            Show {entries.length - maxVisible} more events
          </button>
        </div>
      )}
    </div>
  );
}
