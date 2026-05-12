// route: /health — typed mock data for the Health page.
// Consumed via `api.v2.health.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema once component state model stabilises.

export type CanonicalState = "healthy" | "recovering" | "attention" | "paused" | "quarantined";
export type ComponentCategory = "core" | "connector" | "team";

export interface HealthComponent {
  readonly id: string;
  readonly name: string;
  readonly category: ComponentCategory;
  readonly state: CanonicalState;
  readonly lastHeartbeat: string;
  readonly uptimeDays: number;
  readonly activity: string;
  readonly detail?: string;
  readonly etaMinutes?: number;
}

export interface HealStory {
  readonly checked: string;
  readonly changed: string;
  readonly outcome: string;
  readonly followup: string | null;
}

export interface HealEvent {
  readonly id: string;
  readonly when: string;
  readonly whenIso: string;
  readonly component: string;
  readonly componentId: string;
  readonly what: string;
  readonly story: HealStory;
  readonly durationSec: number;
}

export interface PredictiveWarning {
  readonly id: string;
  readonly component: string;
  readonly componentId: string;
  readonly signal: string;
  readonly why: string;
  readonly errorBudgetUsedPct: number;
  readonly windowDays: number;
  readonly trend: readonly number[];
  readonly suggestedAction: "Pause it" | "Keep paused" | "Show graph";
}

export const mockComponents: readonly HealthComponent[] = [
  { id: "c-gateway", name: "Gateway", category: "core", state: "healthy", lastHeartbeat: "8s ago", uptimeDays: 47, activity: "Routing 142 req/min, p95 41ms" },
  { id: "c-runtime", name: "Runtime daemon", category: "core", state: "healthy", lastHeartbeat: "10s ago", uptimeDays: 47, activity: "12 jobs running, queue depth 3" },
  { id: "c-sandbox", name: "Sandbox", category: "core", state: "healthy", lastHeartbeat: "11s ago", uptimeDays: 21, activity: "4 sandboxes warm" },
  { id: "c-memory", name: "Memory store", category: "core", state: "recovering", lastHeartbeat: "9s ago", uptimeDays: 0, activity: "Re-embedding 2,140 docs — 8 min remaining", detail: "A scheduled embedding-model upgrade is in progress. Reads continue from the warm cache; writes are queued and will drain when re-embed finishes. No action needed.", etaMinutes: 8 },
  { id: "c-events", name: "Event store", category: "core", state: "healthy", lastHeartbeat: "5s ago", uptimeDays: 62, activity: "Append rate 38/s, retention 90d" },
  { id: "c-telegram", name: "Telegram connector", category: "connector", state: "quarantined", lastHeartbeat: "1h ago", uptimeDays: 0, activity: "Isolated — auth token rotated upstream", detail: "Telegram desktop client 5.1.4 invalidated the long-lived bot session on May 9 14:02 PT. Outbound messages are queued (4 pending). The connector has been quarantined so it can't retry against a dead session. Re-authorize when you're ready." },
  { id: "c-email", name: "Email connector", category: "connector", state: "healthy", lastHeartbeat: "14s ago", uptimeDays: 47, activity: "118 messages last hour" },
  { id: "c-webhook", name: "Webhook connector", category: "connector", state: "healthy", lastHeartbeat: "6s ago", uptimeDays: 19, activity: "11 receivers active, 0 retries" },
  { id: "c-cal", name: "Calendar connector", category: "connector", state: "healthy", lastHeartbeat: "22s ago", uptimeDays: 47, activity: "Synced 3 calendars" },
  { id: "c-inbox", name: "Inbox team", category: "team", state: "healthy", lastHeartbeat: "11s ago", uptimeDays: 12, activity: "Triaged 26 messages this hour" },
  { id: "c-cal-team", name: "Calendar team", category: "team", state: "healthy", lastHeartbeat: "16s ago", uptimeDays: 12, activity: "3 events drafted, 2 awaiting your review" },
  { id: "c-research", name: "Research team", category: "team", state: "healthy", lastHeartbeat: "12s ago", uptimeDays: 8, activity: "Watching 47 sources" },
  { id: "c-ops", name: "Operations team", category: "team", state: "attention", lastHeartbeat: "20s ago", uptimeDays: 0, activity: "Awaiting your decision on PO-26-118", detail: "Operations needs your sign-off on a $640 standing-order PO that drifted above the $50 auto-approve threshold. The team paused itself to wait for you — nothing else is blocked." },
  { id: "c-draft", name: "Drafting team", category: "team", state: "healthy", lastHeartbeat: "9s ago", uptimeDays: 12, activity: "5 drafts queued for review" },
];

