/**
 * Event types mirroring the Rust event-sourcing domain.
 *
 * These are the TypeScript projections of events emitted by the Rust
 * runtime and consumed by the frontend via the Go gateway.
 */

/** Heartbeat states — mirrors `runtime::core::HeartbeatStatus`. */
export type HeartbeatStatus =
  | "healthy"
  | "quietly-recovering"
  | "needs-attention"
  | "paused"
  | "quarantined";

/** Human-friendly labels for heartbeat statuses (beginner mode). */
export const heartbeatLabel: Record<HeartbeatStatus, string> = {
  healthy: "All good",
  "quietly-recovering": "Fixing itself",
  "needs-attention": "Needs your attention",
  paused: "Paused",
  quarantined: "Isolated for safety",
};

/** Discriminated-union tag for every event the system can emit. */
export type EventKind =
  | "action-taken"
  | "action-proposed"
  | "action-blocked"
  | "action-healed"
  | "action-quarantined"
  | "heartbeat"
  | "research-update"
  | "squad-handoff"
  | "approval-requested"
  | "approval-granted"
  | "approval-denied"
  | "policy-violation"
  | "recipe-activated"
  | "recipe-deactivated"
  | "memory-updated"
  | "checkpoint-created";

/** Base event envelope — every persisted event has these fields. */
export interface Event<K extends EventKind = EventKind, P = unknown> {
  /** Globally unique event ID (UUID v7). */
  readonly id: string;
  /** Monotonically increasing sequence within a stream. */
  readonly sequence: number;
  /** ISO-8601 timestamp. */
  readonly timestamp: string;
  /** Discriminator. */
  readonly kind: K;
  /** Tenant / workspace scope. */
  readonly workspaceId: string;
  /** Optional correlation ID linking related events. */
  readonly correlationId?: string;
  /** Agent that produced this event, if applicable. */
  readonly agentRole?: string;
  /** Type-safe payload. */
  readonly payload: P;
}

/* ------------------------------------------------------------------ */
/*  Typed event payloads                                               */
/* ------------------------------------------------------------------ */

export interface ActionTakenPayload {
  readonly planNodeId: string;
  readonly summary: string;
  readonly durationMs: number;
}

export interface ActionProposedPayload {
  readonly planNodeId: string;
  readonly summary: string;
  readonly riskLevel: string;
}

export interface ActionBlockedPayload {
  readonly planNodeId: string;
  readonly reason: string;
  readonly policyLayer: string;
}

export interface HeartbeatPayload {
  readonly status: HeartbeatStatus;
  readonly message: string;
  readonly uptimeSeconds: number;
}

export interface ResearchUpdatePayload {
  readonly topicId: string;
  readonly title: string;
  readonly confidence: number;
  readonly sourceCount: number;
  readonly hasContradictions: boolean;
}

export interface SquadHandoffPayload {
  readonly fromSquad: string;
  readonly toSquad: string;
  readonly reason: string;
}

/* ------------------------------------------------------------------ */
/*  Convenience aliases                                                */
/* ------------------------------------------------------------------ */

export type ActionTakenEvent = Event<"action-taken", ActionTakenPayload>;
export type ActionProposedEvent = Event<"action-proposed", ActionProposedPayload>;
export type ActionBlockedEvent = Event<"action-blocked", ActionBlockedPayload>;
export type HeartbeatEvent = Event<"heartbeat", HeartbeatPayload>;
export type ResearchUpdateEvent = Event<"research-update", ResearchUpdatePayload>;
export type SquadHandoffEvent = Event<"squad-handoff", SquadHandoffPayload>;
