# Academic State of the Art: Local-First Software, CRDTs, and Replicated Data Sync (2019 to Nov 2026)

Notes compiled 2026-11-04. Legend: items marked **[V]** were checked against the source this session. Items marked **[K]** come from well-established literature known before this session. Their URLs are canonical but were not re-fetched, so verify exact page or figures before citing in a thesis.

## Foundations: local-first ideals, CRDT taxonomy, SEC and causal consistency

### Takeaway
The field rests on three foundations. The Ink & Switch local-first ideals (2019) set the goals. Shapiro et al.'s CRDT model (2011) gives strong eventual consistency (SEC): state-based, op-based and delta-state variants. CALM explains when coordination-free execution is enough. In 2025 a formal result showed that the state-based and op-based variants are observationally equivalent. A sync engine can therefore define its semantics abstractly and choose the wire protocol separately.

### Cited Findings
- **Local-first software (Kleppmann, Wiggins, van Hardenberg, McGranaghan; Onward! 2019).** Seven ideals: no spinners, multi-device, offline, collaboration, longevity, privacy, user control. Proposes CRDTs as the enabling technology and lists open issues in performance, history size, schema change and access control. **[K]** — [Ink & Switch essay](https://www.inkandswitch.com/local-first/)
- **Ink & Switch's long-term agenda:** local-first enables "universal version control" (branches, diffs, review, revert), which in turn enables malleable software. The lab's 2025 "malleable software" essay (June 2025) builds on its Patchwork system, which stores everything as Automerge documents. **[V]** — [Simon Willison summary](https://simonwillison.net/tags/ink-and-switch/); [Patchwork notebook: universal version control](https://inkandswitch.com/patchwork/notebook/01); [2024 version control notes](https://www.inkandswitch.com/patchwork/notebook/2024-version-control/)
- **Upwelling (Mar 2023, Ink & Switch).** Combines real-time collaboration with version control (drafts and layers) for writers. It is evidence that "always-merged real-time" is not always the UX users want. **[V title/date]** — [Kleppmann publications](https://martin.kleppmann.com/)
- **Shapiro, Preguiça, Baquero, Zawirski, "Conflict-free Replicated Data Types" (SSS 2011)** plus the INRIA tech report "A comprehensive study of CRDTs" (2011). Define SEC (eventual delivery + convergence: same set of updates means same state), state-based CvRDTs (a join-semilattice with merge = LUB) and op-based CmRDTs (commutative ops with causal delivery). **[K]** — [HAL inria-00609399](https://hal.inria.fr/inria-00609399)
- **Delta-state CRDTs (Almeida, Shoker, Baquero; JPDC 2018, arXiv 2016).** Ship join-decomposable deltas rather than full state. Causal delta-merging restores causal consistency over unreliable channels. **[K]** — [arXiv:1603.01529](https://arxiv.org/abs/1603.01529)
- **CRDT emulation and representation independence (Liittschwager, Castello, Tsampas, Kuper; ICFP 2025 / PACMPL 9).** First precise definition of state/op-based "emulation", as weak simulation between transition systems. Characterises which properties emulation preserves and proves that clients cannot distinguish a state-based from an op-based implementation. **[V]** — [arXiv:2504.05398](https://arxiv.org/abs/2504.05398)
- **CALM theorem (Hellerstein & Alvaro, CACM 2020).** A program has a consistent, coordination-free distributed implementation iff it is monotonic. **[K]** — [arXiv:1901.01930](https://arxiv.org/abs/1901.01930)
- **"Keep CALM and CRDT On" (Laddad, Power, Milano, Cheung, Crooks, Hellerstein; PVLDB 16(4), 2022/2023).** CRDTs guarantee convergence of updates but leave reads unconstrained: a non-monotone read on an intermediate state can be inconsistent. Proposes a CALM-based query model to identify which reads are coordination-free. **[V]** — [arXiv:2210.12605](https://arxiv.org/abs/2210.12605)
- **Research for Practice: Convergence (Kleppmann & Alvaro, CACM Nov 2022).** Curated overview of the convergence literature. **[V title]** — [Kleppmann publications](https://martin.kleppmann.com/)
- **Causal delivery remains an active research topic.** PaPoC 2026 included "Can you keep a secret? A new protocol for sender-side enforcement of causal message delivery" (Tong, Liittschwager, Kuper, UCSC). **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)

### Inferences
- A thesis sync engine can specify semantics as a convergent abstract data type (SEC + causal consistency) and treat state, op or delta shipping as an implementation detail. The ICFP 2025 result justifies this formally.
- "Keep CALM" points to an under-served design dimension: the semantics of reads and queries over CRDT state, not just merges.

### Gaps
- I did not re-fetch the Onward! 2019 paper or Shapiro 2011 this session. Their exact wording is from prior knowledge.
- I found no formal "five years later" retrospective of the local-first paper by its authors. Ink & Switch's evolution is visible only through essays, notebooks and talks.

## Recent CRDT algorithms and systems (text, rich text, trees, JSON, frameworks, performance)

### Takeaway
Text CRDTs are effectively solved for performance:
- **Eg-walker (EuroSys 2025)** keeps only an event graph and builds CRDT state transiently during merges. It gets OT-like steady-state memory with CRDT-like merging of long-diverged branches.
- **Fugue (TPDS 2025)** fixes interleaving anomalies.
- **Automerge 3 (2025)** cut memory by more than 10x by running on its compressed columnar format.

Rich text (Peritext), moves in trees and JSON (Kleppmann et al.), and undo remain the semantic frontier.

### Cited Findings
**Text / sequence CRDTs**
- **Eg-walker, "Collaborative Text Editing with Eg-walker: Better, Faster, Smaller" (Gentle & Kleppmann; EuroSys 2025, pp. 311–328, DOI 10.1145/3689031.3696076; arXiv Sept 2024; Best Artifact Award).** Hybrid CRDT/OT that stores an event graph. It merges long-diverged branches much faster than OT, uses less memory than existing CRDTs, and needs no central server. Code: github.com/josephg/egwalker-paper. **[V]** — [arXiv:2409.14252](https://arxiv.org/abs/2409.14252); [talk slides](https://speakerdeck.com/ept/collaborative-text-editing-with-eg-walker-better-faster-smaller)
- **Fugue, "The Art of the Fugue: Minimizing Interleaving in Collaborative Text Editing" (Weidner & Kleppmann; arXiv May 2023, IEEE TPDS Nov 2025).** Interleaving of concurrent insertions "has gone unnoticed for decades" and affects both CRDTs and OT. Fugue achieves "maximal non-interleaving". **[V]** — [arXiv:2305.00583](https://arxiv.org/pdf/2305.00583.pdf); [Kleppmann blog, 2025-11-01](https://martin.kleppmann.com/2025/11/01/fugue-minimizing-interleaving.html)
- **Weidner, "Collaborative Text Editing without CRDTs or OT" (blog, May 2025; Local-First Conf 2025 talk).** With a central server, give each character a UUID and use "server reconciliation" (as in Replicache, Zero and FPS games). He argues that any non-server reconciliation is itself a CRDT, and that rebasing branches yields Fugue. No independent evaluation exists. **[V]** — [HN discussion](https://hn-a7xu6ee6i-svelte.vercel.app/item/44053560); [slides](https://docs.google.com/presentation/d/1WHUKnht-RxZ9lrMLBMwrs8BpW7ifN7y6iillhju2rKw/htmlpresent)
- **Yjs / YATA (Nicolaescu, Jahns, Derntl, Klamma; GROUP 2016).** Sequence CRDT with left/right origin integration. It is the most widely deployed web CRDT. An industry counterpoint is the "Lies I was told about collaborative editing pt. 2: why we don't use Yjs" post (Moment.dev). **[K for YATA paper; V for Moment post existence]** — [Moment devlog](https://hn.jakechampion.name/reader/https://www.moment.dev/blog/lies-i-was-told-pt-2)

**Rich text, trees, JSON, undo**
- **Peritext (Litt, Lim, Kleppmann, van Hardenberg; PACMHCI / CSCW2, Nov 2022).** CRDT for rich-text formatting spans that preserves user intent at mark boundaries. **[V title/venue]** — [Kleppmann publications](https://martin.kleppmann.com/); [Ink & Switch Peritext essay](https://www.inkandswitch.com/peritext/) [K]
- **Move in replicated trees (Kleppmann, Mulligan, Gomes, Beresford; IEEE TPDS 2021).** Highly available move with undo-do-redo over a timestamp-ordered log. Prevents cycles and was verified in Isabelle/HOL. **[K]** — [paper PDF](https://martin.kleppmann.com/papers/move-op.pdf)
- **Extending JSON CRDTs with Move Operations (Da & Kleppmann; PaPoC 2024).** **[V]** — [Kleppmann publications](https://martin.kleppmann.com/)
- **Undo and Redo Support for Replicated Registers (Stewen & Kleppmann; PaPoC 2024).** **[V]** — [Kleppmann publications](https://martin.kleppmann.com/)
- **AegisSheet: A Compositional CRDT for Collaborative Spreadsheets (Pfeil, Scandurra, Haas, TU Darmstadt; PaPoC 2026).** **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)
- **Semantic Conflict Model for Collaborative Data Structures (Semenov & Aksenov, ITMO; PaPoC 2026).** Models conflicts as first-class, not just merges. **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)

**Frameworks / libraries**
- **Collabs (Weidner, Miller, et al.; arXiv Dec 2022, EuroSys 2024).** Library for composing custom CRDTs, with a performance evaluation. **[V arXiv; K venue]** — [arXiv:2212.02618](https://arxiv.org/pdf/2212.02618)
- **Automerge 3.0 (2025).** Memory use cut "by over 10x" and sometimes far more. The runtime now operates directly on the compressed columnar format: pasting Moby Dick takes 700 MB in Automerge 2 and 1.3 MB in Automerge 3. Load is also faster, the Text class was removed, and the release is almost backwards compatible. The Ink & Switch beta note claimed up to 100x, so the sources give different figures. **[V]** — [Automerge 3.0 blog](https://automerge.org/blog/automerge-3/); [Ink & Switch Dispatch 011](https://inkandswitch.com/newsletter/dispatch-011)
- **Automerge, July 2026.** v3.3.0–3.3.2 shipped. "Hexane v1", a new columnar storage engine, is reported 2–9x faster. automerge-repo v2.6 alphas harden long-running sync servers. A document anonymizer was added for perf bug reports. **[V]** — [This Month in Automerge: July '26](https://automerge.org/blog/2026-july/)
- **Loro / diamond-types.** Loro is a Rust CRDT library that adopts Fugue and Eg-walker-style ideas. diamond-types is Gentle's Rust reference implementation of the Eg-walker lineage. A July 2026 vendor comparison says Loro and Automerge v3 reduced memory and added version-history APIs. That is secondary and vendor-written, so low weight. **[V for vendor claim only]** — [Velt, July 2026](https://velt.dev/blog/top-crdt-libraries-for-real-time-data-sync)

### Inferences
- The performance problem that blocked CRDT adoption in 2019 (memory, load time) is largely addressed for text. The remaining CRDT problems are semantic (moves, rich text, spreadsheets, undo, conflict surfacing) and systemic (sync protocol, auth, storage).
- Eg-walker's design (keep a plain event log and derive CRDT state only when needed) suits a general sync engine: the persistent format is an event DAG, and the per-type merge is pluggable.
- Weidner's server-reconciliation argument suggests a sync engine could offer two modes: server-authoritative rebase (simpler, needs a server) and CRDT (peer-to-peer). A unified semantics spanning both would be a solid thesis contribution.

### Gaps
- I did not fetch Eg-walker's exact benchmark numbers (ms/MB per trace). Pull them from Section 4 of arXiv:2409.14252.
- I found no peer-reviewed head-to-head benchmark of Loro vs Automerge 3 vs Yjs vs diamond-types published in 2025–2026. The crdt-benchmarks repos are informal.
- I could not confirm Collabs' EuroSys venue this session.

## Invariants, mixed consistency, synthesis and verification

### Takeaway
CRDTs alone cannot preserve global invariants such as uniqueness or non-negative balances. The literature splits into two answers:
- **Analysis:** prove which ops are safe without coordination (I-confluence, CALM, Indigo, Hamsaz, ECROs, LoRe/ConLoc) and coordinate only the rest (RedBlue).
- **Verification and synthesis:** prove CRDTs correct (Isabelle, VeriFx, Sal) or derive them from sequential specs (Katara).

Push-button verification of replicated data types was a PaPoC 2026 keynote theme.

### Cited Findings
- **RedBlue consistency (Li et al., OSDI 2012).** Blue ops commute and are fast; red ops are totally ordered. **[K]** — [USENIX OSDI'12](https://www.usenix.org/conference/osdi12/technical-sessions/presentation/li)
- **Invariant confluence / I-confluence (Bailis et al., PVLDB 2015).** Gives the necessary and sufficient condition for coordination-free invariant preservation. **[K]** — [arXiv:1402.2237](https://arxiv.org/abs/1402.2237)
- **Indigo (Balegas et al., EuroSys 2015)** uses explicit consistency with reservations and escrow. **Hamsaz (Houshmand & Lesani, POPL 2019)** synthesises replicated objects that preserve invariants by analysing conflict and dependency relations. **[K]** — [Hamsaz POPL'19](https://dl.acm.org/doi/10.1145/3290387)
- **ECROs (De Porre, Ferreira, Preguiça, Gonzalez Boix; OOPSLA 2021).** Extended CRDTs derived from sequential code plus invariants. Ops are ordered only when they conflict. **[K]** — [ACM DL](https://dl.acm.org/doi/10.1145/3485484)
- **Katara: Synthesizing CRDTs with Verified Lifting (Laddad, Power, Milano, Cheung, Hellerstein; OOPSLA 2022; preprint Jan 2022).** The preprint is confirmed via OSTI; the OOPSLA venue is from prior knowledge. **[V preprint / K venue]** — [OSTI record](https://www.osti.gov/biblio/2420482); [arXiv:2205.12425](https://arxiv.org/abs/2205.12425) [K]
- **LoRe (Haas, Mogk, Yanakieva, Bieniusa, Mezini; ECOOP 2023, later TOPLAS).** Programming model and compiler that automatically verifies developer-supplied safety properties for local-first apps. **[V]** — [arXiv:2304.07133](https://arxiv.org/abs/2304.07133)
- **ConLoc, "Consistent Local-First Software: Enforcing Safety and Invariants for Local-First Applications" (IEEE TSE, published ~Oct 2024 / Jan 2025; FSE 2025 journal-first).** A Java annotation language that automatically enforces invariants in local-first apps. It reports substantial latency and throughput gains over sequential execution with the same invariants. Maintained by Mirko Köhler (consysT project). **[V]** — [ACM DL / IEEE TSE](https://dl.acm.org/doi/10.1109/TSE.2024.3477723); [FSE 2025 listing](https://conf.researchr.org/details/fse-2025/fse-2025-journal-first/48/Consistent-Local-First-Software-Enforcing-Safety-and-Invariants-for-Local-First-Appl); [code](https://gh.r-mj.com/consysT-project/conloc)
- **Behavioural Types for Local-First Software (ECOOP 2023).** **[V existence]** — [arXiv:2305.04848](https://arxiv.org/pdf/2305.04848)
- **Verified CRDTs.**
  - Gomes, Kleppmann, Mulligan, Beresford, "Verifying strong eventual consistency in distributed systems" (OOPSLA 2017, Isabelle/HOL). **[K]** — [arXiv:1707.01747](https://arxiv.org/abs/1707.01747)
  - VeriFx (De Porre et al., ECOOP 2023). **[K]** — [arXiv:2207.02502](https://arxiv.org/abs/2207.02502)
  - Sal: Multi-modal Verification of Replicated Data Types (Ramesh, Soundarapandian, Sivaramakrishnan; PaPoC 2026, arXiv Mar 2026). KC Sivaramakrishnan's PaPoC 2026 keynote was "From Convergence to Confidence: Push-Button Verification for Replicated Data Types". **[V]** — [arXiv:2603.27202](https://arxiv.org/pdf/2603.27202); [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)
- **Hydro / Hydroflow (Hellerstein group, Berkeley).** Lattice-based dataflow language and compiler stack for coordination-free distributed programs. It is the systems vehicle for the CALM and Katara line. **[K]** — [hydro.run](https://hydro.run)
- **Towards Distributed Constraint Solving with CRDT (Hasan & Talbot; PaPoC 2026).** **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)

### Inferences
- For an enterprise sync engine, an **invariant-aware mixed-consistency layer** is the research-grade differentiator. Most ops run as CRDT/offline, and a small classified set (uniqueness, quotas, payments) escalates to server coordination, chosen by static I-confluence analysis. Industry engines (Replicache/Zero-style server authority, ElectricSQL-style read-path sync) do not do this classification. This is inference; I found no product doing it.
- Verus/Rust and Sal-style push-button verification make it realistic for an undergrad thesis to verify one engine component, such as the merge function or the authorization CRDT.

### Gaps
- No 2025–2026 empirical study measures how often real local-first apps violate invariants.
- I did not verify the exact page numbers or DOIs for Indigo, ECROs or VeriFx this session.

## Access control, BFT, end-to-end encryption, schema evolution, partial replication

### Takeaway
2024–2026 is the "local-first authorization" era:
- **Ink & Switch's Keyhive:** capabilities plus a delegation graph and BeeKEM group key agreement, with Beelay, now Subduction/Sedimentree, as the encrypted sync protocol.
- **Matrix-derived and formal-verification work** at KIT.
- **BFT CRDTs:** from Kleppmann (2022) to Baquero et al. (2026).

Schema evolution (Cambria lenses, Baseline 2025) and partial replication remain the least-solved areas.

### Cited Findings
**Access control and E2EE**
- **Keyhive (Ink & Switch; formerly "Beehive"/"Authomerge"; Zelenka, Good, with Kleppmann).** Local-first access control in which "access control must travel with the data." It uses capabilities and delegation with a membership/delegation graph; groups can contain groups and documents act as folders. The team argues that cloud and blockchain approaches need connectivity to validate updates, so they are not local-first. Rust workspace. **[V]** — [Keyhive notebook 00](https://www.inkandswitch.com/keyhive/notebook/00/); [notebook 01](https://www.inkandswitch.com/keyhive/notebook/01/); [notebook 05](https://www.inkandswitch.com/keyhive/notebook/05/)
- **Keyhive status, July 2026.** Still WIP. Designed for any hash graph, not only Automerge. E2EE in automerge-repo is in progress, and author provenance is being added to support revocation. **[V]** — [Automerge July '26](https://automerge.org/blog/2026-july/)
- **Beelay.** Described as a "(reasonably) generic encrypted sync protocol for CRDTs" (Local-First Conf 2025 talk). It authenticates peers and syncs the membership graph first, then the documents a peer may access. **[V]** — [LFC 2025 talk page](https://localfirstconf-2025-release.vercel.app/schedule/talks-day-1/beelay); [localfirst.fm ep. 19](https://localfirst-fm.vercel.app/19/transcript)
- **Subduction + Sedimentree (Ink & Switch, 2025–2026).** P2P sync protocol for encrypted, partitioned CRDT data without a central server. Sedimentree shards history for diffing and incremental sync. Batch reconciliation takes about 1.5 round trips using 8-byte SipHash fingerprints. Transports: WebSocket, HTTP long-poll, Iroh/QUIC. CRDT-agnostic. Status is "early release preview, not for production". By July 2026 the server used a redb backend, per-peer rate limiting and iroh 1.0. Subduction appears to be the successor to Beelay, but that is inferred from naming and timing, and no source states it. **[V]** — [GitHub inkandswitch/subduction](https://github.com/inkandswitch/subduction); [docs.rs subduction_core](https://docs.rs/crate/subduction_core/0.9.0/source/README.md); [Automerge July '26](https://automerge.org/blog/2026-july/)
- **BeeKEM: Decentralized, Secure and Efficient Group Key Agreement (Yen, Fábrega, Da, Mumm, Park, Zelenka, Kleppmann; IACR ePrint 2026/1434, July 2026).** A concurrent TreeKEM variant without a central server, used by Keyhive. **[V]** — [Kleppmann publications](https://martin.kleppmann.com/); [Automerge July '26](https://automerge.org/blog/2026-july/)
- **Kintsugi: Decentralized E2EE Key Recovery (Ma & Kleppmann; Security Protocols Workshop, Mar 2025; Best Presentation).** **[V]** — [Kleppmann publications](https://martin.kleppmann.com/)
- **DCGKA, "Key Agreement for Decentralized Secure Group Messaging with Strong Security Guarantees" (Weidner, Kleppmann, Hugenroth, Beresford; ACM CCS 2021).** **[K]** — [IACR ePrint 2020/1281](https://eprint.iacr.org/2020/1281)
- **Towards System-Oriented Formal Verification of Local-First Access Control (Jacob, Stuber, Hartenstein, KIT; arXiv Apr 26 2026; PaPoC 2026 lightning talk).**
  - Notes that Matrix and Keyhive both rely on informal specs and unverified reference implementations.
  - Contributes a group-management RDT with invariants, capability-based access control over "hash chronicles", and Rust+Verus verification with zero runtime cost.
  - Scaling to Matrix or Keyhive is future work.
  - **[V]** — [arXiv:2604.23560](https://arxiv.org/abs/2604.23560)
- **ERA: Epoch-Resolved Arbitration for Duelling Admins in Group Management CRDTs (Kegan Dougal, Element/Matrix; PaPoC 2026).** Addresses the concurrent mutual-revocation problem. **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)
- **did:crdt: Coordination-Free Decentralised Identifiers via Signed CRDTs (arXiv:2606.16223, June 2026).** Separates authorisation from ordering by signing each mutation, so no consensus layer is needed. Rust implementation with iroh delta gossip. **[V]** — [arXiv:2606.16223](https://arxiv.org/html/2606.16223v1)

**Byzantine fault tolerance**
- **Making CRDTs Byzantine Fault Tolerant (Kleppmann; PaPoC 2022).** Hash-DAG of ops gives BFT eventual consistency with any number of Byzantine nodes, under Sybil-tolerant assumptions. **[V title/venue]** — [Kleppmann publications](https://martin.kleppmann.com/); [PDF](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf) [K]
- **Bounding Byzantine Impact in Open CRDT Systems (Baquero, Maia, Dantas, Fernández Anta, Frey, Sánchez, Albouy; PaPoC 2026).** **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)

**Sync protocols / bandwidth**
- **ConflictSync: Bandwidth Efficient Synchronization of Divergent State (Baquero, Gomes, Rodrigues; PaPoC 2026).** **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)
- **CobbleDB: Modelling Levelled Storage by Composition (Ma, Pandey, Bieniusa, Shapiro; PaPoC 2026).** **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)

**Schema evolution**
- **Cambria (Litt, van Hardenberg, Henry; Ink & Switch 2020).** Bidirectional lenses that translate data between schema versions, so old and new clients can co-edit. **[K]** — [Ink & Switch Cambria](https://www.inkandswitch.com/cambria/)
- **Schema Evolution in Interactive Programming Systems (arXiv Dec 2024).** The hard part in local-first P2P apps is deploying a schema change across replicas while migrating in-flight operations so all replicas converge without data loss, ideally without central coordination. **[V summary]** — [arXiv:2412.06269](https://arxiv.org/pdf/2412.06269)
- **Baseline: Operation-Based Evolution and Versioning of Data (Edwards & Petricek; arXiv Dec 10 2025, v2 Jun 17 2026; submitted to <Programming>).** "Operational Differencing" treats data as high-level ops, including refactorings and schema changes. Diff and merge work across schema changes, branching is copying, and there is no repository. Queries are conjectured to be expressible as op sequences and run speculatively on branches. **[V]** — [arXiv:2512.09762](https://arxiv.org/abs/2512.09762)

**Reactive relational local-first**
- **Riffle (Litt, Schiefer, Schickling, Jackson; UIST 2023).** Reactive relational state for local-first apps: SQLite in the client plus incrementally maintained queries. The lineage leads to LiveStore (Schickling), which is event-sourced. **[K]** — [riffle.systems](https://riffle.systems/essays/prelude/)

**Other PaPoC 2026 context**
- Ken Birman et al., "Data Consistency Challenges in AI Applications". Guerraoui keynote "Genuine State Machine Replication". **[V]** — [PaPoC 2026 program](https://papoc-workshop.github.io/2026/program.html)

### Inferences
- Authorization is now the hottest local-first research front. Open sub-problems include concurrent revocation ("duelling admins"), revocation under E2EE, provenance, and verified implementations. Keyhive is the reference design but is still unverified and not production-ready as of Jul 2026.
- The industry direction is a CRDT-agnostic sync layer (Subduction: hash-DAG + sharded history + fingerprint reconciliation + E2EE) separated from the data model. This matches the user's "sync engine with proper semantics, reusable across projects" goal.

### Gaps
- **Partial replication / partial sync:** I found no 2025–2026 primary paper that solves partial replication of op-based CRDT histories with causal correctness. It shows up as an engineering topic (Sedimentree sharding, read-path "shapes" in industry engines) but is academically thin.
- **Garbage collection of CRDT history** (tombstones, causal stability, pruning under E2EE without coordination): no 2025–2026 primary source found this session.
- I found no peer-reviewed Keyhive/Beelay paper. Only notebooks, talks, the BeeKEM preprint and code exist.
- Cambria's follow-up status (maintained or abandoned) is unverified.

## Open problems researchers list, and thesis-sized attack angles

### Takeaway
Across the 2019 essay, Ink & Switch's 2024–2026 work, PaPoC 2026 and recent arXiv papers, the consistently listed open problems are:
1. Authorization and revocation, decentralized and verified
2. Schema evolution of live replicated data
3. Partial sync for large datasets
4. Invariants and mixed consistency
5. History GC and storage growth
6. Conflict UX and semantics (surfacing conflicts, branches and review instead of silent merge)
7. BFT in open systems
8. Formal verification of real implementations

### Cited Findings
- Local-first access control needs BFT and fine-grained authorization. Current systems (Matrix, Keyhive) rest on informal specs, and verified scaling is future work. — [arXiv:2604.23560](https://arxiv.org/abs/2604.23560)
- Schema change across replicas with in-flight ops and no central coordinator is unsolved. — [arXiv:2412.06269](https://arxiv.org/pdf/2412.06269); see also [Baseline](https://arxiv.org/abs/2512.09762)
- Conflicts as first-class semantic objects, not just auto-merged, is an active topic. — [PaPoC 2026: Semantic Conflict Model](https://papoc-workshop.github.io/2026/program.html); [Upwelling](https://martin.kleppmann.com/)
- Reads over CRDTs need consistency semantics too. — [Keep CALM and CRDT On](https://arxiv.org/abs/2210.12605)
- Sync bandwidth for divergent state and storage layout are open. — [ConflictSync, CobbleDB at PaPoC 2026](https://papoc-workshop.github.io/2026/program.html)
- Large-scale and many-document sync is a practical frontier: Patchwork loads "low thousands of documents at once", which drove the Subduction server work. — [Automerge July '26](https://automerge.org/blog/2026-july/)
- Version-control primitives (branch, diff, review, revert) become essential for collaboration with AI agents. This is Ink & Switch's stated thesis and an opinion. — [talk summary](https://second-brain-nuxt.vercel.app/local-first-the-secret-master-plan); see also [Birman et al., PaPoC 2026](https://papoc-workshop.github.io/2026/program.html)

### Inferences (candidate thesis topics, ranked by feasibility for an undergrad)
1. **Formally specified sync-engine core.** Define the engine as an event-DAG with pluggable CRDT merge (justified by Eg-walker and the ICFP 2025 representation-independence result). Prove convergence for the engine plus 2–3 types with Verus or VeriFx. Feasible and publishable at PaPoC.
2. **Invariant-classified mixed consistency.** Annotate the schema with invariants, run an I-confluence check, and route non-confluent ops to server coordination while the rest go offline/CRDT. Benchmark against an all-server-authority baseline. This builds on ConLoc, LoRe and RedBlue.
3. **Schema migration for replicated op logs.** Cambria-style lenses applied to in-flight ops in the engine's log. Evaluate with Baseline's challenge problems.
4. **Partial sync with causal correctness.** Shape- or query-based subscriptions over a causal DAG. Prove that a partial replica never observes a causally inconsistent state. This is the clearest academic gap found.
5. **Authorization CRDT.** Group membership with concurrent revocation (ERA, Keyhive). Harder; crypto-heavy.

### Gaps
- No single 2025–2026 survey paper ranks open problems. This list is synthesized from many sources.
- Local-First Conf 2026 talk list and content were not retrieved this session.
- No primary academic source was found on sync engines as a database abstraction (such as "sync as a database" at CIDR, VLDB or SIGMOD 2024–2026). Industry engines (ElectricSQL, Zero, LiveStore, PowerSync) appear mostly outside peer-reviewed venues. This needs a targeted ACM DL / CIDR search.
