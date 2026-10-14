# Security for sync: E2EE, untrusted servers, local-first access control, group keys, Byzantine tolerance, fork detection

Annotated bibliography for the appendix of `reports/Sync engine thesis novelty check.md`. Claim IDs refer to that report's verdict table: C1 is the server-ordered log, C6/S6 is one deterministic validator run on the server or replayed by clients under opt-in E2EE, S3 is the revocation bound, S4 is convergence, and C7 is CRDT fields. Researched November 2026.

Tags: **[F]** means verified this session. The URL resolved (HTTP 200) and the title and authors were checked against page metadata, or the abstract was fetched. **[M]** means model memory or a secondary source that was not re-verified; check it before citing. Venue and date facts carry the tag of the link they came from unless marked otherwise.

---

## 1. Foundations: untrusted servers, fork consistency (pre-2015)

### SUNDR: Secure Untrusted Data Repository
- Jinyuan Li, Maxwell Krohn, David Mazières, Dennis Shasha · OSDI 2004 · Dec 2004
- https://www.usenix.org/conference/osdi-04/secure-untrusted-data-repository-sundr [F]
- Summary: A network file system whose server is untrusted. Clients sign version structures, so a misbehaving server can only *fork* clients into views that never see each other's later updates. It cannot silently tamper with data or reorder updates. The paper introduces **fork consistency**: once forked, clients cannot be joined again without detection.
- Relevance: This is the baseline that **S6** (in E2EE mode) and C1 must cite. Without SUNDR-style signed and hash-chained history, S6 holds only against an honest-ordering server.

### SPORC: Group Collaboration using Untrusted Cloud Resources
- Ariel J. Feldman, William P. Zeller, Michael J. Freedman, Edward W. Felten · OSDI 2010 · Oct 2010
- https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/sporc.pdf [F]
- Summary: The server only orders and stores encrypted client operations. Clients run operational transformation (OT) and keep their own access control list (ACL) replayed from the log. A non-malicious server also rejects unauthorized ACL-changing ops early on plaintext metadata. Fork* consistency plus OT-based recovery lets clients detect and repair server forks.
- Relevance: This is the direct prior art for **C6/S6**: a dual-role validator, with the server checking on plaintext metadata and clients re-checking by replay. It also threatens any "first E2EE validator" framing of **C1/C6**.

### Depot: Cloud Storage with Minimal Trust
- Prince Mahajan, Srinath Setty, Sangmin Lee, Allen Clement, Lorenzo Alvisi, Mike Dahlin, Michael Walfish · OSDI 2010 · Oct 2010
- https://www.usenix.org/conference/osdi10/depot-cloud-storage-minimal-trust [F]
- Summary: A key-value store that tolerates any number of faulty clients and servers. It offers fork-join-causal (FJC) consistency: forks are detected and then *joined* by treating the forked views as concurrent updates. Availability and durability survive even when the servers misbehave.
- Relevance: Its FJC model is the template for stating **S4** (convergence) under a Byzantine server. It shows that fork recovery, not just detection, is prior art.

