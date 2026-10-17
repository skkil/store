use rusqlite::Connection;
use serde::Serialize;
use store_protocol::{
    PROTOCOL_VERSION,
    v1::{Mutation, PushRequest, PushResponse, mutation_result::Status},
};

use crate::{
    Result,
    storage::{Meta, with_savepoint},
};

/// What a push response reported.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PushOutcome {
    /// Mutations the server applied now.
    pub applied: usize,
    /// Mutations the server had already processed.
    pub duplicate: usize,
    /// Mutations the server rejected; they become issues on the next pull.
    pub rejected: usize,
}

/// The next batch of at most `limit` pending mutations in order, or `None`
/// when nothing is waiting to be pushed.
///
/// Mutations the server already answered are not sent again until a
/// [`Reset`](store_protocol::v1::Reset); if a response is lost, the same
/// batch is built again and the server answers it idempotently.
pub fn push_request(conn: &Connection, limit: u32) -> Result<Option<PushRequest>> {
    let meta = Meta::load(conn)?;
    let mut statement = conn.prepare_cached(
        "SELECT mutation_id, name, args, hlc FROM store_pending
          WHERE mutation_id > ?1 ORDER BY mutation_id LIMIT ?2",
    )?;
    let mutations = statement
        .query_map((meta.pushed_mutation_id as i64, i64::from(limit)), |row| {
            Ok(Mutation {
                mutation_id: row.get::<_, i64>(0)? as u64,
                name: row.get(1)?,
                args: row.get::<_, String>(2)?.into_bytes(),
                hlc: row.get(3)?,
                traceparent: String::new(),
            })
        })?
        .collect::<rusqlite::Result<Vec<_>>>()?;
    if mutations.is_empty() {
        return Ok(None);
    }
    Ok(Some(PushRequest {
        protocol_version: PROTOCOL_VERSION,
        client_id: meta.client_id.to_string(),
        mutations,
        schema_version: meta.product_schema_version,
    }))
}

/// Records the server's answer to a push.
///
/// Answered mutations are not pushed again, and the highest version that
/// recorded them becomes the next pull's `min_version` (read-your-writes).
/// Pending mutations stay queued until a pull acknowledges them.
pub fn apply_push_response(conn: &Connection, response: &PushResponse) -> Result<PushOutcome> {
    let mut outcome = PushOutcome::default();
    let mut answered = 0u64;
    let mut version = 0u64;
    for result in &response.results {
        match Status::try_from(result.status) {
            Ok(Status::Applied) => outcome.applied += 1,
            Ok(Status::Duplicate) => outcome.duplicate += 1,
            Ok(Status::Rejected) => outcome.rejected += 1,
            _ => continue,
        }
        answered = answered.max(result.mutation_id);
        version = version.max(result.version);
    }
    with_savepoint(conn, "store_push", |conn| {
        conn.execute(
            "UPDATE store_meta
                SET pushed_mutation_id = max(pushed_mutation_id, min(?1, issued_mutation_id)),
                    min_version = max(min_version, ?2)",
            (answered as i64, version as i64),
        )?;
        Ok(())
    })?;
    Ok(outcome)
}
