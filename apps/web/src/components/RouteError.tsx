/**
 * RouteError — explicit error state for caught-but-non-crash failures.
 *
 * v0.3 Step 6 of `Plans/modular-puzzling-blum.md`. Surfaces a real-mode
 * fetch failure to the user instead of silently substituting mock data
 * (the v0.2 default). Use when `useRouteData()` reports
 * `status === "error"` — never wrap normal mock-mode failures since
 * those should fall through silently as a dev affordance.
 *
 * The retry mechanism is a controlled prop because the data source
 * lives in `useRouteData()`; this component just renders the affordance
 * and notifies. Catching crashes (uncaught exceptions during render) is
 * a separate concern — see `RouteErrorBoundary.tsx`.
 */

import React from "react";

export interface RouteErrorProps {
  /** Route name shown in the heading. e.g. "Inbox", "Health". */
  readonly route: string;
  /** Underlying error — surfaced as the diagnostic detail line. */
  readonly error: unknown;
  /** Optional retry callback. When provided, a Retry button appears. */
  readonly onRetry?: () => void;
}

export function RouteError({
  route,
  error,
  onRetry,
}: RouteErrorProps): React.JSX.Element {
  const message = errorMessage(error);
  return (
    <div className="page-container">
      <div
        className="card card-padded border-l-4 border-blocked"
        role="alert"
        aria-live="polite"
      >
        <h2 className="text-lg font-semibold text-neutral-900">
          {route} couldn't load
        </h2>
        <p className="mt-1 text-sm text-neutral-600">
          The gateway returned an error or was unreachable. Mock data is
          intentionally hidden so the failure stays visible rather than
          silently substituting stale state.
        </p>
        <pre className="mt-3 text-xs bg-neutral-100 text-neutral-700 rounded p-3 overflow-x-auto">
          {message}
        </pre>
        <div className="mt-4 flex items-center gap-2">
          {onRetry ? (
            <button
              type="button"
              onClick={onRetry}
              className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid hover:bg-accent-solid-hover text-text-solid px-3 py-1.5 text-sm font-medium"
            >
              Retry
            </button>
          ) : null}
          <span className="text-xs text-neutral-500">
            See <code className="font-mono">gateway.log</code> for the full
            trace.
          </span>
        </div>
      </div>
    </div>
  );
}

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === "string") return err;
  try {
    return JSON.stringify(err);
  } catch {
    return String(err);
  }
}
