// route: /inbox — typed mock data for the Inbox page.
// Consumed via `api.v2.inbox.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema ApprovalRequest model once backend stabilises.

export type Source = "email" | "telegram" | "webhook" | "calendar";
export type Risk = "low" | "medium" | "high";
export type Status = "awaiting" | "draft" | "held" | "done";

export interface EmailDraft {
  readonly kind: "email";
  readonly from: string;
  readonly to: string;
  readonly cc?: string;
  subject: string;
  body: readonly string[];
}

export interface CalendarDraft {
  readonly kind: "calendar";
  readonly invite: string;
  readonly when: string;
  readonly where: string;
  readonly attendees: readonly string[];
  readonly description: string;
}

export interface WebhookDraft {
  readonly kind: "webhook";
  readonly endpoint: string;
  readonly method: "POST" | "PUT" | "PATCH" | "DELETE";
  readonly fields: ReadonlyArray<{ readonly label: string; readonly value: string }>;
}

export interface TelegramDraft {
  readonly kind: "telegram";
  readonly chat: string;
  readonly reply_to: string;
  body: string;
}

export type Draft = EmailDraft | CalendarDraft | WebhookDraft | TelegramDraft;

export interface SafetyShape {
  readonly can: readonly string[];
  readonly cannot: readonly string[];
  readonly needsApproval: readonly string[];
  readonly stopsIf: readonly string[];
}

export interface AuditStep {
  readonly at: string;
  readonly actor: string;
  readonly note: string;
}

export interface Item {
  readonly id: string;
  status: Status;
  readonly title: string;
  readonly source: Source;
  readonly risk: Risk;
  minutesAgo: number;
  readonly summary: string;
  cause: string;
  readonly routedBy: string;
  unread: boolean;
  draft?: Draft;
  readonly safety: SafetyShape;
  audit: readonly AuditStep[];
}

export const mockInboxItems: readonly Item[] = [
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
      { at: "—9m", actor: "Drafting", note: "Composed reply against template R-04." },
      { at: "—2m", actor: "Tone check", note: "Passed — neutral, factual." },
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
      { at: "—11m", actor: "Calendar", note: "Found common slot Thu 2pm; held Room A." },
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
      { at: "—2d", actor: "Standing rule", note: "Recipient list updated: +sandra@." },
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
      cannot: ["Send without you acknowledging price"],
      needsApproval: ["Any purchase email"],
      stopsIf: ["Price moves another 5% before send"],
    },
    audit: [
      { at: "—1h", actor: "Research", note: "Published: carbon spot +11%." },
      { at: "—6m", actor: "Drafting", note: "Started reply; price block stubbed." },
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
      { at: "—1d", actor: "Calendar", note: "Logged the prep call." },
      { at: "—14m", actor: "Drafting", note: "Composed thank-you draft." },
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
      { at: "—1h", actor: "Operations", note: "Flagged Monday close-day conflict." },
      { at: "—25m", actor: "Calendar", note: "Composed re-route draft." },
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
      { at: "—2d", actor: "Legal", note: "Reviewed template; no further changes." },
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
      { at: "—1h", actor: "Operations", note: "Compiled today's signal bundle." },
      { at: "—47m", actor: "Drafting", note: "Composed status draft." },
    ],
  },
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
      { at: "—4h", actor: "oldcompany.com", note: "Inbound: contract amendment." },
      { at: "—2h", actor: "Inbox triage", note: "Composed first reply." },
      { at: "—73m", actor: "Tone check", note: "Flagged as defensive (score 0.34)." },
      { at: "—73m", actor: "Inbox triage", note: "Held draft for your review." },
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
      { at: "—5h", actor: "Drafting", note: "Composed forward." },
      { at: "—2h", actor: "Policy", note: "Held: recipient not allow-listed." },
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
      { at: "—3h", actor: "Purchasing", note: "Composed PO from invoice." },
      { at: "—2h", actor: "Policy", note: "Held: new payee, never seen here." },
    ],
  },
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
    audit: [{ at: "—4h", actor: "Calendar", note: "Booked focus block on own calendar." }],
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
    audit: [{ at: "—5h", actor: "Drafting", note: "Filed under Travel ($14.20)." }],
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
    audit: [{ at: "—5h", actor: "Drafting", note: "Replied in #ops under standing rule." }],
  },
];
