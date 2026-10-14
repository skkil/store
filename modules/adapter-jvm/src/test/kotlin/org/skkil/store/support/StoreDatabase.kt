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
            connection.createStatement().use { it.execute(Files.readString(migrations.resolve("V1__store.sql"))) }
            connection.createStatement().use {
                it.execute("CREATE TABLE note (id uuid PRIMARY KEY, scope_id uuid NOT NULL, title text NOT NULL)")
                it.execute("SELECT store_register('note')")
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
