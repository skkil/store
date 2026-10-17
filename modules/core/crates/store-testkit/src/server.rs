use std::collections::HashMap;

use sha2::{Digest, Sha256};
use store_core::{
    Hlc, MutationContext, Mutator,
    protocol::v1::{
        Change, Entries, MutationResult, PullRequest, PullResponse, PushRequest, PushResponse,
        Rejection, Reset, Version, change::Op, mutation_result::Status, pull_response::Body,
    },
    rusqlite::{Connection, params},
    serde_json::{self, Value},
    uuid::Uuid,
};

use crate::{AppState, PlannerMutator, app::SERVER_SCHEMA};

const CHANGE_LOG: &str = "
CREATE TABLE change_log (
  seq INTEGER PRIMARY KEY,
  table_name TEXT NOT NULL,
  row_id TEXT NOT NULL,
  op TEXT NOT NULL,
  data TEXT
);
CREATE TRIGGER project_upsert_i AFTER INSERT ON project BEGIN
  INSERT INTO change_log (table_name, row_id, op, data) VALUES ('project', NEW.id, 'upsert',
    json_object('id', NEW.id, 'name', NEW.name, 'archived', json(CASE WHEN NEW.archived THEN 'true' ELSE 'false' END),
                'created_at', NEW.created_at));
END;
CREATE TRIGGER project_upsert_u AFTER UPDATE ON project BEGIN
  INSERT INTO change_log (table_name, row_id, op, data) VALUES ('project', NEW.id, 'upsert',
    json_object('id', NEW.id, 'name', NEW.name, 'archived', json(CASE WHEN NEW.archived THEN 'true' ELSE 'false' END),
                'created_at', NEW.created_at));
END;
CREATE TRIGGER project_delete AFTER DELETE ON project BEGIN
  INSERT INTO change_log (table_name, row_id, op) VALUES ('project', OLD.id, 'delete');
END;
CREATE TRIGGER task_upsert_i AFTER INSERT ON task BEGIN
  INSERT INTO change_log (table_name, row_id, op, data) VALUES ('task', NEW.id, 'upsert',
    json_object('id', NEW.id, 'project_id', NEW.project_id, 'title', NEW.title,
                'done', json(CASE WHEN NEW.done THEN 'true' ELSE 'false' END), 'position', NEW.position,
                'counter', NEW.counter, 'labels', json(NEW.labels), 'created_at', NEW.created_at,
                'server_only', NEW.server_only));
END;
CREATE TRIGGER task_upsert_u AFTER UPDATE ON task BEGIN
  INSERT INTO change_log (table_name, row_id, op, data) VALUES ('task', NEW.id, 'upsert',
    json_object('id', NEW.id, 'project_id', NEW.project_id, 'title', NEW.title,
                'done', json(CASE WHEN NEW.done THEN 'true' ELSE 'false' END), 'position', NEW.position,
                'counter', NEW.counter, 'labels', json(NEW.labels), 'created_at', NEW.created_at,
                'server_only', NEW.server_only));
END;
CREATE TRIGGER task_delete AFTER DELETE ON task BEGIN
  INSERT INTO change_log (table_name, row_id, op) VALUES ('task', OLD.id, 'delete');
END;
";

/// An in-memory product server plus store service.
///
/// It applies pushed mutations with [`PlannerMutator`] in order, at most once
/// per `(client, mutation)`, logs every row change (booleans and JSON arrays
/// encoded the way Postgres' `to_jsonb` does) and serves pulls by cursor with
/// the epoch and hash checks of the real service.
pub struct FakeServer {
    db: Connection,
    epoch: u32,
    versions: Vec<Version>,
    history: Vec<AppState>,
    clients: HashMap<Uuid, u64>,
    pushes: usize,
}

impl Default for FakeServer {
    fn default() -> Self {
        Self::new()
    }
}

