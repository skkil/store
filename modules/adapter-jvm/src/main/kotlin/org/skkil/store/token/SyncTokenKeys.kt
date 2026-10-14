package org.skkil.store.token

import java.security.KeyFactory
import java.security.KeyPair
import java.security.KeyPairGenerator
import java.security.interfaces.ECPrivateKey
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec
import java.security.spec.PKCS8EncodedKeySpec
import java.security.spec.X509EncodedKeySpec
import java.util.Base64

/** P-256 keys for ES256 sync tokens, as PKCS#8 (private) and SPKI (public) PEM. */
public object SyncTokenKeys {
    private const val PRIVATE_LABEL = "PRIVATE KEY"
    private const val PUBLIC_LABEL = "PUBLIC KEY"

    /** A new P-256 key pair. */
    public fun generate(): KeyPair =
        KeyPairGenerator.getInstance("EC").apply { initialize(ECGenParameterSpec("secp256r1")) }.generateKeyPair()

    /** [key] as a PKCS#8 PEM. */
    public fun privateKeyPem(key: ECPrivateKey): String = pem(PRIVATE_LABEL, key.encoded)

    /** [key] as an SPKI PEM, the form the store service is configured with. */
    public fun publicKeyPem(key: ECPublicKey): String = pem(PUBLIC_LABEL, key.encoded)

    /** Reads a PKCS#8 `PRIVATE KEY` PEM. */
    public fun readPrivateKey(pem: String): ECPrivateKey =
        KeyFactory.getInstance("EC").generatePrivate(PKCS8EncodedKeySpec(der(PRIVATE_LABEL, pem))) as ECPrivateKey

    /** Reads an SPKI `PUBLIC KEY` PEM. */
    public fun readPublicKey(pem: String): ECPublicKey =
        KeyFactory.getInstance("EC").generatePublic(X509EncodedKeySpec(der(PUBLIC_LABEL, pem))) as ECPublicKey

    private fun pem(label: String, der: ByteArray): String {
        val body = Base64.getMimeEncoder(64, "\n".toByteArray()).encodeToString(der)
        return "-----BEGIN $label-----\n$body\n-----END $label-----\n"
    }

    private fun der(label: String, pem: String): ByteArray {
        val begin = "-----BEGIN $label-----"
        val end = "-----END $label-----"
        require(begin in pem && end in pem) { "store: expected a PEM with $begin" }
        val body = pem.substringAfter(begin).substringBefore(end).filterNot { it.isWhitespace() }
        return Base64.getDecoder().decode(body)
    }
}
