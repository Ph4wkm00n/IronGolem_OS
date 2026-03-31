# Testing Strategy

IronGolem OS uses five categories of tests to ensure correctness, security,
reliability, and usability.

## 1. Functional Tests

Verify that core features work as designed.

| Area | What to Test |
|------|-------------|
| Recipe activation | Recipes activate from gallery and execute correctly |
| Approval flows | Approval requests route correctly, approvals/rejections take effect |
| Connector messaging | Messages flow correctly through each connector type |
| Schedule execution | One-time and recurring tasks fire on time |
| Squad delegation | Multi-agent handoffs complete successfully |
| Graph updates | Knowledge graph updates reflect new events accurately |

## 2. Isolation Tests

Verify that multi-tenant boundaries hold.

| Area | What to Test |
|------|-------------|
| Tenant boundaries | No data leaks between tenants |
| Workspace isolation | Connectors scoped to correct workspace |
| Channel policy | Per-channel restrictions enforced correctly |
| Admin-only gating | Privileged actions blocked for non-admin users |

## 3. Reliability Tests

Verify that the system recovers gracefully from failures.

| Area | What to Test |
|------|-------------|
| Connector failures | Inject failures and verify self-healing response |
| Heartbeat timeouts | Simulate missed heartbeats and verify escalation |
| Rollback verification | Force failures and verify rollback restores correct state |
| Stale credentials | Simulate expired tokens and verify credential recovery |

## 4. Security Tests

Verify that security defenses work against known attack patterns.

| Area | What to Test |
|------|-------------|
| Prompt injection | Run injection corpus and verify detection/blocking |
| SSRF attacks | Attempt disallowed destinations and verify blocking |
| Command abuse | Attempt shell commands and verify deny patterns |
| Cross-tenant access | Attempt to access other tenants' data |
| Quarantine flows | Trigger quarantine and verify isolation |

## 5. UX Tests

Verify that the product is understandable by target personas.

| Area | What to Test |
|------|-------------|
| Onboarding success | Non-technical users complete setup without help |
| Policy comprehension | Users correctly interpret policy cards |
| Health interpretation | Users understand heartbeat status meanings |
| Security comprehension | Users understand why actions were blocked |

## Testing Principles

- Every autonomous loop must be tested in shadow mode before live deployment
- Isolation tests run as part of CI for every team-mode change
- Security tests include adversarial scenarios, not just happy paths
- UX tests involve actual users from target persona groups

## Canonical Reference

See [specs/04-implementation-plan-v2.md](../specs/04-implementation-plan-v2.md).
