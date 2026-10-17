use rusqlite::Connection;

use crate::{
    Result,
    storage::{
        TableInfo,
        ident::{quote_ident, quote_literal},
        value::BLOB_KEY,
    },
};

const EVENTS: [&str; 3] = ["insert", "update", "delete"];

pub(crate) fn drop_triggers(conn: &Connection, table: &str) -> Result<()> {
    for event in EVENTS {
        conn.execute_batch(&format!(
            "DROP TRIGGER IF EXISTS {}",
            quote_ident(&trigger_name(event, table))
        ))?;
    }
    Ok(())
}

pub(crate) fn create_triggers(conn: &Connection, table: &TableInfo) -> Result<()> {
    let id = quote_ident(&table.id_column);
    let old_image = image(table, "OLD");
    let record = |row_id: String, before: &str, condition: &str| {
        format!(
            "INSERT OR IGNORE INTO store_undo (mutation_id, table_name, row_id, before)
             SELECT capture_mutation_id, {name}, CAST({row_id} AS TEXT), {before}
               FROM store_meta WHERE capture_mutation_id IS NOT NULL{condition};",
            name = quote_literal(&table.name),
        )
    };
    let bodies = [
        record(format!("NEW.{id}"), "NULL", ""),
        format!(
            "{}\n{}",
            record(format!("OLD.{id}"), &old_image, ""),
            record(
                format!("NEW.{id}"),
                "NULL",
                &format!(" AND NEW.{id} IS NOT OLD.{id}")
            ),
        ),
        record(format!("OLD.{id}"), &old_image, ""),
    ];
    for (event, body) in EVENTS.iter().zip(bodies) {
        conn.execute_batch(&format!(
            "CREATE TRIGGER {trigger} AFTER {event} ON {table}
             WHEN (SELECT capture_mutation_id FROM store_meta) IS NOT NULL
             BEGIN
             {body}
             END",
            trigger = quote_ident(&trigger_name(event, &table.name)),
            event = event.to_uppercase(),
            table = quote_ident(&table.name),
        ))?;
    }
    Ok(())
}

fn trigger_name(event: &str, table: &str) -> String {
    format!("store_undo_{event}_{table}")
}

fn image(table: &TableInfo, row: &str) -> String {
    let pairs = table
        .columns
        .iter()
        .map(|column| {
            let value = format!("{row}.{}", quote_ident(column));
            format!(
                "{}, CASE WHEN typeof({value}) = 'blob' THEN json_object('{BLOB_KEY}', hex({value})) ELSE {value} END",
                quote_literal(column)
            )
        })
        .collect::<Vec<_>>()
        .join(", ");
    format!("json_object({pairs})")
}
