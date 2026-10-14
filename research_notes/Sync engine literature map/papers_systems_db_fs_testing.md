# Systems, DB, file sync and testing: annotated bibliography

Appendix input for `reports/Sync engine thesis novelty check.md`. Claim IDs: C1–C8 (contributions), S1–S6 (safety invariants inside C3/C6), F1–F5 (file laws inside C4), D1.

Link legend:
- **[F]**: the link was opened or confirmed by search in this session.
- **[F·prior]**: the link was verified in earlier session notes (`research_notes/Sync engine thesis novelty check/*.md`) and reused here.
- **[M]**: the citation comes from memory. The URL or venue details are unverified.

Built November 2026 from about 26 tool calls. This is not a systematic review.

---

## 1. Transactional and mixed consistency (foundations)

### Highly Available Transactions: Virtues and Limitations
- Bailis, Davidson, Fekete, Ghodsi, Hellerstein, Stoica · PVLDB 7(3) · 2014
- https://www.vldb.org/pvldb/vol7/p181-bailis.pdf [F·prior]
- Summary: The paper maps which isolation and session guarantees are achievable with high availability. It defines Monotonic Atomic View and shows that read committed, MAV and causal session guarantees are available, while SI and serializability are not.
- Relevance: C1 must cite this as the taxonomy for client-side guarantees. MAV underpins "multi-row mutation visible atomically" (S1).

### Scalable Atomic Visibility with RAMP Transactions
- Bailis, Fekete, Hellerstein, Ghodsi, Stoica · SIGMOD 2014
- http://www.bailis.org/papers/ramp-sigmod2014.pdf [F·prior]
- Summary: RAMP gives read-atomic multi-partition transactions without coordination, using per-write metadata so that readers can repair fractured reads.
- Relevance: Prior art for C1/C3a. A pending row and blob reference becoming visible together is a read-atomic problem.

### Coordination Avoidance in Database Systems (I-confluence)
- Bailis, Fekete, Franklin, Ghodsi, Hellerstein, Stoica · PVLDB 8(3) · 2015
- https://dcmpx.remotevs.com/org/bailis/www/PL/papers/ca-vldb2015.pdf [F·prior] (a mirror; the canonical copy is vldb.org/pvldb/vol8/p185-bailis.pdf [M])
- Summary: An invariant can be kept without coordination if and only if the set of invariant-satisfying states is closed under merge (I-confluence). Foreign keys under insert are I-confluent. Uniqueness and some deletes are not.
- Relevance: This is the frame for C3b/C6. I-confluence assumes every operation is eventually accepted, so authority-driven rejection (S2) falls outside it. That gap is the thesis's opening.

### Keeping CALM: When Distributed Consistency Is Easy
- Hellerstein, Alvaro · CACM 63(9) · 2020
- https://arxiv.org/abs/1901.01930 [M]
- Summary: Monotonic programs have coordination-free, consistent implementations, and non-monotonic programs need coordination.
- Relevance: Explains why rejection and revocation (S2, S3) need a coordination point, which justifies the server-ordered log in C1/C6.

### Keep CALM and CRDT On
- Laddad, Power, Milano, Cheung, Crooks, Hellerstein · PVLDB 16(4) · 2022/23
- https://arxiv.org/pdf/2210.12605.pdf [F]
- Summary: CRDTs converge on updates, but reads of their state carry no guarantees. The paper applies CALM monotonicity to decide which queries over CRDT state can run without coordination, and sketches a CRDT data store.
- Relevance: The nearest DB-venue treatment of "safe reads on a weakly consistent replica". Relevant to C5/C7 and to stating S1 as a monotone query.

### Cure: Strong Semantics Meets High Availability and Low Latency
- Akkoorath, Tomsic, Bravo, Li, Crain, Bieniusa, Preguiça, Shapiro · ICDCS 2016
- https://arxiv.org/abs/1602.01425 [F·prior]
- Summary: Defines Transactional Causal Consistency (TCC), which combines causal consistency, atomic visibility and CRDT convergence. It is AntidoteDB's protocol.
- Relevance: TCC is the strongest available model, and C1's client semantics should be positioned against it.

