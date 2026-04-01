//! Plan graph types. A plan is a directed acyclic graph of steps that represent
//! an agent's execution workflow. Each node can be a tool call, LLM call,
//! approval gate, or delegation to another agent.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::risk::RiskMetadata;
use crate::types::AgentId;

/// A plan graph representing an agent's execution workflow.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Plan {
    /// Unique plan identifier.
    pub id: Uuid,
    /// Human-readable description of what this plan does.
    pub description: String,
    /// Agent that owns this plan.
    pub agent_id: AgentId,
    /// Ordered list of nodes in the plan graph.
    pub nodes: Vec<PlanNode>,
    /// Current execution status.
    pub status: PlanStatus,
    /// Risk metadata for the overall plan.
    pub risk: RiskMetadata,
    /// When the plan was created.
    pub created_at: DateTime<Utc>,
    /// When the plan was last updated.
    pub updated_at: DateTime<Utc>,
}

impl Plan {
    /// Create a new plan with the given description and agent.
    pub fn new(description: impl Into<String>, agent_id: AgentId) -> Self {
        let now = Utc::now();
        Self {
            id: Uuid::new_v4(),
            description: description.into(),
            agent_id,
            nodes: Vec::new(),
            status: PlanStatus::Pending,
            risk: RiskMetadata::default(),
            created_at: now,
            updated_at: now,
        }
    }

    /// Add a node to the plan.
    pub fn add_node(&mut self, node: PlanNode) {
        self.nodes.push(node);
        self.updated_at = Utc::now();
    }

    /// Find a node by its ID.
    pub fn find_node(&self, node_id: Uuid) -> Option<&PlanNode> {
        self.nodes.iter().find(|n| n.id == node_id)
    }

    /// Find a mutable node by its ID.
    pub fn find_node_mut(&mut self, node_id: Uuid) -> Option<&mut PlanNode> {
        self.nodes.iter_mut().find(|n| n.id == node_id)
    }

    /// Get the next pending node to execute.
    pub fn next_pending_node(&self) -> Option<&PlanNode> {
        self.nodes
            .iter()
            .find(|n| n.status == PlanNodeStatus::Pending)
    }
}

/// A single step in a plan graph.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlanNode {
    /// Unique node identifier.
    pub id: Uuid,
    /// Human-readable description of this step.
    pub description: String,
    /// What kind of action this step performs.
    pub kind: PlanNodeKind,
    /// Execution status of this node.
    pub status: PlanNodeStatus,
    /// IDs of nodes that must complete before this one can execute.
    pub dependencies: Vec<Uuid>,
    /// Risk metadata for this specific step.
    pub risk: RiskMetadata,
    /// Output produced by this step, if completed.
    pub output: Option<serde_json::Value>,
    /// Error message if this step failed.
    pub error: Option<String>,
}

impl PlanNode {
    pub fn new(description: impl Into<String>, kind: PlanNodeKind) -> Self {
        Self {
            id: Uuid::new_v4(),
            description: description.into(),
            kind,
            status: PlanNodeStatus::Pending,
            dependencies: Vec::new(),
            risk: RiskMetadata::default(),
            output: None,
            error: None,
        }
    }

    pub fn with_dependency(mut self, dep_id: Uuid) -> Self {
        self.dependencies.push(dep_id);
        self
    }
}

/// The kind of action a plan node represents.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum PlanNodeKind {
    /// Call an external tool.
    ToolCall {
        tool_name: String,
        input: serde_json::Value,
    },
    /// Make an LLM inference call.
    LlmCall {
        prompt: String,
        model: Option<String>,
    },
    /// Wait for user approval before proceeding.
    ApprovalGate { description: String },
    /// Delegate to another agent.
    Delegation { target_agent: AgentId, goal: String },
    /// Run a verification check on previous output.
    Verify { target_node_id: Uuid },
    /// Create a checkpoint for potential rollback.
    Checkpoint,
}

/// Execution status of a plan node.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PlanNodeStatus {
    Pending,
    Running,
    WaitingApproval,
    Completed,
    Failed,
    Skipped,
    RolledBack,
}

/// Overall status of a plan.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PlanStatus {
    Pending,
    Running,
    Paused,
    Completed,
    Failed,
    RolledBack,
}
