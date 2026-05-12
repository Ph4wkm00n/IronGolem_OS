// route: /settings
// purpose: Account, Connectors, Deployment, Notifications, Recipe requests,
// Advanced. Replaces the stub when VITE_ENABLE_V2_UI=true. Ported from
// Claude Design's Settings.tsx — source at apps/web/src/_design-inbox/settings/.
//
// Integration notes:
// - Shell chrome from `pages/v2/_shared/WorkspaceTopbar`.
// - Mock data inline; swap for `useSettingsQuery()` once lib/api.ts ships.
// - Dynamic `bg-${tone}` patterns replaced with the static TONE classmap.
// - Drops the preview shim (`window.Settings = ...`).

import React, { useEffect, useMemo, useRef, useState } from "react";

import { WorkspaceTopbar } from "./_shared/WorkspaceTopbar";

import {
  api,
  type Connector,
  type ConnectorScope,
  type DeploymentMode,
  type ModeCard,
  type RecipeRequest,
} from "../../lib/api";

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

type ToneName = "safe" | "warning" | "blocked" | "recovered" | "quarantined" | "accent" | "neutral";
type NotifChannel = "web-push" | "email" | "mobile-push" | "sms" | "slack-dm" | "digest" | "nothing";
type EventType = "awaiting-approval" | "blocked" | "healed" | "completed" | "failed";

interface NotifState {
  quietHours: boolean;
  quietFrom: string;
  quietTo: string;
  event: Record<EventType, NotifChannel>;
}

// ─────────────────────────────────────────────────────────────────────────────
// Tone classmap
// ─────────────────────────────────────────────────────────────────────────────

interface ToneClasses {
  readonly bg: string;
  readonly text: string;
  readonly border: string;
  readonly bgSolid: string;
  readonly textSolid: string;
  readonly bgSolidHover: string;
}

const TONE: Readonly<Record<ToneName, ToneClasses>> = {
  safe: { bg: "bg-safe", text: "text-safe", border: "border-safe", bgSolid: "bg-safe-solid", textSolid: "text-safe-solid", bgSolidHover: "hover:bg-safe-solid-hover" },
  warning: { bg: "bg-warning", text: "text-warning", border: "border-warning", bgSolid: "bg-warning-solid", textSolid: "text-warning-solid", bgSolidHover: "hover:bg-warning-solid-hover" },
  blocked: { bg: "bg-blocked", text: "text-blocked", border: "border-blocked", bgSolid: "bg-blocked-solid", textSolid: "text-blocked-solid", bgSolidHover: "hover:bg-blocked-solid-hover" },
  recovered: { bg: "bg-recovered", text: "text-recovered", border: "border-recovered", bgSolid: "bg-recovered-solid", textSolid: "text-recovered-solid", bgSolidHover: "hover:bg-recovered-solid-hover" },
  quarantined: { bg: "bg-quarantined", text: "text-quarantined", border: "border-quarantined", bgSolid: "bg-quarantined-solid", textSolid: "text-quarantined-solid", bgSolidHover: "hover:bg-quarantined-solid-hover" },
  accent: { bg: "bg-accent", text: "text-accent", border: "border-accent", bgSolid: "bg-accent-solid", textSolid: "text-accent-solid", bgSolidHover: "hover:bg-accent-solid-hover" },
  neutral: { bg: "bg-neutral", text: "text-neutral-700", border: "border-neutral-200", bgSolid: "bg-neutral-solid", textSolid: "text-neutral-solid", bgSolidHover: "hover:bg-neutral-solid-hover" },
};

const cx = (...c: ReadonlyArray<string | false | null | undefined>): string =>
  c.filter(Boolean).join(" ");

const SCOPE_TONE: Readonly<Record<ConnectorScope, ToneName>> = {
  scoped: "safe",
  broad: "warning",
  restricted: "quarantined",
};

// ─────────────────────────────────────────────────────────────────────────────
// Inline icons
// ─────────────────────────────────────────────────────────────────────────────

interface IconProps {
  readonly size?: number;
  readonly className?: string;
}

const Svg = ({
  d, viewBox = "0 0 24 24", size = 16, className = "",
}: {
  readonly d: React.ReactNode;
  readonly viewBox?: string;
  readonly size?: number;
  readonly className?: string;
}) => (
  <svg viewBox={viewBox} width={size} height={size} fill="none" stroke="currentColor" strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" className={className} aria-hidden>
    {d}
  </svg>
);

