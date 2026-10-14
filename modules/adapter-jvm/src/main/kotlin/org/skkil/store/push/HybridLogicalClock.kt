package org.skkil.store.push

import java.time.Instant

/** Decodes `Mutation.hlc`, encoded as `(unix_millis << 16) | counter`. */
public object HybridLogicalClock {
    private const val COUNTER_BITS = 16

    /** The wall time in [hlc], or null when the client sent none. */
    public fun wallTime(hlc: Long): Instant? = if (hlc <= 0) null else Instant.ofEpochMilli(hlc ushr COUNTER_BITS)
}
