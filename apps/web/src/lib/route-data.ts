/**
 * useRouteData — opt-in `{ status, data, error }` envelope for v2 routes.
 *
 * v0.3 Step 6 of `Plans/modular-puzzling-blum.md`. Replaces the v0.2
 * `.catch(() => mockFallback)` pattern (silent fail; user thinks data is
 * fresh when it isn't) with an explicit state machine. Pages opt in per-
 * route — Home/Inbox/Health migrate in v0.3 Step 6; Commitments + Audit
 * (Step 7) use it natively.
 *
 * Two modes:
 *
 * 1. **Seeded mode** (`initialData` provided): the seed (typically the
 *    mock) renders immediately. The loader runs in the background. On
 *    success the data swaps; on failure `status === "error"` while
 *    `data` keeps the seed so the page can degrade gracefully if it
 *    wants. Home / Health adopt this so their layout never flashes
 *    empty.
 *
 * 2. **Skeleton mode** (`initialData === null`): nothing renders until
 *    the loader resolves. `status === "loading"` triggers a
 *    `<RouteSkeleton>` in the page. Commitments + Audit use this.
 *
 * Why a hook instead of an HOC: pages already drive complex local state
 * (reducers, drawer flags, filter chips). A hook composes; an HOC would
 * fight the existing shape.
 */

import { useCallback, useEffect, useRef, useState } from "react";

export type RouteDataStatus = "loading" | "ok" | "error";

export interface RouteDataEnvelope<T> {
  readonly status: RouteDataStatus;
  readonly data: T | null;
  readonly error: unknown;
  /** Re-runs the loader. Safe to invoke multiple times — concurrent
   *  re-fetches are de-duped (only the most recent resolution wins). */
  readonly reload: () => void;
}

export interface UseRouteDataOptions<T> {
  /** Loader function. Called on mount and on every `reload()`. */
  readonly load: () => Promise<T>;
  /** Optional initial data shown while the first load is in flight.
   *  Pass `null` to render the skeleton instead. */
  readonly initialData?: T | null;
  /** Optional dependency list — when these change, the loader re-runs.
   *  Defaults to `[]` so the loader runs once on mount. */
  readonly deps?: readonly unknown[];
}

/**
 * Subscribe to an async loader and surface its state to the page.
 *
 * Cleanup-aware: if the component unmounts before the loader resolves,
 * the resolution is discarded so React doesn't fire `setState` on an
 * unmounted component.
 */
export function useRouteData<T>(
  options: UseRouteDataOptions<T>,
): RouteDataEnvelope<T> {
  const { load, initialData = null, deps = [] } = options;
  const [data, setData] = useState<T | null>(initialData ?? null);
  const [status, setStatus] = useState<RouteDataStatus>(
    initialData != null ? "ok" : "loading",
  );
  const [error, setError] = useState<unknown>(null);

  // Track the latest in-flight invocation so a stale resolve can't
  // overwrite a fresher one.
  const invocationRef = useRef(0);
  // Keep a stable handle to the loader so `reload()` doesn't change
  // identity on every render (which would break consumer memoization).
  const loadRef = useRef(load);
  useEffect(() => {
    loadRef.current = load;
  }, [load]);

  const run = useCallback(() => {
    const myInvocation = ++invocationRef.current;
    setStatus((prev) => (prev === "ok" ? prev : "loading"));
    setError(null);
    loadRef
      .current()
      .then((next) => {
        if (myInvocation !== invocationRef.current) return;
        setData(next);
        setStatus("ok");
      })
      .catch((err) => {
        if (myInvocation !== invocationRef.current) return;
        setError(err);
        setStatus("error");
      });
  }, []);

  useEffect(() => {
    run();
    return () => {
      // Bumping the counter discards any in-flight resolve so a slow
      // request from a previous mount can't smear state across mounts.
      invocationRef.current++;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { status, data, error, reload: run };
}
