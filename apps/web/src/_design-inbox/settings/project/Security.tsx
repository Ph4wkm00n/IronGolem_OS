// Security.tsx — IronGolem OS
// Route: /security
// One-file route. Five-layer safety model + audit trail + policy library.
//
// React 19, TS strict, Tailwind utility classes only.
//
// Mandatory patterns:
//   1. Visible Trust — every audit entry has a one-sentence cause.
//   2. Explainable Autonomy — "Open rule that caught this" is one click
//      away on every audit entry.
//   3. Progressive disclosure — the policy editor lives in a drawer.
//      Rule history + testing live in the drawer, not on the main surface.
//   4. Reversibility — pause is preferred over delete; every action has a
//      30-day undo via the Forgotten/Reverse bucket banner.

import * as React from "react";
const { useState, useMemo, useEffect } = React;

// ───────────────────────────────────────────────────────────────────────────
//  Types
// ───────────────────────────────────────────────────────────────────────────

type LayerId = 1 | 2 | 3 | 4 | 5;
type LayerState = "ok" | "watching" | "paused" | "failed";

type Layer = {
  id: LayerId;
  name: string;
  state: LayerState;
  blurb: string;       // plain-language explanation
  governs: number;     // count of actions currently governed
  examples: string[];  // 2-3 examples
};

type AuditKind = "blocked" | "quarantined";
type Scope = "scoped" | "broad" | "restricted";

type AuditEntry = {
  id: string;
  when: string;
  whenIso: string;
  kind: AuditKind;
  title: string;
  cause: string;            // ONE sentence cause
  permission: string;       // permission that was checked
  scope: Scope;
  layer: LayerId;
  ruleId: string;
  ruleName: string;
  deniedBy: "system" | "you";
  details: string;          // longer explanation
};

type PolicyState = "active" | "paused" | "under-review";

type Policy = {
  id: string;
  name: string;
  purpose: string;
  state: PolicyState;
  layer: LayerId;
  ruleText: string;         // pseudo DSL
  triggeredLast30d: number;
  history: { when: string; what: string }[];
  relatedAuditIds: string[];
};

// ───────────────────────────────────────────────────────────────────────────
//  Mock data
// ───────────────────────────────────────────────────────────────────────────

const LAYERS: Layer[] = [
  { id: 1, name: "Identity", state: "ok", governs: 47,
    blurb: "Who is asking — operator, team, or external sender — and whether their session is current.",
    examples: ["Session valid, MFA satisfied", "Sender domain on the allowlist"] },
  { id: 2, name: "Workspace", state: "ok", governs: 31,
    blurb: "Workspace-wide guardrails: quiet hours, data residency, environment (sandbox vs production).",
    examples: ["Quiet hours 23:00–06:00 PT", "Data residency: US-only"] },
  { id: 3, name: "Team", state: "watching", governs: 18,
    blurb: "Per-team limits on what an assistant team can do without you. Inbox can draft; Operations needs sign-off above $50.",
    examples: ["Operations auto-approve cap $50", "Inbox: drafts only, no sends"] },
  { id: 4, name: "Action", state: "ok", governs: 64,
    blurb: "Per-action checks: tools required, allowed APIs, side effects, rate limits.",
    examples: ["Wire transfers: restricted", "External email send: scoped per thread"] },
  { id: 5, name: "Outcome", state: "ok", governs: 22,
    blurb: "Post-hoc checks on what changed. Bad outcomes trigger rollback and quarantine.",
    examples: ["Diff size limit on docs", "Auto-rollback on PII leak"] },
];

