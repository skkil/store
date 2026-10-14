# Jazz 2.0 SPEC: deep read vs. thesis claims

Source: `garden-co/jazz` `crates/jazz/SPEC/` (main branch, fetched 2026-11-04 via GitHub API/raw). Base URL: https://github.com/garden-co/jazz/blob/main/crates/jazz/SPEC/
Files read [F]: 1_intro, 3_transactions (§3.2–3.8), 4_history_merging (invariant digest, §4.6, §4.7), 7_authorization (INV-RLS-6, open questions), 8_sync_protocol (§8.3, §8.4–8.5 excerpts, §8.12, §8.13), 9_topology (§9.1, §9.5), 10_lenses (overview + digest), 11_branch_views (overview), 13_db_api (overview, offline notes), 15_sharding (overview + digest), 19_large_values (§19.4, 19.7–19.11, open questions), E2EE_CRYPTO_FORMAT, E2EE_EQUALITY_FORMAT, E2EE_LARGE_VALUE_FORMAT, ACCOUNT_HANDLES (revocation ordering), D_testing_gates (§D.4.2 table). Grepped all of the above plus 2, 5, 6, E_glossary.
Not read: 2_data_model, 5, 6_queries, 14, 16, 18, 19_native_relays, B, C bodies (grep only).

## 1. Chapter summaries with key normative text

