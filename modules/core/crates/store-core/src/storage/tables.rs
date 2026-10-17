use std::collections::BTreeMap;

use rusqlite::{Connection, OptionalExtension};

use crate::{
    Error, Result,
    storage::{triggers, with_savepoint},
};

/// A registered table and the columns it has right now.
#[derive(Debug, Clone)]
pub(crate) struct TableInfo {
    pub name: String,
    pub id_column: String,
    pub columns: Vec<String>,
}

impl TableInfo {
    fn read(conn: &Connection, name: &str, id_column: &str) -> Result<Option<Self>> {
        let columns = columns(conn, name)?;
        if columns.is_empty() {
            return Ok(None);
        }
        Ok(Some(Self {
            name: name.to_owned(),
            id_column: id_column.to_owned(),
            columns,
        }))
    }

    /// The table's column called `name`, compared like SQLite compares
    /// identifiers.
    pub fn column(&self, name: &str) -> Option<&str> {
        self.columns
            .iter()
            .find(|column| column.eq_ignore_ascii_case(name))
            .map(String::as_str)
    }
}

/// Every registered table that still exists, by name.
#[derive(Debug, Clone, Default)]
pub(crate) struct Tables {
    by_name: BTreeMap<String, TableInfo>,
}

impl Tables {
    pub fn load(conn: &Connection) -> Result<Self> {
        let mut statement = conn.prepare("SELECT table_name, id_column FROM store_table")?;
        let registered = statement
            .query_map([], |row| {
                Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?;
        let mut by_name = BTreeMap::new();
        for (name, id_column) in registered {
            if let Some(info) = TableInfo::read(conn, &name, &id_column)? {
                by_name.insert(name, info);
            }
        }
        Ok(Self { by_name })
    }

    pub fn get(&self, name: &str) -> Option<&TableInfo> {
        self.by_name.get(name)
    }

    pub fn iter(&self) -> impl Iterator<Item = &TableInfo> {
        self.by_name.values()
    }
}

/// Puts `table` under sync with `id_column` as its row id.
///
/// Installs triggers that, only while a mutation is being captured, record
/// each touched row's pre-image so a pull can undo pending mutations before
/// applying server changes. Call it after [`install`](crate::install) and
/// again after every migration that changes the table: re-registering
/// rebuilds the triggers from the current columns. The id column must be the
/// primary key or have a unique index.
pub fn register_table(conn: &Connection, table: &str, id_column: &str) -> Result<()> {
    if table.len() >= 6 && table[..6].eq_ignore_ascii_case("store_") {
        return Err(Error::ReservedTable(table.to_owned()));
    }
    let columns = columns(conn, table)?;
    if columns.is_empty() {
        return Err(Error::TableNotFound(table.to_owned()));
    }
    let id_column = columns
        .iter()
        .find(|column| column.eq_ignore_ascii_case(id_column))
        .cloned()
        .ok_or_else(|| Error::ColumnNotFound {
            table: table.to_owned(),
            column: id_column.to_owned(),
        })?;
    if !is_unique(conn, table, &id_column)? {
        return Err(Error::IdNotUnique {
            table: table.to_owned(),
            column: id_column,
        });
    }
    let info = TableInfo {
        name: table.to_owned(),
        id_column,
        columns,
    };
    with_savepoint(conn, "store_register", |conn| {
        triggers::drop_triggers(conn, &info.name)?;
        triggers::create_triggers(conn, &info)?;
        conn.execute(
            "INSERT INTO store_table (table_name, id_column) VALUES (?1, ?2)
             ON CONFLICT (table_name) DO UPDATE SET id_column = excluded.id_column",
            (&info.name, &info.id_column),
        )?;
        Ok(())
    })
}

fn columns(conn: &Connection, table: &str) -> Result<Vec<String>> {
    let mut statement =
        conn.prepare_cached("SELECT name FROM pragma_table_info(?1) ORDER BY cid")?;
    let columns = statement
        .query_map([table], |row| row.get(0))?
        .collect::<rusqlite::Result<Vec<String>>>()?;
    Ok(columns)
}

fn is_unique(conn: &Connection, table: &str, column: &str) -> Result<bool> {
    let primary_key: Vec<String> = conn
        .prepare("SELECT name FROM pragma_table_info(?1) WHERE pk > 0")?
        .query_map([table], |row| row.get(0))?
        .collect::<rusqlite::Result<_>>()?;
    if primary_key.len() == 1 && primary_key[0] == column {
        return Ok(true);
    }
    let unique_index = conn
        .query_row(
            "SELECT 1
               FROM pragma_index_list(?1) AS list
               JOIN pragma_index_info(list.name) AS info
              WHERE list.\"unique\" = 1 AND list.partial = 0
              GROUP BY list.name
             HAVING count(*) = 1 AND max(info.name) = ?2",
            (table, column),
            |_| Ok(()),
        )
        .optional()?;
    Ok(unique_index.is_some())
}
