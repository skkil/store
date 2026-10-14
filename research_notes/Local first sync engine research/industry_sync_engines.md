# Industry Sync Engines (offline/local <-> cloud), state as of Nov 2026

Method note: ~17 web searches/fetches on 2026-11-04. Items tagged **[verified 2026-11]** were confirmed in this session. Items tagged **[training, ≤mid-2026]** are from prior knowledge with a stable primary URL and were NOT re-checked this session. The report writer should treat [training] items as lower confidence on status/version claims.

## Q1. Products / frameworks: comparative profiles

### Takeaway
By late 2026 the market has split into (a) server-authoritative "query/shape sync" engines on top of Postgres (Zero, Electric+TanStack DB, PowerSync, InstantDB, Convex), which dominate business-app use cases, and (b) CRDT document engines (Automerge, Yjs, Loro, Jazz, Ditto) used for collaborative content and P2P/edge. Several first-generation BaaS sync products were killed: MongoDB Atlas Device Sync (EOL 2025-09-30), Amplify DataStore (Gen 1 EOL 2027-05-01), old CRDT-based ElectricSQL (rewritten 2024), Triplit (team absorbed into Supabase 2025-10).

### Cited Findings

**Rocicorp: Replicache -> Zero**
- Zero 1.0 shipped mid-2026 (InfoQ, June 2026) after ~2 years of development; 1.0 was a minor step from 0.26.2 with no breaking changes; future breaking changes "expected to be rare". SSR still not shipped. **[verified 2026-11]** — [InfoQ](https://infoq.com/news/2026/06/zero-version-1)
- Zero model: queries written in ZQL (streaming/incremental query engine); run against local cache first, results in next frame, authoritative results synced in background. **[verified 2026-11]** — [InfoQ](https://infoq.com/news/2026/06/zero-version-1)
- Zero server ("zero-cache") consumes Postgres via change data capture / logical replication and keeps a SQLite replica (1.0 notes mention litestream restore and CDC error-handling fixes). **[verified 2026-11]** — [InfoQ](https://infoq.com/news/2026/06/zero-version-1)
- Managed offering "Cloud Zero"; Productlane (Apr 2026) migrated to Zero and said Cloud Zero "greatly reduced operational burden". **[verified 2026-11]** — [Productlane/Speakeasy blog](https://roadmap.speakeasy.com/blog/zero-migration)
- Replicache model: client keeps pending mutations (named mutators); server runs authoritative mutators, client pulls new server state, then rebases (replays) still-pending local mutations on top - the "git-like rebase" pattern. Replicache is now in maintenance and Rocicorp focuses on Zero. **[training, ≤mid-2026]** — [Replicache: How it works](https://doc.replicache.dev/concepts/how-it-works)
- Zero is notably weaker on true offline writes than offline-first tools (it is "instant/optimistic" first); Rocicorp has historically positioned offline writes as limited. **[training — verify]** — [Zero docs](https://zero.rocicorp.dev/docs/introduction)

**ElectricSQL (old vs new)**
- Old ElectricSQL (≤v0.12, 2023–mid-2024): bidirectional, CRDT-based (rich-CRDT on Postgres + SQLite clients, DDLX permissions). Abandoned in July 2024 in favor of a rewrite ("Electric Next"); legacy docs moved to legacy.electric-sql.com. **[training; legacy site existence verified 2026-11]** — [Legacy release notes](https://legacy.electric-sql.com/docs/reference/release_notes); [DDLX/Shapes discussion](https://www.github.com/electric-sql/electric/discussions/624)
- New Electric: read-path-only sync of Postgres subsets ("Shapes") over HTTP, CDN-cacheable; Electric 1.0 GA on 2025-03-17 with stable APIs. **[verified 2026-11]** — [Electric 1.0 released](https://electric.ax/blog/2025/03/17/electricsql-1.0-released)
- Shapes are single-table (subqueries may filter on related data); production apps should request shapes via a backend proxy for authorization. **[verified 2026-11]** — [Shapes guide](https://electric.ax/docs/sync/guides/shapes)
- Write path is delegated: TanStack DB applies optimistic writes, your API writes to Postgres and returns a txid; TanStack DB holds the optimistic state until Electric streams that txid back. **[verified 2026-11]** — [TanStack × Electric partnership](https://tanstack.com/blog/electric-partnership)
- Domain moved to electric.ax; site lists a post "Electric apps get persistence and includes with TanStack DB 0.6" (content not retrieved). **[verified 2026-11, partial]** — [electric.ax](https://electric.ax/sync/)

**TanStack DB**
- Client-side reactive collections + live queries + optimistic mutations, backend-agnostic (Electric, PowerSync, REST, etc.); collection adapters such as the Electric collection. **[verified 2026-11 via Electric/TanStack blog]** — [TanStack blog](https://tanstack.com/blog/electric-partnership)
- Gap: exact 2026 version/1.0 status not verified.

**PowerSync (JourneyApps)**
- Postgres/MongoDB/MySQL source -> PowerSync Service -> client SQLite. Server partitions data into "buckets"; self-hosted service needs a bucket-storage DB (MongoDB or Postgres). Since v1.27.0 default storage version 4 (MongoDB, used by all PowerSync Cloud) / version 2 (Postgres). **[verified 2026-11]** — [Self-host setup](https://docs.powersync.com/self-hosting/installation/powersync-service-setup); [Compatibility](https://docs.powersync.com/sync/advanced/compatibility)
- Partial-replication config evolved from "Sync Rules" to "Sync Streams"; new configs recommended to use `edition: 3` (new compiler, wider SQL). **[verified 2026-11]** — [Sync Streams](https://docs.powersync.com/sync/streams)
- Writes: client-side upload queue; developer's own backend API applies writes to the source DB (server authority via your API), then changes flow back via replication. **[training]** — [PowerSync docs](https://docs.powersync.com)
- Licensing: Open Edition free self-hosted under Functional Source License (FSL); Cloud paid. Listed by MongoDB as a migration target for Device Sync. **[verified 2026-11]** — [Open Edition release](https://powersync.com/blog/powersync-open-edition-release); [MongoDB deprecation page](https://mongodb.com/docs/atlas/app-services/sync/device-sync-deprecation)

**InstantDB**
- Open-sourced Aug 2024. Data model = triples [entity, attribute, value]; all tenants' triples in one big Postgres. Server (Clojure) understands Datalog + InstaQL; client compiles InstaQL to Datalog and evaluates locally against an immutable store. Uses Postgres WAL to detect changes and invalidate queries. Optimistic writes roll back on server rejection. Client cache persisted to IndexedDB / AsyncStorage. Positions itself as "database for the AI era". **[verified 2026-11]** — [GitHub instantdb/instant](https://github.com/instantdb/instant); [About](https://www.instantdb.com/about)
- Authorization via CEL-based permission rules evaluated server-side. **[training]** — [Instant docs](https://www.instantdb.com/docs/permissions)

**Triplit — effectively discontinued as a company**
- 2025-10-08: Triplit co-founder Matt Linkous joined Supabase; Supabase will *not* build Triplit into its platform; he will further open-source Triplit and document patterns. Supabase said offline support "remains an open problem" it is exploring. **[verified 2026-11]** — [Supabase: Triplit joins Supabase](https://supabase.com/blog/triplit-joins-supabase)
- Third-party wiki claims AGPL-3.0 and "transitioned to open-source project" — unverified/low authority. — [clawbot wiki](https://clawbot.ai/wiki/applications/triplit-full-stack-local-first.html)

**Supabase**
- No first-party offline sync as of the Oct 2025 announcement; relies on partners (PowerSync, Electric, sqlite.ai sync to Supabase). **[verified 2026-11]** — [Supabase blog](https://supabase.com/blog/triplit-joins-supabase); [sqlite-sync](https://github.com/sqliteai/sqlite-sync-dev)

**Convex**
- Server-authoritative reactive DB: queries/mutations are deterministic TS functions run on server; clients subscribe to query results; optimistic updates on client. Not offline-first; Convex has published thinking on an "object sync engine" for local-first. Backend open-sourced (FSL) 2024. **[training]** — [Convex Stack: object sync engine](https://stack.convex.dev/object-sync-engine)

**Jazz (Garden Computing)**
- CRDT "CoValues" (CoMap, CoList, CoFeed, etc.) with built-in groups/permissions using cryptographic signing/encryption; open-source sync server (`npx jazz-run sync`, SQLite persistence, port 4200) or Jazz Cloud. **[verified 2026-11]** — [Sync and storage](https://jazz.tools/docs/sync-and-storage); [How sync works](https://jazz.tools/docs/concepts/how-sync-works)
- Pricing (snapshot, date unconfirmed): Starter free (100 MAU, 10 GB storage, 0.2 GB egress), Indie $4/mo, Pro from $19/mo metered ($0.02/GB storage, $1/GB egress), enterprise on-prem. A "classic.jazz.tools" site exists, implying a major new version superseding "classic" Jazz. **[verified 2026-11; version name not confirmed]** — [Jazz pricing](https://jazz.tools/pricing); [classic pricing](https://classic.jazz.tools/pricing)

**LiveStore**
- Event-sourcing: all writes are immutable ordered events, synced via git-inspired push/pull, materialized into reactive local SQLite; explicit read/write model separation, contrasted with CRDT systems. Docs themselves recommend Jazz/Instant for batteries-included auth+storage. **[verified 2026-11]** — [How LiveStore works](https://docs.livestore.dev/evaluation/how-livestore-works); [When to use LiveStore](https://docs.livestore.dev/overview/when-livestore/)
- Gap: no confirmed 1.0 / 2026 release found.

**Evolu**
- Local-first TS library: SQLite in browser, end-to-end encrypted sync to relays, CRDT-style (LWW + Merkle-tree based diffing, derived from James Long's Actual approach). **[training]** — [evolu.dev](https://www.evolu.dev)

**Ditto**
- P2P mesh (Bluetooth LE, LAN, Wi-Fi P2P) "Small Peers" + "Big Peer" (cloud/on-prem server storing CRDTs, WebSocket sync, typically greedily syncs all data). Delta-state CRDTs. $82M Series B 2025-03-12; ARR +250% in prior year. Listed as MongoDB Device Sync migration target. Proprietary/commercial. **[verified 2026-11]** — [SiliconANGLE](https://siliconangle.com/2025/03/12/ditto-raises-82m-funding-edge-database/); [Big Peer architecture](https://docs.ditto.live/v4-4/big-peer-architecture); [Delta-state CRDTs](https://www.ditto.com/blog/dittos-delta-state-crdts)

**Couchbase Lite + Sync Gateway / Capella App Services; CouchDB/PouchDB**
- CouchDB replication protocol: per-document revision trees, MVCC, changes feed + checkpoints; conflicts kept as sibling revisions with deterministic winner, app resolves. PouchDB implements it in JS. Couchbase Sync Gateway adds channels (partial replication) + sync function (authz). Mature, active. **[training]** — [CouchDB replication protocol](https://docs.couchdb.org/en/stable/replication/protocol.html); [Sync Gateway](https://docs.couchbase.com/sync-gateway/current/index.html)

**MongoDB Realm / Atlas Device Sync — DEPRECATED, EOL**
- Deprecated Sept 2024; EOL/removal 2025-09-30. Also deprecated: App Services Auth/User Management, Data Access Rules/Roles, Device SDKs' sync. Triggers & Functions continue. Listed alternatives: Ditto, PowerSync, ObjectBox, AWS AppSync, HiveMQ, Ably, Parse, custom. **[verified 2026-11]** — [Device Sync deprecation](https://mongodb.com/docs/atlas/app-services/sync/device-sync-deprecation); [App Services deprecation](https://mongodb.com/docs/atlas/app-services/deprecation); [Community forum](https://mongodb.com/community/forums/t/atlas-device-sync-end-of-life-and-deprecation/296687)
- Why: MongoDB gave no detailed technical rationale in the docs found; Realm local DB remains as community open source. Gap — see Gaps.

**Firebase / Firestore offline**
- Client SDK cache with offline persistence (IndexedDB on web), latency-compensated local writes queued and replayed; server is authoritative, LWW at document/field level; Security Rules for authz. Active. **[training]** — [Firestore offline](https://firebase.google.com/docs/firestore/manage-data/enable-offline)

**AWS AppSync / Amplify DataStore — DEPRECATING**
- Amplify Gen 1 in maintenance mode; EOL 2027-05-01. DataStore not part of Gen 2. Migration guidance: treat AppSync as single source of truth; push unsynced local data, clear local store, re-sync; AppSync remains the authz authority. **[verified 2026-11]** — [Amplify migrate-from-DataStore](https://docs.amplify.aws/gen1/flutter/build-a-backend/more-features/datastore/migrate-from-datastore/authorization-and-data/)
- DataStore used versioned optimistic concurrency + "Auto Merge"/custom conflict handlers via AppSync delta tables. **[training]** — [Amplify DataStore conflict resolution (Gen1 docs)](https://docs.amplify.aws/gen1/flutter/prev/build-a-backend/more-features/datastore/schema-updates)

**cr-sqlite / vlcn**
- SQLite extension making tables CRRs (column-level LWW/causal length CRDT); author Matt Wonlaw joined Rocicorp; project largely inactive since ~2024. **[training — status not re-verified]** — [vlcn-io/cr-sqlite](https://github.com/vlcn-io/cr-sqlite)

**SQLite Sync (sqlite.ai)**
- SQLite extension, CRDT-based (Causal-Length Set, Delete-Wins, Add-Wins, Grow-Only Set; newer block-level LWW for text/markdown lines). Syncs to SQLite Cloud, PostgreSQL, Supabase. Usage: `cloudsync_init('table')`, normal SQL, `cloudsync_network_sync()`. Binaries for Linux/macOS/Windows/Android/iOS/WASM; Swift/Kotlin/Flutter/Expo/RN packages. Pitched at offline-first and AI agents. License: npm lists "other"/custom — check. Version ambiguous (0.8.x vs 1.0.0). **[verified 2026-11, mostly via mirrors]** — [sqliteai/sqlite-sync-dev](https://github.com/sqliteai/sqlite-sync-dev); [ecosyste.ms listing](https://awesome.ecosyste.ms/projects/github.com%2Fsqliteai%2Fsqlite-sync)

**Turso / libSQL**
- Old: libSQL embedded replicas (`offline: true`) — read replicas + experimental offline writes. New: Turso Sync (`@tursodatabase/sync`) with explicit `push()`/`pull()`, row-level logical logging, default Last-Push-Wins; public beta "not yet recommended for production", no durability guarantees during beta; OIDC auth only Clerk/Auth0 in beta. **[verified 2026-11]** — [Turso offline sync public beta](https://turso.tech/blog/turso-offline-sync-public-beta); [Turso Sync docs](https://docs.turso.tech/sync/usage)

**Automerge / automerge-repo**
- JSON-like CRDT (Rust core, WASM/JS bindings) with full history; automerge-repo adds networking/storage adapters and a simple sync server (relay). Ink & Switch origin. **[training]** — [automerge.org](https://automerge.org); [automerge-repo](https://github.com/automerge/automerge-repo)

**Yjs ecosystem: y-sweet, Liveblocks, PartyKit**
- Yjs: YATA-based sequence CRDT, de facto standard for collaborative text editors (ProseMirror/TipTap/Lexical bindings). **[training]** — [yjs/yjs](https://github.com/yjs/yjs)
- y-sweet (Jamsocket): Yjs server persisting to S3. Jamsocket also argues "you might not need a CRDT" when you have a central server. **[verified 2026-11 (blog exists)]** — [Jamsocket blog](https://jamsocket.xyz/blog/you-might-not-need-a-crdt)
- PartyKit acquired by Cloudflare (Apr 2024); runs on Durable Objects. **[training]** — [Cloudflare blog](https://blog.cloudflare.com/cloudflare-acquires-partykit/)
- Liveblocks: hosted multiplayer infra (Yjs + own storage, presence, comments); published sync-engine comparisons (Figma/Linear/Google Docs). **[verified 2026-11]** — [Liveblocks: understanding sync engines](https://development.liveblocks.io/blog/understanding-sync-engines-how-figma-linear-and-google-docs-work.md)

**Loro**
- Rust CRDT library (Fugue-based text, movable tree, list), 1.0 in late 2024; high-performance alternative to Automerge/Yjs. **[training]** — [loro.dev](https://loro.dev)

**TinyBase / RxDB / WatermelonDB / Legend-State**
- TinyBase: reactive store with built-in MergeableStore (CRDT-like) + synchronizers (WebSocket, BroadcastChannel). **[training]** — [tinybase.org](https://tinybase.org)
- RxDB: NoSQL local DB with a generic pull/push+checkpoint replication protocol pluggable to any backend; markets itself as replacement for DataStore/Realm. **[verified 2026-11 (alt. page exists)]** — [RxDB replication](https://rxdb.info/replication.html); [RxDB vs DataStore](https://rxdb.info/articles/alternatives/aws-amplify-datastore-alternative.html)
- WatermelonDB: RN SQLite lazy-loading DB; sync protocol = pullChanges(lastPulledAt)/pushChanges, you implement the backend. **[training]** — [WatermelonDB sync](https://watermelondb.dev/docs/Sync/Intro)
- Legend-State: signal-based state with sync plugins (Supabase, Keel, CRUD) and local persistence. **[training]** — [Legend-State](https://legendapp.com/open-source/state/)

### Inferences
- Commercial survivors are those that sit *beside* an existing database (Postgres/Mongo) rather than *being* the database (Realm, DataStore, Triplit died; PowerSync, Electric, Zero thrive).
- Licensing trend: FSL / source-available self-host + paid managed cloud (PowerSync, Convex, Zero cloud), or MIT libraries + hosted sync (Jazz, Electric Cloud).

### Gaps
- MongoDB's internal rationale for killing Device Sync not found in primary sources (commonly attributed in community discussion to low adoption vs. cost — unverified).
- Zero's current offline-write story, TanStack DB 1.0 status, LiveStore/Jazz 2026 versions, cr-sqlite status: not verified this session.

## Q2. In-house engines

### Takeaway
The best-known in-house engines (Linear, Figma, Notion, Google Docs) are server-authoritative with a central ordering point; CRDTs are used selectively (Notion offline pages, Actual Budget) rather than as the global model.

### Cited Findings
- **Linear**: workspace has an ordered log of sync actions; clients filter by permissions/subscriptions; client sends checkpoint (last applied sync id) and receives only newer deltas. **[verified 2026-11]** — [Linear: rebuilding delta sync read path](https://linear.app/blog/rebuilding-delta-sync-read-path); [Scaling the Linear Sync Engine](https://linear.app/blog/scaling-the-linear-sync-engine)
- Linear (reverse-engineered by wzhudev): client bootstraps (full/partial), then opens WebSocket for "delta packets"; client mutations are batched into transactions executed by server; each action has a sync id enabling gap detection; MobX object pool + IndexedDB locally. **[verified 2026-11, excerpts]** — [reverse-linear-sync-engine](https://github.com/wzhudev/reverse-linear-sync-engine)
- **Figma (2019)**: server is central authority per document (dedicated process per live doc); per-property last-writer-wins with server defining order (no timestamps); "inspired by CRDTs" but not a true CRDT; client-generated unique IDs. **[verified 2026-11]** — [Figma blog](https://www.figma.com/blog/how-figmas-multiplayer-technology-works/)
- **Notion offline (shipped ~Aug 2025; engineering post 2025-12-11)**: SQLite cache promoted to persistent store; `offline_action` table records *why* each page is offline (toggled/favorite/recent/inherited) - page evicted when no reasons remain; server pushes per-page channel notifications instead of polling; reconnect compares `lastDownloadedTimestamp` vs `lastUpdatedTime`; offline pages migrated to Notion's newer CRDT data model; databases cap at 50 pages of current view; partially loaded pages are not shown. Desktop/mobile only, not web. **[verified 2026-11]** — [Notion: How we made Notion available offline](https://www.notion.com/en-gb/blog/how-we-made-notion-available-offline); launch date per [2sync](https://2sync.com/blog/how-to-use-notion-offline) (secondary)
- **Apple CloudKit / NSPersistentCloudKitContainer**: mirrors Core Data/SwiftData to CloudKit private/shared DBs, record-level LWW, schema must be additive-only once in production. **[training]** — [Apple docs](https://developer.apple.com/documentation/coredata/mirroring_a_core_data_store_with_cloudkit)
- **Dropbox Nucleus (2020)**: Rust rewrite of desktop sync; three trees (remote, local, synced/"merge base") and a planner; heavy deterministic randomized testing (CanopyCheck, Trinity). **[training]** — [Rewriting the heart of our sync engine](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine); [Testing our new sync engine](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)
- **Google Docs**: Operational Transformation with central server (Jupiter-style). **[training]** — [Google Drive blog 2010](https://drive.googleblog.com/2010/09/whats-different-about-new-google-docs_22.html)
- **Excalidraw**: E2E-encrypted relay; element-level versioning with `version`/`versionNonce` reconciliation (higher version wins, nonce tiebreak). **[training]** — [Excalidraw P2P blog](https://plus.excalidraw.com/blog/building-excalidraw-p2p-collaboration-feature)
- **tldraw sync**: server-authoritative record store per room (Cloudflare Durable Objects in their reference), clients send diffs, server rebroadcasts/rejects; not CRDT. **[training]** — [tldraw sync docs](https://tldraw.dev/docs/sync)
- **Actual Budget**: hybrid logical clocks + LWW-per-cell CRDT messages over SQLite, Merkle trie to find divergence; E2E encryption optional. **[training]** — [Actual docs](https://actualbudget.org/docs/)
- **Obsidian Sync**: file-level E2E encrypted sync with version history; markdown conflicts merged via diff-match-patch style 3-way merge. **[training — merge detail unverified]** — [Obsidian Sync help](https://help.obsidian.md/sync)

### Inferences
- Common in-house recipe: ordered server log + monotonically increasing sync id/checkpoint + client object pool + optimistic transactions. This is directly reusable as a thesis baseline.

### Gaps
- Figma's newer (2024+) multiplayer/"LiveGraph" internals and Linear's 2025–26 changes beyond delta read path not fetched.

## Q3. Recurring architectural patterns; which is winning

### Takeaway
Server-authoritative sync (CDC read path + optimistic mutations reconciled by server, i.e., Replicache/Zero/Linear model) is winning for business apps because it preserves existing Postgres, authorization, and invariants; CRDTs win only for rich collaborative content and P2P/edge.

### Cited Findings
- Patterns observed:
  1. **Server-authoritative rebase** — Replicache/Zero, Linear, tldraw. — [Replicache](https://doc.replicache.dev/concepts/how-it-works); [Linear](https://linear.app/blog/rebuilding-delta-sync-read-path)
  2. **CDC/logical-replication read path + separate write API** — Electric, PowerSync, Zero, InstantDB (WAL). — [Electric 1.0](https://electric.ax/blog/2025/03/17/electricsql-1.0-released); [InstantDB](https://github.com/instantdb/instant)
  3. **Query/shape-based partial sync** — Electric Shapes, PowerSync buckets/Sync Streams, Zero ZQL queries, Couchbase channels. — [Shapes](https://electric.ax/docs/sync/guides/shapes); [Sync Streams](https://docs.powersync.com/sync/streams)
  4. **CRDT (P2P or relay)** — Automerge, Yjs, Loro, Jazz, Ditto, sqlite-sync, Actual. — [Ditto](https://www.ditto.com/blog/dittos-delta-state-crdts)
  5. **Event sourcing** — LiveStore. — [LiveStore](https://docs.livestore.dev/evaluation/how-livestore-works)
  6. **LWW row/document sync** — Firestore, Turso (Last-Push-Wins), CloudKit. — [Turso](https://docs.turso.tech/sync/usage)
- ElectricSQL itself abandoned a CRDT bidirectional design for read-path-only + app-owned writes (2024). — [Legacy notes](https://legacy.electric-sql.com/docs/reference/release_notes)
- Central authority lets you drop CRDT overhead (Figma; Jamsocket "you might not need a CRDT"). — [Figma](https://www.figma.com/blog/how-figmas-multiplayer-technology-works/); [Jamsocket](https://jamsocket.xyz/blog/you-might-not-need-a-crdt)
- Even CRDT-adopting Notion uses CRDTs only for offline pages inside an otherwise server-centric system. — [Notion](https://www.notion.com/en-gb/blog/how-we-made-notion-available-offline)

### Inferences
- A reusable engine should: separate read path (subscribable queries/shapes, CDC) from write path (named mutators executed authoritatively), and offer pluggable CRDT field types (text, list) for collaborative fields. This hybrid is where Zero+Yjs, Notion, Electric+Yjs setups converge.

### Gaps
- No quantitative adoption data (downloads/MAU) comparing patterns was gathered.

## Q4. Pain points reported by industry

### Takeaway
Authorization over partial replicas, schema evolution across stale clients, and choosing what subset to sync are the dominant pain points; conflicts are less painful than expected when a server is authoritative.

### Cited Findings
- **Authorization**: Electric requires a proxy backend to authorize shapes; MongoDB had to deprecate Rules/Roles alongside sync; Amplify says AppSync remains authz authority during migration; Linear filters sync log by permissions server-side. — [Electric shapes](https://electric.ax/docs/sync/guides/shapes); [MongoDB](https://mongodb.com/docs/atlas/app-services/deprecation); [Amplify](https://docs.amplify.aws/gen1/flutter/build-a-backend/more-features/datastore/migrate-from-datastore/authorization-and-data/); [Linear](https://linear.app/blog/rebuilding-delta-sync-read-path)
- **Partial sync / what's offline**: Notion built an explicit reason-tracking table and capped DB downloads at 50 rows; refuses to render partially-synced pages. — [Notion](https://www.notion.com/en-gb/blog/how-we-made-notion-available-offline)
- **Schema migrations**: Amplify DataStore schema update docs (destructive local re-sync); CloudKit additive-only production schema; PowerSync storage/edition versioning. — [Amplify schema updates](https://docs.amplify.aws/gen1/flutter/prev/build-a-backend/more-features/datastore/schema-updates); [PowerSync compatibility](https://docs.powersync.com/sync/advanced/compatibility)
- **Durability/maturity**: Turso Sync beta warns of possible data loss; conflict resolution not implemented at public beta launch. — [Turso](https://turso.tech/blog/turso-offline-sync-public-beta)
- **Vendor risk**: Realm/Device Sync EOL, DataStore EOL, Triplit shutdown forced migrations. — [MongoDB](https://mongodb.com/docs/atlas/app-services/sync/device-sync-deprecation); [Supabase/Triplit](https://supabase.com/blog/triplit-joins-supabase)
- **Testing**: Dropbox credits deterministic randomized testing as essential to Nucleus. **[training]** — [Dropbox testing](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)
- **Ops burden**: Productlane cites Cloud Zero removing operational burden. — [Productlane](https://roadmap.speakeasy.com/blog/zero-migration)

### Inferences
- Thesis/product differentiation opportunities: declarative authz that compiles to both read filters and write validation; versioned mutators/schema with stale-client handling; blob/attachment sync (rarely solved first-party — PowerSync and others handle via separate attachment queues [training]).

### Gaps
- Blobs/files sync handling across products and large-dataset benchmarks not researched this session; HN/localfirst.fm opinion sources not fetched.
