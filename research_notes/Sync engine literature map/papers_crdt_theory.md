# Annotated bibliography: CRDT algorithms, local-first theory, replicated-data semantics

Compiled 2026-11-04 for the appendix of `reports/Sync engine thesis novelty check.md`. Claim IDs follow that report: C1 server-ordered log/rebase, C2 CAS blobs+leases, S1 no dangling blob ref, S2 rejection cascade, S3 revocation bound, S4 convergence, S5 offline-aware blob GC, C4/F1–F5 folder codecs and lens laws, C5 per-field merge, C6/S6 validator placement and equivalence, C7 CRDT fields, C8 integration, D1 distribution transparency.

Tags: **[F]** = fetched or surfaced by search this session (link resolves, metadata checked). **[F′]** = verified in the earlier session (`research_notes/Local first sync engine research/academic_crdt_localfirst.md`) and not re-fetched. **[M]** = model memory; treat as a lead.

Sweep done: PaPoC 2023, 2024, 2025, 2026 programs fetched in full. Targeted searches covered arXiv cs.DC/cs.PL 2025–2026, OOPSLA/ECOOP/ICFP, CIDR/VLDB, partial replication, GC, invariants, and tree/rich-text CRDTs. Systems venues (EuroSys/OSDI/SOSP/NSDI/ATC) were only checked through known papers; see Coverage gaps.

---

## 1. Foundations: SEC, CRDT taxonomy, local-first

**Takeaway:** S4 is settled theory. The ICFP 2025 result lets the thesis specify semantics abstractly and treat the wire format (op, state or delta) as an implementation detail.

### Conflict-free Replicated Data Types
- Shapiro, Preguiça, Baquero, Zawirski · SSS 2011 (plus INRIA RR-7506 / inria-00609399) · 2011
- https://hal.inria.fr/inria-00609399 [M]
- Summary: Defines strong eventual consistency (SEC) and state-based (semilattice join) and op-based (commutative ops plus causal delivery) CRDTs. Gives the standard catalog: counters, registers, OR-Set and sequences.
- Relevance: The canonical citation for S4 and C7. It also marks S4 as "Done".

### Delta State Replicated Data Types
- Almeida, Shoker, Baquero · JPDC 2018 (arXiv 2016)
- https://arxiv.org/abs/1603.01529 [M]
- Summary: Ships join-decomposable deltas instead of full state. Causal delta-merging recovers causal consistency over lossy channels.
- Relevance: Prior art for D1's "lossy notification bus". Convergence survives message loss once deltas are re-sent.

### Approaches to Conflict-free Replicated Data Types
- Paulo Sérgio Almeida [M for author] · ACM Computing Surveys 2024 (arXiv 2310.18220)
- https://dl.acm.org/doi/full/10.1145/3695249 ; https://arxiv.org/abs/2310.18220 [F, surfaced]
- Summary: A survey of the op-based, state-based, delta and pure-op design families and their delivery assumptions.
- Relevance: The single survey to cite for the background chapter (S4, C7).

### Local-First Software: You Own Your Data, in spite of the Cloud
- Kleppmann, Wiggins, van Hardenberg, McGranaghan · Onward! 2019
- https://www.inkandswitch.com/local-first/ [M]
- Summary: Sets out seven ideals (offline, multi-device, collaboration, longevity, privacy, user control, no spinners) and proposes CRDTs as the enabler. Lists schema change and access control as open problems.
- Relevance: The framing citation for C8. It stays agnostic between central authority and pure CRDT, which leaves room for C1's server-ordered design.

### Verifying Strong Eventual Consistency in Distributed Systems
- Gomes, Kleppmann, Mulligan, Beresford · OOPSLA 2017
- https://arxiv.org/abs/1707.01747 [M]
- Summary: Isabelle/HOL proofs of SEC for RGA, OR-Set and a counter, over an abstract network model.
- Relevance: The precedent for mechanized S4. The thesis's simulation-checked approach for S1–S6 is the weaker, cheaper alternative.

