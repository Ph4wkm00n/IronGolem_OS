import React, { useState } from "react";
import type { DeploymentMode } from "@irongolem/schema";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

interface SettingsState {
  deploymentMode: DeploymentMode;
  expertiseLevel: "beginner" | "intermediate" | "advanced";
  notificationsEnabled: boolean;
  autoApproveLow: boolean;
  darkMode: boolean;
}

const INITIAL_SETTINGS: SettingsState = {
  deploymentMode: "solo",
  expertiseLevel: "beginner",
  notificationsEnabled: true,
  autoApproveLow: false,
  darkMode: false,
};

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export function Settings() {
  const [settings, setSettings] = useState<SettingsState>(INITIAL_SETTINGS);
  const [saved, setSaved] = useState(false);

  function update<K extends keyof SettingsState>(key: K, value: SettingsState[K]) {
    setSettings((prev) => ({ ...prev, [key]: value }));
    setSaved(false);
  }

  function handleSave() {
    // In production this would persist via the API
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  return (
    <div className="page-container max-w-2xl">
      <h1 className="page-title mb-6">Settings</h1>

      <div className="space-y-8">
        {/* Deployment mode */}
        <SettingsSection title="Deployment mode" description="How IronGolem is set up on your device.">
          <fieldset className="space-y-2">
            <legend className="sr-only">Deployment mode</legend>
            {(
              [
                { value: "solo" as const, label: "Solo", desc: "Just you, running locally." },
                { value: "household" as const, label: "Household", desc: "Shared with family or a small group." },
                { value: "team" as const, label: "Team", desc: "Multi-user with workspace isolation." },
              ] as const
            ).map((opt) => (
              <label
                key={opt.value}
                className={`flex items-start gap-3 rounded-lg border p-3 cursor-pointer transition-colors ${
                  settings.deploymentMode === opt.value
                    ? "border-indigo-300 bg-indigo-50/50"
                    : "border-neutral-200 bg-white hover:border-neutral-300"
                }`}
              >
                <input
                  type="radio"
                  name="deploymentMode"
                  value={opt.value}
                  checked={settings.deploymentMode === opt.value}
                  onChange={() => update("deploymentMode", opt.value)}
                  className="mt-0.5 h-4 w-4 text-indigo-600 border-neutral-300 focus:ring-indigo-500"
                />
                <div>
                  <span className="text-sm font-medium text-neutral-900">{opt.label}</span>
                  <p className="text-xs text-neutral-500">{opt.desc}</p>
                </div>
              </label>
            ))}
          </fieldset>
        </SettingsSection>

        {/* Expertise level */}
        <SettingsSection
          title="Expertise level"
          description="Controls the amount of detail shown in the interface."
        >
          <fieldset className="space-y-2">
            <legend className="sr-only">Expertise level</legend>
            {(
              [
                { value: "beginner" as const, label: "Beginner", desc: "Plain language, no technical jargon." },
                { value: "intermediate" as const, label: "Intermediate", desc: "Some technical details visible." },
                { value: "advanced" as const, label: "Advanced", desc: "Full technical detail and raw event data." },
              ] as const
            ).map((opt) => (
              <label
                key={opt.value}
                className={`flex items-start gap-3 rounded-lg border p-3 cursor-pointer transition-colors ${
                  settings.expertiseLevel === opt.value
                    ? "border-indigo-300 bg-indigo-50/50"
                    : "border-neutral-200 bg-white hover:border-neutral-300"
                }`}
              >
                <input
                  type="radio"
                  name="expertiseLevel"
                  value={opt.value}
                  checked={settings.expertiseLevel === opt.value}
                  onChange={() => update("expertiseLevel", opt.value)}
                  className="mt-0.5 h-4 w-4 text-indigo-600 border-neutral-300 focus:ring-indigo-500"
                />
                <div>
                  <span className="text-sm font-medium text-neutral-900">{opt.label}</span>
                  <p className="text-xs text-neutral-500">{opt.desc}</p>
                </div>
              </label>
            ))}
          </fieldset>
        </SettingsSection>

        {/* Toggles */}
        <SettingsSection title="Preferences" description="General behavior settings.">
          <div className="space-y-4">
            <ToggleRow
              label="Notifications"
              description="Receive alerts when something needs your attention."
              checked={settings.notificationsEnabled}
              onChange={(v) => update("notificationsEnabled", v)}
            />
            <ToggleRow
              label="Auto-approve low-risk actions"
              description="Automatically approve actions marked as low risk."
              checked={settings.autoApproveLow}
              onChange={(v) => update("autoApproveLow", v)}
            />
            <ToggleRow
              label="Dark mode"
              description="Use a darker color scheme (coming soon)."
              checked={settings.darkMode}
              onChange={(v) => update("darkMode", v)}
              disabled
            />
          </div>
        </SettingsSection>

        {/* Save */}
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={handleSave}
            className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-500 transition-colors"
          >
            Save settings
          </button>
          {saved && (
            <span className="text-sm text-emerald-600 font-medium">Settings saved.</span>
          )}
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function SettingsSection({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <section className="card-padded">
      <h2 className="text-base font-semibold text-neutral-900">{title}</h2>
      <p className="text-sm text-neutral-500 mt-0.5 mb-4">{description}</p>
      {children}
    </section>
  );
}

function ToggleRow({
  label,
  description,
  checked,
  onChange,
  disabled = false,
}: {
  label: string;
  description: string;
  checked: boolean;
  onChange: (value: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className={`flex items-center justify-between gap-4 ${disabled ? "opacity-50" : ""}`}>
      <div>
        <p className="text-sm font-medium text-neutral-900">{label}</p>
        <p className="text-xs text-neutral-500">{description}</p>
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ${
          checked ? "bg-indigo-600" : "bg-neutral-200"
        } ${disabled ? "cursor-not-allowed" : ""}`}
      >
        <span
          className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ${
            checked ? "translate-x-5" : "translate-x-0"
          }`}
        />
      </button>
    </div>
  );
}
