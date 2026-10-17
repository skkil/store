/// The synced tables of the planner app and their id columns.
pub const SYNCED_TABLES: &[(&str, &str)] = &[("project", "id"), ("task", "id")];

/// The client's tables: `task.local_note` exists only on devices, and
/// `task.attachment` is excluded from the server's row images.
pub const CLIENT_SCHEMA: &str = "
PRAGMA foreign_keys = ON;
CREATE TABLE project (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  archived INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE task (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES project (id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  done INTEGER NOT NULL DEFAULT 0,
  position INTEGER NOT NULL,
  counter INTEGER NOT NULL DEFAULT 0,
  labels TEXT NOT NULL DEFAULT '[]',
  attachment BLOB,
  created_at INTEGER NOT NULL,
  local_note TEXT NOT NULL DEFAULT ''
);
";

/// The server's tables: `task.server_only` never reaches devices' schema.
pub const SERVER_SCHEMA: &str = "
PRAGMA foreign_keys = ON;
CREATE TABLE project (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  archived INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE TABLE task (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL REFERENCES project (id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  done INTEGER NOT NULL DEFAULT 0,
  position INTEGER NOT NULL,
  counter INTEGER NOT NULL DEFAULT 0,
  labels TEXT NOT NULL DEFAULT '[]',
  attachment BLOB,
  created_at INTEGER NOT NULL,
  server_only TEXT NOT NULL DEFAULT 'secret'
);
";

/// The columns both sides have, compared by [`AppState`](crate::AppState).
pub(crate) const SHARED_COLUMNS: &[(&str, &[&str])] = &[
    ("project", &["id", "name", "archived", "created_at"]),
    (
        "task",
        &[
            "id",
            "project_id",
            "title",
            "done",
            "position",
            "counter",
            "labels",
            "created_at",
        ],
    ),
];