### CRDT Emulation, Simulation, and Representation Independence
- Liittschwager, Castello, Tsampas, Kuper · ICFP 2025, PACMPL 9 (ICFP) Art. 259 · Aug 2025
- https://arxiv.org/abs/2504.05398 ; DOI 10.1145/3747528 [F]
- Summary: Defines emulation of state-based by op-based CRDTs and the reverse as weak simulation, and characterizes which properties emulation preserves. Clients cannot distinguish the two implementations.
- Relevance: Formally justifies separating the semantics (S1–S6) from the transport (C1 op log vs. snapshot). It does not cover server rejection (S2).

### Wait-free Replicated Data Types and Fair Reconciliation
- Kuznetsov, Perion, Tucci-Piergiovanni · arXiv cs.DC 2508.18193 (v3 Aug 2026), no venue listed
- https://arxiv.org/abs/2508.18193 [F]
- Summary: Formalizes CRDT-like types as eventual state-machine replication with added stability and fairness conditions. Gives a generic DAG framework in which reconciliation functions prevent client starvation.
- Relevance: A formal home for C1's "rebase on client" and for the starvation risk in S2, where one client's writes are rejected repeatedly. Cite it as related theory.

---

## 2. Coordination theory: CALM, I-confluence and their 2026 extension

**Takeaway:** Theory says exactly which operations need coordination. The 2026 Complete CALM paper subsumes CRDTs, I-confluence and HATs, which is the strongest theoretical anchor for C6's "non-I-confluent relational invariants".

### Keeping CALM: When Distributed Consistency Is Easy
- Hellerstein, Alvaro · CACM 63(9), 2020
- https://arxiv.org/abs/1901.01930 [M]
- Summary: A program has a consistent, coordination-free implementation if and only if it is monotonic. Negation and universal quantification force coordination.
- Relevance: The background for C6. A validator that checks non-monotone invariants (uniqueness, quotas) must coordinate, which justifies a server or log authority.

### Complete CALM: A Coordination Criterion for Specifications
- Joseph M. Hellerstein · arXiv cs.DC 2602.09435 (v1 10 Feb 2026, v4 14 Jun 2026), no venue
- https://arxiv.org/abs/2602.09435 [F]
- Summary: Extends CALM from relational transducers to general concurrent specifications: coordination-free implementation exists exactly when outcomes are monotone under a declared refinement order ("future-monotone"). A companion CAP result covers distributed-monotone specs. Claims to subsume CRDTs, I-confluence and HATs. Note: the earlier notes call it "The Coordination Criterion", but the arXiv title is now "Complete CALM".
- Relevance: Supports C6. Server-side rejection is necessary precisely for non-monotone invariants. A threat: a reviewer may ask why S2's rejection cascade is not just a corollary, so frame S2 as the *mechanism* rather than the *criterion*.

### Coordination Avoidance in Database Systems (I-confluence)
- Bailis, Fekete, Franklin, Ghodsi, Hellerstein, Stoica · PVLDB 8(3), 2015
- https://arxiv.org/abs/1402.2237 [M]
- Summary: Invariant confluence is necessary and sufficient for coordination-free execution that preserves an invariant. It classifies common SQL constraints, for example FK insert is I-confluent and uniqueness is not.
- Relevance: The precise tool for deciding which mutations in C1/C6 need server validation. It does not model authority-driven rejection, which is the S2 framing.

### Bridging the Gap: Opportunities in Coordination-Avoiding Database Systems
- Bailis, Fekete, Ghodsi, Franklin, Hellerstein, Stoica · PaPoC 2023 Test-of-Time award (original 2013)
- https://papoc-workshop.github.io/2023/program.html [F program listing]
- Summary: An early call to exploit application semantics to avoid coordination in databases. Its test-of-time recognition in 2023 shows the line is still central.
- Relevance: Background for C6. It is cited alongside I-confluence.

### Keep CALM and CRDT On
- Laddad, Power, Milano, Cheung, Crooks, Hellerstein · PVLDB 16(4), 2022/23
- https://arxiv.org/abs/2210.12605 [F′]
- Summary: CRDTs converge on updates but leave reads unconstrained, so non-monotone reads of intermediate state are inconsistent. Proposes a CALM-based query model to identify which reads are coordination-free.
- Relevance: Directly relevant to S1 and S3. "No dangling reference *after convergence*" says nothing about reads before convergence, so the thesis must state its read semantics for pending refs.

