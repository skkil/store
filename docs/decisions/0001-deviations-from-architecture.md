# 0001: Where v1 deviates from ARCHITECTURE.md

Status: accepted, 2026-11-08

1. **Deletes are logged, tombstones are optional.** The log is ordered by the
   server and every change goes through it, so a `delete` change cannot be
   resurrected by a stale write: the stale mutation is rejected instead.
   Capturing deletes (including cascades) means products keep their foreign
   keys and hard deletes.
2. **The client core is sans-IO.** `store-core` builds requests and applies
   responses but never opens a socket. Hosts drive it: a Rust HTTP driver on
   desktop, the TypeScript driver (`modules/client-ts`) in browsers and React
   Native. One core, three hosts, no networking code in WASM or FFI.
3. **Local rebase uses recorded pre-images.** Optimistic writes record each
   touched row's previous image. A pull undoes pending mutations, applies the
   server versions, then replays what is still pending through the product's
   mutator. This is exact for any mutation, idempotent or not.
4. **Mutation ids derive deterministic row ids** (`docs/protocol.md`, Ids)
   instead of shipping a temp-id map.
5. **Pull is authorised by a product-signed sync token** rather than by store
   knowing each product's identity provider.
6. **Snapshots and compaction are designed, not built.** `retained_from` stays
   0 in v1, so new devices replay the log from version 0.
7. **The poke bus is Postgres `LISTEN/NOTIFY` fed after commit plus a polling
   fallback**, behind a `PokeBus` interface for NATS later.
