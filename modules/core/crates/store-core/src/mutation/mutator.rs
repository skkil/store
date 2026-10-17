use rusqlite::Transaction;
use serde_json::Value;

use crate::mutation::{MutationContext, MutationError};

/// The product's named mutations, applied to its own tables.
///
/// The same code runs when the user acts (through [`mutate`](crate::mutate))
/// and again after every pull that rebases still-pending mutations, so it must
/// depend only on the database, its arguments and the context: take ids from
/// [`MutationContext::next_id`] and time from [`MutationContext::now`], never
/// from a random generator or the system clock.
pub trait Mutator {
    /// Applies mutation `name` with `args` inside `tx` and returns its result,
    /// such as the created row. The result is discarded on replay.
    fn apply(
        &self,
        tx: &Transaction<'_>,
        ctx: &mut MutationContext,
        name: &str,
        args: &Value,
    ) -> Result<Value, MutationError>;
}