### Wrapping Rings in Lattices: An Algebraic Symbiosis of Incremental View Maintenance and Eventual Consistency
- Power, Achalla, Cottone, Macasaet, Hellerstein · PaPoC 2024
- https://papoc-workshop.github.io/2024/program.html [F program listing; paper PDF not fetched]
- Summary: Unifies IVM (ring-based deltas) with lattice-based eventual consistency.
- Relevance: Theory for reactive client queries over synced rows (C1 read path). Peripheral.

---

## 3. Invariants and mixed consistency

**Takeaway:** Compile-time or annotation-driven invariant enforcement for local-first apps is an active PL topic (LoRe, ConLoc, No-Op). None of it handles blobs, folders or rejection cascades, which is where S2 sits.

### Ensuring Convergence and Invariants Without Coordination (No-Op)
- Borrego, Ferreira, Gonzalez Boix, Preguiça · ECOOP 2025, LIPIcs ECOOP.2025.4 (artifact DARTS 11.2.1)
- https://drops.dagstuhl.de/entities/document/10.4230/LIPIcs.ECOOP.2025.4 ; https://2025.ecoop.org/details/ecoop-2025-technical-papers/28/Ensuring-Convergence-and-Invariants-Without-Coordination [F]
- Summary: Programmer-defined policies rank concurrent conflicting operations, and the loser becomes a no-side-effect op. This guarantees convergence and invariant preservation without coordination. Includes a formalized replication protocol with a correctness proof.
- Relevance: **Threatens S2** most directly in the PL literature. "Loser becomes a no-op" is a coordination-free analog of rejection. The thesis's delta is authority-driven rejection plus a causal cascade to dependents and non-row side effects (blobs, claims). No-Op neutralizes single operations, not their offline dependents.

### ECROs: Building Global Scale Systems from Sequential Code
- De Porre, Ferreira, Preguiça, Gonzalez Boix · OOPSLA 2021
- https://dl.acm.org/doi/10.1145/3485484 [M]
- Summary: Derives replicated objects from sequential code plus invariants. Operations are ordered only when they conflict.
- Relevance: Prior art for C5/C6. Merge rules derived from invariants, with partial coordination.

### LoRe: A Programming Model for Verifiably Safe Local-First Software
- Haas, Mogk, Yanakieva, Bieniusa, Mezini · ECOOP 2023 (later TOPLAS)
- https://arxiv.org/abs/2304.07133 [F′]
- Summary: A reactive programming model and compiler that verifies developer-stated invariants and inserts coordination only where needed.
- Relevance: Prior art for C6. It does "validator only where needed" statically, whereas the thesis does it at runtime with a placement switch (S6).

### Consistent Local-First Software: Enforcing Safety and Invariants for Local-First Applications (ConLoc)
- Köhler et al. (consysT) · IEEE TSE 2024/25; FSE 2025 journal-first
- https://dl.acm.org/doi/10.1109/TSE.2024.3477723 [F′]
- Summary: A Java annotation language that enforces invariants in local-first apps, with large latency and throughput gains over full serialization.
- Relevance: Same as LoRe. C5/C6 annotations exist, so the thesis must not claim declarative invariants as new.

### RedBlue Consistency (Making Geo-Replicated Systems Fast as Possible, Consistent when Necessary)
- Li, Porto, Clement, Gehrke, Preguiça, Rodrigues · OSDI 2012
- https://www.usenix.org/conference/osdi12/technical-sessions/presentation/li [M]
- Summary: Blue ops commute and run locally. Red ops are totally ordered.
- Relevance: The systems ancestor of C1+C6: a server-ordered log for red ops and local application for blue ops.

---

## 4. Verified and synthesized replicated data types

**Takeaway:** Push-button verification of RDTs is maturing fast (Sal, replication-aware linearizability, convergent-by-construction combinators). It is a credible "future work" path for S1–S6, but too costly for a four-month thesis versus deterministic simulation.

