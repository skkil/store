# core

The client core of skkil store, as a Rust workspace. `store-core` keeps a
product's local SQLite tables in sync with its server: it records pre-images
of rows touched by local mutations, queues the mutations, builds
`PushRequest`/`PullRequest` messages, and applies the responses by undoing
pending mutations, applying the server's versions and replaying what is still
pending. It never opens a socket (sans-IO), so the same crate runs on desktop,
in the browser (`wasm32-unknown-unknown`) and on iOS and Android.

| Crate | Package | What it is |
| --- | --- | --- |
| `crates/store-protocol` | `skkil-store-protocol` | prost types for `skkil.store.v1`, generated and committed |
| `crates/store-core` | `skkil-store-core` | The sans-IO engine |
| `crates/store-http` | `skkil-store-http` | Blocking HTTP driver for native hosts (empty on wasm) |
| `crates/store-testkit` | `skkil-store-testkit` | Fake server and sample planner app for tests (unpublished) |
| `tools/protogen` | `protogen` | Regenerates `store-protocol` from `modules/protocol/proto` |

## Commands

```bash
cargo fmt --check
cargo clippy --workspace --all-targets -- -D warnings
cargo test --workspace                     # unit, integration and property tests
cargo run -p protogen                      # regenerate the protobuf code (needs protoc)

cargo build --target wasm32-unknown-unknown -p skkil-store-core
cargo test --target wasm32-unknown-unknown -p skkil-store-core --test wasm   # runs in Node
cargo ndk -t arm64-v8a -t x86_64 build -p skkil-store-core
cargo check --target aarch64-apple-ios -p skkil-store-core --no-default-features
```

Toolchain for the cross builds: `rustup target add wasm32-unknown-unknown
aarch64-linux-android x86_64-linux-android aarch64-apple-ios
aarch64-apple-ios-sim`, `clang` with the wasm32 target (it compiles SQLite for
wasm), `wasm-bindgen-cli` at the version in `Cargo.lock` for the wasm test,
`cargo install cargo-ndk` with an Android NDK under `$ANDROID_HOME/ndk` or
`$ANDROID_NDK_HOME`. On Linux iOS can only be checked without `bundled`
because compiling SQLite for iOS needs Xcode.

## Code layout

```
crates/store-core/src/
  lib.rs            public API
  error.rs          Error, Result
  clock/            Hlc; wall time read through SQLite
  mutation/         Mutator, MutationContext, MutationError, mutate, derive_id, replay
  storage/          local schema (migrations/), store_meta, table registration,
                    capture triggers, row upserts, value coercion, undo
  sync/             push and pull requests/responses, protobuf byte wrappers
  session/          status, issues, bind_scope, forget
crates/store-core/tests/   integration tests per area, properties.rs, wasm.rs
crates/store-http/src/     transport (Connect unary + streaming), pokes, syncer
```

## Local tables

`install` creates, in the product's database:

| Table | Holds |
| --- | --- |
| `store_meta` | One row: schema version, client id, scope, cursor (epoch, version, hash), acked / issued / pushed mutation ids, `min_version`, last HLC, last pull time |
| `store_table` | Registered tables and their id columns |
| `store_pending` | Mutations not yet acknowledged: id, name, JSON args, HLC, created time |
| `store_undo` | First pre-image per (mutation, table, row): NULL for inserts, the old row as JSON otherwise |
| `store_issue` | This client's rejected mutations, with the reason and a dismissed flag |

## API walkthrough

```rust
use store_core::{
    MutationContext, MutationError, Mutator, rusqlite::{Connection, Transaction},
    serde_json::{json, Value},
};

struct Planner;

impl Mutator for Planner {
    fn apply(
        &self,
        tx: &Transaction<'_>,
        ctx: &mut MutationContext,
        name: &str,
        args: &Value,
    ) -> Result<Value, MutationError> {
        match name {
            "task.create" => {
                let id = ctx.next_id().to_string();
                tx.execute(
                    "INSERT INTO task (id, title, created_at) VALUES (?1, ?2, ?3)",
                    (&id, args["title"].as_str(), ctx.now()),
                )?;
                Ok(json!({ "id": id }))
            }
            other => Err(MutationError::Unknown(other.into())),
        }
    }
}

let mut conn = Connection::open("time.db")?;
store_core::install(&conn)?;
store_core::register_table(&conn, "task", "id")?;
store_core::bind_scope(&conn, scope_id)?;

let created = store_core::mutate(&mut conn, &Planner, "task.create", &json!({"title": "Write"}))?;

if let Some(request) = store_core::push_request(&conn, 100)? {
    let response = /* POST to the product server */;
    store_core::apply_push_response(&conn, &response)?;
}
let request = store_core::pull_request(&conn, 500)?;
let response = /* POST to the store service */;
let outcome = store_core::apply_pull_response(&mut conn, &Planner, &response)?;
// refresh the UI for outcome.changed_tables; pull again while outcome.more
```

