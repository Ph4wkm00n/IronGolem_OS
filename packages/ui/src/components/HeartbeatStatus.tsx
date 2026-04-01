import React from "react";
import type { HeartbeatStatus as HBStatus } from "@irongolem/schema";
import { heartbeatLabel } from "@irongolem/schema";

export interface HeartbeatStatusProps {
  readonly status: HBStatus;
  /** Optional message providing more detail. */
  readonly message?: string;
  /** Uptime in seconds, shown as human-readable duration. */
  readonly uptimeSeconds?: number;
  /** Size variant. */
  readonly size?: "sm" | "md" | "lg";
}

const statusConfig: Record<HBStatus, { color: string; bg: string; ring: string; pulse: boolean }> = {
  healthy: {
    color: "text-emerald-700",
    bg: "bg-emerald-100",
    ring: "ring-emerald-500",
    pulse: false,
  },
  "quietly-recovering": {
    color: "text-blue-700",
    bg: "bg-blue-100",
    ring: "ring-blue-500",
    pulse: true,
  },
  "needs-attention": {
    color: "text-amber-700",
    bg: "bg-amber-100",
    ring: "ring-amber-500",
    pulse: true,
  },
  paused: {
    color: "text-neutral-600",
    bg: "bg-neutral-100",
    ring: "ring-neutral-400",
    pulse: false,
  },
  quarantined: {
    color: "text-purple-700",
    bg: "bg-purple-100",
    ring: "ring-purple-500",
    pulse: false,
  },
};

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

const sizeStyles = {
  sm: { dot: "h-2 w-2", text: "text-xs", gap: "gap-1.5" },
  md: { dot: "h-3 w-3", text: "text-sm", gap: "gap-2" },
  lg: { dot: "h-4 w-4", text: "text-base", gap: "gap-2.5" },
};

/**
 * Heartbeat status indicator showing the current system health state.
 *
 * Five states map to the Rust runtime's `HeartbeatStatus` enum:
 * - Healthy ("All good")
 * - Quietly Recovering ("Fixing itself")
 * - Needs Attention ("Needs your attention")
 * - Paused ("Paused")
 * - Quarantined ("Isolated for safety")
 */
export function HeartbeatStatus({
  status,
  message,
  uptimeSeconds,
  size = "md",
}: HeartbeatStatusProps) {
  const config = statusConfig[status];
  const s = sizeStyles[size];
  const label = heartbeatLabel[status];

  return (
    <div
      className={`inline-flex items-center ${s.gap} rounded-lg px-3 py-2 ${config.bg}`}
      role="status"
      aria-label={`System status: ${label}`}
    >
      {/* Animated dot */}
      <span className="relative flex">
        {config.pulse && (
          <span
            className={`absolute inline-flex h-full w-full rounded-full ${config.ring.replace("ring-", "bg-")} opacity-30 animate-ping`}
          />
        )}
        <span
          className={`relative inline-flex rounded-full ${s.dot} ${config.ring.replace("ring-", "bg-")}`}
        />
      </span>

      <div className="flex flex-col">
        <span className={`font-medium ${s.text} ${config.color}`}>
          {label}
        </span>
        {message && (
          <span className={`text-neutral-600 ${size === "lg" ? "text-sm" : "text-xs"}`}>
            {message}
          </span>
        )}
      </div>

      {uptimeSeconds !== undefined && (
        <span className="ml-auto text-xs text-neutral-500">
          up {formatUptime(uptimeSeconds)}
        </span>
      )}
    </div>
  );
}
