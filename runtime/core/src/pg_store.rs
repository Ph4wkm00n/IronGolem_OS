//! PostgreSQL event store for team deployment mode.
//!
//! `PgEventStore` implements the [`EventStore`] trait using PostgreSQL as the
//! backend, targeting multi-tenant team deployments. The schema mirrors the
//! SQLite event store but uses native PostgreSQL types (UUID, TIMESTAMPTZ,
//! JSONB) for better performance and richer query capabilities.
//!
//! This module is gated behind the `postgres` feature flag.

/// SQL schema for the PostgreSQL events table.
///
/// Uses native PostgreSQL types for optimal storage and querying:
/// - `UUID` for all identifier columns
/// - `TIMESTAMPTZ` for timestamps with timezone awareness
/// - `JSONB` for kind and risk metadata (supports indexing and querying)
pub const PG_SCHEMA: &str = r#"
CREATE TABLE IF NOT EXISTS events (
    id              UUID PRIMARY KEY NOT NULL,
    timestamp       TIMESTAMPTZ NOT NULL,
    workspace_id    UUID NOT NULL,
    user_id         UUID,
    agent_id        UUID,
    session_id      UUID,
    channel_id      UUID,
    kind            JSONB NOT NULL,
    risk            JSONB,
    parent_event_id UUID
);

