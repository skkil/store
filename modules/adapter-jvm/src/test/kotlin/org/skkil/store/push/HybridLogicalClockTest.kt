package org.skkil.store.push

import java.time.Instant
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class HybridLogicalClockTest {
    @Test
    fun `the wall time is the upper 48 bits in milliseconds`() {
        val millis = Instant.parse("2026-11-08T10:00:00.123Z").toEpochMilli()

        assertEquals(Instant.ofEpochMilli(millis), HybridLogicalClock.wallTime((millis shl 16) or 0xffff))
    }

    @Test
    fun `zero means no clock`() {
        assertNull(HybridLogicalClock.wallTime(0))
    }
}