const POLICIES: Policy[] = [
  { id: "pol-01", name: "Standing-order purchases ≤ $50 auto-approve",
    purpose: "Lets routine purchases under $50 go through without your sign-off.",
    state: "active", layer: 3,
    ruleText: "when action == 'purchase.create' and amount.usd <= 50 and vendor.in standing_orders\n  then allow with audit",
    triggeredLast30d: 18,
    history: [
      { when: "May 5, 2026", what: "Threshold raised from $40 → $50 (you)" },
      { when: "Mar 10, 2026", what: "Policy created" },
    ],
    relatedAuditIds: ["a04"] },
  { id: "pol-02", name: "Quiet hours: drafts only, no sends",
    purpose: "Pauses outbound sends between 11pm and 6am PT. Drafts continue.",
    state: "active", layer: 2,
    ruleText: "when local_time.in 23:00..06:00\n  then block 'send.external' and allow 'draft.create'",
    triggeredLast30d: 7,
    history: [{ when: "Mar 18, 2026", what: "Policy created" }],
    relatedAuditIds: ["a02", "a17"] },
  { id: "pol-03", name: "Wire transfers require restricted scope",
    purpose: "Any wire transfer must be initiated by you and use a restricted-scope token.",
    state: "active", layer: 4,
    ruleText: "when action == 'wire.transfer'\n  then require token.scope == 'restricted'\n   and require initiator == 'operator'",
    triggeredLast30d: 2,
    history: [
      { when: "Feb 2, 2026", what: "Tightened: now requires operator initiation" },
      { when: "Jan 5, 2026", what: "Policy created" },
    ],
    relatedAuditIds: ["a09", "a23"] },
  { id: "pol-04", name: "External email: scoped per thread",
    purpose: "Drafting team can reply within an existing thread but cannot start a new external thread without you.",
    state: "active", layer: 4,
    ruleText: "when action == 'email.send.external'\n  then require thread.parent_id != null",
    triggeredLast30d: 5,
    history: [{ when: "Apr 14, 2026", what: "Policy created" }],
    relatedAuditIds: ["a07", "a11"] },
  { id: "pol-05", name: "Diff-size cap on docs (Outcome)",
    purpose: "Auto-rolls back any document edit that changes more than 40% in a single revision.",
    state: "under-review", layer: 5,
    ruleText: "when action == 'doc.edit' and diff.pct > 0.40\n  then rollback and quarantine",
    triggeredLast30d: 3,
    history: [
      { when: "May 9, 2026", what: "Marked under-review — 3 false positives this month" },
      { when: "Jan 22, 2026", what: "Policy created" },
    ],
    relatedAuditIds: ["a13"] },
  { id: "pol-06", name: "Known-sender allowlist",
    purpose: "Auto-archive triage-only mail from 14 known senders.",
    state: "active", layer: 1,
    ruleText: "when email.from.in known_senders\n  then route 'triage.archive'",
    triggeredLast30d: 142,
    history: [{ when: "Feb 1, 2026", what: "Policy created" }],
    relatedAuditIds: [] },
  { id: "pol-07", name: "PII leak detector → quarantine",
    purpose: "Quarantines any draft containing apparent SSN or full credit-card numbers.",
    state: "active", layer: 5,
    ruleText: "when draft.contains(pii.ssn | pii.cc)\n  then quarantine and notify",
    triggeredLast30d: 1,
    history: [{ when: "Dec 1, 2025", what: "Policy created" }],
    relatedAuditIds: ["a15"] },
  { id: "pol-08", name: "Telegram connector — paused while re-auth pending",
    purpose: "Holds outbound Telegram sends until the bot session is re-authorized.",
    state: "paused", layer: 4,
    ruleText: "when connector.telegram.session.invalid\n  then pause connector.telegram.outbound",
    triggeredLast30d: 4,
    history: [
      { when: "yesterday", what: "Auto-paused after upstream session invalidation" },
      { when: "Nov 12, 2025", what: "Policy created" },
    ],
    relatedAuditIds: ["a01", "a08"] },
];

