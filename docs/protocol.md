# store protocol v1

The contract between a product and store. Anything not written here is an
implementation detail of one side and may change without notice.

## Pieces

| Piece | Owner | Contract |
| --- | --- | --- |
| Wire messages | `modules/protocol` | `skkil.store.v1` protobuf; `protocol_version = 1` in every request |
| Write path | `modules/sql` | `store_*` tables and functions, versioned in `store_schema` |
| Pull and Pokes | `modules/service` | Connect RPC `skkil.store.v1.StoreService`, binary protobuf or JSON |
| Push | each product server | Connect RPC `skkil.store.v1.PushService`, same messages |
| Sync token | each product server | ES256 JWT described below |

## Scopes, clients and mutations

- A **scope** (`uuid`) is the unit of order: one user's personal data in v1.
  Every synced row belongs to exactly one scope.
- A **client** (`uuid`, made once per install) belongs to one scope for life;
  signing in to another account makes a new client id.
- A **mutation** is a named, product-defined command with JSON arguments.
  `mutation_id` starts at 1 per client and only grows. The server applies each
  `(client_id, mutation_id)` at most once.

## Write path (product server)

For each pushed mutation, in one transaction:

```sql
BEGIN;
SELECT store_begin(:scope, :client, :mutation_id);   -- 'duplicate' → COMMIT, report DUPLICATE
SAVEPOINT store_mutation;
-- run the mutation: ordinary SQL on registered tables
SELECT store_end();                                    -- returns the new version → APPLIED
-- on a business-rule failure instead:
ROLLBACK TO SAVEPOINT store_mutation;
SELECT store_reject(:reason);                          -- → REJECTED
COMMIT;
SELECT store_poke(:scope, :version);                   -- after COMMIT, outside the transaction
```

- Writes the product makes on its own (REST endpoints, jobs) use
  `store_begin_server(:scope)` instead of `store_begin`, then `store_end()`.
- Products call only the functions below; `store_*` tables and the `store.*`
  settings are internal and may change in any release.

| Function (schema) | Use |
| --- | --- |
| `store_end_if_changed() → bigint` (3) | Closes an open server write: like `store_end()` when it captured a row change, otherwise closes without a version and returns `NULL`. |
| `store_abandon()` (3) | Closes an open server write without a version. It must have captured no row change (roll back to a savepoint first). |
| `store_is_open() → boolean` (3) | Whether a mutation or server write is open on this transaction, so nested writes can join it. |
| `store_claim_client(:scope, :client) → boolean` (3) | Binds a client to a scope for life, as its first `store_begin` would. `false` when it already belongs to another scope; call it before issuing a sync token. |
| `store_drop_scope(:scope)` (3) | Removes a scope with its clients and log after the product deleted its rows (account deletion). A device that syncs it again starts over. |
| `store_capture_existing(:table, :column, :values) → integer` (4) | Inside an open mutation, logs an `upsert` for every row of a registered table whose `:column` is one of `:values` (every row when both are `NULL`), in id order, and returns how many. See "Baselining a scope". |
| `store_bump_epoch(:scope) → integer` (4) | Moves the scope to its next epoch so every older cursor gets `Reset`; returns it, or `NULL` when the scope has no log yet. |

- A device mutation always ends with `store_end()` or `store_reject()`, so its
  client is acked: `store_abandon()` and `store_end_if_changed()` on one, or
  `store_abandon()` after captured changes, raise `22023`. `store_drop_scope`
  on the scope of the open mutation raises `55000`; claiming the server client
  id raises `22023`.
- `store_register(table, id_column, excluded_columns)` puts a table under sync.
  From then on any write to it outside an open mutation raises
  `55000`, unless the transaction sets `store.bypass = on` (migrations only).
- Deletes are logged as `delete` changes, including rows removed by
  `ON DELETE CASCADE`. Tombstone columns are optional.
- `MutationResult.version` for `DUPLICATE` may be the scope's current head
  rather than the version that first recorded the mutation.
- Synced tables must not use deferred constraints: a failure at `COMMIT` can't
  be turned into a rejection, so it would block that client's queue.
- Push answers 2xx with one `MutationResult` per mutation. Transport errors
  mean "try the whole batch again"; they never mean "rejected".

## Baselining a scope

A device that pulls from version 0 receives only what the log holds. Rows the
log never captured are invisible to it: rows written before their table was
registered (a product adopting store with existing data) and rows rewritten
under `store.bypass` (a data migration). To baseline a scope, the product runs
one server write that captures them:

```sql
BEGIN;
SELECT store_begin_server(:scope);
SELECT store_capture_existing('project', 'user_id', ARRAY[:user]);               -- parents first
SELECT store_capture_existing('task', 'user_id', ARRAY[:user]);
SELECT store_capture_existing('task_note', 'task_id', ARRAY(SELECT id::text FROM task WHERE user_id = :user));
SELECT store_end();
COMMIT;
SELECT store_poke(:scope, :version);
```

