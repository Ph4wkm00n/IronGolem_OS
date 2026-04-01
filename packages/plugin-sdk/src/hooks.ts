/**
 * Hook system for the IronGolem OS Plugin SDK.
 *
 * Hooks allow plugins to intercept and react to platform events at
 * well-defined lifecycle points. The HookManager maintains a priority-ordered
 * list of handlers for each hook type and executes them in sequence.
 */

import type { PluginContext, HookHandlerResult } from "./types";

// ---------------------------------------------------------------------------
// Hook types
// ---------------------------------------------------------------------------

/** Enumeration of all hook types the platform supports. */
export enum HookType {
  /** Fires before an autonomous action is executed. Can abort or modify. */
  BeforeAction = "before_action",
  /** Fires after an autonomous action completes. Informational. */
  AfterAction = "after_action",
  /** Fires when a platform event is emitted. */
  OnEvent = "on_event",
  /** Fires on a cron-like schedule. */
  OnSchedule = "on_schedule",
  /** Fires when an approval decision is needed. */
  OnApproval = "on_approval",
}

// ---------------------------------------------------------------------------
// Hook result helpers
// ---------------------------------------------------------------------------

/** Convenience constructors for common hook results. */
export const HookResult = {
  /** Allow the pipeline to continue unchanged. */
  continue(reason?: string): HookHandlerResult {
    return { action: "continue", reason };
  },
  /** Abort the pipeline. */
  abort(reason: string): HookHandlerResult {
    return { action: "abort", reason };
  },
  /** Continue with a modified payload. */
  modify(modifiedPayload: unknown, reason?: string): HookHandlerResult {
    return { action: "modify", modifiedPayload, reason };
  },
} as const;

// ---------------------------------------------------------------------------
// Hook registration entry
// ---------------------------------------------------------------------------

/** Internal representation of a registered hook handler. */
interface HookEntry {
  pluginId: string;
  hookType: HookType;
  priority: number;
  handler: (ctx: PluginContext, payload: unknown) => Promise<HookHandlerResult>;
}

// ---------------------------------------------------------------------------
// HookManager
// ---------------------------------------------------------------------------

export class HookManager {
  private readonly hooks: HookEntry[] = [];

  /**
   * Register a hook handler for a given type.
   *
   * @param pluginId  - The plugin that owns this hook.
   * @param hookType  - Which lifecycle point to bind to.
   * @param priority  - Execution priority (lower runs first, default 100).
   * @param handler   - The handler function.
   */
  register(
    pluginId: string,
    hookType: HookType,
    priority: number,
    handler: (ctx: PluginContext, payload: unknown) => Promise<HookHandlerResult>,
  ): void {
    this.hooks.push({ pluginId, hookType, priority, handler });
    // Keep sorted by priority so trigger() can iterate in order.
    this.hooks.sort((a, b) => a.priority - b.priority);
  }

  /**
   * Remove all hooks registered by a specific plugin.
   */
  unregisterPlugin(pluginId: string): void {
    let i = this.hooks.length;
    while (i--) {
      if (this.hooks[i].pluginId === pluginId) {
        this.hooks.splice(i, 1);
      }
    }
  }

  /**
   * Trigger all hooks of a given type in priority order.
   *
   * For `before_action` hooks the pipeline short-circuits on "abort" and
   * threads modified payloads through subsequent handlers. All other hook
   * types are fire-and-forget (results are collected but do not alter flow).
   *
   * @returns The aggregated results from each handler.
   */
  async trigger(
    hookType: HookType,
    ctx: PluginContext,
    payload: unknown,
  ): Promise<{ results: HookHandlerResult[]; finalPayload: unknown }> {
    const relevant = this.hooks.filter((h) => h.hookType === hookType);
    const results: HookHandlerResult[] = [];
    let currentPayload = payload;

    for (const entry of relevant) {
      const result = await entry.handler(ctx, currentPayload);
      results.push(result);

      if (hookType === HookType.BeforeAction) {
        if (result.action === "abort") {
          // Short-circuit: no further hooks run.
          return { results, finalPayload: currentPayload };
        }
        if (result.action === "modify" && result.modifiedPayload !== undefined) {
          currentPayload = result.modifiedPayload;
        }
      }
    }

    return { results, finalPayload: currentPayload };
  }

  /**
   * Return the number of hooks currently registered for a given type.
   */
  countByType(hookType: HookType): number {
    return this.hooks.filter((h) => h.hookType === hookType).length;
  }

  /**
   * Return all hook types that a given plugin has registered.
   */
  getPluginHookTypes(pluginId: string): HookType[] {
    return [
      ...new Set(
        this.hooks
          .filter((h) => h.pluginId === pluginId)
          .map((h) => h.hookType),
      ),
    ];
  }
}
