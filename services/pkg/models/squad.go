package models

import "time"

// AutonomyLevel controls how much independence a squad has.
type AutonomyLevel string

const (
	AutonomyLow    AutonomyLevel = "low"
	AutonomyMedium AutonomyLevel = "medium"
	AutonomyHigh   AutonomyLevel = "high"
)

// SquadMember represents an individual agent within a squad.
type SquadMember struct {
	Role         AgentRole `json:"role"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Tools        []string  `json:"tools"`
	Capabilities []string  `json:"capabilities"`
}

// Squad is a pre-composed multi-agent team that collaborates to handle
// a category of tasks. Squads are the primary unit of autonomous work
// in IronGolem OS.
type Squad struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Purpose       string        `json:"purpose"`
	Members       []SquadMember `json:"members"`
	WorkspaceID   string        `json:"workspace_id"`
	IsActive      bool          `json:"is_active"`
	TrustLevel    int           `json:"trust_level"`
	AutonomyLevel AutonomyLevel `json:"autonomy_level"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	LastRunAt     *time.Time    `json:"last_run_at,omitempty"`
}

// SquadTemplate is a reusable squad definition without workspace binding.
// Built-in templates can be instantiated into any workspace.
type SquadTemplate struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Purpose       string        `json:"purpose"`
	Members       []SquadMember `json:"members"`
	TrustLevel    int           `json:"trust_level"`
	AutonomyLevel AutonomyLevel `json:"autonomy_level"`
}

// InboxSquad returns the built-in Inbox squad template.
// The Inbox squad handles message triage, drafting replies, and verification.
func InboxSquad() SquadTemplate {
	return SquadTemplate{
		ID:          "squad-inbox",
		Name:        "Inbox Squad",
		Description: "Manages incoming messages across all channels with intelligent triage, drafting, and quality verification.",
		Purpose:     "Classify, prioritize, draft responses, and verify outgoing messages for all communication channels.",
		Members: []SquadMember{
			{
				Role:         AgentRoleRouter,
				Name:         "Classifier",
				Description:  "Classifies incoming messages by urgency, topic, and required action.",
				Tools:        []string{"tool.nlp_classify", "tool.sentiment_analysis"},
				Capabilities: []string{"message.classify", "message.prioritize", "message.tag"},
			},
			{
				Role:         AgentRoleExecutor,
				Name:         "Drafter",
				Description:  "Drafts contextual responses using conversation history and user preferences.",
				Tools:        []string{"tool.text_generation", "tool.template_engine", "tool.contact_lookup"},
				Capabilities: []string{"message.draft", "message.send", "contact.read"},
			},
			{
				Role:         AgentRoleVerifier,
				Name:         "Verifier",
				Description:  "Reviews drafted messages for tone, accuracy, and policy compliance before sending.",
				Tools:        []string{"tool.grammar_check", "tool.policy_check"},
				Capabilities: []string{"message.review", "message.approve", "message.reject"},
			},
		},
		TrustLevel:    2,
		AutonomyLevel: AutonomyMedium,
	}
}

// ResearchSquad returns the built-in Research squad template.
// The Research squad performs autonomous information gathering and synthesis.
func ResearchSquad() SquadTemplate {
	return SquadTemplate{
		ID:          "squad-research",
		Name:        "Research Squad",
		Description: "Conducts autonomous research across web, documents, and databases with source verification.",
		Purpose:     "Gather, synthesize, and verify information from multiple sources to answer questions and produce reports.",
		Members: []SquadMember{
			{
				Role:         AgentRolePlanner,
				Name:         "Research Planner",
				Description:  "Decomposes research questions into searchable sub-queries and plans the research workflow.",
				Tools:        []string{"tool.query_decomposition"},
				Capabilities: []string{"research.plan", "research.decompose"},
			},
			{
				Role:         AgentRoleResearcher,
				Name:         "Gatherer",
				Description:  "Searches the web, documents, and databases to collect relevant information.",
				Tools:        []string{"tool.web_search", "tool.document_search", "tool.database_query"},
				Capabilities: []string{"research.search", "research.fetch", "document.read"},
			},
			{
				Role:         AgentRoleNarrator,
				Name:         "Synthesizer",
				Description:  "Synthesizes gathered information into coherent summaries and reports.",
				Tools:        []string{"tool.text_generation", "tool.citation_formatter"},
				Capabilities: []string{"research.summarize", "research.report", "research.cite"},
			},
			{
				Role:         AgentRoleVerifier,
				Name:         "Fact Checker",
				Description:  "Verifies claims against sources and flags unsubstantiated statements.",
				Tools:        []string{"tool.fact_check", "tool.source_verify"},
				Capabilities: []string{"research.verify", "research.flag"},
			},
		},
		TrustLevel:    2,
		AutonomyLevel: AutonomyMedium,
	}
}

