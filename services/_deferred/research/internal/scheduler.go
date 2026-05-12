package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

// Research-specific event kinds extending the base event system.
const (
	EventKindSourceFetched         events.EventKind = "research.source_fetched"
	EventKindContradictionDetected events.EventKind = "research.contradiction_detected"
	EventKindBriefPublished        events.EventKind = "research.brief_published"
)

// ResearchScheduler runs periodic checks on tracked topics, fetches
// sources, generates briefs, and emits events for the event sourcing layer.
type ResearchScheduler struct {
	tracker   *TopicTracker
	fetcher   SourceFetcher
	limiter   *RateLimiter
	generator *BriefGenerator
	logger    *slog.Logger

	pollInterval time.Duration
	stopOnce     sync.Once
	done         chan struct{}

	mu       sync.Mutex
	eventLog []events.Event
}

// SchedulerConfig configures the research scheduler.
type SchedulerConfig struct {
	PollInterval time.Duration
}

// DefaultSchedulerConfig returns sensible defaults.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		PollInterval: 1 * time.Minute,
	}
}

// NewResearchScheduler creates a scheduler that periodically checks
// tracked topics for updates.
func NewResearchScheduler(
	tracker *TopicTracker,
	fetcher SourceFetcher,
	limiter *RateLimiter,
	generator *BriefGenerator,
	cfg SchedulerConfig,
	logger *slog.Logger,
) *ResearchScheduler {
	return &ResearchScheduler{
		tracker:      tracker,
		fetcher:      fetcher,
		limiter:      limiter,
		generator:    generator,
		logger:       logger,
		pollInterval: cfg.PollInterval,
		done:         make(chan struct{}),
		eventLog:     make([]events.Event, 0),
	}
}

// Run starts the background polling loop. It blocks until the context
// is cancelled or Stop is called.
func (rs *ResearchScheduler) Run(ctx context.Context) {
	rs.logger.InfoContext(ctx, "research scheduler started",
		slog.Duration("poll_interval", rs.pollInterval),
	)

	ticker := time.NewTicker(rs.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			rs.logger.InfoContext(ctx, "research scheduler stopping (context cancelled)")
			return
		case <-rs.done:
			rs.logger.InfoContext(ctx, "research scheduler stopping (stop called)")
			return
		case <-ticker.C:
			rs.poll(ctx)
		}
	}
}

// Stop signals the scheduler to stop.
func (rs *ResearchScheduler) Stop() {
	rs.stopOnce.Do(func() {
		close(rs.done)
	})
}

// Events returns a snapshot of all emitted events.
func (rs *ResearchScheduler) Events() []events.Event {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := make([]events.Event, len(rs.eventLog))
	copy(out, rs.eventLog)
	return out
}

// CheckTopicNow forces an immediate check of a single topic by ID.
// This is used by the force-check HTTP endpoint.
func (rs *ResearchScheduler) CheckTopicNow(ctx context.Context, topicID string) error {
	topic, err := rs.tracker.GetTopic(ctx, topicID)
	if err != nil {
		return err
	}
	rs.checkTopic(ctx, topic)
	return nil
}

// poll checks all topics that are due for an update.
func (rs *ResearchScheduler) poll(ctx context.Context) {
	dueTopics, err := rs.tracker.CheckForUpdates(ctx)
	if err != nil {
		rs.logger.ErrorContext(ctx, "failed to check for due topics",
			slog.String("error", err.Error()),
		)
		return
	}

	if len(dueTopics) == 0 {
		return
	}

	rs.logger.InfoContext(ctx, "topics due for check",
		slog.Int("count", len(dueTopics)),
	)

	for _, topic := range dueTopics {
		rs.checkTopic(ctx, topic)
	}
}

