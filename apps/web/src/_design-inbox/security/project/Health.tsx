// Health.tsx — IronGolem OS
// Route: /health
// One-file route. Calm-toned, ambient-operations-first surface.
//
// React 19, TS strict, Tailwind utility classes only. Semantic palette
// (safe / recovered / warning / quarantined / neutral) comes from
// globals.css and behaves in both themes.
//
// Mandatory patterns wired in:
//   1. Ambient Operations — healthy components render as compact green
//      chips, NOT full cards. Only non-healthy components expand to the
//      full HeartbeatStatus card. The grid shows variety without alarm.
//   2. Calm copy — no exclamation marks, no all-caps "ALERT", no red
//      unless a human absolutely needs to act now. The header lede is one
//      reassuring sentence keyed to overall state.
//   3. Recovery story — every healed event carries the four-part trail
//      (checked / changed / outcome / follow-up).
//   4. Predictive — degrading components surface in a "What might fail
//      next" panel with a "Pause it" action, before they actually fail.

import * as React from "react";
const { useState, useMemo, useEffect } = React;

// ───────────────────────────────────────────────────────────────────────────
//  Canonical states
// ───────────────────────────────────────────────────────────────────────────

type CanonicalState =
  | "healthy"
  | "recovering"   // bg-recovered — "Quietly recovering"
  | "attention"    // bg-warning   — "Needs your attention"
  | "paused"       // bg-neutral
  | "quarantined"; // bg-quarantined

const STATE_META: Record<CanonicalState, {
  label: string;
  tone: "safe" | "recovered" | "warning" | "neutral" | "quarantined";
  dot: string;
  // top-band lede + supporting sentence (calm)
  headerLede: string;
  headerSub: string;
}> = {
  healthy: {
    label: "Healthy",
    tone: "safe",
    dot: "bg-safe-solid",
    headerLede: "Everything's running.",
    headerSub:  "Last self-heal 23 minutes ago. No action needed.",
  },
  recovering: {
    label: "Quietly recovering",
    tone: "recovered",
    dot: "bg-recovered-solid",
    headerLede: "One component is recovering on its own.",
    headerSub:  "No action needed — we'll let you know if that changes.",
  },
  attention: {
    label: "Needs your attention",
    tone: "warning",
    dot: "bg-warning-solid",
    headerLede: "One component needs you to look at it.",
    headerSub:  "Open it to see what's blocking. Nothing else is affected.",
  },
  paused: {
    label: "Paused",
    tone: "neutral",
    dot: "bg-neutral-solid",
    headerLede: "Workspace is paused.",
    headerSub:  "Heartbeats are still running so you'll know when things change.",
  },
  quarantined: {
    label: "Quarantined",
    tone: "quarantined",
    dot: "bg-quarantined-solid",
    headerLede: "One component has been quarantined.",
    headerSub:  "It's isolated and won't affect anything else. Review when you're ready.",
  },
};

// ───────────────────────────────────────────────────────────────────────────
//  Mock components — 14 total, mixed states (11 healthy / 1 recovering /
//  1 attention / 1 quarantined). The user spec also asks for one Paused;
//  per the brief's 11/1/1/1 split we keep it strictly to 14 with no
//  Paused — overall workspace is "recovering" not paused.
// ───────────────────────────────────────────────────────────────────────────

type Component = {
  id: string;
  name: string;
  category: "core" | "connector" | "team";
  state: CanonicalState;
  lastHeartbeat: string;   // "12s ago", "2m ago"
  uptimeDays: number;      // current streak in days
  activity: string;        // one-line current activity
  // present only when state !== "healthy"
  detail?: string;         // longer explanation for the card
  // optional eta / progress
  etaMinutes?: number;
};

