// route: /security — typed mock data for the Security page.
// Consumed via `api.v2.security.getMock()`; never imported by pages directly.
// TODO: align with @irongolem/schema (PolicyEvaluation + audit event models).

export type LayerId = 1 | 2 | 3 | 4 | 5;
export type LayerState = "ok" | "watching" | "paused" | "failed";
export type AuditKind = "blocked" | "quarantined";
export type Scope = "scoped" | "broad" | "restricted";
export type PolicyState = "active" | "paused" | "under-review";

export interface Layer {
  readonly id: LayerId;
  readonly name: string;
  readonly state: LayerState;
  readonly blurb: string;
  readonly governs: number;
  readonly examples: readonly string[];
}

export interface AuditEntry {
  readonly id: string;
  readonly when: string;
  readonly whenIso: string;
  readonly kind: AuditKind;
  readonly title: string;
  readonly cause: string;
  readonly permission: string;
  readonly scope: Scope;
  readonly layer: LayerId;
  readonly ruleId: string;
  readonly ruleName: string;
  readonly deniedBy: "system" | "you";
  readonly details: string;
}

export interface PolicyHistoryEntry {
  readonly when: string;
  readonly what: string;
}

export interface Policy {
  readonly id: string;
  readonly name: string;
  readonly purpose: string;
  state: PolicyState;
  readonly layer: LayerId;
  readonly ruleText: string;
  readonly triggeredLast30d: number;
  readonly history: readonly PolicyHistoryEntry[];
  readonly relatedAuditIds: readonly string[];
}

export const mockLayers: readonly Layer[] = [
  { id: 1, name: "Identity", state: "ok", governs: 47, blurb: "Who is asking — operator, team, or external sender — and whether their session is current.", examples: ["Session valid, MFA satisfied", "Sender domain on the allowlist"] },
  { id: 2, name: "Workspace", state: "ok", governs: 31, blurb: "Workspace-wide guardrails: quiet hours, data residency, environment (sandbox vs production).", examples: ["Quiet hours 23:00–06:00 PT", "Data residency: US-only"] },
  { id: 3, name: "Team", state: "watching", governs: 18, blurb: "Per-team limits on what an assistant team can do without you. Inbox can draft; Operations needs sign-off above $50.", examples: ["Operations auto-approve cap $50", "Inbox: drafts only, no sends"] },
  { id: 4, name: "Action", state: "ok", governs: 64, blurb: "Per-action checks: tools required, allowed APIs, side effects, rate limits.", examples: ["Wire transfers: restricted", "External email send: scoped per thread"] },
  { id: 5, name: "Outcome", state: "ok", governs: 22, blurb: "Post-hoc checks on what changed. Bad outcomes trigger rollback and quarantine.", examples: ["Diff size limit on docs", "Auto-rollback on PII leak"] },
];

