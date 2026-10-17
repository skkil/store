use std::collections::BTreeSet;

use rusqlite::{Connection, OptionalExtension, Transaction, TransactionBehavior};
use serde::Serialize;
use serde_json::{Map, Value};
use store_protocol::{
    PROTOCOL_VERSION,
    v1::{
        Cursor, Entries, PullRequest, PullResponse, Reset, Version, change::Op, pull_response::Body,
    },
};
use uuid::Uuid;

use crate::{
    Error, Result,
    clock::wall_millis,
    mutation::{Mutator, replay_pending},
    storage::{ImageEncoding, Meta, Tables, clear_table, delete_row, undo_pending, upsert_row},
};

/// What applying a pull response changed. Serialises with camelCase names
/// (`changedTables`, `more`, `reset`, …) for JavaScript hosts.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PullOutcome {
    /// Registered tables whose rows may have changed, sorted.
    pub changed_tables: Vec<String>,
    /// More versions are waiting; pull again right away.
    pub more: bool,
    /// The server reset this client: synced rows were dropped, the cursor
    /// went back to version 0 and pending mutations will be pushed again.
    pub reset: bool,
    /// Versions applied.
    pub versions: usize,
    /// The highest mutation id the server has acknowledged so far.
    pub acked_mutation_id: u64,
    /// Rejections of this client's mutations that became new issues.
    pub new_issues: usize,
}

/// A pull for at most `limit` versions after the local cursor.
pub fn pull_request(conn: &Connection, limit: u32) -> Result<PullRequest> {
    let meta = Meta::load(conn)?;
    Ok(PullRequest {
        protocol_version: PROTOCOL_VERSION,
        client_id: meta.client_id.to_string(),
        cursor: Some(Cursor {
            epoch: meta.epoch,
            version: meta.cursor_version,
            hash: meta.cursor_hash,
        }),
        min_version: meta.min_version,
        limit,
    })
}

/// Applies a pull response in one transaction and rebases pending mutations.
///
/// For entries: pending mutations are undone from their pre-images, each
/// version's changes are applied (only the columns the local table has;
/// unregistered tables are ignored), rejections of this client's mutations
/// become issues, the cursor advances, acknowledged mutations are dropped and
/// the rest are replayed through `mutator`. Once caught up, mutations the
/// server answered but does not acknowledge (it lost them) are pushed again.
/// For a reset: every registered
/// table is emptied, the cursor restarts at version 0 of the new epoch and
/// pending mutations are kept, replayed and pushed again.
///
/// Foreign keys are deferred while server changes are applied and checked
/// immediately again during the replay, so a replayed mutation that breaks
/// one is left out instead of failing the pull.
pub fn apply_pull_response(
    conn: &mut Connection,
    mutator: &dyn Mutator,
    response: &PullResponse,
) -> Result<PullOutcome> {
    let tx = conn.transaction_with_behavior(TransactionBehavior::Immediate)?;
    tx.execute_batch("PRAGMA defer_foreign_keys = ON")?;
    let meta = Meta::load(&tx)?;
    let now = wall_millis(&tx)?;
    let outcome = match &response.body {
        Some(Body::Entries(entries)) => apply_entries(&tx, mutator, &meta, response, entries, now)?,
        Some(Body::Reset(reset)) => apply_reset(&tx, mutator, &meta, response, reset)?,
        None => return Err(Error::Protocol("pull response has no body".into())),
    };
    tx.execute("UPDATE store_meta SET last_pull_at = ?1", [now])?;
    tx.commit()?;
    Ok(outcome)
}

fn apply_entries(
    tx: &Transaction<'_>,
    mutator: &dyn Mutator,
    meta: &Meta,
    response: &PullResponse,
    entries: &Entries,
    now: i64,
) -> Result<PullOutcome> {
    let epoch = check_epoch(meta, response.epoch)?;
    check_versions(meta.cursor_version, &entries.versions)?;
    let ack = acknowledged(meta, response, entries);
    let mut outcome = PullOutcome {
        more: entries.more,
        versions: entries.versions.len(),
        acked_mutation_id: ack,
        ..PullOutcome::default()
    };
    if !entries.more {
        tx.execute(
            "UPDATE store_meta SET pushed_mutation_id = min(pushed_mutation_id, ?1)",
            [ack as i64],
        )?;
    }
    if entries.versions.is_empty() && ack == meta.last_mutation_id {
        tx.execute("UPDATE store_meta SET epoch = ?1", [epoch])?;
        return Ok(outcome);
    }
    let tables = Tables::load(tx)?;
    let mut changed = undo_pending(tx, &tables)?;
    for version in &entries.versions {
        apply_version(tx, &tables, version, &mut changed)?;
        if is_own(meta, &version.client_id)
            && let Some(rejection) = &version.rejection
            && record_issue(tx, rejection.mutation_id, &rejection.reason, now)?
        {
            outcome.new_issues += 1;
        }
    }
    if let Some(last) = entries.versions.last() {
        tx.execute(
            "UPDATE store_meta SET cursor_version = ?1, cursor_hash = ?2",
            (last.version as i64, &last.hash),
        )?;
    }
    tx.execute("UPDATE store_meta SET epoch = ?1", [epoch])?;
    acknowledge(tx, ack)?;
    tx.execute_batch("PRAGMA defer_foreign_keys = OFF")?;
    changed.extend(replay_pending(tx, mutator, meta.client_id)?);
    outcome.changed_tables = changed.into_iter().collect();
    Ok(outcome)
}

