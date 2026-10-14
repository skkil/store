# Offline-capable telemetry for clip and time (as of 2026-11-04)

Tags: [F] = backed by a cited source; [M] = my inference or recommendation (not from a source). Research stopped at about 22 tool calls because the session's web search budget ran out. Items not checked are listed under Gaps.

## 1. SDKs with offline persistence (OTel, Sentry, PostHog, privacy-first vendors)

### Takeaway
Every major SDK buffers on disk with a bounded size or count and drops the oldest data first. None of them guarantee delivery. Coverage is uneven: the Android and iOS OTel SDKs have disk buffering, Sentry mobile and Electron cache envelopes, and the Rust Sentry and Tauri plugin paths apparently do not cache offline. The Rust/Tauri desktop, Rust daemon, Go CLI and MV3 extension will need a small hand-built disk queue in front of OTLP/HTTP.

### Cited Findings
- [F] opentelemetry-android lists offline buffering ("Disk persistence to buffer telemetry data when the device is offline") as a key feature, enabled by default. Export goes over OTLP/HTTP (`httpExport.baseUrl`, port 4318, `baseHeaders` for auth). Batching is on by default "to reduce network calls and battery use". The BOM shown is 1.8.0. Session config: `backgroundInactivityTimeout` (15 min in the example) and `maxLifetime` (4 days in the example). The page gives no stability label or buffer limits. — [OTel Android docs](https://opentelemetry.io/docs/platforms/client-apps/android/)
- [F] Defaults of the underlying opentelemetry-java-contrib `disk-buffering` module: max folder size 10 MB, max file size 1 MB, max age for writing 30 s, max age for reading 18 h (older files are treated as stale and purged), `deleteItemsOnIteration=true`. With `false`, items stay on disk until `Iterator.remove()` is called after a successful export. Writes go to `<ts>.tmp` files that are atomically renamed, and crash leftovers are recovered on startup. The export loop should stop on the first failure. A `DiscardedFileListener` reports data dropped before it was read. Each signal type has its own directory. — [java-contrib disk-buffering README](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/disk-buffering/README.md)
- [F] Vendor equivalents: Splunk Android RUM caps disk buffering at 25 MB by default (`limitDiskUsageMegabytes`). Elastic EDOT Android has a `DiskBufferingConfiguration`. — [Splunk docs](https://help.splunk.com/en/splunk-observability-cloud/manage-data/instrument-front-end-applications/instrument-mobile-and-web-applications-for-splunk-real-user-monitoring-rum/instrument-android-applications-for-splunk-rum/splunk-rum-android-agent-versions-below-2.0.0/configure-the-splunk-android-rum-instrumentation); [Elastic EDOT Android config](https://docs-v3-preview.elastic.dev/elastic/docs-builder/docs/3866/reference/opentelemetry/edot-sdks/android/configuration.md)
- [F] opentelemetry-swift has a `PersistenceExporter`, a decorator that wraps an existing exporter. It serializes data to disk asynchronously at a `storageURL`, and the data survives app termination and relaunch. When it wraps the OTLP HTTP exporter, pass `requeueOnFailure: false` so the HTTP exporter does not also keep an in-memory copy, which would duplicate spans. The README documents no size or age limits. — [opentelemetry-swift Persistence README](https://github.com/open-telemetry/opentelemetry-swift/blob/main/Sources/Exporters/Persistence/README.md); [CocoaPods 2.3.0](https://cocoapods.org/pods/OpenTelemetry-Swift-PersistenceExporter)
- [F] OpenTelemetry Rust, per a July 2026 third-party guide: the logs and metrics API/SDK are stable, the OTLP exporter is RC, and traces are beta. The crates are on the 0.32 line, and all first-party crates must share the same minor version. Logs are recommended through the `tracing` bridge (`opentelemetry-appender-tracing`). This is not verified against the official repo. — [OpenObserve Rust guide](https://openobserve.ai/opentelemetry/rust.md); [dasroot Jan 2026](https://dasroot.net/posts/2026/01/rust-observability-opentelemetry-tokio/)
- [F] Sentry's default offline cache holds a maximum of 30 envelopes. On overflow it deletes the oldest envelope and migrates its sessions to the next one to keep release-health stats intact. Flutter's `maxCacheItems` (default 30) applies only on Android, iOS and macOS. — [sentry-docs PR](https://github.com/getsentry/sentry-docs/pull/4566.diff); [sentry_flutter maxCacheItems](https://pub.dev/documentation/sentry_flutter/latest/sentry_flutter/SentryFlutterOptions/maxCacheItems.html)
- [F] A proposal for a byte-based `maxCacheSize` exists because envelopes range from a few KB to several MB with screenshots. Whether it shipped is unconfirmed. — [getsentry/team-mobile#163](https://github.com/getsentry/team-mobile/issues/163)
- [F] Sentry support says offline caching is supported by all Sentry mobile SDKs. The browser SDKs use an IndexedDB transport wrapper with a default limit of 30 events. — [Sentry support](https://sentry.zendesk.com/hc/en-us/articles/39205185672731-Can-I-use-Sentry-in-an-offline-environment); [Sentry JS offline caching](https://docs.sentry.io/platforms/javascript/guides/nextjs/best-practices/offline-caching/)
- [F] Sentry Electron offline options (`transportOptions`): `maxQueueSize` (30 in the example), `maxAgeDays` (30 in the example), `flushAtStartup` (default false), and `shouldSend` / `shouldStore` callbacks that can gate sending, for example on consent or a metered network. — [Sentry Electron offline support](https://docs.sentry.io/platforms/javascript/guides/electron/features/offline-support/)
- [F] `tauri-plugin-sentry` 0.6.0 is "experimental" for Tauri v2. It injects `@sentry/browser` into webviews and forwards events to the Rust SDK through Tauri invoke. A Tauri community discussion (undated) says the Rust SDK has no offline caching and suggests implementing the `Transport` trait yourself. — [docs.rs tauri-plugin-sentry](https://docs.rs/crate/tauri-plugin-sentry/latest); [tauri discussion #5959](https://github.com/orgs/tauri-apps/discussions/5959)
- [F] PostHog iOS defaults: `flushAt` 20 (setting it to 1 sends immediately at the cost of battery), `maxQueueSize` 1000 (oldest events are dropped past that), `maxBatchSize` 50. The React Native and Node SDKs expose the same option names. — [PostHog iOS config](https://posthog.com/docs/libraries/ios/configuration); [PostHog RN types](https://posthog.com/docs/references/posthog-react-native/types/PostHogCoreOptions)
- [F] PostHog self-hosting, per secondhand vendor pages and not PostHog's own docs: the hobby deploy needs about 4 GB of RAM and scales to roughly 100k events per month, with Cloud recommended beyond that. There is no support or guarantee for open-source deploys. — [Zima page](https://shop.zimaspace.com/pages/posthog-hardware-requirements)
- [F] Aptabase says it collects no personal data, cookies or device IDs, and so needs no consent banner (vendor claim). Its Tauri plugin tracks only events you send with manual `trackEvent`, adds OS and app version, sends in the background, and offers `flush_events_blocking` on exit. The crates.io README points to a v2 branch for Tauri v2. Offline persistence is not documented. — [Aptabase for Tauri](https://aptabase.com/for-tauri.md); [npm @aptabase/tauri](https://www.npmjs.com/package/@aptabase/tauri)
- [F] TelemetryDeck double-hashes the user ID: a salted hash on the device, then a second salted hash on the server. It argues a single hash is only pseudonymization under GDPR and the second makes the data anonymous (vendor claim, not a regulator view). — [TelemetryDeck anonymization](https://telemetrydeck.com/docs/articles/anonymization-how-it-works/)

### Inferences
- [M] Use one cross-platform envelope for all telemetry: OTLP/HTTP protobuf or JSON to your own OTel Collector. Mobile can use the official disk buffering (Android java-contrib storage, Swift PersistenceExporter). For Rust desktop/daemon, Go CLI/server and MV3, write a roughly 200-line "telemetry spool" that mirrors the java-contrib design: append-only files, atomic rename, a 10 MB cap, 18 h–7 d max age, delete-after-ack, oldest-first eviction, and a dropped-count counter.
- [M] For crashes, Sentry is the pragmatic choice on iOS/Android, which have native crash capture and offline caching. On Rust desktop/daemon, use a panic hook that writes a minidump or panic record into the same spool and uploads it on the next launch. Don't rely on Sentry Rust offline behavior.
- [M] MV3 service workers are ephemeral, so a spool must live in IndexedDB or `chrome.storage.local` and flush on `chrome.alarms` or the next wake. `navigator.onLine` is unreliable. Treat a failed fetch as offline.
- [M] Battery and data: batch at 20–50 events or 30–60 s, flush when the app goes to background or foreground, skip metered networks for anything non-critical, and gzip OTLP. Mobile sampling: keep 100% of errors, crashes and sync-failure spans, sample successful sync spans at 1–10%, and send metrics as pre-aggregated client counters/histograms instead of per-event spans.

### Gaps
- opentelemetry-android disk-buffering stability label and current defaults inside the Android agent. The java-contrib defaults may be overridden.
- Whether Sentry Rust (`sentry` crate) has added offline caching in 2026, and whether `maxCacheSize` shipped.
- Aptabase Tauri v2 offline behavior; Grafana Faro, Embrace, Bugsnag, Firebase Crashlytics offline queue specifics. Not researched because the search budget was exhausted.
- OTel Go SDK status. Not checked; Go traces and metrics have been stable for a long time per prior knowledge, but this is unverified here.

## 2. Separate pipeline vs riding the sync engine (and product data vs telemetry)

### Takeaway
Send telemetry through a separate, loss-tolerant pipeline. Don't add it as a stream type in the sync log. The sync log is ordered, durable, E2EE-bound, per-user and conflict-resolved, while telemetry is unordered, lossy, aggregated, consent-gated and has different retention. The parts worth sharing are the client-side outbox pattern (SQLite or disk spool) and the network scheduler, not the server log. For `time`, ActivityWatch events and time entries are product data: they go through the sync engine, E2EE, and the "window titles never sync unless opted in" rule. They must never be copied into telemetry.

### Cited Findings
- [F] Existing sync products keep sync diagnostics out of the sync data plane. PowerSync exposes client `SyncStatus` (connected, lastSyncedAt, uploading, upload queue size = row count of the `ps_crud` table) and recommends shipping sync errors and last-sync time through a separate logger such as Sentry. Its self-hosted service has a Diagnostics API reporting `replication_lag_bytes`. — [PowerSync production readiness](https://docs.powersync.com/maintenance-ops/production-readiness-guide); [PowerSync troubleshooting](https://docs.powersync.com/debugging/troubleshooting)
- [F] The OTel Collector-style persistent queue deletes a batch from disk only after the exporter confirms delivery. This is the same at-least-once outbox semantics, applied to telemetry separately. — [oneuptime on persistent queues](https://oneuptime.com/blog/post/2026-02-06-persistent-disk-queues-otel-crash-recovery/view)
- [F] Play's Data safety rules say data that is end-to-end encrypted, readable only by sender and recipient and not by the developer, need not be declared as collected. Telemetry the server can read must be declared. — [Play Data safety](https://support.google.com/googleplay/android-developer/answer/10787469)

### Inferences
- [M] Reasons not to ride the sync log:
  1. Sync payloads are E2EE, so the server cannot aggregate them. Telemetry must be server-readable to be useful.
  2. Per-user total order and server-assigned sequence numbers are wasted on telemetry and add write amplification in the Postgres log, which is the hot path the load tests measure.
  3. If telemetry shares the user-data outbox, a telemetry backlog can delay user mutations (head-of-line blocking), inflate the outbox-depth metric, and pollute the measurements.
  4. Consent can be revoked, and revocation should purge pending telemetry. The sync log has tombstone semantics instead.
  5. Retention differs: telemetry 14–90 days, user data indefinite.
  6. Telemetry must be droppable under pressure. User data must never be dropped.
  7. The sync log's account/user ID links every telemetry event to a person. A separate pipeline can use a rotating install ID.
- [M] I found no precedent of a product routing telemetry through its own CRDT/sync log, and I wouldn't cite one.
- [M] If you want code reuse, share a generic `AppendOnlySpool` client primitive with two instances: `outbox.sqlite` (user mutations to the sync server, never dropped, ordered) and `telemetry.spool` (to the OTel Collector, bounded and evictable, opt-in gated, lowest network priority, paused while the sync outbox is non-empty or the network is metered). This is the "append-only stream type" done at the client-library level, not as a server-log stream type.
- [M] Product data vs telemetry for `time`: ActivityWatch buckets, window titles, app names, URLs and time entries are user content and therefore E2EE product data. Telemetry may only say that an ActivityWatch import ran, with an event count bucket (e.g. 100–1k), duration and error code. It never includes titles, app names or URLs. Treat app and process names as content too, because they reveal behavior.
- [M] For `clip`: never put clip text, image hashes, MIME type plus size combinations that could fingerprint content, or source-app names in telemetry. Coarse buckets are fine (size class, type enum text/image/file).

### Gaps
- No public engineering write-up found (Linear, Figma, Replicache/Zero, ElectricSQL) that explicitly discusses rejecting the sync log for telemetry. The search budget ran out before this was checked.

## 3. Privacy and compliance (consent, Apple, Google, Chrome, GDPR/ePrivacy, PIPA, E2EE)

### Takeaway
First-party, self-hosted, content-free telemetry does not trigger ATT. It must still appear in the App Store privacy label, the Play Data safety form and the Chrome Web Store disclosures. In the EU, ePrivacy Art. 5(3) can require consent for reading or storing on-device data that isn't strictly necessary. Under PIPA (Korea), personal information needs disclosed purposes and consent or another legal basis, and overseas transfers need disclosure. Simplest compliant posture for a privacy-branded product: crash reports opt-in or prompted on first crash, product analytics opt-in, a rotating anonymous ID, a self-hosted collector, and zero content.

### Cited Findings
- [F] ATT "tracking" means linking app data with other companies' data for targeted ads or ad measurement, or sharing with data brokers. IDFV may be used for analytics across the same vendor's apps without ATT. Fingerprinting ("derive data from a device for the purpose of uniquely identifying it") is prohibited regardless. You are responsible for all SDK code in your app. — [Apple User Privacy and Data Use](https://developer.apple.com/app-store/user-privacy-and-data-use/)
- [F] Apple's commonly-used third-party SDK list (privacy manifest required, plus a signature for binary dependencies) includes FirebaseCrashlytics, the Firebase core/diagnostics/installations pods, GoogleDataTransport, FBSDKCoreKit and OneSignalOutcomes. Sentry, Mixpanel, Amplitude and Bugsnag are not on the list. — [Apple third-party SDK requirements](https://developer.apple.com/support/third-party-SDK-requirements/)
- [F] UserDefaults is a required-reason API, commonly declared with `NSPrivacyAccessedAPICategoryUserDefaults` reason `CA92.1` (app-specific config). Manifests for third-party SDKs that use required-reason APIs have been required since 2024-05-01. — [Singular FAQ](https://support.singular.net/hc/en-us/articles/24045392537243-iOS-SDK-Privacy-manifest-FAQ); [Nutrient blog](https://www.nutrient.io/blog/privacy-manifests-on-ios/)
- [F] Play Data safety: "collect" means transmitted off the device, including by SDKs. Crash logs and diagnostics fall under "App info and performance", app interactions under "App activity". Pseudonymous data tied to a device ID must still be declared. Ephemeral in-memory processing is excluded from display but must still be in the form. E2EE data is not "collected". Data can be marked "optional" only if every user can opt out. — [Play Data safety](https://support.google.com/googleplay/android-developer/answer/10787469)
- [F] Chrome Web Store Limited Use: extensions may collect only data necessary for the disclosed single purpose, "including related operational purposes" such as performance and reliability. Disclosure and consent must appear in the extension's UI, not only in the store listing. The extension's website must carry an affirmative Limited Use compliance statement. A secondary report says an Aug 1, 2026 policy update bars analytics unrelated to core functionality (not confirmed from an official source). — [Chrome Limited Use](https://developer.chrome.com/docs/webstore/program-policies/limited-use/); [Chrome user data FAQ](https://developer.chrome.com/docs/webstore/user_data/); [CyberInsider](https://cyberinsider.com/?p=327769)
- [F] EDPB Guidelines 2/2023 on the technical scope of ePrivacy Art. 5(3), final v2.0 adopted 2024-10-07, rest on three criteria: "information", "terminal equipment", "gaining access". Coverage goes beyond cookies to tracking pixels and links, fingerprinting, and "certain types of local processing where information is transferred". Use cases are analysed in the final section, which I did not read. — [EDPB v2.0 PDF](https://edpb.europa.eu/system/files/2024-10/edpb_guidelines_202302_technical_scope_art_53_eprivacydirective_v2_en_0.pdf); [Hunton summary](https://www.hunton.com/privacy-and-cybersecurity-law-blog/edpb-adopts-guidelines-on-scope-of-eprivacy-directive)
- [F] PIPA: the 2023 amendment (Act No. 19234, 2023-03-14) took effect 2023-09-15. It added legal bases for overseas transfer beyond consent (treaty, PIPC-certified recipient, adequacy). Entrustment or storage abroad is allowed if disclosed in the privacy policy or notified. The PIPC can order an overseas transfer suspended. Pseudonymization and anonymization concepts date from the 2020 amendment. — [BKL update](https://www.bkl.co.kr/en/law/insight/newsletter/5374); [Ius Laboris](https://iuslaboris.com/insights/south-korea-passes-major-overhaul-of-data-protection-law/); [Chambers](https://chambers.com/articles/second-major-amendment-to-the-personal-information-protection-act-passed-by-national-assembly-i-2)
- [F] Vendor "no consent needed" claims (Aptabase, TelemetryDeck) are the vendors' own legal positions, not regulator rulings. — [Aptabase](https://aptabase.com/for-tauri.md); [TelemetryDeck](https://telemetrydeck.com/docs/articles/anonymization-how-it-works/)

### Inferences
- [M] Consent model. Opt-in for everything except a minimal "strictly necessary" set, which is plausibly sync-error codes needed to operate the service. Even that set should be disclosed and should use no persistent cross-session ID. In Korea this matches PIPA's notice-and-consent norm, and a self-hosted collector in a Korean region avoids the overseas-transfer question. If you use Sentry SaaS or PostHog Cloud (US/EU), list them as overseas entrustees in the 개인정보처리방침 (privacy policy).
- [M] Identifier. Use a random install ID that rotates every 30–90 days and is never derived from the account ID, kept in a separate key. Server-side, store only an HMAC with a server secret. Never send account ID, email, device name or hostname. Desktop hostnames often contain the user's name.
- [M] Scrubbing at the source. Attributes are allowlisted, not denylisted: a typed `TelemetryEvent` enum per product, with no free-form strings except error codes. Scrub panic and error messages, since paths contain usernames and SQL errors can echo values. Add a CI test that fuzzes clip content through the error paths and asserts it never reaches the spool.
- [M] E2EE implication. Server-side sync metrics such as op counts, payload byte sizes and timing are metadata the server already sees. Publish this in the privacy policy as "metadata visible to server". Don't log ciphertext sizes per item in detail, because sizes can leak content type. Bucket them.
- [M] Differential privacy is overkill for a 2-month ship. Aggregate on the client into histograms and counters per flush window, which gives k-anonymity-ish privacy cheaply. Revisit Prio/DAP-style aggregation later if you add usage analytics for `time`.
- [M] Apple. Avoid FirebaseCrashlytics if you want to stay off the listed-SDK path. Self-hosted OTel plus a small crash handler, or Sentry, keeps the manifest simple. Declare "Crash Data", "Performance Data" and "Other Diagnostic Data" as not linked to identity and not used for tracking.

### Gaps
- Official PIPC guidance on whether telemetry with a rotating ID counts as personal information was not fetched.
- The EDPB final text on telemetry/SDK use cases was not read.
- The official wording of the Chrome Web Store 2026 policy update was not confirmed.

## 4. Sync-engine observability metrics (product health and thesis load tests)

### Takeaway
Define a small set of metrics measured at both ends and join them through a mutation ID plus a W3C trace context stored in the outbox row. Use OTel span links, not parent/child spans, across the offline gap, as the messaging semantic conventions recommend. Server-side metrics are authoritative for load tests. Client-side metrics show the user-perceived experience.

### Cited Findings
- [F] OTel messaging semantic conventions correlate producer and consumer with span links, not parent/child, because "the stages of producing and consuming a message can be separated by a considerable time gap". PRODUCER/CONSUMER span kinds ensure the links exist. A batch span can link to many producer spans. Invalid extracted contexts should not create links. The messaging conventions are still in Development status. — [OTel messaging spans](https://opentelemetry.io/docs/specs/semconv/messaging/messaging-spans/); [OTEP 0220](https://github.com/open-telemetry/oteps/blob/dc619dfc70f174ef31caf90f14e8b00600da4049/text/trace/0220-messaging-semantic-conventions-span-structure.md); [SigNoz span links](https://signoz.io/docs/traces-management/guides/span-links/)
- [F] PowerSync's client-health primitives are connected/connecting, hasSynced, lastSyncedAt, uploading, upload queue count, and upload/download errors. Missing-data debugging compares `lastSyncedAt` against the change timestamp. Server-side it reports `replication_lag_bytes`. — [PowerSync production readiness](https://docs.powersync.com/maintenance-ops/production-readiness-guide); [PowerSync Nuxt diagnostics](https://powersync-ja.github.io/powersync-js/nuxt-sdk/functions/usePowerSyncInspectorDiagnostics)

### Inferences
- [M] Proposed metric set (OTel names, low cardinality: no user ID labels, only platform, app version and product):

| Metric | Type | Where | Definition |
|---|---|---|---|
| `sync.outbox.depth` | gauge | client | Pending mutations in the SQLite outbox, sampled at flush |
| `sync.outbox.oldest_age` | gauge (s) | client | now minus the created_at of the oldest pending mutation (offline duration proxy) |
| `sync.push.latency` | histogram | client and server | Client: send to ack. Server: receive to commit in the Postgres log |
| `sync.commit_to_visible` / convergence | histogram | server-stamped, client-observed | Server commit_ts to the moment another device applies the op. Measured on the receiving device as apply_wall_time minus server_commit_ts, corrected by the skew estimate |
| `sync.local_to_remote_visible` | histogram (load test) | harness | Device A local write to device B apply. Only reliable in the test harness with one clock |
| `sync.mutation.rejected` | counter by reason | server | Validation, auth, schema-version and quota rejects / total |
| `sync.conflict` | counter by resolution | server or client | Op needed rebase or LWW override. Ratio to total ops |
| `sync.pull.lag_ops` | gauge | client | server_head_seq minus client_cursor at connect |
| `blob.upload.backlog_bytes` / `_count` | gauge | client | Pending E2EE blobs (clip images/files) |
| `blob.orphan_bytes` | gauge | server (GC job) | Stored blobs unreferenced by any live op after the grace period |
| `push.delivery_latency` | histogram | server send to client receive | APNs/FCM/WebSocket notify to client pull start |
| `sync.connect` | counter + rate | server | Connects per second. A reconnect storm is a spike relative to a baseline or more than N reconnects per device per minute |
| `sync.reconnect.backoff` | histogram | client | Backoff delay actually applied (checks the jitter is working) |
| `clock.skew` | histogram | server | server_receive_ts minus client_sent_ts minus RTT/2, per handshake, bucketed. Never stored per device ID |
| `telemetry.dropped` | counter | client | Spool evictions (meta-metric) |

- [M] Correlation across offline. Store `mutation_id` (ULID/UUIDv7), `traceparent` and `client_created_at` in the outbox row; these are plaintext envelope fields, not inside the E2EE payload. The client opens a short PRODUCER span `sync.enqueue` at write time. When it flushes, a `sync.push` span links to each mutation's context. The server's CONSUMER span `sync.commit` links to the same contexts and records `server_seq`. The receiving device's `sync.apply` span links using the traceparent carried in the log entry. The traceparent is random, so it leaks nothing. Strip it when the user has not opted into tracing; then only the `mutation_id`, which is already part of sync, remains for server-side logs.
- [M] Load tests (thesis). Drive N simulated clients against the Go server and export server metrics to Prometheus. Measure per-op p50/p95/p99 commit latency, convergence time under fan-out (1 writer to K devices), recovery time after a mass reconnect (all clients come back after an outage, with and without jitter), Postgres log write throughput, and outbox drain time after X hours offline with Y ops. Report harness-measured convergence with a single clock. Don't use client wall clocks for this.

### Gaps
- No published metric taxonomy from Zero, ElectricSQL or Linear was checked, because the search budget was exhausted.

## 5. Server side: Collector, LGTM, ClickHouse, cost

### Takeaway
For a small team, run one OTel Collector (persistent queue on) in front of either the Grafana Cloud free tier or a single-VM self-hosted LGTM stack, plus Prometheus metrics from the Go server. Postgres or ClickHouse for product events can wait. Self-host if data residency in Korea or "no third parties" is a brand promise.

### Cited Findings
- [F] Grafana Cloud free tier: 10k active metric series, 50 GB per month each for logs, traces and profiles, 14-day retention, 3 users. Pro: $19 per month platform fee plus usage (metrics $6.50 per 1k series beyond the 10k included; logs and traces $0.05/GB processing, $0.40/GB write, $0.10/GB retained). Pro retention: 13 months for metrics, 30 days for logs and traces. — [Grafana pricing](https://grafana.com/pricing/)
- [F] The OTel Collector storage extension and persistent queue keep batches on disk until the backend acknowledges them, which survives collector restarts. — [oneuptime storage extension](https://oneuptime.com/blog/post/2026-02-06-storage-extension-opentelemetry-collector/view); [oneuptime persistent queue](https://oneuptime.com/blog/post/2026-02-06-persistent-queue-storage-collector-reliability/view)
- [F] PostHog self-host suits low volume (about 100k events per month, about 4 GB RAM, secondhand source). — [Zima page](https://shop.zimaspace.com/pages/posthog-hardware-requirements)

### Inferences
- [M] Collector pipeline: `otlphttp` receiver (public, behind auth and rate limits) → `memory_limiter` → `attributes`/`transform` processors that drop any non-allowlisted attribute (a second scrub layer) → `batch` → exporters to Prometheus remote-write/Mimir, Loki, Tempo, and optionally ClickHouse. Disable IP capture by not adding `client.address`.
- [M] Client-side pre-aggregation keeps the setup inside 10k series and 50 GB per month for a launch user base.
- [M] Use ClickHouse only once product-analytics queries (funnels, retention) are actually needed. Before that, a Postgres `telemetry_events` table on a separate database or schema is enough and is easier to purge on consent revocation.

### Gaps
- OTel Collector ClickHouse exporter stability and the Loki/Tempo self-host footprint were not verified, because search was blocked.

## 6. Minimal viable telemetry for a 2-month ship

### Takeaway
Ship crash reporting, a few sync-health counters and histograms, and a server-side Prometheus dashboard. Defer product analytics, session replay and full client tracing.

### Cited Findings
- [F] Bounded disk queues with drop-oldest are the standard pattern (Sentry: 30 envelopes; PostHog: 1000 events; OTel java-contrib: 10 MB / 18 h). — [Sentry docs PR](https://github.com/getsentry/sentry-docs/pull/4566.diff); [PostHog iOS](https://posthog.com/docs/libraries/ios/configuration); [java-contrib disk-buffering](https://github.com/open-telemetry/opentelemetry-java-contrib/blob/main/disk-buffering/README.md)

### Inferences
- [M] Week 1–2: the Go server exports Prometheus metrics (push/commit latency, rejects, conflicts, connects per second, log head sequence, blob backlog/orphans) plus structured JSON logs carrying `mutation_id`. Grafana runs on the free tier or a VM.
- [M] Week 2–4: crash reporting. Use Sentry SDK on iOS/Android (opt-in; it works offline). Desktop and daemon get a panic hook that writes to the spool. The Go CLI gets an opt-in "send crash report?" prompt on the next run.
- [M] Week 3–5: the shared telemetry spool (Rust crate reused by Tauri and the daemon, a Go package for the CLI, IndexedDB for MV3). It sends OTLP/HTTP JSON metrics (counters and histograms only) for `sync.outbox.depth`, `oldest_age`, `push.latency`, `pull.lag_ops`, `reconnect`, `blob.backlog` and `telemetry.dropped`. It has an opt-in toggle, a rotating install ID and allowlisted attributes.
- [M] Before launch: write the privacy label, Data safety and Chrome Limited Use disclosures and the PIPA privacy policy listing every processor. Add the CI test ensuring clip content and window titles never reach logs.
- [M] Defer: product analytics (PostHog or Aptabase), client traces with span links (enable only for internal or dogfood builds and for the load-test harness), ClickHouse, and session replay. Never add session replay to clip or time.

### Gaps
- Battery and data cost figures for any SDK were not found. Only qualitative guidance exists (PostHog: `flushAt=1` costs more battery; OTel Android batches to save battery).
