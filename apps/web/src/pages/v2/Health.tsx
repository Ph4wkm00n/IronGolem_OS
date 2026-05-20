// route: /health
// purpose: system heartbeats, self-healing log, predictive warnings. Replaces
// the stub when VITE_ENABLE_V2_UI=true. Ported from Claude Design's Health.tsx
// — source at apps/web/src/_design-inbox/health/.
//
// Integration notes:
// - Shell chrome from `pages/v2/_shared/WorkspaceTopbar`. The topbar's
//   heartbeat pill is suppressed here (showHeartbeatPill=false) so it doesn't
//   redundantly tell you everything's fine on the literal Health page.
// - Mock data inline; swap for `useHealthQuery()` once lib/api.ts ships.
// - Dynamic `bg-${tone}` patterns replaced with the static TONE classmap.
// - Drops the preview shim (`window.Health = ...`).

import React, { useEffect, useMemo, useState } from "react";

import { RouteError } from "../../components/RouteError";
import { useRouteData } from "../../lib/route-data";
import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

import {
  api,
  type HealthComponent,
  type HealEvent,
  type PredictiveWarning,
  type HealthState as CanonicalState,
  type ComponentCategory,
} from "../../lib/api";

type ToneName = "safe" | "warning" | "blocked" | "recovered" | "quarantined" | "accent" | "neutral";

// ─────────────────────────────────────────────────────────────────────────────
// Tone classmap
// ─────────────────────────────────────────────────────────────────────────────

interface ToneClasses {
  readonly bg: string;
  readonly text: string;
  readonly border: string;
  readonly bgSolid: string;
  readonly textSolid: string;
  readonly bgSolidHover: string;
}

const TONE: Readonly<Record<ToneName, ToneClasses>> = {
  safe: { bg: "bg-safe", text: "text-safe", border: "border-safe", bgSolid: "bg-safe-solid", textSolid: "text-safe-solid", bgSolidHover: "hover:bg-safe-solid-hover" },
  warning: { bg: "bg-warning", text: "text-warning", border: "border-warning", bgSolid: "bg-warning-solid", textSolid: "text-warning-solid", bgSolidHover: "hover:bg-warning-solid-hover" },
  blocked: { bg: "bg-blocked", text: "text-blocked", border: "border-blocked", bgSolid: "bg-blocked-solid", textSolid: "text-blocked-solid", bgSolidHover: "hover:bg-blocked-solid-hover" },
  recovered: { bg: "bg-recovered", text: "text-recovered", border: "border-recovered", bgSolid: "bg-recovered-solid", textSolid: "text-recovered-solid", bgSolidHover: "hover:bg-recovered-solid-hover" },
  quarantined: { bg: "bg-quarantined", text: "text-quarantined", border: "border-quarantined", bgSolid: "bg-quarantined-solid", textSolid: "text-quarantined-solid", bgSolidHover: "hover:bg-quarantined-solid-hover" },
  accent: { bg: "bg-accent", text: "text-accent", border: "border-accent", bgSolid: "bg-accent-solid", textSolid: "text-accent-solid", bgSolidHover: "hover:bg-accent-solid-hover" },
  neutral: { bg: "bg-neutral", text: "text-neutral-700", border: "border-neutral-200", bgSolid: "bg-neutral-solid", textSolid: "text-neutral-solid", bgSolidHover: "hover:bg-neutral-solid-hover" },
};

const cx = (...c: ReadonlyArray<string | false | null | undefined>): string =>
  c.filter(Boolean).join(" ");

function fmtDuration(sec: number): string {
  if (sec < 60) return `${sec}s`;
  const m = Math.round(sec / 60);
  if (m < 60) return `${m}m`;
  const h = Math.round(m / 60);
  return `${h}h`;
}

interface StateMeta {
  readonly label: string;
  readonly tone: ToneName;
  readonly dot: string;
  readonly headerLede: string;
  readonly headerSub: string;
}

const STATE_META: Readonly<Record<CanonicalState, StateMeta>> = {
  healthy: {
    label: "Healthy", tone: "safe", dot: "bg-safe-solid",
    headerLede: "Everything's running.",
    headerSub: "Last self-heal 23 minutes ago. No action needed.",
  },
  recovering: {
    label: "Quietly recovering", tone: "recovered", dot: "bg-recovered-solid",
    headerLede: "One component is recovering on its own.",
    headerSub: "No action needed — we'll let you know if that changes.",
  },
  attention: {
    label: "Needs your attention", tone: "warning", dot: "bg-warning-solid",
    headerLede: "One component needs you to look at it.",
    headerSub: "Open it to see what's blocking. Nothing else is affected.",
  },
  paused: {
    label: "Paused", tone: "neutral", dot: "bg-neutral-solid",
    headerLede: "Workspace is paused.",
    headerSub: "Heartbeats are still running so you'll know when things change.",
  },
  quarantined: {
    label: "Quarantined", tone: "quarantined", dot: "bg-quarantined-solid",
    headerLede: "One component has been quarantined.",
    headerSub: "It's isolated and won't affect anything else. Review when you're ready.",
  },
};

