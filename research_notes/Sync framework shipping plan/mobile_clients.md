# Mobile clients (iOS + Android) and the shared sync core — November 2026

Tagging: [F] = claim taken from a page fetched or a search result read in this session; [M] = from model memory (training data, cutoff June 2026) and not re-verified here. Note: the session-wide WebSearch budget ran out partway through, so several platform details (iOS file protection, BGContinuedProcessingTask, StoreKit 2) are [M] and should be verified before they go into a final spec.

## 1. Shared core: Rust + UniFFI vs Kotlin Multiplatform vs per-platform; Tauri 2 mobile vs RN/Expo vs native

### Takeaway
Use a single Rust sync-core crate (SQLite + outbox + log apply + blob CAS + crypto), exposed to Swift/Kotlin with UniFFI. Mozilla, Bitwarden and Matrix ship exactly this pattern in production. `time` already has a Rust core, so Tauri 2 mobile is the fastest way to ship it in 2 months, as long as you accept native Swift/Kotlin side targets for widgets and extensions. `clip` mobile is mostly OS extensions (share, keyboard, tile), so it is better as thin native SwiftUI/Compose apps on the same Rust core.

### Cited Findings
- [F] UniFFI says it is "ready for production use" but is pre-1.0, and "more advanced things might break as you upgrade." Mozilla uses it in Firefox mobile and desktop. — [mozilla/uniffi-rs](https://github.com/mozilla/uniffi-rs)
- [F] Mozilla's application-services adopted shared Rust components for sync because separate per-app sync implementations caused problems. The team that owns a component maintains its health. — [Firefox source docs: Developing Rust components](https://firefox-source-docs.mozilla.org/_sources/rust-components/developing-rust-components/index.md.txt)
- [F] Bitwarden's internal SDK is Rust. Mobile bindings (Swift, Kotlin) come from UniFFI, and web uses wasm-bindgen. Its guidance: "everything except presentational logic belongs in the SDK" (bitwarden-core + feature crates). — [Bitwarden SDK architecture](https://contributing.bitwarden.com/architecture/sdk); [Internal SDK](https://contributing.bitwarden.com/architecture/sdk/internal)
- [F] matrix-rust-sdk ships UniFFI bindings (matrix-sdk-ffi, matrix-sdk-crypto-ffi) for Kotlin and Swift, backs Element X, and has a sans-I/O crypto state machine with pluggable stores (matrix-sdk-sqlite). It replaced sled with SQLite. — [FOSDEM 2024 slides](https://archive.fosdem.org/2024/events/attachments/fosdem-2024-3283-the-state-of-the-matrix-rust-sdk-in-2023/slides/22921/FOSDEM_State_of_Rust_SDK_in_2023_eFY77Fu.pdf)
- [F] Stadia Maps' Ferrostar navigation SDK uses a UniFFI Rust core on iOS and Android. — [Stadia Maps blog](https://stadiamaps.com/blog/ferrostar-building-a-cross-platform-navigation-sdk-in-rust-part-1/)
- [F] One practitioner reports thin wrappers (207 lines of Kotlin, 113 of Swift) over 6,740 lines of Rust. Another post notes generated Kotlin bindings run past 1000 lines of boilerplate, and that UniFFI supports async Rust functions. These are opinion and anecdote, not benchmarks. — [levelup.gitconnected](https://levelup.gitconnected.com/the-headless-mobile-architecture-bypassing-the-kmp-internal-war-with-rust-99d759e02f50); [idverse blog, Nov 2025](https://forgestream.idverse.com/blog/20251105-crossplatform-business-logic-in-rust/)
- [F] uniffi-bindgen-react-native (Mozilla + Filament, released Dec 2024) generates TypeScript/JSI Turbo Modules from the same UniFFI Rust crate, and now WASM too. A rename to uniffi-bindgen-javascript is planned, and v0.31.x is listed. — [hacks.mozilla.org](https://hacks.mozilla.org/2024/12/introducing-uniffi-for-react-native-rust-powered-turbo-modules/); [GitHub](https://github.com/jhugman/uniffi-bindgen-react-native)
- [F] Tauri 2.0 RC post: you can ship production mobile apps, but "not all of our desktop features and plugins are ported or available on mobile yet," and 2.0 is not the "mobile as a first class citizen" release. — [Tauri blog](https://v2.tauri.app/blog/tauri-2-0-0-release-candidate/)
- [F] Tauri plugins call Swift on iOS and Kotlin/Java on Android. — [Tauri 2 alpha 4 post](https://v2.tauri.app/blog/tauri-2-0-0-alpha-4/)
- [F] Official plugin support table (plugins-workspace README):
  - **Supported (fully or partly) on iOS and Android:** notification, deep-link, biometric, clipboard-manager, sql, store, http, upload, dialog, opener, os, log, haptics, barcode-scanner.
  - **Unknown or planned on mobile:** fs, websocket, stronghold, global-shortcut, process, shell.
  - **Not supported on mobile:** autostart, updater, window-state, single-instance, positioner, cli.
  — [tauri-apps/plugins-workspace](https://github.com/tauri-apps/plugins-workspace)
- [F] Widgets: the community plugin tauri-plugin-widgets does *not* build the extension. You add a Widget Extension target in Xcode and share data via App Group JSON plus `WidgetCenter.reloadAllTimelines()`, with a ~40–70 refreshes/day budget and a default throttle of 900 s. — [docs.rs tauri-plugin-widgets](https://docs.rs/crate/tauri-plugin-widgets/0.3.0/source/README.md)
- [F] Background work: the community plugin tauri-plugin-background-service uses a foreground service on Android and BGTaskScheduler on iOS ("~30 seconds every 15+ minutes", best-effort). Push: the community plugin tauri-plugin-mobile-push (APNs/FCM) is at v0.1. — [docs.rs](https://docs.rs/crate/tauri-plugin-background-service/0.5.2); [GitHub](https://github.com/yanqianglu/tauri-plugin-mobile-push)
- [F] One team's May 2026 internal evaluation rejected Tauri mobile ("closer to alpha", no reliable official camera plugin). This is a single, non-authoritative source. — [range-day-docs](https://range-day-docs.pages.dev/decisions/Tauri-vs-Capacitor)
- [F] PowerSync lists its Tauri SDK as **alpha** and its Rust SDK as **beta**. — [PowerSync client SDKs](https://docs.powersync.com/client-sdk-references/introduction)

### Inferences
- **Core boundary.** One crate (`skkil-sync`) owns the following, behind a UniFFI interface:
  - rusqlite with bundled SQLite
  - migrations
  - outbox of named mutations
  - pull/apply of the server log
  - CAS blob store and the upload queue state machine
  - E2EE (with the key supplied by the platform)
  
  The interface is `open(path, key_handle)`, `mutate(name, args)`, `sync_once() -> Report`, `subscribe(query)`, `blob_put/get`. The platform layers provide only networking triggers (URLSession/WorkManager), keystore access and UI. This mirrors Bitwarden's rule that everything except presentation lives in the SDK.
- **Make `sync_once()` resumable and bounded.** Mobile background windows are seconds long (see section 3), so the core must be able to run in a 25 s slice, commit progress and exit cleanly. Design for that from day one.
- **Networking.** Let the Rust core do the connect-rpc/HTTP sync calls for foreground sync. Blob uploads and downloads should be handed to the platform (background URLSession / UIDT jobs) so they survive suspension. The core emits "upload this file at path X to URL Y" tasks and gets completion callbacks.
- **`time` on mobile.** Tauri 2 mobile reuses the existing Rust + React UI, and clipboard, notification, sql/store, deep-link, biometric and upload are all supported. Widgets, a share extension and background refresh still require hand-written Swift/Kotlin targets. In 2 months, ship the main app via Tauri and leave widgets for v1.1. The fallback if Tauri mobile proves too rough is React Native/Expo plus uniffi-bindgen-react-native over the same core. That reuses React skills, but the UI is not code-shared with Tauri's DOM React.
- **`clip` on mobile.** The valuable surfaces are a share extension, a keyboard extension (iOS), and a Quick Settings tile, notification action and IME (Android). These are native-only extension targets. A Tauri webview app adds little, so thin SwiftUI and Compose apps that call the Rust core via UniFFI are the lower-risk choice.
- **KMP.** KMP is a poor fit here, because the desktop daemon, the Tauri app and the core are Rust already. KMP would mean a second implementation of the sync core, which is exactly what Mozilla moved away from.
- **2-month risk.** UniFFI cross-compilation for 4+ targets (aarch64-apple-ios, the simulator target, aarch64/armv7/x86_64 android), XCFramework packaging, and an AAR via cargo-ndk usually take 1–2 weeks to stabilize in CI [M].

### Gaps
- No official 2026 Tauri mobile roadmap or status post was found. The plugin-table snapshot date is unknown.
- No direct KMP-vs-UniFFI comparison from a credible engineering org was found.
- 1Password's, Automerge's, Loro's and cr-sqlite's mobile binding approaches were not verified in this session. [M]: Automerge has Swift bindings built with UniFFI (automerge-swift), Loro has loro-swift/loro-kotlin via UniFFI, and 1Password's "core" is Rust shared across clients.

## 2. SQLite on mobile

### Takeaway
Bundle SQLite (rusqlite `bundled`) so every platform gets the same version and features. Keep the main DB in the app's private container with WAL and `CompleteUntilFirstUserAuthentication`. Do **not** put the live SQLite DB in an iOS App Group shared with widgets or extensions: give extensions a small inbox file or a JSON snapshot instead.

### Cited Findings
- [F] iOS kills suspended processes that hold a file or SQLite lock, with exception 0xdead10cc. Apple's watchdog was tightened for this. — [Apple forums 655225](https://developer.apple.com/forums/thread/655225?page=2); [Apple SIGKILL docs via mjtsai](https://mjtsai.com/blog/2025/05/15/sqlite-databases-in-app-group-containers-dont/)
- [F] Marco Arment, Ryan Ashcraft and Gwendal Roué (GRDB) say SQLite in an App Group container causes persistent 0xdead10cc crashes:
  - Wrapping every query in a background task piles up power assertions.
  - GRDB's `suspendNotification` mitigation makes queries fail with SQLITE_INTERRUPT/ABORT.
  - Their recommendation is to keep the DB in the app's own container and use plist/JSON files and Darwin notifications for extension IPC.
  — [mjtsai roundup, May 2025](https://mjtsai.com/blog/2025/05/15/sqlite-databases-in-app-group-containers-dont/); [Ryan Ashcraft](https://ryanashcraft.com/sqlite-databases-in-app-group-containers/); [GRDB DatabaseSharing](https://swiftpackageindex.com/groue/GRDB.swift/v7.5.0/documentation/grdb/databasesharing)
- [F] Opening a Realm in an App Group from WidgetKit reproduced 0xdead10cc every time. — [realm-swift #8017](https://github.com/realm/realm-swift/issues/8017)
- [F] With SQLCipher, iOS's exemption for plain SQLite headers no longer applies. The fix is to leave the first 32 header bytes unencrypted. — [GRDB #302](https://github.com/groue/GRDB.swift/issues/302)
- [F] PowerSync Swift v1.16.4 (2026-10-08) bundles its own SQLite 3.53.4, a vendor precedent for bundling. — [PowerSync release notes](https://releases.powersync.com/announcements/powersync-kotlin-sdk)

### Inferences
- **iOS file protection [M].** Use `NSFileProtectionCompleteUntilFirstUserAuthentication` on the DB, `-wal` and `-shm`, so BGTasks and silent pushes can sync while the phone is locked. `Complete` breaks background sync.
- **E2EE and a stolen unlocked phone.** If you need to protect against this, use app-level encryption: SQLCipher, or encrypting payload columns with a Keychain key.
- **WAL [M].** WAL is fine in the private container. Run `wal_checkpoint(TRUNCATE)` when the app backgrounds, and close or idle connections before suspension to avoid holding locks.
- **Extensions write to an inbox, not the DB.** Share and keyboard extensions write append-only "inbox" files (one JSON or blob file per capture) into the App Group. The main app or a BGTask imports them into SQLite as `capture_clip` mutations. Widgets read a JSON snapshot that the app writes. This is the same pattern as tauri-plugin-widgets.
- **Android.** Use rusqlite from the Rust core rather than Room, since Room would duplicate the schema. App widgets, the tile and the IME all run in the app's own process/UID, so the DB can be shared directly. The 0xdead10cc problem is iOS-specific [M].

### Gaps
- Apple's "Encrypting your app's files" page did not render on fetch, so the protection-class semantics above are [M].

## 3. Background execution and blob transfer

### Takeaway
Neither OS gives a sync engine reliable periodic background time. Sync on foreground and on app launch, nudge with push, and hand large blob transfers to OS-managed transfer APIs: background `URLSession` on iOS, UIDT jobs or WorkManager on Android. The engine must treat every background window as a short, interruptible slice.

### Cited Findings
- [F] iOS throttles background (`content-available`) pushes for apps in the background. Estimates of the device budget vary (1–2/hour on Apple forums, ~2–3/hour from vendors), and delivery is never guaranteed. Pushes need `apns-push-type: background` and priority 5. — [Apple forums 837675](https://developer.apple.com/forums/thread/837675); [Pushwoosh](https://www.pushwoosh.com/blog/silent-push-notifications/)
- [F] BGTaskScheduler refresh windows are about 30 s every 15+ minutes and best-effort (from a plugin's docs). — [tauri-plugin-background-service](https://docs.rs/crate/tauri-plugin-background-service/0.5.2)
- [F] Background URLSession tasks are stopped if the user force-quits (swipes away) the app (Apple DTS, Quinn). — [Apple forums 765196](https://developer.apple.com/forums/thread/765196)
- [F] iOS 17 URLSession supports pausing and resuming uploads (`cancelByProducingResumeData`, `uploadTask(withResumeData:)`). This only works when the server implements the IETF resumable-upload draft that URLSession detects. — [WWDC23 10006](https://developer.apple.com/videos/play/wwdc2023/10006/)
- [F] On apps targeting Android 15+, `dataSync` (and `mediaProcessing`) foreground services get 6 h per 24 h in the background. Then `onTimeout()` fires, and you must `stopSelf()` within seconds or crash. Starting another one throws `ForegroundServiceStartNotAllowedException` until the user foregrounds the app. Google recommends WorkManager or other data-transfer APIs instead. — [Android FGS timeout](https://developer.android.com/develop/background-work/services/fg-service-timeout)
- [F] Android 14+ user-initiated data transfer (UIDT) jobs:
  - They use `JobInfo.setUserInitiated(true)`, need the `RUN_USER_INITIATED_JOBS` permission and a notification, and must be scheduled while the app is visible.
  - They are exempt from App Standby Bucket quotas and can run long, but are stopped on constraint loss, thermal pressure or low memory. Apps must persist transfer state.
  - There is no Jetpack wrapper, so use the WorkManager foreground worker as a fallback before Android 14.
  - Google Maps improved download reliability by 10% with UIDT.
  — [Android UIDT](https://developer.android.com/develop/background-work/background-tasks/uidt)

### Inferences
- **Sync triggers, in priority order:** (1) app foreground/launch, (2) after every local mutation while foregrounded (debounced), (3) a silent push from the server when the user's log advances (rate-limited server-side to roughly ≤2/h per device), (4) BGAppRefreshTask or a WorkManager periodic job (≥15 min) as a catch-up.
- **Visible notifications for `time`.** Reminders should be local notifications scheduled from the synced data. Don't depend on background sync for them [M].
- **Blob uploads.**
  - **iOS:** The core chunks content-addressed blobs (for example 4–8 MiB parts, each with a presigned PUT) and writes the parts to files, because background URLSession requires file uploads [M]. Each part becomes a background upload task, and completion is reconciled on relaunch via `handleEventsForBackgroundURLSession`. Chunked presigned parts make resume server-agnostic, with no dependence on the IETF draft.
  - **Android:** Use a WorkManager unique-work chain per blob for small, interruptible transfers, and UIDT for user-initiated big uploads (screenshots, bulk import).
- **BGContinuedProcessingTask [M].** iOS 26 added this for user-initiated long work that continues in the background with system progress UI. It is a candidate for "export/backup now" flows. Verify against Apple's docs (the page did not render on fetch).
- **Avoid long FGS for sync on Android 15+.** Google Play also requires a declared justification for FGS types [M].

### Gaps
- Apple's current official silent-push budget is not documented. All numbers are forum or vendor estimates.
- BGContinuedProcessingTask details are [M] and unverified (the fetch returned no body).

## 4. Clipboard on mobile (critical for `clip`)

### Takeaway
Neither OS allows background clipboard monitoring. Mobile `clip` therefore has to be explicit-capture plus easy paste-out. On iOS that means the share extension, a custom keyboard (history plus paste), UIPasteControl in-app, and Shortcuts/App Intents. On Android it means a notification action or Quick Settings tile that briefly foregrounds an activity, the share target, and optionally an IME. Universal "copy on phone, appears on desktop" happens only when the user opens the app or taps a capture affordance.

### Cited Findings
- [F] Since iOS 16, programmatic reads of `UIPasteboard.general` trigger a user prompt. UIPasteControl (and the system Paste menu) paste without the prompt. iOS 16.1+ adds a per-app "Paste from Other Apps" setting. — [Apple UIPasteControl](https://developer.apple.com/documentation/uikit/uipastecontrol); [sarunw](https://sarunw.com/posts/uipasteboard-privacy-change-ios16/); [MacRumors](https://forums.macrumors.com/threads/ios-16-1-introduces-new-copy-and-paste-permission-settings-on-per-app-basis.2366005)
- [F] Firefox iOS uses UIPasteControl and checks `UIPasteboard.general.changeCount`, which requires no permission, to detect that the clipboard changed before offering paste. — [firefox-ios PR 20200](https://www.github.com/mozilla-mobile/firefox-ios/pull/20200)
- [F] Paste (pasteapp.io) on iPhone records history only when you open Paste after copying, share via its share extension, or copy through the Paste Keyboard. It states it cannot access the clipboard in the background, and syncs with Mac/iPad. — [Paste help](https://pasteapp.io/help/clipboard-history-on-iphone)
- [F] Android 10+: only the default IME or the focused app can read the clipboard. — [Android secure clipboard](https://developer.android.com/privacy-and-security/risks/secure-clipboard-handling); [Microsoft Security blog](https://www.microsoft.com/en-us/security/blog/?p=126288)
- [F] Android 12+ shows a toast when an app reads the clipboard. Android 13+ auto-clears the clipboard after a time period. Apps should mark sensitive clips with `ClipDescription.EXTRA_IS_SENSITIVE` so the preview is hidden. — [Android secure clipboard](https://developer.android.com/privacy-and-security/risks/secure-clipboard-handling)
- [F] KDE Connect's answer to Android 10 is a persistent-notification "Send Clipboard" action, which foregrounds the app briefly to read the clipboard. Log-watching (READ_LOGS via adb) and IME-switching hacks exist but are fragile and broke on Android 13 for some users. — [KDE bug 417419](https://bugs.kde.org/show_bug.cgi?id=417419); [MR 145](https://invent.kde.org/network/kdeconnect-android/-/merge_requests/145); [KDE bug 446366](https://bugs.kde.org/show_bug.cgi?id=446366)
- [F] Tauri's clipboard-manager plugin is listed as (partially) supported on iOS and Android. — [plugins-workspace](https://github.com/tauri-apps/plugins-workspace)

### Inferences
- **iOS capture paths for v1:**
  - in-app UIPasteControl "Save clipboard" button, shown when `changeCount` differs
  - Share extension (text, URL, image), which writes to the App Group inbox
  - App Intent "Save clipboard to clip" for Shortcuts and the Action button
- **iOS keyboard extension (v1.1).** It browses history and inserts text; inserting requires no network. Reading the pasteboard or syncing from the keyboard requires "Allow Full Access", which triggers a scary prompt and review scrutiny [M]. The keyboard should read a local snapshot file rather than open the DB.
- **Android capture paths:**
  - Quick Settings tile or notification action that launches a transparent activity, reads the clipboard while focused, and finishes (the KDE Connect pattern)
  - share target
  - optional IME later
- **Paste-out.** Setting the clipboard is allowed on both OSes. Desktop → phone sync lands in history, and the user taps to copy. Mark E2EE secrets with `EXTRA_IS_SENSITIVE`.
- **Universal Clipboard.** Apple's own Universal Clipboard (Handoff) is OS-private and gives no third-party API [M]. Samsung's clipboard sync with Windows Phone Link is also OS-level [M]. Neither can be replicated by an app.

### Gaps
- No primary source fetched for Pushbullet's or Samsung's current mobile clipboard behavior.
- iOS keyboard Full Access rules were not fetched [M].

## 5. Files on mobile and "two-way folder mounts"

### Takeaway
Mobile has no user-visible arbitrary folders to mount. Ship mounts as desktop-only. On mobile, expose the CAS blobs as attachments and screenshots inside the app, and plan a File Provider extension (iOS) and a DocumentsProvider (Android) later if users want the folder in the Files app.

### Cited Findings
- [F] Apple File Provider versions content and metadata separately and hands conflicts to the app with a base version. This is already noted in the sync design. — [Apple NSFileProviderItemVersion](https://developer.apple.com/documentation/fileprovider/nsfileprovideritemversion)

### Inferences [M]
- **iOS.** Apps see their sandbox. Folders the user picks via `UIDocumentPickerViewController` give security-scoped URLs that can persist as bookmarks, but iOS cannot watch them for changes in the background. A two-way mount on iOS is realistic only as a File Provider *replicated* extension: the app becomes a "cloud drive" in Files.app, a separate project of several weeks.
- **Android.** Under scoped storage plus SAF, `ACTION_OPEN_DOCUMENT_TREE` gives persistable tree URIs. There is no reliable background watcher (FileObserver does not work on SAF URIs), and `MANAGE_EXTERNAL_STORAGE` is Play-restricted. A periodic WorkManager rescan of a SAF tree is possible but slow. A DocumentsProvider is the cleaner "appear in Files" path.
- **For the 2-month plan.** The mobile replica set is {SQLite, CAS}, and the `fs_entry` mount replica stays desktop-only. Keep the codec and mount code out of the mobile build behind a Cargo feature.
- **Screenshots on Android/iOS.** Importing the user's screenshots needs photo-library permission (`PHPicker` needs none for user-picked items). Auto-import of new screenshots is possible on Android via a MediaStore query on foreground, and on iOS via PhotoKit change observation while the app runs.

### Gaps
- Nothing was fetched this session on SAF or File Provider beyond the conflict-versioning doc. The points above are [M].

## 6. E2EE key storage on mobile

### Takeaway
Store the account root key, or a key that wraps it, in Keychain or Android Keystore. Recover with a recovery code plus an optional passkey-PRF unlock: passkey PRF now works on iOS 18+ and Android (API 28+ via Credential Manager), and synced passkeys produce the same PRF output across devices.

### Cited Findings
- [F] Passkey PRF requirements:
  - It is available on iOS 18.0+ (AuthenticationServices) and Android API 28+ (Credential Manager).
  - It is unavailable on simulators and emulators.
  - It must be enabled when the passkey is created.
  - The output matches across devices only if the passkey syncs (iCloud Keychain / Google Password Manager).
  — [maktub_passkey docs](https://pub.dev/documentation/maktub_passkey/latest/)
- [F] The PRF output goes through HKDF-SHA-256 to derive a client-only key-encryption key. — [Tinfoil passkey kit](https://docs.tinfoil.sh/guides/passkey-kit)
- [F] Google Password Manager includes PRF by default, and iOS 18.4 fixed earlier cross-device PRF bugs (vendor article, 2026 update). — [Corbado](https://www.corbado.com/blog/passkeys-prf-webauthn)
- [F] 1Password announced passkey PRF for encrypting data in July 2024. — [1Password blog](https://1password.com/blog/encrypt-data-saved-passkeys)

### Inferences
- **Key hierarchy (sketch).** A random 256-bit account key (AK) encrypts the per-collection data keys, and AK is stored in three wrapped forms:
  1. **Device:** AK wrapped by a device key. On iOS this is a Keychain item with `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, so background sync works while locked. On Android it is a Keystore AES key (StrongBox if available) used to wrap AK [M].
  2. **Recovery:** AK wrapped by a recovery-code-derived key (Argon2id), shown once at signup.
  3. **Passkey (optional):** AK wrapped by a PRF-derived key, for "sign in on a new phone with your passkey."
- **Avoid Secure Enclave / StrongBox-only keys for AK itself.** The Rust core needs symmetric AK bytes in memory. Use the Secure Enclave to wrap or gate AK, not to perform the data crypto [M].
- **Biometric gating.** Biometric gating (Tauri biometric plugin, `LAContext`, BiometricPrompt) should be optional UI lock, not the key-access policy. A biometric-bound key would block background sync [M].
- **Platform key callback.** Expose a `KeyProvider` callback interface in UniFFI so the platform supplies unwrap operations. The Rust core never touches the keystore APIs.

### Gaps
- No Apple or Google primary docs on PRF were fetched. Plugin docs label PRF "experimental," so test on real devices.

## 7. App Store / Play constraints and subscriptions

### Takeaway
Plan for privacy manifests (required-reason APIs) on iOS, Data Safety forms on Play, and justified background modes and FGS types. For the ~$10/mo `time` sync plan, use StoreKit 2 and Play Billing through RevenueCat for cross-platform entitlement. In the US, since the Epic v. Apple ruling (Apr–May 2025), you may also link out to web checkout.

### Cited Findings
- [F] Privacy manifests:
  - Apps that omit a privacy manifest, or that use required-reason APIs without declared reasons, are rejected (ITMS-91053).
  - UserDefaults is a required-reason API (CA92.1, or 1C8F.1 for App Groups).
  - File-timestamp and disk-space APIs also need reasons, and SQLite wrappers commonly trigger disk-space and file-timestamp declarations.
  - Reason codes vary between secondary sources, so verify them on Apple's page.
  — [Nutrient](https://www.nutrient.io/blog/privacy-manifests-on-ios/); [Donny Wals](https://www.donnywals.com/how-to-add-a-privacy-manifest-file-to-your-app-for-required-reason-api-usage/); [LiveCode](https://lessons.livecode.com/m/4069/l/1800674-how-to-create-a-privacy-manifest-and-include-it-in-your-ios-app)
- [F] In April 2025 the court enjoined Apple's US External Purchase Link restrictions. Apple updated its guidelines in May 2025, and its stay request was denied. Sources conflict on whether any commission remains on US link-outs. — [Median](https://median.co/blog/epic-v-apple-payment-links-ruling-explained); [Mondaq/FKKS](https://www.mondaq.com/unitedstates/consumer-law/1621598/court-finds-apple-violated-order-resulting-in-key-changes-for-ios-external-purchase-methods)
- [F] RevenueCat supports app-to-web purchase flows (Web Purchase Link, Web SDK, paywall button), and states the ruling applies only to the US storefront. — [RevenueCat Web FAQ](https://www.revenuecat.com/docs/web/faqs)
- [F] Android 15 restricts `dataSync` FGS (see section 3). — [Android](https://developer.android.com/develop/background-work/services/fg-service-timeout)

### Inferences [M]
- **iOS background modes.** Request only "Background fetch" (BGAppRefresh), "Remote notifications" and, if needed, "Background processing". Don't request audio or location tricks, which get rejected.
- **iOS privacy manifest for the Rust core.** Statically linked Rust code still needs the app's manifest to declare file timestamp (`stat`/mtime) and disk-space usage if it calls them.
- **Play Data Safety.** E2EE data is still "collected" if it is sent off-device. Declare the encryption in transit, and the deletion path.
- **Account deletion.** Apple and Google both require in-app account deletion for apps with account creation.
- **Subscriptions.**
  - StoreKit 2's `Transaction.currentEntitlements` plus App Store Server Notifications v2 go to the Go server. Play Billing uses RTDN via Pub/Sub.
  - RevenueCat abstracts both and gives a single webhook keyed by `app_user_id`, which saves roughly 1–2 weeks in a 2-month plan. The cost is a fee on revenue above a free tier; verify current pricing.
  - Keep entitlement checks server-side, so the sync server refuses paid-only sync without an active entitlement.
- **Reader-app exemption.** It does not apply to `time`. You need in-app purchase availability on iOS outside the US even if you also sell on the web.

### Gaps
- No current Apple App Review Guidelines or Play policy pages were fetched. Commission terms on US link-outs are conflicting.
- RevenueCat pricing was not verified.

## 8. Off-the-shelf sync SDKs with mobile SDKs (build vs buy)

### Takeaway
PowerSync is the most relevant "buy" option: Postgres-backed, SQLite client, Swift, Kotlin, RN, Flutter, with Rust in beta and Tauri in alpha. However, it does not natively give you E2EE (server-side sync rules read plaintext rows), and you would still build blobs, mounts and crypto yourself. Ditto and Couchbase Lite are document/CRDT stores that don't match a Postgres plus named-mutations design. Building on your own Rust core remains the coherent choice. Borrow PowerSync's client patterns (bundled SQLite, an upload queue of CRUD ops, a connector `uploadData()` callback).

### Cited Findings
- [F] PowerSync client SDKs:
  - JS Web, React Native & Expo, Node, Capacitor (beta), Tauri (alpha), Dart/Flutter, Kotlin, Swift, .NET (beta), Rust (beta).
  — [PowerSync docs](https://docs.powersync.com/client-sdk-references/introduction)
- [F] PowerSync release history:
  - Kotlin SDK v1.12.0 shipped 2026-05-06.
  - Swift SDK v1.14.1 (2026-06-02) dropped the precompiled Kotlin/KMP build, so it is now native Swift.
  - Swift v1.16.4 (2026-10-08) bundles SQLite 3.53.4.
  - PowerSync's Swift SDK had previously been built on KMP via SKIE.
  — [PowerSync releases](https://releases.powersync.com/announcements/powersync-kotlin-sdk); [PowerSync blog on SKIE](https://www.powersync.com/blog/building-a-swift-sdk-with-skie-lessons-in-bridging-kotlin-and-swift)
- [F] Ditto markets built-in P2P/mesh sync (BLE, P2P Wi-Fi, LAN) with CRDT conflict resolution. These are vendor claims. — [SourceForge listing](https://sourceforge.net/software/offline-first-databases/usa/)

### Inferences
- **PowerSync's KMP-to-native-Swift move.** PowerSync moved its Swift SDK off KMP to native Swift in mid-2026. This is a data point that bridging a KMP core into Swift had real costs.
- **PowerSync and E2EE [M].** PowerSync can carry E2EE payloads only as opaque columns, which loses server-side filtering. That conflicts less with this design (per-user log, no server-side query) than with most designs, but it buys little beyond what your own outbox and log already do.
- **Turso / libSQL embedded replicas, and SQLite Sync (sqlite.ai) [M].** Both ship mobile-capable clients, but their models (DB-level replication, CRDT extension) don't match named mutations plus E2EE.
- **Recommendation.** Build. The core is small (outbox, log cursor, apply, CAS), and the Rust core is reused across desktop daemon, Tauri app, CLI (via FFI or reimplementation in Go) and mobile.

### Gaps
- No vendor docs were fetched for Ditto, Couchbase Lite, SQLite Sync or Turso mobile in this session (search budget exhausted). Their current SDK and E2EE status is unverified.
- PowerSync's per-SDK stability labels for Swift and Kotlin were not shown on the fetched page.
