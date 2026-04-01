/**
 * Policy types matching the five-layer security model.
 *
 * Every autonomous action passes through all five layers in order.
 */

/** The five security layers, evaluated top-to-bottom. */
export type PolicyLayer =
  | "gateway-identity"
  | "global-tool-policy"
  | "per-agent-permissions"
  | "per-channel-restrictions"
  | "admin-only-controls";

/** User-facing labels for each layer (no jargon). */
export const policyLayerLabel: Record<PolicyLayer, string> = {
  "gateway-identity": "Identity check",
  "global-tool-policy": "Global safety rules",
  "per-agent-permissions": "Assistant permissions",
  "per-channel-restrictions": "Channel limits",
  "admin-only-controls": "Admin controls",
};

/** Short descriptions for each layer. */
export const policyLayerDescription: Record<PolicyLayer, string> = {
  "gateway-identity":
    "Confirms who is making the request and that they are authenticated.",
  "global-tool-policy":
    "Enforces system-wide rules about which tools can be used and when.",
  "per-agent-permissions":
    "Checks that this specific assistant role is allowed to perform the action.",
  "per-channel-restrictions":
    "Applies any limits set for the channel (e.g., email, Slack) being used.",
  "admin-only-controls":
    "Final gate — only admins can override blocks at this level.",
};

/** The effect a policy evaluation produces. */
export type PolicyEffect = "allow" | "deny" | "require-approval";

/** Result of evaluating a single policy layer. */
export interface PolicyDecision {
  readonly layer: PolicyLayer;
  readonly effect: PolicyEffect;
  readonly reason: string;
  readonly evaluatedAt: string;
}

/** A named permission that can be granted or revoked. */
export interface Permission {
  readonly id: string;
  readonly name: string;
  /** User-friendly description. */
  readonly description: string;
  readonly resource: string;
  readonly action: string;
  readonly effect: PolicyEffect;
  /** Which layer governs this permission. */
  readonly layer: PolicyLayer;
}

/** Aggregated policy evaluation result across all five layers. */
export interface PolicyEvaluation {
  readonly requestId: string;
  readonly decisions: readonly PolicyDecision[];
  /** Final combined effect — deny wins over require-approval wins over allow. */
  readonly finalEffect: PolicyEffect;
  readonly evaluatedAt: string;
}
