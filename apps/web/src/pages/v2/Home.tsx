// route: /
// purpose: Workspace landing — heartbeat + recent activity. Replaces the
// legacy Home page when VITE_ENABLE_V2_UI=true. Ported from Claude Design's
// "Workspace Dashboard.html" with the resolved variant set (density=cozy,
// layout=list, perm=inline, trust=chip, why=link/drawer, theme=light).
//
// Integration notes for the next pass:
// - The placeholder components below (HeartbeatStatus, PolicyCard, ResearchCard,
//   RiskBadge, SafetyCard, Timeline) stand in for @irongolem/ui components of
//   the same name. Swap imports at promotion. The audit script (Step F3 of
//   Plans/integrate-claude-design.md) flags these systematically.
// - Mock data is inline at the top per the design guide. A later pass moves
//   it to apps/web/src/_mocks/dashboard.ts and wires through lib/api.ts.

import React, { useEffect, useMemo, useReducer, useState } from "react";

import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

import {
  api,
  type EventItem,
  type EventStatus,
  type Team,
  type ResearchFinding,
  type HeartbeatState,
} from "../../lib/api";

type ToneName =
  | "safe"
  | "warning"
  | "blocked"
  | "recovered"
  | "quarantined"
  | "accent"
  | "neutral";

interface StatusMeta {
  readonly label: string;
  readonly color: ToneName;
  readonly order: number;
  readonly needsAttn: boolean;
}

// ─────────────────────────────────────────────────────────────────────────────
// Tone classmap — replaces dynamic `bg-${tone}` template strings so Tailwind
// sees every class at compile time. Each row is the complete kit for one tone.
// ─────────────────────────────────────────────────────────────────────────────

interface ToneClasses {
  readonly bg: string;
  readonly text: string;
  readonly border: string;
  readonly bgSolid: string;
  readonly bgSolidHover: string;
  readonly textSolid: string;
}

const TONE: Readonly<Record<ToneName, ToneClasses>> = {
  safe: {
    bg: "bg-safe",
    text: "text-safe",
    border: "border-safe",
    bgSolid: "bg-safe-solid",
    bgSolidHover: "hover:bg-safe-solid-hover",
    textSolid: "text-safe-solid",
  },
  warning: {
    bg: "bg-warning",
    text: "text-warning",
    border: "border-warning",
    bgSolid: "bg-warning-solid",
    bgSolidHover: "hover:bg-warning-solid-hover",
    textSolid: "text-warning-solid",
  },
  blocked: {
    bg: "bg-blocked",
    text: "text-blocked",
    border: "border-blocked",
    bgSolid: "bg-blocked-solid",
    bgSolidHover: "hover:bg-blocked-solid-hover",
    textSolid: "text-blocked-solid",
  },
  recovered: {
    bg: "bg-recovered",
    text: "text-recovered",
    border: "border-recovered",
    bgSolid: "bg-recovered-solid",
    bgSolidHover: "hover:bg-recovered-solid-hover",
    textSolid: "text-recovered-solid",
  },
  quarantined: {
    bg: "bg-quarantined",
    text: "text-quarantined",
    border: "border-quarantined",
    bgSolid: "bg-quarantined-solid",
    bgSolidHover: "hover:bg-quarantined-solid-hover",
    textSolid: "text-quarantined-solid",
  },
  accent: {
    bg: "bg-accent",
    text: "text-accent",
    border: "border-accent",
    bgSolid: "bg-accent-solid",
    bgSolidHover: "hover:bg-accent-solid-hover",
    textSolid: "text-accent-solid",
  },
  neutral: {
    bg: "bg-neutral",
    text: "text-neutral-700",
    border: "border-neutral-200",
    bgSolid: "bg-neutral-solid",
    bgSolidHover: "hover:bg-neutral-solid-hover",
    textSolid: "text-neutral-solid",
  },
};

const cls = (...c: ReadonlyArray<string | false | null | undefined>): string =>
  c.filter(Boolean).join(" ");

// ─────────────────────────────────────────────────────────────────────────────
// Inline SVG icons — matches the existing app shell's Heroicons-style strokes.
// ─────────────────────────────────────────────────────────────────────────────

type IconProps = { size?: number; className?: string };

const svgBase = (size: number, className: string) => ({
  xmlns: "http://www.w3.org/2000/svg",
  viewBox: "0 0 24 24",
  width: size,
  height: size,
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.5,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  className,
  "aria-hidden": true,
});

