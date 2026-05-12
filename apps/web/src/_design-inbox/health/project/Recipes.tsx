// Recipes.tsx — IronGolem OS
// Route: /recipes
// One-file route per house style. Mock data at the top, then the route.
// Anything tagged TODO(integrator) is a placeholder the integrator will
// swap for the real @irongolem/ui import or live API call.
//
// React 19, TS strict, Tailwind utility classes only. Semantic palette
// (bg-safe / bg-warning / bg-blocked / bg-accent / bg-neutral / text-*)
// is provided by globals.css and behaves correctly in light + dark themes.
//
// Mandatory patterns wired in:
//   1. SafetyCard preview sits ABOVE the activation control on every card.
//   2. Visible Trust — required permissions render before the activate button.
//   3. Progressive disclosure — schedule, cron, fallback, retry behavior
//      live inside the Inspect drawer, never on the card.
//   4. Empty state for any category with no recipes is rendered inline.

import * as React from "react";
const { useState, useMemo, useEffect, useRef } = React;

// ───────────────────────────────────────────────────────────────────────────
//  Types
// ───────────────────────────────────────────────────────────────────────────

type Category =
  | "inbox" | "calendar" | "purchasing"
  | "research" | "operations" | "drafting";

type RecipeStatus = "active" | "paused" | "new";
type Risk = "low" | "medium" | "high";
type PermScope = "scoped" | "broad" | "restricted";

type Permission = {
  key: string;        // e.g. "mail.send.known_contact"
  label: string;      // short human label
  scope: PermScope;
  approvals?: number; // historic approval count for trust math
};

type SafetyShape = {
  can: string[];
  cannot: string[];
  needsApproval: string[];
  stopsIf: string[];
};

type PolicyLayer = {
  id: 1 | 2 | 3 | 4 | 5;
  name: string;
  note: string;
  state: "active" | "watching";
};

type RunOutcome =
  | "completed"      // ran clean end-to-end
  | "approved"       // routed to inbox, you approved
  | "held"           // safety layer held it
  | "skipped"        // schedule fired but conditions unmet
  | "denied";        // you denied

type RunEvent = {
  at: string;        // "12m ago" / "yesterday 9:02"
  outcome: RunOutcome;
  note: string;
};

type Recipe = {
  id: string;
  name: string;
  purpose: string;          // one sentence
  category: Category;
  status: RecipeStatus;
  risk: Risk;
  lastRun: string | null;   // null = "Never run"
  schedule: string;         // human description ("every 10m", "daily 9am PT")
  cron: string;             // canonical cron / trigger (drawer only)
  fallback: string;         // what happens on failure
  retry: string;            // retry policy (drawer only)
  permissions: Permission[];
  safety: SafetyShape;
  policy: PolicyLayer[];
  recentRuns: RunEvent[];
};

// ───────────────────────────────────────────────────────────────────────────
//  Mock recipes — 15, spread across the six categories with mixed statuses
//  TODO(integrator): replace with `useRecipesQuery()` from the data layer.
// ───────────────────────────────────────────────────────────────────────────

const FIVE_LAYERS: { name: string; note: string }[] = [
  { name: "Identity",      note: "Caller and workspace identity verified." },
  { name: "Allow-list",    note: "Targets restricted to approved contacts and endpoints." },
  { name: "Tone & accuracy", note: "Outbound content checked for tone, factual claims, attached files." },
  { name: "Spend & scope", note: "Caps, rate limits, and blast radius enforced." },
  { name: "Operator review", note: "Anything above thresholds routed to your inbox." },
];

const policy = (watching: number[] = []): PolicyLayer[] =>
  FIVE_LAYERS.map((l, i) => ({
    id: (i + 1) as 1 | 2 | 3 | 4 | 5,
    name: l.name,
    note: l.note,
    state: watching.includes(i + 1) ? "watching" : "active",
  }));

