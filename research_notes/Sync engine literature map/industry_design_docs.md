# Industry design documents, engineering posts, specs and essays on sync engines (annotated catalog)

Feeds the appendix of `reports/Sync engine thesis novelty check.md`. Claim IDs follow that report: C1–C8 (contributions), S1–S6 (blob/rejection/validator invariants), F1–F5 (folder lens laws; F4 crash recovery, F5 echo freedom), D1 (distribution transparency).

Method, 2026-11-04. I checked about 180 URLs with HTTP requests and fetched or searched about 20 of them for content. The tags mean:
- **[F]** The link resolved (HTTP 200, title checked), and the summary rests on content fetched this session or in an earlier session's notes that were marked verified.
- **[F-link]** The link resolved and the title matches, but the summary comes from model memory. Re-read the source before citing a specific detail.
- **[M]** Memory only. The link was not resolved this session.

Dates come from page metadata (`published_time` or `datePublished`) where it was available. Redirects are recorded: Linear moved `/blog` to `/now`, `jamsocket.com` now redirects to Modal's acquisition post, and Electric moved from `electric-sql.com` to `electric.ax`.

## Catalog

### A. Sync-engine vendors: architecture and design docs

#### Takeaway
The vendor docs that matter split along one line. Some vendors own reads only and hand writes to your API (Electric, PowerSync). Others run named mutators on both sides with a server rebase (Replicache/Zero, Convex, LiveStore). Every one of them treats rejection as a revert of a single write, and none specifies blob or folder semantics. That is the gap behind S2, S5 and F4/F5.

#### Cited Findings

### Replicache: How it works
- Rocicorp (Aaron Boodman et al.) · design doc · c. 2021–2023, now in maintenance
- https://doc.replicache.dev/concepts/how-it-works [F-link]
- Summary: Clients keep a queue of pending named mutations. They apply each one speculatively, push them to the server, pull the authoritative state, then rebase (replay) the mutations that are still pending on top of it. The server runs its own mutator implementations, so it holds authority, and a rejected mutation just disappears from the next pulled state.
- Relevance: This is the canonical prior art for C1 (server-ordered multi-row mutations with client rebase), and it is the "implicit cascade through re-execution" counter-argument the S2 claim has to beat.

### Zero docs: introduction, mutators, self-host/deployment, permissions, when-to-use, release notes
- Rocicorp · product docs · 2024–2026 (1.0 "First Stable Release", reported June 2026)
- https://zero.rocicorp.dev/docs/introduction ; https://zero.rocicorp.dev/docs/mutators ; https://zero.rocicorp.dev/docs/self-host (the old `/docs/deployment` redirects here) ; https://zero.rocicorp.dev/docs/auth#permissions (the old `/docs/permissions` redirects here) ; https://zero.rocicorp.dev/docs/when-to-use ; https://zero.rocicorp.dev/docs/release-notes ; https://www.infoq.com/news/2026/06/zero-version-1/ [F]
- Summary: zero-cache tails Postgres through logical replication into a SQLite replica and serves incremental ZQL queries. Custom mutators (0.18) run on the client and are then re-run authoritatively on your server. When a server mutator throws, the client reverts that one mutation, and "the next mutation runs as normal". Release-note titles show a permissions path of 0.3 write perms, 0.7 read perms, 0.15/0.16 permission deployment, 0.18 custom mutators and 0.23 synced queries, after which CRUD mutators were moved under a "deprecated" docs path. No page found states outright that declarative permissions were deprecated.
- Relevance: C1 and C6 (server re-execution as the validator), and the per-write rejection baseline for S2. The "when to use" page states that offline writes are unsupported, which positions the thesis's offline-first claim.

### PowerSync Service architecture, client architecture and consistency (write checkpoints)
- JourneyApps/PowerSync · design doc · 2024–2026
- https://docs.powersync.com/architecture/powersync-service ; https://docs.powersync.com/architecture/client-architecture ; https://docs.powersync.com/architecture/consistency ; https://docs.powersync.com/handling-writes/custom-write-checkpoints [F]
- Summary: The service replicates the source database (Postgres, MongoDB or MySQL) into per-client "buckets", and clients receive checkpoints that contain only fully committed transactions. After an upload, a client requests a write checkpoint and applies server state only once that checkpoint appears in a sync checkpoint, which keeps local writes from flickering. The docs claim causal+ consistency and cite Jepsen testing. Custom write checkpoints, with a `checkpoints` table and the `checkpoint_requests` event from service 1.26.0, cover backends that apply uploads asynchronously.
- Relevance: C1 (an ordered upload queue applied by your backend). Write checkpoints are the closest industry analogue to the "pending → committed" visibility state in S1/C3a.

### PowerSync Sync Streams
- PowerSync · spec/docs · 2025–2026
- https://docs.powersync.com/sync/streams/overview (redirected from `/sync/streams`) ; https://docs.powersync.com/sync/advanced/compatibility [F]
- Summary: Sync Streams replace the older bucket-based Sync Rules with query-shaped subscriptions, and new configs should use `edition: 3` with a new compiler. The compatibility page versions both bucket storage and edition semantics so that the server can change without breaking existing clients.
- Relevance: Partial replication (S3, revocation from partial replicas) and D1 (server-side storage versioning).

### PowerSync attachments and write-validation errors
- PowerSync · docs · 2025–2026
- https://docs.powersync.com/client-sdks/advanced/attachments.md ; https://docs.powersync.com/handling-writes/handling-write-validation-errors.md [F]
- Summary: The attachment helpers are marked alpha. They keep a local attachment queue, upload separately from row sync, and garbage-collect by *local* reference set plus an LRU archive. On validation failure the docs tell the backend to return 2xx, after which the client "will revert to the last known state". Nothing is said about dependent writes or blobs already uploaded.
- Relevance: The most direct counterpart for C2, S1, S2 and S5. It shows the gap exactly: no cross-device reference state, no rejection cascade to blobs, and GC that ignores offline holders.

