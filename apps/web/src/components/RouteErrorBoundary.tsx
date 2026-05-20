/**
 * RouteErrorBoundary — React error boundary scoped to one v2 route.
 *
 * v0.3 Step 6 of `Plans/modular-puzzling-blum.md`. Before this, a single
 * uncaught exception inside any v2 page white-screened the entire app
 * because the only error boundary was the global Suspense fallback. Now
 * each lazy route gets wrapped automatically in `registry.ts`, so a
 * crash in `Inbox.tsx` only takes down Inbox — Home/Health/etc keep
 * rendering.
 *
 * Stays a class component because React error boundaries still require
 * `componentDidCatch` / `getDerivedStateFromError` lifecycle hooks
 * (functional alternatives don't exist as of React 19). Kept minimal so
 * any future migration is mechanical.
 */

import React from "react";

export interface RouteErrorBoundaryProps {
  /** Route name surfaced in the fallback. */
  readonly route?: string;
  /** What the boundary protects. */
  readonly children: React.ReactNode;
}

interface RouteErrorBoundaryState {
  readonly hasError: boolean;
  readonly error: Error | null;
}

export class RouteErrorBoundary extends React.Component<
  RouteErrorBoundaryProps,
  RouteErrorBoundaryState
> {
  override state: RouteErrorBoundaryState = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): RouteErrorBoundaryState {
    return { hasError: true, error };
  }

  override componentDidCatch(error: Error, info: React.ErrorInfo): void {
    // Surfaced through DevTools console + future telemetry hook. Kept
    // intentionally lean — anything fancier belongs in a real Sentry-
    // class telemetry sink that v0.3 doesn't ship.
    // eslint-disable-next-line no-console
    console.error(
      `[RouteErrorBoundary:${this.props.route ?? "unknown"}]`,
      error,
      info.componentStack,
    );
  }

  reset = (): void => {
    this.setState({ hasError: false, error: null });
  };

  override render(): React.ReactNode {
    if (!this.state.hasError) return this.props.children;

    const route = this.props.route ?? "this page";
    const message =
      this.state.error?.message ?? "Unknown render exception";

    return (
      <main className="min-h-screen bg-neutral-50">
        <div className="page-container">
          <div
            role="alert"
            aria-live="assertive"
            className="card card-padded border-l-4 border-blocked"
          >
            <h2 className="text-lg font-semibold text-neutral-900">
              {route} crashed during render
            </h2>
            <p className="mt-1 text-sm text-neutral-600">
              An unhandled exception interrupted this page. Other routes
              continue to work — only {route} is affected.
            </p>
            <pre className="mt-3 text-xs bg-neutral-100 text-neutral-700 rounded p-3 overflow-x-auto">
              {message}
            </pre>
            <div className="mt-4 flex items-center gap-2">
              <button
                type="button"
                onClick={this.reset}
                className="inline-flex items-center gap-1.5 rounded-md bg-accent-solid hover:bg-accent-solid-hover text-text-solid px-3 py-1.5 text-sm font-medium"
              >
                Try to render again
              </button>
              <a
                href="/"
                className="text-sm text-accent hover:text-accent-solid"
              >
                Back to the dashboard
              </a>
            </div>
          </div>
        </div>
      </main>
    );
  }
}