### Highly-Available and Consistent Group Collaboration at the Edge with Colony
- Toumlilt, Sutra, Shapiro · Middleware 2021 · DOI 10.1145/3464298.3493405
- https://hal.archives-ouvertes.fr/hal-03353663 [F]
- Summary: A forest topology runs from the core cloud out to far-edge peer groups. It guarantees TCC+ globally and snapshot isolation inside edge groups, uses CRDTs, and supports offline mode and migration between groups.
- Relevance: Closest academic prior art for **mixed consistency with partial replication at the edge** (C1, S3/S4). It threatens any claim to novelty in a "hybrid client/server consistency" topology. Note that its GitHub README wrongly says Middleware'22.

### Putting Consistency Back into Eventual Consistency (Indigo)
- Balegas, Duarte, Ferreira, Rodrigues, Preguiça, Najafzadeh, Shapiro · EuroSys 2015
- https://doi.org/10.1145/2741948.2741972 [M]
- Summary: "Explicit consistency": invariants are declared, conflicting operations are identified, and multi-level reservations (escrow) let them run without coordination where possible.
- Relevance: Prior art for invariant-preserving offline writes. Contrasts with C2 claims and S5 leases, since reservations are also time-bounded rights.

### IPA: Invariant-Preserving Applications for Weakly Consistent Replicated Databases
- Balegas, Duarte, Ferreira, Rodrigues, Preguiça · PVLDB 12(4) · 2019
- https://www.dpss.inesc-id.pt/~rodrigo/vldb-2019.pdf [F·prior]
- Summary: Static analysis finds operation pairs that break invariants and adds repair effects ("touch"), so that referential integrity, uniqueness and aggregate invariants hold without coordination.
- Relevance: The main threat to C3b/S2. IPA already repairs RI under concurrency. The thesis's delta is post-hoc authority rejection plus blob and claim side effects.

### AQL / AntidoteSQL: Towards a Relational Database Interface for Geo-Replicated Systems
- Lopes, Sousa, Preguiça et al. · 2019 (arXiv); related Antidote line
- https://github.com/JPDSousa/AntidoteSQL [M]
- Summary: A SQL interface over Antidote CRDTs with declarable referential-integrity policies, such as update-wins and delete-wins with cascade.
- Relevance: Prior art for "cascade on conflict" (C3b). Its cascade is a merge policy, not a compensation after rejection.

### Conflict-free Replicated Relations for Multi-Synchronous Database Management at Edge
- Weihai Yu, Claudia-Lavinia Ignat · IEEE SMDS 2020
- https://hal.inria.fr/hal-02983557 [M]
- Summary: CRDT semantics are layered onto SQLite relations (CRR), so that edge replicas run SQL offline and merge row and attribute state.
- Relevance: Direct prior art for C5 (per-attribute merge) and for SQLite as a replica (C1). Must cite.

### Referential Integrity under Weak Consistency (Shapiro, Bieniusa, Zeller, Petri)
- PaPoC 2018
- https://arxiv.org/pdf/1803.03482 [F·prior]
- Summary: Argues that causal consistency is enough to maintain referential integrity, and gives CRDT pseudocode.
- Relevance: Supports S1, since the dangling-reference freedom is achievable without strong consistency for rows. Blobs (C3a) are the extension.

---

## 2. Partial replication, edge and local-first (2024–2026 additions)

### Practical Client-side Replication: Weak Consistency Semantics for Insecure Settings
- van der Linde, Leitão, Preguiça · PVLDB 13(11) · 2020
- https://www.vldb.org/pvldb/vol13/p2590-linde.pdf [F]
- Summary: Studies client-side causal replication when clients may misbehave, and which weak-consistency guarantees survive malicious clients.
- Relevance: Bears on C6/S6 (an untrusted client, a validator on the server) and on partial client replicas (S3).

### Partially Replicated Causally Consistent Shared Memory: Lower Bounds and an Algorithm
- Xiang, Vaidya · PODC 2018 (BA) / arXiv 1703.05424
- https://arxiv.org/pdf/1703.05424 [F]
- Summary: Gives lower bounds on timestamp metadata for causal consistency when each replica stores only a subset of objects.
- Relevance: The theoretical cost of S3/S4 on partial replicas. Cite it if the thesis claims cheap causal metadata for shape-subscribed clients.

