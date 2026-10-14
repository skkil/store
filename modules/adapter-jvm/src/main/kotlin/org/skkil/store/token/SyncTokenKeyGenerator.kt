package org.skkil.store.token

import java.security.interfaces.ECPrivateKey
import java.security.interfaces.ECPublicKey

/** Prints a new development key pair for sync tokens: `./gradlew syncTokenKeys`. */
public fun main() {
    val pair = SyncTokenKeys.generate()
    print(SyncTokenKeys.privateKeyPem(pair.private as ECPrivateKey))
    print(SyncTokenKeys.publicKeyPem(pair.public as ECPublicKey))
}
