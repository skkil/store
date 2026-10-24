-- Functions for what products did by reading or writing store_ internals:
-- closing a server write that changed nothing, claiming a client for a sync
-- token and dropping a scope. Captured updates that change only excluded
-- columns are no longer logged.

UPDATE store_schema SET version = 3;

-- True while a mutation (device or server) is open on this transaction.
CREATE FUNCTION store_is_open() RETURNS boolean
LANGUAGE sql STABLE AS $$ SELECT coalesce(current_setting('store.scope', true), '') <> '' $$;

CREATE FUNCTION store_require_server_open() RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  PERFORM store_require_open();
  IF current_setting('store.client')::uuid <> store_server_client() THEN
    RAISE EXCEPTION 'store: a device mutation must end or be rejected' USING ERRCODE = '22023';
  END IF;
END $$;

-- Closes the open server write without a version. Only valid while it has
-- captured no row change; roll back to a savepoint first otherwise.
CREATE FUNCTION store_abandon() RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  PERFORM store_require_server_open();
  IF current_setting('store.seq')::integer > 0 THEN
    RAISE EXCEPTION 'store: the open write changed synced rows; end it or roll back first' USING ERRCODE = '22023';
  END IF;
  PERFORM set_config('store.scope', '', true);
END $$;

-- Closes the open server write: like store_end when it captured a row
-- change, otherwise like store_abandon, returning NULL.
CREATE FUNCTION store_end_if_changed() RETURNS bigint
LANGUAGE plpgsql AS $$
BEGIN
  PERFORM store_require_server_open();
  IF current_setting('store.seq')::integer = 0 THEN
    PERFORM store_abandon();
    RETURN NULL;
  END IF;
  RETURN store_end();
END $$;

-- Binds a client to a scope for life, as store_begin does on its first
-- mutation, so a product can check ownership before issuing a sync token.
-- Returns false when the client already belongs to another scope.
CREATE FUNCTION store_claim_client(p_scope uuid, p_client uuid) RETURNS boolean
LANGUAGE plpgsql AS $$
DECLARE
  v_owner uuid;
BEGIN
  IF p_client = store_server_client() THEN
    RAISE EXCEPTION 'store: the server client id is reserved' USING ERRCODE = '22023';
  END IF;
  PERFORM store_ensure_scope(p_scope);
  INSERT INTO store_client (client_id, scope_id) VALUES (p_client, p_scope) ON CONFLICT DO NOTHING;
  SELECT scope_id INTO v_owner FROM store_client WHERE client_id = p_client;
  RETURN v_owner = p_scope;
END $$;

-- Removes a scope with its clients and log, after the product deleted its
-- rows. A device that syncs it again starts a new history.
CREATE FUNCTION store_drop_scope(p_scope uuid) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  IF current_setting('store.scope', true) = p_scope::text THEN
    RAISE EXCEPTION 'store: close the open mutation before dropping its scope' USING ERRCODE = '55000';
  END IF;
  DELETE FROM store_scope WHERE scope_id = p_scope;
END $$;

CREATE OR REPLACE FUNCTION store_capture() RETURNS trigger
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
    IF TG_OP = 'UPDATE' AND to_jsonb(OLD) - v_table.excluded_columns = v_row - v_table.excluded_columns THEN
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
