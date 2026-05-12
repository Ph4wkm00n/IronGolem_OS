// inbox.tsx — IronGolem OS
// Route: /inbox
// One-file route per house style. Mock data sits at the top; the
// rest of the file is the component. Anything tagged TODO(integrator)
// is a placeholder the integrator will swap for the real @irongolem/ui
// import or live API call.
//
// React 19, TS strict, Tailwind utility classes only. Semantic palette
// (bg-warning / bg-blocked / bg-safe / bg-accent / text-*) is provided
// by globals.css and behaves correctly in light + dark themes.

import * as React from "react";
const { useState, useMemo, useReducer, useEffect, useRef } = React;

// ───────────────────────────────────────────────────────────────────────────
//  Types
// ───────────────────────────────────────────────────────────────────────────

type Source = "email" | "telegram" | "webhook" | "calendar";
type Risk = "low" | "medium" | "high";
type Status = "awaiting" | "draft" | "held" | "done";

type EmailDraft = {
  kind: "email";
  from: string;
  to: string;
  cc?: string;
  subject: string;
  body: string[];
};

type CalendarDraft = {
  kind: "calendar";
  invite: string;
  when: string;
  where: string;
  attendees: string[];
  description: string;
};

type WebhookDraft = {
  kind: "webhook";
  endpoint: string;
  method: "POST" | "PUT" | "PATCH" | "DELETE";
  fields: { label: string; value: string }[];
};

type TelegramDraft = {
  kind: "telegram";
  chat: string;
  reply_to: string;
  body: string;
};

type Draft = EmailDraft | CalendarDraft | WebhookDraft | TelegramDraft;

type SafetyShape = {
  can: string[];
  cannot: string[];
  needsApproval: string[];
  stopsIf: string[];
};

type AuditStep = {
  at: string;       // "07:14" or "2m ago"
  actor: string;    // team / system that did the step
  note: string;
};

type Item = {
  id: string;
  status: Status;
  title: string;            // 5–10 words
  source: Source;
  risk: Risk;
  minutesAgo: number;
  summary: string;          // one line
  cause: string;            // "why this is here"
  routedBy: string;         // assistant team
  unread: boolean;
  draft?: Draft;
  safety: SafetyShape;
  audit: AuditStep[];
};

// ───────────────────────────────────────────────────────────────────────────
//  Mock items — 20, mixed sources / risks / statuses
//  Integrator: replace with `useInboxQuery()` from the data layer.
// ───────────────────────────────────────────────────────────────────────────

