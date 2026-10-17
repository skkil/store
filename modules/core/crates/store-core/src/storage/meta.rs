use rusqlite::Connection;
use uuid::{Builder, Uuid};

use crate::{Error, Result, clock::Hlc};

/// The single `store_meta` row: identity, cursor and counters.
#[derive(Debug, Clone)]
pub(crate) struct Meta {
    pub client_id: Uuid,
    pub scope_id: Option<Uuid>,
    pub epoch: u32,
    pub cursor_version: u64,
    pub cursor_hash: Vec<u8>,
    pub last_mutation_id: u64,
    pub issued_mutation_id: u64,
    pub pushed_mutation_id: u64,
    pub min_version: u64,
    pub hlc: Hlc,
    pub product_schema_version: u32,
    pub last_pull_at: Option<i64>,
}

impl Meta {
    pub fn create(conn: &Connection, schema_version: i64) -> Result<()> {
        let client_id = new_client_id(conn)?;
        conn.execute(
            "INSERT INTO store_meta (id, schema_version, client_id) VALUES (1, ?1, ?2)",
            (schema_version, client_id.to_string()),
        )?;
        Ok(())
    }

    pub fn load(conn: &Connection) -> Result<Self> {
        let row = conn.query_row(
            "SELECT client_id, scope_id, epoch, cursor_version, cursor_hash, last_mutation_id,
                    issued_mutation_id, pushed_mutation_id, min_version, hlc,
                    product_schema_version, last_pull_at
               FROM store_meta",
            [],
            |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, Option<String>>(1)?,
                    Self {
                        client_id: Uuid::nil(),
                        scope_id: None,
                        epoch: row.get(2)?,
                        cursor_version: row.get::<_, i64>(3)? as u64,
                        cursor_hash: row.get(4)?,
                        last_mutation_id: row.get::<_, i64>(5)? as u64,
                        issued_mutation_id: row.get::<_, i64>(6)? as u64,
                        pushed_mutation_id: row.get::<_, i64>(7)? as u64,
                        min_version: row.get::<_, i64>(8)? as u64,
                        hlc: Hlc::from_i64(row.get(9)?),
                        product_schema_version: row.get(10)?,
                        last_pull_at: row.get(11)?,
                    },
                ))
            },
        )?;
        let (client_id, scope_id, mut meta) = row;
        meta.client_id = parse_uuid(&client_id)?;
        meta.scope_id = scope_id.as_deref().map(parse_uuid).transpose()?;
        Ok(meta)
    }

    pub fn reset_identity(conn: &Connection) -> Result<()> {
        conn.execute(
            "UPDATE store_meta
                SET client_id = ?1, scope_id = NULL, epoch = 0, cursor_version = 0,
                    cursor_hash = x'', last_mutation_id = 0, issued_mutation_id = 0,
                    pushed_mutation_id = 0, min_version = 0, hlc = 0, last_pull_at = NULL,
                    capture_mutation_id = NULL",
            [new_client_id(conn)?.to_string()],
        )?;
        Ok(())
    }
}

/// Turns pre-image capture on for one mutation, or off.
pub(crate) struct Capture;

impl Capture {
    pub fn start(conn: &Connection, mutation_id: u64) -> Result<()> {
        conn.execute(
            "UPDATE store_meta SET capture_mutation_id = ?1",
            [mutation_id as i64],
        )?;
        Ok(())
    }

    pub fn stop(conn: &Connection) -> Result<()> {
        conn.execute("UPDATE store_meta SET capture_mutation_id = NULL", [])?;
        Ok(())
    }
}

fn new_client_id(conn: &Connection) -> Result<Uuid> {
    let random: Vec<u8> = conn.query_row("SELECT randomblob(16)", [], |row| row.get(0))?;
    let bytes: [u8; 16] = random
        .try_into()
        .map_err(|_| Error::Corrupt("randomblob(16) returned the wrong length".into()))?;
    Ok(Builder::from_random_bytes(bytes).into_uuid())
}

fn parse_uuid(text: &str) -> Result<Uuid> {
    Uuid::parse_str(text).map_err(|_| Error::Corrupt(format!("invalid uuid {text}")))
}
