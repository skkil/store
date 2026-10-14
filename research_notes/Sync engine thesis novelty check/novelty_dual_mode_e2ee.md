# Novelty check: one deterministic validator, two trust modes (server-validated plaintext vs client-replayed E2EE)

As of 2026-11-04. Tags: [F] = fetched/read during this session; [M] = from model memory, not re-verified this session.

The claim under test is from `reports/Local first sync engine research.md`, section "E2EE is opt-in: one protocol, two trust modes". The server always assigns a per-user total order. One shared deterministic validator (cycles, bounded counts, recurrence) runs on the server in standard mode and on every client by replaying the log in E2EE mode. Structural checks (references, leases, quotas, idempotency) stay on the server and work on metadata. Switching modes means migrating the log. Invariant S6: every replica that has processed the same log prefix holds the same state and makes the same accept/reject decisions. The report says "none of the surveyed products has this design".

## Q1: Does academic prior art already do "server orders an encrypted log, clients validate deterministically by replay"?

### Takeaway
Yes. This core mechanism is well established. SPORC (2010) has it, including a dual role in which an honest server rejects early on plaintext metadata while every client re-checks by replay. Bayou (1995) has deterministic, app-defined validation over a primary-ordered log. SCUBA (ACSAC 2025) runs app-defined write validators on the client over an E2EE log. Encrypted Spaces (June 2026) runs the same `apply` function, including app-declared access-control predicates and "actions" with assertions, on the server, on the client and in a zkVM, so that all three "reach the same decision". The academic picture is therefore close to fully covered.

