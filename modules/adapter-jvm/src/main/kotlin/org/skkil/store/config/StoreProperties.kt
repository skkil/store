package org.skkil.store.config

import org.springframework.boot.context.properties.ConfigurationProperties
import org.springframework.boot.context.properties.bind.DefaultValue
import java.time.Duration

/** `skkil.store.*` settings read by [StoreAutoConfiguration]. */
@ConfigurationProperties("skkil.store")
public class StoreProperties(
    /** Sync token signing; tokens are issued only when [Token.privateKey] is set. */
    @DefaultValue
    public val token: Token,
) {
    /** `skkil.store.token.*`. */
    public class Token(
        /** The `iss` claim, the product name such as `time`. */
        public val issuer: String?,
        /** The signing key as a PKCS#8 PEM. */
        public val privateKey: String?,
        /** The matching verification key as an SPKI PEM. */
        public val publicKey: String?,
        /** Token lifetime, at most 15 minutes. */
        @DefaultValue("15m")
        public val ttl: Duration,
    )
}
