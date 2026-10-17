use rusqlite::{Connection, TransactionBehavior};
use serde_json::Value;

use crate::{
    Error, Result,
    clock::wall_millis,
    mutation::{MutationContext, MutationOutcome, Mutator},
    storage::{Capture, Meta},
};

/// Applies mutation `name` locally and queues it for push, in one
/// transaction.
///
/// Capture is turned on, so every row the mutator touches records its
/// pre-image; the mutation is stored in `store_pending` with the next
/// mutation id and a fresh [`Hlc`](crate::Hlc). If the mutator fails, the
/// transaction rolls back, nothing is queued and the error is returned as
/// [`Error::Mutation`].
pub fn mutate(
    conn: &mut Connection,
    mutator: &dyn Mutator,
    name: &str,
    args: &Value,
) -> Result<MutationOutcome> {
    if !args.is_object() {
        return Err(Error::ArgsNotObject);
    }
    let tx = conn.transaction_with_behavior(TransactionBehavior::Immediate)?;
    let meta = Meta::load(&tx)?;
    let now = wall_millis(&tx)?;
    let mutation_id = meta.issued_mutation_id.max(meta.last_mutation_id) + 1;
    let hlc = meta.hlc.tick(now);
    let mut ctx = MutationContext::new(meta.client_id, mutation_id, hlc);
    Capture::start(&tx, mutation_id)?;
    let value = mutator.apply(&tx, &mut ctx, name, args)?;
    Capture::stop(&tx)?;
    tx.execute(
        "INSERT INTO store_pending (mutation_id, name, args, hlc, created_at)
         VALUES (?1, ?2, ?3, ?4, ?5)",
        (
            mutation_id as i64,
            name,
            serde_json::to_string(args)?,
            hlc.as_i64(),
            now,
        ),
    )?;
    tx.execute(
        "UPDATE store_meta SET issued_mutation_id = ?1, hlc = ?2",
        (mutation_id as i64, hlc.as_i64()),
    )?;
    tx.commit()?;
    Ok(MutationOutcome {
        mutation_id,
        hlc,
        value,
    })
}
