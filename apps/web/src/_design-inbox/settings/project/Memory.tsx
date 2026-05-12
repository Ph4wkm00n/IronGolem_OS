// Memory.tsx — IronGolem OS
// Route: /memory
// One-file route. Mock data first, then the route. TODO(integrator) tags
// mark @irongolem/ui imports and live API calls to wire in later.
//
// React 19, TS strict, Tailwind utility classes only. Semantic palette
// (bg-safe / bg-warning / bg-blocked / bg-accent / bg-neutral / text-*)
// comes from globals.css and behaves in both themes.
//
// Mandatory patterns wired in:
//   1. Explainable Autonomy — "Why do you know this?" is one click away on
//      every item; it expands the evidence trail inline.
//   2. Forget-easily — "Forget this" is always visible in the card footer
//      (never behind a menu). Forgetting is non-destructive: moves to a
//      Forgotten bucket with a 30-day undo, shown in a banner with a
//      one-tap restore.
//   3. Freshness-first — anything untouched for >30 days carries a
//      "Re-verify" pill in the warning palette.
//   4. Progressive disclosure — graph view is opt-in via a top-right
//      toggle. Default surface is the list because it's easier to skim,
//      search, and edit.

import * as React from "react";
const { useState, useMemo, useEffect, useRef } = React;

// ───────────────────────────────────────────────────────────────────────────
//  Types
// ───────────────────────────────────────────────────────────────────────────

type Subject = "people" | "accounts" | "preferences" | "patterns";
type FreshnessBucket = "hours" | "days" | "weeks" | "months";

type Evidence = {
  id: string;
  when: string;            // "2h ago" / "yesterday"
  source: string;          // human-readable source
  ref: string;             // canonical reference, font-mono
  snippet: string;         // what was observed
};

type MemoryItem = {
  id: string;
  subject: Subject;
  subjectLabel: string;    // "Sarah Lopez — calendar"
  fact: string;            // plain-language fact
  confidence: number;      // 0..100
  lastTouchedHours: number;
  lastTouchedLabel: string; // "Verified 2h ago" / "Last seen 17d ago"
  verified: boolean;        // true → "Verified Xh ago"; false → "Last seen Xd ago"
  evidence: Evidence[];
  tags: string[];           // freeform tags
};

// ───────────────────────────────────────────────────────────────────────────
//  Mock memory — 22 items, mixed subjects and confidence.
//  TODO(integrator): replace with `useMemoryQuery()` from @irongolem/data.
// ───────────────────────────────────────────────────────────────────────────

