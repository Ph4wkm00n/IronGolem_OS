/**
 * Registry of v2 routes ready for promotion.
 *
 * App.tsx consults this map when `VITE_ENABLE_V2_UI=true`. Each entry says
 * "route X has a v2 page; render it instead of the legacy page." Routes
 * absent from this map continue to use their `pages/<Route>.tsx` original.
 *
 * Lazy loaders + named-export shim — each route becomes its own Vite chunk
 * so the main bundle stays under the 500 KB threshold and a user who only
 * hits `/` doesn't download Memory + Security + Settings on first load.
 * Each `import("./X").then((m) => ({ default: m.X }))` adapts the named
 * export to the `{ default: Component }` shape `React.lazy` requires.
 *
 * v0.3 Step 6 — every lazy component is wrapped in a `RouteErrorBoundary`
 * so a crash inside one v2 route only takes down that route, not the
 * entire app. The wrap is centralised here so new entries inherit the
 * safety net automatically.
 */

import React, {
  Fragment,
  lazy,
  type ComponentType,
  type LazyExoticComponent,
} from "react";

import { RouteErrorBoundary } from "../../components/RouteErrorBoundary";

const HomeLazy = lazy(() => import("./Home").then((m) => ({ default: m.Home })));
const InboxLazy = lazy(() => import("./Inbox").then((m) => ({ default: m.Inbox })));
const RecipesLazy = lazy(() => import("./Recipes").then((m) => ({ default: m.Recipes })));
const ResearchLazy = lazy(() => import("./Research").then((m) => ({ default: m.Research })));
const MemoryLazy = lazy(() => import("./Memory").then((m) => ({ default: m.Memory })));
const HealthLazy = lazy(() => import("./Health").then((m) => ({ default: m.Health })));
const SecurityLazy = lazy(() => import("./Security").then((m) => ({ default: m.Security })));
const SettingsLazy = lazy(() => import("./Settings").then((m) => ({ default: m.Settings })));
const CommitmentsLazy = lazy(() => import("./Commitments").then((m) => ({ default: m.Commitments })));
const AuditLazy = lazy(() => import("./Audit").then((m) => ({ default: m.Audit })));

/**
 * Wrap a lazy route in a `RouteErrorBoundary` keyed by route name so
 * each gets its own scoped fallback UI (one crash doesn't propagate).
 * The wrapper is a plain `ComponentType` so the App.tsx Routes still
 * render it directly inside `<Suspense>`.
 */
function withBoundary(
  name: string,
  Inner: LazyExoticComponent<ComponentType>,
): ComponentType {
  const Wrapped: ComponentType = () => (
    <RouteErrorBoundary route={name}>
      <Fragment>
        <Inner />
      </Fragment>
    </RouteErrorBoundary>
  );
  Wrapped.displayName = `WithBoundary(${name})`;
  return Wrapped;
}

const HomeWithBoundary = withBoundary("Home", HomeLazy);
const InboxWithBoundary = withBoundary("Inbox", InboxLazy);
const RecipesWithBoundary = withBoundary("Recipes", RecipesLazy);
const ResearchWithBoundary = withBoundary("Research", ResearchLazy);
const MemoryWithBoundary = withBoundary("Memory", MemoryLazy);
const HealthWithBoundary = withBoundary("Health", HealthLazy);
const SecurityWithBoundary = withBoundary("Security", SecurityLazy);
const SettingsWithBoundary = withBoundary("Settings", SettingsLazy);
const CommitmentsWithBoundary = withBoundary("Commitments", CommitmentsLazy);
const AuditWithBoundary = withBoundary("Audit", AuditLazy);

/**
 * Map of route path → lazy v2 component. Consumers must render results
 * inside a `<React.Suspense>` boundary (App.tsx does this).
 */
export const V2_ROUTE_REGISTRY: Readonly<Record<string, ComponentType>> = Object.freeze({
  "/": HomeWithBoundary,
  "/inbox": InboxWithBoundary,
  "/recipes": RecipesWithBoundary,
  "/research": ResearchWithBoundary,
  "/memory": MemoryWithBoundary,
  "/health": HealthWithBoundary,
  "/security": SecurityWithBoundary,
  "/settings": SettingsWithBoundary,
  "/commitments": CommitmentsWithBoundary,
  "/audit": AuditWithBoundary,
});

/**
 * Whether the v2 UI is opted into at build time.
 *
 * Read once at module load — Vite inlines `import.meta.env` values so
 * downstream branches dead-code-eliminate when the flag is off.
 */
export const ENABLE_V2_UI: boolean = import.meta.env.VITE_ENABLE_V2_UI === "true";

/**
 * Resolve which component to render for a given route. Falls back to the
 * legacy page when v2 is off OR the registry has no entry for the path.
 *
 * When v2 returns a `LazyExoticComponent`, the caller must be inside a
 * Suspense boundary. When v2 is off, the legacy `ComponentType` is returned
 * directly and no Suspense is required.
 */
export function pickPage(path: string, legacy: ComponentType): ComponentType {
  if (!ENABLE_V2_UI) return legacy;
  return (V2_ROUTE_REGISTRY[path] ?? legacy) as ComponentType;
}
