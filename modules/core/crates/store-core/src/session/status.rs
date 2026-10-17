use rusqlite::Connection;
use serde::Serialize;
use uuid::Uuid;

use crate::{Result, storage::Meta};

/// A snapshot of the local sync state, for status indicators.
///
/// It serialises with camelCase names (`clientId`, `pending`, `issues`,
/// `cursorVersion`, …), the shape JavaScript hosts read.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SyncStatus {
    /// This install's client id.
    pub client_id: Uuid,
    /// The bound scope, if any.
    pub scope_id: Option<Uuid>,
    /// Mutations not yet acknowledged by a pull.
    pub pending: u64,
    /// When the oldest pending mutation was made, in Unix milliseconds.
    pub oldest_pending_at: Option<i64>,
    /// The cursor's epoch; 0 before the first pull.
    pub epoch: u32,
    /// The last version pulled.
    pub cursor_version: u64,
    /// The highest mutation id the server acknowledged.
    pub acked_mutation_id: u64,
    /// When a pull response was last applied, in Unix milliseconds.
    pub last_pull_at: Option<i64>,
    /// Issues not dismissed yet.
    #[serde(rename = "issues")]
    pub open_issues: u64,
}

/// The current [`SyncStatus`].
pub fn status(conn: &Connection) -> Result<SyncStatus> {
    let meta = Meta::load(conn)?;
    let (pending, oldest_pending_at): (i64, Option<i64>) = conn.query_row(
        "SELECT count(*), min(created_at) FROM store_pending",
        [],
        |row| Ok((row.get(0)?, row.get(1)?)),
    )?;
    let open_issues: i64 = conn.query_row(
        "SELECT count(*) FROM store_issue WHERE dismissed = 0",
        [],
        |row| row.get(0),
    )?;
    Ok(SyncStatus {
        client_id: meta.client_id,
        scope_id: meta.scope_id,
        pending: pending as u64,
        oldest_pending_at,
        epoch: meta.epoch,
        cursor_version: meta.cursor_version,
        acked_mutation_id: meta.last_mutation_id,
        last_pull_at: meta.last_pull_at,
        open_issues: open_issues as u64,
    })
}

/// [`status`] as JSON with camelCase names, for bindings.
pub fn status_json(conn: &Connection) -> Result<String> {
    Ok(serde_json::to_string(&status(conn)?)?)
}
