//! Event store trait and SQLite implementation.
//!
//! The `EventStore` trait abstracts event persistence, and `SqliteEventStore`
//! provides the concrete SQLite-backed implementation for solo/household mode.

use async_trait::async_trait;
use rusqlite::{params, Connection};
use std::sync::Mutex;
use uuid::Uuid;

use crate::event::{Event, EventKind};
use crate::risk::RiskMetadata;
use crate::types::{AgentId, ChannelId, SessionId, UserId, WorkspaceId};
use crate::{Error, Result};

/// Trait for event log persistence backends.
#[async_trait]
pub trait EventStore: Send + Sync {
    /// Append an event to the log.
    async fn append(&self, event: &Event) -> Result<()>;

    /// List events for a workspace, newest first, up to `limit`.
    async fn list_for_workspace(
        &self,
        workspace_id: WorkspaceId,
        limit: usize,
    ) -> Result<Vec<Event>>;

    /// Get a single event by ID.
    async fn get(&self, event_id: Uuid) -> Result<Option<Event>>;

    /// List events for a workspace filtered by event kind discriminant, newest first.
    async fn list_by_kind(
        &self,
        workspace_id: WorkspaceId,
        kind_filter: &str,
        limit: usize,
    ) -> Result<Vec<Event>>;
}

/// SQLite-backed event store for solo and household deployment modes.
pub struct SqliteEventStore {
    conn: Mutex<Connection>,
}

impl SqliteEventStore {
    /// Open (or create) the event store at the given database path.
    pub fn open(path: &str) -> Result<Self> {
        let conn = Connection::open(path).map_err(|e| Error::Database(e.to_string()))?;
        let store = Self {
            conn: Mutex::new(conn),
        };
        store.init_db()?;
        Ok(store)
    }

    /// Create an in-memory event store (useful for tests).
    pub fn open_in_memory() -> Result<Self> {
        let conn = Connection::open_in_memory().map_err(|e| Error::Database(e.to_string()))?;
        let store = Self {
            conn: Mutex::new(conn),
        };
        store.init_db()?;
        Ok(store)
    }

