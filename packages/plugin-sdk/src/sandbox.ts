/**
 * Plugin sandboxing interfaces.
 *
 * Plugins run in isolation so a buggy or malicious extension cannot
 * compromise the host. Two concrete strategies are planned:
 *
 *   - WasmSandbox  - WASM-based isolation (backed by the Rust runtime)
 *   - ProcessSandbox - OS process-based isolation with IPC
 *
 * Both share a common PluginSandbox interface so the registry and hook
 * manager are strategy-agnostic.
 */

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

/** Configuration for a sandbox instance. */
export interface SandboxConfig {
  /** Maximum execution time in milliseconds. */
  timeoutMs: number;
  /** Maximum heap memory in bytes (0 = unlimited). */
  memoryLimitBytes: number;
  /** Allowlist of platform APIs the plugin may call. */
  allowedApis: string[];
  /** Whether outbound network access is permitted. */
  networkAccess: boolean;
}

/** Sensible defaults for plugin sandboxes. */
export const DEFAULT_SANDBOX_CONFIG: SandboxConfig = {
  timeoutMs: 30_000,
  memoryLimitBytes: 64 * 1024 * 1024, // 64 MiB
  allowedApis: ["logger", "events", "storage.read"],
  networkAccess: false,
};

// ---------------------------------------------------------------------------
// Sandbox interface
// ---------------------------------------------------------------------------

/** The result of executing a plugin action inside a sandbox. */
export interface SandboxExecutionResult<T = unknown> {
  success: boolean;
  output?: T;
  error?: string;
  durationMs: number;
}

/** Strategy-agnostic sandbox interface. */
export interface PluginSandbox {
  /** Execute a plugin action inside the sandbox. */
  execute<T = unknown>(
    pluginId: string,
    action: string,
    input: unknown,
  ): Promise<SandboxExecutionResult<T>>;

  /** Terminate a running execution early. */
  terminate(pluginId: string): Promise<void>;

  /** Dispose of sandbox resources. */
  dispose(): Promise<void>;
}

// ---------------------------------------------------------------------------
// WASM Sandbox (placeholder)
// ---------------------------------------------------------------------------

/**
 * WASM-based sandbox backed by the Rust runtime's WASM host.
 *
 * This is a placeholder that will be wired up to the `runtime/sandbox` crate
 * once the WASM plugin ABI is finalised.
 */
export class WasmSandbox implements PluginSandbox {
  private readonly config: SandboxConfig;

  constructor(config: Partial<SandboxConfig> = {}) {
    this.config = { ...DEFAULT_SANDBOX_CONFIG, ...config };
  }

  async execute<T = unknown>(
    pluginId: string,
    action: string,
    _input: unknown,
  ): Promise<SandboxExecutionResult<T>> {
    // TODO: integrate with runtime/sandbox WASM host
    return {
      success: false,
      error: `WasmSandbox not yet implemented (plugin=${pluginId}, action=${action}, timeout=${this.config.timeoutMs}ms)`,
      durationMs: 0,
    };
  }

  async terminate(_pluginId: string): Promise<void> {
    // no-op until WASM host is wired
  }

  async dispose(): Promise<void> {
    // no-op until WASM host is wired
  }
}

// ---------------------------------------------------------------------------
// Process Sandbox (placeholder)
// ---------------------------------------------------------------------------

/**
 * Process-based sandbox that runs each plugin in a child process
 * with restricted capabilities.
 *
 * Communication happens over structured IPC (JSON over stdio).
 */
export class ProcessSandbox implements PluginSandbox {
  private readonly config: SandboxConfig;

  constructor(config: Partial<SandboxConfig> = {}) {
    this.config = { ...DEFAULT_SANDBOX_CONFIG, ...config };
  }

  async execute<T = unknown>(
    pluginId: string,
    action: string,
    _input: unknown,
  ): Promise<SandboxExecutionResult<T>> {
    // TODO: spawn child process with restricted capabilities
    return {
      success: false,
      error: `ProcessSandbox not yet implemented (plugin=${pluginId}, action=${action}, timeout=${this.config.timeoutMs}ms)`,
      durationMs: 0,
    };
  }

  async terminate(_pluginId: string): Promise<void> {
    // TODO: kill child process
  }

  async dispose(): Promise<void> {
    // TODO: clean up all child processes
  }
}
