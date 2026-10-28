package org.skkil.store.support

import org.postgresql.PGConnection
import org.postgresql.ds.PGSimpleDataSource
import org.springframework.jdbc.core.simple.JdbcClient
import org.springframework.jdbc.datasource.DataSourceTransactionManager
import org.springframework.transaction.support.TransactionTemplate
import org.testcontainers.postgresql.PostgreSQLContainer
import java.nio.file.Files
import java.nio.file.Path
import java.sql.Connection
import java.util.UUID
import javax.sql.DataSource

object StoreDatabase {
    private val MIGRATION = Regex("""V\d+__.+\.sql""")

    private val container =
        PostgreSQLContainer("postgres:17-alpine").apply { start() }

    val dataSource: DataSource =
        PGSimpleDataSource().apply {
            setUrl(container.jdbcUrl)
            user = container.username
            password = container.password
        }

    val transactionManager = DataSourceTransactionManager(dataSource)

    val transactions = TransactionTemplate(transactionManager)

    val jdbc: JdbcClient = JdbcClient.create(dataSource)

    init {
        val migrations = Path.of(System.getProperty("store.migrations"))
        dataSource.connection.use { connection ->
            Files
                .list(migrations)
                .use { files -> files.filter { it.fileName.toString().matches(MIGRATION) }.toList() }
                .sortedBy { it.fileName.toString().substringAfter('V').substringBefore("__").toInt() }
                .forEach { file -> connection.createStatement().use { it.execute(Files.readString(file)) } }
            connection.createStatement().use {
                it.execute(
                    "CREATE TABLE note (id uuid PRIMARY KEY, scope_id uuid NOT NULL, title text NOT NULL, secret text)",
                )
                it.execute("SELECT store_register('note', 'id', ARRAY['secret'])")
            }
        }
    }

    fun notes(scope: UUID): List<String> =
        jdbc.sql(
            "SELECT title FROM note WHERE scope_id = ? ORDER BY title",
        ).param(scope).query(String::class.java).list().filterNotNull()

    fun insertNote(scope: UUID, title: String, id: UUID = UUID.randomUUID()) {
        jdbc.sql("INSERT INTO note (id, scope_id, title) VALUES (?, ?, ?)").params(id, scope, title).update()
    }

    fun insertUnlogged(scope: UUID, title: String, id: UUID = UUID.randomUUID()) {
        transactions.execute {
            jdbc.sql("SET LOCAL store.bypass = on").update()
            jdbc
                .sql("INSERT INTO note (id, scope_id, title, secret) VALUES (?, ?, ?, 'hidden')")
                .params(id, scope, title)
                .update()
        }
    }

    fun patches(scope: UUID): List<String> =
        jdbc
            .sql(
                "SELECT row_id || ' ' || data::text FROM store_log WHERE scope_id = ? AND seq > 0 ORDER BY version, seq",
            )
            .param(scope)
            .query(String::class.java)
            .list()
            .filterNotNull()

    fun epoch(scope: UUID): Int? =
        jdbc
            .sql("SELECT epoch FROM store_scope WHERE scope_id = ?")
            .param(scope)
            .query(Int::class.javaObjectType)
            .list()
            .firstOrNull()

    fun headVersion(scope: UUID): Long =
        jdbc
            .sql("SELECT head_version FROM store_scope WHERE scope_id = ?")
            .param(scope)
            .query(Long::class.javaObjectType)
            .list()
            .filterNotNull()
            .firstOrNull() ?: 0

    fun logKinds(scope: UUID): List<String> =
        jdbc
            .sql("SELECT version || ':' || kind FROM store_log WHERE scope_id = ? AND seq = 0 ORDER BY version")
            .param(scope)
            .query(String::class.java)
            .list()
            .filterNotNull()

    fun <T> listening(block: (PokeListener) -> T): T =
        dataSource.connection.use { connection ->
            connection.createStatement().use { it.execute("LISTEN store_poke") }
            block(PokeListener(connection))
        }

    class PokeListener(
        private val connection: Connection,
    ) {
        fun next(timeoutMillis: Int = 2000): List<String> =
            connection
                .unwrap(PGConnection::class.java)
                .getNotifications(timeoutMillis)
                .orEmpty()
                .map { it.parameter }
    }
}