fn apply_reset(
    tx: &Transaction<'_>,
    mutator: &dyn Mutator,
    meta: &Meta,
    response: &PullResponse,
    reset: &Reset,
) -> Result<PullOutcome> {
    let tables = Tables::load(tx)?;
    let mut changed = BTreeSet::new();
    for table in tables.iter() {
        clear_table(tx, table)?;
        changed.insert(table.name.clone());
    }
    tx.execute("DELETE FROM store_undo", [])?;
    let ack = meta.last_mutation_id.max(response.last_mutation_id);
    tx.execute(
        "UPDATE store_meta
            SET epoch = ?1, cursor_version = 0, cursor_hash = x'', min_version = 0,
                pushed_mutation_id = ?2",
        (reset.epoch, ack as i64),
    )?;
    acknowledge(tx, ack)?;
    tx.execute_batch("PRAGMA defer_foreign_keys = OFF")?;
    changed.extend(replay_pending(tx, mutator, meta.client_id)?);
    Ok(PullOutcome {
        changed_tables: changed.into_iter().collect(),
        more: true,
        reset: true,
        versions: 0,
        acked_mutation_id: ack,
        new_issues: 0,
    })
}

fn check_epoch(meta: &Meta, epoch: u32) -> Result<u32> {
    match (meta.epoch, epoch) {
        (local, 0) => Ok(local),
        (_, server) if meta.cursor_version == 0 => Ok(server),
        (local, server) if local == server => Ok(local),
        (local, server) => Err(Error::Protocol(format!(
            "entries for epoch {server} while the cursor is at epoch {local}"
        ))),
    }
}

fn check_versions(cursor: u64, versions: &[Version]) -> Result<()> {
    for (expected, version) in (cursor + 1..).zip(versions) {
        if version.version != expected {
            return Err(Error::Protocol(format!(
                "expected version {expected}, got {}",
                version.version
            )));
        }
    }
    Ok(())
}

fn acknowledged(meta: &Meta, response: &PullResponse, entries: &Entries) -> u64 {
    let target = if entries.more {
        entries
            .versions
            .iter()
            .filter(|version| is_own(meta, &version.client_id))
            .map(|version| version.mutation_id)
            .max()
            .unwrap_or(0)
            .min(response.last_mutation_id)
    } else {
        response.last_mutation_id
    };
    meta.last_mutation_id.max(target)
}

fn apply_version(
    tx: &Transaction<'_>,
    tables: &Tables,
    version: &Version,
    changed: &mut BTreeSet<String>,
) -> Result<()> {
    for change in &version.changes {
        let Some(table) = tables.get(&change.table) else {
            continue;
        };
        match Op::try_from(change.op) {
            Ok(Op::Upsert) => {
                let image = row_image(&change.data, &change.table, &change.row_id)?;
                upsert_row(tx, table, &change.row_id, &image, ImageEncoding::Server)?;
            }
            Ok(Op::Delete) => delete_row(tx, table, &change.row_id)?,
            _ => continue,
        }
        changed.insert(table.name.clone());
    }
    Ok(())
}

fn row_image(data: &[u8], table: &str, row_id: &str) -> Result<Map<String, Value>> {
    if data.is_empty() {
        return Ok(Map::new());
    }
    match serde_json::from_slice(data)? {
        Value::Object(map) => Ok(map),
        _ => Err(Error::Protocol(format!(
            "row {row_id} of {table} is not a JSON object"
        ))),
    }
}

fn is_own(meta: &Meta, client_id: &str) -> bool {
    Uuid::parse_str(client_id).is_ok_and(|id| id == meta.client_id)
}

fn record_issue(tx: &Transaction<'_>, mutation_id: u64, reason: &str, now: i64) -> Result<bool> {
    let pending: Option<(String, String)> = tx
        .query_row(
            "SELECT name, args FROM store_pending WHERE mutation_id = ?1",
            [mutation_id as i64],
            |row| Ok((row.get(0)?, row.get(1)?)),
        )
        .optional()?;
    let Some((name, args)) = pending else {
        return Ok(false);
    };
    let inserted = tx.execute(
        "INSERT OR IGNORE INTO store_issue (mutation_id, name, args, reason, created_at)
         VALUES (?1, ?2, ?3, ?4, ?5)",
        (mutation_id as i64, name, args, reason, now),
    )?;
    Ok(inserted > 0)
}

fn acknowledge(tx: &Transaction<'_>, ack: u64) -> Result<()> {
    let ack = ack as i64;
    tx.execute("DELETE FROM store_pending WHERE mutation_id <= ?1", [ack])?;
    tx.execute("DELETE FROM store_undo WHERE mutation_id <= ?1", [ack])?;
    tx.execute(
        "UPDATE store_meta
            SET last_mutation_id = ?1,
                issued_mutation_id = max(issued_mutation_id, ?1),
                pushed_mutation_id = max(pushed_mutation_id, ?1)",
        [ack],
    )?;
    Ok(())
}
