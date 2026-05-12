/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Set to `"true"` to opt into the Claude Design v2 route family. */
  readonly VITE_ENABLE_V2_UI?: string;
  /** Workspace-wide mock-vs-real toggle in `apps/web/src/lib/api.ts`. */
  readonly VITE_API_MODE?: "mock" | "real";
  /** Per-route override for the v0.1 backend rollout. Set to `"real"` for routes whose backend endpoint has landed; leave unset to fall back to `VITE_API_MODE`. */
  readonly VITE_API_MODE_HOME?: "mock" | "real";
  readonly VITE_API_MODE_INBOX?: "mock" | "real";
  readonly VITE_API_MODE_RECIPES?: "mock" | "real";
  readonly VITE_API_MODE_RESEARCH?: "mock" | "real";
  readonly VITE_API_MODE_MEMORY?: "mock" | "real";
  readonly VITE_API_MODE_HEALTH?: "mock" | "real";
  readonly VITE_API_MODE_SECURITY?: "mock" | "real";
  readonly VITE_API_MODE_SETTINGS?: "mock" | "real";
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
