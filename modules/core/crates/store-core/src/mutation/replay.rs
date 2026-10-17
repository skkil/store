use std::collections::BTreeSet;

use rusqlite::Transaction;
use serde_json::Value;
use uuid::Uuid;

use crate::{
    Error, Result,
    clock::Hlc,
    mutation::{MutationContext, Mutator},
    storage::Capture,
};

/// Re-applies every pending mutation in order with capture on, recording
/// fresh pre-images. A mutation whose mutator fails is rolled back to its
/// savepoint and stays pending without effects. Returns the tables touched.
pub(crate) fn replay_pending(
    tx: &Transaction<'_>,
    mutator: &dyn Mutator,
    client_id: Uuid,
) -> Result<BTreeSet<String>> {
    let pending = {
        let mut statement = tx.prepare(
            "SELECT mutation_id, name, args, hlc FROM store_pending ORDER BY mutation_id",
        )?;
        statement
            .query_map([], |row| {
                Ok((
                    row.get::<_, i64>(0)? as u64,
                    row.get::<_, String>(1)?,
                    row.get::<_, String>(2)?,
                    row.get::<_, i64>(3)?,
                ))
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?
    };
    for (mutation_id, name, args, hlc) in pending {
        let args: Value = serde_json::from_str(&args)
            .map_err(|_| Error::Corrupt(format!("arguments of mutation {mutation_id}")))?;
        let mut ctx = MutationContext::new(client_id, mutation_id, Hlc::from_i64(hlc)).replaying();
        tx.execute_batch("SAVEPOINT store_replay")?;
        Capture::start(tx, mutation_id)?;
        match mutator.apply(tx, &mut ctx, &name, &args) {
            Ok(_) => tx.execute_batch("RELEASE store_replay")?,
            Err(_) => tx.execute_batch("ROLLBACK TO store_replay; RELEASE store_replay")?,
        }
    }
    Capture::stop(tx)?;
    let mut statement = tx.prepare("SELECT DISTINCT table_name FROM store_undo")?;
    let touched = statement
        .query_map([], |row| row.get(0))?
        .collect::<rusqlite::Result<BTreeSet<String>>>()?;
    Ok(touched)
}
