// route: /research
// purpose: research findings with confidence, freshness, and contradiction
// awareness. Replaces the stub when VITE_ENABLE_V2_UI=true. Ported from
// Claude Design's Research.tsx (single-file route) — source lives at
// apps/web/src/_design-inbox/research/.
//
// Integration notes:
// - Shell chrome (sticky topbar) imported from `pages/v2/_shared/WorkspaceTopbar`.
// - Mock data inline; swap for `useResearchQuery()` once lib/api.ts ships.
// - Dynamic `bg-${tone}` patterns replaced with the static TONE classmap so
//   Tailwind's JIT compiles every class — same pattern as Home + Inbox + Recipes.
// - Drops the preview shim (`window.Research = ...`).

import React, { useEffect, useMemo, useState } from "react";

import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

import {
  api,
  type ResearchFinding2 as Finding,
  type ResearchTopic2 as Topic,
  type ResearchImpact as Impact,
  type ResearchAction as Action,
  type ResearchSourceKind as SourceKind,
  type ResearchAgreement as Agreement,
  type SourceSnippet,
  type ClassifierTrace,
} from "../../lib/api";

type ToneName = "safe" | "warning" | "blocked" | "recovered" | "quarantined" | "accent" | "neutral";

// ─────────────────────────────────────────────────────────────────────────────
// Tone classmap — Tailwind needs every utility class as a literal string.
// Mirrors Home + Inbox + Recipes for audit-pipeline dedup later.
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

function confidenceTone(c: number): ToneName {
  if (c >= 85) return "safe";
  if (c >= 70) return "warning";
  return "blocked";
}

function impactRank(i: Impact): number {
  return ({ low: 1, medium: 2, high: 3 } as const)[i];
}

const IMPACT_TONE: Readonly<Record<Impact, ToneName>> = {
  low: "neutral",
  medium: "warning",
  high: "blocked",
};