### Towards Local-First Distributed Property Graphs
- Pandey, Dumbrava, Shapiro, Ferreira, Pereira, Preguiça · PaPoC 2025
- https://papoc-workshop.github.io/2025/program.html [F] (program listing only; the paper itself was not opened)
- Summary: Extends local-first replication to property graphs, from the Shapiro and Preguiça group. Details are unverified.
- Relevance: Evidence that "local-first beyond documents" (C8) is an active 2025 research line, so C8 should not claim it is the first non-document local-first work.

### Towards System-Oriented Formal Verification of Local-First Access Control
- Jacob, Stuber, Hartenstein · arXiv 2604.23560 · April 2026
- https://arxiv.org/abs/2604.23560 [F]
- Summary: Formally verifies a replicated data type for capability-based access control in small Byzantine-tolerant collaboration groups, using hash chronicles for replication.
- Relevance: The nearest 2026 work on S3 (revocation) and S6 (verified rules on clients). It is peer-to-peer and capability-based, while the thesis has a server authority.

---

## 3. Blob and metadata consistency, and GC

### Delta Lake: High-Performance ACID Table Storage over Cloud Object Stores
- Armbrust et al. · PVLDB 13(12) · 2020
- https://vldb.org/pvldb/vol13/p3411-armbrust.pdf [F]
- Summary: An ACID transaction log stored in the object store decides which immutable objects belong to a table, and it gives time travel and safe caching. Vacuum removes files that are no longer referenced, after a retention window [M: retention detail not confirmed this session].
- Relevance: The strongest academic analog for C2/S1/S5. The metadata log is authoritative over immutable blobs, and GC uses a time-based retention, which is close to a lease. The thesis's delta is retention tied to the pending references of *offline replicas*.

### Leases: An Efficient Fault-Tolerant Mechanism for Distributed File Cache Consistency
- Gray, Cheriton · SOSP 1989
- https://dl.acm.org/doi/10.1145/74850.74870 [M]
- Summary: A lease is a time-bounded grant, so a holder that has failed or is unreachable does not block progress forever.
- Relevance: Required citation for S5 and the C2 lease, claim and fence lifecycle.

### Pure Operation-Based CRDTs (causal stability for GC)
- Baquero, Almeida, Shoker · DAIS 2014 / DC 2017
- https://arxiv.org/abs/1710.04469 [M]
- Summary: Uses causal stability to prune metadata and tombstones once every replica has seen an operation.
- Relevance: The alternative to leases for S5. Causal stability stalls on an offline replica, so it motivates leases with expiry.

---

## 4. File synchronizer correctness

### What Is a File Synchronizer? / What's in Unison?
- Balasubramaniam, Pierce · MobiCom 1998; Pierce, Vouillon · TR 2004 (Coq-verified reference implementation)
- https://www.researchgate.net/publication/32205844_What's_in_Unison_A_Formal_Specification_and_Reference_Implementation_of_a_File_Synchronizer [F]
- Summary: A formal specification of state-based two-replica sync. Conflicts are flagged, never auto-resolved, and a reference implementation is proved against the spec in Coq.
- Relevance: The origin of "never silently overwrite" (F3) and of merge-base reasoning for C4.

### An Algebraic Approach to File Synchronization
- Ramsey, Csirmaz · ESEC/FSE 2001
- https://www.cs.tufts.edu/~nr/pubs/sync.pdf [F]
- Summary: Reasons about operation sequences rather than states, and finds essentially the same conflicts as state-based sync.
- Relevance: Theory for F5 (echo freedom), since an operation algebra makes no-op and echo cancellation explicit.

### Co-Design and Verification of an Available File System
- Najafzadeh, Shapiro, Eugster · VMCAI 2018
- https://www.lip6.fr/Marc.Shapiro/papers/2018/VMCAI-2018-filesys.pdf [F]
- Summary: Uses CISE and Event-B to verify a replicated file system that keeps tree invariants, such as no cycles under concurrent moves, without coordination.
- Relevance: Prior art for verified folder-replica semantics (C4) and for method choice, since F1–F5 could be machine-checked the same way.

