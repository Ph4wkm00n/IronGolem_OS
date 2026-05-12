// route: /recipes
// purpose: browse, configure, and activate automation recipes. Every recipe
// ships with a safety summary above the activation control. Replaces the
// stub when VITE_ENABLE_V2_UI=true. Ported from Claude Design's Recipes.tsx
// — source lives at apps/web/src/_design-inbox/recipes/.
//
// Integration notes:
// - Shell chrome (sticky topbar) imported from `pages/v2/_shared/WorkspaceTopbar`.
// - Mock data inline; swap for `useRecipesQuery()` once lib/api.ts ships.
// - Dynamic `bg-${tone}` patterns replaced with the static TONE classmap so
//   Tailwind's JIT compiles every class — same pattern as Home + Inbox.
// - Drops the design-exploration preview shim (`window.Recipes = ...`).

import React, { useEffect, useMemo, useState } from "react";

import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

import {
  api,
  type Recipe,
  type RecipeCategory as Category,
  type RecipeStatus,
  type RecipeRisk as Risk,
  type PermScope,
  type RecipePermission as Permission,
  type RecipeSafetyShape as SafetyShape,
  type RecipePolicyLayer as PolicyLayer,
  type RunOutcome,
  type RunEvent,
} from "../../lib/api";

type ToneName = "safe" | "warning" | "blocked" | "recovered" | "quarantined" | "accent" | "neutral";

// ─────────────────────────────────────────────────────────────────────────────
// Tone classmap — Tailwind needs every utility class as a literal string.
// Mirrors Home.tsx and Inbox.tsx so the audit pipeline can dedupe later.
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

const RISK_TONE: Readonly<Record<Risk, ToneName>> = {
  low: "safe",
  medium: "warning",
  high: "blocked",
};

const STATUS_META: Readonly<Record<RecipeStatus, { readonly label: string; readonly tone: ToneName }>> = {
  active: { label: "Active", tone: "safe" },
  paused: { label: "Paused", tone: "neutral" },
  new: { label: "New", tone: "accent" },
};

const SCOPE_META: Readonly<Record<PermScope, { readonly label: string; readonly tone: ToneName }>> = {
  scoped: { label: "Scoped", tone: "neutral" },
  broad: { label: "Broad", tone: "warning" },
  restricted: { label: "Restricted", tone: "blocked" },
};