const ICON = {
  User: (p: IconProps) => <Svg {...p} d={<><circle cx={12} cy={8} r={4} /><path d="M4 21c1.5-4 4.5-6 8-6s6.5 2 8 6" /></>} />,
  Plug: (p: IconProps) => <Svg {...p} d={<><path d="M9 7v4" /><path d="M15 7v4" /><path d="M7 11h10" /><path d="M12 17v4" /><path d="M9 13a3 3 0 0 0 6 0" /></>} />,
  Server: (p: IconProps) => <Svg {...p} d={<><rect x={4} y={4} width={16} height={6} rx={1.5} /><rect x={4} y={14} width={16} height={6} rx={1.5} /><path d="M8 7h.01M8 17h.01" /></>} />,
  Bell: (p: IconProps) => <Svg {...p} d={<><path d="M6 16V11a6 6 0 0 1 12 0v5l1.5 2H4.5L6 16Z" /><path d="M10 20a2 2 0 0 0 4 0" /></>} />,
  Sparkles: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1" /></>} />,
  Wrench: (p: IconProps) => <Svg {...p} d={<path d="M14.7 6.3a4 4 0 0 1 5 5L17 14l3 3-3 3-3-3-2.7 2.7a4 4 0 0 1-5-5L9 12 6 9l3-3 3 3 2.7-2.7Z" />} />,
  Check: (p: IconProps) => <Svg {...p} d={<path d="m5 12 5 5L20 7" />} />,
  X: (p: IconProps) => <Svg {...p} d={<><path d="M6 6l12 12" /><path d="M18 6 6 18" /></>} />,
  ChevronDown: (p: IconProps) => <Svg {...p} d={<path d="m6 9 6 6 6-6" />} />,
  ChevronUp: (p: IconProps) => <Svg {...p} d={<path d="m6 15 6-6 6 6" />} />,
  ArrowRight: (p: IconProps) => <Svg {...p} d={<><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /></>} />,
  Info: (p: IconProps) => <Svg {...p} d={<><circle cx={12} cy={12} r={9} /><path d="M12 11v5" /><circle cx={12} cy={8} r={0.5} fill="currentColor" stroke="none" /></>} />,
  AlertTriangle: (p: IconProps) => <Svg {...p} d={<><path d="M12 4 2.5 20h19L12 4Z" /><path d="M12 10v4" /><circle cx={12} cy={17.5} r={0.5} fill="currentColor" stroke="none" /></>} />,
  Lock: (p: IconProps) => <Svg {...p} d={<><rect x={5} y={11} width={14} height={9} rx={2} /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></>} />,
  Download: (p: IconProps) => <Svg {...p} d={<><path d="M12 3v12" /><path d="m7 11 5 5 5-5" /><path d="M4 19h16" /></>} />,
  Trash: (p: IconProps) => <Svg {...p} d={<><path d="M4 7h16" /><path d="M9 7V4h6v3" /><path d="M6 7v13a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1V7" /></>} />,
  Undo: (p: IconProps) => <Svg {...p} d={<><path d="M9 14H4v-5" /><path d="M4 14a8 8 0 1 1 2.5 5.7" /></>} />,
  Refresh: (p: IconProps) => <Svg {...p} d={<><path d="M3 12a9 9 0 0 1 15.6-6.2L21 8" /><path d="M21 3v5h-5" /><path d="M21 12a9 9 0 0 1-15.6 6.2L3 16" /><path d="M3 21v-5h5" /></>} />,
} as const;

type IconName = keyof typeof ICON;

// ─────────────────────────────────────────────────────────────────────────────
// Mock data — sourced from `api.v2.settings.getMock()`. Mock vs real toggles
// via `VITE_API_MODE`; pages remain sync until real endpoints land (F6).
// ─────────────────────────────────────────────────────────────────────────────

const SETTINGS_MOCK = api.v2.settings.getMock();
const OPERATOR = SETTINGS_MOCK.operator;
const WORKSPACE = SETTINGS_MOCK.workspace;
const SESSIONS = SETTINGS_MOCK.sessions;
const CONNECTORS = SETTINGS_MOCK.connectors;
const MODES = SETTINGS_MOCK.modes;

const CHANNEL_LABELS: Readonly<Record<NotifChannel, string>> = {
  "web-push": "Web push",
  email: "Email",
  "mobile-push": "Mobile push",
  sms: "SMS",
  "slack-dm": "Slack DM",
  digest: "Daily digest",
  nothing: "Don't notify",
};

const EVENT_LABELS: Readonly<Record<EventType, string>> = {
  "awaiting-approval": "Awaiting your approval",
  blocked: "Blocked by a policy",
  healed: "Recovered on its own",
  completed: "Completed cleanly",
  failed: "Failed and needs you",
};

const EVENT_HINTS: Readonly<Record<EventType, string>> = {
  "awaiting-approval": "Something is paused waiting on you.",
  blocked: "A safety rule held something back.",
  healed: "A component fixed itself — fyi only.",
  completed: "A recipe finished without issue.",
  failed: "An action failed and could not auto-recover.",
};

const RECIPE_REQUESTS_INITIAL = SETTINGS_MOCK.recipeRequests;

const SECTIONS: ReadonlyArray<{ readonly id: string; readonly label: string; readonly icon: IconName }> = [
  { id: "account", label: "Account", icon: "User" },
  { id: "connectors", label: "Connectors", icon: "Plug" },
  { id: "deployment", label: "Deployment", icon: "Server" },
  { id: "notifications", label: "Notifications", icon: "Bell" },
  { id: "recipes", label: "Recipes", icon: "Sparkles" },
  { id: "advanced", label: "Advanced", icon: "Wrench" },
];

// ─────────────────────────────────────────────────────────────────────────────
// Atoms
// ─────────────────────────────────────────────────────────────────────────────

interface ChipProps {
  readonly tone: ToneName;
  readonly dot?: boolean;
  readonly children: React.ReactNode;
}
function Chip({ tone, dot = false, children }: ChipProps) {
  const t = TONE[tone];
  return (
    <span className={cx("inline-flex items-center gap-1.5 rounded-full border text-[10.5px] font-medium px-1.5 py-0.5", t.bg, t.text, t.border)}>
      {dot && <span className={cx("h-1.5 w-1.5 rounded-full", t.bgSolid)} />}
      {children}
    </span>
  );
}

function Label({ children, htmlFor }: { readonly children: React.ReactNode; readonly htmlFor?: string }) {
  return (
    <label htmlFor={htmlFor} className="text-xs font-medium uppercase tracking-wide text-neutral-500">
      {children}
    </label>
  );
}

function Consequence({ children }: { readonly children: React.ReactNode }) {
  return (
    <div className="inline-flex items-start gap-1.5 rounded-md bg-accent border border-accent px-2 py-1 text-[11.5px] text-accent leading-relaxed">
      <ICON.Info size={11} className="mt-[2px] shrink-0" />
      <span><span className="font-semibold mr-1">If you save:</span>{children}</span>
    </div>
  );
}

function Row({ label, children }: { readonly label: string; readonly children: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-2.5">
      <span className="text-[12.5px] text-neutral-500">{label}</span>
      <span className="text-[13px] text-neutral-900 font-medium text-right truncate">{children}</span>
    </div>
  );
}

