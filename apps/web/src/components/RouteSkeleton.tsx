/**
 * RouteSkeleton — generic loading silhouette for v2 routes.
 *
 * v0.3 Step 6 of `Plans/modular-puzzling-blum.md`. Replaces the silent
 * `.catch(() => mockFallback)` pattern that landed during v0.2 — once a
 * route opts into the `{ status, data, error }` envelope (`useRouteData`),
 * its initial render shows a shape-matched skeleton instead of a flash
 * of stale mock data.
 *
 * Three variants cover every v2 layout the page registry ships today:
 *
 *   - "cards"    — Home, Settings, Recipes (grid of self-contained cards)
 *   - "list"     — Inbox, Commitments, Audit (vertical row list)
 *   - "timeline" — Health, Memory (event timeline + status block)
 *
 * Variants are matched to the page's hero shape so the page-loaded view
 * doesn't jump on transition. New routes pick the closest match; if
 * none fit, fall back to "cards" — it's the most generic.
 */

import React from "react";

export type RouteSkeletonVariant = "cards" | "list" | "timeline";

export interface RouteSkeletonProps {
  /** Layout to silhouette. Defaults to "cards" — the most generic shape. */
  readonly variant?: RouteSkeletonVariant;
  /** Number of placeholder blocks. Defaults to a sensible per-variant value. */
  readonly count?: number;
  /** Optional aria-label for screen readers. */
  readonly label?: string;
}

/** Tailwind classes shared by every silhouette block. */
const BLOCK_BASE = "rounded-md bg-neutral-200/70 animate-pulse";

export function RouteSkeleton({
  variant = "cards",
  count,
  label = "Loading…",
}: RouteSkeletonProps): React.JSX.Element {
  switch (variant) {
    case "list":
      return <ListSkeleton count={count ?? 5} label={label} />;
    case "timeline":
      return <TimelineSkeleton count={count ?? 4} label={label} />;
    case "cards":
    default:
      return <CardsSkeleton count={count ?? 3} label={label} />;
  }
}

function CardsSkeleton({
  count,
  label,
}: {
  readonly count: number;
  readonly label: string;
}): React.JSX.Element {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label={label}
      className="page-container grid grid-cols-1 lg:grid-cols-2 gap-4"
    >
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="card card-padded space-y-3">
          <div className={`${BLOCK_BASE} h-5 w-1/3`} />
          <div className={`${BLOCK_BASE} h-3 w-full`} />
          <div className={`${BLOCK_BASE} h-3 w-5/6`} />
          <div className={`${BLOCK_BASE} h-3 w-4/6`} />
          <div className="pt-2 flex gap-2">
            <div className={`${BLOCK_BASE} h-7 w-20`} />
            <div className={`${BLOCK_BASE} h-7 w-24`} />
          </div>
        </div>
      ))}
    </div>
  );
}

function ListSkeleton({
  count,
  label,
}: {
  readonly count: number;
  readonly label: string;
}): React.JSX.Element {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label={label}
      className="page-container space-y-2"
    >
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="card card-padded flex items-center gap-3">
          <div className={`${BLOCK_BASE} h-10 w-10 shrink-0 rounded-full`} />
          <div className="flex-1 space-y-2">
            <div className={`${BLOCK_BASE} h-4 w-1/2`} />
            <div className={`${BLOCK_BASE} h-3 w-3/4`} />
          </div>
          <div className={`${BLOCK_BASE} h-6 w-16 shrink-0`} />
        </div>
      ))}
    </div>
  );
}

function TimelineSkeleton({
  count,
  label,
}: {
  readonly count: number;
  readonly label: string;
}): React.JSX.Element {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label={label}
      className="page-container space-y-6"
    >
      <div className="card card-padded space-y-3">
        <div className={`${BLOCK_BASE} h-5 w-2/5`} />
        <div className={`${BLOCK_BASE} h-3 w-full`} />
        <div className={`${BLOCK_BASE} h-3 w-4/5`} />
      </div>
      <div className="space-y-3">
        {Array.from({ length: count }).map((_, i) => (
          <div key={i} className="flex gap-3 items-start">
            <div className={`${BLOCK_BASE} h-3 w-3 shrink-0 rounded-full mt-1.5`} />
            <div className="flex-1 space-y-2">
              <div className={`${BLOCK_BASE} h-3 w-1/3`} />
              <div className={`${BLOCK_BASE} h-3 w-2/3`} />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
