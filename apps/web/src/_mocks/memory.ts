// route: /memory — typed mock data for the Memory page.
// Consumed via `api.v2.memory.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema once the backend models stabilise.

export type Subject = "people" | "accounts" | "preferences" | "patterns";

export interface Evidence {
  readonly id: string;
  readonly when: string;
  readonly source: string;
  readonly ref: string;
  readonly snippet: string;
}

export interface MemoryItem {
  readonly id: string;
  readonly subject: Subject;
  readonly subjectLabel: string;
  readonly fact: string;
  readonly confidence: number;
  readonly lastTouchedHours: number;
  readonly lastTouchedLabel: string;
  readonly verified: boolean;
  readonly evidence: readonly Evidence[];
  readonly tags: readonly string[];
}

export const mockMemory: readonly MemoryItem[] = [
  {
    id: "m01", subject: "people", subjectLabel: "Sarah Lopez — calendar",
    fact: "Prefers Thursday afternoons for status meetings; declines anything before 10am.",
    confidence: 94, lastTouchedHours: 2, lastTouchedLabel: "Verified 2h ago", verified: true,
    tags: ["scheduling", "exec"],
    evidence: [
      { id: "e01a", when: "2h ago", source: "Calendar history", ref: "calendar/sarah-lopez", snippet: "Declined 09:30 invite, suggested Thursday 14:00." },
      { id: "e01b", when: "1w ago", source: "Calendar history", ref: "calendar/sarah-lopez", snippet: "Three of last four 1:1s held Thursday 14:00–14:30." },
      { id: "e01c", when: "3w ago", source: "Direct statement", ref: "thread/2026-04-18", snippet: "\"Mornings are blocked for deep work, Thursdays are easiest.\"" },
    ],
  },
  {
    id: "m02", subject: "accounts", subjectLabel: "Riverbend Co — MSA",
    fact: "Master services agreement renews on Aug 14, 2026. Renewal owner is Priya Shah.",
    confidence: 99, lastTouchedHours: 6, lastTouchedLabel: "Verified 6h ago", verified: true,
    tags: ["contracts"],
    evidence: [
      { id: "e02a", when: "6h ago", source: "Contracts vault", ref: "contracts/riverbend/msa-v3", snippet: "Renewal date field reads 2026-08-14." },
      { id: "e02b", when: "1m ago", source: "Email thread", ref: "thread/priya-msa", snippet: "Priya confirmed she owns the renewal cycle." },
    ],
  },
  {
    id: "m03", subject: "preferences", subjectLabel: "You — drafting tone",
    fact: "You prefer short, plain-English replies. No greetings, no sign-offs in internal threads.",
    confidence: 91, lastTouchedHours: 5, lastTouchedLabel: "Verified 5h ago", verified: true,
    tags: ["voice", "writing"],
    evidence: [
      { id: "e03a", when: "5h ago", source: "Edit history", ref: "edits/draft-114", snippet: "You removed \"Hi team,\" and \"Thanks,\" before sending." },
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
      { id: "e04b", when: "1w ago", source: "Activity log", ref: "log/2026-05-02", snippet: "Same shape: Friday afternoon batch, similar duration." },
      { id: "e04c", when: "5w ago", source: "Activity log", ref: "log/2026-04-04", snippet: "Cadence first detected; 4 of last 6 weeks match." },
    ],
  },
  {
    id: "m05", subject: "people", subjectLabel: "Marcus Hill — drafting voice",
    fact: "Marcus's review notes use clipped declaratives; he edits out hedges like \"might\" and \"perhaps\".",
    confidence: 82, lastTouchedHours: 11, lastTouchedLabel: "Verified 11h ago", verified: true,
    tags: ["voice"],
    evidence: [
      { id: "e05a", when: "11h ago", source: "Edit history", ref: "edits/draft-122", snippet: "Replaced \"might be worth considering\" → \"consider\"." },
      { id: "e05b", when: "5d ago", source: "Edit history", ref: "edits/draft-101", snippet: "Replaced \"perhaps\" → cut entirely." },
    ],
  },
  {
    id: "m06", subject: "accounts", subjectLabel: "Halford — pump supplier",
    fact: "Halford ships maintenance pumps; PO-24-099 references lot 24-118 currently under recall.",
    confidence: 96, lastTouchedHours: 30, lastTouchedLabel: "Verified yesterday", verified: true,
    tags: ["purchasing", "alert"],
    evidence: [
      { id: "e06a", when: "yesterday", source: "Recall feed", ref: "halford.io/recall/2024-118", snippet: "Lot 24-118 listed for replacement." },
      { id: "e06b", when: "1m ago", source: "Workspace PO history", ref: "po/24-099", snippet: "PO references lot 24-118, qty 2." },
    ],
  },
  {
    id: "m07", subject: "preferences", subjectLabel: "You — approval threshold",
    fact: "You approve standing-order purchases under $50 without further review.",
    confidence: 88, lastTouchedHours: 70, lastTouchedLabel: "Verified 3d ago", verified: true,
    tags: ["purchasing"],
    evidence: [
      { id: "e07a", when: "3d ago", source: "Approval log", ref: "approvals/may-2026", snippet: "12 approvals at $42 or less in last 30 days, all single-tap." },
      { id: "e07b", when: "8d ago", source: "Direct statement", ref: "thread/2026-05-02", snippet: "\"Under fifty is fine, just approve.\"" },
    ],
  },
  {
    id: "m08", subject: "patterns", subjectLabel: "Monthly — board update",
    fact: "First Monday of each month you draft the board update between 6pm and 8pm PT.",
    confidence: 84, lastTouchedHours: 24 * 9, lastTouchedLabel: "Last seen 9d ago", verified: false,
    tags: ["recurring", "drafting"],
    evidence: [
      { id: "e08a", when: "9d ago", source: "Activity log", ref: "log/2026-05-02", snippet: "Drafting session 18:14–19:48 PT." },
      { id: "e08b", when: "1m ago", source: "Activity log", ref: "log/2026-04-07", snippet: "Same window; matching doc title pattern." },
      { id: "e08c", when: "2m ago", source: "Activity log", ref: "log/2026-03-03", snippet: "Cadence confirmed across 3 cycles." },
    ],
  },
  {
    id: "m09", subject: "people", subjectLabel: "Dev Patel — escalation contact",
    fact: "Dev is the on-call escalation for ops-bot incidents after 6pm PT weekdays.",
    confidence: 76, lastTouchedHours: 24 * 17, lastTouchedLabel: "Last seen 17d ago", verified: false,
    tags: ["on-call"],
    evidence: [
      { id: "e09a", when: "17d ago", source: "Email thread", ref: "thread/2026-04-24", snippet: "\"Dev's got it after 6 weekdays.\"" },
      { id: "e09b", when: "5w ago", source: "Pager log", ref: "pager/2026-04-08", snippet: "Dev acked at 19:22." },
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
      { id: "e11a", when: "6d ago", source: "Invoice history", ref: "invoices/trent", snippet: "12/12 invoices last year paid on or before the 20th." },
      { id: "e11b", when: "3m ago", source: "Direct statement", ref: "thread/2026-02-10", snippet: "\"Around the 20th works best on our end.\"" },
    ],
  },
  {
    id: "m12", subject: "preferences", subjectLabel: "You — notification style",
    fact: "You want one daily digest at 8am PT; no per-event push notifications.",
    confidence: 96, lastTouchedHours: 4, lastTouchedLabel: "Verified 4h ago", verified: true,
    tags: ["notifications"],
    evidence: [
      { id: "e12a", when: "4h ago", source: "Settings", ref: "settings/notifications", snippet: "Per-event push disabled; daily digest enabled at 08:00 PT." },
      { id: "e12b", when: "2m ago", source: "Direct statement", ref: "thread/2026-03-15", snippet: "\"Stop pinging me — just send one digest in the morning.\"" },
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
      { id: "e15a", when: "4d ago", source: "Direct statement", ref: "thread/2026-05-07", snippet: "\"Drop me from FYI threads, just send the Friday roll-up.\"" },
    ],
  },
  {
    id: "m16", subject: "accounts", subjectLabel: "Yates Holdings — banking",
    fact: "Yates Holdings updated bank routing on May 10, 2026; old routing on file is stale.",
    confidence: 86, lastTouchedHours: 26, lastTouchedLabel: "Verified yesterday", verified: true,
    tags: ["alert", "ap"],
    evidence: [
      { id: "e16a", when: "yesterday", source: "IRS public filings", ref: "irs.gov/filings/yates", snippet: "W-9 amendment filed; new routing reported." },
      { id: "e16b", when: "yesterday", source: "Vendor email", ref: "ops@yatesholdings.com", snippet: "\"Please update payment details to attached.\"" },
    ],
  },
  {
    id: "m17", subject: "preferences", subjectLabel: "You — quiet hours",
    fact: "You don't want assistant actions executed between 11pm and 6am PT (drafts okay; sends not).",
    confidence: 93, lastTouchedHours: 24 * 5, lastTouchedLabel: "Verified 5d ago", verified: true,
    tags: ["policy"],
    evidence: [
      { id: "e17a", when: "5d ago", source: "Settings", ref: "settings/quiet-hours", snippet: "Quiet hours configured 23:00–06:00 PT, drafts only." },
      { id: "e17b", when: "2m ago", source: "Direct statement", ref: "thread/2026-03-18", snippet: "\"Hold sends overnight, drafts are fine.\"" },
    ],
  },
  {
    id: "m18", subject: "patterns", subjectLabel: "Calendar — 3pm context-switch slump",
    fact: "Between 3pm and 4pm PT you're 3× more likely to defer decisions to \"tomorrow\".",
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
      { id: "e19a", when: "8d ago", source: "Review log", ref: "reviews/priya", snippet: "11/12 last reviews landed Tue/Fri 09:00–10:30." },
      { id: "e19b", when: "1m ago", source: "Direct statement", ref: "thread/2026-04-10", snippet: "\"I batch reviews Tuesdays and Fridays.\"" },
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
