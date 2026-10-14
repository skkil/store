package org.skkil.store.token

import com.nimbusds.jose.JOSEObjectType
import com.nimbusds.jose.JWSAlgorithm
import com.nimbusds.jose.JWSHeader
import com.nimbusds.jose.JWSObject
import com.nimbusds.jose.Payload
import com.nimbusds.jose.crypto.ECDSASigner
import com.nimbusds.jose.crypto.ECDSAVerifier
import com.nimbusds.jose.jwk.Curve
import com.nimbusds.jose.jwk.ECKey
import com.nimbusds.jose.jwk.JWKSet
import com.nimbusds.jose.jwk.KeyUse
import com.nimbusds.jwt.JWTClaimsSet
import com.nimbusds.jwt.SignedJWT
import java.security.interfaces.ECPrivateKey
import java.security.interfaces.ECPublicKey
import java.time.Clock
import java.time.Duration
import java.time.temporal.ChronoUnit
import java.util.Date
import java.util.UUID

/**
 * Issues ES256 sync tokens (`docs/protocol.md`, Sync token) that the store service verifies with the
 * public key from [publicKeyPem] or [jwks].
 */
public class SyncTokenIssuer(
    private val issuer: String,
    privateKey: ECPrivateKey,
    publicKey: ECPublicKey,
    private val ttl: Duration = MAX_TTL,
    private val clock: Clock = Clock.systemUTC(),
) {
    private val jwk: ECKey =
        ECKey
            .Builder(Curve.P_256, publicKey)
            .privateKey(privateKey)
            .keyUse(KeyUse.SIGNATURE)
            .algorithm(JWSAlgorithm.ES256)
            .keyIDFromThumbprint()
            .build()
    private val signer = ECDSASigner(jwk)

    init {
        require(issuer.isNotBlank()) { "store: the sync token issuer must not be blank" }
        require(!ttl.isNegative && !ttl.isZero && ttl <= MAX_TTL) { "store: a sync token lives at most $MAX_TTL" }
        val probe = JWSObject(JWSHeader(JWSAlgorithm.ES256), Payload("probe")).apply { sign(signer) }
        require(probe.verify(ECDSAVerifier(publicKey))) { "store: the public key does not match the private key" }
    }

    /** A token that lets [client] pull and receive pokes for [scope]. */
    public fun issue(scope: UUID, client: UUID): SyncToken {
        val issuedAt = clock.instant().truncatedTo(ChronoUnit.SECONDS)
        val expiresAt = issuedAt.plus(ttl)
        val claims =
            JWTClaimsSet
                .Builder()
                .issuer(issuer)
                .audience(AUDIENCE)
                .subject(scope.toString())
                .claim(CLIENT_CLAIM, client.toString())
                .issueTime(Date.from(issuedAt))
                .expirationTime(Date.from(expiresAt))
                .build()
        val header = JWSHeader.Builder(JWSAlgorithm.ES256).type(JOSEObjectType.JWT).keyID(jwk.keyID).build()
        val jwt = SignedJWT(header, claims).apply { sign(signer) }
        return SyncToken(jwt.serialize(), expiresAt)
    }

    /** The key id (RFC 7638 thumbprint) written to each token's `kid` header. */
    public fun keyId(): String = jwk.keyID

    /** The verification key as an SPKI PEM. */
    public fun publicKeyPem(): String = SyncTokenKeys.publicKeyPem(jwk.toECPublicKey())

    /** The verification key as a JWKS JSON document, suitable for serving at a JWKS URL. */
    public fun jwks(): String = JWKSet(jwk.toPublicJWK()).toString()

    public companion object {
        /** The `aud` every sync token carries. */
        public const val AUDIENCE: String = "skkil-store"

        /** The claim that carries the client id. */
        public const val CLIENT_CLAIM: String = "cid"

        /** The longest lifetime the protocol allows. */
        public val MAX_TTL: Duration = Duration.ofMinutes(15)

        /** An issuer from PKCS#8 and SPKI PEM strings. */
        public fun fromPem(
            issuer: String,
            privateKeyPem: String,
            publicKeyPem: String,
            ttl: Duration = MAX_TTL,
            clock: Clock = Clock.systemUTC(),
        ): SyncTokenIssuer =
            SyncTokenIssuer(
                issuer,
                SyncTokenKeys.readPrivateKey(privateKeyPem),
                SyncTokenKeys.readPublicKey(publicKeyPem),
                ttl,
                clock,
            )
    }
}
