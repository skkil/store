-- Baselining rows the log never captured: rows that predate a table's
-- registration or were rewritten under store.bypass. Bumping a scope's epoch
-- through a function instead of updating store_scope.

UPDATE store_schema SET version = 4;

-- Appends an upsert patch to the open mutation for every row of the
-- registered table p_table whose p_column equals one of p_values (every row
-- when both are NULL), in id order, with the image the capture trigger would
-- log. Returns how many rows it captured. p_column is an identifier and
-- p_values are bound parameters cast to the column's type, so neither is
-- ever interpolated as SQL.
CREATE FUNCTION store_capture_existing(p_table regclass, p_column text DEFAULT NULL, p_values text[] DEFAULT NULL)
RETURNS integer LANGUAGE plpgsql AS $$
DECLARE
  v_scope uuid := store_require_open();
  v_table store_table;
  v_type text;
  v_filter text := '';
  v_seq integer := current_setting('store.seq')::integer;
  v_count integer;
BEGIN
  SELECT * INTO v_table FROM store_table WHERE table_name = (SELECT relname FROM pg_class WHERE oid = p_table);
  IF NOT FOUND THEN
    RAISE EXCEPTION 'store: % is not registered', p_table USING ERRCODE = '22023';
  END IF;
  IF (p_column IS NULL) <> (p_values IS NULL) THEN
    RAISE EXCEPTION 'store: pass a column with its values, or neither' USING ERRCODE = '22023';
  END IF;
  IF p_column IS NOT NULL THEN
    SELECT format_type(atttypid, atttypmod) INTO v_type
      FROM pg_attribute
     WHERE attrelid = p_table AND attname = p_column AND attnum > 0 AND NOT attisdropped;
    IF v_type IS NULL THEN
      RAISE EXCEPTION 'store: % has no column %', p_table, p_column USING ERRCODE = '22023';
    END IF;
    v_filter := format('WHERE t.%I = ANY ($1::%s[])', p_column, v_type);
  END IF;
  EXECUTE format(
    'INSERT INTO store_log (scope_id, version, seq, kind, table_name, row_id, op, data, client_id, mutation_id)
     SELECT $2, $3, $4 + row_number() OVER (ORDER BY s.id), ''patch'', $5, s.image ->> $6, ''upsert'', s.image - $7, $8, $9
       FROM (SELECT t.%I AS id, to_jsonb(t) AS image FROM %s t %s) s',
    v_table.id_column, p_table, v_filter)
  USING p_values,
        v_scope,
        current_setting('store.version')::bigint,
        v_seq,
        v_table.table_name,
        v_table.id_column,
        v_table.excluded_columns,
        current_setting('store.client')::uuid,
        current_setting('store.mutation')::bigint;
  GET DIAGNOSTICS v_count = ROW_COUNT;
  PERFORM set_config('store.seq', (v_seq + v_count)::text, true);
  RETURN v_count;
END $$;

-- Moves the scope to a new epoch, so every cursor from before Resets. Call it
-- for anything that rewinds or rewrites the scope's history (a restore, a
-- failover that lost commits, a shard move). Returns the new epoch, or NULL
-- when the scope has no log yet.
CREATE FUNCTION store_bump_epoch(p_scope uuid) RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
  v_epoch integer;
BEGIN
  UPDATE store_scope
     SET epoch = epoch + 1, updated_at = clock_timestamp()
   WHERE scope_id = p_scope
  RETURNING epoch INTO v_epoch;
  RETURN v_epoch;
END $$;