### Syncpal: A Simple and Iterative Reconciliation Algorithm for File Synchronizers
- Shekow · DAIS 2019
- https://link.springer.com/chapter/10.1007/978-3-030-22496-7_1 [F]
- Summary: An iterative reconciliation algorithm over a formally defined file-system model, with steps argued to be provably correct.
- Relevance: The closest recent "correct file sync algorithm" paper. Cite it beside the Dropbox Nucleus work for C4.

### ViewBox: Integrating Local File Systems with Cloud Storage Services
- Zhang, Dragga, Arpaci-Dusseau, Arpaci-Dusseau · FAST 2014
- https://www.usenix.org/conference/fast14/technical-sessions/presentation/zhang [F·prior]
- Summary: Shows that Dropbox-style sync propagates local corruption and crash inconsistency to the cloud, and fixes this with checksums and consistent snapshots (view-based sync).
- Relevance: Direct prior art for F4 (crash recovery). Sync must only upload a crash-consistent view.

### Mysteries of Dropbox: Property-Based Testing of a Distributed Synchronization Service
- Hughes, Pierce, Arts, Norell · ICST 2016
- https://www.cis.upenn.edu/~bcpierce/papers/mysteriesofdropbox.pdf [F·prior]
- Summary: A QuickCheck model of Dropbox semantics with conflict files. It found real anomalies in Dropbox, including lost files after deletes and conflicts.
- Relevance: The test method for F1–F5. A property-based model of the folder replica is the expected evaluation.

### Testing Sync at Dropbox (Nucleus: CanopyCheck, Trinity)
- Dropbox engineering blog · 2020
- https://dropbox.tech/infrastructure/-testing-our-new-sync-engine [F·prior]; also https://dropbox.tech/infrastructure/rewriting-the-heart-of-our-sync-engine [F·prior]
- Summary: Nucleus models Remote, Local and Synced trees, with the Synced tree as the merge base. CanopyCheck randomizes the planner over trees, and Trinity runs deterministic, seeded concurrency fuzzing of the whole engine.
- Relevance: The industry baseline that C4 and its evaluation must match. The Synced tree threatens any merge-base novelty claim.

---

## 5. Bidirectional transformations and lenses (files and DBs)

### Exploiting Schemas in Data Synchronization (Harmony)
- Foster, Greenwald, Kirkegaard, Pierce, Schmitt · DBPL 2005; JCSS 73(4) 2007
- https://www.cis.upenn.edu/~bcpierce/papers/harmony_bib.html [F] (bibliography page; the paper PDF was not opened)
- Summary: Pairs lenses, which map heterogeneous formats to abstract trees, with a schema-aware synchronizer that never overwrites conflicting data.
- Relevance: The nearest ancestor of C4. F3 comes from here, and C4's delta is a relational replica plus multi-device sync.

### Relational Lenses: A Language for Updatable Views
- Bohannon, Pierce, Vaughan · PODS 2006
- https://doi.org/10.1145/1142351.1142399 [M]
- Summary: Lenses over relations with select, project and join, typed by functional dependencies, with the well-behavedness laws GetPut and PutGet.
- Relevance: F1/F2 are these laws. Cite them, do not claim them.

### Boomerang: Resourceful Lenses for String Data
- Bohannon, Foster, Pierce, Pilkiewicz, Schmitt · POPL 2008
- https://doi.org/10.1145/1328438.1328487 [F]; project page https://www.seas.upenn.edu/~harmony/ [F]
- Summary: Dictionary and resourceful lenses for ordered text formats, where keys align chunks so that put preserves unseen formatting.
- Relevance: Prior art for structured file codecs and stable per-line IDs in C4. F1/F2 hold for Boomerang lenses.

