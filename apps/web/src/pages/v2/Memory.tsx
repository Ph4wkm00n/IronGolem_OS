// route: /memory
// purpose: what the system has learned about this workspace — facts with
// evidence trails, "why do you know this?" one click away, "Forget this"
// always visible. Replaces the stub when VITE_ENABLE_V2_UI=true. Ported
// from Claude Design's Memory.tsx — source at apps/web/src/_design-inbox/memory/.
//
// Integration notes:
// - Shell chrome (sticky topbar) imported from `pages/v2/_shared/WorkspaceTopbar`.
// - Mock data inline; swap for `useMemoryQuery()` once lib/api.ts ships.
// - Dynamic `bg-${tone}` patterns replaced with the static TONE classmap so
//   Tailwind's JIT compiles every class — same pattern as Home + Inbox +
//   Recipes + Research.
// - Drops the preview shim (`window.Memory = ...`).

import React, { useEffect, useMemo, useRef, useState } from "react";

import { api, type MemoryItem, type MemoryEvidence as Evidence, type MemorySubject as Subject } from "../../lib/api";
import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

type FreshnessBucket = "hours" | "days" | "weeks" | "months";
type ToneName = "safe" | "warning" | "blocked" | "recovered" | "quarantined" | "accent" | "neutral";

// ─────────────────────────────────────────────────────────────────────────────
// Tone classmap — same pattern as the rest of v2.
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

function freshnessOf(hours: number): FreshnessBucket {
  if (hours < 24) return "hours";
  if (hours < 24 * 7) return "days";
  if (hours < 24 * 30) return "weeks";
  return "months";
}

function confidenceTone(c: number): ToneName {
  if (c >= 85) return "safe";
  if (c >= 70) return "warning";
  return "blocked";
}

const STALE_HOURS = 30 * 24;

const SUBJECT_META: Readonly<Record<Subject, { readonly label: string; readonly tone: ToneName }>> = {
  people: { label: "People", tone: "accent" },
  accounts: { label: "Accounts", tone: "recovered" },
  preferences: { label: "Preferences", tone: "quarantined" },
  patterns: { label: "Patterns", tone: "warning" },
};