// ─────────────────────────────────────────────────────────────────────────────
// Inline icons (Heroicons-style)
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
  Inbox: (p: IconProps) => <Svg {...p} d={<><path d="M3 12V5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v7" /><path d="M3 12h5l1.5 2.5h5L16 12h5v6a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-6Z" /></>} />,
  Calendar: (p: IconProps) => <Svg {...p} d={<><rect x={3} y={5} width={18} height={16} rx={2} /><path d="M3 9h18M8 3v4M16 3v4" /></>} />,
  Cart: (p: IconProps) => <Svg {...p} d={<><path d="M3 4h2l2 12h12l2-8H6" /><circle cx={9} cy={20} r={1.2} /><circle cx={18} cy={20} r={1.2} /></>} />,
  Search: (p: IconProps) => <Svg {...p} d={<><circle cx={11} cy={11} r={6} /><path d="m20 20-4.3-4.3" /></>} />,
  Wand: (p: IconProps) => <Svg {...p} d={<><path d="m3 21 12-12" /><path d="m13 5 2 2" /><path d="m17 9 2 2" /><path d="m9 1 1 2 2 1-2 1-1 2-1-2-2-1 2-1 1-2Z" /></>} />,
  Edit: (p: IconProps) => <Svg {...p} d={<><path d="M4 20h4l10-10-4-4L4 16v4Z" /><path d="m14 6 4 4" /></>} />,
  Cpu: (p: IconProps) => <Svg {...p} d={<><rect x={5} y={5} width={14} height={14} rx={2} /><rect x={9} y={9} width={6} height={6} rx={1} /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>} />,
  Check: (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  X: (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Play: (p: IconProps) => <Svg {...p} d={<path d="M7 4v16l13-8L7 4Z" />} />,
  Pause: (p: IconProps) => <Svg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  Bolt: (p: IconProps) => <Svg {...p} d={<path d="M13 2 4 14h7l-1 8 9-12h-7l1-8Z" />} />,
  Clock: (p: IconProps) => <Svg {...p} d={<><circle cx={12} cy={12} r={9} /><path d="M12 7v5l3 2" /></>} />,
  Shield: (p: IconProps) => <Svg {...p} d={<><path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" /></>} />,
  Lock: (p: IconProps) => <Svg {...p} d={<><rect x={5} y={11} width={14} height={9} rx={2} /><path d="M8 11V8a4 4 0 1 1 8 0v3" /></>} />,
  Eye: (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx={12} cy={12} r={3} /></>} />,
  Bell: (p: IconProps) => <Svg {...p} d={<><path d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2Z" /><path d="M10 20a2 2 0 0 0 4 0" /></>} />,
  Slash: (p: IconProps) => <Svg {...p} d={<path d="M5 19 19 5" />} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  ArrowLeft: (p: IconProps) => <Svg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
  Sliders: (p: IconProps) => <Svg {...p} d={<><path d="M4 6h12" /><path d="M4 12h7" /><path d="M4 18h10" /><circle cx={18} cy={6} r={2} /><circle cx={14} cy={12} r={2} /><circle cx={16} cy={18} r={2} /></>} />,
  Plus: (p: IconProps) => <Svg {...p} d={<><path d="M12 5v14" /><path d="M5 12h14" /></>} />,
  Layers: (p: IconProps) => <Svg {...p} d={<><path d="m12 3 9 5-9 5-9-5 9-5Z" /><path d="m3 13 9 5 9-5" /><path d="m3 18 9 5 9-5" /></>} />,
} as const;

const CATEGORIES: ReadonlyArray<{ readonly id: Category; readonly label: string }> = [
  { id: "inbox", label: "Inbox" },
  { id: "calendar", label: "Calendar" },
  { id: "purchasing", label: "Purchasing" },
  { id: "research", label: "Research" },
  { id: "operations", label: "Operations" },
  { id: "drafting", label: "Drafting" },
];

const CATEGORY_META: Readonly<Record<Category, { readonly label: string; readonly Icon: React.ComponentType<IconProps> }>> = {
  inbox: { label: "Inbox", Icon: ICON.Inbox },
  calendar: { label: "Calendar", Icon: ICON.Calendar },
  purchasing: { label: "Purchasing", Icon: ICON.Cart },
  research: { label: "Research", Icon: ICON.Search },
  operations: { label: "Operations", Icon: ICON.Cpu },
  drafting: { label: "Drafting", Icon: ICON.Edit },
};

const OUTCOME_META: Readonly<Record<RunOutcome, { readonly label: string; readonly tone: ToneName; readonly Icon: React.ComponentType<IconProps> }>> = {
  completed: { label: "Completed", tone: "safe", Icon: ICON.Check },
  approved: { label: "Approved", tone: "accent", Icon: ICON.Check },
  held: { label: "Held", tone: "quarantined", Icon: ICON.Pause },
  skipped: { label: "Skipped", tone: "neutral", Icon: ICON.Slash },
  denied: { label: "Denied", tone: "blocked", Icon: ICON.X },
};


// ─────────────────────────────────────────────────────────────────────────────
// Filtering
// ─────────────────────────────────────────────────────────────────────────────

type TabId = Category | "all";

function countByTab(recipes: readonly Recipe[]): Record<TabId, number> {
  const out: Record<TabId, number> = {
    all: recipes.length,
    inbox: 0, calendar: 0, purchasing: 0, research: 0, operations: 0, drafting: 0,
  };
  for (const r of recipes) out[r.category] += 1;
  return out;
}

function applyFilters(recipes: readonly Recipe[], tab: TabId, q: string): readonly Recipe[] {
  const needle = q.trim().toLowerCase();
  return recipes.filter((r) => {
    if (tab !== "all" && r.category !== tab) return false;
    if (!needle) return true;
    return (
      r.name.toLowerCase().includes(needle) ||
      r.purpose.toLowerCase().includes(needle) ||
      r.permissions.some((p) => p.key.includes(needle))
    );
  });
}

// ─────────────────────────────────────────────────────────────────────────────
// Sub-components
// ─────────────────────────────────────────────────────────────────────────────

// TODO(integrator): swap for <RiskBadge /> from @irongolem/ui.
function RiskBadge({ level, size = "sm" }: { readonly level: Risk; readonly size?: "sm" | "md" }) {
  const tone = TONE[RISK_TONE[level]];
  const label = ({ low: "low risk", medium: "med risk", high: "high risk" } as const)[level];
  const sizeCx = size === "sm" ? "text-[10px] px-1.5 py-0.5" : "text-[11px] px-2 py-0.5";
  return (
    <span className={cx("inline-flex items-center gap-1 rounded-full border font-medium", sizeCx, tone.bg, tone.text, tone.border)}>
      <span className={cx("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
      {label}
    </span>
  );
}

function StatusBadge({ status }: { readonly status: RecipeStatus }) {
  const m = STATUS_META[status];
  const tone = TONE[m.tone];
  return (
    <span className={cx("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[10.5px] font-medium uppercase tracking-wide", tone.bg, tone.text)}>
      <span className={cx("h-1.5 w-1.5 rounded-full", tone.bgSolid, status === "active" && "ig-pulse")} />
      {m.label}
    </span>
  );
}

function CategoryPill({ category }: { readonly category: Category }) {
  const m = CATEGORY_META[category];
  const Icn = m.Icon;
  return (
    <span className="inline-flex items-center gap-1.5 rounded-md bg-neutral-100 px-1.5 py-0.5 text-[10.5px] font-medium text-neutral-600">
      <Icn size={11} /> {m.label}
    </span>
  );
}

// TODO(integrator): swap for <SafetyCard /> from @irongolem/ui.
function SafetyCard({ safety }: { readonly safety: SafetyShape }) {
  const sections: ReadonlyArray<{ readonly label: string; readonly items: readonly string[]; readonly tone: ToneName; readonly Icn: React.ComponentType<IconProps> }> = [
    { label: "Can", items: safety.can, tone: "safe", Icn: ICON.Check },
    { label: "Needs approval", items: safety.needsApproval, tone: "warning", Icn: ICON.Bell },
    { label: "Cannot", items: safety.cannot, tone: "blocked", Icn: ICON.Slash },
    { label: "Stops if", items: safety.stopsIf, tone: "quarantined", Icn: ICON.Pause },
  ];
  return (
    <div className="rounded-xl border border-neutral-200 bg-neutral-50/60 p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Safety summary</div>
        <span className="inline-flex items-center gap-1.5 text-[11px] text-safe font-medium">
          <ICON.Shield size={12} /> Posture: active
        </span>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-4">
        {sections.map((s) => {
          const tone = TONE[s.tone];
          return (
            <div key={s.label}>
              <div className="flex items-center gap-1.5 mb-2">
                <span className={tone.text}><s.Icn size={13} /></span>
                <span className={cx("text-[11px] font-medium uppercase tracking-wide", tone.text)}>{s.label}</span>
              </div>
              <ul className="space-y-1">
                {s.items.length === 0 && <li className="text-[12px] text-neutral-400">—</li>}
                {s.items.map((it) => (
                  <li key={it} className="text-[13px] text-neutral-700 flex gap-2 leading-snug">
                    <span className={cx("mt-1.5 h-1 w-1 rounded-full shrink-0", tone.bgSolid)} />
                    <span>{it}</span>
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

function SafetyCardPreview({ safety, onSeeAll }: { readonly safety: SafetyShape; readonly onSeeAll: () => void }) {
  const cans = safety.can.slice(0, 1);
  const approvals = safety.needsApproval.slice(0, 1);
  const cannots = safety.cannot.slice(0, 1);
  const totalLines = safety.can.length + safety.cannot.length + safety.needsApproval.length + safety.stopsIf.length;
  const shown = cans.length + approvals.length + cannots.length;
  const remaining = totalLines - shown;

  return (
    <div className="rounded-lg border border-neutral-200 bg-neutral-50/70 p-3">
      <div className="flex items-center justify-between mb-2">
        <div className="inline-flex items-center gap-1.5 text-[10.5px] font-medium uppercase tracking-wide text-neutral-500">
          <ICON.Shield size={11} /> Safety summary
        </div>
        <button type="button" onClick={onSeeAll} className="text-[11px] font-medium text-accent hover:text-accent-solid inline-flex items-center gap-0.5">
          See all {totalLines}
          <ICON.ArrowRight size={10} />
        </button>
      </div>
      <ul className="space-y-1">
        {cans.map((line) => (
          <li key={`c-${line}`} className="text-[12.5px] text-neutral-700 flex gap-2 leading-snug">
            <span className="mt-1.5 h-1 w-1 rounded-full shrink-0 bg-safe-solid" />
            <span><span className="text-safe font-medium">Can</span> · {line}</span>
          </li>
        ))}
        {approvals.map((line) => (
          <li key={`a-${line}`} className="text-[12.5px] text-neutral-700 flex gap-2 leading-snug">
            <span className="mt-1.5 h-1 w-1 rounded-full shrink-0 bg-warning-solid" />
            <span><span className="text-warning font-medium">Needs approval</span> · {line}</span>
          </li>
        ))}
        {cannots.map((line) => (
          <li key={`n-${line}`} className="text-[12.5px] text-neutral-700 flex gap-2 leading-snug">
            <span className="mt-1.5 h-1 w-1 rounded-full shrink-0 bg-blocked-solid" />
            <span><span className="text-blocked font-medium">Cannot</span> · {line}</span>
          </li>
        ))}
      </ul>
      {remaining > 0 && (
        <div className="mt-2 text-[10.5px] text-neutral-400">
          + {remaining} more across <span className="text-neutral-500">cannot / stops if</span>
        </div>
      )}
    </div>
  );
}

// TODO(integrator): swap for <PolicyCard /> from @irongolem/ui.
function PolicyCard({ layers }: { readonly layers: readonly PolicyLayer[] }) {
  return (
    <div className="rounded-xl border border-neutral-200 bg-white p-4">
      <div className="flex items-center justify-between mb-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Safety rules</div>
          <div className="text-[14.5px] font-semibold tracking-tight text-neutral-900 mt-0.5">
            Five layers, {layers.filter((l) => l.state === "active").length} active
          </div>
        </div>
        <span className="inline-flex items-center gap-1.5 text-[11px] text-safe font-medium">
          <ICON.Layers size={12} /> Enforced top-down
        </span>
      </div>
      <ol className="space-y-1.5">
        {layers.map((layer) => {
          const isWatching = layer.state === "watching";
          return (
            <li
              key={layer.id}
              className={cx(
                "flex items-start gap-3 rounded-lg border px-3 py-2",
                isWatching ? "bg-warning border-warning" : "border-neutral-100 bg-neutral-50/40",
              )}
            >
              <span className={cx(
                "h-6 w-6 rounded-full inline-flex items-center justify-center text-[11px] font-semibold shrink-0",
                isWatching ? "bg-warning-solid text-white" : "bg-safe-solid text-white",
              )}>
                {layer.id}
              </span>
              <div className="flex-1 min-w-0">
                <div className="text-[13px] font-medium text-neutral-900">{layer.name}</div>
                <div className="text-[12px] text-neutral-500 leading-snug">{layer.note}</div>
              </div>
              {isWatching
                ? <ICON.Eye size={14} className="text-warning mt-1" />
                : <ICON.Check size={14} className="text-safe mt-1" />}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

function RunsTimeline({ runs }: { readonly runs: readonly RunEvent[] }) {
  if (runs.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-neutral-200 bg-neutral-50/60 p-6 text-center">
        <div className="text-[13px] text-neutral-600">No runs yet.</div>
        <div className="text-[11.5px] text-neutral-400 mt-1">
          Once activated, the ten most recent runs will appear here.
        </div>
      </div>
    );
  }
  return (
    <ol className="relative pl-5 space-y-3">
      <span className="absolute left-[7px] top-1 bottom-1 w-px bg-neutral-200" />
      {runs.slice(0, 10).map((r, i) => {
        const m = OUTCOME_META[r.outcome];
        const Icn = m.Icon;
        const tone = TONE[m.tone];
        return (
          <li key={i} className="relative">
            <span className={cx("absolute -left-[19px] top-[3px] h-3.5 w-3.5 rounded-full inline-flex items-center justify-center", tone.bg, tone.text)}>
              <Icn size={9} />
            </span>
            <div className="flex items-baseline gap-2 flex-wrap">
              <span className={cx("text-[11px] font-medium uppercase tracking-wide", tone.text)}>{m.label}</span>
              <span className="text-[11px] text-neutral-400 font-mono">{r.at}</span>
            </div>
            <div className="text-[12.5px] text-neutral-600 leading-snug">{r.note}</div>
          </li>
        );
      })}
    </ol>
  );
}

interface CategoryTabsProps {
  readonly active: TabId;
  readonly counts: Record<TabId, number>;
  readonly onChange: (t: TabId) => void;
}
function CategoryTabs({ active, counts, onChange }: CategoryTabsProps) {
  const tabs: ReadonlyArray<{ readonly id: TabId; readonly label: string; readonly Icon?: React.ComponentType<IconProps> }> = [
    { id: "all", label: "All" },
    ...CATEGORIES.map((c) => ({ id: c.id as TabId, label: c.label, Icon: CATEGORY_META[c.id].Icon })),
  ];
  return (
    <div className="flex items-center gap-1 overflow-x-auto scrollbar-thin -mx-1 px-1 pb-1">
      {tabs.map((t) => {
        const isActive = active === t.id;
        const n = counts[t.id];
        const Icn = t.Icon;
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onChange(t.id)}
            className={cx(
              "shrink-0 inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[13px] font-medium border transition-colors",
              isActive ? "bg-neutral-900 text-white border-neutral-900" : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
            )}
          >
            {Icn && <Icn size={13} />}
            {t.label}
            <span className={cx("rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px", isActive ? "bg-white/15 text-white" : "bg-neutral-100 text-neutral-500")}>
              {n}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function PrimaryAction({ status, onClick }: { readonly status: RecipeStatus; readonly onClick: () => void }) {
  if (status === "active") {
    return (
      <button type="button" onClick={onClick} className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white text-neutral-700 hover:bg-neutral-50 px-3 py-1.5 text-[12.5px] font-medium transition-colors">
        <ICON.Pause size={12} /> Pause
      </button>
    );
  }
  if (status === "paused") {
    return (
      <button type="button" onClick={onClick} className="inline-flex items-center gap-1.5 rounded-md bg-safe-solid text-white hover:bg-safe-solid-hover px-3 py-1.5 text-[12.5px] font-medium transition-colors">
        <ICON.Play size={12} /> Activate
      </button>
    );
  }
  return (
    <button type="button" onClick={onClick} className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-3 py-1.5 text-[12.5px] font-medium transition-colors">
      <ICON.Play size={12} /> Activate
    </button>
  );
}

function RunOnceButton({ onClick }: { readonly onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="inline-flex items-center gap-1.5 rounded-md text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50 px-2 py-1.5 text-[12.5px] font-medium transition-colors">
      <ICON.Bolt size={12} /> Run once
    </button>
  );
}

function TrustStrip({ recipe }: { readonly recipe: Recipe }) {
  const broad = recipe.permissions.filter((p) => p.scope === "broad").length;
  const restricted = recipe.permissions.filter((p) => p.scope === "restricted").length;
  const trustToneName: ToneName = restricted ? "blocked" : broad ? "warning" : "neutral";
  const trustTone = TONE[trustToneName];
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
      <RiskBadge level={recipe.risk} />
      <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
        <ICON.Lock size={11} className={trustTone.textSolid} />
        <span>
          <span className={cx("font-medium", trustTone.text)}>{recipe.permissions.length}</span>{" "}
          permission{recipe.permissions.length === 1 ? "" : "s"}
          {broad > 0 && <span className="text-warning"> · {broad} broad</span>}
        </span>
      </span>
      <span className="text-neutral-300">·</span>
      <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
        <ICON.Clock size={11} />
        {recipe.lastRun ? `Last run ${recipe.lastRun}` : <span className="text-neutral-400">Never run</span>}
      </span>
    </div>
  );
}

interface RecipeCardProps {
  readonly recipe: Recipe;
  readonly onInspect: () => void;
  readonly onToggle: () => void;
  readonly onRunOnce: () => void;
}
function RecipeCard({ recipe, onInspect, onToggle, onRunOnce }: RecipeCardProps) {
  const CatIcn = CATEGORY_META[recipe.category].Icon;
  return (
    <article className="group card flex flex-col h-full overflow-hidden hover:shadow-md transition-shadow">
      <header className="flex items-center justify-between px-4 pt-4">
        <div className="inline-flex items-center gap-1.5">
          <span className="h-7 w-7 rounded-lg bg-neutral-100 inline-flex items-center justify-center text-neutral-600">
            <CatIcn size={14} />
          </span>
          <CategoryPill category={recipe.category} />
        </div>
        <StatusBadge status={recipe.status} />
      </header>

      <div className="px-4 pt-3">
        <h3 className="text-[15.5px] font-semibold tracking-tight text-neutral-900 leading-snug">{recipe.name}</h3>
        <p className="mt-1 text-[12.5px] text-neutral-600 leading-relaxed">{recipe.purpose}</p>
      </div>

      <div className="px-4 pt-3">
        <SafetyCardPreview safety={recipe.safety} onSeeAll={onInspect} />
      </div>

      <div className="px-4 pt-3 pb-3">
        <TrustStrip recipe={recipe} />
      </div>

      <footer className="mt-auto border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/50">
        <div className="flex items-center gap-1">
          <PrimaryAction status={recipe.status} onClick={onToggle} />
          {recipe.status !== "new" && <RunOnceButton onClick={onRunOnce} />}
        </div>
        <button type="button" onClick={onInspect} className="inline-flex items-center gap-1 text-[12.5px] font-medium text-accent hover:text-accent-solid transition-colors">
          Inspect
          <ICON.ArrowRight size={11} />
        </button>
      </footer>
    </article>
  );
}

function PermissionsList({ permissions }: { readonly permissions: readonly Permission[] }) {
  return (
    <ul className="divide-y divide-neutral-100 rounded-xl border border-neutral-200 bg-white overflow-hidden">
      {permissions.map((p) => {
        const m = SCOPE_META[p.scope];
        const tone = TONE[m.tone];
        return (
          <li key={p.key} className="px-3 py-2.5 flex items-start gap-3">
            <span className={cx("shrink-0 h-7 w-7 rounded-md inline-flex items-center justify-center", tone.bg, tone.text)}>
              <ICON.Lock size={13} />
            </span>
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-2 flex-wrap">
                <span className="text-[12px] font-mono text-neutral-900">{p.key}</span>
                <span className={cx("text-[10px] font-medium uppercase tracking-wide rounded-full border px-1.5 py-0.5", tone.bg, tone.text, tone.border)}>
                  {m.label} scope
                </span>
              </div>
              <div className="text-[12.5px] text-neutral-600 leading-snug">{p.label}</div>
            </div>
            <div className="text-[11px] text-neutral-400 font-mono tabular-nums whitespace-nowrap">
              {p.approvals != null ? `${p.approvals.toLocaleString()}× approved` : "0× approved"}
            </div>
          </li>
        );
      })}
    </ul>
  );
}

function CustomizePanel({ recipe }: { readonly recipe: Recipe }) {
  return (
    <div className="rounded-xl border border-neutral-200 bg-white overflow-hidden">
      <div className="px-4 py-3 border-b border-neutral-100 bg-neutral-50/60 flex items-center justify-between">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Advanced</div>
          <div className="text-[14px] font-semibold tracking-tight text-neutral-900">Customize behavior</div>
        </div>
        <button type="button" className="inline-flex items-center gap-1.5 rounded-md bg-white border border-neutral-200 hover:bg-neutral-50 text-neutral-700 px-2.5 py-1 text-[12px] font-medium transition-colors">
          <ICON.Sliders size={12} /> Open editor
        </button>
      </div>
      <div className="px-4 py-3 grid grid-cols-[6.5rem_1fr] gap-y-2 gap-x-3 text-[12.5px]">
        <span className="text-neutral-500">Trigger</span>
        <span className="font-mono text-neutral-900 break-all">{recipe.cron}</span>
        <span className="text-neutral-500">On failure</span>
        <span className="text-neutral-700">{recipe.fallback}</span>
        <span className="text-neutral-500">Retry policy</span>
        <span className="text-neutral-700">{recipe.retry}</span>
      </div>
      <div className="px-4 py-2 border-t border-neutral-100 bg-neutral-50/40 text-[11px] text-neutral-500">
        Changes here override the recipe defaults. Operator review (layer 5) will run on every change before it goes live.
      </div>
    </div>
  );
}

interface DetailDrawerProps {
  readonly recipe: Recipe;
  readonly onClose: () => void;
  readonly onToggle: () => void;
  readonly onRunOnce: () => void;
}
function DetailDrawer({ recipe, onClose, onToggle, onRunOnce }: DetailDrawerProps) {
  const CatIcn = CATEGORY_META[recipe.category].Icon;

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-40 flex justify-end" role="dialog" aria-modal="true" aria-label={`Inspect ${recipe.name}`}>
      <div className="ig-drawer-backdrop absolute inset-0" onClick={onClose} />
      <aside className="relative h-full w-full max-w-[640px] bg-white shadow-xl border-l border-neutral-200 flex flex-col ig-slide-in">
        <header className="sticky top-0 z-10 bg-white/90 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
          <button type="button" onClick={onClose} className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
            <ICON.ArrowLeft size={13} /> Back
          </button>
          <span className="text-[11px] font-mono text-neutral-400">{recipe.id.toUpperCase()}</span>
          <span className="text-neutral-300">·</span>
          <CategoryPill category={recipe.category} />
          <div className="ml-auto flex items-center gap-2">
            <StatusBadge status={recipe.status} />
            <button type="button" onClick={onClose} aria-label="Close drawer" className="h-7 w-7 inline-flex items-center justify-center rounded-md text-neutral-500 hover:text-neutral-900 hover:bg-neutral-50">
              <ICON.X size={14} />
            </button>
          </div>
        </header>

        <div className="flex-1 overflow-y-auto scrollbar-thin">
          <div className="px-6 py-6">
            <div className="flex items-start gap-3">
              <span className="h-10 w-10 rounded-xl bg-accent text-accent inline-flex items-center justify-center shrink-0">
                <CatIcn size={20} />
              </span>
              <div className="min-w-0">
                <h1 className="text-[22px] font-semibold tracking-tight text-neutral-900 leading-tight">{recipe.name}</h1>
                <p className="mt-1.5 text-[13.5px] text-neutral-600 leading-relaxed">{recipe.purpose}</p>
              </div>
            </div>

            <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-2">
              <RiskBadge level={recipe.risk} size="md" />
              <span className="inline-flex items-center gap-1.5 text-[12px] text-neutral-500">
                <ICON.Clock size={12} />
                {recipe.lastRun ? `Last run ${recipe.lastRun}` : <span className="text-neutral-400">Never run</span>}
              </span>
              <span className="text-neutral-300">·</span>
              <span className="inline-flex items-center gap-1.5 text-[12px] text-neutral-500">
                <ICON.Calendar size={12} />
                {recipe.schedule}
              </span>
            </div>

            <section className="mt-6">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Safety summary</h2>
                <span className="text-[10.5px] text-neutral-400">Always above activation</span>
              </div>
              <SafetyCard safety={recipe.safety} />
            </section>

            <section className="mt-5">
              <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">Policy layers</h2>
              <PolicyCard layers={recipe.policy} />
            </section>

            <section className="mt-5">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Required permissions</h2>
                <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">{recipe.permissions.length} total</span>
              </div>
              <PermissionsList permissions={recipe.permissions} />
            </section>

            <section className="mt-5">
              <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">Schedule &amp; advanced</h2>
              <CustomizePanel recipe={recipe} />
            </section>

            <section className="mt-5 pb-6">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Recent runs</h2>
                <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">
                  last {Math.min(recipe.recentRuns.length, 10)} of {recipe.recentRuns.length}
                </span>
              </div>
              <RunsTimeline runs={recipe.recentRuns} />
            </section>
          </div>
        </div>

        <footer className="sticky bottom-0 bg-white/95 backdrop-blur border-t border-neutral-100 px-5 py-3 flex items-center justify-between gap-2">
          <RunOnceButton onClick={onRunOnce} />
          <div className="flex items-center gap-2">
            <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium transition-colors">
              <ICON.Sliders size={12} /> Customize
            </button>
            <PrimaryAction status={recipe.status} onClick={onToggle} />
          </div>
        </footer>
      </aside>
    </div>
  );
}

function EmptyState({ tab, q }: { readonly tab: TabId; readonly q: string }) {
  if (q.trim()) {
    return (
      <div className="card card-padded">
        <div className="flex flex-col items-center text-center py-10 max-w-md mx-auto">
          <div className="h-10 w-10 rounded-full bg-neutral-100 inline-flex items-center justify-center mb-3">
            <ICON.Search size={18} className="text-neutral-400" />
          </div>
          <h3 className="text-[15px] font-semibold tracking-tight text-neutral-900">No recipes match "{q}"</h3>
          <p className="text-[13px] text-neutral-500 mt-1.5 leading-relaxed">
            Try a different search, or switch the category tab above.
          </p>
        </div>
      </div>
    );
  }
  const label = tab === "all" ? "in any category" : `in ${CATEGORY_META[tab as Category].label}`;
  return (
    <div className="card card-padded">
      <div className="flex flex-col items-center text-center py-10 max-w-md mx-auto">
        <div className="h-10 w-10 rounded-full bg-accent inline-flex items-center justify-center mb-3 text-accent">
          <ICON.Wand size={18} />
        </div>
        <h3 className="text-[15px] font-semibold tracking-tight text-neutral-900">No recipes here yet</h3>
        <p className="text-[13px] text-neutral-500 mt-1.5 leading-relaxed">
          Nothing has been shipped {label}. You can request a recipe in{" "}
          <a href="#settings" className="text-accent hover:text-accent-solid font-medium">
            Settings → Recipe Requests
          </a>.
        </p>
      </div>
    </div>
  );
}

interface RecipesHeaderProps {
  readonly active: number;
  readonly q: string;
  readonly onQuery: (v: string) => void;
}
function RecipesHeader({ active, q, onQuery }: RecipesHeaderProps) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div className="flex items-center gap-2">
          <h1 className="page-title">Recipes</h1>
          <span className="inline-flex items-center gap-1.5 rounded-full bg-safe px-2 py-0.5 text-[11px] text-safe font-medium">
            <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
            {active} active
          </span>
        </div>
        <p className="mt-1 text-[13.5px] text-neutral-500 max-w-xl leading-relaxed">
          Pre-composed automations you can activate. Every recipe ships with a safety summary — what it can do, what it can't, what needs your approval, and what stops it.
        </p>
      </div>
      <div className="flex items-center gap-2">
        <label className="relative inline-flex items-center">
          <span className="absolute left-2.5 text-neutral-400">
            <ICON.Search size={13} />
          </span>
          <input
            type="search"
            value={q}
            onChange={(e) => onQuery(e.target.value)}
            placeholder="Search recipes or permissions…"
            className="w-64 max-w-full pl-8 pr-2 py-1.5 text-[13px] bg-white border border-neutral-200 rounded-lg focus:outline-none focus:ring-2 ring-accent placeholder:text-neutral-400"
          />
        </label>
        <button type="button" className="inline-flex items-center gap-1.5 rounded-lg bg-neutral-900 text-white hover:bg-neutral-800 px-3 py-1.5 text-[13px] font-medium transition-colors">
          <ICON.Plus size={13} /> Request a recipe
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route component
// ─────────────────────────────────────────────────────────────────────────────

export function Recipes() {
  const [recipes, setRecipes] = useState<Recipe[]>(() => api.v2.recipes.getMock().recipes.map((r) => ({ ...r })));
  const [tab, setTab] = useState<TabId>("all");
  const [q, setQ] = useState("");
  const [openId, setOpenId] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  useEffect(() => {
    document.documentElement.style.overflow = openId ? "hidden" : "";
    return () => {
      document.documentElement.style.overflow = "";
    };
  }, [openId]);

  useEffect(() => {
    if (!toast) return undefined;
    const t = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(t);
  }, [toast]);

  const counts = useMemo(() => countByTab(recipes), [recipes]);
  const filtered = useMemo(() => applyFilters(recipes, tab, q), [recipes, tab, q]);
  const opened = useMemo(() => recipes.find((r) => r.id === openId) ?? null, [recipes, openId]);

  const onToggle = (id: string) => {
    setRecipes((prev) =>
      prev.map((r) => {
        if (r.id !== id) return r;
        const next: RecipeStatus = r.status === "active" ? "paused" : "active";
        const verb = next === "active" ? "Activated" : "Paused";
        setToast(`${verb} · ${r.name}`);
        return { ...r, status: next };
      }),
    );
  };
  const onRunOnce = (id: string) => {
    const r = recipes.find((x) => x.id === id);
    if (r) setToast(`Triggered one run · ${r.name}`);
  };

  const activeCount = recipes.filter((r) => r.status === "active").length;

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar />

      <main className="page-container max-w-[78rem]">
        <RecipesHeader active={activeCount} q={q} onQuery={setQ} />

        <div className="mt-5">
          <CategoryTabs active={tab} counts={counts} onChange={setTab} />
        </div>

        <div className="mt-5">
          {filtered.length === 0 ? (
            <EmptyState tab={tab} q={q} />
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {filtered.map((r) => (
                <RecipeCard
                  key={r.id}
                  recipe={r}
                  onInspect={() => setOpenId(r.id)}
                  onToggle={() => onToggle(r.id)}
                  onRunOnce={() => onRunOnce(r.id)}
                />
              ))}
            </div>
          )}
        </div>

        <footer className="mt-10 mb-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-neutral-500">
          <div className="inline-flex items-center gap-1.5">
            <ICON.Shield size={13} className="text-safe" />
            All recipes route through the five-layer safety system before any external action.
          </div>
          <a href="#policy" className="text-accent hover:text-accent-solid font-medium">
            How safety works →
          </a>
        </footer>
      </main>

      {opened && (
        <DetailDrawer
          recipe={opened}
          onClose={() => setOpenId(null)}
          onToggle={() => onToggle(opened.id)}
          onRunOnce={() => onRunOnce(opened.id)}
        />
      )}

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
