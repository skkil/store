use rusqlite::Connection;

/// Unix milliseconds read through SQLite, so every target (native, wasm with
/// its host clock, mobile) uses the clock its SQLite build was given. Only
/// `julianday` is used, so old system SQLite builds (iOS) work too.
pub(crate) fn wall_millis(conn: &Connection) -> rusqlite::Result<i64> {
    conn.query_row(
        "SELECT CAST((julianday('now') - 2440587.5) * 86400000.0 AS INTEGER)",
        [],
        |row| row.get(0),
    )
}

#[cfg(all(test, not(target_arch = "wasm32")))]
mod tests {
    use std::time::{SystemTime, UNIX_EPOCH};

    use rusqlite::Connection;

    use super::wall_millis;

    #[test]
    fn reads_unix_milliseconds() {
        let conn = Connection::open_in_memory().unwrap();
        let system = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_millis() as i64;
        assert!((wall_millis(&conn).unwrap() - system).abs() < 2_000);
    }
}
