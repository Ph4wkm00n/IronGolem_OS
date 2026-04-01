/**
 * Core type definitions for the IronGolem OS Plugin SDK.
 *
 * Plugins extend the platform with new tools, connectors, recipe templates,
 * squad templates, or UI widgets. Every plugin must declare its capabilities
 * and the permissions it requires so the workspace policy engine can gate
 * activation.
 */

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------

/** The kind of extension a plugin provides. */
export type PluginCapability =
  | "tool"
  | "connector"
  | "recipe_template"
  | "squad_template"
  | "ui_widget";

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

/** Lifecycle hook identifiers. */
export type PluginHookName =
  | "onActivate"
  | "onDeactivate"
  | "onEvent"
  | "beforeAction"
  | "afterAction";

/** A concrete hook implementation provided by a plugin. */
export interface PluginHook {
  /** Which lifecycle point this hook binds to. */
  name: PluginHookName;
  /** Execution priority (lower runs first). Default 100. */
  priority?: number;
  /** The handler function invoked when the hook fires. */
  handler: (ctx: PluginContext, payload: unknown) => Promise<HookHandlerResult>;
}

/** The result a hook handler returns to the runtime. */
export interface HookHandlerResult {
  /** Whether to continue the pipeline, abort, or modify the payload. */
  action: "continue" | "abort" | "modify";
  /** When action is "modify", the replacement payload. */
  modifiedPayload?: unknown;
  /** Optional human-readable reason (surfaced in audit trail). */
  reason?: string;
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

/** Logger interface injected into plugins. */
export interface PluginLogger {
  debug(msg: string, meta?: Record<string, unknown>): void;
  info(msg: string, meta?: Record<string, unknown>): void;
  warn(msg: string, meta?: Record<string, unknown>): void;
  error(msg: string, meta?: Record<string, unknown>): void;
}

/** Minimal event emitter exposed to plugins. */
export interface PluginEventEmitter {
  emit(event: string, data: unknown): void;
  on(event: string, handler: (data: unknown) => void): void;
  off(event: string, handler: (data: unknown) => void): void;
}

/** Information about the workspace the plugin is running in. */
export interface WorkspaceInfo {
  id: string;
  name: string;
  deploymentMode: "solo" | "household" | "team";
}

/** Information about the user who activated the plugin. */
export interface UserInfo {
  id: string;
  displayName: string;
  role: string;
}

/** Runtime context provided to every plugin invocation. */
export interface PluginContext {
  workspace: WorkspaceInfo;
  user: UserInfo;
  events: PluginEventEmitter;
  logger: PluginLogger;
}

// ---------------------------------------------------------------------------
// Plugin interface
// ---------------------------------------------------------------------------

/** The main plugin contract that every plugin must satisfy. */
export interface Plugin {
  /** Globally unique plugin identifier (reverse-domain recommended). */
  id: string;
  /** Human-readable plugin name. */
  name: string;
  /** Semver version string. */
  version: string;
  /** Short description shown in the marketplace and admin console. */
  description: string;
  /** Author or organisation name. */
  author: string;
  /** The set of capabilities this plugin provides. */
  capabilities: PluginCapability[];
  /** Lifecycle hooks the plugin registers. */
  hooks: PluginHook[];

  /** Called once when the plugin is first loaded. */
  onActivate?(ctx: PluginContext): Promise<void>;
  /** Called when the plugin is being unloaded. */
  onDeactivate?(ctx: PluginContext): Promise<void>;
}

// ---------------------------------------------------------------------------
// Manifest
// ---------------------------------------------------------------------------

/** Permission a plugin declares it needs. */
export interface PluginPermission {
  /** The resource or API the plugin requires access to. */
  resource: string;
  /** Actions needed on that resource. */
  actions: string[];
}

/**
 * Manifest file shipped alongside a plugin.
 * The registry validates this before allowing activation.
 */
export interface PluginManifest {
  /** Must match the plugin's id. */
  id: string;
  name: string;
  version: string;
  description: string;
  author: string;
  /** Relative path to the plugin entry module. */
  entryPoint: string;
  /** Capabilities the plugin provides. */
  capabilities: PluginCapability[];
  /** Permissions the plugin requires to operate. */
  permissions: PluginPermission[];
  /** Minimum IronGolem OS version required. */
  minPlatformVersion?: string;
  /** Optional icon URL for UI display. */
  iconUrl?: string;
  /** Optional homepage / docs URL. */
  homepage?: string;
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

/** Lifecycle states a plugin passes through. */
export type PluginLifecycleState =
  | "loaded"
  | "validated"
  | "activated"
  | "running"
  | "deactivated"
  | "error";

/** Runtime record kept for each registered plugin. */
export interface PluginRecord {
  manifest: PluginManifest;
  state: PluginLifecycleState;
  enabled: boolean;
  activatedAt?: Date;
  error?: string;
}
