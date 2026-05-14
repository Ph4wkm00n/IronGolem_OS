// route: /inbox
// purpose: queue of agent proposals + drafts awaiting your approval. Replaces
// the legacy /inbox stub when VITE_ENABLE_V2_UI=true. Ported from Claude
// Design's Inbox.html (single-file TSX) — design source lives in
// `apps/web/src/_design-inbox/inbox/`.
//
// Integration notes:
// - Shell chrome (sticky topbar) comes from `pages/v2/_shared/WorkspaceTopbar`.
// - Mock data inline at the top; swap for `useInboxQuery()` once lib/api.ts
//   has the endpoint.
// - Dynamic `bg-${tone}` strings replaced with a static TONE classmap so
//   Tailwind's JIT compiles every class — same pattern as `Home.tsx`.
// - Drops the design-exploration `showEmpty` toggle; production never
//   needs a "preview empty state" affordance.

import React, { useEffect, useMemo, useReducer, useState } from "react";

import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

import {
  api,
  type InboxItem as Item,
  type InboxSource as Source,
  type InboxRisk as Risk,
  type InboxStatus as Status,
  type InboxDraft as Draft,
  type EmailDraft,
  type CalendarDraft,
  type WebhookDraft,
  type TelegramDraft,
  type InboxSafetyShape as SafetyShape,
  type AuditStep,
} from "../../lib/api";

type ToneName = "safe" | "warning" | "blocked" | "recovered" | "quarantined" | "accent" | "neutral";

// ─────────────────────────────────────────────────────────────────────────────
// Tone classmap — Tailwind needs every utility class as a literal string to
// include it in the bundle. Mirrors the pattern in Home.tsx so the audit
// pipeline can dedupe later.
// ─────────────────────────────────────────────────────────────────────────────

interface ToneClasses {
  readonly bg: string;
  readonly text: string;
  readonly border: string;
  readonly bgSolid: string;
  readonly textSolid: string;
  readonly bgSolidHover: string;
  readonly bgHover: string;
}

const TONE: Readonly<Record<ToneName, ToneClasses>> = {
  safe: {
    bg: "bg-safe", text: "text-safe", border: "border-safe",
    bgSolid: "bg-safe-solid", textSolid: "text-safe-solid",
    bgSolidHover: "hover:bg-safe-solid-hover", bgHover: "hover:bg-safe-hover",
  },
  warning: {
    bg: "bg-warning", text: "text-warning", border: "border-warning",
    bgSolid: "bg-warning-solid", textSolid: "text-warning-solid",
    bgSolidHover: "hover:bg-warning-solid-hover", bgHover: "hover:bg-warning-hover",
  },
  blocked: {
    bg: "bg-blocked", text: "text-blocked", border: "border-blocked",
    bgSolid: "bg-blocked-solid", textSolid: "text-blocked-solid",
    bgSolidHover: "hover:bg-blocked-solid-hover", bgHover: "hover:bg-blocked-hover",
  },
  recovered: {
    bg: "bg-recovered", text: "text-recovered", border: "border-recovered",
    bgSolid: "bg-recovered-solid", textSolid: "text-recovered-solid",
    bgSolidHover: "hover:bg-recovered-solid-hover", bgHover: "hover:bg-recovered-hover",
  },
  quarantined: {
    bg: "bg-quarantined", text: "text-quarantined", border: "border-quarantined",
    bgSolid: "bg-quarantined-solid", textSolid: "text-quarantined-solid",
    bgSolidHover: "hover:bg-quarantined-solid-hover", bgHover: "hover:bg-quarantined-hover",
  },
  accent: {
    bg: "bg-accent", text: "text-accent", border: "border-accent",
    bgSolid: "bg-accent-solid", textSolid: "text-accent-solid",
    bgSolidHover: "hover:bg-accent-solid-hover", bgHover: "hover:bg-accent-hover",
  },
  neutral: {
    bg: "bg-neutral", text: "text-neutral-700", border: "border-neutral-200",
    bgSolid: "bg-neutral-solid", textSolid: "text-neutral-solid",
    bgSolidHover: "hover:bg-neutral-solid-hover", bgHover: "hover:bg-neutral-bg-hover",
  },
};

const cx = (...c: ReadonlyArray<string | false | null | undefined>): string =>
  c.filter(Boolean).join(" ");

const STATUS_TONE: Readonly<Record<Status, ToneName>> = {
  awaiting: "warning",
  draft: "accent",
  held: "blocked",
  done: "safe",
};

const RISK_TONE: Readonly<Record<Risk, ToneName>> = {
  low: "safe",
  medium: "warning",
  high: "blocked",
};

// ─────────────────────────────────────────────────────────────────────────────
// Inline Heroicons-style SVG. TODO(integrator): graduate to `@irongolem/ui/icons`.
// ─────────────────────────────────────────────────────────────────────────────