### Katara: Synthesizing CRDTs with Verified Lifting
- Laddad, Power, Milano, Cheung, Hellerstein · OOPSLA 2022, PACMPL 6 Art. 173
- https://dl.acm.org/doi/10.1145/3563336 ; https://arxiv.org/abs/2205.12425 [F DOI surfaced]
- Summary: Synthesizes verified state-based CRDTs from a sequential type plus a conflict-ordering annotation.
- Relevance: Prior art for C5. Declared per-field merge semantics with correctness guarantees already exist.

### VeriFx: Correct Replicated Data Types for the Masses
- De Porre, Ferreira, Gonzalez Boix · ECOOP 2023
- https://arxiv.org/abs/2207.02502 [M]
- Summary: A functional language that auto-verifies RDT convergence via SMT and transpiles to mainstream languages.
- Relevance: A candidate tool to verify C5 merge rules if the thesis wants a proof for S4.

### Automatically Verifying Replication-aware Linearizability
- Soundarapandian, Nagar, Rastogi, Sivaramakrishnan · OOPSLA 2025 (arXiv 2502.19967 extended)
- https://arxiv.org/abs/2502.19967 [F]
- Summary: Automated verification of replication-aware linearizability for MRDTs using algebraic properties of ops and merge, with "bottom-up linearization". Includes a new JSON MRDT.
- Relevance: A stronger correctness notion than SEC that could underpin C5 or C7 field merges. Related work only.

### Sal: Multi-modal Verification of Replicated Data Types
- Ramesh, Soundarapandian, Sivaramakrishnan · PaPoC 2026 (arXiv 2603.27202)
- https://arxiv.org/abs/2603.27202 ; https://papoc-workshop.github.io/2026/program.html [F program; arXiv F′]
- Summary: Combines verification modes (automated plus interactive) for RDTs. It accompanies KC Sivaramakrishnan's PaPoC 2026 keynote "From Convergence to Confidence: Push-Button Verification for Replicated Data Types".
- Relevance: Shows that the community expects verified, not just tested, convergence. Reviewers may ask why S1–S6 are only simulation-checked, so the answer is scope plus the non-row effects (blobs, files) these tools cannot model.

### Composing CRDTs Convergent by Construction
- Städing Dominguez, Zakhour, Weisenburger, Salvaneschi · OOPSLA 2026, PACMPL 10 (OOPSLA2)
- https://programming-group.com/assets/pdf/papers/2026_Composing_CRDTs_Convergent_by_Construction.pdf [F, surfaced via search; PDF not read]
- Summary: CRDT combinators whose compositions are CRDTs by construction.
- Relevance: Supports a typed C7 design (CRDT fields composed inside relational rows) as a future-work path.

### AMC: Towards Trustworthy and Explorable CRDT Applications with the Automerge Model Checker
- Jeffery, Mortier · PaPoC 2023
- https://papoc-workshop.github.io/2023/program.html [F program listing]
- Summary: Model-checks applications built on Automerge to explore interleavings.
- Relevance: The closest precedent for the thesis's simulation-checked S1–S6 oracle. Cite it as methodology prior art.

---

## 5. Sequence and text CRDTs

**Takeaway:** Text CRDTs are a mature, crowded area. The thesis should consume one (C7), not contribute one. Weidner's server reconciliation directly supports C1's design choice.

### Collaborative Text Editing with Eg-walker: Better, Faster, Smaller
- Gentle, Kleppmann · EuroSys 2025 · arXiv Sep 2024
- https://arxiv.org/abs/2409.14252 ; DOI 10.1145/3689031.3696076 [F]
- Summary: Stores an event graph and builds CRDT state only transiently while merging concurrent ranges. It merges long-diverged branches orders of magnitude faster than OT, with lower steady-state memory than CRDTs.
- Relevance: C7 prior art. The "event graph + transient merge" model mirrors C1's "log + replay". It also makes long offline divergence (the thesis's central scenario) cheap for text fields.