### Electric 1.0 released, plus the Shapes and Writes guides
- Electric (James Arthur, Kyle Mathews et al.) · blog + docs · 2025-03-17
- https://electric.ax/blog/2025/03/17/electricsql-1.0-released ; https://electric.ax/docs/sync/guides/shapes ; https://electric.ax/docs/guides/writes [F]
- Summary: Electric was rewritten in 2024, dropping its earlier CRDT design for read-path-only sync. 1.0 syncs single-table "shapes" of Postgres over plain HTTP that CDNs can cache, and production apps authorize shapes through a backend proxy. Writes go through your own API, and optimistic state is held until the write's txid streams back.
- Relevance: C1 (Electric chose *not* to own the write path) and D1, since it exploits stale immutable caches and CDN fan-out, which is the "stale immutable caches" condition D1 must survive.

### Announcing Durable Streams / Durable Streams 0.1.0 and State Protocol
- Electric · blog + protocol repo · 2025-12-09 and 2025-12-23 (hosted version announced 2026-01-22)
- https://electric.ax/blog/2025/12/09/announcing-durable-streams ; https://electric.ax/blog/2025/12/23/durable-streams-0.1.0.md ; https://github.com/durable-streams/durable-streams [F]
- Summary: Durable Streams turns Electric's HTTP shape-log transport into a standalone protocol: an append-only, offset-addressed stream that can be resumed and cached over HTTP, with a "State Protocol" on top for materialized state. Electric's 2026 posts build Yjs-over-HTTP, StreamDB and forking (branching) on it, aimed at agent sessions.
- Relevance: C1 and D1. It is a standardized, cache-friendly, resumable log transport the thesis could reuse for its per-user log instead of inventing one.

### Electric is joining Databricks
- Electric (James Arthur, Valter Balegas) · blog · 2026-08-11
- https://electric.ax/blog/2026/08/11/electric-joining-databricks [F]
- Summary: Electric joined Neon at Databricks to put sync and PGlite into Lakebase. Electric Cloud is winding down, and cloud users must self-host or migrate. Postgres Sync, Durable Streams, PGlite and TanStack DB stay open source under Apache 2.0.
- Relevance: C8 and impact. This is evidence that standalone sync-SDK economics are weak and that BYO-Postgres plus self-hosting is the expectation.

### TanStack DB overview, 0.1 launch post and Electric partnership
- TanStack (Tanner Linsley, Kyle Mathews, Sam Willis) · design/blog · 2025-07-30 (0.6, March 2026, adds persistence and includes)
- https://tanstack.com/db/latest/docs/overview ; https://tanstack.com/blog/tanstack-db-0.1-the-embedded-client-database-for-tanstack-query ; https://tanstack.com/blog/electric-partnership [F]
- Summary: TanStack DB is a backend-agnostic client store with collections, differential-dataflow live queries and transactional optimistic mutations. Pending optimistic state is layered over synced state and dropped when the server confirms. Collection adapters cover Electric, PowerSync and plain REST.
- Relevance: C1 (a client-side transaction overlay). It shows the industry separating the "local transactional overlay" from the transport, the same layering the thesis uses.

### How LiveStore works / design decisions / syncing / encryption
- LiveStore (Johannes Schickling) · design doc · 2024–2026, pre-1.0
- https://docs.livestore.dev/overview/how-livestore-works (redirected) ; https://docs.livestore.dev/understanding-livestore/design-decisions/ ; https://docs.livestore.dev/building-with-livestore/syncing/ ; https://docs.livestore.dev/patterns/encryption [F]
- Summary: All writes are immutable, ordered events. A sync backend gives them a total order through git-like push/pull with client rebase, and every client materializes them into reactive SQLite. Encryption is left to an application pattern rather than built in.
- Relevance: C1 and C6. Materializers that every client replays deterministically are a relative of the S6 "validator replayed on clients", but they apply state and do not validate it.

