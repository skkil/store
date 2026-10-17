# store-adapter-jvm

Kotlin library for Spring Boot product servers: the store write path, a
mutation registry, the `PushService/Push` handler with its Connect codec, and
ES256 sync tokens. Coordinates `org.skkil.store:store-adapter-jvm`.

## Commands

```bash
./gradlew spotlessApply      # format (ktlint)
./gradlew spotlessCheck test build
./gradlew syncTokenKeys      # print a development ES256 key pair (PKCS#8 + SPKI PEM)
./gradlew publishToMavenLocal
```

Tests start `postgres:17-alpine` with Testcontainers (Docker required) and
apply every `../sql/migrations/V*.sql` in order. Java classes for `skkil.store.v1` are
generated from `../protocol/proto` at build time.

## Using it from a product

Composite build, in the product's `settings.gradle.kts`:

```kotlin
includeBuild("../../../store/modules/adapter-jvm")
```

and `implementation("org.skkil.store:store-adapter-jvm:0.1.0")`. Copy each
`../sql/migrations/V*__*.sql` into the product's own Flyway migrations, in
order, then `store_register` each synced table.

`StoreAutoConfiguration` wires `StoreWritePath`, `MutationRegistry`,
`RejectionPolicy` (rejects `MutationRejectedException`), `PushHandler` and
`ConnectPushCodec`. Define any of them yourself to replace it. Setting the
properties below adds a `SyncTokenIssuer`:

```yaml
skkil:
  store:
    token:
      issuer: time
      private-key: ${STORE_TOKEN_PRIVATE_KEY}   # PKCS#8 PEM
      public-key: ${STORE_TOKEN_PUBLIC_KEY}     # SPKI PEM, also given to the store service
      ttl: 15m                                  # at most 15m
```

### Mutations

```kotlin
@Configuration
class Mutations(registry: MutationRegistry, tasks: TaskRepository) {
    init {
        registry.register("task.create") { args, ctx ->
            val title = args.required("title").asString()
            if (title.isBlank()) throw MutationRejectedException("A task needs a title")
            tasks.insert(id = ctx.nextId(), scope = ctx.scope, title = title, createdAt = ctx.now())
        }
    }
}
```

- Handlers run plain SQL on the transaction `PushHandler` opened; don't start
  a new one (`REQUIRES_NEW`) inside.
- `ctx.nextId()` returns the same ids the device created optimistically.
- `ctx.now()` is the device's wall time from `Mutation.hlc`
  (`(unix_millis << 16) | counter`), or the server time when `hlc` is 0. Use
  it instead of the server clock.
- Throw an exception your `RejectionPolicy` accepts to reject. The handler's
  row changes are rolled back to the savepoint taken after `store_begin`, and
  the exception message becomes the user-visible reason. Any other exception
  aborts the batch with a Connect error and the client retries it.
- Avoid deferred constraints on synced tables: a failure at `COMMIT` cannot be
  turned into a rejection and blocks the client's queue.

### Push controller

```kotlin
@RestController
class PushController(private val push: PushHandler, private val codec: ConnectPushCodec) {
    @PostMapping("/skkil.store.v1.PushService/Push")
    fun push(
        @RequestBody body: ByteArray,
        @RequestHeader(HttpHeaders.CONTENT_TYPE) contentType: String,
        @RequestHeader(HttpHeaders.CONTENT_ENCODING, required = false) contentEncoding: String?,
        caller: Caller,
    ): ResponseEntity<ByteArray> {
        val reply = codec.handle(body, contentType, contentEncoding) { push.push(caller.scopeId, it) }
        return ResponseEntity
            .status(reply.status)
            .contentType(MediaType.parseMediaType(reply.contentType))
            .body(reply.body)
    }
}
```

The scope always comes from the authenticated caller, never from the request.
A client id that already belongs to another scope answers `permission_denied`.

### Server writes

Writes the product makes on its own (REST endpoints, jobs) go through the same
log:

```kotlin
transactionTemplate.execute {
    writePath.beginServer(scope)
    tasks.rename(id, title)
    writePath.end()
}
```

`end()` and `reject()` schedule `store_poke` for after the commit, on a
separate connection; a failed poke is logged and otherwise ignored.

### Sync tokens

```kotlin
val token = issuer.issue(scope, clientId)   // token.token, token.expiresAt
issuer.publicKeyPem()                       // configure the store service with this
issuer.jwks()                               // or serve this at a JWKS URL
```

## Layout

```
src/main/kotlin/org/skkil/store/
  writepath/  StoreWritePath (begin, beginServer, end, reject, savepoints, poke),
              Begin, StoreException and its SQLSTATE subclasses, StoreSqlErrors
  push/       MutationRegistry, MutationHandler, MutationContext, DerivedIds,
              HybridLogicalClock, RejectionPolicy, MutationRejectedException,
              PushHandler, PushException, ConnectPushCodec, ConnectReply, ConnectCode
  token/      SyncTokenIssuer, SyncToken, SyncTokenKeys, SyncTokenKeyGenerator (dev main)
  config/     StoreAutoConfiguration, StoreProperties
src/test/kotlin/org/skkil/store/
  support/    StoreDatabase (Testcontainers Postgres with the store migration)
  …           one test class per main class
```

## Contract notes

- `DerivedIds.derive(client, mutationId, n)` is
  `uuidv8(sha256(client_id[16] ‖ mutation_id u64 BE ‖ n u32 BE))` with `n`
  counting from 0: the first 16 digest bytes, version nibble 8, variant `10`.
  `DerivedIdsTest` holds the fixed vectors shared with `store-core`.
- A `DUPLICATE` result carries the version that recorded the original
  mutation, or the scope head if the log no longer holds it.
- Requests with `protocol_version != 1` answer `failed_precondition`.
