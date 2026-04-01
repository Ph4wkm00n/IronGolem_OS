//! SQLite-backed memory store for the knowledge graph in solo/household modes.

use async_trait::async_trait;
use chrono::Utc;
use irongolem_core::types::WorkspaceId;
use irongolem_core::{Error, Result};
use rusqlite::{Connection, params};
use std::sync::Mutex;
use uuid::Uuid;

use crate::graph::{Edge, EdgeKind, Evidence, GraphNode, NodeKind};
use crate::store::MemoryStore;

/// SQLite implementation of [`MemoryStore`].
pub struct SqliteMemoryStore {
    conn: Mutex<Connection>,
}

impl SqliteMemoryStore {
    /// Open or create a SQLite memory store at the given path.
    pub fn open(path: &str) -> Result<Self> {
        let conn = Connection::open(path).map_err(|e| Error::Database(e.to_string()))?;
        let store = Self {
            conn: Mutex::new(conn),
        };
        store.init_db()?;
        Ok(store)
    }

    /// Create an in-memory store (useful for testing).
    pub fn in_memory() -> Result<Self> {
        let conn = Connection::open_in_memory().map_err(|e| Error::Database(e.to_string()))?;
        let store = Self {
            conn: Mutex::new(conn),
        };
        store.init_db()?;
        Ok(store)
    }

    /// Initialize database tables and indexes.
    fn init_db(&self) -> Result<()> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS nodes (
                id TEXT PRIMARY KEY NOT NULL,
                workspace_id TEXT NOT NULL,
                kind TEXT NOT NULL,
                name TEXT NOT NULL,
                description TEXT,
                confidence REAL NOT NULL DEFAULT 1.0,
                freshness TEXT NOT NULL,
                has_contradiction INTEGER NOT NULL DEFAULT 0,
                evidence TEXT NOT NULL DEFAULT '[]',
                metadata TEXT NOT NULL DEFAULT 'null',
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_nodes_workspace
                ON nodes (workspace_id);

            CREATE INDEX IF NOT EXISTS idx_nodes_kind
                ON nodes (workspace_id, kind);

            CREATE TABLE IF NOT EXISTS edges (
                id TEXT PRIMARY KEY NOT NULL,
                source_id TEXT NOT NULL,
                target_id TEXT NOT NULL,
                kind TEXT NOT NULL,
                weight REAL NOT NULL DEFAULT 1.0,
                metadata TEXT NOT NULL DEFAULT 'null',
                created_at TEXT NOT NULL,
                FOREIGN KEY (source_id) REFERENCES nodes(id) ON DELETE CASCADE,
                FOREIGN KEY (target_id) REFERENCES nodes(id) ON DELETE CASCADE
            );

            CREATE INDEX IF NOT EXISTS idx_edges_source
                ON edges (source_id);

            CREATE INDEX IF NOT EXISTS idx_edges_target
                ON edges (target_id);

            -- Virtual FTS table for full-text search on node names.
            CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts
                USING fts5(id, name, content=nodes, content_rowid=rowid);

            -- Triggers to keep FTS in sync with the nodes table.
            CREATE TRIGGER IF NOT EXISTS nodes_ai AFTER INSERT ON nodes BEGIN
                INSERT INTO nodes_fts(rowid, id, name) VALUES (new.rowid, new.id, new.name);
            END;

            CREATE TRIGGER IF NOT EXISTS nodes_ad AFTER DELETE ON nodes BEGIN
                INSERT INTO nodes_fts(nodes_fts, rowid, id, name) VALUES ('delete', old.rowid, old.id, old.name);
            END;

            CREATE TRIGGER IF NOT EXISTS nodes_au AFTER UPDATE ON nodes BEGIN
                INSERT INTO nodes_fts(nodes_fts, rowid, id, name) VALUES ('delete', old.rowid, old.id, old.name);
                INSERT INTO nodes_fts(rowid, id, name) VALUES (new.rowid, new.id, new.name);
            END;

            PRAGMA foreign_keys = ON;",
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    /// Reconstruct a [`GraphNode`] from a row.
    fn row_to_node(row: &rusqlite::Row<'_>) -> std::result::Result<GraphNode, rusqlite::Error> {
        let id_str: String = row.get(0)?;
        let workspace_str: String = row.get(1)?;
        let kind_str: String = row.get(2)?;
        let name: String = row.get(3)?;
        let description: Option<String> = row.get(4)?;
        let confidence: f64 = row.get(5)?;
        let freshness_str: String = row.get(6)?;
        let has_contradiction: bool = row.get(7)?;
        let evidence_json: String = row.get(8)?;
        let metadata_json: String = row.get(9)?;
        let created_str: String = row.get(10)?;
        let updated_str: String = row.get(11)?;

        let map_err = |e: String| {
            rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::from(e))
        };

        let id = Uuid::parse_str(&id_str).map_err(|e| map_err(e.to_string()))?;
        let workspace_id =
            WorkspaceId(Uuid::parse_str(&workspace_str).map_err(|e| map_err(e.to_string()))?);
        let kind: NodeKind = serde_json::from_str(&format!("\"{}\"", kind_str))
            .map_err(|e| map_err(e.to_string()))?;
        let freshness = chrono::DateTime::parse_from_rfc3339(&freshness_str)
            .map_err(|e| map_err(e.to_string()))?
            .with_timezone(&Utc);
        let evidence: Vec<Evidence> =
            serde_json::from_str(&evidence_json).map_err(|e| map_err(e.to_string()))?;
        let metadata: serde_json::Value =
            serde_json::from_str(&metadata_json).map_err(|e| map_err(e.to_string()))?;
        let created_at = chrono::DateTime::parse_from_rfc3339(&created_str)
            .map_err(|e| map_err(e.to_string()))?
            .with_timezone(&Utc);
        let updated_at = chrono::DateTime::parse_from_rfc3339(&updated_str)
            .map_err(|e| map_err(e.to_string()))?
            .with_timezone(&Utc);

        Ok(GraphNode {
            id,
            workspace_id,
            kind,
            name,
            description,
            confidence,
            freshness,
            has_contradiction,
            evidence,
            metadata,
            created_at,
            updated_at,
        })
    }

