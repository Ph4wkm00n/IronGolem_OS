/**
 * @irongolem/plugin-sdk
 *
 * TypeScript SDK for building IronGolem OS plugins. Provides type
 * definitions, a plugin registry, sandboxing interfaces, and a hook
 * system for extending the platform safely.
 */

// Types & interfaces
export type {
  Plugin,
  PluginCapability,
  PluginHookName,
  PluginHook,
  HookHandlerResult,
  PluginLogger,
  PluginEventEmitter,
  WorkspaceInfo,
  UserInfo,
  PluginContext,
  PluginPermission,
  PluginManifest,
  PluginLifecycleState,
  PluginRecord,
} from "./types";

// Registry
export {
  PluginRegistry,
  PluginValidationError,
  AllowAllPolicyChecker,
} from "./registry";
export type { WorkspacePolicyChecker } from "./registry";

// Sandbox
export {
  DEFAULT_SANDBOX_CONFIG,
  WasmSandbox,
  ProcessSandbox,
} from "./sandbox";
export type {
  SandboxConfig,
  SandboxExecutionResult,
  PluginSandbox,
} from "./sandbox";

// Hooks
export { HookType, HookResult, HookManager } from "./hooks";
