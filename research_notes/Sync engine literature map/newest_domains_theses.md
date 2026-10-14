# Newest items, domain references and theses (as of 2026-11-04)

Link tags: [F] = fetched or confirmed in search results this session; [M] = from memory, not verified this session. Claim IDs follow the verdict table in `reports/Sync engine thesis novelty check.md`. Many domain references (geodiff, QFieldCloud, Esri, tus, Resolve, LucidLink, tldraw, Figma, Hocuspocus) are already cited [F] in that report's "Extra domains" section. They are listed here briefly so the appendix is complete, not because they were re-verified.

## Newest (May–November 2026)

### Takeaway
The one serious new threat is **Jazz 2.0's normative SPEC** (garden-co/jazz, alpha, last edited 2026-11-04). It specifies a SQLite relational local-first database with authority-side serializable validation, **rejection that cascades to writes built on the rejected one**, large `bytes` values stored as content-addressed chunk trees committed inside row transactions, **upload leases and receipts with refcount GC**, an S3/R2 chunk store, and E2EE envelope formats. It covers R+B+V, with E as formats only. Its E2EE-plus-validation interplay is an open question, and it has no folder replica. S2, S5 and C8 must be re-scoped against it. The rest of the 2026 academic output (PaPoC 2026, arXiv) is CRDT and verification work that does not touch the surviving claims.

### Cited Findings