CREATE INDEX IF NOT EXISTS idx_events_workspace
    ON events(workspace_id, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_events_kind
    ON events(workspace_id, (kind->>'type'));

CREATE INDEX IF NOT EXISTS idx_events_timestamp
    ON events(timestamp DESC);
"#;

#[cfg(feature = "postgres")]
mod impl_pg {
    use async_trait::async_trait;
    use postgres::{Client, NoTls};
    use std::sync::Mutex;
    use uuid::Uuid;

    use crate::event::{Event, EventKind};
    use crate::risk::RiskMetadata;
    use crate::store::EventStore;
    use crate::types::{AgentId, ChannelId, SessionId, UserId, WorkspaceId};
    use crate::{Error, Result};

    use super::PG_SCHEMA;

    /// PostgreSQL-backed event store for team deployment mode.
    ///
    /// Uses a synchronous `postgres::Client` behind a `Mutex`, mirroring the
    /// approach used by [`SqliteEventStore`](crate::store::SqliteEventStore).
    /// For production workloads, consider migrating to `tokio-postgres` with
    /// a connection pool (e.g., `deadpool-postgres`).
    pub struct PgEventStore {
        client: Mutex<Client>,
    }

    impl PgEventStore {
        /// Connect to a PostgreSQL database and initialize the schema.
        ///
        /// # Arguments
        /// * `connection_string` - A PostgreSQL connection string, e.g.
        ///   `"host=localhost user=irongolem dbname=irongolem_events"`
        ///
        /// # Errors
        /// Returns `Error::Database` if the connection or schema initialization fails.
        pub fn connect(connection_string: &str) -> Result<Self> {
            let client = Client::connect(connection_string, NoTls)
                .map_err(|e| Error::Database(e.to_string()))?;
            let store = Self {
                client: Mutex::new(client),
            };
            store.init_db()?;
            Ok(store)
        }

        /// Create the events table and indexes if they do not exist.
        fn init_db(&self) -> Result<()> {
            let mut client = self
                .client
                .lock()
                .map_err(|e| Error::Database(e.to_string()))?;
            client
                .batch_execute(PG_SCHEMA)
                .map_err(|e| Error::Database(e.to_string()))?;
            Ok(())
        }

        /// Convert a PostgreSQL row into an `Event`.
        fn row_to_event(row: &postgres::Row) -> Result<Event> {
            let id: Uuid = row.try_get("id").map_err(|e| Error::Database(e.to_string()))?;
            let timestamp: chrono::DateTime<chrono::Utc> =
                row.try_get("timestamp").map_err(|e| Error::Database(e.to_string()))?;
            let workspace_id: Uuid = row
                .try_get("workspace_id")
                .map_err(|e| Error::Database(e.to_string()))?;
            let user_id: Option<Uuid> =
                row.try_get("user_id").map_err(|e| Error::Database(e.to_string()))?;
            let agent_id: Option<Uuid> =
                row.try_get("agent_id").map_err(|e| Error::Database(e.to_string()))?;
            let session_id: Option<Uuid> = row
                .try_get("session_id")
                .map_err(|e| Error::Database(e.to_string()))?;
            let channel_id: Option<Uuid> = row
                .try_get("channel_id")
                .map_err(|e| Error::Database(e.to_string()))?;
            let kind_json: serde_json::Value =
                row.try_get("kind").map_err(|e| Error::Database(e.to_string()))?;
            let risk_json: Option<serde_json::Value> =
                row.try_get("risk").map_err(|e| Error::Database(e.to_string()))?;
            let parent_event_id: Option<Uuid> = row
                .try_get("parent_event_id")
                .map_err(|e| Error::Database(e.to_string()))?;

            let kind: EventKind = serde_json::from_value(kind_json)?;
            let risk: Option<RiskMetadata> =
                risk_json.map(serde_json::from_value).transpose()?;

            Ok(Event {
                id,
                timestamp,
                workspace_id: WorkspaceId(workspace_id),
                user_id: user_id.map(UserId),
                agent_id: agent_id.map(AgentId),
                session_id: session_id.map(SessionId),
                channel_id: channel_id.map(ChannelId),
                kind,
                risk,
                parent_event_id,
            })
        }
    }

    #[async_trait]
    impl EventStore for PgEventStore {
        async fn append(&self, event: &Event) -> Result<()> {
            let kind_json = serde_json::to_value(&event.kind)?;
            let risk_json = event
                .risk
                .as_ref()
                .map(serde_json::to_value)
                .transpose()?;

            let mut client = self
                .client
                .lock()
                .map_err(|e| Error::Database(e.to_string()))?;
            client
                .execute(
                    "INSERT INTO events (id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id)
                     VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
                    &[
                        &event.id,
                        &event.timestamp,
                        &event.workspace_id.0,
                        &event.user_id.map(|u| u.0),
                        &event.agent_id.map(|a| a.0),
                        &event.session_id.map(|s| s.0),
                        &event.channel_id.map(|c| c.0),
                        &kind_json,
                        &risk_json,
                        &event.parent_event_id,
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
            let client = self
                .client
                .lock()
                .map_err(|e| Error::Database(e.to_string()))?;
            let rows = client
                .query(
                    "SELECT id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id
                     FROM events
                     WHERE workspace_id = $1
                     ORDER BY timestamp DESC
                     LIMIT $2",
                    &[&workspace_id.0, &(limit as i64)],
                )
                .map_err(|e| Error::Database(e.to_string()))?;

            let mut events = Vec::with_capacity(rows.len());
            for row in &rows {
                events.push(Self::row_to_event(row)?);
            }
            Ok(events)
        }

        async fn get(&self, event_id: Uuid) -> Result<Option<Event>> {
            let client = self
                .client
                .lock()
                .map_err(|e| Error::Database(e.to_string()))?;
            let rows = client
                .query(
                    "SELECT id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id
                     FROM events
                     WHERE id = $1",
                    &[&event_id],
                )
                .map_err(|e| Error::Database(e.to_string()))?;

            match rows.first() {
                Some(row) => Ok(Some(Self::row_to_event(row)?)),
                None => Ok(None),
            }
        }

        async fn list_by_kind(
            &self,
            workspace_id: WorkspaceId,
            kind_filter: &str,
            limit: usize,
        ) -> Result<Vec<Event>> {
            let client = self
                .client
                .lock()
                .map_err(|e| Error::Database(e.to_string()))?;
            // Use the JSONB ->> operator to extract the serde tag discriminant.
            let rows = client
                .query(
                    "SELECT id, timestamp, workspace_id, user_id, agent_id, session_id, channel_id, kind, risk, parent_event_id
                     FROM events
                     WHERE workspace_id = $1 AND kind->>'type' = $2
                     ORDER BY timestamp DESC
                     LIMIT $3",
                    &[&workspace_id.0, &kind_filter, &(limit as i64)],
                )
                .map_err(|e| Error::Database(e.to_string()))?;

            let mut events = Vec::with_capacity(rows.len());
            for row in &rows {
                events.push(Self::row_to_event(row)?);
            }
            Ok(events)
        }
    }
}

#[cfg(feature = "postgres")]
pub use impl_pg::PgEventStore;
