use std::collections::BTreeSet;

use rusqlite::Connection;
use serde_json::{Map, Value};

use crate::{
    Error, Result,
    storage::{ImageEncoding, Tables, delete_row, upsert_row},
};

/// Restores every row touched by pending mutations to its recorded
/// pre-image, newest mutation first, then clears the undo log. Returns the
/// tables it touched.
pub(crate) fn undo_pending(conn: &Connection, tables: &Tables) -> Result<BTreeSet<String>> {
    let entries = {
        let mut statement = conn.prepare(
            "SELECT table_name, row_id, before FROM store_undo ORDER BY mutation_id DESC, seq DESC",
        )?;
        statement
            .query_map([], |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, String>(1)?,
                    row.get::<_, Option<String>>(2)?,
                ))
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?
    };
    let mut touched = BTreeSet::new();
    for (table_name, row_id, before) in entries {
        let Some(table) = tables.get(&table_name) else {
            continue;
        };
        match before {
            None => delete_row(conn, table, &row_id)?,
            Some(json) => {
                let image: Map<String, Value> = serde_json::from_str(&json)
                    .map_err(|_| Error::Corrupt(format!("pre-image of {table_name} {row_id}")))?;
                upsert_row(conn, table, &row_id, &image, ImageEncoding::PreImage)?;
            }
        }
        touched.insert(table_name);
    }
    conn.execute("DELETE FROM store_undo", [])?;
    Ok(touched)
}
