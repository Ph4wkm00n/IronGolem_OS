// Package models - recipe.go defines the Recipe domain model and built-in
// recipe templates for IronGolem OS.
//
// Recipes are the primary user-facing automation abstraction. Each recipe
// includes a safety summary that clearly communicates what the recipe can
// and cannot do, what actions need approval, and under what conditions the
// recipe will stop automatically.
package models

import (
	"encoding/json"
	"time"
)

// RecipeCategory classifies recipes for the gallery view.
type RecipeCategory string

const (
	RecipeCategoryEmailTriage        RecipeCategory = "email_triage"
	RecipeCategoryCalendarManagement RecipeCategory = "calendar_management"
	RecipeCategoryResearchMonitor    RecipeCategory = "research_monitor"
	RecipeCategoryFileOrganization   RecipeCategory = "file_organization"
	RecipeCategoryCustom             RecipeCategory = "custom"
)

// RiskLevel indicates how risky a recipe or step is.
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

// SafetySummary communicates what an automation can and cannot do in
// plain language, following the "trust before power" principle.
type SafetySummary struct {
	CanAccess        []string `json:"can_access"`
	CannotAccess     []string `json:"cannot_access"`
	NeedsApprovalFor []string `json:"needs_approval_for"`
	StopsIf          []string `json:"stops_if"`
}

// RecipeStep represents a single step within a recipe's execution plan.
type RecipeStep struct {
	ID               string          `json:"id"`
	Description      string          `json:"description"`
	ToolName         string          `json:"tool_name"`
	Input            json.RawMessage `json:"input"`
	RequiresApproval bool            `json:"requires_approval"`
	RiskLevel        RiskLevel       `json:"risk_level"`
}

// DetailedRecipe extends the base Recipe with structured safety summaries,
// categorized steps, and risk levels used by the recipe system handlers.
type DetailedRecipe struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Category      RecipeCategory `json:"category"`
	SafetySummary SafetySummary  `json:"safety_summary"`
	Steps         []RecipeStep   `json:"steps"`
	RiskLevel     RiskLevel      `json:"risk_level"`
	IsActive      bool           `json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// EmailTriageRecipe returns a pre-configured recipe for email triage automation.
func EmailTriageRecipe() DetailedRecipe {
	now := time.Now().UTC()
	return DetailedRecipe{
		ID:          "recipe-email-triage-001",
		Name:        "Email Triage Assistant",
		Description: "Automatically categorizes incoming emails by urgency and topic, drafts replies for routine messages, and flags important items for your review.",
		Category:    RecipeCategoryEmailTriage,
		SafetySummary: SafetySummary{
			CanAccess:        []string{"Read email inbox", "Read email metadata (sender, subject, date)", "View email labels and folders"},
			CannotAccess:     []string{"Email attachments containing sensitive files", "Contacts outside your organization", "Email account settings or forwarding rules"},
			NeedsApprovalFor: []string{"Sending any reply", "Moving emails to archive", "Creating new labels or folders"},
			StopsIf:          []string{"More than 5 emails are flagged as urgent in a single batch", "An email from an unknown domain is detected", "Any email contains financial transaction data"},
		},
		Steps: []RecipeStep{
			{
				ID:               "step-et-001",
				Description:      "Scan inbox for new unread messages",
				ToolName:         "connector.email.read",
				Input:            json.RawMessage(`{"folder": "inbox", "filter": "is:unread"}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-et-002",
				Description:      "Classify emails by urgency (low, medium, high, critical)",
				ToolName:         "agent.classifier",
				Input:            json.RawMessage(`{"model": "urgency_classifier_v2", "categories": ["low", "medium", "high", "critical"]}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-et-003",
				Description:      "Draft replies for routine low-urgency emails",
				ToolName:         "agent.composer",
				Input:            json.RawMessage(`{"style": "professional", "max_length": 200, "urgency_filter": "low"}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelMedium,
			},
			{
				ID:               "step-et-004",
				Description:      "Send drafted replies after approval",
				ToolName:         "connector.email.send",
				Input:            json.RawMessage(`{"require_review": true}`),
				RequiresApproval: true,
				RiskLevel:        RiskLevelHigh,
			},
		},
		RiskLevel: RiskLevelMedium,
		IsActive:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// CalendarManagerRecipe returns a pre-configured recipe for calendar management.
