# FS + SQLite, Transactions, Conflicts, Postgres Mapping

Research date: 2026-11-04. Legend: [F] = fetched or search-verified this session. [M] = from prior knowledge; URL not re-fetched this session, so verify before citing in the final report.

---

## Q1. Multi-row transactions in offline sync

### Takeaway
Most engines keep a local transaction as one unit on the upload path, as a group, a mutator, or a txid. Few keep it as one unit on the download path. The engines that do are those with a single server-ordered log plus checkpoints (PowerSync checkpoints, Zero, Linear lastSyncId, LiteFS LTX). Engines built on row or column CRDTs (cr-sqlite, Actual, Turso LWW) can split transactions on the wire. Document stores (CouchDB, Couchbase) have no multi-doc atomicity at all. Design implication: put a transaction or mutation id on every change, apply each server batch in one local SQLite transaction, and only expose a batch at a consistent server cut (a checkpoint or LSN).

### Cited findings
- **PowerSync (upload).** `getNextCrudTransaction()` returns "the oldest transaction that hasn't been completed yet" and "only returns data from a single transaction at a time". `getCrudBatch` can span transactions. You call `complete(writeCheckpoint?)` after a successful upload. [F] https://pub.dev/documentation/powersync/latest/powersync/PowerSyncDatabase/getNextCrudTransaction.html ; https://docs.powersync.com/handling-writes/custom-write-checkpoints.md
- **PowerSync (errors).** Network or temporary errors: "The change is kept in the queue" and is retried. Validation or conflict errors: the server should "respond with "success" (HTTP 2xx) even in the case of write conflicts or validation failures". The client then "will revert to the last known state" once the queue is empty and the server copy is current. Bugs or schema mismatches block the queue. An optional backend dead-letter queue unblocks it but risks out-of-order updates. [F] https://docs.powersync.com/handling-writes/handling-write-validation-errors
- **PowerSync (download).** Local data is only made consistent with the server once the upload queue is empty. Write checkpoints are how a client knows it has seen its own write. The new `requestCheckpoint().waitForSync()` is alpha and needs Service v1.24.0+. [F] https://docs.powersync.com/client-sdk-references/javascript-web . From [M]: the sync protocol applies server data per checkpoint in one SQLite transaction, which gives atomic visibility at checkpoint boundaries.
- **Zero.** A mutator runs on the client first, then server-side inside a DB transaction ("the entire mutation is rolled back" on throw). It is retried at most 5 times. On a skipped or failed mutation, "The optimistic mutation on the client will be reverted". Replication to clients is asynchronous: after a server ack, a read "can still reflect optimistic rather than authoritative state". The docs make no explicit cross-mutation ordering guarantee. [F] https://zero.rocicorp.dev/docs/mutators
- **Replicache.** Mutators are named functions, replayed (rebased) on the server and on top of new server state. `lastMutationID` per client gives idempotence and ordering. [M] https://doc.replicache.dev/concepts/how-it-works
- **Electric + TanStack DB.** Writes go through your own API to Postgres. The API returns `pg_current_xact_id()` read in the same txn. The client calls `awaitTxId(txid)` (15 s default timeout) and drops optimistic state when that txid streams in. If the txid comes from a different txn, the call "stalls forever". There is also snapshot-based txid visibility (`isVisibleInSnapshot`). [F] https://tanstack.com/db/v0/docs/collections/electric-collection ; https://github.com/TanStack/db/pull/648 ; https://github.com/TanStack/db/pull/651. Shapes are single-table, and no cross-shape atomic-apply guarantee was found. [F] https://electric-sql.com/docs/guides/shapes
- **cr-sqlite.** Each commit gets a `db_version`. When syncing, "a transaction might be split up across two db_versions unless you've written a custom networking layer". Querying by db_version misses values that later txns overwrote. `seq` orders changes inside a db_version. `cl` (causal length) tracks deletes. [F] https://vlcn.io/blog/how-crsqlite-transactions-work-today ; https://vlcn.io/docs/cr-sqlite/api-methods/crsql_changes
- **SQLite session extension.**
  - Changesets store old and new values. Patchsets store PK plus new values only, so DATA conflicts are not detected for UPDATE/DELETE. [F] https://www.sqlite.org/sessionintro.html
  - `sqlite3changeset_apply` runs inside a savepoint. Conflict types are DATA, NOTFOUND, CONFLICT (PK exists on INSERT), CONSTRAINT and FOREIGN_KEY.
  - The handler returns OMIT, REPLACE (only for DATA or CONFLICT) or ABORT. Errors roll back the savepoint, "restoring the target database to its original state".
  - The handler may run SQL. Missing tables are skipped with SQLITE_SCHEMA. In 3.51.0+ the error message is left on the handle. [F] https://www.sqlite.org/session/sqlite3changeset_apply.html
  - From [M]: FK violations are checked once at the end; the handler gets FOREIGN_KEY and can only OMIT (commit anyway) or ABORT.
