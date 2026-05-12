// route: / — typed mock data for the Workspace Dashboard.
// Consumed via `api.v2.home.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema (HeartbeatStatus, Event, Squad models).

export type EventStatus =
  | "taken"
  | "proposed"
  | "blocked"
  | "healed"
  | "quarantined"
  | "research-update"
  | "squad-handoff";

export type ToneName =
  | "safe"
  | "warning"
  | "blocked"
  | "recovered"
  | "quarantined"
  | "accent"
  | "neutral";

export interface EventItem {
  readonly id: string;
  status: EventStatus;
  title: string;
  readonly teamId: string;
  readonly permission: string;
  readonly permissionScope: "scoped" | "broad" | "restricted";
  readonly risk: "low" | "medium" | "high";
  minutesAgo: number;
  cause?: string;
  why: string;
  readonly target?: string;
  readonly approvals?: number;
}

export interface Team {
  readonly id: string;
  readonly name: string;
  readonly color: ToneName;
  readonly members: number;
  readonly description: string;
}

export interface ResearchFinding {
  readonly id: string;
  readonly title: string;
  readonly source: string;
  readonly confidence: number;
  readonly freshness: string;
  readonly summary: string;
}

export interface HeartbeatState {
  readonly status: "healthy" | "degraded" | "down";
  readonly systemsGreen: number;
  readonly systemsTotal: number;
  readonly oneDegraded: { readonly name: string; readonly reason: string };
}

export interface WorkspaceInfo {
  readonly name: string;
  readonly initials: string;
  readonly region: string;
  readonly uptimeHours: number;
  readonly uptimeStreak: string;
  readonly lastSync: string;
}

export interface SafetyLayer {
  readonly id: number;
  readonly name: string;
  readonly state: "ok" | "watching" | "alerting";
  readonly note: string;
}

export interface SafetyShape {
  readonly posture: string;
  readonly layers: readonly SafetyLayer[];
  readonly can: readonly string[];
  readonly cannot: readonly string[];
  readonly needsApproval: readonly string[];
  readonly stopsIf: readonly string[];
}

export const mockWorkspace: WorkspaceInfo = {
  name: "Eastside Production",
  initials: "EP",
  region: "us-east-1",
  uptimeHours: 412,
  uptimeStreak: "17 days",
  lastSync: "37 seconds ago",
};

export const mockHeartbeat: HeartbeatState = {
  status: "healthy",
  systemsGreen: 18,
  systemsTotal: 19,
  oneDegraded: {
    name: "Research index",
    reason: "Re-embedding 2,140 documents — 8 min remaining.",
  },
};

export const mockTeams: readonly Team[] = [
  {
    id: "inbox-triage",
    name: "Inbox triage",
    color: "accent",
    members: 3,
    description: "Sorts mail, drafts replies, escalates the rest.",
  },
  {
    id: "calendar",
    name: "Calendar",
    color: "recovered",
    members: 2,
    description: "Schedules, reschedules, and protects focus blocks.",
  },
  {
    id: "purchasing",
    name: "Purchasing",
    color: "warning",
    members: 4,
    description: "Drafts orders against the approved vendor list.",
  },
  {
    id: "research",
    name: "Research",
    color: "quarantined",
    members: 5,
    description: "Pulls papers, summarizes, tracks freshness.",
  },
  {
    id: "ops",
    name: "Operations",
    color: "neutral",
    members: 3,
    description: "Monitors systems and self-heals routine breakage.",
  },
  {
    id: "drafts",
    name: "Drafting",
    color: "safe",
    members: 2,
    description: "Writes first-pass docs from briefs.",
  },
];

export const mockTrustHistory: Readonly<Record<string, readonly number[]>> = {
  "inbox-triage": [9, 9, 9, 9, 9, 8, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9],
  calendar: [9, 9, 9, 9, 9, 9, 9, 9, 8, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9],
  purchasing: [9, 9, 9, 8, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 7, 9, 9, 9, 9, 9, 9, 9, 9, 9],
  research: [9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 8, 9, 9, 9, 9, 9, 9],
  ops: [9, 9, 9, 9, 9, 9, 9, 9, 9, 5, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9],
  drafts: [9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9],
};

