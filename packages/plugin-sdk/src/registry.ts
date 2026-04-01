/**
 * Plugin registry - manages the full plugin lifecycle from registration
 * through activation and eventual deactivation.
 *
 * Lifecycle: load -> validate -> activate -> running -> deactivate
 */

import type {
  Plugin,
  PluginContext,
  PluginManifest,
  PluginPermission,
  PluginRecord,
  PluginLifecycleState,
} from "./types";

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

/** Errors surfaced when a manifest fails validation. */
export class PluginValidationError extends Error {
  constructor(
    public readonly pluginId: string,
    public readonly issues: string[],
  ) {
    super(`Plugin "${pluginId}" failed validation: ${issues.join("; ")}`);
    this.name = "PluginValidationError";
  }
}

/** Validate a manifest has all required fields and sane values. */
function validateManifest(manifest: PluginManifest): string[] {
  const issues: string[] = [];

  if (!manifest.id || manifest.id.trim().length === 0) {
    issues.push("id is required");
  }
  if (!manifest.name || manifest.name.trim().length === 0) {
    issues.push("name is required");
  }
  if (!manifest.version || !/^\d+\.\d+\.\d+/.test(manifest.version)) {
    issues.push("version must be a valid semver string");
  }
  if (!manifest.entryPoint || manifest.entryPoint.trim().length === 0) {
    issues.push("entryPoint is required");
  }
  if (!manifest.capabilities || manifest.capabilities.length === 0) {
    issues.push("at least one capability is required");
  }
  if (!manifest.author || manifest.author.trim().length === 0) {
    issues.push("author is required");
  }

  return issues;
}

// ---------------------------------------------------------------------------
// Workspace policy checker (simple interface)
// ---------------------------------------------------------------------------

/** Policy checker that the host provides to the registry. */
export interface WorkspacePolicyChecker {
  /** Return true if every permission in the list is allowed. */
  arePermissionsAllowed(permissions: PluginPermission[]): boolean;
}

/** Default policy checker that allows everything (development mode). */
export class AllowAllPolicyChecker implements WorkspacePolicyChecker {
  arePermissionsAllowed(_permissions: PluginPermission[]): boolean {
    return true;
  }
}

// ---------------------------------------------------------------------------
// PluginRegistry
// ---------------------------------------------------------------------------

export class PluginRegistry {
  private readonly plugins = new Map<string, PluginRecord>();
  private readonly instances = new Map<string, Plugin>();
  private readonly policyChecker: WorkspacePolicyChecker;

  constructor(policyChecker?: WorkspacePolicyChecker) {
    this.policyChecker = policyChecker ?? new AllowAllPolicyChecker();
  }

  // -----------------------------------------------------------------------
  // Query
  // -----------------------------------------------------------------------

  /** List all registered plugin records. */
  list(): PluginRecord[] {
    return Array.from(this.plugins.values());
  }

  /** Get a single plugin record by id, or undefined if not found. */
  getById(id: string): PluginRecord | undefined {
    return this.plugins.get(id);
  }

  // -----------------------------------------------------------------------
  // Registration
  // -----------------------------------------------------------------------

  /**
   * Register a plugin from its manifest and implementation.
   *
   * The method validates the manifest, checks permissions against workspace
   * policy, and transitions the plugin through load -> validate states.
   */
  register(manifest: PluginManifest, plugin: Plugin): PluginRecord {
    // Step 1: validate manifest schema
    const issues = validateManifest(manifest);
    if (issues.length > 0) {
      throw new PluginValidationError(manifest.id ?? "unknown", issues);
    }

    // Step 2: check permissions against workspace policy
    if (!this.policyChecker.arePermissionsAllowed(manifest.permissions)) {
      throw new PluginValidationError(manifest.id, [
        "required permissions are not allowed by workspace policy",
      ]);
    }

    // Step 3: store
    const record: PluginRecord = {
      manifest,
      state: "validated" as PluginLifecycleState,
      enabled: false,
    };
    this.plugins.set(manifest.id, record);
    this.instances.set(manifest.id, plugin);

    return record;
  }

  /** Remove a plugin from the registry entirely. */
  unregister(id: string): boolean {
    this.instances.delete(id);
    return this.plugins.delete(id);
  }

  // -----------------------------------------------------------------------
  // Lifecycle
  // -----------------------------------------------------------------------

  /** Activate a registered plugin, calling its onActivate hook. */
  async activate(id: string, ctx: PluginContext): Promise<void> {
    const record = this.requireRecord(id);
    const plugin = this.instances.get(id);
    if (!plugin) {
      throw new Error(`Plugin instance not found for "${id}"`);
    }

    try {
      if (plugin.onActivate) {
        await plugin.onActivate(ctx);
      }
      record.state = "running";
      record.enabled = true;
      record.activatedAt = new Date();
      record.error = undefined;
    } catch (err) {
      record.state = "error";
      record.error = err instanceof Error ? err.message : String(err);
      throw err;
    }
  }

  /** Deactivate a running plugin, calling its onDeactivate hook. */
  async deactivate(id: string, ctx: PluginContext): Promise<void> {
    const record = this.requireRecord(id);
    const plugin = this.instances.get(id);
    if (!plugin) {
      throw new Error(`Plugin instance not found for "${id}"`);
    }

    try {
      if (plugin.onDeactivate) {
        await plugin.onDeactivate(ctx);
      }
      record.state = "deactivated";
      record.enabled = false;
    } catch (err) {
      record.state = "error";
      record.error = err instanceof Error ? err.message : String(err);
      throw err;
    }
  }

  /** Enable a plugin (mark it as active without calling hooks). */
  enable(id: string): void {
    const record = this.requireRecord(id);
    record.enabled = true;
  }

  /** Disable a plugin (mark it as inactive without calling hooks). */
  disable(id: string): void {
    const record = this.requireRecord(id);
    record.enabled = false;
  }

  // -----------------------------------------------------------------------
  // Internal
  // -----------------------------------------------------------------------

  private requireRecord(id: string): PluginRecord {
    const record = this.plugins.get(id);
    if (!record) {
      throw new Error(`Plugin "${id}" is not registered`);
    }
    return record;
  }
}
