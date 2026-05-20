package commitments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Extractor turns a closed assistant turn into commitment candidates.
//
// The interface stays narrow: in / out / err. v0.3 ships a
// `HeuristicExtractor` (regex+keyword based — no LLM round trip, fast,
// good enough to validate the storage + firing path). v0.4 will add an
// `LLMExtractor` once the runtimed binary exposes a direct-LLM IPC verb
// (today the runtime only accepts `ExecutePlan` shapes, which would
// pollute the timeline with extraction plan events).
type Extractor interface {
	Extract(ctx context.Context, in Turn) ([]Candidate, error)
}

// Turn is one user→assistant exchange. The extractor sees both sides
// because:
//
//   - the user message often carries the time anchor ("Tuesday at 6pm")
//   - the assistant message carries the *commitment language*
//     ("I'll remind you", "I'll check in", "I'll follow up")
//
// Together they're sufficient to extract candidates without a session
// history fetch.
type Turn struct {
	UserText      string
	AssistantText string
	// Provenance for the candidate.
	WorkspaceID   string
	TenantID      string
	SourceEventID string
	ConnectorID   string
	ChannelID     string
	UserID        string
	// NowMs lets tests inject deterministic wall-clock. Default 0 →
	// `time.Now().UnixMilli()`.
	NowMs int64
}

// HeuristicExtractor uses regex + keyword matching. Deliberately
// conservative — false positives are worse than misses (a spurious
// reminder is annoying, a missed one is recoverable).
//
// v0.3 detects two commitment shapes:
//
//   1. Explicit time anchors: "Tuesday at 6pm", "in 2 hours", "tomorrow
//      morning". Produces an `event_check_in` with the parsed window.
//   2. Open-loop language: "I'll keep an eye on", "I'll follow up".
//      Produces an `open_loop` candidate with a 24h-72h follow-up
//      window. Confidence is lower so the MinConfidence threshold
//      filters most of them.
type HeuristicExtractor struct{}

// NewHeuristicExtractor returns a ready-to-use extractor. No
// configuration; the heuristics are deliberately fixed at v0.3.
func NewHeuristicExtractor() *HeuristicExtractor { return &HeuristicExtractor{} }

// Extract implements Extractor.
func (h *HeuristicExtractor) Extract(_ context.Context, in Turn) ([]Candidate, error) {
	now := in.NowMs
	if now == 0 {
		now = time.Now().UTC().UnixMilli()
	}
	combined := strings.ToLower(in.UserText + "\n" + in.AssistantText)

	var candidates []Candidate

	// 1. Relative-time matches: "in N hours/minutes/days".
	if c, ok := matchRelativeTime(combined, now); ok {
		c.DedupeKey = makeDedupeKey(in, c.Reason)
		candidates = append(candidates, c)
	}

	// 2. Day-of-week match: "Tuesday at 6pm", "Friday morning".
	if c, ok := matchDayOfWeek(combined, now); ok {
		c.DedupeKey = makeDedupeKey(in, c.Reason)
		candidates = append(candidates, c)
	}

	// 3. Open-loop language: "I'll follow up", "I'll keep an eye on".
	if c, ok := matchOpenLoop(in.AssistantText, now); ok {
		c.DedupeKey = makeDedupeKey(in, c.Reason)
		candidates = append(candidates, c)
	}

	return candidates, nil
}

var relativeRe = regexp.MustCompile(`in (\d+) (minute|hour|day)s?`)

func matchRelativeTime(text string, nowMs int64) (Candidate, bool) {
	m := relativeRe.FindStringSubmatch(text)
	if m == nil {
		return Candidate{}, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return Candidate{}, false
	}
	unit := m[2]
	var offset time.Duration
	switch unit {
	case "minute":
		offset = time.Duration(n) * time.Minute
	case "hour":
		offset = time.Duration(n) * time.Hour
	case "day":
		offset = time.Duration(n) * 24 * time.Hour
	default:
		return Candidate{}, false
	}
	target := time.UnixMilli(nowMs).Add(offset)
	// 10-minute fire window centered on the target so an idle ticker
	// still has a fair chance to catch it.
	earliest := target.Add(-5 * time.Minute)
	latest := target.Add(5 * time.Minute)
	return Candidate{
		Kind:          KindEventCheckIn,
		Sensitivity:   SensitivityRoutine,
		Reason:        "user mentioned a relative time window",
		SuggestedText: "Reminder: " + m[0],
		Confidence:    0.75,
		DueWindow: DueWindow{
			EarliestMs: earliest.UnixMilli(),
			LatestMs:   latest.UnixMilli(),
		},
	}, true
}

