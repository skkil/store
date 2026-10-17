use rusqlite::{Connection, params_from_iter, types::Value as SqlValue};
use serde_json::{Map, Value};

use crate::{
    Result,
    storage::{ImageEncoding, TableInfo, ident::quote_ident, json_to_sql},
};

/// Inserts or updates row `row_id` of `table` from a JSON image, writing only
/// the columns the local table has. Local columns missing from the image keep
/// their value (or their default on insert).
pub(crate) fn upsert_row(
    conn: &Connection,
    table: &TableInfo,
    row_id: &str,
    image: &Map<String, Value>,
    encoding: ImageEncoding,
) -> Result<()> {
    let mut columns = vec![table.id_column.as_str()];
    let mut values = vec![SqlValue::Text(row_id.to_owned())];
    for (key, value) in image {
        match table.column(key) {
            Some(column) if column != table.id_column => {
                columns.push(column);
                values.push(json_to_sql(value, encoding));
            }
            _ => {}
        }
    }
    let names = columns
        .iter()
        .map(|column| quote_ident(column))
        .collect::<Vec<_>>();
    let placeholders = (1..=names.len())
        .map(|index| format!("?{index}"))
        .collect::<Vec<_>>();
    let on_conflict = if names.len() == 1 {
        "DO NOTHING".to_owned()
    } else {
        let updates = names[1..]
            .iter()
            .map(|name| format!("{name} = excluded.{name}"))
            .collect::<Vec<_>>()
            .join(", ");
        format!("DO UPDATE SET {updates}")
    };
    let sql = format!(
        "INSERT INTO {table} ({columns}) VALUES ({placeholders}) ON CONFLICT ({id}) {on_conflict}",
        table = quote_ident(&table.name),
        columns = names.join(", "),
        placeholders = placeholders.join(", "),
        id = names[0],
    );
    conn.prepare_cached(&sql)?
        .execute(params_from_iter(values))?;
    Ok(())
}

/// Deletes row `row_id` of `table`; a missing row is not an error.
pub(crate) fn delete_row(conn: &Connection, table: &TableInfo, row_id: &str) -> Result<()> {
    let sql = format!(
        "DELETE FROM {} WHERE {} = ?1",
        quote_ident(&table.name),
        quote_ident(&table.id_column)
    );
    conn.prepare_cached(&sql)?.execute([row_id])?;
    Ok(())
}

/// Deletes every row of `table`.
pub(crate) fn clear_table(conn: &Connection, table: &TableInfo) -> Result<()> {
    conn.execute(&format!("DELETE FROM {}", quote_ident(&table.name)), [])?;
    Ok(())
}
