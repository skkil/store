//! Full sync rounds over HTTP against the fake server.

#![cfg(not(target_arch = "wasm32"))]

mod support;

use std::sync::{
    Arc, Mutex,
    atomic::{AtomicBool, Ordering},
};

use store_core::{
    protocol::{
        Message,
        v1::{PullRequest, PushRequest},
    },
    serde_json::json,
};
use store_http::{Endpoints, HttpTransport, StaticCredentials, Syncer};
use store_testkit::{Client, FakeServer, PlannerMutator};
use support::{Reply, TestServer};

fn serve(server: Arc<Mutex<FakeServer>>, push_down: Arc<AtomicBool>) -> TestServer {
    TestServer::start(move |request| {
        let mut fake = server.lock().unwrap();
        if request.path.ends_with("/Push") {
            if push_down.load(Ordering::SeqCst) {
                return Reply::json(503, r#"{"code":"unavailable","message":"maintenance"}"#);
            }
            let push = PushRequest::decode(request.body.as_slice()).unwrap();
            Reply::proto(fake.push(&push).encode_to_vec())
        } else {
            let pull = PullRequest::decode(request.body.as_slice()).unwrap();
            Reply::proto(fake.pull(&pull).encode_to_vec())
        }
    })
}

fn transport(http: &TestServer) -> HttpTransport<StaticCredentials> {
    HttpTransport::new(
        Endpoints::new(&http.url, &http.url),
        StaticCredentials {
            product_token: "p".into(),
            sync_token: "s".into(),
        },
    )
}

#[test]
fn clients_converge_through_http() {
    let fake = Arc::new(Mutex::new(FakeServer::new()));
    let http = serve(Arc::clone(&fake), Arc::new(AtomicBool::new(false)));
    let transport = transport(&http);
    let syncer = Syncer::new(&transport);
    let mut alice = Client::new();
    let mut bob = Client::new();

    let project = alice
        .mutate("project.create", json!({"name": "Home"}))
        .unwrap();
    let report = syncer.sync(&mut alice.conn, &PlannerMutator).unwrap();
    assert_eq!(report.pushed.applied, 1);
    assert!(report.pulls >= 1);
    assert!(alice.pending().is_empty());

    syncer.sync(&mut bob.conn, &PlannerMutator).unwrap();
    assert_eq!(bob.state().count("project"), 1);
    bob.mutate(
        "task.create",
        json!({"project_id": project.value["id"], "title": "Bob"}),
    )
    .unwrap();
    syncer.sync(&mut bob.conn, &PlannerMutator).unwrap();
    let report = syncer.sync(&mut alice.conn, &PlannerMutator).unwrap();
    assert!(report.changed_tables.contains("task"));

    bob.mutate(
        "task.create",
        json!({"project_id": project.value["id"], "title": "Late"}),
    )
    .unwrap();
    alice
        .mutate("project.delete", json!({"id": project.value["id"]}))
        .unwrap();
    syncer.sync(&mut alice.conn, &PlannerMutator).unwrap();
    let report = syncer.sync(&mut bob.conn, &PlannerMutator).unwrap();
    assert_eq!(report.pushed.rejected, 1);
    assert_eq!(report.new_issues, 1);
    syncer.sync(&mut alice.conn, &PlannerMutator).unwrap();

    let state = fake.lock().unwrap().state();
    assert_eq!(alice.state(), state);
    assert_eq!(bob.state(), state);
}

#[test]
fn a_failed_push_keeps_mutations_queued_for_the_next_round() {
    let fake = Arc::new(Mutex::new(FakeServer::new()));
    let push_down = Arc::new(AtomicBool::new(true));
    let http = serve(Arc::clone(&fake), Arc::clone(&push_down));
    let transport = transport(&http);
    let syncer = Syncer::new(&transport);
    let mut alice = Client::new();
    alice
        .mutate("project.create", json!({"name": "Later"}))
        .unwrap();

    let error = syncer.sync(&mut alice.conn, &PlannerMutator).unwrap_err();
    assert!(error.is_retryable());
    assert_eq!(alice.pending().len(), 1);

    push_down.store(false, Ordering::SeqCst);
    syncer.sync(&mut alice.conn, &PlannerMutator).unwrap();
    assert!(alice.pending().is_empty());
    assert_eq!(alice.state(), fake.lock().unwrap().state());
}