impl FakeServer {
    /// An empty server at epoch 1.
    pub fn new() -> Self {
        let db = Connection::open_in_memory().expect("open server database");
        db.execute_batch(SERVER_SCHEMA).expect("server schema");
        db.execute_batch(CHANGE_LOG).expect("server change log");
        Self {
            db,
            epoch: 1,
            versions: Vec::new(),
            history: vec![AppState::default()],
            clients: HashMap::new(),
            pushes: 0,
        }
    }

    /// The server's database, for reading only.
    pub fn connection(&self) -> &Connection {
        &self.db
    }

    /// The current epoch.
    pub fn epoch(&self) -> u32 {
        self.epoch
    }

    /// The newest version.
    pub fn head(&self) -> u64 {
        self.versions.len() as u64
    }

    /// Push requests received so far.
    pub fn pushes(&self) -> usize {
        self.pushes
    }

    /// Every version logged so far.
    pub fn versions(&self) -> &[Version] {
        &self.versions
    }

    /// The shared columns of every row right now.
    pub fn state(&self) -> AppState {
        AppState::of(&self.db)
    }

    /// The shared state right after `version`.
    pub fn state_at(&self, version: u64) -> &AppState {
        &self.history[version as usize]
    }

    /// The last mutation id processed for `client_id`.
    pub fn last_mutation_id(&self, client_id: Uuid) -> u64 {
        self.clients.get(&client_id).copied().unwrap_or(0)
    }

    /// Handles a push like a product server's write path.
    pub fn push(&mut self, request: &PushRequest) -> PushResponse {
        self.pushes += 1;
        let client_id = Uuid::parse_str(&request.client_id).expect("client id");
        let mut results = Vec::new();
        for mutation in &request.mutations {
            let last = self.last_mutation_id(client_id);
            if mutation.mutation_id <= last {
                results.push(MutationResult {
                    mutation_id: mutation.mutation_id,
                    status: Status::Duplicate as i32,
                    reason: String::new(),
                    version: 0,
                });
                continue;
            }
            let args: Value = serde_json::from_slice(&mutation.args).expect("args");
            let ctx =
                MutationContext::new(client_id, mutation.mutation_id, Hlc::from_i64(mutation.hlc));
            let (version, status, reason) =
                self.apply(client_id, mutation.mutation_id, ctx, &mutation.name, &args);
            self.clients.insert(client_id, mutation.mutation_id);
            results.push(MutationResult {
                mutation_id: mutation.mutation_id,
                status: status as i32,
                reason,
                version,
            });
        }
        PushResponse { results }
    }

    /// A write the product makes on its own (a REST endpoint or a job),
    /// logged under the server client id. Returns the new version, or the
    /// rejection reason.
    pub fn server_write(&mut self, name: &str, args: &Value) -> Result<u64, String> {
        let mutation_id = self.head() + 1;
        let ctx = MutationContext::new(
            Uuid::nil(),
            mutation_id,
            Hlc::new(1_000 + mutation_id as i64, 0),
        );
        match self.apply(Uuid::nil(), mutation_id, ctx, name, args) {
            (version, Status::Applied, _) => Ok(version),
            (_, _, reason) => {
                self.versions.pop();
                self.history.pop();
                Err(reason)
            }
        }
    }

