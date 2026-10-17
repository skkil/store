//! Installing the local store and registering tables.

use store_core::{
    Error, MutationContext, MutationError, Mutator, SCHEMA_VERSION, install, mutate,
    register_table,
    rusqlite::{Connection, Transaction},
    serde_json::{Value, json},
    status,
};

fn connection() -> Connection {
    let conn = Connection::open_in_memory().unwrap();
    conn.execute_batch("CREATE TABLE note (id TEXT PRIMARY KEY, body TEXT NOT NULL);")
        .unwrap();
    conn
}

struct NoteMutator;

impl Mutator for NoteMutator {
    fn apply(
        &self,
        tx: &Transaction<'_>,
        _ctx: &mut MutationContext,
        _name: &str,
        args: &Value,
    ) -> Result<Value, MutationError> {
        tx.execute(
            "UPDATE note SET body = ?2 WHERE id = ?1",
            (args["id"].as_str(), args["body"].as_str()),
        )?;
        Ok(Value::Null)
    }
}

fn pre_image(conn: &Connection) -> Value {
    let before: String = conn
        .query_row("SELECT before FROM store_undo", [], |row| row.get(0))
        .unwrap();
    serde_json_from(&before)
}

fn serde_json_from(text: &str) -> Value {
    store_core::serde_json::from_str(text).unwrap()
}

#[test]
fn install_is_idempotent_and_keeps_the_client_id() {
    let conn = connection();
    install(&conn).unwrap();
    let first = status(&conn).unwrap().client_id;
    install(&conn).unwrap();
    assert_eq!(status(&conn).unwrap().client_id, first);
    assert_eq!(first.get_version_num(), 4);
}

#[test]
fn install_works_inside_an_open_transaction() {
    let mut conn = connection();
    let tx = conn.transaction().unwrap();
    install(&tx).unwrap();
    tx.commit().unwrap();
    assert_eq!(status(&conn).unwrap().pending, 0);
}

#[test]
fn install_refuses_a_newer_schema() {
    let conn = connection();
    install(&conn).unwrap();
    conn.execute("UPDATE store_meta SET schema_version = 99", [])
        .unwrap();
    match install(&conn) {
        Err(Error::SchemaTooNew { found, supported }) => {
            assert_eq!(found, 99);
            assert_eq!(supported, SCHEMA_VERSION);
        }
        other => panic!("expected SchemaTooNew, got {other:?}"),
    }
}

#[test]
fn register_table_validates_its_arguments() {
    let conn = connection();
    install(&conn).unwrap();
    conn.execute_batch(
        "CREATE TABLE loose (id TEXT, body TEXT);
         CREATE TABLE keyed (key TEXT NOT NULL UNIQUE, body TEXT);",
    )
    .unwrap();
    assert!(matches!(
        register_table(&conn, "missing", "id"),
        Err(Error::TableNotFound(_))
    ));
    assert!(matches!(
        register_table(&conn, "note", "uuid"),
        Err(Error::ColumnNotFound { .. })
    ));
    assert!(matches!(
        register_table(&conn, "loose", "id"),
        Err(Error::IdNotUnique { .. })
    ));
    assert!(matches!(
        register_table(&conn, "store_pending", "mutation_id"),
        Err(Error::ReservedTable(_))
    ));
    register_table(&conn, "keyed", "key").unwrap();
    register_table(&conn, "note", "ID").unwrap();
    let id_column: String = conn
        .query_row(
            "SELECT id_column FROM store_table WHERE table_name = 'note'",
            [],
            |row| row.get(0),
        )
        .unwrap();
    assert_eq!(id_column, "id");
}

#[test]
fn writes_outside_a_mutation_are_not_captured() {
    let conn = connection();
    install(&conn).unwrap();
    register_table(&conn, "note", "id").unwrap();
    conn.execute("INSERT INTO note VALUES ('a', 'local')", [])
        .unwrap();
    conn.execute("UPDATE note SET body = 'changed'", [])
        .unwrap();
    let undo: i64 = conn
        .query_row("SELECT count(*) FROM store_undo", [], |row| row.get(0))
        .unwrap();
    assert_eq!(undo, 0);
}

#[test]
fn re_registering_after_a_schema_change_captures_new_columns() {
    let mut conn = connection();
    install(&conn).unwrap();
    register_table(&conn, "note", "id").unwrap();
    conn.execute("INSERT INTO note VALUES ('a', 'first')", [])
        .unwrap();
    conn.execute_batch("ALTER TABLE note ADD COLUMN pinned INTEGER NOT NULL DEFAULT 1;")
        .unwrap();
    register_table(&conn, "note", "id").unwrap();
    mutate(
        &mut conn,
        &NoteMutator,
        "note.edit",
        &json!({"id": "a", "body": "second"}),
    )
    .unwrap();
    assert_eq!(
        pre_image(&conn),
        json!({"id": "a", "body": "first", "pinned": 1})
    );
}

#[test]
fn table_names_with_quotes_are_handled() {
    let conn = Connection::open_in_memory().unwrap();
    conn.execute_batch(r#"CREATE TABLE "it's ""odd""" ("my id" TEXT PRIMARY KEY, v TEXT);"#)
        .unwrap();
    install(&conn).unwrap();
    register_table(&conn, r#"it's "odd""#, "my id").unwrap();
}
