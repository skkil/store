use rusqlite::{Connection, TransactionBehavior};
use uuid::Uuid;

use crate::{
    Error, Result,
    storage::{Meta, Tables, clear_table, with_savepoint},
};

/// Binds the local store to `scope_id`, the account the user signed in to.
///
/// Binding the same scope again is a no-op. A client belongs to one scope for
/// life, so binding another one fails with [`Error::ScopeMismatch`]; call
/// [`forget`] first.
pub fn bind_scope(conn: &Connection, scope_id: Uuid) -> Result<()> {
    with_savepoint(conn, "store_bind", |conn| {
        match Meta::load(conn)?.scope_id {
            Some(bound) if bound == scope_id => Ok(()),
            Some(bound) => Err(Error::ScopeMismatch {
                bound,
                requested: scope_id,
            }),
            None => {
                conn.execute(
                    "UPDATE store_meta SET scope_id = ?1",
                    [scope_id.to_string()],
                )?;
                Ok(())
            }
        }
    })
}

/// Clears everything for sign-out: the rows of every registered table, the
/// pending mutations, undo log, issues and cursor. The store gets a new
/// client id and no scope; table registrations stay. Returns the new client
/// id.
pub fn forget(conn: &mut Connection) -> Result<Uuid> {
    let tx = conn.transaction_with_behavior(TransactionBehavior::Immediate)?;
    tx.execute_batch("PRAGMA defer_foreign_keys = ON")?;
    for table in Tables::load(&tx)?.iter() {
        clear_table(&tx, table)?;
    }
    tx.execute_batch(
        "DELETE FROM store_pending; DELETE FROM store_undo; DELETE FROM store_issue;",
    )?;
    Meta::reset_identity(&tx)?;
    let client_id = Meta::load(&tx)?.client_id;
    tx.commit()?;
    Ok(client_id)
}

/// Sets the product's schema version sent as `PushRequest.schema_version`.
pub fn set_product_schema_version(conn: &Connection, version: u32) -> Result<()> {
    conn.execute(
        "UPDATE store_meta SET product_schema_version = ?1",
        [version],
    )?;
    Ok(())
}