const MOCK_MEMORY: MemoryItem[] = [
  {
    id: "m01", subject: "people", subjectLabel: "Sarah Lopez — calendar",
    fact: "Prefers Thursday afternoons for status meetings; declines anything before 10am.",
    confidence: 94, lastTouchedHours: 2, lastTouchedLabel: "Verified 2h ago", verified: true,
    tags: ["scheduling", "exec"],
    evidence: [
      { id: "e01a", when: "2h ago", source: "Calendar history", ref: "calendar/sarah-lopez", snippet: "Declined 09:30 invite, suggested Thursday 14:00." },
      { id: "e01b", when: "1w ago", source: "Calendar history", ref: "calendar/sarah-lopez", snippet: "Three of last four 1:1s held Thursday 14:00–14:30." },
      { id: "e01c", when: "3w ago", source: "Direct statement", ref: "thread/2026-04-18",     snippet: "“Mornings are blocked for deep work, Thursdays are easiest.”" },
    ],
  },
  {
    id: "m02", subject: "accounts", subjectLabel: "Riverbend Co — MSA",
    fact: "Master services agreement renews on Aug 14, 2026. Renewal owner is Priya Shah.",
    confidence: 99, lastTouchedHours: 6, lastTouchedLabel: "Verified 6h ago", verified: true,
    tags: ["contracts"],
    evidence: [
      { id: "e02a", when: "6h ago", source: "Contracts vault", ref: "contracts/riverbend/msa-v3", snippet: "Renewal date field reads 2026-08-14." },
      { id: "e02b", when: "1m ago", source: "Email thread",     ref: "thread/priya-msa",          snippet: "Priya confirmed she owns the renewal cycle." },
    ],
  },
  {
    id: "m03", subject: "preferences", subjectLabel: "You — drafting tone",
    fact: "You prefer short, plain-English replies. No greetings, no sign-offs in internal threads.",
    confidence: 91, lastTouchedHours: 5, lastTouchedLabel: "Verified 5h ago", verified: true,
    tags: ["voice", "writing"],
    evidence: [
      { id: "e03a", when: "5h ago", source: "Edit history", ref: "edits/draft-114", snippet: "You removed “Hi team,” and “Thanks,” before sending." },
      { id: "e03b", when: "4d ago", source: "Edit history", ref: "edits/draft-098", snippet: "You shortened a 3-paragraph draft to 4 lines." },
      { id: "e03c", when: "3w ago", source: "Edit history", ref: "edits/draft-072", snippet: "Same pattern: removed openers, kept first sentence." },
    ],
  },
  {
    id: "m04", subject: "patterns", subjectLabel: "Weekly — invoice batch",
    fact: "On Fridays around 3pm you process the supplier invoice batch end-to-end.",
    confidence: 87, lastTouchedHours: 26, lastTouchedLabel: "Last seen yesterday", verified: false,
    tags: ["recurring", "ap"],
    evidence: [
      { id: "e04a", when: "yesterday", source: "Activity log", ref: "log/2026-05-09", snippet: "Approved 7 invoices in a single 22-minute session, 15:02–15:24." },
      { id: "e04b", when: "1w ago",    source: "Activity log", ref: "log/2026-05-02", snippet: "Same shape: Friday afternoon batch, similar duration." },
      { id: "e04c", when: "5w ago",    source: "Activity log", ref: "log/2026-04-04", snippet: "Cadence first detected; 4 of last 6 weeks match." },
    ],
  },
  {
    id: "m05", subject: "people", subjectLabel: "Marcus Hill — drafting voice",
    fact: "Marcus's review notes use clipped declaratives; he edits out hedges like “might” and “perhaps”.",
    confidence: 82, lastTouchedHours: 11, lastTouchedLabel: "Verified 11h ago", verified: true,
    tags: ["voice"],
    evidence: [
      { id: "e05a", when: "11h ago", source: "Edit history", ref: "edits/draft-122", snippet: "Replaced “might be worth considering” → “consider”." },
      { id: "e05b", when: "5d ago",  source: "Edit history", ref: "edits/draft-101", snippet: "Replaced “perhaps” → cut entirely." },
    ],
  },
  {
    id: "m06", subject: "accounts", subjectLabel: "Halford — pump supplier",
    fact: "Halford ships maintenance pumps; PO-24-099 references lot 24-118 currently under recall.",
    confidence: 96, lastTouchedHours: 30, lastTouchedLabel: "Verified yesterday", verified: true,
    tags: ["purchasing", "alert"],
    evidence: [
      { id: "e06a", when: "yesterday", source: "Recall feed", ref: "halford.io/recall/2024-118", snippet: "Lot 24-118 listed for replacement." },
      { id: "e06b", when: "1m ago",    source: "Workspace PO history", ref: "po/24-099", snippet: "PO references lot 24-118, qty 2." },
    ],
  },
  {
    id: "m07", subject: "preferences", subjectLabel: "You — approval threshold",
    fact: "You approve standing-order purchases under $50 without further review.",
    confidence: 88, lastTouchedHours: 70, lastTouchedLabel: "Verified 3d ago", verified: true,
    tags: ["purchasing"],
    evidence: [
      { id: "e07a", when: "3d ago", source: "Approval log", ref: "approvals/may-2026", snippet: "12 approvals at $42 or less in last 30 days, all single-tap." },
      { id: "e07b", when: "8d ago", source: "Direct statement", ref: "thread/2026-05-02", snippet: "“Under fifty is fine, just approve.”" },
    ],
  },
  {
    id: "m08", subject: "patterns", subjectLabel: "Monthly — board update",
    fact: "First Monday of each month you draft the board update between 6pm and 8pm PT.",
    confidence: 84, lastTouchedHours: 24 * 9, lastTouchedLabel: "Last seen 9d ago", verified: false,
    tags: ["recurring", "drafting"],
    evidence: [
      { id: "e08a", when: "9d ago",  source: "Activity log", ref: "log/2026-05-02", snippet: "Drafting session 18:14–19:48 PT." },
      { id: "e08b", when: "1m ago",  source: "Activity log", ref: "log/2026-04-07", snippet: "Same window; matching doc title pattern." },
      { id: "e08c", when: "2m ago",  source: "Activity log", ref: "log/2026-03-03", snippet: "Cadence confirmed across 3 cycles." },
    ],
  },
  {
    id: "m09", subject: "people", subjectLabel: "Dev Patel — escalation contact",
    fact: "Dev is the on-call escalation for ops-bot incidents after 6pm PT weekdays.",
    confidence: 76, lastTouchedHours: 24 * 17, lastTouchedLabel: "Last seen 17d ago", verified: false,
    tags: ["on-call"],
    evidence: [
      { id: "e09a", when: "17d ago", source: "Email thread", ref: "thread/2026-04-24", snippet: "“Dev's got it after 6 weekdays.”" },
      { id: "e09b", when: "5w ago",  source: "Pager log",    ref: "pager/2026-04-08",  snippet: "Dev acked at 19:22." },
    ],
  },
  {
    id: "m10", subject: "people", subjectLabel: "Olivia Chen — comms cadence",
    fact: "Olivia replies within an hour during US-East business hours; expect 12-24h elsewhere.",
    confidence: 72, lastTouchedHours: 24 * 12, lastTouchedLabel: "Last seen 12d ago", verified: false,
    tags: ["responsiveness"],
    evidence: [
      { id: "e10a", when: "12d ago", source: "Reply latency log", ref: "metrics/olivia.reply", snippet: "Median reply 47m within ET business window; 18h overnight." },
    ],
  },
  {
    id: "m11", subject: "accounts", subjectLabel: "Trent & Co — billing",
    fact: "Trent & Co bills NET-30 by wire; preferred payment day is the 20th of the month.",
    confidence: 89, lastTouchedHours: 24 * 6, lastTouchedLabel: "Verified 6d ago", verified: true,
    tags: ["ap"],
    evidence: [
      { id: "e11a", when: "6d ago",  source: "Invoice history", ref: "invoices/trent",   snippet: "12/12 invoices last year paid on or before the 20th." },
      { id: "e11b", when: "3m ago",  source: "Direct statement", ref: "thread/2026-02-10", snippet: "“Around the 20th works best on our end.”" },
    ],
  },
  {
    id: "m12", subject: "preferences", subjectLabel: "You — notification style",
    fact: "You want one daily digest at 8am PT; no per-event push notifications.",
    confidence: 96, lastTouchedHours: 4, lastTouchedLabel: "Verified 4h ago", verified: true,
    tags: ["notifications"],
    evidence: [
      { id: "e12a", when: "4h ago", source: "Settings", ref: "settings/notifications", snippet: "Per-event push disabled; daily digest enabled at 08:00 PT." },
      { id: "e12b", when: "2m ago", source: "Direct statement", ref: "thread/2026-03-15", snippet: "“Stop pinging me — just send one digest in the morning.”" },
    ],
  },
  {
    id: "m13", subject: "patterns", subjectLabel: "Inbox — known senders auto-triage",
    fact: "Mail from 14 known senders is auto-archived after triage with no draft reply required.",
    confidence: 80, lastTouchedHours: 24 * 2, lastTouchedLabel: "Verified 2d ago", verified: true,
    tags: ["inbox", "automation"],
    evidence: [
      { id: "e13a", when: "2d ago", source: "Triage log", ref: "triage/known-senders", snippet: "14 senders auto-archived in trailing 7 days; 0 user reversals." },
    ],
  },
  {
    id: "m14", subject: "patterns", subjectLabel: "Research — pricing watch cadence",
    fact: "You read the pricing-watch digest within 2 hours of it landing on weekdays.",
    confidence: 68, lastTouchedHours: 24 * 35, lastTouchedLabel: "Last seen 35d ago", verified: false,
    tags: ["research", "stale"],
    evidence: [
      { id: "e14a", when: "35d ago", source: "Read log", ref: "metrics/pricing-watch.read", snippet: "Open within 2h on 11 of last 14 weekdays; pattern weakening." },
    ],
  },
  {
    id: "m15", subject: "people", subjectLabel: "Sarah Lopez — direct statement preference",
    fact: "Sarah dislikes being CC'd on FYI threads; she prefers a 5pm-Friday weekly summary instead.",
    confidence: 90, lastTouchedHours: 24 * 4, lastTouchedLabel: "Verified 4d ago", verified: true,
    tags: ["exec", "comms"],
    evidence: [
      { id: "e15a", when: "4d ago", source: "Direct statement", ref: "thread/2026-05-07", snippet: "“Drop me from FYI threads, just send the Friday roll-up.”" },
    ],
  },
  {
    id: "m16", subject: "accounts", subjectLabel: "Yates Holdings — banking",
    fact: "Yates Holdings updated bank routing on May 10, 2026; old routing on file is stale.",
    confidence: 86, lastTouchedHours: 26, lastTouchedLabel: "Verified yesterday", verified: true,
    tags: ["alert", "ap"],
    evidence: [
      { id: "e16a", when: "yesterday", source: "IRS public filings", ref: "irs.gov/filings/yates", snippet: "W-9 amendment filed; new routing reported." },
      { id: "e16b", when: "yesterday", source: "Vendor email",       ref: "ops@yatesholdings.com", snippet: "“Please update payment details to attached.”" },
    ],
  },
  {
    id: "m17", subject: "preferences", subjectLabel: "You — quiet hours",
    fact: "You don't want assistant actions executed between 11pm and 6am PT (drafts okay; sends not).",
    confidence: 93, lastTouchedHours: 24 * 5, lastTouchedLabel: "Verified 5d ago", verified: true,
    tags: ["policy"],
    evidence: [
      { id: "e17a", when: "5d ago", source: "Settings",        ref: "settings/quiet-hours",  snippet: "Quiet hours configured 23:00–06:00 PT, drafts only." },
      { id: "e17b", when: "2m ago", source: "Direct statement", ref: "thread/2026-03-18",    snippet: "“Hold sends overnight, drafts are fine.”" },
    ],
  },
  {
    id: "m18", subject: "patterns", subjectLabel: "Calendar — 3pm context-switch slump",
    fact: "Between 3pm and 4pm PT you're 3× more likely to defer decisions to “tomorrow”.",
    confidence: 64, lastTouchedHours: 24 * 41, lastTouchedLabel: "Last seen 41d ago", verified: false,
    tags: ["energy", "low-confidence"],
    evidence: [
      { id: "e18a", when: "41d ago", source: "Decision log", ref: "metrics/defer.rate", snippet: "Defer rate 3.1× baseline in the 15:00–16:00 window over 90 days." },
    ],
  },
  {
    id: "m19", subject: "people", subjectLabel: "Priya Shah — review style",
    fact: "Priya reviews drafts in batches every Tuesday and Friday morning.",
    confidence: 78, lastTouchedHours: 24 * 8, lastTouchedLabel: "Verified 8d ago", verified: true,
    tags: ["cadence"],
    evidence: [
      { id: "e19a", when: "8d ago", source: "Review log", ref: "reviews/priya",        snippet: "11/12 last reviews landed Tue/Fri 09:00–10:30." },
      { id: "e19b", when: "1m ago", source: "Direct statement", ref: "thread/2026-04-10", snippet: "“I batch reviews Tuesdays and Fridays.”" },
    ],
  },
  {
    id: "m20", subject: "accounts", subjectLabel: "Atlas Logistics — risk profile",
    fact: "Atlas Logistics flagged as elevated counterparty risk pending Q2 covenant disclosure.",
    confidence: 67, lastTouchedHours: 24 * 32, lastTouchedLabel: "Last seen 32d ago", verified: false,
    tags: ["risk", "stale"],
    evidence: [
      { id: "e20a", when: "32d ago", source: "Analyst note", ref: "moodys.com/notes/atlas-2026", snippet: "Watchlist with negative implication; awaiting Q2 detail." },
    ],
  },
  {
    id: "m21", subject: "preferences", subjectLabel: "You — language",
    fact: "English (US) only for outbound communications; never auto-translate to other languages.",
    confidence: 98, lastTouchedHours: 24 * 3, lastTouchedLabel: "Verified 3d ago", verified: true,
    tags: ["voice"],
    evidence: [
      { id: "e21a", when: "3d ago", source: "Settings", ref: "settings/locale", snippet: "outboundLocale = en-US, autoTranslate = false." },
    ],
  },
  {
    id: "m22", subject: "patterns", subjectLabel: "Drafts — re-edit ratio",
    fact: "Drafts you accept on first pass: 38%. Drafts you accept after one edit: 47%. Two edits: 12%.",
    confidence: 75, lastTouchedHours: 24 * 7, lastTouchedLabel: "Verified 7d ago", verified: true,
    tags: ["drafting", "metric"],
    evidence: [
      { id: "e22a", when: "7d ago", source: "Edit log", ref: "metrics/draft.edits", snippet: "Over 412 drafts in trailing 60 days; distribution stable ±2pp." },
    ],
  },
];

