//! Applying pull responses: column tolerance, coercion, cursor, acks and errors.

use store_core::{
    Error, apply_pull_response, bind_scope, dismiss_issue, forget, issues,
    protocol::v1::{
        Change, Cursor, Entries, PullResponse, Rejection, Version, change::Op, pull_response::Body,
    },
    pull_request,
    rusqlite::types::Value as SqlValue,
    serde_json::json,
    status,
    uuid::Uuid,
};
use store_testkit::{Client, FakeServer, PlannerMutator};

const PROJECT: &str = "0192e000-0000-7000-8000-000000000001";

fn version(number: u64, changes: Vec<Change>) -> Version {
    Version {
        version: number,
        client_id: Uuid::nil().to_string(),
        mutation_id: number,
        hash: vec![number as u8; 4],
        committed_at: 0,
        changes,
        rejection: None,
    }
}

fn upsert(table: &str, id: &str, data: store_core::serde_json::Value) -> Change {
    Change {
        table: table.into(),
        row_id: id.into(),
        op: Op::Upsert as i32,
        data: data.to_string().into_bytes(),
    }
}

fn entries(versions: Vec<Version>, more: bool, last_mutation_id: u64) -> PullResponse {
    PullResponse {
        body: Some(Body::Entries(Entries { versions, more })),
        last_mutation_id,
        head_version: 0,
        epoch: 1,
    }
}

fn apply(
    client: &mut Client,
    response: &PullResponse,
) -> store_core::Result<store_core::PullOutcome> {
    apply_pull_response(&mut client.conn, &PlannerMutator, response)
}