export const mockSafety: SafetyShape = {
  posture: "active",
  layers: [
    { id: 1, name: "Identity", state: "ok", note: "Operator signed in 14m ago" },
    { id: 2, name: "Workspace", state: "ok", note: "Production rules in effect" },
    { id: 3, name: "Team", state: "ok", note: "6 teams within charter" },
    { id: 4, name: "Action", state: "watching", note: "1 high-risk action awaiting approval" },
    { id: 5, name: "Outcome", state: "ok", note: "All outcomes verified" },
  ],
  can: [
    "Read mail, calendar, and shared drives",
    "Draft replies (held for review)",
    "Reschedule internal meetings",
    "File expense receipts under $200",
  ],
  cannot: [
    "Send external email without approval",
    "Spend over $200 without approval",
    "Modify finance records",
    "Touch anything labeled Personal",
  ],
  needsApproval: [
    "Any external send",
    "Any purchase",
    "Any calendar change involving a customer",
  ],
  stopsIf: [
    "Two failures in a row on the same action",
    "Cause text would be missing or unclear",
    "Heartbeat goes red",
  ],
};

export const mockResearchFindings: readonly ResearchFinding[] = [
  {
    id: "rf-1",
    title: "Q3 carbon credit pricing shifted 11% upward",
    source: "Bloomberg Carbon Index",
    confidence: 0.91,
    freshness: "2 hours ago",
    summary:
      "Spot prices closed at $89.40, breaking the three-month range. Likely impact on the Riverbend offset purchase queued for next week.",
  },
  {
    id: "rf-2",
    title: "Outlook API rate limit lowered for legacy tenants",
    source: "Microsoft 365 release notes",
    confidence: 0.84,
    freshness: "yesterday",
    summary:
      "Calls per minute drop from 240 to 180. Inbox triage already adjusted polling cadence; no operator action needed.",
  },
  {
    id: "rf-3",
    title: "Three vendors flagged in updated supplier risk feed",
    source: "Dun & Bradstreet weekly",
    confidence: 0.77,
    freshness: "5 hours ago",
    summary:
      "Northgate Logistics moved to medium risk. Purchasing paused drafts against this vendor until you confirm.",
  },
];

