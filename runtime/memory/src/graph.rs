//! Knowledge graph types. Nodes represent entities (people, topics, sources,
//! tasks) and edges represent relationships between them.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

use irongolem_core::types::WorkspaceId;

/// A node in the knowledge graph.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GraphNode {
    /// Unique node identifier.
    pub id: Uuid,
    /// Workspace this node belongs to.
    pub workspace_id: WorkspaceId,
    /// What kind of entity this node represents.
    pub kind: NodeKind,
    /// Human-readable name/label.
    pub name: String,
    /// Optional description or summary.
    pub description: Option<String>,
    /// Confidence score for this node's information (0.0 to 1.0).
    pub confidence: f64,
    /// When this information was last verified or updated.
    pub freshness: DateTime<Utc>,
    /// Whether contradicting information has been detected.
    pub has_contradiction: bool,
    /// Source evidence links supporting this node.
    pub evidence: Vec<Evidence>,
    /// Arbitrary metadata.
    pub metadata: serde_json::Value,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

impl GraphNode {
    pub fn new(workspace_id: WorkspaceId, kind: NodeKind, name: impl Into<String>) -> Self {
        let now = Utc::now();
        Self {
            id: Uuid::new_v4(),
            workspace_id,
            kind,
            name: name.into(),
            description: None,
            confidence: 1.0,
            freshness: now,
            has_contradiction: false,
            evidence: Vec::new(),
            metadata: serde_json::Value::Null,
            created_at: now,
            updated_at: now,
        }
    }
}

/// Categories of graph nodes.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NodeKind {
    /// A person (contact, team member).
    Person,
    /// A topic being tracked or researched.
    Topic,
    /// An information source (URL, document, API).
    Source,
    /// A task or action item.
    Task,
    /// A user preference learned from behavior.
    Preference,
    /// A research finding or claim.
    Claim,
    /// An organization or company.
    Organization,
}

/// An edge connecting two graph nodes.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Edge {
    pub id: Uuid,
    pub source_id: Uuid,
    pub target_id: Uuid,
    pub kind: EdgeKind,
    pub weight: f64,
    pub metadata: serde_json::Value,
    pub created_at: DateTime<Utc>,
}

/// Categories of relationships between nodes.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum EdgeKind {
    /// A person is related to a topic.
    RelatedTo,
    /// A source supports a claim.
    Supports,
    /// A source contradicts a claim.
    Contradicts,
    /// A preference was learned from an event.
    LearnedFrom,
    /// A task is assigned to a person.
    AssignedTo,
    /// An entity belongs to an organization.
    BelongsTo,
    /// A topic is a subtopic of another.
    SubtopicOf,
    /// A claim cites a source.
    CitedFrom,
}

/// Evidence linking a graph node to its source material.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Evidence {
    /// Where this evidence came from.
    pub source: String,
    /// When this evidence was collected.
    pub collected_at: DateTime<Utc>,
    /// Relevant excerpt or summary.
    pub excerpt: Option<String>,
    /// Trust score of the source (0.0 to 1.0).
    pub trust_score: f64,
}