const AGREEMENT_TONE: Readonly<Record<Agreement, ToneName>> = {
  agrees: "safe",
  conflicts: "warning",
  neutral: "neutral",
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
  Clock: (p: IconProps) => <Svg {...p} d={<><circle cx={12} cy={12} r={9} /><path d="M12 7v5l3 2" /></>} />,
  AlertTriangle: (p: IconProps) => <Svg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx={12} cy={17.5} r={0.5} fill="currentColor" stroke="none" /></>} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  Check: (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  X: (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  ArrowLeft: (p: IconProps) => <Svg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
  Link: (p: IconProps) => <Svg {...p} d={<><path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" /><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" /></>} />,
  File: (p: IconProps) => <Svg {...p} d={<><path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8l-5-5Z" /><path d="M14 3v5h5" /></>} />,
  Activity: (p: IconProps) => <Svg {...p} d={<path d="M3 12h4l3-7 4 14 3-7h4" />} />,
  Eye: (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx={12} cy={12} r={3} /></>} />,
  Bolt: (p: IconProps) => <Svg {...p} d={<path d="M13 2 4 14h7l-1 8 9-12h-7l1-8Z" />} />,
  Cpu: (p: IconProps) => <Svg {...p} d={<><rect x={5} y={5} width={14} height={14} rx={2} /><rect x={9} y={9} width={6} height={6} rx={1} /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>} />,
  Layers: (p: IconProps) => <Svg {...p} d={<><path d="m12 3 9 5-9 5-9-5 9-5Z" /><path d="m3 13 9 5 9-5" /><path d="m3 18 9 5 9-5" /></>} />,
} as const;

const TOPIC_META: Readonly<Record<Topic, { readonly label: string; readonly tone: ToneName }>> = {
  pricing: { label: "Pricing", tone: "accent" },
  api: { label: "API changes", tone: "recovered" },
  supplier: { label: "Supplier risk", tone: "warning" },
  industry: { label: "Industry", tone: "quarantined" },
  internal: { label: "Internal", tone: "neutral" },
};

const IMPACT_META: Readonly<Record<Impact, { readonly label: string }>> = {
  low: { label: "Low impact" },
  medium: { label: "Medium impact" },
  high: { label: "High impact" },
};

const ACTION_META: Readonly<Record<Action, { readonly label: string }>> = {
  "apply-finding": { label: "Apply finding" },
  "mark-reviewed": { label: "Mark reviewed" },
  "discuss-standup": { label: "Discuss in standup" },
};

const KIND_META: Readonly<Record<SourceKind, { readonly label: string; readonly Icon: React.ComponentType<IconProps> }>> = {
  "release-notes": { label: "Release notes", Icon: ICON.File },
  index: { label: "Index", Icon: ICON.Activity },
  alert: { label: "Alert", Icon: ICON.AlertTriangle },
  paper: { label: "Paper", Icon: ICON.File },
  digest: { label: "Digest", Icon: ICON.Sparkles },
  filing: { label: "Filing", Icon: ICON.File },
  blog: { label: "Blog", Icon: ICON.Link },
};


// ─────────────────────────────────────────────────────────────────────────────
// Sub-components
// ─────────────────────────────────────────────────────────────────────────────

function ConfidencePill({ value, size = "sm" }: { readonly value: number; readonly size?: "sm" | "md" }) {
  const tone = TONE[confidenceTone(value)];
  const sizeCx = size === "sm" ? "text-[10.5px] px-1.5 py-0.5" : "text-[12px] px-2 py-0.5";
  return (
    <span className={cx("inline-flex items-center gap-1.5 rounded-full border font-medium tabular-nums", sizeCx, tone.bg, tone.text, tone.border)}>
      <span className={cx("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
      {value}% confidence
    </span>
  );
}

function ConfidenceBar({ value, size = "sm" }: { readonly value: number; readonly size?: "sm" | "md" }) {
  const tone = TONE[confidenceTone(value)];
  return (
    <div className={cx("w-full rounded-full overflow-hidden bg-neutral-100", size === "sm" ? "h-1" : "h-1.5")} aria-hidden>
      <div
        className={cx("h-full rounded-full transition-all duration-300", tone.bgSolid)}
        style={{ width: `${Math.max(2, Math.min(100, value))}%` }}
      />
    </div>
  );
}

function TopicChip({ topic, size = "sm" }: { readonly topic: Topic; readonly size?: "sm" | "md" }) {
  const m = TOPIC_META[topic];
  const tone = TONE[m.tone];
  const sizeCx = size === "sm" ? "text-[10.5px] px-1.5 py-0.5" : "text-[11px] px-2 py-0.5";
  return (
    <span className={cx("inline-flex items-center gap-1 rounded-full border font-medium", sizeCx, tone.bg, tone.text, tone.border)}>
      {m.label}
    </span>
  );
}

function ContradictionChip({ count }: { readonly count: number }) {
  if (count === 0) return null;
  return (
    <span className="inline-flex items-center gap-1 rounded-md bg-warning border border-warning text-warning text-[10.5px] font-medium px-1.5 py-0.5">
      <ICON.AlertTriangle size={11} />
      {count} conflicting source{count === 1 ? "" : "s"}
    </span>
  );
}

function FreshnessChip({ value }: { readonly value: string }) {
  return (
    <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
      <ICON.Clock size={11} />
      {value}
    </span>
  );
}

function ImpactDot({ impact }: { readonly impact: Impact }) {
  const tone = TONE[IMPACT_TONE[impact]];
  return (
    <span className="inline-flex items-center gap-1 text-[10.5px] text-neutral-500">
      <span className={cx("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
      {IMPACT_META[impact].label}
    </span>
  );
}

interface ResearchCardProps {
  readonly finding: Finding;
  readonly onWhy: () => void;
  readonly onAct: () => void;
  readonly featured?: boolean;
}
function ResearchCard({ finding, onWhy, onAct, featured = false }: ResearchCardProps) {
  const action = ACTION_META[finding.suggestedAction].label;
  const conflict = finding.contradictionCount > 0;
  return (
    <article className={cx("group card flex flex-col h-full overflow-hidden transition-shadow", featured ? "ring-1 ring-accent-border" : "hover:shadow-md")}>
      {featured && (
        <div className="bg-accent-solid text-white px-4 py-1.5 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wide">
          <ICON.Sparkles size={12} /> Top impact today
        </div>
      )}

      <div className={cx("px-4 pt-4 pb-3 flex flex-col gap-3", featured && "px-6 pt-5")}>
        <div className="flex items-center justify-between gap-2 flex-wrap">
          <ConfidencePill value={finding.confidence} size={featured ? "md" : "sm"} />
          <div className="flex items-center gap-2">
            <ImpactDot impact={finding.impact} />
            <FreshnessChip value={finding.freshness} />
          </div>
        </div>

        <div>
          <h3 className={cx("font-semibold tracking-tight text-neutral-900 leading-snug", featured ? "text-[19px]" : "text-[15px]")}>
            {finding.title}
          </h3>
          <div className="mt-2">
            <ConfidenceBar value={finding.confidence} size={featured ? "md" : "sm"} />
          </div>
        </div>

        <p className={cx("text-neutral-600 leading-relaxed", featured ? "text-[13.5px]" : "text-[12.5px]")}>
          {finding.summary}
        </p>

        <div className="flex flex-wrap items-center gap-1.5">
          <TopicChip topic={finding.topic} size={featured ? "md" : "sm"} />
          {finding.sources.length > 1 && (
            <span className="inline-flex items-center gap-1 rounded-full bg-neutral-100 px-1.5 py-0.5 text-[10.5px] font-medium text-neutral-600">
              <ICON.Layers size={10} /> {finding.sources.length} sources
            </span>
          )}
          <ContradictionChip count={finding.contradictionCount} />
        </div>
      </div>

      <div className="px-4 py-2 border-t border-neutral-100 bg-neutral-50/50 flex items-center justify-between gap-2 text-[11px]">
        <span className="inline-flex items-center gap-1 text-neutral-500 min-w-0">
          <ICON.Link size={11} className="shrink-0" />
          <span className="font-mono truncate" title={finding.primarySource}>
            {finding.primarySource}
          </span>
        </span>
        <button type="button" onClick={onWhy} className="shrink-0 inline-flex items-center gap-0.5 text-accent hover:text-accent-solid font-medium">
          Why this finding?
          <ICON.ArrowRight size={10} />
        </button>
      </div>

      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-white">
        <span className={cx("inline-flex items-center gap-1 text-[10.5px]", conflict ? "text-warning" : "text-neutral-400")}>
          {conflict ? (
            <><ICON.AlertTriangle size={11} /> Surfaced with conflict</>
          ) : (
            <><ICON.Cpu size={11} /> Surfaced by {finding.classifier.name}</>
          )}
        </span>
        <button type="button" onClick={onAct} className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-2.5 py-1.5 text-[12.5px] font-medium transition-colors">
          <ICON.Check size={12} /> {action}
        </button>
      </footer>
    </article>
  );
}

function SourceRow({ source }: { readonly source: SourceSnippet }) {
  const k = KIND_META[source.kind];
  const KIcn = k.Icon;
  const tone = TONE[AGREEMENT_TONE[source.agreement]];
  return (
    <li className={cx("px-3 py-3 flex gap-3 border-l-2", source.agreement === "conflicts" ? "border-l-warning-solid bg-warning/40" : "border-l-transparent")}>
      <span className={cx("shrink-0 h-7 w-7 rounded-md inline-flex items-center justify-center", tone.bg, tone.text)}>
        <KIcn size={13} />
      </span>
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 flex-wrap">
          <span className="text-[12.5px] font-semibold text-neutral-900 truncate">{source.name}</span>
          <span className={cx("text-[10px] font-medium uppercase tracking-wide rounded-full border px-1.5 py-0.5", tone.bg, tone.text, tone.border)}>
            {source.agreement}
          </span>
          <span className="text-[10.5px] text-neutral-400">· {k.label}</span>
          <span className="text-[10.5px] text-neutral-400 ml-auto font-mono tabular-nums">{source.publishedAt}</span>
        </div>
        <div className="text-[11.5px] font-mono text-neutral-500 truncate mt-0.5" title={source.url}>{source.url}</div>
        <blockquote className="mt-2 rounded-md bg-neutral-50 border border-neutral-100 px-3 py-2 text-[12.5px] text-neutral-700 leading-relaxed">
          "{source.snippet}"
        </blockquote>
      </div>
    </li>
  );
}

function ClassifierTraceCard({ trace }: { readonly trace: ClassifierTrace }) {
  const tone = TONE[confidenceTone(Math.round(trace.confidence * 100))];
  return (
    <div className="rounded-xl border border-neutral-200 bg-white p-4">
      <div className="flex items-center justify-between mb-3">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Why this surfaced</div>
          <div className="text-[14px] font-semibold tracking-tight text-neutral-900 mt-0.5">{trace.name}</div>
        </div>
        <ConfidencePill value={Math.round(trace.confidence * 100)} size="md" />
      </div>
      <div className="rounded-md bg-neutral-50 border border-neutral-100 px-3 py-2">
        <div className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-400">Rule</div>
        <div className="font-mono text-[12.5px] text-neutral-700 mt-0.5 leading-snug">{trace.rule}</div>
      </div>
      <div className={cx("mt-3 text-[11.5px] flex items-center gap-1.5", tone.text)}>
        <ICON.Bolt size={12} />
        Classifier confidence: <span className="font-medium tabular-nums">{Math.round(trace.confidence * 100)}%</span>
      </div>
    </div>
  );
}

function DetailDrawer({ finding, onClose }: { readonly finding: Finding; readonly onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const ordered = useMemo(() => {
    const conflicts = finding.sources.filter((s) => s.agreement === "conflicts");
    const rest = finding.sources.filter((s) => s.agreement !== "conflicts");
    return [...conflicts, ...rest];
  }, [finding.sources]);

  return (
    <div className="fixed inset-0 z-40 flex justify-end" role="dialog" aria-modal="true" aria-label={`Inspect ${finding.title}`}>
      <div className="ig-drawer-backdrop absolute inset-0" onClick={onClose} />
      <aside className="relative h-full w-full max-w-[680px] bg-white shadow-xl border-l border-neutral-200 flex flex-col ig-slide-in">
        <header className="sticky top-0 z-10 bg-white/90 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
          <button type="button" onClick={onClose} className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
            <ICON.ArrowLeft size={13} /> Back
          </button>
          <span className="text-[11px] font-mono text-neutral-400">{finding.id.toUpperCase()}</span>
          <span className="text-neutral-300">·</span>
          <TopicChip topic={finding.topic} />
          <div className="ml-auto flex items-center gap-2">
            <ConfidencePill value={finding.confidence} />
            <button type="button" onClick={onClose} aria-label="Close drawer" className="h-7 w-7 inline-flex items-center justify-center rounded-md text-neutral-500 hover:text-neutral-900 hover:bg-neutral-50">
              <ICON.X size={14} />
            </button>
          </div>
        </header>

        <div className="flex-1 overflow-y-auto scrollbar-thin">
          <div className="px-6 py-5">
            <h1 className="text-[20px] font-semibold tracking-tight text-neutral-900 leading-tight">{finding.title}</h1>
            <div className="mt-3">
              <ConfidenceBar value={finding.confidence} size="md" />
            </div>

            <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1.5">
              <ImpactDot impact={finding.impact} />
              <FreshnessChip value={finding.freshness} />
              <span className="text-neutral-300">·</span>
              <span className="inline-flex items-center gap-1 text-[11.5px] text-neutral-500">
                <ICON.Layers size={11} /> {finding.sources.length} source{finding.sources.length === 1 ? "" : "s"}
              </span>
              <ContradictionChip count={finding.contradictionCount} />
            </div>

            <p className="mt-4 text-[13.5px] text-neutral-700 leading-relaxed">{finding.summary}</p>

            {finding.contradictionCount > 0 && (
              <div className="mt-4 rounded-xl border border-warning bg-warning p-4">
                <div className="flex items-start gap-3">
                  <span className="h-7 w-7 shrink-0 rounded-md bg-warning-solid text-white inline-flex items-center justify-center">
                    <ICON.AlertTriangle size={14} />
                  </span>
                  <div className="min-w-0">
                    <div className="text-[11px] font-medium uppercase tracking-wide text-warning">Sources disagree</div>
                    <div className="mt-1 text-[13px] text-warning leading-relaxed">
                      {finding.contradictionCount} of {finding.sources.length} source{finding.sources.length === 1 ? "" : "s"} contradict the headline finding.{" "}
                      Conflicting source{finding.contradictionCount === 1 ? " is" : "s are"} pinned to the top of the evidence below.
                    </div>
                  </div>
                </div>
              </div>
            )}

            <section className="mt-5">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Evidence</h2>
                <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">
                  {finding.sources.length} source{finding.sources.length === 1 ? "" : "s"}
                </span>
              </div>
              <ul className="divide-y divide-neutral-100 rounded-xl border border-neutral-200 bg-white overflow-hidden">
                {ordered.map((s) => <SourceRow key={s.id} source={s} />)}
              </ul>
            </section>

            <section className="mt-5 pb-6">
              <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">Classifier</h2>
              <ClassifierTraceCard trace={finding.classifier} />
            </section>
          </div>
        </div>

        <footer className="sticky bottom-0 bg-white/95 backdrop-blur border-t border-neutral-100 px-5 py-3 flex items-center justify-between gap-2">
          <span className="text-[11px] text-neutral-500">
            Suggested: <span className="text-neutral-800 font-medium">{ACTION_META[finding.suggestedAction].label}</span>
          </span>
          <div className="flex items-center gap-2">
            <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium transition-colors">
              <ICON.Eye size={12} /> Mark reviewed
            </button>
            <button type="button" className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-3 py-1.5 text-[12.5px] font-medium transition-colors">
              <ICON.Check size={12} /> {ACTION_META[finding.suggestedAction].label}
            </button>
          </div>
        </footer>
      </aside>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Filter / sort controls
// ─────────────────────────────────────────────────────────────────────────────

type SortKey = "recent" | "impact" | "confidence";
type TopicFilter = "all" | Topic;

function SortPicker({ value, onChange }: { readonly value: SortKey; readonly onChange: (v: SortKey) => void }) {
  const opts: ReadonlyArray<{ readonly id: SortKey; readonly label: string }> = [
    { id: "recent", label: "Most recent" },
    { id: "impact", label: "Highest impact" },
    { id: "confidence", label: "Highest confidence" },
  ];
  return (
    <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
      {opts.map((o) => (
        <button
          key={o.id}
          type="button"
          onClick={() => onChange(o.id)}
          className={cx(
            "px-2.5 py-1 rounded-md text-[12px] font-medium transition-colors",
            value === o.id ? "bg-neutral-900 text-white" : "text-neutral-600 hover:text-neutral-900",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

interface TopicChipsProps {
  readonly value: TopicFilter;
  readonly counts: Record<TopicFilter, number>;
  readonly onChange: (v: TopicFilter) => void;
}
function TopicChips({ value, counts, onChange }: TopicChipsProps) {
  const all: ReadonlyArray<{ readonly id: TopicFilter; readonly label: string }> = [
    { id: "all", label: "All" },
    ...(Object.keys(TOPIC_META) as Topic[]).map((t) => ({ id: t, label: TOPIC_META[t].label })),
  ];
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {all.map((t) => {
        const isActive = value === t.id;
        const n = counts[t.id] ?? 0;
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onChange(t.id)}
            className={cx(
              "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
              isActive ? "bg-neutral-900 text-white border-neutral-900" : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
            )}
          >
            {t.label}
            <span className={cx("rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px", isActive ? "bg-white/20 text-white" : "bg-neutral-100 text-neutral-500")}>
              {n}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function HideLowToggle({ value, onChange }: { readonly value: boolean; readonly onChange: (v: boolean) => void }) {
  return (
    <label className="inline-flex items-center gap-2 text-[12px] text-neutral-600 cursor-pointer select-none">
      <span className={cx("relative inline-flex h-4 w-7 items-center rounded-full transition-colors", value ? "bg-accent-solid" : "bg-neutral-200")}>
        <span className={cx("absolute h-3 w-3 bg-white rounded-full shadow transition-all", value ? "left-[14px]" : "left-[2px]")} />
      </span>
      <input type="checkbox" checked={value} onChange={(e) => onChange(e.target.checked)} className="sr-only" />
      Hide low-confidence (&lt;70%)
    </label>
  );
}

function EmptyState() {
  return (
    <div className="card card-padded">
      <div className="flex flex-col items-center text-center py-12 max-w-md mx-auto">
        <div className="h-12 w-12 rounded-full bg-safe inline-flex items-center justify-center mb-4 text-safe">
          <ICON.Sparkles size={22} />
        </div>
        <h3 className="section-title">No new findings</h3>
        <p className="text-[13px] text-neutral-500 mt-2 leading-relaxed">
          The research team is monitoring <span className="text-neutral-700 font-medium tabular-nums">{SOURCES_MONITORED} sources</span>.{" "}
          You'll see findings here whenever something materially new comes up.
        </p>
        <div className="mt-4 inline-flex items-center gap-1.5 text-[11px] text-neutral-400">
          <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
          Last source check: 2 minutes ago
        </div>
      </div>
    </div>
  );
}

function ResearchHeader({ surfaced, archived }: { readonly surfaced: number; readonly archived: number }) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div className="flex items-center gap-2">
          <h1 className="page-title">Research</h1>
          <span className="inline-flex items-center gap-1.5 rounded-full bg-accent px-2 py-0.5 text-[11px] text-accent font-medium">
            <ICON.Sparkles size={11} />
            {surfaced} new findings
          </span>
        </div>
        <p className="mt-1 text-[13.5px] text-neutral-500 max-w-2xl leading-relaxed">
          What the research team found in external sources that matters to this workspace.{" "}
          <span className="text-neutral-700">{archived.toLocaleString()}</span>{" "}
          other items checked today were quietly archived because nothing changed.
        </p>
      </div>
      <div className="inline-flex items-center gap-1.5 text-[11.5px] text-neutral-500">
        <ICON.Activity size={12} className="text-safe" />
        Watching <span className="text-neutral-700 font-medium tabular-nums">{SOURCES_MONITORED}</span> sources
      </div>
    </div>
  );
}

interface FilterBarProps {
  readonly sort: SortKey;
  readonly topic: TopicFilter;
  readonly hideLow: boolean;
  readonly counts: Record<TopicFilter, number>;
  readonly onSort: (s: SortKey) => void;
  readonly onTopic: (t: TopicFilter) => void;
  readonly onHideLow: (v: boolean) => void;
}
function FilterBar({ sort, topic, hideLow, counts, onSort, onTopic, onHideLow }: FilterBarProps) {
  return (
    <div className="card card-padded flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-2">
          <span className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Sort by</span>
          <SortPicker value={sort} onChange={onSort} />
        </div>
        <HideLowToggle value={hideLow} onChange={onHideLow} />
      </div>
      <div>
        <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">Topic</div>
        <TopicChips value={topic} counts={counts} onChange={onTopic} />
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route component
// ─────────────────────────────────────────────────────────────────────────────

const RESEARCH_MOCK = api.v2.research.getMock();
const QUIETLY_ARCHIVED_TODAY = RESEARCH_MOCK.quietlyArchivedToday;
const SOURCES_MONITORED = RESEARCH_MOCK.sourcesMonitored;

export function Research() {
  const [findings] = useState<readonly Finding[]>(RESEARCH_MOCK.findings);
  const [sort, setSort] = useState<SortKey>("recent");
  const [topic, setTopic] = useState<TopicFilter>("all");
  const [hideLow, setHideLow] = useState(false);
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
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  const counts: Record<TopicFilter, number> = useMemo(() => {
    const out: Record<TopicFilter, number> = {
      all: findings.length,
      pricing: 0, api: 0, supplier: 0, industry: 0, internal: 0,
    };
    for (const f of findings) out[f.topic] += 1;
    return out;
  }, [findings]);

  const featured = useMemo(() => {
    const sorted = [...findings].sort((a, b) => {
      const i = impactRank(b.impact) - impactRank(a.impact);
      if (i !== 0) return i;
      return a.freshnessMinutes - b.freshnessMinutes;
    });
    return sorted[0] ?? null;
  }, [findings]);

  const rest = useMemo(() => {
    const out = findings.filter((f) => f.id !== featured?.id);
    if (topic !== "all") return out.filter((f) => f.topic === topic);
    return out;
  }, [findings, featured, topic]);

  const filtered = useMemo(() => {
    let xs = rest;
    if (hideLow) xs = xs.filter((f) => f.confidence >= 70);
    const sorted = [...xs];
    sorted.sort((a, b) => {
      if (sort === "recent") return a.freshnessMinutes - b.freshnessMinutes;
      if (sort === "impact") return impactRank(b.impact) - impactRank(a.impact);
      if (sort === "confidence") return b.confidence - a.confidence;
      return 0;
    });
    return sorted;
  }, [rest, sort, hideLow]);

  const showFeatured = featured && topic === "all" && (!hideLow || featured.confidence >= 70);
  const totalShown = filtered.length + (showFeatured ? 1 : 0);
  const opened = useMemo(() => findings.find((f) => f.id === openId) ?? null, [findings, openId]);

  const handleAct = (f: Finding) => {
    setToast(`${ACTION_META[f.suggestedAction].label} · ${f.title.slice(0, 56)}${f.title.length > 56 ? "…" : ""}`);
  };

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar />

      <main className="page-container max-w-[78rem]">
        <ResearchHeader surfaced={findings.length} archived={QUIETLY_ARCHIVED_TODAY} />

        <div className="mt-5">
          <FilterBar
            sort={sort} topic={topic} hideLow={hideLow}
            counts={counts}
            onSort={setSort} onTopic={setTopic} onHideLow={setHideLow}
          />
        </div>

        <div className="mt-5">
          {totalShown === 0 ? (
            <EmptyState />
          ) : (
            <>
              {showFeatured && featured && (
                <div className="mb-4">
                  <ResearchCard
                    finding={featured}
                    featured
                    onWhy={() => setOpenId(featured.id)}
                    onAct={() => handleAct(featured)}
                  />
                </div>
              )}
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {filtered.map((f) => (
                  <ResearchCard
                    key={f.id}
                    finding={f}
                    onWhy={() => setOpenId(f.id)}
                    onAct={() => handleAct(f)}
                  />
                ))}
              </div>
            </>
          )}
        </div>

        <footer className="mt-10 mb-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-neutral-500">
          <div className="inline-flex items-center gap-1.5">
            <ICON.Eye size={13} className="text-accent" />
            Every finding can be traced back to its sources and the classifier that flagged it.
          </div>
          <a href="#sources" className="text-accent hover:text-accent-solid font-medium">
            Manage sources →
          </a>
        </footer>
      </main>

      {opened && <DetailDrawer finding={opened} onClose={() => setOpenId(null)} />}

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
