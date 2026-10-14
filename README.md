# store

Offline-first sync for skkil products. A product keeps its own Postgres and
business rules; store adds an ordered change log per scope, a read service
that devices pull from, and a client core that keeps a local SQLite replica
with an outbox of named mutations.

| Module | Language | What it is |
| --- | --- | --- |
| [`modules/protocol`](modules/protocol) | Protobuf | The wire contract (`skkil.store.v1`) |
| [`modules/sql`](modules/sql) | PL/pgSQL | The write path a product applies to its database |
| [`modules/service`](modules/service) | Go | The stateless read service: Pull and Pokes |
| [`modules/core`](modules/core) | Rust | The client core: local replica, outbox, rebase |
| [`modules/client-ts`](modules/client-ts) | TypeScript | The driver for browsers and React Native |
| [`modules/adapter-jvm`](modules/adapter-jvm) | Kotlin | The write path and Push handler kit for Spring servers |
| [`modules/conformance`](modules/conformance) | Go | Simulator and conformance suite over the wire |

Read [`docs/protocol.md`](docs/protocol.md) for the contract,
[`ARCHITECTURE.md`](ARCHITECTURE.md) for the design and
[`docs/decisions`](docs/decisions) for where v1 departs from it.

```bash
skkil doctor    # toolchains
skkil ci        # every module's format, lint, test and build
```

Releases are git tags `vX.Y.Z`; products pin one.
