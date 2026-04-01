//! Memory store trait for knowledge graph persistence.

use async_trait::async_trait;
use irongolem_core::Result;
use irongolem_core::types::WorkspaceId;
use uuid::Uuid;

use crate::graph::{Edge, GraphNode, NodeKind};

/// Trait for knowledge graph persistence backends (SQLite or PostgreSQL).
#[async_trait]
pub trait MemoryStore: Send + Sync {
    // -- Node operations --

    /// Insert or update a graph node.
    async fn upsert_node(&self, node: &GraphNode) -> Result<()>;

    /// Get a node by ID.
    async fn get_node(&self, id: Uuid) -> Result<Option<GraphNode>>;

    /// Search nodes by kind within a workspace.
    async fn find_nodes_by_kind(
        &self,
        workspace_id: WorkspaceId,
        kind: NodeKind,
    ) -> Result<Vec<GraphNode>>;

    /// Search nodes by name (partial match).
    async fn search_nodes(&self, workspace_id: WorkspaceId, query: &str) -> Result<Vec<GraphNode>>;

    /// Delete a node and its edges.
    async fn delete_node(&self, id: Uuid) -> Result<()>;

    // -- Edge operations --

    /// Add an edge between two nodes.
    async fn add_edge(&self, edge: &Edge) -> Result<()>;

    /// Get all edges from a node.
    async fn get_edges_from(&self, node_id: Uuid) -> Result<Vec<Edge>>;

    /// Get all edges to a node.
    async fn get_edges_to(&self, node_id: Uuid) -> Result<Vec<Edge>>;

    /// Delete an edge.
    async fn delete_edge(&self, edge_id: Uuid) -> Result<()>;

    // -- Query operations --

    /// Find nodes with contradictions in a workspace.
    async fn find_contradictions(&self, workspace_id: WorkspaceId) -> Result<Vec<GraphNode>>;

    /// Find nodes with stale freshness (older than given duration).
    async fn find_stale_nodes(
        &self,
        workspace_id: WorkspaceId,
        older_than_days: u32,
    ) -> Result<Vec<GraphNode>>;
}
