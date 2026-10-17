//! Rebasing pending mutations over server changes, end to end against the fake server.

use store_core::{
    derive_id, issues,
    rusqlite::types::Value as SqlValue,
    serde_json::{Value, json},
    status,
};
use store_testkit::{Client, FakeServer};

fn id(value: &Value) -> String {
    value["id"].as_str().unwrap().to_owned()
}

fn shared_project(server: &mut FakeServer, clients: &mut [&mut Client]) -> String {
    server
        .server_write("project.create", &json!({"name": "Shared"}))
        .unwrap();
    for client in clients.iter_mut() {
        client.pull(server, 100);
    }
    id(&json!({"id": derive_id(store_core::uuid::Uuid::nil(), 1, 0).to_string()}))
}

#[test]
fn pending_mutations_rebase_over_concurrent_server_changes() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let mut bob = Client::new();
    let project = shared_project(&mut server, &mut [&mut alice, &mut bob]);

    let mine = alice
        .mutate(
            "task.create",
            json!({"project_id": project, "title": "Alice"}),
        )
        .unwrap();
    assert_eq!(mine.value["position"], json!(0));
    bob.mutate(
        "task.create",
        json!({"project_id": project, "title": "Bob"}),
    )
    .unwrap();
    bob.sync(&mut server);

    let outcome = alice.pull(&server, 100).remove(0);
    assert_eq!(outcome.changed_tables, vec!["task"]);
    assert_eq!(alice.pending().len(), 1);
    assert_eq!(alice.state(), alice.expected(&server));
    let position = alice
        .state()
        .value("task", &id(&mine.value), "position")
        .cloned();
    assert_eq!(position, Some(SqlValue::Integer(1)));

    alice.sync(&mut server);
    bob.pull(&server, 100);
    assert_eq!(alice.state(), server.state());
    assert_eq!(bob.state(), server.state());
    assert!(alice.pending().is_empty());
}

#[test]
fn non_idempotent_mutations_replay_exactly_once() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let mut bob = Client::new();
    let project = shared_project(&mut server, &mut [&mut alice, &mut bob]);
    let task = id(&alice
        .mutate(
            "task.create",
            json!({"project_id": project, "title": "Count"}),
        )
        .unwrap()
        .value);
    alice.sync(&mut server);
    bob.pull(&server, 100);

    alice.mutate("task.increment", json!({"id": task})).unwrap();
    alice.mutate("task.increment", json!({"id": task})).unwrap();
    bob.mutate("task.increment", json!({"id": task})).unwrap();
    bob.sync(&mut server);
    for _ in 0..3 {
        alice.pull(&server, 100);
    }
    assert_eq!(
        alice.state().value("task", &task, "counter"),
        Some(&SqlValue::Integer(3))
    );
    alice.sync(&mut server);
    assert_eq!(
        server.state().value("task", &task, "counter"),
        Some(&SqlValue::Integer(3))
    );
    assert_eq!(alice.state(), server.state());
}

#[test]
fn replays_keep_the_original_clock() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let mut bob = Client::new();
    let project = shared_project(&mut server, &mut [&mut alice, &mut bob]);
    let created = alice
        .mutate(
            "task.create",
            json!({"project_id": project, "title": "Offline"}),
        )
        .unwrap();
    bob.mutate("project.archive", json!({"id": project}))
        .unwrap();
    bob.sync(&mut server);
    alice.pull(&server, 100);
    let task = id(&created.value);
    let expected = SqlValue::Integer(created.hlc.millis());
    assert_eq!(
        alice.state().value("task", &task, "created_at"),
        Some(&expected)
    );
    alice.sync(&mut server);
    assert_eq!(
        server.state().value("task", &task, "created_at"),
        Some(&expected)
    );
}

#[test]
fn a_replay_that_fails_stays_pending_without_effects_and_becomes_an_issue() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let mut bob = Client::new();
    let project = shared_project(&mut server, &mut [&mut alice, &mut bob]);
    let task = id(&alice
        .mutate(
            "task.create",
            json!({"project_id": project, "title": "Doomed"}),
        )
        .unwrap()
        .value);
    alice
        .mutate("task.rename", json!({"id": task, "title": "Still doomed"}))
        .unwrap();
    bob.mutate("project.delete", json!({"id": project}))
        .unwrap();
    bob.sync(&mut server);

    alice.pull(&server, 100);
    assert_eq!(alice.pending().len(), 2);
    assert_eq!(alice.state().count("task"), 0);
    assert_eq!(alice.state().count("project"), 0);
    assert_eq!(alice.state(), alice.expected(&server));

    alice.sync(&mut server);
    assert!(alice.pending().is_empty());
    let open = issues(&alice.conn).unwrap();
    let reasons: Vec<&str> = open.iter().map(|issue| issue.reason.as_str()).collect();
    assert_eq!(reasons, vec!["project not found", "task not found"]);
    assert_eq!(alice.state(), server.state());
}

