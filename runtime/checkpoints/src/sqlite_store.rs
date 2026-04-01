//! SQLite-backed checkpoint store for solo and household deployment modes.

use async_trait::async_trait;
use chrono::{DateTime, Utc};
use irongolem_core::{Error, Result};
use rusqlite::{Connection, params};
use std::sync::Mutex;
use uuid::Uuid;

use crate::store::{Checkpoint, CheckpointStore};

/// SQLite-backed implementation of `CheckpointStore`.
pub struct SqliteCheckpointStore {
    conn: Mutex<Connection>,
}

impl SqliteCheckpointStore {
    /// Open (or create) the checkpoint store at the given database path.
    pub fn open(path: &str) -> Result<Self> {
        let conn = Connection::open(path).map_err(|e| Error::Database(e.to_string()))?;
        let store = Self {
            conn: Mutex::new(conn),
        };
        store.init_db()?;
        Ok(store)
    }

    /// Create an in-memory checkpoint store (useful for tests).
    pub fn open_in_memory() -> Result<Self> {
        let conn = Connection::open_in_memory().map_err(|e| Error::Database(e.to_string()))?;
        let store = Self {
            conn: Mutex::new(conn),
        };
        store.init_db()?;
        Ok(store)
    }

    /// Create the checkpoints table and indexes if they do not exist.
    fn init_db(&self) -> Result<()> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS checkpoints (
                id                  TEXT PRIMARY KEY NOT NULL,
                plan_id             TEXT NOT NULL,
                last_completed_step TEXT,
                plan_state          TEXT NOT NULL,
                created_at          TEXT NOT NULL,
                label               TEXT
            );
            CREATE INDEX IF NOT EXISTS idx_checkpoints_plan_id
                ON checkpoints(plan_id, created_at DESC);",
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    fn row_to_checkpoint(
        row: &rusqlite::Row<'_>,
    ) -> std::result::Result<Checkpoint, rusqlite::Error> {
        let id_str: String = row.get(0)?;
        let plan_id_str: String = row.get(1)?;
        let step_str: Option<String> = row.get(2)?;
        let state_json: String = row.get(3)?;
        let created_str: String = row.get(4)?;
        let label: Option<String> = row.get(5)?;

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
        let plan_id = parse_uuid(&plan_id_str)?;
        let last_completed_step = step_str.map(|s| parse_uuid(&s)).transpose()?;
        let plan_state: serde_json::Value = serde_json::from_str(&state_json).map_err(|e| {
            rusqlite::Error::FromSqlConversionFailure(3, rusqlite::types::Type::Text, Box::new(e))
        })?;
        let created_at: DateTime<Utc> = chrono::DateTime::parse_from_rfc3339(&created_str)
            .map(|dt| dt.with_timezone(&Utc))
            .map_err(|e| {
                rusqlite::Error::FromSqlConversionFailure(
                    4,
                    rusqlite::types::Type::Text,
                    Box::new(e),
                )
            })?;

        Ok(Checkpoint {
            id,
            plan_id,
            last_completed_step,
            plan_state,
            created_at,
            label,
        })
    }
}

