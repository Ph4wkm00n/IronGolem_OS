// route: /security
// purpose: five-layer safety model + audit trail + editable policy library.
// Replaces the stub when VITE_ENABLE_V2_UI=true. Ported from Claude Design's
// Security.tsx — source at apps/web/src/_design-inbox/security/.
//
// Integration notes:
// - Shell chrome from `pages/v2/_shared/WorkspaceTopbar`.
// - Mock data inline; swap for `useSecurityQuery()` once lib/api.ts ships.
// - Dynamic `bg-${tone}` patterns replaced with the static TONE classmap.
// - Drops the preview shim (`window.Security = ...`).

import React, { useEffect, useMemo, useState } from "react";

import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

import {
  api,
  type SecurityLayer as Layer,
  type LayerId,
  type LayerState,
  type SecurityScope as Scope,
  type SecurityAuditEntry as AuditEntry,
  type SecurityAuditKind as AuditKind,
  type SecurityPolicy as Policy,
  type PolicyState,
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

const LAYER_TONE: Readonly<Record<LayerState, ToneName>> = {
  ok: "safe", watching: "warning", paused: "neutral", failed: "blocked",
};
const LAYER_LABEL: Readonly<Record<LayerState, string>> = {
  ok: "OK", watching: "Watching", paused: "Paused", failed: "Failed",
};
const SCOPE_TONE: Readonly<Record<Scope, ToneName>> = {
  scoped: "safe", broad: "warning", restricted: "quarantined",
};
const POLICY_TONE: Readonly<Record<PolicyState, ToneName>> = {
  active: "safe", paused: "neutral", "under-review": "warning",
};
const KIND_TONE: Readonly<Record<AuditKind, ToneName>> = {
  blocked: "blocked", quarantined: "quarantined",
};

function hoursAgo(label: string): number {
  if (label.endsWith("h ago")) return parseInt(label, 10);
  if (label === "yesterday") return 24;
  if (label.endsWith("d ago")) return parseInt(label, 10) * 24;
  return 9999;
}

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
  Shield: (p: IconProps) => <Svg {...p} d={<path d="M12 3 4 6v6c0 4.5 3.4 8.4 8 9 4.6-.6 8-4.5 8-9V6l-8-3Z" />} />,
  Lock: (p: IconProps) => <Svg {...p} d={<><rect x={5} y={11} width={14} height={9} rx={2} /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></>} />,
  X: (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Check: (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  ArrowLeft: (p: IconProps) => <Svg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
  Pencil: (p: IconProps) => <Svg {...p} d={<><path d="m4 20 4-1 11-11-3-3L5 16l-1 4Z" /></>} />,
  Pause: (p: IconProps) => <Svg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  Beaker: (p: IconProps) => <Svg {...p} d={<><path d="M9 3v6L4 19a2 2 0 0 0 2 3h12a2 2 0 0 0 2-3l-5-10V3" /><path d="M9 3h6" /></>} />,
  Clock: (p: IconProps) => <Svg {...p} d={<><circle cx={12} cy={12} r={9} /><path d="M12 7v5l3 2" /></>} />,
  Eye: (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx={12} cy={12} r={3} /></>} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  Undo: (p: IconProps) => <Svg {...p} d={<><path d="M9 14H4v-5" /><path d="M4 14a8 8 0 1 1 2.5 5.7" /></>} />,
} as const;


// ─────────────────────────────────────────────────────────────────────────────
// Chips
// ─────────────────────────────────────────────────────────────────────────────

interface ChipProps {
  readonly tone: ToneName;
  readonly children: React.ReactNode;
  readonly dot?: boolean;
}
function Chip({ tone, children, dot = false }: ChipProps) {
  const t = TONE[tone];
  return (
    <span className={cx("inline-flex items-center gap-1.5 rounded-full border text-[10.5px] font-medium px-1.5 py-0.5", t.bg, t.text, t.border)}>
      {dot && <span className={cx("h-1.5 w-1.5 rounded-full", t.bgSolid)} />}
      {children}
    </span>
  );
}

function ScopeChip({ scope }: { readonly scope: Scope }) {
  return <Chip tone={SCOPE_TONE[scope]}>{scope}</Chip>;
}

function StatusMark({ kind }: { readonly kind: AuditKind }) {
  const tone = TONE[KIND_TONE[kind]];
  return (
    <span className={cx("inline-flex h-7 w-7 items-center justify-center rounded-md shrink-0", tone.bg, tone.text)}>
      {kind === "blocked" ? <ICON.X size={13} /> : <ICON.Lock size={12} />}
    </span>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Header
// ─────────────────────────────────────────────────────────────────────────────

function SecurityHeader({ layers }: { readonly layers: readonly Layer[] }) {
  const okCount = layers.filter((l) => l.state === "ok").length;
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div className="flex items-center gap-2 flex-wrap">
          <h1 className="page-title">Security</h1>
          <Chip tone="safe" dot>Five layers, all active</Chip>
        </div>
        <p className="mt-1 text-[13.5px] text-neutral-500 max-w-2xl leading-relaxed">
          {okCount === 5
            ? "All five safety layers are running normally. Below: what they're governing, what got blocked, and the rules you can adjust."
            : "Most safety layers are running normally; one is being watched. Below: what they're governing, what got blocked, and the rules you can adjust."}
        </p>
      </div>
      <div className="inline-flex items-center gap-1.5 text-[11.5px] text-neutral-500">
        <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
        Heartbeat green for <span className="font-mono tabular-nums text-neutral-700">17 days</span>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Five-layer card
// ─────────────────────────────────────────────────────────────────────────────

function LayerRow({ layer }: { readonly layer: Layer }) {
  const tone = TONE[LAYER_TONE[layer.state]];
  return (
    <li className="px-4 py-3.5 sm:px-5 sm:py-4 flex items-start gap-4">
      <div className="shrink-0">
        <div className={cx("h-8 w-8 rounded-md inline-flex items-center justify-center font-mono tabular-nums text-[12px] font-semibold", tone.bg, tone.text)}>
          {layer.id}
        </div>
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <h3 className="text-[14px] font-semibold tracking-tight text-neutral-900">
            Layer {layer.id} — {layer.name}
          </h3>
          <Chip tone={LAYER_TONE[layer.state]} dot>{LAYER_LABEL[layer.state]}</Chip>
          <span className="text-[10.5px] text-neutral-500 font-mono tabular-nums">
            governs {layer.governs} action{layer.governs === 1 ? "" : "s"}
          </span>
        </div>
        <p className="mt-1 text-[12.5px] text-neutral-600 leading-relaxed">{layer.blurb}</p>
        <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
          {layer.examples.map((ex) => (
            <span key={ex} className="rounded bg-neutral-100 px-1.5 py-0.5 text-[10.5px] text-neutral-600 font-mono">
              {ex}
            </span>
          ))}
        </div>
      </div>
    </li>
  );
}

function FiveLayerCard({ layers }: { readonly layers: readonly Layer[] }) {
  return (
    <section className="card overflow-hidden">
      <header className="px-5 py-4 border-b border-neutral-100 flex items-center justify-between gap-3 flex-wrap">
        <div className="flex items-center gap-2">
          <span className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-accent text-accent">
            <ICON.Shield size={14} />
          </span>
          <div>
            <h2 className="section-title">Five layers, all active</h2>
            <p className="text-[12px] text-neutral-500 mt-0.5">
              How every action is checked, from the operator down to the outcome.
            </p>
          </div>
        </div>
      </header>
      <ol className="divide-y divide-neutral-100">
        {layers.map((l) => <LayerRow key={l.id} layer={l} />)}
      </ol>
    </section>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Audit log
// ─────────────────────────────────────────────────────────────────────────────

type AuditFilter = "all" | "blocked" | "quarantined" | "by-me" | "24h" | "7d";

interface AuditFilterChipsProps {
  readonly value: AuditFilter;
  readonly counts: Record<AuditFilter, number>;
  readonly onChange: (v: AuditFilter) => void;
}
function AuditFilterChips({ value, counts, onChange }: AuditFilterChipsProps) {
  const opts: ReadonlyArray<{ readonly id: AuditFilter; readonly label: string }> = [
    { id: "all", label: "All" },
    { id: "blocked", label: "Blocked" },
    { id: "quarantined", label: "Quarantined" },
    { id: "by-me", label: "Denied by me" },
    { id: "24h", label: "Last 24h" },
    { id: "7d", label: "Last 7d" },
  ];
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {opts.map((o) => {
        const active = value === o.id;
        return (
          <button
            key={o.id}
            type="button"
            onClick={() => onChange(o.id)}
            className={cx(
              "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
              active ? "bg-neutral-900 text-white border-neutral-900" : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
            )}
          >
            {o.label}
            <span className={cx("rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px", active ? "bg-white/20 text-white" : "bg-neutral-100 text-neutral-500")}>
              {counts[o.id]}
            </span>
          </button>
        );
      })}
    </div>
  );
}

interface AuditRowProps {
  readonly e: AuditEntry;
  readonly onOpenRule: (ruleId: string) => void;
}
function AuditRow({ e, onOpenRule }: AuditRowProps) {
  return (
    <li className="px-4 py-3.5 sm:px-5 flex items-start gap-3">
      <StatusMark kind={e.kind} />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 flex-wrap">
          <Chip tone={KIND_TONE[e.kind]}>{e.kind}</Chip>
          <h3 className="text-[13.5px] font-semibold text-neutral-900 leading-snug">{e.title}</h3>
          <span className="text-[10.5px] font-mono text-neutral-400">· {e.whenIso}</span>
          {e.deniedBy === "you" && (
            <span className="text-[10.5px] inline-flex items-center gap-1 text-accent">
              <ICON.Eye size={10} /> Denied by you
            </span>
          )}
        </div>

        <p className="mt-1 text-[12.5px] text-neutral-700 leading-relaxed">
          <span className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 mr-1.5">Cause</span>
          {e.cause}
        </p>

        <div className="mt-2 flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1 text-[10.5px] text-neutral-500">
            <ICON.Lock size={10} />
            <span className="font-mono">{e.permission}</span>
          </span>
          <ScopeChip scope={e.scope} />
          <Chip tone="neutral">Layer {e.layer}</Chip>
          <button
            type="button"
            onClick={() => onOpenRule(e.ruleId)}
            className="ml-auto inline-flex items-center gap-0.5 text-[12px] font-medium text-accent hover:text-accent-solid"
          >
            Open rule that caught this
            <ICON.ArrowRight size={11} />
          </button>
        </div>
      </div>
    </li>
  );
}

interface AuditLogProps {
  readonly entries: readonly AuditEntry[];
  readonly filter: AuditFilter;
  readonly onFilter: (f: AuditFilter) => void;
  readonly counts: Record<AuditFilter, number>;
  readonly onOpenRule: (ruleId: string) => void;
  readonly totalEntries: number;
}
function AuditLog({ entries, filter, onFilter, counts, onOpenRule, totalEntries }: AuditLogProps) {
  return (
    <section className="card overflow-hidden">
      <header className="px-5 py-4 border-b border-neutral-100 flex flex-col gap-3">
        <div className="flex items-end justify-between gap-3 flex-wrap">
          <div>
            <h2 className="section-title">Audit log</h2>
            <p className="text-[12.5px] text-neutral-500 mt-1">
              Everything that was blocked or quarantined. Each entry shows the cause and the rule that caught it.
            </p>
          </div>
          <span className="text-[11px] font-mono text-neutral-400 tabular-nums">
            {entries.length} of {totalEntries} entries
          </span>
        </div>
        <AuditFilterChips value={filter} counts={counts} onChange={onFilter} />
      </header>

      {entries.length === 0 ? (
        <div className="p-8 text-center">
          <div className="mx-auto h-10 w-10 rounded-full bg-safe inline-flex items-center justify-center text-safe">
            <ICON.Sparkles size={18} />
          </div>
          <h3 className="mt-3 text-[14px] font-semibold text-neutral-900">No safety rules have triggered in the last 24 hours.</h3>
          <p className="text-[12.5px] text-neutral-500 mt-1">Heartbeat green for 17 days.</p>
        </div>
      ) : (
        <ol className="divide-y divide-neutral-100">
          {entries.map((e) => (
            <AuditRow key={e.id} e={e} onOpenRule={onOpenRule} />
          ))}
        </ol>
      )}
    </section>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Policy library
// ─────────────────────────────────────────────────────────────────────────────

interface PolicyCardItemProps {
  readonly p: Policy;
  readonly onEdit: () => void;
  readonly onPause: () => void;
  readonly onTest: () => void;
}
function PolicyCardItem({ p, onEdit, onPause, onTest }: PolicyCardItemProps) {
  const stateLabel = p.state === "under-review" ? "Under review" : p.state[0]!.toUpperCase() + p.state.slice(1);
  return (
    <article className="card overflow-hidden flex flex-col">
      <div className="px-4 py-3 flex flex-col gap-2">
        <div className="flex items-start justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-1.5 flex-wrap">
            <Chip tone="neutral">Layer {p.layer}</Chip>
            <Chip tone={POLICY_TONE[p.state]} dot>{stateLabel}</Chip>
          </div>
          <span className="text-[10.5px] font-mono text-neutral-400 tabular-nums">
            {p.triggeredLast30d} triggers / 30d
          </span>
        </div>
        <h3 className="text-[14px] font-semibold tracking-tight text-neutral-900 leading-snug">{p.name}</h3>
        <p className="text-[12.5px] text-neutral-600 leading-relaxed">{p.purpose}</p>
      </div>
      <footer className="mt-auto border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/60">
        <button type="button" onClick={onEdit} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-neutral-900">
          <ICON.Pencil size={11} /> Edit
        </button>
        <button type="button" onClick={onPause} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
          <ICON.Pause size={11} />
          {p.state === "paused" ? "Resume" : "Pause"}
        </button>
        <button type="button" onClick={onTest} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
          <ICON.Beaker size={11} /> Test
        </button>
      </footer>
    </article>
  );
}

interface PolicyLibraryProps {
  readonly policies: readonly Policy[];
  readonly onOpenPolicy: (id: string, focus?: "editor" | "test") => void;
  readonly onPausePolicy: (id: string) => void;
}
function PolicyLibrary({ policies, onOpenPolicy, onPausePolicy }: PolicyLibraryProps) {
  return (
    <section>
      <header className="flex items-end justify-between gap-3 flex-wrap mb-3">
        <div>
          <h2 className="section-title">Policy library</h2>
          <p className="text-[12.5px] text-neutral-500 mt-1">
            The rules above. Edit happens in a drawer with rule history and a test against recent audit events. Pause is preferred over delete.
          </p>
        </div>
      </header>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        {policies.map((p) => (
          <PolicyCardItem
            key={p.id}
            p={p}
            onEdit={() => onOpenPolicy(p.id, "editor")}
            onPause={() => onPausePolicy(p.id)}
            onTest={() => onOpenPolicy(p.id, "test")}
          />
        ))}
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Policy drawer
// ─────────────────────────────────────────────────────────────────────────────

interface PolicyDrawerProps {
  readonly policy: Policy;
  readonly focus: "editor" | "test";
  readonly onClose: () => void;
  readonly onPause: () => void;
  readonly audit: readonly AuditEntry[];
}
function PolicyDrawer({ policy, focus, onClose, onPause, audit }: PolicyDrawerProps) {
  const [tab, setTab] = useState<"editor" | "history" | "test">(focus);
  const [text, setText] = useState(policy.ruleText);

  useEffect(() => {
    setText(policy.ruleText);
  }, [policy.id, policy.ruleText]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const related = useMemo(() => audit.filter((a) => a.ruleId === policy.id), [audit, policy.id]);
  const stateLabel = policy.state === "under-review" ? "Under review" : policy.state[0]!.toUpperCase() + policy.state.slice(1);

  return (
    <div className="fixed inset-0 z-40 flex justify-end" role="dialog" aria-modal="true" aria-label={`Edit ${policy.name}`}>
      <div className="ig-drawer-backdrop absolute inset-0" onClick={onClose} />
      <aside className="relative h-full w-full max-w-[720px] bg-white shadow-xl border-l border-neutral-200 flex flex-col ig-slide-in">
        <header className="sticky top-0 z-10 bg-white/95 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
          <button type="button" onClick={onClose} className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
            <ICON.ArrowLeft size={13} /> Back
          </button>
          <span className="text-[11px] font-mono text-neutral-400">{policy.id.toUpperCase()}</span>
          <span className="text-neutral-300">·</span>
          <Chip tone="neutral">Layer {policy.layer}</Chip>
          <Chip tone={POLICY_TONE[policy.state]} dot>{stateLabel}</Chip>
          <button type="button" onClick={onClose} aria-label="Close" className="ml-auto h-7 w-7 inline-flex items-center justify-center rounded-md text-neutral-500 hover:text-neutral-900 hover:bg-neutral-50">
            <ICON.X size={14} />
          </button>
        </header>

        <div className="flex-1 overflow-y-auto scrollbar-thin">
          <div className="px-6 py-5">
            <h1 className="text-[20px] font-semibold tracking-tight text-neutral-900 leading-tight">{policy.name}</h1>
            <p className="mt-2 text-[13.5px] text-neutral-700 leading-relaxed">{policy.purpose}</p>
            <div className="mt-3 text-[11px] text-neutral-500 font-mono tabular-nums">
              {policy.triggeredLast30d} triggers in the last 30 days · {related.length} related audit entries
            </div>
          </div>

          <div className="px-6 border-b border-neutral-100">
            <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
              {(["editor", "history", "test"] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setTab(t)}
                  className={cx(
                    "px-2.5 py-1 rounded-md text-[12px] font-medium transition-colors capitalize",
                    tab === t ? "bg-neutral-900 text-white" : "text-neutral-600 hover:text-neutral-900",
                  )}
                >
                  {t === "editor" ? "Editor" : t === "history" ? "History" : "Test"}
                </button>
              ))}
            </div>
          </div>

          <div className="px-6 py-5 flex flex-col gap-4">
            {tab === "editor" && (
              <>
                <div>
                  <div className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 mb-1">Rule</div>
                  <textarea
                    value={text}
                    onChange={(e) => setText(e.target.value)}
                    spellCheck={false}
                    className="w-full min-h-[140px] rounded-lg border border-neutral-200 bg-neutral-50 font-mono text-[12.5px] text-neutral-800 leading-relaxed p-3 focus:outline-none focus:border-accent-solid"
                  />
                  <div className="mt-2 text-[11px] text-neutral-500">
                    Reversibility: changes here apply immediately, and the previous version is held for 30 days. Use <strong>Pause</strong> instead of deleting.
                  </div>
                </div>
                <div className="flex items-center justify-between gap-2 flex-wrap">
                  <button type="button" onClick={onPause} className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
                    <ICON.Pause size={12} />
                    {policy.state === "paused" ? "Resume policy" : "Pause policy"}
                  </button>
                  <div className="flex items-center gap-2">
                    <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
                      Reword
                    </button>
                    <button type="button" className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-3 py-1.5 text-[12.5px] font-medium">
                      <ICON.Check size={12} /> Save changes
                    </button>
                  </div>
                </div>
              </>
            )}

            {tab === "history" && (
              <ol className="relative ml-2 pl-4 border-l border-neutral-200">
                {policy.history.map((h, i) => (
                  <li key={i} className={cx("relative", i === policy.history.length - 1 ? "" : "pb-3")}>
                    <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-accent-solid" />
                    <div className="text-[12px] font-mono text-neutral-400 tabular-nums">{h.when}</div>
                    <div className="text-[13px] text-neutral-800 leading-snug">{h.what}</div>
                  </li>
                ))}
              </ol>
            )}

            {tab === "test" && (
              <div>
                <div className="rounded-lg border border-accent bg-accent text-accent px-3 py-2 text-[12.5px]">
                  <strong className="font-semibold">Test result —</strong> If we replayed the last 30 days against this rule as written, {related.length} audit event{related.length === 1 ? "" : "s"} would have matched.
                </div>
                <div className="mt-4 text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 mb-2">Related audit entries</div>
                {related.length === 0 ? (
                  <div className="text-[12.5px] text-neutral-500">No audit entries match this rule in the current window.</div>
                ) : (
                  <ul className="divide-y divide-neutral-100 rounded-xl border border-neutral-200 bg-white overflow-hidden">
                    {related.map((a) => (
                      <li key={a.id} className="px-3 py-2.5 flex items-start gap-2">
                        <StatusMark kind={a.kind} />
                        <div className="min-w-0">
                          <div className="text-[12.5px] font-semibold text-neutral-900 truncate">{a.title}</div>
                          <div className="text-[11.5px] text-neutral-500 leading-snug">{a.cause}</div>
                          <div className="mt-1 text-[10.5px] font-mono text-neutral-400">{a.whenIso}</div>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}
          </div>
        </div>

        <footer className="sticky bottom-0 bg-white/95 backdrop-blur border-t border-neutral-100 px-5 py-3 flex items-center justify-between gap-2">
          <span className="text-[11px] text-neutral-500 inline-flex items-center gap-1">
            <ICON.Undo size={12} className="text-accent" />
            30-day undo on every change.
          </span>
          <button type="button" onClick={onClose} className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
            Close
          </button>
        </footer>
      </aside>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route component
// ─────────────────────────────────────────────────────────────────────────────

export function Security() {
  const SECURITY_MOCK = useMemo(() => api.v2.security.getMock(), []);
  const [layers] = useState<readonly Layer[]>(SECURITY_MOCK.layers);
  const [audit] = useState<readonly AuditEntry[]>(SECURITY_MOCK.audit);
  const [policies, setPolicies] = useState<Policy[]>(() => SECURITY_MOCK.policies.map((p) => ({ ...p })));

  const [filter, setFilter] = useState<AuditFilter>("all");
  const [openPolicy, setOpenPolicy] = useState<{ id: string; focus: "editor" | "test" } | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  useEffect(() => {
    if (!openPolicy) return undefined;
    document.documentElement.style.overflow = "hidden";
    return () => {
      document.documentElement.style.overflow = "";
    };
  }, [openPolicy]);

  useEffect(() => {
    if (!toast) return undefined;
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  const filtered = useMemo(() => {
    return audit.filter((e) => {
      switch (filter) {
        case "all": return true;
        case "blocked": return e.kind === "blocked";
        case "quarantined": return e.kind === "quarantined";
        case "by-me": return e.deniedBy === "you";
        case "24h": return hoursAgo(e.when) <= 24;
        case "7d": return hoursAgo(e.when) <= 24 * 7;
      }
    });
  }, [audit, filter]);

  const counts: Record<AuditFilter, number> = useMemo(() => ({
    all: audit.length,
    blocked: audit.filter((e) => e.kind === "blocked").length,
    quarantined: audit.filter((e) => e.kind === "quarantined").length,
    "by-me": audit.filter((e) => e.deniedBy === "you").length,
    "24h": audit.filter((e) => hoursAgo(e.when) <= 24).length,
    "7d": audit.filter((e) => hoursAgo(e.when) <= 24 * 7).length,
  }), [audit]);

  const handleOpenRule = (ruleId: string) => {
    setOpenPolicy({ id: ruleId, focus: "editor" });
  };
  const handleOpenPolicy = (id: string, focus: "editor" | "test" = "editor") => {
    setOpenPolicy({ id, focus });
  };
  const handlePause = (id: string) => {
    setPolicies((prev) =>
      prev.map((p) => (p.id === id ? { ...p, state: p.state === "paused" ? "active" : "paused" } : p)),
    );
    const p = policies.find((x) => x.id === id);
    if (p) setToast(`${p.state === "paused" ? "Resumed" : "Paused"} · ${p.name}`);
  };

  const opened = openPolicy ? policies.find((p) => p.id === openPolicy.id) ?? null : null;

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar />

      <main className="page-container max-w-[78rem] flex flex-col gap-6">
        <SecurityHeader layers={layers} />

        <FiveLayerCard layers={layers} />

        <AuditLog
          entries={filtered}
          filter={filter}
          onFilter={setFilter}
          counts={counts}
          onOpenRule={handleOpenRule}
          totalEntries={audit.length}
        />

        <PolicyLibrary
          policies={policies}
          onOpenPolicy={handleOpenPolicy}
          onPausePolicy={handlePause}
        />

        <footer className="mt-2 mb-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-neutral-500">
          <div className="inline-flex items-center gap-1.5">
            <ICON.Undo size={12} className="text-accent" />
            Every change has a 30-day undo. Pause is preferred over delete.
          </div>
          <a href="#runbook" className="text-accent hover:text-accent-solid font-medium">
            Operator runbook →
          </a>
        </footer>
      </main>

      {opened && (
        <PolicyDrawer
          policy={opened}
          focus={openPolicy?.focus ?? "editor"}
          onClose={() => setOpenPolicy(null)}
          onPause={() => handlePause(opened.id)}
          audit={audit}
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