#[test]
fn rows_created_offline_get_the_same_ids_on_the_server() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let project = alice
        .mutate("project.create", json!({"name": "Mine"}))
        .unwrap();
    let task = alice
        .mutate(
            "task.create",
            json!({"project_id": project.value["id"], "title": "T"}),
        )
        .unwrap();
    alice.sync(&mut server);
    assert_eq!(server.state().count("project"), 1);
    assert!(
        server
            .state()
            .value("task", &id(&task.value), "title")
            .is_some()
    );
    assert_eq!(
        id(&project.value),
        derive_id(alice.client_id(), 1, 0).to_string()
    );
    assert_eq!(alice.state(), server.state());
}

#[test]
fn reset_drops_synced_rows_keeps_pending_and_re_pushes() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    alice
        .mutate("project.create", json!({"name": "Kept"}))
        .unwrap();
    alice.sync(&mut server);
    alice
        .mutate("project.create", json!({"name": "Lost"}))
        .unwrap();
    alice.sync(&mut server);
    assert_eq!(server.head(), 2);

    server.restore(1);
    alice
        .mutate("project.create", json!({"name": "Offline"}))
        .unwrap();
    let outcome = alice.pull_once(&server, 100);
    assert!(outcome.reset);
    assert!(outcome.more);
    assert_eq!(outcome.changed_tables, vec!["project", "task"]);
    assert_eq!(status(&alice.conn).unwrap().epoch, server.epoch());
    assert_eq!(status(&alice.conn).unwrap().cursor_version, 0);
    assert_eq!(alice.pending().len(), 1);
    assert_eq!(alice.state().count("project"), 1);

    alice.pull(&server, 100);
    assert_eq!(alice.state(), alice.expected(&server));
    alice.sync(&mut server);
    assert!(alice.pending().is_empty());
    assert_eq!(server.state().count("project"), 2);
    assert_eq!(alice.state(), server.state());
}

#[test]
fn reset_re_pushes_mutations_the_server_already_answered() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    alice
        .mutate("project.create", json!({"name": "Answered"}))
        .unwrap();
    alice.push(&mut server, 10);
    server.restore(0);
    alice.pull(&server, 100);
    assert_eq!(alice.pending().len(), 1);
    let pushed = alice.push(&mut server, 10);
    assert_eq!(pushed.applied, 1);
    alice.pull(&server, 100);
    assert!(alice.pending().is_empty());
    assert_eq!(alice.state(), server.state());
}

#[test]
fn a_tampered_cursor_hash_resets() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    alice
        .mutate("project.create", json!({"name": "A"}))
        .unwrap();
    alice.sync(&mut server);
    alice
        .conn
        .execute("UPDATE store_meta SET cursor_hash = x'00'", [])
        .unwrap();
    assert!(alice.pull_once(&server, 100).reset);
    alice.pull(&server, 100);
    assert_eq!(alice.state(), server.state());
}

#[test]
fn undo_restores_blobs_from_pre_images() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let project = alice
        .mutate("project.create", json!({"name": "P"}))
        .unwrap();
    let task = id(&alice
        .mutate(
            "task.create",
            json!({"project_id": project.value["id"], "title": "T", "attachment_hex": "cafe00"}),
        )
        .unwrap()
        .value);
    alice.mutate("task.complete", json!({"id": task})).unwrap();
    server
        .server_write("project.create", &json!({"name": "Other"}))
        .unwrap();
    alice.pull(&server, 100);
    let attachment: SqlValue = alice
        .conn
        .query_row(
            "SELECT attachment FROM task WHERE id = ?1",
            [&task],
            |row| row.get(0),
        )
        .unwrap();
    assert_eq!(attachment, SqlValue::Blob(vec![0xca, 0xfe, 0x00]));
}

#[test]
fn local_only_columns_survive_a_rebase() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let project = alice
        .mutate("project.create", json!({"name": "P"}))
        .unwrap();
    let task = id(&alice
        .mutate(
            "task.create",
            json!({"project_id": project.value["id"], "title": "T"}),
        )
        .unwrap()
        .value);
    alice.sync(&mut server);
    alice
        .conn
        .execute("UPDATE task SET local_note = 'remember'", [])
        .unwrap();
    alice
        .mutate("task.rename", json!({"id": task, "title": "T2"}))
        .unwrap();
    server
        .server_write("task.increment", &json!({"id": task}))
        .unwrap();
    alice.pull(&server, 100);
    let (title, counter, note): (String, i64, String) = alice
        .conn
        .query_row(
            "SELECT title, counter, local_note FROM task WHERE id = ?1",
            [&task],
            |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)),
        )
        .unwrap();
    assert_eq!(
        (title.as_str(), counter, note.as_str()),
        ("T2", 1, "remember")
    );
}
