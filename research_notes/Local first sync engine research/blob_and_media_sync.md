# Blob and Media Sync: File Sync Systems, Attachments in Sync Engines, and Bridging the skkil Media Design

Research date: 2026-11-04. Sources were accessed on that date unless noted. Where I could only verify search snippets and not full pages, the finding says so.

## Q1. How do file sync and content-addressed systems sync large binaries?

### Takeaway
Mature file sync systems all split **metadata/namespace sync** from **content/block sync**. Content is stored as immutable, hash-addressed chunks: fixed-size in Dropbox and Syncthing, content-defined in dedup tools, and BLAKE3-tree-verified in Iroh. The metadata layer tells the block layer what to move. Correctness effort goes almost entirely into the metadata/tree model.

### Cited Findings
- **Dropbox Nucleus (desktop sync rewrite).** Shipped to all users in March 2020 after about 4 years of work. The main motivation was Sync Engine Classic's data model. — [Rewriting the heart of our sync engine, dropbox.tech, 2020](https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine)
- Nucleus keeps three trees: a **Remote Tree** (cloud state), a **Local Tree** (last observed disk state), and a **Synced Tree** (last known fully synced state, used as a merge base). Nodes have unique IDs rather than paths, so a move is a single attribute update. The protocol rules out invalid states, such as a file existing without its parent. — [Testing sync at Dropbox, Isaac Goldberg, 2020-04-20](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)
- Nucleus testing:
  - Nearly all logic runs on one control thread, which makes it deterministic.
  - Tens of millions of seeded randomized tests run nightly.
  - **CanopyCheck** tests the planner. It perturbs one random tree to produce the other two, then checks three invariants: termination (an iteration cap of 200), no panics, and that a server-only file ends up in all three trees.
  - **Trinity** tests concurrency with a mocked filesystem, network (reordering, delays, failures) and time, acting as a custom Rust Future executor.
  - The post does not describe the block-level design. — [Testing sync at Dropbox, 2020-04-20](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)