// checkTopic fetches all active sources for a topic, runs analysis, and
// generates a brief.
func (rs *ResearchScheduler) checkTopic(ctx context.Context, topic models.TrackedTopic) {
	rs.logger.InfoContext(ctx, "checking topic",
		slog.String("topic_id", topic.ID),
		slog.String("topic_name", topic.Name),
	)

	var sourceContents []SourceContent

	for _, src := range topic.Sources {
		if !src.IsActive {
			continue
		}

		// Rate-limit per domain.
		parsed, err := url.Parse(src.URL)
		if err != nil {
			rs.logger.WarnContext(ctx, "invalid source URL",
				slog.String("url", src.URL),
				slog.String("error", err.Error()),
			)
			continue
		}

		if err := rs.limiter.Wait(ctx, parsed.Hostname()); err != nil {
			rs.logger.WarnContext(ctx, "rate limiter cancelled",
				slog.String("domain", parsed.Hostname()),
				slog.String("error", err.Error()),
			)
			return
		}

		result, err := rs.fetcher.FetchSource(ctx, src.URL)
		if err != nil {
			rs.logger.WarnContext(ctx, "source fetch failed",
				slog.String("url", src.URL),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Emit SourceFetched event.
		rs.emitEvent(ctx, EventKindSourceFetched, topic.WorkspaceID, map[string]any{
			"topic_id":      topic.ID,
			"source_url":    src.URL,
			"status_code":   result.StatusCode,
			"response_time": result.ResponseTime.String(),
		})

		sourceContents = append(sourceContents, SourceContent{
			Source:  src,
			Content: result.Content,
		})
	}

	if len(sourceContents) == 0 {
		rs.logger.InfoContext(ctx, "no source content fetched for topic",
			slog.String("topic_id", topic.ID),
		)
		return
	}

	// Generate brief.
	brief, err := rs.generator.Generate(ctx, topic, sourceContents)
	if err != nil {
		rs.logger.ErrorContext(ctx, "brief generation failed",
			slog.String("topic_id", topic.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Store the brief.
	if err := rs.tracker.Store().AddBrief(ctx, brief); err != nil {
		rs.logger.ErrorContext(ctx, "failed to store brief",
			slog.String("brief_id", brief.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Emit events for contradictions.
	for _, c := range brief.Contradictions {
		rs.emitEvent(ctx, EventKindContradictionDetected, topic.WorkspaceID, map[string]any{
			"topic_id": topic.ID,
			"brief_id": brief.ID,
			"claim_a":  c.ClaimA,
			"claim_b":  c.ClaimB,
			"source_a": c.SourceA,
			"source_b": c.SourceB,
			"severity": string(c.Severity),
		})
	}

	// Emit BriefPublished event.
	rs.emitEvent(ctx, EventKindBriefPublished, topic.WorkspaceID, map[string]any{
		"topic_id":       topic.ID,
		"brief_id":       brief.ID,
		"confidence":     brief.Confidence,
		"sources_count":  len(brief.Sources),
		"contradictions": len(brief.Contradictions),
	})

	// Mark topic as checked.
	if err := rs.tracker.MarkChecked(ctx, topic.ID); err != nil {
		rs.logger.ErrorContext(ctx, "failed to mark topic checked",
			slog.String("topic_id", topic.ID),
			slog.String("error", err.Error()),
		)
	}

	rs.logger.InfoContext(ctx, "topic check complete",
		slog.String("topic_id", topic.ID),
		slog.String("brief_id", brief.ID),
	)
}

// emitEvent creates and stores a research event.
func (rs *ResearchScheduler) emitEvent(ctx context.Context, kind events.EventKind, workspaceID string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		rs.logger.ErrorContext(ctx, "failed to marshal event payload",
			slog.String("error", err.Error()),
		)
		return
	}

	evt := events.NewEvent(kind, "", "research", data)
	evt.WorkspaceID = workspaceID

	rs.mu.Lock()
	rs.eventLog = append(rs.eventLog, evt)
	rs.mu.Unlock()

	rs.logger.InfoContext(ctx, "event emitted",
		slog.String("kind", string(kind)),
		slog.String("event_id", evt.ID),
	)
}
