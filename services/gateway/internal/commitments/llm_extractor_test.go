package commitments

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// fixedNow is an arbitrary deterministic wall-clock for these tests.
var fixedNow = time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)

func testTurn() Turn {
	return Turn{
		UserText:      "I have the final interview on Friday, pretty nervous about it",
		AssistantText: "You'll do great. I'll check in with you after it's done.",
		WorkspaceID:   "ws-1",
		SourceEventID: "evt-1",
		NowMs:         fixedNow.UnixMilli(),
	}
}

func newTestExtractor(call LLMFunc) *LLMExtractor {
	e := NewLLMExtractor(call, NewHeuristicExtractor(), nil)
	e.nowFn = func() time.Time { return fixedNow }
	return e
}

func candidateJSON(kind, sensitivity string, confidence float64) string {
	return fmt.Sprintf(`{"candidates":[{"kind":%q,"sensitivity":%q,"reason":"interview follow-up","suggested_text":"How did the interview go?","confidence":%v,"dedupe_key":"interview:friday","due_in_earliest_minutes":60,"due_in_latest_minutes":720}]}`,
		kind, sensitivity, confidence)
}

func TestLLMExtractorConfidenceBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		sens       string
		confidence float64
		wantKept   bool
	}{
		{"routine at threshold kept", "event_check_in", "routine", 0.72, true},
		{"routine below threshold dropped", "event_check_in", "routine", 0.719, false},
		{"care at care threshold kept", "care_check_in", "care", 0.86, true},
		{"care below care threshold dropped", "care_check_in", "care", 0.859, false},
		{"care KIND with mislabeled routine sensitivity still needs 0.86", "care_check_in", "routine", 0.80, false},
		{"personal uses default threshold", "open_loop", "personal", 0.75, true},
		{"confidence above 1 dropped", "event_check_in", "routine", 1.5, false},
		{"negative confidence dropped", "event_check_in", "routine", -0.1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExtractor(func(_ context.Context, _, _, _ string) (string, error) {
				return candidateJSON(tt.kind, tt.sens, tt.confidence), nil
			})
			got, err := e.Extract(context.Background(), testTurn())
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			if kept := len(got) == 1; kept != tt.wantKept {
				t.Fatalf("kept = %v (n=%d), want %v", kept, len(got), tt.wantKept)
			}
		})
	}
}

func TestLLMExtractorCallErrorFallsBackToHeuristicAndCoolsDown(t *testing.T) {
	calls := 0
	e := newTestExtractor(func(_ context.Context, _, _, _ string) (string, error) {
		calls++
		return "", errors.New("provider exploded")
	})

	// Heuristic-visible turn: "in 2 hours" matches the relative-time rule.
	turn := testTurn()
	turn.UserText = "can you check on the deploy in 2 hours"

	got, err := e.Extract(context.Background(), turn)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected heuristic fallback candidates, got none")
	}
	if calls != 1 {
		t.Fatalf("llm calls = %d, want 1", calls)
	}

	// Within the cooldown the LLM path is self-disabled: the heuristic
	// answers and the LLM func is NOT invoked again.
	if _, err := e.Extract(context.Background(), turn); err != nil {
		t.Fatalf("Extract during cooldown: %v", err)
	}
	if calls != 1 {
		t.Fatalf("llm calls during cooldown = %d, want still 1", calls)
	}
}

func TestLLMExtractorParseFailureFallsBackWithoutCooldown(t *testing.T) {
	calls := 0
	e := newTestExtractor(func(_ context.Context, _, _, _ string) (string, error) {
		calls++
		return "I am a model that ignored the JSON instruction entirely", nil
	})
	turn := testTurn()
	turn.UserText = "remind me about this in 2 hours"

	if _, err := e.Extract(context.Background(), turn); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	// Parse garbage must NOT trip the cooldown: next turn retries the LLM.
	if _, err := e.Extract(context.Background(), turn); err != nil {
		t.Fatalf("Extract second: %v", err)
	}
	if calls != 2 {
		t.Fatalf("llm calls = %d, want 2 (no cooldown on parse failure)", calls)
	}
}