### The Art of the Fugue: Minimizing Interleaving in Collaborative Text Editing
- Weidner, Kleppmann · IEEE TPDS (Nov 2025); arXiv 2305.00583
- https://arxiv.org/abs/2305.00583 ; https://martin.kleppmann.com/2025/11/01/fugue-minimizing-interleaving.html [F′]
- Summary: Shows that interleaving anomalies affect both CRDTs and OT, and gives Fugue, which achieves maximal non-interleaving.
- Relevance: C7. The right algorithm choice for any text field, and it cannot be claimed.

### Collaborative Text Editing without CRDTs or OT
- Matthew Weidner · blog, 21 May 2025 (Local-First Conf 2025 talk); not peer-reviewed
- https://mattweidner.com/2025/05/21/text-without-crdts.html [F]
- Summary: Each character gets a permanent ID. Clients send "insert after ID" mutations, and the server reconciles optimistic local updates (Replicache/Zero style).
- Relevance: **Supports C1** as a deliberate design: server reconciliation with client rebase is enough for text when a server exists. It also weakens any claim that C7 needs a full CRDT.

### ESBT: A Scalable and Deterministic Sequence CRDT for Distributed Collaborative Editing
- Mechaoui, Imine · arXiv cs.DC 2607.28101 · 30 Jul 2026, no venue
- https://arxiv.org/abs/2607.28101 [F]
- Summary: Stern-Brocot-tree identifier allocation for sequence CRDTs (Logoot/LSEQ family). Reports 50–75% lower identifier memory than Logoot/LSEQ in some insertion patterns. A search snippet says it critiques Eg-walker's GC complexity, which was not read in full.
- Relevance: Shows that 2026 sequence-CRDT work is incremental. C7 low priority.

### Peritext: A CRDT for Collaborative Rich Text Editing
- Litt, Lim, Kleppmann, van Hardenberg · PACMHCI 6 (CSCW2), Nov 2022
- https://dl.acm.org/doi/10.1145/3555644 [F DOI surfaced]
- Summary: Formatting-span CRDT that preserves intent at mark boundaries under concurrent edits.
- Relevance: C7. It is also relevant to C4 if structured file codecs ever include rich text (Markdown), because line-level field merges lose span intent.

---

## 6. Trees, JSON, undo, spreadsheets, conflict semantics

**Takeaway:** Structured-data CRDTs (move, undo, spreadsheets, semantic conflicts) give the vocabulary for C4's structured-file codecs and C5's per-field rules. Undo and conflicts-as-objects are the most relevant to S2's compensation story.

### A Highly-Available Move Operation for Replicated Trees
- Kleppmann, Mulligan, Gomes, Beresford · IEEE TPDS 33(7), 2022 (online 2021)
- https://martin.kleppmann.com/papers/move-op.pdf ; DOI 10.1109/TPDS.2021.3118603 [F, surfaced]
- Summary: Move with undo-do-redo over a timestamp-ordered log prevents cycles. Verified in Isabelle/HOL.
- Relevance: Relevant to C4 when a mounted folder's renames and moves map to tree ops (F2 and F5). Its undo-do-redo replay is also the same mechanism as C1's rebase.

### A coordination-free, convergent, and safe replicated tree (Maram)
- Nair, Meirim, Pereira, Ferreira, Shapiro · INRIA RR-9395, arXiv 2103.04828 (2021–22)
- https://arxiv.org/abs/2103.04828 [F, surfaced; authors M]
- Summary: Coordination-free atomic subtree moves with mechanized proofs of convergence and the tree invariant.
- Relevance: An alternative for C4 folder-tree semantics. It shows invariant-preserving moves without a server.

### Extending JSON CRDTs with Move Operations
- Da, Kleppmann · PaPoC 2024 (arXiv 2311.14007)
- https://arxiv.org/abs/2311.14007 ; https://papoc-workshop.github.io/2024/program.html [F]
- Summary: Adds move to JSON CRDTs (Automerge-style) for lists and maps.
- Relevance: C7 and C4. Prior art for moving structured fields within decoded file content.

