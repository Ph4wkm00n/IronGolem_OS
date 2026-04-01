//! Event sourcing types. Events are the canonical source of truth for all
//! actions in IronGolem OS. Every significant action produces an immutable event.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::risk::RiskMetadata;
use crate::types::{AgentId, ChannelId, SessionId, UserId, WorkspaceId};

/// An immutable event record in the IronGolem OS event log.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    /// Unique event identifier.
    pub id: Uuid,
    /// When the event occurred.
    pub timestamp: DateTime<Utc>,
    /// Workspace this event belongs to.
    pub workspace_id: WorkspaceId,
    /// User who triggered or is associated with this event.
    pub user_id: Option<UserId>,
    /// Agent that produced this event.
    pub agent_id: Option<AgentId>,
    /// Session context for the event.
    pub session_id: Option<SessionId>,
    /// Channel through which the event originated.
    pub channel_id: Option<ChannelId>,
    /// The kind of event and its payload.
    pub kind: EventKind,
    /// Risk metadata propagated with this event.
    pub risk: Option<RiskMetadata>,
    /// Optional parent event for causal chains.
    pub parent_event_id: Option<Uuid>,
}

impl Event {
    /// Create a new event with the given workspace and kind.
    pub fn new(workspace_id: WorkspaceId, kind: EventKind) -> Self {
        Self {
            id: Uuid::new_v4(),
            timestamp: Utc::now(),
            workspace_id,
            user_id: None,
            agent_id: None,
            session_id: None,
            channel_id: None,
            kind,
            risk: None,
            parent_event_id: None,
        }
    }

    pub fn with_user(mut self, user_id: UserId) -> Self {
        self.user_id = Some(user_id);
        self
    }

    pub fn with_agent(mut self, agent_id: AgentId) -> Self {
        self.agent_id = Some(agent_id);
        self
    }

    pub fn with_session(mut self, session_id: SessionId) -> Self {
        self.session_id = Some(session_id);
        self
    }

    pub fn with_channel(mut self, channel_id: ChannelId) -> Self {
        self.channel_id = Some(channel_id);
        self
    }

    pub fn with_risk(mut self, risk: RiskMetadata) -> Self {
        self.risk = Some(risk);
        self
    }

    pub fn with_parent(mut self, parent_id: Uuid) -> Self {
        self.parent_event_id = Some(parent_id);
        self
    }
}

/// Categories of events in the system.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", content = "data")]
pub enum EventKind {
    // -- Agent actions --
    PlanCreated {
        plan_id: Uuid,
        description: String,
    },
    PlanStepStarted {
        plan_id: Uuid,
        step_id: Uuid,
    },
    PlanStepCompleted {
        plan_id: Uuid,
        step_id: Uuid,
        output: serde_json::Value,
    },
    PlanStepFailed {
        plan_id: Uuid,
        step_id: Uuid,
        error: String,
    },
    PlanCompleted {
        plan_id: Uuid,
    },
    PlanRolledBack {
        plan_id: Uuid,
        to_step: Uuid,
    },
    ToolCalled {
        tool_name: String,
        input: serde_json::Value,
    },
    ToolResult {
        tool_name: String,
        output: serde_json::Value,
    },
    ApprovalRequested {
        action_description: String,
        risk_level: String,
    },
    ApprovalGranted {
        request_id: Uuid,
    },
    ApprovalDenied {
        request_id: Uuid,
        reason: String,
    },

    // -- System events --
    HeartbeatEmitted {
        service: String,
        status: HeartbeatStatus,
    },
    RecoveryStarted {
        service: String,
        reason: String,
    },
    RecoveryCompleted {
        service: String,
        strategy: String,
    },
    RecoveryFailed {
        service: String,
        error: String,
    },
    ConfigChanged {
        key: String,
        previous: serde_json::Value,
        current: serde_json::Value,
    },
    CheckpointCreated {
        checkpoint_id: Uuid,
        plan_id: Uuid,
    },

    // -- User actions --
    RecipeActivated {
        recipe_id: String,
        name: String,
    },
    RecipeDeactivated {
        recipe_id: String,
    },
    PreferenceUpdated {
        key: String,
        value: serde_json::Value,
    },

    // -- Security events --
    ActionBlocked {
        action: String,
        reason: String,
        policy_layer: u8,
    },
    QuarantineTriggered {
        target: String,
        reason: String,
    },
    InjectionDetected {
        source: String,
        confidence: f64,
    },

    // -- Research events --
    SourceFetched {
        topic: String,
        source_url: String,
        trust_score: f64,
    },
    ContradictionDetected {
        topic: String,
        claim_a: String,
        claim_b: String,
    },
    BriefPublished {
        topic: String,
        summary: String,
    },

    // -- Connector events --
    ConnectorConnected {
        connector_id: String,
        connector_type: String,
    },
    ConnectorDisconnected {
        connector_id: String,
        reason: String,
    },
    MessageReceived {
        connector_id: String,
        message_type: String,
    },
    MessageSent {
        connector_id: String,
        message_type: String,
    },
}

/// Health status for heartbeat events.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum HeartbeatStatus {
    Healthy,
    QuietlyRecovering,
    NeedsAttention,
    Paused,
    Quarantined,
}

impl std::fmt::Display for HeartbeatStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Healthy => write!(f, "Healthy"),
            Self::QuietlyRecovering => write!(f, "Quietly Recovering"),
            Self::NeedsAttention => write!(f, "Needs Attention"),
            Self::Paused => write!(f, "Paused"),
            Self::Quarantined => write!(f, "Quarantined"),
        }
    }
}