const MOCK_RECIPES: Recipe[] = [
  // ── Inbox ────────────────────────────────────────────────────────────
  {
    id: "r01",
    name: "Triage inbox while I sleep",
    purpose: "Classify incoming mail and surface only the messages that need you in the morning.",
    category: "inbox",
    status: "active",
    risk: "low",
    lastRun: "12m ago",
    schedule: "Every 10 minutes",
    cron: "*/10 * * * *",
    fallback: "If classification confidence drops below 0.7, the message is left untouched in the inbox.",
    retry: "Retries twice with exponential backoff; on third failure logs and pauses for the window.",
    permissions: [
      { key: "mail.read",        label: "Read incoming mail",       scope: "broad",   approvals: 4112 },
      { key: "mail.label.write", label: "Apply labels to messages", scope: "scoped",  approvals: 3801 },
      { key: "inbox.route",      label: "Route to operator inbox",  scope: "scoped",  approvals: 1240 },
    ],
    safety: {
      can: [
        "Read every inbound mail to classify it",
        "Apply labels like 'follow-up', 'fyi', 'newsletter'",
        "Route classified items to the operator inbox",
      ],
      cannot: [
        "Send mail on your behalf",
        "Open attachments or click links",
        "Forward messages outside the workspace",
      ],
      needsApproval: [
        "First classification of a brand-new domain",
        "Any message touching wire instructions or contracts",
      ],
      stopsIf: [
        "Classification confidence drops below 0.7",
        "An inbound domain is added to the deny-list",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "12m ago",      outcome: "completed", note: "Classified 14 messages, routed 2 to inbox." },
      { at: "22m ago",      outcome: "completed", note: "Classified 8 messages, routed 0." },
      { at: "32m ago",      outcome: "skipped",   note: "Mailbox empty; nothing to do." },
      { at: "42m ago",      outcome: "approved",  note: "Held one PO classification; you approved 'follow-up'." },
      { at: "52m ago",      outcome: "completed", note: "Classified 6 messages." },
      { at: "1h ago",       outcome: "completed", note: "Classified 11 messages, routed 1." },
      { at: "1h 10m ago",   outcome: "completed", note: "Classified 5 messages." },
      { at: "1h 20m ago",   outcome: "held",      note: "New domain hartlaw.com — held for your call." },
      { at: "1h 30m ago",   outcome: "completed", note: "Classified 9 messages." },
      { at: "1h 40m ago",   outcome: "completed", note: "Classified 4 messages." },
    ],
  },
  {
    id: "r02",
    name: "Draft replies to known senders",
    purpose: "Compose a tone-checked draft for any message from a contact you've replied to before — hold for your read.",
    category: "inbox",
    status: "active",
    risk: "medium",
    lastRun: "7m ago",
    schedule: "On every inbound mail from a known sender",
    cron: "trigger: mail.inbound { sender.known: true }",
    fallback: "If tone check fails, draft is held in your inbox with the flag inline; nothing is sent.",
    retry: "No retry — drafts are one-shot. Operator can re-run from the inbox row.",
    permissions: [
      { key: "mail.read",            label: "Read the inbound message",     scope: "scoped",   approvals: 2204 },
      { key: "drafts.write",         label: "Compose a draft reply",        scope: "scoped",   approvals: 2196 },
      { key: "contacts.read.known",  label: "Read known-contact list",      scope: "scoped",   approvals: 2196 },
    ],
    safety: {
      can: [
        "Compose drafts to senders you've replied to ≥3 times",
        "Pull prior thread context for tone matching",
        "Hold drafts for your read before any send",
      ],
      cannot: [
        "Send anything without your explicit approval",
        "Compose drafts to first-time senders",
        "Attach files not already in the thread",
      ],
      needsApproval: [
        "Every outbound send (always, no exceptions)",
        "Any draft that fails the tone check on first pass",
      ],
      stopsIf: [
        "Sender drops off the known-contact list",
        "Tone check fails twice in a row on the same thread",
      ],
    },
    policy: policy([3]),
    recentRuns: [
      { at: "7m ago",      outcome: "approved",  note: "Reply drafted to Marcus Yi — you approved and sent." },
      { at: "31m ago",     outcome: "held",      note: "Tone check flagged 'defensive'; held for your read." },
      { at: "44m ago",     outcome: "approved",  note: "Reply drafted to Sandra Lopez — you approved." },
      { at: "1h 14m ago",  outcome: "approved",  note: "Reply drafted to Asha (Trent & Co) — you approved." },
      { at: "2h ago",      outcome: "skipped",   note: "Inbound from new domain; not eligible." },
      { at: "3h ago",      outcome: "approved",  note: "Reply drafted to team@riverbend — sent." },
      { at: "yesterday",   outcome: "approved",  note: "Reply drafted to Halford intake — sent." },
      { at: "yesterday",   outcome: "denied",    note: "You declined — sent a manual reply instead." },
    ],
  },
  {
    id: "r03",
    name: "Auto-archive newsletters older than 30 days",
    purpose: "Quietly archive newsletter mail you haven't opened in a month — never touches anything else.",
    category: "inbox",
    status: "paused",
    risk: "low",
    lastRun: "3d ago",
    schedule: "Daily at 6:00 AM PT",
    cron: "0 6 * * *",
    fallback: "If a candidate message is younger than 30d, it's left alone.",
    retry: "Retries once at 6:15 AM PT on failure.",
    permissions: [
      { key: "mail.read.labeled", label: "Read mail labeled 'newsletter'",   scope: "scoped",   approvals: 96 },
      { key: "mail.archive",      label: "Move messages to Archive",         scope: "scoped",   approvals: 96 },
    ],
    safety: {
      can: [
        "Move messages labeled 'newsletter' to Archive",
        "Skip anything you've opened or replied to",
      ],
      cannot: [
        "Delete messages permanently",
        "Touch mail not labeled 'newsletter'",
        "Read message bodies (header + label only)",
      ],
      needsApproval: [
        "Bulk archive batches larger than 50 messages",
      ],
      stopsIf: [
        "More than 5% of archives are recalled within 24h",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "3d ago",  outcome: "completed", note: "Archived 18 newsletters." },
      { at: "4d ago",  outcome: "completed", note: "Archived 22 newsletters." },
      { at: "5d ago",  outcome: "completed", note: "Archived 15 newsletters." },
    ],
  },

  // ── Calendar ─────────────────────────────────────────────────────────
  {
    id: "r04",
    name: "Hold Friday-morning focus blocks",
    purpose: "Book a recurring two-hour focus block on Fridays whenever the calendar is clear.",
    category: "calendar",
    status: "active",
    risk: "low",
    lastRun: "yesterday",
    schedule: "Every Thursday at 5:00 PM PT (for Friday)",
    cron: "0 17 * * 4",
    fallback: "If Friday morning has any meeting, the block is skipped that week.",
    retry: "No retry — single attempt per week.",
    permissions: [
      { key: "calendar.read",  label: "Read your calendar",       scope: "scoped",   approvals: 312 },
      { key: "calendar.write", label: "Create events on your own calendar", scope: "scoped", approvals: 312 },
    ],
    safety: {
      can: [
        "Block 9–11 AM on your own calendar",
        "Title the block consistently for visibility",
      ],
      cannot: [
        "Invite anyone else",
        "Move existing events out of the way",
        "Book recurring blocks anywhere except your own calendar",
      ],
      needsApproval: [
        "Any change to the block window or cadence",
      ],
      stopsIf: [
        "Friday morning already has a customer-facing meeting",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "yesterday",  outcome: "completed", note: "Booked focus block for Friday 9–11 AM PT." },
      { at: "8d ago",     outcome: "completed", note: "Booked focus block." },
      { at: "15d ago",    outcome: "skipped",   note: "Conflict — customer review at 10 AM." },
      { at: "22d ago",    outcome: "completed", note: "Booked focus block." },
    ],
  },
  {
    id: "r05",
    name: "Reschedule internal 1:1s on conflict",
    purpose: "When an internal 1:1 collides with a new commitment, propose a clean slot and route the move for approval.",
    category: "calendar",
    status: "new",
    risk: "medium",
    lastRun: null,
    schedule: "On any new event that conflicts with a 1:1",
    cron: "trigger: calendar.conflict { event.type: '1on1', participants.internal: true }",
    fallback: "If no common slot is found within 14 days, the conflict is surfaced to your inbox.",
    retry: "Re-runs every 4 hours until resolved or 7 days elapse.",
    permissions: [
      { key: "calendar.read.org", label: "Read internal teammates' calendars (free/busy)", scope: "broad",   approvals: 0 },
      { key: "calendar.propose",  label: "Propose calendar changes",                       scope: "scoped",  approvals: 0 },
      { key: "inbox.route",       label: "Route reschedule to operator inbox",             scope: "scoped",  approvals: 1240 },
    ],
    safety: {
      can: [
        "Read free/busy for internal teammates",
        "Hold a tentative slot on both calendars",
        "Route the proposed move to your inbox",
      ],
      cannot: [
        "Move customer-facing meetings",
        "Send a calendar update without your approval",
        "Read meeting titles or descriptions on external calendars",
      ],
      needsApproval: [
        "Every reschedule send (no auto-moves on first run)",
      ],
      stopsIf: [
        "Either participant is in a declared focus week",
        "The new slot lands inside a focus block",
      ],
    },
    policy: policy([5]),
    recentRuns: [], // never run
  },
  {
    id: "r06",
    name: "Decline meetings without an agenda",
    purpose: "Auto-reply with a soft decline to incoming invites that arrive without an agenda field.",
    category: "calendar",
    status: "paused",
    risk: "medium",
    lastRun: "6d ago",
    schedule: "On every inbound invite",
    cron: "trigger: calendar.invite.received",
    fallback: "If the invite has an agenda field of any length, no action.",
    retry: "No retry — single response per invite.",
    permissions: [
      { key: "calendar.read.invite", label: "Read inbound invites",            scope: "scoped",  approvals: 88 },
      { key: "mail.send.invite",     label: "Reply to invite organizers",      scope: "scoped",  approvals: 88 },
    ],
    safety: {
      can: [
        "Decline invites missing an agenda field",
        "Send a templated polite-decline reply",
      ],
      cannot: [
        "Decline invites from your direct reports or your manager",
        "Decline invites tagged 'customer'",
      ],
      needsApproval: [
        "Declining anyone you've met with in the last 14 days",
      ],
      stopsIf: [
        "More than 3 declines happen in a single hour",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "6d ago",  outcome: "completed", note: "Declined 1 invite (no agenda) — recruiter outreach." },
      { at: "8d ago",  outcome: "approved",  note: "Routed for your call — invite was from a board member." },
    ],
  },

  // ── Purchasing ───────────────────────────────────────────────────────
  {
    id: "r07",
    name: "Approve standing-order purchases under $50",
    purpose: "Auto-approve recurring purchases from your standing-order list when the amount is below $50.",
    category: "purchasing",
    status: "active",
    risk: "medium",
    lastRun: "2h ago",
    schedule: "On every standing-order PO event",
    cron: "trigger: purchasing.po.standing",
    fallback: "If the amount is at or above $50, the PO is routed to your inbox.",
    retry: "Retries once after 5 minutes if vendor API times out.",
    permissions: [
      { key: "purchasing.po.read",    label: "Read submitted POs",                 scope: "scoped",  approvals: 1844 },
      { key: "purchasing.po.approve", label: "Approve POs on the standing list",   scope: "scoped",  approvals: 1820 },
    ],
    safety: {
      can: [
        "Approve POs to vendors on your standing list",
        "Apply your stored payment method",
      ],
      cannot: [
        "Approve POs to new or one-off vendors",
        "Approve POs ≥ $50",
        "Change vendor bank details under any circumstance",
      ],
      needsApproval: [
        "Any PO at or above $50",
        "Any first-of-month PO regardless of amount",
      ],
      stopsIf: [
        "Vendor fraud score rises above 0.2",
        "Three POs to one vendor inside a 24h window",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "2h ago",   outcome: "completed", note: "Approved $14.20 to Stagecoach Coffee (Travel)." },
      { at: "yesterday", outcome: "completed", note: "Approved $32.00 to OfficePantry (Snacks)." },
      { at: "2d ago",   outcome: "held",      note: "Held $812 to Yates Holdings — above threshold." },
      { at: "2d ago",   outcome: "completed", note: "Approved $9.99 to Notion (Software)." },
      { at: "3d ago",   outcome: "completed", note: "Approved $41.60 to Caltrain (Travel)." },
      { at: "4d ago",   outcome: "skipped",   note: "Vendor not on standing list — routed to inbox." },
    ],
  },
  {
    id: "r08",
    name: "Re-order office supplies on low stock",
    purpose: "When tracked supplies fall below 20% of par level, draft a PO to the preferred vendor.",
    category: "purchasing",
    status: "new",
    risk: "low",
    lastRun: null,
    schedule: "Daily at 7:00 AM PT",
    cron: "0 7 * * *",
    fallback: "If a candidate item is above 20%, no PO is drafted.",
    retry: "No retry — checked once per day.",
    permissions: [
      { key: "inventory.read",       label: "Read inventory levels",     scope: "scoped",  approvals: 0 },
      { key: "purchasing.po.draft",  label: "Draft (not submit) POs",    scope: "scoped",  approvals: 0 },
      { key: "inbox.route",          label: "Route to operator inbox",   scope: "scoped",  approvals: 1240 },
    ],
    safety: {
      can: [
        "Draft POs for tracked items below par",
        "Use the preferred vendor on file for each item",
      ],
      cannot: [
        "Submit POs without your approval",
        "Change preferred vendors",
        "Re-order non-tracked items",
      ],
      needsApproval: [
        "Every PO draft (always)",
      ],
      stopsIf: [
        "Total drafted POs in one day exceed $400",
      ],
    },
    policy: policy([5]),
    recentRuns: [],
  },

  // ── Research ─────────────────────────────────────────────────────────
  {
    id: "r09",
    name: "Daily competitor pricing scan",
    purpose: "Walk the competitor price index every morning and post material moves (>5%) to the research feed.",
    category: "research",
    status: "active",
    risk: "low",
    lastRun: "this morning",
    schedule: "Daily at 8:30 AM PT",
    cron: "30 8 * * *",
    fallback: "If a price source returns stale data (>24h), the source is skipped and noted.",
    retry: "Retries up to 3 times with 2-minute backoff per source.",
    permissions: [
      { key: "research.read.sources",  label: "Read approved price sources",   scope: "scoped",  approvals: 612 },
      { key: "research.feed.write",    label: "Post to the research feed",     scope: "scoped",  approvals: 612 },
    ],
    safety: {
      can: [
        "Pull from approved price sources only",
        "Post moves of 5% or greater to the research feed",
      ],
      cannot: [
        "Add new sources without your approval",
        "Trade, hedge, or execute on any signal",
        "Send research outside the workspace",
      ],
      needsApproval: [
        "Adding a new price source",
        "Any move flagged as 'unusual' by the variance check",
      ],
      stopsIf: [
        "More than 2 sources return stale data on the same run",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "this morning", outcome: "completed", note: "Scanned 11 sources, posted 2 moves (carbon, lithium)." },
      { at: "yesterday",    outcome: "completed", note: "Scanned 11 sources, posted 0 moves." },
      { at: "2d ago",       outcome: "held",      note: "Variance check flagged carbon +18%; routed for review." },
      { at: "3d ago",       outcome: "completed", note: "Scanned 11 sources, posted 1 move (nickel)." },
      { at: "4d ago",       outcome: "completed", note: "Scanned 11 sources, posted 0 moves." },
    ],
  },
  {
    id: "r10",
    name: "Weekly carbon credit market digest",
    purpose: "Compile a one-page Monday digest summarizing the week's carbon credit market activity.",
    category: "research",
    status: "paused",
    risk: "low",
    lastRun: "12d ago",
    schedule: "Mondays at 7:00 AM PT",
    cron: "0 7 * * 1",
    fallback: "If fewer than 3 sources reported, the digest is held with a note.",
    retry: "Retries once at 7:30 AM PT if a source is unavailable.",
    permissions: [
      { key: "research.read.sources", label: "Read approved sources",      scope: "scoped",  approvals: 18 },
      { key: "drafts.write",          label: "Draft digest in your inbox", scope: "scoped",  approvals: 18 },
    ],
    safety: {
      can: [
        "Compile a digest from sources you've approved",
        "Draft the digest into your inbox for review",
      ],
      cannot: [
        "Send the digest outside the workspace",
        "Pull from sources not on the approved list",
      ],
      needsApproval: [
        "Every digest send",
      ],
      stopsIf: [
        "Fewer than 3 approved sources are available",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "12d ago", outcome: "approved",  note: "Digest drafted, you reviewed and sent." },
      { at: "19d ago", outcome: "approved",  note: "Digest drafted, you reviewed and sent." },
    ],
  },

  // ── Operations ───────────────────────────────────────────────────────
  {
    id: "r11",
    name: "File expense receipts under $200",
    purpose: "Auto-file inbound receipts under the matching travel or category tag when below $200.",
    category: "operations",
    status: "active",
    risk: "low",
    lastRun: "26m ago",
    schedule: "On every inbound receipt webhook",
    cron: "trigger: webhook.receipt.received",
    fallback: "If a receipt is ≥ $200, it's routed to your inbox unfiled.",
    retry: "Retries twice on storage failure with 30s backoff.",
    permissions: [
      { key: "expenses.write.scoped", label: "File receipts under tracked categories", scope: "scoped",  approvals: 944 },
      { key: "storage.attach",        label: "Attach receipt PDFs to filings",         scope: "scoped",  approvals: 944 },
    ],
    safety: {
      can: [
        "File receipts under existing categories",
        "Match receipts to travel-tagged calendar events",
      ],
      cannot: [
        "Reimburse anything to a bank account",
        "File receipts ≥ $200",
        "Create new expense categories",
      ],
      needsApproval: [
        "Any receipt at or above $200",
        "Any receipt with no matching category",
      ],
      stopsIf: [
        "Two receipts in a row from the same merchant ≥ $50",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "26m ago",   outcome: "completed", note: "Filed $14.20 to Travel." },
      { at: "2h ago",    outcome: "completed", note: "Filed $41.60 to Travel." },
      { at: "yesterday", outcome: "held",      note: "Receipt $228 — above threshold, routed." },
      { at: "yesterday", outcome: "completed", note: "Filed $9.99 to Software." },
      { at: "2d ago",    outcome: "completed", note: "Filed $32.00 to Snacks." },
    ],
  },
  {
    id: "r12",
    name: "Acknowledge Stripe disputes within SLA",
    purpose: "When a Stripe dispute opens, attach the matching proof-of-delivery and route the bundle for your approval.",
    category: "operations",
    status: "new",
    risk: "high",
    lastRun: null,
    schedule: "On every Stripe dispute webhook",
    cron: "trigger: webhook.stripe.dispute.created",
    fallback: "If no proof-of-delivery is found, the dispute is routed to your inbox with the gap flagged.",
    retry: "Retries every 30 minutes until the SLA window closes (24h).",
    permissions: [
      { key: "stripe.read.dispute",      label: "Read open Stripe disputes",         scope: "scoped",   approvals: 0 },
      { key: "storage.read.proof",       label: "Read proof-of-delivery files",      scope: "scoped",   approvals: 0 },
      { key: "stripe.evidence.draft",    label: "Draft (not submit) evidence",       scope: "scoped",   approvals: 0 },
      { key: "inbox.route",              label: "Route bundle for operator approval", scope: "scoped",  approvals: 1240 },
    ],
    safety: {
      can: [
        "Read open disputes and their case metadata",
        "Pull matching proof-of-delivery from storage",
        "Draft an evidence bundle for your review",
      ],
      cannot: [
        "Submit evidence to Stripe without your approval",
        "Issue refunds or close disputes directly",
        "Contact the disputing customer",
      ],
      needsApproval: [
        "Every evidence submission (always)",
      ],
      stopsIf: [
        "Stripe webhook signature fails verification",
        "Case status changes between bundle and submit",
      ],
    },
    policy: policy([5]),
    recentRuns: [],
  },
  {
    id: "r13",
    name: "Roll deploy logs to cold storage nightly",
    purpose: "Move yesterday's CI deploy logs into cold storage and prune anything older than 90 days.",
    category: "operations",
    status: "active",
    risk: "low",
    lastRun: "last night",
    schedule: "Daily at 2:00 AM PT",
    cron: "0 2 * * *",
    fallback: "If cold storage is unreachable, hot logs are kept; nothing is pruned.",
    retry: "Retries every 30 minutes for up to 6 hours.",
    permissions: [
      { key: "logs.read.deploy",   label: "Read CI deploy logs",       scope: "scoped",  approvals: 240 },
      { key: "storage.write.cold", label: "Write to cold storage",     scope: "scoped",  approvals: 240 },
      { key: "logs.prune",         label: "Prune logs older than 90d", scope: "scoped",  approvals: 240 },
    ],
    safety: {
      can: [
        "Move deploy logs from hot to cold storage",
        "Prune logs older than 90 days",
      ],
      cannot: [
        "Read application or customer data logs",
        "Prune anything younger than 90 days",
      ],
      needsApproval: [
        "Any retention-policy change",
      ],
      stopsIf: [
        "Hot storage drops below 10% free space",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "last night", outcome: "completed", note: "Moved 47 log files, pruned 12 over 90d." },
      { at: "2d ago",     outcome: "completed", note: "Moved 51 log files, pruned 9 over 90d." },
      { at: "3d ago",     outcome: "completed", note: "Moved 44 log files, pruned 14 over 90d." },
    ],
  },

  // ── Drafting ─────────────────────────────────────────────────────────
  {
    id: "r14",
    name: "Compose Monday status digest",
    purpose: "Pull last week's signals and draft the Monday digest into your inbox by 7 AM — never sends.",
    category: "drafting",
    status: "active",
    risk: "low",
    lastRun: "last Monday",
    schedule: "Mondays at 6:30 AM PT",
    cron: "30 6 * * 1",
    fallback: "If signal sources are unavailable, the digest is drafted with what's available + a gap note.",
    retry: "Retries once at 6:45 AM PT.",
    permissions: [
      { key: "signals.read",  label: "Read internal signal sources",   scope: "scoped",  approvals: 412 },
      { key: "drafts.write",  label: "Draft digest in your inbox",     scope: "scoped",  approvals: 412 },
    ],
    safety: {
      can: [
        "Read internal signal sources (CI, heartbeat, inbox stats)",
        "Draft the digest into your inbox",
      ],
      cannot: [
        "Send the digest outside the workspace",
        "Read customer-facing data sources",
      ],
      needsApproval: [
        "Every send of the digest",
      ],
      stopsIf: [
        "Body of the draft diverges materially from the template",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "last Monday", outcome: "approved", note: "Digest drafted, you reviewed and sent at 8:12 AM." },
      { at: "8d ago",      outcome: "approved", note: "Digest drafted, you reviewed and sent at 8:04 AM." },
      { at: "15d ago",     outcome: "approved", note: "Digest drafted, you reviewed and sent at 7:58 AM." },
    ],
  },
  {
    id: "r15",
    name: "Draft thank-you notes after customer calls",
    purpose: "After any customer-tagged calendar event ends, draft a one-paragraph thank-you in your voice.",
    category: "drafting",
    status: "paused",
    risk: "low",
    lastRun: "9d ago",
    schedule: "30 minutes after any 'customer' calendar event ends",
    cron: "trigger: calendar.event.ended { tag: 'customer' }",
    fallback: "If the call ran less than 10 minutes, no draft is composed.",
    retry: "No retry — single attempt per event.",
    permissions: [
      { key: "calendar.read.customer", label: "Read customer-tagged events", scope: "scoped",  approvals: 64 },
      { key: "drafts.write",           label: "Draft thank-you in inbox",    scope: "scoped",  approvals: 412 },
    ],
    safety: {
      can: [
        "Draft a personal-tone thank-you note in your inbox",
        "Pull context only from the calendar event title and your notes",
      ],
      cannot: [
        "Send anything without your read",
        "Reference internal notes you've marked private",
      ],
      needsApproval: [
        "Every send",
      ],
      stopsIf: [
        "Tone check reads as templated or generic",
      ],
    },
    policy: policy(),
    recentRuns: [
      { at: "9d ago",  outcome: "approved", note: "Thank-you to Sandra Lopez — you edited two lines and sent." },
      { at: "16d ago", outcome: "skipped",  note: "Call was 6 minutes; under threshold." },
      { at: "23d ago", outcome: "approved", note: "Thank-you to Marcus Yi — sent as-drafted." },
    ],
  },
];