### Undo and Redo Support for Replicated Registers
- Stewen, Kleppmann · PaPoC 2024
- https://papoc-workshop.github.io/2024/program.html [F program listing]
- Summary: Defines undo/redo semantics for replicated (LWW/multi-value) registers under concurrency.
- Relevance: **Relevant to S2**. Compensating a rejected write resembles a system-issued undo. Cite it when defining what "revert" means for per-field LWW registers (C5).

### A Study of Semantics for CRDT-based Collaborative Spreadsheets
- Yanakieva, Bird, Bieniusa · PaPoC 2023
- https://papoc-workshop.github.io/2023/program.html [F program listing]
- Summary: Catalogs the intended merge semantics for concurrent spreadsheet edits, such as row and column insertion versus cell edits.
- Relevance: Relevant to C4/C5 for structured codecs (CSV files mapped to rows). Use it as the semantics checklist.

### AegisSheet: A Compositional CRDT for Collaborative Spreadsheets
- Pfeil, Scandurra, Haas · PaPoC 2026
- https://papoc-workshop.github.io/2026/program.html [F program listing]
- Summary: Composes spreadsheet CRDTs from smaller components.
- Relevance: C4/C5. The current state of the art for tabular file merge.

### Directed Acyclic Graph CRDTs
- Borth, Lersch, Bieniusa · PaPoC 2025
- https://papoc-workshop.github.io/2025/program.html [F program listing]
- Summary: CRDTs for DAG-shaped data, beyond trees.
- Relevance: Possibly relevant to S2, since the causal-dependency closure over rows and blobs is a DAG. Peripheral.

### Semantic Conflict Model for Collaborative Data Structures
- Semenov, Aksenov · PaPoC 2026
- https://papoc-workshop.github.io/2026/program.html [F program listing]
- Summary: Treats conflicts as first-class semantic objects rather than resolving them silently.
- Relevance: Supports the user-visible conflict surface in C4 (Harmony-style "never overwrite", F3) and S2 (visible rejection). Cite it to show conflicts-as-data is a live topic.

### For-Each Operations in Collaborative Apps
- Weidner, Pradeep, Geordie, Miller · PaPoC 2023
- https://papoc-workshop.github.io/2023/program.html [F program listing]
- Summary: Semantics for operations that apply to every element of a collection, including concurrently inserted ones.
- Relevance: C1/C5. Multi-row mutations ("mark all done") under concurrency is exactly where per-row rebase is subtle.

---

## 7. Frameworks and implementations

**Takeaway:** Library-level CRDT frameworks are well covered. Automerge 3/Hexane is engineering, with no paper.

### Collabs: A Flexible and Performant CRDT Collaboration Framework
- Weidner, Qi, Kjaer, Pradeep, Geordie, Zhang, Schare, Tang, Xing, Miller · arXiv 2212.02618 (v2 Oct 2023)
- https://arxiv.org/abs/2212.02618 [F]
- Summary: A framework for composing custom CRDTs. Its rich-text app scales to over 100 concurrent users, with lower latency and server CPU than an OT framework.
- Relevance: C7 prior art. **Venue unverified**: the arXiv page has no journal-ref, so the earlier notes' "EuroSys 2024" claim is unconfirmed. Cite it as arXiv unless the venue is confirmed.

### ReDunT: Automatically Deriving Redundancy Relations for Pure Op-Based CRDTs
- Borrego, Vilalonga, Domingos, Preguiça, Gonzalez Boix, Ferreira · PaPoC 2025
- https://papoc-workshop.github.io/2025/program.html [F program listing]
- Summary: Derives the redundancy relations that let pure op-based CRDTs prune their op logs.
- Relevance: **Relevant to S5/GC and to log compaction of C1**. The closest 2025 work on safe log pruning, though it covers ops, not blobs.

### Automerge 3.0 / Hexane columnar engine (not a paper)
- Automerge team, Ink & Switch · blog posts 2025 and July 2026
- https://automerge.org/blog/automerge-3/ ; https://automerge.org/blog/2026-july/ [F′]
- Summary: Automerge 3 runs on its compressed columnar format and reports more than 10x memory reduction. Hexane v1 is a new columnar storage engine reported 2–9x faster. No paper or technical report was found.
- Relevance: C7 engineering context only. Cite it as a vendor blog.

---