const MOCK_ITEMS: Item[] = [
  {
    id: "i01", status: "awaiting", unread: true,
    title: "Reply to Marcus Yi about the Riverbend purchase order",
    source: "email", risk: "medium", minutesAgo: 2,
    summary: "Confirms quantity and pushes pickup to Friday at 9am.",
    cause: "Marcus has been waiting 14 hours and the draft passed tone + accuracy checks.",
    routedBy: "Inbox triage",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "marcus@riverbend.co",
      subject: "Re: Riverbend PO — pickup window",
      body: [
        "Hi Marcus,",
        "Confirmed on the 240 units. We'll have everything palletized for a Friday 9am pickup at the Eastside dock — your driver can ask for Sam at the gate.",
        "I've attached the updated PO and the COI you asked about. Let me know if you need anything else before Friday.",
        "— Adam",
      ],
    },
    safety: {
      can: ["Send to known contact", "Attach approved PO file"],
      cannot: ["Add new recipients", "Change PO amount"],
      needsApproval: ["Any external send"],
      stopsIf: ["Recipient domain changes", "Attachment differs from PO record"],
    },
    audit: [
      { at: "—14h", actor: "Marcus Yi (riverbend.co)", note: "Inbound: asked about pickup window." },
      { at: "—12h", actor: "Inbox triage", note: "Classified as PO follow-up (97% conf)." },
      { at: "—9m",  actor: "Drafting",     note: "Composed reply against template R-04." },
      { at: "—2m",  actor: "Tone check",   note: "Passed — neutral, factual." },
    ],
  },
  {
    id: "i02", status: "awaiting", unread: true,
    title: "Move customer review with Sandra to Thursday 2pm",
    source: "calendar", risk: "medium", minutesAgo: 11,
    summary: "Both calendars are clear Thursday afternoon. Conference room A is held.",
    cause: "Customer-touching reschedules always need your approval, even when calendars align.",
    routedBy: "Calendar",
    draft: {
      kind: "calendar",
      invite: "Quarterly review — Sandra Lopez × Eastside",
      when: "Thu, May 14 · 2:00 – 3:00 PM PT",
      where: "Conference Room A · video link auto-attached",
      attendees: ["Adam Stern (you)", "Sandra Lopez (Riverbend)", "Priya N.", "Marcus Yi (Riverbend)"],
      description:
        "Move from Wed 11am at Sandra's request. Agenda unchanged: Q2 utilization, renewal scope, Riverbend dock changeover.",
    },
    safety: {
      can: ["Move internal slots", "Hold a new room"],
      cannot: ["Add new attendees", "Move into a focus block"],
      needsApproval: ["Any customer reschedule"],
      stopsIf: ["Customer hasn't acknowledged the move"],
    },
    audit: [
      { at: "—42m", actor: "Sandra Lopez", note: "Emailed asking to move (conflict on her side)." },
      { at: "—30m", actor: "Inbox triage", note: "Routed to Calendar (94% conf)." },
      { at: "—11m", actor: "Calendar",     note: "Found common slot Thu 2pm; held Room A." },
    ],
  },
  {
    id: "i03", status: "awaiting", unread: true,
    title: "Approve outbound wire instructions for Trent & Co",
    source: "email", risk: "high", minutesAgo: 18,
    summary: "Reply to Trent's AP team with the routing details from the signed MSA.",
    cause: "Anything touching wire instructions always needs you, even on routine sends.",
    routedBy: "Inbox triage",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "ap@trentandco.com",
      subject: "Re: Wire instructions for PO-8821",
      body: [
        "Hi Asha,",
        "Per the signed MSA, please use the routing details on the attached W-9 + bank letter. Routing/Account on the letter is canonical.",
        "If anything differs from what's printed there, please stop and call me at the number in my signature.",
        "— Adam",
      ],
    },
    safety: {
      can: ["Re-send attachments already on file"],
      cannot: ["Type wire details in body", "Change recipient domain"],
      needsApproval: ["Any wire-instruction email"],
      stopsIf: ["A new bank letter is detected", "Recipient hasn't been verified by voice"],
    },
    audit: [
      { at: "—3h", actor: "Asha (trentandco.com)", note: "Inbound: requested wire details." },
      { at: "—2h", actor: "Inbox triage", note: "Classified high-risk: wire-adjacent." },
      { at: "—18m", actor: "Drafting", note: "Composed safe reply pointing to canonical doc." },
    ],
  },
  {
    id: "i04", status: "awaiting", unread: false,
    title: "Acknowledge Stripe dispute on charge_3PqQzN…",
    source: "webhook", risk: "low", minutesAgo: 24,
    summary: "Open the dispute, attach the proof-of-delivery PDF, submit evidence.",
    cause: "Disputes always come here so you can see the customer and amount before responding.",
    routedBy: "Operations",
    draft: {
      kind: "webhook",
      endpoint: "POST /v1/disputes/dp_1NaC9k/close",
      method: "POST",
      fields: [
        { label: "evidence.product_description", value: "Riverbend uniform order, lot 24-118" },
        { label: "evidence.receipt", value: "rcpt_2KkLM (attached)" },
        { label: "evidence.shipping_documentation", value: "POD-24-118.pdf (attached)" },
        { label: "submit", value: "true" },
      ],
    },
    safety: {
      can: ["Attach files already in this case", "Submit evidence"],
      cannot: ["Issue a refund", "Contact the customer directly"],
      needsApproval: ["Submitting dispute evidence"],
      stopsIf: ["Stripe webhook signature fails", "Case status changes mid-flight"],
    },
    audit: [
      { at: "—1d", actor: "Stripe", note: "Dispute opened (reason: product_not_received)." },
      { at: "—28m", actor: "Operations", note: "Matched dispute to PO-24-118." },
      { at: "—24m", actor: "Drafting", note: "Composed evidence bundle." },
    ],
  },
  {
    id: "i05", status: "awaiting", unread: false,
    title: "Reply on ops chat about staging deploy window",
    source: "telegram", risk: "low", minutesAgo: 31,
    summary: "Confirm the deploy window for tonight (10pm–11pm PT).",
    cause: "Cross-team scheduling reads as low-risk but should still be your call.",
    routedBy: "Operations",
    draft: {
      kind: "telegram",
      chat: "Eastside · #ops",
      reply_to: "Priya N. — 'Are we still good for staging at 10?'",
      body: "Yes — window is 10:00–11:00 PT tonight. I'll hold off the digest cron until 11:15. Ping if anything in CI changes.",
    },
    safety: {
      can: ["Reply in approved channels", "Tag operator on call"],
      cannot: ["DM customers", "Send files"],
      needsApproval: ["Replies that touch deploy timing"],
      stopsIf: ["Channel becomes archived", "Tone check fails"],
    },
    audit: [
      { at: "—45m", actor: "Priya N.", note: "Asked about deploy window." },
      { at: "—31m", actor: "Drafting", note: "Composed concise confirmation." },
    ],
  },
  {
    id: "i06", status: "awaiting", unread: false,
    title: "Send weekly status to the Riverbend team",
    source: "email", risk: "low", minutesAgo: 42,
    summary: "Standing Monday digest — template, recipients, send window all match.",
    cause: "It's the regular cadence send, but the recipient list grew by one this week.",
    routedBy: "Drafting",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "team@riverbend.co",
      cc: "sandra@riverbend.co",
      subject: "Eastside × Riverbend — weekly status, May 11",
      body: [
        "Hi all,",
        "Three updates this week:",
        "1) Dock changeover stays on track for May 22.",
        "2) Lot 24-118 shipped Friday; POD attached for AP.",
        "3) Q3 forecast looks within ±4% of plan.",
        "Holler if anything's off.",
        "— Adam",
      ],
    },
    safety: {
      can: ["Send to addresses on the standing list"],
      cannot: ["Add anyone not on the standing list"],
      needsApproval: ["First send after a recipient-list change"],
      stopsIf: ["Body diverges materially from template"],
    },
    audit: [
      { at: "—2d",  actor: "Standing rule", note: "Recipient list updated: +sandra@." },
      { at: "—42m", actor: "Drafting", note: "Composed week-of-May-11 digest." },
    ],
  },
  {
    id: "i07", status: "awaiting", unread: false,
    title: "Cancel the offsite so Q3 close stays clear",
    source: "calendar", risk: "high", minutesAgo: 58,
    summary: "Two people on close-team are on the offsite — pulls focus.",
    cause: "Mass-cancel of an event with 14 invitees is always your call.",
    routedBy: "Calendar",
    draft: {
      kind: "calendar",
      invite: "[CANCELLED] Q3 offsite — Tahoe",
      when: "Jun 23–25 · all day",
      where: "Tahoe — Granlibakken",
      attendees: ["14 invitees"],
      description:
        "Two close-team members in attendance. Suggested: reschedule to Jul 14–16 after Q3 close is filed.",
    },
    safety: {
      can: ["Send polite cancellation note"],
      cannot: ["Cancel events you don't own"],
      needsApproval: ["Mass cancel (≥5 invitees)"],
      stopsIf: ["Any invitee has booked travel"],
    },
    audit: [
      { at: "—3h", actor: "Calendar", note: "Conflict detected with Q3 close window." },
      { at: "—58m", actor: "Calendar", note: "Proposed cancel + reschedule." },
    ],
  },
  {
    id: "i08", status: "awaiting", unread: false,
    title: "Send NDA file to onboarding intake at Halford",
    source: "email", risk: "low", minutesAgo: 76,
    summary: "Halford asked for the standard mutual NDA — file is the canonical one.",
    cause: "First send to halford.io — domain isn't on the standing allow-list yet.",
    routedBy: "Drafting",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "intake@halford.io",
      subject: "Mutual NDA — Eastside",
      body: [
        "Hi team,",
        "Attached is our mutual NDA. Pre-signed by us. If your form requires changes, send a redline and I'll route it.",
        "— Adam",
      ],
    },
    safety: {
      can: ["Send canonical NDA"],
      cannot: ["Send pre-signed contracts other than NDA"],
      needsApproval: ["First send to a new domain"],
      stopsIf: ["File hash differs from canonical"],
    },
    audit: [
      { at: "—2h", actor: "Halford intake", note: "Requested NDA via webform." },
      { at: "—76m", actor: "Drafting", note: "Composed standard NDA send." },
    ],
  },

  // ── Drafts (still being authored, not yet ready for approval) ──────────
  {
    id: "i09", status: "draft", unread: true,
    title: "Drafting: Q3 carbon credit purchase to Riverbend",
    source: "email", risk: "medium", minutesAgo: 6,
    summary: "Half-written — pricing pulled from this morning's Bloomberg index.",
    cause: "Pricing moved 11% overnight; the draft needs your eye before it goes to ready.",
    routedBy: "Drafting",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "sandra@riverbend.co",
      subject: "Carbon credit allocation — Q3",
      body: [
        "Hi Sandra,",
        "Quick heads-up before the call: spot moved to $89.40 this morning, up 11% on the three-month range.",
        "[DRAFT — pricing block needs your acknowledgement before I close this out.]",
      ],
    },
    safety: {
      can: ["Draft in private", "Pull from Bloomberg index"],
      cannot: ["Send without you ackowledging price"],
      needsApproval: ["Any purchase email"],
      stopsIf: ["Price moves another 5% before send"],
    },
    audit: [
      { at: "—1h",  actor: "Research",  note: "Published: carbon spot +11%." },
      { at: "—6m",  actor: "Drafting",  note: "Started reply; price block stubbed." },
    ],
  },
  {
    id: "i10", status: "draft", unread: false,
    title: "Drafting: thank-you note to Sandra Lopez",
    source: "email", risk: "low", minutesAgo: 14,
    summary: "Short note for the customer-review prep call yesterday.",
    cause: "Personal-tone notes are surfaced so you can sign in your own voice.",
    routedBy: "Drafting",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "sandra@riverbend.co",
      subject: "Yesterday",
      body: [
        "Sandra,",
        "Genuinely appreciated you walking through the inventory hand-off yesterday. The detail on the dock crew made the difference.",
        "[DRAFT — add the line about her dad's surgery? Up to you.]",
      ],
    },
    safety: {
      can: ["Draft personal notes"],
      cannot: ["Send without you reading it"],
      needsApproval: ["Personal-tone external sends"],
      stopsIf: ["Tone reads as generic / templated"],
    },
    audit: [
      { at: "—1d",  actor: "Calendar",  note: "Logged the prep call." },
      { at: "—14m", actor: "Drafting",  note: "Composed thank-you draft." },
    ],
  },
  {
    id: "i11", status: "draft", unread: false,
    title: "Drafting: Monday all-hands re-route",
    source: "calendar", risk: "medium", minutesAgo: 25,
    summary: "Moves all-hands to Tuesday 11am — most attendees free, two are not.",
    cause: "All-hands moves always hit you; the conflict count is non-zero.",
    routedBy: "Calendar",
    draft: {
      kind: "calendar",
      invite: "Eastside all-hands — weekly",
      when: "Tue, May 12 · 11:00 – 11:45 AM PT (was Mon 10am)",
      where: "Conference Room A + video",
      attendees: ["Org-wide (47)"],
      description:
        "Move to clear Monday for Q3 close. Two attendees have conflicts: Priya (1:1) and Sam (off-site). Optional invite or async update for both.",
    },
    safety: {
      can: ["Move recurring all-hands once"],
      cannot: ["Move into focus blocks"],
      needsApproval: ["Any all-hands move"],
      stopsIf: ["More than 3 hard conflicts"],
    },
    audit: [
      { at: "—1h",  actor: "Operations", note: "Flagged Monday close-day conflict." },
      { at: "—25m", actor: "Calendar",   note: "Composed re-route draft." },
    ],
  },
  {
    id: "i12", status: "draft", unread: false,
    title: "Drafting: termination notice for vendor Yates Holdings",
    source: "email", risk: "high", minutesAgo: 38,
    summary: "Sensitive — legal pre-reviewed phrasing, awaiting your final read.",
    cause: "All vendor terminations are sensitive enough to hold for your final word.",
    routedBy: "Drafting",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "ops@yatesholdings.com",
      cc: "legal@eastside.co",
      subject: "Notice of termination — MSA dated 2023-11",
      body: [
        "To whom it may concern,",
        "Per Section 12.2 of the master services agreement dated 2023-11, Eastside is terminating the MSA for cause, effective 30 days from receipt of this notice.",
        "[DRAFT — legal phrasing pre-reviewed. Awaiting your final read.]",
      ],
    },
    safety: {
      can: ["Compose against legal-reviewed template"],
      cannot: ["Send without your read"],
      needsApproval: ["Any termination notice"],
      stopsIf: ["Counterparty contact has changed"],
    },
    audit: [
      { at: "—2d",  actor: "Legal",    note: "Reviewed template; no further changes." },
      { at: "—38m", actor: "Drafting", note: "Composed termination notice." },
    ],
  },
  {
    id: "i13", status: "draft", unread: false,
    title: "Drafting: end-of-day status for ops chat",
    source: "telegram", risk: "low", minutesAgo: 47,
    summary: "Auto-compiled from today's CI / heartbeat / inbox events.",
    cause: "End-of-day status is held for a quick edit before posting.",
    routedBy: "Drafting",
    draft: {
      kind: "telegram",
      chat: "Eastside · #ops",
      reply_to: "(new message)",
      body:
        "EOD status — CI: green (47 builds). Heartbeat: healthy, 18/19 systems. Inbox: 3 awaiting, 0 blocked. Deploy window 10–11pm PT. PSA: research-index rebuild finishes by 9pm.",
    },
    safety: {
      can: ["Post EOD status to #ops"],
      cannot: ["@everyone tags", "Repost into customer channels"],
      needsApproval: ["EOD status when blocked > 0"],
      stopsIf: ["Heartbeat goes red before post"],
    },
    audit: [
      { at: "—1h",  actor: "Operations", note: "Compiled today's signal bundle." },
      { at: "—47m", actor: "Drafting",   note: "Composed status draft." },
    ],
  },

  // ── Held for review ───────────────────────────────────────────────────
  {
    id: "i14", status: "held", unread: true,
    title: "Held: defensive-toned reply to legal@oldcompany",
    source: "email", risk: "high", minutesAgo: 73,
    summary: "Tone check flagged the original draft as defensive; isolated until you read it.",
    cause: "Two of three reviewers wanted softer language. Held so nothing went out.",
    routedBy: "Inbox triage",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "legal@oldcompany.com",
      subject: "Re: contract amendment — pushback",
      body: [
        "[Held draft — original tone flagged. A softer rewrite is queued below.]",
        "Hi —",
        "We're not in a position to accept the indemnification language as written; the scope reads broader than the underlying engagement. Happy to walk through what we can sign.",
        "— Adam",
      ],
    },
    safety: {
      can: ["Hold a draft until operator reads it"],
      cannot: ["Send a draft that failed tone check"],
      needsApproval: ["Releasing a held draft"],
      stopsIf: ["Tone check still fails on re-write"],
    },
    audit: [
      { at: "—4h",  actor: "oldcompany.com", note: "Inbound: contract amendment." },
      { at: "—2h",  actor: "Inbox triage",   note: "Composed first reply." },
      { at: "—73m", actor: "Tone check",     note: "Flagged as defensive (score 0.34)." },
      { at: "—73m", actor: "Inbox triage",   note: "Held draft for your review." },
    ],
  },
  {
    id: "i15", status: "held", unread: false,
    title: "Held: NDA forward to external counsel (not on allow-list)",
    source: "email", risk: "high", minutesAgo: 124,
    summary: "Drafted forward of confidential NDA; recipient not on allow-list.",
    cause: "External counsel ben@hartlaw.com isn't on this workspace's allow-list yet.",
    routedBy: "Drafting",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "ben@hartlaw.com",
      subject: "FYI — NDA from Riverbend",
      body: [
        "Ben,",
        "FYI — Riverbend's signed NDA attached. No action needed unless you spot something.",
        "[Held — hartlaw.com is not on this workspace's allow-list.]",
      ],
    },
    safety: {
      can: ["Forward to allow-listed counsel"],
      cannot: ["Forward to non-allow-listed recipients"],
      needsApproval: ["Adding a new recipient to allow-list"],
      stopsIf: ["Recipient is on a deny-list"],
    },
    audit: [
      { at: "—5h",  actor: "Drafting", note: "Composed forward." },
      { at: "—2h",  actor: "Policy",   note: "Held: recipient not allow-listed." },
    ],
  },
  {
    id: "i16", status: "held", unread: false,
    title: "Held: unusual $812 purchase to Yates Holdings",
    source: "webhook", risk: "high", minutesAgo: 168,
    summary: "Purchasing tried to submit a PO; vendor has never appeared here before.",
    cause: "First-time vendors get held until you confirm the supplier is real.",
    routedBy: "Purchasing",
    draft: {
      kind: "webhook",
      endpoint: "POST /v1/po/submit",
      method: "POST",
      fields: [
        { label: "vendor", value: "Yates Holdings (NEW)" },
        { label: "amount_usd", value: "812.00" },
        { label: "category", value: "Maintenance" },
        { label: "memo", value: "Pump replacement, lot 24-099" },
      ],
    },
    safety: {
      can: ["Submit POs to known vendors"],
      cannot: ["Submit POs to unverified vendors"],
      needsApproval: ["First PO to any new vendor"],
      stopsIf: ["Vendor fails fraud check"],
    },
    audit: [
      { at: "—3h",  actor: "Purchasing", note: "Composed PO from invoice." },
      { at: "—2h",  actor: "Policy",     note: "Held: new payee, never seen here." },
    ],
  },

  // ── Done today ─────────────────────────────────────────────────────────
  {
    id: "i17", status: "done", unread: false,
    title: "Sent: weekly status to Riverbend (last Monday's cadence)",
    source: "email", risk: "low", minutesAgo: 244,
    summary: "Standing Monday digest; sent inside the cadence rule.",
    cause: "Recurring send — would not be in your inbox if anything had drifted.",
    routedBy: "Drafting",
    draft: {
      kind: "email",
      from: "adam@eastside.co", to: "team@riverbend.co",
      subject: "Eastside × Riverbend — weekly status, May 4",
      body: ["(sent — full message archived in Drafting/Sent)"],
    },
    safety: { can: [], cannot: [], needsApproval: [], stopsIf: [] },
    audit: [
      { at: "—4h", actor: "Drafting", note: "Sent under standing rule R-04." },
      { at: "—4h", actor: "Outbound mail", note: "Delivered to 6 recipients." },
    ],
  },
  {
    id: "i18", status: "done", unread: false,
    title: "Booked: Friday morning focus block",
    source: "calendar", risk: "low", minutesAgo: 252,
    summary: "Two-hour focus block; calendar was clear.",
    cause: "You asked for a weekly block and Friday was open.",
    routedBy: "Calendar",
    draft: {
      kind: "calendar",
      invite: "Focus block — strategy",
      when: "Fri · 9:00 – 11:00 AM PT",
      where: "(blocked, no room)",
      attendees: ["Adam Stern (you)"],
      description: "Recurring weekly focus block; calendar self-managed.",
    },
    safety: { can: [], cannot: [], needsApproval: [], stopsIf: [] },
    audit: [
      { at: "—4h", actor: "Calendar", note: "Booked focus block on own calendar." },
    ],
  },
  {
    id: "i19", status: "done", unread: false,
    title: "Filed: Stagecoach Coffee receipt under Travel",
    source: "webhook", risk: "low", minutesAgo: 270,
    summary: "$14.20 receipt matched a travel-tagged calendar event.",
    cause: "Auto-categorized under your standing $200/expense rule.",
    routedBy: "Drafting",
    draft: {
      kind: "webhook",
      endpoint: "POST /v1/expenses",
      method: "POST",
      fields: [
        { label: "category", value: "Travel" },
        { label: "amount_usd", value: "14.20" },
        { label: "receipt", value: "rcpt_2KkLM.pdf" },
      ],
    },
    safety: { can: [], cannot: [], needsApproval: [], stopsIf: [] },
    audit: [
      { at: "—5h", actor: "Drafting", note: "Filed under Travel ($14.20)." },
    ],
  },
  {
    id: "i20", status: "done", unread: false,
    title: "Sent: reply to operations weekly digest ping",
    source: "telegram", risk: "low", minutesAgo: 312,
    summary: "Quick ack to Priya's weekly digest question.",
    cause: "Single-line internal acks under the standing reply rule.",
    routedBy: "Drafting",
    draft: {
      kind: "telegram",
      chat: "Eastside · #ops",
      reply_to: "Priya N. — 'digest still goes 5pm?'",
      body: "yep — same time.",
    },
    safety: { can: [], cannot: [], needsApproval: [], stopsIf: [] },
    audit: [
      { at: "—5h", actor: "Drafting", note: "Replied in #ops under standing rule." },
    ],
  },
];