export const mockPoliciesInitial: readonly Policy[] = [
  { id: "pol-01", name: "Standing-order purchases ≤ $50 auto-approve", purpose: "Lets routine purchases under $50 go through without your sign-off.", state: "active", layer: 3, ruleText: "when action == 'purchase.create' and amount.usd <= 50 and vendor.in standing_orders\n  then allow with audit", triggeredLast30d: 18, history: [{ when: "May 5, 2026", what: "Threshold raised from $40 → $50 (you)" }, { when: "Mar 10, 2026", what: "Policy created" }], relatedAuditIds: ["a04"] },
  { id: "pol-02", name: "Quiet hours: drafts only, no sends", purpose: "Pauses outbound sends between 11pm and 6am PT. Drafts continue.", state: "active", layer: 2, ruleText: "when local_time.in 23:00..06:00\n  then block 'send.external' and allow 'draft.create'", triggeredLast30d: 7, history: [{ when: "Mar 18, 2026", what: "Policy created" }], relatedAuditIds: ["a02", "a17"] },
  { id: "pol-03", name: "Wire transfers require restricted scope", purpose: "Any wire transfer must be initiated by you and use a restricted-scope token.", state: "active", layer: 4, ruleText: "when action == 'wire.transfer'\n  then require token.scope == 'restricted'\n   and require initiator == 'operator'", triggeredLast30d: 2, history: [{ when: "Feb 2, 2026", what: "Tightened: now requires operator initiation" }, { when: "Jan 5, 2026", what: "Policy created" }], relatedAuditIds: ["a09", "a23"] },
  { id: "pol-04", name: "External email: scoped per thread", purpose: "Drafting team can reply within an existing thread but cannot start a new external thread without you.", state: "active", layer: 4, ruleText: "when action == 'email.send.external'\n  then require thread.parent_id != null", triggeredLast30d: 5, history: [{ when: "Apr 14, 2026", what: "Policy created" }], relatedAuditIds: ["a07", "a11"] },
  { id: "pol-05", name: "Diff-size cap on docs (Outcome)", purpose: "Auto-rolls back any document edit that changes more than 40% in a single revision.", state: "under-review", layer: 5, ruleText: "when action == 'doc.edit' and diff.pct > 0.40\n  then rollback and quarantine", triggeredLast30d: 3, history: [{ when: "May 9, 2026", what: "Marked under-review — 3 false positives this month" }, { when: "Jan 22, 2026", what: "Policy created" }], relatedAuditIds: ["a13"] },
  { id: "pol-06", name: "Known-sender allowlist", purpose: "Auto-archive triage-only mail from 14 known senders.", state: "active", layer: 1, ruleText: "when email.from.in known_senders\n  then route 'triage.archive'", triggeredLast30d: 142, history: [{ when: "Feb 1, 2026", what: "Policy created" }], relatedAuditIds: [] },
  { id: "pol-07", name: "PII leak detector → quarantine", purpose: "Quarantines any draft containing apparent SSN or full credit-card numbers.", state: "active", layer: 5, ruleText: "when draft.contains(pii.ssn | pii.cc)\n  then quarantine and notify", triggeredLast30d: 1, history: [{ when: "Dec 1, 2025", what: "Policy created" }], relatedAuditIds: ["a15"] },
  { id: "pol-08", name: "Telegram connector — paused while re-auth pending", purpose: "Holds outbound Telegram sends until the bot session is re-authorized.", state: "paused", layer: 4, ruleText: "when connector.telegram.session.invalid\n  then pause connector.telegram.outbound", triggeredLast30d: 4, history: [{ when: "yesterday", what: "Auto-paused after upstream session invalidation" }, { when: "Nov 12, 2025", what: "Policy created" }], relatedAuditIds: ["a01", "a08"] },
];