    /// Serves a pull like the store service.
    pub fn pull(&self, request: &PullRequest) -> PullResponse {
        let client_id = Uuid::parse_str(&request.client_id).expect("client id");
        let cursor = request.cursor.clone().unwrap_or_default();
        let fresh = cursor.version == 0 && cursor.hash.is_empty();
        let body = if !fresh && cursor.epoch != self.epoch {
            Body::Reset(Reset {
                epoch: self.epoch,
                reason: "epoch changed".into(),
            })
        } else if cursor.version > self.head() {
            Body::Reset(Reset {
                epoch: self.epoch,
                reason: "cursor is ahead of the log".into(),
            })
        } else if cursor.version > 0
            && self.versions[cursor.version as usize - 1].hash != cursor.hash
        {
            Body::Reset(Reset {
                epoch: self.epoch,
                reason: "hash mismatch".into(),
            })
        } else {
            let limit = if request.limit == 0 {
                usize::MAX
            } else {
                request.limit as usize
            };
            let versions: Vec<Version> = self.versions[cursor.version as usize..]
                .iter()
                .take(limit)
                .cloned()
                .collect();
            let more = cursor.version + (versions.len() as u64) < self.head();
            Body::Entries(Entries { versions, more })
        };
        PullResponse {
            body: Some(body),
            last_mutation_id: self.last_mutation_id(client_id),
            head_version: self.head(),
            epoch: self.epoch,
        }
    }

    /// Simulates restoring the database to `version`: later versions are
    /// lost and the epoch is bumped, so every client gets a `Reset`.
    pub fn restore(&mut self, version: u64) {
        self.versions.truncate(version as usize);
        self.history.truncate(version as usize + 1);
        self.epoch += 1;
        self.db
            .execute_batch("DELETE FROM task; DELETE FROM project;")
            .expect("clear server");
        self.history[version as usize].write(&self.db);
        self.db
            .execute("DELETE FROM change_log", [])
            .expect("clear log");
        self.clients.clear();
        for logged in &self.versions {
            if let Ok(client) = Uuid::parse_str(&logged.client_id)
                && !client.is_nil()
            {
                self.clients.insert(client, logged.mutation_id);
            }
        }
    }

    fn apply(
        &mut self,
        client_id: Uuid,
        mutation_id: u64,
        mut ctx: MutationContext,
        name: &str,
        args: &Value,
    ) -> (u64, Status, String) {
        let tx = self.db.transaction().expect("begin");
        tx.execute("DELETE FROM change_log", []).expect("clear log");
        tx.execute_batch("SAVEPOINT mutation").expect("savepoint");
        let outcome = PlannerMutator.apply(&tx, &mut ctx, name, args);
        let (changes, rejection, status, reason) = match outcome {
            Ok(_) => (read_changes(&tx), None, Status::Applied, String::new()),
            Err(error) => {
                tx.execute_batch("ROLLBACK TO mutation").expect("rollback");
                let reason = error.to_string();
                let rejection = Rejection {
                    mutation_id,
                    reason: reason.clone(),
                };
                (Vec::new(), Some(rejection), Status::Rejected, reason)
            }
        };
        tx.execute_batch("RELEASE mutation").expect("release");
        tx.commit().expect("commit");
        let number = self.head() + 1;
        let previous = self
            .versions
            .last()
            .map(|v| v.hash.clone())
            .unwrap_or_default();
        let hash = Sha256::digest(
            [
                previous.as_slice(),
                format!("{number}|{client_id}|{mutation_id}|{}", changes.len()).as_bytes(),
            ]
            .concat(),
        )
        .to_vec();
        self.versions.push(Version {
            version: number,
            client_id: client_id.to_string(),
            mutation_id,
            hash,
            committed_at: number as i64,
            changes,
            rejection,
        });
        self.history.push(self.state());
        (number, status, reason)
    }
}

fn read_changes(db: &Connection) -> Vec<Change> {
    let mut statement = db
        .prepare("SELECT table_name, row_id, op, data FROM change_log ORDER BY seq")
        .expect("read change log");
    statement
        .query_map(params![], |row| {
            let op: String = row.get(2)?;
            Ok(Change {
                table: row.get(0)?,
                row_id: row.get(1)?,
                op: if op == "delete" {
                    Op::Delete as i32
                } else {
                    Op::Upsert as i32
                },
                data: row
                    .get::<_, Option<String>>(3)?
                    .unwrap_or_default()
                    .into_bytes(),
            })
        })
        .expect("query change log")
        .collect::<Result<_, _>>()
        .expect("change rows")
}