#[test]
fn upserts_only_known_columns_and_coerces_booleans_and_json() {
    let mut client = Client::new();
    let response = entries(
        vec![version(
            1,
            vec![
                upsert(
                    "project",
                    PROJECT,
                    json!({"id": PROJECT, "name": "Home", "archived": true, "created_at": 5, "color": "red"}),
                ),
                upsert(
                    "task",
                    "t1",
                    json!({"id": "t1", "project_id": PROJECT, "title": "T", "done": false, "position": 0,
                           "counter": 2, "labels": ["a", {"b": 1}], "created_at": 6, "server_only": "x"}),
                ),
                upsert("comment", "c1", json!({"id": "c1"})),
            ],
        )],
        false,
        0,
    );
    let outcome = apply(&mut client, &response).unwrap();
    assert_eq!(outcome.changed_tables, vec!["project", "task"]);
    assert_eq!(outcome.versions, 1);
    let state = client.state();
    assert_eq!(
        state.value("project", PROJECT, "archived"),
        Some(&SqlValue::Integer(1))
    );
    assert_eq!(
        state.value("task", "t1", "done"),
        Some(&SqlValue::Integer(0))
    );
    assert_eq!(
        state.value("task", "t1", "labels"),
        Some(&SqlValue::Text(r#"["a",{"b":1}]"#.into()))
    );
    let note: String = client
        .conn
        .query_row("SELECT local_note FROM task WHERE id = 't1'", [], |row| {
            row.get(0)
        })
        .unwrap();
    assert_eq!(note, "");
}

#[test]
fn upserts_keep_local_only_columns_and_deletes_remove_rows() {
    let mut client = Client::new();
    let project = json!({"id": PROJECT, "name": "Home", "archived": false, "created_at": 5});
    let task = json!({"id": "t1", "project_id": PROJECT, "title": "T", "done": false, "position": 0,
                      "counter": 0, "labels": [], "created_at": 6});
    apply(
        &mut client,
        &entries(
            vec![version(
                1,
                vec![
                    upsert("project", PROJECT, project),
                    upsert("task", "t1", task.clone()),
                ],
            )],
            false,
            0,
        ),
    )
    .unwrap();
    client
        .conn
        .execute("UPDATE task SET local_note = 'mine'", [])
        .unwrap();
    let mut renamed = task;
    renamed["title"] = json!("Renamed");
    apply(
        &mut client,
        &entries(
            vec![version(2, vec![upsert("task", "t1", renamed)])],
            false,
            0,
        ),
    )
    .unwrap();
    let (title, note): (String, String) = client
        .conn
        .query_row(
            "SELECT title, local_note FROM task WHERE id = 't1'",
            [],
            |row| Ok((row.get(0)?, row.get(1)?)),
        )
        .unwrap();
    assert_eq!((title.as_str(), note.as_str()), ("Renamed", "mine"));
    let delete = Change {
        table: "project".into(),
        row_id: PROJECT.into(),
        op: Op::Delete as i32,
        data: Vec::new(),
    };
    apply(
        &mut client,
        &entries(vec![version(3, vec![delete])], false, 0),
    )
    .unwrap();
    assert_eq!(client.state().count("project"), 0);
    assert_eq!(client.state().count("task"), 0);
}

#[test]
fn the_cursor_advances_to_the_last_version() {
    let mut client = Client::new();
    apply(
        &mut client,
        &entries(vec![version(1, vec![]), version(2, vec![])], true, 0),
    )
    .unwrap();
    let request = pull_request(&client.conn, 50).unwrap();
    assert_eq!(
        request.cursor,
        Some(Cursor {
            epoch: 1,
            version: 2,
            hash: vec![2; 4]
        })
    );
    assert_eq!(request.limit, 50);
    assert_eq!(request.protocol_version, 1);
    assert!(status(&client.conn).unwrap().last_pull_at.is_some());
}

#[test]
fn gaps_epoch_changes_and_empty_bodies_are_protocol_errors() {
    let mut client = Client::new();
    assert!(matches!(
        apply(&mut client, &entries(vec![version(2, vec![])], false, 0)),
        Err(Error::Protocol(_))
    ));
    apply(&mut client, &entries(vec![version(1, vec![])], false, 0)).unwrap();
    let mut other_epoch = entries(vec![version(2, vec![])], false, 0);
    other_epoch.epoch = 2;
    assert!(matches!(
        apply(&mut client, &other_epoch),
        Err(Error::Protocol(_))
    ));
    let empty = PullResponse::default();
    assert!(matches!(
        apply(&mut client, &empty),
        Err(Error::Protocol(_))
    ));
    assert_eq!(status(&client.conn).unwrap().cursor_version, 1);
}

#[test]
fn a_failed_pull_changes_nothing() {
    let mut client = Client::new();
    client
        .mutate("project.create", json!({"name": "Mine"}))
        .unwrap();
    let bad = entries(
        vec![version(
            1,
            vec![Change {
                table: "project".into(),
                row_id: "p".into(),
                op: Op::Upsert as i32,
                data: b"[1]".to_vec(),
            }],
        )],
        false,
        0,
    );
    assert!(matches!(apply(&mut client, &bad), Err(Error::Protocol(_))));
    assert_eq!(client.state().count("project"), 1);
    assert_eq!(client.pending().len(), 1);
    assert_eq!(status(&client.conn).unwrap().cursor_version, 0);
}

#[test]
fn rejections_of_own_pending_mutations_become_issues() {
    let mut client = Client::new();
    client
        .mutate("project.create", json!({"name": "Mine"}))
        .unwrap();
    let mut rejected = version(1, vec![]);
    rejected.client_id = client.client_id().to_string();
    rejected.mutation_id = 1;
    rejected.rejection = Some(Rejection {
        mutation_id: 1,
        reason: "quota exceeded".into(),
    });
    let outcome = apply(&mut client, &entries(vec![rejected.clone()], false, 1)).unwrap();
    assert_eq!(outcome.new_issues, 1);
    assert_eq!(outcome.acked_mutation_id, 1);
    assert!(client.pending().is_empty());
    assert_eq!(client.state().count("project"), 0);
    let open = issues(&client.conn).unwrap();
    assert_eq!(open.len(), 1);
    assert_eq!(open[0].mutation_id, 1);
    assert_eq!(open[0].name, "project.create");
    assert_eq!(open[0].args, json!({"name": "Mine"}));
    assert_eq!(open[0].reason, "quota exceeded");
    assert_eq!(status(&client.conn).unwrap().open_issues, 1);
    assert!(dismiss_issue(&client.conn, open[0].id).unwrap());
    assert!(!dismiss_issue(&client.conn, open[0].id).unwrap());
    assert!(issues(&client.conn).unwrap().is_empty());
}

#[test]
fn rejections_of_other_clients_are_not_issues() {
    let mut client = Client::new();
    let mut rejected = version(1, vec![]);
    rejected.client_id = Uuid::from_u128(9).to_string();
    rejected.rejection = Some(Rejection {
        mutation_id: 1,
        reason: "no".into(),
    });
    let outcome = apply(&mut client, &entries(vec![rejected], false, 0)).unwrap();
    assert_eq!(outcome.new_issues, 0);
    assert!(issues(&client.conn).unwrap().is_empty());
}

#[test]
fn the_ack_waits_for_the_versions_while_more_are_coming() {
    let mut client = Client::new();
    let mut server = FakeServer::new();
    for index in 0..3 {
        server
            .server_write("project.create", &json!({"name": format!("S{index}")}))
            .unwrap();
    }
    client
        .mutate("project.create", json!({"name": "Mine"}))
        .unwrap();
    client.push(&mut server, 10);
    let first = client.pull_once(&server, 2);
    assert!(first.more);
    assert_eq!(first.acked_mutation_id, 0);
    assert_eq!(client.pending().len(), 1);
    assert_eq!(client.state().count("project"), 3);
    let second = client.pull_once(&server, 2);
    assert!(!second.more);
    assert_eq!(second.acked_mutation_id, 1);
    assert!(client.pending().is_empty());
    assert_eq!(client.state(), server.state());
}

#[test]
fn an_empty_pull_with_nothing_new_touches_nothing() {
    let mut client = Client::new();
    client
        .mutate("project.create", json!({"name": "Mine"}))
        .unwrap();
    let outcome = apply(&mut client, &entries(vec![], false, 0)).unwrap();
    assert!(outcome.changed_tables.is_empty());
    assert_eq!(client.pending().len(), 1);
}

#[test]
fn status_reports_the_local_state() {
    let mut client = Client::new();
    let fresh = status(&client.conn).unwrap();
    assert_eq!(fresh.pending, 0);
    assert_eq!(fresh.oldest_pending_at, None);
    assert_eq!(fresh.scope_id, None);
    assert_eq!(fresh.last_pull_at, None);
    let outcome = client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap();
    client
        .mutate("project.create", json!({"name": "B"}))
        .unwrap();
    let busy = status(&client.conn).unwrap();
    assert_eq!(busy.pending, 2);
    let oldest = busy.oldest_pending_at.unwrap();
    assert!((oldest - outcome.hlc.millis()).abs() < 1_000);
}

#[test]
fn a_store_binds_to_one_scope() {
    let client = Client::new();
    let scope = Uuid::from_u128(1);
    bind_scope(&client.conn, scope).unwrap();
    bind_scope(&client.conn, scope).unwrap();
    assert_eq!(status(&client.conn).unwrap().scope_id, Some(scope));
    assert!(matches!(
        bind_scope(&client.conn, Uuid::from_u128(2)),
        Err(Error::ScopeMismatch { bound, .. }) if bound == scope
    ));
}

#[test]
fn forget_clears_everything_and_makes_a_new_client() {
    let mut client = Client::new();
    let mut server = FakeServer::new();
    bind_scope(&client.conn, Uuid::from_u128(1)).unwrap();
    client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap();
    client.sync(&mut server);
    client
        .mutate("project.create", json!({"name": "B"}))
        .unwrap();
    let old = client.client_id();
    let new = forget(&mut client.conn).unwrap();
    assert_ne!(new, old);
    let after = status(&client.conn).unwrap();
    assert_eq!(after.client_id, new);
    assert_eq!(after.scope_id, None);
    assert_eq!(after.pending, 0);
    assert_eq!(after.cursor_version, 0);
    assert_eq!(client.state().count("project"), 0);
    bind_scope(&client.conn, Uuid::from_u128(2)).unwrap();
    let next = client
        .mutate("project.create", json!({"name": "C"}))
        .unwrap();
    assert_eq!(next.mutation_id, 1);
}

#[test]
fn status_and_outcomes_serialise_for_javascript_hosts() {
    let mut client = Client::new();
    client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap();
    let status: store_core::serde_json::Value =
        store_core::serde_json::from_str(&store_core::status_json(&client.conn).unwrap()).unwrap();
    assert_eq!(status["clientId"], json!(client.client_id().to_string()));
    assert_eq!(status["pending"], json!(1));
    assert_eq!(status["issues"], json!(0));
    assert_eq!(status["cursorVersion"], json!(0));
    let outcome = apply(&mut client, &entries(vec![version(1, vec![])], true, 0)).unwrap();
    let outcome = store_core::serde_json::to_value(&outcome).unwrap();
    assert_eq!(outcome["changedTables"], json!(["project"]));
    assert_eq!(outcome["more"], json!(true));
    assert_eq!(outcome["reset"], json!(false));
}
