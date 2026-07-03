/**
 * Plugin permission vocabulary.
 *
 * v0.4 adoption wave. Locks in the closed resource/action vocabulary a
 * plugin manifest may request BEFORE the plugin loader lands (the same
 * lock-the-contract-early move v0.3 made with hook decisions). Informed
 * by the openclaw install-scan study (2026-07): supply-chain defense
 * starts with a manifest whose permission surface is enumerable —
 * free-text permissions can't be reviewed, diffed, or gated.
 *
 * Mirrored in Rust at `runtime/core/src/plugin.rs`. The two lists MUST
 * stay label-identical; both sides carry a sync test (the hook-decision
 * pattern from v0.3 Step 2).
 */

/** Every resource a plugin permission may name. */
export const PLUGIN_PERMISSION_RESOURCES = [
  "connectors",
  "events",
  "llm",
  "storage",
  "network",
  "commitments",
  "audit",
  "ui",
] as const;

export type PluginPermissionResource = (typeof PLUGIN_PERMISSION_RESOURCES)[number];

/** Every action a plugin permission may request on a resource. */
export const PLUGIN_PERMISSION_ACTIONS = ["read", "write", "execute", "subscribe"] as const;

export type PluginPermissionAction = (typeof PLUGIN_PERMISSION_ACTIONS)[number];

/** One declared permission: a resource plus the actions needed on it. */
export interface PluginPermissionDeclaration {
  readonly resource: PluginPermissionResource;
  readonly actions: readonly PluginPermissionAction[];
}

/** A single validation problem, human-readable and stable enough to test. */
export interface PluginPermissionIssue {
  readonly resource: string;
  readonly problem: string;
}

/**
 * Validate a raw permissions list against the closed vocabulary.
 * Returns one issue per problem; an empty array means valid. Unknown
 * resources and unknown actions are rejected — never silently allowed —
 * per the trust-before-power rule (least privilege by default).
 */
export function validatePluginPermissions(
  permissions: ReadonlyArray<{ resource: string; actions: readonly string[] }>,
): PluginPermissionIssue[] {
  const issues: PluginPermissionIssue[] = [];
  const knownResources = new Set<string>(PLUGIN_PERMISSION_RESOURCES);
  const knownActions = new Set<string>(PLUGIN_PERMISSION_ACTIONS);
  const seen = new Set<string>();

  for (const perm of permissions) {
    if (!knownResources.has(perm.resource)) {
      issues.push({
        resource: perm.resource,
        problem: `unknown resource "${perm.resource}"; must be one of: ${PLUGIN_PERMISSION_RESOURCES.join(", ")}`,
      });
      continue;
    }
    if (seen.has(perm.resource)) {
      issues.push({ resource: perm.resource, problem: "duplicate resource declaration" });
      continue;
    }
    seen.add(perm.resource);
    if (!perm.actions || perm.actions.length === 0) {
      issues.push({ resource: perm.resource, problem: "at least one action is required" });
      continue;
    }
    for (const action of perm.actions) {
      if (!knownActions.has(action)) {
        issues.push({
          resource: perm.resource,
          problem: `unknown action "${action}"; must be one of: ${PLUGIN_PERMISSION_ACTIONS.join(", ")}`,
        });
      }
    }
  }
  return issues;
}