### Intro (ch. 1) [F]
"jazz is a local-first, distributed, real-time database with full edit history, RLS-authorized sync, and two transaction kinds ... Its storage and query execution are SQLite ... This document is both the design and the contract." ([1_intro.md](https://github.com/garden-co/jazz/blob/main/crates/jazz/SPEC/1_intro.md))

### Transactions, authority, validation (ch. 3) [F]
- §3.2: both kinds sync "_only at commit_, as one idempotent `SyncMessage::CommitUnit { tx, versions }`; the authority answers with `FateUpdate`". Same payload is idempotent; a different payload under the same id is `ConflictingCommitUnit` (INV-TX-4).
- §3.6: "Fate authority is **structural**. A node acts as fate authority exactly when the host wires it as Core." "Client-side staging is deliberately not an authorization boundary ... MUST NOT evaluate read or write policy and turn partial local evidence into a definitive denial." Local permission probes are advisory and return `Unknown` (INV-API-28).
- §3.6: "Mergeable transactions carry no read set, version parents or causal dependency graph: a caller that needs 'only if I observed X' uses a serializable transaction and its read set."
- §3.7: serializable validation re-checks row reads, absent reads, and predicate reads (with narrowed reads across joins) at Core (INV-TX-16..20). An offline partial client "can prepare a serializable transaction offline and commit it later while its reads still hold." "Read-set validation ... is not an authorization boundary ... Invariants every writer must respect belong in permission policies."
- §3.8: "Rejection cascades only along bases ... writes built on it, whose images carried its values: those are rejected too (`predecessor_rejected`) ... every other transaction is decided on its own." On origin, "the pending overlay is refolded without it, while the node's other pending writes, including edits to the same row, stay visible and settle by their own fates (INV-TX-8)". `RejectionReason::Cascade` "remain[s] decodable ... but Core no longer produces [it]". Origin keeps rejected payloads in retry stores (INV-TX-9). A runtime not attached when the rejection arrives "does not receive a retroactive `onMutationError` callback ... applications that need durable user-visible outcomes must record their own domain-level status."

### History and merging (ch. 4) [F]
- Model: "git with one remote. Origin is `origin/main`, the local ref is the local branch with unpushed commits, and uploading is a push. Core accepting a write is a rebase onto origin with per-cell auto-resolve" (§4.7).
- §4.6 / INV-HIST-8, -15: per-cell last-arrival-wins in Core seq order; counters, sets, text (three-way merge at Core only, INV-HIST-24).
- Base kinds: `Seq(S)`, `Own{back}`, `Commit{node, tx}` ("an unsettled commit by its writer's id, as a relay names the commit that wrote one of its unsettled versions").
- INV-HIST-20: "a predecessor of another node counts only over a trusted link or for a node the link's identity writes as, and Core MUST reject a write built on any other ... Core MUST NOT wait for a pending predecessor it holds no fate for ... MUST reject the write as built on a rejected write."
- INV-HIST-18: "A version parent ... MUST NOT encode a cross-row transaction dependency or a dependency between the content and deletion layers."
- INV-HIST-23: one writer's commit units reach Core in written order; a late predecessor is refused `MalformedCommit`. Parked predecessor wait is bounded (`PARKED_PREDECESSOR_WAIT_MS` = 60 s).
- §4.6: "A `P` rejected locally before it reached Core (for example its large value was not staged) therefore ends like a `P` rejected at Core: `W` is rejected too." "While one of its writes waits (a large value rate-limited or still uploading), none of that writer's later writes goes up ahead of it."

### Sync protocol (ch. 8) [F]
- §8.3 fates: `Accepted{at, accepted_at, as_written}` / `Refused{through, error}`; on refusal "the rows the commit wrote shipped back ahead of the fate". Lost fates are recovered by re-upload (idempotent).
- §8.5: "A relay reconciles the same way every session a one-shot answer shipped one of its unsettled versions to (another session's pending write): once that version settles, the row ships again as it is now, or is evicted."
- §8.12: "A writer pushes a new value's nodes in `PushChunks` before the commit that uses them; the server holds them under a receipt until the commit arrives or the receipt expires ... A row is installed only once its chunks are present (INV-SYNC-46)."
- §8.13 relays: the relay "ships its unsettled versions at their pending `_clock`, each with its writer". "On a new link a session drops the relay versions it shows (pending versions other writers wrote) back to the newest accepted version it holds." A relay with no server leaves commits pending; "a serializable commit waits for that server rather than being validated by the relay".

### Topology (ch. 9) [F]
- §9.1: "Possessing bytes is never authorization or permission to assign a fate (INV-TX-23)." "No intermediate server can substitute a locally computed result for Core confirmation."
- §9.5: "Jazz has exactly one relay tier ... one durable, scope-isolated client relay (the browser shared worker, or a native relay) ... Relays are not chained ... no relay serves more than one auth scope." "A relay shared across auth scopes (server-deployed or otherwise multiplexed) is not supported." So peer-visible pending writes exist only between same-identity tabs/processes on one device.

### Authorization / RLS (ch. 7) [F]
- INV-RLS-6: "Read-policy revocation MUST remove rows from future settled subscription result sets, and the server MUST evict them ... Eviction is coherence, not redaction: it makes no confidentiality promise about data a node already received."
- Open design question: "**Per-column encryption and authorization.** If encrypted columns are added, policy evaluation must define what can be evaluated server-side, what requires client-side keys, and how key loss/revocation interacts with read policy." ([7_authorization.md](https://github.com/garden-co/jazz/blob/main/crates/jazz/SPEC/7_authorization.md))
- ACCOUNT_HANDLES "Revocation ordering": "Revocation is an admission boundary, not an erase or synchronous drain barrier ... A bounded operation admitted before revocation may finish ... Every later inbound operation and outbound subscription tick must recheck admission."

### Lenses / migrations (ch. 10) [F, overview + digest]
Multiple schema versions coexist; lenses translate "without rewriting history". INV-LENS-14: "translating then applying MUST equal applying then translating" (a commutation law). INV-LENS-17: TransformColumn only if "registered as bijective and canonical-equality-preserving". INV-LENS-16: commits are not rejected because their schema differs from the current write schema. Unknown-schema commits park (INV-LENS-5). Lenses are over relational schemas only; no file/codec lenses.

### Branches / time travel (ch. 11) [F, overview]
"Jazz does not own a branch object"; branches are `branchBy` columns. INV-TIME-1/2: historical reads use settled transactions `<= position` and evaluate read policy at that cut.

### Db API (ch. 13) [F, overview + offline notes]
`Db::open` is never a fate authority (INV-API-2); writes are `Local` until fate (INV-API-14); `WriteHandle::wait(Global)` needs `Fate::Accepted`, rejected returns `WriteRejected` (INV-API-15). Intentional offline mode (INV-API-31/32).

### Sharding (ch. 15) [F, overview]
"Sharding is **exploratory** ... It does not specify implemented shard behavior (INV-SHARD-1)." Sketch: serializable txs single-shard unless specified (INV-SHARD-8); HLC positions, holds and fixed cross-shard parts (INV-SHARD-15..21); global catalogue/sequencer retained (INV-SHARD-10).

### Large values / storage / GC (ch. 19) [F]
- §19.4: staged nodes "held by an upload lease (`_jazz_lv_upload`) or, when pushed by a client, by a receipt (`_jazz_lv_receipt`) until the commit referencing them lands or the staging maximum age passes ... a commit whose lease expired meanwhile is refused". INV-CONTENT-4: "staged nodes are unreachable from any row".
- Possession rule: a tree is admitted only if the writer possesses it (under a readable version its commit names, or pushed under its own receipt). This blocks read-bypass-by-hash and existence oracles. "Losing read access takes effect at once" (§19.9).
- §19.7: on rejection "the replica restores the settled version and replays its other pending writes over it ... Epochs are never truncated, so no cell can name an edit that disappeared."
- §19.10: leaves can move to `ChunkStore` (FileChunkStore directory, S3, R2). A tick moves only leaves "held by no lease, receipt or unpushed mark". INV-CONTENT-9: "A leaf leaves the file only after its put succeeded, and leaves the chunk store only after its reclamation committed."
- §19.11: trigger refcounts; INV-CONTENT-8: "collection removes only epochs and nodes unreachable from every retained version, lease and receipt." "The authority keeps every version" (§19.8). Open question: "Retention and history thinning: removal of old versions must be ordinary refcounted row deletions."

### E2EE formats [F]
- Envelope `JE2E` + canonical context `JE2C` (application, policy, scope, identifier, table, row, column, epoch, recipient). Cell: XChaCha20-Poly1305 with BLAKE2b-derived per-epoch cell key. Ed25519 device signatures, but "The lifecycle's canonical signed-record layout and durable enrolment format remain to be implemented."
- Equality tokens: deterministic keyed BLAKE2b; "encrypted query integration are separate contracts and are not supplied by this layer."
- Large values: libsodium secretstream, 64 KiB records, final-record authentication. "Each replacement starts a fresh secretstream and does not read, diff or merge the previous plaintext." It is "not a new storage or transport protocol". E2EE is mentioned in no numbered normative chapter (grep), so encrypted values and chunk-tree dedup/deltas/server validation are not integrated.

### Files / filesystem [F, grep]
No folder, filesystem-replica or file-watch concept. Only "filesystem" mentions are a FileChunkStore directory backend (§19.10), lease state in ch. 2, and a maintainer tooling note in ch. 4.

### Hash-chained log [F, grep]
No hash-chained log. Identities are content-addressed (schemas, lenses, shapes, chunk nodes). The only Merkle text is a *target test ladder* for "authenticated sparse-Merkle" manifest transitions in D.4.2 (a sync-closure proof, not a log chain).

## 2. Per-claim verdicts

**S1 (no dangling blob refs; pending → committed → revoked visible to other devices).** Jazz already specifies that blobs commit inside the row transaction, staged nodes are unreachable and leased/receipted, a commit with an expired lease is refused, a row installs only with its chunks (INV-SYNC-46), and lost read access cuts chunk pulls at once. Surviving delta: there is no *blob lifecycle state visible to other devices*. Other devices see a value only once it is accepted (only same-scope relay sessions see pending versions), and revocation is row eviction ("coherence, not redaction"), not a revoked state. The S1 claim must be stated as a cross-device observable state machine, not as "no orphans".

**S2 (causal cascade incl. dependents seen by others, plus blobs/claims).** Jazz already specifies a transitive `predecessor_rejected` cascade along same-row bases, including other nodes' predecessors over a trusted relay link (INV-HIST-20, `BuiltOn::Commit`), relay reconciliation of sessions shown another's pending write (§8.5), and a large-value-not-staged rejection that cascades (§4.6). Surviving delta: (a) cross-row/semantic dependents: INV-HIST-18 forbids cross-row dependency edges, and mergeable txs have "no ... causal dependency graph"; (b) cross-device: relays are one tier, one auth scope, so other users' devices never build on pending writes; (c) Jazz explicitly drops the `Cascade` reason and gives no durable notification ("must record their own domain-level status"). Blob cleanup on rejection is pre-empted (refcounts + "no cell can name an edit that disappeared").

**S3 (bounded revocation).** Jazz specifies forward-only revocation: INV-RLS-6 eviction, ACCOUNT_HANDLES "A bounded operation admitted before revocation may finish", recheck on every later op/tick. Surviving delta: no stated bound (time or op count) on post-revocation exposure, and no offline-device bound. Key revocation for E2EE is an open question. Claim survives if it is a quantified bound.

**S5 (GC honoring references offline devices have not pushed; device leases).** Jazz specifies local "unpushed mark" protection, server upload leases/receipts with max age, and refcount GC over "every retained version, lease and receipt", but it sidesteps the hard case by retaining all history ("The authority keeps every version"; history thinning is an open question). An expired lease simply refuses the commit. Surviving delta: GC under history thinning/retention that honors bases and dedup-references held by long-offline devices (per-device leases on server state). Weak if the thesis also keeps all history.

**S6 (single deterministic validator server-side or client replay, opt-in E2EE mode switch).** Jazz has one structural authority (Core), deterministic post-image (INV-HIST-15), and advisory local validation; clients and relays never decide fate. E2EE exists only as byte formats; "per-column encryption and authorization" is an open question. Surviving delta: intact. Same validator run by clients under E2EE, plus a runtime mode switch, is unspecified.

**C4 / F1–F5 (two-way user-folder replica with codecs).** Jazz has no filesystem replica. Lenses are relational, with a commutation law (INV-LENS-14) and bijective transforms (INV-LENS-17), which partly pre-empts a "lens laws for schema evolution" framing. Surviving delta: folder replica, codecs and file-to-row lenses are fully intact. The thesis should cite INV-LENS-14/17 as prior art for its lens laws.

**D1 (distributed server).** Sharding is "exploratory" (INV-SHARD-1), but there is a concrete sketch: HLC positions, holds, fixed cross-shard parts, single-shard serializable by default. Surviving delta: an implemented and specified distributed authority. The novelty is thin if it only reuses that sketch's shape.

**Hash-chained log.** Not in Jazz (content addressing only; Ed25519 device signatures without a pinned record layout). Surviving delta: intact.
