// Package internal - content analysis components for the research service.
//
// The analyzer uses LLM providers to extract claims, detect contradictions,
// score confidence, and generate research briefs from fetched source content.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/provider"
)

// Claim represents a single factual assertion extracted from source content.
type Claim struct {
	Text      string  `json:"text"`
	SourceURL string  `json:"source_url"`
	Confidence float64 `json:"confidence"`
}

// ContentAnalyzer uses an LLM provider to extract claims, detect
// contradictions, score confidence, and generate brief summaries from
// fetched source content.
type ContentAnalyzer struct {
	provider provider.Provider
	model    string
	logger   *slog.Logger
}

// NewContentAnalyzer creates an analyzer backed by the given LLM provider.
func NewContentAnalyzer(p provider.Provider, model string, logger *slog.Logger) *ContentAnalyzer {
	return &ContentAnalyzer{
		provider: p,
		model:    model,
		logger:   logger,
	}
}

// ExtractClaims asks the LLM to identify the key factual claims from
// the given content and return them as structured data.
func (ca *ContentAnalyzer) ExtractClaims(ctx context.Context, content, sourceURL string) ([]Claim, error) {
	prompt := fmt.Sprintf(
		"Extract the key factual claims from the following content. "+
			"Return a JSON array of objects with fields: text (string), confidence (float 0-1). "+
			"Only include concrete, verifiable claims. Return at most 10 claims.\n\nContent:\n%s",
		truncate(content, 4000),
	)

	resp, err := ca.provider.Complete(ctx, provider.CompletionRequest{
		Model:       ca.model,
		Messages:    []provider.Message{{Role: provider.RoleUser, Content: prompt}},
		MaxTokens:   1024,
		Temperature: 0.1,
		SystemPrompt: "You are a research assistant that extracts factual claims from text. " +
			"Always respond with valid JSON only, no markdown fences.",
	})
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	var extracted []struct {
		Text       string  `json:"text"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(resp.Content)), &extracted); err != nil {
		ca.logger.WarnContext(ctx, "failed to parse LLM claim extraction",
			slog.String("error", err.Error()),
			slog.String("raw", resp.Content),
		)
		return nil, fmt.Errorf("parsing claims: %w", err)
	}

	claims := make([]Claim, 0, len(extracted))
	for _, e := range extracted {
		claims = append(claims, Claim{
			Text:       e.Text,
			SourceURL:  sourceURL,
			Confidence: e.Confidence,
		})
	}

	ca.logger.DebugContext(ctx, "claims extracted",
		slog.String("source", sourceURL),
		slog.Int("count", len(claims)),
	)
	return claims, nil
}

// GenerateSummary asks the LLM to produce a concise summary of the
// provided content pieces for a research brief.
func (ca *ContentAnalyzer) GenerateSummary(ctx context.Context, topicName string, contents []string) (string, error) {
	combined := strings.Join(contents, "\n\n---\n\n")
	prompt := fmt.Sprintf(
		"Summarize the following source materials about %q into a concise research brief (2-4 paragraphs). "+
			"Highlight key findings, trends, and any areas of uncertainty.\n\n%s",
		topicName, truncate(combined, 6000),
	)

	resp, err := ca.provider.Complete(ctx, provider.CompletionRequest{
		Model:        ca.model,
		Messages:     []provider.Message{{Role: provider.RoleUser, Content: prompt}},
		MaxTokens:    1024,
		Temperature:  0.3,
		SystemPrompt: "You are a research analyst producing clear, evidence-based summaries.",
	})
	if err != nil {
		return "", fmt.Errorf("LLM summary failed: %w", err)
	}
	return strings.TrimSpace(resp.Content), nil
}

// SuggestActions asks the LLM to propose actionable next steps based on
// the research findings.
func (ca *ContentAnalyzer) SuggestActions(ctx context.Context, topicName, summary string) ([]string, error) {
	prompt := fmt.Sprintf(
		"Based on this research summary about %q, suggest 1-5 concrete action items the user could take. "+
			"Return a JSON array of strings.\n\nSummary:\n%s",
		topicName, summary,
	)

	resp, err := ca.provider.Complete(ctx, provider.CompletionRequest{
		Model:       ca.model,
		Messages:    []provider.Message{{Role: provider.RoleUser, Content: prompt}},
		MaxTokens:   512,
		Temperature: 0.3,
		SystemPrompt: "You are a research assistant. Respond with valid JSON only, no markdown fences.",
	})
	if err != nil {
		return nil, fmt.Errorf("LLM action suggestion failed: %w", err)
	}

	var actions []string
	if err := json.Unmarshal([]byte(cleanJSON(resp.Content)), &actions); err != nil {
		ca.logger.WarnContext(ctx, "failed to parse action suggestions",
			slog.String("error", err.Error()),
		)
		return []string{"Review the research brief for further details"}, nil
	}
	return actions, nil
}

// ContradictionDetector compares claims across sources and flags conflicts.
type ContradictionDetector struct {
	provider provider.Provider
	model    string
	logger   *slog.Logger
}

// NewContradictionDetector creates a detector backed by the given LLM provider.
func NewContradictionDetector(p provider.Provider, model string, logger *slog.Logger) *ContradictionDetector {
	return &ContradictionDetector{
		provider: p,
		model:    model,
		logger:   logger,
	}
}

// DetectContradictions compares claims from different sources and identifies
// contradictions using the LLM. It returns any detected conflicts with
// severity ratings.
func (cd *ContradictionDetector) DetectContradictions(ctx context.Context, claims []Claim) ([]models.Contradiction, error) {
	if len(claims) < 2 {
		return nil, nil
	}

	// Group claims by source for the prompt.
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("marshalling claims: %w", err)
	}

	prompt := fmt.Sprintf(
		"Analyze the following claims from multiple sources and identify any contradictions. "+
			"Return a JSON array of objects with fields: "+
			"claim_a (string), claim_b (string), source_a (string URL), source_b (string URL), "+
			"severity (one of: low, medium, high). "+
			"If there are no contradictions, return an empty array [].\n\nClaims:\n%s",
		string(claimsJSON),
	)

	resp, err := cd.provider.Complete(ctx, provider.CompletionRequest{
		Model:       cd.model,
		Messages:    []provider.Message{{Role: provider.RoleUser, Content: prompt}},
		MaxTokens:   1024,
		Temperature: 0.1,
		SystemPrompt: "You are a fact-checking analyst detecting contradictions between sources. " +
			"Respond with valid JSON only, no markdown fences.",
	})
	if err != nil {
		return nil, fmt.Errorf("LLM contradiction detection failed: %w", err)
	}

	var detected []struct {
		ClaimA   string `json:"claim_a"`
		ClaimB   string `json:"claim_b"`
		SourceA  string `json:"source_a"`
		SourceB  string `json:"source_b"`
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(resp.Content)), &detected); err != nil {
		cd.logger.WarnContext(ctx, "failed to parse contradiction detection",
			slog.String("error", err.Error()),
			slog.String("raw", resp.Content),
		)
		return nil, nil // non-fatal: just no contradictions detected
	}

	contradictions := make([]models.Contradiction, 0, len(detected))
	for _, d := range detected {
		severity := models.ContradictionSeverityLow
		switch strings.ToLower(d.Severity) {
		case "medium":
			severity = models.ContradictionSeverityMedium
		case "high":
			severity = models.ContradictionSeverityHigh
		}
		contradictions = append(contradictions, models.Contradiction{
			ClaimA:   d.ClaimA,
			ClaimB:   d.ClaimB,
			SourceA:  d.SourceA,
			SourceB:  d.SourceB,
			Severity: severity,
		})
	}

	cd.logger.InfoContext(ctx, "contradictions detected",
		slog.Int("count", len(contradictions)),
	)
	return contradictions, nil
}

// BriefGenerator synthesizes research findings into a ResearchBrief with
// evidence links, contradiction analysis, and action suggestions.
type BriefGenerator struct {
	analyzer   *ContentAnalyzer
	detector   *ContradictionDetector
	scorer     *TrustScorer
	logger     *slog.Logger
}

// NewBriefGenerator creates a generator that orchestrates claim extraction,
// contradiction detection, and summary generation.
func NewBriefGenerator(analyzer *ContentAnalyzer, detector *ContradictionDetector, scorer *TrustScorer, logger *slog.Logger) *BriefGenerator {
	return &BriefGenerator{
		analyzer: analyzer,
		detector: detector,
		scorer:   scorer,
		logger:   logger,
	}
}

// SourceContent pairs a TopicSource with its fetched content.
type SourceContent struct {
	Source  models.TopicSource
	Content string
}

// Generate produces a ResearchBrief from the given fetched source contents.
// It extracts claims, detects contradictions, computes confidence, generates
// a summary, and suggests actions.
func (bg *BriefGenerator) Generate(ctx context.Context, topic models.TrackedTopic, sources []SourceContent) (models.ResearchBrief, error) {
	now := time.Now().UTC()

	// Extract claims from each source.
	var allClaims []Claim
	var briefSources []models.BriefSource
	var contentTexts []string

	for _, sc := range sources {
		claims, err := bg.analyzer.ExtractClaims(ctx, sc.Content, sc.Source.URL)
		if err != nil {
			bg.logger.WarnContext(ctx, "claim extraction failed for source",
				slog.String("url", sc.Source.URL),
				slog.String("error", err.Error()),
			)
			continue
		}
		allClaims = append(allClaims, claims...)
		contentTexts = append(contentTexts, sc.Content)

		excerpt := truncate(sc.Content, 200)
		briefSources = append(briefSources, models.BriefSource{
			URL:        sc.Source.URL,
			Title:      sc.Source.Name,
			Excerpt:    excerpt,
			TrustScore: sc.Source.TrustScore,
			FetchedAt:  sc.Source.LastFetched,
		})
	}

	// Detect contradictions across claims.
	contradictions, err := bg.detector.DetectContradictions(ctx, allClaims)
	if err != nil {
		bg.logger.WarnContext(ctx, "contradiction detection failed",
			slog.String("error", err.Error()),
		)
		contradictions = nil
	}

	// Compute confidence score based on source agreement and trust.
	confidence := computeConfidence(allClaims, contradictions, briefSources)

	// Generate summary.
	summary, err := bg.analyzer.GenerateSummary(ctx, topic.Name, contentTexts)
	if err != nil {
		summary = "Summary generation failed; see individual sources for details."
		bg.logger.WarnContext(ctx, "summary generation failed",
			slog.String("error", err.Error()),
		)
	}

	// Generate action suggestions.
	actions, err := bg.analyzer.SuggestActions(ctx, topic.Name, summary)
	if err != nil {
		actions = nil
		bg.logger.WarnContext(ctx, "action suggestion failed",
			slog.String("error", err.Error()),
		)
	}

	brief := models.ResearchBrief{
		ID:                generateID(),
		TopicID:           topic.ID,
		Title:             fmt.Sprintf("Research Brief: %s", topic.Name),
		Summary:           summary,
		Confidence:        confidence,
		Freshness:         time.Duration(0), // current as of generation
		Sources:           briefSources,
		Contradictions:    contradictions,
		ActionSuggestions: actions,
		CreatedAt:         now,
	}

	bg.logger.InfoContext(ctx, "brief generated",
		slog.String("brief_id", brief.ID),
		slog.String("topic_id", topic.ID),
		slog.Float64("confidence", confidence),
		slog.Int("sources", len(briefSources)),
		slog.Int("contradictions", len(contradictions)),
	)

	return brief, nil
}

// computeConfidence derives an overall confidence score from the claim
// agreement, contradiction severity, and source trust scores.
func computeConfidence(claims []Claim, contradictions []models.Contradiction, sources []models.BriefSource) float64 {
	if len(claims) == 0 {
		return 0
	}

	// Base confidence from average claim confidence.
	var total float64
	for _, c := range claims {
		total += c.Confidence
	}
	base := total / float64(len(claims))

	// Penalize for contradictions.
	penalty := 0.0
	for _, c := range contradictions {
		switch c.Severity {
		case models.ContradictionSeverityHigh:
			penalty += 0.15
		case models.ContradictionSeverityMedium:
			penalty += 0.08
		case models.ContradictionSeverityLow:
			penalty += 0.03
		}
	}

	// Boost for high-trust sources.
	trustBoost := 0.0
	if len(sources) > 0 {
		var trustSum float64
		for _, s := range sources {
			trustSum += s.TrustScore
		}
		avgTrust := trustSum / float64(len(sources))
		trustBoost = (avgTrust - 0.5) * 0.2
	}

	confidence := base - penalty + trustBoost

	// Clamp to [0, 1].
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	return confidence
}

// truncate shortens a string to the given maximum rune count.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// cleanJSON attempts to strip markdown code fences from LLM output so
// that the JSON inside can be parsed.
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	// Remove ```json ... ``` fences.
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s[3:], "\n"); idx >= 0 {
			s = s[3+idx+1:]
		}
		if strings.HasSuffix(s, "```") {
			s = s[:len(s)-3]
		}
	}
	return strings.TrimSpace(s)
}
