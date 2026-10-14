-- store write path, schema version 1.
-- Applied by the product into its own Postgres. Every object is prefixed
-- store_ and never touches product tables except through the registered
-- capture trigger.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE store_schema (
  version integer PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO store_schema (version) VALUES (1);

CREATE TABLE store_scope (
  scope_id uuid PRIMARY KEY,
  epoch integer NOT NULL DEFAULT 1,
  head_version bigint NOT NULL DEFAULT 0,
  head_hash bytea NOT NULL DEFAULT '\x'::bytea,
  retained_from bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX store_scope_updated ON store_scope (updated_at);

CREATE TABLE store_client (
  client_id uuid PRIMARY KEY,
  scope_id uuid NOT NULL REFERENCES store_scope ON DELETE CASCADE,
  last_mutation_id bigint NOT NULL DEFAULT 0,
  last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX store_client_scope ON store_client (scope_id);

CREATE TABLE store_log (
  scope_id uuid NOT NULL REFERENCES store_scope ON DELETE CASCADE,
  version bigint NOT NULL,
  seq integer NOT NULL,
  kind text NOT NULL CHECK (kind IN ('commit', 'rejection', 'patch')),
  table_name text,
  row_id text,
  op text CHECK (op IN ('upsert', 'delete')),
  data jsonb,
  client_id uuid NOT NULL,
  mutation_id bigint NOT NULL,
  hash bytea,
  committed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (scope_id, version, seq)
);

CREATE TABLE store_table (
  table_name text PRIMARY KEY,
  id_column text NOT NULL,
  excluded_columns text[] NOT NULL DEFAULT '{}'
);

-- The client id that records writes made by the product itself (REST
-- endpoints, jobs, imports) rather than by a syncing device.
CREATE FUNCTION store_server_client() RETURNS uuid
LANGUAGE sql IMMUTABLE AS $$ SELECT '00000000-0000-0000-0000-000000000000'::uuid $$;

CREATE FUNCTION store_open(p_scope uuid, p_client uuid, p_mutation bigint)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  v_head bigint;
BEGIN
  SELECT head_version INTO v_head FROM store_scope WHERE scope_id = p_scope FOR UPDATE;
  PERFORM set_config('store.scope', p_scope::text, true);
  PERFORM set_config('store.client', p_client::text, true);
  PERFORM set_config('store.mutation', p_mutation::text, true);
  PERFORM set_config('store.version', (v_head + 1)::text, true);
  PERFORM set_config('store.seq', '0', true);
END $$;

CREATE FUNCTION store_ensure_scope(p_scope uuid) RETURNS void
LANGUAGE sql AS $$
  INSERT INTO store_scope (scope_id) VALUES (p_scope) ON CONFLICT DO NOTHING
$$;

-- Starts one device mutation. Returns 'duplicate' when this
-- (client, mutation) was already processed; the caller then commits without
-- running the mutation and reports DUPLICATE. Otherwise returns 'begun' and
-- holds the scope lock until the transaction ends.
CREATE FUNCTION store_begin(p_scope uuid, p_client uuid, p_mutation bigint)
RETURNS text LANGUAGE plpgsql AS $$
DECLARE
  v_owner uuid;
  v_last bigint;
BEGIN
  IF p_client = store_server_client() THEN
    RAISE EXCEPTION 'store: the server client id is reserved' USING ERRCODE = '22023';
  END IF;
  IF p_mutation < 1 THEN
    RAISE EXCEPTION 'store: mutation ids start at 1' USING ERRCODE = '22023';
  END IF;
  PERFORM store_ensure_scope(p_scope);
  PERFORM 1 FROM store_scope WHERE scope_id = p_scope FOR UPDATE;
  SELECT scope_id, last_mutation_id INTO v_owner, v_last
    FROM store_client WHERE client_id = p_client FOR UPDATE;
  IF v_owner IS NULL THEN
    INSERT INTO store_client (client_id, scope_id) VALUES (p_client, p_scope);
    v_last := 0;
  ELSIF v_owner <> p_scope THEN
    RAISE EXCEPTION 'store: client % belongs to another scope', p_client USING ERRCODE = '42501';
  END IF;
  UPDATE store_client SET last_seen_at = now() WHERE client_id = p_client;
  IF p_mutation <= v_last THEN
    RETURN 'duplicate';
  END IF;
  PERFORM store_open(p_scope, p_client, p_mutation);
  RETURN 'begun';
END $$;

-- Starts a write made by the product itself. It is logged like any mutation,
-- under the server client id, so every device pulls it.
CREATE FUNCTION store_begin_server(p_scope uuid) RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
  v_head bigint;
BEGIN
  PERFORM store_ensure_scope(p_scope);
  SELECT head_version INTO v_head FROM store_scope WHERE scope_id = p_scope FOR UPDATE;
  PERFORM store_open(p_scope, store_server_client(), v_head + 1);
END $$;

CREATE FUNCTION store_require_open() RETURNS uuid
LANGUAGE plpgsql AS $$
DECLARE
  v_scope text := current_setting('store.scope', true);
BEGIN
  IF v_scope IS NULL OR v_scope = '' THEN
    RAISE EXCEPTION 'store: no mutation is open; call store_begin first' USING ERRCODE = '55000';
  END IF;
  RETURN v_scope::uuid;
END $$;

-- Row trigger on every synced table: appends the row change to the open
-- version. Writes outside the write path fail, unless store.bypass is on
-- (migrations that must not be synced).
CREATE FUNCTION store_capture() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  v_table store_table;
  v_scope uuid;
  v_seq integer;
  v_row jsonb;
  v_op text;
BEGIN
  IF current_setting('store.bypass', true) = 'on' THEN
    RETURN NULL;
  END IF;
  v_scope := store_require_open();
  SELECT * INTO v_table FROM store_table WHERE table_name = TG_TABLE_NAME;
  IF TG_OP = 'DELETE' THEN
    v_row := to_jsonb(OLD);
    v_op := 'delete';
  ELSE
    v_row := to_jsonb(NEW);
    v_op := 'upsert';
    IF TG_OP = 'UPDATE' AND to_jsonb(OLD) = v_row THEN
      RETURN NULL;
    END IF;
  END IF;
  v_seq := current_setting('store.seq')::integer + 1;
  PERFORM set_config('store.seq', v_seq::text, true);
  INSERT INTO store_log (scope_id, version, seq, kind, table_name, row_id, op, data, client_id, mutation_id)
  VALUES (
    v_scope,
    current_setting('store.version')::bigint,
    v_seq,
    'patch',
    TG_TABLE_NAME,
    v_row ->> v_table.id_column,
    v_op,
    CASE WHEN v_op = 'delete' THEN NULL ELSE v_row - v_table.excluded_columns END,
    current_setting('store.client')::uuid,
    current_setting('store.mutation')::bigint
  );
  RETURN NULL;
END $$;

CREATE FUNCTION store_close(p_kind text, p_reason text) RETURNS bigint
LANGUAGE plpgsql AS $$
DECLARE
  v_scope uuid := store_require_open();
  v_version bigint := current_setting('store.version')::bigint;
  v_client uuid := current_setting('store.client')::uuid;
  v_mutation bigint := current_setting('store.mutation')::bigint;
  v_prev bytea;
  v_patches bytea;
  v_hash bytea;
BEGIN
  SELECT head_hash INTO v_prev FROM store_scope WHERE scope_id = v_scope;
  SELECT digest(coalesce(string_agg(
           seq::text || '|' || table_name || '|' || row_id || '|' || op || '|' || coalesce(data::text, ''),
           E'\n' ORDER BY seq), ''), 'sha256')
    INTO v_patches
    FROM store_log
   WHERE scope_id = v_scope AND version = v_version AND kind = 'patch';
  IF p_kind = 'rejection' THEN
    v_patches := digest(coalesce(p_reason, ''), 'sha256');
  END IF;
  v_hash := digest(
    v_prev || convert_to(v_version::text || '|' || v_client::text || '|' || v_mutation::text || '|' || p_kind, 'UTF8') || v_patches,
    'sha256');
  INSERT INTO store_log (scope_id, version, seq, kind, data, client_id, mutation_id, hash)
  VALUES (v_scope, v_version, 0, p_kind,
          CASE WHEN p_kind = 'rejection' THEN jsonb_build_object('reason', p_reason) END,
          v_client, v_mutation, v_hash);
  UPDATE store_scope
     SET head_version = v_version, head_hash = v_hash, updated_at = now()
   WHERE scope_id = v_scope;
  IF v_client <> store_server_client() THEN
    UPDATE store_client SET last_mutation_id = v_mutation WHERE client_id = v_client;
  END IF;
  PERFORM set_config('store.scope', '', true);
  RETURN v_version;
END $$;

-- Closes the open mutation: chains its hash, advances the scope head and the
-- client's last_mutation_id. Returns the new version.
CREATE FUNCTION store_end() RETURNS bigint
LANGUAGE sql AS $$ SELECT store_close('commit', NULL) $$;

-- Records the open mutation as rejected. Call it after rolling back to a
-- savepoint taken right after store_begin, so none of its row changes stay.
-- Every device learns about the rejection from the log.
CREATE FUNCTION store_reject(p_reason text) RETURNS bigint
LANGUAGE plpgsql AS $$
DECLARE
  v_scope uuid := store_require_open();
BEGIN
  DELETE FROM store_log
   WHERE scope_id = v_scope AND version = current_setting('store.version')::bigint;
  PERFORM set_config('store.seq', '0', true);
  RETURN store_close('rejection', p_reason);
END $$;

-- Puts a product table under sync. Its changes are only allowed inside the
-- write path from now on.
CREATE FUNCTION store_register(p_table regclass, p_id_column text DEFAULT 'id', p_excluded text[] DEFAULT '{}')
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  v_name text := (SELECT relname FROM pg_class WHERE oid = p_table);
BEGIN
  INSERT INTO store_table (table_name, id_column, excluded_columns)
  VALUES (v_name, p_id_column, p_excluded)
  ON CONFLICT (table_name) DO UPDATE SET id_column = excluded.id_column, excluded_columns = excluded.excluded_columns;
  EXECUTE format('DROP TRIGGER IF EXISTS store_capture ON %s', p_table);
  EXECUTE format(
    'CREATE TRIGGER store_capture AFTER INSERT OR UPDATE OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION store_capture()',
    p_table);
END $$;

-- Signals that the scope advanced. Call it after COMMIT, never inside the
-- mutation transaction, so NOTIFY's commit lock stays off the write path.
CREATE FUNCTION store_poke(p_scope uuid, p_version bigint) RETURNS void
LANGUAGE sql AS $$ SELECT pg_notify('store_poke', p_scope::text || ':' || p_version::text) $$;
