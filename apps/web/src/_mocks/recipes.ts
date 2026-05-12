// route: /recipes — typed mock data for the Recipes page.
// Consumed via `api.v2.recipes.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema Recipe model once backend stabilises.

export type Category =
  | "inbox"
  | "calendar"
  | "purchasing"
  | "research"
  | "operations"
  | "drafting";

export type RecipeStatus = "active" | "paused" | "new";
export type Risk = "low" | "medium" | "high";
export type PermScope = "scoped" | "broad" | "restricted";
export type RunOutcome = "completed" | "approved" | "held" | "skipped" | "denied";

export interface Permission {
  readonly key: string;
  readonly label: string;
  readonly scope: PermScope;
  readonly approvals?: number;
}

export interface SafetyShape {
  readonly can: readonly string[];
  readonly cannot: readonly string[];
  readonly needsApproval: readonly string[];
  readonly stopsIf: readonly string[];
}

export interface PolicyLayer {
  readonly id: 1 | 2 | 3 | 4 | 5;
  readonly name: string;
  readonly note: string;
  readonly state: "active" | "watching";
}

export interface RunEvent {
  readonly at: string;
  readonly outcome: RunOutcome;
  readonly note: string;
}

export interface Recipe {
  readonly id: string;
  readonly name: string;
  readonly purpose: string;
  readonly category: Category;
  status: RecipeStatus;
  readonly risk: Risk;
  readonly lastRun: string | null;
  readonly schedule: string;
  readonly cron: string;
  readonly fallback: string;
  readonly retry: string;
  readonly permissions: readonly Permission[];
  readonly safety: SafetyShape;
  readonly policy: readonly PolicyLayer[];
  readonly recentRuns: readonly RunEvent[];
}

export const mockFiveLayers: ReadonlyArray<{ readonly name: string; readonly note: string }> = [
  { name: "Identity", note: "Caller and workspace identity verified." },
  { name: "Allow-list", note: "Targets restricted to approved contacts and endpoints." },
  { name: "Tone & accuracy", note: "Outbound content checked for tone, factual claims, attached files." },
  { name: "Spend & scope", note: "Caps, rate limits, and blast radius enforced." },
  { name: "Operator review", note: "Anything above thresholds routed to your inbox." },
];

const policy = (watching: readonly number[] = []): readonly PolicyLayer[] =>
  mockFiveLayers.map((l, i) => ({
    id: (i + 1) as 1 | 2 | 3 | 4 | 5,
    name: l.name,
    note: l.note,
    state: watching.includes(i + 1) ? "watching" : "active",
  }));