## 8. Schema evolution and versioning

**Takeaway:** Schema change across live replicas is named an open problem by Edwards/Petricek/van der Storm/Litt. The thesis can cite it as a limitation rather than solve it.

### Schema Evolution in Interactive Programming Systems
- Edwards, Petricek, van der Storm, Litt · The Art, Science, and Engineering of Programming 9(1) Art. 2, 2025 · arXiv 2412.06269 (Dec 2024)
- https://arxiv.org/abs/2412.06269 [F]
- Summary: Argues that schema change forces ad hoc migration of data and code. Poses challenge problems across databases, live front-ends, model-driven development and collaborative documents, including local-first replicas with in-flight ops.
- Relevance: Relevant to C1 and C4. Validator or schema version skew across clients is exactly what S6 warns about. Cite it as the open-problem statement and scope it out.

### Baseline: Operation-Based Evolution and Versioning of Data
- Edwards, Petricek · arXiv 2512.09762 (v1 Dec 2025, v2 Jun 2026); submitted to ⟨Programming⟩
- https://arxiv.org/abs/2512.09762 [F′]
- Summary: "Operational differencing" represents data as high-level ops, including schema changes, so diff and merge work across schema versions without a repository.
- Relevance: A threat to C4 framing if read broadly, since it is also op-based structured merge. Mostly future work for schema migration of C1's log.

### Cambria: Schema Evolution in Distributed Systems with Edit Lenses
- Litt, van Hardenberg, Henry · Ink & Switch essay 2020 (also PaPoC 2021 [M])
- https://www.inkandswitch.com/cambria/ [M]
- Summary: Bidirectional lenses translate documents between schema versions so old and new clients co-edit.
- Relevance: **Prior art for F1/F2** (lens laws applied to replicated data). C4 must cite it next to Boomerang.

### The Essence of Git: Concepts for P2P Replication
- Erick Lavoie · arXiv cs.DC 2609.13555 · 11 Sep 2026, 8 pp, no venue
- https://arxiv.org/abs/2609.13555 [F]
- Summary: Analyzes Git's data model and sync as a convergent replicated structure and compares it with P2P frameworks, using Jackson's concept design.
- Relevance: Background for C2 (content addressing) and for folder-as-replica (C4). Peripheral.

---

## 9. Partial replication, sync bandwidth, GC

**Takeaway:** The academic record is thin. Partial replication is mostly 2015–2017 work, and 2025–2026 offers bandwidth (ConflictSync) and op pruning (ReDunT) but nothing on reference-aware GC of external blobs. This supports S5's novelty and S3's "weakest evidence" status.

### Conflict-Free Partially Replicated Data Types
- Briquemont, Bravo, Li, Van Roy [M for authors] · IEEE CloudCom 2015
- https://ieeexplore.ieee.org/document/7396168/ [F, surfaced]
- Summary: Defines partial replication of CRDTs that preserves SEC on partial replicas. Integrated into SwiftCloud.
- Relevance: Prior art for S3/D1 partial replicas. It does not address revocation-driven eviction or time bounds, so S3's gap stands.

### Non-uniform Replication
- Cabrita, Preguiça · OPODIS 2017 [M venue] · arXiv 1711.07733
- https://arxiv.org/abs/1711.07733 [F, surfaced]
- Summary: Replicas store different subsets of state while still providing eventual consistency for query results.
- Relevance: Relevant to S3. A partial replica that holds less data while staying correct for its queries.

### ConflictSync: Bandwidth Efficient Synchronization of Divergent State
- Baquero, Gomes, Rodrigues · PaPoC 2026
- https://papoc-workshop.github.io/2026/program.html [F program listing]
- Summary: Reduces sync bandwidth for replicas that have diverged substantially.
- Relevance: Relevant to C1 and D1 catch-up after long offline periods. An optimization the thesis can cite and skip.

### Probabilistic Causal Contexts for Scalable CRDTs
- Fernandes, Baquero · PaPoC 2023
- https://papoc-workshop.github.io/2023/program.html [F program listing]
- Summary: Compact probabilistic representations of causal metadata.
- Relevance: Peripheral to S5/C1 metadata growth.

