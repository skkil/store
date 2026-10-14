# Competitor feature matrix: cross-domain sync SDK (rows + files + folders + validator + optional E2EE), as of 2026-11-04

Tag legend: **[F]** = confirmed this session from a fetched/search-returned source (dated where possible). **[M]** = from model knowledge (training cutoff ~mid-2026), not re-verified this session; the URL given is the canonical doc to check. Treat every [M] cell as "probably right, verify before publishing". **DEPRECATED / SHUTTING DOWN** flagged in caps.

Abbreviations for the five thesis capabilities used in KQ1: **R** = relational + multi-row transactions (up and down), **B** = files/blobs, **F** = two-way user filesystem folder sync, **E** = optional (not mandatory) E2EE, **V** = server-side validation of writes.

## Comparison matrix and per-product notes

### Takeaway
The market splits into row-sync engines (PowerSync, Zero, Electric, Convex, InstantDB, SQLite Sync), document/edge sync (Ditto, Couchbase, Firestore, ObjectBox), CRDT doc libraries (Automerge, Yjs, Loro), E2EE-mandatory personal-data stacks (Evolu, Anytype, Etebase, Jazz 1.x), and file synchronizers (Syncthing, rclone, Dropbox/Drive). In 2026 there was major consolidation: Electric was acquired by Databricks (Aug 2026, hosted cloud shutting), InstantDB's team joined OpenAI (Aug 2026, cloud shuts 2027-08-31), Triplit's founder joined Supabase (Oct 2025), MongoDB Device Sync reached end of life (Sep 2025). Zero hit GA (Mar 2026).

### Cited Findings

#### Matrix A: row and document sync engines