// ───────────────────────────────────────────────────────────────────────────
//  Static maps (categories, status, etc.)
// ───────────────────────────────────────────────────────────────────────────

const CATEGORIES: { id: Category; label: string }[] = [
  { id: "inbox",      label: "Inbox" },
  { id: "calendar",   label: "Calendar" },
  { id: "purchasing", label: "Purchasing" },
  { id: "research",   label: "Research" },
  { id: "operations", label: "Operations" },
  { id: "drafting",   label: "Drafting" },
];

const STATUS_META: Record<RecipeStatus, { label: string; tone: "safe" | "neutral" | "accent" }> = {
  active: { label: "Active", tone: "safe"    },
  paused: { label: "Paused", tone: "neutral" },
  new:    { label: "New",    tone: "accent"  },
};

const SCOPE_META: Record<PermScope, { label: string; tone: "neutral" | "warning" | "blocked" }> = {
  scoped:     { label: "Scoped",     tone: "neutral" },
  broad:      { label: "Broad",      tone: "warning" },
  restricted: { label: "Restricted", tone: "blocked" },
};

// ───────────────────────────────────────────────────────────────────────────
//  Tiny utility helpers
// ───────────────────────────────────────────────────────────────────────────

const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(" ");

// ───────────────────────────────────────────────────────────────────────────
//  Inline icons (Heroicons-style, stroke 1.5).
//  TODO(integrator): replace with `@irongolem/ui/icons`.
// ───────────────────────────────────────────────────────────────────────────