const IconCheck = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="m5 12 5 5L20 7" />
  </svg>
);
const IconCheckCircle = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <circle cx={12} cy={12} r={9} />
    <path d="m8 12 3 3 5-6" />
  </svg>
);
const IconXCircle = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <circle cx={12} cy={12} r={9} />
    <path d="m9 9 6 6M15 9l-6 6" />
  </svg>
);
const IconAlertTriangle = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M12 4 2.5 20h19L12 4Z" />
    <path d="M12 10v4" />
    <circle cx={12} cy={17.5} r={0.5} fill="currentColor" stroke="none" />
  </svg>
);
const IconShield = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" />
  </svg>
);
const IconShieldOff = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" />
    <path d="m5 5 14 14" />
  </svg>
);
const IconLock = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <rect x={5} y={11} width={14} height={9} rx={2} />
    <path d="M8 11V8a4 4 0 1 1 8 0v3" />
  </svg>
);
const IconBell = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2Z" />
    <path d="M10 20a2 2 0 0 0 4 0" />
  </svg>
);
const IconPause = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M9 5v14" />
    <path d="M15 5v14" />
  </svg>
);
const IconSlash = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M5 19 19 5" />
  </svg>
);
const IconRefresh = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M3 12a9 9 0 0 1 15-6.7L21 8" />
    <path d="M21 3v5h-5" />
    <path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
    <path d="M3 21v-5h5" />
  </svg>
);
const IconClock = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <circle cx={12} cy={12} r={9} />
    <path d="M12 7v5l3 2" />
  </svg>
);
const IconArrowRight = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M5 12h14" />
    <path d="m13 6 6 6-6 6" />
  </svg>
);
const IconSparkles = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" />
  </svg>
);
const STATUS_ICON: Readonly<Record<EventStatus, React.ComponentType<IconProps>>> = {
  proposed: IconBell,
  blocked: IconShieldOff,
  quarantined: IconLock,
  taken: IconCheck,
  healed: IconRefresh,
  "research-update": IconSparkles,
  "squad-handoff": IconArrowRight,
};

// ─────────────────────────────────────────────────────────────────────────────
// Mock data — sourced from `api.v2.home.getMock()`. Mock vs real toggles via
// `VITE_API_MODE`; pages remain sync until real endpoints land (F6).
// ─────────────────────────────────────────────────────────────────────────────

const HOME_MOCK = api.v2.home.getMock();
const WORKSPACE = HOME_MOCK.workspace;
const HEARTBEAT = HOME_MOCK.heartbeat;
const TEAMS = HOME_MOCK.teams;
const TRUST_HISTORY = HOME_MOCK.trustHistory;
const SAFETY = HOME_MOCK.safety;
const RESEARCH_FINDINGS = HOME_MOCK.researchFindings;
const INITIAL_EVENTS = HOME_MOCK.events;

const STATUS_META: Readonly<Record<EventStatus, StatusMeta>> = {
  proposed: { label: "Awaiting approval", color: "warning", order: 0, needsAttn: true },
  blocked: { label: "Blocked", color: "blocked", order: 1, needsAttn: true },
  quarantined: { label: "Quarantined", color: "quarantined", order: 2, needsAttn: true },
  taken: { label: "Done", color: "safe", order: 3, needsAttn: false },
  healed: { label: "Auto-healed", color: "recovered", order: 4, needsAttn: false },
  "research-update": { label: "Research", color: "accent", order: 5, needsAttn: false },
  "squad-handoff": { label: "Handoff", color: "neutral", order: 6, needsAttn: false },
};

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