- Dropbox's sync protocol has a metadata sub-protocol and a block sub-protocol. Blocks are aligned chunks of up to 4 MiB, and metadata tells the block engine which blocks to upload or download. This detail is from search-result summaries of the Broccoli post, not a full read. — [Broccoli: Syncing faster by syncing less, dropbox.tech](https://dropbox.tech/infrastructure/-broccoli--syncing-faster-by-syncing-less)
- **Magic Pocket** (Dropbox's own blob store):
  - Files are split into blocks of up to 4 MB, keyed by SHA-256, and compressed and encrypted.
  - Blocks are grouped into 1 GB buckets, replicated first, then erasure-coded.
  - Blocks are immutable, which lets the system move them freely.
  - At its core it is a very large key-value store of blobs.
  - [Inside the Magic Pocket, dropbox.tech, 2016-05](https://dropbox.tech/infrastructure/inside-the-magic-pocket); [InfoQ, Magic Pocket exabyte storage](https://www.infoq.com/articles/dropbox-magic-pocket-exabyte-storage); [SNIA SDC 2024 slides](https://www.snia.org/sites/default/files/2025-05/SNIA-SDC2024-Ummadi-Shobe-Scaling-Magic-Pocket-Exabyte-Scale.pdf)
- **Syncthing BEP v1.** Files are split into power-of-two blocks from 128 KiB to 16 MiB. The chosen size is the smallest that gives fewer than 2000 blocks.
  - `FileInfo` carries a block list (offset, size, hash), a version vector (device ID plus counter), and a sequence number.
  - `Index` and `IndexUpdate` messages sync metadata. `Request`/`Response` fetch byte ranges, optionally with the expected hash.
  - `DownloadProgress` and a `from_temporary` flag let peers serve blocks from partially downloaded temp files.
  - [Syncthing BEP v1 spec](https://docs.syncthing.net/specs/bep-v1.html)
- **FastCDC** (Xia et al., USENIX ATC 2016) is Gear-hash content-defined chunking with three techniques: simplified hash judgment, skipping cut-points below the minimum chunk size, and normalized chunking. It reports about 10x the speed of Rabin-based CDC and about 3x Gear/AE-based CDC, with near-equal dedup ratio. The speed figures are from abstract summaries; I did not check them against the PDF. — [USENIX ATC'16](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia); [PDF](https://ranger.uta.edu/~jiang/publication/Conferences/2016/2016-USENIX-ATC-FastCDC-%20Design%20of%20a%20Fast%20and%20Efficient%20Content-Defined%20Chunking%20Approach%20for%20Data%20Deduplication.pdf); [fastcdc Rust crate](https://docs.rs/crate/fastcdc/4.0.0/source/src/v2016/mod.rs)
- CDC has now moved into data formats too: Apache Arrow's Rust Parquet writer has a CDC chunker. — [arrow-rs parquet cdc.rs](https://arrow.apache.org/rust/src/parquet/column/chunker/cdc.rs.html)
- **Iroh blobs.**
  - Blobs are named by their 32-byte BLAKE3 root hash.
  - A separate "outboard" file stores the chunk-tree hashes (1 KiB chunks by default, about 6% overhead). It enables verified streaming, verified range downloads and resumption, and both sender and receiver check each chunk.
  - "Hash sequences" (concatenated 32-byte hashes) are the only way to relate blobs, and are used for collections.
  - **Tags** pin blobs against garbage collection.
  - [iroh docs, Blobs protocol](https://docs.iroh.computer/protocols/blobs)
- **Git LFS.**
  - The repository stores a small text pointer (`version`, `oid sha256:…`, `size`), and the bytes live externally.
  - Clients POST to `/objects/batch` with OID and size; the hash defaults to SHA-256.
  - Per-object errors are returned inside a 200 response.
  - An optional **verify** action confirms the server holds the OID and size after upload.
  - [git-lfs batch API spec (Debian copy of v3.7.1)](https://sources.debian.org/src/git-lfs/3.7.1-1/docs/api/batch.md); [spec.md mirror](https://demo.gitea.com/lunny/git-lfs/raw/commit/07be1f54e1f1d239f42783c83bb8552a4298d6b4/docs/spec.md); [sageox/ox Go LFS client](https://pkg.go.dev/github.com/sageox/ox/internal/lfs)
- Git itself has a design document proposing that large blobs live on "promisor remotes" instead of LFS, which moves lazy blob fetch into core Git. — [large-object-promisors.adoc](https://code.googlesource.com/git/+/1b93acd13a18e8a08c8255fa193b62ed53f29ee7/Documentation/technical/large-object-promisors.adoc)

### Inferences
- The pattern repeats everywhere: *immutable hash-named content* plus *a mutable metadata structure that references it*. All conflict resolution happens in the metadata. The block layer only needs "eventually present and verified".
- Git LFS's "verify" action is the same idea as the skkil commit step (HEAD the object, check size and checksum): the server confirms the bytes exist before the reference is trusted.
- BLAKE3/bao verified streaming is a stronger version of the skkil whole-object `sha256` check. It allows chunk-level verification and resume for multipart uploads and peer-to-peer fetch.

### Gaps
- I did not fetch primary sources for Google Drive's desktop sync design, iCloud Drive internals, restic/borg chunking parameters, or Perkeep's blobref/schema model. Claims about them are omitted.
- The Broccoli post's block details are from snippets only.

## Q2. How do structured-data sync engines handle attachments?

### Takeaway
Nearly every engine **keeps blobs out of the replicated data model**. The row or document holds a small token (a hash or ID plus metadata), and bytes move on a separate, lazy, resumable channel. That channel is usually a separate queue the app wires up itself (PowerSync, tldraw, Firestore/Storage). Only a few systems make the blob a first-class synced object: Jazz FileStream, Ditto, and Couchbase Lite. Even these keep blob transfer separate from document transfer.

### Cited Findings
- **PowerSync**:
  - Attachment support moved into each SDK (alpha) and replaced the deprecated `@powersync/attachments` package. Minimum versions include Web v1.33.0 and Flutter v1.16.0.
  - The attachment table is **local-only**, not synced. Its states are `QUEUED_UPLOAD`, `QUEUED_DOWNLOAD`, `SYNCED`, `QUEUED_DELETE` and `ARCHIVED`.
  - `saveFile()` writes locally and, through an `updateHook`, links the attachment to app data **in the same local transaction**. Other devices see the reference via `watchAttachments` and queue a download.
  - Unreferenced files are archived; `archivedCacheLimit` deletes the oldest when the limit is hit.
  - `verifyAttachments()` on startup repairs `localUri` values, archives records whose file is missing, and re-queues downloads.
  - Error handlers return retry or archive. Files never uploaded are lost on migration.
  - [PowerSync docs, Attachments/Files](https://docs.powersync.com/usage/use-case-examples/attachments-files)
- PowerSync's blog says posts appear immediately through local-only tables, so incomplete posts don't sync to other users. — [PowerSync blog, offline-first file uploads](https://powersync.com/blog/building-offline-first-file-uploads-with-powersync-attachments-helper.md)
- **Couchbase Lite**:
  - Blobs live in a separate content-addressable store, indexed from the document by a `digest` (SHA-1). They are loaded on demand.
  - The replicator adds an `_attachments` dictionary.
  - Sync Gateway accepts blobs up to 20 MB. A larger blob is not replicated even though its document is.
  - Deleting a document leaves the blob until `performMaintenance(.compact)` collects unreferenced blobs.
  - [Couchbase Lite Swift, Blobs](https://docs.couchbase.com/couchbase-lite/current/swift/blob.html)
- **Ditto**:
  - The document holds an attachment token (`id` = content hash, `len`, metadata). Attachments are immutable once linked; to change one, you repoint the token.
  - Blobs sync asynchronously over a separate protocol from documents, lazily (the app must fetch). Transfers resume, possibly from another peer.
  - Big Peers (servers) may hold the token without the blob.
  - Small Peers run reference-counted GC every 10 minutes; it does not run on Ditto Server. Attachments cannot be deleted directly.
  - [Ditto docs, Working with attachments](https://docs.ditto.live/sdk/latest/crud/working-with-attachments); [Ditto v4.5 Large Binary Files](https://docs.ditto.live/v4-5/best-practices/large-binary-files)
- **Jazz**: FileStream (internal `_type: "BinaryCoStream"`) is a CoValue.
  - `createFromBlob` chunks automatically; large files can be pushed in chunks (for example 100 KB) and then `end()`ed.
  - `waitForSync` waits for the stream to reach connected peers.
  - `loadAsBlob({allowUnfinished})` reads partially synced streams.
  - The blob therefore sits **inside** the same permissioned, synced object graph.
  - [Jazz docs, FileStreams](https://jazz.tools/docs/react/using-covalues/filestreams); [Jazz API reference](https://jazz.tools/api-reference/jazz-tools)
- **CloudKit**:
  - `CKAsset` data is stored apart from the record but uploaded **in the same record save**.
  - An asset belongs to only one record.
  - Filename and metadata must be kept in record fields.
  - CloudKit doesn't delete the local file.
  - Server assets are removed by setting the field to nil; CloudKit then GCs the orphan.
  - Assets don't count toward the 1 MB record limit.
  - [Apple CKAsset docs](https://developer.apple.com/documentation/cloudkit/ckasset.md); [CKRecord](https://developer.apple.com/documentation/cloudkit/ckrecord.md)
- **tldraw sync**:
  - The app implements `TLAssetStore` with `upload(asset, file) → {src, meta?}`, `resolve` (which can add auth tokens or pick variants) and an optional `remove`. It is passed as `useSync({assets})`.
  - Without an asset store, files are inlined as base64, which is suitable only for prototypes.
  - [tldraw assets](https://tldraw.dev/sdk-features/assets); [TLAssetStore reference](https://tldraw.dev/reference/tlschema/TLAssetStore); [tldraw sync](https://tldraw.dev/docs/sync)
- **InstantDB**: files live in a `$files` namespace governed by the same permission rules as data, which can traverse relations. The example rule restricts create and delete to a path prefixed by the user ID. — [InstantDB Storage](https://www.instantdb.com/product/storage); [example repo](https://worker-github.zxx930427.workers.dev/jsventures/instant-storage-avatar-example)
- **ElectricSQL**: I found no attachment or file guidance. Electric syncs Postgres "Shapes" on the read path, with a Postgres backend as control plane. — [Electric docs, local-first](https://electric-sql.com/docs/reference/local-first)
- **Automerge**: I found no official guidance on large binaries. The automerge-repo storage adapter API is described as a binary key-value store with range queries that knows nothing about documents. This comes from an HN comment and a blog post, not docs. — [Automerge concepts](https://automerge.org/docs/reference/concepts/); [HN discussion](https://hn.jakechampion.name/item/38193640)

### Inferences
- There are three integration styles:
  - **(a) App-wired side channel**: tldraw, Firestore + Storage, Supabase, and Electric/Replicache by default.
  - **(b) Engine-provided side queue with local-only state**: PowerSync, CloudKit, Couchbase Lite, Ditto.
  - **(c) Blob as a native synced object**: Jazz FileStream, Iroh/Willow-style content addressing.
- None of the documented systems state an explicit guarantee about what other devices see while a referenced blob is still uploading. PowerSync hides this with local-only tables; Ditto and Couchbase Lite let the token arrive first, which creates a temporary dangling reference that apps must render as "pending".
- GC is weak or local everywhere:
  - Couchbase Lite needs manual compaction.
  - Ditto runs GC only on small peers.
  - PowerSync uses a count-based archive cache.
  - CloudKit uses opaque server GC.
  - None documents coordinated server-side GC that accounts for offline devices that may still reference a blob. This is exactly the "claims" problem in skkil §18.

### Gaps
- Replicache, Zero, Linear, Notion, Realm/Atlas Device Sync (deprecated by MongoDB, per prior knowledge; not verified here), Firestore + Cloud Storage and Supabase Storage attachment patterns were not fetched.
- Google Drive's internal design was not fetched.
- No confirmation found of Automerge or automerge-repo first-class blob support as of late 2026.

## Q3. Consistency between metadata rows and blobs offline: techniques

### Takeaway
The standard toolkit is:
- content-hash IDs (for dedup and verification);
- upload-before-publish ordering (bytes uploaded before the reference becomes visible to others);
- immutable blobs (to change one, repoint the reference);
- refcount or mark-and-sweep GC with a grace period;
- resumable transfer (tus, S3 multipart, BLAKE3 ranges);
- lazy fetch with a bounded local cache.

None of these is specified in an engine together with offline-device liveness for GC.

### Cited Findings
- **Resumable uploads.** The IETF httpbis draft "tus – Resumable Uploads Protocol" has editors from Transloadit, Apple and Cloudflare (draft -00, September 2022).
  - It is backwards-compatible with plain HTTP uploads.
  - It explicitly does **not** support parallel transfer of parts, unlike S3 multipart.
  - At IETF 123 (July 2025) the draft was presented as ready, with no open issues. I did not confirm whether it is now an RFC.
  - [draft-ietf-httpbis-resumable-upload-00](https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-resumable-upload-00); [IETF 123 slides](https://httpwg.org/wg-materials/ietf123/resumable.pdf)
- **Dedup by content hash** is used by Magic Pocket (SHA-256 blocks), Couchbase Lite (SHA-1 digest), Ditto (content-hash id), Iroh (BLAKE3) and Git LFS (SHA-256 OID). — sources in Q1/Q2.
- **Ordering.** PowerSync links the attachment row and the data row in one local transaction and uploads in the background. CloudKit uploads asset and record in one save. Couchbase Lite still replicates the document if the blob exceeds 20 MB, which leaves a dangling reference. — [PowerSync](https://docs.powersync.com/usage/use-case-examples/attachments-files); [CKAsset](https://developer.apple.com/documentation/cloudkit/ckasset.md); [Couchbase Lite blobs](https://docs.couchbase.com/couchbase-lite/current/swift/blob.html)
- **GC**:
  - Ditto: refcount, 10-minute cadence, small peers only.
  - Couchbase Lite: compaction.
  - Iroh: tags pin blobs.
  - PowerSync: archived-cache limit.
  - CloudKit: nil the field and the server GCs.
  - [Ditto](https://docs.ditto.live/sdk/latest/crud/working-with-attachments); [Couchbase](https://docs.couchbase.com/couchbase-lite/current/swift/blob.html); [iroh](https://docs.iroh.computer/protocols/blobs); [PowerSync](https://docs.powersync.com/usage/use-case-examples/attachments-files); [CKAsset](https://developer.apple.com/documentation/cloudkit/ckasset.md)
- **Lazy and partial fetch**: Ditto requires explicit fetch, Couchbase Lite loads on demand, Jazz has `allowUnfinished`, and Iroh supports verified range requests. — sources above.
- **E2E encryption at block level**: Magic Pocket encrypts blocks server-side. CYRUS (EuroSys, Chung, Joe-Wong, Ha) scatters erasure-coded fragments across multiple cloud providers so that no single provider can read the data, and detects concurrent-update conflicts client-side after the fact. It reportedly reached 99.999999% reliability across four providers, a figure from a thesis summary. — [CYRUS EuroSys PDF](https://www.andrew.cmu.edu/user/cjoewong/CYRUS_EuroSys.pdf); [Princeton record](https://collaborate.princeton.edu/en/publications/cyrus-towards-client-defined-cloud-storage/); [POSTECH thesis record](https://oasis.postech.ac.kr/handle/2014.oak/93497)

### Inferences
- Content-addressing plus E2E encryption conflict: hashing the plaintext enables cross-user dedup but leaks equality. Hashing the ciphertext (per-file DEK, as in skkil §10) loses dedup. This is a known tradeoff and a design decision to state.
- For an offline-first engine, "upload-before-reference" cannot be enforced across devices, because the reference is created offline. The realistic invariant is "a reference may be *pending*, and readers must handle a pending blob". This is similar to skkil's `pending` state, but on the reader side.

### Gaps
- I did not find a published spec or paper that defines the visibility states of a remote reference to a not-yet-uploaded blob in a sync engine.
- Cache and eviction policy studies for mobile local-first apps were not found.

## Q4. Academic work on blob/metadata and cloud sync correctness

### Takeaway
There is a small but solid literature:
- ViewBox (FAST'14): sync clients propagate local corruption and crash inconsistency to the cloud.
- Hughes et al. (ICST'16): formal, testable model of a file synchronizer; found surprising, data-losing behavior.
- Dropbox's CanopyCheck/Trinity: industrial deterministic testing.
- CYRUS: client-defined multi-cloud storage.

None of these studies **structured-row + blob** consistency inside a database sync engine.

### Cited Findings
- **ViewBox** (Zhang, Dragga, A. Arpaci-Dusseau, R. Arpaci-Dusseau; FAST'14, pp. 119–132).
  - Local corruption can propagate to the cloud and pollute every device's copy.
  - A crash can leave the local file and the cloud copy inconsistent.
  - The fix combines ext4-cksum (a checksumming ext4), a cloud helper that fetches correct data back, and a view manager that uploads consistent snapshots.
  - It was integrated with Dropbox and Seafile, with reportedly 5% or less overhead (overhead figure from secondary press).
  - [USENIX FAST'14](https://www.usenix.org/conference/fast14/technical-sessions/presentation/zhang); [UW ADSL page](https://research.cs.wisc.edu/wind/Publications/viewbox-fast14.html)
- **Mysteries of Dropbox** (Hughes, Pierce, Arts, Norell; ICST 2016, DOI 10.1109/ICST.2016.37).
  - The authors call it the first formal, testable model of the core behavior of a modern file synchronizer.
  - It uses QuickCheck with a technique for nondeterministic systems that doesn't need visibility of internal choices.
  - The model covers a single file with read, write and delete. It was applied to Dropbox, Google Drive and ownCloud, and found surprising behavior in two, including possible data loss.
  - [Chalmers record](https://research.chalmers.se/en/publication/232551); [PDF](https://www.cis.upenn.edu/~bcpierce/papers/mysteriesofdropbox.pdf)
- **Dropbox Nucleus testing** (CanopyCheck, Trinity, Heirloom), already cited in Q1. Heirloom tests against a real server and runs about 100x slower. — [dropbox.tech 2020-04-20](https://dropbox.tech/infrastructure/-testing-our-new-sync-engine)
- **CYRUS** (EuroSys): client-defined, multi-provider, erasure-coded storage with after-the-fact conflict detection. — [PDF](https://www.andrew.cmu.edu/user/cjoewong/CYRUS_EuroSys.pdf)
- **Dual-write and outbox prior art** is already mapped in the user's own design §0.1: transactional outbox, TCC/Seata fence, Epoxy (VLDB 2023), SQL/MED DATALINK, leases (Gray & Cheriton SOSP'89), and the SIGMOD 2022 study finding correctness issues in 53 of 91 application-level transactions. — /home/hyoungjoojin/Desktop/skkil/ARCHITECTURE.md §0.1, §26

### Inferences
- A thesis could reuse the methodology here directly. It could combine Hughes-style model-based testing or Dropbox-style seeded deterministic simulation with ViewBox-style fault injection, and target the **row ↔ blob ↔ claim** invariants (skkil I1–I6) instead of file trees.

### Gaps
- I did not re-verify other candidate papers, such as studies of sync-induced data loss in multiple clients or Dropbox's "Pocket Watch" verifier. They are omitted rather than cited.

## Q5. Is unified structured + blob sync with consistent offline references an open gap, and how does the skkil media design fit?

### Takeaway
It is a **real, under-specified gap**, but "nobody does it" is not accurate. Jazz, Ditto and Couchbase Lite ship integrated blobs, and PowerSync ships a queue. What is missing is a *specified and tested* consistency contract covering three things:
- (1) reference visibility before the bytes are durable;
- (2) GC that is safe against offline devices still holding references;
- (3) revocation and erasure that propagates to device caches.

The user's media design already solves the server half of (1)–(3). It becomes the engine's **blob plane**, with the claim layer (§18) acting as the engine's reference tracker.

### Cited Findings
- Integrated or semi-integrated systems exist: [Jazz FileStream](https://jazz.tools/docs/react/using-covalues/filestreams), [Ditto attachments](https://docs.ditto.live/sdk/latest/crud/working-with-attachments), [Couchbase Lite blobs](https://docs.couchbase.com/couchbase-lite/current/swift/blob.html), and [PowerSync attachments (alpha)](https://docs.powersync.com/usage/use-case-examples/attachments-files).
- PowerSync attachments are still labeled **alpha** in late 2026, and its attachment state is local-only per device. — [PowerSync docs](https://docs.powersync.com/usage/use-case-examples/attachments-files)
- Ditto GC runs only on small peers, and Big Peers can hold tokens without blobs. — [Ditto docs](https://docs.ditto.live/sdk/latest/crud/working-with-attachments)
- Couchbase Lite silently replicates documents whose blobs exceed 20 MB without the blob. — [Couchbase docs](https://docs.couchbase.com/couchbase-lite/current/swift/blob.html)
- The skkil design already has these pieces (/home/hyoungjoojin/Desktop/skkil/ARCHITECTURE.md):
  - `pending → active` commit with HEAD verification (§5, §6.1–6.2);
  - leases covering presigned and multipart capabilities, plus the store-side fence against late writes (§0.1, §6.6);
  - I1 no dangling reference, I4 no leak, I6 bounded late-write persistence (§13);
  - claims with sequence-guarded claim/release, an unclaimed timeout, leased claims, and governance deletion that revokes immediately (§18).

### Inferences
Mapping skkil onto a sync engine:

| Sync-engine need | skkil component |
| --- | --- |
| Device creates a reference offline | Engine creates a local `media` row in a **local-pending** state, plus a local outbox entry. On reconnect it calls upload intent with `idempotency_key = device:op` (§6.1), already supported. |
| Upload-before-publish | The engine sync server accepts the row mutation referencing `media_id` but marks it `blob_pending` to other readers until `media.committed` arrives over outbox→Kafka (§9). Alternatively, it gates publication on commit. |
| GC safe against offline devices | Claims become **engine-maintained**: each synced row that references M is a claim with `seq` = the row's version or HLC (§18.2). Leased claims model devices that might still be offline. The open research question is how long the lease or grace must be for devices offline for weeks. |
| Revocation and erasure | `media.deleted{reason}` maps to a tombstone in the sync stream. Device caches must evict. This extends I2 bounded revocation to device caches. |
| CRDT collaboration later | Blobs stay immutable and hash-named; a CRDT field holds `media_id`/hash. Concurrent "replace image" becomes a register conflict on the reference, never on the bytes. The loser's media loses its claim and becomes GC-eligible after its grace period. |
| Dedup, verified resume | Add BLAKE3/bao outboards or chunk hashes for part-level verification and P2P fetch (Iroh), and optionally CDC for large mutable files. |

- **Candidate thesis contribution:**
  - A sync engine where references and blobs share one consistency contract: explicit pending, committed and revoked reference states visible to readers.
  - GC coordinated with sync-state and device leases.
  - The existing invariants I1–I6, lifted from "server DB ↔ S3" to "N devices ↔ sync server ↔ object store".
  - Evaluated by deterministic simulation in the style of Dropbox Trinity/CanopyCheck and fault injection in the style of ViewBox.
  - This bridges the media-service work and the sync-engine thesis directly, without discarding the former.
- **Risk:** Jazz and Ditto already demonstrate "integrated blobs" in product form. Novelty must rest on the *specified invariants, offline-safe GC, revocation and the fence*, not on integration alone.

### Gaps
- No academic paper was found that defines or evaluates offline-safe blob GC or reference-visibility semantics for local-first or sync-engine systems. This is "not found", not proven absent.
- I did not verify whether Jazz documents GC or revocation for FileStreams, or how Zero, Replicache, Linear or Notion handle uploads (likely app-wired presigned URLs, but unverified).