### Incremental Relational Lenses
- Horn, Perera, Cheney · ICFP 2018 · PACMPL 2(ICFP):74
- https://arxiv.org/abs/1807.01948v2 [F]
- Summary: Gives relational lenses a change-propagating semantics, so that small view deltas map to small table deltas, with orders-of-magnitude speedups in Links.
- Relevance: Supports C4. File edits should become row deltas, not whole-table rewrites. This is the delta-lens basis for per-field merge.

### TxForest: A DSL for Concurrent Filestores
- DiLorenzo, Mancini, Fisher, Foster · APLAS 2019
- https://ar5iv.arxiv.org/html/1908.10273 [F·prior]
- Summary: Type-directed load and store between on-disk filestores and in-memory data, with serializable optimistic transactions and round-trip laws. Crashes are not covered.
- Relevance: Threatens C4's "transactional folder" framing. F4 (crash recovery) is the delta TxForest leaves open.

### Dejima / BISCUITS: Bidirectional Data Sharing among Peers
- Ishihara, Kato, Nakano, Hu et al. · BX 2018 / arXiv 2112.00288
- https://arxiv.org/pdf/2112.00288 [F·prior]
- Summary: Peers share "Dejima tables" defined by BX over their base tables, so that each peer controls what it provides and accepts. DB to DB only.
- Relevance: Prior art for multi-peer BX (C4). It has no files and no blobs.

### BIRDS: Programming View Update Strategies in Datalog
- Tran Van Dang, Ko, Hu et al. · VLDB 2020 demo / arXiv 1809.10357
- https://arxiv.org/pdf/1809.10357 [F·prior]
- Summary: Lets users write putback strategies for relational views in Datalog and verifies their well-behavedness automatically.
- Relevance: A verification tool for F1/F2 if codecs are expressed relationally (C4).

### Bidirectionalization for the Common People (Bifrons)
- Dončević, Brčić, Mlinarić · arXiv 2502.18954 · Feb 2025 (preprint)
- https://arxiv.org/abs/2502.18954 [F]
- Summary: A C# library of simple symmetric lenses with a lens-testing framework, demonstrated by syncing two heterogeneous databases.
- Relevance: The only 2024–2026 BX-for-sync paper found. It is weak prior art for C4 (DB to DB, no files, no multi-replica), and it shows the field still lacks file to DB sync.

---

## 6. Merkle structures and set reconciliation

### Range-Based Set Reconciliation and Authenticated Set Representations
- Aljoscha Meyer · arXiv 2212.13567 · 2022/23
- https://arxiv.org/abs/2212.13567v1 [F·prior]
- Summary: Recursively partitions a sorted key space and compares range fingerprints, and analyses which fingerprint schemes are secure and incremental.
- Relevance: An anti-entropy option for blob and claim sets (C2/S5 reconciliation). Prior art if the thesis does digest-based catch-up.

### Merkle Search Trees: Efficient State-Based CRDTs in Open Networks
- Auvolat, Taïani · SRDS 2019
- https://inria.hal.science/hal-02303490 [F·prior]
- Summary: The hash-derived key level gives a deterministic tree shape, so the tree is itself a state-based CRDT that peers reconcile by recursive hash comparison. AT Protocol uses it.
- Relevance: Prior art for verifiable state digests (S6 fork detection, C6).

### Merkle-CRDTs: Merkle-DAGs Meet CRDTs
- Sanjuán, Pöyhtäri, Teixeira, Psaras · arXiv 2004.00107 · 2020
- https://arxiv.org/abs/2004.00107 [F·prior]
- Summary: A Merkle-DAG serves as transport and persistence for CRDTs. The Merkle-Clock is itself a state-based CRDT and gives per-object causality without a trusted server.
- Relevance: An alternative to C1's server log. The hash-chained log needed for S6 under E2EE is close to this.

### The Essence of Git: Concepts for P2P Replication
- Erick Lavoie · arXiv 2609.13555 · Sept 2026
- https://arxiv.org/abs/2609.13555 [F·prior]
- Summary: Argues that Git's object model and incremental replication protocols generalize beyond version control, and ties commit history to convergent replicated structures.
- Relevance: Recent positioning for content-addressed history (C2, C6 hash-chained log).

---

