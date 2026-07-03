// LLM-based commitment extraction. v0.4 adoption wave.
//
// Design ported from the openclaw commitments study (src/commitments/
// extraction.ts + config.ts, 2026-07 scan):
//
//   - Hidden background classification: the model is told it is an
//     internal extractor, never addresses the user, and outputs JSON only.
//   - Inferred follow-ups only: explicit "remind me at 3" belongs to
//     scheduled recipes, not commitments — the prompt says skip those.
//   - Sensitivity-scaled confidence: `care` candidates need 0.86, others
//     0.72. Care check-ins must be gentle, rare, and high-confidence.
//   - Self-disable after terminal failure: a provider/auth failure
//     disables the LLM path for a cooldown window so a broken key
//     doesn't add latency to every turn; the heuristic path covers the
//     gap.
//
// The extractor is opt-in (IRONGOLEM_COMMITMENTS_EXTRACTOR=llm in
// cmd/main.go); the v0.3 heuristic remains the default and is also the
// fallback whenever the model call or parse fails.
package commitments

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// LLM confidence thresholds (openclaw: confidenceThreshold 0.72,
// careConfidenceThreshold 0.86). Distinct from the heuristic
// MinConfidence — the heuristic's fixed scores were calibrated against
// the 0.6 floor and keep using it.
const (
	LLMConfidenceThreshold     = 0.72
	LLMCareConfidenceThreshold = 0.86

	// llmDisableCooldown is how long the LLM path stays self-disabled
	// after a terminal call failure.
	llmDisableCooldown = 10 * time.Minute

	// llmCallTimeout bounds one extraction call; model latency beyond
	// this degrades to the heuristic for that turn.
	llmCallTimeout = 45 * time.Second

	// llmMaxTokens caps extraction output. Candidate lists are small;
	// a runaway response is a malfunction, not a bigger answer.
	llmMaxTokens = 1024

	// defaultDueWindowSpan fills in a missing/invalid latest bound
	// (openclaw defaults a 12h window when `latest` is absent).
	defaultDueWindowSpan = 12 * time.Hour

	// minLeadTime floors earliest-due so a candidate can never fire in
	// the past or immediately on extraction.
	minLeadTime = 5 * time.Minute
)

// LLMFunc performs one direct model call and returns the raw text.
// cmd/main.go adapts the runtime client's LlmCall to this shape; tests
// substitute canned responses. Keeping it a func type avoids an import
// cycle onto the runtime client package.
type LLMFunc func(ctx context.Context, workspaceID, system, prompt string) (string, error)

// LLMExtractor implements Extractor via a direct-LLM IPC call with a
// heuristic fallback path.
type LLMExtractor struct {
	call     LLMFunc
	fallback Extractor
	logger   *slog.Logger

	// nowFn is injectable wall-clock for deterministic tests.
	nowFn func() time.Time

	mu            sync.Mutex
	disabledUntil time.Time
}

// NewLLMExtractor wires an LLM call path with a heuristic fallback.
// fallback must be non-nil — the whole design contract is "LLM trouble
// degrades, never drops".
func NewLLMExtractor(call LLMFunc, fallback Extractor, logger *slog.Logger) *LLMExtractor {
	if logger == nil {
		logger = slog.Default()
	}
	return &LLMExtractor{
		call:     call,
		fallback: fallback,
		logger:   logger,
		nowFn:    time.Now,
	}
}

// Extract implements Extractor. Failure of the model path (call error,
// unparseable output, self-disabled cooldown) falls back to the
// heuristic extractor. Candidates the model DID return are filtered by
// the sensitivity-scaled confidence threshold — a low-confidence model
// verdict is a "no", not a reason to resurrect regex matches.
func (e *LLMExtractor) Extract(ctx context.Context, in Turn) ([]Candidate, error) {
	now := e.nowFn().UTC()

	e.mu.Lock()
	disabled := now.Before(e.disabledUntil)
	e.mu.Unlock()
	if disabled {
		return e.fallback.Extract(ctx, in)
	}

	callCtx, cancel := context.WithTimeout(ctx, llmCallTimeout)
	defer cancel()

	raw, err := e.call(callCtx, in.WorkspaceID, llmExtractionSystem, buildExtractionPrompt(in))
	if err != nil {
		e.mu.Lock()
		e.disabledUntil = now.Add(llmDisableCooldown)
		e.mu.Unlock()
		e.logger.Warn("commitments: llm extraction failed; using heuristic and cooling down",
			slog.String("error", err.Error()),
			slog.Duration("cooldown", llmDisableCooldown))
		return e.fallback.Extract(ctx, in)
	}

	parsed, err := parseLLMCandidates(raw)
	if err != nil {
		// Parse garbage is a per-call problem, not a provider outage —
		// fall back for this turn but don't disable the path.
		e.logger.Warn("commitments: llm extraction output unparseable; using heuristic",
			slog.String("error", err.Error()))
		return e.fallback.Extract(ctx, in)
	}

	nowMs := in.NowMs
	if nowMs == 0 {
		nowMs = now.UnixMilli()
	}
	return validateLLMCandidates(parsed, in, nowMs), nil
}