const STALE_HOURS = 30 * 24; // >30 days → re-verify

// ───────────────────────────────────────────────────────────────────────────
//  Static maps
// ───────────────────────────────────────────────────────────────────────────

const SUBJECT_META: Record<Subject, { label: string; tone: "accent" | "recovered" | "quarantined" | "warning"; node: string }> = {
  people:      { label: "People",      tone: "accent",      node: "P" },
  accounts:    { label: "Accounts",    tone: "recovered",   node: "A" },
  preferences: { label: "Preferences", tone: "quarantined", node: "•" },
  patterns:    { label: "Patterns",    tone: "warning",     node: "~" },
};

const FRESHNESS_META: Record<FreshnessBucket, { label: string; max: number }> = {
  hours:  { label: "Hours",  max: 24            },
  days:   { label: "Days",   max: 24 * 7        },
  weeks:  { label: "Weeks",  max: 24 * 30       },
  months: { label: "Months", max: Number.MAX_SAFE_INTEGER },
};

function freshnessOf(hours: number): FreshnessBucket {
  if (hours < 24)        return "hours";
  if (hours < 24 * 7)    return "days";
  if (hours < 24 * 30)   return "weeks";
  return "months";
}

function confidenceTone(c: number): "safe" | "warning" | "blocked" {
  if (c >= 85) return "safe";
  if (c >= 70) return "warning";
  return "blocked";
}