| Product | Data model | Local store | Server DB (BYO Postgres?) | Offline writes | Multi-row tx up / down | Conflict model | Server validation | E2EE | Files/blobs | FS folder sync | CRDT/collab | Partial sync | Clients | Self-host | License | Pricing | Company/funding | Status |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **PowerSync** | Relational (SQLite client ↔ Postgres/MongoDB/MySQL/SQL Server) [F] | SQLite | BYO Postgres, MongoDB, MySQL, SQL Server [F] | Yes (local SQLite + upload queue) [M] | Up: one local tx at a time via `getNextCrudTransaction` [F, from report]; Down: checkpoints, atomic [F, from report] | Server-authoritative; your backend applies writes, client rebases [M] | Yes, your own backend upload handler; 2xx+reject pattern recommended [F, from report] | None built in (DIY encrypted columns) [M] | Attachments helper library: JS GA/stable, Dart; files stored in your S3/Supabase [F] | No | No (text CRDT only via app) [M] | Yes (sync rules / sync streams) [M] | TS/JS (web, RN, Node), Dart/Flutter, Kotlin MP, Swift, .NET; Rust SQLite core [M] | Yes, Open Edition (beta label, "feature-equivalent with Cloud") [F] | Service: FSL (source-available); SDKs Apache-2.0 [F] | Cloud free tier + paid tiers [M] | Spun out of JourneyApps 2022; parent raised ~$7.5M per CB Insights (unverified); no PowerSync-specific round found [F] | GA (Cloud); Open Edition beta [F] |
| **Electric** (+ TanStack DB, Durable Streams) | Postgres "shapes" (table + where) streamed to client [M] | Any (in-memory, PGlite, TanStack DB collections) [M] | BYO Postgres (read path) [M] | Via TanStack DB optimistic mutations + your API [M] | Up: your API; Down: per-shape log, cross-shape atomicity weak [M] | Server-authoritative, write path DIY [M] | Yes, your API [M] | None [M] | None built in [M] | No | Yjs provider over Durable Streams proposed (PR, Mar 2026, open) [F] | Yes (shapes) [M] | TS primary; Durable Streams clients TS, Go, Python; Rust server [F] | Yes (OSS) [F] | Apache-2.0 [F] | Hosted service **SHUTTING DOWN** after acquisition [F] | **Acquired by Databricks, announced 2026-08-11**, team into Neon/Lakebase group; OSS (PGlite, sync, Durable Streams, TanStack DB) to continue [F] | Sync GA (2025) [M]; Durable Streams announced 2025-12-09, 0.1.0 2025-12-23, hosted 2026-01-22 [F] |
| **Zero** (Rocicorp; Replicache predecessor) | Relational queries (ZQL) over Postgres replica [M] | IndexedDB/SQLite client cache [M] | BYO Postgres (logical replication) [M] | Yes, optimistic; limited offline duration [M] | Up: each custom mutator runs in one server DB tx [F, from report]; Down: atomic pokes [M] | Server-authoritative rebase (Replicache model) [M] | Yes, server re-runs custom mutators [F, from report] | None [M] | None [M] | No | No [M] | Yes (query-driven) [M] | TS (React, Solid, React Native) [M] | Yes (zero-cache) [M] | Apache-2.0 [M] | Cloud Zero "full public availability in 2026" [F] | Rocicorp, small, privately funded [M] | **GA as of March 2026 ("Zero to 1.0")** [F]; Replicache in maintenance [M] |
| **InstantDB** | Triple store with relational queries (InstaQL) [M] | IndexedDB [M] | Instant's own Postgres-backed server; not BYO [M] | Yes [M] | `transact` atomic [M] | Server-authoritative [M] | Permission rules (CEL-like) [M] | None [M] | Storage (files) [M] | No | Presence/topics [M] | Yes (queries) [M] | TS (React, RN, vanilla), admin SDK [M] | Yes (OSS) [F] | Apache-2.0 [M] | Cloud refunds for subs after 2026-07-31 [F] | YC S22; $3.4M seed [F]; Series A amount conflicting ($16M vs $18M) [F]; **team joined OpenAI 2026-08-22** [F] | **CLOUD SHUTTING DOWN: apps supported to 2027-08-31, backups to 2028-08-31** [F] |
| **Convex** | Documents with relational-style queries; server functions [M] | Client cache; offline limited [M] | Convex's own DB; not BYO Postgres (self-host can use Postgres/SQLite as backing store) [M] | Limited (optimistic updates, not durable offline queue) [M] | Yes, mutations are serializable server tx (online) [M] | Server-authoritative OCC [M] | Yes (mutations are TS server code) [M] | None [M] | File storage built in [M] | No | Via components (ProseMirror sync) [M] | Query subscriptions [M] | TS, Python, Rust, Swift, Kotlin [M] | Yes (open-source backend, 2025) [M] | FSL-1.1-Apache-2.0 [M] | Free + Pro per-seat [M] | VC-backed (a16z-led Series A, 2022) [M] | GA [M] |
| **Jazz** (Garden Computing) | 1.x: CoValues (CRDT maps/lists/streams); 2.0: "local-first relational database" [F] | IndexedDB / SQLite [M] | Jazz sync server / Jazz Cloud; not BYO Postgres [M] | Yes [M] | Per-CoValue; no multi-object atomic [M] | CRDT [M] | Server workers; permissions enforced cryptographically on clients [M] | 1.x: encrypted + signed by default (effectively mandatory) [M] | FileStream / binary CoValues [M] | No | Yes (CoText, rich text) [M] | Yes (lazy per-CoValue load) [M] | TS (React, RN, Svelte, Node) [F] | Yes (sync server OSS) [M] | MIT [M] | Jazz Cloud free + paid [M] | Garden Computing, Inc. [F]; funding not verified | **2.0 alpha, new API, no releases published** [F]; 1.x "Classic" |
| **Evolu** | Relational SQLite with Kysely; CRDT-replicated rows [M] | SQLite (wasm/native) [M] | Relay only, no app DB [M] | Yes [M] | Per-field LWW messages; no cross-row atomic up/down guarantee [M] | LWW per column, HLC timestamps [M] | None (server sees ciphertext) [M] | **Mandatory** [M] | None [M] | No | No [M] | No (whole owner) [M] | TS (web, React Native/Expo) [M] | Yes (relay) [M] | MIT [M] | Free OSS [M] | Indie (Daniel Steigerwald) [M] | Active [M] |
| **LiveStore** | Event-sourced; events materialized into SQLite [M] | SQLite (wasm/native) [M] | Pluggable sync backend (Cloudflare DO, Electric, S2) [M] | Yes [M] | Each event atomic; global ordered eventlog (git-like rebase) [M] | Rebase on total order; deterministic materializers replayed on clients [M] | Not central; validation is client-side replay of materializers [M] | None built in (encrypt events DIY) [M] | None [M] | No | No [M] | Per store; no intra-store partial [M] | TS (web, Expo, Node) [M] | Yes [M] | Apache-2.0 [M] | Free OSS [M] | Indie (Johannes Schickling), sponsor-funded [M] | Beta / pre-1.0 [M] |
| **Triplit** | Triple store, relational queries [M] | IndexedDB/SQLite [M] | Own server (SQLite) [M] | Yes [M] | Client tx [M] | Attribute LWW [M] | Schema + permissions [M] | None [M] | None [M] | No | No [M] | Yes [M] | TS [M] | Yes [M] | License conflicting (AGPL vs MIT in directories) [F] | — | **Co-founder Matt Linkous joined Supabase 2025-10-08; Supabase will not build Triplit into its platform** [F] | **Effectively community/OSS-only; no active company** [F, partly third-party] |
| **SQLite Sync** (sqlite.ai) | SQLite tables replicated by CRDT extension [F] | SQLite | SQLite Cloud, **any Postgres or Supabase** (since 1.0.0, 2026-03-24) [F] | Yes [F] | No cross-row atomic guarantee documented (per-change CRDT) [M] | CRDT: causal-length set, delete-wins, add-wins, G-set; block-level LWW for text [F] | RLS/ACL on SQLite Cloud [F] | None [M] | None [M] | No | Line-level text merge [F] | Unknown | Any SQLite host via C extension; packages for Swift, Android, Flutter, Expo, WASM [M] | Yes (Postgres extension) [F] | Elastic License 2.0? (unverified) [M] | SQLite Cloud free tier for 3 devices [F] | SQLite Cloud, Inc. [M] | 1.0 (Mar 2026) [F] |
| **Turso sync** | SQLite (libSQL/Turso rewrite) whole-db replication [F] | SQLite file | Turso Cloud only [F] | Yes (explicit `push()`/`pull()`) [F] | Pushes logical statements; "last push wins" [F] | Last push wins [F] | None [M] | Encryption at rest [M] | None | No | No | No (whole db) [M] | TS, Python, Go [F]; Rust [M] | Turso DB OSS; sync server cloud [M] | MIT (Turso) [M] | Free + paid [M] | Turso (ChiselStrike), VC-backed [M] | **Beta ("not recommended for production" in beta post); no GA found** [F] |
| **Graft** (orbitinghail) | Transactional page storage on object storage; SQLite VFS [F] | SQLite pages | Object storage (S3); not Postgres [F] | Yes [M] | Page-level snapshot isolation; whole-db tx [M] | Server-serialized log; rebase/reject [M] | None [M] | None [M] | N/A | No | No | **Yes, lazy partial page replication** [F] | Rust, SQLite extension [F] | Yes [M] | Apache-2.0/MIT [M] | Free | Indie (Carl Sverre) [F] | **Alpha; last crate release 2025-06-20** [F] |
| **cr-sqlite** | CRDT SQLite extension (CRRs) [M] | SQLite | Peer/any [M] | Yes | Tx spread over `db_version`s; not atomic on receive [F, from report] | Per-column LWW / causal length [M] | None | None | None | No | No | No | C/Rust ext, JS wasm [M] | Yes | MIT [M] | Free | Author moved on (Rocicorp) [M] | **Stalled / unmaintained since ~2024** [M] |
| **Ditto** | JSON documents (DQL) [M] | Embedded store [M] | Ditto Big Peer (cloud/self-host); **MongoDB Connector** to replace Atlas Device Sync [F] | Yes, incl. mesh P2P (BLE, LAN, Wi-Fi Direct) [F] | Local tx; sync per-document [M] | CRDT documents [M] | Auth webhooks/permissions, not arbitrary logic [M] | None app-level [M] | Attachments [M] | No | CRDT counters/registers [M] | Yes (subscriptions) [M] | Swift, Kotlin/Java, JS, Flutter, RN, C#, C++, Rust [M] | Big Peer on-prem (enterprise) [M] | Proprietary [M] | Enterprise, custom [M] | **$82M Series B (Mar 2025) at $462M post; $136M total** [F] | GA [F] |
| **Couchbase Lite + Capella App Services / Sync Gateway** | JSON documents (SQL++) [M] | Couchbase Lite [M] | Couchbase Server/Capella only [M] | Yes [M] | Local batch; per-document replication [M] | Revision tree, custom resolver [M] | Yes, JS sync function [M] | None app-level [M] | Blobs [M] | No | No | Yes (channels) [M] | Swift/ObjC, Kotlin/Java, C#, C, JS [M] | Yes (Sync Gateway) [M] | CE Apache-2.0 / EE commercial [M] | Commercial [M] | **Taken private by Haveli Investments, $1.5B, closed 2025-09-24** [F] | GA [F] |
| **ObjectBox Sync** | Objects (embedded object DB) [M] | ObjectBox [M] | ObjectBox Sync Server; MongoDB connector [M] | Yes [M] | Local tx; sync tx-based [M] | Server-authoritative [M] | Limited [M] | None [M] | No dedicated [M] | No | No | Sync filters [M] | Java/Kotlin, Swift, Dart/Flutter, C/C++, Go, Python [M] | Yes (commercial) [M] | Bindings Apache-2.0, core/sync proprietary [M] | Commercial [M] | ObjectBox GmbH [M] | GA [M] |
| **MongoDB Atlas Device Sync / Realm** | Objects [M] | Realm [M] | Atlas only [M] | Yes | Local tx [M] | OT-ish server merge [M] | Rules [M] | None | No | No | No | Flexible sync [M] | Swift, Kotlin, JS, .NET, Flutter [M] | No | Realm SDK Apache-2.0 [M] | — | MongoDB; migration partners Ditto [F], PowerSync (Oct 2024 partnership) [F] | **DEPRECATED Sep 2024, END OF LIFE 2025-09-30** [M] |
| **Firebase / Firestore** | Documents/collections [M] | Persistent cache [M] | Firestore only (Data Connect = Postgres, separate) [M] | Yes [M] | Batched writes atomic offline; transactions online only [M] | LWW per document/field [M] | Security Rules (declarative) + Functions [M] | None | Cloud Storage for Firebase [M] | No | No | Query-based cache [M] | Web/JS, Swift, Kotlin, Flutter, Unity, C++ [M] | No (emulator only) [M] | Proprietary (SDKs Apache-2.0) [M] | Spark free / Blaze PAYG [M] | Google | GA |
| **Supabase** (Postgres, Realtime, Storage) | Relational Postgres [M] | None (no offline sync engine) [M] | Is Postgres [M] | No native [M] | Server tx only [M] | N/A | RLS + Edge Functions [M] | None | Storage (S3-compatible) [M] | No | Realtime broadcast/presence [M] | N/A | JS, Flutter, Swift, Kotlin, Python, C# [M] | Yes [M] | Apache-2.0 [M] | Free / Pro $25/mo [M] | Large VC rounds 2025 (multi-$B valuation) [M]; hired Triplit founder [F] | GA |
| **Appwrite** | Documents/tables [M] | None native offline sync [M] | Own (MariaDB) [M] | No [M] | — | — | Permissions + Functions [M] | None | Storage [M] | No | Realtime | — | Many incl. Flutter, Swift, Kotlin [M] | Yes [M] | BSD-3 [M] | Free / Pro [M] | VC-backed [M] | GA |
| **AWS AppSync / Amplify DataStore** | GraphQL models [M] | IndexedDB/SQLite [M] | DynamoDB (AppSync resolvers can hit RDS) [M] | Yes (DataStore) [M] | Per-model, no multi-row atomic [M] | Auto-merge / optimistic versioning / custom Lambda [M] | Yes (resolvers/Lambda) [M] | None | Amplify Storage (S3) [M] | No | No | Selective sync [M] | JS, Swift, Android, Flutter [M] | No | Apache-2.0 libs [M] | AWS PAYG | AWS | DataStore de-emphasized in Amplify Gen 2 (maintenance) [M] |
| **Apple CloudKit / SwiftData** | Records in zones [M] | Core Data / SwiftData [M] | iCloud only [M] | Yes [M] | Atomic zone saves [M] | Server change tags; app resolves [M] | None custom [M] | **Optional per-field `encryptedValues`** [M] | CKAsset [M] | No (iCloud Drive is separate) | No | Zones / subscriptions [M] | Swift, CloudKit JS [M] | No | Proprietary | Free within iCloud quotas [M] | Apple | GA |
| **Dexie Cloud** | IndexedDB tables [M] | IndexedDB | Dexie Cloud server (Postgres-backed), not BYO [M] | Yes | Consistent ops on server [M] | Server-side operation merge [M] | Access realms/roles [M] | None [M] | Blob support uncertain | No | Y.js integration (2025) [M] | Realms [M] | TS [M] | Yes (license) [M] | Dexie Apache-2.0; Cloud commercial [M] | Free + per-user [M] | Awarica AB (indie) [M] | GA |
| **RxDB** | NoSQL JSON docs [M] | IndexedDB/SQLite/OPFS [M] | Any via replication plugins (Postgres DIY) [M] | Yes | No | Custom handler [M] | DIY endpoint [M] | Optional encryption plugin (at-rest, premium) [M] | Attachments [M] | No | No | Query replication [M] | TS (Flutter via plugin) [M] | Yes | Apache-2.0 + premium [M] | Premium plugins [M] | Indie [M] | GA |
| **WatermelonDB** | Relational tables (SQLite) [M] | SQLite / LokiJS [M] | DIY backend (pull/push protocol) [M] | Yes | Pull/push batch [M] | Per-column client-wins [M] | DIY [M] | None | No | No | No | DIY | JS (React Native, web) [M] | Yes | MIT [M] | Free | Nozbe [M] | Maintained, slow [M] |
| **TinyBase** | Tables + key-values [M] | Memory/IndexedDB/SQLite persisters [M] | Persist to Postgres/PGlite/D1 etc. [M] | Yes | No | MergeableStore CRDT (HLC LWW) [M] | None | None | No | No | Via Yjs/Automerge persisters [M] | No | TS [M] | Yes | MIT [M] | Free | Indie (James Pearce) [M] | GA |

