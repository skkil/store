use rusqlite::Connection;

use crate::Result;

/// Runs `body` inside a savepoint, so it is atomic whether or not the caller
/// already has a transaction open.
pub(crate) fn with_savepoint<T>(
    conn: &Connection,
    name: &str,
    body: impl FnOnce(&Connection) -> Result<T>,
) -> Result<T> {
    conn.execute_batch(&format!("SAVEPOINT {name}"))?;
    match body(conn) {
        Ok(value) => {
            conn.execute_batch(&format!("RELEASE {name}"))?;
            Ok(value)
        }
        Err(error) => {
            conn.execute_batch(&format!("ROLLBACK TO {name}; RELEASE {name}"))?;
            Err(error)
        }
    }
}