- **`install`** is idempotent and versioned (`store_meta.schema_version`); call
  it on every start.
- **`register_table`** installs `AFTER INSERT/UPDATE/DELETE` triggers that
  record pre-images only while a mutation is captured. Re-register after every
  migration that changes a synced table; it rebuilds the triggers from the
  current columns. The id column must be the primary key or uniquely indexed.
- **`mutate`** runs the mutator in one transaction with capture on and queues
  the mutation with the next id and an HLC. On a mutator error nothing is
  queued and `Error::Mutation` is returned. The returned `MutationOutcome`
  carries the mutator's value.
- **`push_request` / `apply_push_response`**: pending mutations in order,
  `protocol_version` 1. Answered mutations are not re-sent, and the highest
  recorded version becomes the next pull's `min_version`. Mutations stay
  pending until a pull acknowledges them.
- **`pull_request` / `apply_pull_response`**: in one transaction, undo pending
  mutations from their pre-images, apply each version (only the columns the
  local table has; JSON booleans become 0/1, objects and arrays JSON text;
  unregistered tables are ignored), record rejections of this client's
  mutations as issues, advance the cursor, drop acknowledged mutations and
  replay the rest. A replay that fails stays pending without effects. While
  `more` is set, only mutations whose versions were seen are dropped. On
  `Reset` every registered table is emptied, the cursor restarts at version 0
  and pending mutations are replayed and pushed again.
- **`status` / `status_json`**, **`issues`**, **`dismiss_issue`** for the UI.
  `SyncStatus`, `PullOutcome` and `PushOutcome` serialise with camelCase names
  (`clientId`, `pending`, `issues`, `cursorVersion`, `changedTables`, …).
- **`bind_scope`** fails if the store is bound to another scope; **`forget`**
  clears synced rows and bookkeeping for sign-out and makes a new client id.
- **`*_bytes`** variants take and return encoded protobuf for bindings.

Mutators must be deterministic: ids from `ctx.next_id()` (`derive_id`, the
same ids the server derives), time from `ctx.now()` (the mutation's HLC wall
time, stable across replays), nothing from random generators or the system
clock. `ctx.is_replay()` tells a replay from the first run.

## Embedding in a product crate

A product such as `time` keeps one crate (`time-core`) that owns a single
`rusqlite::Connection` with its app tables, and compiles it for Tauri, the
browser and React Native.

1. **One connection.** Open it, run the product's migrations, then
   `store_core::install` and `register_table` for every synced table. Use
   `store_core::rusqlite` (re-exported) so the types match. Writes to synced
   tables outside `mutate` are local only and are overwritten by the next
   pull that touches the row.
2. **Implement `Mutator`** with the same names and rules as the server's
   mutation registry (`modules/adapter-jvm`). Prefer rejecting in the mutator
   over relying on constraints; foreign keys are checked immediately during
   replay, so a broken one leaves that mutation out.
3. **Drive it.** On desktop, use `skkil-store-http`:

   ```rust
   let transport = HttpTransport::new(
       Endpoints::new("https://api.example.com", "https://store.example.com"),
       credentials, // impl Credentials: product token for Push, sync token for Pull/Pokes
   );
   let report = Syncer::new(&transport).sync(&mut conn, &Planner)?;
   for poke in Syncer::new(&transport).pokes(&conn)? { /* sync when ahead */ }
   ```

   In browsers and React Native, expose the `*_bytes` functions and
   `status_json` through wasm-bindgen or uniffi and let
   `modules/client-ts` move the bytes.

### Feature flags

| Target | `skkil-store-core` | Extra dependencies of the product crate |
| --- | --- | --- |
| Desktop, Android | default (`bundled`: SQLite compiled in) | `skkil-store-http` under `[target.'cfg(not(target_arch = "wasm32"))'.dependencies]` |
| iOS | `bundled` when building on macOS, or `default-features = false` to link the system `libsqlite3` | — |
| Browser (`wasm32-unknown-unknown`) | either; `bundled` has no effect there, because rusqlite's default `ffi-sqlite-wasm-rs` swaps in `sqlite-wasm-rs` | `sqlite-wasm-rs = { version = "0.6", features = ["wasm-bindgen"] }` for the JS host glue, and `sqlite-wasm-vfs = { version = "0.3", features = ["sahpool"] }` to keep the database on OPFS; install the SAH-pool VFS in a dedicated worker before opening the connection |

`skkil-store-http` compiles to an empty crate on wasm, so listing it
unconditionally also works. SQLite on wasm is single-threaded
(`SQLITE_THREADSAFE=0`); keep the connection in one worker and let one tab
own it (the TypeScript driver's tab lock).