#### Matrix B: CRDT/collab libraries and relays

| Product | Data model | Local store | Server | Offline | Tx | Conflict | Server validation | E2EE | Files | FS folders | Collab | Partial | Clients | Self-host | License | Pricing / company | Status |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **Automerge / automerge-repo** | JSON CRDT documents [M] | IndexedDB / NodeFS storage adapters [M] | Sync server (relay) [M] | Yes | Per-doc change | CRDT | None | None built in (Keyhive research) [M] | Bytes in doc only | No (NodeFS is storage, not user files) [M] | Yes | Per document | Rust core, JS/WASM, Swift, Java/Kotlin, Go, Python [M] | Yes | MIT [M] | Ink & Switch, grant funded [M] | 2.x/3.x stable [M] |
| **Yjs** (+ Hocuspocus, Y-Sweet, Liveblocks, PartyKit) | CRDT shared types [M] | y-indexeddb [M] | Hocuspocus (hooks: auth/onChange) [M]; Y-Sweet (S3-backed) [M]; Liveblocks (hosted) [M]; PartyKit (Cloudflare) [M] | Yes | Per-doc | CRDT | Hooks/webhooks only [M] | None (community encrypted providers) [M] | No | No | Yes (dominant editor binding ecosystem) [M] | Per document / subdocs [M] | JS; Yrs Rust; pycrdt; Swift/Kotlin via yffi [M] | Yes (Hocuspocus, Y-Sweet) | MIT (Yjs); Liveblocks proprietary [M] | PartyKit acquired by Cloudflare (2024) [M]; Liveblocks VC-backed [M] | GA |
| **Loro** | CRDT (text, list, map, movable tree) [M] | Snapshot/updates bytes [M] | None (bring transport) [M] | Yes | Per-doc | CRDT incl. movable tree [M] | None | None | No | No | Yes | Shallow snapshots [M] | Rust, JS/WASM, Swift, Python [M] | N/A | MIT [M] | Small team [M] | 1.x [M] |
| **DXOS** | ECHO graph DB (Automerge-based) [M] | OPFS/IndexedDB [M] | P2P + agents [M] | Yes | — | CRDT | None | HALO identity, encrypted transport [M] | Unclear | No | Yes | Spaces | TS [M] | Yes | MIT [M] | VC-backed [M] | Pre-1.0 (Composer app) [M] |
| **Microsoft Fluid Framework / Azure Fluid Relay** | Distributed data structures (SharedTree, SharedMap) [M] | Client memory + summaries [M] | Azure Fluid Relay (managed) or Routerlicious (OSS) [M] | Limited [M] | Op-ordered via sequencer [M] | Total-order sequenced ops [M] | Token-based auth only [M] | None (CMK at rest) [F, doc index] | No | No | Yes | Containers | TS [M] | Yes (Routerlicious) [M] | MIT [M] | Microsoft | **CONFLICTING: innFactory page says Fluid Relay retired 2025-09-30 [F]; Microsoft Learn docs still live, updated 2026-08-02, with no retirement notice [F]. Treat as "status unclear, likely de-emphasized".** |
| **Anytype any-sync** | Object graph CRDT spaces [M] | Local (Go middleware) [M] | any-sync nodes (sync, file, consensus, coordinator) [M] | Yes | — | CRDT (DAG of signed changes) [M] | Consensus node for ACLs only [M] | **Mandatory** [M] | Yes (file nodes, content addressed) [M] | No | Yes (multiplayer spaces) [M] | Per space | Go protocol; Swift/Kotlin/Electron apps via Go lib [M] | Yes (self-host any-sync) [M] | any-sync MIT; app "Any Source Available License" [M] | Any Association (Swiss) [M] | GA |
| **Etebase** (EteSync) | Encrypted collections/items [M] | Client cache | Etebase server (Django) [M] | Yes | Item batch with etags [M] | Optimistic, conflict on etag [M] | None (ciphertext) | **Mandatory** [M] | Yes, chunked items [M] | No | No | Per collection | Rust core, JS, Python, Java/Kotlin, Swift, C [M] | Yes | Server AGPL; libs BSD-3 [M] | Indie/donations [M] | Maintained, low activity [M] |