interface SectionCardProps {
  readonly id: string;
  readonly title: string;
  readonly subtitle?: string;
  readonly right?: React.ReactNode;
  readonly children: React.ReactNode;
}
function SectionCard({ id, title, subtitle, right, children }: SectionCardProps) {
  return (
    <section id={id} className="card overflow-hidden scroll-mt-20">
      <header className="px-5 py-4 border-b border-neutral-100 flex items-end justify-between gap-3 flex-wrap">
        <div>
          <h2 className="section-title">{title}</h2>
          {subtitle && <p className="text-[12.5px] text-neutral-500 mt-1 max-w-2xl">{subtitle}</p>}
        </div>
        {right}
      </header>
      <div className="p-5">{children}</div>
    </section>
  );
}

interface SwitchProps {
  readonly on: boolean;
  readonly onChange: (v: boolean) => void;
  readonly label?: string;
}
function Switch({ on, onChange, label }: SwitchProps) {
  return (
    <button
      type="button"
      onClick={() => onChange(!on)}
      aria-pressed={on}
      aria-label={label}
      className={cx("inline-flex items-center h-5 w-9 rounded-full p-0.5 transition-colors", on ? "bg-accent-solid" : "bg-neutral-200")}
    >
      <span className={cx("h-4 w-4 rounded-full bg-white shadow-sm transition-transform", on ? "translate-x-4" : "translate-x-0")} />
    </button>
  );
}

function ScopeChip({ scope }: { readonly scope: ConnectorScope }) {
  return <Chip tone={SCOPE_TONE[scope]}>{scope}</Chip>;
}

// ─────────────────────────────────────────────────────────────────────────────
// Sidebar
// ─────────────────────────────────────────────────────────────────────────────