## 7. Testing: deterministic simulation, Jepsen and Elle, model checking

### FoundationDB: A Distributed Unbundled Transactional Key Value Store
- Zhou, Xu, Shraer et al. · SIGMOD 2021 · DOI 10.1145/3448016.3457559
- https://doi.org/10.1145/3448016.3457559 [F]
- Summary: Strictly serializable unbundled KV. A deterministic, randomized single-process simulator injects disk, process and network faults, which the authors credit for stability and release speed.
- Relevance: The canonical citation for S6 and the plan to test C1–C3 with deterministic simulation.

### Elle: Inferring Isolation Anomalies from Experimental Observations
- Kingsbury, Alvaro · PVLDB 14(3) · 2020/21
- https://arxiv.org/abs/2003.10554 [F]
- Summary: Infers Adya dependency graphs from client-observed histories, using list-append and register workloads, and detects every non-predicate Adya anomaly with explanations.
- Relevance: The checker for C1's log semantics (no skipped cursor, prefix consistency) and for S4.

### Jepsen: PostgreSQL 12.3
- Kingsbury · jepsen.io · 2020
- https://jepsen.io/analyses/postgresql-12.3 [F·prior]
- Summary: Elle found G2-item anomalies under SERIALIZABLE in fault-free runs, caused by a real SSI bug.
- Relevance: Warns C1 not to trust Postgres SSI alone to sequence the sync log. It argues for an explicit sequencer and Elle checks.

### Jepsen: Bufstream 0.1.0
- Kingsbury · jepsen.io · 2024
- https://jepsen.io/analyses/bufstream-0.1.0 [F·prior]
- Summary: A 2024 analysis of a Kafka-compatible log on object storage, which found consistency issues in a log built over S3-class stores.
- Relevance: Shows the failure modes when logs and blobs share an object store (C2 fence, S1).

### Plume: Efficient and Complete Black-Box Checking of Weak Isolation Levels
- Liu, Gu, Wei, Basin · OOPSLA 2024 · PACMPL 8(OOPSLA2):302
- https://hengxin.github.io/papers/2024-OOPSLA-Plume.pdf [F]
- Summary: Sound and complete characterizations of RC, read atomicity and **transactional causal consistency**, checked fast with vector clocks and tree clocks.
- Relevance: The checker for C1 client-side TCC claims and for S4 histories. It is more complete than Elle at weak levels.

### IsoVista: Black-Box Checking Database Isolation Guarantees
- Gu, Liu, Xing, Wei, Chen, Basin · PVLDB 17(12) · 2024
- https://mail.vldb.org/pvldb/volumes/17/paper/IsoVista%3A%20Black-box%20Checking%20Database%20Isolation%20Guarantees [F]
- Summary: An end-to-end isolation-checking system with anomaly visualisation and checker benchmarking.
- Relevance: Ready-made tooling for the C1 evaluation.

### ER-π: Exhaustive Interleaving Replay for Testing Replicated Data Library Integration
- Mondal, Tilevich · Middleware 2025 · DOI 10.1145/3721462.3730947
- https://people.cs.vt.edu/tilevich/papers/middleware2025.pdf [F]
- Summary: Replays event interleavings for apps that use replicated data libraries, with four pruning algorithms and checkpoint/reset between runs. It reproduced 12 known bugs and found 5 common misconceptions, across Go, Java and JS libraries.
- Relevance: The closest 2025 work on **testing apps built on sync libraries**. A must-cite for the S1–S6 evaluation method, and a baseline against the thesis's simulator.

### SandTable: Scalable Distributed System Model Checking with Specification-Level State Exploration
- Tang, Sun, Huang, Wei, Ouyang, Ma · EuroSys 2024
- https://marshtompsxd.github.io/pub/sandtable.pdf [F]
- Summary: Model-checks a TLA+ spec, then drives the real system through the explored states by interposing at the syscall level (conformance checking). It cannot see thread interleavings.
- Relevance: A way to tie a TLA+ model of the S1–S6 protocol to the implementation.

