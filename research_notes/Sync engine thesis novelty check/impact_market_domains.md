# Impact potential of a cross-domain offline↔cloud sync SDK (files + relational + optional E2EE), as of Nov 2026

Tags: [F] = sourced fact, [M] = my inference or judgment. Adoption numbers were pulled live on 2026-11-04 from the GitHub API and the npm downloads API (window 2026-10-04 to 2026-11-02).

## Market and demand: funding, market sizing, adoption metrics

### Takeaway
Real money is flowing into sync infrastructure. Ditto raised $82M (Mar 2025, $462M post-money) and Convex raised $57M (Aug 2026). Analysts put "edge data sync" at about $2.4–4.8B in 2025, growing about 21% a year. The developer-tool layer, though, is crowded with well-funded or long-running players (Electric, Zero, PowerSync, InstantDB, Convex, RxDB, Dexie), and the companies that tried to be the whole database were shut down or absorbed. Demand is proven. A new general SDK has room only in a niche, such as files + relational + E2EE across domains.

### Cited Findings
**Funding / corporate events**
- [F] Ditto closed an oversubscribed $82M Series B in March 2025 at a $462M post-money valuation, led by Top Tier Capital Partners and Acrew Capital. — [SiliconAngle](https://siliconangle.com/?p=689403); [CIO Influence](https://cioinfluence.com/?p=376891)
- [F] Ditto reported ARR up 250% year over year and customer count doubled; named airline customers include Delta, Lufthansa and Japan Airlines. — [BusinessWire, 2025-03-12](https://www.businesswire.com/news/home/20250312402967/en)
- [F] Ditto raised a $45M Series A in April 2023. — [SiliconAngle 2023](https://siliconangle.com/?p=613007); [Yahoo/TechCrunch](https://uk.news.yahoo.com/ditto-raises-45m-help-companies-123045225.html)
- [F] Reports of Ditto's total funding conflict: roughly $89M to $97.6M. — [ZoomInfo](https://www.zoominfo.com/financial/dittolive-inc/11485834); [SeedTable](https://seedtable.com/companies/ditto-com/funding-rounds/series-b-2025-03)
- [F] Convex raised a $57M Series B in August 2026, led by Insight Partners, with a16z, Spark Capital and Etna Labs participating. It follows a $24M round in November 2025 led by a16z. Reported total funding varies between $83M and $110.5M. — [FinSMEs 2026-08](https://www.finsmes.com/2026/08/convex-raises-57m-in-series-b-funding.html); [TheSaaSNews](https://www.thesaasnews.com/news/convex-raises-57m-series-b/)
- [F] Triplit: Supabase announced on 2025-10-08 that co-founder Matt Linkous was joining Supabase, and said it would not integrate Triplit into its platform directly. — [Supabase blog](https://supabase.com/blog/triplit-joins-supabase)
- [F] The Triplit repo's last push was 2026-01-19, so it is effectively dormant. — GitHub API (aspen-cloud/triplit)
- [F] Rocicorp (Zero/Replicache) is backed by angels including Elad Gil and Dylan Field. PitchBook lists about $7M early-stage in 2022, and other sources conflict. — [rocicorp.dev](https://rocicorp.dev/); [PitchBook](https://pitchbook.com/profiles/company/509669-29)
- [F] ElectricSQL funding reports are inconsistent: a $600K pre-seed from Lunar Ventures (2022), plus mentions of a Spark Capital-led seed. — [BounceWatch](https://bouncewatch.com/company/electricsql); [4dayweek](https://4dayweek.io/company/electric-sql)
- [F] Liveblocks shows only a 2022 seed round ($5M, Boldstart) in public trackers. — [TechCrunch 2022](https://techcrunch.com/2022/03/16/liveblocks-grabs-5-million-for-its-real-time-collaboration-api/)
- [F] PowerSync began as the sync engine inside JourneyApps (10+ years in business, Denver) and was spun out as a standalone product in 2022. It targets energy, manufacturing and mining customers, including unnamed "Fortune 500" companies. — [PowerSync company page](https://www.powersync.com/company)
- [F] MongoDB has partnered with PowerSync for offline-first enterprise apps (energy, field services) after shutting down Realm. — [MongoDB blog](https://mongodb.com/blog/post/mongodb-atlas-power-sync-future-offline-first-enterprise-apps)
- [F] A Microsoft DevBlogs post (Oct 2026) walks through a field-service app built on Azure DocumentDB + PowerSync. — [Microsoft DevBlogs](https://devblogs.microsoft.com/documentdb/?p=189)

**Market sizing (analyst reports, low reliability)**
- [F] The Business Research Company sizes "edge data sync platform" at $2.36B (2025), $2.88B (2026) and $6.25B (2030), about 21.5% CAGR. It names "offline-first data architecture adoption" as a key trend. — [TBRC](https://www.thebusinessresearchcompany.com/report/edge-data-sync-platform-global-market-report); [EIN Presswire](https://www.einpresswire.com/article/866905279/edge-data-sync-platform-market-size-worth-5-15-billion-by-2029-exclusive-report-by-the-business-research-company)
- [F] Market Intelo sizes the same market at $4.8B (2025) and $25.6B (2034), with a 22.4% CAGR. — [Market Intelo](https://marketintelo.com/report/edge-data-sync-platform-market/amp)

**Adoption metrics (GitHub stars and npm downloads, pulled 2026-11-04)**

| Project | GitHub stars | npm package | Downloads, last month |
|---|---|---|---|
| Yjs | 23,020 | yjs | 42.6M |
| RxDB | 23,419 | rxdb | 0.40M |
| Turso (Rust SQLite rewrite) | 25,137 | n/a | n/a |
| Dexie | 14,673 | dexie | 10.0M |
| InstantDB | 10,563 | @instantdb/react | 0.92M |
| Electric | 10,420 | @electric-sql/client | 6.5M |
| Automerge | 6,711 | @automerge/automerge | 0.29M |
| Loro | 6,270 | loro-crdt | 0.83M |
| Realm JS | 6,004 | realm | 0.17M |
| TinyBase | 5,209 | tinybase | 0.06M |
| Liveblocks | 4,768 | n/a | n/a |
| cr-sqlite | 3,824 | n/a | n/a |
| Rocicorp mono (Zero) | 3,435 | @rocicorp/zero | 0.81M |
| Triplit | 3,118 | @triplit/client | 0.82M |
| Evolu | 1,915 | n/a | n/a |
| PowerSync JS SDK | 737 | @powersync/web + @powersync/react-native | 0.48M + 0.26M |
| Jazz | 222 | jazz-tools | 0.03M |
| Convex | not retrieved | convex | 7.1M |

Source: GitHub REST API (api.github.com/repos/…) and the npm downloads API (api.npmjs.org/downloads/point/last-month/…). The Convex stars and the Jazz repo (garden-co/jazz shows 222 stars, which may be a moved or renamed repo) are unreliable.

### Inferences
- [M] Download counts are inflated by transitive dependencies and CI. Triplit's 0.82M a month while its repo is dormant, and Electric's 6.5M (probably pulled in through TanStack DB and similar libraries), show that downloads are not "apps in production". Treat them as a rough upper bound.
- [M] Enterprise money (Ditto, PowerSync/JourneyApps, Couchbase) sits in vertical, offline-critical domains: POS, airlines, defense, field service. Developer-tool money (Convex, Instant, Zero, Electric) sits in "reactive web app" UX. The user's pitch, cross-domain files + relational, sits closer to the enterprise side.
- [M] Ditto's 250% ARR growth and the MongoDB→PowerSync partnership suggest the Realm shutdown (2025-09-30) left migrating customers on the market.

### Gaps
- I found no reliable 2023–2026 funding figures for Electric, InstantDB (only a 2022 YC seed, ~$2M, per Tracxn), Garden Computing/Jazz (search returned an unrelated "Jazz" DLP company), Turso, or any Liveblocks Series A.
- I did not find State of JS, Stack Overflow survey items, or Local-First Conf attendance numbers that quantify offline/sync pain.
- I found no market size for offline-first *mobile* SDKs specifically.

## Domains that need offline + files + relational data, and the sync tech they use

### Takeaway
The strongest documented demand is in vertical, connectivity-hostile workflows that combine forms (relational) with photos or attachments (files). Examples: inspection (SafetyCulture on Couchbase), field service and energy (PowerSync, Couchbase), healthcare in low-resource settings (AEDES, BackpackEMR, SyncThink on Couchbase), POS/QSR (Chick-fil-A on Ditto), airlines (Delta, Lufthansa, JAL on Ditto) and defense DDIL (US Air Force ABMS on Ditto). I found no Korean examples.

### Cited Findings
- [F] **Retail/QSR POS:** Chick-fil-A rebuilt its in-house POS on Ditto, which keeps working peer-to-peer without cloud or Wi-Fi, and finished about two years ahead of plan. — [Ditto blog](https://www.ditto.com/blog/chick-fil-a-point-of-sale-system-goes-cloud-optional-with-ditto); [QSR Magazine](https://www.qsrmagazine.com/operations/fast-food/ditto-and-chick-fil-a-partner-on-point-of-sale/)
- [F] **Airlines:** Delta's flight-attendant app uses the Ditto SDK on company iPhones. Lufthansa, JAL and Alaska are also named as customers. — [BusinessWire](https://www.businesswire.com/news/home/20250312402967/en); [SiliconAngle 2023](https://siliconangle.com/?p=613007)
- [F] **Defense/DDIL:** Ditto lists a US Air Force edge-sync use case in degraded environments and an ABMS-related contract with a ceiling of $950M. A ceiling is not committed revenue. — [casestudies.com/Ditto](https://www.casestudies.com/company/ditto/case-study/us-air-force-utilizes-edge-sync-in-degraded-environments); [Ditto press](https://ditto.live/press-releases)
- [F] **Inspection:** SafetyCulture's iAuditor, used by more than 9,000 organizations, was rebuilt on Couchbase Lite for offline sync. Inspections are a forms + photos workload. — [casestudies.com/Couchbase](https://www.casestudies.com/company/couchbase/case-study/safetyculture-customer-case-study)
- [F] **Healthcare, low connectivity:** AEDES built a tablet hospital information system in sub-Saharan Africa. Its earlier intranet version failed when connectivity dropped and users went back to paper, so the team chose Couchbase for offline Android sync. — [AEDES case study PDF](https://info.couchbase.com/rs/302-GJY-034/images/aedes-case-study.pdf)
- [F] **Healthcare:** BackpackEMR runs pop-up clinics peer-to-peer offline and reports 20% lower charting cost than paper. SyncThink uses Couchbase Mobile for HIPAA-compliant sideline concussion tests. BD uses Couchbase Lite and Sync Gateway for diabetes care. — [Couchbase blog](https://www.couchbase.com/blog/?p=4754); [SyncThink case](https://www.casestudies.com/company/couchbase/case-study/enabling-rapid-secure-concussion-assessments-on-the-sidelines-and-beyond); [BD case](https://www.casestudies.com/company/couchbase/case-study/becton-dickinson-innovating-healthcare-with-iot)
- [F] **Field service / energy / mining:** PowerSync's origin market is syncing large relational datasets to frequently offline field users. — [PowerSync](https://www.powersync.com/company); [Couchbase field service](https://couchbase.com/use-cases/field-service/)
- [F] **Enterprise low-code:** Microsoft Dynamics 365 Field Service ships its own offline profiles and sync, so the big platforms build offline in-house. — [Microsoft Learn](https://learn.microsoft.com/en-us/dynamics365/field-service/mobile-power-app-system-offline)
- [F] **Consumer/indie:** PrayerMate rewrote its sync on PowerSync (2025). — [geero.net](http://www.geero.net/2025/01/rewriting-prayermate-sync-with-powersync)
- [F] Vendor material claims that Figma and Linear use CRDT-like techniques but keep the server as source of truth, and that regulated domains need server-side audit logs. — [tianpan.co forum](https://tianpan.co/forum/t/local-first-software-is-having-a-moment-crdts-automerge-and-the-end-of-save-buttons/606) (low-authority source)

### Inferences
- [M] The common pattern across inspection, healthcare, field service and logistics proof of delivery is "a relational record (form or job) plus N binary attachments (photos or scans) captured offline, plus server-side validation and audit". That is exactly the proposal's blob + row semantics gap, which is a good sign for the thesis framing.
- [M] Couchbase and Ditto mostly sell peer-to-peer and document models. Postgres-native engines (PowerSync, Electric, Zero) mostly leave files to the app (see the companion notes). So "files + relational" is under-served at the SDK level, though apps solve it ad hoc.
- [M] For a thesis, the most citable domains are inspection/field service and healthcare in low-connectivity settings, because published case studies show that failed connectivity sends users back to paper.

### Gaps
- I found no sourced examples for aviation EFB, agriculture, education, journalism, DAM, legal or CAD/BIM sync tech in this pass.
- Korean relevance: a Korean-language search turned up no Korean startups or Naver/Kakao products using local-first or CRDT sync. This is unverified, not a confirmed absence.

## Why sync SDKs failed or pivoted, and what adopters require

### Takeaway
The sync products that died owned the whole data model on a vendor-hosted backend: Realm Device Sync (shut down 2025-09-30), Amplify DataStore (Gen 1 end-of-life 2027, no Gen 2 replacement) and Triplit (founder joined Supabase, repo dormant). Survivors attach to the customer's existing database (Postgres or Mongo via PowerSync, Electric, Zero), which suggests adopters care most about bring-your-own-database, self-hosting and avoiding lock-in.

### Cited Findings
- [F] MongoDB deprecated Realm Device Sync in September 2024 and shut it down on 2025-09-30. The local Realm database stays open source without sync. — [realm-js discussion #6884](https://github.com/realm/realm-js/discussions/6884); [Realm, Wikipedia](https://en.wikipedia.org/wiki/Realm_(database)); [realm-swift #8680](https://github.com/realm/realm-swift/discussions/8680)
- [F] Amplify Gen 1, including DataStore, is in maintenance mode with end-of-life on 2027-05-01, and Gen 2 has no DataStore replacement. The source is a competitor (RxDB), so verify it with AWS. — [RxDB alternatives](https://rxdb.info/alternatives.html)
- [F] MongoDB pointed its Realm sync users to PowerSync as a partner. — [MongoDB blog](https://mongodb.com/blog/post/mongodb-atlas-power-sync-future-offline-first-enterprise-apps)
- [F] PowerSync positions itself as "database-agnostic on the server and platform-agnostic on the client". — [PowerSync](https://www.powersync.com/company)
- [F] Supabase is online-only and has no client storage or replication feature. — [RxDB](https://rxdb.info/alternatives.html)

### Inferences
- [M] The Realm and DataStore shutdowns left customers with forced rewrites. That gives adopters a reason to require an exit path: open protocol, self-hostable server, and data in their own Postgres or S3. A thesis SDK should treat "your DB, your bucket" as a design rule.
- [M] Further requirements, inferred from the case studies: compliance and audit trails (healthcare, defense), multi-platform SDKs (iOS, Android, web; Ditto and Couchbase both ship many), observability of sync state, and migration from an existing hand-written sync. None of these were quantified in a survey I found.
- [M] Commercial risk: big platforms (Microsoft Dynamics, MongoDB + PowerSync, Supabase acquiring talent) absorb sync as a feature, which squeezes standalone SDKs.

### Gaps
- I found no primary post-mortem from AWS on DataStore or from Triplit explaining why they stopped.
- I found no survey ranking adopter requirements. Fluid Relay status was not checked.

## What a credible thesis "impact" demonstration looks like

### Takeaway
The most influential sync libraries came from academic work: Yjs from RWTH Aachen research (GROUP 2016 paper), Automerge from Cambridge with Ink & Switch, and the "Local-first software" essay. They won through a rigorous core plus a usable open-source library, not through market share. For an undergrad thesis, credible impact means three things: the same SDK running in two or more real apps from different domains, measured reductions in app-side sync code, and correctness evidence such as a deterministic simulator or a formal model. GitHub stars are a bonus, not the claim.

### Cited Findings
- [F] Yjs's academic origin: Nicolaescu, Jahns, Derntl and Klamma, "Near Real-Time Peer-to-Peer Shared Editing on Extensible Data Types", ACM GROUP 2016 (DOI 10.1145/2957276.2957310), which introduced YATA. An earlier version appeared in 2015 (Springer) and was presented at FOSDEM 2015, from RWTH Aachen. — [EUSSET DL](https://dl.eusset.eu/handle/20.500.12015/4549); [Springer](https://link.springer.com/10.1007/978-3-319-19890-3_55); [FOSDEM 2015](https://archive.fosdem.org/2015/schedule/event/yatta/)
- [F] Yjs now has about 23k GitHub stars and about 42.6M npm downloads a month, the largest in this group. — GitHub/npm API (above)
- [F] The "Local-first software" paper by Kleppmann et al. (Cambridge, Ink & Switch, Onward! 2019) defined the field's vocabulary. — [Cambridge repository](https://www.repository.cam.ac.uk/handle/1810/297581); [Ink & Switch](https://inkandswitch.com/local-first/)
- [F] Automerge is presented by Kleppmann as an open-source CRDT library for local-first collaboration. — [ACM TechTalk](https://learning.acm.org/techtalks/softwareautomerge)
- [F] Loro published its rich-text CRDT algorithm alongside its open-source release. — [Loro blog](https://www.loro.dev/blog/loro-now-open-source)

### Inferences
- [M] A credible evaluation package for this thesis:
  1. Port two or more existing apps in different domains (for example the user's `clip` and `time`, plus one forms-and-photos app such as inspection or proof of delivery). Report LOC and the number of sync-related bugs before and after.
  2. Run a deterministic fault-injection simulator showing the blob + row invariants hold under crashes, partitions and server rejection.
  3. Benchmark sync latency, bandwidth and storage overhead against PowerSync or Electric on the same workload.
  4. Release the work open source with docs. Adoption metrics are optional upside.
- [M] Yjs took years after 2015–2016 to reach its current scale, so a thesis cannot point to adoption. It should point to evidence that the SDK is general (several domains) and correct (formal or simulation results).
- [M] Positioning: "a semantic contract for files + relational sync under rejection" can be shown within a thesis timeline. "A product that beats Ditto or PowerSync" cannot.

### Gaps
- I did not verify in this pass whether Yjs was specifically Kevin Jahns's master's thesis, nor details on Collabs, Fugue (CMU, Weidner et al.) or cr-sqlite origins. Cite their papers directly if used.
- I found no published study measuring LOC reduction from adopting a sync engine, so this metric would be novel but has no baseline.