// OpsSquad returns the built-in Operations squad template.
// The Ops squad manages system health, deployments, and infrastructure tasks.
func OpsSquad() SquadTemplate {
	return SquadTemplate{
		ID:          "squad-ops",
		Name:        "Ops Squad",
		Description: "Monitors system health, manages deployments, and performs routine operational tasks with self-healing.",
		Purpose:     "Keep systems healthy through monitoring, automated remediation, and proactive maintenance.",
		Members: []SquadMember{
			{
				Role:         AgentRoleHealer,
				Name:         "Health Monitor",
				Description:  "Continuously monitors system health metrics and triggers healing workflows when anomalies are detected.",
				Tools:        []string{"tool.metrics_query", "tool.log_search", "tool.alert_manager"},
				Capabilities: []string{"system.monitor", "system.alert", "healing.trigger"},
			},
			{
				Role:         AgentRoleExecutor,
				Name:         "Remediator",
				Description:  "Executes automated remediation actions such as restarts, scaling, and configuration rollbacks.",
				Tools:        []string{"tool.service_restart", "tool.config_rollback", "tool.scale_service"},
				Capabilities: []string{"system.restart", "system.rollback", "system.scale"},
			},
			{
				Role:         AgentRoleVerifier,
				Name:         "Ops Verifier",
				Description:  "Verifies that remediation actions resolved the issue and the system is healthy.",
				Tools:        []string{"tool.health_check", "tool.smoke_test"},
				Capabilities: []string{"system.verify", "system.test"},
			},
		},
		TrustLevel:    3,
		AutonomyLevel: AutonomyHigh,
	}
}

// SecuritySquad returns the built-in Security squad template.
// The Security squad defends against threats and enforces policies.
func SecuritySquad() SquadTemplate {
	return SquadTemplate{
		ID:          "squad-security",
		Name:        "Security Squad",
		Description: "Defends against prompt injection, data exfiltration, and policy violations with real-time threat detection.",
		Purpose:     "Detect, block, and report security threats while enforcing organizational security policies.",
		Members: []SquadMember{
			{
				Role:         AgentRoleDefender,
				Name:         "Threat Detector",
				Description:  "Scans inputs and outputs for prompt injection, SSRF attempts, and other attack patterns.",
				Tools:        []string{"tool.prompt_scanner", "tool.pattern_matcher", "tool.anomaly_detector"},
				Capabilities: []string{"security.scan", "security.detect", "security.block"},
			},
			{
				Role:         AgentRoleVerifier,
				Name:         "Policy Enforcer",
				Description:  "Validates all actions against the five-layer security model and organizational policies.",
				Tools:        []string{"tool.policy_check", "tool.permission_verify"},
				Capabilities: []string{"security.enforce", "security.audit", "security.report"},
			},
			{
				Role:         AgentRoleNarrator,
				Name:         "Security Reporter",
				Description:  "Generates security reports, incident summaries, and compliance documentation.",
				Tools:        []string{"tool.text_generation", "tool.report_formatter"},
				Capabilities: []string{"security.report", "security.summarize"},
			},
		},
		TrustLevel:    4,
		AutonomyLevel: AutonomyHigh,
	}
}

// ExecutiveAssistantSquad returns the built-in Executive Assistant squad template.
// The EA squad coordinates calendars, tasks, and daily briefings.
func ExecutiveAssistantSquad() SquadTemplate {
	return SquadTemplate{
		ID:          "squad-executive-assistant",
		Name:        "Executive Assistant Squad",
		Description: "Manages calendars, tasks, daily briefings, and cross-squad coordination for maximum productivity.",
		Purpose:     "Provide comprehensive personal assistance including scheduling, task management, and daily intelligence briefings.",
		Members: []SquadMember{
			{
				Role:         AgentRolePlanner,
				Name:         "Schedule Planner",
				Description:  "Manages calendar events, resolves conflicts, and optimizes daily schedules.",
				Tools:        []string{"tool.calendar_read", "tool.calendar_write", "tool.conflict_resolver"},
				Capabilities: []string{"calendar.read", "calendar.write", "calendar.optimize"},
			},
			{
				Role:         AgentRoleExecutor,
				Name:         "Task Manager",
				Description:  "Tracks tasks, sets reminders, delegates to other squads, and follows up on pending items.",
				Tools:        []string{"tool.task_create", "tool.task_update", "tool.reminder_set"},
				Capabilities: []string{"task.create", "task.update", "task.delegate", "task.remind"},
			},
			{
				Role:         AgentRoleNarrator,
				Name:         "Briefing Narrator",
				Description:  "Compiles daily briefings from all squad activities, upcoming events, and priority items.",
				Tools:        []string{"tool.text_generation", "tool.data_aggregation"},
				Capabilities: []string{"briefing.compile", "briefing.deliver", "summary.generate"},
			},
			{
				Role:         AgentRoleRouter,
				Name:         "Squad Coordinator",
				Description:  "Routes requests to the appropriate squad and coordinates cross-squad workflows.",
				Tools:        []string{"tool.squad_dispatch", "tool.workflow_trigger"},
				Capabilities: []string{"squad.route", "squad.coordinate", "workflow.trigger"},
			},
		},
		TrustLevel:    2,
		AutonomyLevel: AutonomyLow,
	}
}

// AllSquadTemplates returns all built-in squad templates.
func AllSquadTemplates() []SquadTemplate {
	return []SquadTemplate{
		InboxSquad(),
		ResearchSquad(),
		OpsSquad(),
		SecuritySquad(),
		ExecutiveAssistantSquad(),
	}
}

// SquadFromTemplate creates a workspace-bound Squad from a SquadTemplate.
func SquadFromTemplate(tmpl SquadTemplate, workspaceID string) Squad {
	now := time.Now().UTC()
	return Squad{
		ID:            tmpl.ID,
		Name:          tmpl.Name,
		Description:   tmpl.Description,
		Purpose:       tmpl.Purpose,
		Members:       tmpl.Members,
		WorkspaceID:   workspaceID,
		IsActive:      false,
		TrustLevel:    tmpl.TrustLevel,
		AutonomyLevel: tmpl.AutonomyLevel,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