func TestLLMExtractorParsesFencedOutput(t *testing.T) {
	e := newTestExtractor(func(_ context.Context, _, _, _ string) (string, error) {
		return "Here you go:\n```json\n" + candidateJSON("open_loop", "routine", 0.9) + "\n```", nil
	})
	got, err := e.Extract(context.Background(), testTurn())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1 (brace-span fallback)", len(got))
	}
	if got[0].DedupeKey != "interview:friday" {
		t.Fatalf("dedupe key = %q", got[0].DedupeKey)
	}
}

func TestLLMExtractorDueWindowSanity(t *testing.T) {
	// Model emits a past/zero earliest and no latest: earliest floors to
	// now+5min, latest defaults to earliest+12h.
	raw := `{"candidates":[{"kind":"deadline_check","sensitivity":"routine","reason":"r","suggested_text":"s","confidence":0.9,"dedupe_key":"k","due_in_earliest_minutes":-30,"due_in_latest_minutes":0}]}`
	e := newTestExtractor(func(_ context.Context, _, _, _ string) (string, error) { return raw, nil })

	got, err := e.Extract(context.Background(), testTurn())
	if err != nil || len(got) != 1 {
		t.Fatalf("Extract: %v, n=%d", err, len(got))
	}
	nowMs := fixedNow.UnixMilli()
	wantEarliest := nowMs + minLeadTime.Milliseconds()
	if got[0].DueWindow.EarliestMs != wantEarliest {
		t.Fatalf("earliest = %d, want %d (now+5m floor)", got[0].DueWindow.EarliestMs, wantEarliest)
	}
	if got[0].DueWindow.LatestMs != wantEarliest+defaultDueWindowSpan.Milliseconds() {
		t.Fatalf("latest = %d, want earliest+12h", got[0].DueWindow.LatestMs)
	}
}

func TestLLMExtractorEnumRejection(t *testing.T) {
	raw := `{"candidates":[
		{"kind":"mind_control","sensitivity":"routine","reason":"r","suggested_text":"s","confidence":0.95,"dedupe_key":"a","due_in_earliest_minutes":60,"due_in_latest_minutes":120},
		{"kind":"open_loop","sensitivity":"radioactive","reason":"r","suggested_text":"s","confidence":0.95,"dedupe_key":"b","due_in_earliest_minutes":60,"due_in_latest_minutes":120},
		{"kind":"open_loop","sensitivity":"routine","reason":"","suggested_text":"s","confidence":0.95,"dedupe_key":"c","due_in_earliest_minutes":60,"due_in_latest_minutes":120},
		{"kind":"open_loop","sensitivity":"routine","reason":"valid one","suggested_text":"s","confidence":0.95,"dedupe_key":"d","due_in_earliest_minutes":60,"due_in_latest_minutes":120}
	]}`
	e := newTestExtractor(func(_ context.Context, _, _, _ string) (string, error) { return raw, nil })
	got, err := e.Extract(context.Background(), testTurn())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 || got[0].DedupeKey != "d" {
		t.Fatalf("want only the valid candidate 'd', got %d", len(got))
	}
}

func TestLLMExtractorEmptyDedupeKeyGetsDerived(t *testing.T) {
	raw := `{"candidates":[{"kind":"open_loop","sensitivity":"routine","reason":"r","suggested_text":"s","confidence":0.9,"dedupe_key":"","due_in_earliest_minutes":60,"due_in_latest_minutes":120}]}`
	e := newTestExtractor(func(_ context.Context, _, _, _ string) (string, error) { return raw, nil })
	got, err := e.Extract(context.Background(), testTurn())
	if err != nil || len(got) != 1 {
		t.Fatalf("Extract: %v, n=%d", err, len(got))
	}
	if got[0].DedupeKey == "" {
		t.Fatal("empty dedupe key should be derived, not left empty")
	}
}
