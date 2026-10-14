# Sync framework build-vs-buy and 2-month shipping plan (as of 2026-11-04)

Tags: [F] = stated by a cited source; [M] = my inference or model, not directly sourced. Inputs from the internal reports: `reports/Sync engine thesis novelty check.md` (distributed-server rules 1–8, recommendation) and `reports/Local first sync engine research.md` (Case studies: three kinds of local state, mounts/codecs, F1–F5, outbox/CAS, per-user `server_version`, 2xx rejection protocol, E2EE dual-mode S6).

## Q1. Effort evidence: how long comparable engines took

### Takeaway
Every production sync engine checked took **years**, usually with a team, and often needed a rewrite. PowerSync's lineage starts in 2009. Replicache had about five years before Zero, and Zero was still pre-1.0 when last checked. Electric threw away its first architecture. Linear's CTO had built several sync engines before. The solo case (Actual Budget) ended in burnout and open-sourcing. Treat a general engine in 2 months as impossible. A **narrow, app-specific** engine in 2 months is feasible.

### Cited Findings
- **PowerSync:** the original engine was built in **2009** inside JourneyApps' industrial app platform and ran in production at Fortune 500 companies for more than a decade before it became a standalone product [F] — [PowerSync company](https://www.powersync.com/company). The spin-off year conflicts: the company page says **2022**, the docs migration guide says **2023** [F, conflicting] — [PowerSync docs](https://docs.powersync.com/resources/migration-guides).
- **Replicache → Zero (Rocicorp):** created by Aaron Boodman and Erik Arvidsson [F] — [basehub forum](https://forums.basehub.com/rocicorp/mono/1). An aggregator says Replicache had "five years of development and use" before entering maintenance mode while the team moved to Zero [F, aggregator, low confidence] — [toolradar](https://toolradar.com/logo/replicache). Rocicorp calls itself "a small team" [F] — [The Sync: 001](https://rocicorp.dev/blog/the-sync-001). Zero docs snapshots I found go up to **0.22**, with no confirmed 1.0 [F, snapshot date unknown] — [Zero 0.22 notes](https://zero-docs-m9do7erzv-rocicorp.vercel.app/docs/release-notes/0.22).
- **Electric:** in July 2024 it did a "clean rebuild" (Electric Next) in a new repo, porting the minimum code and freezing the old system. The earlier design was vertically integrated and too complex [F] — [Electric blog 2024-07-17](https://electric-sql.com/blog/2024/07/17/electric-next). The rebuild dropped the write path at first [F] — [PowerSync comparison](https://powersync.com/blog/electricsql-electric-next-vs-powersync). v1.1 (Aug 2025) needed a new storage engine after production scaling problems [F] — [Electric v1.1](https://electric-sql.com/blog/2025/08/13/electricsql-v1.1-released).
- **Linear:** CTO Tuomas Artman built sync engines for over a decade before Linear (Groupon, Uber) [F] — [localfirst.fm #15](https://chartable.com/podcasts/localfirstfm/episodes/178924192-15-tuomas-artman-linear-sync-engines-rethought-startup-mvp). A secondhand talk recap says "Linear's first lines of code were the sync engine" [F, secondhand] — [talk recap](https://second-brain-nuxt.vercel.app/unexpected-benefits-of-going-local-first).
- **Actual Budget (solo):** James Long wrote "Using CRDTs in the Wild" on 2019-06-06 [F] — [jlongster](https://archive.jlongster.com/using-crdts-in-the-wild). He was "lonely and burnt-out" and "could not continue solo", tried lightly to sell the app, then open-sourced it [F] — [tmpdir community](https://community.tmpdir.org/t/actual-budget-is-switching-to-open-source/497). In 2024 he said "I open sourced it about two years ago" (≈2022) [F, recollection] — [localfirst.fm ep 7](https://ro.player.fm/series/localfirstfm/ep-7-james-long-actual-budget-electron-absurd-sql). The project is now MIT-licensed and community-maintained [F] — same.
- **LiveStore:** 0.2.0 went to early access around late 2024 / Jan 2025, and the beta was announced around May 2025. The changelog still said "beta… breaking changes before 1.0" [F] — [LiveStore changelog](https://docs.livestore.dev/changelog). I found no 1.0.
- **Evolu:** MIT-licensed with a self-hostable relay [F] — [ecosyste.ms](https://awesome.ecosyste.ms/projects/github.com%2Fevoluhq%2Fevolu). The README says the code is being rewritten to drop Effect, with experimental releases before stable [F] — same.
- **Electric was acquired by Databricks on 2026-08-11**; the team is joining Neon/Lakebase and **Electric Cloud is shutting down**. The OSS sync engine, PGlite and TanStack DB are said to continue [F] — [The New Stack](https://thenewstack.io/databricks-electric-wasm-agentic-postgres/); [Blocks & Files](https://blocksandfiles.com/ai-ml/2026/08/12/oh-no-not-another-one-databricks-buys-electric/5286721).

### Inferences
- [M] The pattern across vendors is that the first architecture gets replaced (Electric) or ships for years before it is generalized (PowerSync, Replicache). Rewrites come from generality: multiple write patterns, many backends, partial replication. A thin engine for two known schemas and one owner avoids most of that.
- [M] Actual Budget is the right calibration for a solo developer. LWW-over-HLC sync for one person's data worked and shipped. What killed it was maintaining the product alone, not the sync algorithm. Lesson: keep the custom surface small enough to maintain alongside a thesis.
- [M] The vendor churn (Electric Cloud shutting down, Replicache in maintenance, LiveStore and Zero still pre-1.0) is itself an argument for a thin custom core over SQLite + Postgres, which carries no vendor-lifetime risk.

### Gaps
- I found no reliable headcounts or dated "time to first production" for Zero, LiveStore, Jazz or Linear's v1 engine. Things, Bear and Obsidian sync war stories were not researched because of the tool budget. Zero and LiveStore 1.0 status as of Nov 2026 is unverified, since the snapshots found were older.

## Q2. Buy/borrow options per layer (self-hostable, mobile-capable, license)

### Takeaway
Borrow commodities: SQLite, Postgres, tusd for blob upload, an auth provider, RevenueCat, Sentry or PostHog. Avoid making a source-available or beta sync product the core. PowerSync's FSL is usable commercially but adds a second ordered system with its own semantics. SQLite Sync needs a paid commercial license. Turso offline sync was a beta with "no durability guarantees". Use Loro or Automerge only for future rich-text fields.

### Cited Findings
| Layer | Option | License / status | Fit |
|---|---|---|---|
| Row sync | **PowerSync Open Edition** | Service under **FSL** (commercial use allowed; the only restriction is a non-compete with PowerSync; converts to OSI open source after 2 years). Client SDKs **Apache 2.0** [F] — [PowerSync licensing](https://powersync.com/legal/overview); [FSL post](https://powersync.com/blog/powersync-supports-fair-source). Open Edition was marked **beta** at its 2024 release; Postgres ingester stable; no dashboard when self-hosted [F] — [Open Edition release](https://www.powersync.com/blog/powersync-open-edition-release); [self-hosting docs](https://docs.powersync.com/intro/self-hosting.md) | Mobile SDKs exist. One replication process per source DB; operation IDs are instance-local, so moving a user forces a full resync (internal report, distributed-server rule 4). Write path stays yours. |
| Row sync | **SQLite Sync (sqlite.ai)** | **Elastic License 2.0** plus a grant for OSS projects. **Commercial production use requires a commercial license** from SQLite Cloud, Inc.; no managed-service offering [F] — [pub.dev sqlite_sync license](https://pub.dev/packages/sqlite_sync/versions/1.1.1/license) (exact sqlite-sync text not fully verified) | Paid dependency for a paid product. Reject for v1 [M]. |
| Row sync | **Turso Sync** | Offline-sync public beta: bidirectional push/pull, conflict *detection* without resolution, "**no durability guarantees… data loss is possible**" during beta; default **Last-Push-Wins** per row [F] — [Turso offline beta](https://turso.tech/blog/turso-offline-sync-public-beta); [Turso Sync intro](https://turso.tech/blog/introducing-databases-anywhere-with-turso-sync). Python driver still beta (0.5.x) [F] — [pyturso](https://pypi.org/project/pyturso/) | Not for a paid product's source of truth [M]. |
| Row sync | **cr-sqlite** | No official deprecation found. Third-party trackers show repo metadata last updated **2025-05-05**, with 34 PRs merged in the prior year [F, third-party] — [ecosyste.ms](https://repos.ecosyste.ms/hosts/GitHub/repositories/vlcn-io%2Fcr-sqlite) | A transaction can span several `db_version`s, so there is no atomic visibility (internal report). Maintenance risk. Avoid [M]. |
| Row sync | **Electric OSS** | Acquired by Databricks (Aug 2026); Cloud shutting down; OSS "continues" [F] — [The New Stack](https://thenewstack.io/databricks-electric-wasm-agentic-postgres/) | Read-path shapes only; you build writes. Roadmap now driven by the AI-agent use case [M]. |
| Framework | **LiveStore** | Beta, pre-1.0, breaking changes [F] — [changelog](https://docs.livestore.dev/changelog) | JS/TS-centric. Rust daemon and Go CLI not served [M]. |
| Framework | **Evolu** | MIT, self-hostable relay, rewrite in progress [F] — [ecosyste.ms](https://awesome.ecosyste.ms/projects/github.com%2Fevoluhq%2Fevolu) | E2EE + LWW, TS-only. No server-side validation (internal report). |
| Framework | **Jazz 2.0** | I found no release announcement (search failed) | Internal report treats its SPEC as prior art. Unverified as a dependency. |
| CRDT fields | **Loro** | **MIT**; 1.0 released; Rust, JS/WASM and Swift (Swift "experimental"); UniFFI bindings list has no Kotlin [F] — [loro-ffi](https://docs.rs/crate/loro-ffi/latest/source/README.md); [loro-swift](https://github.com/loro-dev/loro-swift) | Defer. Only for notes text once collaboration arrives [M]. |
| Blob upload | **tusd** | **MIT**, reference tus server, disk/GCS/S3-compatible storage [F] — [tus/tusd](https://github.com/tus/tusd) | Go matches the clip server. Embed it as a library behind the media service [M]. |
| Payments | **RevenueCat** | Free up to **$2,500 MTR**, then 1%. Sources disagree on whether the 1% applies to all revenue or only the excess [F] — [RevenueCat pricing](https://revenuecat.com/pricing); [metacto](https://www.metacto.com/blogs/the-real-cost-of-revenuecat-what-app-publishers-need-to-know) | At $10/mo it costs nothing until about 250 subscribers [M]. Exit path: receipts are Apple's and Google's own [M]. |
| Mobile background | Tauri community background-service plugin | iOS BGTaskScheduler is best-effort: ~30 s windows, ≥15 min intervals; force-quit kills tasks [F] — [tauri-plugin-background-service](https://docs.rs/crate/tauri-plugin-background-service/0.5.3) | Sync on foreground plus push; don't rely on background sync [M]. |

### Inferences
- [M] **Auth:** pick one managed or self-hosted OIDC provider (Supabase Auth, Ory Kratos, Keycloak, or simply Sign in with Apple/Google plus your own session tokens in Go). I did not research licenses here. Keycloak (Apache-2.0) and Ory Kratos (Apache-2.0) are commonly cited as permissive, but **verify**. Apple requires Sign in with Apple if you offer other third-party social logins (verify the current guideline 4.8).
- [M] **Telemetry:** Sentry and PostHog both have self-host and cloud options and Tauri/JS/Go/Rust SDKs. Use the cloud free tiers for v1. Not verified this session.
- [M] **Lock-in ranking (lowest to highest):** own outbox on SQLite + Postgres < tusd/S3 < RevenueCat < PowerSync (FSL, its bucket/sync-rules model shapes your schema) < SQLite Sync / Turso (license or beta durability).
- [M] PowerSync is the only buy option that is credible for the mobile row-sync layer. However, the thesis needs control of the ordered log (per-user sequencer, hash chain, 2xx rejection, S6 replay), and PowerSync owns that order. Adopting it would split the thesis engine from the product engine. **Recommendation: don't buy row sync.**

### Gaps
- Exact Ory, Keycloak, Supabase Auth, Sentry and PostHog licenses and pricing as of Nov 2026: not researched. Jazz 2.0 status: search failed. Automerge 3 mobile bindings: not checked. Current PowerSync Open Edition maturity label (still "beta"?) for 2026: not checked.

## Q3. Thin custom core: what to own vs borrow, and where the thesis plugs in

### Takeaway
Own about five small components that the internal report already specifies. Borrow everything else. The thesis section (folder replica, F1–F5) is a **plugin on the client side of the outbox**. The product can ship without it and the thesis can be built against it, so the two schedules decouple.

### Cited Findings (design inputs)
- The server-side pattern of one ordered source per scope plus disposable readers is used by Zero, PowerSync, Electric, Figma, Linear and Notion (internal report, distributed-server table; sources there).
- Advance idempotency `last_mutation_id` in the same transaction, even on rejection [F] — [Replicache push](https://doc.replicache.dev/reference/server-push).
- Non-2xx errors block the PowerSync upload queue, so rejections should return 2xx with a status [F] — [PowerSync write validation](https://docs.powersync.com/handling-writes/handling-write-validation-errors).
- Per-command idempotency with `uuid` and `temp_id` mapping [F] — [Todoist Sync API](https://developer.todoist.com/api/v1/).

### Inferences — the layer stack [M]
| # | Component | Own / borrow | Size estimate [M] |
|---|---|---|---|
| 1 | Local store: SQLite (rusqlite on desktop/Tauri mobile, IndexedDB or SQLite-WASM in Chrome MV3) | Borrow | — |
| 2 | **Client sync core (Rust crate)**: `pending_mutation` outbox, `server_version` cursor, pull/apply in one transaction, HLC, per-field LWW, tombstones, CAS blob store | **Own** | ~2–3k LoC |
| 3 | **Server sync service (Go, connect-rpc)**: `push` (per-user `sync_scope` row sequencer, mutation table, deferred FKs, 2xx rejection), `pull(cursor)`, poke via Postgres LISTEN/NOTIFY or Redis pub/sub | **Own** | ~1.5k LoC |
| 4 | **Media service**: tusd embedded, content-hash keys, `pending→committed` claim, orphan GC | Own the glue, borrow tusd | ~800 LoC |
| 5 | Per-app mutation handlers / validator (Rust, shared native + WASM for the extension) | Own | per app |
| 6 | Auth (OIDC), payments (RevenueCat + StoreKit/Play Billing; Stripe for desktop/web), crash/analytics (Sentry/PostHog), push (APNs/FCM directly) | Borrow | — |
| 7 | **Thesis plugin: mount layer** (`fs_entry`, codecs, watcher + scan, echo suppression, intent record) | Own, *thesis* | desktop only in v1 |
| 8 | E2EE (encrypt mutation payloads with a per-user key; server sees metadata) | Own, **post-v1** | — |
- The Chrome MV3 extension is the awkward client because it can't run the Rust crate natively. Compile the core to WASM, or make the extension an online-only client of the server API for v1 (cheapest).
- The thesis plugs in at the boundary where the mount layer turns disk changes into normal named mutations in the same SQLite transaction as the outbox record. The core needs no folder awareness. That keeps F1–F5 testable in a deterministic simulator without the network.
- Keep the wire protocol versioned from day one (`protocol_version` in push/pull) so that the E2EE and S6 work later is a migration, not a rewrite.

### Gaps
- No measured LoC for comparable thin engines. The estimates are [M] only.

## Q4. Feature-scope triage for v1

### Takeaway
v1 = plaintext server sync with TLS, accounts, and one paid tier. Defer E2EE, folder mounts on mobile, collaboration and calendar mirroring. E2EE is the biggest scope risk because of key recovery UX.

### Inferences [M]
**clip v1 (ship):** text and image clips; cross-device delivery (desktop daemon, CLI, extension, iOS, Android); dedup by content hash (plaintext `sha256` per user in standard mode); pin/delete with tombstones; local-only retention ("keep last N" never syncs deletes, per the internal report); images via the media service with thumbnails eager and originals lazy on mobile. **Defer:** E2EE (keep the HMAC identity scheme for v1.1), OCR sync, file clips, the screenshot folder mount (desktop-only thesis demo), iOS keyboard extension or share-sheet polish. Note that iOS restricts background clipboard reading, so mobile clip capture is likely share-sheet or manual paste. Verify the current iOS pasteboard rules.

**time v1 (ship):** tasks, projects, sections, recurrence with the idempotent `complete(task_id, occurrence_date)`, fractional `order_key`, timers keyed per device, local notification reminders, a paid sync tier. **Defer:** Google Calendar mirroring (a server-side outbox effect that needs OAuth verification for calendar scopes), habits and ActivityWatch auto logs (sync them later), weekly-review bounded-count soft invariants (local warning only), the markdown folder mount (thesis), collaboration and sharing, E2EE.

**Required groundwork for time (from the internal report):** client-generated IDs, `updated_at`/HLC, tombstones, fractional ordering, and every multi-row action as one named mutation.

### Gaps
- The user's actual MVP priorities were not consulted. The Jennifer spec (`.jennifer/products/personal-time/spec-v1-2026-10-23.md`) was not read in this pass.

## Q5. Risk list for a 2-month ship

### Takeaway
The longest non-code lead times are **Google Play's 14-day/12-tester closed test** (personal accounts created after 2023-11-13) plus up to 7 days of production-access review. That is about 3 weeks, and it must start by week 4–5. Apple review is usually ≤48 h. The engineering risks are Tauri mobile maturity, iOS background limits, the time.db ID migration, and E2EE recovery (defer it).

### Cited Findings
- **Google Play:** personal accounts created after **13 Nov 2023** must run a closed test with **≥12 testers opted in for 14 consecutive days**. Testers who leave don't count, and rejoining restarts the count. Production-access review takes "seven days or less", sometimes longer, and Google may require more testing [F] — [Play Console Help 14151465](https://support.google.com/googleplay/android-developer/answer/14151465?hl=en-GB). Organization accounts are exempt but need a D-U-N-S number (up to ~30 days) [F, secondary] — [dev.to](https://dev.to/theionproject/the-12-tester-rule-that-quietly-adds-two-weeks-to-your-first-google-play-launch-4bp3). One secondary source says a rejected application restarts the closed test [F, secondary] — [choicely](https://www.choicely.com/blog/google-play-12-tester-rule).
- **Target API:** since **31 Aug 2026**, new apps and updates must target **Android 16 (API 36)** [F] — [Android target SDK](https://developer.android.com/google/play/requirements/target-sdk).
- **Apple review:** Apple's App Review page says 90% reviewed within 24 h. A June 2026 statement reported by 9to5Mac says 90% within 48 h (average 1.5 days). Forum reports of multi-week stalls exist [F, conflicting] — [summary incl. Apple Help wording](https://blakecrosley.com/blog/app-review-what-reviewers-look-for-2026); [Apple forums](https://developer.apple.com/forums/thread/841789).
- **Tauri mobile:** the 2.0 team said 2.0 was not "the mobile release"; not all desktop plugins are ported; the mobile DX needed work [F, 2024] — [Tauri 2.0 RC](https://v2.tauri.app/blog/tauri-2-0-0-release-candidate/). Many mobile capabilities (IAP, ASWebAuthenticationSession, Photos) come from **community plugins** [F] — [Tauri vs Capacitor eval, May 2026](https://range-day-docs.pages.dev/decisions/Tauri-vs-Capacitor). iOS background tasks are best-effort [F] — [background-service plugin](https://docs.rs/crate/tauri-plugin-background-service/0.5.3).

### Risk register [M]
| Risk | Severity | Mitigation |
|---|---|---|
| Play 12×14-day gate + ≤7-day review | High (calendar) | Create the Play account in **week 1**. Ship a minimal closed-test build by **week 3**. Recruit 15–20 testers (classmates) so drop-outs don't reset the count. Or use one Play app for both products? No: each app needs its own test. Run both closed tests in parallel. |
| Apple Developer enrollment / D-U-N-S if publishing as the skkil org | Medium | Enroll as an individual now, or start D-U-N-S in week 1. TestFlight external testing needs Beta App Review (unverified timing). |
| Tauri mobile plugin gaps (IAP, share extension, notifications) | High | Spike in week 1: build a hello-world Tauri iOS + Android app with SQLite, notifications and IAP via RevenueCat. If it fails, fall back to a thin native shell (SwiftUI/Kotlin) over the Rust core via UniFFI. |
| Chrome extension can't host the Rust core | Medium | Online-only client of the API for v1. |
| time.db migration from INTEGER IDs to ULID, dense position to fractional keys | High (data loss) | One-shot migration 011 on first launch after update: create mapping table `old_id → ulid`, rewrite FKs in one transaction, back up `time.db` first, keep the old file 30 days. Test on real user DB copies. |
| macOS notarization / Windows signing for desktop | Medium | Apple Developer ID is needed for notarization (same $99 program, unverified fee). Windows: ship unsigned or with Azure Trusted Signing (verify). Linux: AppImage/Flatpak. |
| Subscriptions across platforms (desktop is outside the app stores) | Medium | RevenueCat for iOS/Android plus Stripe (or RevenueCat Web Billing, verify) for desktop. Entitlement checked server-side by the sync service. |
| E2EE key recovery UX | High if in v1 | **Defer.** If needed, use a recovery key with a printed or exported phrase and no server escrow. Losing the key loses the data, which is the support-burden trap. |
| Thesis and product competing for the same weeks; December finals | High | Product freeze at week 8. The thesis plugin is off the product's critical path. |

### Gaps
- Apple Developer Program fee and enrollment time, TestFlight external review timing, Sign in with Apple rule, macOS notarization steps, and Chrome Web Store review time for MV3: not verified this session.

## Q6. 8-week plan and parallel thesis timeline

### Takeaway
Weeks 1–3 build the thin core and start the store clocks. Weeks 4–6 port both apps. Weeks 7–8 run payments, the closed tests and launch. The thesis builds the mount plugin and simulator on top of the same core from week 3, through finals, to March 2027.

### Inferences — week-by-week [M] (assumes starting week of 2026-11-09; launch ≈ 2027-01-04)
| Week | Product (critical path) | Store/admin clocks | Thesis (parallel) |
|---|---|---|---|
| 1 (Nov 9) | Tauri mobile spike (iOS + Android: SQLite, notifications, RevenueCat). Freeze the wire protocol v0 (push/pull/poke, 2xx rejection). time schema migration design (ULID, HLC, tombstones, `order_key`). | Create Play personal account, Apple Developer enrollment, Chrome Web Store account. Recruit 20 testers. | Write the thesis proposal around F1–F5. Skim Harmony and lens papers. |
| 2 (Nov 16) | Go sync service: `sync_scope` sequencer, mutation table, pull by cursor, two stateless instances, LISTEN/NOTIFY poke. Rust client core: outbox, apply, HLC/LWW. | — | Deterministic simulator skeleton (seeded scheduler around the client core plus an in-memory server). |
| 3 (Nov 23) | Auth (OIDC + sessions). Media service (tusd + content hash + claim). **clip** on the core: desktop daemon + CLI. | **Ship a minimal Android closed-test build of both apps** (even with sync disabled) to start the 14-day clock. TestFlight internal. | `fs_entry`, opaque codec spec; F1/F2 property tests. |
| 4 (Nov 30) | clip on iOS/Android (Tauri) and the extension (online client). Image thumbnails and lazy originals. | Closed test running (day 1–7). | Opaque mount (screenshots) with watcher + scan + echo suppression (F5). |
| 5 (Dec 7) | **time** migration 011 (ID remap, fractional order) plus named mutations (move, complete occurrence, archive cascade). time desktop on the core. | Closed test day 8–14. | **Finals: thesis paused.** |
| 6 (Dec 14) | time on mobile (Tauri). Local notification reminders. Sync-issues list UI for rejections. | Apply for Play production access (≤7 days). | Finals. |
| 7 (Dec 21) | RevenueCat entitlements + server-side check; Stripe for desktop. Sentry/PostHog. Backups (pg_dump + bucket versioning). | App Store submissions (≤48 h typical). Chrome Web Store submission. | Crash-point injection harness for F4 (intent record). |
| 8 (Dec 28) | Bug-fix, migration testing on real DBs, load test of pull. **Launch** both apps. Freeze features. | Production on all stores. | Structured codec `markdown_tasklist` started. |
| Jan 2027 | Maintenance only | — | Structured codec + F3 (three-way row merge); simulator runs, 10^6 schedules; baselines. |
| Feb 2027 | v1.1: E2EE behind a flag (if time) | — | S6 or S2 as a secondary section only if F-section done; evaluation (violations/10^6, convergence, bytes). |
| Mar 2027 | — | — | Write-up and submit. |

- The internal report planned four months starting with media-service chapter one (I1–I6). This plan compresses that by making the product's core the thesis's substrate, and by narrowing the thesis to **one lead claim (F1–F5)**. S6 and S2/S5 become optional sections.
- If the week-1 Tauri mobile spike fails, push mobile to v1.1 and launch desktop + extension + CLI on schedule. Keep the Play closed test running with a minimal build so the 14-day clock still completes.

### Gaps
- The plan assumes about 50+ hours a week of availability and no coursework conflicts except December finals. Not validated with the user.
