// route: /commitments — typed mock data for the Commitments page.
// Consumed via `api.v2.commitments.getMock()`; never imported by pages
// directly. v0.3 Step 7 of Plans/modular-puzzling-blum.md.
//
// Shape mirrors the gateway's `commitments.Commitment` JSON shape so the
// page renders identically when VITE_API_MODE_COMMITMENTS=real flips on.

export type CommitmentKind =
  | "event_check_in"
  | "deadline_check"
  | "care_check_in"
  | "open_loop";

export type CommitmentSensitivity = "routine" | "personal" | "care";

export type CommitmentStatus =
  | "pending"
  | "sent"
  | "dismissed"
  | "snoozed"
  | "expired";

export interface DueWindow {
  readonly earliest_ms: number;
  readonly latest_ms: number;
  readonly timezone?: string;
}

export interface Commitment {
  readonly id: string;
  readonly workspace_id: string;
  readonly tenant_id: string;
  readonly kind: CommitmentKind;
  readonly sensitivity: CommitmentSensitivity;
  readonly status: CommitmentStatus;
  readonly reason: string;
  readonly suggested_text: string;
  readonly dedupe_key: string;
  readonly confidence: number;
  readonly due_window: DueWindow;
  readonly connector_id?: string;
  readonly channel_id?: string;
  readonly user_id?: string;
  readonly source_event_id?: string;
  readonly created_at_ms: number;
  readonly updated_at_ms: number;
  readonly sent_at_ms?: number;
  readonly dismissed_at_ms?: number;
  readonly snoozed_until_ms?: number;
  readonly expired_at_ms?: number;
  readonly attempts: number;
}

// Use a frozen `now` so the mock window deltas stay deterministic
// across renders + visual snapshots.
const NOW = Date.parse("2026-05-19T14:00:00Z");
const H = 60 * 60 * 1000;
const M = 60 * 1000;
const D = 24 * H;

export const mockCommitments: readonly Commitment[] = [
  {
    id: "c-001",
    workspace_id: "00000000-0000-0000-0000-000000000000",
    tenant_id: "default",
    kind: "event_check_in",
    sensitivity: "routine",
    status: "pending",
    reason: "user mentioned a relative time window",
    suggested_text: "Reminder: in 2 hours",
    dedupe_key: "abc123",
    confidence: 0.85,
    due_window: { earliest_ms: NOW + 110 * M, latest_ms: NOW + 130 * M },
    connector_id: "telegram",
    channel_id: "chat-42",
    created_at_ms: NOW - 5 * M,
    updated_at_ms: NOW - 5 * M,
    attempts: 0,
  },
  {
    id: "c-002",
    workspace_id: "00000000-0000-0000-0000-000000000000",
    tenant_id: "default",
    kind: "care_check_in",
    sensitivity: "care",
    status: "pending",
    reason: "assistant promised to check in on wellbeing",
    suggested_text: "Just checking in — how are you feeling today?",
    dedupe_key: "def456",
    confidence: 0.9,
    due_window: { earliest_ms: NOW + 1 * D, latest_ms: NOW + 1 * D + 2 * H },
    connector_id: "telegram",
    channel_id: "chat-42",
    created_at_ms: NOW - 2 * H,
    updated_at_ms: NOW - 2 * H,
    attempts: 0,
  },
  {
    id: "c-003",
    workspace_id: "00000000-0000-0000-0000-000000000000",
    tenant_id: "default",
    kind: "deadline_check",
    sensitivity: "personal",
    status: "sent",
    reason: "tax filing deadline reminder",
    suggested_text: "Reminder: tax filing deadline is in 24 hours.",
    dedupe_key: "ghi789",
    confidence: 0.95,
    due_window: { earliest_ms: NOW - 30 * M, latest_ms: NOW + 30 * M },
    connector_id: "telegram",
    channel_id: "chat-42",
    created_at_ms: NOW - 4 * D,
    updated_at_ms: NOW - 10 * M,
    sent_at_ms: NOW - 10 * M,
    attempts: 1,
  },
  {
    id: "c-004",
    workspace_id: "00000000-0000-0000-0000-000000000000",
    tenant_id: "default",
    kind: "open_loop",
    sensitivity: "routine",
    status: "dismissed",
    reason: "open-loop follow-up on contract review",
    suggested_text: "Following up on the contract review",
    dedupe_key: "jkl012",
    confidence: 0.65,
    due_window: { earliest_ms: NOW - 5 * H, latest_ms: NOW - 1 * H },
    connector_id: "telegram",
    channel_id: "chat-42",
    created_at_ms: NOW - 3 * D,
    updated_at_ms: NOW - 1 * D,
    dismissed_at_ms: NOW - 1 * D,
    attempts: 0,
  },
];