const Svg = ({ d, vb = "0 0 24 24", size = 16, className = "" }:
  { d: React.ReactNode; vb?: string; size?: number; className?: string }) => (
  <svg viewBox={vb} width={size} height={size} fill="none" stroke="currentColor"
       strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round"
       className={className} aria-hidden="true">{d}</svg>
);

type IconProps = { size?: number; className?: string };

const ICON = {
  Inbox:      (p: IconProps) => <Svg {...p} d={<><path d="M3 12V5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v7" /><path d="M3 12h5l1.5 2.5h5L16 12h5v6a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-6Z" /></>} />,
  Calendar:   (p: IconProps) => <Svg {...p} d={<><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M3 9h18M8 3v4M16 3v4" /></>} />,
  Cart:       (p: IconProps) => <Svg {...p} d={<><path d="M3 4h2l2 12h12l2-8H6" /><circle cx="9" cy="20" r="1.2" /><circle cx="18" cy="20" r="1.2" /></>} />,
  Search:     (p: IconProps) => <Svg {...p} d={<><circle cx="11" cy="11" r="6" /><path d="m20 20-4.3-4.3" /></>} />,
  Wand:       (p: IconProps) => <Svg {...p} d={<><path d="m3 21 12-12" /><path d="m13 5 2 2" /><path d="m17 9 2 2" /><path d="m9 1 1 2 2 1-2 1-1 2-1-2-2-1 2-1 1-2Z" /></>} />,
  Edit:       (p: IconProps) => <Svg {...p} d={<><path d="M4 20h4l10-10-4-4L4 16v4Z" /><path d="m14 6 4 4" /></>} />,
  Cpu:        (p: IconProps) => <Svg {...p} d={<><rect x="5" y="5" width="14" height="14" rx="2" /><rect x="9" y="9" width="6" height="6" rx="1" /><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" /></>} />,
  Check:      (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  X:          (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Play:       (p: IconProps) => <Svg {...p} d={<path d="M7 4v16l13-8L7 4Z" />} />,
  Pause:      (p: IconProps) => <Svg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  Bolt:       (p: IconProps) => <Svg {...p} d={<path d="M13 2 4 14h7l-1 8 9-12h-7l1-8Z" />} />,
  Clock:      (p: IconProps) => <Svg {...p} d={<><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>} />,
  Shield:     (p: IconProps) => <Svg {...p} d={<><path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" /></>} />,
  Lock:       (p: IconProps) => <Svg {...p} d={<><rect x="5" y="11" width="14" height="9" rx="2" /><path d="M8 11V8a4 4 0 1 1 8 0v3" /></>} />,
  Eye:        (p: IconProps) => <Svg {...p} d={<><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></>} />,
  Bell:       (p: IconProps) => <Svg {...p} d={<><path d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2Z" /><path d="M10 20a2 2 0 0 0 4 0" /></>} />,
  Slash:      (p: IconProps) => <Svg {...p} d={<path d="M5 19 19 5" />} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  ArrowLeft:  (p: IconProps) => <Svg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
  Sliders:    (p: IconProps) => <Svg {...p} d={<><path d="M4 6h12" /><path d="M4 12h7" /><path d="M4 18h10" /><circle cx="18" cy="6" r="2" /><circle cx="14" cy="12" r="2" /><circle cx="16" cy="18" r="2" /></>} />,
  Plus:       (p: IconProps) => <Svg {...p} d={<><path d="M12 5v14" /><path d="M5 12h14" /></>} />,
  Sparkles:   (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  Layers:     (p: IconProps) => <Svg {...p} d={<><path d="m12 3 9 5-9 5-9-5 9-5Z" /><path d="m3 13 9 5 9-5" /><path d="m3 18 9 5 9-5" /></>} />,
};

const CATEGORY_META: Record<Category, { label: string; icon: React.ComponentType<IconProps> }> = {
  inbox:      { label: "Inbox",      icon: ICON.Inbox    },
  calendar:   { label: "Calendar",   icon: ICON.Calendar },
  purchasing: { label: "Purchasing", icon: ICON.Cart     },
  research:   { label: "Research",   icon: ICON.Search   },
  operations: { label: "Operations", icon: ICON.Cpu      },
  drafting:   { label: "Drafting",   icon: ICON.Edit     },
};

// ───────────────────────────────────────────────────────────────────────────
//  Placeholder versions of @irongolem/ui components.
//  TODO(integrator): import { RiskBadge, SafetyCard, PolicyCard, Timeline,
//    PermissionBadge } from "@irongolem/ui".
// ───────────────────────────────────────────────────────────────────────────

function RiskBadge({ level, size = "sm" }: { level: Risk; size?: "sm" | "md" }) {
  const m = ({ low: "safe", medium: "warning", high: "blocked" } as const)[level];
  const label = ({ low: "low risk", medium: "med risk", high: "high risk" } as const)[level];
  const sizeCx = size === "sm" ? "text-[10px] px-1.5 py-0.5" : "text-[11px] px-2 py-0.5";
  return (
    <span className={cx(
      "inline-flex items-center gap-1 rounded-full border font-medium",
      sizeCx, `bg-${m}`, `text-${m}`, `border-${m}`,
    )}>
      <span className={cx("h-1.5 w-1.5 rounded-full", `bg-${m}-solid`)} />
      {label}
    </span>
  );
}

function StatusBadge({ status }: { status: RecipeStatus }) {
  const m = STATUS_META[status];
  return (
    <span className={cx(
      "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[10.5px] font-medium uppercase tracking-wide",
      `bg-${m.tone}`, `text-${m.tone}`,
    )}>
      <span className={cx("h-1.5 w-1.5 rounded-full",
        status === "paused" ? "bg-neutral-solid" : `bg-${m.tone}-solid`,
        status === "active" && "ig-pulse",
      )} />
      {m.label}
    </span>
  );
}

function CategoryPill({ category }: { category: Category }) {
  const m = CATEGORY_META[category];
  const Icn = m.icon;
  return (
    <span className="inline-flex items-center gap-1.5 rounded-md bg-neutral-100 px-1.5 py-0.5 text-[10.5px] font-medium text-neutral-600">
      <Icn size={11} /> {m.label}
    </span>
  );
}

function PermissionBadge({ p }: { p: Permission }) {
  const m = SCOPE_META[p.scope];
  return (
    <span className={cx(
      "inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[10.5px] font-medium",
      `bg-${m.tone}`, `text-${m.tone}`, `border-${m.tone}`,
    )} title={`${p.label} · ${m.label} scope`}>
      <ICON.Lock size={10} />
      <span className="font-mono lowercase">{p.key}</span>
    </span>
  );
}

// ── SafetyCard — full four-section version (drawer) ─────────────────────
function SafetyCard({ safety }: { safety: SafetyShape }) {
  const Section = ({ label, items, tone, IconCmp }: {
    label: string; items: string[];
    tone: "safe" | "warning" | "blocked" | "quarantined";
    IconCmp: React.ComponentType<IconProps>;
  }) => (
    <div>
      <div className="flex items-center gap-1.5 mb-2">
        <span className={`text-${tone}`}><IconCmp size={13} /></span>
        <span className={cx("text-[11px] font-medium uppercase tracking-wide", `text-${tone}`)}>{label}</span>
      </div>
      <ul className="space-y-1">
        {items.length === 0 && <li className="text-[12px] text-neutral-400">—</li>}
        {items.map((it, i) => (
          <li key={i} className="text-[13px] text-neutral-700 flex gap-2 leading-snug">
            <span className={cx("mt-1.5 h-1 w-1 rounded-full shrink-0", `bg-${tone}-solid`)} />
            <span>{it}</span>
          </li>
        ))}
      </ul>
    </div>
  );
  return (
    <div className="rounded-xl border border-neutral-200 bg-neutral-50/60 p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Safety summary</div>
        <span className="inline-flex items-center gap-1.5 text-[11px] text-safe font-medium">
          <ICON.Shield size={12} /> Posture: active
        </span>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-4">
        <Section label="Can"            items={safety.can}           tone="safe"        IconCmp={ICON.Check} />
        <Section label="Needs approval" items={safety.needsApproval} tone="warning"     IconCmp={ICON.Bell}  />
        <Section label="Cannot"         items={safety.cannot}        tone="blocked"     IconCmp={ICON.Slash} />
        <Section label="Stops if"       items={safety.stopsIf}       tone="quarantined" IconCmp={ICON.Pause} />
      </div>
    </div>
  );
}

// ── SafetyCard — preview (card variant): 1–2 lines from Can + Needs approval
function SafetyCardPreview({
  safety, onSeeAll,
}: {
  safety: SafetyShape;
  onSeeAll: () => void;
}) {
  const cans = safety.can.slice(0, 1);
  const approvals = safety.needsApproval.slice(0, 1);
  const cannots = safety.cannot.slice(0, 1);
  const totalLines = safety.can.length + safety.cannot.length
    + safety.needsApproval.length + safety.stopsIf.length;
  const shown = cans.length + approvals.length + cannots.length;
  const remaining = totalLines - shown;

  return (
    <div className="rounded-lg border border-neutral-200 bg-neutral-50/70 p-3">
      <div className="flex items-center justify-between mb-2">
        <div className="inline-flex items-center gap-1.5 text-[10.5px] font-medium uppercase tracking-wide text-neutral-500">
          <ICON.Shield size={11} /> Safety summary
        </div>
        <button type="button" onClick={onSeeAll}
                className="text-[11px] font-medium text-accent hover:text-accent-solid inline-flex items-center gap-0.5">
          See all {totalLines}
          <ICON.ArrowRight size={10} />
        </button>
      </div>
      <ul className="space-y-1">
        {cans.map((line, i) => (
          <li key={`c-${i}`} className="text-[12.5px] text-neutral-700 flex gap-2 leading-snug">
            <span className="mt-1.5 h-1 w-1 rounded-full shrink-0 bg-safe-solid" />
            <span><span className="text-safe font-medium">Can</span> · {line}</span>
          </li>
        ))}
        {approvals.map((line, i) => (
          <li key={`a-${i}`} className="text-[12.5px] text-neutral-700 flex gap-2 leading-snug">
            <span className="mt-1.5 h-1 w-1 rounded-full shrink-0 bg-warning-solid" />
            <span><span className="text-warning font-medium">Needs approval</span> · {line}</span>
          </li>
        ))}
        {cannots.map((line, i) => (
          <li key={`n-${i}`} className="text-[12.5px] text-neutral-700 flex gap-2 leading-snug">
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

// ── PolicyCard — five layers (drawer only) ──────────────────────────────
function PolicyCard({ layers }: { layers: PolicyLayer[] }) {
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
            <li key={layer.id}
                className={cx(
                  "flex items-start gap-3 rounded-lg border px-3 py-2",
                  isWatching ? "bg-warning border-warning" : "border-neutral-100 bg-neutral-50/40",
                )}>
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
                ? <ICON.Eye   size={14} className="text-warning mt-1" />
                : <ICON.Check size={14} className="text-safe mt-1" />}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

// ── Timeline — last N runs, drawer only ─────────────────────────────────
const OUTCOME_META: Record<RunOutcome, { label: string; tone: "safe" | "accent" | "blocked" | "neutral" | "quarantined"; IconCmp: React.ComponentType<IconProps> }> = {
  completed: { label: "Completed", tone: "safe",        IconCmp: ICON.Check  },
  approved:  { label: "Approved",  tone: "accent",      IconCmp: ICON.Check  },
  held:      { label: "Held",      tone: "quarantined", IconCmp: ICON.Pause  },
  skipped:   { label: "Skipped",   tone: "neutral",     IconCmp: ICON.Slash  },
  denied:    { label: "Denied",    tone: "blocked",     IconCmp: ICON.X      },
};

function RunsTimeline({ runs }: { runs: RunEvent[] }) {
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
        const Icn = m.IconCmp;
        return (
          <li key={i} className="relative">
            <span className={cx(
              "absolute -left-[19px] top-[3px] h-3.5 w-3.5 rounded-full inline-flex items-center justify-center",
              `bg-${m.tone}`, `text-${m.tone}`,
            )}>
              <Icn size={9} />
            </span>
            <div className="flex items-baseline gap-2 flex-wrap">
              <span className={cx("text-[11px] font-medium uppercase tracking-wide", `text-${m.tone}`)}>{m.label}</span>
              <span className="text-[11px] text-neutral-400 font-mono">{r.at}</span>
            </div>
            <div className="text-[12.5px] text-neutral-600 leading-snug">{r.note}</div>
          </li>
        );
      })}
    </ol>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Filtering — counts + active filters
// ───────────────────────────────────────────────────────────────────────────

type TabId = Category | "all";

function countByTab(recipes: Recipe[]): Record<TabId, number> {
  const out: Record<TabId, number> = {
    all: recipes.length,
    inbox: 0, calendar: 0, purchasing: 0,
    research: 0, operations: 0, drafting: 0,
  };
  for (const r of recipes) out[r.category]++;
  return out;
}

function applyFilters(recipes: Recipe[], tab: TabId, q: string): Recipe[] {
  const needle = q.trim().toLowerCase();
  return recipes.filter((r) => {
    if (tab !== "all" && r.category !== tab) return false;
    if (!needle) return true;
    return r.name.toLowerCase().includes(needle)
        || r.purpose.toLowerCase().includes(needle)
        || r.permissions.some((p) => p.key.includes(needle));
  });
}

// ───────────────────────────────────────────────────────────────────────────
//  Sub-components
// ───────────────────────────────────────────────────────────────────────────

function CategoryTabs({
  active, counts, onChange,
}: {
  active: TabId;
  counts: Record<TabId, number>;
  onChange: (t: TabId) => void;
}) {
  const tabs: { id: TabId; label: string; IconCmp?: React.ComponentType<IconProps> }[] = [
    { id: "all", label: "All" },
    ...CATEGORIES.map((c) => ({ id: c.id as TabId, label: c.label, IconCmp: CATEGORY_META[c.id].icon })),
  ];
  return (
    <div className="flex items-center gap-1 overflow-x-auto scrollbar-thin -mx-1 px-1 pb-1">
      {tabs.map((t) => {
        const isActive = active === t.id;
        const n = counts[t.id];
        const Icn = t.IconCmp;
        return (
          <button
            key={t.id}
            type="button"
            onClick={() => onChange(t.id)}
            className={cx(
              "shrink-0 inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-[13px] font-medium border transition-colors",
              isActive
                ? "bg-neutral-900 text-white border-neutral-900"
                : "bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50",
            )}
          >
            {Icn && <Icn size={13} />}
            {t.label}
            <span className={cx(
              "rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px",
              isActive ? "bg-white/15 text-white" : "bg-neutral-100 text-neutral-500",
            )}>
              {n}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function PrimaryAction({
  status, onClick,
}: {
  status: RecipeStatus;
  onClick: () => void;
}) {
  if (status === "active") {
    return (
      <button type="button" onClick={onClick}
              className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white text-neutral-700 hover:bg-neutral-50 px-3 py-1.5 text-[12.5px] font-medium transition-colors">
        <ICON.Pause size={12} /> Pause
      </button>
    );
  }
  if (status === "paused") {
    return (
      <button type="button" onClick={onClick}
              className="inline-flex items-center gap-1.5 rounded-md bg-safe-solid text-white hover:bg-safe-solid-hover px-3 py-1.5 text-[12.5px] font-medium transition-colors">
        <ICON.Play size={12} /> Activate
      </button>
    );
  }
  // new
  return (
    <button type="button" onClick={onClick}
            className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid text-white hover:bg-accent-solid-hover px-3 py-1.5 text-[12.5px] font-medium transition-colors">
      <ICON.Play size={12} /> Activate
    </button>
  );
}

function RunOnceButton({ onClick }: { onClick: () => void }) {
  return (
    <button type="button" onClick={onClick}
            className="inline-flex items-center gap-1.5 rounded-md text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50 px-2 py-1.5 text-[12.5px] font-medium transition-colors">
      <ICON.Bolt size={12} /> Run once
    </button>
  );
}

// ── Trust strip — permissions + risk + last run, ALWAYS visible above action
function TrustStrip({ recipe }: { recipe: Recipe }) {
  const broad = recipe.permissions.filter((p) => p.scope === "broad").length;
  const restricted = recipe.permissions.filter((p) => p.scope === "restricted").length;
  const trustTone = restricted ? "blocked" : broad ? "warning" : "neutral";
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
      <RiskBadge level={recipe.risk} />
      <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
        <ICON.Lock size={11} className={`text-${trustTone}-solid`} />
        <span>
          <span className={cx("font-medium", `text-${trustTone}`)}>{recipe.permissions.length}</span>
          {" "}permission{recipe.permissions.length === 1 ? "" : "s"}
          {broad > 0 && (
            <span className="text-warning"> · {broad} broad</span>
          )}
        </span>
      </span>
      <span className="text-neutral-300">·</span>
      <span className="inline-flex items-center gap-1 text-[11px] text-neutral-500">
        <ICON.Clock size={11} />
        {recipe.lastRun ? `Last run ${recipe.lastRun}` : (
          <span className="text-neutral-400">Never run</span>
        )}
      </span>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Recipe card
// ───────────────────────────────────────────────────────────────────────────

function RecipeCard({
  recipe, onInspect, onToggle, onRunOnce,
}: {
  recipe: Recipe;
  onInspect: () => void;
  onToggle: () => void;
  onRunOnce: () => void;
}) {
  const CatIcn = CATEGORY_META[recipe.category].icon;
  return (
    <article className="group card flex flex-col h-full overflow-hidden hover:shadow-elevated transition-shadow"
             style={{ boxShadow: "var(--shadow-card)" }}>
      {/* Header — category icon + status badge */}
      <header className="flex items-center justify-between px-4 pt-4">
        <div className="inline-flex items-center gap-1.5">
          <span className="h-7 w-7 rounded-lg bg-neutral-100 inline-flex items-center justify-center text-neutral-600">
            <CatIcn size={14} />
          </span>
          <CategoryPill category={recipe.category} />
        </div>
        <StatusBadge status={recipe.status} />
      </header>

      {/* Title + purpose */}
      <div className="px-4 pt-3">
        <h3 className="text-[15.5px] font-semibold tracking-tight text-neutral-900 leading-snug">
          {recipe.name}
        </h3>
        <p className="mt-1 text-[12.5px] text-neutral-600 leading-relaxed">
          {recipe.purpose}
        </p>
      </div>

      {/* Safety preview — ABOVE the activation control, per the pattern */}
      <div className="px-4 pt-3">
        <SafetyCardPreview safety={recipe.safety} onSeeAll={onInspect} />
      </div>

      {/* Trust strip — permissions visible BEFORE activation button */}
      <div className="px-4 pt-3 pb-3">
        <TrustStrip recipe={recipe} />
      </div>

      {/* Footer — primary action + Inspect link */}
      <footer className="mt-auto border-t border-neutral-100 px-3 py-2 flex items-center justify-between bg-neutral-50/50">
        <div className="flex items-center gap-1">
          <PrimaryAction status={recipe.status} onClick={onToggle} />
          {recipe.status !== "new" && <RunOnceButton onClick={onRunOnce} />}
        </div>
        <button type="button" onClick={onInspect}
                className="inline-flex items-center gap-1 text-[12.5px] font-medium text-accent hover:text-accent-solid transition-colors">
          Inspect
          <ICON.ArrowRight size={11} />
        </button>
      </footer>
    </article>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Detail drawer
// ───────────────────────────────────────────────────────────────────────────

function PermissionsList({ permissions }: { permissions: Permission[] }) {
  return (
    <ul className="divide-y divide-neutral-100 rounded-xl border border-neutral-200 bg-white overflow-hidden">
      {permissions.map((p) => {
        const m = SCOPE_META[p.scope];
        return (
          <li key={p.key} className="px-3 py-2.5 flex items-start gap-3">
            <span className={cx(
              "shrink-0 h-7 w-7 rounded-md inline-flex items-center justify-center",
              `bg-${m.tone}`, `text-${m.tone}`,
            )}>
              <ICON.Lock size={13} />
            </span>
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-2 flex-wrap">
                <span className="text-[12px] font-mono text-neutral-900">{p.key}</span>
                <span className={cx(
                  "text-[10px] font-medium uppercase tracking-wide rounded-full border px-1.5 py-0.5",
                  `bg-${m.tone}`, `text-${m.tone}`, `border-${m.tone}`,
                )}>
                  {m.label} scope
                </span>
              </div>
              <div className="text-[12.5px] text-neutral-600 leading-snug">{p.label}</div>
            </div>
            <div className="text-[11px] text-neutral-400 font-mono tabular-nums whitespace-nowrap">
              {p.approvals != null
                ? `${p.approvals.toLocaleString()}× approved`
                : "0× approved"}
            </div>
          </li>
        );
      })}
    </ul>
  );
}

function CustomizePanel({ recipe }: { recipe: Recipe }) {
  // Progressive disclosure — cron, fallback, retry behavior live here only.
  return (
    <div className="rounded-xl border border-neutral-200 bg-white overflow-hidden">
      <div className="px-4 py-3 border-b border-neutral-100 bg-neutral-50/60 flex items-center justify-between">
        <div>
          <div className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">Advanced</div>
          <div className="text-[14px] font-semibold tracking-tight text-neutral-900">Customize behavior</div>
        </div>
        <button type="button"
                className="inline-flex items-center gap-1.5 rounded-md bg-white border border-neutral-200 hover:bg-neutral-50 text-neutral-700 px-2.5 py-1 text-[12px] font-medium transition-colors">
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
        Changes here override the recipe defaults. Operator review (layer 5)
        will run on every change before it goes live.
      </div>
    </div>
  );
}

function DetailDrawer({
  recipe, onClose, onToggle, onRunOnce,
}: {
  recipe: Recipe;
  onClose: () => void;
  onToggle: () => void;
  onRunOnce: () => void;
}) {
  const CatIcn = CATEGORY_META[recipe.category].icon;

  // ESC to close
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") onClose(); };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-40 flex justify-end" role="dialog" aria-modal="true">
      <div className="ig-drawer-backdrop absolute inset-0" onClick={onClose} />
      <aside className="relative h-full w-full max-w-[640px] bg-white shadow-xl border-l border-neutral-200 flex flex-col"
             style={{ boxShadow: "var(--shadow-elevated)" }}>
        {/* Sticky top bar */}
        <header className="sticky top-0 z-10 bg-white/90 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
          <button type="button" onClick={onClose}
                  className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
            <ICON.ArrowLeft size={13} /> Back
          </button>
          <span className="text-[11px] font-mono text-neutral-400">{recipe.id.toUpperCase()}</span>
          <span className="text-neutral-300">·</span>
          <CategoryPill category={recipe.category} />
          <div className="ml-auto flex items-center gap-2">
            <StatusBadge status={recipe.status} />
            <button type="button" onClick={onClose}
                    className="h-7 w-7 inline-flex items-center justify-center rounded-md text-neutral-500 hover:text-neutral-900 hover:bg-neutral-50">
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
                <h1 className="text-[22px] font-semibold tracking-tight text-neutral-900 leading-tight">
                  {recipe.name}
                </h1>
                <p className="mt-1.5 text-[13.5px] text-neutral-600 leading-relaxed">
                  {recipe.purpose}
                </p>
              </div>
            </div>

            {/* Trust strip — also visible up here, before any activation */}
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

            {/* ── Safety first ──────────────────────────────────────── */}
            <section className="mt-6">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">
                  Safety summary
                </h2>
                <span className="text-[10.5px] text-neutral-400">
                  Always above activation
                </span>
              </div>
              <SafetyCard safety={recipe.safety} />
            </section>

            {/* ── Policy (five layers) ─────────────────────────────── */}
            <section className="mt-5">
              <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">
                Policy layers
              </h2>
              <PolicyCard layers={recipe.policy} />
            </section>

            {/* ── Permissions ──────────────────────────────────────── */}
            <section className="mt-5">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">
                  Required permissions
                </h2>
                <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">
                  {recipe.permissions.length} total
                </span>
              </div>
              <PermissionsList permissions={recipe.permissions} />
            </section>

            {/* ── Schedule + customize (progressive disclosure) ────── */}
            <section className="mt-5">
              <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500 mb-2">
                Schedule &amp; advanced
              </h2>
              <CustomizePanel recipe={recipe} />
            </section>

            {/* ── Recent runs ──────────────────────────────────────── */}
            <section className="mt-5 pb-6">
              <div className="flex items-baseline justify-between mb-2">
                <h2 className="text-[11px] font-medium uppercase tracking-wide text-neutral-500">
                  Recent runs
                </h2>
                <span className="text-[10.5px] text-neutral-400 font-mono tabular-nums">
                  last {Math.min(recipe.recentRuns.length, 10)} of {recipe.recentRuns.length}
                </span>
              </div>
              <RunsTimeline runs={recipe.recentRuns} />
            </section>
          </div>
        </div>

        {/* Sticky footer — activation lives at the bottom, after safety */}
        <footer className="sticky bottom-0 bg-white/95 backdrop-blur border-t border-neutral-100 px-5 py-3 flex items-center justify-between gap-2">
          <RunOnceButton onClick={onRunOnce} />
          <div className="flex items-center gap-2">
            <button type="button"
                    className="inline-flex items-center gap-1.5 rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium transition-colors">
              <ICON.Sliders size={12} /> Customize
            </button>
            <PrimaryAction status={recipe.status} onClick={onToggle} />
          </div>
        </footer>
      </aside>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Empty state
// ───────────────────────────────────────────────────────────────────────────

function EmptyState({ tab, q }: { tab: TabId; q: string }) {
  if (q.trim()) {
    return (
      <div className="card card-padded">
        <div className="flex flex-col items-center text-center py-10 max-w-md mx-auto">
          <div className="h-10 w-10 rounded-full bg-neutral-100 inline-flex items-center justify-center mb-3">
            <ICON.Search size={18} className="text-neutral-400" />
          </div>
          <h3 className="text-[15px] font-semibold tracking-tight text-neutral-900">
            No recipes match "{q}"
          </h3>
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
        <h3 className="text-[15px] font-semibold tracking-tight text-neutral-900">
          No recipes here yet
        </h3>
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

// ───────────────────────────────────────────────────────────────────────────
//  Header / page chrome
// ───────────────────────────────────────────────────────────────────────────

function RecipesHeader({
  total, active, q, onQuery,
}: {
  total: number;
  active: number;
  q: string;
  onQuery: (v: string) => void;
}) {
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
          Pre-composed automations you can activate. Every recipe ships with a
          safety summary — what it can do, what it can't, what needs your
          approval, and what stops it.
        </p>
      </div>
      <div className="flex items-center gap-2">
        <label className="relative inline-flex items-center">
          <span className="absolute left-2.5 text-neutral-400"><ICON.Search size={13} /></span>
          <input
            type="search"
            value={q}
            onChange={(e) => onQuery(e.target.value)}
            placeholder="Search recipes or permissions…"
            className="w-64 max-w-full pl-8 pr-2 py-1.5 text-[13px] bg-white border border-neutral-200 rounded-lg focus:outline-none focus:ring-2 ring-accent placeholder:text-neutral-400"
          />
        </label>
        <button type="button"
                className="inline-flex items-center gap-1.5 rounded-lg bg-neutral-900 text-white hover:bg-neutral-800 px-3 py-1.5 text-[13px] font-medium transition-colors">
          <ICON.Plus size={13} /> Request a recipe
        </button>
      </div>
    </div>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  Recipes — the route
// ───────────────────────────────────────────────────────────────────────────

export function Recipes(): JSX.Element {
  const [recipes, setRecipes] = useState<Recipe[]>(MOCK_RECIPES);
  const [tab, setTab]         = useState<TabId>("all");
  const [q, setQ]             = useState("");
  const [openId, setOpenId]   = useState<string | null>(null);
  const [toast, setToast]     = useState<string | null>(null);

  // Lock body scroll while a drawer is open
  useEffect(() => {
    document.documentElement.style.overflow = openId ? "hidden" : "";
    return () => { document.documentElement.style.overflow = ""; };
  }, [openId]);

  // Auto-dismiss the toast
  useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(() => setToast(null), 2200);
    return () => window.clearTimeout(t);
  }, [toast]);

  const counts   = useMemo(() => countByTab(recipes), [recipes]);
  const filtered = useMemo(() => applyFilters(recipes, tab, q), [recipes, tab, q]);
  const opened   = useMemo(() => recipes.find((r) => r.id === openId) || null, [recipes, openId]);

  const onToggle = (id: string) => {
    setRecipes((prev) => prev.map((r) => {
      if (r.id !== id) return r;
      const next: RecipeStatus = r.status === "active" ? "paused" : "active";
      const verb = next === "active" ? "Activated" : "Paused";
      setToast(`${verb} · ${r.name}`);
      return { ...r, status: next };
    }));
  };
  const onRunOnce = (id: string) => {
    const r = recipes.find((x) => x.id === id);
    if (r) setToast(`Triggered one run · ${r.name}`);
  };

  const activeCount = recipes.filter((r) => r.status === "active").length;

  return (
    <main className="min-h-screen bg-app">
      <div className="page-container max-w-[78rem]">
        <RecipesHeader
          total={recipes.length}
          active={activeCount}
          q={q}
          onQuery={setQ}
        />

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

        {/* Footer hint — gentle pointer to the safety system */}
        <footer className="mt-10 mb-4 flex flex-wrap items-center justify-between gap-3 text-[12px] text-neutral-500">
          <div className="inline-flex items-center gap-1.5">
            <ICON.Shield size={13} className="text-safe" />
            All recipes route through the five-layer safety system before any external action.
          </div>
          <a href="#policy" className="text-accent hover:text-accent-solid font-medium">
            How safety works →
          </a>
        </footer>
      </div>

      {/* Drawer */}
      {opened && (
        <DetailDrawer
          recipe={opened}
          onClose={() => setOpenId(null)}
          onToggle={() => onToggle(opened.id)}
          onRunOnce={() => onRunOnce(opened.id)}
        />
      )}

      {/* Toast */}
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

// Babel-via-CDN can't do real ESM, so we mirror the named export onto
// `window` so the host HTML can mount it without bundling.
;(window as unknown as { Recipes: typeof Recipes }).Recipes = Recipes;