const COMPONENTS: Component[] = [
  // — Core (5) —
  { id: "c-gateway",  name: "Gateway",         category: "core", state: "healthy",
    lastHeartbeat: "8s ago",   uptimeDays: 47, activity: "Routing 142 req/min, p95 41ms" },
  { id: "c-runtime",  name: "Runtime daemon",  category: "core", state: "healthy",
    lastHeartbeat: "10s ago",  uptimeDays: 47, activity: "12 jobs running, queue depth 3" },
  { id: "c-sandbox",  name: "Sandbox",         category: "core", state: "healthy",
    lastHeartbeat: "11s ago",  uptimeDays: 21, activity: "4 sandboxes warm" },
  { id: "c-memory",   name: "Memory store",    category: "core", state: "recovering",
    lastHeartbeat: "9s ago",   uptimeDays: 0,  activity: "Re-embedding 2,140 docs — 8 min remaining",
    detail: "A scheduled embedding-model upgrade is in progress. Reads continue from the warm cache; writes are queued and will drain when re-embed finishes. No action needed.",
    etaMinutes: 8 },
  { id: "c-events",   name: "Event store",     category: "core", state: "healthy",
    lastHeartbeat: "5s ago",   uptimeDays: 62, activity: "Append rate 38/s, retention 90d" },

  // — Connectors (4) —
  { id: "c-telegram", name: "Telegram connector", category: "connector", state: "quarantined",
    lastHeartbeat: "1h ago",  uptimeDays: 0,
    activity: "Isolated — auth token rotated upstream",
    detail: "Telegram desktop client 5.1.4 invalidated the long-lived bot session on May 9 14:02 PT. Outbound messages are queued (4 pending). The connector has been quarantined so it can't retry against a dead session. Re-authorize when you're ready." },
  { id: "c-email",    name: "Email connector",     category: "connector", state: "healthy",
    lastHeartbeat: "14s ago", uptimeDays: 47, activity: "118 messages last hour" },
  { id: "c-webhook",  name: "Webhook connector",   category: "connector", state: "healthy",
    lastHeartbeat: "6s ago",  uptimeDays: 19, activity: "11 receivers active, 0 retries" },
  { id: "c-cal",      name: "Calendar connector",  category: "connector", state: "healthy",
    lastHeartbeat: "22s ago", uptimeDays: 47, activity: "Synced 3 calendars" },

  // — Teams (5) —
  { id: "c-inbox",    name: "Inbox team",       category: "team", state: "healthy",
    lastHeartbeat: "11s ago", uptimeDays: 12, activity: "Triaged 26 messages this hour" },
  { id: "c-cal-team", name: "Calendar team",    category: "team", state: "healthy",
    lastHeartbeat: "16s ago", uptimeDays: 12, activity: "3 events drafted, 2 awaiting your review" },
  { id: "c-research", name: "Research team",    category: "team", state: "healthy",
    lastHeartbeat: "12s ago", uptimeDays: 8,  activity: "Watching 47 sources" },
  { id: "c-ops",      name: "Operations team",  category: "team", state: "attention",
    lastHeartbeat: "20s ago", uptimeDays: 0,
    activity: "Awaiting your decision on PO-26-118",
    detail: "Operations needs your sign-off on a $640 standing-order PO that drifted above the $50 auto-approve threshold. The team paused itself to wait for you — nothing else is blocked." },
  { id: "c-draft",    name: "Drafting team",    category: "team", state: "healthy",
    lastHeartbeat: "9s ago",  uptimeDays: 12, activity: "5 drafts queued for review" },
];

// ───────────────────────────────────────────────────────────────────────────
//  Mock self-heal events (8) — every entry carries the 4-part recovery
//  story. Some have rule-review follow-ups.
// ───────────────────────────────────────────────────────────────────────────

type HealEvent = {
  id: string;
  when: string;            // "23m ago", "2h ago"
  whenIso: string;         // for tabular sort, displayed as font-mono
  component: string;       // which component
  componentId: string;
  what: string;            // one-line summary
  story: {
    checked: string;       // what we checked
    changed: string;       // what we changed
    outcome: string;       // recovery succeeded?
    followup: string | null; // "rule will be reviewed Friday" / null
  };
  durationSec: number;
};

