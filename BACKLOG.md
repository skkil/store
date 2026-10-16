# Backlog

v1: everything `time` needs on desktop, web and mobile. Check items off as
they land.

## 1. Foundation
- [x] 1.1 Repo layout, docs, decisions, `skkil.yml`
- [x] 1.2 Protocol v1 (`skkil.store.v1`), buf lint
- [x] 1.3 SQL write path v1: scope sequencer, idempotency, capture trigger,
      hash chain, rejection, server writes, poke

## 2. Service (Go)
- [x] 2.1 Config, logging, health, metrics, graceful shutdown, Dockerfile
- [x] 2.2 Sync-token verification (ES256, PEM or JWKS)
- [x] 2.3 Pull by cursor with epoch/hash checks and Reset
- [x] 2.4 Pokes stream; LISTEN/NOTIFY bus with polling fallback
- [x] 2.5 Tests on real Postgres, two instances

## 3. Client core (Rust)
- [ ] 3.1 Local schema: meta, pending, undo, issues; table registration
- [ ] 3.2 Mutate with pre-image capture; push request/response
- [ ] 3.3 Pull: undo, apply versions, ack, replay, rejections → issues, Reset
- [ ] 3.4 HLC, derived ids, status
- [ ] 3.5 Native HTTP driver (Pull, Push, Pokes)
- [ ] 3.6 Builds for wasm32 (SQLite on OPFS) and iOS/Android targets

## 4. Adapters
- [ ] 4.1 JVM: write path, mutation registry, Push handler, sync tokens
- [ ] 4.2 TypeScript driver: transport, poke stream, scheduler, tab lock

## 5. Conformance
- [ ] 5.1 Simulator: N clients, offline windows, rejections, poke loss
- [ ] 5.2 Suite over the wire against a product server; L1–L4 checks

## 6. Release
- [ ] 6.1 `skkil ci` green; tag v0.1.0