- **Turso Sync.**
  - Row-level "last push wins". Push sends logical statements. No write txn should be open during a push.
  - Pull with unpushed changes "rolls back to the last synced state, applies remote changes, then replays your unpushed changes" atomically. A transform hook allows custom resolution. [F] https://docs.turso.tech/sync/conflict-resolution ; https://docs.turso.tech/sync/usage
- **Litestream / LiteFS.** Both are single-writer physical replication, so there are no conflicts and transactions are preserved by construction. LiteFS ships per-transaction LTX files; Litestream v0.5 (2025) moved to LTX too. Neither works for multi-device offline writes. [M] https://fly.io/docs/litefs/how-it-works/ ; https://fly.io/blog/litestream-v050-is-here/
- **CouchDB.** No multi-document transactions; `_bulk_docs` is not atomic (`all_or_nothing` was removed in 2.0). Conflicts are per document, with a deterministic winner plus kept `_conflicts` revisions. [M] https://docs.couchdb.org/en/stable/replication/conflicts.html
- **Couchbase Lite.** Local `inBatch` gives a local transaction only. Replication via Sync Gateway is per document; the conflict resolver is per document. [M] https://docs.couchbase.com/couchbase-lite/current/
- **Theory.**
  - Bailis et al., *Highly Available Transactions* (VLDB 2014): read committed, monotonic atomic view and causal can all stay highly available; snapshot isolation and serializability cannot. [M] https://www.vldb.org/pvldb/vol7/p181-bailis.pdf
  - *RAMP* (SIGMOD 2014): read atomic isolation = all-or-nothing visibility of a txn's writes across partitions, without locks. [M] http://www.bailis.org/papers/ramp-sigmod2014.pdf
  - *Cure* (ICDCS 2016, AntidoteDB): Transactional Causal Consistency = causal + atomic visibility + convergent CRDTs. [M] https://arxiv.org/abs/1602.01425 ; https://www.antidotedb.eu/

### Inferences
- Engines fall into two models:
  - **Op/mutation model** (Replicache, Zero, Linear, Todoist commands, PowerSync with a single-txn upload): a transaction is preserved upstream because the server re-executes or atomically applies one unit.
  - **State/CRDT model** (cr-sqlite, Actual, Turso): only per-row or per-cell convergence. The read path can show half a transaction unless the transport batches by txn and the apply is atomic.
- For a personal app, "monotonic atomic view" (HAT) is the right target, plus a server-assigned total order. Serializability is not needed.
- Partially applied txns:
  - Upstream: the server must apply all of a txn or none. PowerSync leaves this to your backend; Zero does it with a server-side txn.
  - Downstream: apply each server batch in one SQLite txn, and never expose a state between checkpoints.

### Gaps
- PowerSync docs on downstream checkpoint atomicity were not fetched directly.
- No authoritative Electric statement was found on cross-shape or multi-table transactional apply on the client.
- No source checked for whether cr-sqlite's "tomorrow" transactional work shipped.
- Couchbase and LiteFS claims were not re-verified.

---

## Q2. Filesystem as a synced data source

