package org.skkil.store.config

import org.skkil.store.push.ConnectPushCodec
import org.skkil.store.push.MutationRegistry
import org.skkil.store.push.PushHandler
import org.skkil.store.push.RejectionPolicy
import org.skkil.store.token.SyncTokenIssuer
import org.skkil.store.writepath.StoreWritePath
import org.springframework.boot.autoconfigure.AutoConfiguration
import org.springframework.boot.autoconfigure.condition.ConditionalOnBean
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty
import org.springframework.boot.context.properties.EnableConfigurationProperties
import org.springframework.context.annotation.Bean
import org.springframework.transaction.PlatformTransactionManager
import org.springframework.transaction.TransactionDefinition
import org.springframework.transaction.support.TransactionTemplate
import javax.sql.DataSource

/**
 * Wires the write path, the mutation registry, the Push handler and codec, and (when
 * `skkil.store.token.private-key` is set) the sync token issuer. Every bean backs off when the
 * application defines its own.
 */
@AutoConfiguration(
    afterName = [
        "org.springframework.boot.jdbc.autoconfigure.DataSourceAutoConfiguration",
        "org.springframework.boot.jdbc.autoconfigure.DataSourceTransactionManagerAutoConfiguration",
    ],
)
@EnableConfigurationProperties(StoreProperties::class)
public class StoreAutoConfiguration {
    /** The write path over the application's [DataSource]. */
    @Bean
    @ConditionalOnMissingBean
    @ConditionalOnBean(DataSource::class)
    public fun storeWritePath(dataSource: DataSource): StoreWritePath = StoreWritePath(dataSource)

    /** An empty registry the application fills with its mutations. */
    @Bean
    @ConditionalOnMissingBean
    public fun mutationRegistry(): MutationRegistry = MutationRegistry()

    /** Rejects [org.skkil.store.push.MutationRejectedException] only. */
    @Bean
    @ConditionalOnMissingBean
    public fun rejectionPolicy(): RejectionPolicy = RejectionPolicy.DEFAULT

    /** The Push handler, with each mutation in a new transaction. */
    @Bean
    @ConditionalOnMissingBean
    @ConditionalOnBean(PlatformTransactionManager::class, StoreWritePath::class)
    public fun pushHandler(
        registry: MutationRegistry,
        writePath: StoreWritePath,
        transactionManager: PlatformTransactionManager,
        rejectionPolicy: RejectionPolicy,
    ): PushHandler {
        val template =
            TransactionTemplate(transactionManager).apply {
                propagationBehavior = TransactionDefinition.PROPAGATION_REQUIRES_NEW
            }
        return PushHandler(registry, writePath, template, rejectionPolicy)
    }

    /** The Connect codec for `POST /skkil.store.v1.PushService/Push`. */
    @Bean
    @ConditionalOnMissingBean
    public fun connectPushCodec(): ConnectPushCodec = ConnectPushCodec()

    /** The sync token issuer from `skkil.store.token.*`. */
    @Bean
    @ConditionalOnMissingBean
    @ConditionalOnProperty("skkil.store.token.private-key")
    public fun syncTokenIssuer(properties: StoreProperties): SyncTokenIssuer {
        val token = properties.token
        return SyncTokenIssuer.fromPem(
            issuer = requireNotNull(token.issuer) { "store: set skkil.store.token.issuer" },
            privateKeyPem = requireNotNull(token.privateKey),
            publicKeyPem = requireNotNull(token.publicKey) { "store: set skkil.store.token.public-key" },
            ttl = token.ttl,
        )
    }
}
