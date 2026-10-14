# AGENTS.md

store is the sync engine for skkil products (`time`, `clip`). The design is
`ARCHITECTURE.md`; the contract products rely on is `docs/protocol.md`;
deviations are in `docs/decisions/`. The open work list is `BACKLOG.md`.

## Layout

All code lives in `modules/`; the root holds docs, `BACKLOG.md` and
`skkil.yml` only. Each module is self-contained with its own toolchain and
README:

- `protocol/`: `skkil.store.v1` protobuf, linted and breaking-checked with buf.
  Generated code is committed in the modules that use it (`make generate` or
  the module's generate script).
- `sql/`: `migrations/V<n>__*.sql`. Never edit a shipped migration; add one
  and bump `store_schema`.
- `service/`: Go, `cmd/store` plus `internal/<area>` packages (config, auth,
  pull, poke, bus, server). Tests use real Postgres via Testcontainers.
- `core/`: Rust workspace. `store-core` is sans-IO and compiles for native,
  `wasm32-unknown-unknown`, iOS and Android.
- `client-ts/`: TypeScript driver for any core handle (WASM or React Native).
- `adapter-jvm/`: Kotlin library for Spring servers.
- `conformance/`: Go simulator and suite run against any product server.

## Rules

- The single-transaction rule: a mutation's row changes, the scope's version
  and the idempotency record commit together. Nothing else decides order.
- Push is a poke, never the data. Every downstream piece (service instances,
  bus, caches) may be stale or lossy; clients recover by pulling by cursor.
- A change to `docs/protocol.md` is a protocol change: bump the version or
  keep it backward compatible, and say so in the release notes.
- Products depend only on tagged releases and on what `docs/protocol.md`
  promises.
- No code comments beyond doc comments on public items; code reads on its own.
- Commit per landed feature, conventional commit messages.
