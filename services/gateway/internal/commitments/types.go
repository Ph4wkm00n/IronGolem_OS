// Package commitments implements the user-facing future-obligation
// tracking subsystem.
//
// v0.3 Step 4 of `Plans/modular-puzzling-blum.md`. Adopts the data model
// from openclaw/openclaw `src/commitments/`. Distinct from heartbeats
// (which model runtime *health* states) — commitments model future
// promises the assistant made to the user ("I'll remind you Tuesday
// at 6pm to call Mom") and the lifecycle of those promises through
// fire/dismiss/snooze/expire.
//
// Lifecycle, end-to-end:
//
//  1. Assistant turn closes → `Extractor.Extract(user, assistant)`
//     returns candidate commitments.
//  2. Dedup against existing pending via DedupeKey, threshold-gate on
//     Confidence, persist survivors.
//  3. Runtime ticker (`runtime.go`, 60s cadence) scans pending
//     commitments. Any with `EarliestMs <= now <= LatestMs` fires —
//     outbound message via the connector manager, status → sent,
//     `commitment.fired` event.
//  4. Past-due-without-firing (now > LatestMs + grace) → expired.
//  5. User actions: dismiss (don't fire), snooze (push the window).
package commitments

import "time"

// CommitmentKind classifies what the assistant promised.
//
// event_check_in   — promise to follow up at a scheduled time
//
//	("I'll text you when the meeting starts")
//
// deadline_check   — promise to remind before a deadline
//
//	("I'll nudge you Friday about the tax filing")
//
// care_check_in    — emotional/wellbeing follow-up
//
//	("I'll check on you tomorrow")
//
// open_loop        — unresolved task with no specific time
//
//	("I'll keep an eye on the contract review")
type CommitmentKind string

const (
	KindEventCheckIn  CommitmentKind = "event_check_in"
	KindDeadlineCheck CommitmentKind = "deadline_check"
	KindCareCheckIn   CommitmentKind = "care_check_in"
	KindOpenLoop      CommitmentKind = "open_loop"
)

// Valid reports whether k is one of the canonical kinds.
func (k CommitmentKind) Valid() bool {
	switch k {
	case KindEventCheckIn, KindDeadlineCheck, KindCareCheckIn, KindOpenLoop:
		return true
	}
	return false
}

// CommitmentSensitivity controls how the firing UX treats the
// commitment. Higher sensitivity → richer notification path (e.g. a
// `care` commitment may push to a different channel than `routine`).
type CommitmentSensitivity string

const (
	SensitivityRoutine  CommitmentSensitivity = "routine"
	SensitivityPersonal CommitmentSensitivity = "personal"
	SensitivityCare     CommitmentSensitivity = "care"
)

// Valid reports whether s is one of the canonical sensitivities.
func (s CommitmentSensitivity) Valid() bool {
	switch s {
	case SensitivityRoutine, SensitivityPersonal, SensitivityCare:
		return true
	}
	return false
}

// CommitmentStatus tracks lifecycle. Forward-only except for
// snoozed → pending when the snooze window passes.
type CommitmentStatus string

const (
	StatusPending   CommitmentStatus = "pending"
	StatusSent      CommitmentStatus = "sent"
	StatusDismissed CommitmentStatus = "dismissed"
	StatusSnoozed   CommitmentStatus = "snoozed"
	StatusExpired   CommitmentStatus = "expired"
)

// Valid reports whether s is one of the canonical statuses.
func (s CommitmentStatus) Valid() bool {
	switch s {
	case StatusPending, StatusSent, StatusDismissed, StatusSnoozed, StatusExpired:
		return true
	}
	return false
}

// DueWindow describes the time range during which the commitment is
// eligible to fire. The runtime fires at the earliest tick where
// `EarliestMs <= now <= LatestMs`; outside the window it stays pending
// (before EarliestMs) or expires (after LatestMs + grace).
type DueWindow struct {
	EarliestMs int64  `json:"earliest_ms"`
	LatestMs   int64  `json:"latest_ms"`
	Timezone   string `json:"timezone,omitempty"`
}

// Earliest converts EarliestMs to a time.Time in UTC. Used by the
// runtime's tick scanner.
func (w DueWindow) Earliest() time.Time {
	return time.UnixMilli(w.EarliestMs).UTC()
}

// Latest converts LatestMs to a time.Time in UTC.
func (w DueWindow) Latest() time.Time {
	return time.UnixMilli(w.LatestMs).UTC()
}

// Commitment is the canonical record persisted in `gateway_commitments`.
//
// Fields ending in `Ms` are unix-millis (the wire format the v2 frontend
// already uses for time values). Channel/connector fields capture where
// to send the firing message — copied from the source turn so the
// commitment is self-contained when it fires (the originating connector
// may be gone by then).
type Commitment struct {
	ID            string                `json:"id"`
	WorkspaceID   string                `json:"workspace_id"`
	TenantID      string                `json:"tenant_id"`
	Kind          CommitmentKind        `json:"kind"`
	Sensitivity   CommitmentSensitivity `json:"sensitivity"`
	Status        CommitmentStatus      `json:"status"`
	Reason        string                `json:"reason"`
	SuggestedText string                `json:"suggested_text"`
	DedupeKey     string                `json:"dedupe_key"`
	Confidence    float64               `json:"confidence"`
	DueWindow     DueWindow             `json:"due_window"`

	// Routing — where to fire when the window opens.
	ConnectorID string `json:"connector_id,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`

	// Source provenance — which inbound event spawned this commitment.
	SourceEventID string `json:"source_event_id,omitempty"`

	CreatedAtMs    int64 `json:"created_at_ms"`
	UpdatedAtMs    int64 `json:"updated_at_ms"`
	SentAtMs       int64 `json:"sent_at_ms,omitempty"`
	DismissedAtMs  int64 `json:"dismissed_at_ms,omitempty"`
	SnoozedUntilMs int64 `json:"snoozed_until_ms,omitempty"`
	ExpiredAtMs    int64 `json:"expired_at_ms,omitempty"`
	Attempts       int   `json:"attempts"`
}

// Candidate is a pre-persistence extractor output. The runtime
// validates + dedupes + threshold-gates these before turning them into
// Commitments. Kept separate from `Commitment` so an extractor can't
// accidentally invent IDs or set lifecycle fields.
type Candidate struct {
	Kind          CommitmentKind        `json:"kind"`
	Sensitivity   CommitmentSensitivity `json:"sensitivity"`
	Reason        string                `json:"reason"`
	SuggestedText string                `json:"suggested_text"`
	DedupeKey     string                `json:"dedupe_key"`
	Confidence    float64               `json:"confidence"`
	DueWindow     DueWindow             `json:"due_window"`
}

// MinConfidence is the default floor below which extractor candidates
// are dropped before persistence. Calibrated for v0.3: a confidence of
// 0.6 is "the assistant said this clearly enough that a human reader
// would also call it a commitment." Tune in observability data once
// real traffic lands.
const MinConfidence = 0.6