    /// Reconstruct an [`Edge`] from a row.
    fn row_to_edge(row: &rusqlite::Row<'_>) -> std::result::Result<Edge, rusqlite::Error> {
        let id_str: String = row.get(0)?;
        let source_str: String = row.get(1)?;
        let target_str: String = row.get(2)?;
        let kind_str: String = row.get(3)?;
        let weight: f64 = row.get(4)?;
        let metadata_json: String = row.get(5)?;
        let created_str: String = row.get(6)?;

        let map_err = |e: String| {
            rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::from(e))
        };

        let id = Uuid::parse_str(&id_str).map_err(|e| map_err(e.to_string()))?;
        let source_id = Uuid::parse_str(&source_str).map_err(|e| map_err(e.to_string()))?;
        let target_id = Uuid::parse_str(&target_str).map_err(|e| map_err(e.to_string()))?;
        let kind: EdgeKind = serde_json::from_str(&format!("\"{}\"", kind_str))
            .map_err(|e| map_err(e.to_string()))?;
        let metadata: serde_json::Value =
            serde_json::from_str(&metadata_json).map_err(|e| map_err(e.to_string()))?;
        let created_at = chrono::DateTime::parse_from_rfc3339(&created_str)
            .map_err(|e| map_err(e.to_string()))?
            .with_timezone(&Utc);

        Ok(Edge {
            id,
            source_id,
            target_id,
            kind,
            weight,
            metadata,
            created_at,
        })
    }

    /// Serialize a NodeKind to its serde string representation (snake_case).
    fn kind_to_str(kind: &NodeKind) -> Result<String> {
        let s = serde_json::to_string(kind)?;
        // serde_json wraps in quotes: "\"person\"" -> strip them
        Ok(s.trim_matches('"').to_string())
    }

    /// Serialize an EdgeKind to its serde string representation (snake_case).
    fn edge_kind_to_str(kind: &EdgeKind) -> Result<String> {
        let s = serde_json::to_string(kind)?;
        Ok(s.trim_matches('"').to_string())
    }
}