var dowRe = regexp.MustCompile(`\b(monday|tuesday|wednesday|thursday|friday|saturday|sunday)( at (\d{1,2})(?::(\d{2}))? ?(am|pm)?)?`)

var dowToWeekday = map[string]time.Weekday{
	"sunday":    time.Sunday,
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
}

func matchDayOfWeek(text string, nowMs int64) (Candidate, bool) {
	m := dowRe.FindStringSubmatch(text)
	if m == nil {
		return Candidate{}, false
	}
	dow, ok := dowToWeekday[m[1]]
	if !ok {
		return Candidate{}, false
	}
	now := time.UnixMilli(nowMs).UTC()
	daysAhead := int(dow - now.Weekday())
	if daysAhead <= 0 {
		daysAhead += 7
	}
	target := time.Date(now.Year(), now.Month(), now.Day()+daysAhead, 9, 0, 0, 0, time.UTC)
	// Apply optional "at HH(:MM) am/pm".
	if hh := m[3]; hh != "" {
		h, _ := strconv.Atoi(hh)
		mm := 0
		if m[4] != "" {
			mm, _ = strconv.Atoi(m[4])
		}
		ampm := m[5]
		if ampm == "pm" && h < 12 {
			h += 12
		}
		if ampm == "am" && h == 12 {
			h = 0
		}
		target = time.Date(target.Year(), target.Month(), target.Day(), h, mm, 0, 0, time.UTC)
	}
	earliest := target.Add(-30 * time.Minute)
	latest := target.Add(30 * time.Minute)
	return Candidate{
		Kind:          KindEventCheckIn,
		Sensitivity:   SensitivityRoutine,
		Reason:        "assistant promised a day-of-week check-in",
		SuggestedText: "Reminder: " + m[0],
		Confidence:    0.7,
		DueWindow: DueWindow{
			EarliestMs: earliest.UnixMilli(),
			LatestMs:   latest.UnixMilli(),
		},
	}, true
}

var openLoopPhrases = []string{
	"i'll follow up",
	"i'll check in",
	"i'll check on",
	"i'll keep an eye on",
	"i will follow up",
	"i'll remind you",
	"let me know if",
}

func matchOpenLoop(assistant string, nowMs int64) (Candidate, bool) {
	low := strings.ToLower(assistant)
	for _, phrase := range openLoopPhrases {
		if strings.Contains(low, phrase) {
			now := time.UnixMilli(nowMs)
			earliest := now.Add(24 * time.Hour)
			latest := now.Add(72 * time.Hour)
			// "i'll check on" suggests care; everything else routine.
			sensitivity := SensitivityRoutine
			kind := KindOpenLoop
			if strings.Contains(low, "i'll check on") {
				sensitivity = SensitivityCare
				kind = KindCareCheckIn
			}
			return Candidate{
				Kind:          kind,
				Sensitivity:   sensitivity,
				Reason:        "assistant signaled an open-loop follow-up",
				SuggestedText: "Following up on what we discussed",
				Confidence:    0.6,
				DueWindow: DueWindow{
					EarliestMs: earliest.UnixMilli(),
					LatestMs:   latest.UnixMilli(),
				},
			}, true
		}
	}
	return Candidate{}, false
}

// makeDedupeKey hashes the workspace + source event + reason so a
// repeated extraction over the same turn doesn't spawn duplicate
// commitments. Truncated to 32 hex chars — collision probability is
// negligible at the row counts the gateway will ever hold.
func makeDedupeKey(in Turn, reason string) string {
	h := sha256.Sum256([]byte(in.WorkspaceID + "|" + in.SourceEventID + "|" + reason))
	return hex.EncodeToString(h[:16])
}