export const mockInitialEvents: readonly EventItem[] = [
  { id: "e01", status: "proposed", title: "Send response to Marcus Yi about the Riverbend purchase order", teamId: "inbox-triage", permission: "send external email", permissionScope: "broad", risk: "medium", minutesAgo: 2, why: "Marcus has waited 14 hours and the draft passed the tone and accuracy check.", target: "marcus@riverbend.co", approvals: 47 },
  { id: "e02", status: "taken", title: "Moved 9 newsletters to the digest folder", teamId: "inbox-triage", permission: "organize inbox", permissionScope: "scoped", risk: "low", minutesAgo: 4, why: "Sender domains match your approved newsletter list.", approvals: 1840 },
  { id: "e03", status: "blocked", title: "Wire $4,200 to a new vendor on the Riverbend invoice", teamId: "purchasing", permission: "initiate transfer", permissionScope: "restricted", risk: "high", minutesAgo: 6, cause: "Wires to new vendors always need your approval.", why: "Safety rule 'Money — new payee' caught it before the action ran.", target: "Riverbend Logistics LLC", approvals: 0 },
  { id: "e04", status: "healed", title: "Reconnected the research index after a 90-second outage", teamId: "ops", permission: "restart service", permissionScope: "scoped", risk: "low", minutesAgo: 8, why: "Auto-heal rule matched the failure signature in 12 seconds.", target: "research-index/primary", approvals: 124 },
  { id: "e05", status: "proposed", title: "Move tomorrow's 11am with Sandra to Thursday at 2pm", teamId: "calendar", permission: "modify customer meeting", permissionScope: "broad", risk: "medium", minutesAgo: 11, why: "Sandra emailed asking to move and both calendars are clear Thursday afternoon.", target: "Sandra Lopez · Riverbend", approvals: 23 },
  { id: "e06", status: "research-update", title: "Carbon credit pricing rose 11% — affects queued purchase", teamId: "research", permission: "publish finding", permissionScope: "scoped", risk: "low", minutesAgo: 17, why: "Bloomberg index closed outside the 3-month range; queued purchase flagged." },
  { id: "e07", status: "taken", title: "Filed receipt from Stagecoach Coffee ($14.20) under Travel", teamId: "drafts", permission: "file expense", permissionScope: "scoped", risk: "low", minutesAgo: 22, why: "Receipt matched a calendar event with the 'travel' tag.", approvals: 312 },
  { id: "e08", status: "quarantined", title: "Draft reply to legal@oldcompany.com held for review", teamId: "inbox-triage", permission: "send external email", permissionScope: "broad", risk: "high", minutesAgo: 28, cause: "Tone-check flagged the draft as defensive; held until you read it.", why: "Two of three reviewers wanted softer language. Draft is in your inbox.", approvals: 0 },
  { id: "e09", status: "squad-handoff", title: "Inbox triage handed the Riverbend thread to Purchasing", teamId: "inbox-triage", permission: "route work", permissionScope: "scoped", risk: "low", minutesAgo: 33, why: "Thread crossed the 'purchase order' classifier with 94% confidence." },
  { id: "e10", status: "taken", title: "Booked focus block on Friday morning", teamId: "calendar", permission: "modify own calendar", permissionScope: "scoped", risk: "low", minutesAgo: 41, why: "You wanted a 2-hour block once a week and Friday was free.", approvals: 58 },
  { id: "e11", status: "blocked", title: "Forward customer NDA to external counsel", teamId: "drafts", permission: "share confidential doc", permissionScope: "restricted", risk: "high", minutesAgo: 52, cause: "External counsel isn't on this workspace's allow-list.", why: "Safety rule 'Confidential — external' requires you to add the recipient first.", target: "ben@hartlaw.com", approvals: 0 },
  { id: "e12", status: "healed", title: "Retried the failed weekly digest send", teamId: "ops", permission: "retry job", permissionScope: "scoped", risk: "low", minutesAgo: 64, why: "First attempt timed out; second attempt succeeded in 4 seconds.", target: "digest@eastside", approvals: 89 },
  { id: "e13", status: "proposed", title: "Order 3 boxes of letterhead from the approved vendor", teamId: "purchasing", permission: "submit PO", permissionScope: "scoped", risk: "medium", minutesAgo: 78, why: "Stock is below the 2-week threshold and the vendor is on the standing list.", target: "Stationer's Direct", approvals: 12 },
  { id: "e14", status: "research-update", title: "Microsoft lowered Outlook polling limits for legacy tenants", teamId: "research", permission: "publish finding", permissionScope: "scoped", risk: "low", minutesAgo: 92, why: "Release notes detected; Inbox triage adjusted polling on its own." },
  { id: "e15", status: "taken", title: "Summarized the 47-page vendor risk PDF into a 1-page brief", teamId: "drafts", permission: "read shared drive", permissionScope: "scoped", risk: "low", minutesAgo: 110, why: "You asked for one of these before each procurement meeting.", approvals: 38 },
  { id: "e16", status: "quarantined", title: "Unrecognized purchasing pattern: $812 to Yates Holdings", teamId: "purchasing", permission: "submit PO", permissionScope: "broad", risk: "high", minutesAgo: 134, cause: "Yates Holdings has never appeared in this workspace before.", why: "Safety rule 'Money — new payee' isolated the draft. Nothing left this workspace.", target: "Yates Holdings", approvals: 0 },
  { id: "e17", status: "taken", title: "Sent the standing Monday status to the Riverbend team", teamId: "drafts", permission: "send external email", permissionScope: "broad", risk: "medium", minutesAgo: 161, why: "Template, recipients, and send window all matched the standing rule.", target: "team@riverbend.co", approvals: 41 },
  { id: "e18", status: "healed", title: "Re-authenticated the calendar connection", teamId: "ops", permission: "refresh credentials", permissionScope: "scoped", risk: "low", minutesAgo: 184, why: "Token expired; refresh succeeded automatically.", approvals: 412 },
  { id: "e19", status: "squad-handoff", title: "Operations passed an HR question to your queue", teamId: "ops", permission: "route work", permissionScope: "scoped", risk: "low", minutesAgo: 210, why: "HR topics aren't in any team's charter; routed to you." },
  { id: "e20", status: "taken", title: "Replied to 4 routine vendor confirmations", teamId: "inbox-triage", permission: "send external email", permissionScope: "scoped", risk: "low", minutesAgo: 232, why: "All four matched a template you approved 47 times before.", target: "4 vendors", approvals: 47 },
];