---

## 10. Byzantine and untrusted servers (validator placement)

**Takeaway:** BFT CRDT work uses hash DAGs and signed ops. It strengthens the report's warning that S6 holds only against an honest server unless the log is hash-chained.

### Making CRDTs Byzantine Fault Tolerant
- Kleppmann · PaPoC 2022
- https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf [M; title F′]
- Summary: A hash-DAG of ops gives BFT eventual consistency with any number of Byzantine replicas.
- Relevance: Relevant to C6/S6. The hash-chain recommendation for the server log comes from here.

### Proof-Carrying CRDTs allow Succinct Non-Interactive Byzantine Update Validation
- Marx, Jacob, Hartenstein · PaPoC 2025
- https://papoc-workshop.github.io/2025/program.html [F program listing]
- Summary: Updates carry proofs so that replicas validate them succinctly without interaction.
- Relevance: **Threatens or extends S6**. It is client-verifiable validation without trusting a validator, close to the E2EE mode of C6.

### Decoupling Trust in Byzantine CRDTs: Fine-grained Post-Compromise Handling without Breaking Causality
- arXiv 2606.31759 (Jun 2026) [F, surfaced; authors and venue not checked]
- https://arxiv.org/html/2606.31759
- Summary: Delta-state JSON CRDT (Melda) with Byzantine-resilience handling. Replicas observing the same delta blocks converge despite compromise.
- Relevance: Relevant to S3/S6. Post-compromise handling is analogous to revocation within a bound.

### Towards System-Oriented Formal Verification of Local-First Access Control
- Jacob, Stuber, Hartenstein · arXiv 2604.23560 (Apr 2026); PaPoC 2026 lightning talk
- https://arxiv.org/abs/2604.23560 [F′; PaPoC listing F]
- Summary: Argues that local-first access control (Matrix, Keyhive) rests on informal specs and proposes system-oriented formal verification.
- Relevance: Supports S3 being hard and open. Cite it to justify deferring S3.

---

## Coverage gaps

- **Systems venues 2024–2026 (EuroSys/OSDI/SOSP/NSDI/ATC):** Eg-walker (EuroSys 2025) is the only confirmed CRDT or local-first paper found. Accepted-paper lists were not fetched, so other papers may exist.
- **VLDB/SIGMOD/CIDR 2024–2026:** No local-first or sync-engine paper was found. CIDR 2026 "Rosé" and PVLDB 18 "LAW theorem" are server-side replication and were left out as off-topic.
- **PLDI/POPL/ICFP 2024–2026:** Only the ICFP 2025 emulation paper was found. No POPL 2025/2026 or PLDI CRDT paper surfaced, which is not proof of absence.
- **Collabs venue:** The arXiv page carries no journal-ref. The "EuroSys 2024" claim in earlier notes is unverified.
- **Automerge 3/Hexane:** No paper or technical report exists. Only blogs.
- **CRDT GC 2024–2026:** No primary paper on tombstone or history GC was found beyond ReDunT (op redundancy) and ESBT's remarks. Nothing on GC of *external* blobs referenced by CRDT or relational state, which supports S5 as "not found".
- **Partial replication 2024–2026:** No new primary paper was found. The 2015/2017 work stands.
- **Undo CRDTs beyond PaPoC 2024:** No 2025–2026 follow-up found.
- **Rich-text CRDTs 2024–2026:** Nothing new beyond Peritext and Fugue found. Not checked: any Peritext-successor paper, Loro papers.
- **PaPoC 2024 and 2025 PDFs:** Only the program listings were fetched. Summaries of those papers are based on titles and are marked accordingly.
- **[M] items:** Shapiro 2011, delta-CRDTs, CALM CACM, I-confluence, Gomes 2017, VeriFx, ECROs, RedBlue, Cambria, Maram authors, partial-replication authors and BFT CRDT links were not re-fetched.
- Several 2026 arXiv hits were skipped as off-topic: CRDTs for neural-network model merging (2605.19373) and Dynamic Software Updates using CRDTs (2606.10920). The latter is possibly relevant to validator version skew in S6 but was not read.
