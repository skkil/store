use std::collections::BTreeSet;

use store_core::{
    Mutator, PushOutcome, apply_pull_response, apply_push_response,
    protocol::{PROTOCOL_VERSION, v1::PokesRequest},
    pull_request, push_request,
    rusqlite::Connection,
    status,
};

use crate::{Credentials, HttpTransport, PokeStream, Result};

const PUSH_BATCH: u32 = 100;
const PULL_LIMIT: u32 = 500;
const MAX_ROUNDS: usize = 8;

/// What one [`Syncer::sync`] did.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct SyncReport {
    /// The summed push results.
    pub pushed: PushOutcome,
    /// Pull responses applied.
    pub pulls: usize,
    /// Registered tables whose rows may have changed, for UI refreshes.
    pub changed_tables: BTreeSet<String>,
    /// True if the server reset this client during the round.
    pub reset: bool,
    /// Rejections that became new issues.
    pub new_issues: usize,
}

/// Runs sync rounds of the store core over an [`HttpTransport`].
pub struct Syncer<'a, C> {
    transport: &'a HttpTransport<C>,
}

impl<'a, C: Credentials> Syncer<'a, C> {
    /// A syncer over `transport`.
    pub fn new(transport: &'a HttpTransport<C>) -> Self {
        Self { transport }
    }

    /// Pushes every pending mutation, pulls until caught up, and repeats
    /// while a pull asks for mutations to be pushed again. On a transport
    /// error the round stops; everything already applied stays applied and
    /// the next round resumes from there.
    pub fn sync(&self, conn: &mut Connection, mutator: &dyn Mutator) -> Result<SyncReport> {
        let mut report = SyncReport::default();
        for _ in 0..MAX_ROUNDS {
            self.push_all(conn, &mut report)?;
            self.pull_all(conn, mutator, &mut report)?;
            if push_request(conn, 1)?.is_none() {
                break;
            }
        }
        Ok(report)
    }

    /// Opens the poke stream for this client. Sync when a poke's version is
    /// ahead of [`status`]'s `cursor_version`.
    pub fn pokes(&self, conn: &Connection) -> Result<PokeStream> {
        self.transport.pokes(&PokesRequest {
            protocol_version: PROTOCOL_VERSION,
            client_id: status(conn)?.client_id.to_string(),
        })
    }

    fn push_all(&self, conn: &Connection, report: &mut SyncReport) -> Result<()> {
        while let Some(request) = push_request(conn, PUSH_BATCH)? {
            let response = self.transport.push(&request)?;
            let outcome = apply_push_response(conn, &response)?;
            report.pushed.applied += outcome.applied;
            report.pushed.duplicate += outcome.duplicate;
            report.pushed.rejected += outcome.rejected;
            if outcome.applied + outcome.duplicate + outcome.rejected == 0 {
                break;
            }
        }
        Ok(())
    }

    fn pull_all(
        &self,
        conn: &mut Connection,
        mutator: &dyn Mutator,
        report: &mut SyncReport,
    ) -> Result<()> {
        loop {
            let response = self.transport.pull(&pull_request(conn, PULL_LIMIT)?)?;
            let outcome = apply_pull_response(conn, mutator, &response)?;
            report.pulls += 1;
            report.reset |= outcome.reset;
            report.new_issues += outcome.new_issues;
            report.changed_tables.extend(outcome.changed_tables);
            if !outcome.more {
                return Ok(());
            }
        }
    }
}
