package org.skkil.store.push

import com.google.protobuf.ByteString
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.skkil.store.support.StoreDatabase
import org.skkil.store.v1.Mutation
import org.skkil.store.v1.MutationResult.Status
import org.skkil.store.v1.PushRequest
import org.skkil.store.writepath.ClientScopeMismatchException
import org.skkil.store.writepath.StoreWritePath
import org.springframework.dao.DataIntegrityViolationException
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import java.util.UUID
import kotlin.test.assertEquals

class PushHandlerTest {
    private val scope = UUID.randomUUID()
    private val client = UUID.randomUUID()
    private val serverNow = Instant.parse("2026-11-08T12:00:00Z")
    private val contexts = mutableListOf<MutationContext>()

    private val registry =
        MutationRegistry()
            .register("note.create") { args, context ->
                contexts += context
                val title = args.required("title").asString()
                StoreDatabase.insertNote(context.scope, title, context.nextId())
                if (title.length > 10) throw MutationRejectedException("title is too long")
            }.register("note.crash") { _, _ -> error("database is down") }

    private val handler =
        PushHandler(
            registry,
            StoreWritePath(StoreDatabase.dataSource),
            StoreDatabase.transactions,
            RejectionPolicy.DEFAULT,
            clock = Clock.fixed(serverNow, ZoneOffset.UTC),
        )

    @Test
    fun `a batch reports applied, duplicate and rejected mutations in order`() {
        handler.push(scope, request(create(1, "first")))

        val response =
            handler.push(
                scope,
                request(
                    create(1, "first"),
                    create(2, "second"),
                    create(3, "much too long"),
                    mutation(4, "note.unknown", "{}"),
                    mutation(5, "note.create", "[1]"),
                    create(6, "third"),
                ),
            )

        val results = response.resultsList.map { Triple(it.mutationId, it.status, it.version) }
        assertEquals(
            listOf(
                Triple(1L, Status.STATUS_DUPLICATE, 1L),
                Triple(2L, Status.STATUS_APPLIED, 2L),
                Triple(3L, Status.STATUS_REJECTED, 3L),
                Triple(4L, Status.STATUS_REJECTED, 4L),
                Triple(5L, Status.STATUS_REJECTED, 5L),
                Triple(6L, Status.STATUS_APPLIED, 6L),
            ),
            results,
        )
        assertEquals(
            listOf(
                "",
                "",
                "title is too long",
                "unknown mutation: note.unknown",
                "invalid arguments: expected a JSON object",
                "",
            ),
            response.resultsList.map { it.reason },
        )
        assertEquals(listOf("first", "second", "third"), StoreDatabase.notes(scope))
    }

    @Test
    fun `a rejected mutation leaves no row changes`() {
        handler.push(scope, request(create(1, "much too long")))

        assertEquals(emptyList(), StoreDatabase.notes(scope))
        assertEquals(listOf("1:rejection"), StoreDatabase.logKinds(scope))
    }

    @Test
    fun `an infrastructure failure aborts the batch after committing the mutations before it`() {
        assertThrows<IllegalStateException> {
            handler.push(scope, request(create(1, "kept"), mutation(2, "note.crash", "{}"), create(3, "never")))
        }

        assertEquals(listOf("kept"), StoreDatabase.notes(scope))
        assertEquals(1, StoreDatabase.headVersion(scope))
    }

    @Test
    fun `the rejection policy decides which exceptions reject`() {
        val strict =
            PushHandler(
                MutationRegistry().register("note.clash") { _, context ->
                    val id = UUID.randomUUID()
                    StoreDatabase.insertNote(context.scope, "one", id)
                    StoreDatabase.insertNote(context.scope, "two", id)
                },
                StoreWritePath(StoreDatabase.dataSource),
                StoreDatabase.transactions,
                RejectionPolicy.of(DataIntegrityViolationException::class),
            )

        val result = strict.push(scope, request(mutation(1, "note.clash", "{}"))).resultsList.single()

        assertEquals(Status.STATUS_REJECTED, result.status)
        assertEquals(emptyList(), StoreDatabase.notes(scope))
    }

    @Test
    fun `the context carries the mutation, its wall time and derived ids`() {
        val hlc = (Instant.parse("2026-11-08T10:00:00Z").toEpochMilli() shl 16) or 3
        handler.push(scope, request(create(7, "offline", hlc), create(8, "no clock")))

        val (offline, noClock) = contexts
        assertEquals(scope, offline.scope)
        assertEquals(client, offline.clientId)
        assertEquals(7, offline.mutationId)
        assertEquals("note.create", offline.mutationName)
        assertEquals("offline", offline.args["title"].asString())
        assertEquals(Instant.parse("2026-11-08T10:00:00Z"), offline.now())
        assertEquals(serverNow, noClock.now())
        val id =
            StoreDatabase.jdbc
                .sql("SELECT id FROM note WHERE title = 'offline'")
                .query(UUID::class.java)
                .single()
        assertEquals(DerivedIds.derive(client, 7, 0), id)
    }

    @Test
    fun `only protocol version 1 is accepted`() {
        val error =
            assertThrows<PushException> {
                handler.push(scope, request(create(1, "a")).toBuilder().setProtocolVersion(2).build())
            }

        assertEquals(ConnectCode.FAILED_PRECONDITION, error.code)
    }

    @Test
    fun `a client id that is not a UUID is an invalid argument`() {
        val error =
            assertThrows<PushException> { handler.push(scope, request().toBuilder().setClientId("phone").build()) }

        assertEquals(ConnectCode.INVALID_ARGUMENT, error.code)
    }

    @Test
    fun `a client of another scope aborts the batch`() {
        handler.push(scope, request(create(1, "a")))

        assertThrows<ClientScopeMismatchException> { handler.push(UUID.randomUUID(), request(create(2, "b"))) }
    }

    @Test
    fun `push refuses to run inside a transaction`() {
        assertThrows<IllegalStateException> {
            StoreDatabase.transactions.execute { handler.push(scope, request(create(1, "a"))) }
        }
    }

    private fun request(vararg mutations: Mutation): PushRequest =
        PushRequest
            .newBuilder()
            .setProtocolVersion(1)
            .setClientId(client.toString())
            .addAllMutations(mutations.toList())
            .build()

    private fun create(id: Long, title: String, hlc: Long = 0): Mutation =
        mutation(
            id,
            "note.create",
            """{"title":"$title"}""",
            hlc,
        )

    private fun mutation(id: Long, name: String, args: String, hlc: Long = 0): Mutation =
        Mutation
            .newBuilder()
            .setMutationId(id)
            .setName(name)
            .setArgs(ByteString.copyFromUtf8(args))
            .setHlc(hlc)
            .build()
}