const AUDIT: AuditEntry[] = [
  { id: "a01", when: "1h ago", whenIso: "11:02 PT", kind: "quarantined", deniedBy: "system",
    title: "Telegram outbound batch held",
    cause: "Bot session was invalidated upstream and outbound retries would have hit a dead endpoint.",
    permission: "connector.telegram.send", scope: "scoped", layer: 4,
    ruleId: "pol-08", ruleName: "Telegram connector — paused while re-auth pending",
    details: "4 messages currently queued. The connector quarantined itself and will resume when you re-authorize." },
  { id: "a02", when: "2h ago", whenIso: "10:14 PT", kind: "blocked", deniedBy: "system",
    title: "Drafting team tried to send during quiet hours",
    cause: "Local time was inside the configured quiet-hours window (23:00–06:00 PT) and the policy allows drafts but blocks sends.",
    permission: "email.send.external", scope: "scoped", layer: 2,
    ruleId: "pol-02", ruleName: "Quiet hours: drafts only, no sends",
    details: "The draft is preserved and queued for 06:01 PT delivery." },
  { id: "a03", when: "3h ago", whenIso: "08:48 PT", kind: "blocked", deniedBy: "you",
    title: "You denied an auto-approve on a $612 standing order",
    cause: "Amount was above the $50 auto-approve cap and you reviewed it manually.",
    permission: "purchase.create", scope: "scoped", layer: 3,
    ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve",
    details: "Order PO-26-118 routed to your inbox; you marked it 'review next week'." },
  { id: "a04", when: "5h ago", whenIso: "06:48 PT", kind: "blocked", deniedBy: "system",
    title: "Auto-approve denied: vendor not on standing-orders list",
    cause: "Vendor 'Brisbane Imports' is not in the standing-orders list, so the auto-approve rule did not apply.",
    permission: "purchase.create", scope: "scoped", layer: 3,
    ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve",
    details: "Routed to your inbox for one-tap approval." },
  { id: "a05", when: "yesterday", whenIso: "May 11 21:02 PT", kind: "blocked", deniedBy: "system",
    title: "Webhook receiver throttled by policy",
    cause: "Receiver p95 exceeded 4s for 3 consecutive checks; the rate-limit policy reduced concurrency.",
    permission: "webhook.send", scope: "broad", layer: 4,
    ruleId: "pol-04", ruleName: "External email: scoped per thread",
    details: "Concurrency dropped 4 → 1 for 90 seconds; receiver recovered without operator action." },
  { id: "a06", when: "yesterday", whenIso: "May 11 16:01 PT", kind: "blocked", deniedBy: "system",
    title: "Auth probe failed: failed over to backup provider",
    cause: "Primary auth provider returned 502 on 8/10 probes within 60 seconds.",
    permission: "identity.session.refresh", scope: "restricted", layer: 1,
    ruleId: "pol-06", ruleName: "Known-sender allowlist",
    details: "Failover lasted 11 minutes; primary recovered and we re-pinned." },
  { id: "a07", when: "yesterday", whenIso: "May 11 15:18 PT", kind: "blocked", deniedBy: "system",
    title: "Drafting team tried to start a new external thread",
    cause: "Action lacked a parent thread id; per policy, the team can reply within threads but not start new ones.",
    permission: "email.send.external", scope: "scoped", layer: 4,
    ruleId: "pol-04", ruleName: "External email: scoped per thread",
    details: "Draft saved for your review at /inbox." },
  { id: "a08", when: "yesterday", whenIso: "May 11 14:02 PT", kind: "quarantined", deniedBy: "system",
    title: "Telegram session quarantined after upstream change",
    cause: "Upstream client 5.1.4 invalidated the long-lived bot session, so the connector quarantined itself.",
    permission: "connector.telegram.session", scope: "restricted", layer: 4,
    ruleId: "pol-08", ruleName: "Telegram connector — paused while re-auth pending",
    details: "Quarantine prevents retry churn until you re-authorize." },
  { id: "a09", when: "2d ago", whenIso: "May 10 10:32 PT", kind: "blocked", deniedBy: "you",
    title: "You denied a wire transfer initiated by a recipe",
    cause: "Wire transfers require operator initiation; the recipe attempted to act on your behalf.",
    permission: "wire.transfer", scope: "restricted", layer: 4,
    ruleId: "pol-03", ruleName: "Wire transfers require restricted scope",
    details: "The recipe is paused pending your review." },
  { id: "a10", when: "2d ago", whenIso: "May 10 09:11 PT", kind: "blocked", deniedBy: "system",
    title: "Sender domain not on allowlist",
    cause: "Inbound mail came from a domain that does not match any allowlisted sender pattern.",
    permission: "identity.sender.verify", scope: "scoped", layer: 1,
    ruleId: "pol-06", ruleName: "Known-sender allowlist",
    details: "Routed to triage for your manual review." },
  { id: "a11", when: "2d ago", whenIso: "May 10 08:02 PT", kind: "blocked", deniedBy: "system",
    title: "New thread denied: lacking parent",
    cause: "External-email policy requires a parent thread id and one was not present.",
    permission: "email.send.external", scope: "scoped", layer: 4,
    ruleId: "pol-04", ruleName: "External email: scoped per thread",
    details: "Operator can override; draft preserved." },
  { id: "a12", when: "3d ago", whenIso: "May 9 19:48 PT", kind: "quarantined", deniedBy: "system",
    title: "Document revision quarantined: 62% diff",
    cause: "A single revision changed 62% of the document, which exceeds the diff-size cap.",
    permission: "doc.edit", scope: "broad", layer: 5,
    ruleId: "pol-05", ruleName: "Diff-size cap on docs (Outcome)",
    details: "Auto-rollback restored the previous revision; quarantined copy held in review." },
  { id: "a13", when: "3d ago", whenIso: "May 9 14:22 PT", kind: "quarantined", deniedBy: "system",
    title: "Outcome check rolled back doc edit (false positive flagged)",
    cause: "Diff exceeded the policy threshold, though human review later marked the edit safe.",
    permission: "doc.edit", scope: "broad", layer: 5,
    ruleId: "pol-05", ruleName: "Diff-size cap on docs (Outcome)",
    details: "Marked false positive — contributes to the policy's under-review status." },
  { id: "a14", when: "4d ago", whenIso: "May 8 16:18 PT", kind: "blocked", deniedBy: "system",
    title: "Auto-approve denied: amount $54 exceeded cap",
    cause: "Amount of $54 was $4 above the standing-order auto-approve threshold.",
    permission: "purchase.create", scope: "scoped", layer: 3,
    ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve",
    details: "Routed for one-tap approval; you approved 14 minutes later." },
  { id: "a15", when: "5d ago", whenIso: "May 7 11:05 PT", kind: "quarantined", deniedBy: "system",
    title: "Draft quarantined: matched PII pattern",
    cause: "Draft contained a 9-digit sequence that matched a US SSN pattern.",
    permission: "draft.send", scope: "restricted", layer: 5,
    ruleId: "pol-07", ruleName: "PII leak detector → quarantine",
    details: "Marked false positive after review (sequence was an order number); rule kept as-is." },
  { id: "a16", when: "6d ago", whenIso: "May 6 09:38 PT", kind: "blocked", deniedBy: "system",
    title: "Data-residency policy blocked outbound transfer",
    cause: "Destination region was outside the US, which violates the workspace residency rule.",
    permission: "data.export.external", scope: "restricted", layer: 2,
    ruleId: "pol-02", ruleName: "Quiet hours: drafts only, no sends",
    details: "Destination overridden by the operator after review." },
  { id: "a17", when: "7d ago", whenIso: "May 5 23:48 PT", kind: "blocked", deniedBy: "system",
    title: "Send blocked: quiet hours",
    cause: "Time was 23:48 PT, within the configured quiet-hours window.",
    permission: "email.send.external", scope: "scoped", layer: 2,
    ruleId: "pol-02", ruleName: "Quiet hours: drafts only, no sends",
    details: "Draft preserved; queued for 06:01 delivery." },
  { id: "a18", when: "8d ago", whenIso: "May 4 14:11 PT", kind: "blocked", deniedBy: "you",
    title: "You denied a calendar override for an executive 1:1",
    cause: "The recipe tried to move a meeting flagged 'do not move'; you reviewed and denied.",
    permission: "calendar.event.move", scope: "broad", layer: 3,
    ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve",
    details: "Recipe paused; rule will be reviewed Friday." },
  { id: "a19", when: "9d ago", whenIso: "May 3 17:00 PT", kind: "blocked", deniedBy: "system",
    title: "Inbox tried to send (drafts-only policy)",
    cause: "Inbox team has draft-only scope and cannot send externally.",
    permission: "email.send.external", scope: "scoped", layer: 3,
    ruleId: "pol-04", ruleName: "External email: scoped per thread",
    details: "Draft routed to /inbox for your review." },
  { id: "a20", when: "10d ago", whenIso: "May 2 12:34 PT", kind: "quarantined", deniedBy: "system",
    title: "Outbound link in draft pointed to a flagged domain",
    cause: "A linked domain matched the reputation blocklist; draft was held pending operator review.",
    permission: "email.send.external", scope: "scoped", layer: 5,
    ruleId: "pol-07", ruleName: "PII leak detector → quarantine",
    details: "You released the draft after confirming the domain." },
  { id: "a21", when: "12d ago", whenIso: "Apr 30 09:22 PT", kind: "blocked", deniedBy: "system",
    title: "Broad-scope token rejected for restricted action",
    cause: "Action required a restricted-scope token; only a broad-scope token was offered.",
    permission: "wire.transfer", scope: "restricted", layer: 4,
    ruleId: "pol-03", ruleName: "Wire transfers require restricted scope",
    details: "Recipe paused. You can grant a restricted token from Settings." },
  { id: "a22", when: "14d ago", whenIso: "Apr 28 16:02 PT", kind: "blocked", deniedBy: "you",
    title: "You denied an attachment auto-share",
    cause: "Attachment looked sensitive; you held the share for manual review.",
    permission: "doc.share.external", scope: "scoped", layer: 4,
    ruleId: "pol-04", ruleName: "External email: scoped per thread",
    details: "Marked for redaction; later shared with PII removed." },
  { id: "a23", when: "15d ago", whenIso: "Apr 27 11:00 PT", kind: "blocked", deniedBy: "system",
    title: "Wire transfer blocked: not operator-initiated",
    cause: "Wire transfers require operator initiation; the request came from an internal recipe.",
    permission: "wire.transfer", scope: "restricted", layer: 4,
    ruleId: "pol-03", ruleName: "Wire transfers require restricted scope",
    details: "Recipe paused; operator review pending." },
  { id: "a24", when: "21d ago", whenIso: "Apr 21 13:48 PT", kind: "quarantined", deniedBy: "system",
    title: "Outcome rollback: doc edit changed unrelated section",
    cause: "Edit touched a section outside the requested scope, which exceeded the action's permission.",
    permission: "doc.edit", scope: "broad", layer: 5,
    ruleId: "pol-05", ruleName: "Diff-size cap on docs (Outcome)",
    details: "Quarantined copy retained 30 days; revert was clean." },
  { id: "a25", when: "28d ago", whenIso: "Apr 14 08:11 PT", kind: "blocked", deniedBy: "system",
    title: "Connector access denied: token expired",
    cause: "OAuth refresh token had expired and no operator session was active to re-issue.",
    permission: "connector.calendar.read", scope: "scoped", layer: 1,
    ruleId: "pol-06", ruleName: "Known-sender allowlist",
    details: "Resolved after you re-authenticated the connector." },
];

