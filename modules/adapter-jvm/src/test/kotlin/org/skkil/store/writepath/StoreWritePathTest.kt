package org.skkil.store.writepath

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.skkil.store.support.StoreDatabase
import org.skkil.store.support.StoreDatabase.transactions
import java.util.UUID
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class StoreWritePathTest {
    private val writePath = StoreWritePath(StoreDatabase.dataSource)
    private val scope = UUID.randomUUID()
    private val client = UUID.randomUUID()

    @Test
    fun `begin and end commit the mutation as the next version`() {
        val first = applied(1, "a")
        val second = applied(2, "b")

        assertEquals(1, first)
        assertEquals(2, second)
        assertEquals(listOf("a", "b"), StoreDatabase.notes(scope))
        assertEquals(listOf("1:commit", "2:commit"), StoreDatabase.logKinds(scope))
    }

    @Test
    fun `a processed mutation begins as a duplicate and reports its version`() {
        applied(1, "a")
        applied(2, "b")

        val outcome = transactions.execute { writePath.begin(scope, client, 1) }
        val version = transactions.execute { writePath.recordedVersion(scope, client, 1) }

        assertEquals(Begin.DUPLICATE, outcome)
        assertEquals(1, version)
        assertEquals(2, StoreDatabase.headVersion(scope))
    }

    @Test
    fun `reject after rolling back to the savepoint records a rejection without row changes`() {
        val version =
            transactions.execute {
                assertEquals(Begin.BEGUN, writePath.begin(scope, client, 1))
                val savepoint = writePath.savepoint()
                StoreDatabase.insertNote(scope, "discarded")
                writePath.rollbackTo(savepoint)
                writePath.reject("title taken")
            }

        assertEquals(1, version)
        assertEquals(emptyList(), StoreDatabase.notes(scope))
        assertEquals(listOf("1:rejection"), StoreDatabase.logKinds(scope))
        assertEquals(Begin.DUPLICATE, transactions.execute { writePath.begin(scope, client, 1) })
    }

    @Test
    fun `rolling back to a savepoint recovers a transaction aborted by a failed statement`() {
        val version =
            transactions.execute {
                writePath.begin(scope, client, 1)
                val savepoint = writePath.savepoint()
                val id = UUID.randomUUID()
                StoreDatabase.insertNote(scope, "first", id)
                runCatching { StoreDatabase.insertNote(scope, "clash", id) }
                writePath.rollbackTo(savepoint)
                writePath.reject("duplicate id")
            }

        assertEquals(1, version)
        assertEquals(emptyList(), StoreDatabase.notes(scope))
    }

    @Test
    fun `server writes are logged under the server client id`() {
        applied(1, "device")
        val version =
            transactions.execute {
                writePath.beginServer(scope)
                StoreDatabase.insertNote(scope, "server")
                writePath.end()
            }

        assertEquals(2, version)
        val client =
            StoreDatabase.jdbc
                .sql("SELECT client_id FROM store_log WHERE scope_id = ? AND version = 2 AND seq = 0")
                .param(scope)
                .query(UUID::class.java)
                .single()
        assertEquals(UUID(0, 0), client)
    }

    @Test
    fun `end and reject poke the scope after commit`() {
        StoreDatabase.listening { listener ->
            applied(1, "a")
            assertEquals(listOf("$scope:1"), listener.next())

            transactions.execute {
                writePath.begin(scope, client, 2)
                writePath.reject("no")
            }
            assertEquals(listOf("$scope:2"), listener.next())
        }
    }

    @Test
    fun `a rolled back mutation sends no poke`() {
        StoreDatabase.listening { listener ->
            transactions.execute { status ->
                writePath.begin(scope, client, 1)
                writePath.end()
                status.setRollbackOnly()
            }
            assertEquals(emptyList(), listener.next(500))
            assertEquals(0, StoreDatabase.headVersion(scope))
        }
    }

    @Test
    fun `writing a synced table outside a mutation raises MutationNotOpenException`() {
        val error =
            assertThrows<RuntimeException> {
                transactions.execute { StoreDatabase.insertNote(scope, "stray") }
            }

        assertTrue(StoreSqlErrors.translate(error) is MutationNotOpenException)
    }

    @Test
    fun `end without an open mutation raises MutationNotOpenException`() {
        assertThrows<MutationNotOpenException> { transactions.execute { writePath.end() } }
    }

    @Test
    fun `a client of another scope raises ClientScopeMismatchException`() {
        applied(1, "a")

        val error =
            assertThrows<ClientScopeMismatchException> {
                transactions.execute { writePath.begin(UUID.randomUUID(), client, 2) }
            }
        assertTrue(error.message!!.startsWith("store: client"))
    }

    @Test
    fun `mutation ids below 1 and the server client id raise InvalidStoreArgumentException`() {
        assertThrows<InvalidStoreArgumentException> { transactions.execute { writePath.begin(scope, client, 0) } }
        assertThrows<InvalidStoreArgumentException> { transactions.execute { writePath.begin(scope, UUID(0, 0), 1) } }
    }

    @Test
    fun `the write path refuses to run outside a transaction`() {
        assertThrows<IllegalStateException> { writePath.begin(scope, client, 1) }
    }

    @Test
    fun `endIfChanged closes an unchanged server write without a version or poke`() {
        StoreDatabase.listening { listener ->
            val version =
                transactions.execute {
                    writePath.beginServer(scope)
                    writePath.endIfChanged().also { assertFalse(writePath.isOpen()) }
                }

            assertNull(version)
            assertEquals(0, StoreDatabase.headVersion(scope))
            assertEquals(emptyList(), listener.next(500))
        }
    }

    @Test
    fun `endIfChanged logs and pokes a server write that changed rows`() {
        StoreDatabase.listening { listener ->
            val version =
                transactions.execute {
                    writePath.beginServer(scope)
                    StoreDatabase.insertNote(scope, "server")
                    writePath.endIfChanged()
                }

            assertEquals(1, version)
            assertEquals(listOf("$scope:1"), listener.next())
        }
    }

    @Test
    fun `abandon closes a server write rolled back to its savepoint`() {
        transactions.execute {
            writePath.beginServer(scope)
            val savepoint = writePath.savepoint()
            StoreDatabase.insertNote(scope, "discarded")
            writePath.rollbackTo(savepoint)
            writePath.abandon()
            assertFalse(writePath.isOpen())
        }

        assertEquals(emptyList(), StoreDatabase.notes(scope))
        assertEquals(0, StoreDatabase.headVersion(scope))
    }

    @Test
    fun `abandoning a device mutation or captured changes raises InvalidStoreArgumentException`() {
        assertThrows<InvalidStoreArgumentException> {
            transactions.execute {
                writePath.begin(scope, client, 1)
                writePath.abandon()
            }
        }
        assertThrows<InvalidStoreArgumentException> {
            transactions.execute {
                writePath.begin(scope, client, 1)
                writePath.endIfChanged()
            }
        }
        assertThrows<InvalidStoreArgumentException> {
            transactions.execute {
                writePath.beginServer(scope)
                StoreDatabase.insertNote(scope, "kept")
                writePath.abandon()
            }
        }
    }

    @Test
    fun `isOpen follows begin and end`() {
        transactions.execute {
            assertFalse(writePath.isOpen())
            writePath.begin(scope, client, 1)
            assertTrue(writePath.isOpen())
            writePath.end()
            assertFalse(writePath.isOpen())
        }
    }

    @Test
    fun `claimClient binds a client to its first scope`() {
        assertTrue(writePath.claimClient(scope, client))
        assertTrue(writePath.claimClient(scope, client))
        assertFalse(writePath.claimClient(UUID.randomUUID(), client))
        assertThrows<InvalidStoreArgumentException> { writePath.claimClient(scope, UUID(0, 0)) }
        assertEquals(1, applied(1, "a"))
    }

    @Test
    fun `dropScope removes the scope with its clients and log`() {
        applied(1, "a")
        transactions.execute {
            writePath.beginServer(scope)
            StoreDatabase.jdbc
                .sql("DELETE FROM note WHERE scope_id = ?")
                .param(scope)
                .update()
            writePath.end()
        }

        writePath.dropScope(scope)

        assertEquals(0, StoreDatabase.headVersion(scope))
        assertEquals(emptyList(), StoreDatabase.logKinds(scope))
        assertTrue(writePath.claimClient(UUID.randomUUID(), client))
    }

    private fun applied(mutationId: Long, title: String): Long =
        transactions.execute {
            assertEquals(Begin.BEGUN, writePath.begin(scope, client, mutationId))
            StoreDatabase.insertNote(scope, title)
            writePath.end()
        }
}
