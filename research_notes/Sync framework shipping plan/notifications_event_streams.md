# Notifications and event streams for the skkil sync framework (clip + time), as of 2026-11-04

Tags: [F] = read in a primary source during this session (2026-11-04). [M] = model knowledge or inference, not re-verified this session. Web search budget ran out midway (shared session cap), so several items that would normally be verified are marked [M] or listed under Gaps.

Context assumed (from `reports/Sync engine thesis novelty check.md` §"The server is distributed", rules 1–9, and `ARCHITECTURE.md` §9): server-ordered per-scope log in Postgres; poke-then-pull over a lossy bus; Kafka only for service-to-service; outbox relay at-least-once with version-gated consumers; optional E2EE.

## 1. Push: APNs, FCM, Web Push (Chrome extension), desktop; silent push as sync wake-up; self-hosted alternatives

### Takeaway
Every push channel is lossy, coalescing, and throttled. iOS background pushes are capped at "two or three per hour" and are dropped after force-quit. FCM high-priority messages are downgraded if they don't produce visible notifications. Treat push only as a "poke" that triggers a pull from the log, never as the delivery channel, and never rely on silent push for `clip`'s near-real-time delivery while an app is backgrounded.

### Cited Findings
**APNs** [F]
- Token auth: JWT signed with ES256 only. Refresh the token "no more than once every 20 minutes and no less than once every 60 minutes". APNs rejects tokens with a timestamp more than one hour old. — [Apple: token-based connection](https://developer.apple.com/documentation/usernotifications/establishing-a-token-based-connection-to-apns)
- Payload max 4 KB (4096 bytes), or 5 KB for VoIP. The JSON payload must not be compressed. — [Apple: sending requests](https://developer.apple.com/documentation/usernotifications/sending-notification-requests-to-apns)
- `apns-collapse-id` merges multiple notifications into one on the device. The value must not exceed 64 bytes. — [Apple: sending requests](https://developer.apple.com/documentation/usernotifications/sending-notification-requests-to-apns)
- `apns-push-type` is "required for watchOS 6 and later; recommended for macOS, iOS, tvOS, and iPadOS". — [Apple: sending requests](https://developer.apple.com/documentation/usernotifications/sending-notification-requests-to-apns)
- APNs may reorder notifications sent to the same device token. When the device is offline it stores notifications for up to 30 days, depending on `apns-expiration`, and keeps **only one stored notification per bundle ID**, usually the latest but not guaranteed. Priority 5 and 1 pushes "might get grouped and delivered in bursts", and may be "throttled, saved in storage, and in some cases, not delivered." — [Apple: sending requests](https://developer.apple.com/documentation/usernotifications/sending-notification-requests-to-apns)
- Background (silent) push: `content-available: 1`, `apns-push-type: background`, `apns-priority: 5`. Apple: "the system doesn't guarantee their delivery… may throttle… don't try to send more than two or three per hour." The system holds only the newest background notification. "If something force quits or kills the app, the system discards the held notification." The app gets 30 seconds to run. — [Apple: pushing background updates](https://developer.apple.com/documentation/usernotifications/pushing-background-updates-to-your-app)

**FCM HTTP v1** [F]
- Notification messages are displayed automatically by the SDK in the background and delivered to a callback in the foreground. Data messages carry only custom key-value pairs and are handled by app code in all states. Combined messages deliver the data payload only when the user taps the notification while the app is backgrounded. Maximum payload is 4096 bytes. — [Firebase: set message type](https://firebase.google.com/docs/cloud-messaging/customize-messages/set-message-type)
- Normal priority (the default) is delayed during Doze. High priority "FCM attempts to deliver… immediately" and can wake the device with "very limited network access". The handler gets several seconds in `onMessageReceived`; longer work goes to WorkManager (expedited for high priority). — [Firebase: message priority](https://firebase.google.com/docs/cloud-messaging/android/message-priority)
- **Deprioritization:** if high-priority messages don't result in user-visible notifications, FCM may downgrade them to normal or hand them to Google Play services for proxied display. This is evaluated over 7 days per app instance. Disabled notification permission also deprioritizes. Compare `getPriority()` with `getOriginalPriority()` to detect it. — [Firebase: message priority](https://firebase.google.com/docs/cloud-messaging/android/message-priority)
- Notification proxying is on by default for HTTP v1 on Android Q+ with recent Play services. It can be opted out per message (`proxy: DENY`) or in the manifest. — [Firebase: message priority](https://firebase.google.com/docs/cloud-messaging/android/message-priority)
- The same page mentions AndroidConfigV2 with `RemoteNotification` vs `BackgroundSync` message types, where urgency comes from the message type rather than a priority field. — [Firebase: message priority](https://firebase.google.com/docs/cloud-messaging/android/message-priority) [F; details of the V2 API rollout not verified]

**Web Push in an MV3 Chrome extension** [F]
- The extension service worker is woken by a push. Chrome's push service is FCM, and self-hosted providers such as `web-push` work. The `"notifications"` permission is required, otherwise `pushManager` fails. Adding it later disables existing installs until the user approves. — [Chrome: extension web push](https://developer.chrome.com/docs/extensions/how-to/integrate/web-push)
- From **Chrome 121**, `userVisibleOnly: false` is allowed in extensions, so silent pushes need no visible notification. Before 121, a visible notification was required. — [Chrome: extension web push](https://developer.chrome.com/docs/extensions/how-to/integrate/web-push)

**Self-hosted alternatives** [F]
- UnifiedPush is "a decentralized push notification system that lets you choose the service". Distributors on Android: Sunup, ntfy, Conversations, NextPush, gCompat-UP. On Linux: KUnifiedPush (KDE). Users include Element, Tusky, FluffyChat, Molly, DAVx5 and Nextcloud Talk. — [unifiedpush.org](https://unifiedpush.org/)

**Desktop** [M]
- Tauri/Linux (Wayland) has no OS push service comparable to APNs or FCM. The app must hold its own connection (WebSocket/SSE) while running and show notifications through the desktop notification API (freedesktop `org.freedesktop.Notifications` via D-Bus; Tauri's notification plugin). macOS desktop could use APNs, but that is unnecessary if the app is a resident tray app. [M]
- CLI: no push. Pull on invocation; optionally `--watch` holds an SSE stream. [M]

### Inferences
- **`clip` cross-device delivery**: in the foreground, over the persistent connection. When the target device is backgrounded, iOS cannot be woken reliably (2–3 per hour, dropped after force-quit), so a clip will appear on next open or via a visible alert push. Android high-priority data messages work only if they usually produce a visible notification; otherwise they are downgraded within 7 days. So a "silent wake to prefetch clips" design is fragile on both. Ship: sync on foreground and on connection, and use visible pushes only for user-relevant events (an optional "clip from Mac" alert). [M, derived from F]
- Use `apns-collapse-id` / FCM `collapse_key` = `scope:{id}` for sync pokes so that only the latest "head ≥ N" survives. That matches poke semantics exactly (the latest poke subsumes earlier ones), and APNs already keeps only one stored notification per bundle. [M]
- APNs reordering plus the one-stored-per-bundle rule means push payloads must be idempotent hints, `{scope, head_version}` at most. [M]
- Self-hosted push (UnifiedPush/ntfy) only matters for de-Googled Android users and F-Droid builds. Defer it; the poke-then-pull design makes adding a UnifiedPush distributor later a pure transport addition. Gotify was not checked this session. [M]

### Gaps
- Exact FCM per-device and per-project rate limits, TTL max (believed to be 4 weeks), the max of 4 collapse keys per device, and the 100 stored non-collapsible messages limit: the fetched pages did not state them. [M, unverified]
- Status of the `chrome.gcm` API and whether extension Web Push requires VAPID: not confirmed.
- Gotify and ntfy-specific iOS behavior (ntfy iOS relays through APNs) was not verified.

## 2. E2EE push: decrypting in the iOS Notification Service Extension and Android data messages

### Takeaway
The standard pattern is a content-free push (an ID or ciphertext under 4 KB) followed by local decryption or fetch: an NSE on iOS with `mutable-content: 1`, and a data message plus local decrypt on Android. Never put plaintext titles or bodies in the push. Apple's filtering entitlement, which lets the NSE drop a push silently, is gated and aimed at E2EE messaging apps, so plan for every alert push to display *something*.

### Cited Findings
- The NSE runs only for alert pushes with `mutable-content: 1` and an `alert` dictionary. "Notification service app extensions only operate on remote notifications configured… to display an alert." If alerts are disabled, or the payload only plays a sound or badges, the extension doesn't run. — [Apple: modifying content](https://developer.apple.com/documentation/usernotifications/modifying-content-in-newly-delivered-notifications) [F]
- `didReceive(_:withContentHandler:)` has "only about 30 seconds" to modify the content. After that, `serviceExtensionTimeWillExpire()` fires and the original content is shown. — [Apple: modifying content](https://developer.apple.com/documentation/usernotifications/modifying-content-in-newly-delivered-notifications) [F]
- The `com.apple.developer.usernotifications.filtering` entitlement (iOS 13.3+) lets an NSE suppress a notification by passing empty content. It requires an application to Apple and is "intended for encrypted messaging applications". Developers report rejections for other use cases. — [Apple forums 707406](https://developer.apple.com/forums/thread/707406), [apple-bundle docs](https://docs.rs/apple-bundle/latest/i686-unknown-linux-gnu/src/apple_bundle/entitlements/push_notifications.rs.html) [F, secondary]
- Matrix push gateway: a pusher can request `format: "event_id_only"`, in which case only `event_id`, `room_id`, `counts` and `devices` are sent. The client then fetches and decrypts the event. Gateways return a `rejected` list of dead pushkeys, which the homeserver must stop using. — [Matrix push gateway spec](https://spec.matrix.org/latest/push-gateway-api/) [F]
- Signal/WhatsApp specifics: no primary source was retrieved this session. Common understanding: Signal sends an empty wake push and the NSE fetches and decrypts messages from the server. [M]

### Inferences
- For skkil: push payload = `{scope_id, head_version}` (or an opaque notification ID) plus a generic `alert` such as "New reminder" or "New clip", localized. The NSE then (a) pulls the specific log entries or a server-held *encrypted notification record*, (b) decrypts with a key shared via the App Group keychain, and (c) rewrites the title and body. If decryption times out, the user sees the generic text, which leaks nothing. [M]
- Alternatively, the encrypted notification body can sit inside the push (it must fit under 4 KB including base64 and AEAD overhead). This avoids a network round trip inside the 30 s budget. Encrypt to each device's key, or to a per-user notification key wrapped per device, which reuses the `media_key_envelopes` idea from ARCHITECTURE §10. [M]
- Android: send FCM **data** messages (never `notification` messages, which the SDK auto-displays with plaintext) with the ciphertext or ID, decrypt in `onMessageReceived`, and post the notification locally. Because these produce visible notifications, high priority is not deprioritized. Set `proxy: DENY`, since proxied display bypasses the app and would show only what is in the payload. [M, derived from F]
- Chrome extension: Web Push payloads are already encrypted end-to-end to the subscription (RFC 8291) [M], but the skkil server holds those keys, so for E2EE mode apply the same ID-or-ciphertext rule. [M]
- Metadata still leaks to Apple and Google: timing, device token, size. Note this alongside ARCHITECTURE §10's metadata section. [M]

### Gaps
- Primary Signal-iOS and WhatsApp engineering sources were not fetched (search budget exhausted).
- Whether the filtering entitlement would be granted for a task or clipboard app is unknown, and likely not granted.

## 3. Local vs server notifications for task reminders (`time`)

### Takeaway
Task reminders should be **local notifications scheduled from synced data on every device**. This works offline, survives server outages, and is E2EE-safe. The server only pokes devices to resync and re-schedule. Design within the iOS 64-pending cap with a rolling window, and on Android use exact alarms with `SCHEDULE_EXACT_ALARM` (user-granted, revocable) or `USE_EXACT_ALARM` (restricted to calendar and alarm apps). "Dismiss everywhere" is an ordinary synced mutation.

### Cited Findings
- iOS: an Apple engineer confirms that apps are limited to 64 simultaneously pending notification requests: "a system limit and there is no way around it". A repeating trigger counts as one request. Which 64 are kept on overflow is reported inconsistently, so don't rely on eviction order. — [Apple forums 811171](https://developer.apple.com/forums/thread/811171), [Apple forums 765490](https://developer.apple.com/forums/thread/765490) [F]
- Android exact alarms: apps targeting 12+ must declare an "Alarms & reminders" permission or get a `SecurityException`. — [Android: schedule alarms](https://developer.android.com/develop/background-work/services/alarms/schedule) [F]
  - `SCHEDULE_EXACT_ALARM` is user-granted and revocable. It is **not pre-granted to fresh installs targeting Android 13+**, and is denied after backup-restore to an Android 14 device. Check `canScheduleExactAlarms()`. On revoke, the app is stopped and all future exact alarms are cancelled. Listen for `ACTION_SCHEDULE_EXACT_ALARM_PERMISSION_STATE_CHANGED` and reschedule. [F]
  - `USE_EXACT_ALARM` (API 33+) is auto-granted and not revocable, but limited to specific use cases under Google Play policy. The docs name alarm clock and **calendar apps** as acceptable cases. [F]
  - `setExactAndAllowWhileIdle()` fires during Doze. `setAlarmClock()` is never adjusted and exits low-power modes. Inexact `set()` on Android 12+ fires within an hour unless battery restrictions apply. Alarms are cancelled on reboot, so re-register them on `BOOT_COMPLETED`. [F]
  - Real-time-clock (RTC) alarms depend on wall-clock time; the docs warn that a user changing the device time setting causes unexpected behavior. [F]
- Google Play's exact-alarm policy details: [Play policy 12253906](https://support.google.com/googleplay/android-developer/answer/12253906) (linked from the Android docs, not fetched). [M]

### Inferences
- **Scheduling model.** Store a reminder as `(task_id, rule, tz_id, local_wall_time)`, not as a UTC instant. Each device computes the next fire instant with the IANA tz database. This handles DST correctly: a reminder at "09:00 Europe/Berlin" stays at 09:00 after the change. Floating-time reminders (follow the device's tz) versus pinned-tz reminders are a product choice; store which one is meant. [M]
- **iOS 64 cap.** Keep a rolling window: schedule the soonest ~50 one-off instances and leave headroom for snoozes. Refill on app foreground, on background-push-triggered sync, on `BGAppRefreshTask`, and when a notification fires (from the NSE or a delegate). The window is a derived cache, rebuilt deterministically from the local DB after each sync. [M]
- **Android.** Ask for `SCHEDULE_EXACT_ALARM` with a rationale screen. Fall back to `setAndAllowWhileIdle` (inexact, roughly within an hour) when it is denied. Only claim `USE_EXACT_ALARM` if `time` really qualifies as a calendar app under Play policy; there is review risk. Use WorkManager for refill. [M, from F]
- **Rescheduling after sync.** Rescheduling is a pure function `schedule = f(local_db, now, device_tz)`. Run it after every applied pull batch that touches reminder rows, and diff against pending OS requests by deterministic IDs (`reminder:{task_id}:{occurrence_ts}`). That makes it idempotent and convergent. [M]
- **Multi-device dedup.** By default every device fires; that is what Apple Reminders and Todoist do [M, not verified]. Offer "dismiss everywhere": dismiss, complete or snooze becomes a synced mutation (`reminder_ack{occurrence}`). Other devices receive it via pull, triggered by a poke or a push, and call `removeDeliveredNotifications` / `NotificationManager.cancel` for the matching ID. On iOS this removal needs the app to run, via a background push (best-effort, 2–3 per hour) or the next foreground. Accept that a stale banner can linger. An optional "notify only on most recently active device" policy needs server presence and goes against offline-first, so defer it. [M]
- **Server-sent reminder pushes** (a server cron sends the alert) are the fallback only for clients that can't schedule locally: the Chrome extension (it can use `chrome.alarms` plus notifications when the browser is running [M]) and the CLI. Server-side reminders need plaintext on the server, or a pre-encrypted notification body stored by the client, for E2EE. [M]

### Gaps
- Current (2026) Play policy text for `USE_EXACT_ALARM` eligibility was not fetched.
- Android 15/16 changes to exact alarms (if any) were not verified.
- Competitor behavior (Todoist, Things, TickTick) for cross-device reminder dismissal was not sourced.

## 4. Real-time channel: WebSocket vs SSE vs long-poll vs WebTransport; connect-rpc; scaling; reconnect; gap recovery

### Takeaway
The channel only carries pokes (`scope X head=N`), so pick the simplest transport that every client supports: **SSE or connect-rpc server-streaming over HTTP** (browser-compatible, no bidi needed), with WebSocket only if bidirectional traffic appears. Correctness comes from pull-by-cursor on connect, on gap, and on a timer, not from the channel.

### Cited Findings
- Connect in browsers supports **server streaming only**. Client and bidi streaming are not supported, because browsers haven't implemented streaming request bodies in fetch. — [Connect web docs](https://connectrpc.com/docs/web/getting-started) [F]
- Durable Streams (Electric, open-sourced 2025-12-09) is an HTTP protocol of append-only streams with opaque monotonic offsets. Clients resume from their last offset; the server keeps no per-client session. Catch-up uses `GET`, and live tailing uses long-poll or SSE. Offset URLs are CDN-cacheable. v0.1.0 shipped 2025-12-23; hosted 0.2.0 on 2026-01-22 added idempotent producers. — [Electric announcement](https://electric.ax/blog/2025/12/09/announcing-durable-streams) [F]; release dates via [search results](https://electric.ax/blog/2025/12/23/durable-streams-0.1.0) [F, snippet]
- Redis pub/sub is **at-most-once**: "If the subscriber is unable to handle the message (for example, due to an error or a network disconnect) the message is forever lost." Sharded pub/sub (Redis 7+) scales in cluster mode. — [Redis pub/sub docs](https://redis.io/docs/latest/develop/pubsub/) [F]

### Inferences
- **Transport choice** [M]:
  - SSE / connect server-stream: works through HTTP/2 and proxies, auto-reconnects (`EventSource` with `Last-Event-ID`), is one-way, and fits pokes. On HTTP/1.1, browsers cap about 6 connections per origin; with HTTP/2 that isn't an issue.
  - WebSocket: bidi; ARCHITECTURE §9 already says the "clipboard sync gateway → WebSocket". Fine for desktop and mobile native. Needs custom heartbeat and reconnect logic and sticky L4/L7 handling.
  - Long-poll: the fallback; Durable Streams uses it, as does Electric's shape API.
  - WebTransport/HTTP3: no benefit for pokes, uneven support (Safari) [M]. Defer.
  - Recommendation: one connect-rpc `Subscribe(scopes, cursors) returns stream Poke` that runs over HTTP/2 for native clients and over connect-web for the extension. Push and pull stay unary RPCs.
- **Scaling across instances** [M]: each gateway instance subscribes to the bus channel `scope:{id}` for its connected users (Redis sharded pub/sub `SSUBSCRIBE`, or NATS subject `poke.{scope}`). The committing transaction's outbox, or a post-commit hook, publishes `{scope, head}`. Since the bus is lossy, the gateway also sends a periodic heartbeat that includes the current `head` per scope, read from a cache of "head ≥ N" hints (allowed by rule 6). A client that sees a head greater than its cursor pulls.
- **Mobile**: hold the stream only in the foreground. Close it on background, so iOS doesn't kill it anyway, and rely on push for alerts. Pull on every foreground. [M]
- **Reconnect** [M]: exponential backoff with full jitter (e.g. base 500 ms, cap 30–60 s), reset after a stable connection. On reconnect, send cursors and pull before trusting pokes. On network-change events (iOS `NWPathMonitor`, Android `ConnectivityManager`), reconnect immediately.
- **Ordering and gap recovery**: pokes carry `head`, and the client pulls `(cursor, head]` from the log. A poke with `head ≤ cursor` is ignored, so duplicates, reordering and loss are all harmless. This is rule 7 restated, and it makes the channel choice non-critical. [M]
- Durable Streams' resume-from-offset model closely matches the skkil pull protocol. The pull endpoint could adopt its URL and offset conventions to gain CDN cacheability of historical log ranges, but only in plaintext mode or with ciphertext rows, and only with per-user auth at the CDN. Treat it as an option, not a dependency; the third-party server ecosystem is young. [M]

### Gaps
- Benchmarks of connection count per instance (Go/Rust WebSocket vs SSE) were not gathered.
- iOS background URLSession or keep-alive limits for long-lived streams were not verified.

## 5. Event-stream backends, per-user ordering, idempotency, outbox/inbox, webhooks, Google Calendar sync

### Takeaway
For a two-month ship, Postgres can be the queue (outbox table + `SKIP LOCKED` workers), with a lossy bus for pokes. Kafka's exactly-once ends at Kafka's boundary, so consumers must be idempotent regardless. Google Calendar mirroring is itself an external event stream: watch channels (lossy, contentless, expiring) as pokes, plus `syncToken` incremental pulls. That is the same poke-then-pull shape, inverted.

### Cited Findings
**Kafka** [F]
- The idempotent producer and transactions exist since 0.11. Exactly-once applies in Kafka Streams, where offsets commit in the same transaction as output. "Exactly-once delivery for other destination systems generally requires cooperation with such systems." The recommendation is to store the consumer offset together with the output. — [Kafka design docs](https://kafka.apache.org/42/design/design/)

**NATS JetStream** [F]
- Deduplication by the `Nats-Msg-Id` header within a duplicate window, **default 2 minutes**. Retention options: Limits (default), Interest, WorkQueue. Discard policy defaults to Old. — [NATS streams](https://docs.nats.io/nats-concepts/jetstream/streams)
- The JetStream docs cover ack, nak, term, in-progress, **double ack** and redelivery; I did not fetch the detail page. — [NATS JetStream learn index](https://docs.nats.io/learn/jetstream/)

**Redis** [F]
- Pub/sub is at-most-once. Streams persist messages and support at-least-once delivery (consumer groups). — [Redis pub/sub](https://redis.io/docs/latest/develop/pubsub/)

**Postgres LISTEN/NOTIFY** [F]
- Payload under 8000 bytes by default. The queue is 8 GB; when it is full, transactions calling `NOTIFY` fail at commit. Notifications are delivered only on commit, in commit order across transactions. Identical channel+payload pairs in one transaction collapse into one. Delivery goes only to sessions that have executed `LISTEN`, so there is no persistence. — [PostgreSQL 18 NOTIFY](https://www.postgresql.org/docs/current/sql-notify.html)
- Recall.ai (2025-07): a NOTIFY inside a transaction takes a global lock at commit that serializes commits, which caused outages under many concurrent writers. — [Recall.ai](https://www.recall.ai/blog/postgres-listen-notify-does-not-scale), [Simon Willison](https://simonwillison.net/2025/Jul/11/postgres-listen-notify/). Counterpoint: DBOS says batching and buffering NOTIFYs keeps the lock off the per-write path, and that a Postgres 19 patch doesn't remove the lock. — [DBOS](https://dbos.dev/blog/postgres-listen-notify-scalability) [F via search snippets]

**Durable Streams** [F]: see §4. Exactly-once and idempotent producers are claimed for hosted 0.2.0 (vendor claim).

**Google Calendar as an external stream** [F]
- `events.watch`: POST with `id` (≤64 chars, unique), `type: web_hook`, `address` (HTTPS with a valid certificate; self-signed won't work), an optional `token` (≤256 chars, echoed back; don't put secrets in it unencrypted) and an optional `expiration`. — [Calendar push](https://developers.google.com/workspace/calendar/api/guides/push)
- Notifications have **no body**. Headers carry `X-Goog-Channel-ID`, `X-Goog-Message-Number`, `X-Goog-Resource-ID` and `X-Goog-Resource-State` (`sync` on creation, `exists` on change). Acknowledge with 200, 201, 202, 204 or 102. — [Calendar push](https://developers.google.com/workspace/calendar/api/guides/push)
- **No automatic renewal.** Create a new channel with a new `id` before expiry; they overlap. Stop a channel with `channels/stop(id, resourceId)`, from the same user and OAuth client. Each calendar needs its own channel. "Notifications aren't guaranteed. Some messages will be dropped." — [Calendar push](https://developers.google.com/workspace/calendar/api/guides/push)
- Incremental sync: do a full list, then save the `nextSyncToken` (it appears only on the last page). Later calls send `syncToken` and paginate with `pageToken` until a new `nextSyncToken` arrives. A **410 Gone** means the token is invalidated: wipe the local mirror and fully resync. Filters are mostly incompatible with `syncToken`, and the same parameter set must be used throughout, otherwise the server returns 400. Incremental results always include deleted entries. — [Calendar sync](https://developers.google.com/workspace/calendar/api/guides/sync)

### Inferences
- **Per-user ordering**: the authoritative order is the Postgres scope version (rule 1). Downstream: Kafka key = scope (per-partition order); NATS subject `evt.{scope}` (per-subject order within a stream [M]); Redis Streams key per scope. Since ARCHITECTURE §9 notes that multiple relays reorder, keep **version-gated consumers** (`apply if version > seen`). That makes the bus's ordering irrelevant. [M]
- **Exactly-once myth**: across Kafka → external system you get at-least-once plus an idempotent effect. Use an inbox table `(source, event_id)` with a unique constraint in the consumer's DB, written in the same transaction as the effect (Kafka's own "store offset with output" advice). For third-party effects such as APNs, FCM, email and Google Calendar writes, there is no transaction, so use idempotency keys (`apns-id`; Calendar event IDs chosen by the client [M]) and accept rare duplicates. [M, from F]
- **Outbox note**: ARCHITECTURE §9's relay uses `WHERE published_at IS NULL ORDER BY id FOR UPDATE SKIP LOCKED`. That's fine for at-least-once (it doesn't skip, because it isn't a cursor), but it reorders. The novelty-check rule 5 (`xid8` watermark) only matters for cursor-style readers. These are consistent if consumers stay version-gated. [M]
- **Webhooks for integrations (outbound)**: dispatch from the outbox, sign with HMAC (timestamp in the signature to prevent replay), send an event ID for consumer dedup, retry with exponential backoff, and disable after N failures. This is the Stripe/Svix pattern [M, unverified]. Prefer "thin" webhooks (an ID plus a pointer to fetch); this is mandatory in E2EE mode because the server has only ciphertext. Defer until there is an integration customer. [M]
- **Google Calendar mirroring design** [M]:
  - Watch handler: verify the channel token, enqueue `gcal_sync{calendar_id}` (deduplicated, coalesced), and return 200 immediately.
  - A worker runs `events.list?syncToken=…`, maps results into server mutations in the user's log through the **inbox** (the same path as media events in rule 9), and saves `nextSyncToken` in the same transaction.
  - A periodic fallback poll (e.g. every 15–60 min) covers dropped notifications. A renewal cron recreates channels before expiry. On 410, rebuild the mirror and diff it against the existing mirrored rows, rather than deleting user-visible tasks.
  - Outbound (skkil → Google): use the outbox, set skkil's own stable event IDs (or store the Google ID) to make writes idempotent, and tag events, e.g. with `extendedProperties.private.skkil_origin`, to break echo loops when the change returns through watch. [M]
  - Google mirroring requires the server to see plaintext tasks, so it is incompatible with E2EE scopes. Make it an explicit opt-out of E2EE per calendar-linked scope, or run it client-side (the Calendar API from the device, no webhooks). [M]

### Gaps
- Google Calendar default and maximum channel TTL are not stated on the fetched page. They are commonly reported as about 7 days for events but unverified.
- NATS per-subject ordering guarantees and the double-ack semantics page were not fetched.
- Redis Streams consumer-group specifics (PEL, XAUTOCLAIM) were not fetched.
- pg logical replication / Debezium operational cost was not researched this session.

## 6. Minimal stack for a 2-month ship; what to defer

### Takeaway
**Postgres + one lossy bus (Redis or NATS core) + APNs/FCM direct** is enough. Postgres is the log, the outbox, the job queue (reminder fan-out, push dispatch, Google Calendar sync) and the inbox. Kafka, JetStream persistence, Durable Streams, UnifiedPush, webhooks and CDC are all deferrable because poke-then-pull tolerates a lossy bus.

### Cited Findings
- ARCHITECTURE §9 already argues that reclaim work is a Postgres queue, because it needs `due_at`, leases and "what's left" queries, and "Kafka has no delayed delivery". — `/home/hyoungjoojin/Desktop/skkil/ARCHITECTURE.md` §9 [F, local]
- Redis pub/sub at-most-once is acceptable only because clients recover by pull. — [Redis](https://redis.io/docs/latest/develop/pubsub/) [F]; novelty-check rule 7 [F, local]
- LISTEN/NOTIFY has a global commit lock under many writers. — [Recall.ai](https://www.recall.ai/blog/postgres-listen-notify-does-not-scale) [F]

### Inferences
- **Ship (months 1–2)** [M]:
  1. Postgres: the sync log, plus an `outbox` table (push jobs, gcal jobs) drained by `SKIP LOCKED` workers in the same Go/Rust binary.
  2. Poke bus: Redis pub/sub if Redis is already deployed for caches; otherwise NATS core, which is a single binary with subject wildcards and a JetStream upgrade path. Postgres LISTEN/NOTIFY is acceptable at low write rates if NOTIFY is sent from a *separate* post-commit worker (batched) rather than inside every mutation transaction, which avoids the commit-lock issue.
  3. A gateway: connect-rpc server-stream pokes, plus heartbeat heads.
  4. Push: direct APNs (HTTP/2 + .p8 token) and FCM v1. Generic alert text plus NSE decrypt for E2EE. Device-token table with pruning on APNs 410 or FCM `UNREGISTERED`. Collapse IDs per scope.
  5. Reminders: local scheduling on iOS, Android and desktop; `chrome.alarms` in the extension; none for the CLI. "Dismiss everywhere" as a synced mutation.
  6. Google Calendar: one-way import (Google → `time`) via watch plus syncToken, if it must ship. Two-way mirroring is the riskiest item; defer it if time is short.
- **Defer**: Kafka (introduce it only when there is more than one consumer service needing replay; the outbox relay already abstracts it), JetStream/Redis Streams durability, Durable Streams/CDN log serving, UnifiedPush/ntfy, outbound webhooks, Debezium CDC, WebTransport, presence-based "notify one device", and server-sent reminder pushes. [M]
- This partly conflicts with ARCHITECTURE §9 and the Feb roadmap row ("Outbox → Kafka"). Recommend keeping the outbox schema and publishing to the poke bus plus internal workers first, then adding Kafka as an outbox sink later. [M]

### Gaps
- No measured numbers were gathered for Postgres `SKIP LOCKED` queue throughput or NATS/Redis fan-out at the expected scale. A pgbench run or load test is needed.