#### Matrix C: file-only and API-as-sync

| Product | Model | Two-way FS folders | E2EE | Conflicts | Rows/relational | Self-host | License | Status |
|---|---|---|---|---|---|---|---|---|
| **Syncthing** | Block-hashed files, P2P [F, from report] | **Yes** (watcher hint + periodic rescan) [F, from report] | Transport encrypted; **optional "untrusted (encrypted) device"** mode [M] | Newer wins, older renamed `.sync-conflict-…` [F, from report] | No | Yes | MPL-2.0 [M] | GA |
| **rclone (bisync)** | Files ↔ 70+ cloud backends [M] | Yes, `bisync` (was beta, stabilized 2024) [M] | Optional `crypt` remote [M] | Rename conflict copies [M] | No | Yes | MIT [M] | GA |
| **Dropbox API** | Files + cursor-based delta (`list_folder/continue`) [M] | Desktop client yes; API is building block only [M] | Team-folder E2EE for business tiers (2024) [M] | Conflicted copy [M] | No | No | Proprietary | GA |
| **Google Drive API as sync** | Files + `changes.list` page tokens; appDataFolder [M] | Drive for desktop yes; API building block [M] | Workspace client-side encryption (enterprise) only [M] | Revisions; app resolves [M] | No | No | Proprietary | GA |

