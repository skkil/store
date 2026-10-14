package org.skkil.store.config

import org.skkil.store.push.ConnectPushCodec
import org.skkil.store.push.MutationRegistry
import org.skkil.store.push.PushHandler
import org.skkil.store.push.RejectionPolicy
import org.skkil.store.token.SyncTokenIssuer
import org.skkil.store.token.SyncTokenKeys
import org.skkil.store.writepath.StoreWritePath
import org.springframework.boot.autoconfigure.AutoConfigurations
import org.springframework.boot.test.context.runner.ApplicationContextRunner
import org.springframework.jdbc.datasource.DataSourceTransactionManager
import org.springframework.jdbc.datasource.SimpleDriverDataSource
import org.springframework.transaction.PlatformTransactionManager
import java.security.interfaces.ECPrivateKey
import java.security.interfaces.ECPublicKey
import javax.sql.DataSource
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class StoreAutoConfigurationTest {
    private val runner =
        ApplicationContextRunner()
            .withConfiguration(AutoConfigurations.of(StoreAutoConfiguration::class.java))
            .withBean(DataSource::class.java, { SimpleDriverDataSource() })
            .withBean(PlatformTransactionManager::class.java, {
                DataSourceTransactionManager(SimpleDriverDataSource())
            })

    @Test
    fun `the write path, registry, push handler and codec are wired`() {
        runner.run { context ->
            listOf(
                StoreWritePath::class.java,
                MutationRegistry::class.java,
                RejectionPolicy::class.java,
                PushHandler::class.java,
                ConnectPushCodec::class.java,
            ).forEach { assertEquals(1, context.getBeanNamesForType(it).size, it.simpleName) }
            assertTrue(context.getBeanNamesForType(SyncTokenIssuer::class.java).isEmpty())
        }
    }

    @Test
    fun `the sync token issuer is wired from properties`() {
        val keys = SyncTokenKeys.generate()
        runner
            .withPropertyValues(
                "skkil.store.token.issuer=time",
                "skkil.store.token.private-key=${SyncTokenKeys.privateKeyPem(keys.private as ECPrivateKey)}",
                "skkil.store.token.public-key=${SyncTokenKeys.publicKeyPem(keys.public as ECPublicKey)}",
                "skkil.store.token.ttl=5m",
            ).run { context ->
                val issuer = context.getBean(SyncTokenIssuer::class.java)
                assertEquals(SyncTokenKeys.publicKeyPem(keys.public as ECPublicKey), issuer.publicKeyPem())
            }
    }

    @Test
    fun `application beans take precedence`() {
        val policy = RejectionPolicy { true }
        runner.withBean(RejectionPolicy::class.java, { policy }).run { context ->
            assertEquals(policy, context.getBean(RejectionPolicy::class.java))
        }
    }
}
