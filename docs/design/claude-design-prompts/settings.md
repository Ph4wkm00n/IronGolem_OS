# Settings — Claude Design Brief

**Route:** `/settings` · **Job:** Account, connector credentials, deployment mode (Solo / Household / Team), and workspace preferences.

Paste the system prompt from `docs/design/claude-design-guide.md` section 2 FIRST, then paste the brief below.

## Route brief (paste into Claude Design)

```text
Design the Settings route for IronGolem OS — `/settings`. This is where
the operator configures their account, connects external services, picks
a deployment mode, and tunes workspace preferences. Settings touch
sensitive configuration — every change needs a clear consequence
statement.

Primary surface:
- Two-column layout on desktop: sticky left rail with section anchors
  (Account / Connectors / Deployment / Notifications / Recipes / Advanced),
  main content on the right. Single-column on mobile.
- Account section:
  - Operator name, email, role, sign-in method
  - Workspace name + region (read-only here, change requires admin)
  - "Sign in history" — last 5 sessions with device + location
  - Action: "Sign out everywhere"
- Connectors section:
  - Each connector card: name + icon, status badge (Connected / Needs
    auth / Disabled), permission scope, last sync, SafetyCard preview
    (what this connector CAN and CANNOT do)
  - Buttons: "Reconnect" / "Disconnect" / "Customize permissions"
  - List the existing 3 real connectors (Email, Telegram, Webhook) +
    placeholder for 6 deferred (Discord, Slack, WhatsApp, Feishu, Browser,
    Filesystem) marked as "Available in v0.2"
- Deployment section:
  - Three large radio cards: Solo (SQLite, local), Household (shared
    SQLite), Team (PostgreSQL multi-tenant)
  - Currently active one is highlighted
  - Each card lists what changes if you switch (data location, sharing
    surface, security model)
  - "Switch deployment mode" is a guarded action — confirmation dialog
    with a 7-day undo window
- Notifications section:
  - Channels (email / web push / mobile / nothing)
  - Quiet hours toggle
  - Per-event-type defaults (Awaiting approval / Blocked / Healed / etc.)
- Recipes section:
  - "Recipe requests" — a small form to suggest a new recipe
  - List of requested recipes with vote counts
- Advanced section (progressive disclosure, collapsed by default):
  - Telemetry opt-in
  - Debug mode toggle
  - Export workspace data
  - Delete workspace (guarded)

Mandatory patterns:
- Visible Trust — every setting change shows its consequence BEFORE save.
- Reversibility — every destructive action (sign out, disconnect, delete)
  has a confirmation + undo window where possible.
- Progressive disclosure — Advanced is collapsed by default; non-technical
  users never need to expand it.
- Plain language — "Sign in method" not "Authentication provider";
  "Quiet hours" not "Notification schedule policy".

Tech reminders:
- React 19, TypeScript strict, Tailwind utility classes only.
- Form labels: use `text-xs font-medium uppercase tracking-wide text-neutral-500`
  (the label-token scale).
- Save behavior: prefer immediate (optimistic) for benign settings; opt-in
  confirmation for sensitive ones. Always toast the result.
- Mock the user, workspace, 9 connectors, 3 deployment modes, 7
  notification channels inline.

Output: ONE TSX file with a named export `export function Settings()`.
```

## Mock data shape

```ts
interface OperatorAccount {
  readonly name: string;
  readonly email: string;
  readonly role: "operator" | "admin" | "viewer";
  readonly signInMethod: "magic-link" | "sso" | "passkey";
  readonly sessions: ReadonlyArray<{
    readonly device: string;
    readonly location: string;
    readonly lastSeen: string;
  }>;
}

interface Connector {
  readonly id: string;
  readonly name: string;
  readonly category: "messaging" | "calendar" | "drive" | "browser" | "filesystem" | "voice";
  status: "connected" | "needs-auth" | "disabled" | "unavailable";
  readonly availableInV01: boolean;     // false for the 6 deferred connectors
  readonly permissionScope: "scoped" | "broad" | "restricted";
  readonly lastSync: string | null;
  readonly safety: {
    readonly can: readonly string[];
    readonly cannot: readonly string[];
  };
}

type DeploymentMode = "solo" | "household" | "team";

interface DeploymentOption {
  readonly mode: DeploymentMode;
  readonly title: string;
  readonly summary: string;
  readonly dataLocation: string;
  readonly sharingSurface: string;
  readonly securityModel: string;
  active: boolean;
}
```

## Components to reuse (TODO substitution markers)

- `SafetyCard` — connector permission summaries
- `RiskBadge` — small "Sensitive" pill on settings that mutate shared state
- `WorkspaceTopbar` — page chrome

## Page patterns

- **Progressive disclosure** — Advanced section is collapsed by default.
- **Reversibility** — every destructive action shows undo windows.
- **Plain language** — no engineering labels.

## Route-specific anti-patterns

| Don't | Do |
|---|---|
| Hide deployment-mode consequences | Each card spells out what changes (data location, sharing, security) |
| Use engineering labels ("Auth provider") | Use outcome labels ("Sign in method") |
| Default-expand Advanced | Default-collapse; only the user who needs it expands |
| Save destructive actions without confirmation | Confirmation + undo window for sign-out, disconnect, delete |
| Show 9 connectors as if all are usable in v0.1 | Mark the 6 deferred as "Available in v0.2" and grey them |