#### In-house engines (not SDKs; for positioning only)

- **Linear**: client object graph, server-assigned `lastSyncId` total order, transactions over one ordered log; not an SDK [F, from report via reverse-linear-sync-engine].
- **Dropbox Nucleus**: three-tree model (Remote, Local, Synced tree as merge base) [F, from report].
- **Figma**: server-authoritative multiplayer, LWW per property, fractional indexing [M] ([Figma blog](https://www.figma.com/blog/how-figmas-multiplayer-technology-works/)).
- **Google Docs**: operational transformation with central server [M].
- **Notion**: block model, SQLite cache on clients, offline mode added 2025 [M].
- **Obsidian Sync**: plain Markdown files canonical, **optional E2EE**, version history, attachments; two-way folder sync of the vault; but no relational rows and no SDK [M] ([Obsidian Sync](https://obsidian.md/sync)).
- **Logseq**: moving from Markdown-canonical to SQLite-canonical with Markdown only as export; users ask whether two-way DB↔Markdown survives [F, from report].

#### Sources for [F] items
- Electric Durable Streams announcement 2025-12-09 — [Electric blog](https://electric-sql.com/blog/2025/12/09/announcing-durable-streams); 0.1.0 — [electric.ax](https://electric.ax/blog/2025/12/23/durable-streams-0.1.0.md); hosted 2026-01-22 — [Electric blog](https://electric-sql.com/blog/2026/01/22/announcing-hosted-durable-streams); Yjs provider PR — [GitHub PR 4059](https://github.com/electric-sql/electric/pull/4059)
- Databricks acquires Electric (2026-08-11, hosted service shutdown, OSS continues) — [Techzine](https://www.techzine.eu/news/analytics/143565/databricks-acquires-electric-and-brings-postgres-to-ai-agents/); [Blocks & Files 2026-08-12](https://blocksandfiles.com/ai-ml/2026/08/12/oh-no-not-another-one-databricks-buys-electric/5286721); [techstrong.ai](https://techstrong.ai/articles/databricks-buys-electric-to-provide-postgres-for-ai-agents/). Note: trade press only, no primary Databricks release fetched.
- Zero GA March 2026, Cloud Zero 2026 — [Zero docs status](https://zero.rocicorp.dev/docs/status). A third-party timeline lists "Zero 1.0 Stable" on 2026-06-10, conflicting with March — [local-first.com timeline](https://local-first.com/timeline/)
- InstantDB team to OpenAI 2026-08-22, cloud to 2027-08-31, backups to 2028-08-31 — [RuntimeWire](https://runtimewire.com/article/instant-team-joins-openai-cloud-shutdown); [Dealroom](https://dealroom.co/news/146358-openai-acquires-instantdb-team-to-bolster-ai-infrastructure/); seed $3.4M — [InstantDB hiring page](https://www.instantdb.com/hiring/full-stack-engineer); Series A conflict — [Caplight](https://www.caplight.com/company/instantdb) vs [Extruct](https://www.extruct.ai/hub/instantdb-com-funding/)
- Triplit founder joins Supabase 2025-10-08 — [Supabase blog](https://supabase.com/blog/triplit-joins-supabase)
- SQLite Sync 1.0.0 Postgres support 2026-03-24, CRDT algorithms, SQLite Cloud free 3 devices — [GitHub release](https://github.com/sqliteai/sqlite-sync/releases/tag/1.0.0)
- Turso sync push/pull, last-push-wins, TS/Python/Go — [Turso docs](https://docs.turso.tech/sync/usage); beta caveat — [Turso blog](https://turso.tech/blog/turso-offline-sync-public-beta)
- Graft alpha, partial replication, last release 2025-06-20 — [lib.rs graft-proto](https://lib.rs/crates/graft-proto); [orbitinghail GitHub](https://github.com/orbitinghail)
- PowerSync Open Edition (FSL, beta, feature-equivalent), attachments libs GA — [PowerSync blog](https://powersync.com/blog/powersync-open-edition-release); Feb 2026 changelog — [PowerSync](https://powersync.com/blog/powersync-changelog-february-2026); JourneyApps spin-off 2022 — [PowerSync company](https://powersync.com/company); funding — [CB Insights](https://www.cbinsights.com/company/journeyapps/financials); MongoDB partnership Oct 2024 — [VentureBeat](https://venturebeat.com/business/powersync-partners-with-mongodb-to-offer-enterprise-grade-sync-engine/)
- Ditto $82M Series B, $462M valuation, MongoDB connector — [TechCrunch 2025-03-12](https://techcrunch.com/2025/03/12/ditto-lands-82m-to-synchronize-data-from-the-edge-to-the-cloud/); [Ditto blog](https://ditto.com/blog/ditto-lands-82m-to-pioneer-the-edge-native-revolution); total — [Clay](https://www.clay.com/dossier/ditto-funding)
- Couchbase taken private, closed 2025-09-24 — [Couchbase press release](https://www.couchbase.com/press-releases/haveli-investments-completes-acquisition-of-couchbase/)
- Jazz 2.0 alpha, "local-first relational database", no releases — [garden-co/jazz](https://github.com/garden-co/jazz)
- Fluid Relay retirement claim — [innFactory](https://innfactory.de/en/cloud/azure/products/fluid-relay/); docs live — [Microsoft Learn](https://learn.microsoft.com/en-us/azure/azure-fluid-relay/)
- Report-derived [F] items (PowerSync tx API, Zero mutator tx, cr-sqlite db_version, Syncthing, Linear, Dropbox) — `/home/hyoungjoojin/Desktop/skkil/store/reports/Local first sync engine research.md`, section "Case studies"

#### Canonical docs for [M] items (verify)
[Convex](https://docs.convex.dev) · [InstantDB](https://www.instantdb.com/docs) · [Evolu](https://www.evolu.dev) · [LiveStore](https://livestore.dev) · [Jazz](https://jazz.tools) · [Ditto](https://docs.ditto.live) · [Couchbase Mobile](https://docs.couchbase.com/couchbase-lite/current/) · [ObjectBox Sync](https://sync.objectbox.io) · [MongoDB Device Sync deprecation](https://www.mongodb.com/docs/atlas/app-services/sync/device-sync-deprecation/) · [Firestore offline](https://firebase.google.com/docs/firestore/manage-data/enable-offline) · [Supabase](https://supabase.com/docs) · [Appwrite](https://appwrite.io/docs) · [Amplify DataStore](https://docs.amplify.aws/) · [CloudKit encryptedValues](https://developer.apple.com/documentation/cloudkit/ckrecord/encryptedvalues) · [Automerge](https://automerge.org) · [Yjs](https://docs.yjs.dev) · [Hocuspocus](https://tiptap.dev/docs/hocuspocus) · [Y-Sweet](https://github.com/jamsocket/y-sweet) · [Liveblocks](https://liveblocks.io) · [Loro](https://loro.dev) · [DXOS](https://dxos.org) · [any-sync](https://github.com/anyproto/any-sync) · [cr-sqlite](https://github.com/vlcn-io/cr-sqlite) · [RxDB](https://rxdb.info) · [Dexie Cloud](https://dexie.org/cloud/) · [WatermelonDB](https://watermelondb.dev) · [TinyBase](https://tinybase.org) · [Syncthing](https://docs.syncthing.net) · [rclone bisync](https://rclone.org/bisync/) · [Dropbox API](https://www.dropbox.com/developers/documentation) · [Drive changes API](https://developers.google.com/drive/api/guides/manage-changes) · [Etebase](https://docs.etebase.com) · [Fluid Framework](https://fluidframework.com)

### Inferences
- The proposal's closest single competitors by architecture are **PowerSync** (SQLite↔BYO Postgres, transactions up/down, attachments helper, backend validation) and **Zero** (server-re-run mutators). Neither does folders, structured codecs, or E2EE.
- The closest by trust model is **LiveStore** (deterministic replay of an ordered eventlog on clients) plus **Evolu/Jazz** (E2EE). None of these validates on a server in plaintext mode with the same code.
- 2026 consolidation removed three hosted competitors (Electric Cloud, Instant Cloud, Triplit) and shows acquirers buying sync teams for **AI agent state**, not for consumer local-first apps. That leaves self-hostable, app-developer-focused sync less crowded, but also signals weak standalone vendor economics.

### Gaps
- Exact current pricing tiers (PowerSync, Convex, Liveblocks, Ditto, Dexie Cloud, Turso) not verified this session.
- PowerSync funding: no PowerSync-specific round found; only parent JourneyApps data (conflicting dates).
- Zero 1.0 date conflicts (March 2026 per official docs vs 2026-06-10 per third-party timeline); official docs preferred.
- Fluid Relay status unresolved (conflicting sources, no Microsoft retirement notice found).
- Jazz 2.0's E2EE and transaction semantics are unknown (alpha, new API).
- SQLite Sync license, transaction atomicity, and partial sync not confirmed.
- Whether PowerSync attachments work on the self-hosted Open Edition is stated as intent, not confirmed.

## Which products cover ≥3 of R, B, F, E, V? Does any cover all five?

### Takeaway
**No product covers all five.** Products reaching 3 fall into two disjoint clusters: row engines with **R+B+V** (PowerSync, Convex, InstantDB, and Supabase server-side only) and file/personal stacks with **B+F+E** (Syncthing, Obsidian Sync, rclone). Nobody combines R with F, and nobody offers *optional* E2EE alongside server validation.

### Cited Findings
Scoring (✓ = native, ~ = partial/DIY, ✗ = no). Basis is Matrix A–C above; tags inherit from those cells.

| Product | R | B | F | E (optional) | V | Score |
|---|---|---|---|---|---|---|
| PowerSync | ✓ [F] | ✓ attachments [F] | ✗ | ✗ (DIY) [M] | ✓ [F] | **3** |
| Convex | ✓ online only [M] | ✓ [M] | ✗ | ✗ | ✓ [M] | **3** (weak offline) |
| InstantDB | ~ triples + atomic transact [M] | ✓ [M] | ✗ | ✗ | ~ rules [M] | 2–3 (cloud shutting) [F] |
| Supabase | ✓ server only, no offline engine [M] | ✓ [M] | ✗ | ✗ | ✓ RLS [M] | 3 server-side, 0 sync |
| Zero | ✓ [F] | ✗ | ✗ | ✗ | ✓ [F] | 2 |
| Electric + TanStack DB | ~ [M] | ✗ | ✗ | ✗ | ~ DIY API [M] | 1–2 |
| Couchbase Mobile | ✗ docs | ✓ [M] | ✗ | ✗ | ✓ sync function [M] | 2 |
| Firestore | ✗ docs (~ batched writes) | ✓ [M] | ✗ | ✗ | ✓ rules [M] | 2 |
| CloudKit | ~ atomic zone saves [M] | ✓ CKAsset [M] | ✗ | ✓ optional encryptedValues [M] | ✗ | 2–3 (Apple-only) |
| Jazz 1.x | ✗ | ✓ [M] | ✗ | ✗ (mandatory) [M] | ✗ | 1 |
| Anytype / Etebase / Evolu | ✗ / ✗ / ~ | ✓ / ✓ / ✗ | ✗ | ✗ (mandatory) | ✗ | 1 |
| SQLite Sync | ~ relational, no tx [F] | ✗ | ✗ | ✗ | ~ RLS [F] | 1–2 |
| LiveStore | ~ events atomic [M] | ✗ | ✗ | ✗ | ~ client replay [M] | 1–2 |
| Syncthing | ✗ | ✓ | ✓ [F] | ✓ untrusted-device mode [M] | ✗ | **3** |
| Obsidian Sync (in-house) | ✗ | ✓ | ✓ vault folders [M] | ✓ optional [M] | ✗ | **3** (not an SDK) |
| rclone bisync + crypt | ✗ | ✓ | ✓ [M] | ✓ [M] | ✗ | **3** |
| Dropbox API | ✗ | ✓ | ~ (desktop app, not API) | ~ business team folders [M] | ✗ | 2 |

### Inferences
- The two 3-score clusters are complementary halves of the proposal. An app today would need PowerSync/Zero **plus** Syncthing-like folder sync **plus** an E2EE layer, and would hand-write the glue the proposal makes first-class (fs_entry merge base, codecs, one validator).
- "Optional E2EE with the *same* validator run server-side or by client replay" has no direct counterpart. CloudKit has optional per-field encryption but no custom validation; LiveStore has client replay but no plaintext server mode with the same code.

### Gaps
- Whether PowerSync's E2EE demo is an official pattern or just a blog sample was not checked.
- Dexie Cloud blob support and Ditto attachment limits not verified.

## Where are the white spaces?

### Takeaway
Four clear white spaces: (1) **user-folder ↔ relational rows ↔ cloud** two-way sync with structured codecs (Markdown ↔ rows); (2) **one validator, two trust modes** (plaintext server vs E2EE client replay); (3) **transactional blob plane** with leases/claims tied to row mutations; (4) per-field merge declarations that apply identically to DB rows and decoded file content. A fifth, softer gap: a self-hostable, multi-language (Swift/Kotlin/Rust/Go/Flutter) engine now that Electric Cloud, Instant Cloud and Triplit are gone.

### Cited Findings
- Nothing surveyed syncs user-editable files ↔ relational rows ↔ cloud with specified semantics; nearest academic ancestor is Harmony (lenses) — [report, Case studies](file:///home/hyoungjoojin/Desktop/skkil/store/reports/Local%20first%20sync%20engine%20research.md)
- Logseq is moving to SQLite-canonical with Markdown export only; users ask whether two-way DB↔Markdown survives — [Logseq forum](https://discuss.logseq.com/t/is-there-still-a-bi-directional-approach-of-db-markdown-or-only-export-to-markdown-remains/26051?page=2)
- PowerSync's attachments helper stores files in your own storage, and the blob queue is separate from the row transaction — [PowerSync Open Edition post](https://powersync.com/blog/powersync-open-edition-release) [F]; detail of separation [M]
- E2EE engines (Evolu, Anytype, Etebase, Jazz 1.x) make encryption mandatory and drop server validation [M]; row engines (PowerSync, Zero, Electric) offer no E2EE [M].
- Hosted exits: Electric hosted shutting (Aug 2026) [F]; Instant Cloud ends 2027-08-31 [F]; Triplit not integrated into Supabase [F] — sources in KQ0.
- SQLite Sync's block-level LWW for text (line-split merge) is the nearest thing to "structured file merge", but it lives inside DB columns, not user files — [sqlite-sync 1.0.0](https://github.com/sqliteai/sqlite-sync/releases/tag/1.0.0) [F]

### Inferences
- The folder-as-third-replica with lens laws (F1–F5) is the most defensible novelty; it is also the hardest to build (platform quirks, codecs) so it is unlikely to be copied quickly.
- The "one validator" idea is novel as a product, but LiveStore's deterministic materializers and Replicache/Zero's re-run mutators are strong prior art; position it as unifying those two, not as invention from scratch.
- Blob plane with leases/claims is novel as a sync-engine feature, but users may see it as "attachments done right" rather than a headline; it is a differentiator, not a category.
- Risk: the 2026 acquisitions suggest the money is in AI-agent state; a consumer/personal-app positioning may need to explain why folders + E2EE matter for agents too (e.g., agents editing Markdown files that are rows).

### Gaps
- No user-demand data (survey, GitHub issues counts) on two-way folder ↔ DB sync was gathered.
- Did not check Notion/Linear/Obsidian roadmaps for SDK plans.

## Which recent entrants (2025–2026) target the same space?

### Takeaway
Recent entrants and pivots are all row-side or protocol-side: Zero GA (Mar 2026), SQLite Sync 1.0 with Postgres (Mar 2026), Durable Streams (Dec 2025), Jazz 2.0 relational alpha, Turso offline sync (beta), Graft (alpha), NextGraph (FOSDEM 2026). None targets user folders. Net market motion in 2026 was consolidation toward AI-agent infrastructure.

### Cited Findings
- Zero generally available March 2026; Cloud Zero public in 2026 — [Zero status](https://zero.rocicorp.dev/docs/status)
- SQLite Sync 1.0.0 (2026-03-24) adds a PostgreSQL build of the CRDT extension; works with any Postgres or Supabase — [GitHub](https://github.com/sqliteai/sqlite-sync/releases/tag/1.0.0)
- Durable Streams protocol (2025-12-09), State Protocol in 0.1.0, exactly-once in 0.2.0 (2026-01-22), Apache-2.0, independent spec at durablestreams.com — [Electric](https://electric-sql.com/blog/2025/12/09/announcing-durable-streams); [Electric](https://electric-sql.com/blog/2026/01/22/announcing-hosted-durable-streams)
- Jazz 2.0 alpha as "local-first relational database for the browser, React Native and your backend" — [GitHub](https://github.com/garden-co/jazz)
- Turso offline sync with push/pull — [Turso docs](https://docs.turso.tech/sync/usage); still labeled beta — [Turso blog](https://turso.tech/blog/turso-offline-sync-public-beta)
- Graft, lazy partial replication on object storage, alpha — [orbitinghail](https://github.com/orbitinghail)
- NextGraph "E2EE sync engine, SDK, graph DB, reactive ORM" talk at FOSDEM 2026 local-first devroom — [FOSDEM 2026](https://fosdem.org/2026/schedule/track/local-first/)
- Ditto MongoDB Connector positioned to replace Atlas Device Sync (2025) — [SiliconANGLE](https://siliconangle.com/2025/03/12/ditto-raises-82m-funding-edge-database/)
- Consolidation: Databricks–Electric (2026-08-11) [F]; OpenAI–Instant team (2026-08-22) [F]; Supabase–Triplit founder (2025-10-08) [F]; Haveli–Couchbase (2025-09-24) [F] — sources in KQ0.
- TanStack DB (Electric-sponsored client store with optimistic transactions) launched in 2025 and continues as OSS post-acquisition [M]; continuation — [Techzine](https://www.techzine.eu/news/analytics/143565/databricks-acquires-electric-and-brings-postgres-to-ai-agents/) [F]

### Inferences
- NextGraph is the only 2026 entrant found that pairs E2EE with an SDK + ORM; it is the one to watch for overlap with the opt-in E2EE mode (mandatory vs optional unverified).
- SQLite Sync 1.0 is the most direct new rival for "local SQLite ↔ BYO Postgres", but its per-change CRDT model lacks the multi-row transactional log the proposal requires.

### Gaps
- No search confirmed other 2025–2026 startups (e.g., new YC sync companies); a Crunchbase/YC directory sweep was not done.
- NextGraph details (license, E2EE optionality, platforms) not fetched.