// ───────────────────────────────────────────────────────────────────────────
//  Inline icons
// ───────────────────────────────────────────────────────────────────────────

const Svg = ({ d, size = 16, className = "" }:
  { d: React.ReactNode; size?: number; className?: string }) => (
  <svg viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor"
       strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden="true">
    {d}
  </svg>
);
type IconProps = { size?: number; className?: string };
const ICON = {
  Search:   (p: IconProps) => <Svg {...p} d={<><circle cx="11" cy="11" r="6" /><path d="m20 20-4.3-4.3" /></>} />,
  X:        (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Check:    (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  ChevronDown: (p: IconProps) => <Svg {...p} d={<path d="m6 9 6 6 6-6" />} />,
  ChevronUp:   (p: IconProps) => <Svg {...p} d={<path d="m6 15 6-6 6 6" />} />,
  Clock:    (p: IconProps) => <Svg {...p} d={<><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>} />,
  Pencil:   (p: IconProps) => <Svg {...p} d={<><path d="m4 20 4-1 11-11-3-3L5 16l-1 4Z" /></>} />,
  Trash:    (p: IconProps) => <Svg {...p} d={<><path d="M4 7h16" /><path d="M9 7V4h6v3" /><path d="M6 7l1 13h10l1-13" /></>} />,
  Tag:      (p: IconProps) => <Svg {...p} d={<><path d="M3 12V4h8l10 10-8 8-10-10Z" /><circle cx="8" cy="8" r="1.5" /></>} />,
  Eye:      (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>} />,
  AlertTriangle: (p: IconProps) => <Svg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx="12" cy="17.5" r=".5" fill="currentColor" stroke="none" /></>} />,
  Undo:     (p: IconProps) => <Svg {...p} d={<><path d="M9 14H4v-5" /><path d="M4 14a8 8 0 1 1 2.5 5.7" /></>} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  List:     (p: IconProps) => <Svg {...p} d={<><path d="M8 6h13M8 12h13M8 18h13" /><circle cx="4" cy="6" r="1" fill="currentColor" stroke="none" /><circle cx="4" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="4" cy="18" r="1" fill="currentColor" stroke="none" /></>} />,
  Graph:    (p: IconProps) => <Svg {...p} d={<><circle cx="6" cy="6" r="2" /><circle cx="18" cy="6" r="2" /><circle cx="12" cy="18" r="2" /><circle cx="5" cy="14" r="1.5" /><path d="m8 7 8 0M7 7l4 9M17 7l-4 9M6 8l-.5 5M12 16l-7-1" /></>} />,
  Link:     (p: IconProps) => <Svg {...p} d={<><path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1" /><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1" /></>} />,
};

// ───────────────────────────────────────────────────────────────────────────
//  Small chips
// ───────────────────────────────────────────────────────────────────────────

const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(" ");

function ConfidencePill({ value }: { value: number }) {
  const tone = confidenceTone(value);
  return (
    <span className={cx(
      "inline-flex items-center gap-1.5 rounded-full border font-medium tabular-nums text-[10.5px] px-1.5 py-0.5",
      `bg-${tone}`, `text-${tone}`, `border-${tone}`,
    )}>
      <span className={cx("h-1.5 w-1.5 rounded-full", `bg-${tone}-solid`)} />
      {value}% confidence
    </span>
  );
}

function SubjectChip({ subject }: { subject: Subject }) {
  const m = SUBJECT_META[subject];
  return (
    <span className={cx(
      "inline-flex items-center gap-1 rounded-full border text-[10.5px] font-medium px-1.5 py-0.5",
      `bg-${m.tone}`, `text-${m.tone}`, `border-${m.tone}`,
    )}>
      {m.label}
    </span>
  );
}

function FreshnessLabel({ item }: { item: MemoryItem }) {
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

function ReVerifyPill({ item }: { item: MemoryItem }) {
  if (item.lastTouchedHours <= STALE_HOURS) return null;
  const days = Math.round(item.lastTouchedHours / 24);
  return (
    <span className="inline-flex items-center gap-1 rounded-md border border-warning bg-warning text-warning text-[10.5px] font-medium px-1.5 py-0.5">
      <ICON.AlertTriangle size={11} />
      Re-verify · {days}d untouched
    </span>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Evidence trail (inline)
// ───────────────────────────────────────────────────────────────────────────

function EvidenceTrail({ items }: { items: Evidence[] }) {
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

// ───────────────────────────────────────────────────────────────────────────
//  Memory card
// ───────────────────────────────────────────────────────────────────────────

function MemoryCard({
  item, expanded, onToggleExpand, onCorrect, onForget, onTag,
}: {
  item: MemoryItem;
  expanded: boolean;
  onToggleExpand: () => void;
  onCorrect: () => void;
  onForget: () => void;
  onTag: () => void;
}) {
  return (
    <article className="card overflow-hidden">
      <div className="px-4 py-3.5 sm:px-5 flex flex-col gap-2.5">
        {/* Header: subject label + chips */}
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

        {/* The fact in plain language */}
        <p className="text-[14px] text-neutral-900 leading-relaxed">
          {item.fact}
        </p>

        {/* Meta row */}
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
          <button type="button" onClick={onToggleExpand}
                  className="inline-flex items-center gap-0.5 text-[12px] font-medium text-accent hover:text-accent-solid">
            Why do you know this?
            {expanded ? <ICON.ChevronUp size={12} /> : <ICON.ChevronDown size={12} />}
          </button>
        </div>

        {/* Inline evidence trail */}
        {expanded && (
          <div className="mt-1 rounded-lg bg-neutral-50/60 border border-neutral-100 px-3 py-3">
            <div className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 mb-2">
              Evidence trail
            </div>
            <EvidenceTrail items={item.evidence} />
          </div>
        )}
      </div>

      {/* Footer: text-only actions. "Forget this" is ALWAYS visible. */}
      <footer className="border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/60">
        <div className="flex items-center gap-1">
          <button type="button" onClick={onCorrect}
                  className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-neutral-900">
            <ICON.Pencil size={12} /> Correct this
          </button>
          <button type="button" onClick={onForget}
                  className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-blocked">
            <ICON.Trash size={12} /> Forget this
          </button>
          <button type="button" onClick={onTag}
                  className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-neutral-900">
            <ICON.Tag size={12} /> Tag
          </button>
        </div>
        <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">{item.id.toUpperCase()}</span>
      </footer>
    </article>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Facet pills
// ───────────────────────────────────────────────────────────────────────────

type SubjectFilter = "all" | Subject;
type FreshnessFilter = "all" | FreshnessBucket;

function Pill({ active, children, onClick, count }:
  { active: boolean; children: React.ReactNode; onClick: () => void; count?: number }) {
  return (
    <button type="button" onClick={onClick} className={cx(
      "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
      active
        ? "bg-neutral-900 text-white border-neutral-900"
        : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
    )}>
      {children}
      {typeof count === "number" && (
        <span className={cx(
          "rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px",
          active ? "bg-white/20 text-white" : "bg-neutral-100 text-neutral-500",
        )}>
          {count}
        </span>
      )}
    </button>
  );
}

function FacetRow({
  label, children,
}: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 shrink-0 w-[68px]">
        {label}
      </span>
      <div className="flex items-center gap-1.5 flex-wrap">{children}</div>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Search + view toggle
// ───────────────────────────────────────────────────────────────────────────

function SearchBar({ value, onChange, inputRef }: {
  value: string;
  onChange: (v: string) => void;
  inputRef: React.RefObject<HTMLInputElement>;
}) {
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
        <button type="button" onClick={() => onChange("")}
                aria-label="Clear search"
                className="absolute right-2 top-1/2 -translate-y-1/2 h-6 w-6 inline-flex items-center justify-center rounded-md text-neutral-400 hover:text-neutral-900 hover:bg-neutral-100">
          <ICON.X size={12} />
        </button>
      )}
    </div>
  );
}

function ViewToggle({ view, onChange }: { view: "list" | "graph"; onChange: (v: "list" | "graph") => void }) {
  return (
    <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
      <button type="button" onClick={() => onChange("list")}
              className={cx("inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors",
                view === "list" ? "bg-neutral-900 text-white" : "text-neutral-600 hover:text-neutral-900")}>
        <ICON.List size={12} /> List
      </button>
      <button type="button" onClick={() => onChange("graph")}
              className={cx("inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[12px] font-medium transition-colors",
                view === "graph" ? "bg-neutral-900 text-white" : "text-neutral-600 hover:text-neutral-900")}>
        <ICON.Graph size={12} /> Graph
      </button>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Empty states
// ───────────────────────────────────────────────────────────────────────────

function EmptyState({ kind, query }: { kind: "no-memory" | "no-match"; query?: string }) {
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

// ───────────────────────────────────────────────────────────────────────────
//  Forgotten banner (30-day undo)
// ───────────────────────────────────────────────────────────────────────────

function ForgetBanner({ item, onUndo, onDismiss }: {
  item: MemoryItem; onUndo: () => void; onDismiss: () => void;
}) {
  return (
    <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50">
      <div className="rounded-xl bg-neutral-900 text-white shadow-lg px-3.5 py-2.5 flex items-center gap-3 max-w-[640px]">
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-md bg-white/10 shrink-0">
          <ICON.Trash size={13} />
        </span>
        <div className="text-[12.5px] leading-snug min-w-0">
          <div className="font-medium truncate">Moved to Forgotten · {item.subjectLabel}</div>
          <div className="text-white/70 text-[11.5px]">30 days to restore from Settings → Forgotten memory.</div>
        </div>
        <button type="button" onClick={onUndo}
                className="inline-flex items-center gap-1 rounded-md bg-white text-neutral-900 hover:bg-white/90 px-2.5 py-1 text-[12px] font-semibold">
          <ICON.Undo size={12} /> Undo
        </button>
        <button type="button" onClick={onDismiss} aria-label="Dismiss"
                className="inline-flex items-center justify-center h-7 w-7 rounded-md text-white/60 hover:text-white hover:bg-white/10">
          <ICON.X size={13} />
        </button>
      </div>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Graph view (placeholder per spec — DO NOT pull in d3/vis-network)
// ───────────────────────────────────────────────────────────────────────────

function GraphView({ items }: { items: MemoryItem[] }) {
  // Tiny structural sketch: legend chips + a placeholder canvas the
  // integrator will replace with a force-directed network.
  const counts: Record<Subject, number> = { people: 0, accounts: 0, preferences: 0, patterns: 0 };
  for (const it of items) counts[it.subject]++;
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
            return (
              <span key={s} className={cx(
                "inline-flex items-center gap-1.5 rounded-full border text-[11px] font-medium px-2 py-0.5",
                `bg-${m.tone}`, `text-${m.tone}`, `border-${m.tone}`,
              )}>
                <span className={cx("h-1.5 w-1.5 rounded-full", `bg-${m.tone}-solid`)} />
                {m.label}
                <span className="font-mono tabular-nums opacity-70">{counts[s]}</span>
              </span>
            );
          })}
        </div>
      </div>

      {/* Placeholder canvas */}
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

// ───────────────────────────────────────────────────────────────────────────
//  Memory — the route
// ───────────────────────────────────────────────────────────────────────────

export function Memory(): JSX.Element {
  const [allItems, setAllItems] = useState<MemoryItem[]>(MOCK_MEMORY);
  const [subject, setSubject]     = useState<SubjectFilter>("all");
  const [freshness, setFreshness] = useState<FreshnessFilter>("all");
  const [query, setQuery]         = useState("");
  const [view, setView]           = useState<"list" | "graph">("list");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  // Forget queue (30-day undo). Items in `forgotten` are NOT in `allItems`.
  const [forgotten, setForgotten] = useState<MemoryItem[]>([]);
  const [lastForgotten, setLastForgotten] = useState<MemoryItem | null>(null);

  const [toast, setToast] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Cmd/Ctrl-K focuses search.
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
    if (!toast) return;
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  // Counts (always against the full live set, not the filtered slice).
  const subjectCounts: Record<SubjectFilter, number> = useMemo(() => {
    const out: Record<SubjectFilter, number> = {
      all: allItems.length, people: 0, accounts: 0, preferences: 0, patterns: 0,
    };
    for (const it of allItems) out[it.subject]++;
    return out;
  }, [allItems]);

  const freshnessCounts: Record<FreshnessFilter, number> = useMemo(() => {
    const out: Record<FreshnessFilter, number> = {
      all: allItems.length, hours: 0, days: 0, weeks: 0, months: 0,
    };
    for (const it of allItems) out[freshnessOf(it.lastTouchedHours)]++;
    return out;
  }, [allItems]);

  // Apply search + facets.
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

  // Sort: stale items get demoted (freshness-first: surface the verified
  // ones; show stale at the end so they look correctable).
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

  const handleCorrect = (it: MemoryItem) => {
    setToast(`Correct · ${it.subjectLabel}`);
  };
  const handleTag = (it: MemoryItem) => {
    setToast(`Add tag · ${it.subjectLabel}`);
  };

  const previewEmpty = allItems.length === 0;

  return (
    <main className="min-h-screen bg-app">
      <div className="page-container max-w-[78rem]">
        {/* Title + view toggle */}
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
              What the system has learned about this workspace.
              Every fact has evidence and a trail you can inspect or correct in one click.
            </p>
          </div>
          <ViewToggle view={view} onChange={setView} />
        </div>

        {/* Search */}
        <div className="mt-5 flex items-center justify-between gap-3 flex-wrap">
          <SearchBar value={query} onChange={setQuery} inputRef={inputRef} />
          <kbd className="hidden md:inline-flex items-center gap-1 rounded border border-neutral-200 bg-white text-[10.5px] font-mono text-neutral-500 px-1.5 py-0.5">
            ⌘ K
          </kbd>
        </div>

        {/* Facets */}
        <div className="mt-4 card card-padded flex flex-col gap-3">
          <FacetRow label="Subject">
            <Pill active={subject === "all"} onClick={() => setSubject("all")} count={subjectCounts.all}>
              All
            </Pill>
            {(Object.keys(SUBJECT_META) as Subject[]).map((s) => (
              <Pill key={s}
                    active={subject === s}
                    onClick={() => setSubject(s)}
                    count={subjectCounts[s]}>
                {SUBJECT_META[s].label}
              </Pill>
            ))}
          </FacetRow>
          <FacetRow label="Freshness">
            <Pill active={freshness === "all"} onClick={() => setFreshness("all")} count={freshnessCounts.all}>
              All
            </Pill>
            {(Object.keys(FRESHNESS_META) as FreshnessBucket[]).map((f) => (
              <Pill key={f}
                    active={freshness === f}
                    onClick={() => setFreshness(f)}
                    count={freshnessCounts[f]}>
                {FRESHNESS_META[f].label}
              </Pill>
            ))}
          </FacetRow>
        </div>

        {/* Body */}
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
      </div>

      {/* Undo banner — appears for the most recent forget action */}
      {lastForgotten && (
        <ForgetBanner
          item={lastForgotten}
          onUndo={handleUndoForget}
          onDismiss={() => setLastForgotten(null)}
        />
      )}

      {/* Generic toast */}
      {toast && !lastForgotten && (
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

;(window as unknown as { Memory: typeof Memory }).Memory = Memory;
