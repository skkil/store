# Semantics and Abstraction: How Much of Local-Cloud Sync Can a Generic Engine Own?

Research date: 2026-11-04. Sources were gathered by web search and fetched docs during this session. Items marked "(not re-verified this session)" come from well-known canonical sources that I did not re-fetch. Treat them as leads to confirm.

## Q1. Which dimensions must an application declare to a generic sync engine?

### Takeaway
The mechanics are generic: transport, change capture, local storage, causality tracking, the op log and rebase, CRDT merge of standard types, and subscription bookkeeping. What the application must declare comes down to about ten parameters:
1. data model and type per field
2. merge rule per field
3. invariants, and the consistency class they force on each operation
4. authority
5. partial-replication scope
6. authorization
7. schema-version lenses
8. deletion and retention
9. side-effect policy
10. conflict-surfacing policy

The theory says exactly when an operation can be merged without coordination: I-confluence, CALM, and the 2026 "future-monotone" Coordination Criterion. In principle, then, an engine could classify operations automatically if the app declares its invariants.

### Cited Findings

**Consistency level per operation (the central dimension)**
- I-confluence: an operation set needs no coordination for an invariant if and only if merging independently valid replica states always preserves that invariant. The condition is necessary and sufficient for coordination-free, available, convergent execution. Examples: per-record inequality constraints are I-confluent for insert, update, and delete. Foreign keys are I-confluent for deletes but not for inserts or updates. A "greater than" threshold holds for counter increments but not decrements. 10 of the 12 TPC-C invariants are I-confluent. Bailis et al., VLDB 2015 — [arXiv 1402.2237](https://arxiv.org/pdf/1402.2237); [Bailis blog](https://www.bailis.org/blog/when-does-consistency-require-coordination/); [Morning Paper summary](https://blog.acolyer.org/2015/03/19/coordination-avoidance-in-database-systems/)
- Segmented invariant confluence extends this so that non-I-confluent objects can be replicated with only a small amount of coordination — [par.nsf.gov](https://par.nsf.gov/servlets/purl/10221255)
- CALM theorem: a program has a consistent, coordination-free distributed implementation if and only if it is monotonic. Negation and universal quantification are the non-monotonic features. Hellerstein & Alvaro 2019; formalized by Ameloot, Neven, Van den Bussche — [arXiv 1901.01930](https://arxiv.org/abs/1901.01930v2); [RISE overview](https://rise.cs.berkeley.edu/blog/an-overview-of-the-calm-theorem/)
- Hellerstein, "The Coordination Criterion" (2026): a specification admits a coordination-free implementation if and only if it is "future-monotone" — [arXiv 2602.09435](https://arxiv.org/pdf/2602.09435v2)
- Escrow / Bounded Counter (Balegas et al.): the slack between a counter's value and its bound is split among replicas. Each replica can decrement locally until its share runs out, and no master is needed. Antidote ships this as a data type. The developer sets only the bound, the initial value, and each replica's initial share. Replicas can "donate" rights to each other ahead of need. The approach generalizes to invariants that span multiple CRDTs — [arXiv 1503.09052](https://ar5iv.arxiv.org/html/1503.09052); [Just-Right Consistency, arXiv 1801.06340](https://arxiv.org/pdf/1801.06340); [Shapiro AntidoteDB slides](https://speakerdeck.com/marcshapiro/antidotedb-a-developer-friendly-geo-replicated-crdt-data-store)
- ECROs (De Porre, Ferreira, Preguiça, Gonzalez Boix; OOPSLA 2021): the developer writes sequential code plus a distributed specification of invariants. The system statically finds conflicting operations and makes them commute where it can, for example by reordering concurrent add/remove locally instead of coordinating as Hamsaz does. Performance is similar to CRDTs for commutative operations and better than prior work for non-commutative ones. Follow-up work (EFx) aims to remove the hand-written first-order-logic specification, which the authors call hard to write — [OOPSLA'21 PDF](https://asc.di.fct.unl.pt/~nmp/pubs/oopsla-2021.pdf); [AMiner](https://aminer.org/pub/6183bf2c5244ab9dcbb695fd/ecros-building-global-scale-systems-from-sequential-code); [VUB portal](https://researchportal.vub.be/en/publications/concurrency-contracts-for-designing-highly-available-replicated-d/)
- Gallifrey (Milano, Recto, Magrino, Myers; Cornell): a "restriction" narrows a sequential object's interface and supplies a merge function for concurrent use of the allowed methods. An object's restrictions can change over time. A type system ensures replicated fields are reached only through a valid restriction. Restrictions can declare "provisional" operations that need not commute — [core.ac.uk PDF](https://core.ac.uk/download/225000489.pdf)

**Per-type / per-field merge semantics**
- Katara (Laddad, Power, Milano, Cheung, Hellerstein; OOPSLA 2022, Hydro project): synthesizes verified state-based CRDTs from sequential data types. The only annotation the user writes is the order in which conflicting operations should appear to occur. For example, "remove before concurrent add" yields an add-wins set — [arXiv 2205.12425](https://arxiv.org/pdf/2205.12425)
- MRDTs (Kaki et al., OOPSLA 2019): ordinary data types plus a three-way merge, implemented in Quark (OCaml, Git-like infrastructure). The PLDI 2022 "Certified MRDTs" work (Peepul, F*) verifies them, with extraction to Irmin — [Morning Paper Part I](https://blog.acolyer.org/2019/11/25/mergeable-replicated-data-types-part-i/); [arXiv 2203.14518](https://ar5iv.labs.arxiv.org/html/2203.14518)
- Collabs (Weidner et al., CMU, 2022): a library of composable collaborative data structures. App developers can add custom CRDTs and compose them, for example with semidirect products (ICFP 2020). Performance is comparable to state-of-the-art web CRDT libraries, with 100 simultaneous users at low latency — [arXiv 2212.02618 via DeepAI](https://deepai.org/publication/collabs-composable-collaborative-data-structures); [docs](https://collabs.readthedocs.io/)
- PowerSync's default write semantics are per-field last-write-wins, and deletes always win. Anything else, such as subtracting inventory quantities instead of overwriting them, is custom backend code — [PowerSync: handling update conflicts](https://docs.powersync.com/handling-writes/handling-update-conflicts); [custom conflict resolution](https://docs.powersync.com/handling-writes/custom-conflict-resolution)

**Authority model**
- Zero mutators run twice. They run first optimistically on the client datastore, then on the server inside a DB transaction, and "the server's result is authoritative." Pending local effects are rolled back once the server applies them. The server copy may differ from the client copy, for example by adding permission checks or calling external systems — [Zero docs: mutators](https://zero.rocicorp.dev/docs/mutators.md)
- Weidner's "Architectures for Central Server Collaboration" (June 2024) describes "server reconciliation," the model used by Replicache and Zero. It is event sourcing with server-ordered replay and does not need CRDT algebraic laws. The post covers ordering options: timestamps, topological sort, CRDT list orders, and business rules. It contrasts this model with decentralized, CRDT-based approaches. (Seen only via secondary summaries; the original is at mattweidner.com) — [aggregator](https://searchengineworld.com/energy/Architectures_for_Central_Server_Collaboration); [localfirst.fm #14](https://localfirst-fm.vercel.app/14/transcript)

**Partial replication (scope)**
- PowerSync Sync Streams are server-defined SQL queries that clients subscribe to on demand. They replace Sync Rules, which are deprecated. They require Service v1.20+, and the beta shipped in March 2026. A release note says they are GA, but the docs still say "Beta" (conflict as of this date). Auto-migrating complex Sync Rules "still has kinks" (Feb 2026 changelog). v1.22.0 (2026-06-04) fixed Sync Streams semantics such as division by zero — [releases](https://releases.powersync.com/announcements/ann_7UgfQuNwRmpQk); [docs](https://docs.powersync.com/sync/streams); [Feb 2026 changelog](https://powersync.com/blog/powersync-changelog-february-2026)
- CVE-2026-30870 (PowerSync 1.20.0, edition 3): certain subquery filters were ignored when deciding which data to sync to users. It was fixed in 1.20.1. This is a concrete case where a bug in the declarative scope language became a data leak — [vuln.today](https://vuln.today/cve/CVE-2026-30870)
- Electric's shapes are read-path only. Writes go through the app's own API, which returns a txid so the client knows when its write has synced back. Shapes should be defined server-side, not as client-supplied WHERE clauses. Offline writes need another tool. (Secondary sources; no official built-in write path found) — [electric shapes guide](https://electric-sql.com/docs/guides/shapes); [write patterns blog](https://telemetry-landing.pages.dev/blog/write-patterns-for-electricsql/)

**Authorization and offline writes**
- Couchbase Sync Gateway: a single JavaScript "sync function" both routes documents to channels (scope) and validates writes with `requireAccess` / `requireUser` / `requireRole`. The recommended pattern also checks `oldDoc.channels`. After access is revoked, local changes are still pushed but rejected. Admin REST writes bypass all checks — [requireAccess](https://docs.couchbase.com/sync-gateway/current/sync-function-api-require-access-cmd.html); [access control concepts](https://docs.couchbase.com/sync-gateway/current/access-control-concepts.html)
- Realm / Atlas Device Sync: a client can write only within the intersection of the sync config, the permissions, and its Flexible Sync subscription query. A write outside that intersection is reverted by the server with a non-fatal "compensating write" error, `ErrorCompensatingWrite` / `CompensatingWriteInfo` — [MongoDB docs](https://www.mongodb.com/docs/realm/sdk/swift/sync/write-to-synced-realm/). (Atlas Device Sync's announced end of life, Sept 2025, was not re-verified this session.)
- Zero has deprecated its declarative row-based permissions in favor of plain code inside mutators: "there is no need for a special permissions system." Credentials must come from server `ctx`, not from mutator arguments. A 401 or 403 puts the client in a needs-auth state, and queued mutations are retried after reconnect — [Zero docs: mutators](https://zero.rocicorp.dev/docs/mutators.md); [Zero permissions (deprecated)](https://zero.rocicorp.dev/docs/debug/permissions)
- Jazz: every CoValue has an owning Group with reader, writer, and admin roles. Groups nest and roles are inherited. Holding a reference does not grant access. `withPermissions` on a schema sets default ownership. Enforcement is cryptographic, so the data carries the permissions — [Jazz groups](https://jazz.tools/docs/react/groups/intro); [cascading permissions](https://classic.jazz.tools/docs/react/permissions-and-sharing/cascading-permissions)
- Keyhive (Ink & Switch, formerly "Authomerge"): local-first systems have no network boundary, so access rules must travel with the data. It is built on capabilities and delegation, and concurrent membership changes create hard edge cases — [Keyhive notebook 01](https://www.inkandswitch.com/keyhive/notebook/01/)

**Schema evolution**
- Cambria (Litt, van Hardenberg, Henry; PaPoC 2021): bidirectional edit lenses translate writes between schema versions so old and new clients can coexist. Schema migration is "a lot harder" local-first than on a server — [Cambria](https://inkandswitch.com/cambria); [MIT DSpace paper](https://dspace.mit.edu/handle/1721.1/145983); [localfirst.fm #3](https://localfirst-fm.vercel.app/3/transcript). The panproto-lens crate cites Cambria's combinators as an influence — [docs.rs](https://docs.rs/panproto-lens/0.52.0/src/panproto_lens/lib.rs.html)

**Side effects**
- Zero recommends a transactional outbox for notifications and async work. Post-commit async tasks can't block the response. Work outside the DB, such as email, is not rolled back on `retryOn`, so only idempotent mutators should be retried — [Zero docs](https://zero.rocicorp.dev/docs/mutators.md)
- PowerSync requires idempotent backend operations because a write can be delivered more than once. If `complete()` is never called, the upload queue blocks. A dead-letter queue risks out-of-order updates — [PowerSync handling writes](https://docs.powersync.com/handling-writes/); [write validation errors](https://docs.powersync.com/handling-writes/handling-write-validation-errors)

### Inferences
- Proposed declaration surface for a generic engine. Each item maps to a cited precedent:
  1. **Collection/type schema**: rows, documents, trees, text, counters, blobs (Jazz, Collabs).
  2. **Per-field merge policy**: picked from a CRDT library (LWW, MV-register, OR-set, PN-counter, sequence), or a Katara-style "conflict order" annotation, or a custom MRDT 3-way merge.
  3. **Invariants**: the engine classifies each mutation as local-safe, escrow-able (bounded counter or reservation), or coordination-required, using I-confluence/CALM-style analysis (ECROs, Gallifrey "provisional" ops).
  4. **Authority per operation**: optimistic plus server-authoritative (Zero), or peer-merged (CRDT).
  5. **Scope**: server-side parameterized queries or channels (Sync Streams, shapes, channels, Flexible Sync).
  6. **Authorization predicate** over (user, old row, new row) (Couchbase), plus a rejection policy (compensating write, rollback, or quarantine).
  7. **Schema versions plus lenses** (Cambria).
  8. **Deletion and retention**: delete-wins, tombstone TTL, GC horizon.
  9. **Side-effect hooks**: outbox, fired only after authoritative commit, with idempotency keys.
  10. **Conflict-surfacing policy**: silent, notify, or keep both values (MV-register) for the UI.
- An engine that wants both enterprise and collaboration support likely needs two merge regimes behind one API. Rows and business entities go through server-reconciled mutators. Rich text and trees go through CRDTs, which the server stores and relays opaquely or merges.
- Ordering and causality, undo/intent, and offline transactions are mostly engine-generic once authority is fixed. Server reconciliation gives a total order for free. CRDT mode needs causal delivery plus app-declared intent, for example "move" versus "delete+insert" for trees.

### Gaps
- No primary source fetched for typed CRDT schema efforts in Automerge or Loro (e.g., Automerge schema/Patchwork work, Loro's typed containers); not covered this session.
- Lasp, Bloom/Hydro (as a sync DSL), Legion, Cure/Antidote transactional semantics, Verdi, and Elm-style approaches were not individually re-fetched. Only CALM, Katara, and Antidote's bounded counter were covered.
- Firestore security rules' behavior on offline writes that are later rejected (silent rollback in the local cache) was not re-verified this session.
- Zanzibar-style relationship authz (OpenFGA/SpiceDB) applied to sync scope: I found no primary source tying it to a sync engine.
- Tombstone GC and retention design across these systems was not researched in depth.

## Q2. Which prior attempts at declarative sync specs exist, and how far did they get?

### Takeaway
Industry has converged on declarative read scope (SQL queries, shapes, channels) and moved away from declarative write semantics, toward imperative server code. Zero deprecated declarative permissions. Electric dropped its bidirectional CRDT design. PowerSync leaves conflict resolution to the backend. Academia (ECROs, Gallifrey, Katara, Bounded Counters) has declarative invariant and merge specs, but these have not been productized in mainstream sync engines.

### Cited Findings
- PowerSync: declarative scope (Sync Streams SQL) plus imperative writes (`uploadData()` to the app's API; LWW by default) — [docs](https://docs.powersync.com/sync/streams); [handling writes](https://docs.powersync.com/handling-writes/)
- Zero: declarative queries, imperative two-sided mutators, and permissions as code. The declarative permissions API is "deprecated... will be removed" — [Zero permissions](https://zero.rocicorp.dev/docs/debug/permissions); [mutators](https://zero.rocicorp.dev/docs/mutators.md). Custom mutators arrived in Zero 0.18 — [release notes](https://zero-docs-13jwy0f9l-rocicorp.vercel.app/docs/release-notes)
- Electric: declarative read shapes only. The mid-2024 rewrite dropped the earlier CRDT/bidirectional SQLite design — [shapes guide](https://electric-sql.com/docs/guides/shapes); [PowerSync comparison (vendor-biased)](https://powersync.com/blog/electricsql-electric-next-vs-powersync)
- Couchbase Sync Gateway: one imperative JavaScript function covers routing, access grants, and write validation. It is the oldest example of "scope plus authz in one spec" — [Sync Function API](https://docs.couchbase.com/sync-gateway/current/sync-function-api.html)
- Realm Flexible Sync: declarative subscription queries plus permission expressions, with server-side compensating writes on violation — [MongoDB docs](https://www.mongodb.com/docs/realm/sdk/swift/sync/write-to-synced-realm/)
- Jazz: declarative schema plus group-based permissions embedded in the data model — [Jazz groups](https://jazz.tools/docs/react/groups/intro)
- Academic: ECROs (invariant spec, then automatic commutativity), Gallifrey (restrictions as types), Katara (conflict-order annotation, then synthesized CRDT), MRDTs (3-way merge), Bounded Counter (bound plus share) — sources in Q1.

### Inferences
- The pattern across products is that declarative specs work for **what each client sees**. For **what a write means**, products fall back to code, because business validation needs arbitrary logic. A thesis contribution could be bridging this gap: declarative invariants plus merge types, compiled to a client-optimistic and server-authoritative mutator, or to an escrow token.
- A declarative scope language is security-critical. The PowerSync CVE shows that errors in its semantics leak data, so it needs tests or verification.

### Gaps
- Hasura/GraphQL-based offline sync and Firestore rules were not fetched; their exact current state is not established here.
- I found no published comparison that benchmarks the expressiveness of these specs against each other.

## Q3. Event sourcing vs state-based abstraction; engine-as-database vs middleware over existing databases

### Takeaway
Server reconciliation (an op/event log of named mutations replayed and rebased against authoritative server state) is the most general abstraction for enterprise apps. It needs no algebraic laws, and the server can apply arbitrary business logic. State/CRDT merge is needed for peer and rich-collaboration data. The market has shifted to middleware over existing Postgres: PowerSync, Electric, and Zero all replicate from Postgres and write back through app code.

### Cited Findings
- Server reconciliation, as used by Replicache and Zero, sidesteps CRDT algebraic rules and lets the server impose ordering by business rules — [Weidner, via aggregator](https://searchengineworld.com/energy/Architectures_for_Central_Server_Collaboration); [Weidner slides](https://docs.google.com/presentation/d/1WHUKnht-RxZ9lrMLBMwrs8BpW7ifN7y6iillhju2rKw/htmlpresent)
- In Zero the mutator is the unit of sync. Zero "calls them during sync for conflict resolution," so mutations are named operations (intent), not state diffs — [Zero docs](https://zero.rocicorp.dev/docs/mutators.md)
- PowerSync and Electric are middleware: reads stream from the source DB, and writes go through the developer's API to the source DB. PowerSync advises processing writes synchronously against the source DB — [PowerSync handling writes](https://docs.powersync.com/handling-writes/); [Electric write patterns](https://telemetry-landing.pages.dev/blog/write-patterns-for-electricsql/)
- Jazz and Couchbase are "engine as database": data, permissions, and sync are one system — [Jazz](https://jazz.tools/docs/react/groups/intro); [Couchbase](https://docs.couchbase.com/sync-gateway/current/access-control-concepts.html)

### Inferences
- In the middleware architecture the engine can own capture, fan-out, local storage, and the op queue. The app still owns the write handler. That handler is exactly where the declarative semantics (merge type, invariant class, authz predicate, side-effect hook) could be compiled in.
- Event-sourced mutations preserve intent, which helps with undo, audit, and rebase. State-based CRDTs are better for convergence without a server. A hybrid design would use mutations as the wire format, with CRDT-typed fields whose server handlers merge rather than overwrite.

### Gaps
- I couldn't fetch Weidner's original post. Claims about it are based on secondary summaries.
- No data was found on the operational costs of op-log retention and compaction in the two models.

## Q4. What fundamentally cannot be abstracted?

### Takeaway
Three things stay app-specific:
1. **The invariants themselves.** The engine can enforce them once declared, but it cannot infer them, and ECROs' own authors call the specs hard to write.
2. **Side effects in the real world.** The engine can only offer an outbox and idempotency. Compensation for an email that was sent or a payment that was taken is business logic.
3. **Conflict and rejection UX.** What to show the user when an offline write is rejected or merged surprisingly.

The theory (I-confluence, CALM) also proves that some operations, such as uniqueness, non-negative balances, and foreign-key inserts, cannot be both offline-available and correct. The app must choose one, or pre-allocate rights through escrow.

### Cited Findings
- No coordination-free strategy exists for non-I-confluent invariants — [Bailis blog](https://www.bailis.org/blog/when-does-consistency-require-coordination/)
- Escrow lets an app trade pre-allocated rights for offline availability, but the bound and share policy is an app decision — [Just-Right Consistency](https://arxiv.org/pdf/1801.06340)
- ECRO authors describe the hand-written first-order-logic invariant specs as hard to write — [VUB portal](https://researchportal.vub.be/en/publications/concurrency-contracts-for-designing-highly-available-replicated-d/)
- External side effects are not rolled back on retry, and a transactional outbox is recommended — [Zero docs](https://zero.rocicorp.dev/docs/mutators.md)
- Server rejection reverts the optimistic write, and the error is returned to the client for the app to handle — [Zero docs](https://zero.rocicorp.dev/docs/mutators.md); compensating writes in [Realm](https://www.mongodb.com/docs/realm/sdk/swift/sync/write-to-synced-realm/); revoked-access pushes rejected in [Couchbase](https://docs.couchbase.com/sync-gateway/current/access-control-concepts.html)
- PowerSync's custom-conflict guide includes recording unresolvable conflicts for a human to decide — [PowerSync custom conflict resolution](https://docs.powersync.com/handling-writes/custom-conflict-resolution)
- Concurrent access-control changes, such as revocation racing with an edit, are a hard edge case even for a dedicated project (Keyhive) — [Keyhive notebook](https://www.inkandswitch.com/keyhive/notebook/01/)

### Inferences
- **Generic (engine-owned):** transport, CDC, local store, op queue with idempotency keys, causal or total ordering, optimistic apply and rebase, a standard CRDT library, subscription and scope evaluation, tombstone GC mechanics, lens application, outbox plumbing, and a rejection/compensation event channel.
- **Declarable (app supplies data, engine enforces):** merge type per field, invariant declarations mapped to a consistency class, escrow bounds, scope queries, authz predicates, schema lenses, retention TTLs, and side-effect bindings.
- **Inherently app-specific (code or human):** the content of invariants, compensation logic for real-world effects, conflict and rejection UI, and domain-specific merges such as intent-preserving tree moves or semantic text merges.

### Gaps
- I found no empirical study measuring what fraction of real app logic falls into each category. This would be a useful thesis evaluation: port N apps and count declarative versus custom-code lines.
