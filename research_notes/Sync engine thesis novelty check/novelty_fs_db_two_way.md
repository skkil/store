# Novelty check: two-way sync of user-editable files ↔ local SQLite ↔ cloud relational DB + object storage, with specified semantics

Scope: the claim in `reports/Local first sync engine research.md` §"Two-way filesystem ↔ database sync as a first-class replica" (mounts + codecs, fs_entry merge base, echo suppression, intent-log crash recovery, laws F1–F5).
As of 2026-11-04. Tags: [F] = page fetched and read; [S] = search-engine result summary (not fetched in full, treat as weaker); [M] = from model memory, not re-verified this session.

## Q1. Do products already keep files on disk AND a structured DB both editable and in sync?

### Takeaway
Yes, many do in narrow, hand-built forms. In each one, one side is the real source of truth and the other is a derived index or a manually reconciled copy. None I found treats the filesystem as a full replica peer with a per-file three-way merge base that joins multi-device row sync to a cloud relational DB, and none publishes semantics. The most direct competitor is Logseq's two-way Markdown Mirror. As of Oct 2026 it is announced but not shipped.

### Cited Findings
**Logseq (closest live competitor)**
- [F] 2026-05-16: Logseq DB shipped "Markdown Mirror": "Logseq writes a plain markdown projection of your blocks to a folder on disk and keeps it in sync as you edit". "Each block gets a stable id embedded in a comment so the mirror survives renames and moves". "Path collisions are resolved automatically". The two-way version "is in active development on a feature branch and not yet shipped". "Edits to the on-disk markdown will sync back into the DB graph, with ADR 0017 spelling out the model". The post says nothing about conflict semantics for two-way edits. — [Logseq forum, What's New May 16 2026](https://discuss.logseq.com/t/whats-new-with-logseq-db-may-16th-2026/35020)
- [F] Post dated 2026-08-01, updated 2026-10-20: the mirror is one-way ("edits flow from the DB to the files, not back"). "Two-way editing is still marked in progress on the public roadmap." — [listlis blog](https://listlis.com/blog/logseq-og-vs-database/)
- [S] A third-party npm package, `@geml/logseq-sync`, offers a two-way option for DB graphs. Its own docs call its Markdown output "one-way, LOSSY". — [socket.dev](https://socket.dev/npm/package/@geml/logseq-sync)
- [S] Logseq's older file graph has files as the source of truth, with a re-parse on external change. Users report silent overwrites when cloud sync clients (Dropbox, iCloud) rewrite files: "logseq reloaded the old files and threw away my changes". These are user reports, not root-cause analyses. — [Logseq forum: lost journal entries](https://discuss.logseq.com/t/lost-journal-entries/24270); [empty journal replaces copy on disk](https://discuss.logseq.com/t/empty-journal-replaces-copy-on-disk/3085)
- I could not find ADR 0017 in public Logseq GitHub search results.

**Photo catalogs with XMP sidecars (catalog DB ↔ file, two-way, manual conflict handling)**
- [S] Lightroom Classic can "Automatically Write Changes Into XMP" (DB → file) and "Read Metadata From File" (file → DB). It flags a "Metadata Was Changed Externally" state and a separate state for metadata changed in both Lightroom and on disk. The user resolves these by hand. — [Adobe help: advanced metadata actions](https://helpx.adobe.com/si/lightroom/help/advanced-metadata-actions.html)
- [S] Aug 2026 bug report: Lightroom "repeatedly detects metadata conflicts and writes XMP changes when no edits were made". This is a live example of failing echo freedom (F5) and GetPut (F2). — [Adobe community](https://community.adobe.com/bug-reports-674/p-lightroom-classic-repeatedly-detects-metadata-conflicts-and-writes-xmp-changes-when-no-edits-were-made-1635848)
- [F] darktable writes XMP sidecars ("never / on import / after edit"). With "look for updated XMP files on startup" (off by default), "a menu is opened for the user to choose which of the XMP files to reload". The choice is reload (XMP overwrites DB) or overwrite (DB overwrites XMP). No automatic or field-level merge is documented. — [darktable 4.0 manual, storage](https://docs.darktable.org/usermanual/4.0/en/preferences-settings/storage/)
- [S] darktable issue #5486: choosing "update database from selected xmp files" left the DB unchanged, and the XMP files were rewritten anyway. — [GitHub issue](https://github.com/darktable-org/darktable/issues/5486)
- [M] digiKam uses the same pattern (DB plus optional XMP sidecar read/write, with a manual "reread metadata from files" action).

**Markdown-canonical tools with a DB index and write-back**
- [S] Obsidian Bases (1.9+): table cells are frontmatter keys, and "editing a cell writes back to that note's frontmatter". The file stays canonical and the index is derived. These are community guides, not official docs. — [Atlas guide, 2026-05-30](https://laoujin.github.io/Atlas/research/2026-05-30-beginner-intermediate-obsidian-a-roadmap-for-a-developer-with-a-1-200-note-vault/bases-turn-existing-reference-notes-into-live-views/)
- [S] SilverBullet builds an object index from all pages. Task refs are `page@pos`, which is a positional identity, not a stable ID. "The index can be flushed at any time and be rebuilt from its source markdown files." — [SilverBullet Objects](https://silverbullet.md/Objects); [Tasks](https://silverbullet.md/Tasks)
- [S] mdbase spec v0.3.0 treats folders of Markdown and YAML frontmatter as typed, queryable collections that tools "update ... safely". The reference implementation uses a SQLite-backed cache. It is single-replica, with no multi-device sync semantics found. — [mdbase-spec](https://github.com/mdbase-dev/mdbase-spec)
- [S] TinaCMS: the DB (MongoDB/abstract-level) is "an ephemeral cache". "The single source of truth for your content is really your Markdown/JSON files." Edits made through the API go back to Git via `onPut`/`onDelete`. This is the file → DB index → cloud path with write-back, but Git is the sync layer and conflicts are Git's. — [Tina data layer](https://tina.io/docs/reference/content-api/data-layer); [self-hosted overview](https://tina.io/docs/self-hosted/overview)
- [M] Nuxt Content and Contentlayer compile Markdown into SQLite/JSON one-way. Keystatic and Decap edit files directly with no DB. org-roam keeps a one-way SQLite cache. Dataview is a read-only index.

**Structured file ↔ task rows with inline stable IDs (the `markdown_tasklist` codec idea)**
- [S] Taskwiki: Vimwiki checklist lines ↔ Taskwarrior DB, "bidirectional synchronization", linked by a UUID on each line. The earlier vimwiki-tasks appended the UUID to the line and re-synced on file open. Taskwarrior itself then syncs to a server. Conflict and delete semantics are undocumented. — [taskwiki](https://github.com/shfattig/taskwiki); [vimwiki-tasks](https://relatedrepos.com/gh/teranex/vimwiki-tasks)
- [S] Several Obsidian ↔ Todoist two-way plugins put the remote ID on the Markdown line: Markdoist (ID tag, temporary local placeholder ID), Todoist Bridge (inline Dataview field), and Sync Todoist (`<!-- todoist-id:... -->`). — [Markdoist](https://community.obsidian.md/plugins/markdoist); [Todoist Bridge](https://www.obsidianstats.com/plugins/todoist-bridge); [Sync Todoist](https://www.obsidianstats.com/plugins/sync-todoist)
- [M] Other point-to-point examples of the same pattern: org-caldav, org-gcal, org-jira, and org-trello (Org file ↔ remote service, with IDs stored in property drawers).

**Note apps with SQLite plus file sync targets**
- [S] Joplin: SQLite is canonical, and the sync target is files. Editing .md files in the target directly is unsupported: "use Joplin to edit your notes (or its external editor feature) and let the sync engine update the files". Conflicts are detected by timestamp and moved to a Conflict folder. A 2026 GSoC proposal says notes edited on several devices are not merged automatically. — [Joplin forum: WebDAV edits](https://discourse.joplinapp.org/t/joplin-does-not-show-latest-markdown-file-after-sync/23211); [sync overwrote changes](https://discourse.joplinapp.org/t/synchronization-overwritten-my-changes-no-conflicts-no-history/49626); [GSoC 2026 proposal](https://discourse.joplinapp.org/t/gsoc-2026-proposal-draft-idea-9-automatic-conflict-resolution-gherardi/49160)
- [M] Zotero: SQLite metadata is canonical. Stored attachment files sync through Zotero storage or WebDAV. "Linked files" are not synced. Files are not a structured editable replica of rows.
- [M] Calibre: `metadata.db` is canonical, and per-book OPF files are one-way backups (restorable, but not watched).
- [M] Bear, Craft, Tana, Capacities, AppFlowy, AFFiNE, Anytype, and Notion use DB or cloud stores with Markdown import/export only. They are not continuous two-way.
- [M] Dolt and Datasette are DB-native (git-for-data / publishing). Neither binds user-editable files as replicas.

### Inferences
- Every product falls into one of three groups: (a) files canonical with a derived index (Obsidian, SilverBullet, TinaCMS, mdbase), (b) DB canonical with files as export or backup (Logseq DB, Calibre, Joplin, darktable by default), or (c) two-way with manual whole-record conflict pick (Lightroom, darktable). None has both sides authoritative with an automatic three-way, field-level merge.
- Lightroom's 2026 false-conflict and rewrite loop, and darktable #5486, are useful evidence that F2 and F5 are hard and not solved in mature products.
- Logseq is a real threat to a "first" claim if ADR 0017 ships with a merge model. It is still scoped to one app, not a reusable SDK.

### Gaps
- The contents of Logseq ADR 0017 (the two-way conflict model) could not be found publicly.
- Taskwiki's delete and conflict semantics were not verified (README only).
- The Obsidian Bases write-back claim is sourced from community guides, not official help.
- Obsidian Sync, Foam/Dendron, Fava/beancount, and the CSV/Excel ↔ DB tools were not checked this session. From memory, none are two-way file ↔ relational with semantics.

## Q2. Research prior art: BX, lenses, file synchronizer specs, filestore DSLs

### Takeaway
The theory for every piece exists: lens laws, complements that preserve formatting, key-based alignment, DB-to-DB BX sharing, verified file-synchronizer specs, and transactional filestores. It exists in separate works. No paper combines file ↔ relational lenses with multi-replica sync, crash recovery and echo freedom.

### Cited Findings
- [F] TxForest (DiLorenzo, Mancini, Fisher, Foster; APLAS 2019) is a type-based filestore description. The compiler generates "load and store functions for mapping between in-memory and on-disk representations". It uses optimistic concurrency with serializable transactions (proved), and proves round-trip identities such as "storing just loaded data is equivalent to Skip". It does not cover crashes or recovery, has no distributed sync, and positions itself as an alternative to a DB. — [arXiv 1908.10273](https://ar5iv.arxiv.org/html/1908.10273)
- [S] Forest (Fisher, Foster, Walker, Zhu; ICFP 2011) is the predecessor: a Haskell-embedded filestore description language. — [ICFP'11 paper](https://www.cs.princeton.edu/~dpw/papers/forest-icfp11.pdf)
- [S] Dejima / BISCUITS: each peer has base tables and shared "Dejima tables", linked by BX that control "what to provide and what to accept". Peers form span compositions of BX. It is DB ↔ DB only, with no files. — [arXiv 2112.00288](https://arxiv.org/pdf/2112.00288); [BX 2018 lightweight data sharing](https://2018.programming-conference.org/event/bx-2018-papers-lightweight-data-sharing-system-based-on-bidirectional-transformations)
- [S] BIRDS (Tran Van Dang et al.) makes view-update strategies programmable, with verified putback over relational views. — [arXiv 1809.10357](https://arxiv.org/pdf/1809.10357)
- [S] Augeas parses config files into a tree with lenses and writes edits back, preserving comments and formatting. It is the production precedent for `codec_complement` and F2. It covers file ↔ tree only, with no DB or sync. — [Debian augeas-lenses](https://packages.debian.org/zh-cn/sid/misc/augeas-lenses)
- [M] Harmony / "Exploiting schemas in data synchronization" (Foster, Greenwald, Kirkegaard, Pierce, Schmitt; DBPL 2005 / JCSS 2007) is lenses plus a schema-aware sync spec. It syncs file to file. (Already cited in the report.)
- [M] Lens theory: relational lenses (Bohannon, Pierce, Vaughan, PODS 2006); Boomerang/resourceful lenses with key-based alignment (POPL 2008); quotient lenses (ICFP 2008); matching lenses (Barbosa et al., ICFP 2010); symmetric lenses (POPL 2011); edit lenses (POPL 2012); delta lenses (Diskin et al. 2011); incremental relational lenses (Horn, Perera, Cheney, ICFP 2018); BiGUL putback (Ko, Zan, Hu 2016). Resourceful and matching lenses address exactly the row-identity-under-reorder problem in structured files.
- [M] File synchronizer specs: Balasubramaniam & Pierce, "What is a file synchronizer?" (MobiCom 1998); Pierce & Vouillon, "What's in Unison?" (2004); Hughes, Pierce, Arts, Norell, "Mysteries of Dropbox" (ICST 2016), a property-based model of Dropbox's sync. These cover file ↔ file only.
- [M] Cambria (Ink & Switch, 2020) uses lenses for schema evolution over CRDT documents. It does not involve files.

### Inferences
- F1 and F2 are standard lens laws, and the thesis must cite them as such, not claim them. F3 is Harmony's "never overwrite". Serializable access to a filestore exists in TxForest.
- F4 (crash recovery across a non-atomic file write plus a DB commit, via an intent log) and F5 (echo freedom or quiescence across disk, SQLite and cloud) have no formal treatment in the BX or sync work found. Practitioners solve them ad hoc (the Dropbox "Synced tree", Syncthing).

### Gaps
- BX workshop proceedings from 2022–2026 were not exhaustively scanned. One CEUR BX 2022 paper surfaced ([ceur-ws Vol-3250](https://ceur-ws.org/Vol-3250/bxpaper1.pdf)) but was not read.
- No paper on "lenses + CRDT/row sync + filesystem" was found. Absence of evidence comes from about 15 searches, not a systematic review.

## Q3. Verdict: novel, partially novel, or done? Which sub-claim remains novel?

### Takeaway
The contribution is partially novel. The primitives are all prior art. The specific integration, with specified and tested end-to-end semantics, appears unclaimed as of Nov 2026. The open risk is Logseq's unshipped two-way mirror.

### Cited Findings
- Comparison matrix (sources as above):

| System | Directions | Specified semantics | Conflict handling | Cloud relational DB | Reusable SDK |
|---|---|---|---|---|---|
| Logseq DB Mirror (2026) | DB→file shipped; file→DB in dev | ADR 0017, not public | path collisions auto; edit conflicts unknown | own sync server | No |
| Lightroom / darktable | two-way (XMP) | No | manual whole-file pick | No | No |
| Obsidian Bases / SilverBullet / mdbase | file canonical, UI writes back | mdbase: format spec only | none / last write | No | mdbase: spec + libs |
| TinaCMS | file→index; API→Git | No | Git | Git, not relational | CMS framework |
| Taskwiki / Todoist plugins | file lines ↔ task rows, UUID per line | No | ad hoc | remote task service | No |
| Joplin | DB canonical; files = sync target | No | timestamp + Conflict folder | No | No |
| Harmony | file↔file via lenses | Yes | schema-aware, proven | No | research |
| TxForest | filestore ↔ memory | Yes (serializable) | OCC restart | No | research DSL |
| Dejima | DB↔DB via BX | Yes | BX put | Yes (peers) | research |

### Inferences
- **Not novel, so cite and do not claim:** lens laws F1/F2 and complements (Boomerang, Augeas); key-based alignment and stable inline IDs (resourceful/matching lenses; Taskwiki, Todoist plugins, Logseq mirror block IDs); catalog ↔ sidecar two-way sync (Lightroom, darktable); "never overwrite" F3 (Harmony); the merge base / Synced tree (Dropbox); conflict copies (Syncthing).
- **Defensible novel sub-claim:** a reusable, declarative engine in which a mounted folder is a full replica peer in a multi-device relational sync (SQLite ↔ Postgres + object storage). It has four parts:
  1. A per-file three-way merge base (`synced_hash`, `synced_row_version`).
  2. A reduction of structured-file conflicts into the engine's ordinary per-field row merge rules, rather than a whole-file pick (Lightroom, darktable, Joplin) or Git merges (TinaCMS).
  3. Opaque files bound to the object-storage lifecycle (leases, fences).
  4. Stated and tested end-to-end laws F4 (crash recovery for file-write plus row-commit via an intent log) and F5 (echo freedom / quiescence) across disk, local DB and cloud.
- Suggested honest phrasing: "first specified, app-agnostic treatment of the filesystem as a replica in a relational sync engine". Avoid "first two-way file ↔ DB sync".
- Risks: if Logseq ships ADR 0017 with field-level merge before the thesis, the claim narrows to "reusable SDK + cloud relational/object storage + F4/F5 proofs or tests". Lightroom's 2026 false-conflict loop is useful motivation that F2 and F5 are unsolved in practice.

### Gaps
- No exhaustive patent search was done. Dropbox, Apple File Provider and Microsoft Cloud Files patents may cover the merge-base and placeholder mechanics.
- The novelty verdict is bounded by about 15 searches and fetches in this session. A systematic ACM DL / DBLP sweep of "bidirectional transformation" + "file system" + "synchronization" from 2018–2026 is still needed before thesis submission.