// ───────────────────────────────────────────────────────────────────────────
//  Filter chips
// ───────────────────────────────────────────────────────────────────────────

type ChipId = "all" | "awaiting" | "draft" | "held" | "done";

const CHIPS: { id: ChipId; label: string; tone: "neutral" | "warning" | "accent" | "blocked" | "safe" }[] = [
  { id: "all",      label: "All",                 tone: "neutral" },
  { id: "awaiting", label: "Awaiting approval",   tone: "warning" },
  { id: "draft",    label: "Drafts",              tone: "accent"  },
  { id: "held",     label: "Held for review",     tone: "blocked" },
  { id: "done",     label: "Done today",          tone: "safe"    },
];

function applyChip(items: Item[], chip: ChipId): Item[] {
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

// ───────────────────────────────────────────────────────────────────────────
//  Reducer — handles approve / deny / snooze / edit-commit with optimistic UI
// ───────────────────────────────────────────────────────────────────────────

type ItemAction =
  | { type: "approve"; id: string }
  | { type: "deny"; id: string; cause: string }
  | { type: "snooze"; id: string }
  | { type: "edit-commit"; id: string; draft: Draft }
  | { type: "mark-read"; id: string };

function itemsReducer(state: Item[], action: ItemAction): Item[] {
  switch (action.type) {
    case "approve":
      return state.map((it) =>
        it.id === action.id
          ? { ...it, status: "done", minutesAgo: 0, unread: false,
              cause: "You approved this just now — sent.",
              audit: [{ at: "just now", actor: "You", note: "Approved." }, ...it.audit] }
          : it,
      );
    case "deny":
      return state.map((it) =>
        it.id === action.id
          ? { ...it, status: "held", minutesAgo: 0, unread: false,
              cause: action.cause,
              audit: [{ at: "just now", actor: "You", note: `Denied — ${action.cause}` }, ...it.audit] }
          : it,
      );
    case "snooze":
      return state.filter((it) => it.id !== action.id)
        .concat(state.filter((it) => it.id === action.id)
          .map((it) => ({ ...it, minutesAgo: 60, unread: true,
            audit: [{ at: "just now", actor: "You", note: "Snoozed 1h." }, ...it.audit] })));
    case "edit-commit":
      return state.map((it) =>
        it.id === action.id ? { ...it, draft: action.draft,
          audit: [{ at: "just now", actor: "You", note: "Edited draft." }, ...it.audit] } : it,
      );
    case "mark-read":
      return state.map((it) => it.id === action.id ? { ...it, unread: false } : it);
    default:
      return state;
  }
}

// ───────────────────────────────────────────────────────────────────────────
//  Inline icons (Heroicons-style, stroke 1.5).
//  TODO(integrator): replace with `@irongolem/ui/icons`.
// ───────────────────────────────────────────────────────────────────────────

const IconSvg = ({ d, vb = "0 0 24 24", size = 16, className = "" }:
  { d: React.ReactNode; vb?: string; size?: number; className?: string }) => (
  <svg viewBox={vb} width={size} height={size} fill="none" stroke="currentColor"
       strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round"
       className={className} aria-hidden="true">{d}</svg>
);

const ICON = {
  Mail:     (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><rect x="3" y="5" width="18" height="14" rx="2" /><path d="m4 7 8 6 8-6" /></>} />,
  Calendar: (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M3 9h18M8 3v4M16 3v4" /></>} />,
  Webhook:  (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><circle cx="6" cy="8" r="3" /><path d="m8 11 5 8" /><circle cx="18" cy="18" r="3" /><path d="M15 18H8" /><circle cx="9" cy="18" r="3" /><path d="m20 15-4-6" /></>} />,
  Telegram: (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="m21 4-9 16-3-7-7-3 19-6Z" /><path d="m9 13 7-5" /></>} />,
  Check:    (p: { size?: number; className?: string }) => <IconSvg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  X:        (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  Edit:     (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M4 20h4l10-10-4-4L4 16v4Z" /><path d="m14 6 4 4" /></>} />,
  Clock:    (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>} />,
  Search:   (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><circle cx="11" cy="11" r="6" /><path d="m20 20-4.3-4.3" /></>} />,
  Inbox:    (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M3 12V5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v7" /><path d="M3 12h5l1.5 2.5h5L16 12h5v6a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-6Z" /></>} />,
  Alert:    (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx="12" cy="17.5" r=".5" fill="currentColor" stroke="none" /></>} />,
  Shield:   (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M12 3 5 6v6c0 4 3 7.5 7 9 4-1.5 7-5 7-9V6l-7-3Z" /></>} />,
  Slash:    (p: { size?: number; className?: string }) => <IconSvg {...p} d={<path d="M5 19 19 5" />} />,
  Bell:     (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2Z" /><path d="M10 20a2 2 0 0 0 4 0" /></>} />,
  Pause:    (p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M9 5v14" /><path d="M15 5v14" /></>} />,
  ArrowLeft:(p: { size?: number; className?: string }) => <IconSvg {...p} d={<><path d="M19 12H5" /><path d="m11 6-6 6 6 6" /></>} />,
  Logo:     (p: { size?: number; className?: string }) => <IconSvg {...p} vb="0 0 20 24" d={<><rect x="3" y="6" width="6" height="2.5" rx="1" /><rect x="3" y="11" width="14" height="2.5" rx="1" /><rect x="3" y="16" width="9" height="2.5" rx="1" /></>} />,
};

const SourceMeta: Record<Source, { label: string; icon: React.ComponentType<{ size?: number; className?: string }> }> = {
  email:    { label: "Email",    icon: ICON.Mail     },
  calendar: { label: "Calendar", icon: ICON.Calendar },
  webhook:  { label: "Webhook",  icon: ICON.Webhook  },
  telegram: { label: "Telegram", icon: ICON.Telegram },
};

// ───────────────────────────────────────────────────────────────────────────
//  Local placeholders for @irongolem/ui components.
//  Real shop replaces these with the imports.
// ───────────────────────────────────────────────────────────────────────────

// TODO(integrator): import { RiskBadge } from "@irongolem/ui";
function RiskBadge({ level, size = "sm" }: { level: Risk; size?: "sm" | "md" }) {
  const m = ({ low: "safe", medium: "warning", high: "blocked" } as const)[level];
  const label = ({ low: "low risk", medium: "med risk", high: "high risk" } as const)[level];
  const sizeCx = size === "sm" ? "text-[10px] px-1.5 py-0.5" : "text-xs px-2 py-0.5";
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

// TODO(integrator): import { SafetyCard } from "@irongolem/ui";
function SafetyCard({ safety }: { safety: SafetyShape }) {
  const Section = ({ label, items, tone, IconCmp }:
    { label: string; items: string[]; tone: "safe" | "warning" | "blocked" | "quarantined";
      IconCmp: React.ComponentType<{ size?: number; className?: string }> }) => (
    <div>
      <div className="flex items-center gap-1.5 mb-2">
        <span className={`text-${tone}`}><IconCmp size={13} /></span>
        <span className={cx("text-[11px] font-medium uppercase tracking-wide", `text-${tone}`)}>{label}</span>
      </div>
      <ul className="space-y-1">
        {items.length === 0 && <li className="text-xs text-neutral-400">—</li>}
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
        <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">Safety</div>
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

// ───────────────────────────────────────────────────────────────────────────
//  Sub-components
// ───────────────────────────────────────────────────────────────────────────

const cx = (...c: (string | false | null | undefined)[]) => c.filter(Boolean).join(" ");

function SourcePill({ source }: { source: Source }) {
  const m = SourceMeta[source];
  const Icn = m.icon;
  return (
    <span className="inline-flex items-center gap-1 rounded-md border border-neutral-200 bg-white px-1.5 py-0.5 text-[10px] font-medium text-neutral-600">
      <span className="text-neutral-500"><Icn size={11} /></span>
      {m.label}
    </span>
  );
}

function StatusDot({ status }: { status: Status }) {
  const tone = ({ awaiting: "warning", draft: "accent", held: "blocked", done: "safe" } as const)[status];
  return <span className={cx("h-1.5 w-1.5 rounded-full shrink-0", `bg-${tone}-solid`)} />;
}

function FilterChips({
  active, onChange, counts,
}: {
  active: ChipId;
  onChange: (c: ChipId) => void;
  counts: Record<ChipId, number>;
}) {
  return (
    <div className="flex flex-wrap items-center gap-1.5 p-3 border-b border-neutral-100">
      {CHIPS.map((c) => {
        const isActive = c.id === active;
        const n = counts[c.id];
        return (
          <button
            key={c.id}
            type="button"
            onClick={() => onChange(c.id)}
            className={cx(
              "inline-flex items-center gap-1.5 rounded-full text-[12px] font-medium px-2.5 py-1 border transition-colors",
              isActive
                ? `bg-${c.tone}-solid text-white border-${c.tone}-solid`
                : `bg-white text-neutral-700 border-neutral-200 hover:bg-neutral-50`,
            )}
          >
            {c.label}
            <span className={cx(
              "rounded-full font-mono tabular-nums text-[10px] px-1.5 py-px",
              isActive ? "bg-white/25 text-white" : "bg-neutral-100 text-neutral-500",
            )}>
              {n}
            </span>
          </button>
        );
      })}
    </div>
  );
}

function InboxRow({
  item, selected, onSelect,
}: {
  item: Item;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cx(
        "w-full text-left px-4 py-3 border-l-2 transition-colors block",
        selected
          ? "bg-accent border-l-accent-solid"
          : "bg-white border-l-transparent hover:bg-neutral-50",
      )}
    >
      <div className="flex items-start gap-2">
        <span className={cx(
          "mt-1.5 h-2 w-2 rounded-full shrink-0",
          item.unread ? "bg-accent-solid" : "bg-transparent",
        )} aria-label={item.unread ? "unread" : "read"} />

        <div className="flex-1 min-w-0">
          <div className="flex items-baseline justify-between gap-2">
            <div className={cx(
              "text-[13.5px] leading-snug truncate",
              item.unread ? "font-semibold text-neutral-900" : "font-medium text-neutral-800",
            )}>
              {item.title}
            </div>
            <div className="text-[11px] text-neutral-400 shrink-0 tabular-nums">
              {relTime(item.minutesAgo)}
            </div>
          </div>

          <div className="mt-1.5 flex items-center gap-1.5 flex-wrap">
            <SourcePill source={item.source} />
            {/* TODO(integrator): swap for <RiskBadge level={item.risk} /> from @irongolem/ui */}
            <RiskBadge level={item.risk} />
            <span className="inline-flex items-center gap-1 text-[10px] text-neutral-500">
              <StatusDot status={item.status} />
              {item.routedBy}
            </span>
          </div>

          <p className="mt-1.5 text-[12.5px] text-neutral-600 line-clamp-1">
            {item.summary}
          </p>
          <p className="mt-0.5 text-[11.5px] text-neutral-400 leading-snug line-clamp-1">
            <span className="text-neutral-500 font-medium">Why this is here:</span> {item.cause}
          </p>
        </div>
      </div>
    </button>
  );
}

function DraftedBlock({ draft, editing, onChange }: {
  draft: Draft;
  editing: boolean;
  onChange: (next: Draft) => void;
}) {
  if (draft.kind === "email") {
    return (
      <article className="rounded-xl border border-neutral-200 bg-white overflow-hidden">
        <header className="px-4 py-3 border-b border-neutral-100 bg-neutral-50/60">
          <KV label="From" value={draft.from} />
          <KV label="To" value={draft.to} />
          {draft.cc && <KV label="Cc" value={draft.cc} />}
          <div className="mt-1.5">
            <span className="text-[10px] font-medium uppercase tracking-wide text-neutral-400 mr-2">Subject</span>
            {editing
              ? <input value={draft.subject}
                       onChange={(e) => onChange({ ...draft, subject: e.target.value })}
                       className="text-[14px] font-semibold tracking-tight text-neutral-900 bg-transparent w-[calc(100%-4.5rem)] outline-none focus:bg-white focus:ring-2 ring-accent rounded px-1" />
              : <span className="text-[14px] font-semibold tracking-tight text-neutral-900">{draft.subject}</span>}
          </div>
        </header>
        <div className="px-4 py-4 space-y-3 text-[14px] leading-relaxed text-neutral-800">
          {editing
            ? <textarea
                value={draft.body.join("\n\n")}
                onChange={(e) => onChange({ ...draft, body: e.target.value.split("\n\n") })}
                className="w-full min-h-[160px] resize-y outline-none bg-neutral-50/60 border border-neutral-200 rounded-md p-2 focus:ring-2 ring-accent font-sans" />
            : draft.body.map((para, i) => <p key={i}>{para}</p>)
          }
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
          <span className="text-neutral-500">When</span><span className="text-neutral-900 font-medium">{draft.when}</span>
          <span className="text-neutral-500">Where</span><span className="text-neutral-700">{draft.where}</span>
          <span className="text-neutral-500">With</span>
          <span className="flex flex-wrap gap-1">
            {draft.attendees.map((a) => (
              <span key={a} className="inline-flex items-center rounded-md bg-neutral-100 px-1.5 py-0.5 text-[11px] text-neutral-700">{a}</span>
            ))}
          </span>
          <span className="text-neutral-500">Note</span><span className="text-neutral-700 leading-relaxed">{draft.description}</span>
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
            {editing
              ? <textarea value={draft.body}
                          onChange={(e) => onChange({ ...draft, body: e.target.value })}
                          className="w-full min-h-[60px] bg-transparent outline-none resize-none" />
              : draft.body}
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

function KV({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline gap-2 leading-tight">
      <span className="text-[10px] font-medium uppercase tracking-wide text-neutral-400 w-12 shrink-0">{label}</span>
      <span className="text-[13px] text-neutral-700 truncate">{value}</span>
    </div>
  );
}

function OriginChips({ item }: { item: Item }) {
  const m = SourceMeta[item.source];
  const Icn = m.icon;
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
        <StatusDot status={item.status} /> routed by <span className="text-neutral-700 font-medium">{item.routedBy}</span>
      </span>
    </div>
  );
}

function WhyCallout({ cause }: { cause: string }) {
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

function AuditTrail({ steps }: { steps: AuditStep[] }) {
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

function DetailDrawer({
  item, editing, draftBuffer,
  onEdit, onCancelEdit, onCommitEdit, onChangeBuffer,
  onApprove, onDeny, onSnooze, onBack,
}: {
  item: Item;
  editing: boolean;
  draftBuffer: Draft | undefined;
  onEdit: () => void;
  onCancelEdit: () => void;
  onCommitEdit: () => void;
  onChangeBuffer: (d: Draft) => void;
  onApprove: () => void;
  onDeny: () => void;
  onSnooze: () => void;
  onBack: () => void;
}) {
  const isActionable = item.status !== "done";

  return (
    <section className="h-full flex flex-col bg-white">
      {/* sticky top bar */}
      <header className="sticky top-0 z-10 bg-white/85 backdrop-blur border-b border-neutral-100 px-5 py-3 flex items-center gap-2">
        <button type="button" onClick={onBack}
                className="md:hidden inline-flex items-center gap-1 text-[12px] font-medium text-neutral-700 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
          <ICON.ArrowLeft size={14} /> Back
        </button>
        <span className="text-[11px] font-mono text-neutral-400 truncate">{item.id.toUpperCase()}</span>
        <span className="text-neutral-300">·</span>
        <span className="inline-flex items-center gap-1.5 text-[11px] text-neutral-500">
          <StatusDot status={item.status} />
          {({ awaiting: "Awaiting approval", draft: "Draft", held: "Held for review", done: "Done" } as const)[item.status]}
        </span>
        <div className="ml-auto flex items-center gap-1">
          <button type="button" onClick={onSnooze}
                  className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-600 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
            <ICON.Clock size={13} /> Snooze 1h
          </button>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto scrollbar-thin">
        <div className="px-6 sm:px-8 py-6 max-w-3xl">
          <h1 className="page-title">{item.title}</h1>
          <div className="mt-3"><OriginChips item={item} /></div>

          {item.draft && (
            <div className="mt-6">
              <div className="flex items-center justify-between mb-2">
                <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">Drafted content</div>
                {isActionable && (
                  editing ? (
                    <div className="flex items-center gap-1">
                      <button type="button" onClick={onCancelEdit}
                              className="text-[12px] font-medium text-neutral-500 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
                        Cancel
                      </button>
                      <button type="button" onClick={onCommitEdit}
                              className="text-[12px] font-medium text-accent hover:text-accent-solid px-2 py-1 rounded-md hover:bg-accent-hover">
                        Save changes
                      </button>
                    </div>
                  ) : (
                    <button type="button" onClick={onEdit}
                            className="inline-flex items-center gap-1 text-[12px] font-medium text-neutral-600 hover:text-neutral-900 px-2 py-1 rounded-md hover:bg-neutral-50">
                      <ICON.Edit size={12} /> Edit draft
                    </button>
                  )
                )}
              </div>
              <DraftedBlock
                draft={editing && draftBuffer ? draftBuffer : item.draft}
                editing={editing}
                onChange={onChangeBuffer}
              />
            </div>
          )}

          <div className="mt-6"><WhyCallout cause={item.cause} /></div>

          <div className="mt-6">
            {/* TODO(integrator): swap for <SafetyCard ... /> from @irongolem/ui */}
            <SafetyCard safety={item.safety} />
          </div>

          <div className="mt-8">
            <div className="text-xs font-medium uppercase tracking-wide text-neutral-500 mb-3">Audit trail</div>
            <AuditTrail steps={item.audit} />
          </div>

          {/* breathing room above sticky action bar */}
          <div className="h-24" />
        </div>
      </div>

      {/* Sticky action row at the bottom */}
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

function Toast({ toast }: { toast: { kind: "approved" | "denied" | "snoozed"; title: string } }) {
  const tone = toast.kind === "approved" ? "safe" : toast.kind === "denied" ? "blocked" : "neutral";
  const verb = toast.kind === "approved" ? "Approved · sent" : toast.kind === "denied" ? "Denied · held for review" : "Snoozed 1h";
  return (
    <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-50 pointer-events-none"
         style={{ animation: "ig-toast-in 200ms ease-out" }}>
      <div className={cx(
        "pointer-events-auto rounded-lg border shadow-lg bg-white px-4 py-3 flex items-center gap-3 min-w-[280px] max-w-[420px]",
        `border-${tone}`,
      )}>
        <span className={`text-${tone}`}>
          {toast.kind === "approved" ? <ICON.Check size={16} /> : toast.kind === "denied" ? <ICON.X size={16} /> : <ICON.Clock size={16} />}
        </span>
        <div className="flex-1 min-w-0">
          <div className={cx("text-[10.5px] font-medium uppercase tracking-wide", `text-${tone}`)}>{verb}</div>
          <div className="text-[13.5px] text-neutral-900 truncate">{toast.title}</div>
        </div>
      </div>
      <style>{`@keyframes ig-toast-in { from { transform: translate(-50%, 8px); opacity: 0; } to { transform: translate(-50%, 0); opacity: 1; } }`}</style>
    </div>
  );
}

function EmptyInbox() {
  return (
    <div className="h-full flex items-center justify-center px-8">
      <div className="text-center max-w-md">
        <span className="h-12 w-12 rounded-full bg-safe text-safe inline-flex items-center justify-center mb-4">
          <ICON.Check size={22} />
        </span>
        <h2 className="text-[20px] font-semibold tracking-tight text-neutral-900">
          Your inbox is empty
        </h2>
        <p className="mt-2 text-[14px] text-neutral-600 leading-relaxed">
          Your assistant teams are handling everything inside the rules. We'll surface anything that needs you here — and only that.
        </p>
      </div>
    </div>
  );
}

// Minimal in-route header so the page reads as part of the app shell.
// TODO(integrator): swap for the shared <AppShell> chrome.
function AppHeader({ counts }: { counts: Record<ChipId, number> }) {
  return (
    <header className="sticky top-0 z-30 bg-white border-b border-neutral-100"
            style={{ backdropFilter: "saturate(160%) blur(8px)" }}>
      <div className="page-container py-0">
        <div className="flex items-center h-14 gap-4">
          <div className="flex items-center gap-2 shrink-0">
            <span className="h-7 w-7 rounded-md bg-neutral-900 text-white inline-flex items-center justify-center">
              <ICON.Logo size={16} />
            </span>
            <span className="text-sm font-semibold tracking-tight text-neutral-900">IronGolem</span>
            <span className="text-neutral-300">/</span>
            <span className="text-sm text-neutral-700 inline-flex items-center gap-1.5">
              <span className="h-5 w-5 rounded bg-accent text-accent inline-flex items-center justify-center text-[10px] font-semibold">EP</span>
              Eastside Production
            </span>
          </div>

          <nav className="hidden md:flex items-center gap-1 ml-4">
            {[
              ["Workspace", false],
              ["Inbox", true],
              ["Timeline", false],
              ["Teams", false],
              ["Research", false],
              ["Rules", false],
            ].map(([label, active]) => (
              <a key={label as string} href={`#${(label as string).toLowerCase()}`}
                 className={cx(
                   "px-2.5 py-1.5 rounded-md text-sm font-medium transition-colors inline-flex items-center gap-1.5",
                   active ? "text-neutral-900 bg-neutral-100" : "text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50",
                 )}>
                {label === "Inbox" && <ICON.Inbox size={14} />}
                {label}
                {active && counts.awaiting > 0 && (
                  <span className="ml-1 rounded-full bg-warning-solid text-white font-mono text-[10px] px-1.5 py-px tabular-nums">
                    {counts.awaiting}
                  </span>
                )}
              </a>
            ))}
          </nav>

          <div className="flex-1" />

          <span className="h-7 w-7 rounded-full bg-accent text-accent inline-flex items-center justify-center text-[11px] font-semibold">
            AS
          </span>
        </div>
      </div>
    </header>
  );
}

// ───────────────────────────────────────────────────────────────────────────
//  The route component
// ───────────────────────────────────────────────────────────────────────────

export function Inbox() {
  const [items, dispatch] = useReducer(itemsReducer, MOCK_ITEMS);
  const [chip, setChip] = useState<ChipId>("awaiting");
  const [selectedId, setSelectedId] = useState<string | null>(MOCK_ITEMS[0].id);
  const [editing, setEditing] = useState(false);
  const [draftBuffer, setDraftBuffer] = useState<Draft | undefined>(undefined);
  const [toast, setToast] = useState<{ kind: "approved" | "denied" | "snoozed"; title: string } | null>(null);
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false);
  const [showEmpty, setShowEmpty] = useState(false); // toggle to preview empty state

  // Counts per chip, computed from the live state so optimistic moves tick.
  const counts: Record<ChipId, number> = useMemo(() => ({
    all:      items.length,
    awaiting: items.filter((i) => i.status === "awaiting").length,
    draft:    items.filter((i) => i.status === "draft").length,
    held:     items.filter((i) => i.status === "held").length,
    done:     items.filter((i) => i.status === "done").length,
  }), [items]);

  const visible = useMemo(() => {
    const filtered = applyChip(items, chip);
    // Sort: unread first, then most recent (lowest minutesAgo).
    return filtered.slice().sort((a, b) => {
      if (a.unread !== b.unread) return a.unread ? -1 : 1;
      return a.minutesAgo - b.minutesAgo;
    });
  }, [items, chip]);

  const selected = useMemo(
    () => items.find((i) => i.id === selectedId) || visible[0] || null,
    [items, selectedId, visible],
  );

  // Mark-read on selection.
  useEffect(() => {
    if (selected && selected.unread) {
      dispatch({ type: "mark-read", id: selected.id });
    }
  }, [selected?.id]);

  // Toast auto-dismiss
  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 2400);
    return () => clearTimeout(t);
  }, [toast]);

  // If chip empties, jump selection to the first visible item.
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
      <AppHeader counts={counts} />

      {/* Page heading band */}
      <div className="page-container pt-6 pb-4">
        <div className="flex items-end justify-between gap-4 flex-wrap">
          <div>
            <h1 className="page-title">Inbox</h1>
            <p className="text-[14px] text-neutral-600 mt-1 max-w-xl">
              Everything that needs your eye. Suppressed-on-OK — items the system handled cleanly never appear here.
            </p>
          </div>
          <div className="flex items-center gap-3">
            <label className="inline-flex items-center gap-2 text-[12px] text-neutral-500 select-none cursor-pointer">
              <input type="checkbox" checked={showEmpty} onChange={(e) => setShowEmpty(e.target.checked)}
                     className="h-3.5 w-3.5 accent-current text-accent-solid" />
              Preview empty state
            </label>
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
      </div>

      {/* Two-column container */}
      <div className="page-container pb-6 flex-1">
        <div className="rounded-2xl border border-neutral-200 bg-white overflow-hidden shadow-sm">
          <div className="md:grid md:grid-cols-[24rem_1fr] md:min-h-[640px]">
            {/* ── List column ───────────────────────────────────────────── */}
            <aside className={cx(
              "border-r border-neutral-100 flex flex-col",
              mobileDetailOpen ? "hidden md:flex" : "flex",
            )}>
              <FilterChips active={chip} onChange={setChip} counts={counts} />

              <div className="flex-1 overflow-y-auto scrollbar-thin divide-y divide-neutral-100">
                {showEmpty || visible.length === 0 ? (
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

            {/* ── Detail column ─────────────────────────────────────────── */}
            <div className={cx(
              "min-h-[640px]",
              !mobileDetailOpen ? "hidden md:block" : "block",
            )}>
              {showEmpty ? (
                <EmptyInbox />
              ) : selected ? (
                <DetailDrawer
                  item={selected}
                  editing={editing}
                  draftBuffer={draftBuffer}
                  onEdit={onEdit}
                  onCancelEdit={() => { setEditing(false); setDraftBuffer(undefined); }}
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

// Expose for the in-browser preview shell. Real builds drop this branch.
declare const window: any;
if (typeof window !== "undefined") {
  (window as any).Inbox = Inbox;
}
