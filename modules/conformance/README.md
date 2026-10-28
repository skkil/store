# conformance

The store conformance suite and its reference product. The suite drives
simulated clients over the wire against a product server's `Push` and the
store service's `Pull` and `Pokes`, and checks the invariants of
[`ARCHITECTURE.md`](../../ARCHITECTURE.md) §17. A product server is correct
only if it passes (§14, §19). The contract under test is
[`docs/protocol.md`](../../docs/protocol.md).

The module also holds the Go adapter for the write path (`writepath`) and a
tiny todo-list product built on it (`reference`), which the suite is proven
against.

## Run the suite against a product

Build the binary from the store tag your product pins:

```bash
git clone --branch vX.Y.Z <store repo> store && cd store/modules/conformance
go build -o conformance ./cmd/conformance
```

Then, with the product server, its Postgres and a store service running:

```bash
./conformance \
  -push-url http://localhost:8081 \
  -store-url http://localhost:8080 \
  -token-url http://localhost:8081/sync-token \
  -profile conformance-profile.json \
  -database-url postgres://app:app@localhost:5432/app \
  -runs 20
```

It prints one `PASS`/`FAIL` line per scenario and seed, and exits 0 when
every run passed, 1 when any run failed and 2 on bad usage. A failure prints
the broken invariants, the line to reproduce it and the trace of a minimal
failing run:

```text
FAIL duplicates   seed=1 steps=3 30ms
  L2 [c0]: m1 processed twice: APPLIED v1, then REJECTED v2
  reproduce: -seed 1 -runs 1 -scenarios duplicates
  trace:
    scope a194ef7a-…, user conformance-duplicates-1-e331e85e9408, 3 clients
    #1 c0 mutate: m1 todo.create (create) {"title":"tango lima foxtrot"}
    #2 c0 push-lost: response lost: m1 APPLIED v1
    #3 c0 push: m1 REJECTED v2 "todo already exists"
```

### Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-push-url` | required | Product base URL serving `skkil.store.v1.PushService` (Connect, binary protobuf) |
| `-store-url` | required | Store service base URL |
| `-token-url` | | URL that answers `POST {"user", "client_id"}` with credentials (below) |
| `-token-cmd` | | Shell command that prints credentials; gets `$STORE_CONFORMANCE_USER` and `$STORE_CONFORMANCE_CLIENT_ID` |
| `-token-header` | | Extra `Name: value` header for `-token-url`, repeatable |
| `-profile` | required | The product's mutation profile (below) |
| `-seed` | random, printed | First seed; run `i` uses `seed + i` |
| `-runs` | `1` | How many consecutive seeds to run |
| `-scenarios` | all | Comma-separated subset of the scenarios below |
| `-clients` | `3` | Simulated clients per scenario, all in one scope |
| `-steps` | `60` | Steps per scenario, roughly |
| `-page-size` | `50` | `limit` of a client's full pull |
| `-database-url` | | Product Postgres (store schema 4 or later); enables epoch bumps (`store_bump_epoch`) |
| `-poke-timeout` | `10s` | How long a listening client may wait to be poked up to the head |
| `-shrink` | `true` | Re-run a failing scenario with fewer steps to print a minimal trace |
| `-shrink-runs` | `40` | Most re-runs while shrinking |
| `-v` | `false` | Print the trace of passing runs too |

Set exactly one of `-token-url` and `-token-cmd`.

### Credentials

Each scenario uses a fresh user name; every client of that user must get a
token for the same scope. The suite asks once per client (and again shortly
before `exp`):

- `-token-url`: `POST` with `{"user": "<name>", "client_id": "<uuid>"}`.
- `-token-cmd`: `sh -c '<command>'` with `STORE_CONFORMANCE_USER` and
  `STORE_CONFORMANCE_CLIENT_ID` set.

The answer (response body or standard output) is either a bare sync token or

```json
{ "token": "<sync token>", "push_headers": { "Authorization": "Bearer <user session>" } }
```

The token goes to `Pull` and `Pokes`; its `sub` is the scope and its `cid`
must equal the client id. `push_headers` go on every `Push`; without them the
suite sends `Authorization: Bearer <token>`. A product whose Push uses its own
sign-in returns that sign-in in `push_headers`.

### Mutation profile

A JSON file describing the product's mutations as templates, so the suite can
drive any product:

