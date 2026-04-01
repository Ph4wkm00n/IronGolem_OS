import React from "react";

export default function Settings() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-semibold text-neutral-900">Settings</h1>
        <p className="text-neutral-500 mt-1">Configure your workspace and preferences</p>
      </div>

      {/* Profile */}
      <section className="bg-white rounded-xl border border-neutral-200 p-6">
        <h2 className="text-lg font-semibold text-neutral-900 mb-4">Profile</h2>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-neutral-700 mb-1">Display name</label>
            <input type="text" className="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm" placeholder="Your name" />
          </div>
          <div>
            <label className="block text-sm font-medium text-neutral-700 mb-1">Email</label>
            <input type="email" className="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm" placeholder="you@example.com" />
          </div>
        </div>
      </section>

      {/* Workspace */}
      <section className="bg-white rounded-xl border border-neutral-200 p-6">
        <h2 className="text-lg font-semibold text-neutral-900 mb-4">Workspace</h2>
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-neutral-700 mb-1">Workspace name</label>
            <input type="text" className="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm" defaultValue="My Workspace" />
          </div>
          <div>
            <label className="block text-sm font-medium text-neutral-700 mb-1">Deployment mode</label>
            <select className="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm">
              <option value="solo">Solo (personal use)</option>
              <option value="household">Household (shared family)</option>
              <option value="team">Team (organization)</option>
            </select>
          </div>
        </div>
      </section>

      {/* Connectors */}
      <section className="bg-white rounded-xl border border-neutral-200 p-6">
        <h2 className="text-lg font-semibold text-neutral-900 mb-4">Connectors</h2>
        <p className="text-sm text-neutral-500 mb-4">Manage connections to external services</p>
        <div className="space-y-3">
          {["Email", "Calendar", "Telegram", "Filesystem"].map((name) => (
            <div key={name} className="flex items-center justify-between py-2 border-b border-neutral-100 last:border-0">
              <span className="text-sm font-medium text-neutral-700">{name}</span>
              <button className="text-xs bg-neutral-100 text-neutral-600 px-3 py-1 rounded-lg hover:bg-neutral-200">
                Configure
              </button>
            </div>
          ))}
        </div>
      </section>

      {/* Advanced */}
      <section className="bg-white rounded-xl border border-neutral-200 p-6">
        <h2 className="text-lg font-semibold text-neutral-900 mb-4">Advanced</h2>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-neutral-700">Advanced mode</p>
              <p className="text-xs text-neutral-500">Show traces, cache metrics, and provider routing</p>
            </div>
            <button className="relative w-10 h-6 bg-neutral-200 rounded-full">
              <span className="absolute left-1 top-1 w-4 h-4 bg-white rounded-full shadow transition-transform" />
            </button>
          </div>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-neutral-700">Telemetry export</p>
              <p className="text-xs text-neutral-500">Send traces via OTLP (team mode)</p>
            </div>
            <button className="relative w-10 h-6 bg-neutral-200 rounded-full">
              <span className="absolute left-1 top-1 w-4 h-4 bg-white rounded-full shadow transition-transform" />
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
