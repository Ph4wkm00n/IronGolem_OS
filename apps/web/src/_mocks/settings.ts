// route: /settings — typed mock data for the Settings page.
// Consumed via `api.v2.settings.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema once workspace + connector models stabilise.

export type ToneName = "safe" | "warning" | "blocked" | "recovered" | "quarantined" | "accent" | "neutral";
export type ConnectorState = "connected" | "needs-auth" | "disabled" | "deferred";
export type ConnectorScope = "scoped" | "broad" | "restricted";
export type DeploymentMode = "solo" | "household" | "team";
export type RequestStatus = "open" | "in-review" | "shipped";

export interface Connector {
  readonly id: string;
  readonly name: string;
  readonly glyph: string;
  readonly tint: ToneName;
  readonly state: ConnectorState;
  readonly scope: ConnectorScope;
  readonly lastSync: string;
  readonly can: readonly string[];
  readonly cannot: readonly string[];
  readonly note?: string;
}

export interface ModeCard {
  readonly id: DeploymentMode;
  readonly name: string;
  readonly tagline: string;
  readonly changes: {
    readonly dataLocation: string;
    readonly sharing: string;
    readonly security: string;
  };
  readonly bullets: readonly string[];
}

export interface RecipeRequest {
  readonly id: string;
  readonly title: string;
  readonly one: string;
  votes: number;
  mine: boolean;
  readonly status: RequestStatus;
}

export interface Session {
  readonly id: string;
  readonly device: string;
  readonly where: string;
  readonly when: string;
  readonly current: boolean;
}

export interface Operator {
  readonly name: string;
  readonly email: string;
  readonly role: string;
  readonly signIn: string;
  readonly avatar: string;
}

export interface Workspace {
  readonly name: string;
  readonly region: string;
  readonly plan: string;
  readonly createdOn: string;
}

export const mockOperator: Operator = {
  name: "Mira Okafor",
  email: "mira@okafor.studio",
  role: "Workspace owner",
  signIn: "Passkey · iCloud Keychain",
  avatar: "MO",
};

export const mockWorkspace: Workspace = {
  name: "Okafor Studio",
  region: "US-West (Oregon)",
  plan: "Solo · Beta",
  createdOn: "Aug 14, 2025",
};

export const mockSessions: readonly Session[] = [
  { id: "s1", device: "MacBook Pro · Safari 18.4", where: "Portland, OR", when: "now", current: true },
  { id: "s2", device: "iPhone 16 · IronGolem iOS", where: "Portland, OR", when: "12m ago", current: false },
  { id: "s3", device: "MacBook Pro · Chrome 132", where: "Portland, OR", when: "3h ago", current: false },
  { id: "s4", device: "iPad · Safari 18.3", where: "Cannon Beach, OR", when: "2d ago", current: false },
  { id: "s5", device: "MacBook Pro · Safari 18.4", where: "Bend, OR", when: "9d ago", current: false },
];

export const mockConnectors: readonly Connector[] = [
  { id: "email", name: "Email (IMAP)", glyph: "@", tint: "accent", state: "connected", scope: "scoped", lastSync: "2m ago",
    can: ["Read incoming mail", "Draft replies in your tone", "Apply labels and archive"],
    cannot: ["Send without your approval", "Read mail in 'private' label", "Delete mail permanently"] },
  { id: "telegram", name: "Telegram", glyph: "Tg", tint: "warning", state: "needs-auth", scope: "scoped", lastSync: "yesterday · session invalidated",
    can: ["Receive messages in 3 channels", "Post drafts for your review"],
    cannot: ["Add or remove channel members", "Forward outside the workspace"],
    note: "Upstream client 5.1.4 invalidated the bot session." },
  { id: "webhook", name: "Webhook receiver", glyph: "Wh", tint: "neutral", state: "disabled", scope: "broad", lastSync: "12d ago",
    can: ["Accept inbound JSON payloads from known senders"],
    cannot: ["Initiate outbound calls", "Bypass rate limits", "Persist payloads beyond 30 days"] },
  { id: "discord", name: "Discord", glyph: "Ds", tint: "quarantined", state: "deferred", scope: "scoped", lastSync: "—", can: [], cannot: [] },
  { id: "slack", name: "Slack", glyph: "Sk", tint: "quarantined", state: "deferred", scope: "scoped", lastSync: "—", can: [], cannot: [] },
  { id: "whatsapp", name: "WhatsApp", glyph: "Wa", tint: "quarantined", state: "deferred", scope: "scoped", lastSync: "—", can: [], cannot: [] },
  { id: "feishu", name: "Feishu", glyph: "Fs", tint: "quarantined", state: "deferred", scope: "scoped", lastSync: "—", can: [], cannot: [] },
  { id: "browser", name: "Browser", glyph: "Br", tint: "quarantined", state: "deferred", scope: "broad", lastSync: "—", can: [], cannot: [] },
  { id: "fs", name: "Filesystem", glyph: "Fs", tint: "quarantined", state: "deferred", scope: "restricted", lastSync: "—", can: [], cannot: [] },
];

export const mockModes: readonly ModeCard[] = [
  { id: "solo", name: "Solo", tagline: "SQLite, local to your device.",
    changes: { dataLocation: "Stays on this laptop in ~/Library/IronGolem.", sharing: "Only you. No external surface.", security: "Device-bound passkey, no inbound network." },
    bullets: ["~250 MB on disk", "Encrypted at rest", "Offline-capable"] },
  { id: "household", name: "Household", tagline: "Shared SQLite for up to 6 operators.",
    changes: { dataLocation: "On a designated 'host' device; replicated to housemates over local network.", sharing: "Selected housemates see shared inboxes and recipes; private items stay private.", security: "Per-operator passkeys, shared workspace token, household-only sync." },
    bullets: ["LAN sync only", "Per-room scope", "Recipe inheritance"] },
  { id: "team", name: "Team", tagline: "PostgreSQL, multi-tenant.",
    changes: { dataLocation: "Managed Postgres in your selected region.", sharing: "Org admin controls who sees what; role-based access.", security: "SSO + audit trail + per-tenant encryption keys." },
    bullets: ["Up to 200 operators", "SOC 2 ready", "External audit feed"] },
];

export const mockRecipeRequests: readonly RecipeRequest[] = [
  { id: "r1", title: "Renew domain registrations 30 days early", one: "Catch lapses before any registrar surprise.", votes: 41, mine: true, status: "in-review" },
  { id: "r2", title: "Summarize my standing weekly meetings", one: "One paragraph per meeting with action items.", votes: 36, mine: false, status: "open" },
  { id: "r3", title: "Reconcile bank export with invoices", one: "Match Plaid statements to outstanding invoices.", votes: 29, mine: false, status: "open" },
  { id: "r4", title: "Watch for SSL cert expiry on 4 domains", one: "Open a draft renewal 21 days before expiry.", votes: 22, mine: true, status: "shipped" },
  { id: "r5", title: "Triage Discord mentions while focused", one: "Hold non-urgent pings until focus mode ends.", votes: 17, mine: false, status: "open" },
];