#### ⚠ THREAT — Jazz 2.0 SPEC: "the design and the contract"
- garden-co (Anselm Eickhoff et al.) · Normative spec in repo (alpha) · Chapters committed 2025–2026. `3_transactions.md` last changed 2026-10-27; `19_large_values.md` first committed 2026-08-23.
- Links: [README](https://github.com/garden-co/jazz) [F]; SPEC dir `crates/jazz/SPEC/` (`1_intro.md`, `3_transactions.md`, `7_authorization.md`, `19_large_values.md`, `INVARIANTS.md`, `E2EE_CRYPTO_FORMAT.md`, `D_testing_gates.md`), fetched via GitHub API [F]
- Summary: Jazz 2.0 is "a local-first, distributed, real-time database with full edit history, RLS-authorized sync, and two transaction kinds: eventually-consistent `mergeable` and `serializable`", with SQLite storage. The spec says "This document is both the design and the contract", and its invariants (INV-TX-*, INV-CONTENT-*) are linked to tests by a gate script. §3.8 "Rejection and cascade": "Rejection cascades only along bases … writes built on it … are rejected too (`predecessor_rejected`)", while other pending writes "settle by their own fates" (INV-TX-8). Rejected payloads are kept in retry storage on the origin (INV-TX-9). §19 large values: values over 64 KiB are content-defined chunk trees. Staged nodes are "held by an upload lease … or … a receipt until the commit referencing them lands or the staging maximum age passes", and "a commit whose lease expired meanwhile is refused". GC is trigger-maintained refcounts: "collection removes only epochs and nodes unreachable from every retained version, lease and receipt" (INV-CONTENT-8). On rejection, "the replica restores the settled version and replays its other pending writes" (§19.7). Leaf bytes can move to an S3/R2 chunk store and are hash-verified on read.
- Relevance: **S2**, partly pre-empted. A dependent-write cascade on rejection exists, and blobs live inside the row transaction, so a rejected write leaves no orphan by construction. **S5/C2**: upload leases, receipts and refcount GC now exist in a spec. **C8**: R+B+V (+E formats) in one product. The narrowed claims still standing: (a) Jazz's cascade follows *bases* only (same-row lineage), not semantic or FK dependents across rows, and not dependents already *seen by other devices*. (b) Its GC protects nodes held by *server-side* leases and receipts plus a local "unpushed mark", not references an offline device has not yet announced, and a lease expiry simply refuses the commit. (c) "Per-column encryption and authorization" is listed as an open design question in `7_authorization.md` §open questions, so the **S6** trust-mode switch is untouched. (d) It has no folder replica, so **F4/F5** are untouched. Jazz is also explicitly alpha ("Releases are currently locked to the alpha prerelease channel"). The thesis must cite it and state the differences.

#### PaPoC 2026 program (EuroSys workshop, 27 Apr 2026)
- PaPoC organizers · Workshop program · April 2026
- Link: [program](https://papoc-workshop.github.io/2026/program.html) [F]
- Summary: Keynotes by Guerraoui ("Genuine State Machine Replication") and Sivaramakrishnan ("Push-Button Verification for RDTs"). Papers include ERA (Dougal, group-management CRDT admin conflicts), Sal (multi-modal RDT verification, [arXiv 2603.27202](https://arxiv.org/abs/2603.27202)), sender-side causal delivery (Tong/Liittschwager/Kuper, [arXiv 2603.14690](https://arxiv.org/abs/2603.14690)), Bounding Byzantine Impact in Open CRDT Systems (Baquero et al.), ConflictSync, AegisSheet (spreadsheet CRDT), CobbleDB, and a lightning talk on formal verification of local-first access control (Jacob/Stuber/Hartenstein).
- Relevance: none of them addresses authority-driven rejection, blob GC, folder replicas or trust-mode switching. They are useful as "related, not overlapping" citations for **S6** (Byzantine bounds, verification) and **S3** (access control). No threat.

#### Towards System-Oriented Formal Verification of Local-First Access Control
- Jacob, Stuber, Hartenstein (KIT) · arXiv preprint · Apr 2026
- Link: [arXiv 2604.23560](https://arxiv.org/pdf/2604.23560) [F, listing only; contents not read]
- Summary: The longer companion to the PaPoC lightning talk, on verifying local-first access-control systems at system level. Only the reference list was visible in search.
- Relevance: **S3** (revocation on partial replicas) and the S6 validator; a possible model for the testing method. Unread, so threat level is unknown.

#### Local-First Conf 2026 (Berlin, 12–14 Jul 2026)
- Local-First Conf · Conference with talks on YouTube · Jul 2026; day-3 videos by 24 Sep 2026
- Links: [2026 archive page](https://www.localfirstconf.com/lfc2026) [F]; [recordings newsletter](https://buttondown.com/local-first/archive/talk-recordings-and-photos-for-2026/) [F]; [YouTube channel](https://www.youtube.com/@localfirstconf) [F, link only]
- Summary: Speakers include Martin Kleppmann (opening keynote), Steve Ruiz (tldraw), Paul Frazee (Bluesky), Seph Gentle, Armin Ronacher, Jeffrey Heer, Sunil Pai and Confidence Okoghenun (Cloudflare), Kegan Dougal, Brendan O'Brien & Rae McKelvey (n0/iroh), Matias Capeletto and others. Ink & Switch demoed Patchwork. The talk titles could not be retrieved: the schedule pages render client-side.
- Relevance: background only. Watch Kleppmann's keynote, the Cloudflare and Durable Objects talks (collab tier, C7) and the n0/iroh talk (blobs, C2) before submission. Unverified, so no threat assessed.

#### Turso Sync / offline SQLite guide
- Turso · Vendor guide · Jul 2026
- Link: [Turso blog](https://turso.tech/blog/building-local-first-apps-the-complete-guide-to-offline-first-database-sync) [F, search snippet]
- Summary: SQLite local reads and writes with background sync, custom conflict resolution, and a local sync server in the Turso CLI.
- Relevance: one more R-cluster competitor for the **C8** matrix. No blobs, folders or E2EE are mentioned.

#### Electric 1.4.x; Zero still 0.x; Automerge 3.2.x
- Vendors · Releases · 2026
- Links: [electric on Hex, 1.4.10, 26 Feb 2026](https://hex.pm/packages/electric/1.4.4) [F]; [Zero release notes, up to 0.22 seen](https://zero-docs-m9do7erzv-rocicorp.vercel.app/docs/release-notes/0.22) [F, cached]; [Automerge JS API v3.2.3](https://automerge.org/automerge/api-docs/js/types/JsReleaseInfo.html) [F]; [Automerge 2→3 migration](https://automerge.org/docs/migrating-from-automerge-2-to-automerge-3) [F]
- Summary: No Zero 1.0 was found. Automerge 3.0 (Jul 2025) cut memory use by more than 10x and removed the old Text API, and 3.2.3 is the current JS release. A `sedimentree_automerge` crate (0.1.0) appeared on docs.rs ([link](https://docs.rs/crate/sedimentree_automerge/latest)) [F]. It is Ink & Switch's sync/storage layer and a possible blob-and-sync comparator.
- Relevance: version facts for the **C1/C7** citations. No threat.

#### PowerSync attachments consolidation
- PowerSync · Changelog · Feb 2026 (published 5 Mar 2026)
- Link: [PowerSync Changelog Feb 2026](https://powersync.com/blog/powersync-changelog-february-2026.md) [F, snippet]
- Summary: JS attachment APIs moved into `@powersync/common`, and the Dart `powersync_core` 1.6.0 attachments helper shipped in Oct 2025. The snippets mention no rejection-cascade or offline-reference GC semantics.
- Relevance: **S1/S2/S5**. The attachment queue is still local-only, so the existing verdict stands. Re-check the full changelog before submission.

#### Weidner: Collaborative text editing over PowerSync
- Matthew Weidner · Guest blog post · 30 Sep 2025
- Link: [PowerSync blog](https://www.powersync.com/blog/collaborative-text-editing-over-powersync) [F, listed on mattweidner.com]
- Summary: Server-ordered text editing on a relational sync engine without CRDTs, following his May 2025 "Collaborative Text Editing without CRDTs or OT" post.
- Relevance: **C7**. Prior art for "record-mode CRDT/collab fields over a server-ordered row log" that supports treating collaboration as a plugin.

### Inferences
- Jazz 2.0 is now the closest single system to the thesis design. Its spec style (numbered normative chapters, invariant IDs, a test gate) is also a model the thesis can follow. Its existence argues for re-framing S2 as **cross-row, cross-device causal closure**, and S5 as **GC that honors references not yet pushed**, neither of which Jazz specifies.
- Searches for "offline write rejection" and "attachment consistency" papers on arXiv in 2026 returned only GitHub issues and app PRs, not papers. That is weak evidence that the academic gap for S2 and S5 remains open.

### Gaps
- The Local-First Conf 2026 talk titles and abstracts were not retrieved (client-side schedule).
- FOSDEM 2026 local-first devroom: not searched (budget).
- Lobste.rs and HN 2026 threads: searches surfaced only 2023–2025 Show HNs, so no 2026 items were found.
- Loro 1.x 2026 and Yjs 14 status: not checked.
- Encrypted Spaces (Jun 2026) follow-ups: not checked; the security agent covers them.
- The full Jazz `4_history_merging.md` (INV-HIST-20, the cascade definition) and `13_db_api.md` were not read. Read them before finalizing the S2 wording.

## Domains

### Takeaway
Every domain uses the same split: mutable metadata over immutable, content-addressed bytes, and Speckle is a clean BIM example. No domain reference specifies causal rejection or offline-reference GC. The domain evidence supports the "plugin" scoping and does not threaten the claims.

### Cited Findings

#### Geospatial
- **geodiff (Lutra/Mergin Maps)**: GeoPackage/SQLite ↔ PostGIS changesets, rebase and conflict file. [GitHub](https://github.com/MerginMaps/geodiff) [F in main report]. Relevance: C4/C5 prior art for per-row diff/rebase. It must be cited.
- **QFieldCloud**: timestamp last-wins with a manual "re-apply" job. [docs](https://docs.qfield.org/it/reference/qfieldcloud/jobs/) [F in main report]. Relevance: S2 baseline. Its rejection handling is manual.
- **Esri branch versioning**: the default branch has no conflict detection, and named versions reconcile. [Esri](https://doc.esri.com/en/arcgis-pro/latest/help/data/geodatabases/overview/) [M]. Relevance: versioning plugin, C1.
- **OSM API 0.6**: changesets with optimistic version numbers, where a 409 Conflict rejects the whole upload diff. [OSM wiki API v0.6](https://wiki.openstreetmap.org/wiki/API_v0.6) [M]. Relevance: a long-running example of **authority rejection of an offline batch**, cited as a per-batch baseline for S2. It has no cascade to dependents across changesets.

#### Media and binaries
- **tus resumable upload**: 409 on offset mismatch, plus Expiration and Termination extensions. [tus.io](https://tus.io/protocols/resumable-upload) [F in main report]. Relevance: C2. The Expiration extension is the reserve-then-expire lease folklore.
- **S3 multipart upload and lifecycle `AbortIncompleteMultipartUpload`**: [AWS docs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html) [M]. Relevance: C2/S5 store-side lease analog.
- **Frame.io Camera-to-Cloud**: devices upload proxies and originals tied to project assets, and offline queues resume. [Frame.io C2C](https://frame.io/c2c) [M]. Relevance: a B-heavy field domain for the demand argument.
- **Blackmagic Cloud / DaVinci Resolve collaboration**: pessimistic bin and timeline locks, with media synced separately. [Blackmagic](https://www.blackmagicdesign.com/uk/products/davinciresolve/collaboration) [F in main report]. Relevance: lock plugin.
- **Proton Drive / Ente**: per-file keys with chunked client-side encryption (Proton: per-block encryption with a signed manifest; Ente: per-file key, libsodium secretstream chunks). [Proton Drive security model](https://proton.me/blog/protondrive-security) [M]; [Ente architecture](https://ente.io/architecture) [M]. Relevance: E for blobs. Content addressing on ciphertext breaks cross-user dedup, which the B+E design must state.

#### CAD/BIM
- **Speckle**: "Every object has a unique object ID (also called a hash) that's deterministically generated from its content", and "Versions are immutable snapshots … Each version references a Root Collection by its object ID". The decomposition API splits large objects into child objects. [Speckle data schema](https://docs.speckle.systems/developers/data-schema/overview) [F]; [Decomposition API (preview docs)](https://deploy-preview-234--speckle-docs.netlify.app/dev/decomposition) [F]. Relevance: C2/C4. CAS object trees with version pointers; immutable versions, no merge, no offline rejection semantics.
- **Onshape**: a cloud-native CAD document with a microversion history, branches and merge, and online-only editing. [Onshape versions & history](https://cad.onshape.com/help/Content/document_history.htm) [M]. Relevance: the versioning/branching plugin row. Its online-only editing contrasts with the offline focus.

#### Data versioning
- **Dolt**: Prolly-tree tables with cell-level three-way merge. [docs.dolthub.com](https://docs.dolthub.com/) [M]. Relevance: the "out of scope" row (cell merge of arbitrary snapshots).
- **lakeFS**: Git-like branches over object storage with metadata-only commits. [lakeFS docs](https://docs.lakefs.io/) [M]. Relevance: branch = base plus overlay over CAS blobs, close to the branch plugin design.
- **Neon branching**: copy-on-write Postgres branches at an LSN. [Neon docs](https://neon.tech/docs/introduction/branching) [M]. Relevance: time travel as "snapshot + log to seq N" (versioning core row).
- **Jazz 2.0 `11_branch_views_time_travel.md`** and **`10_lenses_migrations.md`**: present in the SPEC directory [F, filenames only]. Relevance: Jazz also specifies branches, time travel and lens migrations, which may overlap the C4 lens-law framing for schema evolution (not for folders). Unread.

#### Time-series and IoT
- **AWS IoT Greengrass stream manager**: local append-only streams with TTL, size limits, priority export and per-stream export configs. [AWS docs](https://docs.aws.amazon.com/greengrass/v2/developerguide/manage-data-streams.html) [M]. Relevance: the "Streams" plugin row (device+seq, TTL, priority export).

#### Field service and inspection
- **Team IM inspection management**: "powered by JourneyApps' offline-first PowerSync architecture", with photos attached to survey items. [Team IM](https://www.teamim.com/products/inspection-management) [F, vendor page]. Relevance: demand evidence for forms plus photos (S1/S2 use case).
- **SafetyCulture (iAuditor)**: offline inspections with media, synced later. [SafetyCulture help: offline](https://help.safetyculture.com/) [M]. Relevance: demand.
- **Microsoft Dynamics 365 Field Service mobile offline profiles**: offline sync filters and file/image sync limits. [MS Learn](https://learn.microsoft.com/en-us/dynamics365/field-service/mobile/offline-data-sync) [M]. Relevance: scope/closure filters and attachment limits as an industry baseline.
- **AEDES**: not found this session. See Gaps.

#### Notion-style collaboration storage
- **Automerge storage / automerge-repo storage adapters**: chunked incremental changes plus snapshot compaction, keyed by document ID. [automerge.org docs](https://automerge.org/docs/repositories/storage/) [M]. Relevance: C7, the "snapshot + update rows" opaque CRDT field.
- **Hocuspocus**: Yjs server with debounced `onStoreDocument` persistence. [Hocuspocus docs](https://tiptap.dev/docs/hocuspocus/server/extensions/database) [M]. Relevance: C7, one in-memory authority per document with Postgres as the checkpoint.

### Inferences
- OSM's 409-whole-upload rejection and QFieldCloud's manual re-apply are the domain baselines for S2. Both are per-batch with no causal closure, which supports the S2 framing.
- The E2EE blob stores (Proton, Ente) and Speckle's CAS conflict on dedup. The thesis should state that dedup is per-key-scope under E mode.

### Gaps
- Most domain links are [M]. The budget went to the Jazz threat; verify the Proton, Ente, Greengrass, Dynamics, Onshape and Automerge storage URLs before citing them.
- AEDES and the Microsoft field service + PowerSync case study were not found. The JourneyApps/OXIDE case-study pages are the likely source.

## Theses

### Takeaway
No 2022–2026 PhD thesis specifically on relational sync engines with blobs was found. The usable structural models are De Porre (VUB 2022), a Cambridge systems PhD such as Howard's, and the Jazz SPEC's normative "contract" structure. Weidner's CMU thesis was not found in public; he is still listed as a PhD student "(absentia)".

### Cited Findings

#### When sequential code meets replicated data
- Kevin De Porre (VUB; advisor Elisa Gonzalez Boix) · PhD thesis · 19 Dec 2022
- Link: [VUB defense notice](https://www.we.vub.ac.be/nl/node/1790?page=7) [F]; related [ECROs, OOPSLA/PACMPL 2021](https://doi.org/10.1145/3485484) [M for DOI]
- Summary: Derives replicated data types from sequential specifications plus invariants (ECROs), with SMT-checked ordering, and builds SECROs.
- Relevance: **S2/S6**. Invariant-preserving replication of non-I-confluent ops, so the validator chapter should cite it. It is also a model for a "spec → verify → evaluate" thesis arc.

#### Matthew Weidner, CMU
- Matthew Weidner (advisor Heather Miller) · PhD in progress (absentia) · no thesis found
- Link: [mattweidner.com](https://mattweidner.com/) [F]
- Summary: Collabs ([arXiv 2212.02618](https://arxiv.org/abs/2212.02618)) [F], for-each ops ([arXiv 2304.03141](https://arxiv.org/abs/2304.03141)) [F], and the 2025 server-ordered text-editing posts.
- Relevance: C7. Cite his papers, not a thesis.

#### Local-First Software: Promises and Pitfalls
- Unnamed author · Bachelor's thesis · ~2025 (ResearchGate)
- Link: [ResearchGate](https://www.researchgate.net/publication/392870932_Local-First_Software_Promises_and_Pitfalls) [F, listing]
- Summary: A survey of local-first approaches and technologies.
- Relevance: background only. Its weak structure is a counter-model.

#### Distributed consensus revised
- Heidi Howard · Cambridge PhD · 2019 (outside the window, structure model)
- Link: [Cambridge tech report UCAM-CL-TR-935](https://www.cl.cam.ac.uk/techreports/UCAM-CL-TR-935.pdf) [M]
- Summary: Generalizes Paxos via quorum intersection, with a clear "claims → proofs → evaluation" thesis structure.
- Relevance: a structural model for an invariant-family thesis (S1–S6).

#### Peter Zeller: Repliss, testing and verification on weakly consistent databases
- Peter Zeller (TU Kaiserslautern; Bieniusa/Poetzsch-Heffter) · PhD · ~2020–2021
- Link: [Repliss GitHub](https://github.com/peterzeller/repliss) [M]
- Summary: Property-based testing and verification of invariants for apps on causally consistent databases.
- Relevance: **S2/S6 testing method**. The closest academic precedent for "simulation-checked invariants".

### Inferences
- The thesis can follow Jazz's spec style (normative numbered chapters with invariant IDs tied to tests) together with De Porre's or Zeller's evaluation style. That fits the "specified, simulation-checked contract" framing.

### Gaps
- No Kleppmann-student PhD theses from 2022–2026 were confirmed (for example, on access control or Byzantine EC). Search the Cambridge/TUM repositories.
- Yjs origin: Kevin Jahns's work was a master's at RWTH Aachen (~2015–2016) [M, not verified]. Seph Gentle has no thesis; cite Eg-walker (EuroSys 2025) instead [M].
- No MSc theses evaluating PowerSync, Electric or Zero were found. Search TU Delft, KTH, Aalto and Chalmers repositories.
- Weidner's thesis status is unconfirmed; check CMU KiltHub.

## Coverage gaps (overall)
- FOSDEM 2026 local-first devroom, lobste.rs and localfirst.fm 2026 episodes: not searched.
- Loro and Yjs 2026 versions: not checked.
- Jazz chapters `4_history_merging`, `8_sync_protocol`, `10_lenses_migrations`, `11_branch_views_time_travel`, `13_db_api` and `E2EE_LARGE_VALUE_FORMAT`: unread. They are the highest-priority follow-up for the S2, S5, C4 and S6 wording.
- Most domain references are [M] and need URL verification.
- The Local-First Conf 2026 talk list is unretrieved.
