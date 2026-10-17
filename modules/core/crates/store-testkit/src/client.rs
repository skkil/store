use store_core::{
    MutationOutcome, PullOutcome, PushOutcome, Result, SyncStatus, apply_pull_response,
    apply_push_response, install, mutate, pull_request, push_request, register_table,
    rusqlite::Connection,
    serde_json::{self, Value},
    status,
    uuid::Uuid,
};

use crate::{
    AppState, FakeServer, PlannerMutator, app::CLIENT_SCHEMA, app::SYNCED_TABLES, expected_state,
};

/// One device: an in-memory database with the planner tables, the store
/// installed and every synced table registered.
pub struct Client {
    /// The device's database.
    pub conn: Connection,
}

impl Default for Client {
    fn default() -> Self {
        Self::new()
    }
}

impl Client {
    /// A fresh device.
    pub fn new() -> Self {
        let conn = Connection::open_in_memory().expect("open client database");
        conn.execute_batch(CLIENT_SCHEMA).expect("client schema");
        install(&conn).expect("install store");
        for (table, id_column) in SYNCED_TABLES {
            register_table(&conn, table, id_column).expect("register table");
        }
        Self { conn }
    }

    /// This device's client id.
    pub fn client_id(&self) -> Uuid {
        self.status().client_id
    }

    /// The store's status.
    pub fn status(&self) -> SyncStatus {
        status(&self.conn).expect("status")
    }

    /// Applies a planner mutation locally.
    pub fn mutate(&mut self, name: &str, args: Value) -> Result<MutationOutcome> {
        mutate(&mut self.conn, &PlannerMutator, name, &args)
    }

    /// Pushes every pending mutation to `server` in batches of `limit`.
    pub fn push(&mut self, server: &mut FakeServer, limit: u32) -> PushOutcome {
        let mut total = PushOutcome::default();
        while let Some(request) = push_request(&self.conn, limit).expect("push request") {
            let response = server.push(&request);
            let outcome = apply_push_response(&self.conn, &response).expect("apply push");
            total.applied += outcome.applied;
            total.duplicate += outcome.duplicate;
            total.rejected += outcome.rejected;
        }
        total
    }

    /// Pulls one page of at most `limit` versions from `server`.
    pub fn pull_once(&mut self, server: &FakeServer, limit: u32) -> PullOutcome {
        let request = pull_request(&self.conn, limit).expect("pull request");
        apply_pull_response(&mut self.conn, &PlannerMutator, &server.pull(&request))
            .expect("apply pull")
    }

    /// Pulls until the client has caught up with `server`.
    pub fn pull(&mut self, server: &FakeServer, limit: u32) -> Vec<PullOutcome> {
        let mut outcomes = Vec::new();
        loop {
            let outcome = self.pull_once(server, limit);
            let more = outcome.more;
            outcomes.push(outcome);
            if !more {
                return outcomes;
            }
        }
    }

    /// Pushes and pulls until nothing is left to push, the way a host's sync
    /// loop does: a pull can ask for answered mutations to be pushed again.
    pub fn sync(&mut self, server: &mut FakeServer) {
        loop {
            self.push(server, 100);
            self.pull(server, 100);
            if push_request(&self.conn, 1).expect("push request").is_none() {
                return;
            }
        }
    }

    /// The shared columns of every synced row.
    pub fn state(&self) -> AppState {
        AppState::of(&self.conn)
    }

    /// Pending mutations as `(mutation_id, name, args, hlc)`.
    pub fn pending(&self) -> Vec<(u64, String, Value, i64)> {
        let mut statement = self
            .conn
            .prepare("SELECT mutation_id, name, args, hlc FROM store_pending ORDER BY mutation_id")
            .expect("read pending");
        statement
            .query_map([], |row| {
                let args: String = row.get(2)?;
                Ok((
                    row.get::<_, i64>(0)? as u64,
                    row.get(1)?,
                    serde_json::from_str(&args).expect("pending args"),
                    row.get(3)?,
                ))
            })
            .expect("query pending")
            .collect::<std::result::Result<_, _>>()
            .expect("pending rows")
    }

    /// What this client must show: the server's state at the client's cursor
    /// with its pending mutations replayed.
    pub fn expected(&self, server: &FakeServer) -> AppState {
        let base = server.state_at(self.status().cursor_version);
        expected_state(base, self.client_id(), &self.pending())
    }
}
