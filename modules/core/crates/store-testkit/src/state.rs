use std::collections::BTreeMap;

use store_core::{
    Hlc, MutationContext, Mutator,
    rusqlite::{Connection, params_from_iter, types::Value as SqlValue},
    serde_json::Value,
    uuid::Uuid,
};

use crate::{
    PlannerMutator,
    app::{CLIENT_SCHEMA, SHARED_COLUMNS},
};

/// Rows by table and id, each with the columns clients and server share.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct AppState {
    tables: BTreeMap<String, BTreeMap<String, BTreeMap<String, SqlValue>>>,
}

impl AppState {
    /// Reads the shared columns of a client or server database.
    pub fn of(conn: &Connection) -> Self {
        Self::read(conn, SHARED_COLUMNS)
    }

    pub(crate) fn read(conn: &Connection, spec: &[(&str, &[&str])]) -> Self {
        let mut tables = BTreeMap::new();
        for (table, columns) in spec {
            let sql = format!("SELECT {} FROM {table}", columns.join(", "));
            let mut statement = conn.prepare(&sql).expect("read state");
            let rows = statement
                .query_map([], |row| {
                    let mut values = BTreeMap::new();
                    for (index, column) in columns.iter().enumerate() {
                        values.insert((*column).to_owned(), row.get::<_, SqlValue>(index)?);
                    }
                    Ok(values)
                })
                .expect("query state")
                .collect::<Result<Vec<_>, _>>()
                .expect("state rows");
            let by_id = rows
                .into_iter()
                .map(|values| {
                    let id = match &values["id"] {
                        SqlValue::Text(id) => id.clone(),
                        other => format!("{other:?}"),
                    };
                    (id, values)
                })
                .collect();
            tables.insert((*table).to_owned(), by_id);
        }
        Self { tables }
    }

    /// The number of rows in `table`.
    pub fn count(&self, table: &str) -> usize {
        self.tables.get(table).map_or(0, BTreeMap::len)
    }

    /// One column of one row.
    pub fn value(&self, table: &str, id: &str, column: &str) -> Option<&SqlValue> {
        self.tables.get(table)?.get(id)?.get(column)
    }

    pub(crate) fn write(&self, conn: &Connection) {
        for (table, columns) in SHARED_COLUMNS {
            let Some(rows) = self.tables.get(*table) else {
                continue;
            };
            let sql = format!(
                "INSERT INTO {table} ({}) VALUES ({})",
                columns.join(", "),
                vec!["?"; columns.len()].join(", ")
            );
            for row in rows.values() {
                let values = columns.iter().map(|column| row[*column].clone());
                conn.execute(&sql, params_from_iter(values))
                    .expect("write state");
            }
        }
    }
}

/// The state a client must show: `base` (the server at the client's cursor)
/// with `pending` mutations of `client_id` replayed in order, leaving out
/// those the mutator refuses.
pub fn expected_state(
    base: &AppState,
    client_id: Uuid,
    pending: &[(u64, String, Value, i64)],
) -> AppState {
    let mut conn = Connection::open_in_memory().expect("open reference database");
    conn.execute_batch(CLIENT_SCHEMA).expect("reference schema");
    base.write(&conn);
    let tx = conn.transaction().expect("begin");
    for (mutation_id, name, args, hlc) in pending {
        let mut ctx = MutationContext::new(client_id, *mutation_id, Hlc::from_i64(*hlc));
        tx.execute_batch("SAVEPOINT replay").expect("savepoint");
        match PlannerMutator.apply(&tx, &mut ctx, name, args) {
            Ok(_) => tx.execute_batch("RELEASE replay").expect("release"),
            Err(_) => tx
                .execute_batch("ROLLBACK TO replay; RELEASE replay")
                .expect("rollback"),
        }
    }
    let state = AppState::of(&tx);
    tx.commit().expect("commit");
    state
}