export const mockAudit: readonly AuditEntry[] = [
  { id: "a01", when: "1h ago", whenIso: "11:02 PT", kind: "quarantined", deniedBy: "system", title: "Telegram outbound batch held", cause: "Bot session was invalidated upstream and outbound retries would have hit a dead endpoint.", permission: "connector.telegram.send", scope: "scoped", layer: 4, ruleId: "pol-08", ruleName: "Telegram connector — paused while re-auth pending", details: "4 messages currently queued. The connector quarantined itself and will resume when you re-authorize." },
  { id: "a02", when: "2h ago", whenIso: "10:14 PT", kind: "blocked", deniedBy: "system", title: "Drafting team tried to send during quiet hours", cause: "Local time was inside the configured quiet-hours window (23:00–06:00 PT) and the policy allows drafts but blocks sends.", permission: "email.send.external", scope: "scoped", layer: 2, ruleId: "pol-02", ruleName: "Quiet hours: drafts only, no sends", details: "The draft is preserved and queued for 06:01 PT delivery." },
  { id: "a03", when: "3h ago", whenIso: "08:48 PT", kind: "blocked", deniedBy: "you", title: "You denied an auto-approve on a $612 standing order", cause: "Amount was above the $50 auto-approve cap and you reviewed it manually.", permission: "purchase.create", scope: "scoped", layer: 3, ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve", details: "Order PO-26-118 routed to your inbox; you marked it 'review next week'." },
  { id: "a04", when: "5h ago", whenIso: "06:48 PT", kind: "blocked", deniedBy: "system", title: "Auto-approve denied: vendor not on standing-orders list", cause: "Vendor 'Brisbane Imports' is not in the standing-orders list, so the auto-approve rule did not apply.", permission: "purchase.create", scope: "scoped", layer: 3, ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve", details: "Routed to your inbox for one-tap approval." },
  { id: "a05", when: "yesterday", whenIso: "May 11 21:02 PT", kind: "blocked", deniedBy: "system", title: "Webhook receiver throttled by policy", cause: "Receiver p95 exceeded 4s for 3 consecutive checks; the rate-limit policy reduced concurrency.", permission: "webhook.send", scope: "broad", layer: 4, ruleId: "pol-04", ruleName: "External email: scoped per thread", details: "Concurrency dropped 4 → 1 for 90 seconds; receiver recovered without operator action." },
  { id: "a06", when: "yesterday", whenIso: "May 11 16:01 PT", kind: "blocked", deniedBy: "system", title: "Auth probe failed: failed over to backup provider", cause: "Primary auth provider returned 502 on 8/10 probes within 60 seconds.", permission: "identity.session.refresh", scope: "restricted", layer: 1, ruleId: "pol-06", ruleName: "Known-sender allowlist", details: "Failover lasted 11 minutes; primary recovered and we re-pinned." },
  { id: "a07", when: "yesterday", whenIso: "May 11 15:18 PT", kind: "blocked", deniedBy: "system", title: "Drafting team tried to start a new external thread", cause: "Action lacked a parent thread id; per policy, the team can reply within threads but not start new ones.", permission: "email.send.external", scope: "scoped", layer: 4, ruleId: "pol-04", ruleName: "External email: scoped per thread", details: "Draft saved for your review at /inbox." },
  { id: "a08", when: "yesterday", whenIso: "May 11 14:02 PT", kind: "quarantined", deniedBy: "system", title: "Telegram session quarantined after upstream change", cause: "Upstream client 5.1.4 invalidated the long-lived bot session, so the connector quarantined itself.", permission: "connector.telegram.session", scope: "restricted", layer: 4, ruleId: "pol-08", ruleName: "Telegram connector — paused while re-auth pending", details: "Quarantine prevents retry churn until you re-authorize." },
  { id: "a09", when: "2d ago", whenIso: "May 10 10:32 PT", kind: "blocked", deniedBy: "you", title: "You denied a wire transfer initiated by a recipe", cause: "Wire transfers require operator initiation; the recipe attempted to act on your behalf.", permission: "wire.transfer", scope: "restricted", layer: 4, ruleId: "pol-03", ruleName: "Wire transfers require restricted scope", details: "The recipe is paused pending your review." },
  { id: "a10", when: "2d ago", whenIso: "May 10 09:11 PT", kind: "blocked", deniedBy: "system", title: "Sender domain not on allowlist", cause: "Inbound mail came from a domain that does not match any allowlisted sender pattern.", permission: "identity.sender.verify", scope: "scoped", layer: 1, ruleId: "pol-06", ruleName: "Known-sender allowlist", details: "Routed to triage for your manual review." },
  { id: "a11", when: "2d ago", whenIso: "May 10 08:02 PT", kind: "blocked", deniedBy: "system", title: "New thread denied: lacking parent", cause: "External-email policy requires a parent thread id and one was not present.", permission: "email.send.external", scope: "scoped", layer: 4, ruleId: "pol-04", ruleName: "External email: scoped per thread", details: "Operator can override; draft preserved." },
  { id: "a12", when: "3d ago", whenIso: "May 9 19:48 PT", kind: "quarantined", deniedBy: "system", title: "Document revision quarantined: 62% diff", cause: "A single revision changed 62% of the document, which exceeds the diff-size cap.", permission: "doc.edit", scope: "broad", layer: 5, ruleId: "pol-05", ruleName: "Diff-size cap on docs (Outcome)", details: "Auto-rollback restored the previous revision; quarantined copy held in review." },
  { id: "a13", when: "3d ago", whenIso: "May 9 14:22 PT", kind: "quarantined", deniedBy: "system", title: "Outcome check rolled back doc edit (false positive flagged)", cause: "Diff exceeded the policy threshold, though human review later marked the edit safe.", permission: "doc.edit", scope: "broad", layer: 5, ruleId: "pol-05", ruleName: "Diff-size cap on docs (Outcome)", details: "Marked false positive — contributes to the policy's under-review status." },
  { id: "a14", when: "4d ago", whenIso: "May 8 16:18 PT", kind: "blocked", deniedBy: "system", title: "Auto-approve denied: amount $54 exceeded cap", cause: "Amount of $54 was $4 above the standing-order auto-approve threshold.", permission: "purchase.create", scope: "scoped", layer: 3, ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve", details: "Routed for one-tap approval; you approved 14 minutes later." },
  { id: "a15", when: "5d ago", whenIso: "May 7 11:05 PT", kind: "quarantined", deniedBy: "system", title: "Draft quarantined: matched PII pattern", cause: "Draft contained a 9-digit sequence that matched a US SSN pattern.", permission: "draft.send", scope: "restricted", layer: 5, ruleId: "pol-07", ruleName: "PII leak detector → quarantine", details: "Marked false positive after review (sequence was an order number); rule kept as-is." },
  { id: "a16", when: "6d ago", whenIso: "May 6 09:38 PT", kind: "blocked", deniedBy: "system", title: "Data-residency policy blocked outbound transfer", cause: "Destination region was outside the US, which violates the workspace residency rule.", permission: "data.export.external", scope: "restricted", layer: 2, ruleId: "pol-02", ruleName: "Quiet hours: drafts only, no sends", details: "Destination overridden by the operator after review." },
  { id: "a17", when: "7d ago", whenIso: "May 5 23:48 PT", kind: "blocked", deniedBy: "system", title: "Send blocked: quiet hours", cause: "Time was 23:48 PT, within the configured quiet-hours window.", permission: "email.send.external", scope: "scoped", layer: 2, ruleId: "pol-02", ruleName: "Quiet hours: drafts only, no sends", details: "Draft preserved; queued for 06:01 delivery." },
  { id: "a18", when: "8d ago", whenIso: "May 4 14:11 PT", kind: "blocked", deniedBy: "you", title: "You denied a calendar override for an executive 1:1", cause: "The recipe tried to move a meeting flagged 'do not move'; you reviewed and denied.", permission: "calendar.event.move", scope: "broad", layer: 3, ruleId: "pol-01", ruleName: "Standing-order purchases ≤ $50 auto-approve", details: "Recipe paused; rule will be reviewed Friday." },
  { id: "a19", when: "9d ago", whenIso: "May 3 17:00 PT", kind: "blocked", deniedBy: "system", title: "Inbox tried to send (drafts-only policy)", cause: "Inbox team has draft-only scope and cannot send externally.", permission: "email.send.external", scope: "scoped", layer: 3, ruleId: "pol-04", ruleName: "External email: scoped per thread", details: "Draft routed to /inbox for your review." },
  { id: "a20", when: "10d ago", whenIso: "May 2 12:34 PT", kind: "quarantined", deniedBy: "system", title: "Outbound link in draft pointed to a flagged domain", cause: "A linked domain matched the reputation blocklist; draft was held pending operator review.", permission: "email.send.external", scope: "scoped", layer: 5, ruleId: "pol-07", ruleName: "PII leak detector → quarantine", details: "You released the draft after confirming the domain." },
  { id: "a21", when: "12d ago", whenIso: "Apr 30 09:22 PT", kind: "blocked", deniedBy: "system", title: "Broad-scope token rejected for restricted action", cause: "Action required a restricted-scope token; only a broad-scope token was offered.", permission: "wire.transfer", scope: "restricted", layer: 4, ruleId: "pol-03", ruleName: "Wire transfers require restricted scope", details: "Recipe paused. You can grant a restricted token from Settings." },
  { id: "a22", when: "14d ago", whenIso: "Apr 28 16:02 PT", kind: "blocked", deniedBy: "you", title: "You denied an attachment auto-share", cause: "Attachment looked sensitive; you held the share for manual review.", permission: "doc.share.external", scope: "scoped", layer: 4, ruleId: "pol-04", ruleName: "External email: scoped per thread", details: "Marked for redaction; later shared with PII removed." },
  { id: "a23", when: "15d ago", whenIso: "Apr 27 11:00 PT", kind: "blocked", deniedBy: "system", title: "Wire transfer blocked: not operator-initiated", cause: "Wire transfers require operator initiation; the request came from an internal recipe.", permission: "wire.transfer", scope: "restricted", layer: 4, ruleId: "pol-03", ruleName: "Wire transfers require restricted scope", details: "Recipe paused; operator review pending." },
  { id: "a24", when: "21d ago", whenIso: "Apr 21 13:48 PT", kind: "quarantined", deniedBy: "system", title: "Outcome rollback: doc edit changed unrelated section", cause: "Edit touched a section outside the requested scope, which exceeded the action's permission.", permission: "doc.edit", scope: "broad", layer: 5, ruleId: "pol-05", ruleName: "Diff-size cap on docs (Outcome)", details: "Quarantined copy retained 30 days; revert was clean." },
  { id: "a25", when: "28d ago", whenIso: "Apr 14 08:11 PT", kind: "blocked", deniedBy: "system", title: "Connector access denied: token expired", cause: "OAuth refresh token had expired and no operator session was active to re-issue.", permission: "connector.calendar.read", scope: "scoped", layer: 1, ruleId: "pol-06", ruleName: "Known-sender allowlist", details: "Resolved after you re-authenticated the connector." },
];