const HEAL_EVENTS: HealEvent[] = [
  {
    id: "h01", when: "23m ago", whenIso: "11:32 PT", component: "Webhook connector", componentId: "c-webhook",
    what: "Recovered from a slow downstream receiver",
    story: {
      checked:  "p95 latency on receiver `slack/ops-bot` exceeded 4s for 3 consecutive checks.",
      changed:  "Switched the receiver to the back-off queue and reduced concurrency from 4 → 1.",
      outcome:  "p95 returned to 380ms within 90 seconds; queue drained without retries.",
      followup: null,
    },
    durationSec: 96,
  },
  {
    id: "h02", when: "2h ago", whenIso: "09:55 PT", component: "Email connector", componentId: "c-email",
    what: "Reconnected after a transient IMAP IDLE drop",
    story: {
      checked:  "IMAP IDLE socket closed without notice; no new messages for 4 minutes.",
      changed:  "Restarted the IDLE session against the secondary endpoint.",
      outcome:  "Stream resumed; backfill caught 3 messages that arrived during the gap.",
      followup: "Same drop pattern seen 3× this month — rule will be reviewed Friday.",
    },
    durationSec: 240,
  },
  {
    id: "h03", when: "5h ago", whenIso: "06:48 PT", component: "Memory store", componentId: "c-memory",
    what: "Dropped a stale embedding-cache shard",
    story: {
      checked:  "Cache hit-rate on shard 7 had fallen to 4% over 24h.",
      changed:  "Evicted shard 7 and rebuilt it from cold storage.",
      outcome:  "Hit-rate climbed to 81% within 20 minutes; no read failures during rebuild.",
      followup: null,
    },
    durationSec: 1200,
  },
  {
    id: "h04", when: "9h ago", whenIso: "02:32 PT", component: "Event store", componentId: "c-events",
    what: "Rode out an S3 throttling window",
    story: {
      checked:  "Append latency p95 jumped to 1.8s between 02:14 and 02:32 PT.",
      changed:  "Held writes in the local buffer and replayed when throttling cleared.",
      outcome:  "All 412 appends delivered, in-order, with no operator action needed.",
      followup: "Cloud-provider status confirmed regional throttling.",
    },
    durationSec: 1080,
  },
  {
    id: "h05", when: "13h ago", whenIso: "22:14 PT (May 11)", component: "Runtime daemon", componentId: "c-runtime",
    what: "Restarted a worker stuck on a long-running job",
    story: {
      checked:  "Worker `runtime-w3` had not heartbeated for 7 minutes.",
      changed:  "Killed worker, redrained its queue to siblings, restarted the worker.",
      outcome:  "Queue cleared in 4 minutes; no jobs lost.",
      followup: null,
    },
    durationSec: 240,
  },
  {
    id: "h06", when: "yesterday", whenIso: "May 11 16:01 PT", component: "Gateway", componentId: "c-gateway",
    what: "Failed over to the backup auth provider",
    story: {
      checked:  "Primary auth provider returned 502 on 8/10 probes in 60s.",
      changed:  "Switched to backup provider and held the route there for 15 minutes.",
      outcome:  "All sessions stayed valid; primary recovered after 11 minutes and we re-pinned.",
      followup: "Rule will be reviewed Friday — failover triggered twice this week.",
    },
    durationSec: 660,
  },
  {
    id: "h07", when: "yesterday", whenIso: "May 11 11:09 PT", component: "Drafting team", componentId: "c-draft",
    what: "Throttled itself when model latency spiked",
    story: {
      checked:  "Draft generation p95 rose from 1.4s to 9.2s for 4 minutes.",
      changed:  "Dropped concurrency 8 → 2 and switched to the cheaper drafting model.",
      outcome:  "Latency returned to 1.5s; quality regression on 0 of 7 drafts checked.",
      followup: null,
    },
    durationSec: 360,
  },
  {
    id: "h08", when: "2d ago", whenIso: "May 10 04:17 PT", component: "Sandbox", componentId: "c-sandbox",
    what: "Rotated a sandbox with a leaking file descriptor",
    story: {
      checked:  "Sandbox `sbx-118` held 1,920 open fds, growing 3/min.",
      changed:  "Drained sessions, recycled the sandbox, restarted with a clean baseline.",
      outcome:  "fd count back to 184 (nominal). No user-facing impact.",
      followup: null,
    },
    durationSec: 540,
  },
];

// ───────────────────────────────────────────────────────────────────────────
//  Predictive — "What might fail next"
// ───────────────────────────────────────────────────────────────────────────

type PredictiveWarning = {
  id: string;
  component: string;
  componentId: string;
  signal: string;            // headline observation
  why: string;               // explanation
  errorBudgetUsedPct: number; // 0..100
  windowDays: number;        // window over which the budget is measured
  // mini-graph data: 30 points of 0..1 (recent → right). Used to draw a
  // small inline sparkline.
  trend: number[];
  suggestedAction: string;   // "Pause it" / "Re-run probe" / etc
};

const PREDICTIVE: PredictiveWarning[] = [
  {
    id: "p01",
    component: "Email connector",
    componentId: "c-email",
    signal: "IMAP IDLE drops recurring on a weekly cadence",
    why: "Three IDLE drops this month, each followed by a successful self-heal. Pattern matches a known network-path quirk; reliability is trending below the 99.9% objective.",
    errorBudgetUsedPct: 72,
    windowDays: 30,
    trend: [.94,.95,.94,.93,.93,.92,.93,.93,.91,.91,.92,.9,.9,.9,.89,.89,.88,.88,.87,.87,.87,.86,.85,.85,.84,.83,.83,.82,.81,.8],
    suggestedAction: "Pause it",
  },
  {
    id: "p02",
    component: "Telegram connector",
    componentId: "c-telegram",
    signal: "Already quarantined — recommend keeping it paused until re-auth",
    why: "Long-lived bot session was invalidated by an upstream client change. The connector quarantined itself; predictive monitor recommends staying paused to avoid retry churn.",
    errorBudgetUsedPct: 100,
    windowDays: 7,
    trend: [.9,.92,.91,.92,.93,.92,.93,.94,.94,.93,.9,.86,.78,.7,.6,.5,.42,.32,.22,.15,.1,.05,.02,0,0,0,0,0,0,0],
    suggestedAction: "Keep paused",
  },
  {
    id: "p03",
    component: "Operations team",
    componentId: "c-ops",
    signal: "Decision queue is growing faster than it's draining",
    why: "Six new approvals landed in the last 24h; you reviewed two. If the trend holds for another week, Operations will start deferring drafts.",
    errorBudgetUsedPct: 41,
    windowDays: 14,
    trend: [.99,.99,.98,.98,.98,.97,.97,.96,.96,.95,.95,.94,.93,.92,.91,.9,.88,.86,.84,.82,.8,.78,.76,.74,.72,.7,.68,.66,.64,.62],
    suggestedAction: "Show graph",
  },
];

