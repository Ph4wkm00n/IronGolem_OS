/**
 * WorkspaceTopbar — sticky navigation chrome shared across every v2 route.
 *
 * Lifted from the Workspace Dashboard's DashHeader so stubs and future
 * Claude Design pages don't each ship a divergent copy. The component is
 * mock-data-aware today (heartbeat + workspace come from a constant); when
 * lib/api.ts grows real endpoints these become fetched props.
 */

import React from "react";
import { Link, useLocation } from "react-router-dom";

interface IconProps {
  readonly size?: number;
  readonly className?: string;
}

const svgBase = (size: number, className: string) => ({
  xmlns: "http://www.w3.org/2000/svg",
  viewBox: "0 0 24 24",
  width: size,
  height: size,
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.5,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  className,
  "aria-hidden": true,
});

const IconRefresh = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M3 12a9 9 0 0 1 15-6.7L21 8" />
    <path d="M21 3v5h-5" />
    <path d="M21 12a9 9 0 0 1-15 6.7L3 16" />
    <path d="M3 21v-5h5" />
  </svg>
);

const IconBell = ({ size = 16, className = "" }: IconProps) => (
  <svg {...svgBase(size, className)}>
    <path d="M18 16v-5a6 6 0 1 0-12 0v5l-2 2h16l-2-2Z" />
    <path d="M10 20a2 2 0 0 0 4 0" />
  </svg>
);

const IconLogo = ({ size = 16, className = "" }: IconProps) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 20 24"
    width={size}
    height={size}
    fill="currentColor"
    className={className}
    aria-hidden
  >
    <rect x={3} y={6} width={6} height={2.5} rx={1} />
    <rect x={3} y={11} width={14} height={2.5} rx={1} />
    <rect x={3} y={16} width={9} height={2.5} rx={1} />
  </svg>
);

interface WorkspaceMeta {
  readonly name: string;
  readonly initials: string;
  readonly region: string;
  readonly lastSync: string;
}

const DEFAULT_WORKSPACE: WorkspaceMeta = {
  name: "Eastside Production",
  initials: "EP",
  region: "us-east-1",
  lastSync: "37 seconds ago",
};

/**
 * Nav items map to the v2 route registry. Items without a `to` render as
 * non-navigating placeholders for surfaces that haven't been carved into
 * their own routes yet (Timeline, Teams).
 */
const NAV_ITEMS: ReadonlyArray<{ readonly label: string; readonly to?: string }> = [
  { label: "Workspace", to: "/" },
  { label: "Inbox", to: "/inbox" },
  // v0.3 Step 7 — Commitments + Audit nav entries.
  { label: "Commitments", to: "/commitments" },
  { label: "Audit", to: "/audit" },
  { label: "Research", to: "/research" },
  { label: "Rules", to: "/security" },
];

export interface WorkspaceTopbarProps {
  readonly workspace?: WorkspaceMeta;
  /** Show "All systems normal" pulse — pass `false` to suppress (e.g. on Health page where it would be redundant). */
  readonly showHeartbeatPill?: boolean;
  /** When provided, the Reset button renders and fires this callback. */
  readonly onResetDemo?: () => void;
}

export function WorkspaceTopbar({
  workspace = DEFAULT_WORKSPACE,
  showHeartbeatPill = true,
  onResetDemo,
}: WorkspaceTopbarProps) {
  const { pathname } = useLocation();
  return (
    <header
      className="sticky top-0 z-30 bg-white border-b border-neutral-100"
      style={{ backdropFilter: "saturate(160%) blur(8px)" }}
    >
      <div className="page-container py-0">
        <div className="flex items-center h-14 gap-4">
          <div className="flex items-center gap-2 shrink-0">
            <span className="h-7 w-7 rounded-md bg-neutral-900 text-white inline-flex items-center justify-center">
              <IconLogo size={16} />
            </span>
            <span className="text-sm font-semibold tracking-tight text-neutral-900">
              IronGolem
            </span>
            <span className="text-neutral-300">/</span>
            <span className="text-sm text-neutral-700 inline-flex items-center gap-1.5">
              <span className="h-5 w-5 rounded bg-accent text-accent inline-flex items-center justify-center text-[10px] font-semibold">
                {workspace.initials}
              </span>
              {workspace.name}
            </span>
            <span className="text-[11px] font-mono text-neutral-400 hidden md:inline">
              {workspace.region}
            </span>
          </div>

          <nav className="hidden md:flex items-center gap-1 ml-4" aria-label="Workspace sections">
            {NAV_ITEMS.map((item) => {
              const active = item.to !== undefined && pathname === item.to;
              const base = "px-2.5 py-1.5 rounded-md text-sm font-medium transition-colors";
              const cls = active
                ? "text-neutral-900 bg-neutral-100"
                : "text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50";
              if (item.to) {
                return (
                  <Link key={item.label} to={item.to} className={`${base} ${cls}`}>
                    {item.label}
                  </Link>
                );
              }
              return (
                <span
                  key={item.label}
                  className={`${base} text-neutral-400 cursor-default`}
                  title="No dedicated route yet"
                >
                  {item.label}
                </span>
              );
            })}
          </nav>

          <div className="flex-1" />

          {showHeartbeatPill && (
            <div className="hidden md:flex items-center gap-2 text-xs text-neutral-500">
              <span className="inline-flex items-center gap-1.5">
                <span className="h-1.5 w-1.5 rounded-full bg-safe-solid ig-pulse" />
                All systems normal
              </span>
              <span className="text-neutral-300">·</span>
              <span>last sync {workspace.lastSync}</span>
            </div>
          )}

          {onResetDemo && (
            <button
              type="button"
              onClick={onResetDemo}
              title="Reset mock data"
              className="inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1.5 rounded-md text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50 transition-colors"
            >
              <IconRefresh size={12} />
              <span className="hidden sm:inline">Reset</span>
            </button>
          )}

          <button
            type="button"
            aria-label="Notifications"
            className="inline-flex items-center gap-1.5 text-xs font-medium px-2.5 py-1.5 rounded-md text-neutral-600 hover:text-neutral-900 hover:bg-neutral-50 transition-colors"
          >
            <IconBell size={14} />
          </button>

          <span className="h-7 w-7 rounded-full bg-accent text-accent inline-flex items-center justify-center text-[11px] font-semibold">
            AS
          </span>
        </div>
      </div>
    </header>
  );
}
