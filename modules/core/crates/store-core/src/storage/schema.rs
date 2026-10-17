use rusqlite::{Connection, OptionalExtension};

use crate::{
    Error, Result,
    storage::{Meta, with_savepoint},
};

/// The version of the local store tables this build creates.
pub const SCHEMA_VERSION: i64 = 1;

const MIGRATIONS: &[&str] = &[include_str!("migrations/v1.sql")];

/// Creates or upgrades the local store tables (`store_meta`, `store_table`,
/// `store_pending`, `store_undo`, `store_issue`) on the product's connection.
///
/// Idempotent: call it on every start, before any other function. A new
/// store gets a fresh client id. Fails with [`Error::SchemaTooNew`] when the
/// file was written by a newer build.
pub fn install(conn: &Connection) -> Result<()> {
    with_savepoint(conn, "store_install", |conn| {
        let current = installed_version(conn)?;
        if current > SCHEMA_VERSION {
            return Err(Error::SchemaTooNew {
                found: current,
                supported: SCHEMA_VERSION,
            });
        }
        for (index, migration) in MIGRATIONS.iter().enumerate().skip(current as usize) {
            conn.execute_batch(migration)?;
            let version = index as i64 + 1;
            if version == 1 {
                Meta::create(conn, version)?;
            } else {
                conn.execute("UPDATE store_meta SET schema_version = ?1", [version])?;
            }
        }
        Ok(())
    })
}

fn installed_version(conn: &Connection) -> Result<i64> {
    let exists: bool = conn.query_row(
        "SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'store_meta')",
        [],
        |row| row.get(0),
    )?;
    if !exists {
        return Ok(0);
    }
    let version = conn
        .query_row("SELECT schema_version FROM store_meta", [], |row| {
            row.get(0)
        })
        .optional()?;
    version.ok_or_else(|| Error::Corrupt("store_meta has no row".into()))
}
