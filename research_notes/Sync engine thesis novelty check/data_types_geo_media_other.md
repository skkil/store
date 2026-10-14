# Specialized data types: offline↔cloud sync and SDK type-plugin requirements (as of 2026-11-04)

Tagging: [F] = retrieved this session (fetched page or search-result summary of a primary/vendor page); [M] = model memory, not verified this session, so treat as unconfirmed. Dates are as of November 2026 unless stated.

## Geospatial (GeoPackage, Mergin/QField/geodiff, ArcGIS, OSM, tiles, spatial partial sync)

### Takeaway
Vector GIS data is the closest fit for the target SDK. geodiff already does "SQLite/GeoPackage ↔ PostGIS changeset diff + rebase + conflict file", which is the same shape as the proposed local SQLite ↔ Postgres mutation log. Rows with geometry merge per-feature or per-attribute, with LWW or manual resolution. Tiles are derived, read-only bulk blobs that get region-scoped downloads, not merges. The new SDK needs are geometry-aware partial scopes (bbox/region), large initial snapshots, and geometry treated as one atomic field value.

### Cited Findings
- [F] geodiff (MerginMaps, MIT) is "a library for handling diffs for geospatial data". It compares two datasets with the same schema and produces a binary changeset of inserts, updates and deletes per row. It can apply, invert, and print the changeset as JSON. It also **rebases** a diff onto another diff to merge independently modified copies, and writes unresolvable conflicts (same row or value changed differently) to a separate conflict file — [geodiff GitHub](https://github.com/MerginMaps/geodiff)
- [F] geodiff drivers: SQLite/GeoPackage always; PostgreSQL/PostGIS optional at compile time. It can sync across backends (PostGIS schema ↔ GeoPackage) but needs identical table structure. Interfaces are a C API, a CLI, and pygeodiff, which ships inside the Mergin QGIS plugin — [geodiff GitHub](https://github.com/MerginMaps/geodiff)
- [F] Mergin Maps creates conflict files when it "could not synchronise changes automatically", e.g. `survey_polygon (edit conflict, sarah v4).json`. Causes include several surveyors editing the same feature and syncing projects with different schemas. Resolution is manual copy/paste in QGIS, or semi-automatic via the "Detect dataset changes" algorithm after aligning the schema — [Mergin Maps docs: missing data](https://merginmaps.com/docs/manage/missing-data/)
- [F] QFieldCloud: the device records edits as JSON **deltas** (create/patch/delete, with geometry and attributes) bundled into a deltafile and uploaded when online. The server validates schema and permissions, stores each delta, then runs an apply job. CONFLICT means the same primary key on the same layer was edited by two users, or a new feature reused an existing PK. The default policy is timestamp "last wins". The alternative is manual admin resolution with a "Re-apply" action. The docs recommend unique PKs and assigning disjoint feature sets per user — [QField docs: jobs](https://docs.qfield.org/it/reference/qfieldcloud/jobs/); [mirror of QFieldCloud docs: deltas](https://www.mintlify.com/opengisch/qfieldcloud/features/deltas) (third-party mirror, less authoritative); [qfieldcloud issue #609](https://www.github.com/opengisch/qfieldcloud/issues/609) shows the apply logic has had rough edges
- [F] Esri branch versioning: edits go through feature services. In the default version each edit is its own transaction with **no conflict detection (last update wins)**. Named versions (one level deep, branched from default) support reconcile, which pulls default and surfaces conflicts, and post, which merges into default and requires the admin if default is protected. History is kept as archived rows, and a Prune Branch History tool controls growth. Up to Enterprise 11.5 a named-version editor holds an exclusive lock. From 12.0, others can read until reconcile, and reconcile needs exclusive connection. Offline goes through "sync-enabled web feature layers" and Field Maps or Pro Download Map — [Esri: branch version scenarios](https://doc.esri.com/en/arcgis-pro/latest/help/data/geodatabases/overview/branch-version-scenarios.html)
- [F] Field Maps offline-area problems seen in the field (anecdotal, Esri Community): sync must be enabled per layer, and toggling it can unregister replicas so offline edits can't sync. Edits made online sometimes don't flow down to offline areas, with "recreate area" as the workaround. Removing an offline area discards unsynced edits — [Esri Community thread](https://community.esri.com/t5/arcgis-field-maps-questions/offline-map-area-unidirectional-sync/td-p/1304924); [Esri Community thread 2](https://community.esri.com/t5/arcgis-field-maps-questions/field-maps-offline-maps-unable-to-sync/m-p/1501129)
- [F] OSM API v0.6 concurrency model:
  - Every node, way and relation has a `version`. A modify or delete must supply the current version or the server returns **HTTP 409** (optimistic locking).
  - Edits must belong to an open changeset. A changeset closes after 10,000 edits, after 24 h open, or after 1 h idle. Changesets are **not atomic**; only a single diff upload (osmChange) is transactional.
  - The upload response maps placeholder negative IDs to new IDs and versions (`old_id`/`new_id`/`new_version`). Processing stops at the first error.
  - Limits: `delete if-unused` gives conditional deletes. CGImap caps diffs at 50 MB uncompressed. The `map?bbox` read fails above 50,000 nodes, and the max query area is 0.25 sq deg.
  - A bbox read returns ways that have ≥1 node inside the box (not ways that merely cross it), and relations non-recursively. This is a real-world example of how hard referential closure is in spatial partial sync.
  - Source: [OSM wiki API v0.6](https://wiki.openstreetmap.org/wiki/API_v0.6)
- [M] JOSM shows a conflict dialog on 409 that merges per element (tags, coordinates, node lists) after re-downloading the server version. iD rebases local edits onto fresh server data and asks the user to choose "keep mine/theirs" per conflicting entity.
- [F] PMTiles is a single-file archive of a tile pyramid read with HTTP Range requests: no tile server, but the host needs Range and CORS. `pmtiles extract` cuts a sub-archive by bbox or zoom range for offline bundles. MapLibre needs a protocol handler, and truly offline use means bundling the file locally — [MapLibre Flutter PMTiles docs](https://maplibre.org/flutter-maplibre-gl/advanced/pmtiles/); [Protomaps PMTiles docs](https://docs.protomaps.com/pmtiles/)
- [F] Mapbox offline: by default a cumulative cap of **6,000 tiles per user** across regions, roughly Greater London at z0–15. Tiles are shared across regions. Exceeding the cap fires a "tile count limit exceeded" event. From iOS SDK v5.3.0 there is no hard limit, but tiles are billed — [Mapbox Android OfflineRegionManager](https://docs.mapbox.com/android/maps/api/latest/mapbox-maps-android/com.mapbox.maps/-offline-region-manager/set-offline-mapbox-tile-count-limit.html); [Mapbox iOS legacy offline guide](https://docs.mapbox.com/ios/legacy/maps/guides/offline/) (legacy docs; v11 uses TileStore/"offline regions" — [M])
- [M] GeoPackage is an OGC standard SQLite container with gpkg_* metadata tables, an R-tree spatial index via triggers, and geometry stored as a GPKG-header + WKB blob. MBTiles is a SQLite file of a `tiles(zoom_level, tile_column, tile_row, tile_data)` table.

### Inferences
- geodiff is prior art the thesis must cite. It is a "SQLite ↔ Postgres changeset + rebase" engine, but it is file-oriented: the whole GeoPackage is the sync unit, rebase happens at sync time, and conflicts are dumped to a file. The SDK's differences would be a server-ordered log, per-field merge rules, and a generic schema (not GIS-specific).
- Geometry should be an **atomic LWW-or-manual field**. Nobody merges vertices automatically (OSM merges way node lists only with human help). A type plugin would provide WKB codec, validity checks, and an optional "geometry conflict → manual" policy.
- Spatial partial sync needs a scope plugin: the predicate is bbox or polygon intersection on an indexed column, plus **closure rules** (OSM's way/relation rule shows that naive bbox scoping breaks references). Moving features in and out of scope needs "move-out" tombstones.
- Tiles and basemaps are derived, read-mostly bulk data. They belong in the blob/media layer with region-pack manifests and range reads, not the mutation log.
- The OSM model (per-entity version + 409 + client-side rebase) is the same as the planned server-ordered optimistic log. It shows that "non-atomic long-running changesets + atomic batch uploads" is a workable separation.

### Gaps
- Not fetched: geodiff's changeset format doc (it appears to derive from the SQLite session extension, [M]), or how Mergin handles concurrent schema changes.
- No official Esri doc fetched on offline replica conflict rules (by object vs. by attribute; favor-edit vs favor-default) or on how Field Maps offline replicas interact with branch versions.
- No numbers on very large feature-set initial sync (e.g., millions of features) were found.

## Video/audio (chunked upload, derivatives, proxies, NLE collaboration, streaming FS, E2EE media)

### Takeaway
Media fits as **content-addressed immutable blobs + small mergeable/lockable project metadata**. Every leading pro tool uses one of three patterns:
- proxies plus relink by filename/timecode (Frame.io C2C, Resolve)
- pessimistic bin or timeline locking for the project graph (Resolve; Premiere [M])
- block streaming with a local cache (LucidLink)

None of them merges binary media. Derivatives (proxies, HLS renditions, thumbnails, waveforms) are regenerated, not synced as source.

### Cited Findings
- [F] tus 1.0.0 (2016-03-25):
  - HEAD returns `Upload-Offset`. A PATCH with a mismatched offset gets 409.
  - Extensions: creation (POST + `Upload-Length`/defer-length), creation-with-upload, expiration (`Upload-Expires`), checksum (sha1 required; 460 on mismatch), termination (DELETE), and concatenation (parallel partial uploads merged via `Upload-Concat`).
  - Source: [tus protocol](https://tus.io/protocols/resumable-upload)
- [M] An IETF HTTPBIS "Resumable Uploads for HTTP" draft (built on tus) is in progress. S3 multipart limits: 5 MiB–5 GiB parts (except the last), max 10,000 parts, max 5 TiB object.
- [F] Frame.io Camera to Cloud: on-set devices upload low-bandwidth proxies (e.g., 720p) with the **same filename, timecode and metadata** as camera originals. Editorial starts on proxies and relinks to the originals later. Devices such as Teradek Cube can buffer proxies locally for later upload. Canon EOS C400 and C80 transmit proxies natively. Relinking still differs by NLE (a 720p interpretation persists in one test) — [FiLMiC C2C help](https://filmicpro.helpscoutdocs.com/article/77-frame-io-camera-to-cloud-c2c); [Teradek](https://teradek.com/blogs/articles/frameio-c2c); [ProVideo Coalition test](https://www.provideocoalition.com/frame-io-brings-filmic-pro-to-the-cloud-testing-the-workflow/amp/); [Canon C400/C80 C2C](https://www.provideocoalition.com/canon-eos-c400-and-eos-c80-to-get-native-frame-io-camera-to-cloud/) (sources are 2021–2024)
- [F] DaVinci Resolve with Blackmagic Cloud: a project library is hosted on a chosen region server, and users are invited by Cloud ID. "Automatic bin and timeline locking": bins and timelines are **read-only until unlocked by the current user**. Optional media sync gives each collaborator a local copy. Blackmagic Proxy Generator makes proxies that auto-link to the originals. Project Server is the self-hosted alternative — [Blackmagic collaboration page](https://www.blackmagicdesign.com/uk/products/davinciresolve/collaboration). The claim that multi-user needs Studio comes from a vendor blog — [Fast.io](https://fast.io/resources/davinci-resolve-collaboration/) (unverified)
- [F] LucidLink Filespaces:
  - Splits files into chunks stored as objects in S3/Azure-compatible storage and **streams on demand**, so there is no full download.
  - A metadata service is the source of truth. A local cache with prefetching acknowledges writes locally (default cache reportedly 25 GB, from a single review).
  - Writes are compressed, encrypted, and sent in parallel to the object store. Connectivity is required; the cache is not an offline copy.
  - In 2026-03, "LucidLink Connect" streams existing S3 buckets without ingest.
  - Sources: [Blocks & Files 2019](https://blocksandfiles.com/2019/05/29/how-lucidlinks-metadata-acceleration-and-parallel-io-gets-faster-cloud-file-access/); [StorageReview](https://www.storagereview.com/review/lucidlink-filespaces-how-it-works); [TidBITS 2023](https://tidbits.com/2023/03/23/lucidlink-offers-streaming-vision-of-cloud-based-storage/); [Blocks & Files 2026-03-10](https://www.blocksandfiles.com/object/2026/03/10/lucidlink-connect-streams-s3-buckets-without-the-ingest-headache/5208840)
- [F] Ente E2EE media: file data is encrypted **in chunks with libsodium crypto_secretstream** (XChaCha20-Poly1305). The key hierarchy is masterKey → per-collection collectionKey → per-file fileKey, and fileKey also encrypts the file's metadata. The architecture page does not state chunk size or thumbnail handling — [Ente architecture](https://ente.com/architecture). [M] Ente's client code uses ~4 MiB secretstream chunks and encrypts thumbnails as separate objects (unverified).
- [F] Proton Drive E2EE: content is split into **blocks of max 4 MB**, each encrypted with the file's node/session key. The encrypted-block hashes are chained and signed with the uploader's address key, forming a manifest. Blocks are uploaded independently, then the revision is sealed — [Proton Drive security blog](https://proton.me/blog/protondrive-security); upload sequence per unofficial [proton-sdk-rs docs](https://docs.rs/crate/proton-sdk/0.3.0)
- [M] Adobe Premiere Team Projects (Creative Cloud-hosted project, local media, explicit "share my changes / get changes", conflict prompts) was deprecated in favor of Premiere Productions plus Frame.io integrations. Productions lock projects so only one user writes each project file, and the Production splits work into many small projects to reduce contention.
- [M] Splice Studio (DAW project versioning and backup) was discontinued in 2023. Ableton .als is gzip-compressed XML, referencing samples by path. Collect All and Save is the standard way to bundle media, so DAW collaboration is snapshot-and-lock, not merge.
- [M] Descript stores projects as a cloud document (transcript-anchored edit list) over media. Media is uploaded and transcoded server-side, and the composition is synced as structured data.

### Inferences
- The SDK's content-addressed media service already covers the source blobs. It needs a chunked, resumable upload protocol (tus or S3 multipart), per-chunk hashing so a resumed upload can check which chunks the server already has ("has-chunks" negotiation), and **range or streaming fetch** with cache eviction (LRU, pinned, with an "offline-available" flag).
- Derivatives should be a **derivation plugin**: (source hash, recipe, version) → derived hash. They are computed server-side or on-device, cached, never part of conflict resolution, and invalidated when the source or recipe changes. Proxies are a special case that must keep identity metadata (filename, timecode) for relinking.
- Project and timeline graphs need **lock policy plugins** (bin or timeline lease). Resolve's model maps to "record-group lease + read-only for others". Merging timelines is generally avoided in industry.
- For E2EE large media, the Proton/Ente pattern (per-file key, fixed ~4 MB blocks encrypted independently, signed block manifest) conflicts with cross-user dedup. Chunk dedup only works within one key domain, or with convergent encryption, which leaks equality.
- LucidLink shows that streaming a filesystem is different from offline sync: it needs connectivity. The SDK's two-way filesystem mounts should state clearly whether they hydrate fully (offline) or stream (online-only).

### Gaps
- Didn't verify the current Premiere Productions/Team Projects status, Frame.io V4 storage-connect features, or Ente's chunk size.
- No primary source fetched on HLS rendition pipelines or waveform generation as derived data. This is standard practice, [M].

## Other domains (time-series/IoT, 3D/CAD/BIM, spreadsheets, ML datasets, binary app docs, photos+XMP)

### Takeaway
- Time-series and IoT data is append-only and needs no merging, only buffered, prioritized, at-least-once export with TTL and downsampling.
- CAD/BIM uses either immutable object graphs with branches (Speckle, Onshape) or pessimistic element borrowing (Revit).
- Office and Figma show that "structured doc + server authority + per-property LWW/locks" works even for "binary" app formats once the format is decomposed.
- ML datasets need content-addressed snapshots with branch pointers (lakeFS/DVC), not row sync.

### Cited Findings
- [F] Ditto:
  - An embedded "small peer" database with **CRDT** merge by data type and peer-to-peer mesh over BLE, P2P Wi-Fi and LAN, plus an optional server.
  - Query subscriptions decide what each peer syncs, and property-level deltas reduce bandwidth over BLE.
  - mTLS between peers.
  - Example from the vendor blog: two scanners converge on a combined count rather than overwriting each other.
  - Sources: [Ditto docs: about](https://docs.ditto.live/home/about-ditto); [Ditto docs: syncing data](https://docs.ditto.live/key-concepts/syncing-data); [Ditto blog 2025-09-30](https://www.ditto.com/blog/offline-online-always-consistent) (vendor claims)
- [F] AWS IoT Greengrass stream manager: local durable streams with TTL. Automatic export to S3, Kinesis and other destinations, batched by size or interval. Streams have **export priority** (higher priority first). **Greengrass V1 support ended 2026-10-07**, so V2 is current — [AWS docs: stream export configurations](https://docs.aws.amazon.com/greengrass/latest/developerguide/stream-export-configurations.html); [V1 page with EOL notice](https://docs.aws.amazon.com/greengrass/v1/developerguide/stream-export-configurations.html). Behavior when storage fills was not confirmed (gap).
- [F] Speckle: objects are serialized with **content-hash IDs** (immutable). Large objects are decomposed into a graph of child objects. A version (commit) is an immutable snapshot pointing at a root object hash. Models are labels, and objects can exist without a version. Federation commits reference other commits — [Speckle architecture](https://deploy-preview-220--speckle-docs.netlify.app/dev/architecture); [Speckle data schema concepts](https://docs.speckle.systems/developers/data-schema/concepts); [Speckle forum: federating](https://speckle.community/t/federating-speckle-models/5047)
- [F] Onshape: a version graph of workspaces, branches and immutable versions. Merges are **per tab**; you "cannot select individual changes within a tab". Each tab gets a strategy: Keep target, Merge changes (default for Part Studios and assemblies, which can error), or Replace with source. **Drawings and PCB Studios can't be merged** (keep or replace only). Merging is blocked while others have the document open, and it can only be reverted via a revert link — [Onshape help: merging](https://cad.onshape.com/help/Content/Document/merging.htm); [Onshape versions & history](https://cad.onshape.com/help/Content/Document/versions_and_history.htm)
- [F] Revit worksharing: editing an unowned element makes you its **borrower** automatically. If someone else holds it you must send an editing request. Synchronize with Central pushes local changes, pulls others' changes, and by default relinquishes borrowed elements. Never edit the central model directly — [Autodesk Revit help: best practices](https://help.autodesk.com/cloudhelp/2022/ENU/Revit-Collaborate/files/GUID-80D390B3-D751-4703-BC79-1A66497C1928.htm); [Autodesk: saving/reloading workshared](https://help.autodesk.com/cloudhelp/2015/ENU/Revit-Collaborate/files/GUID-9AE28D47-8A56-466D-8F59-902322C44533.htm)
- [M] Autodesk ACC / BIM 360 Design Collaboration hosts Revit Cloud Worksharing (cloud central model) plus "packages" exchanged between teams, i.e. publish/consume snapshots rather than live merge.
- [F] Microsoft Office co-authoring (MS-FSSHTTP/"Cobalt"):
  - A file is split into **partitions** (binary content, coauthoring info, format-specific data), each synced independently and incrementally via MS-FSSHTTPB with a client cache.
  - Concurrent uploads: all but the first fail with a **coherency error**. The client then downloads, merges, and re-uploads, i.e. optimistic concurrency.
  - The server supports exclusive and **shared (coauthoring) locks**.
  - Source: [MS-FSSHTTP overview](https://learn.microsoft.com/en-us/openspecs/sharepoint_protocols/ms-fsshttp/6d078cbe-2651-43a0-b460-685ac3f14c45); [applicability](https://learn.microsoft.com/en-us/openspecs/sharepoint_protocols/ms-fsshttp/7566450e-ed16-4e26-a7b7-2ebd55bd57df)
- [F] Figma multiplayer (2019):
  - Server-authoritative, with a per-document server process. Not OT, and CRDT-inspired but not a pure CRDT.
  - **Per-property LWW** on an object tree. Client IDs let objects be created offline. The parent link and fractional index are stored as one atomic property, and the server rejects cycles.
  - Reconnecting after offline re-applies local edits onto a fresh copy of the document.
  - Comments, teams and projects live in Postgres and are synced separately.
  - Source: [Figma blog: multiplayer](https://www.figma.com/blog/how-figmas-multiplayer-technology-works/)
- [M] Excel co-authoring merges at cell level with server-side last-writer-wins per cell, plus recalculation. Formulas referencing moved ranges are the hard case. Google Sheets uses OT-style server sequencing.
- [M] Sketch moved to a cloud document model ("Sketch workspace" with real-time collaboration). The local .sketch file is a zip of JSON plus assets, which some tools diff (e.g., Abstract, now shut down).
- [F] lakeFS: branches are **metadata pointers to commits**, so creating a branch is zero-copy and objects are duplicated only when changed. Metadata uses Graveler, a content-addressable KV format with hash-addressed Range nodes. lakeFS acquired DVC. DVC keeps data in cloud storage, version pointers in Git, and moves bytes through a local cache (`dvc add/push/pull`). dvc-to-lakefs imports without re-upload — [lakeFS: data dev environment](https://lakefs.io/building-a-data-development-environment-with-lakefs/); [DVC blog: migrate to lakeFS](https://dvc.org/blog/migrate-dvc-to-lakefs/); [lakeFS acquires DVC](https://lakefs.io/?p=28068)
- [M] Photos with sidecars: Lightroom Classic, darktable and others write non-destructive edits to XMP sidecar files (or embed them in DNG/JPEG). Sync tools (e.g., Syncthing, Nextcloud, Immich) treat the sidecar as a separate file tied to the original by naming convention, and conflicts produce duplicate sidecars.

### Inferences
- Time-series data should be a **stream type**: append-only partitions keyed by (device, time bucket), no conflicts, idempotent IDs (device + sequence), local TTL and quota with drop or downsample policy, priority classes, and server-side rollups as derived data. Routing it through a per-field-merge mutation log would waste overhead.
- Speckle's model (content-hashed object graph + immutable commits pointing at a root) matches the SDK's content-addressed blob layer. Hierarchical decomposition could be a generic **chunking/decomposition plugin** for any tree-structured document, giving structural dedup.
- Revit borrowing and Resolve bin locks are the same primitive: **element-level or group-level leases**, held offline at the risk of being stale, released on sync. The SDK lock plugin needs grant, request, relinquish-on-sync, and an admin "force release".
- Onshape and Office show that merge granularity is a type decision (tab, partition, cell). Some sub-types are always keep/replace only (drawings), which matches a merge plugin returning "unmergeable → pick one".
- Figma is the best precedent that the SDK's server-ordered log with per-field LWW and client-generated IDs scales to rich "app documents" once they are decomposed into objects and properties. Opaque binary formats (.als, .psd, .blend, .rvt files) remain lock-only.

### Gaps
- Didn't fetch primary docs for Excel cell-merge internals, Sketch's current file and collab model, ACC packages, or XMP sidecar sync behavior in specific tools.
- No current (2026) primary info gathered on Splice or Ableton collaboration features.

## Cross-cutting patterns and synthesis: type-plugin extension points and poor fits

### Takeaway
One SDK can cover these domains if a "data type" is a plugin bundle with these hooks:
- **codec** (local repr ↔ wire ↔ file)
- **diff/merge** (field, row, or tree granularity; returns merged, conflict, or unmergeable)
- **chunker** (fixed, CDC, or structural decomposition)
- **derivation** (recipe → derived artifacts, not synced as source)
- **scope** (predicate + closure for partial sync: bbox, time window, subtree)
- **lock policy** (none, advisory, lease/borrow, exclusive check-out)
- **retention/eviction** (TTL, downsample, LRU, pin)
- **fetch mode** (eager, lazy, range/stream)

Poor fits: real-time streaming media, online-only streaming filesystems, mesh/P2P without a server (Ditto), and tools whose project format is proprietary and opaque, which only work with locks.

### Cited Findings
- [F] Optimistic concurrency with version checks is everywhere: OSM 409 on version mismatch ([OSM wiki](https://wiki.openstreetmap.org/wiki/API_v0.6)), tus 409 on offset mismatch ([tus](https://tus.io/protocols/resumable-upload)), and the Office coherency error with download-merge-retry ([MS-FSSHTTP](https://learn.microsoft.com/en-us/openspecs/sharepoint_protocols/ms-fsshttp/6d078cbe-2651-43a0-b460-685ac3f14c45)).
- [F] Pessimistic locking for unmergeable data: Git LFS `lockable` files are made **read-only on disk** until `git lfs lock`, and the attribute works without LFS storage ([Git LFS wiki: File Locking](https://github.com/git-lfs/git-lfs/wiki/File-Locking)). Revit element borrowing ([Autodesk](https://help.autodesk.com/cloudhelp/2022/ENU/Revit-Collaborate/files/GUID-80D390B3-D751-4703-BC79-1A66497C1928.htm)). Resolve bin and timeline locks ([Blackmagic](https://www.blackmagicdesign.com/uk/products/davinciresolve/collaboration)). Esri named-version exclusive lock before 12.0 ([Esri](https://doc.esri.com/en/arcgis-pro/latest/help/data/geodatabases/overview/branch-version-scenarios.html)). [M] Perforce `+l` exclusive-open filetype is the classic game-asset pattern.
- [F] Manual conflict artifacts are the norm in GIS: geodiff conflict files and Mergin "(edit conflict, user vN)" files ([geodiff](https://github.com/MerginMaps/geodiff); [Mergin](https://merginmaps.com/docs/manage/missing-data/)), and QFieldCloud manual "Re-apply" ([QField](https://docs.qfield.org/it/reference/qfieldcloud/jobs/)).
- [F] Content-addressed immutable snapshots plus mutable pointers: Speckle versions → root hash ([Speckle](https://docs.speckle.systems/developers/data-schema/concepts)), lakeFS branches → commits ([lakeFS](https://lakefs.io/building-a-data-development-environment-with-lakefs/)), and Proton's signed block-hash manifests ([Proton](https://proton.me/blog/protondrive-security)).
- [F] Partial sync by scope: Ditto query subscriptions ([Ditto](https://docs.ditto.live/key-concepts/syncing-data)), OSM bbox reads with closure caveats ([OSM](https://wiki.openstreetmap.org/wiki/API_v0.6)), PMTiles extract by bbox/zoom ([MapLibre](https://maplibre.org/flutter-maplibre-gl/advanced/pmtiles/)), and Mapbox region packs with a shared-tile cap ([Mapbox](https://docs.mapbox.com/ios/legacy/maps/guides/offline/)).
- [F] Stream versus hydrate: LucidLink streams chunks via cache but needs connectivity ([StorageReview](https://www.storagereview.com/review/lucidlink-filespaces-how-it-works)). Resolve optionally syncs full local media copies ([Blackmagic](https://www.blackmagicdesign.com/uk/products/davinciresolve/collaboration)).

### Inferences
Domain → plugin mapping. [Inference from the findings above]

| Domain | Data shape | Sync unit / conflict | Size/perf | SDK hooks needed | Fit |
|---|---|---|---|---|---|
| Vector GIS (GeoPackage/PostGIS) | rows + WKB geometry + R-tree | per-feature/attribute; geometry atomic; LWW or manual | 10^5–10^7 features; large initial snapshot | geometry codec, spatial scope + closure + move-out, conflict-to-review queue, snapshot bootstrap (ship a GeoPackage file and then the log) | Strong |
| OSM-like topology | nodes/ways/relations with refs | per-entity version, 409, client rebase | referential integrity | reference-aware scope closure, placeholder-ID remap, "delete if-unused" | Good |
| Basemap tiles | immutable tile archives | none (derived/read-only) | GB-scale | blob + region-pack manifest, range fetch, quota | Good (blob layer) |
| Video/audio media | large immutable binaries | none on media | GB–TB; resumable | chunked resumable upload, range/stream fetch, eviction/pin, derivation plugin (proxy/HLS/thumbs/waveform), E2EE block encryption | Good (blob layer) |
| NLE/DAW project | object graph or opaque file | locks (bin/timeline lease); opaque file = exclusive | small–medium | lock policy plugin; tree decomposition if format is open | Medium; poor if opaque |
| Time-series/IoT | append-only points | no conflicts | high rate, constrained storage | stream type: idempotent seq IDs, TTL, downsample, priority export, server rollups | Medium (separate path) |
| CAD (Onshape-like) | feature tree / parametric history | branch + per-tab merge; some keep/replace only | medium | branch/version pointers, merge plugin returning unmergeable | Medium |
| BIM (Revit-like) | element DB | element borrowing | very large models | element leases, relinquish-on-sync | Medium |
| Speckle-like 3D objects | hashed object DAG | immutable commits | large meshes | structural chunker, CAS dedup | Strong (fits CAS) |
| Spreadsheets | cell grid + formulas | per-cell LWW; structural ops hard | 10^6 cells | sparse cell keys, row/col ID indirection (fractional index), recalculation as derived data | Medium |
| App docs (Figma-like) | object tree + props | per-property LWW, server-ordered | medium | fractional index + parent atomic, cycle rejection | Strong |
| Office binary docs | partitions | coherency error + shared lock | medium | partition chunker + optimistic retry + shared lock | Medium |
| ML datasets | huge file sets | snapshot/branch, no row merge | TB | CAS + branch pointers, lazy fetch, GC | Medium (versioning, not live sync) |
| Photos + XMP | binary + small XML sidecar | sidecar mergeable by field; image immutable | medium | sidecar codec (XMP ↔ fields), binding of sidecar to blob hash | Strong |

- Poor fits or out of scope:
  - live or real-time A/V (WebRTC)
  - online-only streaming filesystems (LucidLink-style), because the value there is latency, not offline
  - serverless P2P mesh (Ditto/BLE), because a server-ordered log assumes server reachability
  - very high-rate telemetry if forced through the per-field merge log
  - proprietary opaque project files (.rvt, .als, .prproj, .psd), where only locks and whole-file versioning are possible
- Thesis angle: published systems each solve one domain (geodiff for GIS, tus/Proton for blobs, Revit/Resolve for locks, Speckle/lakeFS for CAS snapshots). Their common primitives are version-checked optimistic writes, CAS blobs + pointers, scoped subscriptions, leases, and derived-data recipes. Combining all of them in one plugin interface over a single SQLite↔Postgres log looks uncovered in the sources reviewed, though this session did not include an exhaustive literature search.

### Gaps
- No academic prior art on a "type-plugin" sync framework covering GIS + media + CAD was searched. Novelty needs a dedicated literature check (e.g., Ink & Switch, Automerge/Peritext, Electric/PowerSync docs on custom types).
- No benchmarks gathered on per-feature log overhead versus geodiff file-level changesets.