- Each captured row becomes an ordinary `upsert` with the image the capture
  trigger would log (excluded columns removed), so clients apply it like any
  other change and the hash chain covers it. Rows come in id order, so the
  same rows always hash the same.
- `:values` are bound as data and cast to the column's type, and `:column`
  must name a column of the table; neither is ever run as SQL. Tables without
  the scope column pass the ids of their parent rows, as above.
- Capture parents before children, so a client that applies changes in order
  with foreign keys on never sees an orphan.
- A baseline duplicates rows the log already holds; that is harmless, since
  upserts are idempotent. It never deletes: rows removed outside the log stay
  on devices until a delete for them is logged.
- `store_capture_existing` on a table that is not registered, an unknown
  column, or only one of `:column` and `:values` raises `22023`; outside an
  open mutation it raises `55000`.
- A restore replaces rows through the write path (so its deletes are logged),
  then calls `store_bump_epoch(:scope)` and baselines in the same
  transaction, so every device resets onto the restored rows.

## Read path (store service)

- `Pull(cursor)` returns the versions after `cursor.version` in order, each
  with every change of that version or its rejection, plus the client's
  `last_mutation_id` (the ack for its pending mutations).
- `Reset` is returned when the cursor's epoch differs, its hash does not match
  the log, or it is ahead of the head. The client then drops synced rows, pulls
  from version 0 and keeps its pending mutations.
- Anything that rewinds a scope's log (a restore, a failover that loses
  commits, a shard move) must call `store_bump_epoch(:scope)`. That is what
  turns a stale cursor into `Reset`; without it, a client whose `min_version`
  is above the rewound head would wait forever.
- `Pokes` is a server stream of `{version}` hints for the token's scope. Pokes
  may be lost; clients also pull on start, on reconnect, after pushing and on
  a timer.

## Hash chain

Each version's `hash` chains every earlier one, so a cursor names one exact
history. With `‖` for byte concatenation and SHA-256 throughout:

```
body    = sha256(join("\n", seq "|" table "|" row_id "|" op "|" data))   -- commit, seq from 1, data = jsonb::text or ""
        = sha256(reason)                                               -- rejection
hash(v) = sha256(hash(v-1) ‖ utf8(v "|" client_id "|" mutation_id "|" kind) ‖ body)
```

`hash(0)` is empty. `data` is Postgres `jsonb::text` byte for byte, so only
readers of the raw log (the service, the conformance suite) can recompute it;
clients store the hash and send it back.

## Sync token

The store service never sees the product's users. The product server issues
a short-lived token that the store service verifies:

| Claim | Value |
| --- | --- |
| `iss` | the product, e.g. `time` |
| `aud` | `skkil-store` |
| `sub` | the scope id |
| `cid` | the client id |
| `exp` | at most 15 minutes after `iat` |

Signed with ES256. The store service is configured with the product's public
key (PEM) or JWKS URL and rejects any other issuer, audience or expired token.
Clients send it as `Authorization: Bearer <token>` to `Pull` and `Pokes`.

## Row images

A change's `data` is the row as JSON (`to_jsonb`), minus the registered
excluded columns. An `UPDATE` whose image minus those columns is unchanged is
not logged (schema 3; before it, only fully identical rows were skipped). Clients upsert the columns they know and ignore the rest, so
server-only columns never break a client.

## Ids

Rows created on a device need ids that never collide and that the server
reproduces when it replays the mutation. Products derive them from the
mutation: `uuidv8(sha256(client_id ‖ mutation_id ‖ n))` for the n-th id a
mutation creates (`store-core` provides `derive_id`). The input is the 16 raw
bytes of `client_id`, `mutation_id` as a big-endian u64 and `n` as a
big-endian u32 counting from 0; the first 16 digest bytes become the UUID with
version 8 and the RFC 9562 variant. Test vector: client
`01923e4c-8b1a-7c3d-9e2f-0a1b2c3d4e5f`, mutation 1, n 0 →
`7c95d028-8ca6-8ddf-b6a6-888317f17cb1`. Server-originated writes
use random UUIDv7. If a server replay ever creates ids in a different order,
the device's optimistic rows are replaced by the server's on the next pull, so
a mismatch is self-healing.

## Clock

`Mutation.hlc` is a hybrid logical clock encoded as
`(unix_millis << 16) | counter`, with the counter in `0..=0xFFFF`. The client
takes it when the mutation is made and never changes it. A mutation's wall
time is `hlc >> 16`: mutators read it from the mutation context (`now()` in
`store-core`, the decoded `hlc` on the server) instead of the system clock, so
a mutation made offline at 10:00 and replayed or pushed at 12:00 still records
10:00. Order still comes from the server's versions; the HLC only decides
fields declared last-writer-wins.

## Versioning

- Wire: `protocol_version` in every request. A server answers a version it
  does not support with `failed_precondition`.
- SQL: `store_schema.version`. New versions ship as additional migrations;
  shipped migrations never change.
- Breaking wire changes are checked with `buf breaking` against the last tag.
