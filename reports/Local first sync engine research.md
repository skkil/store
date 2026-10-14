# Make the media service the sync engine's blob plane

A reusable local-to-cloud sync engine can own nearly all of the *mechanics*: transport, change capture, local storage, op queue, ordering, optimistic apply and rebase, a standard CRDT library, partial-replication bookkeeping, and garbage collection plumbing. What it cannot own is the *meaning* of data. The application has to declare about ten things: field types, a merge rule per field, invariants, authority, sync scope, authorization, schema versions, retention, side effects, and how conflicts are shown. Theory tells you exactly which operations can run offline without coordination (I-confluence, CALM), so an engine can sort operations into "local-safe", "escrow-able" and "needs the server" once the app declares its invariants. Industry has settled on one pattern: declarative *read* scope over an existing Postgres, imperative server-side *writes*, and CRDTs only for rich collaborative fields. Zero, PowerSync and Electric all work this way, while engines that tried to *be* the database (Realm Device Sync, Amplify DataStore, Triplit) were shut down. A full general-purpose engine is not a credible undergraduate thesis against funded competitors. A narrow semantics contribution is, and the gap that fits your existing work best is this: **what happens to an offline write that references a not-yet-uploaded blob and is later rejected by the server**. No product documents invariants for this, and no PaPoC 2026 paper covers blobs. My recommendation is a hybrid. Keep the media service (A) as the blob subsystem. Lift its invariants I1–I6 from "one server DB ↔ S3" to "N offline devices ↔ sync server ↔ object store". Prove it with a deterministic simulator and a small formal model. Watch the schedule: A's plan ends in March, so the hybrid has to stay deliberately small. The case-study section applies all of this to `clip` and `time`. It covers files plus SQLite plus Postgres with transactions, and conflict rules between one user's own devices. It also designs two-way sync between user folders and the database, and makes end-to-end encryption an opt-in mode of the same protocol: one deterministic validator that runs on the server by default and on clients when the server holds only ciphertext.

## Ten declarations and three things no engine can own

The cleanest way to scope "how much fits in one module" is to split everything into three tiers. **Engine-generic** covers transport, change data capture, the local store, the op queue with idempotency keys, causal or total ordering, optimistic apply and rebase, a CRDT library, scope evaluation, tombstone GC mechanics, lens application, outbox plumbing, and a rejection/compensation event channel. **Declarable** covers things the app supplies as data and the engine enforces. **Inherently app-specific** covers what has to stay code or human judgment. The declaration surface below draws on precedents across academia and products.