### Venus: Verification for Untrusted Cloud Storage
- Alexander Shraer, Christian Cachin, Asaf Cidon, Idit Keidar, Yan Michalevsky, Dani Shaket · ACM CCSW 2010 · Oct 2010
- https://people.csail.mit.edu/idish/ftp/venus-ccsw2010.pdf [F]
- Summary: Venus wraps an unmodified object store (evaluated on Amazon S3) so that clients complete operations optimistically. It later verifies integrity and consistency and notifies the application, with no trusted components. Its follow-up, VICOS ([arXiv 1502.04496](https://arxiv.org/abs/1502.04496) [F, via existing notes]), handles mutually trusting client groups on cloud object stores.
- Relevance: It is a pattern for **S6** over an unmodified Postgres/S3 backend: verify after the fact with eventual notification rather than blocking. It also supports a "fork detection as an add-on layer" framing for C2 blobs.

### Frientegrity: Privacy and Integrity with an Untrusted Provider
- Ariel J. Feldman, Aaron Blankstein, Michael J. Freedman, Edward W. Felten · USENIX Security 2012 · Aug 2012
- https://www.usenix.org/conference/usenixsecurity12/technical-sessions/presentation/feldman [F]
- Summary: An E2EE social network on an untrusted provider. Each object's history is a hash-chained, signed log with fork* consistency. ACLs live in persistent authenticated dictionaries, so clients can verify membership efficiently without downloading the full history. It scales the SPORC ideas to many objects and large groups.
- Relevance: It shows how to keep **S6**-style client verification sub-linear by verifying ACLs with authenticated dictionaries instead of replaying the full log. That is relevant to S3 revocation as well.

### CONIKS: Bringing Key Transparency to End Users
- Marcela Melara, Aaron Blankstein, Joseph Bonneau, Edward Felten, Michael Freedman · USENIX Security 2015 · Aug 2015
- https://www.usenix.org/conference/usenixsecurity15/technical-sessions/presentation/melara [F]
- Summary: A privacy-preserving Merkle-prefix-tree key directory. Each epoch's signed tree root (STR) is chained to the previous one, and clients audit their own bindings and gossip STRs to detect equivocation. It became the template for Signal, WhatsApp and IETF key transparency.
- Relevance: A transparency log over the sync log's head would give **S6** fork detection. Encrypted Spaces assumes such a signer but has not implemented it.

---

## 2. E2EE collaborative documents and encrypted sync (2019–2026)

### Encrypted Spaces (whitepaper)
- Harvard Applied Social Media Lab (Trapp, Perrin et al.), with Microsoft Research · Whitepaper / preview library · 11 Jun 2026
- https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf [F]; https://www.microsoft.com/en-us/research/project/encrypted-spaces/ [F]
- Summary: An architecture for E2EE collaborative apps. The server stores and orders data and acts as the sync point but never sees plaintext. Clients run the same `apply` function and verify server responses against committed history. Columns are encrypted by default, and `plaintext=#true` opts a column into server-side indexing. Fork resistance relies on a transparency signer that the authors say is not fully implemented.
- Relevance: This is the closest prior art for **C6/S6**. It differs from the thesis claim in having no runtime per-user trust-mode switch or migration, and it leaves encrypted-content invariants as future work.

### SCUBA
- Natalie Popescu et al. (Princeton) · ACSAC 2025 · Dec 2025
- https://www.cs.princeton.edu/~npopescu/papers/scuba.pdf [F, URL; content via Encrypted Spaces related work, full text not read]
- Summary: Secure collaborative apps over an untrusted backend. App-defined write validators run on clients over an E2EE log, so the server cannot enforce application rules on ciphertext. Characterization is secondhand; read §3–4 before citing details.
- Relevance: It supports "client-replayed validator under E2EE is done" for **C6/S6**, and threatens any novelty claim on the mechanism.

### Acumen: A Platform for Encrypted and Accountable Collaborative Editing
- Ryan Cottone (Stanford), Darya Kaviani, Conor Power, Will Giorza, Evelyn Koo, Natacha Crooks, Raluca Ada Popa (UC Berkeley) · OSDI 2026 · Jul 2026
- https://www.usenix.org/conference/osdi26/presentation/cottone [F]; PDF https://www.usenix.org/system/files/osdi26-cottone.pdf [F]
- Summary: A cryptographic layer for CRDT-based real-time apps. It provides confidentiality, integrity and **fork-causal consistency**, plus "strong snapshot consistency": an untrusted user can make a verifiable snapshot to onboard newcomers without revealing prior edit history. It uses cryptographic accumulators and a secure garbage collection scheme, and handles 25 users at 60 WPM with negligible added latency.
- Relevance: This is a 2026 top-venue threat or citation for **S6** fork detection and for E2EE compaction. Its "secure GC" is adjacent to **S5** and must be distinguished, since it covers history GC, not blob leases.

### End-to-End Encrypted Collaborative Documents (SignalCD)
- Christian Knabenhans (EPFL), Zayd Maradni (MPI-SWS), Carmela Troncoso (MPI-SP and EPFL) · USENIX Security 2026 · Aug 2026
- https://www.usenix.org/conference/usenixsecurity26/presentation/knabenhans [F]
- Summary: The paper formalizes the security and functional requirements of E2EE collaborative documents. It proves a generic construction secure: an E2EE asynchronous broadcast channel plus *any* reconciliation mechanism that yields globally consistent views. SignalCD instantiates it on Signal group messaging, aimed at investigative journalists, and reaches roughly 120 ms response times (per search snippet).
- Relevance: It offers formal backing for "the merge or validator is orthogonal to the encrypted channel", which supports the **S6** modularity argument. It also supplies a security definition to borrow.

### Snapdoc: Authenticated snapshots with history privacy in peer-to-peer collaborative editing
- Stephan A. Kollmann, Martin Kleppmann, Alastair R. Beresford · PoPETs 2019(3) · Jul 2019
- https://petsymposium.org/popets/2019/popets-2019-0044.php [F]
- Summary: The paper lets an existing collaborator hand a newcomer an authenticated snapshot instead of the full encrypted edit history. The newcomer can still verify consistency, and earlier history stays private. Data sent to newcomers drops by a median of 84%.
- Relevance: It covers snapshot and compaction under E2EE for **C1** log compaction in E2EE mode, and is the precursor of Acumen and Beelay Sedimentree.

### Secure Conflict-free Replicated Data Types
- Manuel Barbosa, Bernardo Ferreira, João Marques, Bernardo Portela, Nuno Preguiça · ICDCN 2021 (ACM, DOI 10.1145/3427796.3427831) · Jan 2021
- https://eprint.iacr.org/2020/944 [F]
- Summary: The first formal cryptographic treatment of CRDTs. It builds set and counter CRDTs whose merges run over encrypted data on untrusted replicas, using homomorphic and property-preserving encryption, with proofs. It is integrated into AntidoteDB and evaluated on the Danish Shared Medication Record. A 2023 MPC-based general-purpose follow-up exists [M].
- Relevance: It is the alternative to **S6** client replay: the *server* merges ciphertext. That relates to C5/C7 and shows the trade-off space.

### WhatsApp app-state sync and LtHash ("Securing Update Propagation with Homomorphic Hashing")
- Kevin Lewi, Wonho Kim, Ilya Maykov, Stephen Weis (Facebook) · IACR ePrint 2019/227 · 2019; WhatsApp Security Whitepaper (living doc)
- https://eprint.iacr.org/2019/227 [F]; https://www.whatsapp.com/security/WhatsApp-Security-Whitepaper.pdf [F, URL]
- Summary: LtHash is a lattice-based homomorphic set hash, so adding or removing an element updates the digest in O(1). WhatsApp uses it with MACs for E2EE **app state sync** across a user's devices (mutes, pins, contacts). The server stores encrypted mutations, and devices detect dropped or rolled-back mutations by checking the LtHash of the state. The whitepaper section details are [M].
- Relevance: It is a deployed, cheap integrity check for **S6** in E2EE mode, an alternative to a full hash chain, and it suits detecting server rollback of C1 log state.

### Apple Advanced Data Protection for iCloud
- Apple · Platform Security Guide plus support article 102630 · ongoing (2022 launch)
- https://support.apple.com/guide/security/advanced-data-protection-for-icloud-sec973254c5f/web [F, URL]; https://support.apple.com/102630 [F]
- Summary: Opt-in E2EE for most iCloud categories. With ADP on, iCloud.com web access is disabled because servers lack keys, unless a trusted device grants per-session access. Enabling it re-keys the categories and removes service keys; disabling it re-uploads them [M].
- Relevance: It is the product precedent for the opt-in mode switch with feature loss and key migration in **C6/S6**. It does not use a shared validator.

### Proton Drive security model
- Proton AG · Engineering blog · 2022 (updated)
- https://proton.me/blog/protondrive-security [F]
- Summary: Files, filenames and folder structure are encrypted client-side with per-node keys in a key hierarchy (OpenPGP). Shares re-wrap node keys, and the server holds only ciphertext plus signatures for authenticity [M, content not re-read].
- Relevance: It is prior art for E2EE blobs and folders (**C2/C4** under E). A per-node key tree is the obvious design for **S3** revocation by re-keying subtrees.

### CryptPad and ChainPad
- XWiki SAS / CryptPad team · Whitepaper plus ChainPad library · whitepaper 2023 [M], ongoing
- https://blueprints.cryptpad.org/document/whitepaper/ [F]; https://github.com/xwiki-contrib/chainpad [F]
- Summary: A real-time E2EE editor whose server relays and stores an append-only log of encrypted patches. ChainPad orders patches by a hash-chained "blockchain-like" consensus among clients, and the server never sees content. Access is by capability URLs (view or edit keys in the URL fragment).
- Relevance: It is a deployed C1-under-E2EE system with client-side ordering checks. Its weak revocation (key-in-URL) is a cautionary example for **S3**.

### NextGraph: E2EE sync engine, SDK, graph DB, and reactive ORM
- Niko Bonnieure · FOSDEM 2026 (Local-First devroom, plus a talk in the Decentralised Internet devroom) · 31 Jan–1 Feb 2026
- https://archive.fosdem.org/2026/schedule/speaker/niko_bonnieure/ [F]; slides https://fosdem.org/2026/events/attachments/CRSTQ8-nextgraph/slides/267598/nextgraph_v8pnpxa.pdf [F, URL]
- Summary: The brokers only store and forward encrypted blocks, and devices hold keys. NextGraph syncs Yjs, Automerge and its own RDF CRDT, with Rust, Python and Node SDKs and a reactive ORM.
- Relevance: It is a product counterpart for **C8** (E+C7 cluster) with no server-side validation, which supports the claim that no product has R+B+F+E+V.

---

## 3. Local-first access control and capabilities

### Keyhive, Beelay and Sedimentree (Ink & Switch)
- Brooklyn Zelenka, Alex Good, John Mumm et al. · Lab notebook plus code · 2024–2026
- https://www.inkandswitch.com/keyhive/notebook/ [F]; https://www.inkandswitch.com/keyhive/notebook/05/ [F] ("Syncing Keyhive"); https://github.com/inkandswitch/keyhive [F]; https://github.com/automerge/beelay [F]
- Summary: Keyhive is a capability-based group and document access control CRDT (convergent capabilities) for Automerge. BeeKEM provides continuous group key agreement (see §4). Beelay is the auth-aware sync RPC, where the server checks capabilities on metadata without reading content. Sedimentree lets an E2EE server serve compacted ciphertext. Status is pre-alpha or research.
- Relevance: It is the main local-first counterpart to **C6/S6** (metadata validated on the server, content on clients) and to **S3** (revocation). Cite it and contrast it with a relational setting that has non-I-confluent invariants.

### Towards System-Oriented Formal Verification of Local-First Access Control
- Florian Jacob, Johanna Stuber, Hannes Hartenstein (KIT) · arXiv 2604.23560 · 26 Apr 2026
- https://arxiv.org/abs/2604.23560 [F]
- Summary: Matrix and Keyhive pair informal specs with unverified implementations. This paper verifies, in Rust and Verus with Z3 at zero runtime cost, a replicated data type for simplified collaboration groups with capability-based access control and "hash chronicles" for replication. Duelling-admin mutual revocation is out of scope.
- Relevance: It is a model for checking **S6/S3** invariants mechanically. The thesis uses deterministic simulation instead, so cite this as the stronger-assurance alternative.

### ERA: Epoch-Resolved Arbitration for Duelling Admins in Group Management CRDTs
- Kegan Dougal (Element/Matrix) · PaPoC 2026 (Edinburgh), arXiv 2601.22963 · Jan 2026 (v2 Apr 2026)
- https://arxiv.org/abs/2601.22963 [F]
- Summary: Concurrent mutual revocations make permission CRDT views "roll back". A Byzantine admin can exploit concurrency, so the paper argues an external arbiter is needed. ERA adds optional asynchronous "epoch events" from a finality arbiter, which give a bounded total order within epochs, and so a form of finality, without losing availability.
- Relevance: It strongly supports the **C1/C6** design choice of a server-ordered log for authority decisions, since an arbiter is in effect a server, and it frames **S2** server rejection as "finality".

### Keynote: Byzantine Eventual Consistency and Local-First Access Control
- Martin Kleppmann · PaPoC 2025 keynote · 31 Mar 2025
- https://martin.kleppmann.com/2025/03/31/papoc-keynote-byzantine.html [F]
- Summary: The keynote surveys BFT CRDTs and the access control problem in local-first software. It introduces the duelling-admins problem and proposes seniority ranking as one resolution.
- Relevance: It is a citable framing of why access control (**S3**) and Byzantine tolerance are open problems in local-first systems.

### Willow Protocol and Meadowcap
- Aljoscha Meyer, Sam Gwilym (Worm Blossom) · Spec · 2024–2025 (Meadowcap 1.0 [M])
- https://willowprotocol.org/specs/meadowcap/ [F]
- Summary: Meadowcap is a capability system for Willow's namespaced, path-addressed data. Capabilities are delegatable and restricted by subspace, path prefix and time range. Owned namespaces need a root signature, while communal namespaces give each author their own subspace. Willow supports prefix-pruning deletes and is E2EE-friendly (encrypted paths are [M]).
- Relevance: It is a capability-plus-destructive-edit design relevant to **S3** (revocation and deletion propagation) and to C2 blob lifecycle.

### did:crdt: Coordination-Free Decentralised Identifiers via Signed CRDTs
- Hugo O'Connor, Claire Barnes (Anuna Research) · arXiv 2606.16223 · 15 Jun 2026
- https://arxiv.org/abs/2606.16223 [F]
- Summary: A W3C DID method where each DID document field is a signed CRDT. It needs no ledger, sequencer or total order. BFT holds for signed deltas and verified-bundle replay, while the unauthenticated state-merge path is a trusted-domain optimization. A Rust reference implementation exists at codeberg.org/anuna/did-crdt [M].
- Relevance: It is tangential, covering identity and device keys for E2EE mode. It is useful as a counterpoint: coordination-free updates work for identity documents, but **C6** relational invariants need ordering.

### p2panda group and message encryption
- p2panda team (Andreas Dzialocha, Sam Andreae et al.) · Blog plus `p2panda-encryption` crate · 24 Feb 2025
- https://p2panda.org/2025/02/24/group-encryption.html [F]; https://docs.rs/p2panda-encryption/latest/ [F, via search]
- Summary: Two schemes. The first is a "data encryption" scheme with a symmetric group key (XChaCha20-Poly1305) rotated on member removal, giving post-compromise security for new data. The second is a Double-Ratchet-like "message encryption" scheme with forward secrecy. Key agreement runs over 2SM inside a DCGKA-inspired group protocol, and groups and access control are under active development.
- Relevance: It shows that rotate-on-remove is the practical **S3** baseline, under which old data stays readable to the removed member. It is a usable library-level reference for E2EE mode.

### Matrix State Resolution v2, v2.1 and Project Hydra
- Matrix.org (spec); Kegan Dougal et al. (Hydra blog) · Room v2 spec; blog 14 Aug 2025; room version 12
- https://spec.matrix.org/v1.16/rooms/v2/ [F]; https://matrix.org/blog/2025/08/project-hydra-improving-state-res/ [F]
- Summary: Matrix replicates room state (including power levels and ACLs) as an event DAG across federated, mutually distrusting servers, and resolves conflicts deterministically. Hydra fixed "state resets", where a malicious server could revert ACLs (CVE-2025-49090), and weak create-event uniqueness (CVE-2025-54315). The fixes are MSC4289 (privileged creators), MSC4291 (room ID = create-event hash) and MSC4297 (State Res v2.1), shipped as room v12.
- Relevance: It is the largest deployed decentralized ACL CRDT, and a cautionary tale for **S3/S4**: without a total-order authority, permission state can roll back. That supports the server-ordered **C1** choice.

---

## 4. Group key agreement

### Key Agreement for Decentralized Secure Group Messaging with Strong Security Guarantees (DCGKA)
- Matthew Weidner, Martin Kleppmann, Daniel Hugenroth, Alastair R. Beresford · ACM CCS 2021 [M venue]; IACR ePrint 2020/1281 · Nov 2021
- https://eprint.iacr.org/2020/1281 [F]
- Summary: Decentralized continuous group key agreement that works without a server providing total order. It tolerates concurrent membership changes and gives forward secrecy and post-compromise security. Updates cost O(n).
- Relevance: It is the reference for E2EE key management when **C1**'s server order is *not* trusted. In the thesis the server orders anyway, so MLS-style protocols become applicable.

### BeeKEM: Decentralized, Secure and Efficient Group Key Agreement
- Derek Yen, Andrés Fábrega, Liangrun Da, Martin Kleppmann, John Mumm, et al. · IACR ePrint 2026/1434 · 2026
- https://eprint.iacr.org/2026/1434 [F]
- Summary: The first decentralized group key agreement with logarithmic update cost in the common case (linear in the worst case) and security proofs. Its performance is competitive with OpenMLS. It is the key-agreement layer for Keyhive.
- Relevance: It makes E2EE mode for large shared workspaces feasible without trusting server ordering. It is relevant to the **S3** revocation cost and the **C6** E2EE-mode overhead.

### The Messaging Layer Security (MLS) Protocol, and the MLS architecture
- R. Barnes, B. Beurdouche, R. Robert, J. Millican, E. Omara, K. Cohn-Gordon · IETF RFC 9420 · Jul 2023; architecture RFC 9750 · 2025 [M date]
- https://www.rfc-editor.org/rfc/rfc9420 [F]; https://www.rfc-editor.org/rfc/rfc9750 [F]
- Summary: Tree-based (TreeKEM) group key agreement with O(log n) updates, forward secrecy and post-compromise security. It **assumes a Delivery Service that totally orders Commits per epoch**, and its transcript hash binds members to one history, so forks are detectable on the next shared commit.
- Relevance: It maps directly onto the **C1** server-ordered log, so the sync server can act as the MLS Delivery Service. The transcript hash provides **S6** fork detection without trusting the server, and member removal plus a new epoch gives a key-level **S3**.

### MLS AppSync / "A Safe Application Interface to MLS"
- Richard Barnes, Rohan Mahy (00); plus J. Alwen (01) · IETF Internet-Draft draft-barnes-mls-appsync · 00 Jul 2024, 01 Dec 2024
- https://www.ietf.org/archive/id/draft-barnes-mls-appsync-01.html [F]; https://datatracker.ietf.org/doc/html/draft-barnes-mls-appsync-00 [F, via search]; related https://datatracker.ietf.org/doc/draft-ietf-mls-extensions/ (rev -11) [F]
- Summary: Version 00 adds an `application_states` GroupContext extension plus an `AppSync` proposal, so members *agree on application state* as part of the epoch. Version 01 generalizes this to `application_data` components that reuse an MLS group's crypto state safely.
- Relevance: It is a standards-track way to bind replicated app state (or a state hash) to the group transcript. It would give **S6** a consensus on the validator output per epoch, and is close prior art for "validator outcome in the E2EE log".

### The Signal Private Group System
- Melissa Chase, Trevor Perrin, Greg Zaverucha · ACM CCS 2020 [M venue]; ePrint 2019/1416 · 2019–2020
- https://eprint.iacr.org/2019/1416 [F]; blog https://signal.org/blog/signal-private-group-system/ [F, via existing notes]
- Summary: The server stores encrypted group membership and enforces access control over it with anonymous credentials and verifiable encryption. It never learns who is in the group.
- Relevance: It shows that a server *can* validate access control rules on encrypted state using zero-knowledge proofs. This is a third option beside "server validates plaintext" and "clients replay" for **C6/S6**, and is worth one sentence.

---

## 5. Byzantine-tolerant CRDTs and sync

### Making CRDTs Byzantine Fault Tolerant
- Martin Kleppmann · PaPoC 2022 · Apr 2022
- https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf [F]
- Summary: Hash-linked op DAGs, where each op references its predecessors by hash, and a reconciliation that checks validity on every replica. Together these make many CRDTs tolerate any number of Byzantine nodes while keeping strong eventual consistency (SEC).
- Relevance: It is the BFT baseline for **S4** and **C7**. A hash-linked log is also the cheapest fork-detection add-on for **S6**.

### Byzantine Eventual Consistency and the Fundamental Limits of Peer-to-Peer Databases
- Martin Kleppmann, Heidi Howard · arXiv 2012.00472 · Dec 2020
- https://arxiv.org/abs/2012.00472 [F]
- Summary: The paper defines Byzantine eventual consistency (BEC). It shows BEC is achievable for I-confluent invariants with any number of Byzantine nodes, and impossible for non-I-confluent invariants without coordination.
- Relevance: It formally justifies **C6/S2**: relational invariants that are not I-confluent *need* an ordering authority, which here is the server log or validator.

### On CRDTs and Equivocation in Byzantine Setups
- Florian Jacob, Saskia Bayreuther, Hannes Hartenstein (KIT) · arXiv 2109.10554 · Sep 2021
- https://arxiv.org/abs/2109.10554 [F]
- Summary: Equivocation, meaning sending different ops to different peers, is the core Byzantine threat to CRDTs. Hash-chained causal histories (as in Matrix) handle it: equivocated ops become concurrent branches rather than divergence.
- Relevance: It supports **S4** under a malicious client and frames server forking (**S6**) as equivocation by the orderer.

### Bounding Byzantine Impact in Open CRDT Systems
- Authors not verified · PaPoC 2026 per caller brief [M]; abstract seen only via a third-party mirror · 2026
- No primary link found. The only source was an aggregator mirror (https://researchclopedia.vercel.app/research/W7147698200, HTTP 402 on fetch) [M]
- Summary: Per the mirror's abstract, validity checks alone do not stop a Byzantine peer from flooding a permissionless CRDT with well-formed adversarial updates. The proposed "bounded Byzantine CRDTs" attach a cost to each update so that adversarial influence is capped relative to the resources spent.
- Relevance: It is adjacent to rate-limiting and quota in **C6** (Encrypted Spaces also notes billing and rate limits are server-only). Do not cite it until a primary PDF and author list are found.

### Decoupling Trust in Byzantine CRDTs: Fine-grained Post-Compromise Handling without Breaking Causality
- Amos Brocco · arXiv 2606.31759 · 30 Jun 2026 (v2 1 Jul 2026)
- https://arxiv.org/abs/2606.31759 [F]
- Summary: When a previously honest peer is compromised, retroactively excluding its updates can break causal dependents. The paper separates identity trust from content trust and uses deterministic reconstruction, so replicas can selectively include or exclude updates by author and by update semantics under app policies without breaking causality.
- Relevance: It is the closest Byzantine-CRDT analogue to **S2** (causal cascade on rejection). Cite it and contrast it: Brocco re-derives state while keeping dependents, whereas S2 compensates dependents, blobs and claims.

### Proof-carrying CRDTs (Byzantine update validation)
- Nick Marx, Florian Jacob, Hannes Hartenstein (KIT) · PaPoC 2025 · 2025
- https://publikationen.bibliothek.kit.edu/1000180970 [F, via search; record not opened]
- Summary: Updates carry proofs of their own validity, so replicas can check Byzantine updates locally. It is demonstrated on an increment-only counter and a Matrix-like update-history CRDT.
- Relevance: It is an alternative to **S6** full replay: validate each op with an attached proof instead of re-running the validator over the whole log.

### Signal Automatic Key Verification (key transparency)
- Signal Messenger · Blog · 11 Aug 2026
- https://signal.org/blog/automatic-key-verification/ [F]; IETF KT protocol draft https://datatracker.ietf.org/doc/draft-ietf-keytrans-protocol/ [F]
- Summary: The feature is built on key transparency (CONIKS-lineage), with Cloudflare and Trail of Bits as third-party auditors. It detects a compromised key directory swapping a user's key. It is opt-in under Settings > Privacy > Advanced, and does not prove who controls a phone number or username.
- Relevance: It is the deployed pattern for externally audited fork and equivocation detection, a cheap path to making **S6** hold against a malicious server.

---

## Coverage gaps

- **Kintsugi**: the only "Kintsugi" found is a *decentralized E2EE key recovery* protocol (Emilie Ma; https://github.com/kewbish/kintsugi [F]; Cambridge talk https://talks.cam.ac.uk/talk/index/229855 [F, via search]). No Kintsugi access control paper was found. The paper and venue are unverified, so this is relevant only to E2EE-mode key recovery.
- **Bounding Byzantine Impact (PaPoC 2026)**: there is no primary source, author list or PDF. The PaPoC 2026 program page was not reached.
- **SCUBA** full text was not read. Its characterization is secondhand via Encrypted Spaces.
- **Acumen** and **SignalCD** full PDFs were not read. Their fork-handling and ordering assumptions (does a server order?) are unknown, and need checking because both are the strongest 2026 threats to an "E2EE + fork detection" framing of S6.
- **DCGKA venue (CCS 2021)**, the **Signal Private Group System venue (CCS 2020)** and the **RFC 9750 date** are from memory.
- **WhatsApp whitepaper app-state-sync section** was not re-read. The LtHash/MAC mechanism details are [M].
- **Proton Drive** and **Apple ADP** internals (key hierarchy, migration on toggle) are [M].
- No paper was found on **revocation with a time bound on cached replicas (S3)**, on **blob or attachment security in E2EE sync (C2 under E)**, or on **mode-switchable plaintext/E2EE with validator equivalence (S6)**. These absences support the novelty claims but are not proof.
- Searches not run for lack of budget: NDSS and IEEE S&P 2024–2026 for "encrypted CRDT", Jazz/Evolu security write-ups, Automerge "auth" RFCs, Tamarin/ProVerif analyses of Keyhive, and Hypercore/Autobase multi-writer fork handling.