const FRESHNESS_META: Readonly<Record<FreshnessBucket, { readonly label: string }>> = {
  hours: { label: "Hours" },
  days: { label: "Days" },
  weeks: { label: "Weeks" },
  months: { label: "Months" },
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
  Search: (p: IconProps) => <Svg {...p} d={<><circle cx={11} cy={11} r={6} /><path d="m20 20-4.3-4.3" /></>} />,
  X: (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Check: (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  ChevronDown: (p: IconProps) => <Svg {...p} d={<path d="m6 9 6 6 6-6" />} />,
  ChevronUp: (p: IconProps) => <Svg {...p} d={<path d="m6 15 6-6 6 6" />} />,
  Clock: (p: IconProps) => <Svg {...p} d={<><circle cx={12} cy={12} r={9} /><path d="M12 7v5l3 2" /></>} />,
  Pencil: (p: IconProps) => <Svg {...p} d={<><path d="m4 20 4-1 11-11-3-3L5 16l-1 4Z" /></>} />,
  Trash: (p: IconProps) => <Svg {...p} d={<><path d="M4 7h16" /><path d="M9 7V4h6v3" /><path d="M6 7l1 13h10l1-13" /></>} />,
  Tag: (p: IconProps) => <Svg {...p} d={<><path d="M3 12V4h8l10 10-8 8-10-10Z" /><circle cx={8} cy={8} r={1.5} /></>} />,
  Eye: (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx={12} cy={12} r={3} /></>} />,
  AlertTriangle: (p: IconProps) => <Svg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx={12} cy={17.5} r={0.5} fill="currentColor" stroke="none" /></>} />,
  Undo: (p: IconProps) => <Svg {...p} d={<><path d="M9 14H4v-5" /><path d="M4 14a8 8 0 1 1 2.5 5.7" /></>} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  List: (p: IconProps) => <Svg {...p} d={<><path d="M8 6h13M8 12h13M8 18h13" /><circle cx={4} cy={6} r={1} fill="currentColor" stroke="none" /><circle cx={4} cy={12} r={1} fill="currentColor" stroke="none" /><circle cx={4} cy={18} r={1} fill="currentColor" stroke="none" /></>} />,
  Graph: (p: IconProps) => <Svg {...p} d={<><circle cx={6} cy={6} r={2} /><circle cx={18} cy={6} r={2} /><circle cx={12} cy={18} r={2} /><circle cx={5} cy={14} r={1.5} /><path d="m8 7 8 0M7 7l4 9M17 7l-4 9M6 8l-.5 5M12 16l-7-1" /></>} />,
  Link: (p: IconProps) => <Svg {...p} d={<><path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" /><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" /></>} />,
} as const;


// ─────────────────────────────────────────────────────────────────────────────
// Small chips
// ─────────────────────────────────────────────────────────────────────────────

function ConfidencePill({ value }: { readonly value: number }) {
  const tone = TONE[confidenceTone(value)];
  return (
    <span className={cx(
      "inline-flex items-center gap-1.5 rounded-full border font-medium tabular-nums text-[10.5px] px-1.5 py-0.5",
      tone.bg, tone.text, tone.border,
    )}>
      <span className={cx("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
      {value}% confidence
    </span>
  );
}

function SubjectChip({ subject }: { readonly subject: Subject }) {
  const m = SUBJECT_META[subject];
  const tone = TONE[m.tone];
  return (
    <span className={cx(
      "inline-flex items-center gap-1 rounded-full border text-[10.5px] font-medium px-1.5 py-0.5",
      tone.bg, tone.text, tone.border,
    )}>
      {m.label}
    </span>
  );
}

function FreshnessLabel({ item }: { readonly item: MemoryItem }) {
  const stale = item.lastTouchedHours > STALE_HOURS;
  return (
    <span className={cx(
      "inline-flex items-center gap-1 text-[11px]",
      stale ? "text-warning" : item.verified ? "text-safe" : "text-neutral-500",
    )}>
      <ICON.Clock size={11} />
      {item.lastTouchedLabel}
    </span>
  );
}

function ReVerifyPill({ item }: { readonly item: MemoryItem }) {
  if (item.lastTouchedHours <= STALE_HOURS) return null;
  const days = Math.round(item.lastTouchedHours / 24);
  return (
    <span className="inline-flex items-center gap-1 rounded-md border border-warning bg-warning text-warning text-[10.5px] font-medium px-1.5 py-0.5">
      <ICON.AlertTriangle size={11} />
      Re-verify · {days}d untouched
    </span>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Evidence trail (inline)
// ─────────────────────────────────────────────────────────────────────────────

function EvidenceTrail({ items }: { readonly items: readonly Evidence[] }) {
  return (
    <ol className="relative ml-2 pl-4 border-l border-neutral-200">
      {items.map((e, i) => (
        <li key={e.id} className={cx("relative", i === items.length - 1 ? "" : "pb-3")}>
          <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-accent-solid" />
          <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className="text-[12px] font-semibold text-neutral-800">{e.source}</span>
            <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">{e.when}</span>
          </div>
          <div className="text-[11px] font-mono text-neutral-500 truncate" title={e.ref}>{e.ref}</div>
          <blockquote className="mt-1 rounded-md bg-neutral-50 border border-neutral-100 px-2.5 py-1.5 text-[12px] text-neutral-700 leading-relaxed">
            "{e.snippet}"
          </blockquote>
        </li>
      ))}
    </ol>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Memory card
// ─────────────────────────────────────────────────────────────────────────────

interface MemoryCardProps {
  readonly item: MemoryItem;
  readonly expanded: boolean;
  readonly onToggleExpand: () => void;
  readonly onCorrect: () => void;
  readonly onForget: () => void;
  readonly onTag: () => void;
}
function MemoryCard({ item, expanded, onToggleExpand, onCorrect, onForget, onTag }: MemoryCardProps) {
  return (
    <article className="card overflow-hidden">
      <div className="px-4 py-3.5 sm:px-5 flex flex-col gap-2.5">
        <div className="flex items-start justify-between gap-3 flex-wrap">
          <div className="min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <SubjectChip subject={item.subject} />
              <span className="text-[12.5px] text-neutral-500 truncate">{item.subjectLabel}</span>
            </div>
          </div>
          <div className="flex items-center gap-2 flex-wrap shrink-0">
            <ReVerifyPill item={item} />
            <ConfidencePill value={item.confidence} />
          </div>
        </div>

        <p className="text-[14px] text-neutral-900 leading-relaxed">{item.fact}</p>

        <div className="flex items-center justify-between gap-3 flex-wrap">
          <div className="flex items-center gap-3 flex-wrap">
            <FreshnessLabel item={item} />
            <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
              <ICON.Link size={11} /> {item.evidence.length} source{item.evidence.length === 1 ? "" : "s"}
            </span>
            {item.tags.length > 0 && (
              <span className="inline-flex items-center gap-1 text-[11px] text-neutral-400">
                {item.tags.map((t) => (
                  <span key={t} className="rounded bg-neutral-100 px-1.5 py-0.5 font-mono text-[10.5px]">
                    {t}
                  </span>
                ))}
              </span>
            )}
          </div>
          <button type="button" onClick={onToggleExpand} className="inline-flex items-center gap-0.5 text-[12px] font-medium text-accent hover:text-accent-solid">
            Why do you know this?
            {expanded ? <ICON.ChevronUp size={12} /> : <ICON.ChevronDown size={12} />}
          </button>
        </div>

        {expanded && (
          <div className="mt-1 rounded-lg bg-neutral-50/60 border border-neutral-100 px-3 py-3">
            <div className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 mb-2">
              Evidence trail
            </div>
            <EvidenceTrail items={item.evidence} />
          </div>
        )}
      </div>

      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/60">
        <div className="flex items-center gap-1">
          <button type="button" onClick={onCorrect} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-neutral-900">
            <ICON.Pencil size={12} /> Correct this
          </button>
          <button type="button" onClick={onForget} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-blocked">
            <ICON.Trash size={12} /> Forget this
          </button>
          <button type="button" onClick={onTag} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-neutral-900">
            <ICON.Tag size={12} /> Tag
          </button>
        </div>
        <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">{item.id.toUpperCase()}</span>
      </footer>
    </article>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Facet pills
// ─────────────────────────────────────────────────────────────────────────────

type SubjectFilter = "all" | Subject;
type FreshnessFilter = "all" | FreshnessBucket;

interface PillProps {
  readonly active: boolean;
  readonly children: React.ReactNode;
  readonly onClick: () => void;
  readonly count?: number;
}
function Pill({ active, children, onClick, count }: PillProps) {
  return (
    <button type="button" onClick={onClick} className={cx(
      "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
      active ? "bg-neutral-900 text-white border-neutral-900" : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
    )}>
      {children}
      {typeof count === "number" && (
        <span className={cx("rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px", active ? "bg-white/20 text-white" : "bg-neutral-100 text-neutral-500")}>
          {count}
        </span>
      )}
    </button>
  );
}

function FacetRow({ label, children }: { readonly label: string; readonly children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 shrink-0 w-[68px]">
        {label}
      </span>
      <div className="flex items-center gap-1.5 flex-wrap">{children}</div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Search + view toggle
// ─────────────────────────────────────────────────────────────────────────────

interface SearchBarProps {
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly inputRef: React.RefObject<HTMLInputElement | null>;
}
function SearchBar({ value, onChange, inputRef }: SearchBarProps) {
  return (
    <div className="relative w-full max-w-2xl">
      <span className="absolute left-3 top-1/2 -translate-y-1/2 text-neutral-400">
        <ICON.Search size={15} />
      </span>
      <input
        ref={inputRef}
        type="search"
        autoComplete="off"
        spellCheck={false}
        placeholder="Search memory…"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-neutral-200 bg-white pl-9 pr-9 py-2.5 text-[13.5px] text-neutral-900 placeholder:text-neutral-400 focus:outline-none focus:border-accent-solid focus:ring-accent transition-colors"
      />
      {value && (
        <button type="button" onClick={() => onChange("")} aria-label="Clear search" className="absolute right-2 top-1/2 -translate-y-1/2 h-6 w-6 inline-flex items-center justify-center rounded-md text-neutral-400 hover:text-neutral-900 hover:bg-neutral-100">
          <ICON.X size={12} />
        </button>
      )}
    </div>
  );
}

function ViewToggle({ view, onChange }: { readonly view: "list" | "graph"; readonly onChange: (v: "list" | "graph") => void }) {
  return (
    <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
      <button type="button" onClick={() => onChange("list")} className={cx("inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors", view === "list" ? "bg-neutral-900 text-white" : "text-neutral-600 hover:text-neutral-900")}>
        <ICON.List size={12} /> List
      </button>
      <button type="button" onClick={() => onChange("graph")} className={cx("inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors", view === "graph" ? "bg-neutral-900 text-white" : "text-neutral-600 hover:text-neutral-900")}>
        <ICON.Graph size={12} /> Graph
      </button>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Empty states
// ─────────────────────────────────────────────────────────────────────────────

function EmptyState({ kind, query }: { readonly kind: "no-memory" | "no-match"; readonly query?: string }) {
  if (kind === "no-memory") {
    return (
      <div className="card card-padded">
        <div className="flex flex-col items-center text-center py-12 max-w-md mx-auto">
          <div className="h-12 w-12 rounded-full bg-accent inline-flex items-center justify-center mb-4 text-accent">
            <ICON.Sparkles size={22} />
          </div>
          <h3 className="section-title">Memory will grow as you use it</h3>
          <p className="text-[13px] text-neutral-500 mt-2 leading-relaxed">
            The system hasn't built up much yet — keep using your assistant teams and memory will grow.
          </p>
        </div>
      </div>
    );
  }
  return (
    <div className="card card-padded">
      <div className="flex flex-col items-center text-center py-10 max-w-md mx-auto">
        <div className="h-12 w-12 rounded-full bg-neutral inline-flex items-center justify-center mb-4 text-neutral-500">
          <ICON.Search size={22} />
        </div>
        <h3 className="section-title">No memory matches "{query}"</h3>
        <p className="text-[13px] text-neutral-500 mt-2 leading-relaxed">
          Try a broader query, or check the recent activity timeline on the workspace dashboard.
        </p>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Forgotten banner (30-day undo)
// ─────────────────────────────────────────────────────────────────────────────

interface ForgetBannerProps {
  readonly item: MemoryItem;
  readonly onUndo: () => void;
  readonly onDismiss: () => void;
}
function ForgetBanner({ item, onUndo, onDismiss }: ForgetBannerProps) {
  return (
    <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50 ig-toast-in">
      <div className="rounded-xl bg-neutral-900 text-white shadow-lg px-3.5 py-2.5 flex items-center gap-3 max-w-[640px]">
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-white/10 shrink-0">
          <ICON.Trash size={13} />
        </span>
        <div className="text-[12.5px] leading-snug min-w-0">
          <div className="font-medium truncate">Moved to Forgotten · {item.subjectLabel}</div>
          <div className="text-white/70 text-[11.5px]">30 days to restore from Settings → Forgotten memory.</div>
        </div>
        <button type="button" onClick={onUndo} className="inline-flex items-center gap-1 rounded-md bg-white text-neutral-900 hover:bg-white/90 px-2.5 py-1 text-[12px] font-semibold">
          <ICON.Undo size={12} /> Undo
        </button>
        <button type="button" onClick={onDismiss} aria-label="Dismiss" className="inline-flex items-center justify-center h-7 w-7 rounded-md text-white/60 hover:text-white hover:bg-white/10">
          <ICON.X size={13} />
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Graph view (placeholder per spec — DO NOT pull in d3/vis-network)
// ─────────────────────────────────────────────────────────────────────────────

function GraphView({ items }: { readonly items: readonly MemoryItem[] }) {
  const counts: Record<Subject, number> = { people: 0, accounts: 0, preferences: 0, patterns: 0 };
  for (const it of items) counts[it.subject] += 1;
  return (
    <div className="card card-padded">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Graph view</div>
          <h3 className="section-title mt-0.5">Memory network</h3>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          {(Object.keys(SUBJECT_META) as Subject[]).map((s) => {
            const m = SUBJECT_META[s];
            const tone = TONE[m.tone];
            return (
              <span key={s} className={cx("inline-flex items-center gap-1.5 rounded-full border text-[11px] font-medium px-2 py-0.5", tone.bg, tone.text, tone.border)}>
                <span className={cx("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
                {m.label}
                <span className="font-mono tabular-nums opacity-70">{counts[s]}</span>
              </span>
            );
          })}
        </div>
      </div>

      <div className="card-padded mt-4 rounded-xl border border-dashed border-neutral-200 bg-neutral-50/60 min-h-[360px] flex flex-col items-center justify-center text-center">
        <ICON.Graph size={32} className="text-neutral-300" />
        <div className="mt-3 text-[13px] font-medium text-neutral-700">Graph rendering goes here</div>
        <p className="mt-1 text-[12px] text-neutral-500 max-w-md leading-relaxed">
          Force-directed network, color-coded by subject (People / Accounts / Preferences / Patterns).
          TODO(integrator): mount a d3-force or vis-network instance against this slot.
        </p>
        <div className="mt-4 text-[10.5px] font-mono text-neutral-400">
          nodes = {items.length} · edges ≈ {Math.max(0, items.length * 2 - 4)}
        </div>
      </div>

      <p className="mt-3 text-[11.5px] text-neutral-500">
        List view remains the default — it's easier to skim, search, and edit. Switch back any time.
      </p>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route component
// ─────────────────────────────────────────────────────────────────────────────

export function Memory() {
  const [allItems, setAllItems] = useState<readonly MemoryItem[]>(() => api.v2.memory.getMock());
  const [subject, setSubject] = useState<SubjectFilter>("all");
  const [freshness, setFreshness] = useState<FreshnessFilter>("all");
  const [query, setQuery] = useState("");
  const [view, setView] = useState<"list" | "graph">("list");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [forgotten, setForgotten] = useState<readonly MemoryItem[]>([]);
  const [lastForgotten, setLastForgotten] = useState<MemoryItem | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (!toast) return undefined;
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  const subjectCounts: Record<SubjectFilter, number> = useMemo(() => {
    const out: Record<SubjectFilter, number> = {
      all: allItems.length, people: 0, accounts: 0, preferences: 0, patterns: 0,
    };
    for (const it of allItems) out[it.subject] += 1;
    return out;
  }, [allItems]);

  const freshnessCounts: Record<FreshnessFilter, number> = useMemo(() => {
    const out: Record<FreshnessFilter, number> = {
      all: allItems.length, hours: 0, days: 0, weeks: 0, months: 0,
    };
    for (const it of allItems) out[freshnessOf(it.lastTouchedHours)] += 1;
    return out;
  }, [allItems]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return allItems.filter((it) => {
      if (subject !== "all" && it.subject !== subject) return false;
      if (freshness !== "all" && freshnessOf(it.lastTouchedHours) !== freshness) return false;
      if (!q) return true;
      const hay = [
        it.subjectLabel, it.fact, it.tags.join(" "),
        ...it.evidence.flatMap((e) => [e.source, e.snippet, e.ref]),
      ].join(" ").toLowerCase();
      return hay.includes(q);
    });
  }, [allItems, subject, freshness, query]);

  const sorted = useMemo(() => {
    return [...filtered].sort((a, b) => {
      const aStale = a.lastTouchedHours > STALE_HOURS ? 1 : 0;
      const bStale = b.lastTouchedHours > STALE_HOURS ? 1 : 0;
      if (aStale !== bStale) return aStale - bStale;
      return a.lastTouchedHours - b.lastTouchedHours;
    });
  }, [filtered]);

  const handleForget = (it: MemoryItem) => {
    setAllItems((prev) => prev.filter((x) => x.id !== it.id));
    setForgotten((prev) => [it, ...prev]);
    setLastForgotten(it);
  };
  const handleUndoForget = () => {
    if (!lastForgotten) return;
    setAllItems((prev) => [lastForgotten, ...prev]);
    setForgotten((prev) => prev.filter((x) => x.id !== lastForgotten.id));
    setToast(`Restored · ${lastForgotten.subjectLabel}`);
    setLastForgotten(null);
  };
  const handleCorrect = (it: MemoryItem) => setToast(`Correct · ${it.subjectLabel}`);
  const handleTag = (it: MemoryItem) => setToast(`Add tag · ${it.subjectLabel}`);

  const previewEmpty = allItems.length === 0;

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar />

      <main className="page-container max-w-[78rem]">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="page-title">Memory</h1>
              <span className="inline-flex items-center gap-1.5 rounded-full bg-neutral px-2 py-0.5 text-[11px] text-neutral-600 font-medium">
                <span className="h-1.5 w-1.5 rounded-full bg-safe-solid" />
                {allItems.length} facts · {forgotten.length} forgotten (30d undo)
              </span>
            </div>
            <p className="mt-1 text-[13.5px] text-neutral-500 max-w-2xl leading-relaxed">
              What the system has learned about this workspace. Every fact has evidence and a trail you can inspect or correct in one click.
            </p>
          </div>
          <ViewToggle view={view} onChange={setView} />
        </div>

        <div className="mt-5 flex items-center justify-between gap-3 flex-wrap">
          <SearchBar value={query} onChange={setQuery} inputRef={inputRef} />
          <kbd className="hidden md:inline-flex items-center gap-1 rounded border border-neutral-200 bg-white text-[10.5px] font-mono text-neutral-500 px-1.5 py-0.5">
            ⌘ K
          </kbd>
        </div>

        <div className="mt-4 card card-padded flex flex-col gap-3">
          <FacetRow label="Subject">
            <Pill active={subject === "all"} onClick={() => setSubject("all")} count={subjectCounts.all}>
              All
            </Pill>
            {(Object.keys(SUBJECT_META) as Subject[]).map((s) => (
              <Pill key={s} active={subject === s} onClick={() => setSubject(s)} count={subjectCounts[s]}>
                {SUBJECT_META[s].label}
              </Pill>
            ))}
          </FacetRow>
          <FacetRow label="Freshness">
            <Pill active={freshness === "all"} onClick={() => setFreshness("all")} count={freshnessCounts.all}>
              All
            </Pill>
            {(Object.keys(FRESHNESS_META) as FreshnessBucket[]).map((f) => (
              <Pill key={f} active={freshness === f} onClick={() => setFreshness(f)} count={freshnessCounts[f]}>
                {FRESHNESS_META[f].label}
              </Pill>
            ))}
          </FacetRow>
        </div>

        <div className="mt-5">
          {view === "graph" ? (
            <GraphView items={allItems} />
          ) : previewEmpty ? (
            <EmptyState kind="no-memory" />
          ) : sorted.length === 0 ? (
            <EmptyState kind="no-match" query={query} />
          ) : (
            <ul className="flex flex-col gap-3">
              {sorted.map((it) => (
                <li key={it.id}>
                  <MemoryCard
                    item={it}
                    expanded={expandedId === it.id}
                    onToggleExpand={() => setExpandedId(expandedId === it.id ? null : it.id)}
                    onCorrect={() => handleCorrect(it)}
                    onForget={() => handleForget(it)}
                    onTag={() => handleTag(it)}
                  />
                </li>
              ))}
            </ul>
          )}
        </div>

        <footer className="mt-10 mb-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-neutral-500">
          <div className="inline-flex items-center gap-1.5">
            <ICON.Eye size={13} className="text-accent" />
            Every fact is one click from its evidence trail. Forgetting is non-destructive.
          </div>
          <a href="#forgotten" className="text-accent hover:text-accent-solid font-medium">
            Forgotten memory →
          </a>
        </footer>
      </main>

      {lastForgotten && (
        <ForgetBanner
          item={lastForgotten}
          onUndo={handleUndoForget}
          onDismiss={() => setLastForgotten(null)}
        />
      )}

      {toast && !lastForgotten && (
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
