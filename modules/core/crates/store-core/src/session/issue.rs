use rusqlite::Connection;
use serde_json::Value;

use crate::{Error, Result};

/// A mutation of this client that the server rejected.
#[derive(Debug, Clone, PartialEq)]
pub struct SyncIssue {
    /// The issue's local id, for [`dismiss_issue`].
    pub id: i64,
    /// The rejected mutation.
    pub mutation_id: u64,
    /// The mutation's name.
    pub name: String,
    /// The mutation's arguments.
    pub args: Value,
    /// The server's reason, readable by the user.
    pub reason: String,
    /// When the rejection was pulled, in Unix milliseconds.
    pub created_at: i64,
}

/// Issues the user has not dismissed yet, oldest first.
pub fn issues(conn: &Connection) -> Result<Vec<SyncIssue>> {
    let mut statement = conn.prepare(
        "SELECT id, mutation_id, name, args, reason, created_at
           FROM store_issue WHERE dismissed = 0 ORDER BY id",
    )?;
    let rows = statement
        .query_map([], |row| {
            Ok((
                row.get::<_, i64>(0)?,
                row.get::<_, i64>(1)?,
                row.get::<_, String>(2)?,
                row.get::<_, String>(3)?,
                row.get::<_, String>(4)?,
                row.get::<_, i64>(5)?,
            ))
        })?
        .collect::<rusqlite::Result<Vec<_>>>()?;
    rows.into_iter()
        .map(|(id, mutation_id, name, args, reason, created_at)| {
            Ok(SyncIssue {
                id,
                mutation_id: mutation_id as u64,
                name,
                args: serde_json::from_str(&args)
                    .map_err(|_| Error::Corrupt(format!("arguments of issue {id}")))?,
                reason,
                created_at,
            })
        })
        .collect()
}

/// Hides an issue. Returns false when no open issue has this id.
pub fn dismiss_issue(conn: &Connection, id: i64) -> Result<bool> {
    let updated = conn.execute(
        "UPDATE store_issue SET dismissed = 1 WHERE id = ?1 AND dismissed = 0",
        [id],
    )?;
    Ok(updated > 0)
}