### What is Jazz? and the Jazz 2.0 alpha repo / How sync works
- Garden Computing (Anselm Eickhoff, Guido D'Orsi et al.) · blog + repo · April 2026 (2.0 alpha)
- https://jazz.tools/blog/what-is-jazz ; https://github.com/garden-co/jazz ; https://jazz.tools/docs/concepts/how-sync-works [F]
- Summary: Jazz 1.x used CRDT "CoValues" with signed, encrypted group permissions. 2.0 (alpha, entirely new API) repositions as a "local-first relational database" with table schemas, relations, queries and mutations, tunable per-state consistency (eventual for most, global coordination for some) and fluid schema migrations. Sync covers partial tables, streams and files, and server builds use RocksDB.
- Relevance: C8 and C6/E (crypto permissions instead of a server validator). It is the vendor moving fastest toward R+B+E, so watch it as a C8 threat.

### Instant: "A backend for AI-coded apps" architecture essay and sunset banner
- InstantDB (Joe Averbukh, Stepan Parunashvili, Daniel Woelfel, Drew Harris) · design essay · c. April 2026
- https://www.instantdb.com/essays/architecture ; https://runtimewire.com/article/instant-team-joins-openai-cloud-shutdown [F]
- Summary: The client keeps EAV triples in IndexedDB, runs Datalog locally, and holds a pending queue for optimistic updates and undo. The Clojure backend compiles each query into "topics", and an invalidator tails the Postgres WAL and refreshes only queries whose topics match. All tenants share one multi-tenant triples table. A site banner reads "Instant is sunsetting. Services will continue until August 31st, 2027", following the team's move to OpenAI.
- Relevance: C1 and D1 (WAL-driven invalidation fan-out across stateless servers). The shutdown is impact evidence for C8.

### Convex: How Convex Works / An Object Sync Engine for Local-first Apps / Convex sync page
- Convex (Sujay Jayakar, James Cowling et al.) · design essays · c. 2024–2025
- https://stack.convex.dev/how-convex-works ; https://stack.convex.dev/object-sync-engine ; https://www.convex.dev/sync [F-link]
- Summary: Queries and mutations are deterministic TypeScript functions run inside a transactional, serializable server database. Clients subscribe to query results, and the server re-runs a query when its read set is invalidated. The "object sync engine" essay sketches how Convex would extend this to local-first, with a local store, offline mutations and a server rebase, while staying server-authoritative.
- Relevance: C6 (deterministic server functions as the validator) and C1. A good contrast case for "validator runs only on the server".

### Triplit joins Supabase
- Supabase (Matt Linkous) · blog · 2025-10-08
- https://supabase.com/blog/triplit-joins-supabase [F]; Triplit docs now return HTTP 410 (https://www.triplit.dev/docs)
- Summary: Triplit's co-founder joined Supabase, which said it would not integrate Triplit and that offline support "remains an open problem". Triplit's own design docs (a relational-ish triple store with a server and client query engine) are gone from the web.
- Relevance: C8 and impact. It also shows how fragile vendor design docs are as citations, so archive what you cite.

### SQLite Sync (sqlite.ai)
- SQLite Cloud / sqlite.ai · README + product docs · 1.0 on 2026-03-24
- https://github.com/sqliteai/sqlite-sync ; https://www.sqlite.ai/sqlite-sync [F]
- Summary: A SQLite extension (`cloudsync_init`, `cloudsync_network_sync`) that turns tables into CRDT-merged replicas (causal-length set, delete-wins, add-wins, block-level LWW for text) and syncs them with SQLite Cloud or any Postgres. Merges happen per change, with no cross-row transaction semantics.
- Relevance: C5 (per-column merge types) and the C1 contrast (no multi-row atomic mutation).

### Turso Offline Sync public beta + Turso Sync usage
- Turso · blog + docs · 2025-03-31
- https://turso.tech/blog/turso-offline-sync-public-beta ; https://docs.turso.tech/sync/usage [F]
- Summary: Explicit `push()`/`pull()` over row-level logical logging, with "last push wins" as the default. The beta was not production-ready and gave no durability guarantees at launch.
- Relevance: C1 and C5 baseline. It is the simplest LWW row-sync design to compare against.

### Graft internals (and "Stop syncing everything")
- orbitinghail (Carl Sverre) · design doc + essay · 2025–2026
- https://graft.rs/docs/internals/ (moved from https://github.com/orbitinghail/graft/blob/main/docs/design.md) ; https://sqlsync.dev/posts/stop-syncing-everything/ [F]
- Summary: Graft is a transactional storage engine for lazy, partial, page-level replication to the edge. A volume is a point-in-time view built from LSN ranges, and pages are fetched from object storage only on a cache miss. Local commits roll up into compressed segments in object storage and are committed with conditional writes, with optimistic validation and retry on conflict.
- Relevance: C2 and S5. It uses object storage with conditional writes as the commit fence, which is precedent for the S3 `If-None-Match` fence spike, and it shows lazy blob or page fetch on partial replicas.

### An inside look at Ditto's delta-state CRDTs / Ditto backend (Big Peer) architecture / data authorization
- Ditto · blog + docs · c. 2023–2025
- https://www.ditto.com/blog/an-inside-look-at-dittos-delta-state-crdts (redirected) ; https://docs.ditto.live/cloud/backend-architecture ; https://docs.ditto.live/key-concepts/syncing-data [F] (the old `v4-4/big-peer-architecture` URL now returns 404)
- Summary: Small peers sync delta-state CRDT documents over a BLE, LAN or P2P Wi-Fi mesh. The "Big Peer" cloud tier stores CRDTs and syncs over WebSocket. Permissions decide which documents a peer may receive, so unauthorized data is never synced to it.
- Relevance: C5 and C7 (CRDT by type), and the closest prior art for S3 (never sync unreadable data). Attachments are a separate type, relevant to C2.

### Couchbase Sync Gateway sync function + CouchDB replication protocol
- Couchbase / Apache CouchDB · docs/spec · living docs
- https://docs.couchbase.com/sync-gateway/current/access-control/sync-function/sync-function.html (redirected) ; https://docs.couchdb.org/en/stable/replication/protocol.html [F-link]
- Summary: CouchDB replicates per-document revision trees using a changes feed and checkpoints. Conflicts survive as sibling revisions with a deterministic winner. Sync Gateway adds a JavaScript "sync function" that validates each document write, can throw `forbidden`, and routes documents into channels for partial replication. Attachments ride alongside documents.
- Relevance: C6 (a server-side per-write validator) and S2. Forum evidence already in the report shows attachments uploaded even when the document was rejected with 403.

### Evolu docs
- Evolu (Daniel Steigerwald) · docs · living
- https://www.evolu.dev/docs [F-link]
- Summary: Local SQLite with mandatory end-to-end encryption, syncing through relays. It uses column-level LWW with hybrid logical clocks and Merkle-tree range diffing, in the lineage of Actual Budget.
- Relevance: C5 and E (mandatory rather than optional E2EE). It is a contrast point for C6's opt-in trust mode.

### Actual Budget: "Using CRDTs in the Wild" and sync docs
- James Long / Actual · blog + docs · c. 2019–2020; docs are living
- https://archive.jlongster.com/using-crdts-in-the-wild ; https://actualbudget.org/docs/getting-started/sync/ [F] (the original `jlongster.com` URL returns 404, so cite the archive)
- Summary: Each budget cell is an LWW register keyed by a hybrid logical clock. Clients exchange CRDT messages through a dumb server, a Merkle trie over timestamps finds where replicas diverge, and E2EE is opt-in on the same protocol. The server runs no validation in either mode.
- Relevance: C5 and C6 (opt-in E2EE without a validator). It is the reference for "a Merkle trie solves the missing total order", which the thesis does not need.

#### Inferences
- The vendors that survived 2025–2026 (PowerSync, Zero, Electric's open source) all sit beside BYO Postgres. Those that owned the database (Triplit, Instant Cloud, Realm) shut down.
- No vendor document specifies what happens to a blob or a dependent row when a write is rejected. The documented contract ends at "revert this write".

#### Gaps
- No page found states outright that Zero deprecated declarative row permissions; the release-note titles suggest it but do not say so. The Databricks side of the Electric acquisition post (databricks.com) returned 404.

### B. In-house engineering: product teams' own engines

#### Takeaway
The in-house systems are the real benchmarks. All of them are server-ordered (Linear, Figma, Notion, Google Docs, tldraw) or files-canonical (Dropbox, Obsidian). Each solves one domain. Dropbox is the only one with a published merge-base and testing methodology for file trees.

#### Cited Findings

### The data model behind Notion's flexibility
- Notion · engineering blog · 2021-05-18
- https://www.notion.com/blog/data-model-behind-notion [F]
- Summary: Everything is a block, with typed properties, a parent pointer and a content array. Edits are batched into operations grouped as transactions (`/saveTransactions`), and the server validates them against before and after state. A MessageStore fans out version bumps, and clients then pull the changed records.
- Relevance: C1 (server-validated multi-op transactions) and D1 (a notification bus plus pull, the same "poke then pull" split the thesis uses).

### Herding elephants: sharding Postgres at Notion / The Great Re-shard
- Notion · engineering blog · 2021-10-06 and 2023-07-17
- https://www.notion.com/blog/sharding-postgres-at-notion ; https://www.notion.com/blog/the-great-re-shard [F]
- Summary: Notion partitioned by workspace ID into 480 logical shards over 32 physical databases, later expanded to 96, with double-writes, backfill, verification and a zero-downtime cutover.
- Relevance: D1 (shard moves and failover while invariants still hold) and the "scope-row sequencer" partitioning choice.

### Building and scaling Notion's data lake
- Notion · engineering blog · 2024-07-01
- https://www.notion.com/blog/building-and-scaling-notions-data-lake [F-link]
- Summary: Notion moved from Snowflake ingestion to a CDC pipeline (Debezium, Kafka, Hudi, S3, Spark) because its block table is update-heavy, not append-only.
- Relevance: D1 only in passing, since a CDC-off-the-log design keeps analytics outside the sync path. This is low-priority background.

### How we made Notion available offline
- Notion · engineering blog · 2025-12-11
- https://www.notion.com/blog/how-we-made-notion-available-offline [F]
- Summary: The SQLite cache was promoted to a durable store, and an `offline_action` table records *why* each page is kept offline. A page is evicted once no reasons remain. Offline pages moved to Notion's newer CRDT text model. The server pushes per-page channel notifications, partially loaded pages are never shown, and database views are capped at 50 rows.
- Relevance: S5 (reference-reason tracking for retention is the same structure as lease or claim reasons), S3 (partial-replica scope), and C7 (CRDTs only where offline editing needs them).

### Linear: Scaling the Linear Sync Engine (talk + post) and the React Helsinki 2020 talk
- Linear (Tuomas Artman) · talk + blog · 2023-06-29; original talk at React Helsinki in February 2020
- https://linear.app/now/scaling-the-linear-sync-engine ; https://www.youtube.com/watch?v=WxK11RsLqp4 (React Helsinki Feb 2020 meetup video; that it contains Artman's talk is from memory) [F]
- Summary: Linear uses an in-memory MobX object graph persisted to IndexedDB, transactions that the server executes, and one ordered stream of sync actions per workspace keyed by a monotonically increasing `lastSyncId`. The 2023 sequel covers scaling: partial bootstrap and lazy loading, and how the API shaped itself around those constraints.
- Relevance: C1 (a global sync id with gap detection) and D1 (one sequencer per workspace).

### Linear: Rebuilding the delta sync read path + reverse-linear-sync-engine
- Linear engineering / wzhudev (community reverse-engineering) · blog + repo · 2024–2025
- https://linear.app/now/rebuilding-delta-sync-read-path ; https://github.com/wzhudev/reverse-linear-sync-engine [F]
- Summary: Clients send a checkpoint (the last applied sync id) and receive permission-filtered "delta packets" over WebSocket after a full or partial bootstrap. The reverse-engineered write-up documents the transaction batching, the object pool and per-action sync ids.
- Relevance: C1 and S3. It shows how permission filtering on the read path is done in production.

### How Figma's multiplayer technology works
- Figma (Evan Wallace) · engineering blog · 2019
- https://www.figma.com/blog/how-figmas-multiplayer-technology-works/ [F]
- Summary: One server process per live document acts as the central authority. Properties merge per property with last-writer-wins in server order, without timestamps, and the design is "inspired by CRDTs" but is not one. The server rejects tree-reparenting cycles, and fractional indices are stored with the parent pointer.
- Relevance: C5 (per-property LWW) and C7, plus C6 (the server rejects invalid structure, a precursor of authority-driven rejection).

### Realtime editing of ordered sequences (fractional indexing)
- Figma (Evan Wallace) · engineering blog · 2017
- https://www.figma.com/blog/realtime-editing-of-ordered-sequences/ [F-link]
- Summary: Figma orders children by fractional-index strings between neighbors instead of using an OT or CRDT sequence. This is enough when a central server orders conflicting moves, and it has known interleaving trade-offs.
- Relevance: C5 and C7 (ordered fields in records without a sequence CRDT). It is useful for `time` task ordering.

### Making multiplayer more reliable
- Figma · engineering blog · 2022-10-20
- https://www.figma.com/blog/making-multiplayer-more-reliable/ [F]
- Summary: Figma added a write-ahead journal so the multiplayer server persists every accepted change durably before checkpointing whole files. This cut the window in which a server crash loses acknowledged edits.
- Relevance: F4 and D1 (crash recovery of an acknowledged log versus snapshots). It is the closest industry statement of "acknowledged means durable".

### LiveGraph: real-time data fetching at Figma (+ "Keeping it 100(x)" follow-up)
- Figma · engineering blog · 2021-10-14; the follow-up dates to about April 2026 according to secondary sources
- https://www.figma.com/blog/livegraph-real-time-data-fetching-at-figma/ [F]; for the 100x post only mirrors were found, and the figma.com URL guessed for it returned 404 [M]
- Summary: LiveGraph serves GraphQL-like subscriptions over non-document data (comments, files, teams) by tailing the Postgres replication stream and invalidating cached queries. The 100x redesign broke the dependency on a single globally ordered replication stream once the database was sharded.
- Relevance: D1 (invalidation fan-out across sharded sources and the loss of a global order). It also shows Figma syncing relational metadata separately from document state.

### Rewriting the heart of our sync engine / Testing our new sync engine (Nucleus)
- Dropbox · engineering blog · 2020
- https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine ; https://dropbox.tech/infrastructure/-testing-our-new-sync-engine [F-link]
- Summary: Nucleus models sync as three trees (Remote, Local and Synced, the last being the merge base) and a planner that converges them. It is single-threaded and deterministic by design, so randomized testing (CanopyCheck, Trinity) can replay any failure from a seed.
- Relevance: C4 and F1–F5 (the merge-base model for folders) and the testing methodology for F4/F5 and the S-oracles. This is the strongest industry precedent for the thesis's deterministic simulator.

### What's different about the new Google Docs (OT)
- Google Drive team · blog · 2010-09
- https://drive.googleblog.com/2010/09/whats-different-about-new-google-docs_22.html [F-link]
- Summary: Google Docs keeps a server-ordered revision log of character-level operations, and operational transformation (Jupiter-style, central server) rebases concurrent client ops. The series explains why a central server makes OT tractable.
- Relevance: C1 (server order plus client rebase, applied to text) and C7. It is classic, and the field still cites it.

### tldraw sync docs and sync-core
- tldraw · docs + source · 2024–2026
- https://tldraw.dev/docs/sync ; https://github.com/tldraw/tldraw/tree/main/packages/sync-core [F-link]
- Summary: Each room holds a server-authoritative record store, typically one Cloudflare Durable Object per room. Clients push diffs that the server applies, rebroadcasts or rejects, with clients rebasing pending diffs. It does not use CRDTs.
- Relevance: C1 and C7 (central authority instead of CRDTs for high-frequency collaboration) and D1 (single-writer actor per room).

### End-to-End Encryption in the Browser (Excalidraw)
- Excalidraw · engineering blog · c. 2020
- https://plus.excalidraw.com/blog/end-to-end-encryption [F-link]
- Summary: The key lives in the URL fragment, which never reaches the server. The relay and storage only see ciphertext. Element-level reconciliation uses `version` with a `versionNonce` tiebreak.
- Relevance: C6 and E (a server that cannot validate) and C5 (per-element versioned LWW).

### Obsidian Sync: Security and privacy
- Obsidian · help docs · living
- https://obsidian.md/help/sync/security (redirected from help.obsidian.md) [F-link]
- Summary: End-to-end encrypted, files-canonical vault sync with version history and an optional choice between E2EE and managed keys. Conflicting Markdown edits are merged rather than forked (memory; check before citing).
- Relevance: C4 and E (the B+F+E cluster). It is not an SDK and has no relational layer, which is the C8 contrast.

### Logseq: What's new with Logseq DB (May 16, 2026), Markdown Mirror
- Logseq · forum release post · 2026-05-16
- https://discuss.logseq.com/t/whats-new-with-logseq-db-may-16th-2026/35020 [F]
- Summary: The DB version writes a one-way Markdown projection of each graph to disk, with stable block IDs embedded in comments so that renames and moves survive. Two-way editing "is in active development on a feature branch". No public ADR 0017 was found; `github.com/logseq/logseq/tree/feat/db/docs/adr` returned 404.
- Relevance: C4, F1–F5. This is the live threat to the folder-replica claim, so check it monthly.

### Apple: Mirroring a Core Data store with CloudKit, WWDC19 "Using Core Data with CloudKit", WWDC21 "Build apps that share data through CloudKit and Core Data", WWDC22 10094, File Provider
- Apple · docs + talks · 2019–2022
- https://developer.apple.com/documentation/coredata/mirroring-a-core-data-store-with-cloudkit ; https://developer.apple.com/videos/play/wwdc2019/202/ ; https://developer.apple.com/videos/play/wwdc2021/10182/ ; https://developer.apple.com/videos/play/wwdc2022/10094/ ; https://developer.apple.com/documentation/fileprovider [F-link]
- Summary: `NSPersistentCloudKitContainer` mirrors Core Data to CloudKit record zones using record-level LWW, and production schemas may only change additively. CKAsset handles blobs, `encryptedValues` provides per-field encryption, and atomic zone saves keep a record and its assets together. File Provider is the OS-level replicated-folder API that iCloud Drive and third-party drives use, with materialization on demand. (Mapping WWDC22 10094 to CloudKit is from memory.)
- Relevance: C3a/S1 (an atomic record plus asset save, the online case of S1), C4 (File Provider is the platform's folder-replica primitive) and C6/E.

### WhatsApp Security Whitepaper: App State Syncing
- WhatsApp/Meta · security whitepaper · v10, updated 2026-10-30
- https://www.whatsapp.com/security/WhatsApp-Security-Whitepaper.pdf [F] (section "App State Syncing Security", p. 25)
- Summary: Settings and chat properties are grouped into "collections" and synced between a user's devices end-to-end encrypted. The server stores ciphertext it cannot decrypt. Integrity relies on LtHash, a homomorphic hash, so each device can verify the state it holds after applying patches and snapshots.
- Relevance: C6 and S6 (integrity of an encrypted, server-relayed state, and the role of a hash chain or state hash in detecting a dishonest server).

#### Inferences
- Notion's `offline_action` reasons and Dropbox's Synced tree are the two in-house designs closest to S5 (retention by reason) and F1–F5 (merge base). Cite both as reuse.

#### Gaps
- No primary WhatsApp engineering post on syncd internals was found beyond the whitepaper. The figma.com URL for "Keeping it 100(x)" was not found. No Linear engineering post newer than 2024–2025 was found.

### C. Open protocols and specs

#### Takeaway
Open protocols bring the integrity and reconciliation tools the thesis needs: signed repos, range-based set reconciliation and encrypted sync. None of them carries relational or validator semantics.

#### Cited Findings

### AT Protocol: Repository spec + Sync spec
- Bluesky · spec · living (2023–2026)
- https://atproto.com/specs/repository ; https://atproto.com/specs/sync [F]
- Summary: Each user's repo is a Merkle Search Tree of records with a signed commit. The commit's `prev` field is usually null, so the signed root proves current state but not history continuity. Firehose sync carries commit diffs, and relays verify them.
- Relevance: S6 and C6. It shows why a state root alone cannot detect dropped operations, which motivates the per-user hash chain.

### NIP-77: Negentropy syncing
- Nostr · spec · 2024
- https://github.com/nostr-protocol/nips/blob/master/77.md [F-link]
- Summary: Range-based set reconciliation (Negentropy) between a client and a relay over sorted event IDs, so reconciliation costs roughly the size of the difference.
- Relevance: S5 and C2. This is the recommended anti-entropy plugin over the blob hash set for repair after restore or GC.

### Willow: data model, confidential sync (WGPS)
- Willow (Aljoscha Meyer, Sammy Gwilym) · spec · 2024–2026
- https://willowprotocol.org/specs/data-model/ ; https://willowprotocol.org/specs/confidential-sync/ (redirected from `/specs/sync/`) [F]
- Summary: Willow is a namespaced, three-dimensional (subspace, path, time) data model. Prefix pruning gives true deletion, capabilities provide authorization, and payloads are content-addressed. Confidential sync uses 3D range-based set reconciliation and private area intersection, so peers reveal only what both are authorized to see.
- Relevance: S3 and S5 (deletion that really propagates, and capability-scoped sync) and C2 (content-addressed payloads separated from entries).

### Automerge Beelay protocol + Sedimentree / Subduction / Keyhive (ARK)
- Ink & Switch / Automerge (Alex Good, Brooklyn Zelenka) · protocol docs + repos · 2024–2026
- https://github.com/automerge/beelay/blob/main/docs/protocol.md ; https://github.com/automerge/beelay/blob/main/docs/sedimentree.md ; https://github.com/inkandswitch/subduction ; https://automerge.org/docs/keyhive/ark-api-guide/ [F]
- Summary: Beelay is an RPC sync protocol for collections of Automerge documents that a relay server can run without reading them. Sedimentree chunks history deterministically into content-addressed strata, so encrypted servers can reconcile without materializing documents. Subduction is the newer peer-to-peer CRDT sync layer over encrypted partitions. Keyhive (alpha) supplies capability-based access control and group encryption.
- Relevance: C6 and E (sync over ciphertext with no server validator, the opposite pole from S6) and C2 (content-addressed chunks).

### Cloudflare Durable Objects (intro post, "easy, fast, correct: choose three", docs)
- Cloudflare · blog + docs · 2020-09 onward; the storage post is from 2021
- https://blog.cloudflare.com/introducing-workers-durable-objects/ ; https://blog.cloudflare.com/durable-objects-easy-fast-correct-choose-three/ ; https://developers.cloudflare.com/durable-objects/ [F-link]
- Summary: A globally unique, single-threaded actor per ID with transactional storage. Output gating holds outgoing messages until writes are durable. tldraw, PartyKit and Y-Sweet-style rooms run on it.
- Relevance: D1 (a single-writer sequencer per scope, the alternative to a stateless-instance sequencer) and F4 (output gating is "acknowledge only after durable").

#### Inferences
- An open design would combine atproto's signed-root lesson, Negentropy's reconciliation and Willow's capability-scoped deletion to cover the S3, S5 and S6 integrity needs without inventing primitives.

#### Gaps
- Willow and Subduction status (v1, released or not) was not confirmed.

### D. Collaboration infrastructure

#### Takeaway
Collaboration infrastructure vendors (Y-Sweet, Liveblocks) store CRDT documents next to the relational database rather than inside it. Several were acqui-hired in 2025–2026.

#### Cited Findings

### Y-Sweet (Jamsocket) and "You might not need a CRDT"
- Jamsocket (Paul Butler) · repo + essay · 2023–2024; Jamsocket joined Modal in 2026
- https://github.com/jamsocket/y-sweet [F]; https://jamsocket.com/blog/you-might-not-need-a-crdt now redirects to https://modal.com/blog/jamsocket-is-joining-modal [F], so the essay itself is offline; use the Wayback Machine [M]
- Summary: Y-Sweet is a Yjs server that keeps documents in S3, one actor per document. The essay argues that when a central server exists, server-ordered operations plus rebase are simpler than CRDTs.
- Relevance: C7 (CRDT storage beside the database) and C1 (the argument for server order).

### Liveblocks: Understanding sync engines (Figma, Linear, Google Docs) + WebSocket infrastructure docs
- Liveblocks · blog + docs · c. 2024–2025
- https://liveblocks.io/blog/understanding-sync-engines-how-figma-linear-and-google-docs-work ; https://liveblocks.io/docs/platform/websocket-infrastructure [F-link]
- Summary: The blog compares OT (Google Docs), server-ordered LWW (Figma) and an ordered transaction log (Linear). The platform docs describe room-based WebSocket servers holding Yjs and Storage state.
- Relevance: C1, C5 and C7. A secondary but well-organized comparison, good for an appendix pointer.

#### Gaps
- None material for this group.

### E. Essays that shaped the field

#### Takeaway
The essays trace an arc. Ink & Switch's local-first ideals (2019) gave way to a 2024–2025 consensus around "server-authoritative sync engine with a local store" (Weidner, Prokopov, Boodman). Ink & Switch then pivoted back toward encrypted, capability-based sync (Keyhive). That arc frames C6/S6 as a trust-mode switch between the two camps.

#### Cited Findings

### Local-first software: you own your data, in spite of the cloud
- Ink & Switch (Kleppmann, Wiggins, van Hardenberg, McGranaghan) · essay + Onward! 2019 paper · 2019
- https://www.inkandswitch.com/essay/local-first/ (the old `/local-first/` also resolves) ; https://martin.kleppmann.com/2019/10/23/local-first-at-onward.html [F]
- Summary: Sets seven ideals: no spinners, multi-device, offline, collaboration, longevity, privacy and user control. It argues CRDTs are the enabling technology and evaluates existing apps against the ideals.
- Relevance: C8 framing and E (privacy ideal). Most reviewers will expect this as the field's anchor citation.

### Architectures for Central Server Collaboration
- Matthew Weidner · essay · 2024-06-04
- https://mattweidner.com/2024/06/04/server-architectures.html [F]
- Summary: Classifies central-server collaboration designs, including server reconciliation with client rebase, CRDT relay and OT, and argues that server reconciliation (Replicache/Figma style) is usually the simplest correct choice when a server exists.
- Relevance: C1 and C6. It justifies treating C1 as deliberate reuse rather than a contribution.

### Sync Engines are the Future
- Nikita Prokopov, on InstantDB's essays · essay · 2025-03-17 (about 327 points on HN)
- https://www.instantdb.com/essays/sync_future [F]
- Summary: Argues that fetch and REST only solve one-shot retrieval, and that a sync engine collapses the database and the delivery layer into one, so apps should subscribe to data rather than request it.
- Relevance: C8 framing. It is the most-cited 2025 "why sync engines" essay.

### Local, first, forever
- Nikita Prokopov (tonsky) · essay · 2024-06-24
- https://tonsky.me/blog/crdt-filesync/ [F]
- Summary: Proposes using an ordinary file-sync service (Dropbox and similar) as the transport for CRDT local-first apps: one file per device, merged locally, so app data outlives any vendor.
- Relevance: C4 and F1–F5. It is prior art for "folders as a sync substrate", although its direction is the reverse (DB over files, not files as a DB replica).

### Building data-centric apps with a reactive relational database (Riffle prelude)
- Geoffrey Litt, Nicholas Schiefer, Johannes Schickling, Daniel Jackson · essay · 2022
- https://riffle.systems/essays/prelude/ [F]
- Summary: Argues for keeping all app state, UI state included, in a local reactive relational database (SQLite) queried declaratively, with sync as a separate concern. It is the intellectual root of LiveStore.
- Relevance: C8 and C4 (relational-first local store as the app's single source of truth).

### Ink & Switch: Cambria, Upwelling, Patchwork notebook, Keyhive notebook, Dispatch 014
- Ink & Switch · research essays/notebooks · 2020 (Cambria), 2023 (Upwelling), 2024–2025 (Patchwork, Keyhive), 2025-11-27 (Dispatch 014)
- https://www.inkandswitch.com/cambria/ ; https://www.inkandswitch.com/upwelling/ ; https://www.inkandswitch.com/patchwork/notebook/ ; https://www.inkandswitch.com/keyhive/notebook/ ; https://www.inkandswitch.com/newsletter/dispatch-014/ [F]
- Summary: Cambria applies bidirectional lenses to translate schema versions between replicas. Upwelling explores private drafts with explicit publishing in collaborative writing. Patchwork covers version control, branches and diffs for local-first documents. The Keyhive notebook covers local-first access control with capabilities, group key agreement and revocation. Dispatch 014 reports progress on Subduction and Keyhive.
- Relevance: Cambria is prior art for F1/F2 (lens laws across schema versions). Keyhive matters for S3 (revocation) and E. Patchwork matters for the deferred branching future work.

### Aaron Boodman: General-purpose sync with IVM (Local-First Conf 2025) and writings
- Aaron Boodman (Rocicorp) · talk + personal site · 2025-05-27
- https://localfirstconf-2025-release.vercel.app/schedule/talks-day-1/general-purpose-sync ; https://aaronboodman.com/ [F-link]
- Summary: Argues that incremental view maintenance (ZQL) makes query-driven partial sync general-purpose, so clients subscribe to queries rather than to predeclared buckets. This was the design rationale for Zero.
- Relevance: C1 and S3 (query-scoped partial replication, and what eviction means under query scopes).

#### Gaps
- No standalone written Boodman essay of 2024–2026 was located. His argument lives mostly in talks and Zero docs.

### F. Conferences, tracks and podcasts

#### Takeaway
These are the venues where 2024–2026 design rationale was actually published: Local-First Conf talks, the FOSDEM 2026 devroom and localfirst.fm. Cite talks with their recording link, not secondary summaries.

#### Cited Findings

### Local-First Conf 2024 / 2025 / 2026 (talk recordings)
- Local-First Conf (Berlin) · conference · 2024-05 (first edition, which Kleppmann blogged about), 2025-05-26..28, 2026 edition (recordings referenced July 2026)
- https://www.localfirstconf.com/ ; https://www.youtube.com/@localfirstconf ; https://martin.kleppmann.com/2024/05/30/local-first-conference.html [F]; per-year URLs such as `/2025` return 404
- Summary: Sync-relevant 2025 talks include Aaron Boodman's "General-purpose sync with IVM", Johannes Schickling's "Sync different: event sourcing in local-first apps", Tuomas Artman's "Building a synchronous experience with asynchronous data: Linear's sync engine", Rasmus Andersson's "Playbit sync", Alex Good's "Beelay, a (reasonably) generic encrypted sync protocol for CRDTs" and a Yjs provider on Cloudflare Workers (Timo Wilhelm). The 2026 talk list was not retrieved.
- Relevance: C1, C6 and E (Beelay) and D1 (Linear). The best primary sources for 2025 design rationale.

### Sync Conf 2025 (San Francisco) recordings
- Local-First/Sync Conf organizers · conference · November 2025 (newsletter dated 2025-12-08)
- https://www.youtube.com/playlist?list=PL4isNRKAwz2NCmk5oQq4qIBXreLLdAAJ_ (from the Buttondown newsletter https://buttondown.com/local-first/archive/recorded-talks-from-sync-conf) [F-link]
- Summary: A sync-engine-focused conference. The first eleven talks were posted, but the talk list was not retrieved.
- Relevance: General C1–C8 context. Pull the specific talks before citing.

### FOSDEM 2026 Local-First, sync engines, CRDTs devroom
- FOSDEM · conference track · 2026-02-01 (23 talks)
- https://archive.fosdem.org/2026/schedule/track/local-first/ [F]
- Summary: Notable talks: "Automerge + Keyhive Design Overview" (Good, Zelenka), "NextGraph: E2EE sync engine, SDK, graph DB, and reactive ORM", "ElectricSQL: Query-driven Sync in TanStack DB" (De Porre), "CRDTs, E2EE, permissions and Jazz!" (Ricci), "Willow – Protocols for an uncertain future" (Gwilym), "Plane's collaborative wiki with Yjs", "Miru: collaborative video editor with offline support", "Get to know PouchDB & CouchDB" and "Composing capability security and CRDTs" (Goblins/OCapN).
- Relevance: C6 and E (NextGraph and Jazz threaten the trust-mode story), C2 (offline video editor and media), and C7.

### localfirst.fm #15: Tuomas Artman (Linear, sync engines)
- localfirst.fm (host Johannes Schickling) · podcast · c. 2024-10-01
- https://www.localfirst.fm/15 [F-link]
- Summary: Linear's CTO on why Linear built a sync engine before product–market fit, how its object graph and sync ids work, and the scaling lessons. The companion talk is "Scaling the Linear Sync Engine".
- Relevance: C1 and D1. Other notable episodes (Boodman, Kleppmann, the Electric and PowerSync founders) exist from memory but were not resolved this session [M].

#### Gaps
- No full talk lists were retrieved for Local-First Conf 2026 or Sync Conf 2025.

## Coverage gaps

#### Takeaway
Most of the requested list was covered. The remaining gaps are mainly ADRs that are not public, recent posts whose canonical URLs are missing, and talk lists that were not retrieved.

#### Cited Findings
- Electric's acquisition post is primary (https://electric.ax/blog/2026/08/11/electric-joining-databricks); no Databricks-side post was found.
- Instant's shutdown is confirmed by the site banner (https://www.instantdb.com/essays/architecture).

#### Inferences
- The catalog is biased toward English-language, US/EU vendors.

#### Gaps
- **Logseq ADRs (including 0017)**: not public; repo ADR paths return 404. Logseq's DB and file-sync design is known only from forum posts.
- **Zero permissions deprecation**: there is no explicit deprecation notice. The 0.18 and 0.23 release notes and the "deprecated" CRUD path are indirect. Read https://zero.rocicorp.dev/llms.txt or the 0.2x release notes in full.
- **Figma "Keeping it 100(x) with real-time data at scale" (c. April 2026)**: only mirrors were found, not the figma.com canonical URL.
- **Jamsocket "You might not need a CRDT"**: the original URL now redirects to Modal, so cite it through the Wayback Machine.
- **Triplit design docs**: return HTTP 410. Nothing primary remains except the Supabase post.
- **WhatsApp syncd internals**: only the whitepaper is primary; there is no engineering post on its conflict model.
- **Talk lists for Local-First Conf 2026 and Sync Conf 2025**: not retrieved.
- **localfirst.fm episodes**: only #15 resolved; the others are memory.
- **Not covered**: Apple File Provider WWDC talk numbers (only the docs page was verified), Google Drive's changes API as a file-sync design, Syncthing's BEP spec (https://docs.syncthing.net was not re-checked), Anytype's any-sync docs, NextGraph docs beyond the FOSDEM listing, Loro's design docs, PartyKit/Cloudflare acquisition post, Realm/Atlas Device Sync deprecation rationale (none was published), Microsoft Dynamics 365 offline sync design, Firebase/Firestore offline internals and Kleppmann's 2024–2026 blog posts on Bluesky and Automerge.
- **No documents found specifying (a) rejection cascades to blobs (S2), (b) blob GC that honors offline holders (S5) or (c) crash and echo laws for a folder replica of a database (F4/F5).** That absence is the appendix's main point. It is "not found in about 200 sources checked", not proof that nothing exists.