### Cited Findings
- **SPORC (Feldman, Zeller, Freedman, Felten, OSDI 2010) [F].** The server's "sole purpose is to order and store client-generated operations". Document-op payloads "are encrypted under a symmetric key … unknown to the server". Meta-operations (ACL changes) are sent in the clear. — [SPORC paper](https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/sporc.pdf)
- **SPORC already has the dual-validation pattern [F].** "Because ModifyUserOps are not encrypted, a non-malicious server will immediately reject operations from users with insufficient privileges. But because the server is untrusted, every client maintains its own copy of the ACL, based on the history's ModifyUserOps, and refuses to apply operations that came from unauthorized users." This is the report's "server verdict is an early answer, client replay is the authority" idea, applied to the ACL. — [SPORC paper](https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/sporc.pdf)
- **SPORC also has [F]:**
  - a hash chain over the committed history, which clients use to validate incoming ops;
  - fork* consistency, so a server can fork clients but cannot join them back undetected, plus OT-based recovery after a fork;
  - "barrier" operations that the server enforces and clients verify;
  - a "local coherence" invariant: local state = in-order application of the committed history + the pending queue. This is the same idea as S6.

  — [SPORC paper](https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/sporc.pdf); [search summary of fork recovery](https://static.usenix.org/events/osdi10/tech/slides/feldman.pdf)
- **SPORC motivates its central server with a business invariant [F].** The example is meeting-room reservation: a user needs "timely notification that her operation has been committed and will not be overridden". — [SPORC paper](https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/sporc.pdf)
- **Bayou (Terry et al., SOSP 1995) [F, via lecture notes].**
  - Every write carries a dependency check (query + expected result) and a merge procedure. Both must be deterministic and may not use time-dependent or machine-specific information.
  - A primary assigns commit sequence numbers.
  - Replicas roll back and replay in commit order; "the log holds the truth; the DB is just an optimization".

  This is deterministic app-level invariant validation by replay of a server-ordered log, without encryption. — [Bayou notes (Whittaker)](https://mwhittaker.github.io/papers/html/terry1995managing.html); [Princeton COS418 slides](https://www.cs.princeton.edu/courses/archive/fall18/cos418/docs/L5-bayou.pdf)
- **SCUBA (Popescu et al., ACSAC 2025) [F, via the Encrypted Spaces related-work section + venue metadata].**
  - An E2EE sync-engine-like middleware with strong consistency under a Byzantine server.
  - "a key-value store supporting application-specific functions to validate writes, and these are executed on the client".
  - Clients can prove server misbehavior to third parties.
  - Store-and-forward: data is not persisted on the server.

  — [SCUBA PDF](https://www.cs.princeton.edu/~npopescu/papers/scuba.pdf); [researchr record](https://researchr.org/publication/PopescuCSCL25); [Encrypted Spaces whitepaper §related work](https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf)
- **Encrypted Spaces (Harvard ASML, Trapp, Perrin et al., whitepaper 2026-06-11) [F]. This is the closest prior art.**
  - "A space is a transition system: a deterministic function maps an authenticated state and a signed change to a new state … The same apply function is executed in three settings": Server, fast-forward zkVM prover, and Client.
  - The server orders changes.
  - Access-control predicates "are evaluated inside apply against authenticated state, so the server, the per-change client verifier, and the fast-forward zkVM all reach the same decision".
  - Apps define "actions" with "declarative assertions over the change and the current state". The examples are business rules: authors-only delete, "prevent removing a non-empty directory", "fix a completed task's timestamp".
  - "Native extensions" cover "cross-table invariants the assertion grammar cannot capture".

  — [Encrypted Spaces whitepaper](https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf); [ASML project page](https://asml.cyber.harvard.edu/encrypted-spaces/)
- **Encrypted Spaces has a per-column plaintext/encrypted switch [F].** Columns are encrypted by default. `plaintext=#true` allows server-side indexing and filtering. Its limitation: verifying encrypted column values, "proving that an encrypted Number … is a valid integer, to prevent malicious users from breaking client-side formulas", is listed as *future work*. Encrypting structural fields "would require client-side proofs". In other words, its server-side assertions effectively run over plaintext/structural state, and invariants over encrypted content are not verified. — [Encrypted Spaces whitepaper §4, §Future Directions](https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf)
- **Encrypted Spaces fork resistance [F].** It relies on a transparency signer with consistency proofs. The authors say this is not fully implemented. Some checks can be "enforced only on the server and not be verifiable", for example rate-limiting by billing policy. This matches the report's "structural checks stay server-side" split. — [Encrypted Spaces whitepaper §6, §threat model](https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf)
- **Kleppmann, "Making CRDTs Byzantine Fault Tolerant" (PaPoC 2022) [F, abstract-level].**
  - Hash-linked events tolerate any number of Byzantine nodes and give strong eventual consistency.
  - CRDTs with semantic constraints can be violated by malicious nodes unless every replica checks them.
  - Kleppmann & Howard 2020 tie what is achievable to invariant confluence.

  — [Kleppmann PaPoC 2022](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf); [Byzantine Eventual Consistency, arXiv 2012.00472](https://arxiv.org/pdf/2012.00472)
- **Matrix MSC2801 [F, snippet].** "any client implementing E2EE must be prepared to deal with any encrypted content, since by definition a server cannot validate it". — [MSC2801](https://github.com/matrix-org/matrix-spec-proposals/blob/main/proposals/2801-untrusted-event-data.md)
- **Signal Private Group System (Chase, Perrin, Zaverucha, CCS 2020) [F].** The server enforces access control over *encrypted* membership entries using zero-knowledge proofs. This is an alternative to client-side replay for structural rules. — [Signal blog](https://signal.org/blog/signal-private-group-system/); [ePrint 2019/1416](https://eprint.iacr.org/2019/1416)
- **Other systems in the same lineage [M]:**
  - SUNDR (OSDI 2004): fork consistency for untrusted file servers.
  - Depot (OSDI 2010): fork-join-causal consistency; tolerates faulty clients and servers.
  - Frientegrity (USENIX Security 2012): untrusted social-network server; clients verify object histories.
  - Venus (CCSW 2010): verification for untrusted cloud storage.
  - CryptPad ChainPad: the server orders encrypted patches and clients validate and apply them.
  - Rollups (Ethereum L2): a sequencer orders transactions; anyone can deterministically replay them and reject or prove invalid state transitions (fraud/validity proofs). This is the exact "untrusted sequencer + deterministic replay validator" structure.
- **Keyhive/Beelay (Ink & Switch, 2024–2025) [F, secondary].** Auth-enabled sync over encrypted data. The sync server checks capabilities and audiences on metadata without reading content. Pre-alpha. — [Keyhive notebook](https://www.inkandswitch.com/keyhive/notebook/05/); [Keyhive README mirror](https://github-users.yhajoa.workers.dev/inkandswitch/keyhive/blob/main/README.md)

### Inferences
- The E2EE-mode half of the claim (server orders ciphertext; clients enforce rules by deterministic replay; the server enforces only metadata/structural rules) is prior art from SPORC (2010) onward. Bayou (1995) covers deterministic app-level invariant checks by replay.
- The "same validator, server and client reach the same decision" half is prior art in Encrypted Spaces (2026), in SPORC for ACLs, and in rollups generally.
- Encrypted Spaces leaves one gap: enforcing invariants over *encrypted content* by replay on clients. But that is exactly what SPORC- and SCUBA-style client validators do, so the gap is filled by older work rather than open.

### Gaps
- I did not read the SCUBA full text. The characterization of client-side validators comes from the Encrypted Spaces related-work section.
- I did not re-fetch Depot, Frientegrity, Venus or ChainPad [M]. I found no 2022–2026 "Secure CRDT" (Barbosa et al.) or Snapdoc text this session.

## Q2: Do products already ship this (LiveStore, Jazz, Evolu, Actual, etc.)?

### Takeaway
No surveyed product ships *both* modes with one validator. Products split into two groups. One group is plaintext servers that run imperative or server-side validation (Zero, PowerSync, Electric, LiveStore's backend ordering). The other is E2EE systems that do client-side signature/permission checks only, with LWW/CRDT merges and no business invariants (Jazz, Evolu, Actual, Keyhive). Actual Budget is a real precedent for "opt-in E2EE, same sync protocol", but it has no validator in either mode.

### Cited Findings
- **LiveStore [F].** "The sync backend acts as the global authority and determines the total order of events". LiveStore "doesn't yet support encryption but might in the future". The suggested workaround is to encrypt events yourself via a custom Effect Schema. I found no documented server-side event validation or rejection. So LiveStore already provides "server total order + deterministic client rematerialization", but has no E2EE mode and no validator. — [LiveStore syncing docs](https://docs.livestore.dev/building-with-livestore/syncing/); [LiveStore encryption pattern](https://docs.livestore.dev/patterns/encryption)
- **Jazz [F].**
  - Each transaction is signed.
  - "Whether transactions are valid depends on your permissions in the Group that owns the CoValue".
  - Content is encrypted with the Group read key.
  - The pitch: "Invisible to servers, verifiable by anyone".

  Validation is cryptographic permission checking only, not business invariants. I could not confirm whether the server validates. — [Jazz encryption docs](https://jazz.tools/docs/react/reference/encryption)
- **Actual Budget [F].** "E2EE is optional and needs a second password", and "the server will no longer be able to access your budget information". This is an opt-in E2EE mode on the same sync protocol. The server does no semantic validation; merges are CRDT/LWW [M]. — [Actual sync docs](https://actualbudget.org/docs/getting-started/sync/)
- **Evolu [M].** Always-on E2EE, SQLite, LWW CRDT. The relay stores ciphertext. There is no plaintext mode and no invariant validator. My search returned no Evolu docs.
- **Open-source design that matches E2EE mode exactly (sens-o-matic, issue #61, opened 2026-10-25) [F]:**
  - The server "gives a sequence number to each log, stores ciphertext and orders it".
  - "The server cannot judge an event it cannot read".
  - Matching, lifecycle and validation run on clients from "one deterministic Rust crate".
  - "an event that breaks a rule is ignored by every client the same way"; the first valid event wins.
  - The rule version is pinned per entity.
  - A hash chain or MLS transcript hash is used for fork detection.

  It explicitly *migrates away* from plaintext server logic and keeps no plaintext mode. — [sens-o-matic #61](https://github.com/n8behavior/sens-o-matic/issues/61)
- **Encrypted Spaces [F].** A preview library, not a product, released June 2026 (WIRED coverage). Matt Green called it "the Signal protocol for collaboration apps". — [Harvard Cyber page](https://cyber.harvard.edu/story/2026-06/encrypted-spaces); [aggregated WIRED summary](https://discuss.privacyguides.net/t/signal-alums-reveal-encrypted-spaces-a-system-for-making-private-collaboration-apps/38492)
- **Not checked this session [M]:** Anytype any-sync, Etebase, Standard Notes, Notesnook, Proton, Tresorit, Ente, Bitwarden, 1Password, Fireproof, Ditto, Willow/Meadowcap, p2panda, Earthstar, Iroh. From memory, these are either always-E2EE with client-side merge (no shared server/client validator) or plaintext-server systems. None is known to offer a server-validated/client-replayed mode switch. Fireproof advertises a verifiable, encrypted Merkle-CRDT ledger, but its validation is integrity, not business invariants [M].

### Inferences
- The report's sentence "none of the surveyed products has this design" holds for products. It does not hold for the literature (SPORC, SCUBA, Encrypted Spaces), nor for a 2026 open-source design (sens-o-matic) that covers the E2EE-mode half.

### Gaps
- I did not verify whether the Jazz sync server rejects invalid transactions (`cojson/permissions.ts` is unread).
- I did not check whether Replicache/Zero or PowerSync have published E2EE guidance in 2026.

## Q3: Is "opt-in E2EE with identical semantics, switching between server-validated and client-replayed validation" documented anywhere?

### Takeaway
Opt-in E2EE with *feature loss* is well documented (Apple ADP, Actual Budget). "Same semantics across modes, with validation relocated from server to clients" is not documented as a named, switchable design anywhere I found. The closest relative is Encrypted Spaces' per-column plaintext/encrypted schema, which is a static per-field choice and not a per-user runtime mode with a log migration.

### Cited Findings
- **Apple Advanced Data Protection [F].** When ADP is enabled, iCloud.com web access is turned off because "iCloud web servers no longer have access to the keys required to decrypt". It can be re-enabled with per-session approval from a trusted device. Some data is never on the web. This is a precedent for "server-side features disabled or run by a designated device". — [Apple support 102630](https://support.apple.com/102630); [Apple Platform Security guide](https://support.apple.com/en-ge/guide/security-pdf/sec973254c5f/web)
- **Apple ADP migration [M].** Turning ADP on re-keys protected categories and removes the service keys from Apple. Turning it off uploads the keys back. This is analogous to "mode switch = migration".
- **Actual Budget [F].** Optional E2EE on the same sync server, where the server loses access. — [Actual sync docs](https://actualbudget.org/docs/getting-started/sync/)
- **Encrypted Spaces [F].** Schema-level `plaintext=#true` per column trades server queryability against confidentiality. The same `apply` runs everywhere, but only over what each party can see. — [Encrypted Spaces whitepaper](https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf)

### Inferences
- The remaining novel sub-claim is narrow. It is a per-user (or per-scope) runtime switch between:
  - (a) a trusted plaintext server running validator V as an early, authoritative replica;
  - (b) clients running the *same* V by replay, with the server reduced to ordering plus metadata checks.

  It also requires an explicit equivalence theorem/test (S6: identical state and accept/reject decisions for the same prefix, in both modes) and a defined log migration between modes, applied to personal relational data with non-I-confluent invariants (cycles, counts, recurrence). I found no prior art that frames the *equivalence across the two trust modes plus the migration* as the contribution.
- Encrypted Spaces could be read as subsuming this, because every column can be plaintext or encrypted and `apply` is shared. The distinction must be stated explicitly:
  - Encrypted Spaces' server never *trusts itself* to validate encrypted-content invariants, and listed that as future work.
  - Encrypted Spaces has no runtime per-user mode switch or migration.
  - The thesis's standard mode treats the server as a trusted plaintext replica.

### Gaps
- I did not check Google's opt-in E2EE products (Google Messages RCS, Google Drive client-side encryption) for documented feature loss. My search returned nothing relevant.

## Q4: Verdict and risks

### Takeaway
**Verdict: partially novel, and the novel part is narrow.** "One deterministic validator on server or clients over a server-ordered (encrypted) log" is done: SPORC 2010, Bayou 1995, SCUBA 2025, Encrypted Spaces 2026, rollups. What remains is the dual-mode system property: mode-switchable placement of the same validator, the S6 cross-mode equivalence as a tested invariant (deterministic simulation: N clients + server), and log migration between modes, for relational personal-app invariants. This works as an engineering/systems contribution with simulation evidence. It is not a new mechanism. The thesis should cite SPORC, Bayou, SCUBA and Encrypted Spaces as the foundation and claim only the delta.

### Cited Findings
- **Fork attacks.** In E2EE mode a malicious server can fork clients. SPORC uses hash chains + fork* consistency. Encrypted Spaces needs a transparency signer and calls it unimplemented. sens-o-matic notes forks in quiet groups may go undetected. The report's E2EE design does not mention hash chains or fork detection, so S6 holds only under an honest-ordering server. — [SPORC](https://www.cs.princeton.edu/courses/archive/fall15/cos518/papers/sporc.pdf); [Encrypted Spaces §6](https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf); [sens-o-matic #61](https://github.com/n8behavior/sens-o-matic/issues/61)
- **Validator versioning.** sens-o-matic pins the rule version per entity because clients on different versions derive different states. Bayou forbids time- and machine-dependent inputs. Both match the report's "determinism discipline". — [sens-o-matic #61](https://github.com/n8behavior/sens-o-matic/issues/61); [Bayou notes](https://mwhittaker.github.io/papers/html/terry1995managing.html)
- **Malicious clients.**
  - In E2EE mode the server cannot reject a semantically invalid op. All honest clients ignore it deterministically, but it still occupies log, storage and quota.
  - Encrypted Spaces notes malicious users can break client-side logic with malformed encrypted values.
  - Kleppmann notes semantic constraints can be violated by Byzantine nodes unless checked at every replica.

  — [Encrypted Spaces Future Directions](https://encryptedspaces.org/whitepapers/encrypted-spaces.pdf); [Kleppmann PaPoC 2022](https://martin.kleppmann.com/papers/bft-crdt-papoc22.pdf)

### Inferences
- **Further risks I identified (my own analysis):**
  - **Rejection latency.** The time to reject rises from upload-time to next-pull, which changes rejection UX and may require compensation of optimistic local effects.
  - **Equivocation.** A malicious client can sign an op that the validator rejects and still collect side effects off-log (for example, blob uploads). Leases and quotas on metadata mitigate this.
  - **Standard-mode trust.** In standard mode the server's verdict is authoritative. If server and client validator builds diverge (version skew, non-determinism in a native vs WASM build such as float or sort order), S6 breaks silently. The report's suggestion to keep client replay as a self-check is the right mitigation and should be a required test.
  - **Migration atomicity.** A snapshot plus the future log must hold a single cut point, or decisions straddling the cut differ.
- **Positioning for the thesis.** Claim: "We show that validation placement can be a runtime trust-mode parameter, not an architectural fork: a server-ordered log plus a deterministic validator yields identical decisions whether run by a trusted plaintext server or by E2EE clients (S6), verified by deterministic simulation, with a defined migration between modes." Cite SPORC, Bayou, SCUBA and Encrypted Spaces as foundations. Do not claim the mechanism itself.

### Gaps
- I found no published measurement of determinism failures across native/WASM validator builds.
- I found no prior art that specifically tests cross-mode equivalence. This is an absence of evidence, so a dedicated arXiv sweep for 2025–2026 "E2EE sync engine" papers is advisable before the thesis is submitted.
