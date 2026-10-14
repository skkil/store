package org.skkil.store.push

import java.nio.ByteBuffer
import java.security.MessageDigest
import java.util.UUID

/**
 * Row ids derived from a mutation, identical to `derive_id` in `store-core`.
 *
 * `uuidv8(sha256(client_id ‖ mutation_id ‖ n))`, where `client_id` is the 16 raw UUID bytes, `mutation_id` a
 * big-endian u64 and `n` a big-endian u32 counting from 0. The first 16 digest bytes become the UUID with its
 * version nibble set to 8 and its variant bits to `10`.
 */
public object DerivedIds {
    /** The [n]-th id (from 0) created by mutation [mutationId] of [clientId]. */
    public fun derive(clientId: UUID, mutationId: Long, n: Int): UUID {
        val input =
            ByteBuffer
                .allocate(28)
                .putLong(clientId.mostSignificantBits)
                .putLong(clientId.leastSignificantBits)
                .putLong(mutationId)
                .putInt(n)
                .array()
        val digest = MessageDigest.getInstance("SHA-256").digest(input)
        digest[6] = ((digest[6].toInt() and 0x0f) or 0x80).toByte()
        digest[8] = ((digest[8].toInt() and 0x3f) or 0x80).toByte()
        val bytes = ByteBuffer.wrap(digest, 0, 16)
        return UUID(bytes.long, bytes.long)
    }
}