```json
{
  "schema_version": 1,
  "mutations": [
    { "name": "todo.create", "kind": "create", "weight": 4, "args": { "title": "{{text}}" }, "creates": ["todo"] },
    { "name": "todo.rename", "kind": "update", "weight": 4, "args": { "id": "{{row:todo}}", "title": "{{text}}" } },
    { "name": "todo.delete", "kind": "delete", "args": { "id": "{{row:todo}}" } },
    { "name": "todo.create", "kind": "reject", "args": { "title": "" } }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `schema_version` | Sent as `PushRequest.schema_version` |
| `name` | The mutation name pushed |
| `kind` | `create`, `update` or `delete` (may be applied or rejected) or `reject` (must always be rejected) |
| `weight` | How often it is picked relative to the others; `0` leaves it out; default `1` |
| `args` | A JSON object; strings may hold placeholders |
| `creates` | Tables of the rows it creates with derived ids, in order: entry `n` gets derived id `n`. When it applies, its version must upsert each of them |

Placeholders, filled per pushed mutation:

| Placeholder | Value |
| --- | --- |
| `{{new_id}}`, `{{new_id:N}}` | The `N`-th id (from 0) derived from the mutation (`docs/protocol.md`, Ids) |
| `{{row:TABLE}}` | The id of a row of `TABLE` (the wire table name) the client has pulled; the mutation is skipped while there is none |
| `{{text}}` | A few random words, including non-ASCII and JSON-escaped characters |
| `{{int}}`, `{{bool}}` | A random number or boolean (a JSON number or boolean when it is the whole string) |
| `{{client_id}}`, `{{mutation_id}}` | The client id and the mutation id |

A profile needs at least one non-`reject` mutation without `{{row:…}}` and at
least one `reject` mutation. The reference product's profile is
[`reference/profile.json`](reference/profile.json).

## Scenarios

Each scenario runs in a fresh scope with `-clients` clients, then every client
reconnects, pushes and pulls until it has the head, a fresh auditor pulls the
whole log, and the checks run.

| Scenario | What it does |
| --- | --- |
| `offline` | A client makes mutations offline while others write, then reconnects and pushes |
| `concurrent` | Every client pushes at the same time, then pulls at the same time |
| `duplicates` | Push responses are lost, so the same mutations are pushed again (also concurrently) |
| `rejections` | Mutations of kind `reject` among ordinary ones |
| `poke-loss` | Half the clients ignore pokes and rely on their own pulls; the others pull when poked |
| `small-pulls` | Pulls with `limit` 1–3 interleaved with pushes |
| `reconnect` | Offline/online cycles, a cursor with the wrong hash, a cursor past the head, and epoch bumps (with `-database-url`) |
| `random` | A seeded mix of every step |

Each simulated client is a pending queue, a cursor and an in-memory replica
(`suite/sim`). It acknowledges pending mutations by `last_mutation_id`, sends
`min_version` after a push, applies pulled versions and handles `Reset` by
dropping its rows and keeping its pending mutations.

A seed fixes every scenario's steps and every mutation's arguments, and row
picks follow pull order rather than id values, so a run is reproducible step
for step. Concurrent steps and poke timing still depend on the servers.

## Checks

| ID | Check |
| --- | --- |
| `L1` | Pulled versions follow the cursor one by one with no gap; `last_mutation_id` never goes back; after convergence the log holds `1..head` and each client's mutations in order |
| `L2` | Each `(client, mutation)` is answered `APPLIED`/`REJECTED` at most once and recorded once in the log at the answered version; a re-push answers `DUPLICATE` (with no version, the recorded one or a later head) and adds no version |
| `L3` | Every delivery of a version (to any client, any page size, before and after a `Reset`) is identical; a rejection carries no changes |
| `L4` | After convergence every client's replica equals a fresh pull of the whole log |
| `S2` | A `REJECTED` answer is a rejection entry in the log with the same reason, which every client pulls; a `reject` mutation is never applied |
| `HASH` | Every version's hash equals the chain recomputed from the pulled versions, as `store_close` computes it |
| `RESET` | A stale cursor (wrong hash, past the head, older epoch) gets `Reset`, to the new epoch after a bump; a matching cursor never does |
| `ID` | An applied mutation upserts the rows listed in `creates` under its derived ids |
| `POKE` | A client holding a `Pokes` stream is poked up to the head within `-poke-timeout` |
| `LIVE` | Every mutation is acknowledged and every client reaches the head after convergence |
| `PROTO` | One result per pushed mutation in order, statuses set, versions on `APPLIED`/`REJECTED`, `limit` respected |

## In a product's CI

A product (for example `time`'s Kotlin server) runs the suite against itself
on every change:

1. Start Postgres and apply `modules/sql/migrations` and the product schema.
2. Start the product server with a test-only way to get credentials: a
   token endpoint that signs in a throwaway user (`-token-url`), or a script
   that signs one in with the mock OIDC issuer, calls the product's
   sync-token endpoint and prints `{"token", "push_headers"}` (`-token-cmd`).
3. Start the store service (`docker build -t skkil-store modules/service`)
   with the product's token issuer and public key.
4. Run the suite with the product's profile and `-database-url`, and fail the
   build on a non-zero exit.

```bash
docker run -d --name store --network host \
  -e STORE_DATABASE_URL=postgres://app:app@localhost:5432/app \
  -e STORE_TOKEN_ISSUER=time \
  -e STORE_TOKEN_JWKS_URL=http://localhost:8081/sync/jwks.json \
  skkil-store
