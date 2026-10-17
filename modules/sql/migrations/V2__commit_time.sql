-- Stamp the scope with the commit-time clock rather than the transaction's
-- start, so the store service's polling fallback sees long transactions.

UPDATE store_schema SET version = 2;

CREATE OR REPLACE FUNCTION store_close(p_kind text, p_reason text) RETURNS bigint
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
     SET head_version = v_version, head_hash = v_hash, updated_at = clock_timestamp()
   WHERE scope_id = v_scope;
  IF v_client <> store_server_client() THEN
    UPDATE store_client SET last_mutation_id = v_mutation WHERE client_id = v_client;
  END IF;
  PERFORM set_config('store.scope', '', true);
  RETURN v_version;
END $$;