interface IconProps {
  readonly size?: number;
  readonly className?: string;
}

const IconSvg = ({
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
  Mail: (p: IconProps) => <IconSvg {...p} d={<><rect x={3} y={5} width={18} height={14} rx={2} /><path d="m4 7 8 6 8-6" /></>} />,
  Calendar: (p: IconProps) => <IconSvg {...p} d={<><rect x={3} y={5} width={18} height={16} rx={2} /><path d="M3 9h18M8 3v4M16 3v4" /></>} />,
  Webhook: (p: IconProps) => <IconSvg {...p} d={<><circle cx={6} cy={8} r={3} /><path d="m8 11 5 8" /><circle cx={18} cy={18} r={3} /><path d="M15 18H8" /><circle cx={9} cy={18} r={3} /><path d="m20 15-4-6" /></>} />,
  Telegram: (p: IconProps) => <IconSvg {...p} d={<><path d="m21 4-9 16-3-7-7-3 19-6Z" /><path d="m9 13 7-5" /></>} />,
  Check: (p: IconProps) => <IconSvg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  X: (p: IconProps) => <IconSvg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Edit: (p: IconProps) => <IconSvg {...p} d={<><path d="M4 20h4l10-10-4-4L4 16v4Z" /><path d="m14 6 4 4" /></>} />,
  Clock: (p: IconProps) => <IconSvg {...p} d={<><circle cx={12} cy={12} r={9} /><path d="M12 7v5l3 2" /></>} />,
  Inbox: (p: IconProps) => <IconSvg {...p} d={<><path d="M3 12V5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v7" /><path d="M3 12h5l1.5 2.5h5L16 12h5v6a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-6Z" /></>} />,
  Alert: (p: IconProps) => <IconSvg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx={12} cy={17.5} r={0.5} fill="currentColor" stroke="none" /></>} />,
  Shield: (p: IconProps) => <IconSvg {...p} d={<><path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" /></>} />,
  Slash: (p: IconProps) => <IconSvg {...p} d={<path d="M5 19 19 5" />} />,
  Bell: (p: IconProps) => <IconSvg {...p} d={<><path d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2Z" /><path d="M10 20a2 2 0 0 0 4 0" /></>} />,
  Pause: (p: IconProps) => <IconSvg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  ArrowLeft: (p: IconProps) => <IconSvg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
} as const;

const SOURCE_META: Readonly<Record<Source, { readonly label: string; readonly Icon: React.ComponentType<IconProps> }>> = {
  email: { label: "Email", Icon: ICON.Mail },
  calendar: { label: "Calendar", Icon: ICON.Calendar },
  webhook: { label: "Webhook", Icon: ICON.Webhook },
  telegram: { label: "Telegram", Icon: ICON.Telegram },
};


// ─────────────────────────────────────────────────────────────────────────────
// Filter chips
// ─────────────────────────────────────────────────────────────────────────────

type ChipId = "all" | "awaiting" | "draft" | "held" | "done";

const CHIPS: ReadonlyArray<{ readonly id: ChipId; readonly label: string; readonly tone: ToneName }> = [
  { id: "all", label: "All", tone: "neutral" },
  { id: "awaiting", label: "Awaiting approval", tone: "warning" },
  { id: "draft", label: "Drafts", tone: "accent" },
  { id: "held", label: "Held for review", tone: "blocked" },
  { id: "done", label: "Done today", tone: "safe" },
];

function applyChip(items: readonly Item[], chip: ChipId): readonly Item[] {
  if (chip === "all") return items;
  return items.filter((i) => i.status === chip);
}

function relTime(min: number): string {
  if (min < 1) return "just now";
  if (min < 60) return `${min}m ago`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

// ─────────────────────────────────────────────────────────────────────────────
// Reducer — optimistic state transitions for approve / deny / snooze / edit
// ─────────────────────────────────────────────────────────────────────────────

type ItemAction =
  | { type: "approve"; id: string }
  | { type: "deny"; id: string; cause: string }
  | { type: "snooze"; id: string }
  | { type: "edit-commit"; id: string; draft: Draft }
  | { type: "mark-read"; id: string }
  // v0.2 Step 3: full replacement so the real-API useEffect can swap the
  // mocked seed with the response from GET /api/v1/inbox once it arrives.
  | { type: "replace"; items: Item[] };

function itemsReducer(state: Item[], action: ItemAction): Item[] {
  switch (action.type) {
    case "replace":
      return action.items.map((e) => ({ ...e }));
    case "approve":
      return state.map((it) =>
        it.id === action.id
          ? {
              ...it,
              status: "done",
              minutesAgo: 0,
              unread: false,
              cause: "You approved this just now — sent.",
              audit: [{ at: "just now", actor: "You", note: "Approved." }, ...it.audit],
            }
          : it,
      );
    case "deny":
      return state.map((it) =>
        it.id === action.id
          ? {
              ...it,
              status: "held",
              minutesAgo: 0,
              unread: false,
              cause: action.cause,
              audit: [{ at: "just now", actor: "You", note: `Denied — ${action.cause}` }, ...it.audit],
            }
          : it,
      );
    case "snooze":
      return state
        .filter((it) => it.id !== action.id)
        .concat(
          state
            .filter((it) => it.id === action.id)
            .map((it) => ({
              ...it,
              minutesAgo: 60,
              unread: true,
              audit: [{ at: "just now", actor: "You", note: "Snoozed 1h." }, ...it.audit],
            })),
        );
    case "edit-commit":
      return state.map((it) =>
        it.id === action.id
          ? { ...it, draft: action.draft, audit: [{ at: "just now", actor: "You", note: "Edited draft." }, ...it.audit] }
          : it,
      );
    case "mark-read":
      return state.map((it) => (it.id === action.id ? { ...it, unread: false } : it));
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Sub-components
// ─────────────────────────────────────────────────────────────────────────────

function SourcePill({ source }: { readonly source: Source }) {
  const m = SOURCE_META[source];
  const Icn = m.Icon;
  return (
    <span className="inline-flex items-center gap-1 rounded-md border border-neutral-200 bg-white px-1.5 py-0.5 text-[10px] font-medium text-neutral-600">
      <span className="text-neutral-500">
        <Icn size={11} />
      </span>
      {m.label}
    </span>
  );
}

function StatusDot({ status }: { readonly status: Status }) {
  const tone = TONE[STATUS_TONE[status]];
  return <span className={cx("h-1.5 w-1.5 rounded-full shrink-0", tone.bgSolid)} />;
}

// TODO(integrator): swap for <RiskBadge /> from @irongolem/ui.
function RiskBadge({ level, size = "sm" }: { readonly level: Risk; readonly size?: "sm" | "md" }) {
  const tone = TONE[RISK_TONE[level]];
  const label = ({ low: "low risk", medium: "med risk", high: "high risk" } as const)[level];
  const sizeCx = size === "sm" ? "text-[10px] px-1.5 py-0.5" : "text-xs px-2 py-0.5";
  return (
    <span
      className={cx(
        "inline-flex items-center gap-1 rounded-full border font-medium",
        sizeCx,
        tone.bg,
        tone.text,
        tone.border,
      )}
    >
      <span className={cx("h-1.5 w-1.5 rounded-full", tone.bgSolid)} />
      {label}
    </span>
  );
}

// TODO(integrator): swap for <SafetyCard /> from @irongolem/ui.
function SafetyCard({ safety }: { readonly safety: SafetyShape }) {
  const sections: ReadonlyArray<{
    readonly label: string;
    readonly items: readonly string[];
    readonly tone: ToneName;
    readonly Icn: React.ComponentType<IconProps>;
  }> = [
    { label: "Can", items: safety.can, tone: "safe", Icn: ICON.Check },
    { label: "Needs approval", items: safety.needsApproval, tone: "warning", Icn: ICON.Bell },
    { label: "Cannot", items: safety.cannot, tone: "blocked", Icn: ICON.Slash },
    { label: "Stops if", items: safety.stopsIf, tone: "quarantined", Icn: ICON.Pause },
  ];
  return (
    <div className="rounded-xl border border-neutral-200 bg-neutral-50/60 p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">Safety</div>
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
                <span className={tone.text}>
                  <s.Icn size={13} />
                </span>
                <span className={cx("text-[11px] font-medium uppercase tracking-wide", tone.text)}>
                  {s.label}
                </span>
              </div>
              <ul className="space-y-1">
                {s.items.length === 0 && <li className="text-xs text-neutral-400">—</li>}
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

interface FilterChipsProps {
  readonly active: ChipId;
  readonly onChange: (c: ChipId) => void;
  readonly counts: Record<ChipId, number>;
}
function FilterChips({ active, onChange, counts }: FilterChipsProps) {
  return (
    <div className="flex flex-wrap items-center gap-1.5 p-3 border-b border-neutral-100">
      {CHIPS.map((c) => {
        const isActive = c.id === active;
        const n = counts[c.id];
        const tone = TONE[c.tone];
        return (
          <button
            key={c.id}
            type="button"
            onClick={() => onChange(c.id)}
            className={cx(
              "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
              isActive
                ? cx(tone.bgSolid, "text-white", tone.border)
                : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
            )}
          >
            {c.label}
            <span
              className={cx(
                "rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px",
                isActive ? "bg-white/25 text-white" : "bg-neutral-100 text-neutral-500",
              )}
            >
              {n}
            </span>
          </button>
        );
      })}
    </div>
  );
}

interface InboxRowProps {
  readonly item: Item;
  readonly selected: boolean;
  readonly onSelect: () => void;
}
function InboxRow({ item, selected, onSelect }: InboxRowProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cx(
        "w-full text-left px-4 py-3 border-l-2 transition-colors block",
        selected ? "bg-accent border-l-accent-solid" : "bg-white border-l-transparent hover:bg-neutral-50",
      )}
    >
      <div className="flex items-start gap-2">
        <span
          className={cx(
            "mt-1.5 h-2 w-2 rounded-full shrink-0",
            item.unread ? "bg-accent-solid" : "bg-transparent",
          )}
          aria-label={item.unread ? "unread" : "read"}
        />
        <div className="flex-1 min-w-0">
          <div className="flex items-baseline justify-between gap-2">
            <div
              className={cx(
                "text-[13.5px] leading-snug truncate",
                item.unread ? "font-semibold text-neutral-900" : "font-medium text-neutral-800",
              )}
            >
              {item.title}
            </div>
            <div className="text-[11px] text-neutral-400 shrink-0 tabular-nums">{relTime(item.minutesAgo)}</div>
          </div>

          <div className="mt-1.5 flex items-center gap-1.5 flex-wrap">
            <SourcePill source={item.source} />
            <RiskBadge level={item.risk} />
            <span className="inline-flex items-center gap-1 text-[10px] text-neutral-500">
              <StatusDot status={item.status} />
              {item.routedBy}
            </span>
          </div>

          <p className="mt-1.5 text-[12.5px] text-neutral-600 line-clamp-1">{item.summary}</p>
          <p className="mt-0.5 text-[11.5px] text-neutral-400 leading-snug line-clamp-1">
            <span className="text-neutral-500 font-medium">Why this is here:</span> {item.cause}
          </p>
        </div>
      </div>
    </button>
  );
}

interface DraftedBlockProps {
  readonly draft: Draft;
  readonly editing: boolean;
  readonly onChange: (next: Draft) => void;
}
function DraftedBlock({ draft, editing, onChange }: DraftedBlockProps) {
  if (draft.kind === "email") {
    return (
      <article className="rounded-xl border border-neutral-200 bg-white overflow-hidden">
        <header className="px-4 py-3 border-b border-neutral-100 bg-neutral-50/60">
          <KV label="From" value={draft.from} />
          <KV label="To" value={draft.to} />
          {draft.cc && <KV label="Cc" value={draft.cc} />}
          <div className="mt-1.5">
            <span className="text-[10px] font-medium uppercase tracking-wide text-neutral-400 mr-2">Subject</span>
            {editing ? (
              <input
                value={draft.subject}
                onChange={(e) => onChange({ ...draft, subject: e.target.value })}
                className="text-[14px] font-semibold tracking-tight text-neutral-900 bg-transparent w-[calc(100%-4.5rem)] outline-none focus:bg-white focus:ring-2 ring-accent rounded px-1"
              />
            ) : (
              <span className="text-[14px] font-semibold tracking-tight text-neutral-900">{draft.subject}</span>
            )}
          </div>
        </header>
        <div className="px-4 py-4 space-y-3 text-[14px] leading-relaxed text-neutral-800">
          {editing ? (
            <textarea
              value={draft.body.join("\n\n")}
              onChange={(e) => onChange({ ...draft, body: e.target.value.split("\n\n") })}
              className="w-full min-h-[160px] resize-y outline-none bg-neutral-50/60 border border-neutral-200 rounded-md p-2 focus:ring-2 ring-accent font-sans"
            />
          ) : (
            draft.body.map((para, i) => <p key={i}>{para}</p>)
          )}
        </div>
      </article>
    );
  }
  if (draft.kind === "calendar") {
    return (
      <article className="rounded-xl border border-neutral-200 bg-white overflow-hidden">
        <header className="px-4 py-3 border-b border-neutral-100 bg-neutral-50/60 flex items-center gap-3">
          <span className="h-8 w-8 rounded-lg bg-recovered text-recovered inline-flex items-center justify-center">
            <ICON.Calendar size={16} />
          </span>
          <div>
            <div className="text-[10px] font-medium uppercase tracking-wide text-neutral-400">Calendar invite</div>
            <div className="text-[14px] font-semibold tracking-tight text-neutral-900">{draft.invite}</div>
          </div>
        </header>
        <div className="px-4 py-4 grid grid-cols-[6rem_1fr] gap-y-2 gap-x-3 text-[13.5px]">
          <span className="text-neutral-500">When</span>
          <span className="text-neutral-900 font-medium">{draft.when}</span>
          <span className="text-neutral-500">Where</span>
          <span className="text-neutral-700">{draft.where}</span>
          <span className="text-neutral-500">With</span>
          <span className="flex flex-wrap gap-1">
            {draft.attendees.map((a) => (
              <span key={a} className="inline-flex items-center rounded-md bg-neutral-100 px-1.5 py-0.5 text-[11px] text-neutral-700">
                {a}
              </span>
            ))}
          </span>
          <span className="text-neutral-500">Note</span>
          <span className="text-neutral-700 leading-relaxed">{draft.description}</span>
        </div>
      </article>
    );
  }
  if (draft.kind === "telegram") {
    return (
      <article className="rounded-xl border border-neutral-200 bg-white overflow-hidden">
        <header className="px-4 py-2.5 border-b border-neutral-100 bg-neutral-50/60 flex items-center gap-2">
          <ICON.Telegram size={13} className="text-neutral-500" />
          <span className="text-[12px] font-medium text-neutral-700">{draft.chat}</span>
        </header>
        <div className="px-4 py-3 space-y-3">
          <div className="text-[12px] text-neutral-500">
            ↳ replying to <span className="text-neutral-700">{draft.reply_to}</span>
          </div>
          <div className="max-w-[80%] rounded-2xl rounded-bl-sm bg-accent text-accent px-3.5 py-2 text-[14px] leading-relaxed">
            {editing ? (
              <textarea
                value={draft.body}
                onChange={(e) => onChange({ ...draft, body: e.target.value })}
                className="w-full min-h-[60px] bg-transparent outline-none resize-none"
              />
            ) : (
              draft.body
            )}
          </div>
        </div>
      </article>
    );
  }
  // webhook
  return (
    <article className="rounded-xl border border-neutral-200 bg-white overflow-hidden">
      <header className="px-4 py-2.5 border-b border-neutral-100 bg-neutral-50/60 flex items-center gap-2">
        <span className="inline-flex items-center gap-1 rounded-md bg-recovered text-recovered px-1.5 py-0.5 text-[10px] font-mono font-semibold">
          {draft.method}
        </span>
        <span className="text-[12.5px] font-mono text-neutral-700 truncate">{draft.endpoint}</span>
      </header>
      <div className="px-4 py-3">
        <table className="w-full text-[12.5px]">
          <tbody className="[&_tr+tr]:border-t [&_tr]:border-neutral-100">
            {draft.fields.map((f) => (
              <tr key={f.label}>
                <td className="py-1.5 pr-3 font-mono text-neutral-500 w-1/3 align-top">{f.label}</td>
                <td className="py-1.5 font-mono text-neutral-900 break-all">{f.value}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </article>
  );
}

function KV({ label, value }: { readonly label: string; readonly value: string }) {
  return (
    <div className="flex items-baseline gap-2 leading-tight">
      <span className="text-[10px] font-medium uppercase tracking-wide text-neutral-400 w-12 shrink-0">{label}</span>
      <span className="text-[13px] text-neutral-700 truncate">{value}</span>
    </div>
  );
}

function OriginChips({ item }: { readonly item: Item }) {
  const m = SOURCE_META[item.source];
  const Icn = m.Icon;
  return (
    <div className="flex flex-wrap items-center gap-2 text-[12px] text-neutral-500">
      <span className="inline-flex items-center gap-1.5 rounded-full bg-neutral-100 px-2 py-0.5">
        <Icn size={12} /> {m.label}
      </span>
      <span className="text-neutral-300">·</span>
      <RiskBadge level={item.risk} size="md" />
      <span className="text-neutral-300">·</span>
      <span className="inline-flex items-center gap-1">
        <ICON.Clock size={12} /> {relTime(item.minutesAgo)}
      </span>
      <span className="text-neutral-300">·</span>
      <span className="inline-flex items-center gap-1.5">
        <StatusDot status={item.status} /> routed by{" "}
        <span className="text-neutral-700 font-medium">{item.routedBy}</span>
      </span>
    </div>
  );
}

function WhyCallout({ cause }: { readonly cause: string }) {
  return (
    <div className="rounded-xl border border-warning bg-warning p-4">
      <div className="flex items-start gap-3">
        <span className="h-7 w-7 shrink-0 rounded-md bg-warning-solid text-white inline-flex items-center justify-center">
          <ICON.Alert size={14} />
        </span>
        <div className="min-w-0">
          <div className="text-[11px] font-medium uppercase tracking-wide text-warning">Why this is in your inbox</div>
          <div className="mt-1 text-[14px] text-warning leading-relaxed">{cause}</div>
        </div>
      </div>
    </div>
  );
}

function AuditTrail({ steps }: { readonly steps: readonly AuditStep[] }) {
  return (
    <ol className="relative pl-5 space-y-3">
      <span className="absolute left-[7px] top-1 bottom-1 w-px bg-neutral-200" />
      {steps.map((s, i) => (
        <li key={i} className="relative">
          <span className="absolute -left-[18px] top-[5px] h-2.5 w-2.5 rounded-full bg-white border-2 border-neutral-300" />
          <div className="flex items-baseline gap-2 flex-wrap">
            <span className="text-[12px] font-medium text-neutral-900">{s.actor}</span>
            <span className="text-[11px] text-neutral-400 font-mono">{s.at}</span>
          </div>
          <div className="text-[12.5px] text-neutral-600 leading-snug">{s.note}</div>
        </li>
      ))}
    </ol>
  );
}

interface DetailDrawerProps {
  readonly item: Item;
  readonly editing: boolean;
  readonly draftBuffer: Draft | undefined;
  readonly onEdit: () => void;
  readonly onCancelEdit: () => void;
  readonly onCommitEdit: () => void;
  readonly onChangeBuffer: (d: Draft) => void;
  readonly onApprove: () => void;
  readonly onDeny: () => void;
  readonly onSnooze: () => void;
  readonly onBack: () => void;
}
function DetailDrawer({
  item, editing, draftBuffer,
  onEdit, onCancelEdit, onCommitEdit, onChangeBuffer,
  onApprove, onDeny, onSnooze, onBack,
}: DetailDrawerProps) {
  const isActionable = item.status !== "done";
  const statusLabel = ({
    awaiting: "Awaiting approval",
    draft: "Draft",
    held: "Held for review",
    done: "Done",
  } as const)[item.status];

  return (
    <section className="h-full flex flex-col bg-white">
      <header className="sticky top-0 z-10 bg-white/85 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
        <button
          type="button"
          onClick={onBack}
          className="md:hidden inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50"
        >
          <ICON.ArrowLeft size={14} /> Back
        </button>
        <span className="text-[11px] font-mono text-neutral-400 truncate">{item.id.toUpperCase()}</span>
        <span className="text-neutral-300">·</span>
        <span className="inline-flex items-center gap-1.5 text-[11px] text-neutral-500">
          <StatusDot status={item.status} />
          {statusLabel}
        </span>
        <div className="ml-auto flex items-center gap-1">
          <button
            type="button"
            onClick={onSnooze}
            className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-600 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50"
          >
            <ICON.Clock size={13} /> Snooze 1h
          </button>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto scrollbar-thin">
        <div className="px-6 sm:px-8 py-6 max-w-3xl">
          <h1 className="page-title">{item.title}</h1>
          <div className="mt-3">
            <OriginChips item={item} />
          </div>

          {item.draft && (
            <div className="mt-6">
              <div className="flex items-center justify-between mb-2">
                <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">Drafted content</div>
                {isActionable &&
                  (editing ? (
                    <div className="flex items-center gap-1">
                      <button
                        type="button"
                        onClick={onCancelEdit}
                        className="text-[12px] font-medium text-neutral-500 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50"
                      >
                        Cancel
                      </button>
                      <button
                        type="button"
                        onClick={onCommitEdit}
                        className="text-[12px] font-medium text-accent hover:text-accent-solid px-2 py-1 rounded-md hover:bg-accent-hover"
                      >
                        Save changes
                      </button>
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={onEdit}
                      className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-600 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50"
                    >
                      <ICON.Edit size={12} /> Edit draft
                    </button>
                  ))}
              </div>
              <DraftedBlock
                draft={editing && draftBuffer ? draftBuffer : item.draft}
                editing={editing}
                onChange={onChangeBuffer}
              />
            </div>
          )}

          <div className="mt-6">
            <WhyCallout cause={item.cause} />
          </div>

          <div className="mt-6">
            <SafetyCard safety={item.safety} />
          </div>

          <div className="mt-8">
            <div className="text-xs font-medium uppercase tracking-wide text-neutral-500 mb-3">Audit trail</div>
            <AuditTrail steps={item.audit} />
          </div>

          <div className="h-24" />
        </div>
      </div>

      {isActionable && (
        <footer className="sticky bottom-0 bg-white/95 backdrop-blur border-t border-neutral-100 px-5 sm:px-8 py-3.5 flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={onApprove}
            className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid hover:bg-accent-solid-hover text-white text-[13.5px] font-semibold px-4 py-2 shadow-sm transition-colors"
          >
            <ICON.Check size={14} />
            Approve {item.draft?.kind === "email" ? "& send" : item.draft?.kind === "calendar" ? "& send invite" : ""}
          </button>
          <button
            type="button"
            onClick={onEdit}
            className="inline-flex items-center gap-1.5 rounded-md border border-neutral-300 hover:bg-neutral-50 text-neutral-800 text-[13.5px] font-medium px-3.5 py-2 transition-colors"
          >
            <ICON.Edit size={14} /> Edit draft
          </button>
          <button
            type="button"
            onClick={onDeny}
            className="inline-flex items-center gap-1.5 rounded-md text-blocked hover:bg-blocked-hover text-[13.5px] font-medium px-3 py-2 transition-colors"
          >
            <ICON.X size={14} /> Deny
          </button>
          <div className="flex-1" />
          <button
            type="button"
            onClick={onSnooze}
            className="hidden sm:inline-flex items-center gap-1.5 rounded-md text-neutral-500 hover:text-neutral-900 hover:bg-neutral-50 text-[12.5px] font-medium px-3 py-2 transition-colors"
          >
            <ICON.Clock size={13} /> Snooze 1h
          </button>
        </footer>
      )}
    </section>
  );
}

interface ToastState {
  readonly kind: "approved" | "denied" | "snoozed";
  readonly title: string;
}

function Toast({ toast }: { readonly toast: ToastState }) {
  const tone = TONE[toast.kind === "approved" ? "safe" : toast.kind === "denied" ? "blocked" : "neutral"];
  const verb =
    toast.kind === "approved" ? "Approved · sent" : toast.kind === "denied" ? "Denied · held for review" : "Snoozed 1h";
  const Icn = toast.kind === "approved" ? ICON.Check : toast.kind === "denied" ? ICON.X : ICON.Clock;
  return (
    <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50 pointer-events-none ig-toast-in" role="status" aria-live="polite">
      <div
        className={cx(
          "pointer-events-auto rounded-lg border shadow-lg bg-white px-4 py-3 flex items-center gap-3 min-w-[280px] max-w-[420px]",
          tone.border,
        )}
      >
        <span className={tone.text}>
          <Icn size={16} />
        </span>
        <div className="flex-1 min-w-0">
          <div className={cx("text-[10.5px] font-medium uppercase tracking-wide", tone.text)}>{verb}</div>
          <div className="text-[13.5px] text-neutral-900 truncate">{toast.title}</div>
        </div>
      </div>
    </div>
  );
}

function EmptyInbox() {
  return (
    <div className="h-full flex items-center justify-center px-8 py-16">
      <div className="text-center max-w-md">
        <span className="h-12 w-12 rounded-full bg-safe text-safe inline-flex items-center justify-center mb-4">
          <ICON.Check size={22} />
        </span>
        <h2 className="text-[20px] font-semibold tracking-tight text-neutral-900">Your inbox is empty</h2>
        <p className="mt-2 text-[14px] text-neutral-600 leading-relaxed">
          Your assistant teams are handling everything inside the rules. We'll surface anything that needs you here — and only that.
        </p>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route component
// ─────────────────────────────────────────────────────────────────────────────

export function Inbox() {
  const initialItems = useMemo(() => api.v2.inbox.getMock(), []);
  const [items, dispatch] = useReducer(itemsReducer, initialItems, (initial) => initial.map((e) => ({ ...e })));
  const [chip, setChip] = useState<ChipId>("awaiting");
  const [selectedId, setSelectedId] = useState<string | null>(initialItems[0]?.id ?? null);
  const [editing, setEditing] = useState(false);
  const [draftBuffer, setDraftBuffer] = useState<Draft | undefined>(undefined);
  const [toast, setToast] = useState<ToastState | null>(null);

  // v0.2 Step 3 — F6 Inbox real-API. When VITE_API_MODE_INBOX=real the
  // first render shows the mock seed (so the layout never flashes empty),
  // then this effect fetches the real list from the gateway and swaps it
  // in via the reducer's `replace` action. In mock mode `api.v2.inbox.list()`
  // resolves to the same mock array synchronously, so the dispatch is a
  // no-op `replace` — kept unconditional for the sole benefit of catching
  // shape drift between the mock and the real wire contract on the next
  // hot-reload.
  useEffect(() => {
    let cancelled = false;
    api.v2.inbox
      .list()
      .then((next) => {
        if (cancelled) return;
        dispatch({ type: "replace", items: next.map((e) => ({ ...e })) as Item[] });
        if (next.length > 0) setSelectedId(next[0]!.id);
      })
      .catch(() => {
        // Real-mode failures keep the mock seed visible; the gateway log
        // is the source of truth for diagnostics.
      });
    return () => {
      cancelled = true;
    };
  }, []);
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false);

  const counts: Record<ChipId, number> = useMemo(
    () => ({
      all: items.length,
      awaiting: items.filter((i) => i.status === "awaiting").length,
      draft: items.filter((i) => i.status === "draft").length,
      held: items.filter((i) => i.status === "held").length,
      done: items.filter((i) => i.status === "done").length,
    }),
    [items],
  );

  const visible = useMemo(() => {
    const filtered = applyChip(items, chip);
    return filtered.slice().sort((a, b) => {
      if (a.unread !== b.unread) return a.unread ? -1 : 1;
      return a.minutesAgo - b.minutesAgo;
    });
  }, [items, chip]);

  const selected = useMemo<Item | null>(
    () => items.find((i) => i.id === selectedId) ?? visible[0] ?? null,
    [items, selectedId, visible],
  );

  useEffect(() => {
    if (selected && selected.unread) {
      dispatch({ type: "mark-read", id: selected.id });
    }
  }, [selected?.id, selected?.unread]);

  useEffect(() => {
    if (!toast) return undefined;
    const t = setTimeout(() => setToast(null), 2400);
    return () => clearTimeout(t);
  }, [toast]);

  useEffect(() => {
    if (!selected && visible[0]) setSelectedId(visible[0].id);
  }, [selected, visible]);

  const onSelect = (id: string) => {
    setSelectedId(id);
    setEditing(false);
    setDraftBuffer(undefined);
    setMobileDetailOpen(true);
  };
  const onApprove = () => {
    if (!selected) return;
    setToast({ kind: "approved", title: selected.title });
    dispatch({ type: "approve", id: selected.id });
    setEditing(false);
  };
  const onDeny = () => {
    if (!selected) return;
    setToast({ kind: "denied", title: selected.title });
    dispatch({ type: "deny", id: selected.id, cause: "You denied this just now." });
    setEditing(false);
  };
  const onSnooze = () => {
    if (!selected) return;
    setToast({ kind: "snoozed", title: selected.title });
    dispatch({ type: "snooze", id: selected.id });
  };
  const onEdit = () => {
    if (!selected || !selected.draft) return;
    setDraftBuffer(selected.draft);
    setEditing(true);
  };
  const onCommitEdit = () => {
    if (!selected || !draftBuffer) return;
    dispatch({ type: "edit-commit", id: selected.id, draft: draftBuffer });
    setEditing(false);
  };

  return (
    <div className="min-h-screen bg-neutral-50 flex flex-col">
      <WorkspaceTopbar />

      <div className="page-container pt-6 pb-4">
        <div className="flex items-end justify-between gap-4 flex-wrap">
          <div>
            <h1 className="page-title">Inbox</h1>
            <p className="text-[14px] text-neutral-600 mt-1 max-w-xl">
              Everything that needs your eye. Suppressed-on-OK — items the system handled cleanly never appear here.
            </p>
          </div>
          <div className="hidden md:flex items-center gap-2 text-[12px] text-neutral-500">
            <span className="inline-flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-warning-solid" />
              {counts.awaiting} awaiting
            </span>
            <span className="text-neutral-300">·</span>
            <span className="inline-flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-blocked-solid" />
              {counts.held} held
            </span>
            <span className="text-neutral-300">·</span>
            <span className="inline-flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-safe-solid" />
              {counts.done} done today
            </span>
          </div>
        </div>
      </div>

      <div className="page-container pb-6 flex-1">
        <div className="rounded-2xl border border-neutral-200 bg-white overflow-hidden shadow-sm">
          <div className="md:grid md:grid-cols-[24rem_1fr] md:min-h-[640px]">
            <aside
              className={cx(
                "border-r border-neutral-100 flex flex-col",
                mobileDetailOpen ? "hidden md:flex" : "flex",
              )}
            >
              <FilterChips active={chip} onChange={setChip} counts={counts} />

              <div className="flex-1 overflow-y-auto scrollbar-thin divide-y divide-neutral-100">
                {visible.length === 0 ? (
                  <div className="px-6 py-10 text-center">
                    <div className="text-[13px] text-neutral-500 leading-relaxed">
                      Nothing in this filter. Your assistant teams are handling everything inside the rules.
                    </div>
                  </div>
                ) : (
                  visible.map((it) => (
                    <InboxRow
                      key={it.id}
                      item={it}
                      selected={!!selected && selected.id === it.id}
                      onSelect={() => onSelect(it.id)}
                    />
                  ))
                )}
              </div>
            </aside>

            <div className={cx("min-h-[640px]", !mobileDetailOpen ? "hidden md:block" : "block")}>
              {selected ? (
                <DetailDrawer
                  item={selected}
                  editing={editing}
                  draftBuffer={draftBuffer}
                  onEdit={onEdit}
                  onCancelEdit={() => {
                    setEditing(false);
                    setDraftBuffer(undefined);
                  }}
                  onCommitEdit={onCommitEdit}
                  onChangeBuffer={setDraftBuffer}
                  onApprove={onApprove}
                  onDeny={onDeny}
                  onSnooze={onSnooze}
                  onBack={() => setMobileDetailOpen(false)}
                />
              ) : (
                <EmptyInbox />
              )}
            </div>
          </div>
        </div>
      </div>

      {toast && <Toast toast={toast} />}
    </div>
  );
}
