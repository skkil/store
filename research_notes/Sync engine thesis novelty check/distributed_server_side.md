# Distributed server side of the sync engine (multi-instance, caches, microservices)

As of 2026-11-04. Tags: **[F]** = fetched this session (source linked), **[M]** = from memory, not re-verified.
Builds on `reports/Local first sync engine research.md` (per-user `server_version`, named mutations in one Postgres txn, hash-chained log, media service with claims §18) and `ARCHITECTURE.md` §8 (Redis), §9 (outbox→Kafka), §25 (MSA map).

**Bottom line.** Sync holds up across many instances if you keep three things in **one Postgres transaction on one shard**: the version counter, the mutation's effects, and the idempotency record. Everything else (Redis, Kafka, WebSocket gateways, other microservices, CDNs) sits *downstream* of that commit and may be lossy, stale or duplicated, as long as clients can always heal by pulling from the durable log with a cursor. Production systems converge on this shape: one ordered source per scope, many stateless or disposable readers, and lossy pokes.

---

## 1. How production sync servers scale out

| System | Writer / ordering | Readers / fan-out | Stateful? Routing | Lesson for us |
|---|---|---|---|---|
| **Zero** (Rocicorp) | Postgres is the truth. **Exactly one replication-manager** owns the replication slot and produces the change stream [F](https://zero.rocicorp.dev/docs/deployment) | N **view-syncers**, each with a SQLite replica (restored from a Litestream backup in S3), scale horizontally. CVRs (what each client group has) live in a Postgres `ZERO_CVR_DB` [F] | Sticky sessions recommended (cookie, ClientIP, Fly Replay) because pipelines are hydrated in memory. If two view-syncers serve the same `clientGroupID`, the loser forces a reconnect ("Rehome") [F] | Single ordered producer, many disposable readers. Stickiness is a **performance** hint; correctness comes from the CVR in the DB, so any node can resume [F, via DeepWiki summary: CVRVersion = `stateVersion` + `configVersion`] |
| **PowerSync Service** | One **replication container** reads the source DB's CDC and writes buckets to MongoDB/Postgres bucket storage. Rough capacity ~50k–100k concurrent clients per replication instance [F](https://docs.powersync.com/maintenance-ops/self-hosting/aws-eks) | 2+ **stateless API containers**, about one per 100 connections; streaming reads bucket storage only, never the source DB [F](https://docs.powersync.com/architecture/powersync-service) | Beyond that you run **multiple complete instances**, each with its own bucket DB, and route with `hash(user_id) % n`. Clients must always hit the same instance, because op lists and checksums differ between instances [F](https://docs.powersync.com/maintenance-ops/self-hosting/multiple-instances) | Cursors are **instance-local**. Re-routing a user to a different shard is a full resync unless cursors are portable. Shared buckets dedupe per-user state [F] |
| **Electric** | One instance holds the **replication lock**; rolling upgrades can deadlock on it [F](https://electric.ax/docs/guides/deployment) | Shape logs cached on local disk (NVMe; disk > memory > CPU). Designed to sit behind a CDN; long-poll `live=true` requests are collapsed by the CDN [F](https://electric.ax/docs/api/http) | Disk state must stay consistent with the PG slot/publication. If it is lost, clients get **`must-refetch`** and re-sync from scratch [F]. Offsets look like `123_4` (LSN_opIndex per Electric [M]) | Offsets derived from **WAL LSN** are commit-ordered for free. Immutable log segments are CDN-cacheable. Rewind is handled by an explicit reset signal |
| **Linear** | One **global `lastSyncId`** spanning the whole database, not per workspace. It jumps after a single change [F, unofficial reverse-engineering](https://github.com/Aslam97/reverse-linear-sync-engine) | Deltas broadcast over WebSocket, filtered by **sync groups** (user/team). The client compares `lastSyncId` on connect and fetches missed deltas. Bootstrap is served from a MongoDB cache [F, unofficial] | Each mutation's response returns a `lastSyncId`, and the client waits for that sync action (read-your-writes) [F, unofficial] | A global ID removes cross-workspace ordering questions but makes the sequencer a global hot spot. Linear's 2023 scaling talk covers falling-behind paths but has no transcript [F](https://linear.app/now/scaling-the-linear-sync-engine) |
| **Figma multiplayer** | **One process per document**. All clients of a file must reach the instance that owns it, or the file splits [F](https://www.figma.com/blog/rust-in-production-at-figma/) | In-memory state; journal writes to DynamoDB every ~0.5 s with per-file sequence numbers; S3 checkpoint every 30–60 s; 95% of changes durable in ~600 ms [F](https://www.figma.com/blog/making-multiplayer-more-reliable/) | Ownership through a **DynamoDB lock** (file, lock UUID). **Journal writes are conditional on the lock**, which acts as a fencing token. Recovery = checkpoint + replay of journal entries > checkpoint seq [F] | The actor-per-scope model needs a fence on the durable write, not just routing. Accepts <1 s of loss; our design must not (mutations are acked only after commit) |
| **Cloudflare Durable Objects / tldraw** | One single-threaded actor per ID. **Input gates** block other events during storage ops; **output gates** hold network messages until writes are durable [F](https://developers.cloudflare.com/durable-objects/reference/glossary/) | Gates open on `fetch()`/other I/O, so interleaving is still possible [F, community gotchas] | Platform does routing and uniqueness | A strong per-scope sequencer for free; the cost is platform lock-in and cross-DO transactions. Output gates = "don't ack before durable" |
| **Notion** | Postgres sharded by **workspace_id**: 480 logical shards across 32 physical DBs (2021); 480 chosen for its many divisors; routing in the application; migration by double-write [F](https://notion.com/blog/sharding-postgres-at-notion). Later expansion to 96 hosts [F, secondary only] | MessageStore / version pushes [M] | Logical shards move as units | **Logical shard ≫ physical host.** Shard key = sync scope key, so a scope never spans DBs |
| **Couchbase Sync Gateway** | Many SG nodes allocate from one counter `_sync:seq` in batches. Gaps ("skipped sequences") come from failed writes and contention; SG waits up to `MaxChannelLogMissingWaitTime` (60 min default) before abandoning a missing sequence. Counter rollback after vbucket failover (CBG-4036) and inflation (CBG-4016) were real bugs [F](https://www.couchbase.com/forums/t/sync-gateway-assigns-abandoned-sequence-number/20231) | Channels | Stateless-ish | A counter that is **not inside the write transaction** produces gaps and out-of-order visibility, which then need a skipped-sequence buffer. Avoid this |
| **Convex** | A single committer assigns commit timestamps (OCC); subscriptions re-run queries on read-set overlap [M, Sujay Jayakar talks/blog; not fetched] | WebSocket push of query results [F](https://convex.dev/sync) | | Single-writer timestamp oracle; TiKV-style oracles allocate ranges in memory and can't scale past one node [F](https://tikv.org/deep-dive/distributed-transaction/timestamp-oracle/) |
| **Ditto Big Peer** | Looks like one node from outside; inside, a partitioned distributed DB on k8s stateful sets; reconfigures through new cluster configurations; HLC = physical ms + per-peer counter; CDC to Kafka [F](https://docs.ditto.live/cloud/backend-architecture) | | | Treats the server as one logical peer. Server-side ordering details are not public |
| **Replicache (BYOB guidance)** | Push handler takes the write lock; mutation effects and `lastMutationID` are committed **in one transaction**. Invalid mutations must still advance `lastMutationID`; transient errors abort without advancing [F](https://doc.replicache.dev/reference/server-push) | After the push loop, **poke** clients so they pull. Await the poke or the framework may kill the request [F](https://v12.doc.replicache.dev/byob/remote-mutations) | Stateless | Idempotency lives in the DB transaction. Pokes are hints |

**Pattern.** Every system has **one ordered source per scope**: a replication slot (Zero, Electric, PowerSync), a per-file lock holder (Figma), a DO (tldraw), or a DB counter (Linear, Replicache). Readers are disposable and rebuildable from durable state. No system uses Redis as the ordering authority.

---

## 2. Ordering with multiple instances

### 2.1 Options for the per-scope sequencer

| Option | Correct? | Notes |
|---|---|---|
| **A. Counter row in the mutation txn**: `UPDATE sync_scope SET head = head + 1, head_hash = $h WHERE id = $scope RETURNING head` | **Yes.** The row lock is held until commit, so a scope's commit order = version order and there are no visible gaps | Any stateless instance can handle any push. Cost: one scope's writes are serialized for the length of the transaction. Throughput ≈ 1 / (txn time with the lock held), on the order of hundreds to ~1–2k/s per scope with `synchronous_commit=on` [M estimate; measure it]. Plenty for personal scopes. Shared workspaces need short txns (validate before taking the lock, take the counter lock **last**) |
| B. `SELECT … FOR UPDATE` on the scope row, then insert | Same as A | A is one round trip shorter |
| C. `pg_advisory_xact_lock(hash(scope))` + `max(version)+1` | Yes if xact-scoped | Hash collisions serialize unrelated scopes; invisible to schema; no stored head/hash. Prefer A |
| D. Global `bigserial` / sequence | **No** for cursor pulls | Values are allocated before commit, so commits land out of order and `WHERE id > cursor` skips rows that commit late [F](https://event-driven.io/en/ordering_in_postgres_outbox/). Same failure as Couchbase skipped sequences |
| E. Redis `INCR` | **No** | Not transactional with the PG write: gaps on abort, out-of-order commit, and rollback on Redis failover (async replication) [M] |
| F. Kafka partition keyed by scope as the log | Order yes; atomicity with PG no | Dual write. Jepsen (Nov 2024) found Kafka's transaction protocol allows write loss, aborted reads and torn transactions, still outstanding at publication; G1a at default isolation [F](https://jepsen.io/analyses/bufstream-0.1.0). Keep Kafka downstream of an outbox |
| G. Single-writer routing (consistent hash / actor per scope) + in-memory counter | Only with a **fence** | A paused GC or partition creates two owners. The durable write must be conditional on an epoch/lease (Figma's conditional journal writes [F]; Kleppmann's fencing tokens [M]). With PG, option A already *is* the fence |

### 2.2 Streams that cross scopes (global feeds, CDC, outbox relays, search indexers)

Use **commit order, not allocation order**:
- **WAL LSN** via logical replication (Electric offsets, Zero's replication-manager, PowerSync, Debezium) [F/M].
- **xid8 watermark** for pollers: store `pg_current_xact_id()` on each row and read only `xid < pg_snapshot_xmin(pg_current_snapshot())` [F](https://event-driven.io/en/ordering_in_postgres_outbox/). Or diff two snapshots with `pg_visible_in_snapshot` (pgque). Caveat: one long-running txn stalls the watermark [F](https://thebuild.com/blog/pgque-two-snapshots-and-a-diff/).
- The media service's relay already has this issue (§9: several relays publish out of order, so consumers version-gate). Keep version gating.

### 2.3 Hot scopes
- Personal scope: serialization costs almost nothing.
- Shared workspace: Linear-style per-workspace (or global) ID, so one counter row per workspace. Mitigations: keep the txn short, validate optimistically outside the lock, **batch** queued mutations from one push into one txn (Replicache allows this [F]), group commit. If a workspace still needs more, split it into sub-scopes (per project/document) with **per-scope cursors**. The client then holds a cursor vector `{scope: version}`, which is how PowerSync buckets and Zero CVRs work. Cross-sub-scope atomicity is lost. That is the real trade-off.

---

## 3. Caches: where Redis is safe

**Safe (immutable or advisory):**
- **Log entries keyed by `(scope, version)`** are immutable after commit, so caching them anywhere (Redis, local LRU, CDN) is safe. Electric's CDN design depends on this [F].
- **Snapshots at a fixed version** (bootstrap blobs) are immutable. Linear serves bootstrap from a MongoDB cache [F, unofficial]; PowerSync v4 puts prepared sync data in S3 [F].
- Presence, cursors-on-screen, typing indicators (ephemeral, loss is fine).
- Pub/sub **pokes** (lossy hints, §4).
- Rate limits, single-flight locks for expensive *idempotent* work (§8 `sf:*`).
- A **"scope head" hint** (`head:{scope} = version`) used only to skip a pull (`if client.version == head → 304`). If stale-low, the next poke or periodic pull fixes it. If stale-high, the client pulls and gets nothing. Stale values never cost correctness, only latency.

**Unsafe (breaks invariants):**
- **Cache-aside of mutable current state on the pull path.** The set-after-delete race (§8's "known race") gives a reader an old row *with a new cursor*. The client then advances its cursor past a change it never saw, which permanently violates monotonic reads and convergence. Pulls must read log entries > cursor from the DB (primary or a replica with a sufficient LSN), never "current row from cache".
- **Idempotency / `lastMutationID` in Redis.** It must commit atomically with the effects (Replicache [F]; §8 already says "never in Redis"). Losing Redis means double-applying non-idempotent named mutations.
- **Locks that guard ordering** (Redlock). No fencing, clock assumptions [M Kleppmann 2016]. §25 already rejects Redisson.
- **Authorization decisions cached past revocation** without a bound. The revocation window becomes cache TTL plus pub/sub lag, so measure it as §8 does.

**If you must cache mutable state**: version-guarded sets (`SET` only if the cached version is older; Lua CAS), **memcache-style leases**. On a miss the cache issues a 64-bit token bound to the key, and a later delete invalidates the token, so a stale set is rejected (load-link/store-conditional). Token issue is rate-limited to stop thundering herds [F, extracted from NSDI'13 PDF](https://www.usenix.org/system/files/conference/nsdi13/nsdi13-final170_update.pdf). Invalidate from the **commit log** (memcache's mcsqueal tails committed SQL and broadcasts deletes [F]); this matches ARCHITECTURE §9's cache-invalidator consumer. TAO adds read-after-write within a tier through write-through caching and version numbers [M].

**Read-your-writes across instances.** The push response returns `server_version = v`. The next pull says `min_version = v`. The pull handler serves from a replica only if `pg_last_wal_replay_lsn() ≥ commit_lsn(v)` (store the commit LSN, or keep it simple and always read the head scope from the primary). Otherwise it waits briefly or falls back to the primary. Linear does the client half: it waits for the sync action whose ID the mutation returned [F, unofficial].

---

## 4. Fan-out: WebSocket gateways across instances

- **Poke-then-pull** is the standard pattern. The poke carries `(scope, version)` and no data; the client pulls with its cursor (Replicache [F], Zero's poke/pull protocol [F, DeepWiki], Linear's delta-plus-catch-up [F, unofficial]). Sending data in the poke for small deltas is an optimization only if the client checks `delta.base == local_version`; otherwise it pulls.
- **Lost pokes are fine by construction.** Heal with (a) a pull on every (re)connect, (b) a periodic pull (30–60 s jittered) or a heartbeat that carries the head version, and (c) a pull whenever a received version jumps (`v > local + 1`).
- **Transport choice.** The bus is advisory, so pick for operational cost:
  - Redis pub/sub: at-most-once, no replay, fine for pokes. Redis Streams if you want short replay.
  - NATS core: at-most-once with subject routing (`scope.<id>`), JetStream for replay.
  - Kafka: durable, but per-gateway consumer groups and partition rebalances add latency. Use it for service-to-service events (media lifecycle, §9), not as the hot poke path. A gateway can still consume `sync.committed` from Kafka if it already exists.
- **Source of pokes.** Emit after commit from the push handler (fast path) **and** from the CDC/outbox stream (reliable path, which also covers server-originated changes from other services). Duplicates are harmless.
- **Gateway topology.** Gateways are stateless WebSocket terminators that subscribe to `scope.<id>` for connected users. Draining on deploy only drops pokes; clients reconnect and pull. Zero's advice to use long shutdown grace periods for draining applies [F].
- **Thundering herd** when a shared workspace with N clients gets a write: N pulls of the same `(scope, v)`. Serve log ranges from an immutable cache or coalesce like Electric's CDN collapsing [F].

---

## 5. Microservices

### 5.1 Recommendation: the sync core is a modular monolith per shard
- The sync log, version counter, idempotency table, and the **domain tables that mutations touch** live in **one Postgres per shard**, so a mutation is one local txn. Zero, PowerSync and Electric all assume one upstream DB [F]. Linear syncs one global ID space [F, unofficial]. Notion stayed a sharded monolithic Postgres rather than splitting by service [F].
- Domain code (tasks, notes, files tree) is modules inside the sync core: separate schemas and ownership, the same transaction.
- **Services outside the sync DB are referenced, never joined atomically.** The media service is the canonical case.

### 5.2 Cross-service mutations (sync core ↔ media service, own DB)
This is already solved by §18, carried over unchanged:
1. A mutation that references media M commits `{row, log entry, outbox(claim M, seq = server_version)}` in the sync DB.
2. A relay calls `PUT /media/M/claims/...` (idempotent, `last_seq` guarded). Media rejects with `410 Gone` → the relay writes a **server-originated compensating mutation** into the log (reference state `revoked`), which clients pull like any other change.
3. Clients see the reference state `pending → committed → revoked`, not cross-service atomicity. **Don't promise atomic visibility across services**; promise a bounded convergence window (outbox lag SLO < claim grace, §18).
- Inbound direction (media lifecycle events from Kafka, e.g. `media.deleted{moderation}`): an **idempotent inbox consumer** in the sync core turns each event into a server mutation in the owning scope's log (dedupe by `event_id`, version-gated by media `version`). The sync log stays the single ordered source clients read.
- Sagas/TCC: needed only when a *client* mutation must reserve something in another service before it can commit (quota, payment). Use reserve (TCC try) → commit the mutation → confirm, with expiry, which is the same as upload reservation §6.1.

### 5.3 Alternative: CDC from many service DBs into one sync log
- With Debezium per service DB (outbox event router; key = aggregate id, so order holds per aggregate only; at-least-once, dedupe by event-id header [F](https://debezium.io/blog/2021/11/30/debezium-1.8-beta1-released)), there is **no shared commit order across sources**. The merged log must either (a) give each source its own cursor and use a cursor vector, or (b) re-sequence on ingest (the sync core assigns `server_version` when applying each CDC event, which is what 5.2 does).
- Cross-service causal visibility (task created in task-svc, attachment in media-svc) is then only per-source. If a UI needs "both or neither", put both in the sync core or use reference states.
- Verdict: per-service sync logs multiply cursors, hash chains and failure modes. Use **one sync log per scope**, fed by in-txn writes plus an inbox for other services.

### 5.4 Sync gateway / BFF
A stateless gateway terminates WebSockets/HTTP, does authn, routes `scope → shard`, and fans out pokes. It must not own state. Business validators run in the sync core next to the DB txn, or on clients for E2EE replay. The gateway should not call domain services synchronously on the push path (latency, partial failure).

---

## 6. Sharding, multi-region, failover, restore

- **Shard key = sync scope id** (user or workspace). Notion-style **logical shards** (e.g. 512 or 480) mapped to physical DBs in a routing table or config [F]. A scope never spans shards, so its counter, log and hash chain stay local.
- **Rebalancing without breaking cursors.** Move a logical shard by copying (logical replication or double-write as Notion did [F]), catch up, briefly fence writes to that shard (the routing epoch increments, and the old primary rejects writes through a `shard_epoch` check in the txn), then flip routing. Cursors stay valid because `(scope, version, hash)` moves with the data. Contrast PowerSync, where moving a user between instances means a resync because bucket op ids are instance-local [F]. Make cursors data-derived, not instance-derived.
- **Multi-region.** Single home region per scope (the write leader); other regions serve pulls from replicas with the LSN check (§3). Active-active per scope reintroduces concurrent sequencers, so avoid it. Figma replicates checkpoints across regions, not the journal [F].
- **Failover.** With async replication, a promoted standby may lack the last k committed versions, and clients that already pulled them now have `client.version > server.head`, or the same version with a different hash. Required:
  - **Log epoch / `log_id`** in every cursor. Bump it on any failover that may have lost data, and on restore.
  - **Rewind detection**: on pull, the client sends `(epoch, version, hash)`. A mismatch → reset (Electric `must-refetch` [F], Replicache reset strategy [F](https://doc.replicache.dev/strategies/reset)). The client re-pushes its **unacked and lost-acked** mutations. Idempotency still holds because `(client_id, mutation_id)` rows were lost with the data, so they reapply once.
  - **Hash chain across epochs**: the first entry of the new epoch hashes over `(prev_epoch_last_durable_hash, epoch)` and records the fork point, so the chain is continuous but shows the rewind. With `synchronous_commit = remote_apply` / synchronous standby (quorum), failover loses nothing and no epoch bump is needed. That costs write latency, and is the choice to make for the thesis prototype.
- **Backup restore (PITR)** is a guaranteed rewind: always bump the epoch. The same issue exists for blobs. The DB↔blob divergence on restore (Supabase) is already in the thesis report. Run the §6.8 reconciliation after restore.

---

## 7. Testing a distributed sync server

- **Deterministic simulation** of the whole server cluster (instances, Redis, bus, PG modeled or real in a single-threaded harness). FoundationDB's simulator; TigerBeetle's VOPR; Antithesis offers deterministic hypervisor-level simulation for real binaries [M]. Dropbox Nucleus (CanopyCheck/Trinity) is already cited in the report.
- **Jepsen/Elle-style checkers on the sync log**: per scope, versions are dense, monotonic, and the commit order matches; no client cursor ever skips; prefix consistency between any two clients; read-your-writes after push. Elle found G2-item anomalies in PostgreSQL 12.3 SERIALIZABLE (2020) in fault-free runs [F](https://jepsen.io/analyses/postgresql-12.3). Don't rely on SSI for sequencing; the explicit row lock in option A is safer and easier to reason about. A 2026 pgsql-bugs thread reports another possible G2-item at SERIALIZABLE, still unresolved [F, snippet only](https://hackorum.dev/topics/139733).
- **Faults to inject**: instance kill between commit and poke; Redis flush or failover (must cost only latency); Kafka redelivery and reordering (version gate); replica lag on pull; split-brain owner (if using routing + in-memory state); PG failover with async replication (epoch path); PITR restore; shard move mid-push; clock skew (validators must not read clocks).
- **Ablations for the thesis**: (1) a `bigserial` cursor vs. a scope counter vs. an LSN cursor, counting skipped rows under concurrency; (2) cache-aside pull vs. log pull, counting monotonic-read violations; (3) poke loss rate vs. convergence time with and without periodic pull.

---

## 8. Design rules (for the SDK's server side)

1. **One scope, one counter row, one shard.** `server_version` comes from `UPDATE sync_scope … RETURNING` inside the mutation txn, with the lock taken last. This counter row *is* the sequencer, the fence and the hash-chain tip. Never use sequences, Redis or Kafka to assign it.
2. **Stateless push/pull instances.** Correctness never depends on routing. Sticky or consistent-hash routing is optional for cache warmth; if in-memory state is authoritative, writes need a fencing epoch.
3. **Idempotency (`client_id`, `last_mutation_id`) is committed in the same txn.** Rejected mutations advance it too.
4. **Cursor = `(shard_epoch/log_id, scope, version, hash)`.** It is data-derived and portable across instances and shard moves. A mismatch triggers reset plus re-push.
5. **Cross-scope or global consumers read in commit order** (LSN or xid8 watermark), never `id > cursor` on a sequence.
6. **Only immutable data goes in caches** (log entries by version, snapshots at a version, head hints). No cache-aside on the pull path. Idempotency and locks stay out of Redis. Mutable caches need version-guarded or lease sets plus CDC invalidation, with staleness measured as a revocation bound.
7. **Poke-then-pull over a lossy bus.** Pull on reconnect, periodically, and on version gaps. Pokes come from both the commit path and CDC.
8. **Read-your-writes**: push returns the version; pull takes `min_version`; replicas serve only if their replay LSN is high enough.
9. **The sync core is a modular monolith per shard.** Other services (media) are reached through outbox → idempotent seq-guarded calls (§18 claims) and an inbox for their events, which turns them into server mutations in the log. Expose reference states, not cross-service atomicity.
10. **Failover and restore bump the epoch**; the hash chain records the fork. Use synchronous replication if no client-visible rewind is acceptable.
11. **Kafka sits downstream of the log**, for service fan-out and analytics, with version-gated consumers.
12. **Prove it with simulation plus log checkers**, and with ablations that show each rule's failure when it is removed.

## Open / unverified
- Linear's server internals beyond the reverse-engineered client (Redis use, number of sync servers): not public in fetched sources.
- PowerSync checkpoint and op_id mechanics across restarts: not in fetched docs; check `powersync-service` source.
- Zero `stateVersion` ↔ LSN mapping: secondary source only.
- Throughput numbers for option A per hot scope: estimate only, must be benchmarked (pgbench with a custom script on one row).
- Notion's 96-host expansion: secondary source.
