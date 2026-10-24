package org.skkil.store.writepath

import org.slf4j.LoggerFactory
import org.springframework.jdbc.core.simple.JdbcClient
import org.springframework.jdbc.datasource.ConnectionHolder
import org.springframework.transaction.support.TransactionSynchronization
import org.springframework.transaction.support.TransactionSynchronizationManager
import java.sql.Savepoint
import java.util.UUID
import javax.sql.DataSource

/**
 * The store write path (`docs/protocol.md`) over the product's [DataSource].
 *
 * Every method except [poke] runs on the connection of the caller's Spring-managed transaction,
 * so the mutation's row changes, the scope version and the idempotency record commit together.
 * [end], [endIfChanged] (when it logs a version) and [reject] schedule `store_poke` to run after that
 * transaction commits.
 */
public class StoreWritePath(
    private val dataSource: DataSource,
) {
    private val jdbc = JdbcClient.create(dataSource)

    /** Opens the device mutation `(client, mutationId)` in [scope], or reports that it was already processed. */
    public fun begin(scope: UUID, client: UUID, mutationId: Long): Begin {
        requireTransaction()
        val outcome =
            translated {
                jdbc
                    .sql("SELECT store_begin(?, ?, ?)")
                    .params(scope, client, mutationId)
                    .query(String::class.java)
                    .single()
            }
        return when (outcome) {
            "begun" -> Begin.BEGUN
            "duplicate" -> Begin.DUPLICATE
            else -> error("store_begin returned an unknown outcome: $outcome")
        }
    }

    /** Opens a write the product makes on its own, recorded under the server client id. */
    public fun beginServer(scope: UUID) {
        requireTransaction()
        translated { jdbc.sql("SELECT store_begin_server(?)").param(scope).query().singleRow() }
    }

    /** Closes the open mutation as applied and returns its version. */
    public fun end(): Long = close { jdbc.sql("SELECT store_end()").query(Long::class.javaObjectType).single() }

    /**
     * Closes the open server write: as [end] when it changed a synced row, returning the new version, otherwise
     * without a version, returning null. Device mutations must [end] or [reject] instead.
     */
    public fun endIfChanged(): Long? {
        requireTransaction()
        val scope = openScope()
        val version =
            translated {
                jdbc
                    .sql("SELECT store_end_if_changed()")
                    .query(Long::class.javaObjectType)
                    .list()
                    .firstOrNull()
            }
        version?.let { pokeAfterCommit(scope, it) }
        return version
    }

    /**
     * Closes the open server write without a version. It must not have changed a synced row: roll back to a
     * savepoint first. Device mutations must [end] or [reject] instead.
     */
    public fun abandon() {
        requireTransaction()
        translated { jdbc.sql("SELECT store_abandon()").query().singleRow() }
    }

    /** True while a device mutation or server write is open on the caller's transaction. */
    public fun isOpen(): Boolean {
        requireTransaction()
        return jdbc.sql("SELECT store_is_open()").query(Boolean::class.javaObjectType).single()
    }

    /**
     * Binds [client] to [scope] for life, as its first mutation would, and returns false when it already belongs to
     * another scope. Call it before issuing a sync token.
     */
    public fun claimClient(scope: UUID, client: UUID): Boolean =
        translated {
            jdbc
                .sql("SELECT store_claim_client(?, ?)")
                .params(scope, client)
                .query(Boolean::class.javaObjectType)
                .single()
        }

    /** Removes [scope] with its clients and log, once the product has deleted its rows. */
    public fun dropScope(scope: UUID) {
        translated { jdbc.sql("SELECT store_drop_scope(?)").param(scope).query().singleRow() }
    }

    /**
     * Records the open mutation as rejected with [reason] and returns its version.
     * Roll back to a savepoint taken right after [begin] first, so none of its row changes stay.
     */
    public fun reject(reason: String): Long =
        close { jdbc.sql("SELECT store_reject(?)").param(reason).query(Long::class.javaObjectType).single() }

    /** The version that recorded an already processed mutation, or the scope head if the log no longer holds it. */
    public fun recordedVersion(scope: UUID, client: UUID, mutationId: Long): Long =
        translated {
            jdbc
                .sql(
                    """
                    SELECT coalesce(
                      (SELECT version FROM store_log
                        WHERE scope_id = ? AND client_id = ? AND mutation_id = ? AND seq = 0),
                      (SELECT head_version FROM store_scope WHERE scope_id = ?),
                      0)
                    """.trimIndent(),
                ).params(scope, client, mutationId, scope)
                .query(Long::class.javaObjectType)
                .single()
        }

    /** Sets a savepoint on the transaction's connection, normally right after [begin]. */
    public fun savepoint(): Savepoint = connectionHolder().createSavepoint()

    /**
     * Rolls back to [savepoint], undoing every row change made after it, and clears a rollback-only mark
     * left by code that failed inside the transaction.
     */
    public fun rollbackTo(savepoint: Savepoint) {
        val holder = connectionHolder()
        holder.connection.rollback(savepoint)
        holder.resetRollbackOnly()
    }

    /** Releases [savepoint] without undoing anything. */
    public fun release(savepoint: Savepoint) {
        connectionHolder().connection.releaseSavepoint(savepoint)
    }

    /** Signals that [scope] reached [version] on a separate connection. Failures are logged, since pokes are hints. */
    public fun poke(scope: UUID, version: Long) {
        try {
            dataSource.connection.use { connection ->
                connection.prepareStatement("SELECT store_poke(?, ?)").use { statement ->
                    statement.setObject(1, scope)
                    statement.setLong(2, version)
                    statement.execute()
                }
                if (!connection.autoCommit) connection.commit()
            }
        } catch (error: Exception) {
            log.warn("store_poke failed for scope {} at version {}", scope, version, error)
        }
    }

    private fun close(statement: () -> Long): Long {
        requireTransaction()
        val scope = openScope()
        val version = translated(statement)
        pokeAfterCommit(scope, version)
        return version
    }

    private fun pokeAfterCommit(scope: UUID, version: Long) {
        TransactionSynchronizationManager.registerSynchronization(
            object : TransactionSynchronization {
                override fun afterCommit() = poke(scope, version)
            },
        )
    }

    private fun openScope(): UUID {
        val scope: String? =
            jdbc.sql("SELECT current_setting('store.scope', true)").query(String::class.java).list().firstOrNull()
        if (scope.isNullOrEmpty()) {
            throw MutationNotOpenException("store: no mutation is open; call store_begin first")
        }
        return UUID.fromString(scope)
    }

    private fun connectionHolder(): ConnectionHolder =
        TransactionSynchronizationManager.getResource(dataSource) as? ConnectionHolder
            ?: throw IllegalStateException("store: savepoints need a transaction bound to the write path's DataSource")

    private fun requireTransaction() {
        check(
            TransactionSynchronizationManager.isActualTransactionActive() &&
                TransactionSynchronizationManager.isSynchronizationActive(),
        ) { "store: the write path must run inside a Spring-managed transaction" }
    }

    private fun <T> translated(block: () -> T): T =
        try {
            block()
        } catch (error: RuntimeException) {
            throw StoreSqlErrors.translate(error) ?: error
        }

    private companion object {
        private val log = LoggerFactory.getLogger(StoreWritePath::class.java)
    }
}
