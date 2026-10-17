CREATE TABLE store_meta (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  schema_version INTEGER NOT NULL,
  client_id TEXT NOT NULL,
  scope_id TEXT,
  epoch INTEGER NOT NULL DEFAULT 0,
  cursor_version INTEGER NOT NULL DEFAULT 0,
  cursor_hash BLOB NOT NULL DEFAULT x'',
  last_mutation_id INTEGER NOT NULL DEFAULT 0,
  issued_mutation_id INTEGER NOT NULL DEFAULT 0,
  pushed_mutation_id INTEGER NOT NULL DEFAULT 0,
  min_version INTEGER NOT NULL DEFAULT 0,
  hlc INTEGER NOT NULL DEFAULT 0,
  product_schema_version INTEGER NOT NULL DEFAULT 0,
  last_pull_at INTEGER,
  capture_mutation_id INTEGER
);

CREATE TABLE store_table (
  table_name TEXT PRIMARY KEY,
  id_column TEXT NOT NULL
);

CREATE TABLE store_pending (
  mutation_id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  args TEXT NOT NULL,
  hlc INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE store_undo (
  seq INTEGER PRIMARY KEY,
  mutation_id INTEGER NOT NULL,
  table_name TEXT NOT NULL,
  row_id TEXT NOT NULL,
  before TEXT,
  UNIQUE (mutation_id, table_name, row_id)
);

CREATE TABLE store_issue (
  id INTEGER PRIMARY KEY,
  mutation_id INTEGER NOT NULL UNIQUE,
  name TEXT NOT NULL,
  args TEXT NOT NULL,
  reason TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  dismissed INTEGER NOT NULL DEFAULT 0
);
