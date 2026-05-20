/**
 * Hook decision types.
 *
 * v0.3 Step 2 of `Plans/modular-puzzling-blum.md`. Locks in the
 * `Allow | Deny | Modify | Observe` taxonomy ahead of any plugin system
 * (deferred to v0.4+ alongside `runtime/sandbox` WASM completion) so future
 * plugin authors cannot invent ad-hoc decision types. Adopted from
 * openclaw/openclaw `src/plugins/hook-decision-types.ts`.
 *
 * Vocabulary is shared with the five-layer policy engine in `policy.ts`:
 * - `Allow` ~ `PolicyEffect.allow`
 * - `Deny`  ~ `PolicyEffect.deny`
 * - `Modify` and `Observe` are hook-only; the policy engine never modifies
 *   payloads or runs purely observer-side without producing an effect.
 */

/** What a single hook returns. */
export type HookDecision = "allow" | "deny" | "modify" | "observe";

/** The lifecycle phase a hook fires at. */
export type HookPhase =
  | "before-agent-start"
  | "before-agent-reply"
  | "before-tool-call"
  | "after-tool-call"
  | "before-install";

/** Context passed to every hook invocation. Minimal by design — hooks
 *  that need richer state should accept a typed payload alongside this
 *  envelope rather than expanding it. */
export interface HookContext {
  /** Correlation across all events in this turn / plan execution. */
  readonly correlationId: string;
  /** Lifecycle phase firing. */
  readonly phase: HookPhase;
  /** Which agent is in scope (planner/executor/etc.). Optional for
   *  install-time hooks that pre-date any agent assignment. */
  readonly agentId?: string;
  /** Workspace owning this invocation. */
  readonly workspaceId: string;
  /** Tenant owning the workspace. */
  readonly tenantId: string;
}

/**
 * Result of a single hook invocation.
 *
 * - `decision = "allow"`: proceed; ignore `reason` and `modifiedPayload`.
 * - `decision = "deny"`: block; `reason` is surfaced to the user as the
 *    block explanation. `modifiedPayload` ignored.
 * - `decision = "modify"`: proceed with `modifiedPayload` REPLACING the
 *    original payload. `reason` shown alongside as the modification
 *    explanation. `modifiedPayload` is required.
 * - `decision = "observe"`: no effect; the hook was only logging /
 *    emitting telemetry. `reason` optional. `modifiedPayload` ignored.
 */
export interface HookResult {
  readonly decision: HookDecision;
  readonly reason?: string;
  readonly modifiedPayload?: unknown;
}

/** Human-facing labels for each decision — used in audit-trail rendering. */
export const hookDecisionLabel: Record<HookDecision, string> = {
  allow: "Allowed",
  deny: "Blocked",
  modify: "Modified",
  observe: "Observed",
};

/** True when the decision short-circuits hook chain evaluation. `deny`
 *  short-circuits the chain; `modify` keeps going (later hooks see the
 *  modified payload); `allow` and `observe` always continue. */
export function decisionShortCircuits(d: HookDecision): boolean {
  return d === "deny";
}