### Takeaway
Treat the filesystem as an untrusted, eventually observed replica.
- Detect changes with OS watchers as a hint, and periodic scans as the truth. The scan compares (size, mtime, perms/inode), then hashes.
- Write atomically: temp → fsync → rename. Then commit the DB.
- Keep a "synced/base" snapshot so you can do 3-way decisions (Dropbox's three trees).
- Identify files by stable ID, not path, so moves are cheap.
- Resolve true conflicts by forking a conflict copy rather than merging, except for text (Obsidian merges).

### Cited findings
- **Syncthing.**
  - Change detection: an FS watcher (on by default) batches changes for 10 s, with deletes delayed a further minute. A full rescan runs hourly, because the watcher "may miss" changes. The rescan checks mtime, size and permission bits, then rehashes changed files.
  - Blocks: SHA-256 per block (128 KiB–16 MiB). Each received block is verified against its hash.
  - Writes: Syncthing writes to `.syncthing.<name>.tmp` (Windows: `~syncthing~`) and renames over the target. The temp file is kept up to a day after errors.
  - Conflicts: one copy is renamed to `<name>.sync-conflict-<date>-<time>-<modifiedBy>.<ext>`. The older-mtime file is the one renamed; ties break by device ID. If a delete wins against a modify, the modified file becomes a conflict copy. Conflict copies sync like normal files.
  - Case: paths are case-sensitive. On case-insensitive filesystems a clash is reported as a "case conflict" and is not synced. [F] https://docs.syncthing.net/users/syncing.html
- **Dropbox Nucleus.**
  - Three trees: Remote (cloud), Local ("last observed state … on disk"), and Synced (the last fully synced state, used as a merge base). The Synced tree is what tells a local delete apart from a never-downloaded file.
  - Testing: CanopyCheck (random tree generation, checks that the planner converges) and Trinity (injects random async behaviour). [F] https://dropbox.tech/infrastructure/-testing-our-new-sync-engine (Isaac Goldberg, 2020-04-20) ; https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine (Sujay Jayakar, 2020-03-09)
  - From [M]: Nucleus keys nodes by unique ID, not path, so a move is a single attribute change and handles cheaply.
- **Concurrent tree moves.** Concurrent moves cause bugs in Google Drive and Dropbox (Kleppmann et al., TPDS 2021: undo-do-redo move op). [F] https://martin.kleppmann.com/2021/10/07/crdt-tree-move-operation.html
- **Obsidian Sync.** Settings > Sync > Conflict resolution offers "Automatically merge" (default; "may occasionally produce duplicate text") or "Create conflict file". The setting is per device and appeared in 1.9.7 or 1.9.10 (sources disagree). JSON settings merge by keys. Conflict-file naming is reported inconsistently: "(Conflicted copy device YYYYMMDDHHMM)" vs `.sync-conflict-…`. [F] https://obsidian.md/help/sync/troubleshoot ; https://help.obsidian.md/sync/settings ; https://forum.obsidian.md/t/where-are-the-conflict-resolution-options-in-settings/105253
- **Apple File Provider (iCloud Drive and 3rd-party).**
  - `NSFileProviderItemVersion` has separate `contentVersion` and `metadataVersion`, each ≤128 bytes. A metadata-only change avoids moving content.
  - Conflicts reach the extension as `modifyItem`/`deleteItem` with a `baseVersion`.
  - Pattern from an Apple engineer: rename the conflicting item and bump its contentVersion, then push a new item (new identifier) for the local edit as the conflict copy. [F] https://developer.apple.com/documentation/fileprovider/nsfileprovideritemversion ; https://developer.apple.com/forums/thread/776884
- **OneDrive.** Keeps both copies with a `-<MachineName>` suffix. [M] https://support.microsoft.com/office/onedrive-conflict
- **Git.** Snapshot plus 3-way merge with an explicit base. The index caches stat data (mtime, ctime, inode, size) to avoid rehashing; this is the "racy git" problem. `core.ignorecase` and `core.symlinks` handle FS differences. [M] https://git-scm.com/docs/racy-git
- **OS limits.** [M]
  - Windows: MAX_PATH 260 unless long paths are enabled.
  - Reserved names (CON, NUL…) and forbidden chars (`<>:"|?*`) on Windows.
  - APFS/HFS+ are normalization-insensitive, so NFD/NFC differences cause duplicates.
  - Default macOS and Windows filesystems are case-insensitive.

### Inferences (patterns for SQLite + files)
- **Content-addressed store (CAS)** for app-owned blobs (clipboard images and files):
  1. Write `objects/tmp/<rand>`, then fsync the file.
  2. `rename` it to `objects/<sha256>`, then fsync the directory.
  3. Commit the DB row referencing the hash.

  A crash before the DB commit leaves an orphan blob, which is harmless and GC-able. The DB never references a missing file. Deletion goes the other way: commit the DB tombstone first, and let GC remove unreferenced blobs later.
- **User-editable files** (e.g. markdown task export):
  - Keep a DB table `fs_entry(id, path, inode/fileid, size, mtime_ns, hash, synced_hash)`. Its `synced_hash` plays the role of Dropbox's Synced tree.
  - A reconciler compares observed (scan) vs synced vs DB/remote, giving a 3-way decision.
  - Detect renames by (inode/fileID, hash) match between a disappeared path and an appeared path.
  - Treat the watcher as a hint; scan on startup and periodically.
  - Ignore your own temp files.
  - Debounce partial writes until size and mtime are stable, then hash.
- **Path keys:**
  - Store a normalized key (NFC + casefold) with a unique index so case or normalization collisions are detected before sync.
  - Reject or escape names that are illegal on any target OS.
  - Store symlinks as link entries; do not follow them.
- **Conflict policy:** binary files fork a copy (Syncthing style, named with device and time). Structured data in SQLite should never fork files. Resolve it in the DB.

### Gaps
- OneDrive, Git and OS-limit details are [M].
- No fetched source covers Windows ReadDirectoryChangesW overflow or FSEvents coalescing semantics.
- Obsidian's exact conflict filename is unconfirmed.

---

## Q3. Single-user multi-device conflicts in practice

### Takeaway
Real personal-productivity apps use one of three schemes:
- A server-ordered command/transaction log with idempotent commands: Todoist, Linear, OmniFocus, Things.
- Per-record optimistic concurrency with a merge-on-reject: CloudKit / Apple Reminders.
- Per-cell LWW with HLC: Actual.

None needs full CRDT text merging for task fields. The hard cases are domain semantics: recurrence, ordering, tree moves, deletes vs edits, and clipboard dedup/retention.

### Cited findings
- **Todoist Sync API.**
  - Commands carry `uuid` and optional `temp_id`, and run in order. The response has `temp_id_mapping` and a per-command `sync_status`: "ok" or an error with `error_tag/http_code`.
  - "Todoist will not execute a command that has same UUID as a previously executed command", which gives idempotent retry.
  - 400 errors are terminal; don't retry. `sync_token='*'` gives full sync; otherwise the sync is incremental. [F] https://developer.todoist.com/api/v1/
  - Ordering: `section_reorder` is deprecated in favour of `section_update` with `order_key`, because a reorder "updates a single section rather than rewriting the order of every section". This is a move from integer `child_order` rewrites toward fractional-style keys. [F] same page. Recurring `item_close` semantics were not retrieved.
- **CloudKit.** A save with a stale change tag returns `serverRecordChanged`. userInfo holds the client, server and ancestor records. Apple says to merge into the *server* record and retry. Merging into the others re-conflicts (old change tag). The ancestor may be fields-less if rebuilt from `encodeSystemFields`, which is anecdotal. [F] https://developer.apple.com/documentation/cloudkit/ckerror/serverrecordchanged ; https://developer.apple.com/documentation/cloudkit/record-changed-error-keys ; https://developer.apple.com/forums/thread/719333
- **Actual Budget.**
  - One table `messages_crdt(timestamp UNIQUE, dataset, row, column, value)`; each message sets one cell.
  - LWW by HLC timestamp. Clock drift over about 1 min is rejected; ties break by node ID.
  - A Merkle trie over timestamps finds the divergence point. The server is a dumb relay/store. [F] https://jlongster.com/using-crdts-in-the-wild
- **OmniFocus.** The server holds a chain of zip "transaction" files (add, modify or delete, including delete transactions). Sync merges diffs both ways. Compaction removes redundant transactions once all devices reach the same point; per users, it needs transactions older than about 1 h. Encryption is client-side. The compaction details come from user posts. [F] https://support.omnigroup.com/documentation/omnifocus/mac/3.12/en/getting-synced/ ; https://discourse.omnigroup.com/t/omni-sync-server-flooded-with-old-data/24600
- **Things Cloud.** Push-based: each change is sent immediately and relayed to other devices; offline changes are queued. No public conflict model; the design blog dates from 2010–11. [F] https://culturedcode.com/things/cloud/ ; https://culturedcode.com/things/blog/2010/12/state-of-sync-part-1/
- **Linear (reverse-engineered).** Client transactions → the server executes them → delta packets broadcast. A global monotonically increasing `lastSyncId` per executed txn means server total order, which is "closer to OT". [F] https://github.com/wzhudev/reverse-linear-sync-engine
- **Notion.** Per-block ops via a server transaction log. From 2025 it adopted CRDT-based offline text merging. [M] https://www.notion.com/blog/how-we-made-notion-available-offline
- **Clipboard managers.**
  - Raycast: clipboard history is explicitly *not* synced by Cloud Sync, "to ensure sensitive information stays on your Mac" (Cloud Sync beta on macOS/Windows/iOS, v0.69, 2026-07-16). [F] https://manual.raycast.com/cloud-sync.md ; https://www.raycast.com/changelog/macos/0-69
  - Paste: syncs history and pinboards via iCloud; off by default per device. A release note says duplicates from "iCloud sync races" are now collapsed. [F] https://pasteapp.io/help/where-paste-stores-your-data ; https://pasteapp.io/help/icloud-sync-doesn-t-work
  - iOS cannot read the clipboard in the background. [F] same.
  - Maccy has no sync; CopyQ has no built-in sync (scripts/git); Ditto syncs to other machines over the LAN via push. [M]
- **Trees.** Concurrent move-between-parents can create cycles. Kleppmann's move op uses undo-do-redo; Loro implements a movable tree. [F] https://martin.kleppmann.com/2021/10/07/crdt-tree-move-operation.html ; https://loro.dev/blog/movable-tree
- **Ordering.** Figma uses fractional indexing for sibling order. [M] https://www.figma.com/blog/realtime-editing-of-ordered-sequences/ . Jira uses LexoRank. [M]. Fractional indices interleave concurrent multi-item inserts; list CRDTs like Fugue avoid this. [M] https://arxiv.org/abs/2305.00583

### Inferences (conflict classes and recommended rules)
**Task manager**

| Conflict | Rule |
|---|---|
| Same field edited on two devices | Per-field LWW (HLC or server order). Text notes can use a text CRDT if you care. |
| Edit vs delete | Prefer soft-delete tombstones. A delete wins, but an edit after the delete resurrects the task (keep a "deleted_at < edited_at" check), or show a "restored" toast. |
| Ordering | Fractional string keys per parent (Todoist `order_key`, Figma). Tie-break on id. Rebalance on the server. |
| Move between parents | The server validates acyclicity. On a cycle, reject the later move, or use Kleppmann's rule (ignore the move that would create a cycle). |
| Recurring completion on two devices | Do *not* let clients create the next instance as a free insert. Model completion as an idempotent command `complete(task_id, occurrence_date)`. Derive the next instance deterministically (id = hash(series_id, occurrence_date)), so both devices generate the same row and it dedups by PK. |
| "Complete" | Idempotent by (task, occurrence). Uncomplete has its own timestamp. |
| Daily planning | "Today" lists are per-date sets (add-wins set keyed by (date, task_id)) plus a fractional order. |

**Clipboard manager**
- **Dedup:** id = sha256(normalized content + type), so the same copy on two devices becomes the same PK. Merge `last_used_at = max` and keep a devices set. This is the same fix Paste had to ship for duplicates from sync races.
- **Retention "keep last N":**
  - Define it globally, by HLC/`last_used_at` order over the merged set.
  - Eviction is a *derived* local GC, not a synced delete. Pinned items are excluded.
  - Explicit user deletes are synced tombstones.
  - This avoids "whose N wins".
- **Pin vs evict:** a pin is an explicit op, and eviction never tombstones remotely, so the pin always wins. A pin on an already-GC'd item re-fetches its blob if the server still has it.
- **Huge images:** store them in the CAS and upload lazily. Cap their size, or sync metadata and thumbnail only. Fetch the full blob on demand.
- **Privacy:** default to not syncing sensitive types (password-manager markers like `org.nspasteboard.ConcealedType`), as Raycast's stance suggests. [M for the pasteboard type]

### Gaps
- Todoist recurring `item_close` and batch limits were not retrieved: the page was truncated.
- Things Cloud internals are not public.
- Notion's offline-CRDT claim is [M].
- No authoritative Paste conflict policy was found.
- Ditto and CopyQ were not re-verified.

---

## Q4. Remote relational mapping (Postgres)

### Takeaway
Use Postgres as the authority and order changes by the server's commit order. Server→client goes through logical replication (pgoutput/wal2json) or an app-level change log with a monotonic `server_version` (bigint sequence or LSN). Client→server is an idempotent mutation endpoint. It applies each client txn in one PG txn, in client order, and reports per-mutation results back (Todoist `sync_status`, Zero mutation results, PowerSync "2xx + revert").

### Cited findings
- Electric, Zero and PowerSync all consume Postgres logical replication. Zero: changes replicate to zero-cache "via logical replication". Electric txids come from `pg_current_xact_id()`. [F] https://zero.rocicorp.dev/docs/mutators ; https://github.com/TanStack/db/pull/651
- PowerSync: constraint or validation violations from offline writes should be returned as 2xx, with details reported via the body or a separate table. The client reverts to server state. Unhandled errors block the queue. [F] https://docs.powersync.com/handling-writes/handling-write-validation-errors
- Todoist: errors per command (`error_tag`, `http_code`). A 400 is terminal. `temp_id` resolves FK references to rows created in the same batch. [F] https://developer.todoist.com/api/v1/
- Replicache: the server tracks `lastMutationID` per client, in the same txn as the mutation. A mutation that fails is still marked processed, so it doesn't block. [M] https://doc.replicache.dev/strategies/overview
- SQLite changeset apply exposes CONSTRAINT and FOREIGN_KEY classes, useful if Postgres changes are mirrored as changesets on the client. [F] https://www.sqlite.org/session/sqlite3changeset_apply.html

### Inferences (schema pattern)
- **Columns on every synced table:**
  - `id uuid/ulid` (client-generated; avoids temp_id remapping)
  - `user_id`
  - `server_version bigint` (set by a trigger from a sequence on every write)
  - `hlc text` (or per-field HLC in a side table, if you want Actual-style per-field LWW)
  - `deleted_at` (tombstone)
  - `updated_by_device`
- **Change feed:** a `WHERE user_id=$1 AND server_version > $cursor` pull is enough for single-user apps. Logical replication or CDC is only needed for multi-tenant fan-out or push.
  - Caveat [M]: sequence values are assigned before commit, so concurrent txns can commit out of order. A plain `> cursor` can skip rows. Fix with a per-user advisory lock or row lock around writes, or use `pg_current_xact_id` / snapshot `xmin` horizon filtering (as Electric does).
- **Tombstones:** keep them until all devices' cursors pass them, then GC. A device that is offline longer than the TTL must do a full resync.
- **FKs and upload order:**
  - Upload in client commit order, which inserts the parent before the child.
  - Use `DEFERRABLE INITIALLY DEFERRED` FKs so a whole client txn validates at commit.
  - Soft-delete means children referencing a tombstoned parent are still valid. The server then decides: cascade the tombstone, or resurrect the parent.
- **Unique constraints:**
  - Prefer natural-key dedup (clipboard content hash as PK makes `INSERT … ON CONFLICT DO UPDATE` the merge).
  - For user-visible uniques (project names), resolve on the server: rename "Inbox (2)" or merge. Don't reject, since offline users can't retry meaningfully.
- **Reporting:** for each mutation `{id, status: applied | rejected | rewritten, reason, server_row}`. The client drops the optimistic state and applies the server row. Rejected mutations are surfaced in a "sync issues" UI, never silently lost.
- **Objects:** upload the blob first (content-addressed key, idempotent PUT), then the row referencing it. Server-side GC handles unreferenced objects.

### Gaps
- No fetched source covers Postgres sequence/commit-order skew. It is [M] but well known; Electric and Zero docs on snapshot-based cursors would confirm it.
- Electric's write-path docs ("writes" guide) were not fetched.
- No source compared logical replication slot management costs.