func CalendarManagerRecipe() DetailedRecipe {
	now := time.Now().UTC()
	return DetailedRecipe{
		ID:          "recipe-calendar-mgr-001",
		Name:        "Calendar Manager",
		Description: "Monitors calendar invitations, detects scheduling conflicts, suggests optimal meeting times, and manages RSVPs for routine meetings.",
		Category:    RecipeCategoryCalendarManagement,
		SafetySummary: SafetySummary{
			CanAccess:        []string{"Calendar events and invitations", "Free/busy status", "Meeting room availability"},
			CannotAccess:     []string{"Private calendar entries marked confidential", "Other users' calendars without sharing permissions", "Calendar admin settings"},
			NeedsApprovalFor: []string{"Accepting or declining meeting invitations", "Rescheduling existing meetings", "Booking meeting rooms"},
			StopsIf:          []string{"A conflict involves more than 3 participants", "The meeting is with an external organization", "Any calendar operation fails twice consecutively"},
		},
		Steps: []RecipeStep{
			{
				ID:               "step-cm-001",
				Description:      "Fetch pending calendar invitations",
				ToolName:         "connector.calendar.read",
				Input:            json.RawMessage(`{"scope": "pending_invitations", "lookback_hours": 24}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-cm-002",
				Description:      "Detect scheduling conflicts in the next 7 days",
				ToolName:         "agent.scheduler",
				Input:            json.RawMessage(`{"window_days": 7, "conflict_threshold_minutes": 15}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-cm-003",
				Description:      "Suggest optimal rescheduling for conflicts",
				ToolName:         "agent.optimizer",
				Input:            json.RawMessage(`{"strategy": "minimize_fragmentation", "respect_focus_blocks": true}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelMedium,
			},
			{
				ID:               "step-cm-004",
				Description:      "Apply approved calendar changes",
				ToolName:         "connector.calendar.write",
				Input:            json.RawMessage(`{"require_review": true}`),
				RequiresApproval: true,
				RiskLevel:        RiskLevelHigh,
			},
		},
		RiskLevel: RiskLevelMedium,
		IsActive:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ResearchMonitorRecipe returns a pre-configured recipe for automated research monitoring.
func ResearchMonitorRecipe() DetailedRecipe {
	now := time.Now().UTC()
	return DetailedRecipe{
		ID:          "recipe-research-mon-001",
		Name:        "Research Monitor",
		Description: "Continuously monitors specified topics across news sources, academic papers, and industry reports. Generates daily summaries and alerts for breaking developments.",
		Category:    RecipeCategoryResearchMonitor,
		SafetySummary: SafetySummary{
			CanAccess:        []string{"Public web pages and news APIs", "Academic paper databases (arXiv, PubMed)", "Your saved research topics and keywords"},
			CannotAccess:     []string{"Paywalled content without credentials", "Internal company documents", "Social media private accounts"},
			NeedsApprovalFor: []string{"Adding new data sources", "Sharing research summaries externally", "Subscribing to paid content feeds"},
			StopsIf:          []string{"A source returns repeated errors for more than 1 hour", "Content flagged as potentially misleading exceeds 20%", "API rate limits are reached on any source"},
		},
		Steps: []RecipeStep{
			{
				ID:               "step-rm-001",
				Description:      "Fetch latest content from monitored sources",
				ToolName:         "tool.web_search",
				Input:            json.RawMessage(`{"sources": ["news", "arxiv", "industry_reports"], "recency": "24h"}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-rm-002",
				Description:      "Analyze and extract key findings",
				ToolName:         "agent.researcher",
				Input:            json.RawMessage(`{"extract": ["key_findings", "sentiment", "relevance_score"], "min_relevance": 0.7}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-rm-003",
				Description:      "Generate research digest report",
				ToolName:         "agent.narrator",
				Input:            json.RawMessage(`{"format": "digest", "max_items": 10, "include_sources": true}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-rm-004",
				Description:      "Deliver digest via configured channel",
				ToolName:         "connector.email.send",
				Input:            json.RawMessage(`{"template": "research_digest", "require_review": false}`),
				RequiresApproval: true,
				RiskLevel:        RiskLevelMedium,
			},
		},
		RiskLevel: RiskLevelLow,
		IsActive:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// FilesystemOrganizerRecipe returns a pre-configured recipe for file organization.
func FilesystemOrganizerRecipe() DetailedRecipe {
	now := time.Now().UTC()
	return DetailedRecipe{
		ID:          "recipe-fs-org-001",
		Name:        "File Organizer",
		Description: "Watches designated folders for new files, automatically categorizes them by type and content, moves them to appropriate directories, and maintains a searchable index.",
		Category:    RecipeCategoryFileOrganization,
		SafetySummary: SafetySummary{
			CanAccess:        []string{"Designated watch folders only", "File metadata (name, type, size, date)", "File content for categorization (read-only)"},
			CannotAccess:     []string{"System directories", "Application configuration files", "Files outside designated watch folders"},
			NeedsApprovalFor: []string{"Deleting any file", "Moving files to a new directory structure", "Renaming files based on content analysis"},
			StopsIf:          []string{"A file larger than 1GB is encountered", "More than 50 files need processing in a single batch", "Any file operation returns a permission error"},
		},
		Steps: []RecipeStep{
			{
				ID:               "step-fo-001",
				Description:      "Scan watch folders for new or modified files",
				ToolName:         "tool.filesystem.scan",
				Input:            json.RawMessage(`{"watch_paths": ["/documents/inbox", "/downloads"], "recursive": false}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-fo-002",
				Description:      "Classify files by type and content",
				ToolName:         "agent.classifier",
				Input:            json.RawMessage(`{"classify_by": ["file_type", "content_topic", "project"], "confidence_threshold": 0.8}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelLow,
			},
			{
				ID:               "step-fo-003",
				Description:      "Propose file organization plan",
				ToolName:         "agent.planner",
				Input:            json.RawMessage(`{"strategy": "topic_based", "preserve_originals": true}`),
				RequiresApproval: false,
				RiskLevel:        RiskLevelMedium,
			},
			{
				ID:               "step-fo-004",
				Description:      "Execute approved file moves and renames",
				ToolName:         "tool.filesystem.move",
				Input:            json.RawMessage(`{"require_review": true, "create_backup": true}`),
				RequiresApproval: true,
				RiskLevel:        RiskLevelHigh,
			},
		},
		RiskLevel: RiskLevelMedium,
		IsActive:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
