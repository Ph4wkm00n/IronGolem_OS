// route: /audit — typed mock data for the Audit findings page.
// v0.3 Step 7 of Plans/modular-puzzling-blum.md.
//
// Shape mirrors the gateway's `audit.StoredFinding` JSON wire format so
// the page renders identically when VITE_API_MODE_AUDIT=real flips on.

export type AuditSeverity = "info" | "warning" | "critical";

export interface AuditFinding {
  readonly id: string;
  readonly probe_id: string;
  readonly severity: AuditSeverity;
  readonly reason: string;
  readonly evidence?: Record<string, unknown>;
  readonly timestamp: string; // RFC3339Nano
  readonly stored_at: string;
}

const NOW = "2026-05-19T14:00:00Z";

export const mockAuditFindings: readonly AuditFinding[] = [
  {
    id: "f-001",
    probe_id: "trust_model",
    severity: "info",
    reason: "HMAC secret loaded, trust foundation intact",
    evidence: { env_var: "IRONGOLEM_HMAC_SECRET", secret_bytes: 64 },
    timestamp: NOW,
    stored_at: NOW,
  },
  {
    id: "f-002",
    probe_id: "connector_health_drift",
    severity: "warning",
    reason: "1 of 3 connectors failed environment readiness",
    evidence: {
      failing: [
        {
          type: "email",
          label: "Email (IMAP/SMTP)",
          required_env: ["IRONGOLEM_EMAIL_IMAP_HOST"],
        },
      ],
      passing: ["telegram", "webhook"],
      total: 3,
    },
    timestamp: NOW,
    stored_at: NOW,
  },
  {
    id: "f-003",
    probe_id: "channel_dm_policy",
    severity: "info",
    reason: "all channel policy rules resolve to registered connectors",
    evidence: { total_rules: 0, known_connectors: ["telegram", "email", "webhook"] },
    timestamp: NOW,
    stored_at: NOW,
  },
  {
    id: "f-004",
    probe_id: "workspace_skill_escape",
    severity: "info",
    reason: "no skill system in v0.3 — probe passes vacuously",
    evidence: { skill_system_present: false, v03_status: "placeholder" },
    timestamp: NOW,
    stored_at: NOW,
  },
];