### Smart Casual Verification of the Confidential Consortium Framework
- Howard, Kuppe, Ashton, Chamayou, Crooks · NSDI 2025
- https://www.usenix.org/conference/nsdi25/presentation/howard [F]; https://arxiv.org/abs/2406.17455v2 [F]
- Summary: Combines a TLA+ spec with trace validation of production C++ test traces, run in CI. It found 6 bugs before production.
- Relevance: The most practical template for checking C6/S6. Validate client and server traces against a spec in CI.

### Systems Correctness Practices at Amazon Web Services (P, PObserve)
- Brooker, Desai · CACM / ACM Queue · 2025
- https://cacm.acm.org/practice/systems-correctness-practices-at-amazon-web-services [F]; P language https://p-org.github.io/P/ [F]
- Summary: Surveys TLA+, P state-machine models, PObserve (post-hoc log validation against P specs), property-based and fault-injection testing, and simulation at AWS.
- Relevance: Industry backing for a P or TLA+ model of the lease, claim and fence protocol (C2, S5).

### Quint
- Informal Systems · tool, ongoing
- https://github.com/informalsystems/quint [F]
- Summary: An executable specification language on TLA, with a simulator, Apalache model checking, and Quint Connect model-based testing for Rust.
- Relevance: A low-friction option for specifying S1–S6 and driving trace tests against a Rust core. No published sync-protocol specification was found.

### TigerBeetle VOPR
- TigerBeetle · docs, ongoing
- https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/HACKING.md [F] (the run command differs between versions: `simulator_run` vs `vopr`)
- Summary: A deterministic simulator that fuzzes many clusters and clients in one process, with packet and storage fault simulators and an online state checker.
- Relevance: An engineering pattern for C3/S4 crash and partition testing.

### Antithesis deterministic simulation testing
- Antithesis · docs, ongoing
- https://antithesis.com/docs/resources/deterministic_simulation_testing/ [F]
- Summary: Deterministic simulation at the hypervisor level for unmodified binaries, a commercial alternative to building an FDB-style simulator.
- Relevance: An option for testing the real server (Postgres, object store) for S1/S5 without rewriting it for simulation.

### Aurora DSQL: Scalable, Multi-Region OLTP
- AWS authors · arXiv 2607.13276 · 2026
- https://arxiv.org/pdf/2607.13276 [F] (search snippet only; the paper was not opened and the author list was not checked)
- Summary: Reports build-time deterministic simulation testing, credited to FoundationDB, which validated well against testing in production.
- Relevance: 2026 evidence that DST is standard practice for transactional systems, supporting the S6 method.

---

## Coverage gaps

- **No 2024–2026 DB-venue paper on "sync engines"** for relational client-server sync (Replicache, Zero, PowerSync style) turned up in VLDB, SIGMOD or CIDR. Searches returned only blogs and vendor docs. This supports C1/C8 as an under-studied area, but the search was shallow, with no DBLP sweep.
- **Blob and metadata atomicity for offline clients**: no paper found. Delta Lake and Iceberg cover the server-side log over immutable objects. Nothing covers pending references from offline replicas (C3a/S5).
- **Lease-based GC with offline replicas**: none found beyond Gray & Cheriton and causal-stability GC. The S5 framing appears untouched in the literature, though this was not searched exhaustively.
- **File synchronizer verification 2024–2026**: none found. The latest items are VMCAI 2018 and Syncpal 2019. F4/F5 have no recent formal treatment.
- **BX for files ↔ DB, 2024–2026**: only the Bifrons preprint (DB to DB) turned up. The BX workshop proceedings for 2024–2026 were not scanned.
- **Not opened this session [M]**: Indigo, CALM CACM, AQL, CRR (Yu & Ignat), relational lenses PODS 2006, Gray & Cheriton, pure op-based CRDTs. The Harmony PDF, the Delta Lake vacuum retention detail, and the PaPoC 2025 property-graph paper are also unverified.
- **Not searched**: OPODIS and DISC 2024–2026, SoCC and ATC 2024–2026 programs, the full PaPoC 2024 and 2026 programs, "SQLite replication" in academic venues (only products found: SQLite Sync, cr-sqlite, Turso), and Jepsen analyses of sync products (none known).
