//! Property: after any interleaving of local mutations, pushes (some with
//! lost responses), pulls, server writes and server restores, every client
//! shows the server's state at its cursor with its pending mutations
//! replayed, and all clients converge on the server's state once synced.

#![cfg(not(target_arch = "wasm32"))]

use proptest::prelude::*;
use store_core::{
    push_request,
    serde_json::{Value, json},
    status,
};
use store_testkit::{Client, FakeServer};

const CLIENTS: usize = 3;

#[derive(Debug, Clone)]
enum Action {
    CreateProject,
    CreateTask { project: usize, blank: bool },
    Rename { task: usize },
    Complete { task: usize },
    Increment { task: usize },
    DeleteTask { task: usize },
    ArchiveProject { project: usize },
    DeleteProject { project: usize },
}

#[derive(Debug, Clone)]
enum Step {
    Mutate { client: usize, action: Action },
    Push { client: usize, limit: u32 },
    PushLost { client: usize },
    Pull { client: usize, limit: u32 },
    ServerWrite { action: Action },
    Restore { back: u64 },
}

fn action() -> impl Strategy<Value = Action> {
    prop_oneof![
        2 => Just(Action::CreateProject),
        4 => (any::<usize>(), prop::bool::weighted(0.1))
            .prop_map(|(project, blank)| Action::CreateTask { project, blank }),
        2 => any::<usize>().prop_map(|task| Action::Rename { task }),
        1 => any::<usize>().prop_map(|task| Action::Complete { task }),
        3 => any::<usize>().prop_map(|task| Action::Increment { task }),
        1 => any::<usize>().prop_map(|task| Action::DeleteTask { task }),
        1 => any::<usize>().prop_map(|project| Action::ArchiveProject { project }),
        1 => any::<usize>().prop_map(|project| Action::DeleteProject { project }),
    ]
}

fn step() -> impl Strategy<Value = Step> {
    let client = 0..CLIENTS;
    prop_oneof![
        8 => (client.clone(), action()).prop_map(|(client, action)| Step::Mutate { client, action }),
        3 => (client.clone(), 1..4u32).prop_map(|(client, limit)| Step::Push { client, limit }),
        1 => client.clone().prop_map(|client| Step::PushLost { client }),
        4 => (client, 1..4u32).prop_map(|(client, limit)| Step::Pull { client, limit }),
        2 => action().prop_map(|action| Step::ServerWrite { action }),
        1 => (0..4u64).prop_map(|back| Step::Restore { back }),
    ]
}

fn ids(conn: &store_core::rusqlite::Connection, table: &str) -> Vec<String> {
    let mut statement = conn
        .prepare(&format!("SELECT id FROM {table} ORDER BY id"))
        .unwrap();
    statement
        .query_map([], |row| row.get(0))
        .unwrap()
        .collect::<Result<_, _>>()
        .unwrap()
}

fn pick(ids: &[String], index: usize) -> String {
    if ids.is_empty() {
        "missing".to_owned()
    } else {
        ids[index % ids.len()].clone()
    }
}

fn resolve(conn: &store_core::rusqlite::Connection, action: &Action) -> (&'static str, Value) {
    let projects = ids(conn, "project");
    let tasks = ids(conn, "task");
    match action {
        Action::CreateProject => ("project.create", json!({"name": "P"})),
        Action::CreateTask { project, blank } => (
            "task.create",
            json!({"project_id": pick(&projects, *project), "title": if *blank { "" } else { "T" }}),
        ),
        Action::Rename { task } => (
            "task.rename",
            json!({"id": pick(&tasks, *task), "title": "R"}),
        ),
        Action::Complete { task } => ("task.complete", json!({"id": pick(&tasks, *task)})),
        Action::Increment { task } => ("task.increment", json!({"id": pick(&tasks, *task)})),
        Action::DeleteTask { task } => ("task.delete", json!({"id": pick(&tasks, *task)})),
        Action::ArchiveProject { project } => {
            ("project.archive", json!({"id": pick(&projects, *project)}))
        }
        Action::DeleteProject { project } => {
            ("project.delete", json!({"id": pick(&projects, *project)}))
        }
    }
}

fn check_rebase_invariant(client: &Client, server: &FakeServer) -> Result<(), TestCaseError> {
    let status = status(&client.conn).unwrap();
    let same_epoch =
        status.epoch == server.epoch() || (status.epoch == 0 && status.cursor_version == 0);
    if same_epoch && status.cursor_version <= server.head() {
        prop_assert_eq!(client.state(), client.expected(server));
    }
    Ok(())
}

fn run(steps: Vec<Step>) -> Result<(), TestCaseError> {
    let mut server = FakeServer::new();
    let mut clients: Vec<Client> = (0..CLIENTS).map(|_| Client::new()).collect();
    let mut restores = 0;
    for step in steps {
        match step {
            Step::Mutate { client, action } => {
                let (name, args) = resolve(&clients[client].conn, &action);
                let _ = clients[client].mutate(name, args);
            }
            Step::Push { client, limit } => {
                clients[client].push(&mut server, limit);
            }
            Step::PushLost { client } => {
                if let Some(request) = push_request(&clients[client].conn, 10).unwrap() {
                    server.push(&request);
                }
            }
            Step::Pull { client, limit } => {
                clients[client].pull_once(&server, limit);
            }
            Step::ServerWrite { action } => {
                let (name, args) = resolve(server.connection(), &action);
                let _ = server.server_write(name, &args);
            }
            Step::Restore { back } => {
                if restores < 2 {
                    restores += 1;
                    server.restore(server.head().saturating_sub(back));
                }
            }
        }
        for client in &clients {
            check_rebase_invariant(client, &server)?;
        }
    }
    for client in &mut clients {
        client.sync(&mut server);
    }
    for client in &mut clients {
        client.pull(&server, 100);
        prop_assert!(client.pending().is_empty());
        prop_assert_eq!(client.state(), server.state());
    }
    Ok(())
}

proptest! {
    #![proptest_config(ProptestConfig { cases: 128, ..ProptestConfig::default() })]

    #[test]
    fn local_state_is_server_state_plus_pending_replayed(steps in prop::collection::vec(step(), 1..60)) {
        run(steps)?;
    }
}