export const mockRecipes: readonly Recipe[] = [
  {
    id: "r01", name: "Triage inbox while I sleep",
    purpose: "Classify incoming mail and surface only the messages that need you in the morning.",
    category: "inbox", status: "active", risk: "low", lastRun: "12m ago",
    schedule: "Every 10 minutes", cron: "*/10 * * * *",
    fallback: "If classification confidence drops below 0.7, the message is left untouched in the inbox.",
    retry: "Retries twice with exponential backoff; on third failure logs and pauses for the window.",
    permissions: [
      { key: "mail.read", label: "Read incoming mail", scope: "broad", approvals: 4112 },
      { key: "mail.label.write", label: "Apply labels to messages", scope: "scoped", approvals: 3801 },
      { key: "inbox.route", label: "Route to operator inbox", scope: "scoped", approvals: 1240 },
    ],
    safety: {
      can: ["Read every inbound mail to classify it", "Apply labels like 'follow-up', 'fyi', 'newsletter'", "Route classified items to the operator inbox"],
      cannot: ["Send mail on your behalf", "Open attachments or click links", "Forward messages outside the workspace"],
      needsApproval: ["First classification of a brand-new domain", "Any message touching wire instructions or contracts"],
      stopsIf: ["Classification confidence drops below 0.7", "An inbound domain is added to the deny-list"],
    },
    policy: policy(),
    recentRuns: [
      { at: "12m ago", outcome: "completed", note: "Classified 14 messages, routed 2 to inbox." },
      { at: "22m ago", outcome: "completed", note: "Classified 8 messages, routed 0." },
      { at: "32m ago", outcome: "skipped", note: "Mailbox empty; nothing to do." },
      { at: "42m ago", outcome: "approved", note: "Held one PO classification; you approved 'follow-up'." },
      { at: "52m ago", outcome: "completed", note: "Classified 6 messages." },
      { at: "1h ago", outcome: "completed", note: "Classified 11 messages, routed 1." },
      { at: "1h 10m ago", outcome: "completed", note: "Classified 5 messages." },
      { at: "1h 20m ago", outcome: "held", note: "New domain hartlaw.com — held for your call." },
      { at: "1h 30m ago", outcome: "completed", note: "Classified 9 messages." },
      { at: "1h 40m ago", outcome: "completed", note: "Classified 4 messages." },
    ],
  },
  {
    id: "r02", name: "Draft replies to known senders",
    purpose: "Compose a tone-checked draft for any message from a contact you've replied to before — hold for your read.",
    category: "inbox", status: "active", risk: "medium", lastRun: "7m ago",
    schedule: "On every inbound mail from a known sender",
    cron: "trigger: mail.inbound { sender.known: true }",
    fallback: "If tone check fails, draft is held in your inbox with the flag inline; nothing is sent.",
    retry: "No retry — drafts are one-shot. Operator can re-run from the inbox row.",
    permissions: [
      { key: "mail.read", label: "Read the inbound message", scope: "scoped", approvals: 2204 },
      { key: "drafts.write", label: "Compose a draft reply", scope: "scoped", approvals: 2196 },
      { key: "contacts.read.known", label: "Read known-contact list", scope: "scoped", approvals: 2196 },
    ],
    safety: {
      can: ["Compose drafts to senders you've replied to ≥3 times", "Pull prior thread context for tone matching", "Hold drafts for your read before any send"],
      cannot: ["Send anything without your explicit approval", "Compose drafts to first-time senders", "Attach files not already in the thread"],
      needsApproval: ["Every outbound send (always, no exceptions)", "Any draft that fails the tone check on first pass"],
      stopsIf: ["Sender drops off the known-contact list", "Tone check fails twice in a row on the same thread"],
    },
    policy: policy([3]),
    recentRuns: [
      { at: "7m ago", outcome: "approved", note: "Reply drafted to Marcus Yi — you approved and sent." },
      { at: "31m ago", outcome: "held", note: "Tone check flagged 'defensive'; held for your read." },
      { at: "44m ago", outcome: "approved", note: "Reply drafted to Sandra Lopez — you approved." },
      { at: "1h 14m ago", outcome: "approved", note: "Reply drafted to Asha (Trent & Co) — you approved." },
      { at: "2h ago", outcome: "skipped", note: "Inbound from new domain; not eligible." },
      { at: "3h ago", outcome: "approved", note: "Reply drafted to team@riverbend — sent." },
      { at: "yesterday", outcome: "approved", note: "Reply drafted to Halford intake — sent." },
      { at: "yesterday", outcome: "denied", note: "You declined — sent a manual reply instead." },
    ],
  },
  {
    id: "r03", name: "Auto-archive newsletters older than 30 days",
    purpose: "Quietly archive newsletter mail you haven't opened in a month — never touches anything else.",
    category: "inbox", status: "paused", risk: "low", lastRun: "3d ago",
    schedule: "Daily at 6:00 AM PT", cron: "0 6 * * *",
    fallback: "If a candidate message is younger than 30d, it's left alone.",
    retry: "Retries once at 6:15 AM PT on failure.",
    permissions: [
      { key: "mail.read.labeled", label: "Read mail labeled 'newsletter'", scope: "scoped", approvals: 96 },
      { key: "mail.archive", label: "Move messages to Archive", scope: "scoped", approvals: 96 },
    ],
    safety: {
      can: ["Move messages labeled 'newsletter' to Archive", "Skip anything you've opened or replied to"],
      cannot: ["Delete messages permanently", "Touch mail not labeled 'newsletter'", "Read message bodies (header + label only)"],
      needsApproval: ["Bulk archive batches larger than 50 messages"],
      stopsIf: ["More than 5% of archives are recalled within 24h"],
    },
    policy: policy(),
    recentRuns: [
      { at: "3d ago", outcome: "completed", note: "Archived 18 newsletters." },
      { at: "4d ago", outcome: "completed", note: "Archived 22 newsletters." },
      { at: "5d ago", outcome: "completed", note: "Archived 15 newsletters." },
    ],
  },
  {
    id: "r04", name: "Hold Friday-morning focus blocks",
    purpose: "Book a recurring two-hour focus block on Fridays whenever the calendar is clear.",
    category: "calendar", status: "active", risk: "low", lastRun: "yesterday",
    schedule: "Every Thursday at 5:00 PM PT (for Friday)", cron: "0 17 * * 4",
    fallback: "If Friday morning has any meeting, the block is skipped that week.",
    retry: "No retry — single attempt per week.",
    permissions: [
      { key: "calendar.read", label: "Read your calendar", scope: "scoped", approvals: 312 },
      { key: "calendar.write", label: "Create events on your own calendar", scope: "scoped", approvals: 312 },
    ],
    safety: {
      can: ["Block 9–11 AM on your own calendar", "Title the block consistently for visibility"],
      cannot: ["Invite anyone else", "Move existing events out of the way", "Book recurring blocks anywhere except your own calendar"],
      needsApproval: ["Any change to the block window or cadence"],
      stopsIf: ["Friday morning already has a customer-facing meeting"],
    },
    policy: policy(),
    recentRuns: [
      { at: "yesterday", outcome: "completed", note: "Booked focus block for Friday 9–11 AM PT." },
      { at: "8d ago", outcome: "completed", note: "Booked focus block." },
      { at: "15d ago", outcome: "skipped", note: "Conflict — customer review at 10 AM." },
      { at: "22d ago", outcome: "completed", note: "Booked focus block." },
    ],
  },
  {
    id: "r05", name: "Reschedule internal 1:1s on conflict",
    purpose: "When an internal 1:1 collides with a new commitment, propose a clean slot and route the move for approval.",
    category: "calendar", status: "new", risk: "medium", lastRun: null,
    schedule: "On any new event that conflicts with a 1:1",
    cron: "trigger: calendar.conflict { event.type: '1on1', participants.internal: true }",
    fallback: "If no common slot is found within 14 days, the conflict is surfaced to your inbox.",
    retry: "Re-runs every 4 hours until resolved or 7 days elapse.",
    permissions: [
      { key: "calendar.read.org", label: "Read internal teammates' calendars (free/busy)", scope: "broad", approvals: 0 },
      { key: "calendar.propose", label: "Propose calendar changes", scope: "scoped", approvals: 0 },
      { key: "inbox.route", label: "Route reschedule to operator inbox", scope: "scoped", approvals: 1240 },
    ],
    safety: {
      can: ["Read free/busy for internal teammates", "Hold a tentative slot on both calendars", "Route the proposed move to your inbox"],
      cannot: ["Move customer-facing meetings", "Send a calendar update without your approval", "Read meeting titles or descriptions on external calendars"],
      needsApproval: ["Every reschedule send (no auto-moves on first run)"],
      stopsIf: ["Either participant is in a declared focus week", "The new slot lands inside a focus block"],
    },
    policy: policy([5]),
    recentRuns: [],
  },
  {
    id: "r06", name: "Decline meetings without an agenda",
    purpose: "Auto-reply with a soft decline to incoming invites that arrive without an agenda field.",
    category: "calendar", status: "paused", risk: "medium", lastRun: "6d ago",
    schedule: "On every inbound invite", cron: "trigger: calendar.invite.received",
    fallback: "If the invite has an agenda field of any length, no action.",
    retry: "No retry — single response per invite.",
    permissions: [
      { key: "calendar.read.invite", label: "Read inbound invites", scope: "scoped", approvals: 88 },
      { key: "mail.send.invite", label: "Reply to invite organizers", scope: "scoped", approvals: 88 },
    ],
    safety: {
      can: ["Decline invites missing an agenda field", "Send a templated polite-decline reply"],
      cannot: ["Decline invites from your direct reports or your manager", "Decline invites tagged 'customer'"],
      needsApproval: ["Declining anyone you've met with in the last 14 days"],
      stopsIf: ["More than 3 declines happen in a single hour"],
    },
    policy: policy(),
    recentRuns: [
      { at: "6d ago", outcome: "completed", note: "Declined 1 invite (no agenda) — recruiter outreach." },
      { at: "8d ago", outcome: "approved", note: "Routed for your call — invite was from a board member." },
    ],
  },
  {
    id: "r07", name: "Approve standing-order purchases under $50",
    purpose: "Auto-approve recurring purchases from your standing-order list when the amount is below $50.",
    category: "purchasing", status: "active", risk: "medium", lastRun: "2h ago",
    schedule: "On every standing-order PO event", cron: "trigger: purchasing.po.standing",
    fallback: "If the amount is at or above $50, the PO is routed to your inbox.",
    retry: "Retries once after 5 minutes if vendor API times out.",
    permissions: [
      { key: "purchasing.po.read", label: "Read submitted POs", scope: "scoped", approvals: 1844 },
      { key: "purchasing.po.approve", label: "Approve POs on the standing list", scope: "scoped", approvals: 1820 },
    ],
    safety: {
      can: ["Approve POs to vendors on your standing list", "Apply your stored payment method"],
      cannot: ["Approve POs to new or one-off vendors", "Approve POs ≥ $50", "Change vendor bank details under any circumstance"],
      needsApproval: ["Any PO at or above $50", "Any first-of-month PO regardless of amount"],
      stopsIf: ["Vendor fraud score rises above 0.2", "Three POs to one vendor inside a 24h window"],
    },
    policy: policy(),
    recentRuns: [
      { at: "2h ago", outcome: "completed", note: "Approved $14.20 to Stagecoach Coffee (Travel)." },
      { at: "yesterday", outcome: "completed", note: "Approved $32.00 to OfficePantry (Snacks)." },
      { at: "2d ago", outcome: "held", note: "Held $812 to Yates Holdings — above threshold." },
      { at: "2d ago", outcome: "completed", note: "Approved $9.99 to Notion (Software)." },
      { at: "3d ago", outcome: "completed", note: "Approved $41.60 to Caltrain (Travel)." },
      { at: "4d ago", outcome: "skipped", note: "Vendor not on standing list — routed to inbox." },
    ],
  },
  {
    id: "r08", name: "Re-order office supplies on low stock",
    purpose: "When tracked supplies fall below 20% of par level, draft a PO to the preferred vendor.",
    category: "purchasing", status: "new", risk: "low", lastRun: null,
    schedule: "Daily at 7:00 AM PT", cron: "0 7 * * *",
    fallback: "If a candidate item is above 20%, no PO is drafted.",
    retry: "No retry — checked once per day.",
    permissions: [
      { key: "inventory.read", label: "Read inventory levels", scope: "scoped", approvals: 0 },
      { key: "purchasing.po.draft", label: "Draft (not submit) POs", scope: "scoped", approvals: 0 },
      { key: "inbox.route", label: "Route to operator inbox", scope: "scoped", approvals: 1240 },
    ],
    safety: {
      can: ["Draft POs for tracked items below par", "Use the preferred vendor on file for each item"],
      cannot: ["Submit POs without your approval", "Change preferred vendors", "Re-order non-tracked items"],
      needsApproval: ["Every PO draft (always)"],
      stopsIf: ["Total drafted POs in one day exceed $400"],
    },
    policy: policy([5]),
    recentRuns: [],
  },
  {
    id: "r09", name: "Daily competitor pricing scan",
    purpose: "Walk the competitor price index every morning and post material moves (>5%) to the research feed.",
    category: "research", status: "active", risk: "low", lastRun: "this morning",
    schedule: "Daily at 8:30 AM PT", cron: "30 8 * * *",
    fallback: "If a price source returns stale data (>24h), the source is skipped and noted.",
    retry: "Retries up to 3 times with 2-minute backoff per source.",
    permissions: [
      { key: "research.read.sources", label: "Read approved price sources", scope: "scoped", approvals: 612 },
      { key: "research.feed.write", label: "Post to the research feed", scope: "scoped", approvals: 612 },
    ],
    safety: {
      can: ["Pull from approved price sources only", "Post moves of 5% or greater to the research feed"],
      cannot: ["Add new sources without your approval", "Trade, hedge, or execute on any signal", "Send research outside the workspace"],
      needsApproval: ["Adding a new price source", "Any move flagged as 'unusual' by the variance check"],
      stopsIf: ["More than 2 sources return stale data on the same run"],
    },
    policy: policy(),
    recentRuns: [
      { at: "this morning", outcome: "completed", note: "Scanned 11 sources, posted 2 moves (carbon, lithium)." },
      { at: "yesterday", outcome: "completed", note: "Scanned 11 sources, posted 0 moves." },
      { at: "2d ago", outcome: "held", note: "Variance check flagged carbon +18%; routed for review." },
      { at: "3d ago", outcome: "completed", note: "Scanned 11 sources, posted 1 move (nickel)." },
      { at: "4d ago", outcome: "completed", note: "Scanned 11 sources, posted 0 moves." },
    ],
  },
  {
    id: "r10", name: "Weekly carbon credit market digest",
    purpose: "Compile a one-page Monday digest summarizing the week's carbon credit market activity.",
    category: "research", status: "paused", risk: "low", lastRun: "12d ago",
    schedule: "Mondays at 7:00 AM PT", cron: "0 7 * * 1",
    fallback: "If fewer than 3 sources reported, the digest is held with a note.",
    retry: "Retries once at 7:30 AM PT if a source is unavailable.",
    permissions: [
      { key: "research.read.sources", label: "Read approved sources", scope: "scoped", approvals: 18 },
      { key: "drafts.write", label: "Draft digest in your inbox", scope: "scoped", approvals: 18 },
    ],
    safety: {
      can: ["Compile a digest from sources you've approved", "Draft the digest into your inbox for review"],
      cannot: ["Send the digest outside the workspace", "Pull from sources not on the approved list"],
      needsApproval: ["Every digest send"],
      stopsIf: ["Fewer than 3 approved sources are available"],
    },
    policy: policy(),
    recentRuns: [
      { at: "12d ago", outcome: "approved", note: "Digest drafted, you reviewed and sent." },
      { at: "19d ago", outcome: "approved", note: "Digest drafted, you reviewed and sent." },
    ],
  },
  {
    id: "r11", name: "File expense receipts under $200",
    purpose: "Auto-file inbound receipts under the matching travel or category tag when below $200.",
    category: "operations", status: "active", risk: "low", lastRun: "26m ago",
    schedule: "On every inbound receipt webhook", cron: "trigger: webhook.receipt.received",
    fallback: "If a receipt is ≥ $200, it's routed to your inbox unfiled.",
    retry: "Retries twice on storage failure with 30s backoff.",
    permissions: [
      { key: "expenses.write.scoped", label: "File receipts under tracked categories", scope: "scoped", approvals: 944 },
      { key: "storage.attach", label: "Attach receipt PDFs to filings", scope: "scoped", approvals: 944 },
    ],
    safety: {
      can: ["File receipts under existing categories", "Match receipts to travel-tagged calendar events"],
      cannot: ["Reimburse anything to a bank account", "File receipts ≥ $200", "Create new expense categories"],
      needsApproval: ["Any receipt at or above $200", "Any receipt with no matching category"],
      stopsIf: ["Two receipts in a row from the same merchant ≥ $50"],
    },
    policy: policy(),
    recentRuns: [
      { at: "26m ago", outcome: "completed", note: "Filed $14.20 to Travel." },
      { at: "2h ago", outcome: "completed", note: "Filed $41.60 to Travel." },
      { at: "yesterday", outcome: "held", note: "Receipt $228 — above threshold, routed." },
      { at: "yesterday", outcome: "completed", note: "Filed $9.99 to Software." },
      { at: "2d ago", outcome: "completed", note: "Filed $32.00 to Snacks." },
    ],
  },
  {
    id: "r12", name: "Acknowledge Stripe disputes within SLA",
    purpose: "When a Stripe dispute opens, attach the matching proof-of-delivery and route the bundle for your approval.",
    category: "operations", status: "new", risk: "high", lastRun: null,
    schedule: "On every Stripe dispute webhook", cron: "trigger: webhook.stripe.dispute.created",
    fallback: "If no proof-of-delivery is found, the dispute is routed to your inbox with the gap flagged.",
    retry: "Retries every 30 minutes until the SLA window closes (24h).",
    permissions: [
      { key: "stripe.read.dispute", label: "Read open Stripe disputes", scope: "scoped", approvals: 0 },
      { key: "storage.read.proof", label: "Read proof-of-delivery files", scope: "scoped", approvals: 0 },
      { key: "stripe.evidence.draft", label: "Draft (not submit) evidence", scope: "scoped", approvals: 0 },
      { key: "inbox.route", label: "Route bundle for operator approval", scope: "scoped", approvals: 1240 },
    ],
    safety: {
      can: ["Read open disputes and their case metadata", "Pull matching proof-of-delivery from storage", "Draft an evidence bundle for your review"],
      cannot: ["Submit evidence to Stripe without your approval", "Issue refunds or close disputes directly", "Contact the disputing customer"],
      needsApproval: ["Every evidence submission (always)"],
      stopsIf: ["Stripe webhook signature fails verification", "Case status changes between bundle and submit"],
    },
    policy: policy([5]),
    recentRuns: [],
  },
  {
    id: "r13", name: "Roll deploy logs to cold storage nightly",
    purpose: "Move yesterday's CI deploy logs into cold storage and prune anything older than 90 days.",
    category: "operations", status: "active", risk: "low", lastRun: "last night",
    schedule: "Daily at 2:00 AM PT", cron: "0 2 * * *",
    fallback: "If cold storage is unreachable, hot logs are kept; nothing is pruned.",
    retry: "Retries every 30 minutes for up to 6 hours.",
    permissions: [
      { key: "logs.read.deploy", label: "Read CI deploy logs", scope: "scoped", approvals: 240 },
      { key: "storage.write.cold", label: "Write to cold storage", scope: "scoped", approvals: 240 },
      { key: "logs.prune", label: "Prune logs older than 90d", scope: "scoped", approvals: 240 },
    ],
    safety: {
      can: ["Move deploy logs from hot to cold storage", "Prune logs older than 90 days"],
      cannot: ["Read application or customer data logs", "Prune anything younger than 90 days"],
      needsApproval: ["Any retention-policy change"],
      stopsIf: ["Hot storage drops below 10% free space"],
    },
    policy: policy(),
    recentRuns: [
      { at: "last night", outcome: "completed", note: "Moved 47 log files, pruned 12 over 90d." },
      { at: "2d ago", outcome: "completed", note: "Moved 51 log files, pruned 9 over 90d." },
      { at: "3d ago", outcome: "completed", note: "Moved 44 log files, pruned 14 over 90d." },
    ],
  },
  {
    id: "r14", name: "Compose Monday status digest",
    purpose: "Pull last week's signals and draft the Monday digest into your inbox by 7 AM — never sends.",
    category: "drafting", status: "active", risk: "low", lastRun: "last Monday",
    schedule: "Mondays at 6:30 AM PT", cron: "30 6 * * 1",
    fallback: "If signal sources are unavailable, the digest is drafted with what's available + a gap note.",
    retry: "Retries once at 6:45 AM PT.",
    permissions: [
      { key: "signals.read", label: "Read internal signal sources", scope: "scoped", approvals: 412 },
      { key: "drafts.write", label: "Draft digest in your inbox", scope: "scoped", approvals: 412 },
    ],
    safety: {
      can: ["Read internal signal sources (CI, heartbeat, inbox stats)", "Draft the digest into your inbox"],
      cannot: ["Send the digest outside the workspace", "Read customer-facing data sources"],
      needsApproval: ["Every send of the digest"],
      stopsIf: ["Body of the draft diverges materially from the template"],
    },
    policy: policy(),
    recentRuns: [
      { at: "last Monday", outcome: "approved", note: "Digest drafted, you reviewed and sent at 8:12 AM." },
      { at: "8d ago", outcome: "approved", note: "Digest drafted, you reviewed and sent at 8:04 AM." },
      { at: "15d ago", outcome: "approved", note: "Digest drafted, you reviewed and sent at 7:58 AM." },
    ],
  },
  {
    id: "r15", name: "Draft thank-you notes after customer calls",
    purpose: "After any customer-tagged calendar event ends, draft a one-paragraph thank-you in your voice.",
    category: "drafting", status: "paused", risk: "low", lastRun: "9d ago",
    schedule: "30 minutes after any 'customer' calendar event ends",
    cron: "trigger: calendar.event.ended { tag: 'customer' }",
    fallback: "If the call ran less than 10 minutes, no draft is composed.",
    retry: "No retry — single attempt per event.",
    permissions: [
      { key: "calendar.read.customer", label: "Read customer-tagged events", scope: "scoped", approvals: 64 },
      { key: "drafts.write", label: "Draft thank-you in inbox", scope: "scoped", approvals: 412 },
    ],
    safety: {
      can: ["Draft a personal-tone thank-you note in your inbox", "Pull context only from the calendar event title and your notes"],
      cannot: ["Send anything without your read", "Reference internal notes you've marked private"],
      needsApproval: ["Every send"],
      stopsIf: ["Tone check reads as templated or generic"],
    },
    policy: policy(),
    recentRuns: [
      { at: "9d ago", outcome: "approved", note: "Thank-you to Sandra Lopez — you edited two lines and sent." },
      { at: "16d ago", outcome: "skipped", note: "Call was 6 minutes; under threshold." },
      { at: "23d ago", outcome: "approved", note: "Thank-you to Marcus Yi — sent as-drafted." },
    ],
  },
];