function relTime(min: number): string {
  if (min < 1) return "just now";
  if (min < 60) return `${min}m ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

function relTimeFull(min: number): string {
  if (min < 1) return "just now";
  if (min < 60) return `${min} minute${min === 1 ? "" : "s"} ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h} hour${h === 1 ? "" : "s"} ago`;
  return `${Math.floor(h / 24)} day${Math.floor(h / 24) === 1 ? "" : "s"} ago`;
}

function nextStepsFor(status: EventStatus): readonly string[] {
  switch (status) {
    case "proposed":
      return [
        "Approve to run the action now.",
        "Deny to keep it from running.",
        "Edit the draft inline before approving.",
      ];
    case "blocked":
      return ["The action did not run.", "Adjust the safety rule, or take the action yourself."];
    case "quarantined":
      return ["The draft is held in your inbox.", "Open it, edit, then send if it looks right."];
    case "taken":
      return ["This action ran inside the rules.", "It will appear in the daily summary."];
    case "healed":
      return [
        "Auto-heal ran on its own.",
        "If this kind of failure repeats, the rule will be reviewed.",
      ];
    case "research-update":
      return ["No action needed — context updated.", "Affected teams adjusted automatically."];
    case "squad-handoff":
      return [
        "Work moved between teams.",
        "You'll see the next action in the receiving team's events.",
      ];
  }
}

const RISK_TONE: Readonly<Record<"low" | "medium" | "high", ToneName>> = {
  low: "safe",
  medium: "warning",
  high: "blocked",
};

const SCOPE_TONE: Readonly<Record<"scoped" | "broad" | "restricted", ToneName>> = {
  scoped: "neutral",
  broad: "warning",
  restricted: "blocked",
};

// ─────────────────────────────────────────────────────────────────────────────
// Reducer — simulated state transitions for approve/deny/reset.
// ─────────────────────────────────────────────────────────────────────────────

type EventsAction =
  | { type: "approve"; id: string }
  | { type: "deny"; id: string }
  | { type: "reset" };

function eventsReducer(state: EventItem[], action: EventsAction): EventItem[] {
  switch (action.type) {
    case "approve":
      return state.map((e) =>
        e.id === action.id
          ? { ...e, status: "taken", minutesAgo: 0, why: "You approved this just now.", cause: undefined }
          : e,
      );
    case "deny":
      return state.map((e) =>
        e.id === action.id
          ? {
              ...e,
              status: "blocked",
              minutesAgo: 0,
              cause: "You denied this just now.",
              why: "You held this action. Nothing was sent.",
            }
          : e,
      );
    case "reset":
      return INITIAL_EVENTS.map((e) => ({ ...e }));
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Sub-components — kept inline as placeholders for @irongolem/ui. The audit
// pipeline flags these for substitution at promotion.
// ─────────────────────────────────────────────────────────────────────────────

interface StatusMarkProps {
  readonly status: EventStatus;
  readonly size?: number;
}
function StatusMark({ status, size = 28 }: StatusMarkProps) {
  const tone = TONE[STATUS_META[status].color];
  const Icon = STATUS_ICON[status];
  return (
    <span
      className={cls(
        "inline-flex items-center justify-center rounded-full shrink-0",
        tone.bg,
        tone.text,
      )}
      style={{ width: size, height: size }}
    >
      <Icon size={Math.round(size * 0.5)} />
    </span>
  );
}

function StatusChip({ status }: { readonly status: EventStatus }) {
  const meta = STATUS_META[status];
  const tone = TONE[meta.color];
  return (
    <span
      className={cls(
        "inline-flex items-center gap-1 rounded-full border text-[10px] font-medium uppercase tracking-wide px-1.5 py-0.5",
        tone.bg,
        tone.text,
        tone.border,
      )}
    >
      {meta.label}
    </span>
  );
}

function TeamPill({ team }: { readonly team: Team | undefined }) {
  if (!team) return null;
  const tone = TONE[team.color];
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-neutral-600">
      <span className={cls("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
      {team.name}
    </span>
  );
}

// TODO(integrator): swap for <RiskBadge /> from @irongolem/ui.
function RiskBadge({ risk }: { readonly risk: "low" | "medium" | "high" }) {
  const tone = TONE[RISK_TONE[risk]];
  const label = risk === "medium" ? "med risk" : `${risk} risk`;
  return (
    <span
      className={cls(
        "inline-flex items-center gap-1 rounded-full border font-medium text-[10px] px-1.5 py-0.5",
        tone.bg,
        tone.text,
        tone.border,
      )}
    >
      <span className={cls("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
      {label}
    </span>
  );
}

interface PermissionBadgeProps {
  readonly permission: string;
  readonly scope: "scoped" | "broad" | "restricted";
}
function PermissionBadge({ permission, scope }: PermissionBadgeProps) {
  const tone = TONE[SCOPE_TONE[scope]];
  return (
    <span
      className={cls(
        "inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] font-medium",
        tone.bg,
        tone.text,
        tone.border,
      )}
    >
      <IconLock size={11} />
      <span className="font-mono lowercase">{permission}</span>
    </span>
  );
}

interface KpiProps {
  readonly label: string;
  readonly value: number;
  readonly tone: ToneName;
  readonly icon: React.ComponentType<IconProps>;
  readonly hint: string;
}
function Kpi({ label, value, tone, icon: Icon, hint }: KpiProps) {
  const t = TONE[tone];
  return (
    <div className="card card-padded relative overflow-hidden">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-medium uppercase tracking-wider text-neutral-500">
          {label}
        </span>
        <span className={t.text}>
          <Icon size={14} />
        </span>
      </div>
      <div
        className={cls(
          "mt-2 text-2xl font-semibold tracking-tight tabular-nums",
          t.text,
        )}
      >
        {value}
      </div>
      <div className="text-xs text-neutral-500 mt-1 leading-snug">{hint}</div>
    </div>
  );
}

// TODO(integrator): swap for <HeartbeatStatus /> from @irongolem/ui.
function HeartbeatStatus({ onOpenWhy }: { readonly onOpenWhy: () => void }) {
  const isHealthy = HEARTBEAT.status === "healthy";
  return (
    <div className="card card-padded">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <span className="relative inline-flex">
            <span
              className={cls(
                "h-2.5 w-2.5 rounded-full",
                isHealthy ? "bg-safe-solid ig-pulse" : "bg-warning-solid",
              )}
            />
          </span>
          <div>
            <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">
              Heartbeat
            </div>
            <div className="text-base font-semibold tracking-tight text-neutral-900 mt-0.5">
              {isHealthy ? "Everything is running normally" : "One system is recovering"}
            </div>
          </div>
        </div>
        <button
          type="button"
          onClick={onOpenWhy}
          className="text-xs text-accent hover:text-accent-solid font-medium px-2 py-1 rounded hover:bg-accent-hover transition-colors"
        >
          Why?
        </button>
      </div>

      <div className="mt-4 grid grid-cols-3 gap-3">
        <Stat
          label="Systems"
          value={`${HEARTBEAT.systemsGreen}/${HEARTBEAT.systemsTotal}`}
          tone={HEARTBEAT.systemsGreen === HEARTBEAT.systemsTotal ? "safe" : "warning"}
          hint="green / total"
        />
        <Stat label="Uptime" value={WORKSPACE.uptimeStreak} tone="neutral" hint={`${WORKSPACE.uptimeHours}h streak`} />
        <Stat
          label="Last sync"
          value={WORKSPACE.lastSync.replace(" ago", "")}
          tone="neutral"
          hint="ago"
        />
      </div>

      {!isHealthy && (
        <div className="mt-3 rounded-md bg-warning border border-warning p-3 text-sm text-warning">
          <div className="font-medium">{HEARTBEAT.oneDegraded.name}</div>
          <div className="opacity-90 mt-0.5">{HEARTBEAT.oneDegraded.reason}</div>
        </div>
      )}
    </div>
  );
}

interface StatProps {
  readonly label: string;
  readonly value: string;
  readonly hint: string;
  readonly tone: ToneName;
}
function Stat({ label, value, hint, tone }: StatProps) {
  const text = tone === "neutral" ? "text-neutral-900" : TONE[tone].text;
  return (
    <div>
      <div className="text-[10px] font-medium uppercase tracking-wider text-neutral-500">
        {label}
      </div>
      <div className={cls("mt-1 text-lg font-semibold tracking-tight tabular-nums", text)}>
        {value}
      </div>
      <div className="text-[11px] text-neutral-400 mt-0.5">{hint}</div>
    </div>
  );
}

// ─── Filter chips ────────────────────────────────────────────────────────────

interface FilterChipsProps {
  readonly active: FilterId;
  readonly onChange: (id: FilterId) => void;
  readonly counts: EventCounts;
}
type FilterId = "all" | "attention" | "proposed" | "blocked" | "healed";
type EventCounts = Record<EventStatus, number>;

const FILTER_CHIPS: ReadonlyArray<{ readonly id: FilterId; readonly label: string }> = [
  { id: "all", label: "All activity" },
  { id: "attention", label: "Needs attention" },
  { id: "proposed", label: "Awaiting approval" },
  { id: "blocked", label: "Blocked" },
  { id: "healed", label: "Auto-healed" },
];

function FilterChips({ active, onChange, counts }: FilterChipsProps) {
  const countFor = (id: FilterId): number | null => {
    if (id === "all") return null;
    if (id === "attention") return counts.proposed + counts.blocked + counts.quarantined;
    if (id === "proposed") return counts.proposed;
    if (id === "blocked") return counts.blocked;
    if (id === "healed") return counts.healed;
    return null;
  };
  return (
    <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
      {FILTER_CHIPS.map((c) => {
        const isActive = c.id === active;
        const n = countFor(c.id);
        return (
          <button
            key={c.id}
            type="button"
            onClick={() => onChange(c.id)}
            className={cls(
              "inline-flex items-center gap-1.5 rounded-md text-xs font-medium px-2.5 py-1.5 transition-colors",
              isActive ? "bg-neutral-100 text-neutral-900" : "text-neutral-600 hover:text-neutral-900",
            )}
          >
            {c.label}
            {n != null && (
              <span
                className={cls(
                  "rounded-full font-mono text-[10px] px-1.5 py-0.5",
                  isActive
                    ? "bg-white border border-neutral-200 text-neutral-700"
                    : "bg-neutral-100 text-neutral-500",
                )}
              >
                {n}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}

// ─── EventRow + Timeline ─────────────────────────────────────────────────────

interface EventRowProps {
  readonly event: EventItem;
  readonly team: Team | undefined;
  readonly onApprove: (ev: EventItem) => void;
  readonly onDeny: (ev: EventItem) => void;
  readonly onOpenDrawer: (ev: EventItem) => void;
}
function EventRow({ event, team, onApprove, onDeny, onOpenDrawer }: EventRowProps) {
  const isProposed = event.status === "proposed";
  const isCaused = (event.status === "blocked" || event.status === "quarantined") && !!event.cause;
  const causeTone = TONE[event.status === "blocked" ? "blocked" : "quarantined"];

  return (
    <div
      className="flex gap-3 hover:bg-neutral-50 transition-colors group px-4 py-3"
      data-status={event.status}
    >
      <StatusMark status={event.status} size={28} />

      <div className="flex-1 min-w-0">
        <div className="flex items-start gap-2">
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium text-neutral-900 leading-snug">
              {event.title}
            </div>

            {isCaused && event.cause && (
              <div
                className={cls(
                  "mt-1 inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs",
                  causeTone.bg,
                  causeTone.text,
                )}
              >
                <IconAlertTriangle size={12} />
                <span>{event.cause}</span>
              </div>
            )}

            <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-neutral-500">
              <TeamPill team={team} />
              <StatusChip status={event.status} />
              <RiskBadge risk={event.risk} />
              <PermissionBadge permission={event.permission} scope={event.permissionScope} />
              <span className="inline-flex items-center gap-1">
                <IconClock size={11} />
                {relTime(event.minutesAgo)}
              </span>
              {event.target && (
                <span className="font-mono text-[11px] text-neutral-400 truncate max-w-[260px]">
                  → {event.target}
                </span>
              )}
            </div>
          </div>

          <div className="flex flex-col items-end gap-2 shrink-0">
            <button
              type="button"
              onClick={() => onOpenDrawer(event)}
              className="text-xs text-accent hover:text-accent-solid font-medium opacity-0 group-hover:opacity-100 focus-visible:opacity-100 transition-opacity inline-flex items-center gap-1"
            >
              Why?
              <IconArrowRight size={11} />
            </button>
            {isProposed && (
              <div className="flex items-center gap-1.5">
                <button
                  type="button"
                  onClick={() => onDeny(event)}
                  className="text-xs font-medium px-2.5 py-1.5 rounded-md border border-neutral-200 text-neutral-700 hover:bg-neutral-50 transition-colors"
                >
                  Deny
                </button>
                <button
                  type="button"
                  onClick={() => onApprove(event)}
                  className="text-xs font-medium px-2.5 py-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover transition-colors inline-flex items-center gap-1"
                >
                  <IconCheck size={12} />
                  Approve
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// TODO(integrator): swap for <Timeline /> from @irongolem/ui once its API
// accepts approve/deny + drawer callbacks. The repo's current Timeline is
// presentation-only.
interface TimelineProps {
  readonly events: readonly EventItem[];
  readonly onApprove: (ev: EventItem) => void;
  readonly onDeny: (ev: EventItem) => void;
  readonly onOpenDrawer: (ev: EventItem) => void;
}
function Timeline({ events, onApprove, onDeny, onOpenDrawer }: TimelineProps) {
  if (events.length === 0) {
    return (
      <div className="card card-padded">
        <div className="flex flex-col items-center text-center py-10 max-w-md mx-auto">
          <div className="h-12 w-12 rounded-full bg-safe inline-flex items-center justify-center mb-4">
            <IconCheckCircle size={24} className="text-safe" />
          </div>
          <h3 className="section-title">Nothing needs your attention</h3>
          <p className="text-sm text-neutral-500 mt-2">
            Your assistant teams are working inside their charters. New activity will show up here as it happens.
          </p>
          <div className="mt-4 inline-flex items-center gap-1.5 text-xs text-neutral-400">
            <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
            Heartbeat green for 17 days
          </div>
        </div>
      </div>
    );
  }
  return (
    <div className="card overflow-hidden divide-y divide-neutral-100">
      {events.map((ev) => (
        <EventRow
          key={ev.id}
          event={ev}
          team={TEAMS.find((t) => t.id === ev.teamId)}
          onApprove={onApprove}
          onDeny={onDeny}
          onOpenDrawer={onOpenDrawer}
        />
      ))}
    </div>
  );
}

// ─── TeamCard ────────────────────────────────────────────────────────────────

interface TeamCardProps {
  readonly team: Team;
  readonly activity: number;
  readonly history: readonly number[];
}
function TeamCard({ team, activity, history }: TeamCardProps) {
  const score = useMemo(() => {
    if (history.length === 0) return 9;
    const sum = history.reduce((s, n) => s + n, 0);
    return Math.round((sum / history.length) * 10) / 10;
  }, [history]);
  const scoreTone = TONE[score >= 8.9 ? "safe" : score >= 8 ? "warning" : "blocked"];
  const initials = team.name
    .split(" ")
    .map((w) => w[0])
    .join("")
    .slice(0, 2);
  const teamTone = TONE[team.color];

  return (
    <div className="card card-padded">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3 min-w-0">
          <span
            className={cls(
              "h-8 w-8 rounded-lg shrink-0 inline-flex items-center justify-center font-semibold text-[11px]",
              teamTone.bg,
              teamTone.text,
            )}
          >
            {initials}
          </span>
          <div className="min-w-0">
            <div className="text-sm font-semibold text-neutral-900 tracking-tight">
              {team.name}
            </div>
            <div className="text-xs text-neutral-500 mt-0.5 truncate">{team.description}</div>
          </div>
        </div>
        <span
          className={cls(
            "shrink-0 inline-flex items-center gap-1 rounded-full text-[10px] font-medium px-1.5 py-0.5 border",
            scoreTone.bg,
            scoreTone.text,
            scoreTone.border,
          )}
        >
          {score.toFixed(1)} / 10
        </span>
      </div>

      <div className="mt-3">
        <div className="flex items-end gap-[2px] h-7">
          {history.map((v, i) => {
            const tone = TONE[v >= 9 ? "safe" : v >= 7 ? "warning" : "blocked"];
            return (
              <span
                key={i}
                className={cls("flex-1 rounded-sm", tone.bgSolid)}
                style={{ height: `${(v / 10) * 100}%`, opacity: v >= 9 ? 0.55 : 0.85 }}
              />
            );
          })}
        </div>
        <div className="flex justify-between text-[10px] text-neutral-400 mt-1 font-mono">
          <span>24h ago</span>
          <span>{activity} events</span>
          <span>now</span>
        </div>
      </div>
    </div>
  );
}

// ─── ResearchCard ────────────────────────────────────────────────────────────

// TODO(integrator): swap for <ResearchCard /> from @irongolem/ui.
function ResearchCardLocal({ finding }: { readonly finding: ResearchFinding }) {
  const pct = Math.round(finding.confidence * 100);
  const tone = TONE[pct >= 85 ? "safe" : pct >= 70 ? "warning" : "blocked"];
  return (
    <article className="card card-padded h-full flex flex-col">
      <div className="flex items-start justify-between gap-3">
        <h3 className="text-base font-semibold tracking-tight text-neutral-900 leading-snug">
          {finding.title}
        </h3>
        <span
          className={cls(
            "shrink-0 inline-flex items-center gap-1 rounded-full border text-[10px] font-medium px-1.5 py-0.5",
            tone.bg,
            tone.text,
            tone.border,
          )}
        >
          {pct}%
        </span>
      </div>
      <p className="text-sm text-neutral-700 mt-2 flex-1">{finding.summary}</p>
      <div className="mt-3 flex items-center justify-between text-xs text-neutral-500">
        <span className="font-mono">{finding.source}</span>
        <span className="inline-flex items-center gap-1">
          <IconClock size={12} />
          {finding.freshness}
        </span>
      </div>
    </article>
  );
}

// ─── SafetyCard ──────────────────────────────────────────────────────────────

// TODO(integrator): swap for <SafetyCard /> from @irongolem/ui — the real
// component takes flat arrays as props (canAccess, cannotAccess, etc.) so the
// mapping is direct.
function SafetyCardLocal() {
  const sections: ReadonlyArray<{
    readonly label: string;
    readonly items: readonly string[];
    readonly tone: ToneName;
    readonly Icon: React.ComponentType<IconProps>;
  }> = [
    { label: "Can", items: SAFETY.can, tone: "safe", Icon: IconCheck },
    { label: "Needs approval", items: SAFETY.needsApproval, tone: "warning", Icon: IconBell },
    { label: "Cannot", items: SAFETY.cannot, tone: "blocked", Icon: IconSlash },
    { label: "Stops if", items: SAFETY.stopsIf, tone: "quarantined", Icon: IconPause },
  ];
  return (
    <div className="card card-padded">
      <div className="flex items-center justify-between mb-4">
        <h2 className="section-title">What this workspace can do today</h2>
        <span className="inline-flex items-center gap-1.5 text-xs text-safe font-medium">
          <IconShield size={14} />
          Posture: {SAFETY.posture}
        </span>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-5">
        {sections.map((s) => {
          const tone = TONE[s.tone];
          return (
            <div key={s.label}>
              <div className="flex items-center gap-1.5 mb-2">
                <span className={tone.text}>
                  <s.Icon size={14} />
                </span>
                <span
                  className={cls(
                    "text-xs font-medium uppercase tracking-wide",
                    tone.text,
                  )}
                >
                  {s.label}
                </span>
              </div>
              <ul className="space-y-1">
                {s.items.map((item) => (
                  <li key={item} className="text-sm text-neutral-700 flex gap-2">
                    <span
                      className={cls(
                        "mt-1.5 h-1 w-1 rounded-full shrink-0",
                        tone.bgSolid,
                      )}
                    />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── PolicyCard ──────────────────────────────────────────────────────────────

// TODO(integrator): swap for <PolicyCard /> from @irongolem/ui.
function PolicyCardLocal({ onOpenRules }: { readonly onOpenRules: () => void }) {
  return (
    <div className="card card-padded">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">
            Safety rules
          </div>
          <h2 className="section-title mt-0.5">Five layers, all active</h2>
        </div>
        <button
          type="button"
          onClick={onOpenRules}
          className="text-sm text-accent hover:text-accent-solid font-medium"
        >
          Open rules
        </button>
      </div>

      <ol className="mt-4 space-y-2">
        {SAFETY.layers.map((layer) => {
          const isWatching = layer.state === "watching";
          return (
            <li
              key={layer.id}
              className={cls(
                "flex items-center gap-3 rounded-lg border px-3 py-2",
                isWatching ? "bg-warning border-warning" : "border-neutral-100 bg-white",
              )}
            >
              <span
                className={cls(
                  "h-6 w-6 rounded-full inline-flex items-center justify-center text-[11px] font-semibold",
                  isWatching ? "bg-warning-solid text-white" : "bg-safe-solid text-white",
                )}
              >
                {layer.id}
              </span>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-neutral-900">{layer.name}</div>
                <div className="text-xs text-neutral-500 truncate">{layer.note}</div>
              </div>
              {isWatching ? (
                <IconAlertTriangle size={16} className="text-warning" />
              ) : (
                <IconCheckCircle size={16} className="text-safe" />
              )}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

// ─── Drawers ─────────────────────────────────────────────────────────────────

interface WhyDrawerProps {
  readonly event: EventItem;
  readonly team: Team | undefined;
  readonly onClose: () => void;
}
function WhyDrawer({ event, team, onClose }: WhyDrawerProps) {
  const meta = STATUS_META[event.status];
  const causeTone = TONE[event.status === "blocked" ? "blocked" : event.status === "quarantined" ? "quarantined" : "warning"];
  return (
    <div className="fixed inset-0 z-40" role="dialog" aria-modal="true" aria-label="Why this happened">
      <div className="absolute inset-0 ig-drawer-backdrop" onClick={onClose} />
      <aside className="absolute right-0 top-0 bottom-0 w-full sm:w-[440px] bg-white shadow-xl border-l border-neutral-100 overflow-y-auto scrollbar-thin ig-slide-in">
        <div className="sticky top-0 bg-white border-b border-neutral-100 px-5 py-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className={cls("h-2 w-2 rounded-full", TONE[meta.color].bgSolid)} />
            <span
              className={cls(
                "text-xs font-medium uppercase tracking-wide",
                TONE[meta.color].text,
              )}
            >
              Why this happened
            </span>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close drawer"
            className="text-neutral-500 hover:text-neutral-900 p-1.5 rounded-md hover:bg-neutral-50"
          >
            <IconXCircle size={16} />
          </button>
        </div>

        <div className="p-5 space-y-5">
          <div>
            <h2 className="section-title">{event.title}</h2>
            <div className="mt-2 flex items-center gap-3 text-xs text-neutral-500">
              <TeamPill team={team} />
              <span>·</span>
              <span>{relTimeFull(event.minutesAgo)}</span>
            </div>
          </div>

          {event.cause && (
            <div
              className={cls(
                "rounded-lg p-4 border",
                causeTone.bg,
                causeTone.border,
                causeTone.text,
              )}
            >
              <div className="text-xs font-medium uppercase tracking-wide mb-1">Cause</div>
              <div className="text-sm">{event.cause}</div>
            </div>
          )}

          <DrawerSection label="Reason">
            <p className="text-sm text-neutral-700 leading-relaxed">{event.why}</p>
          </DrawerSection>

          <DrawerSection label="Permission used">
            <div className="rounded-md border border-neutral-200 p-3">
              <div className="font-mono text-sm text-neutral-900">{event.permission}</div>
              <div className="mt-1 flex items-center gap-2 text-xs text-neutral-500">
                <span className="capitalize">{event.permissionScope}</span>
                <span>·</span>
                <RiskBadge risk={event.risk} />
              </div>
              {event.approvals != null && event.approvals > 0 && (
                <div className="mt-2 text-xs text-neutral-600">
                  Approved{" "}
                  <span className="font-medium tabular-nums">
                    {event.approvals.toLocaleString()}×
                  </span>{" "}
                  before in this workspace.
                </div>
              )}
              {event.approvals === 0 && (
                <div className="mt-2 text-xs text-warning">
                  First time this exact action has come up.
                </div>
              )}
            </div>
          </DrawerSection>

          {event.target && (
            <DrawerSection label="Target">
              <div className="font-mono text-sm text-neutral-700">{event.target}</div>
            </DrawerSection>
          )}

          <DrawerSection label="What happens next">
            <ul className="text-sm text-neutral-700 space-y-1.5">
              {nextStepsFor(event.status).map((s) => (
                <li key={s} className="flex gap-2">
                  <span className="text-neutral-300 mt-1">·</span>
                  <span>{s}</span>
                </li>
              ))}
            </ul>
          </DrawerSection>
        </div>
      </aside>
    </div>
  );
}

function DrawerSection({ label, children }: { readonly label: string; readonly children: React.ReactNode }) {
  return (
    <div>
      <div className="text-xs font-medium uppercase tracking-wide text-neutral-500 mb-2">
        {label}
      </div>
      {children}
    </div>
  );
}

function SafetyDrawer({ onClose }: { readonly onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-40" role="dialog" aria-modal="true" aria-label="Safety rules">
      <div className="absolute inset-0 ig-drawer-backdrop" onClick={onClose} />
      <aside className="absolute right-0 top-0 bottom-0 w-full sm:w-[480px] bg-white shadow-xl border-l border-neutral-100 overflow-y-auto scrollbar-thin ig-slide-in">
        <div className="sticky top-0 bg-white border-b border-neutral-100 px-5 py-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <IconShield size={16} className="text-accent" />
            <span className="text-xs font-medium uppercase tracking-wide text-neutral-500">
              Safety rules
            </span>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close drawer"
            className="text-neutral-500 hover:text-neutral-900 p-1.5 rounded-md hover:bg-neutral-50"
          >
            <IconXCircle size={16} />
          </button>
        </div>
        <div className="p-5 space-y-4">
          <PolicyCardLocal onOpenRules={onClose} />
          <SafetyCardLocal />
        </div>
      </aside>
    </div>
  );
}

// ─── Toast ───────────────────────────────────────────────────────────────────

interface ToastState {
  readonly kind: "approved" | "denied";
  readonly title: string;
}

function Toast({ toast }: { readonly toast: ToastState }) {
  const tone = TONE[toast.kind === "approved" ? "safe" : "blocked"];
  const Icon = toast.kind === "approved" ? IconCheckCircle : IconXCircle;
  const verb = toast.kind === "approved" ? "Approved" : "Denied";
  return (
    <div
      className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50 pointer-events-none ig-toast-in"
      role="status"
      aria-live="polite"
    >
      <div
        className={cls(
          "pointer-events-auto rounded-lg border shadow-lg bg-white px-4 py-3 flex items-center gap-3 min-w-[280px] max-w-[420px]",
          tone.border,
        )}
      >
        <span className={tone.text}>
          <Icon size={18} />
        </span>
        <div className="flex-1 min-w-0">
          <div
            className={cls(
              "text-xs font-medium uppercase tracking-wide",
              tone.text,
            )}
          >
            {verb}
          </div>
          <div className="text-sm text-neutral-900 truncate">{toast.title}</div>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route component — replaces /'s legacy Home page when VITE_ENABLE_V2_UI.
// ─────────────────────────────────────────────────────────────────────────────

const FILTER_PREDICATE: Readonly<Record<FilterId, (e: EventItem) => boolean>> = {
  all: () => true,
  attention: (e) => STATUS_META[e.status].needsAttn,
  proposed: (e) => e.status === "proposed",
  blocked: (e) => e.status === "blocked",
  healed: (e) => e.status === "healed",
};

export function Home() {
  const [events, dispatch] = useReducer(eventsReducer, INITIAL_EVENTS, (initial) =>
    initial.map((e) => ({ ...e })),
  );
  const [filter, setFilter] = useState<FilterId>("all");
  const [drawerEvent, setDrawerEvent] = useState<EventItem | null>(null);
  const [policyOpen, setPolicyOpen] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);

  const filteredEvents = useMemo(
    () => events.filter(FILTER_PREDICATE[filter]),
    [events, filter],
  );

  const counts = useMemo<EventCounts>(() => {
    const c: EventCounts = {
      proposed: 0,
      blocked: 0,
      quarantined: 0,
      taken: 0,
      healed: 0,
      "research-update": 0,
      "squad-handoff": 0,
    };
    for (const e of events) c[e.status] += 1;
    return c;
  }, [events]);

  useEffect(() => {
    if (!toast) return undefined;
    const id = setTimeout(() => setToast(null), 3200);
    return () => clearTimeout(id);
  }, [toast]);

  const onApprove = (ev: EventItem) => {
    dispatch({ type: "approve", id: ev.id });
    setToast({ kind: "approved", title: ev.title });
  };
  const onDeny = (ev: EventItem) => {
    dispatch({ type: "deny", id: ev.id });
    setToast({ kind: "denied", title: ev.title });
  };

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar onResetDemo={() => dispatch({ type: "reset" })} />

      <main className="page-container">
        <section className="grid grid-cols-1 lg:grid-cols-3 gap-4 mt-4">
          <div className="lg:col-span-2">
            <h1 className="page-title">Good morning, Adam</h1>
            <p className="text-neutral-600 mt-1">
              Here's what your assistant teams did overnight, what's waiting for you, and anything they couldn't handle on their own.
            </p>

            <div className="mt-5 grid grid-cols-2 md:grid-cols-4 gap-3">
              <Kpi
                label="Done overnight"
                tone="safe"
                value={counts.taken}
                icon={IconCheck}
                hint="Inside the rules. Nothing for you."
              />
              <Kpi
                label="Waiting on you"
                tone="warning"
                value={counts.proposed}
                icon={IconBell}
                hint="Drafted, paused for approval."
              />
              <Kpi
                label="Blocked"
                tone="blocked"
                value={counts.blocked}
                icon={IconShieldOff}
                hint="Held by a safety rule."
              />
              <Kpi
                label="Auto-healed"
                tone="recovered"
                value={counts.healed}
                icon={IconRefresh}
                hint="Fixed on its own."
              />
            </div>
          </div>

          <div>
            <HeartbeatStatus onOpenWhy={() => setPolicyOpen(true)} />
          </div>
        </section>

        <section className="mt-8">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <div>
              <h2 className="section-title">Recent activity</h2>
              <p className="text-sm text-neutral-500 mt-1">
                Every blocked or quarantined event includes the cause.
              </p>
            </div>
            <FilterChips active={filter} onChange={setFilter} counts={counts} />
          </div>

          <div className="mt-4">
            <Timeline
              events={filteredEvents}
              onApprove={onApprove}
              onDeny={onDeny}
              onOpenDrawer={setDrawerEvent}
            />
          </div>
        </section>

        <section className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-4">
          <div className="lg:col-span-2">
            <h2 className="section-title">Assistant teams</h2>
            <p className="text-sm text-neutral-500 mt-1">
              Last 24 hours of reliability per team.
            </p>
            <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
              {TEAMS.map((team) => (
                <TeamCard
                  key={team.id}
                  team={team}
                  activity={events.filter((e) => e.teamId === team.id).length}
                  history={TRUST_HISTORY[team.id] ?? []}
                />
              ))}
            </div>
          </div>

          <div>
            <div className="flex items-end justify-between">
              <div>
                <h2 className="section-title">Research</h2>
                <p className="text-sm text-neutral-500 mt-1">
                  Things you should probably know about today.
                </p>
              </div>
              <a
                href="#research"
                className="text-sm text-accent hover:text-accent-solid font-medium"
              >
                All findings
              </a>
            </div>
            <div className="mt-3 space-y-3">
              {RESEARCH_FINDINGS.map((f) => (
                <ResearchCardLocal key={f.id} finding={f} />
              ))}
            </div>
          </div>
        </section>

        <section className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-4 pb-12">
          <div className="lg:col-span-2">
            <SafetyCardLocal />
          </div>
          <div>
            <PolicyCardLocal onOpenRules={() => setPolicyOpen(true)} />
          </div>
        </section>
      </main>

      {drawerEvent && (
        <WhyDrawer
          event={drawerEvent}
          team={TEAMS.find((t) => t.id === drawerEvent.teamId)}
          onClose={() => setDrawerEvent(null)}
        />
      )}
      {policyOpen && <SafetyDrawer onClose={() => setPolicyOpen(false)} />}
      {toast && <Toast toast={toast} />}
    </div>
  );
}
