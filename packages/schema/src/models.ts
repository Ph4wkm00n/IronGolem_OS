/**
 * Core domain models shared between frontend and backend.
 */

/* ------------------------------------------------------------------ */
/*  Enums / unions                                                     */
/* ------------------------------------------------------------------ */

/** Agent roles in the system. */
export type AgentRole =
  | "planner"
  | "executor"
  | "verifier"
  | "researcher"
  | "defender"
  | "healer"
  | "optimizer"
  | "narrator"
  | "router";

/** Human-friendly labels for agent roles (beginner mode). */
export const agentRoleLabel: Record<AgentRole, string> = {
  planner: "Planner",
  executor: "Doer",
  verifier: "Checker",
  researcher: "Researcher",
  defender: "Security Guard",
  healer: "Auto-Fixer",
  optimizer: "Improver",
  narrator: "Narrator",
  router: "Router",
};

/** Deployment modes matching Rust `DeploymentMode`. */
export type DeploymentMode = "solo" | "household" | "team";

/** Risk levels for actions and recipes. */
export type RiskLevel = "low" | "medium" | "high" | "critical";

/** Human-friendly risk labels. */
export const riskLevelLabel: Record<RiskLevel, string> = {
  low: "Low risk",
  medium: "Medium risk",
  high: "High risk",
  critical: "Critical risk",
};

/** Plan execution status. */
export type PlanStatus =
  | "draft"
  | "pending-approval"
  | "running"
  | "paused"
  | "completed"
  | "failed"
  | "rolled-back";

/* ------------------------------------------------------------------ */
/*  Models                                                             */
/* ------------------------------------------------------------------ */

/** A single node in an execution plan graph. */
export interface PlanNode {
  readonly id: string;
  readonly label: string;
  readonly agentRole: AgentRole;
  readonly status: PlanStatus;
  readonly riskLevel: RiskLevel;
  /** IDs of predecessor nodes. */
  readonly dependsOn: readonly string[];
  /** Human-readable summary of what this step does. */
  readonly summary: string;
  /** Duration in milliseconds, if completed. */
  readonly durationMs?: number;
  /** Error message, if failed. */
  readonly error?: string;
}

/** Top-level execution plan. */
export interface Plan {
  readonly id: string;
  readonly workspaceId: string;
  readonly title: string;
  readonly status: PlanStatus;
  readonly nodes: readonly PlanNode[];
  readonly createdAt: string;
  readonly updatedAt: string;
  /** Checkpoint ID for rollback. */
  readonly checkpointId?: string;
}

/** A recipe — user-facing automation template. */
export interface Recipe {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly category: string;
  readonly riskLevel: RiskLevel;
  readonly isActive: boolean;
  /** What this recipe can access. */
  readonly canAccess: readonly string[];
  /** What this recipe cannot access. */
  readonly cannotAccess: readonly string[];
  /** Actions that need explicit approval. */
  readonly needsApprovalFor: readonly string[];
  /** Automatic stop conditions. */
  readonly stopsIf: readonly string[];
  /** Pre-composed squad used by this recipe, if any. */
  readonly squadId?: string;
  readonly createdAt: string;
  readonly updatedAt: string;
}

/** Pre-composed multi-agent team. */
export interface Squad {
  readonly id: string;
  readonly name: string;
  /** User-friendly label (e.g., "Inbox team" rather than "Inbox Squad"). */
  readonly displayName: string;
  readonly description: string;
  readonly roles: readonly AgentRole[];
  readonly status: "active" | "idle" | "paused";
  readonly activeRecipeCount: number;
}

/** An approval request waiting for user action. */
export interface ApprovalRequest {
  readonly id: string;
  readonly planNodeId: string;
  readonly summary: string;
  readonly riskLevel: RiskLevel;
  readonly requestedAt: string;
  readonly agentRole: AgentRole;
  readonly status: "pending" | "approved" | "denied";
}

/** A tracked research topic. */
export interface ResearchTopic {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly confidence: number;
  readonly freshness: "fresh" | "aging" | "stale";
  readonly sourceCount: number;
  readonly hasContradictions: boolean;
  readonly lastUpdated: string;
}

/** A memory entry in the knowledge graph. */
export interface MemoryEntry {
  readonly id: string;
  readonly content: string;
  readonly category: string;
  readonly source: string;
  readonly confidence: number;
  readonly createdAt: string;
  readonly updatedAt: string;
  readonly connections: readonly string[];
}