// ───────────────────────────────────────────────────────────────────────────
//  Helpers + icons
// ───────────────────────────────────────────────────────────────────────────

const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(" ");

function fmtDuration(sec: number): string {
  if (sec < 60) return `${sec}s`;
  const m = Math.round(sec / 60);
  if (m < 60) return `${m}m`;
  const h = Math.round(m / 60);
  return `${h}h`;
}

const Svg = ({ d, size = 16, className = "" }:
  { d: React.ReactNode; size?: number; className?: string }) => (
  <svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor"
       strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden="true">
    {d}
  </svg>
);
type IconProps = { size?: number; className?: string };
const ICON = {
  Heart:   (p: IconProps) => <Svg {...p} d={<path d="M12 20s-7-4.5-7-10a4 4 0 0 1 7-2.5A4 4 0 0 1 19 10c0 5.5-7 10-7 10Z" />} />,
  Pulse:   (p: IconProps) => <Svg {...p} d={<path d="M3 12h4l2-5 4 10 2-5h6" />} />,
  Check:   (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  Eye:     (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>} />,
  Pause:   (p: IconProps) => <Svg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  Wave:    (p: IconProps) => <Svg {...p} d={<path d="M3 12c2 0 2-4 4-4s2 8 4 8 2-8 4-8 2 4 4 4" />} />,
  Sparkles:(p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  Clock:   (p: IconProps) => <Svg {...p} d={<><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>} />,
  ChevronDown: (p: IconProps) => <Svg {...p} d={<path d="m6 9 6 6 6-6" />} />,
  ChevronUp:   (p: IconProps) => <Svg {...p} d={<path d="m6 15 6-6 6 6" />} />,
  ChevronRight:(p: IconProps) => <Svg {...p} d={<path d="m9 6 6 6-6 6" />} />,
  ArrowRight:  (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  Activity:    (p: IconProps) => <Svg {...p} d={<path d="M3 12h4l3-7 4 14 3-7h4" />} />,
  Shield:      (p: IconProps) => <Svg {...p} d={<path d="M12 3 4 6v6c0 4.5 3.4 8.4 8 9 4.6-.6 8-4.5 8-9V6l-8-3Z" />} />,
  Box:         (p: IconProps) => <Svg {...p} d={<><rect x="4" y="4" width="16" height="16" rx="2" /><path d="M4 9h16M9 4v16" /></>} />,
  Plug:        (p: IconProps) => <Svg {...p} d={<><path d="M9 2v6M15 2v6" /><rect x="6" y="8" width="12" height="6" rx="2" /><path d="M12 14v4M9 22h6" /></>} />,
  Users:       (p: IconProps) => <Svg {...p} d={<><circle cx="9" cy="9" r="3" /><path d="M3 20a6 6 0 0 1 12 0" /><circle cx="17" cy="8" r="2.5" /><path d="M16 14a5 5 0 0 1 5 5" /></>} />,
};

const CATEGORY_META: Record<Component["category"], { label: string; IconCmp: React.ComponentType<IconProps> }> = {
  core:      { label: "Core",       IconCmp: ICON.Box   },
  connector: { label: "Connectors", IconCmp: ICON.Plug  },
  team:      { label: "Teams",      IconCmp: ICON.Users },
};

// ───────────────────────────────────────────────────────────────────────────
//  Header band — overall workspace state
// ───────────────────────────────────────────────────────────────────────────

function HealthHeader({ overall, healthyCount, total }:
  { overall: CanonicalState; healthyCount: number; total: number; }) {
  const m = STATE_META[overall];
  return (
    <section className={cx(
      "card overflow-hidden border-2",
      // Use the semantic border for the active tone so the band reads as
      // that state without being loud.
      `border-${m.tone}`,
    )}>
      <div className={cx("px-5 py-5 sm:px-7 sm:py-6 flex items-center gap-5 flex-wrap",
                         `bg-${m.tone}`)}>
        {/* Big calm dot — pulses gently when recovering/attention. */}
        <div className={cx(
          "shrink-0 h-14 w-14 rounded-full bg-white/70 backdrop-blur inline-flex items-center justify-center",
          `text-${m.tone}`,
          overall === "recovering" || overall === "attention" ? "ig-pulse" : "",
        )}>
          {overall === "healthy"     && <ICON.Heart  size={26} />}
          {overall === "recovering"  && <ICON.Wave   size={26} />}
          {overall === "attention"   && <ICON.Eye    size={26} />}
          {overall === "paused"      && <ICON.Pause  size={26} />}
          {overall === "quarantined" && <ICON.Shield size={26} />}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className={cx(
              "inline-flex items-center gap-1.5 rounded-full border text-[11px] font-medium px-2 py-0.5",
              `bg-${m.tone}`, `text-${m.tone}`, `border-${m.tone}`,
              "bg-white/70 backdrop-blur",
            )}>
              <span className={cx("h-1.5 w-1.5 rounded-full", m.dot)} />
              {m.label}
            </span>
            <span className={cx("text-[11px] font-mono tabular-nums", `text-${m.tone}`)}>
              {healthyCount}/{total} components healthy
            </span>
          </div>
          <h1 className={cx(
            "mt-1 text-[24px] sm:text-[26px] font-semibold tracking-tight leading-tight",
            `text-${m.tone}`,
          )}>
            {m.headerLede}
          </h1>
          <p className={cx("mt-1 text-[13.5px] leading-relaxed", `text-${m.tone}`)}>
            {m.headerSub}
          </p>
        </div>

        {/* Right-side: heartbeat pulse */}
        <div className="hidden sm:flex items-center gap-2 shrink-0 text-[11px] font-mono"
             aria-hidden="true">
          <span className={cx("h-2 w-2 rounded-full ig-pulse", m.dot)} />
          <span className={cx(`text-${m.tone}`)}>heartbeat</span>
        </div>
      </div>
    </section>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Heartbeat — healthy chip (compact) + non-healthy card (expanded)
// ───────────────────────────────────────────────────────────────────────────

function HealthyChip({ c }: { c: Component }) {
  const CatIcn = CATEGORY_META[c.category].IconCmp;
  return (
    <div
      className="card flex items-center gap-2 px-3 py-2 hover:shadow-elevated transition-shadow"
      title={`${c.name} · ${c.activity}`}
    >
      <span className="inline-flex h-5 w-5 items-center justify-center rounded-md bg-safe text-safe shrink-0">
        <CatIcn size={11} />
      </span>
      <span className="text-[12.5px] font-medium text-neutral-800 truncate flex-1">{c.name}</span>
      <span className="inline-flex items-center gap-1 text-[10.5px] text-safe shrink-0">
        <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
        <span className="font-mono tabular-nums">{c.lastHeartbeat}</span>
      </span>
    </div>
  );
}

function StatusCard({ c }: { c: Component }) {
  const m = STATE_META[c.state];
  const CatIcn = CATEGORY_META[c.category].IconCmp;
  return (
    <article className={cx(
      "card overflow-hidden border-2",
      `border-${m.tone}`,
    )}>
      <div className={cx("px-4 py-3 flex items-start justify-between gap-3", `bg-${m.tone}`)}>
        <div className="flex items-start gap-3 min-w-0">
          <span className={cx(
            "inline-flex h-8 w-8 items-center justify-center rounded-md shrink-0 bg-white/70",
            `text-${m.tone}`,
          )}>
            <CatIcn size={15} />
          </span>
          <div className="min-w-0">
            <div className="flex items-center gap-1.5">
              <h3 className={cx("text-[14.5px] font-semibold tracking-tight", `text-${m.tone}`)}>{c.name}</h3>
              <span className="text-[10.5px] font-mono text-neutral-500">· {CATEGORY_META[c.category].label}</span>
            </div>
            <div className={cx("mt-0.5 inline-flex items-center gap-1.5 text-[11px]", `text-${m.tone}`)}>
              <span className={cx("h-1.5 w-1.5 rounded-full ig-pulse", m.dot)} />
              {m.label}
              <span className="text-neutral-400">·</span>
              <span className="inline-flex items-center gap-1 text-neutral-500">
                <ICON.Clock size={10} />
                <span className="font-mono tabular-nums">{c.lastHeartbeat}</span>
              </span>
            </div>
          </div>
        </div>
      </div>

      <div className="px-4 py-3 flex flex-col gap-2">
        <p className="text-[13px] text-neutral-800 leading-relaxed">{c.activity}</p>
        {c.detail && (
          <p className="text-[12.5px] text-neutral-600 leading-relaxed">{c.detail}</p>
        )}

        {/* Progress bar for recovering */}
        {c.state === "recovering" && typeof c.etaMinutes === "number" && (
          <div className="mt-1">
            <div className="flex items-center justify-between text-[10.5px] text-recovered mb-1">
              <span className="font-medium uppercase tracking-wide">Recovery in progress</span>
              <span className="font-mono tabular-nums">~{c.etaMinutes}m remaining</span>
            </div>
            <div className="h-1 w-full rounded-full bg-neutral-100 overflow-hidden">
              {/* Indeterminate-style fill: 64% to suggest motion. */}
              <div className="h-full w-[64%] rounded-full bg-recovered-solid" />
            </div>
          </div>
        )}
      </div>

      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-white">
        <span className="text-[10.5px] text-neutral-500">
          Uptime streak: <span className="text-neutral-800 font-mono tabular-nums">
            {c.uptimeDays === 0 ? "—" : `${c.uptimeDays}d`}
          </span>
        </span>
        <div className="flex items-center gap-1">
          {c.state === "attention" && (
            <button type="button"
                    className="inline-flex items-center gap-1.5 rounded-md bg-warning-solid text-white hover:bg-warning-solid-hover px-2.5 py-1 text-[12px] font-medium">
              <ICON.ArrowRight size={11} /> Open it
            </button>
          )}
          {c.state === "recovering" && (
            <span className="text-[11px] text-recovered inline-flex items-center gap-1">
              <ICON.Sparkles size={11} /> No action needed
            </span>
          )}
          {c.state === "quarantined" && (
            <button type="button"
                    className="inline-flex items-center gap-1.5 rounded-md border border-quarantined bg-white hover:bg-quarantined-hover text-quarantined px-2.5 py-1 text-[12px] font-medium">
              Review when ready
            </button>
          )}
        </div>
      </footer>
    </article>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Heartbeat grid
// ───────────────────────────────────────────────────────────────────────────

function HeartbeatGrid({ components }: { components: Component[] }) {
  // Suppress-on-OK: non-healthy first, then a compact green chip grid.
  const nonHealthy = components.filter((c) => c.state !== "healthy");
  const healthy    = components.filter((c) => c.state === "healthy");

  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h2 className="section-title">Heartbeat</h2>
          <p className="text-[12.5px] text-neutral-500 mt-1">
            Components that need a second look are above. Healthy components are compact below.
          </p>
        </div>
        <div className="inline-flex items-center gap-3 text-[11px] text-neutral-500 flex-wrap">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-safe-solid" /> Healthy {healthy.length}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-recovered-solid" /> Recovering {components.filter((c) => c.state === "recovering").length}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-warning-solid" /> Attention {components.filter((c) => c.state === "attention").length}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-quarantined-solid" /> Quarantined {components.filter((c) => c.state === "quarantined").length}
          </span>
        </div>
      </div>

      {nonHealthy.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {nonHealthy.map((c) => <StatusCard key={c.id} c={c} />)}
        </div>
      )}

      <div>
        <div className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 mb-2">
          Healthy ({healthy.length})
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-2">
          {healthy.map((c) => <HealthyChip key={c.id} c={c} />)}
        </div>
      </div>
    </section>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Self-healing log (Timeline component, state="healed" entries)
// ───────────────────────────────────────────────────────────────────────────

function HealEventRow({ ev, open, onToggle }:
  { ev: HealEvent; open: boolean; onToggle: () => void }) {
  return (
    <li className="relative pl-7 pr-1 py-3">
      {/* Timeline rail dot */}
      <span className="absolute left-2.5 top-4 h-2 w-2 rounded-full bg-recovered-solid" />
      <span className="absolute left-[14px] top-6 bottom-0 w-px bg-neutral-100" />

      <div className="flex items-baseline gap-2 flex-wrap">
        <span className="inline-flex items-center gap-1 rounded-full border border-recovered bg-recovered text-recovered text-[10.5px] font-medium px-1.5 py-0.5">
          <ICON.Sparkles size={10} /> Self-healed
        </span>
        <span className="text-[13px] font-semibold text-neutral-900">{ev.what}</span>
        <span className="text-[10.5px] font-mono text-neutral-400">· {ev.whenIso}</span>
        <span className="text-[10.5px] text-neutral-400 ml-auto">
          {ev.component} · took <span className="font-mono tabular-nums">{fmtDuration(ev.durationSec)}</span>
        </span>
      </div>

      <button
        type="button"
        onClick={onToggle}
        className="mt-1 inline-flex items-center gap-0.5 text-[11.5px] font-medium text-accent hover:text-accent-solid"
      >
        {open ? "Hide recovery story" : "Show recovery story"}
        {open ? <ICON.ChevronUp size={11} /> : <ICON.ChevronDown size={11} />}
      </button>

      {open && (
        <div className="mt-2 rounded-lg border border-neutral-100 bg-neutral-50/60 overflow-hidden">
          <div className="grid grid-cols-1 sm:grid-cols-2">
            {[
              { label: "What we checked", text: ev.story.checked  },
              { label: "What changed",    text: ev.story.changed  },
              { label: "Outcome",         text: ev.story.outcome  },
              { label: "Follow-up",       text: ev.story.followup ?? "None — this was an isolated event." },
            ].map((row, i) => (
              <div key={row.label}
                   className={cx(
                     "px-3 py-2.5 border-neutral-100",
                     i % 2 === 0 ? "sm:border-r" : "",
                     i < 2 ? "sm:border-b" : "",
                     i !== 3 ? "border-b sm:border-b" : "",
                   )}>
                <div className="text-[10px] font-medium uppercase tracking-wide text-neutral-500">{row.label}</div>
                <div className="mt-0.5 text-[12.5px] text-neutral-700 leading-relaxed">{row.text}</div>
              </div>
            ))}
          </div>
        </div>
      )}
    </li>
  );
}

function HealLog({ events }: { events: HealEvent[] }) {
  const [openId, setOpenId] = useState<string | null>(events[0]?.id ?? null);

  return (
    <section className="card">
      <header className="px-5 py-4 border-b border-neutral-100 flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h2 className="section-title">Self-healing log</h2>
          <p className="text-[12.5px] text-neutral-500 mt-1">
            The last <span className="text-neutral-800 font-mono tabular-nums">{events.length}</span> things that fixed themselves. Each entry includes the recovery story.
          </p>
        </div>
        <a href="#full-log" className="text-[12px] text-accent hover:text-accent-solid font-medium inline-flex items-center gap-0.5">
          Full log <ICON.ChevronRight size={11} />
        </a>
      </header>

      {events.length === 0 ? (
        <div className="p-8 text-center">
          <div className="mx-auto h-10 w-10 rounded-full bg-safe inline-flex items-center justify-center text-safe">
            <ICON.Heart size={18} />
          </div>
          <h3 className="mt-3 text-[14px] font-semibold text-neutral-900">Nothing's needed self-heal in the last 24 hours.</h3>
          <p className="text-[12.5px] text-neutral-500 mt-1">Heartbeat green for 17 days.</p>
        </div>
      ) : (
        <ol className="px-2 sm:px-3 pb-2">
          {events.map((ev) => (
            <HealEventRow
              key={ev.id}
              ev={ev}
              open={openId === ev.id}
              onToggle={() => setOpenId(openId === ev.id ? null : ev.id)}
            />
          ))}
        </ol>
      )}
    </section>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Sparkline
// ───────────────────────────────────────────────────────────────────────────

function Sparkline({ points, tone }:
  { points: number[]; tone: "warning" | "quarantined" | "neutral" | "safe" }) {
  const W = 120, H = 28;
  const max = 1, min = 0;
  const step = W / Math.max(1, points.length - 1);
  const d = points.map((p, i) => {
    const x = i * step;
    const y = H - ((p - min) / (max - min)) * (H - 4) - 2;
    return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
  }).join(" ");
  return (
    <svg width={W} height={H} viewBox={`0 0 ${W} ${H}`} className={`text-${tone}`}>
      <path d={d} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d={`${d} L${W} ${H} L0 ${H} Z`} fill="currentColor" opacity="0.08" />
    </svg>
  );
}

function PredictiveCard({ w, showGraph, onToggleGraph, onAct }:
  { w: PredictiveWarning; showGraph: boolean; onToggleGraph: () => void; onAct: () => void }) {
  const tone =
    w.errorBudgetUsedPct >= 80 ? "quarantined" :
    w.errorBudgetUsedPct >= 50 ? "warning"     : "neutral";
  return (
    <article className="card flex flex-col">
      <div className="px-4 py-3 flex flex-col gap-2">
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <span className={cx(
            "inline-flex items-center gap-1.5 rounded-full border text-[10.5px] font-medium px-1.5 py-0.5",
            `bg-${tone}`, `text-${tone}`, `border-${tone}`,
          )}>
            <ICON.Activity size={10} /> {tone === "quarantined" ? "Budget exhausted" : tone === "warning" ? "Budget drifting" : "Watching"}
          </span>
          <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">
            {w.errorBudgetUsedPct}% of {w.windowDays}d budget used
          </span>
        </div>
        <h3 className="text-[14px] font-semibold tracking-tight text-neutral-900">
          {w.component}
        </h3>
        <p className="text-[12.5px] text-neutral-700 leading-snug">{w.signal}</p>

        {/* Inline mini error-budget bar */}
        <div className="mt-1">
          <div className="h-1 w-full rounded-full bg-neutral-100 overflow-hidden">
            <div
              className={cx("h-full rounded-full", `bg-${tone}-solid`)}
              style={{ width: `${w.errorBudgetUsedPct}%` }}
            />
          </div>
        </div>

        <p className="text-[12px] text-neutral-500 leading-relaxed">{w.why}</p>

        {/* Sparkline (graph) — collapsed by default */}
        {showGraph && (
          <div className="mt-2 rounded-lg bg-neutral-50/60 border border-neutral-100 px-3 py-2">
            <div className="flex items-center justify-between mb-1">
              <span className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500">
                Reliability, last {w.windowDays}d
              </span>
              <span className="text-[10.5px] font-mono text-neutral-400 tabular-nums">
                {Math.round((w.trend.at(-1) ?? 0) * 100)}%
              </span>
            </div>
            <Sparkline points={w.trend} tone={tone === "neutral" ? "safe" : tone} />
          </div>
        )}
      </div>

      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/60">
        <button type="button" onClick={onToggleGraph}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
          {showGraph ? <ICON.ChevronUp size={12} /> : <ICON.ChevronDown size={12} />}
          {showGraph ? "Hide graph" : "Show graph"}
        </button>
        <button type="button" onClick={onAct}
                className={cx(
                  "inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors",
                  w.suggestedAction === "Pause it"
                    ? "bg-warning-solid text-white hover:bg-warning-solid-hover"
                    : w.suggestedAction === "Keep paused"
                      ? "border border-quarantined bg-white text-quarantined hover:bg-quarantined-hover"
                      : "border border-neutral-200 bg-white text-neutral-700 hover:bg-neutral-50",
                )}>
          {w.suggestedAction === "Pause it" && <ICON.Pause size={11} />}
          {w.suggestedAction}
        </button>
      </footer>
    </article>
  );
}

function PredictivePanel({ warnings, openGraphIds, onToggleGraph, onAct }:
  {
    warnings: PredictiveWarning[];
    openGraphIds: Set<string>;
    onToggleGraph: (id: string) => void;
    onAct: (id: string) => void;
  }) {
  return (
    <section>
      <header className="flex items-end justify-between gap-3 flex-wrap mb-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Predictive</div>
          <h2 className="section-title mt-0.5">What might fail next</h2>
          <p className="text-[12.5px] text-neutral-500 mt-1">
            Surfaced before anything actually breaks. Each warning is tied to an error budget.
          </p>
        </div>
      </header>
      {warnings.length === 0 ? (
        <div className="card card-padded text-center">
          <ICON.Pulse size={20} className="mx-auto text-safe" />
          <div className="mt-2 text-[13px] font-medium text-neutral-800">Nothing's degrading right now.</div>
          <div className="text-[12px] text-neutral-500">Error budgets across all components are above 50%.</div>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {warnings.map((w) => (
            <PredictiveCard
              key={w.id}
              w={w}
              showGraph={openGraphIds.has(w.id)}
              onToggleGraph={() => onToggleGraph(w.id)}
              onAct={() => onAct(w.id)}
            />
          ))}
        </div>
      )}
    </section>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Health — the route
// ───────────────────────────────────────────────────────────────────────────

export function Health(): JSX.Element {
  const [components] = useState<Component[]>(COMPONENTS);
  const [healEvents] = useState<HealEvent[]>(HEAL_EVENTS);
  const [predictive] = useState<PredictiveWarning[]>(PREDICTIVE);

  const [openGraphIds, setOpenGraphIds] = useState<Set<string>>(new Set());
  const [toast, setToast] = useState<string | null>(null);

  useEffect(() => {
    if (!toast) return;
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  // Roll overall health up from the components in the most-alarming-wins
  // direction, while keeping the surface calm. Quarantined > attention >
  // recovering > paused > healthy.
  const overall: CanonicalState = useMemo(() => {
    if (components.some((c) => c.state === "quarantined")) return "quarantined";
    if (components.some((c) => c.state === "attention"))   return "attention";
    if (components.some((c) => c.state === "recovering"))  return "recovering";
    if (components.every((c) => c.state === "paused"))     return "paused";
    return "healthy";
  }, [components]);

  const healthyCount = useMemo(
    () => components.filter((c) => c.state === "healthy").length,
    [components],
  );

  const handleToggleGraph = (id: string) => {
    setOpenGraphIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  };

  const handlePredictiveAct = (id: string) => {
    const w = predictive.find((x) => x.id === id);
    if (!w) return;
    setToast(`${w.suggestedAction} · ${w.component}`);
  };

  return (
    <main className="min-h-screen bg-app">
      <div className="page-container max-w-[78rem] flex flex-col gap-6">
        <HealthHeader overall={overall} healthyCount={healthyCount} total={components.length} />

        <HeartbeatGrid components={components} />

        <PredictivePanel
          warnings={predictive}
          openGraphIds={openGraphIds}
          onToggleGraph={handleToggleGraph}
          onAct={handlePredictiveAct}
        />

        <HealLog events={healEvents} />

        <footer className="mt-2 mb-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-neutral-500">
          <div className="inline-flex items-center gap-1.5">
            <ICON.Heart size={12} className="text-safe" />
            We suppress noise when things are okay. You'll always hear when something needs you.
          </div>
          <a href="#runbook" className="text-accent hover:text-accent-solid font-medium">
            Operator runbook →
          </a>
        </footer>
      </div>

      {toast && (
        <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50">
          <div className="rounded-lg bg-neutral-900 text-white shadow-lg px-3.5 py-2 text-[12.5px] font-medium inline-flex items-center gap-2">
            <ICON.Check size={13} className="text-safe-solid" />
            {toast}
          </div>
        </div>
      )}
    </main>
  );
}

;(window as unknown as { Health: typeof Health }).Health = Health;