#[async_trait]
impl CheckpointStore for SqliteCheckpointStore {
    async fn save(&self, checkpoint: &Checkpoint) -> Result<()> {
        let state_json = serde_json::to_string(&checkpoint.plan_state)?;
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        conn.execute(
            "INSERT OR REPLACE INTO checkpoints (id, plan_id, last_completed_step, plan_state, created_at, label)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            params![
                checkpoint.id.to_string(),
                checkpoint.plan_id.to_string(),
                checkpoint.last_completed_step.map(|s| s.to_string()),
                state_json,
                checkpoint.created_at.to_rfc3339(),
                checkpoint.label,
            ],
        )
        .map_err(|e| Error::Database(e.to_string()))?;
        Ok(())
    }

    async fn load_latest(&self, plan_id: Uuid) -> Result<Option<Checkpoint>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, plan_id, last_completed_step, plan_state, created_at, label
                 FROM checkpoints
                 WHERE plan_id = ?1
                 ORDER BY created_at DESC
                 LIMIT 1",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut rows = stmt
            .query_map(params![plan_id.to_string()], Self::row_to_checkpoint)
            .map_err(|e| Error::Database(e.to_string()))?;

        match rows.next() {
            Some(row) => Ok(Some(row.map_err(|e| Error::Database(e.to_string()))?)),
            None => Ok(None),
        }
    }

    async fn load(&self, checkpoint_id: Uuid) -> Result<Option<Checkpoint>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, plan_id, last_completed_step, plan_state, created_at, label
                 FROM checkpoints
                 WHERE id = ?1",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut rows = stmt
            .query_map(params![checkpoint_id.to_string()], Self::row_to_checkpoint)
            .map_err(|e| Error::Database(e.to_string()))?;

        match rows.next() {
            Some(row) => Ok(Some(row.map_err(|e| Error::Database(e.to_string()))?)),
            None => Ok(None),
        }
    }

    async fn list_for_plan(&self, plan_id: Uuid) -> Result<Vec<Checkpoint>> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;
        let mut stmt = conn
            .prepare(
                "SELECT id, plan_id, last_completed_step, plan_state, created_at, label
                 FROM checkpoints
                 WHERE plan_id = ?1
                 ORDER BY created_at DESC",
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        let rows = stmt
            .query_map(params![plan_id.to_string()], Self::row_to_checkpoint)
            .map_err(|e| Error::Database(e.to_string()))?;

        let mut checkpoints = Vec::new();
        for row in rows {
            checkpoints.push(row.map_err(|e| Error::Database(e.to_string()))?);
        }
        Ok(checkpoints)
    }

    async fn prune(&self, plan_id: Uuid, keep_latest: usize) -> Result<usize> {
        let conn = self
            .conn
            .lock()
            .map_err(|e| Error::Database(e.to_string()))?;

        // Delete all checkpoints for this plan except the N most recent.
        let deleted = conn
            .execute(
                "DELETE FROM checkpoints
                 WHERE plan_id = ?1
                   AND id NOT IN (
                       SELECT id FROM checkpoints
                       WHERE plan_id = ?1
                       ORDER BY created_at DESC
                       LIMIT ?2
                   )",
                params![plan_id.to_string(), keep_latest as i64],
            )
            .map_err(|e| Error::Database(e.to_string()))?;

        Ok(deleted)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[tokio::test]
    async fn test_save_and_load() {
        let store = SqliteCheckpointStore::open_in_memory().unwrap();
        let plan_id = Uuid::new_v4();
        let checkpoint = Checkpoint::new(plan_id, json!({"status": "running"}));
        let cp_id = checkpoint.id;

        store.save(&checkpoint).await.unwrap();

        let loaded = store.load(cp_id).await.unwrap();
        assert!(loaded.is_some());
        let loaded = loaded.unwrap();
        assert_eq!(loaded.id, cp_id);
        assert_eq!(loaded.plan_id, plan_id);
    }

    #[tokio::test]
    async fn test_load_latest() {
        let store = SqliteCheckpointStore::open_in_memory().unwrap();
        let plan_id = Uuid::new_v4();

        let cp1 = Checkpoint::new(plan_id, json!({"step": 1}));
        store.save(&cp1).await.unwrap();

        let cp2 = Checkpoint::new(plan_id, json!({"step": 2}));
        let cp2_id = cp2.id;
        store.save(&cp2).await.unwrap();

        let latest = store.load_latest(plan_id).await.unwrap().unwrap();
        assert_eq!(latest.id, cp2_id);
    }

    #[tokio::test]
    async fn test_prune() {
        let store = SqliteCheckpointStore::open_in_memory().unwrap();
        let plan_id = Uuid::new_v4();

        for i in 0..5 {
            let cp = Checkpoint::new(plan_id, json!({"step": i}));
            store.save(&cp).await.unwrap();
        }

        let pruned = store.prune(plan_id, 2).await.unwrap();
        assert_eq!(pruned, 3);

        let remaining = store.list_for_plan(plan_id).await.unwrap();
        assert_eq!(remaining.len(), 2);
    }
}
