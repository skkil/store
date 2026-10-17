//! Local mutations: queueing, pre-images, derived ids and clocks.

use store_core::{
    Error, MutationError, derive_id, install,
    rusqlite::Connection,
    serde_json::{Value, json},
};
use store_testkit::Client;

fn undo_rows(conn: &Connection) -> Vec<(u64, String, String, Option<Value>)> {
    let mut statement = conn
        .prepare("SELECT mutation_id, table_name, row_id, before FROM store_undo ORDER BY seq")
        .unwrap();
    statement
        .query_map([], |row| {
            let before: Option<String> = row.get(3)?;
            Ok((
                row.get::<_, i64>(0)? as u64,
                row.get(1)?,
                row.get(2)?,
                before.map(|text| store_core::serde_json::from_str(&text).unwrap()),
            ))
        })
        .unwrap()
        .collect::<Result<_, _>>()
        .unwrap()
}

#[test]
fn mutate_applies_queues_and_returns_the_value() {
    let mut client = Client::new();
    let outcome = client
        .mutate("project.create", json!({"name": "Home"}))
        .unwrap();
    assert_eq!(outcome.mutation_id, 1);
    let expected_id = derive_id(client.client_id(), 1, 0).to_string();
    assert_eq!(outcome.value, json!({"id": expected_id}));
    assert_eq!(client.state().count("project"), 1);
    let pending = client.pending();
    assert_eq!(pending.len(), 1);
    assert_eq!(pending[0].1, "project.create");
    assert_eq!(pending[0].2, json!({"name": "Home"}));
    assert_eq!(pending[0].3, outcome.hlc.as_i64());
}

#[test]
fn mutation_ids_and_clocks_increase() {
    let mut client = Client::new();
    let first = client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap();
    let second = client
        .mutate("project.create", json!({"name": "B"}))
        .unwrap();
    assert_eq!(second.mutation_id, first.mutation_id + 1);
    assert!(second.hlc > first.hlc);
}

#[test]
fn the_mutator_sees_the_mutation_clock() {
    let mut client = Client::new();
    let outcome = client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap();
    let id = outcome.value["id"].as_str().unwrap();
    let created_at: i64 = client
        .conn
        .query_row(
            "SELECT created_at FROM project WHERE id = ?1",
            [id],
            |row| row.get(0),
        )
        .unwrap();
    assert_eq!(created_at, outcome.hlc.millis());
}

#[test]
fn a_failing_mutation_rolls_back_and_queues_nothing() {
    let mut client = Client::new();
    let project = client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap()
        .value["id"]
        .clone();
    let error = client
        .mutate("task.create", json!({"project_id": project, "title": " "}))
        .unwrap_err();
    assert!(
        matches!(error, Error::Mutation(MutationError::Rejected(ref reason)) if reason == "title is required")
    );
    assert_eq!(client.pending().len(), 1);
    assert_eq!(client.state().count("task"), 0);
    let next = client
        .mutate("project.create", json!({"name": "B"}))
        .unwrap();
    assert_eq!(next.mutation_id, 2);
    let capture: Option<i64> = client
        .conn
        .query_row("SELECT capture_mutation_id FROM store_meta", [], |row| {
            row.get(0)
        })
        .unwrap();
    assert_eq!(capture, None);
}

#[test]
fn arguments_must_be_an_object() {
    let mut client = Client::new();
    assert!(matches!(
        client.mutate("project.create", json!(["Home"])),
        Err(Error::ArgsNotObject)
    ));
}

#[test]
fn inserts_record_an_empty_pre_image() {
    let mut client = Client::new();
    let project = client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap()
        .value["id"]
        .as_str()
        .unwrap()
        .to_owned();
    assert_eq!(
        undo_rows(&client.conn),
        vec![(1, "project".into(), project, None)]
    );
}

#[test]
fn updates_and_deletes_record_the_old_row_once_per_mutation() {
    let mut client = Client::new();
    let project = client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap()
        .value["id"]
        .clone();
    let task = client
        .mutate(
            "task.create",
            json!({"project_id": project, "title": "One", "attachment_hex": "00ff"}),
        )
        .unwrap()
        .value["id"]
        .clone();
    client
        .mutate("task.increment", json!({"id": task}))
        .unwrap();
    client.mutate("task.delete", json!({"id": task})).unwrap();
    let rows = undo_rows(&client.conn);
    assert_eq!(rows.len(), 4);
    let increment = rows[2].3.clone().unwrap();
    assert_eq!(increment["counter"], json!(0));
    assert_eq!(increment["title"], json!("One"));
    assert_eq!(increment["attachment"], json!({"$blob": "00FF"}));
    let delete = rows[3].3.clone().unwrap();
    assert_eq!(delete["counter"], json!(1));
}

#[test]
fn only_the_first_pre_image_of_a_row_counts() {
    let mut conn = Connection::open_in_memory().unwrap();
    conn.execute_batch(
        "CREATE TABLE counter (id TEXT PRIMARY KEY, n INTEGER NOT NULL);
         INSERT INTO counter VALUES ('c', 0);",
    )
    .unwrap();
    install(&conn).unwrap();
    store_core::register_table(&conn, "counter", "id").unwrap();
    struct Twice;
    impl store_core::Mutator for Twice {
        fn apply(
            &self,
            tx: &store_core::rusqlite::Transaction<'_>,
            _ctx: &mut store_core::MutationContext,
            _name: &str,
            _args: &Value,
        ) -> Result<Value, MutationError> {
            tx.execute("UPDATE counter SET n = n + 1", [])?;
            tx.execute("UPDATE counter SET n = n + 1", [])?;
            Ok(Value::Null)
        }
    }
    store_core::mutate(&mut conn, &Twice, "twice", &json!({})).unwrap();
    let rows = undo_rows(&conn);
    assert_eq!(rows.len(), 1);
    assert_eq!(rows[0].3, Some(json!({"id": "c", "n": 0})));
}

#[test]
fn derived_ids_follow_the_context_counter() {
    let mut client = Client::new();
    let project = client
        .mutate("project.create", json!({"name": "A"}))
        .unwrap();
    let task = client
        .mutate(
            "task.create",
            json!({"project_id": project.value["id"], "title": "T"}),
        )
        .unwrap();
    assert_eq!(
        task.value["id"],
        json!(derive_id(client.client_id(), task.mutation_id, 0).to_string())
    );
}
