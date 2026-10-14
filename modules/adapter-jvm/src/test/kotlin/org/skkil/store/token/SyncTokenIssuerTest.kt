package org.skkil.store.token

import com.nimbusds.jose.crypto.ECDSAVerifier
import com.nimbusds.jose.jwk.ECKey
import com.nimbusds.jose.jwk.JWKSet
import com.nimbusds.jwt.SignedJWT
import org.junit.jupiter.api.assertThrows
import java.security.interfaces.ECPrivateKey
import java.security.interfaces.ECPublicKey
import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.time.ZoneOffset
import java.util.Date
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class SyncTokenIssuerTest {
    private val keys = SyncTokenKeys.generate()
    private val privateKey = keys.private as ECPrivateKey
    private val publicKey = keys.public as ECPublicKey
    private val now = Instant.parse("2026-11-08T12:00:00Z")
    private val issuer = SyncTokenIssuer("time", privateKey, publicKey, clock = Clock.fixed(now, ZoneOffset.UTC))
    private val scope = UUID.randomUUID()
    private val client = UUID.randomUUID()

    @Test
    fun `a token carries the protocol claims`() {
        val token = issuer.issue(scope, client)
        val claims = SignedJWT.parse(token.token).jwtClaimsSet

        assertEquals("time", claims.issuer)
        assertEquals(listOf("skkil-store"), claims.audience)
        assertEquals(scope.toString(), claims.subject)
        assertEquals(client.toString(), claims.getStringClaim("cid"))
        assertEquals(Date.from(now), claims.issueTime)
        assertEquals(Date.from(now.plus(Duration.ofMinutes(15))), claims.expirationTime)
        assertEquals(now.plus(Duration.ofMinutes(15)), token.expiresAt)
    }

    @Test
    fun `the signature verifies with the published PEM and JWKS`() {
        val jwt = SignedJWT.parse(issuer.issue(scope, client).token)

        assertEquals("ES256", jwt.header.algorithm.name)
        assertEquals(issuer.keyId(), jwt.header.keyID)
        assertTrue(jwt.verify(ECDSAVerifier(SyncTokenKeys.readPublicKey(issuer.publicKeyPem()))))
        val jwk = JWKSet.parse(issuer.jwks()).getKeyByKeyId(jwt.header.keyID) as ECKey
        assertTrue(!jwk.isPrivate)
        assertTrue(jwt.verify(ECDSAVerifier(jwk)))
    }

    @Test
    fun `a token from another key does not verify`() {
        val other = SyncTokenKeys.generate()
        val jwt = SignedJWT.parse(issuer.issue(scope, client).token)

        assertTrue(!jwt.verify(ECDSAVerifier(other.public as ECPublicKey)))
    }

    @Test
    fun `PEM keys round trip`() {
        val fromPem =
            SyncTokenIssuer.fromPem(
                "time",
                SyncTokenKeys.privateKeyPem(privateKey),
                SyncTokenKeys.publicKeyPem(publicKey),
            )

        assertEquals(issuer.keyId(), fromPem.keyId())
        assertTrue(SyncTokenKeys.publicKeyPem(publicKey).startsWith("-----BEGIN PUBLIC KEY-----\n"))
    }

    @Test
    fun `lifetimes over 15 minutes and mismatched keys are refused`() {
        assertThrows<IllegalArgumentException> {
            SyncTokenIssuer("time", privateKey, publicKey, Duration.ofMinutes(16))
        }
        assertThrows<IllegalArgumentException> {
            SyncTokenIssuer("time", privateKey, SyncTokenKeys.generate().public as ECPublicKey)
        }
    }
}
