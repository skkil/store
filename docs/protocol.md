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

## Read path (store service)

- `Pull(cursor)` returns the versions after `cursor.version` in order, each
  with every change of that version or its rejection, plus the client's
  `last_mutation_id` (the ack for its pending mutations).
- `Reset` is returned when the cursor's epoch differs, its hash does not match
  the log, or it is ahead of the head. The client then drops synced rows, pulls
  from version 0 and keeps its pending mutations.
- Anything that rewinds a scope's log (a restore, a failover that loses
  commits, a shard move) must bump `store_scope.epoch`. That is what turns a
  stale cursor into `Reset`; without it, a client whose `min_version` is above
  the rewound head would wait forever.
- `Pokes` is a server stream of `{version}` hints for the token's scope. Pokes
  may be lost; clients also pull on start, on reconnect, after pushing and on
  a timer.

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
excluded columns. Clients upsert the columns they know and ignore the rest, so
server-only columns never break a client.

## Ids

Rows created on a device need ids that never collide and that the server
reproduces when it replays the mutation. Products derive them from the
mutation: `uuidv8(sha256(client_id ‖ mutation_id ‖ n))` for the n-th id a
mutation creates (`store-core` provides `derive_id`). Server-originated writes
use random UUIDv7. If a server replay ever creates ids in a different order,
the device's optimistic rows are replaced by the server's on the next pull, so
a mismatch is self-healing.

## Versioning

- Wire: `protocol_version` in every request. A server answers a version it
  does not support with `failed_precondition`.
- SQL: `store_schema.version`. New versions ship as additional migrations;
  shipped migrations never change.
- Breaking wire changes are checked with `buf breaking` against the last tag.
