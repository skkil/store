# Novelty check: blob referential integrity and rejection semantics in offline-first sync (S1, S2, S3, S5)

Date of research: 2026-11-04. Tags: [F] = verified this session from a fetched or search-surfaced source; [M] = from model memory, NOT verified this session (treat as a lead to check, not a citation). Budget: ~18 tool calls, so several vendors stay unverified (see Gaps).

## Q1. Rejected offline writes: what do products do, and does anyone cascade compensation to dependent writes?

### Takeaway
Every product checked treats rejection **per write**: the server drops or 403s that one write, and the client goes back to server state, either through rebase or by reverting the write. No vendor doc found specifies **causal cascading** to later offline writes that depend on the rejected one, or cleanup of blobs the rejected write uploaded. Couchbase forum evidence shows the opposite: attachments were uploaded even though the doc push was rejected.

### Cited Findings
- [F] **PowerSync**: the backend should "respond with "success" (HTTP 2xx) even in the case of write conflicts or validation failures". There is no explicit rollback. Once the upload queue drains, the local DB matches the server, and if the server discards a change "the client will revert to the last known state". An optional server-side dead-letter queue "could result in out-of-order updates if the client continues sending updates". The page says nothing about dependent or later writes, or about attachments. — [PowerSync: Handling write/validation errors](https://docs.powersync.com/handling-writes/handling-write-validation-errors.md)
- [F] **Zero**: when a server mutator throws, "The mutation is skipped and the next mutation runs as normal. The optimistic mutation on the client will be reverted." The docs say nothing about dependent mutations. — [Zero mutators](https://zero.rocicorp.dev/docs/mutators)
- [M] Zero's docs (2025) say it does not support offline writes. Not re-verified this session.
- [F] **Couchbase Sync Gateway**: `throw({forbidden})` rejects the update "with an HTTP 403", and "the channel and access changes will not occur". `requireAccess()` rejects by throwing. The page does not describe client-side behavior or attachments. — [Sync Gateway sync function](https://docs.couchbase.com/sync-gateway/current/sync-function.html)
- [F] **Couchbase Lite (1.x era, 2015–16 forum threads)**: the push replicator treated a 403 as "sent", so no error callback fired. Couchbase said there was no fine-grained API to detect a rejected doc. The rejected doc stays local. One thread reports that **attachments were still uploaded although the document push failed with 403 "missing role"**. — [Couchbase forum: 403 missing role](https://www.couchbase.com/forums/t/403-missing-role-error/8018); [Google Group: pusher stops after 403](https://groups.google.com/g/mobile-couchbase/c/BktP7cnKQ_Q/m/sQAfMuW5BQAJ); [Android FORBIDDEN thread](https://couchbase.com/forums/t/couchbase-lite-android-replication-forbidden-error/13397). Current CBL 3.x behavior was not verified.
- [F] **Firestore**: "Local writes that eventually fail will be reverted in the local cache". The write promise rejects only "if still in memory", so after an app restart the app cannot learn the outcome. Rejected items vanish from listeners. This comes from community and course sources, not official docs. — [StackOverflow mirror](https://kaweah.freedombox.rocks/kiwix/stackoverflow.com_en_all_2023-11/questions/58840642/set-update-collection-or-document-but-only-locally); [Frontend Masters transcript](https://frontendmasters.com/courses/firebase/synchronization-offline-caching)
- [F] **Firebase Realtime Database**: offline writes are queued. If the auth token expires offline, writes pause until re-auth, "otherwise the write operations might fail due to security rules". — [Firebase RTDB offline](https://firebase.google.com/docs/database/android/offline-capabilities)
- [F] **Realm / Atlas Device Sync**: local writes always succeed, and the server later reverts permission-violating writes (compensating writes). Triggers include changed permissions and long-offline batches. — [MongoDB sync errors](https://mongodb.com/docs/atlas/app-services/sync/error-handling/errors)
- [M] Device Sync was deprecated in Sept 2024 with end-of-life in Sept 2025. Its compensating write reverts **only the offending object/changeset**. I recall no cascade, but this was not verified.
- [F] **Ditto**: permissions sit in device certificates. Each peer accepts or rejects reads and writes, and a peer won't forward documents it lacks write authority for ("mutual write permissions"). Nothing is said about what happens to the local copy of a disallowed write. — [Ditto data authorization](https://docs.ditto.live/auth-and-authorization/data-authorization); [legacy designing permissions](https://legacydocs.ditto.live/common/security/designing-permissions)
- [F] **Jazz**: the docs surfaced in search only describe roles (admin/manager/writer/reader/writeOnly). — [Jazz permissions skill](https://www.skills.lc/garden-co/jazz/garden-co-jazz-cursor-skills-jazz-permissions-security-skill-md)
- [M] **Jazz** design: transactions are signed. Every peer evaluates permissions when loading a CoValue and ignores transactions from accounts without the role, so a disallowed edit stays "invalid" everywhere rather than being rolled back. If that holds, it is per-transaction with no cascade, but it was not verified.

### Inferences
- An adversarial reviewer's best counter-argument is **rebase-by-re-execution** (Replicache, Zero, PowerSync-style "revert to server state, replay pending"). Later mutations re-run against authoritative state. If a mutator checks that its parent exists, it also fails, so this yields an *implicit* cascade. S2's novelty therefore cannot be "cascade exists at all". It must be:
  - (a) an explicit, specified dependency/causal closure over offline writes;
  - (b) one that covers **non-row side effects**: uploaded blobs, claims, and capabilities;
  - (c) one that works when dependents were pushed in other transactions or already seen by other devices.
- Couchbase's "attachment uploaded, doc rejected" report is concrete evidence of the S2 orphan-blob gap in a major product.

### Gaps
- Current (3.x / Capella App Services) Couchbase Lite client behavior on 403 and attachment handling. Not fetched.
- InstantDB, Triplit, LiveStore, Convex, Amplify DataStore, and CouchDB `validate_doc_update` were not checked this session.
- Jazz permission semantics for invalid transactions: no primary source retrieved.

## Q2. Attachments / blobs referenced by offline records: who guarantees no dangling references, and who does refcount GC with offline leases?

### Takeaway
Attachment support is **local-queue plus app-written logic** everywhere checked. PowerSync's built-in attachment queue is still **alpha**. It garbage-collects by **local** reference (archive, then an LRU limit) and gives no guarantee between rejected rows and uploaded files. CloudKit's record+asset model is the closest to S1, but only as atomic record save, not offline reference states. No product found does server-side refcount GC that respects **offline device leases**.

### Cited Findings
- [F] **PowerSync attachments**: all built-in SDK helpers are labeled **alpha** (Web ≥1.33.0, RN ≥1.30.0, Node ≥0.17.0, Flutter ≥1.16.0, Swift/Kotlin ≥1.0.0, .NET ≥0.1.2). The older `@powersync/attachments` packages are deprecated. — [PowerSync attachments](https://docs.powersync.com/client-sdks/advanced/attachments.md)
  - States are QUEUED_UPLOAD, QUEUED_DOWNLOAD, SYNCED, QUEUED_DELETE and ARCHIVED.
  - Rows store an attachment ID, and `watchAttachments` derives the reference set from the **local** DB.
  - Unreferenced files are ARCHIVED, and once `archivedCacheLimit` is exceeded the "oldest deleted when limit reached".
  - `updateHook` writes the row and the attachment record in one local transaction.
  - Uploads are always retried.
  - Nothing is said about ordering of the file upload relative to the row push, or about a backend rejecting a row whose file was already uploaded.
- [F] **CloudKit**: atomic `modifyRecords` is all-or-nothing per zone, and only if the zone supports atomic changes. Developers report CKAsset-bearing records failing intermittently. — [Apple modifyRecords](https://developer.apple.com/documentation/cloudkit/ckdatabase/modifyrecords(saving:deleting:savepolicy:atomically:).md); [isAtomic](https://developer.apple.com/documentation/cloudkit/ckmodifyrecordsoperation/isatomic.md); [SO thread](https://kiwix.lema.org/stackoverflow.com_en_all_2023-11/questions/45806010/cloudkit-modifyrecordscompletionblock-fails-randomly)
- [M] **CloudKit**: a CKAsset is uploaded as part of the record save, so a record never references a missing asset server-side (S1 holds by construction, online). Unreferenced assets are GC'd by Apple. There is no "pending" reference state visible to other devices. Not verified.
- [F] **Firebase Storage**: a resumable upload has an `UploadSessionUri` "valid for approximately one week" that can be passed back to resume. Whether the SDK persists it across app kill is undocumented in what was found, so it is likely app-managed. Storage and Firestore have no coupling, so a document can reference an unfinished upload. — [Firebase Unity UploadState](https://firebase.google.com/docs/reference/unity/class/firebase/storage/upload-state)
- [F] **Jazz FileStream**: `co.FileStream` / `createFromBlob(blob, {owner})` makes the file a CoValue owned by a Group. Offline or partial-stream semantics were not found. — [Jazz FileStreams](https://jazz.tools/docs/react/using-covalues/filestreams)
- [M] **Jazz**: a FileStream is a CoValue referenced by ID inside other CoValues, so references sync like data. Bytes stream in chunks, and a peer may hold a reference whose stream hasn't fully arrived ("not loaded" state). There is no GC of unreferenced CoValues. Unverified.
- [M] **Ditto attachments**: attachments are fetched lazily via an attachment token stored in a document. Ditto docs say unreferenced attachments are garbage-collected after some time (on the order of minutes or hours). This is a *local* GC rule with no offline-lease semantics. Unverified.

### Inferences
- **S1 is partially covered**: CloudKit (online atomic save), local transactional row+attachment records (PowerSync `updateHook`), and A's own I1 for the server. What remains open is a *specified* pending → committed → revoked reference state that **other devices** observe, plus convergence to no dangling references when an upload never completes.
- **S5 looks novel as a product feature.** GC is decided either by the local reference set (PowerSync, Ditto) or by server-side record references (CloudKit). Neither accounts for an offline device that holds a not-yet-pushed reference to an existing blob.

### Gaps
- Couchbase Lite blob GC semantics, InstantDB storage, Convex file storage, Supabase storage offline, tldraw/Excalidraw asset stores, Core Data external binary storage + CloudKit, and Android WorkManager patterns were not checked.
- Ditto attachment GC timing: no primary source retrieved.

## Q3. Academic prior art: referential integrity under weak consistency, distributed GC with leases

### Takeaway
Referential integrity under weak consistency is **well studied** for rows:
- IPA's "touch" repair (VLDB 2019).
- I-confluence: many FK constraints need no coordination (VLDB 2015).
- Causal consistency suffices for RI (PaPoC 2018).

Leases for distributed GC are textbook (Gray & Cheriton 1989; Java RMI DGC). None of the papers surfaced addresses **out-of-band large objects uploaded by untrusted clients**, **server rejection of offline writes**, or **revocation on partial replicas**.

### Cited Findings
- [F] **IPA** (Balegas et al., VLDB 2019): static analysis finds invariant-breaking concurrent operations and adds repair updates. Example: enrolling in a tournament also "touches" the tournament, so a concurrent delete loses (an add-wins extension). It covers referential integrity, uniqueness and aggregation inclusion via repairs, and numeric invariants via compensations. — [IPA PDF](https://www.dpss.inesc-id.pt/~rodrigo/vldb-2019.pdf); [arXiv 1802.08474](https://www.arxiv.org/pdf/1802.08474); [Morning Paper summary](https://blog.acolyer.org/2019/09/02/ipa/)
- [F] **I-confluence** (Bailis et al., VLDB 2015): many forms of foreign-key constraints are I-confluent, while primary keys in general are not. — [Coordination avoidance PDF](https://dcmpx.remotevs.com/org/bailis/www/PL/papers/ca-vldb2015.pdf)
- [F] **Shapiro, Bieniusa, Zeller, Petri (PaPoC 2018)** argue that causal consistency suffices to maintain RI, with CRDT pseudocode. — [arXiv 1803.03482](https://arxiv.org/pdf/1803.03482)
- [F] **Leases and RMI DGC** are already cited as prior art in ARCHITECTURE.md §0.1/§18.2 (Gray & Cheriton SOSP 1989; RMI dirty/clean with sequence numbers).
- [M] Related work to check:
  - Indigo, "Putting consistency back into eventual consistency" (EuroSys 2015): reservations/escrow for invariants.
  - AQL/AntidoteSQL: RI policies such as update-wins and delete-wins with cascade.
  - Conflict-free Replicated Relations (Yu & Ignat).
  - IPA's compensations.
  - Antidote SQL's "delete-wins with cascade" is the closest academic analogue to S2's causal cascade, but it handles *concurrent* deletes, not *server authorization rejection* of an already-applied local write.

### Inferences
- S2's "compensation cascades causally" has academic neighbors: AQL cascade policies, IPA compensations, Sagas. Frame S2 as **authority-driven rejection** rather than a concurrent conflict. A trusted server vetoes after the fact, which I-confluence analyses don't model: they assume all operations are eventually accepted. On top of that, S2 adds blob and claim side effects that can't be undone by state merge, and must be undone by fence plus reclaim.
- S5 is a direct application of leases. The novelty is binding leases to *offline replicas' pending references* and checking the result against the S1 oracle, not the lease itself.

### Gaps
- Full texts of Indigo, AQL and Conflict-free Replicated Relations were not opened. Exact cascade semantics are unverified.
- No search was done for papers on "garbage collection of blobs referenced by offline replicas" specifically. Absence is "not found", not proven.

## Verdict per invariant (for the report writer)

| Invariant | Verdict | Covered by | Remaining sub-claim |
|---|---|---|---|
| S1 no dangling blob ref after convergence | **Partially done** | CloudKit atomic record+asset (online) [M]; PowerSync local row+attachment transaction [F]; A's I1 (server) | A cross-device *pending* reference state, plus convergence when an offline upload never completes or is fenced |
| S2 rejected write: no orphan blob or dependent row; causal cascade | **Mostly novel** (rows only partially) | Per-write revert in PowerSync/Zero/Firestore/Realm [F]; implicit cascade via rebase re-execution [inference]; AQL cascade on delete [M] | An explicit causal closure over offline dependents, including already-pushed or observed ones, extended to uploaded blobs and claims. The Couchbase "attachment uploaded, doc rejected" report [F] shows the gap |
| S3 bounded revocation on partial replicas | **Likely novel as a stated bound; weakest evidence** | Ditto "never synced" if not permitted [F]; Jazz key rotation [M] | A time bound for evicting already-cached data and blobs on devices; not found in any doc checked |
| S5 offline-claimed blob not reclaimed before lease expiry | **Novel application of old mechanism** | Leases (1989), RMI DGC; local-only GC in PowerSync/Ditto | Server GC that respects offline devices' unpushed references via leases, verified by oracle |

Caveat: verdicts are "not found in sources checked within ~18 calls", not proof of absence. Several [M] items need primary-source confirmation before a thesis claims the gap: Jazz invalid-transaction semantics, Ditto attachment GC, CloudKit asset GC, Zero no-offline-writes, and Device Sync EOL.