// ─────────────────────────────────────────────────────────────────────────────
// Inline icons
// ─────────────────────────────────────────────────────────────────────────────

interface IconProps {
  readonly size?: number;
  readonly className?: string;
}

const Svg = ({
  d, viewBox = "0 0 24 24", size = 16, className = "",
}: {
  readonly d: React.ReactNode;
  readonly viewBox?: string;
  readonly size?: number;
  readonly className?: string;
}) => (
  <svg
    viewBox={viewBox}
    width={size}
    height={size}
    fill="none"
    stroke="currentColor"
    strokeWidth={1.5}
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    aria-hidden
  >
    {d}
  </svg>
);

const ICON = {
  Heart: (p: IconProps) => <Svg {...p} d={<path d="M12 20s-7-4.5-7-10a4 4 0 0 1 7-2.5A4 4 0 0 1 19 10c0 5.5-7 10-7 10Z" />} />,
  Pulse: (p: IconProps) => <Svg {...p} d={<path d="M3 12h4l2-5 4 10 2-5h6" />} />,
  Check: (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  Eye: (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx={12} cy={12} r={3} /></>} />,
  Pause: (p: IconProps) => <Svg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  Wave: (p: IconProps) => <Svg {...p} d={<path d="M3 12c2 0 2-4 4-4s2 8 4 8 2-8 4-8 2 4 4 4" />} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  Clock: (p: IconProps) => <Svg {...p} d={<><circle cx={12} cy={12} r={9} /><path d="M12 7v5l3 2" /></>} />,
  ChevronDown: (p: IconProps) => <Svg {...p} d={<path d="m6 9 6 6 6-6" />} />,
  ChevronUp: (p: IconProps) => <Svg {...p} d={<path d="m6 15 6-6 6 6" />} />,
  ChevronRight: (p: IconProps) => <Svg {...p} d={<path d="m9 6 6 6-6 6" />} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  Activity: (p: IconProps) => <Svg {...p} d={<path d="M3 12h4l3-7 4 14 3-7h4" />} />,
  Shield: (p: IconProps) => <Svg {...p} d={<path d="M12 3 4 6v6c0 4.5 3.4 8.4 8 9 4.6-.6 8-4.5 8-9V6l-8-3Z" />} />,
  Box: (p: IconProps) => <Svg {...p} d={<><rect x={4} y={4} width={16} height={16} rx={2} /><path d="M4 9h16M9 4v16" /></>} />,
  Plug: (p: IconProps) => <Svg {...p} d={<><path d="M9 2v6M15 2v6" /><rect x={6} y={8} width={12} height={6} rx={2} /><path d="M12 14v4M9 22h6" /></>} />,
  Users: (p: IconProps) => <Svg {...p} d={<><circle cx={9} cy={9} r={3} /><path d="M3 20a6 6 0 0 1 12 0" /><circle cx={17} cy={8} r={2.5} /><path d="M16 14a5 5 0 0 1 5 5" /></>} />,
} as const;

const CATEGORY_META: Readonly<Record<ComponentCategory, { readonly label: string; readonly Icon: React.ComponentType<IconProps> }>> = {
  core: { label: "Core", Icon: ICON.Box },
  connector: { label: "Connectors", Icon: ICON.Plug },
  team: { label: "Teams", Icon: ICON.Users },
};


// ─────────────────────────────────────────────────────────────────────────────
// Sub-components
// ─────────────────────────────────────────────────────────────────────────────

interface HealthHeaderProps {
  readonly overall: CanonicalState;
  readonly healthyCount: number;
  readonly total: number;
}
function HealthHeader({ overall, healthyCount, total }: HealthHeaderProps) {
  const m = STATE_META[overall];
  const tone = TONE[m.tone];
  return (
    <section className={cx("card overflow-hidden border-2", tone.border)}>
      <div className={cx("px-5 py-5 sm:px-7 sm:py-6 flex items-center gap-5 flex-wrap", tone.bg)}>
        <div className={cx(
          "shrink-0 h-14 w-14 rounded-full bg-white/70 backdrop-blur inline-flex items-center justify-center",
          tone.text,
          overall === "recovering" || overall === "attention" ? "ig-pulse" : "",
        )}>
          {overall === "healthy" && <ICON.Heart size={26} />}
          {overall === "recovering" && <ICON.Wave size={26} />}
          {overall === "attention" && <ICON.Eye size={26} />}
          {overall === "paused" && <ICON.Pause size={26} />}
          {overall === "quarantined" && <ICON.Shield size={26} />}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <span className={cx(
              "inline-flex items-center gap-1.5 rounded-full border text-[11px] font-medium px-2 py-0.5 bg-white/70 backdrop-blur",
              tone.text, tone.border,
            )}>
              <span className={cx("h-1.5 w-1.5 rounded-full", m.dot)} />
              {m.label}
            </span>
            <span className={cx("text-[11px] font-mono tabular-nums", tone.text)}>
              {healthyCount}/{total} components healthy
            </span>
          </div>
          <h1 className={cx("mt-1 text-[24px] sm:text-[26px] font-semibold tracking-tight leading-tight", tone.text)}>
            {m.headerLede}
          </h1>
          <p className={cx("mt-1 text-[13.5px] leading-relaxed", tone.text)}>
            {m.headerSub}
          </p>
        </div>

        <div className="hidden sm:flex items-center gap-2 shrink-0 text-[11px] font-mono" aria-hidden>
          <span className={cx("h-2 w-2 rounded-full ig-pulse", m.dot)} />
          <span className={tone.text}>heartbeat</span>
        </div>
      </div>
    </section>
  );
}

function HealthyChip({ c }: { readonly c: HealthComponent }) {
  const CatIcn = CATEGORY_META[c.category].Icon;
  return (
    <div
      className="card flex items-center gap-2 px-3 py-2 hover:shadow-md transition-shadow"
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

function StatusCard({ c }: { readonly c: HealthComponent }) {
  const m = STATE_META[c.state];
  const tone = TONE[m.tone];
  const CatIcn = CATEGORY_META[c.category].Icon;
  return (
    <article className={cx("card overflow-hidden border-2", tone.border)}>
      <div className={cx("px-4 py-3 flex items-start justify-between gap-3", tone.bg)}>
        <div className="flex items-start gap-3 min-w-0">
          <span className={cx("inline-flex h-8 w-8 items-center justify-center rounded-md shrink-0 bg-white/70", tone.text)}>
            <CatIcn size={15} />
          </span>
          <div className="min-w-0">
            <div className="flex items-center gap-1.5">
              <h3 className={cx("text-[14.5px] font-semibold tracking-tight", tone.text)}>{c.name}</h3>
              <span className="text-[10.5px] font-mono text-neutral-500">· {CATEGORY_META[c.category].label}</span>
            </div>
            <div className={cx("mt-0.5 inline-flex items-center gap-1.5 text-[11px]", tone.text)}>
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

        {c.state === "recovering" && typeof c.etaMinutes === "number" && (
          <div className="mt-1">
            <div className="flex items-center justify-between text-[10.5px] text-recovered mb-1">
              <span className="font-medium uppercase tracking-wide">Recovery in progress</span>
              <span className="font-mono tabular-nums">~{c.etaMinutes}m remaining</span>
            </div>
            <div className="h-1 w-full rounded-full bg-neutral-100 overflow-hidden">
              <div className="h-full w-[64%] rounded-full bg-recovered-solid" />
            </div>
          </div>
        )}
      </div>

      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-white">
        <span className="text-[10.5px] text-neutral-500">
          Uptime streak:{" "}
          <span className="text-neutral-800 font-mono tabular-nums">
            {c.uptimeDays === 0 ? "—" : `${c.uptimeDays}d`}
          </span>
        </span>
        <div className="flex items-center gap-1">
          {c.state === "attention" && (
            <button type="button" className="inline-flex items-center gap-1.5 rounded-md bg-warning-solid text-white hover:bg-warning-solid-hover px-2.5 py-1 text-[12px] font-medium">
              <ICON.ArrowRight size={11} /> Open it
            </button>
          )}
          {c.state === "recovering" && (
            <span className="text-[11px] text-recovered inline-flex items-center gap-1">
              <ICON.Sparkles size={11} /> No action needed
            </span>
          )}
          {c.state === "quarantined" && (
            <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-quarantined bg-white hover:bg-quarantined-hover text-quarantined px-2.5 py-1 text-[12px] font-medium">
              Review when ready
            </button>
          )}
        </div>
      </footer>
    </article>
  );
}

function HeartbeatGrid({ components }: { readonly components: readonly HealthComponent[] }) {
  const nonHealthy = components.filter((c) => c.state !== "healthy");
  const healthy = components.filter((c) => c.state === "healthy");
  const recovering = components.filter((c) => c.state === "recovering").length;
  const attention = components.filter((c) => c.state === "attention").length;
  const quarantined = components.filter((c) => c.state === "quarantined").length;

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
            <span className="h-1.5 w-1.5 rounded-full bg-recovered-solid" /> Recovering {recovering}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-warning-solid" /> Attention {attention}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-quarantined-solid" /> Quarantined {quarantined}
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

interface HealEventRowProps {
  readonly ev: HealEvent;
  readonly open: boolean;
  readonly onToggle: () => void;
}
function HealEventRow({ ev, open, onToggle }: HealEventRowProps) {
  const rows: ReadonlyArray<{ readonly label: string; readonly text: string }> = [
    { label: "What we checked", text: ev.story.checked },
    { label: "What changed", text: ev.story.changed },
    { label: "Outcome", text: ev.story.outcome },
    { label: "Follow-up", text: ev.story.followup ?? "None — this was an isolated event." },
  ];
  return (
    <li className="relative pl-7 pr-1 py-3">
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
            {rows.map((row, i) => (
              <div
                key={row.label}
                className={cx(
                  "px-3 py-2.5 border-neutral-100",
                  i % 2 === 0 ? "sm:border-r" : "",
                  i < 2 ? "sm:border-b" : "",
                  i !== 3 ? "border-b sm:border-b" : "",
                )}
              >
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

function HealLog({ events }: { readonly events: readonly HealEvent[] }) {
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

function Sparkline({ points, tone }: { readonly points: readonly number[]; readonly tone: ToneName }) {
  const W = 120;
  const H = 28;
  const max = 1;
  const min = 0;
  const step = W / Math.max(1, points.length - 1);
  const d = points
    .map((p, i) => {
      const x = i * step;
      const y = H - ((p - min) / (max - min)) * (H - 4) - 2;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg width={W} height={H} viewBox={`0 0 ${W} ${H}`} className={TONE[tone].text}>
      <path d={d} fill="none" stroke="currentColor" strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" />
      <path d={`${d} L${W} ${H} L0 ${H} Z`} fill="currentColor" opacity={0.08} />
    </svg>
  );
}

interface PredictiveCardProps {
  readonly w: PredictiveWarning;
  readonly showGraph: boolean;
  readonly onToggleGraph: () => void;
  readonly onAct: () => void;
}
function PredictiveCard({ w, showGraph, onToggleGraph, onAct }: PredictiveCardProps) {
  const toneName: ToneName =
    w.errorBudgetUsedPct >= 80 ? "quarantined" :
    w.errorBudgetUsedPct >= 50 ? "warning" : "neutral";
  const tone = TONE[toneName];
  const badgeLabel =
    toneName === "quarantined" ? "Budget exhausted" :
    toneName === "warning" ? "Budget drifting" : "Watching";
  return (
    <article className="card flex flex-col">
      <div className="px-4 py-3 flex flex-col gap-2">
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <span className={cx("inline-flex items-center gap-1.5 rounded-full border text-[10.5px] font-medium px-1.5 py-0.5", tone.bg, tone.text, tone.border)}>
            <ICON.Activity size={10} /> {badgeLabel}
          </span>
          <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">
            {w.errorBudgetUsedPct}% of {w.windowDays}d budget used
          </span>
        </div>
        <h3 className="text-[14px] font-semibold tracking-tight text-neutral-900">{w.component}</h3>
        <p className="text-[12.5px] text-neutral-700 leading-snug">{w.signal}</p>

        <div className="mt-1">
          <div className="h-1 w-full rounded-full bg-neutral-100 overflow-hidden">
            <div
              className={cx("h-full rounded-full", tone.bgSolid)}
              style={{ width: `${w.errorBudgetUsedPct}%` }}
            />
          </div>
        </div>

        <p className="text-[12px] text-neutral-500 leading-relaxed">{w.why}</p>

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
            <Sparkline points={w.trend} tone={toneName === "neutral" ? "safe" : toneName} />
          </div>
        )}
      </div>

      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/60">
        <button type="button" onClick={onToggleGraph} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
          {showGraph ? <ICON.ChevronUp size={12} /> : <ICON.ChevronDown size={12} />}
          {showGraph ? "Hide graph" : "Show graph"}
        </button>
        <button
          type="button"
          onClick={onAct}
          className={cx(
            "inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors",
            w.suggestedAction === "Pause it"
              ? "bg-warning-solid text-white hover:bg-warning-solid-hover"
              : w.suggestedAction === "Keep paused"
                ? "border border-quarantined bg-white text-quarantined hover:bg-quarantined-hover"
                : "border border-neutral-200 bg-white text-neutral-700 hover:bg-neutral-50",
          )}
        >
          {w.suggestedAction === "Pause it" && <ICON.Pause size={11} />}
          {w.suggestedAction}
        </button>
      </footer>
    </article>
  );
}

interface PredictivePanelProps {
  readonly warnings: readonly PredictiveWarning[];
  readonly openGraphIds: ReadonlySet<string>;
  readonly onToggleGraph: (id: string) => void;
  readonly onAct: (id: string) => void;
}
function PredictivePanel({ warnings, openGraphIds, onToggleGraph, onAct }: PredictivePanelProps) {
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

// ─────────────────────────────────────────────────────────────────────────────
// Main route component
// ─────────────────────────────────────────────────────────────────────────────

export function Health() {
  const HEALTH_MOCK = useMemo(() => api.v2.health.getMock(), []);
  // v0.2 Step 6 — F6 Health real-API. State setters are now exposed so
  // the useEffect below can swap mock → live values when
  // VITE_API_MODE_HEALTH=real. In mock mode the call resolves to the
  // same seed synchronously, so the dispatched setters are no-ops.
  const [components, setComponents] = useState<readonly HealthComponent[]>(HEALTH_MOCK.components);
  const [healEvents, setHealEvents] = useState<readonly HealEvent[]>(HEALTH_MOCK.healEvents);
  const [predictive, setPredictive] = useState<readonly PredictiveWarning[]>(HEALTH_MOCK.predictive);
  const [openGraphIds, setOpenGraphIds] = useState<Set<string>>(new Set());
  const [toast, setToast] = useState<string | null>(null);

  // v0.3 Step 6 — wrap the v0.2 load in `useRouteData` so a real-mode
  // failure surfaces via `<RouteError>` instead of silently keeping the
  // mock seed visible. Mock mode resolves synchronously, so the error
  // branch never fires there.
  const healthLoad = useRouteData({
    initialData: HEALTH_MOCK,
    load: () => api.v2.health.load(),
  });
  useEffect(() => {
    if (healthLoad.status !== "ok" || healthLoad.data == null) return;
    setComponents(healthLoad.data.components);
    setHealEvents(healthLoad.data.healEvents);
    setPredictive(healthLoad.data.predictive);
  }, [healthLoad.status, healthLoad.data]);

  useEffect(() => {
    if (!toast) return undefined;
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  const overall: CanonicalState = useMemo(() => {
    if (components.some((c) => c.state === "quarantined")) return "quarantined";
    if (components.some((c) => c.state === "attention")) return "attention";
    if (components.some((c) => c.state === "recovering")) return "recovering";
    if (components.every((c) => c.state === "paused")) return "paused";
    return "healthy";
  }, [components]);

  const healthyCount = useMemo(
    () => components.filter((c) => c.state === "healthy").length,
    [components],
  );

  const handleToggleGraph = (id: string) => {
    setOpenGraphIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handlePredictiveAct = (id: string) => {
    const w = predictive.find((x) => x.id === id);
    if (!w) return;
    setToast(`${w.suggestedAction} · ${w.component}`);
  };

  // v0.3 Step 6 — explicit error rendering when the gateway fails in
  // real mode. Mock mode never lands here because its load() resolves
  // synchronously with the seed.
  if (healthLoad.status === "error") {
    return (
      <div className="min-h-screen bg-neutral-50">
        <WorkspaceTopbar showHeartbeatPill={false} />
        <RouteError
          route="Health"
          error={healthLoad.error}
          onRetry={healthLoad.reload}
        />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar showHeartbeatPill={false} />

      <main className="page-container max-w-[78rem] flex flex-col gap-6">
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
      </main>

      {toast && (
        <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50 ig-toast-in">
          <div className="rounded-lg bg-neutral-900 text-white shadow-lg px-3.5 py-2 text-[12.5px] font-medium inline-flex items-center gap-2">
            <ICON.Check size={13} className="text-safe-solid" />
            {toast}
          </div>
        </div>
      )}
    </div>
  );
}