export const mockHealEvents: readonly HealEvent[] = [
  {
    id: "h01", when: "23m ago", whenIso: "11:32 PT", component: "Webhook connector", componentId: "c-webhook",
    what: "Recovered from a slow downstream receiver",
    story: {
      checked: "p95 latency on receiver `slack/ops-bot` exceeded 4s for 3 consecutive checks.",
      changed: "Switched the receiver to the back-off queue and reduced concurrency from 4 → 1.",
      outcome: "p95 returned to 380ms within 90 seconds; queue drained without retries.",
      followup: null,
    },
    durationSec: 96,
  },
  {
    id: "h02", when: "2h ago", whenIso: "09:55 PT", component: "Email connector", componentId: "c-email",
    what: "Reconnected after a transient IMAP IDLE drop",
    story: {
      checked: "IMAP IDLE socket closed without notice; no new messages for 4 minutes.",
      changed: "Restarted the IDLE session against the secondary endpoint.",
      outcome: "Stream resumed; backfill caught 3 messages that arrived during the gap.",
      followup: "Same drop pattern seen 3× this month — rule will be reviewed Friday.",
    },
    durationSec: 240,
  },
  {
    id: "h03", when: "5h ago", whenIso: "06:48 PT", component: "Memory store", componentId: "c-memory",
    what: "Dropped a stale embedding-cache shard",
    story: {
      checked: "Cache hit-rate on shard 7 had fallen to 4% over 24h.",
      changed: "Evicted shard 7 and rebuilt it from cold storage.",
      outcome: "Hit-rate climbed to 81% within 20 minutes; no read failures during rebuild.",
      followup: null,
    },
    durationSec: 1200,
  },
  {
    id: "h04", when: "9h ago", whenIso: "02:32 PT", component: "Event store", componentId: "c-events",
    what: "Rode out an S3 throttling window",
    story: {
      checked: "Append latency p95 jumped to 1.8s between 02:14 and 02:32 PT.",
      changed: "Held writes in the local buffer and replayed when throttling cleared.",
      outcome: "All 412 appends delivered, in-order, with no operator action needed.",
      followup: "Cloud-provider status confirmed regional throttling.",
    },
    durationSec: 1080,
  },
  {
    id: "h05", when: "13h ago", whenIso: "22:14 PT (May 11)", component: "Runtime daemon", componentId: "c-runtime",
    what: "Restarted a worker stuck on a long-running job",
    story: {
      checked: "Worker `runtime-w3` had not heartbeated for 7 minutes.",
      changed: "Killed worker, redrained its queue to siblings, restarted the worker.",
      outcome: "Queue cleared in 4 minutes; no jobs lost.",
      followup: null,
    },
    durationSec: 240,
  },
  {
    id: "h06", when: "yesterday", whenIso: "May 11 16:01 PT", component: "Gateway", componentId: "c-gateway",
    what: "Failed over to the backup auth provider",
    story: {
      checked: "Primary auth provider returned 502 on 8/10 probes in 60s.",
      changed: "Switched to backup provider and held the route there for 15 minutes.",
      outcome: "All sessions stayed valid; primary recovered after 11 minutes and we re-pinned.",
      followup: "Rule will be reviewed Friday — failover triggered twice this week.",
    },
    durationSec: 660,
  },
  {
    id: "h07", when: "yesterday", whenIso: "May 11 11:09 PT", component: "Drafting team", componentId: "c-draft",
    what: "Throttled itself when model latency spiked",
    story: {
      checked: "Draft generation p95 rose from 1.4s to 9.2s for 4 minutes.",
      changed: "Dropped concurrency 8 → 2 and switched to the cheaper drafting model.",
      outcome: "Latency returned to 1.5s; quality regression on 0 of 7 drafts checked.",
      followup: null,
    },
    durationSec: 360,
  },
  {
    id: "h08", when: "2d ago", whenIso: "May 10 04:17 PT", component: "Sandbox", componentId: "c-sandbox",
    what: "Rotated a sandbox with a leaking file descriptor",
    story: {
      checked: "Sandbox `sbx-118` held 1,920 open fds, growing 3/min.",
      changed: "Drained sessions, recycled the sandbox, restarted with a clean baseline.",
      outcome: "fd count back to 184 (nominal). No user-facing impact.",
      followup: null,
    },
    durationSec: 540,
  },
];

export const mockPredictive: readonly PredictiveWarning[] = [
  {
    id: "p01",
    component: "Email connector",
    componentId: "c-email",
    signal: "IMAP IDLE drops recurring on a weekly cadence",
    why: "Three IDLE drops this month, each followed by a successful self-heal. Pattern matches a known network-path quirk; reliability is trending below the 99.9% objective.",
    errorBudgetUsedPct: 72,
    windowDays: 30,
    trend: [.94,.95,.94,.93,.93,.92,.93,.93,.91,.91,.92,.9,.9,.9,.89,.89,.88,.88,.87,.87,.87,.86,.85,.85,.84,.83,.83,.82,.81,.8],
    suggestedAction: "Pause it",
  },
  {
    id: "p02",
    component: "Telegram connector",
    componentId: "c-telegram",
    signal: "Already quarantined — recommend keeping it paused until re-auth",
    why: "Long-lived bot session was invalidated by an upstream client change. The connector quarantined itself; predictive monitor recommends staying paused to avoid retry churn.",
    errorBudgetUsedPct: 100,
    windowDays: 7,
    trend: [.9,.92,.91,.92,.93,.92,.93,.94,.94,.93,.9,.86,.78,.7,.6,.5,.42,.32,.22,.15,.1,.05,.02,0,0,0,0,0,0,0],
    suggestedAction: "Keep paused",
  },
  {
    id: "p03",
    component: "Operations team",
    componentId: "c-ops",
    signal: "Decision queue is growing faster than it's draining",
    why: "Six new approvals landed in the last 24h; you reviewed two. If the trend holds for another week, Operations will start deferring drafts.",
    errorBudgetUsedPct: 41,
    windowDays: 14,
    trend: [.99,.99,.98,.98,.98,.97,.97,.96,.96,.95,.95,.94,.93,.92,.91,.9,.88,.86,.84,.82,.8,.78,.76,.74,.72,.7,.68,.66,.64,.62],
    suggestedAction: "Show graph",
  },
];