// ───────────────────────────────────────────────────────────────────────────
//  Maps / helpers
// ───────────────────────────────────────────────────────────────────────────

const LAYER_TONE: Record<LayerState, "safe" | "warning" | "neutral" | "blocked"> = {
  ok: "safe", watching: "warning", paused: "neutral", failed: "blocked",
};
const LAYER_LABEL: Record<LayerState, string> = {
  ok: "OK", watching: "Watching", paused: "Paused", failed: "Failed",
};

const SCOPE_TONE: Record<Scope, "safe" | "warning" | "quarantined"> = {
  scoped: "safe", broad: "warning", restricted: "quarantined",
};

const POLICY_TONE: Record<PolicyState, "safe" | "neutral" | "warning"> = {
  active: "safe", paused: "neutral", "under-review": "warning",
};

const KIND_TONE: Record<AuditKind, "blocked" | "quarantined"> = {
  blocked: "blocked", quarantined: "quarantined",
};

const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(" ");

// ───────────────────────────────────────────────────────────────────────────
//  Icons
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
  Shield:   (p: IconProps) => <Svg {...p} d={<path d="M12 3 4 6v6c0 4.5 3.4 8.4 8 9 4.6-.6 8-4.5 8-9V6l-8-3Z" />} />,
  Lock:     (p: IconProps) => <Svg {...p} d={<><rect x="5" y="11" width="14" height="9" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></>} />,
  X:        (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Check:    (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  ArrowLeft:  (p: IconProps) => <Svg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
  Pencil:   (p: IconProps) => <Svg {...p} d={<><path d="m4 20 4-1 11-11-3-3L5 16l-1 4Z" /></>} />,
  Pause:    (p: IconProps) => <Svg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  Beaker:   (p: IconProps) => <Svg {...p} d={<><path d="M9 3v6L4 19a2 2 0 0 0 2 3h12a2 2 0 0 0 2-3l-5-10V3" /><path d="M9 3h6" /></>} />,
  Clock:    (p: IconProps) => <Svg {...p} d={<><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>} />,
  ChevronDown: (p: IconProps) => <Svg {...p} d={<path d="m6 9 6 6 6-6" />} />,
  ChevronUp:   (p: IconProps) => <Svg {...p} d={<path d="m6 15 6-6 6 6" />} />,
  Eye:      (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  AlertTriangle: (p: IconProps) => <Svg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx="12" cy="17.5" r=".5" fill="currentColor" stroke="none" /></>} />,
  Undo:     (p: IconProps) => <Svg {...p} d={<><path d="M9 14H4v-5" /><path d="M4 14a8 8 0 1 1 2.5 5.7" /></>} />,
};

// ───────────────────────────────────────────────────────────────────────────
//  Chips
// ───────────────────────────────────────────────────────────────────────────

function Chip({ tone, children, dot = false }:
  { tone: "safe" | "warning" | "blocked" | "quarantined" | "neutral" | "accent" | "recovered"; children: React.ReactNode; dot?: boolean }) {
  return (
    <span className={cx(
      "inline-flex items-center gap-1.5 rounded-full border text-[10.5px] font-medium px-1.5 py-0.5",
      `bg-${tone}`, `text-${tone}`, `border-${tone}`,
    )}>
      {dot && <span className={cx("h-1.5 w-1.5 rounded-full", `bg-${tone}-solid`)} />}
      {children}
    </span>
  );
}

function ScopeChip({ scope }: { scope: Scope }) {
  const tone = SCOPE_TONE[scope];
  return <Chip tone={tone}>{scope}</Chip>;
}

function StatusMark({ kind }: { kind: AuditKind }) {
  const tone = KIND_TONE[kind];
  return (
    <span className={cx(
      "inline-flex h-7 w-7 items-center justify-center rounded-md shrink-0",
      `bg-${tone}`, `text-${tone}`,
    )}>
      {kind === "blocked" ? <ICON.X size={13} /> : <ICON.Lock size={12} />}
    </span>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Header
// ───────────────────────────────────────────────────────────────────────────

function SecurityHeader({ layers }: { layers: Layer[] }) {
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

// ───────────────────────────────────────────────────────────────────────────
//  Five-layer PolicyCard
// ───────────────────────────────────────────────────────────────────────────

function LayerRow({ layer }: { layer: Layer }) {
  const tone = LAYER_TONE[layer.state];
  return (
    <li className="px-4 py-3.5 sm:px-5 sm:py-4 flex items-start gap-4">
      {/* Numbered chevron */}
      <div className="shrink-0">
        <div className={cx(
          "h-8 w-8 rounded-md inline-flex items-center justify-center font-mono tabular-nums text-[12px] font-semibold",
          `bg-${tone}`, `text-${tone}`,
        )}>
          {layer.id}
        </div>
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <h3 className="text-[14px] font-semibold tracking-tight text-neutral-900">
            Layer {layer.id} — {layer.name}
          </h3>
          <Chip tone={tone} dot>{LAYER_LABEL[layer.state]}</Chip>
          <span className="text-[10.5px] text-neutral-500 font-mono tabular-nums">
            governs {layer.governs} action{layer.governs === 1 ? "" : "s"}
          </span>
        </div>
        <p className="mt-1 text-[12.5px] text-neutral-600 leading-relaxed">
          {layer.blurb}
        </p>
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

function FiveLayerCard({ layers }: { layers: Layer[] }) {
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

// ───────────────────────────────────────────────────────────────────────────
//  Audit log
// ───────────────────────────────────────────────────────────────────────────

type AuditFilter = "all" | "blocked" | "quarantined" | "by-me" | "24h" | "7d";

function hoursAgo(label: string): number {
  // crude parse just for filter sorting
  if (label.endsWith("h ago")) return parseInt(label, 10);
  if (label === "yesterday") return 24;
  if (label.endsWith("d ago")) return parseInt(label, 10) * 24;
  return 9999;
}

function AuditFilterChips({ value, counts, onChange }:
  { value: AuditFilter; counts: Record<AuditFilter, number>; onChange: (v: AuditFilter) => void }) {
  const opts: { id: AuditFilter; label: string }[] = [
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
          <button key={o.id} type="button" onClick={() => onChange(o.id)}
                  className={cx(
                    "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
                    active
                      ? "bg-neutral-900 text-white border-neutral-900"
                      : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
                  )}>
            {o.label}
            <span className={cx(
              "rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px",
              active ? "bg-white/20 text-white" : "bg-neutral-100 text-neutral-500",
            )}>
              {counts[o.id]}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function AuditRow({ e, onOpenRule }: { e: AuditEntry; onOpenRule: (ruleId: string) => void }) {
  const tone = KIND_TONE[e.kind];
  return (
    <li className="px-4 py-3.5 sm:px-5 flex items-start gap-3">
      <StatusMark kind={e.kind} />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2 flex-wrap">
          <Chip tone={tone}>{e.kind}</Chip>
          <h3 className="text-[13.5px] font-semibold text-neutral-900 leading-snug">{e.title}</h3>
          <span className="text-[10.5px] font-mono text-neutral-400">· {e.whenIso}</span>
          {e.deniedBy === "you" && (
            <span className="text-[10.5px] inline-flex items-center gap-1 text-accent">
              <ICON.Eye size={10} /> Denied by you
            </span>
          )}
        </div>

        {/* Cause — MANDATORY one-sentence */}
        <p className="mt-1 text-[12.5px] text-neutral-700 leading-relaxed">
          <span className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500 mr-1.5">Cause</span>
          {e.cause}
        </p>

        {/* Permission row */}
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1 text-[10.5px] text-neutral-500">
            <ICON.Lock size={10} />
            <span className="font-mono">{e.permission}</span>
          </span>
          <ScopeChip scope={e.scope} />
          <Chip tone="neutral">Layer {e.layer}</Chip>
          <button type="button" onClick={() => onOpenRule(e.ruleId)}
                  className="ml-auto inline-flex items-center gap-0.5 text-[12px] font-medium text-accent hover:text-accent-solid">
            Open rule that caught this
            <ICON.ArrowRight size={11} />
          </button>
        </div>
      </div>
    </li>
  );
}

function AuditLog({ entries, filter, onFilter, counts, onOpenRule }:
  {
    entries: AuditEntry[];
    filter: AuditFilter;
    onFilter: (f: AuditFilter) => void;
    counts: Record<AuditFilter, number>;
    onOpenRule: (ruleId: string) => void;
  }) {
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
            {entries.length} of {AUDIT.length} entries
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

// ───────────────────────────────────────────────────────────────────────────
//  Policy library
// ───────────────────────────────────────────────────────────────────────────

function PolicyCardItem({ p, onEdit, onPause, onTest }:
  { p: Policy; onEdit: () => void; onPause: () => void; onTest: () => void }) {
  const tone = POLICY_TONE[p.state];
  const stateLabel = p.state === "under-review" ? "Under review" : p.state[0].toUpperCase() + p.state.slice(1);
  return (
    <article className="card overflow-hidden flex flex-col">
      <div className="px-4 py-3 flex flex-col gap-2">
        <div className="flex items-start justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-1.5 flex-wrap">
            <Chip tone="neutral">Layer {p.layer}</Chip>
            <Chip tone={tone} dot>{stateLabel}</Chip>
          </div>
          <span className="text-[10.5px] font-mono text-neutral-400 tabular-nums">
            {p.triggeredLast30d} triggers / 30d
          </span>
        </div>
        <h3 className="text-[14px] font-semibold tracking-tight text-neutral-900 leading-snug">{p.name}</h3>
        <p className="text-[12.5px] text-neutral-600 leading-relaxed">{p.purpose}</p>
      </div>
      <footer className="mt-auto border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/60">
        <button type="button" onClick={onEdit}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white hover:text-neutral-900">
          <ICON.Pencil size={11} /> Edit
        </button>
        <button type="button" onClick={onPause}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
          <ICON.Pause size={11} />
          {p.state === "paused" ? "Resume" : "Pause"}
        </button>
        <button type="button" onClick={onTest}
                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
          <ICON.Beaker size={11} /> Test
        </button>
      </footer>
    </article>
  );
}

function PolicyLibrary({ policies, onOpenPolicy, onPausePolicy }:
  {
    policies: Policy[];
    onOpenPolicy: (id: string, focus?: "editor" | "test") => void;
    onPausePolicy: (id: string) => void;
  }) {
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

// ───────────────────────────────────────────────────────────────────────────
//  Policy drawer (editor + history + test)
// ───────────────────────────────────────────────────────────────────────────

function PolicyDrawer({
  policy, focus, onClose, onPause, audit,
}: {
  policy: Policy;
  focus: "editor" | "test";
  onClose: () => void;
  onPause: () => void;
  audit: AuditEntry[];
}) {
  const [tab, setTab] = useState<"editor" | "history" | "test">(focus);
  const [text, setText] = useState(policy.ruleText);

  useEffect(() => { setText(policy.ruleText); }, [policy.id, policy.ruleText]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const related = useMemo(
    () => audit.filter((a) => a.ruleId === policy.id),
    [audit, policy.id],
  );

  const tone = POLICY_TONE[policy.state];
  const stateLabel = policy.state === "under-review" ? "Under review" : policy.state[0].toUpperCase() + policy.state.slice(1);

  return (
    <div className="fixed inset-0 z-40 flex justify-end" role="dialog" aria-modal="true">
      <div className="ig-drawer-backdrop absolute inset-0" onClick={onClose} />
      <aside className="relative h-full w-full max-w-[720px] bg-white shadow-xl border-l border-neutral-200 flex flex-col">
        <header className="sticky top-0 z-10 bg-white/95 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
          <button type="button" onClick={onClose}
                  className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
            <ICON.ArrowLeft size={13} /> Back
          </button>
          <span className="text-[11px] font-mono text-neutral-400">{policy.id.toUpperCase()}</span>
          <span className="text-neutral-300">·</span>
          <Chip tone="neutral">Layer {policy.layer}</Chip>
          <Chip tone={tone} dot>{stateLabel}</Chip>
          <button type="button" onClick={onClose} aria-label="Close"
                  className="ml-auto h-7 w-7 inline-flex items-center justify-center rounded-md text-neutral-500 hover:text-neutral-900 hover:bg-neutral-50">
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

          {/* Tabs */}
          <div className="px-6 border-b border-neutral-100">
            <div className="inline-flex items-center rounded-lg border border-neutral-200 bg-white p-0.5">
              {(["editor", "history", "test"] as const).map((t) => (
                <button key={t} type="button" onClick={() => setTab(t)}
                        className={cx(
                          "px-2.5 py-1 rounded-md text-[12px] font-medium transition-colors capitalize",
                          tab === t ? "bg-neutral-900 text-white" : "text-neutral-600 hover:text-neutral-900",
                        )}>
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
                  <button type="button" onClick={onPause}
                          className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
                    <ICON.Pause size={12} />
                    {policy.state === "paused" ? "Resume policy" : "Pause policy"}
                  </button>
                  <div className="flex items-center gap-2">
                    <button type="button"
                            className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
                      Reword
                    </button>
                    <button type="button"
                            className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-3 py-1.5 text-[12.5px] font-medium">
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
          <button type="button" onClick={onClose}
                  className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
            Close
          </button>
        </footer>
      </aside>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Security — the route
// ───────────────────────────────────────────────────────────────────────────

export function Security(): JSX.Element {
  const [layers]    = useState<Layer[]>(LAYERS);
  const [audit]     = useState<AuditEntry[]>(AUDIT);
  const [policies, setPolicies] = useState<Policy[]>(POLICIES);

  const [filter, setFilter] = useState<AuditFilter>("all");
  const [openPolicy, setOpenPolicy] = useState<{ id: string; focus: "editor" | "test" } | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  useEffect(() => {
    if (!openPolicy) return;
    document.documentElement.style.overflow = "hidden";
    return () => { document.documentElement.style.overflow = ""; };
  }, [openPolicy]);

  useEffect(() => {
    if (!toast) return;
    const id = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(id);
  }, [toast]);

  const filtered = useMemo(() => {
    return audit.filter((e) => {
      switch (filter) {
        case "all":         return true;
        case "blocked":     return e.kind === "blocked";
        case "quarantined": return e.kind === "quarantined";
        case "by-me":       return e.deniedBy === "you";
        case "24h":         return hoursAgo(e.when) <= 24;
        case "7d":          return hoursAgo(e.when) <= 24 * 7;
        default: return true;
      }
    });
  }, [audit, filter]);

  const counts: Record<AuditFilter, number> = useMemo(() => ({
    all:         audit.length,
    blocked:     audit.filter((e) => e.kind === "blocked").length,
    quarantined: audit.filter((e) => e.kind === "quarantined").length,
    "by-me":     audit.filter((e) => e.deniedBy === "you").length,
    "24h":       audit.filter((e) => hoursAgo(e.when) <= 24).length,
    "7d":        audit.filter((e) => hoursAgo(e.when) <= 24 * 7).length,
  }), [audit]);

  const handleOpenRule = (ruleId: string) => {
    setOpenPolicy({ id: ruleId, focus: "editor" });
  };
  const handleOpenPolicy = (id: string, focus: "editor" | "test" = "editor") => {
    setOpenPolicy({ id, focus });
  };
  const handlePause = (id: string) => {
    setPolicies((prev) => prev.map((p) =>
      p.id === id ? { ...p, state: p.state === "paused" ? "active" : "paused" } : p,
    ));
    const p = policies.find((x) => x.id === id);
    if (p) setToast(`${p.state === "paused" ? "Resumed" : "Paused"} · ${p.name}`);
  };

  const opened = openPolicy
    ? policies.find((p) => p.id === openPolicy.id) ?? null
    : null;

  return (
    <main className="min-h-screen bg-app">
      <div className="page-container max-w-[78rem] flex flex-col gap-6">
        <SecurityHeader layers={layers} />

        <FiveLayerCard layers={layers} />

        <AuditLog
          entries={filtered}
          filter={filter}
          onFilter={setFilter}
          counts={counts}
          onOpenRule={handleOpenRule}
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
      </div>

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

;(window as unknown as { Security: typeof Security }).Security = Security;