#[async_trait]
impl MemoryStore for SqliteMemoryStore {
    async fn upsert_node(&self, node: &GraphNode) -> Result<()> {
        let kind_str = Self::kind_to_str(&node.kind)?;
        let evidence_json = serde_json::to_string(&node.evidence)?;
        let metadata_json = serde_json::to_string(&node.metadata)?;

        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        conn.execute(
            "INSERT INTO nodes (id, workspace_id, kind, name, description, confidence, freshness, has_contradiction, evidence, metadata, created_at, updated_at)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)
             ON CONFLICT(id) DO UPDATE SET
                kind = excluded.kind,
                name = excluded.name,
                description = excluded.description,
                confidence = excluded.confidence,
                freshness = excluded.freshness,
                has_contradiction = excluded.has_contradiction,
                evidence = excluded.evidence,
                metadata = excluded.metadata,
                updated_at = excluded.updated_at",
            params![
                node.id.to_string(),
                node.workspace_id.0.to_string(),
                kind_str,
                node.name,
                node.description,
                node.confidence,
                node.freshness.to_rfc3339(),
                node.has_contradiction,
                evidence_json,
                metadata_json,
                node.created_at.to_rfc3339(),
                node.updated_at.to_rfc3339(),
            ],
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    async fn get_node(&self, id: Uuid) -> Result<Option<GraphNode>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, workspace_id, kind, name, description, confidence, freshness, has_contradiction, evidence, metadata, created_at, updated_at
                 FROM nodes WHERE id = ?1",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut rows = stmt
            .query_map(params![id.to_string()], Self::row_to_node)
            .map_err(|e| Error::Database(e.to_string()))?;

        match rows.next() {
            Some(row) => Ok(Some(row.map_err(|e| Error::Database(e.to_string()))?)),
            None => Ok(None),
        }
    }

    async fn find_nodes_by_kind(
        &self,
        workspace_id: WorkspaceId,
        kind: NodeKind,
    ) -> Result<Vec<GraphNode>> {
        let kind_str = Self::kind_to_str(&kind)?;
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, workspace_id, kind, name, description, confidence, freshness, has_contradiction, evidence, metadata, created_at, updated_at
                 FROM nodes
                 WHERE workspace_id = ?1 AND kind = ?2
                 ORDER BY updated_at DESC",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(
                params![workspace_id.0.to_string(), kind_str],
                Self::row_to_node,
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut nodes = Vec::new();
        for row in rows {
            nodes.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(nodes)
    }

    async fn search_nodes(&self, workspace_id: WorkspaceId, query: &str) -> Result<Vec<GraphNode>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;

        // Use FTS5 for full-text search, then join back to nodes for filtering.
        let mut stmt = conn
            .prepare(
                "SELECT n.id, n.workspace_id, n.kind, n.name, n.description, n.confidence, n.freshness, n.has_contradiction, n.evidence, n.metadata, n.created_at, n.updated_at
                 FROM nodes_fts AS f
                 JOIN nodes AS n ON f.id = n.id
                 WHERE nodes_fts MATCH ?1 AND n.workspace_id = ?2
                 ORDER BY rank",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        // FTS5 match expression: wrap each word with * for prefix matching.
        let fts_query = query
            .split_whitespace()
            .map(|w| format!("{}*", w))
            .collect::<Vec<_>>()
            .join(" ");

        let rows = stmt
            .query_map(
                params![fts_query, workspace_id.0.to_string()],
                Self::row_to_node,
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut nodes = Vec::new();
        for row in rows {
            nodes.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(nodes)
    }

    async fn delete_node(&self, id: Uuid) -> Result<()> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        // Delete edges first (in case foreign keys are not enforced), then node.
        conn.execute(
            "DELETE FROM edges WHERE source_id = ?1 OR target_id = ?1",
            params![id.to_string()],
        )
        .map_err(|e| Error::Database(e.to_string()))?;

        conn.execute("DELETE FROM nodes WHERE id = ?1", params![id.to_string()])
            .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    async fn add_edge(&self, edge: &Edge) -> Result<()> {
        let kind_str = Self::edge_kind_to_str(&edge.kind)?;
        let metadata_json = serde_json::to_string(&edge.metadata)?;

        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        conn.execute(
            "INSERT INTO edges (id, source_id, target_id, kind, weight, metadata, created_at)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)",
            params![
                edge.id.to_string(),
                edge.source_id.to_string(),
                edge.target_id.to_string(),
                kind_str,
                edge.weight,
                metadata_json,
                edge.created_at.to_rfc3339(),
            ],
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    async fn get_edges_from(&self, node_id: Uuid) -> Result<Vec<Edge>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, source_id, target_id, kind, weight, metadata, created_at
                 FROM edges WHERE source_id = ?1",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(params![node_id.to_string()], Self::row_to_edge)
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut edges = Vec::new();
        for row in rows {
            edges.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(edges)
    }

    async fn get_edges_to(&self, node_id: Uuid) -> Result<Vec<Edge>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, source_id, target_id, kind, weight, metadata, created_at
                 FROM edges WHERE target_id = ?1",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(params![node_id.to_string()], Self::row_to_edge)
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut edges = Vec::new();
        for row in rows {
            edges.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(edges)
    }

    async fn delete_edge(&self, edge_id: Uuid) -> Result<()> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        conn.execute(
            "DELETE FROM edges WHERE id = ?1",
            params![edge_id.to_string()],
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    async fn find_contradictions(&self, workspace_id: WorkspaceId) -> Result<Vec<GraphNode>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, workspace_id, kind, name, description, confidence, freshness, has_contradiction, evidence, metadata, created_at, updated_at
                 FROM nodes
                 WHERE workspace_id = ?1 AND has_contradiction = 1
                 ORDER BY updated_at DESC",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(params![workspace_id.0.to_string()], Self::row_to_node)
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut nodes = Vec::new();
        for row in rows {
            nodes.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(nodes)
    }

    async fn find_stale_nodes(
        &self,
        workspace_id: WorkspaceId,
        older_than_days: u32,
    ) -> Result<Vec<GraphNode>> {
        let cutoff = Utc::now() - chrono::Duration::days(i64::from(older_than_days));
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, workspace_id, kind, name, description, confidence, freshness, has_contradiction, evidence, metadata, created_at, updated_at
                 FROM nodes
                 WHERE workspace_id = ?1 AND freshness < ?2
                 ORDER BY freshness ASC",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(
                params![workspace_id.0.to_string(), cutoff.to_rfc3339()],
                Self::row_to_node,
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut nodes = Vec::new();
        for row in rows {
            nodes.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(nodes)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::graph::{Edge, EdgeKind, GraphNode, NodeKind};

    #[tokio::test]
    async fn test_upsert_and_get_node() {
        let store = SqliteMemoryStore::in_memory().expect("open in-memory store");
        let ws = WorkspaceId::new();
        let node = GraphNode::new(ws, NodeKind::Person, "Alice");
        let node_id = node.id;

        store.upsert_node(&node).await.expect("upsert");
        let loaded = store
            .get_node(node_id)
            .await
            .expect("get")
            .expect("should exist");
        assert_eq!(loaded.name, "Alice");
    }

    #[tokio::test]
    async fn test_find_by_kind() {
        let store = SqliteMemoryStore::in_memory().expect("open in-memory store");
        let ws = WorkspaceId::new();

        store
            .upsert_node(&GraphNode::new(ws, NodeKind::Person, "Alice"))
            .await
            .expect("upsert");
        store
            .upsert_node(&GraphNode::new(ws, NodeKind::Topic, "Rust"))
            .await
            .expect("upsert");
        store
            .upsert_node(&GraphNode::new(ws, NodeKind::Person, "Bob"))
            .await
            .expect("upsert");

        let people = store
            .find_nodes_by_kind(ws, NodeKind::Person)
            .await
            .expect("find");
        assert_eq!(people.len(), 2);
    }

    #[tokio::test]
    async fn test_search_nodes() {
        let store = SqliteMemoryStore::in_memory().expect("open in-memory store");
        let ws = WorkspaceId::new();

        store
            .upsert_node(&GraphNode::new(ws, NodeKind::Topic, "Rust Programming"))
            .await
            .expect("upsert");
        store
            .upsert_node(&GraphNode::new(ws, NodeKind::Topic, "Go Language"))
            .await
            .expect("upsert");

        let results = store.search_nodes(ws, "Rust").await.expect("search");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].name, "Rust Programming");
    }

    #[tokio::test]
    async fn test_edges() {
        let store = SqliteMemoryStore::in_memory().expect("open in-memory store");
        let ws = WorkspaceId::new();

        let alice = GraphNode::new(ws, NodeKind::Person, "Alice");
        let topic = GraphNode::new(ws, NodeKind::Topic, "Rust");
        store.upsert_node(&alice).await.expect("upsert");
        store.upsert_node(&topic).await.expect("upsert");

        let edge = Edge {
            id: Uuid::new_v4(),
            source_id: alice.id,
            target_id: topic.id,
            kind: EdgeKind::RelatedTo,
            weight: 1.0,
            metadata: serde_json::Value::Null,
            created_at: Utc::now(),
        };
        store.add_edge(&edge).await.expect("add_edge");

        let from = store
            .get_edges_from(alice.id)
            .await
            .expect("get_edges_from");
        assert_eq!(from.len(), 1);

        let to = store.get_edges_to(topic.id).await.expect("get_edges_to");
        assert_eq!(to.len(), 1);

        store.delete_edge(edge.id).await.expect("delete_edge");
        let from_after = store
            .get_edges_from(alice.id)
            .await
            .expect("get_edges_from");
        assert_eq!(from_after.len(), 0);
    }

    #[tokio::test]
    async fn test_delete_node_cascades_edges() {
        let store = SqliteMemoryStore::in_memory().expect("open in-memory store");
        let ws = WorkspaceId::new();

        let alice = GraphNode::new(ws, NodeKind::Person, "Alice");
        let topic = GraphNode::new(ws, NodeKind::Topic, "Rust");
        store.upsert_node(&alice).await.expect("upsert");
        store.upsert_node(&topic).await.expect("upsert");

        let edge = Edge {
            id: Uuid::new_v4(),
            source_id: alice.id,
            target_id: topic.id,
            kind: EdgeKind::RelatedTo,
            weight: 1.0,
            metadata: serde_json::Value::Null,
            created_at: Utc::now(),
        };
        store.add_edge(&edge).await.expect("add_edge");

        store.delete_node(alice.id).await.expect("delete_node");
        let edges = store.get_edges_to(topic.id).await.expect("get_edges_to");
        assert_eq!(edges.len(), 0);
    }

    #[tokio::test]
    async fn test_find_contradictions() {
        let store = SqliteMemoryStore::in_memory().expect("open in-memory store");
        let ws = WorkspaceId::new();

        let mut node = GraphNode::new(ws, NodeKind::Claim, "Earth is flat");
        node.has_contradiction = true;
        store.upsert_node(&node).await.expect("upsert");

        store
            .upsert_node(&GraphNode::new(ws, NodeKind::Claim, "Sky is blue"))
            .await
            .expect("upsert");

        let contradictions = store
            .find_contradictions(ws)
            .await
            .expect("find_contradictions");
        assert_eq!(contradictions.len(), 1);
        assert_eq!(contradictions[0].name, "Earth is flat");
    }
}