| Declaration | What the app states | Precedent |
|---|---|---|
| Data model | rows, documents, trees, text, counters, blobs | Jazz CoValues, Collabs |
| Merge rule per field | LWW, multi-value, OR-set, counter, sequence; or a "conflict order" annotation | Katara synthesizes verified CRDTs from that one annotation ([arXiv 2205.12425](https://arxiv.org/pdf/2205.12425)) |
| Invariants → consistency class | local-safe / escrow / coordinate | I-confluence ([arXiv 1402.2237](https://arxiv.org/pdf/1402.2237)), ECROs ([OOPSLA'21](https://asc.di.fct.unl.pt/~nmp/pubs/oopsla-2021.pdf)) |
| Authority | server-authoritative mutator vs peer-merged CRDT | Zero mutators run on client then server; "the server's result is authoritative" ([Zero docs](https://zero.rocicorp.dev/docs/mutators.md)) |
| Scope (partial replication) | server-side parameterized queries | PowerSync Sync Streams ([docs](https://docs.powersync.com/sync/streams)), Electric shapes |
| Authorization | predicate over (user, old row, new row) plus a rejection policy | Couchbase sync function ([docs](https://docs.couchbase.com/sync-gateway/current/sync-function-api-require-access-cmd.html)); Realm compensating writes ([MongoDB](https://www.mongodb.com/docs/realm/sdk/swift/sync/write-to-synced-realm/)) |
| Schema versions | bidirectional lenses between versions | Cambria ([Ink & Switch](https://inkandswitch.com/cambria)) |
| Deletion / retention | delete-wins, tombstone TTL, GC horizon | PowerSync deletes always win ([docs](https://docs.powersync.com/handling-writes/handling-update-conflicts)) |
| Side effects | outbox hooks fired after authoritative commit, idempotency keys | Zero recommends a transactional outbox ([docs](https://zero.rocicorp.dev/docs/mutators.md)) |
| Conflict surfacing | silent, notify, or keep both values | Upwelling, PaPoC 2026 "Semantic Conflict Model" ([program](https://papoc-workshop.github.io/2026/program.html)) |

The theory behind the invariants row is the strongest argument that sync *can* have proper semantics. **I-confluence is necessary and sufficient** for coordination-free, convergent execution under an invariant. Per-record inequality constraints qualify. Foreign keys qualify for deletes but not inserts. A "greater than" threshold holds for increments but not decrements. **10 of 12 TPC-C invariants are I-confluent** ([Bailis et al.](https://arxiv.org/pdf/1402.2237)). CALM gives the general rule that monotonic programs need no coordination ([arXiv 1901.01930](https://arxiv.org/abs/1901.01930v2)). Hellerstein's 2026 "Coordination Criterion" sharpens it to "future-monotone" specifications ([arXiv 2602.09435](https://arxiv.org/pdf/2602.09435v2)). For invariants that fail the test, escrow or bounded counters split the slack among replicas so each can keep decrementing offline until its share runs out ([Just-Right Consistency](https://arxiv.org/pdf/1801.06340)). For an engine, this turns "can this work offline?" from a judgment call into a classification step.

The same theory sets the limits. Uniqueness, non-negative balances and foreign-key inserts **cannot be both offline-available and correct**, so the app must pick one or pre-allocate rights ([Bailis](https://www.bailis.org/blog/when-does-consistency-require-coordination/)). Three things stay app-specific however good the engine is. The first is the *content* of invariants: ECROs' own authors call hand-written invariant specs hard to write ([VUB](https://researchportal.vub.be/en/publications/concurrency-contracts-for-designing-highly-available-replicated-d/)). The second is compensation for real-world side effects: an email that was sent is not rolled back on retry ([Zero](https://zero.rocicorp.dev/docs/mutators.md)). The third is the UX for rejected or surprisingly merged offline writes. Nobody has measured what fraction of real app logic lands in each tier. That measurement would itself be a useful thesis result.

## The engine needs two merge regimes behind one API

On depth: no single merge model serves both enterprise records and collaborative content. **Server reconciliation** is the most general abstraction for business data. Clients send named mutations, and the server replays them in its own order against authoritative state, so no CRDT algebraic laws are needed and arbitrary business logic is allowed. Replicache and Zero use it ([Weidner, via summary](https://searchengineworld.com/energy/Architectures_for_Central_Server_Collaboration)). Zero calls its mutators "during sync for conflict resolution", so the wire format carries *intent*, not state diffs ([Zero](https://zero.rocicorp.dev/docs/mutators.md)). CRDTs remain necessary for text, trees and anything that must merge without a server. Weidner goes further and argues that with a central server, even collaborative text can skip CRDTs and OT by using per-character IDs and server reconciliation ([slides](https://docs.google.com/presentation/d/1WHUKnht-RxZ9lrMLBMwrs8BpW7ifN7y6iillhju2rKw/htmlpresent)). No one has evaluated that claim independently.

A formal result makes the two-regime design legitimate. An ICFP 2025 paper proves that **clients cannot distinguish a state-based from an op-based CRDT implementation** ([arXiv 2504.05398](https://arxiv.org/abs/2504.05398)). An engine can therefore define its semantics abstractly, as a convergent data type with causal consistency, and choose the wire protocol separately. Eg-walker points to a matching storage design: persist a plain event graph and build CRDT state only during merges. It won Best Artifact at EuroSys 2025 for beating OT on merging diverged branches and beating CRDTs on memory ([arXiv 2409.14252](https://arxiv.org/abs/2409.14252)). The engine shape that follows uses named mutations as the universal wire format and an event log as storage. CRDT-typed fields (a Yjs or Automerge document stored as an opaque value) get merged by their own library rather than overwritten.

Breadth is where most of the remaining problems sit. "Keep CALM and CRDT On" shows that CRDTs constrain merges but **leave reads unconstrained**: a non-monotone query over intermediate state can be inconsistent ([arXiv 2210.12605](https://arxiv.org/abs/2210.12605)). Correct partial replication is therefore a read-semantics problem too. Declarative scope is also security-critical. **CVE-2026-30870** in PowerSync 1.20.0 ignored certain subquery filters when deciding what to sync, so a bug in the scope language became a data leak ([vuln.today](https://vuln.today/cve/CVE-2026-30870)). Blobs belong in the breadth as well. Every mature file sync system splits a mutable metadata layer from immutable, hash-addressed content, and puts all conflict resolution in the metadata. Dropbox Nucleus, for example, uses three trees with a synced merge base and ID-based nodes ([Dropbox](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)). That split is exactly what lets a blob subsystem plug into a row-sync engine.

## Industry converged on declarative reads and imperative writes

Products have stopped trying to make write semantics declarative. Zero **deprecated its declarative permissions** for plain code in mutators ("there is no need for a special permissions system") ([Zero](https://zero.rocicorp.dev/docs/debug/permissions)). Electric dropped its CRDT bidirectional design in 2024 and now syncs reads only, with writes going through your API, which returns a txid ([legacy notes](https://legacy.electric-sql.com/docs/reference/release_notes); [TanStack](https://tanstack.com/blog/electric-partnership)). PowerSync defaults to per-field LWW and leaves anything smarter to backend code ([PowerSync](https://docs.powersync.com/handling-writes/custom-conflict-resolution)). Industry has made declarative "what each client sees" work. "What a write means" has gone back to code, and that gap is where academic invariant work (ECROs, Katara, LoRe, ConLoc) has not reached products.

| System | Read path | Write path / authority | Merge | Offline writes | Blobs |
|---|---|---|---|---|---|
| Zero 1.0 (mid-2026) | ZQL queries, CDC from Postgres into SQLite replica ([InfoQ](https://infoq.com/news/2026/06/zero-version-1)) | Two-sided mutators, server authoritative | Server reconciliation | Officially unsupported ([docs](https://zero.rocicorp.dev/docs/when-to-use)) | App-wired |
| PowerSync | Sync Streams over buckets ([docs](https://docs.powersync.com/sync/streams)) | Upload queue → your API | LWW default | Yes; ops must be idempotent ([docs](https://docs.powersync.com/handling-writes/)) | Attachment queue, still **alpha**, local-only state ([docs](https://docs.powersync.com/usage/use-case-examples/attachments-files)) |
| Electric 1.0 (Mar 2025) | Single-table shapes over HTTP, CDN-cacheable ([Electric](https://electric.ax/blog/2025/03/17/electricsql-1.0-released)) | Your API + TanStack DB optimistic state | n/a | Needs other tools | None documented |
| Linear | Ordered workspace sync log with sync IDs, permission-filtered deltas ([Linear](https://linear.app/blog/rebuilding-delta-sync-read-path)) | Server-executed transactions | Server order | Limited | — |
| Figma | Per-document server process | Server orders all edits | Per-property LWW, "inspired by CRDTs" ([Figma](https://www.figma.com/blog/how-figmas-multiplayer-technology-works/)) | — | — |
| Notion offline (2025) | SQLite store; `offline_action` table records *why* a page is kept ([Notion](https://www.notion.com/en-gb/blog/how-we-made-notion-available-offline)) | Server-centric | CRDT for offline pages only | Desktop/mobile | — |
| Jazz | CoValues with group permissions | Peer-merged, cryptographic | CRDT | Yes | FileStream as a synced object ([Jazz](https://jazz.tools/docs/react/using-covalues/filestreams)) |
| Ditto | Big Peer / Small Peer mesh | Peer-merged | Delta-state CRDT ([Ditto](https://www.ditto.com/blog/dittos-delta-state-crdts)) | Yes | Hash token, lazy fetch, GC only on small peers ([Ditto](https://docs.ditto.live/sdk/latest/crud/working-with-attachments)) |

Two lessons matter for your design. First, the products that survived sit *beside* an existing database. MongoDB Atlas Device Sync reached end of life on 2025-09-30 ([MongoDB](https://mongodb.com/docs/atlas/app-services/sync/device-sync-deprecation)), Amplify DataStore Gen 1 reaches end of life in May 2027 ([Amplify](https://docs.amplify.aws/gen1/flutter/build-a-backend/more-features/datastore/migrate-from-datastore/authorization-and-data/)), and Triplit's founder joined Supabase, which said offline "remains an open problem" ([Supabase](https://supabase.com/blog/triplit-joins-supabase)). An enterprise engine should therefore be middleware over Postgres, not a new database. Second, industry's pain points are **authorization over partial replicas, schema evolution across stale clients, and choosing what to keep offline**, not conflicts. Notion built reason-tracking and capped offline databases at 50 rows. Its `offline_action` table, where a page is evicted when no reasons remain, is structurally the same as your §18 claim layer ([Notion](https://www.notion.com/en-gb/blog/how-we-made-notion-available-offline)). That parallel strengthens the hybrid argument below.

## Research solved text CRDTs; authorization, schema and partial sync remain open

Performance no longer blocks CRDTs. Fugue removes interleaving anomalies that "went unnoticed for decades" ([arXiv 2305.00583](https://arxiv.org/pdf/2305.00583.pdf)). **Automerge 3 cut memory more than 10x**: pasting Moby Dick takes 1.3 MB instead of 700 MB ([Automerge](https://automerge.org/blog/automerge-3/)). Collaboration support therefore costs a thesis integration work, not novelty. Embedding Yjs or Automerge as an opaque field type is the right call. The 2024–2026 frontier lies elsewhere:

- **Authorization.** Keyhive argues that local-first access rules "must travel with the data" ([Keyhive](https://www.inkandswitch.com/keyhive/notebook/01/)). KIT notes that Matrix and Keyhive both rest on informal specs and unverified implementations, and verifies a simplified group-membership RDT in Rust/Verus ([arXiv 2604.23560](https://arxiv.org/abs/2604.23560)). PaPoC 2026 included work on "duelling admins" revoking each other ([program](https://papoc-workshop.github.io/2026/program.html)). This work targets peer-to-peer and Byzantine settings. The enterprise case, where a server rejects a buffered offline write on reconnect, is much less formalized.
- **Schema evolution.** Cambria's lenses remain experimental. Migrating in-flight ops across replicas without a coordinator is open ([arXiv 2412.06269](https://arxiv.org/pdf/2412.06269); [Baseline](https://arxiv.org/abs/2512.09762)).
- **Partial replication with causal correctness.** No 2025–2026 primary paper solves it for op-based histories. Industry treats it as engineering (shapes, buckets, Sedimentree sharding).
- **Invariants.** The closest prior art to "declarative semantics for sync" is **LoRe** ([arXiv 2304.07133](https://arxiv.org/abs/2304.07133)) and **ConLoc** (IEEE TSE 2025), which enforce annotated invariants in local-first apps and insert coordination only where needed ([ACM DL](https://dl.acm.org/doi/10.1109/TSE.2024.3477723)). Any declarative-compiler thesis must position against them. As far as the notes establish, they work at the language level and cover no Postgres-backed engine, partial replication, authorization or blobs.
- **Blobs.** PaPoC 2026 had **no paper on blobs or schema evolution** ([program](https://papoc-workshop.github.io/2026/program.html)).

Blob handling across products is uniformly under-specified. Couchbase Lite silently replicates a document whose blob exceeds 20 MB without the blob ([Couchbase](https://docs.couchbase.com/couchbase-lite/current/swift/blob.html)). Ditto lets servers hold tokens without bytes and runs GC only on small peers ([Ditto](https://docs.ditto.live/sdk/latest/crud/working-with-attachments)). PowerSync uses a count-based local archive cache and loses files that were never uploaded on migration ([PowerSync](https://docs.powersync.com/usage/use-case-examples/attachments-files)). None of these documents three things: what readers see while a referenced blob is still uploading, GC that is safe against devices offline for weeks, or revocation that reaches device caches. Integrated blobs exist as products (Jazz, Ditto), so novelty cannot rest on integration. It has to rest on a **specified, tested contract**.

## The hybrid beats both pure options on novelty per month

A is honest about itself. Its §0.1 calls it "a composition of established patterns ... plus one candidate new mechanism (the store-side fence)". Reviewers will agree. Reserve-then-expire is a commercial feature at Uploadcare, where unstored files are deleted after 24 h ([Uploadcare](https://uploadcare.com/docs/uploads/storage/)), and Supabase's tus store has expiry sweepers ([Supabase](https://github.com/orgs/supabase/discussions/51060)). A has three real strengths: the I1–I6 invariant set with checkers, the fence (if the S3 `If-None-Match` spike works), and the measured B0/B1/B2/P1/P2 comparison, since practitioner material is only qualitative. Supabase restores that bring back rows without bytes show the DB↔blob divergence is a real failure mode in a major product ([SimpleBackups](https://simplebackups.com/blog/restore-supabase-storage-objects)). Candidate B, a general declarative engine, has the opposite profile: high appeal, a crowded field, a direct collision with ConLoc, and a high risk of ending as an unfinished framework demo.

| Option | Contribution | Novelty | Impact | Feasibility* |
|---|---|---|---|---|
| 1. A as-is, v3–v6 cut from thesis text | I1–I6, fence, B0–P2 measurement | Low: lifecycle is folklore; fence is vendor-specific and unverified | Narrow: teams using presigned uploads | High (~85%) |
| 2. A + TLA+/Quint model + deterministic simulator | Option 1 plus machine-checked lease/fence/reclaim | Low–medium | Narrow | High (~75%) |
| **3. Hybrid: offline sync for blob-referencing records, A as blob plane** | Semantics + prototype for offline records that reference unuploaded blobs, rejection of queued writes with cascading compensation to dependent rows and claims, bounded revocation on partial replicas | **Medium–high: gap not covered by PaPoC 2026 or vendor docs** | Enterprise-relevant; reusable across skkil apps | Medium (~55–65%) with CRDT text as an opaque Yjs field |
| 4. Declarative sync DSL compiler | One spec → merge config, coordination points, row filters, authz | Highest appeal, collides with ConLoc/LoRe | High if finished | Low (~35%) |
| 5. Sync-engine fault harness applied to PowerSync/Electric/Automerge-repo | Reusable oracles + simulator; may find real bugs | Depends on findings | Potentially high | Medium–high (~65%) |

*The research notes estimated feasibility for one year with one undergraduate, and the figures are judgment, not data. **A's own schedule runs October to March with December lost to finals**, and today is 4 November 2026. If the deadline really is March 2027, roughly four productive months remain, and Option 3 is feasible only in its smallest form.

The hybrid fits because A's mechanisms map one-to-one onto unsolved sync problems:

- **Claims (§18)** become engine-maintained reference tracking. Each synced row that references media M is a claim, its row version or HLC is the `seq`, and leased claims model devices that may still be offline. This is Notion's reason-tracking, generalized and made sequence-safe.
- **Lease-before-capability** becomes offline capability expiry.
- **The fence** stops late uploads that arrive after the server rejected the write that referenced them.
- **`media.deleted{reason}`** becomes a tombstone in the sync stream, extending I2 bounded revocation to device caches.
- **CRDT collaboration** falls out cleanly. Blobs stay immutable and hash-named, so concurrent "replace image" is a register conflict on the reference, never on the bytes. The loser loses its claim and becomes GC-eligible after its grace period.

Scope the transport minimally. Use a Linear/Replicache-style ordered server log with sync-ID checkpoints and rebase of pending mutations ([Replicache](https://doc.replicache.dev/concepts/how-it-works); [reverse-linear](https://github.com/wzhudev/reverse-linear-sync-engine)), not a new engine.

The recommended title framing is *"Referential integrity and rejection semantics for offline-first apps with large objects."* Use four or five invariants in A's style:

- **S1:** no dangling blob reference after convergence.
- **S2:** a rejected offline write leaves no orphan blob and no dependent row; compensation cascades causally.
- **S3:** revoked data is unreadable on a partial replica within a bound.
- **S4:** convergence holds under partition and crash.
- **S5:** a blob claimed by an offline device is not reclaimed before its lease expires.

Readers see an explicit **pending → committed → revoked** reference state instead of a silent 404. In a four-month version, keep A's core (§5–§6, I1–I6, B2 vs P1) as chapter one, which is already planned. Build S1, S2 and S5 on a minimal op log as the contribution. Treat S3, schema evolution and the declarative DSL as future work. Before claiming the gap, check the offline-rejection and compensation semantics of Ditto, Couchbase/Capella and Jazz v2. The notes flag this as unverified.

## Deterministic simulation is the evaluation that makes the thesis credible

Evaluate in three layers and put the weight on correctness. **Correctness**: encode S1–S5 (and I1–I6) as oracles in a seeded deterministic simulator, with N clients × offline durations × partitions × server crashes × mid-offline authorization revocations × late uploads. Report violations per 10^6 schedules for each design, plus ablations that remove one mechanism at a time (claims, leases, fence). Dropbox's Nucleus team credits exactly this method. CanopyCheck and Trinity run tens of millions of seeded randomized tests nightly with mocked filesystem, network and time ([Dropbox](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)). bones-sim is a reusable Rust starting point for CRDT oracles ([docs.rs](https://docs.rs/crate/bones-sim/latest)), and the Jepsen/Elle causal-consistency configs cover cross-checks on real processes with Toxiproxy ([GitHub](https://www.github.com/nurturenature/jepsen-causal-consistency)). Add a small TLA+ or Quint model of the reference/rejection/claim protocol, checked on small configurations and validated against implementation traces. Hughes et al. show this style of work is publishable: their QuickCheck model of a single-file synchronizer found data-losing behavior in Dropbox and Google Drive ([ICST'16](https://www.cis.upenn.edu/~bcpierce/papers/mysteriesofdropbox.pdf)).

**Baselines** should be the status quo designs:

- (a) a PowerSync-attachments-style local queue with app-written integrity logic;
- (b) Zero-style "no offline writes";
- (c) A's online-only presigned flow;
- (d) the proposed design.

**Performance** metrics:

- time to convergence versus offline duration and queue length;
- bytes per sync for full, delta and partial replicas;
- p50/p99 local write latency;
- reclaim lag and orphan-byte persistence (A's I3/I6, carried over);
- for the opaque CRDT field, overhead against raw Yjs/Automerge on the standard editing traces ([editing-traces](https://github.com/josephg/editing-traces)).

**Expressiveness**: build two case-study apps, the skkil clipboard (`clip/`) and the task manager (`time/`), both analysed in the next section, and count lines of declarative configuration against hand-written sync code. This also gives the first data point on how much of sync logic really abstracts. No standard benchmark exists for partial replication, authorization rejection or blob integrity, so publishing the oracle suite is itself a contribution.

## Case studies: files, SQLite transactions and conflicts in `clip` and `time`

The sections above treat files only as uploaded blobs. They do not specify how an engine keeps local files, local SQLite rows and a remote relational database consistent, or how conflicts between one user's own devices are resolved. This section fills that gap. It is grounded in the two codebases as they stand today (November 2026) and in additional research notes (`fs_sqlite_transactions_conflicts.md`).

### What the two apps look like today

| | `clip` (clipboard manager) | `time` (task manager) |
|---|---|---|
| Clients | Chrome MV3 extension (React), Go CLI, Rust desktop daemon (Wayland; screenshots + OCR) | Tauri 2 desktop app (Rust + React) |
| Local DB | **None.** Chrome keeps only preferences in `chrome.storage.sync` | SQLite (rusqlite, WAL), migrations 001–010 via `PRAGMA user_version`, about 22 tables |
| Local files | Screenshots written as `~/Pictures/Screenshots/clip_<UTC timestamp>.png` (`apps/desktop/src/shot.rs`) | Only preset exports to `~/Downloads/*.skkil-preset.json` |
| Remote DB | Postgres 18: `clips(id uuid DEFAULT gen_random_uuid(), user_id, content TEXT, created_at)` (`apps/server/db/schema.sql`) | None ("there is no server and no sync", `time/AGENTS.md`) |
| IDs / versions | Server-generated UUIDv4; no `updated_at`, no tombstones | `INTEGER PRIMARY KEY` rowids; epoch-second `created_at`; no `updated_at`, no tombstones; hard deletes with FK cascade |
| Stated sync plan | E2EE zero-knowledge relay, per-item LWW over HLC, SHA-256 content-addressed chunked blobs, offline queue (`clip/AGENTS.md`) | E2EE sync with ciphertext-only server, "Evolu-style vs own relay" still open (OQ-7), Google Calendar mirroring (`.jennifer/products/personal-time/spec-v1-2026-10-23.md`) |

Neither app is sync-ready, and both need the same schema groundwork before any engine helps. Primary keys must be generated on the client (UUIDv7/ULID, or a content hash where identity *is* content). Every synced table needs a version and a tombstone. Every user action that touches several rows must become one named mutation.

### Three kinds of local state, not two

Your apps mix three kinds of state, and an engine must treat each one differently.

1. **Engine-owned rows** in SQLite (tasks, clip metadata). These are synced by mutations, as described in the sections above.
2. **Engine-owned files** in a content-addressed store (CAS) that the app controls, for example `<app data>/blobs/sha256/ab/cd…`. Content is immutable and named by its hash, so files never conflict; only the rows that point at them can.
3. **User-owned files** in folders the user or OS can change behind the app's back, such as `~/Pictures/Screenshots`. These are a filesystem sync problem in the Dropbox/Syncthing sense, and they are much harder.

Kind 3 needs change detection and three-way merge. Syncthing treats the file watcher only as a hint, batched about every 10 s, and treats a full rescan (default hourly) as the truth. It compares mtime, size and permissions first, then SHA-256 per block. It writes into a temp file and renames, and on conflict keeps the newer file and renames the older one to `.sync-conflict-<date>-<time>-<device>` ([Syncthing](https://docs.syncthing.net/users/syncing.html)). Dropbox Nucleus keeps three trees (remote, local and a *synced* merge base) so it can tell "edited here" from "edited there" ([Dropbox](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine)). Apple's File Provider versions content and metadata separately and hands conflicts to the app together with a base version ([Apple](https://developer.apple.com/documentation/fileprovider/nsfileprovideritemversion)). Cross-OS problems also belong here: case-insensitive filesystems, path-length limits, symlinks and partially written files.

Two-way sync between user folders and the database is the premise of this engine, not an optional extra. The engine therefore treats each mounted folder as **a third replica on the device**, next to the local SQLite replica and the remote one, and gives it the same semantics as any other replica. The design is in the next subsection.

### Two-way filesystem ↔ database sync as a first-class replica

**Prior art and the gap.** File synchronizers (Syncthing, Dropbox, Unison) sync bytes between filesystems. Row engines (PowerSync, Zero, Electric) sync rows between databases. Nothing in the surveyed products syncs **user-editable files ↔ relational rows ↔ cloud** in both directions with specified semantics.

The closest academic ancestor is Pierce et al.'s **Harmony**. It combines *lenses*, which are bidirectional transformations from concrete file formats to an abstract schema, with a schema-driven synchronization algorithm that is proven against a simple spec ("never overwrite changes, never make up contents") ([DBPL 2005](https://senslab.inrialpes.fr/papers/files/DBPL_2005_schema_sync.pdf); [lenses paper](https://www.cs.tufts.edu/~nr/cs257/archive/benjamin-pierce/lenses-full.pdf); [JCSS 2007](https://repository.upenn.edu/cis_papers/344/)). Harmony synced files to files, and it predates CRDTs, relational sync engines and object storage.

The cautionary tale is **Logseq**. Its Markdown graphs make the files canonical, but its new DB version makes SQLite canonical and offers Markdown only as *export*. Users are asking whether two-way DB↔Markdown sync will survive ([Logseq forum](https://discuss.logseq.com/t/is-there-still-a-bi-directional-approach-of-db-markdown-or-only-export-to-markdown-remains/26051?page=2); [unofficial FAQ](https://logseq.io/page/e87c7359-51f7-44fe-87b3-4a0cd9f2dee3/695feeec-88be-4c5b-8bf2-572513c2f730)). These are community sources, not official statements. A funded team retreated from exactly this problem, so solving it with clear semantics is a real contribution, and it is also the riskiest part of the project.

**Model.** A *mount* binds a folder to a table through a **codec**. Codecs come in two shapes:

| Codec kind | File ↔ data | Example | Conflict unit |
|---|---|---|---|
| **Opaque** | one file ↔ one `file` row (path, name, mime, size, mtime) + one CAS blob | `clip`: `~/Pictures/Screenshots/*.png` ↔ image clips; `time`: `attachments/<task>/…` ↔ attachment rows | the whole file, which forks a conflict copy |
| **Structured** | one file ↔ *many* rows, parsed and rendered | `time`: `Projects/<project>.md` with one line per task ↔ `task` rows; `clip`: optional `Clips/<date>.md` journal | individual rows and fields, merged with the engine's per-field rules |

Directories map to a tree, such as folders to projects or collections. A path is then just *fields* (`parent_id`, `name`) of a row, and a rename or move is a row update, not a delete plus create.

**Per-device state.** For each mounted path, the device keeps a row in `fs_entry(entry_id, row_id, path, path_key, inode/file_id, size, mtime_ns, observed_hash, synced_hash, synced_row_version, codec_complement)`.

- `synced_hash` and `synced_row_version` together form the **merge base**. This is Dropbox's "Synced tree", which tells "edited on disk" apart from "edited elsewhere" ([Dropbox](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine)).
- `path_key` is the NFC-normalized, case-folded path under a unique index. It catches case and Unicode collisions before they sync.
- `codec_complement` stores what the abstract rows cannot hold: formatting, comments, unknown front-matter, ordering of unrecognized lines.

**Inbound (disk → DB):**

1. A watcher event is only a *hint*. A startup scan plus a periodic scan is the truth, as in Syncthing's 10 s watcher batches with an hourly rescan ([Syncthing](https://docs.syncthing.net/users/syncing.html)).
2. Pick candidates with a cheap stat check (size, mtime, inode). Wait until size and mtime are stable, then hash.
3. **Suppress echoes.** If `observed_hash` equals the hash the engine itself last wrote, stop. Otherwise every remote change would come back as a new local edit.
4. Classify the change. A missing path plus a new path with the same inode or hash is a **move**. Everything else is a create, modify or delete.
5. Decode with the codec, diff against the rows at `synced_row_version`, and emit **mutations**. Write those mutations, the `fs_entry` update and the outbox record in **one SQLite transaction**. A 40-line edit to a project file therefore becomes one atomic, named mutation such as `fs_import(project, diff)`.

**Outbound (DB → disk):**

1. When remote or local-UI changes touch rows bound to a file, render the file with the codec, using the stored complement so the user's formatting survives.
2. If the file on disk still equals `synced_hash`, write the new content safely. Record `pending_fs_write(entry, expected_hash)` in SQLite first, then write a temp file, `fsync`, `rename` it over the target, and commit the new `synced_hash`. After a crash, the startup scan compares the file with `expected_hash` and finishes or rolls back. A file and a DB row can never commit atomically, so this intent record replaces atomicity.
3. If the file was edited since `synced_hash`, it is a **three-way conflict**. Resolve it as follows.

**Conflict resolution on files:**

- **Opaque files.** Keep both versions as a conflict copy, `name (conflict <device> <date>).ext`, following Syncthing and OneDrive. The copy is a **new row**, so it syncs to every device and the user resolves the conflict by deleting one of the two. Delete versus modify resolves as *modify wins*, and the modified file survives, as in Syncthing.
- **Structured files.** Never fork. Decode base, local and remote into rows, merge **row by row and field by field** with the same declared rules the database uses (LWW, sets, fractional order, soft invariants), then re-render. A file conflict thus becomes an ordinary row conflict. Only when two edits hit the same field and the rule is "surface" does the engine write a visible marker, or record a sync issue, for the user.
- **Moves.** Concurrent renames are an LWW register on `(parent_id, name)`. Concurrent directory moves that would form a cycle need the tree-move operation, which is known to break Google Drive and Dropbox ([Kleppmann](https://martin.kleppmann.com/2021/10/07/crdt-tree-move-operation.html)). Two different rows that resolve to the same `path_key` get a deterministic suffix.

**Row identity inside structured files.** Rows must keep their identity when the user reorders or edits lines. Store a short stable ID in each item, such as a trailing `^t-01HX…` block reference, an HTML comment, or a front-matter list, and fall back to content matching when IDs are missing (for example, a new line typed by hand). Without stable IDs, every reorder looks like a delete plus create and conflicts multiply.

**Laws to prove or test.** These are the lens laws, which become thesis invariants:

- **F1 (PutGet):** `decode(render(rows, c)) = rows`. Data the app writes reads back unchanged.
- **F2 (GetPut):** `render(decode(file)) = file`. A sync round trip never rewrites a file the user didn't change. This is what makes users trust two-way sync.
- **F3 (no lost update):** an edit made on disk while a remote change arrives is never silently overwritten. It is either merged or kept as a conflict copy. This is Harmony's "never overwrite changes".
- **F4 (crash recovery):** after any crash point, the scan restores `fs_entry` consistent with disk and the DB, with no lost and no duplicated mutations.
- **F5 (echo freedom):** a quiescent system performs no further writes. Without this, devices ping-pong edits forever.

**Platform realities** belong in the codec and mount layer, not in the sync core:

- Case-insensitive and Unicode-normalizing filesystems (APFS, NTFS).
- Reserved Windows names (`CON`, `NUL`) and characters, and the 260-character `MAX_PATH` limit unless long paths are enabled.
- Symlinks, stored as link entries and never followed.
- Ignore rules (`.skkilignore`).
- Large-file placeholders. Files-on-demand needs the OS APIs: Apple File Provider, which versions content and metadata separately and passes a `baseVersion` on conflict ([Apple](https://developer.apple.com/documentation/fileprovider/nsfileprovideritemversion)); the Windows Cloud Files API; or FUSE on Linux. That part is future work.

**Remote side.** `file` rows and their tree sync to Postgres like any other row (`files(id, parent_id, name, path_key, content_hash, size, mime, mtime, server_version, deleted_at)`), and their bytes go through the **media service**. This is where A becomes central rather than incidental: every opaque file on every mount is a media object, with leases, claims and fences. For example, a screenshot deleted on disk on one device while another device is still uploading it is exactly A's dual-write race, now arising from the filesystem.

**What the app declares per mount** (sketch):

```yaml
mounts:
  screenshots:            # clip
    path: ~/Pictures/Screenshots
    table: clip
    codec: opaque
    filter: "*.png"
    direction: both
    delete: { disk_to_db: tombstone, db_to_disk: trash }   # never hard-delete user files
    conflict: copy
  projects:               # time
    path: ~/Documents/Tasks
    table: task
    codec: markdown_tasklist   # line ↔ task, heading ↔ section, file ↔ project
    id_marker: block_ref
    direction: both
    conflict: merge_rows
```

The declarations make deletion propagation explicit for each direction. Deleting a clip in the app should move the PNG to the OS trash, not unlink it. This is a policy the engine enforces, not something each app codes.

### Local crash consistency: file plus row in one step

A mutation that adds a file, such as an image clip, has to survive a crash between "file written" and "row committed". The standard ordering is this:

1. Write the bytes to `blobs/tmp/<random>`, `fsync` them, and hash while writing.
2. `rename` the temp file to `blobs/sha256/<hash>`, then `fsync` the directory. If the target already exists, delete the temp file, which gives dedup for free.
3. In **one SQLite transaction**, insert the domain rows (`clip`, `blob_ref`) *and* the mutation record in a local outbox table (`pending_mutation`).
4. On startup a reconciler deletes CAS files that no row references (after a grace period). It also marks rows whose CAS file is missing as `missing` so the engine re-fetches them.

A crash can then only leave unreferenced bytes behind, never a row pointing at nothing. This is the local equivalent of A's I1 ("a committed reference implies the bytes exist") and of the transactional outbox that A already uses on the server. The order of upload versus push follows S1 from the hybrid section. Either the blob is uploaded through the media service *before* the mutation that references it is pushed, or the mutation is pushed with the reference in a `pending` state that other devices can display. A's lease and fence then decide whether a late upload is still allowed.

### Transactions end to end: SQLite → Postgres → other devices

"Transaction" means three different guarantees, and engines differ on each:

- **Client → server atomicity.** PowerSync hands your upload code one *local transaction* at a time (`getNextCrudTransaction`), and you call `complete()` only after the backend has applied all of it ([PowerSync](https://pub.dev/documentation/powersync/latest/powersync/PowerSyncDatabase/getNextCrudTransaction.html)). Zero runs each mutator inside one server database transaction and retries up to 5 times. On failure it reverts the optimistic change on the client ([Zero](https://zero.rocicorp.dev/docs/mutators)). Electric + TanStack DB return the Postgres transaction ID (`pg_current_xact_id`) from your write API, and the client waits until that txid streams back ([TanStack DB](https://tanstack.com/db/v0/docs/collections/electric-collection)).
- **Server → client atomic visibility.** Another device must never see half of a transaction, such as a task moved without its subtasks. You only get this where the server keeps **one ordered change log with checkpoints**: PowerSync checkpoints, Linear's `lastSyncId` ([reverse-linear](https://github.com/wzhudev/reverse-linear-sync-engine)). cr-sqlite can spread one transaction across several `db_version`s ([vlcn](https://vlcn.io/blog/how-crsqlite-transactions-work-today)). Turso Sync resolves per row with last push wins ([Turso](https://docs.turso.tech/sync/conflict-resolution)). Electric guarantees consistency per single-table shape ([Electric](https://electric-sql.com/docs/guides/shapes)).
- **Conflict handling inside a transaction.** SQLite's own session extension shows the full taxonomy: `changeset_apply` runs inside a savepoint and calls a handler for DATA, NOTFOUND, CONFLICT, CONSTRAINT and FOREIGN_KEY conflicts, which can return OMIT, REPLACE or ABORT ([SQLite](https://www.sqlite.org/session/sqlite3changeset_apply.html)). This is a useful vocabulary for your engine's rejection API.

In theory terms, the target for personal apps is a **monotonic atomic view**: never see part of a transaction, and never go backwards. That is a highly available transaction guarantee and needs no coordination ([Bailis et al., VLDB 2014](https://www.vldb.org/pvldb/vol7/p181-bailis.pdf)). Add a single server-assigned order on top and you get Replicache-style rebase ([Replicache](https://doc.replicache.dev/concepts/how-it-works)). Serializable offline transactions are impossible by I-confluence, and you do not need them.

**Remote relational mapping** (Postgres, though any database with transactions works):

- Every synced table gets four columns: a client-generated `id`, `user_id`, `server_version bigint`, and `deleted_at`. Add per-field HLC columns, or one row HLC, only where field-level LWW is used.
- Assign `server_version` from a **per-user counter updated inside the mutation's transaction**. This serializes writes per user, which is cheap for personal data. It also avoids the known pitfall of global sequences committing out of order, which makes cursor-based pulls skip rows.
- Store a `mutation(client_id, mutation_id, status, result)` table. Retries are then idempotent: Todoist's Sync API makes each command safe to retry with a `uuid`, maps `temp_id`s to real IDs, and returns a status per command ([Todoist](https://developer.todoist.com/api/v1/)).
- Use `DEFERRABLE INITIALLY DEFERRED` foreign keys and apply each client's mutations in client commit order. A child created offline right after its parent then never fails the FK check.
- When the server rejects or rewrites a mutation (unique violation, cycle, authorization), answer with **2xx plus `status: rejected|rewritten` and the authoritative rows**, not a transport error. PowerSync recommends exactly this, because a non-2xx error blocks the whole upload queue ([PowerSync](https://docs.powersync.com/handling-writes/handling-write-validation-errors)). The client reverts to server state and records an entry in a local "sync issues" list.

### Conflicts when the only collaborator is yourself

With personal data, concurrency comes from **offline windows**, not simultaneous typing. Most field conflicts are therefore rare, and the same person sits on both sides, so silent LWW is acceptable for most scalar fields. The dangerous conflicts are *structural*, and here both codebases already have specific traps.

**`clip` conflict plan (almost everything commutes):**

| Data | Rule | Why |
|---|---|---|
| Clip identity | `id = HMAC(user_key, normalized content)` rather than a random UUID | Copying the same text on two devices yields one row, so dedup comes from identity itself. A *keyed* hash keeps the E2EE server from confirming guesses about content. Paste's iCloud sync had to work around duplicate clips from sync races ([Paste](https://pasteapp.io/help/icloud-sync-doesn-t-work)). |
| `last_copied_at`, `copy_count`, `devices` | max-register, per-device counter, grow-only set | "Copy again bumps the timestamp" (`clip/AGENTS.md`) becomes a commutative merge with no conflict. |
| `pinned`, tags | LWW register (HLC) / OR-set | Rare, low stakes. |
| Delete vs. pin/edit | tombstone with HLC; **a later pin revives the clip** | An explicit user delete is intent, so both resolution directions must be declared. |
| "Keep last N" retention | **local eviction only, never a synced delete**; pinned items exempt; cloud retention is a separate server policy | If every device synced deletes from its own "last N", the union of evictions would wipe everything. |
| Images / files | CAS + media service blob; thumbnails eager, originals lazy on mobile | The media service's lease and claim layer applies directly. |
| Sensitive / concealed clips | never enter the outbox | This is a privacy invariant, enforced before sync. |

`clip` is close to an ideal I-confluent workload. Writes are high-rate and append-mostly, with no cross-row invariants, so it needs neither server validation nor CRDT text. It is the right first case study, because it exercises the blob path heavily and the conflict path lightly.

**`time` conflict plan (where the hard cases are):**

| Code today | What breaks under sync | Fix |
|---|---|---|
| `position` is a dense integer 1..n, and every move renumbers *all* open siblings (`features/tasks.rs:263-310`) | One drag rewrites N rows. Two offline reorders conflict on every sibling and LWW scrambles the list. | Switch to fractional index keys: one row changes per move ([Figma](https://www.figma.com/blog/realtime-editing-of-ordered-sequences/)). Todoist moved to an `order_key` for the same reason. For concurrent inserts at the same spot, Fugue avoids interleaving ([arXiv 2305.00583](https://arxiv.org/abs/2305.00583)). |
| Completing a recurring task advances `due_date` in place, outside a transaction (`tasks.rs:422-434`) | Completing the same occurrence on two devices advances twice and silently skips an occurrence. | Make it the idempotent mutation `complete(task_id, occurrence_date)`. Apply it only if `due_date == occurrence_date`; otherwise it is a no-op. If you ever materialize instances, use a deterministic ID `(series_id, date)`. |
| `parent_id` cycle check and `MAX_DEPTH = 3` (`tasks.rs:172-207`, `projects.rs:14`) | Not I-confluent. Offline "move A under B" and "move B under A" each pass locally, then form a cycle. | Server-ordered move that rejects or rewrites the loser ([Kleppmann tree-move](https://martin.kleppmann.com/2021/10/07/crdt-tree-move-operation.html) is the CRDT alternative). |
| Completion cascades to descendants (`tasks.rs:435-452`) | Device B adds a subtask offline while device A completes the parent, leaving an open child under a closed parent. | Declare it: either re-open the parent, or auto-complete late children. Either way it is a *deterministic* rule, not an error. |
| At most 3 musts per day (frontend only) and 3 open outcomes per week (`review.rs:450-485`) | Bounded-count invariants are not I-confluent. Two devices offline can each add one and end with 4. | Treat them as **soft invariants**: accept the merge, flag the violation, and ask the user to pick. For one person this is better UX than rejection. Escrow (give each device a share) is the hard alternative. |
| Outcome carry / uncarry and project-archive cascade (`review.rs:417-447`, `projects.rs:193`) | Multi-row edits whose halves could sync separately. | One named mutation each, applied atomically on both ends (see the transactions subsection above). |
| `running_timer` is a singleton row (id = 1) | Starting timers on two devices means one silently wins and a time entry is lost. | Key it per device, or LWW with a "you had a timer running on X" notice. Also check `time_entry` overlap after the merge. |
| `habit_log` auto rows from ActivityWatch | Each device derives its own auto logs. | Store them per `(habit, date, source_device)`. The existing rule "manual beats auto" becomes the merge. |
| Planned Google Calendar mirroring | If each device pushes events, you get duplicates. | Make it a **server-side outbox effect** after the authoritative commit, idempotent on `external_id`. It must never run from a client. |
| Weather, derived "blocked" state, virtual recurrence occurrences | Nothing breaks, because these are derived. | Do not sync them. Recompute locally. |
| `localStorage` UI preferences | Per-device by design. | Declare them as `scope: device`. |

`time` is the right second case study for the opposite reason: it exercises every structural conflict class (order, tree, recurrence, bounded counts, singletons, external effects) and almost no blobs. Together the two apps cover the space far better than either alone.

### E2EE is opt-in: one protocol, two trust modes

Both specs plan end-to-end encryption, and users can opt out. That means the engine must not fork into two sync systems. The "imperative server-side writes" pattern (Zero, PowerSync, Linear) assumes the server can *read* rows to check cycles, limits or uniqueness. An E2EE server cannot, and pure LWW in the style of Evolu or Actual Budget ([James Long](https://jlongster.com/using-crdts-in-the-wild)) has nowhere to enforce those rules. The design that covers both modes with one code path is the following.

**Every mutation is validated by deterministic replay of one server-ordered log.** The server always assigns the total order per user. The validator is one shared module (for example Rust compiled to native and WASM) that checks cycles, limits, recurrence and so on. It runs on whichever replicas can read plaintext:

| | Standard mode (default) | E2EE mode (opt-in) |
|---|---|---|
| What the server stores | Plaintext rows in Postgres + mutation log | Ciphertext mutation log + plaintext *metadata* only (IDs, sizes, hashes, `server_version`) |
| Who runs the validator | **Server**, as a trusted replica: decides accept or reject at commit and answers with the 2xx status protocol | **Every client**, replaying the same log; decisions are identical by determinism |
| Structural checks (reference integrity, claims, leases, quotas, idempotency) | Server | Server, because they work on opaque IDs and metadata |
| When a rejection is known | At upload (fast) | At replay, after the next pull |
| Server-side features | Calendar mirroring, server search, web app without a key, share-link previews | Disabled, or run by a designated device, or the user grants the server a key for one scope only (for example "calendar blocks only") |
| Blob dedup key | `sha256(plaintext)` per user | Keyed hash `HMAC(user_key, plaintext)`; the object store sees only ciphertext |

Because the server in standard mode runs the *same* validator over the *same* order, it reaches the decisions every client would reach by replay. The server's verdict is therefore just an early answer, never a different one, and clients can keep replay enabled in both modes as a self-check. Switching modes is a migration of the log, not a change of protocol. Opting in means the client encrypts a snapshot plus the future log and the server drops its plaintext tables. Opting out means the client uploads a decrypted snapshot and the server rebuilds its tables. The bytes in the media service are re-encrypted lazily.

The cost is determinism discipline. Validators may not read clocks or randomness (the mutation carries its own HLC and IDs), validation code must be versioned, and a change of rules is a schema-evolution problem. In return, none of the surveyed products has this design: Zero, PowerSync and Electric assume a plaintext server, and Evolu and Actual assume LWW only. It gives the thesis a clear, testable claim: the deterministic simulator replays the same log on N simulated clients plus the server and asserts that they reach identical states.

### Does the report's design handle these data types?

| Requirement | Covered before? | Now |
|---|---|---|
| Files on disk + rows in local SQLite | Partly (blobs only) | CAS for app-owned files. **Two-way mounts** for user folders: opaque and structured codecs, an `fs_entry` merge base, echo suppression, an intent record for crash recovery, and lens laws F1–F5. |
| Multi-row local transactions | No | One SQLite transaction holds the domain rows, the `fs_entry` update and the outbox mutation. The server applies each mutation in one Postgres transaction, and an ordered log gives atomic visibility. |
| Remote relational DB | Postgres assumed, not designed | Column contract (client ID, per-user `server_version`, tombstone, HLC), mutation table, deferred FKs, 2xx rejection protocol, `files` tree. Any transactional store can replace Postgres. |
| Personal-data conflicts | Generic only | The per-app conflict tables above. File conflicts reduce to row conflicts for structured files and to conflict-copy rows for opaque files. |
| Collaboration later | Yes (opaque CRDT fields) | Unchanged: task `notes` can later become a Yjs/Automerge field. Shared projects add authorization, and in E2EE mode they add the group key-management problem Keyhive works on. |
| E2EE | No | Opt-in mode of the same protocol. The validator runs on the server or on the clients, while structural checks always stay on the server. |

The minimum viable engine for both apps now consists of:

- the local outbox;
- the CAS file layer;
- two-way mounts with opaque and structured codecs;
- named mutations with per-user server order;
- per-field merge declarations;
- one deterministic validator that can run on the server or the client;
- the media service as the blob plane.

The thesis invariants S1–S5 still apply, together with F1–F5 above and one more. **S6: every replica that has processed the same log prefix — server in standard mode, or any client — holds identical state and identical accept/reject decisions.**

**Revised thesis framing.** Two contributions are novel and testable: two-way, lens-law-checked sync between filesystem and relational rows (F1–F5), and dual-mode deterministic validation (S6). Together with A's blob integrity (S1, S2, S5), they form one coherent system. All three are too much for four months. One realistic cut covers the opaque codec, S1, S2, S5, S6 and one structured codec for `time`, tested by simulation. The full declarative surface and E2EE key management would be future work.

## Conclusion

The question "how much of sync can be one module" has a sharper answer than it first appears. Mechanics abstract almost completely, semantics abstract into about ten declarations, and three things never abstract: invariant content, real-world compensation, and rejection UX. The market's retreat to imperative write handlers shows that the declarable middle tier is the unclaimed ground, and blobs sit in that tier unaddressed. That changes how to read your media service. As a standalone thesis it is careful engineering on a folklore pattern. As the blob plane of a sync engine, its leases, claims and fence are the missing pieces of an open problem, because offline devices are just untrusted clients holding capabilities for a very long time.

The practical risk is time, not ideas. A thesis that finishes A's measured core and adds offline reference/rejection semantics with simulation evidence is stronger than either an unfinished general engine or a polished but incremental media service. It also leaves the declarative compiler, schema lenses and partial-replica revocation as a clear follow-on roadmap toward the enterprise engine.