./conformance -push-url http://localhost:8081 -store-url http://localhost:8080 \
  -token-cmd ./scripts/conformance-token.sh -profile conformance-profile.json \
  -database-url postgres://app:app@localhost:5432/app -runs 10
```

## The reference product

`cmd/reference` serves a todo list over the Go adapter: `todo.create {title,
items?}` (ids derived from the mutation; an empty item is rejected after the
todo was written, so only the savepoint keeps the rejection whole),
`todo.rename`, `todo.complete`, `todo.delete` (items go by `ON DELETE
CASCADE`) and `item.create`. Push authenticates with the sync tokens it issues
at `POST /sync-token` (`{"user", "client_id"}`, one scope per user); the
verification key is at `GET /sync-token/public-key`.

```bash
(cd ../service && docker compose up -d --wait)        # Postgres with the store migrations
go run ./cmd/reference -database-url postgres://store:store@localhost:55433/store \
  -migrate -public-key-out /tmp/reference.pem
STORE_DATABASE_URL=postgres://store:store@localhost:55433/store STORE_TOKEN_ISSUER=reference \
  STORE_TOKEN_PUBLIC_KEY_PEM="$(cat /tmp/reference.pem)" go run ../service/cmd/store
go run ./cmd/conformance -push-url http://localhost:8081 -store-url http://localhost:8080 \
  -token-url http://localhost:8081/sync-token -profile reference/profile.json \
  -database-url postgres://store:store@localhost:55433/store
```

`-fault` breaks it on purpose, to prove the suite has teeth:
`ignore-duplicates` (idempotency off; breaks `L2`), `sequence-versions`
(versions from one sequence shared by every scope, drawn before the scope
lock; breaks `L1`) and `drop-rejections` (a rejection rolls back the whole
transaction; breaks `S2`).

## The Go adapter

```go
w := writepath.New(pool, logger)
result, err := w.Apply(ctx, scope, client, mutationID, func(ctx context.Context, m *writepath.Mutation) error {
    if title == "" {
        return writepath.Reject("title must not be empty")
    }
    _, err := m.Tx.Exec(ctx, "INSERT INTO todo (id, scope_id, title) VALUES ($1, $2, $3)", m.NextID(), m.Scope, title)
    return err
})
```

`Apply` runs `store_begin`, a savepoint, the body, then `store_end`, or on a
`Rejection` a rollback to the savepoint and `store_reject`, commits, and calls
`store_poke` after the commit. A duplicate answers the recorded version. Any
other error aborts the transaction, so the client retries the batch.
`ApplyServer` records a write the product makes on its own. `DeriveID` matches
`derive_id` in `store-core`. The single statements (`Begin`, `End`,
`EndIfChanged`, `Abandon`, `IsOpen`, `RecordRejection`, `ClaimClient`,
`DropScope`, `CaptureExisting`, `BumpEpoch`, …) are exported for products that manage transactions
themselves. The reference token endpoint claims the client first and answers
409 for a client id another user owns.

## Develop

```bash
go test -race ./...                  # needs Docker (Testcontainers, postgres:17.11-alpine) and builds ../service
CONFORMANCE_SEEDS=50 go test ./test/... -run TestReferenceConforms
go vet ./... && go tool staticcheck ./...
gofmt -l .
buf generate                         # regenerate gen/ from ../protocol/proto
```

The tests start one Postgres container per test binary, build `cmd/store`
from `modules/service` and run it as a subprocess, and serve the reference
product in-process. `test/conformance` runs every scenario for 12 seeds
(`CONFORMANCE_SEEDS`, 2 with `-short`) and runs each fault, which must fail.

## Layout

```text
cmd/conformance/      the suite binary: flags, exit codes
cmd/reference/        the reference product server binary
gen/storev1/          generated protobuf and Connect code (committed)
writepath/            the Go write-path adapter: Writer, statements, DeriveID
reference/            the reference product: schema, todo mutations, Push, sync tokens, faults, profile.json
suite/                Config, Run, Report, scenario execution, convergence, shrinking
  check/              invariants, the Ledger of pushes and deliveries, the final audit
  credentials/        token sources: URL, command, cache
  hashchain/          the log's hash chain recomputed from pulled versions
  profile/            the mutation profile and its templates
  scenario/           seeded step scripts
  sim/                the simulated client: pending queue, cursor, replica, pokes
internal/stacktest/   test-only: Postgres, the store service subprocess, in-process products
test/conformance/     the suite against the reference, and against each fault
```