// llmExtractionSystem frames the hidden classification run (modeled on
// openclaw's extractor prompt).
const llmExtractionSystem = `You are IronGolem's internal commitment extractor. This is a hidden background classification run. Do not address the user. Output JSON only, no prose, no code fences.

Extract INFERRED follow-up commitments from the exchange: things the assistant promised ("I'll follow up", "I'll check how it went") or user context that deserves a check-in later (an interview, a deadline, a health situation). Skip anything explicitly scheduled ("remind me tomorrow at 3") — that belongs to the scheduler, not here. Skip topics already resolved in the exchange.

Output shape:
{"candidates":[{"kind":"event_check_in|deadline_check|care_check_in|open_loop","sensitivity":"routine|personal|care","reason":"...","suggested_text":"...","confidence":0.0,"dedupe_key":"stable-key","due_in_earliest_minutes":60,"due_in_latest_minutes":720}]}

Rules: confidence in [0,1]; care check-ins must be gentle, rare, and high confidence (>=0.9) with non-interrogating suggested_text; dedupe_key stable for the same underlying commitment (e.g. "interview:2026-07-04"); due offsets are minutes from now and must be positive; return {"candidates":[]} when nothing qualifies.`

// buildExtractionPrompt renders the turn context for the model.
func buildExtractionPrompt(in Turn) string {
	var b strings.Builder
	b.WriteString("USER MESSAGE:\n")
	b.WriteString(in.UserText)
	b.WriteString("\n\nASSISTANT RESPONSE:\n")
	b.WriteString(in.AssistantText)
	b.WriteString("\n\nJSON:")
	return b.String()
}

// llmCandidate is the model-output shape. Due offsets are relative
// minutes rather than ISO timestamps so timezone confusion in the model
// can't produce past-due windows.
type llmCandidate struct {
	Kind                 string  `json:"kind"`
	Sensitivity          string  `json:"sensitivity"`
	Reason               string  `json:"reason"`
	SuggestedText        string  `json:"suggested_text"`
	Confidence           float64 `json:"confidence"`
	DedupeKey            string  `json:"dedupe_key"`
	DueInEarliestMinutes int64   `json:"due_in_earliest_minutes"`
	DueInLatestMinutes   int64   `json:"due_in_latest_minutes"`
}

type llmCandidateList struct {
	Candidates []llmCandidate `json:"candidates"`
}

// parseLLMCandidates decodes the model output. Strict unmarshal first;
// if the model wrapped the JSON in prose or fences, fall back to the
// outermost brace span (openclaw's extractJsonObjectCandidates pattern,
// simplified).
func parseLLMCandidates(raw string) ([]llmCandidate, error) {
	trimmed := strings.TrimSpace(raw)
	var list llmCandidateList
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		return list.Candidates, nil
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("commitments: no JSON object in llm output")
	}
	if err := json.Unmarshal([]byte(trimmed[start:end+1]), &list); err != nil {
		return nil, fmt.Errorf("commitments: parse llm output: %w", err)
	}
	return list.Candidates, nil
}

// llmThresholdFor returns the sensitivity-scaled confidence floor.
func llmThresholdFor(s CommitmentSensitivity) float64 {
	if s == SensitivityCare {
		return LLMCareConfidenceThreshold
	}
	return LLMConfidenceThreshold
}

// validateLLMCandidates enforces enum membership, confidence bounds and
// thresholds, and due-window sanity, converting survivors to Candidate.
// Invalid entries are dropped individually — one bad candidate never
// poisons the batch.
func validateLLMCandidates(raw []llmCandidate, in Turn, nowMs int64) []Candidate {
	validKinds := map[CommitmentKind]bool{
		KindEventCheckIn: true, KindDeadlineCheck: true,
		KindCareCheckIn: true, KindOpenLoop: true,
	}
	validSensitivities := map[CommitmentSensitivity]bool{
		SensitivityRoutine: true, SensitivityPersonal: true, SensitivityCare: true,
	}

	minEarliest := nowMs + minLeadTime.Milliseconds()
	out := make([]Candidate, 0, len(raw))
	for _, rc := range raw {
		kind := CommitmentKind(rc.Kind)
		sensitivity := CommitmentSensitivity(rc.Sensitivity)
		if !validKinds[kind] || !validSensitivities[sensitivity] {
			continue
		}
		if rc.Confidence < 0 || rc.Confidence > 1 {
			continue
		}
		// Care-kind candidates carry care sensitivity requirements even
		// if the model mislabeled the sensitivity axis.
		threshold := llmThresholdFor(sensitivity)
		if kind == KindCareCheckIn {
			threshold = LLMCareConfidenceThreshold
		}
		if rc.Confidence < threshold {
			continue
		}
		if rc.Reason == "" || rc.SuggestedText == "" {
			continue
		}

		earliest := nowMs + rc.DueInEarliestMinutes*time.Minute.Milliseconds()
		if rc.DueInEarliestMinutes <= 0 || earliest < minEarliest {
			earliest = minEarliest
		}
		latest := nowMs + rc.DueInLatestMinutes*time.Minute.Milliseconds()
		if rc.DueInLatestMinutes <= 0 || latest <= earliest {
			latest = earliest + defaultDueWindowSpan.Milliseconds()
		}

		c := Candidate{
			Kind:          kind,
			Sensitivity:   sensitivity,
			Reason:        rc.Reason,
			SuggestedText: rc.SuggestedText,
			Confidence:    rc.Confidence,
			DedupeKey:     rc.DedupeKey,
			DueWindow:     DueWindow{EarliestMs: earliest, LatestMs: latest},
		}
		if c.DedupeKey == "" {
			c.DedupeKey = makeDedupeKey(in, c.Reason)
		}
		out = append(out, c)
	}
	return out
}
