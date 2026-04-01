import React, { useState } from "react";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

type SettingsSection = "profile" | "workspace" | "connectors" | "advanced";

interface SectionMeta {
  readonly id: SettingsSection;
  readonly label: string;
  readonly description: string;
}

const SECTIONS: readonly SectionMeta[] = [
  {
    id: "profile",
    label: "Profile",
    description: "Your personal details and display preferences.",
  },
  {
    id: "workspace",
    label: "Workspace",
    description: "Settings that apply to everyone in this workspace.",
  },
  {
    id: "connectors",
    label: "Connectors",
    description: "Manage integrations with email, calendar, messaging, and other services.",
  },
  {
    id: "advanced",
    label: "Advanced",
    description: "Developer settings, API keys, and system configuration.",
  },
];

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Settings() {
  const [activeSection, setActiveSection] = useState<SettingsSection>("profile");

  return (
    <div className="page-container">
      <h1 className="page-title mb-6">Settings</h1>

      <div className="flex flex-col lg:flex-row gap-8">
        {/* Section navigation */}
        <nav className="lg:w-56 flex-shrink-0" aria-label="Settings sections">
          <ul className="space-y-1" role="list">
            {SECTIONS.map((section) => (
              <li key={section.id}>
                <button
                  type="button"
                  onClick={() => setActiveSection(section.id)}
                  className={`w-full text-left rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                    activeSection === section.id
                      ? "bg-indigo-50 text-indigo-700"
                      : "text-neutral-600 hover:bg-neutral-50 hover:text-neutral-900"
                  }`}
                >
                  {section.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>

        {/* Section content */}
        <div className="flex-1 min-w-0">
          {activeSection === "profile" && <ProfileSection />}
          {activeSection === "workspace" && <WorkspaceSection />}
          {activeSection === "connectors" && <ConnectorsSection />}
          {activeSection === "advanced" && <AdvancedSection />}
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Profile section                                                    */
/* ------------------------------------------------------------------ */

function ProfileSection() {
  return (
    <section aria-label="Profile settings">
      <SectionHeader
        title="Profile"
        description="Your personal details and display preferences."
      />

      <div className="card-padded space-y-6">
        <FieldGroup label="Display name">
          <input
            type="text"
            defaultValue="User"
            className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 placeholder-neutral-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            placeholder="Your name"
          />
        </FieldGroup>

        <FieldGroup label="Email address">
          <input
            type="email"
            defaultValue="user@example.com"
            className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 placeholder-neutral-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            placeholder="you@example.com"
          />
        </FieldGroup>

        <FieldGroup label="Language">
          <select
            className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            defaultValue="en"
          >
            <option value="en">English</option>
            <option value="es">Spanish</option>
            <option value="fr">French</option>
            <option value="de">German</option>
            <option value="ja">Japanese</option>
          </select>
        </FieldGroup>

        <FieldGroup label="Experience level">
          <select
            className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            defaultValue="beginner"
          >
            <option value="beginner">
              Beginner - Show me friendly explanations
            </option>
            <option value="intermediate">
              Intermediate - Balance detail and simplicity
            </option>
            <option value="advanced">Advanced - Show me everything</option>
          </select>
          <p className="mt-1 text-xs text-neutral-500">
            This controls how much technical detail you see throughout the app.
          </p>
        </FieldGroup>

        <div className="flex justify-end pt-2">
          <button
            type="button"
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors"
          >
            Save changes
          </button>
        </div>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/*  Workspace section                                                  */
/* ------------------------------------------------------------------ */

function WorkspaceSection() {
  return (
    <section aria-label="Workspace settings">
      <SectionHeader
        title="Workspace"
        description="Settings that apply to everyone in this workspace."
      />

      <div className="card-padded space-y-6">
        <FieldGroup label="Workspace name">
          <input
            type="text"
            defaultValue="My Workspace"
            className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 placeholder-neutral-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            placeholder="Workspace name"
          />
        </FieldGroup>

        <FieldGroup label="Deployment mode">
          <select
            className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            defaultValue="solo"
          >
            <option value="solo">Solo - Just me (local storage)</option>
            <option value="household">Household - Shared with family</option>
            <option value="team">Team - Multi-user with accounts</option>
          </select>
          <p className="mt-1 text-xs text-neutral-500">
            Determines how data is stored and whether others can join.
          </p>
        </FieldGroup>

        <FieldGroup label="Default approval mode">
          <select
            className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            defaultValue="ask"
          >
            <option value="ask">Ask before acting (recommended)</option>
            <option value="shadow">Run in shadow mode (preview only)</option>
            <option value="auto">Auto-approve low-risk actions</option>
          </select>
        </FieldGroup>

        <div className="flex justify-end pt-2">
          <button
            type="button"
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors"
          >
            Save changes
          </button>
        </div>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/*  Connectors section                                                 */
/* ------------------------------------------------------------------ */

interface ConnectorInfo {
  readonly id: string;
  readonly name: string;
  readonly description: string;
  readonly connected: boolean;
}

const CONNECTORS: readonly ConnectorInfo[] = [
  {
    id: "email",
    name: "Email",
    description: "Connect your email account to enable inbox management recipes.",
    connected: true,
  },
  {
    id: "calendar",
    name: "Calendar",
    description: "Sync your calendar for scheduling and conflict detection.",
    connected: true,
  },
  {
    id: "slack",
    name: "Slack",
    description: "Receive notifications and interact with your assistant in Slack.",
    connected: false,
  },
  {
    id: "telegram",
    name: "Telegram",
    description: "Get updates and approvals through Telegram messages.",
    connected: false,
  },
];

function ConnectorsSection() {
  return (
    <section aria-label="Connector settings">
      <SectionHeader
        title="Connectors"
        description="Manage integrations with email, calendar, messaging, and other services."
      />

      <div className="space-y-3">
        {CONNECTORS.map((connector) => (
          <div
            key={connector.id}
            className="card-padded flex items-center justify-between"
          >
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <p className="text-sm font-medium text-neutral-900">
                  {connector.name}
                </p>
                {connector.connected && (
                  <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-emerald-100 text-emerald-800">
                    Connected
                  </span>
                )}
              </div>
              <p className="text-xs text-neutral-500 mt-0.5">
                {connector.description}
              </p>
            </div>

            <button
              type="button"
              className={`flex-shrink-0 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                connector.connected
                  ? "border border-neutral-200 text-neutral-700 hover:bg-neutral-50"
                  : "bg-indigo-600 text-white hover:bg-indigo-500"
              }`}
            >
              {connector.connected ? "Manage" : "Connect"}
            </button>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/*  Advanced section                                                   */
/* ------------------------------------------------------------------ */

function AdvancedSection() {
  return (
    <section aria-label="Advanced settings">
      <SectionHeader
        title="Advanced"
        description="Developer settings, API keys, and system configuration."
      />

      <div className="space-y-6">
        {/* API access */}
        <div className="card-padded space-y-4">
          <h3 className="text-sm font-semibold text-neutral-900">API access</h3>

          <FieldGroup label="Gateway URL">
            <input
              type="text"
              defaultValue="http://localhost:8080"
              className="w-full rounded-lg border border-neutral-300 bg-white px-3 py-2 text-sm text-neutral-900 font-mono focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              placeholder="http://localhost:8080"
            />
          </FieldGroup>

          <FieldGroup label="API key">
            <div className="flex gap-2">
              <input
                type="password"
                defaultValue="sk-demo-key-placeholder"
                className="flex-1 rounded-lg border border-neutral-300 bg-white px-3 py-2 text-xs text-neutral-900 font-mono focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                readOnly
              />
              <button
                type="button"
                className="rounded-lg border border-neutral-200 px-3 py-1.5 text-sm font-medium text-neutral-700 hover:bg-neutral-50 transition-colors"
              >
                Regenerate
              </button>
            </div>
          </FieldGroup>
        </div>

        {/* Data management */}
        <div className="card-padded space-y-4">
          <h3 className="text-sm font-semibold text-neutral-900">
            Data management
          </h3>

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-neutral-700">Export all data</p>
              <p className="text-xs text-neutral-500">
                Download a copy of your settings, memory, and event history.
              </p>
            </div>
            <button
              type="button"
              className="rounded-lg border border-neutral-200 px-3 py-1.5 text-sm font-medium text-neutral-700 hover:bg-neutral-50 transition-colors"
            >
              Export
            </button>
          </div>

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-red-700">Clear all memory</p>
              <p className="text-xs text-neutral-500">
                Permanently erase everything your assistant has learned. This cannot be undone.
              </p>
            </div>
            <button
              type="button"
              className="rounded-lg border border-red-200 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-50 transition-colors"
            >
              Clear memory
            </button>
          </div>
        </div>

        <div className="flex justify-end pt-2">
          <button
            type="button"
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors"
          >
            Save changes
          </button>
        </div>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ */
/*  Shared components                                                  */
/* ------------------------------------------------------------------ */

function SectionHeader({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="mb-6">
      <h2 className="section-title">{title}</h2>
      <p className="mt-1 text-sm text-neutral-500">{description}</p>
    </div>
  );
}

function FieldGroup({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-neutral-700 mb-1.5">
        {label}
      </label>
      {children}
    </div>
  );
}