    /// Create the events table and indexes if they do not exist.
    fn init_db(&self) -> Result<()> {
        let conn = self.conn.lock().map_err(|e| Error::Database(e.to_string()))?;
        conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS events (
                id              TEXT PRIMARY KEY NOT NULL,
                timestamp       TEXT NOT NULL,
                workspace_id    TEXT NOT NULL,
                user_id         TEXT,
                agent_id        TEXT,
                session_id      TEXT,
                channel_id      TEXT,
                kind            TEXT NOT NULL,
                risk            TEXT,
                parent_event_id TEXT
            );
            CREATE INDEX IF NOT EXISTS idx_events_workspace
                ON events(workspace_id, timestamp DESC);
            CREATE INDEX IF NOT EXISTS idx_events_kind
                ON events(workspace_id, kind);",
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    /// Extract the serde tag value (event kind discriminant) from a serialized `EventKind`.
    fn kind_tag(kind: &EventKind) -> Result<String> {
        let v = serde_json::to_value(kind)?;
        match v.get("type").and_then(|t| t.as_str()) {
            Some(t) => Ok(t.to_string()),
            None => Ok("unknown".to_string()),
        }
    }

    fn row_to_event(row: &rusqlite::Row<'_>) -> std::result::Result<Event, rusqlite::Error> {
        let id_str: String = row.get(0)?;
        let ts_str: String = row.get(1)?;
        let ws_str: String = row.get(2)?;
        let user_str: Option<String> = row.get(3)?;
        let agent_str: Option<String> = row.get(4)?;
        let session_str: Option<String> = row.get(5)?;
        let channel_str: Option<String> = row.get(6)?;
        let kind_json: String = row.get(7)?;
        let risk_json: Option<String> = row.get(8)?;
        let parent_str: Option<String> = row.get(9)?;

        let parse_uuid = |s: &str| -> std::result::Result<Uuid, rusqlite::Error> {
            Uuid::parse_str(s).map_err(|e| {
                rusqlite::Error::FromSqlConversionFailure(
                    0,
                    rusqlite::types::Type::Text,
                    Box::new(e),
                )
            })
        };

        let id = parse_uuid(&id_str)?;
        let timestamp = chrono::DateTime::parse_from_rfc3339(&ts_str)
            .map(|dt| dt.with_timezone(&chrono::Utc))
            .map_err(|e| {
                rusqlite::Error::FromSqlConversionFailure(
                    1,
                    rusqlite::types::Type::Text,
                    Box::new(e),
                )
            })?;
        let workspace_id = WorkspaceId(parse_uuid(&ws_str)?);
        let user_id = user_str.map(|s| parse_uuid(&s)).transpose()?.map(UserId);
        let agent_id = agent_str
            .map(|s| parse_uuid(&s))
            .transpose()?
            .map(AgentId);
        let session_id = session_str
            .map(|s| parse_uuid(&s))
            .transpose()?
            .map(SessionId);
        let channel_id = channel_str
            .map(|s| parse_uuid(&s))
            .transpose()?
            .map(ChannelId);

        let kind: EventKind = serde_json::from_str(&kind_json).map_err(|e| {
            rusqlite::Error::FromSqlConversionFailure(
                7,
                rusqlite::types::Type::Text,
                Box::new(e),
            )
        })?;

        let risk: Option<RiskMetadata> = risk_json
            .map(|s| serde_json::from_str(&s))
            .transpose()
            .map_err(|e| {
                rusqlite::Error::FromSqlConversionFailure(
                    8,
                    rusqlite::types::Type::Text,
                    Box::new(e),
                )
            })?;

        let parent_event_id = parent_str.map(|s| parse_uuid(&s)).transpose()?;

        Ok(Event {
            id,
            timestamp,
            workspace_id,
            user_id,
            agent_id,
            session_id,
            channel_id,
            kind,
            risk,
            parent_event_id,
        })
    }
}

#[async_trait]
impl EventStore for SqliteEventStore {
    async fn append(&self, event: &Event) -> Result<()> {
        let kind_json = serde_json::to_string(&event.kind)?;
        let risk_json = event
            .risk
            .as_ref()
            .map(serde_json::to_string)
            .transpose()?;

        let conn = self.conn.lock().map_err(|e| Error::Database(e.to_string()))?;
        conn.execute(
            "INSERT INTO events (id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)",
            params![
                event.id.to_string(),
                event.timestamp.to_rfc3339(),
                event.workspace_id.0.to_string(),
                event.user_id.map(|u| u.0.to_string()),
                event.agent_id.map(|a| a.0.to_string()),
                event.session_id.map(|s| s.0.to_string()),
                event.channel_id.map(|c| c.0.to_string()),
                kind_json,
                risk_json,
                event.parent_event_id.map(|p| p.to_string()),
            ],
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    async fn list_for_workspace(
        &self,
        workspace_id: WorkspaceId,
        limit: usize,
    ) -> Result<Vec<Event>> {
        let conn = self.conn.lock().map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id
                 FROM events
                 WHERE workspace_id = ?1
                 ORDER BY timestamp DESC
                 LIMIT ?2",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(params![workspace_id.0.to_string(), limit as i64], Self::row_to_event)
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut events = Vec::new();
        for row in rows {
            events.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(events)
    }

    async fn get(&self, event_id: Uuid) -> Result<Option<Event>> {
        let conn = self.conn.lock().map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id
                 FROM events
                 WHERE id = ?1",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut rows = stmt
            .query_map(params![event_id.to_string()], Self::row_to_event)
            .map_err(|e| Error::Database(e.to_string()))?;

        match rows.next() {
            Some(row) => Ok(Some(row.map_err(|e| Error::Database(e.to_string()))?)),
            None => Ok(None),
        }
    }

    async fn list_by_kind(
        &self,
        workspace_id: WorkspaceId,
        kind_filter: &str,
        limit: usize,
    ) -> Result<Vec<Event>> {
        // We store the full JSON of the EventKind; the serde tag field is "type".
        // We use a JSON extract to match the discriminant.
        let conn = self.conn.lock().map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id
                 FROM events
                 WHERE workspace_id = ?1 AND json_extract(kind, '$.type') = ?2
                 ORDER BY timestamp DESC
                 LIMIT ?3",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(
                params![workspace_id.0.to_string(), kind_filter, limit as i64],
                Self::row_to_event,
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut events = Vec::new();
        for row in rows {
            events.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(events)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::event::EventKind;
    use crate::types::WorkspaceId;

    #[tokio::test]
    async fn test_append_and_get() {
        let store = SqliteEventStore::open_in_memory().unwrap();
        let ws = WorkspaceId::new();
        let event = Event::new(
            ws,
            EventKind::RecipeActivated {
                recipe_id: "r1".into(),
                name: "Test Recipe".into(),
            },
        );
        let id = event.id;
        store.append(&event).await.unwrap();

        let fetched = store.get(id).await.unwrap();
        assert!(fetched.is_some());
        assert_eq!(fetched.unwrap().id, id);
    }

    #[tokio::test]
    async fn test_list_for_workspace() {
        let store = SqliteEventStore::open_in_memory().unwrap();
        let ws = WorkspaceId::new();
        for i in 0..5 {
            let event = Event::new(
                ws,
                EventKind::RecipeActivated {
                    recipe_id: format!("r{i}"),
                    name: format!("Recipe {i}"),
                },
            );
            store.append(&event).await.unwrap();
        }

        let events = store.list_for_workspace(ws, 3).await.unwrap();
        assert_eq!(events.len(), 3);
    }

    #[tokio::test]
    async fn test_list_by_kind() {
        let store = SqliteEventStore::open_in_memory().unwrap();
        let ws = WorkspaceId::new();
        store
            .append(&Event::new(
                ws,
                EventKind::RecipeActivated {
                    recipe_id: "r1".into(),
                    name: "Test".into(),
                },
            ))
            .await
            .unwrap();
        store
            .append(&Event::new(
                ws,
                EventKind::RecipeDeactivated {
                    recipe_id: "r2".into(),
                },
            ))
            .await
            .unwrap();

        let events = store
            .list_by_kind(ws, "RecipeActivated", 10)
            .await
            .unwrap();
        assert_eq!(events.len(), 1);
    }
}