interface SidebarProps {
  readonly active: string;
  readonly onJump: (id: string) => void;
}
function Sidebar({ active, onJump }: SidebarProps) {
  return (
    <aside className="hidden md:block">
      <div className="sticky top-6">
        <div className="mb-3 px-2">
          <div className="text-[10.5px] font-medium uppercase tracking-wide text-neutral-500">Settings</div>
          <div className="text-[13px] text-neutral-900 font-semibold mt-0.5">{WORKSPACE.name}</div>
          <div className="text-[11px] text-neutral-500 font-mono">{WORKSPACE.region}</div>
        </div>
        <nav className="flex flex-col gap-0.5">
          {SECTIONS.map((s) => {
            const Icon = ICON[s.icon];
            return (
              <button
                key={s.id}
                type="button"
                onClick={() => onJump(s.id)}
                data-active={active === s.id}
                className="ig-nav text-left w-full"
              >
                <Icon size={14} className="ig-nav-icon" />
                <span>{s.label}</span>
              </button>
            );
          })}
        </nav>
        <div className="mt-6 px-2 pt-3 border-t border-neutral-100">
          <p className="text-[11px] text-neutral-500 leading-relaxed">
            <span className="inline-flex items-center gap-1 text-accent font-medium">
              <ICON.Undo size={10} /> 30-day undo
            </span>{" "}
            on every change. Pause is preferred over delete.
          </p>
        </div>
      </div>
    </aside>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Account
// ─────────────────────────────────────────────────────────────────────────────

function AccountSection({ onSignOutEverywhere }: { readonly onSignOutEverywhere: () => void }) {
  return (
    <SectionCard id="account" title="Account" subtitle="Who you are to IronGolem. Workspace metadata is read-only; an admin must change it.">
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_1fr] gap-5">
        <div>
          <Label>Operator</Label>
          <div className="mt-2 flex items-center gap-3">
            <div className="h-10 w-10 rounded-full bg-accent text-accent inline-flex items-center justify-center text-[14px] font-semibold tracking-tight">
              {OPERATOR.avatar}
            </div>
            <div className="min-w-0">
              <div className="text-[14px] font-semibold text-neutral-900 truncate">{OPERATOR.name}</div>
              <div className="text-[12px] text-neutral-500 truncate">{OPERATOR.email}</div>
            </div>
          </div>
          <div className="mt-3 divide-y divide-neutral-100 border-y border-neutral-100">
            <Row label="Role">{OPERATOR.role}</Row>
            <Row label="Sign-in method">
              <span className="inline-flex items-center gap-1.5">
                <ICON.Lock size={11} className="text-safe-solid" />
                {OPERATOR.signIn}
              </span>
            </Row>
          </div>
        </div>

        <div>
          <Label>Workspace</Label>
          <div className="mt-3 divide-y divide-neutral-100 border-y border-neutral-100">
            <Row label="Workspace name">{WORKSPACE.name}</Row>
            <Row label="Region (data residency)">{WORKSPACE.region}</Row>
            <Row label="Plan">{WORKSPACE.plan}</Row>
            <Row label="Created on">{WORKSPACE.createdOn}</Row>
          </div>
          <p className="mt-2 text-[11px] text-neutral-500">
            Workspace name and region are read-only. Ask an admin to change them.
          </p>
        </div>
      </div>

      <div className="mt-6">
        <Label>Sign-in history · last 5 sessions</Label>
        <ul className="mt-2 divide-y divide-neutral-100 border border-neutral-200 rounded-xl overflow-hidden bg-white">
          {SESSIONS.map((s) => (
            <li key={s.id} className="px-3 py-2.5 flex items-center gap-3">
              <div className="h-7 w-7 rounded-md bg-neutral inline-flex items-center justify-center text-neutral">
                <ICON.User size={12} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-[13px] font-medium text-neutral-900 truncate">{s.device}</div>
                <div className="text-[11px] text-neutral-500 font-mono truncate">{s.where} · {s.when}</div>
              </div>
              {s.current ? (
                <Chip tone="safe" dot>This device</Chip>
              ) : (
                <button type="button" className="text-[12px] text-neutral-500 hover:text-blocked font-medium px-2 py-1 rounded-md hover:bg-blocked">
                  Revoke
                </button>
              )}
            </li>
          ))}
        </ul>
        <div className="mt-3">
          <Consequence>You'll be signed out on all 4 other devices. You can sign in again with your passkey — there's no undo for this one because the sign-out is the safety action.</Consequence>
        </div>
        <div className="mt-3 flex justify-end">
          <button type="button" onClick={onSignOutEverywhere} className="inline-flex items-center gap-1.5 rounded-md border border-blocked bg-white hover:bg-blocked text-blocked px-3 py-1.5 text-[12.5px] font-medium">
            Sign out everywhere
          </button>
        </div>
      </div>
    </SectionCard>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Connectors
// ─────────────────────────────────────────────────────────────────────────────

function ConnectorCard({ c, onToast }: { readonly c: Connector; readonly onToast: (s: string) => void }) {
  const stateTone: ToneName =
    c.state === "connected" ? "safe" :
    c.state === "needs-auth" ? "warning" :
    c.state === "disabled" ? "neutral" : "quarantined";
  const stateLabel =
    c.state === "connected" ? "Connected" :
    c.state === "needs-auth" ? "Needs auth" :
    c.state === "disabled" ? "Disabled" : "Available in v0.2";

  const isDeferred = c.state === "deferred";
  const tintTone = TONE[c.tint];

  return (
    <article className={cx("card overflow-hidden flex flex-col", isDeferred && "opacity-75")}>
      <div className="px-4 py-3 flex flex-col gap-2">
        <div className="flex items-start gap-3">
          <div className={cx("h-10 w-10 rounded-lg inline-flex items-center justify-center text-[13px] font-semibold tracking-tight border", tintTone.bg, tintTone.text, tintTone.border)}>
            {c.glyph}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h3 className="text-[14px] font-semibold text-neutral-900 truncate">{c.name}</h3>
              <Chip tone={stateTone} dot={c.state === "connected"}>{stateLabel}</Chip>
            </div>
            <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-neutral-500 font-mono">
              <ScopeChip scope={c.scope} />
              <span>· last sync {c.lastSync}</span>
            </div>
          </div>
        </div>

        {c.note && (
          <p className="rounded bg-warning text-warning border border-warning px-2 py-1 text-[11.5px] leading-snug">
            {c.note}
          </p>
        )}

        {!isDeferred ? (
          <div className="mt-1 rounded-lg border border-neutral-100 bg-neutral-50/60 p-2.5">
            <div className="text-[10.5px] font-medium uppercase tracking-wide text-safe mb-1">Can</div>
            <ul className="space-y-0.5">
              {c.can.map((x) => (
                <li key={x} className="flex items-start gap-1.5 text-[12px] text-neutral-700 leading-snug">
                  <ICON.Check size={11} className="mt-[3px] text-safe-solid shrink-0" />{x}
                </li>
              ))}
            </ul>
            <div className="mt-2 text-[10.5px] font-medium uppercase tracking-wide text-blocked mb-1">Cannot</div>
            <ul className="space-y-0.5">
              {c.cannot.map((x) => (
                <li key={x} className="flex items-start gap-1.5 text-[12px] text-neutral-700 leading-snug">
                  <ICON.X size={11} className="mt-[3px] text-blocked-solid shrink-0" />{x}
                </li>
              ))}
            </ul>
          </div>
        ) : (
          <p className="text-[12px] text-neutral-500 leading-relaxed">
            Coming in IronGolem v0.2. We'll publish a SafetyCard for this connector before it ships, with everything it can and can't do.
          </p>
        )}
      </div>

      <footer className="mt-auto border-t border-neutral-100 px-3 py-2 flex items-center justify-between gap-1 bg-neutral-50/60">
        {isDeferred ? (
          <>
            <span className="text-[11.5px] text-neutral-500">Deferred to v0.2</span>
            <button type="button" disabled className="rounded-md px-2 py-1 text-[12px] font-medium text-neutral-400 cursor-not-allowed">
              Notify me
            </button>
          </>
        ) : (
          <>
            <button type="button" onClick={() => onToast(`Reconnecting ${c.name}…`)} className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
              <ICON.Refresh size={11} /> Reconnect
            </button>
            <button type="button" onClick={() => onToast(`Disconnected ${c.name} · undo within 30 days`)} className="rounded-md px-2 py-1 text-[12px] font-medium text-neutral-700 hover:bg-white">
              Disconnect
            </button>
            <button type="button" onClick={() => onToast(`Opened permission editor for ${c.name}`)} className="rounded-md px-2 py-1 text-[12px] font-medium text-accent hover:bg-white">
              Customize permissions
            </button>
          </>
        )}
      </footer>
    </article>
  );
}

function ConnectorsSection({ onToast }: { readonly onToast: (s: string) => void }) {
  const real = CONNECTORS.filter((c) => c.state !== "deferred");
  const deferred = CONNECTORS.filter((c) => c.state === "deferred");
  return (
    <SectionCard
      id="connectors"
      title="Connectors"
      subtitle="External services IronGolem can read from and write to. Every connector ships with a SafetyCard — what it can and can't do — before it's allowed near your data."
      right={<Chip tone="neutral">3 active · 6 in v0.2</Chip>}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        {real.map((c) => <ConnectorCard key={c.id} c={c} onToast={onToast} />)}
      </div>

      <div className="mt-6">
        <div className="flex items-center justify-between mb-2">
          <Label>Available in v0.2</Label>
          <span className="text-[11px] text-neutral-500 font-mono">{deferred.length} connectors</span>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-2">
          {deferred.map((c) => {
            const tint = TONE[c.tint];
            return (
              <div key={c.id} className="card px-3 py-2.5 flex items-center gap-2">
                <div className={cx("h-7 w-7 rounded-md inline-flex items-center justify-center text-[11px] font-semibold border", tint.bg, tint.text, tint.border)}>{c.glyph}</div>
                <div className="min-w-0">
                  <div className="text-[12.5px] font-medium text-neutral-900 truncate">{c.name}</div>
                  <div className="text-[10.5px] text-neutral-500 font-mono">v0.2</div>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </SectionCard>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Deployment
// ─────────────────────────────────────────────────────────────────────────────

interface ModeOptionProps {
  readonly m: ModeCard;
  readonly active: boolean;
  readonly onSelect: () => void;
}
function ModeOption({ m, active, onSelect }: ModeOptionProps) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cx(
        "text-left card p-4 transition-colors flex flex-col h-full relative",
        active ? "border-accent bg-accent ring-accent" : "hover:bg-neutral-50",
      )}
    >
      <div className="flex items-center justify-between gap-2 mb-1">
        <div className="text-[14px] font-semibold tracking-tight text-neutral-900">{m.name}</div>
        {active ? <Chip tone="accent" dot>Active</Chip> : <span className="h-4 w-4 rounded-full border border-neutral-300" />}
      </div>
      <p className="text-[12px] text-neutral-600 leading-snug">{m.tagline}</p>

      <div className="mt-3 space-y-1.5">
        <div>
          <div className="text-[10px] font-medium uppercase tracking-wide text-neutral-500">Data lives</div>
          <div className="text-[12px] text-neutral-800 leading-snug">{m.changes.dataLocation}</div>
        </div>
        <div>
          <div className="text-[10px] font-medium uppercase tracking-wide text-neutral-500">Sharing surface</div>
          <div className="text-[12px] text-neutral-800 leading-snug">{m.changes.sharing}</div>
        </div>
        <div>
          <div className="text-[10px] font-medium uppercase tracking-wide text-neutral-500">Security model</div>
          <div className="text-[12px] text-neutral-800 leading-snug">{m.changes.security}</div>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap gap-1">
        {m.bullets.map((b) => (
          <span key={b} className="text-[10.5px] font-mono rounded bg-neutral-100 text-neutral-600 px-1.5 py-0.5">{b}</span>
        ))}
      </div>
    </button>
  );
}

interface DeploymentSectionProps {
  readonly active: DeploymentMode;
  readonly onRequestSwitch: (target: DeploymentMode) => void;
}
function DeploymentSection({ active, onRequestSwitch }: DeploymentSectionProps) {
  const [pending, setPending] = useState<DeploymentMode | null>(null);
  const target = pending && pending !== active ? MODES.find((m) => m.id === pending) ?? null : null;
  const current = MODES.find((m) => m.id === active);

  return (
    <SectionCard
      id="deployment"
      title="Deployment mode"
      subtitle="Where your IronGolem data lives and who can reach it. Switching is reversible inside 7 days; after that the migration is one-way."
    >
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
        {MODES.map((m) => (
          <ModeOption key={m.id} m={m} active={m.id === active} onSelect={() => setPending(m.id)} />
        ))}
      </div>

      {target && current && (
        <div className="mt-4 rounded-xl border border-warning bg-warning/50 px-4 py-3">
          <div className="flex items-start gap-2">
            <ICON.AlertTriangle size={14} className="mt-0.5 text-warning shrink-0" />
            <div className="flex-1 min-w-0">
              <div className="text-[13px] font-semibold text-neutral-900">
                Switch from <span className="font-mono">{current.name}</span> → <span className="font-mono">{target.name}</span>?
              </div>
              <p className="mt-1 text-[12px] text-neutral-700 leading-relaxed">
                IronGolem will migrate your workspace data from <span className="font-medium">{current.changes.dataLocation.toLowerCase()}</span>{" "}
                to <span className="font-medium">{target.changes.dataLocation.toLowerCase()}</span>{" "}
                Recipes, audit logs and memory move with it. You can revert within 7 days; after that the old store is wiped.
              </p>
              <ul className="mt-2 grid grid-cols-1 sm:grid-cols-3 gap-1.5 text-[11.5px]">
                <li className="rounded bg-white border border-warning px-2 py-1"><span className="text-neutral-500">Sharing → </span><span className="text-neutral-800">{target.changes.sharing}</span></li>
                <li className="rounded bg-white border border-warning px-2 py-1"><span className="text-neutral-500">Security → </span><span className="text-neutral-800">{target.changes.security}</span></li>
                <li className="rounded bg-white border border-warning px-2 py-1"><span className="text-neutral-500">Undo → </span><span className="text-neutral-800">7-day reversible window</span></li>
              </ul>
              <div className="mt-3 flex items-center justify-end gap-2">
                <button type="button" onClick={() => setPending(null)} className="rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
                  Cancel
                </button>
                <button type="button" onClick={() => { onRequestSwitch(target.id); setPending(null); }} className="rounded-md bg-warning-solid hover:bg-warning-solid-hover text-white px-3 py-1.5 text-[12.5px] font-medium inline-flex items-center gap-1.5">
                  <ICON.Check size={12} />
                  Switch to {target.name}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </SectionCard>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Notifications
// ─────────────────────────────────────────────────────────────────────────────

interface NotificationsSectionProps {
  readonly state: NotifState;
  readonly onChange: (s: NotifState) => void;
  readonly onToast: (s: string) => void;
}
function NotificationsSection({ state, onChange, onToast }: NotificationsSectionProps) {
  const updateEvent = (e: EventType, ch: NotifChannel) => {
    onChange({ ...state, event: { ...state.event, [e]: ch } });
    onToast(`${EVENT_LABELS[e]} → ${CHANNEL_LABELS[ch]}`);
  };

  return (
    <SectionCard id="notifications" title="Notifications" subtitle="Pick a channel per event type. Quiet hours apply on top of every channel.">
      <div className="flex items-center justify-between gap-3 flex-wrap rounded-xl border border-neutral-200 bg-neutral-50/60 px-4 py-3">
        <div className="min-w-0">
          <div className="text-[13.5px] font-semibold text-neutral-900">Quiet hours</div>
          <p className="text-[12px] text-neutral-600 leading-snug mt-0.5">
            We hold push and SMS during this window. Drafts continue; the daily digest captures what you missed.
          </p>
        </div>
        <div className="flex items-center gap-3">
          {state.quietHours && (
            <div className="flex items-center gap-1 text-[12px] text-neutral-700 font-mono">
              <input value={state.quietFrom} onChange={(e) => onChange({ ...state, quietFrom: e.target.value })} className="w-16 rounded-md border border-neutral-200 bg-white px-2 py-1 focus:outline-none focus:border-accent-solid" />
              <span className="text-neutral-400">→</span>
              <input value={state.quietTo} onChange={(e) => onChange({ ...state, quietTo: e.target.value })} className="w-16 rounded-md border border-neutral-200 bg-white px-2 py-1 focus:outline-none focus:border-accent-solid" />
            </div>
          )}
          <Switch on={state.quietHours} label="Quiet hours" onChange={(v) => { onChange({ ...state, quietHours: v }); onToast(v ? "Quiet hours on" : "Quiet hours off"); }} />
        </div>
      </div>

      <div className="mt-5">
        <Label>Per event type</Label>
        <div className="mt-2 divide-y divide-neutral-100 border border-neutral-200 rounded-xl overflow-hidden bg-white">
          {(Object.keys(EVENT_LABELS) as EventType[]).map((e) => (
            <div key={e} className="px-3 py-3 grid grid-cols-1 sm:grid-cols-[1fr_auto] gap-3 sm:items-center">
              <div className="min-w-0">
                <div className="text-[13px] font-semibold text-neutral-900">{EVENT_LABELS[e]}</div>
                <div className="text-[11.5px] text-neutral-500 leading-snug">{EVENT_HINTS[e]}</div>
              </div>
              <select value={state.event[e]} onChange={(ev) => updateEvent(e, ev.target.value as NotifChannel)} className="rounded-md border border-neutral-200 bg-white px-2 py-1.5 text-[12.5px] text-neutral-800 focus:outline-none focus:border-accent-solid">
                {(Object.keys(CHANNEL_LABELS) as NotifChannel[]).map((ch) => (
                  <option key={ch} value={ch}>{CHANNEL_LABELS[ch]}</option>
                ))}
              </select>
            </div>
          ))}
        </div>
        <p className="mt-2 text-[11px] text-neutral-500">
          Changes here save immediately — switching the channel doesn't fire any notifications retroactively.
        </p>
      </div>
    </SectionCard>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Recipe requests
// ─────────────────────────────────────────────────────────────────────────────

interface RecipesSectionProps {
  readonly requests: readonly RecipeRequest[];
  readonly onSubmit: (title: string, one: string) => void;
  readonly onVote: (id: string) => void;
}
function RecipesSection({ requests, onSubmit, onVote }: RecipesSectionProps) {
  const [title, setTitle] = useState("");
  const [one, setOne] = useState("");
  const valid = title.trim().length > 4 && one.trim().length > 6;

  return (
    <SectionCard id="recipes" title="Recipe requests" subtitle="Suggest a new recipe and vote on what others want. We ship the highest-voted requests with a SafetyCard each.">
      <form
        className="rounded-xl border border-neutral-200 bg-neutral-50/60 p-3 grid grid-cols-1 sm:grid-cols-2 gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (!valid) return;
          onSubmit(title.trim(), one.trim());
          setTitle("");
          setOne("");
        }}
      >
        <div>
          <Label htmlFor="rr-title">Recipe name</Label>
          <input id="rr-title" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="e.g. Reconcile bank export with invoices" className="mt-1 w-full rounded-md border border-neutral-200 bg-white px-2.5 py-1.5 text-[13px] text-neutral-900 focus:outline-none focus:border-accent-solid" />
        </div>
        <div>
          <Label htmlFor="rr-one">One-line purpose</Label>
          <input id="rr-one" value={one} onChange={(e) => setOne(e.target.value)} placeholder="What would this recipe do, in plain language?" className="mt-1 w-full rounded-md border border-neutral-200 bg-white px-2.5 py-1.5 text-[13px] text-neutral-900 focus:outline-none focus:border-accent-solid" />
        </div>
        <div className="sm:col-span-2 flex items-center justify-between gap-2 flex-wrap">
          <p className="text-[11px] text-neutral-500">
            Submissions appear instantly. We never ship without a SafetyCard.
          </p>
          <button
            type="submit"
            disabled={!valid}
            className={cx("rounded-md text-[12.5px] font-medium px-3 py-1.5 inline-flex items-center gap-1.5", valid ? "bg-accent-solid hover:bg-accent-solid-hover text-white" : "bg-neutral-100 text-neutral-400 cursor-not-allowed")}
          >
            <ICON.Sparkles size={12} /> Submit request
          </button>
        </div>
      </form>

      <ul className="mt-4 divide-y divide-neutral-100 border border-neutral-200 rounded-xl overflow-hidden bg-white">
        {requests.map((r) => {
          const statusTone: ToneName = r.status === "shipped" ? "safe" : r.status === "in-review" ? "warning" : "neutral";
          const statusLabel = r.status === "shipped" ? "Shipped" : r.status === "in-review" ? "In review" : "Open";
          return (
            <li key={r.id} className="px-3 py-3 flex items-start gap-3">
              <button
                type="button"
                onClick={() => onVote(r.id)}
                disabled={r.status === "shipped"}
                className={cx(
                  "flex flex-col items-center justify-center rounded-md w-12 py-1.5 border text-center transition-colors",
                  r.mine ? "bg-accent border-accent text-accent" : "bg-white border-neutral-200 hover:bg-neutral-50 text-neutral-700",
                  r.status === "shipped" && "opacity-50 cursor-not-allowed",
                )}
              >
                <ICON.ChevronUp size={12} />
                <span className="text-[12px] font-mono font-semibold tabular-nums">{r.votes}</span>
              </button>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <h3 className="text-[13.5px] font-semibold text-neutral-900 truncate">{r.title}</h3>
                  <Chip tone={statusTone} dot={r.status === "shipped"}>{statusLabel}</Chip>
                  {r.mine && <Chip tone="accent">your vote</Chip>}
                </div>
                <p className="text-[12px] text-neutral-600 leading-snug mt-0.5">{r.one}</p>
              </div>
            </li>
          );
        })}
      </ul>
    </SectionCard>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Advanced
// ─────────────────────────────────────────────────────────────────────────────

interface AdvancedSectionProps {
  readonly telemetry: boolean;
  readonly debug: boolean;
  readonly onTelemetry: (v: boolean) => void;
  readonly onDebug: (v: boolean) => void;
  readonly onExport: () => void;
  readonly onDelete: () => void;
}
function AdvancedSection({ telemetry, debug, onTelemetry, onDebug, onExport, onDelete }: AdvancedSectionProps) {
  const [open, setOpen] = useState(false);
  const [confirmText, setConfirmText] = useState("");
  const [confirming, setConfirming] = useState(false);

  return (
    <section id="advanced" className="card overflow-hidden scroll-mt-20">
      <button type="button" onClick={() => setOpen((v) => !v)} className="w-full text-left px-5 py-4 border-b border-neutral-100 flex items-center justify-between gap-3 hover:bg-neutral-50/60">
        <div>
          <div className="section-title flex items-center gap-2">
            Advanced
            <Chip tone="neutral">collapsed by default</Chip>
          </div>
          <p className="text-[12.5px] text-neutral-500 mt-1">
            Telemetry, debug mode, export, and delete. You don't need this for normal operation.
          </p>
        </div>
        {open ? <ICON.ChevronUp size={16} /> : <ICON.ChevronDown size={16} />}
      </button>

      {open && (
        <div className="p-5 flex flex-col gap-4">
          <div className="flex items-center justify-between gap-3 flex-wrap rounded-xl border border-neutral-200 px-4 py-3">
            <div className="min-w-0">
              <div className="text-[13px] font-semibold text-neutral-900">Telemetry</div>
              <p className="text-[11.5px] text-neutral-500 leading-snug">
                Send anonymized crash and performance data. No content, no recipient addresses, no audit text.
              </p>
            </div>
            <Switch on={telemetry} onChange={onTelemetry} label="Telemetry" />
          </div>

          <div className="flex items-center justify-between gap-3 flex-wrap rounded-xl border border-neutral-200 px-4 py-3">
            <div className="min-w-0">
              <div className="text-[13px] font-semibold text-neutral-900">Debug mode</div>
              <p className="text-[11.5px] text-neutral-500 leading-snug">
                Adds verbose console logs and a developer drawer to every route. Reset on each sign-in.
              </p>
            </div>
            <Switch on={debug} onChange={onDebug} label="Debug mode" />
          </div>

          <div className="flex items-center justify-between gap-3 flex-wrap rounded-xl border border-neutral-200 px-4 py-3">
            <div className="min-w-0">
              <div className="text-[13px] font-semibold text-neutral-900">Export workspace data</div>
              <p className="text-[11.5px] text-neutral-500 leading-snug">
                Builds a JSON + Markdown archive of your inbox, recipes, memory, and audit log. We email you the link.
              </p>
            </div>
            <button type="button" onClick={onExport} className="rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-800 px-3 py-1.5 text-[12.5px] font-medium inline-flex items-center gap-1.5">
              <ICON.Download size={12} /> Export
            </button>
          </div>

          <div className="rounded-xl border border-blocked bg-blocked/40 px-4 py-3">
            <div className="flex items-start justify-between gap-3 flex-wrap">
              <div className="min-w-0">
                <div className="text-[13px] font-semibold text-neutral-900 inline-flex items-center gap-1.5">
                  <ICON.AlertTriangle size={13} className="text-blocked" />
                  Delete workspace
                </div>
                <p className="text-[11.5px] text-neutral-700 leading-snug">
                  Permanently removes <span className="font-mono">{WORKSPACE.name}</span>, every recipe, memory item, and audit entry. Connectors are revoked. Recoverable for 30 days from a backup token mailed to you.
                </p>
              </div>
              {!confirming && (
                <button type="button" onClick={() => setConfirming(true)} className="rounded-md border border-blocked bg-white hover:bg-blocked text-blocked px-3 py-1.5 text-[12.5px] font-medium inline-flex items-center gap-1.5">
                  <ICON.Trash size={12} /> Delete…
                </button>
              )}
            </div>

            {confirming && (
              <div className="mt-3 grid grid-cols-1 sm:grid-cols-[1fr_auto] gap-2 items-end">
                <div>
                  <Label htmlFor="conf">Type the workspace name to confirm</Label>
                  <input id="conf" value={confirmText} onChange={(e) => setConfirmText(e.target.value)} placeholder={WORKSPACE.name} className="mt-1 w-full rounded-md border border-blocked bg-white px-2.5 py-1.5 text-[13px] text-neutral-900 focus:outline-none" />
                </div>
                <div className="flex items-center gap-2">
                  <button type="button" onClick={() => { setConfirming(false); setConfirmText(""); }} className="rounded-md border border-neutral-200 bg-white hover:bg-neutral-50 text-neutral-700 px-3 py-1.5 text-[12.5px] font-medium">
                    Cancel
                  </button>
                  <button
                    type="button"
                    disabled={confirmText !== WORKSPACE.name}
                    onClick={() => { onDelete(); setConfirming(false); setConfirmText(""); }}
                    className={cx("rounded-md px-3 py-1.5 text-[12.5px] font-medium inline-flex items-center gap-1.5", confirmText === WORKSPACE.name ? "bg-blocked-solid hover:bg-blocked-solid-hover text-white" : "bg-neutral-100 text-neutral-400 cursor-not-allowed")}
                  >
                    <ICON.Trash size={12} /> Confirm delete
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </section>
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Main route component
// ─────────────────────────────────────────────────────────────────────────────

export function Settings() {
  const [active, setActive] = useState<string>("account");

  const [notif, setNotif] = useState<NotifState>({
    quietHours: true,
    quietFrom: "23:00",
    quietTo: "06:00",
    event: {
      "awaiting-approval": "web-push",
      blocked: "email",
      healed: "digest",
      completed: "nothing",
      failed: "mobile-push",
    },
  });
  const [requests, setRequests] = useState<RecipeRequest[]>(() => RECIPE_REQUESTS_INITIAL.map((r) => ({ ...r })));
  const [deployment, setDeployment] = useState<DeploymentMode>("solo");
  const [telemetry, setTelemetry] = useState(false);
  const [debug, setDebug] = useState(false);

  const [undo, setUndo] = useState<{ msg: string; expiresAt: number } | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [, force] = useState({});

  useEffect(() => {
    if (!toast) return undefined;
    const t = window.setTimeout(() => setToast(null), 1800);
    return () => window.clearTimeout(t);
  }, [toast]);

  useEffect(() => {
    if (!undo) return undefined;
    const i = window.setInterval(() => {
      if (Date.now() > undo.expiresAt) setUndo(null);
      else force({});
    }, 1000);
    return () => window.clearInterval(i);
  }, [undo]);

  const rootRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const obs = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio);
        if (visible[0]) setActive(visible[0].target.id);
      },
      { rootMargin: "-30% 0px -55% 0px", threshold: [0, 0.25, 0.5, 0.75, 1] },
    );
    SECTIONS.forEach((s) => {
      const el = document.getElementById(s.id);
      if (el) obs.observe(el);
    });
    return () => obs.disconnect();
  }, []);

  const onJump = (id: string) => {
    const el = document.getElementById(id);
    if (!el) return;
    const y = el.getBoundingClientRect().top + window.scrollY - 16;
    window.scrollTo({ top: y, behavior: "smooth" });
    setActive(id);
  };

  const showToast = (msg: string) => setToast(msg);
  const showUndo = (msg: string, days = 30) =>
    setUndo({ msg, expiresAt: Date.now() + days * 24 * 3600 * 1000 });

  const remaining = useMemo(() => {
    if (!undo) return "";
    const ms = undo.expiresAt - Date.now();
    const days = Math.floor(ms / (24 * 3600 * 1000));
    if (days >= 1) return `${days}d to undo`;
    const h = Math.floor(ms / (3600 * 1000));
    if (h >= 1) return `${h}h to undo`;
    const m = Math.floor(ms / 60000);
    return `${Math.max(m, 1)}m to undo`;
  }, [undo]);

  return (
    <div className="min-h-screen bg-neutral-50">
      <WorkspaceTopbar />

      <main className="page-container max-w-[80rem]">
        <div className="mb-6 flex items-end justify-between gap-3 flex-wrap">
          <div>
            <h1 className="page-title">Settings</h1>
            <p className="mt-1 text-[13.5px] text-neutral-500 max-w-2xl leading-relaxed">
              Account, connectors, where your data lives, and how IronGolem tells you about things.
            </p>
          </div>
          <Chip tone="safe" dot>All changes are reversible within 30 days, unless we say otherwise.</Chip>
        </div>

        <div ref={rootRef} className="grid grid-cols-1 md:grid-cols-[200px_1fr] gap-6">
          <Sidebar active={active} onJump={onJump} />

          <div className="flex flex-col gap-6 min-w-0">
            <AccountSection onSignOutEverywhere={() => showToast("Signed out on 4 other devices.")} />

            <ConnectorsSection
              onToast={(m) => {
                showToast(m);
                if (m.includes("Disconnect")) showUndo(m);
              }}
            />

            <DeploymentSection
              active={deployment}
              onRequestSwitch={(target) => {
                setDeployment(target);
                const mode = MODES.find((m) => m.id === target);
                if (!mode) return;
                showToast(`Switched to ${mode.name} mode.`);
                showUndo(`Switched to ${mode.name} · revert within 7 days`, 7);
              }}
            />

            <NotificationsSection state={notif} onChange={setNotif} onToast={showToast} />

            <RecipesSection
              requests={requests}
              onSubmit={(title, one) => {
                const id = "rr-" + Math.random().toString(36).slice(2, 7);
                setRequests((prev) => [{ id, title, one, votes: 1, mine: true, status: "open" }, ...prev]);
                showToast(`Submitted "${title}"`);
              }}
              onVote={(id) => {
                setRequests((prev) =>
                  prev.map((r) =>
                    r.id === id ? { ...r, votes: r.mine ? r.votes - 1 : r.votes + 1, mine: !r.mine } : r,
                  ),
                );
              }}
            />

            <AdvancedSection
              telemetry={telemetry}
              debug={debug}
              onTelemetry={(v) => { setTelemetry(v); showToast(v ? "Telemetry on" : "Telemetry off"); }}
              onDebug={(v) => { setDebug(v); showToast(v ? "Debug mode on" : "Debug mode off"); }}
              onExport={() => showToast("Export queued · we'll email you the link.")}
              onDelete={() => showUndo(`Deletion scheduled for ${WORKSPACE.name} · 30-day undo`, 30)}
            />

            <p className="text-[11.5px] text-neutral-500 mt-2 mb-6 inline-flex items-center gap-1">
              <ICON.Lock size={11} className="text-safe-solid" />
              IronGolem never sends your data outside the region listed in Account.
            </p>
          </div>
        </div>
      </main>

      {toast && (
        <div className="fixed bottom-5 left-1/2 -translate-x-1/2 z-40 ig-toast-in">
          <div className="rounded-lg bg-neutral-900 text-white shadow-lg px-3.5 py-2 text-[12.5px] font-medium inline-flex items-center gap-2">
            <ICON.Check size={13} className="text-safe-solid" />
            {toast}
          </div>
        </div>
      )}

      {undo && (
        <div className="fixed bottom-5 right-5 z-40 max-w-sm">
          <div className="rounded-xl border border-accent bg-white shadow-lg px-3.5 py-2.5 flex items-start gap-2.5">
            <ICON.Undo size={14} className="mt-0.5 text-accent shrink-0" />
            <div className="min-w-0 flex-1">
              <div className="text-[12.5px] font-semibold text-neutral-900">{undo.msg}</div>
              <div className="text-[11px] text-neutral-500 font-mono mt-0.5">{remaining}</div>
            </div>
            <button type="button" onClick={() => setUndo(null)} className="rounded-md text-[12px] font-medium text-accent hover:bg-accent px-2 py-1">
              Undo
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
