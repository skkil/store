package org.skkil.store.push

import tools.jackson.databind.JsonNode
import java.time.Clock
import java.time.Instant
import java.util.UUID

/** What a [MutationHandler] knows about the mutation it runs. */
public class MutationContext(
    /** The scope the mutation writes to. */
    public val scope: UUID,
    /** The device that made the mutation. */
    public val clientId: UUID,
    /** The mutation's id within [clientId]. */
    public val mutationId: Long,
    /** The registered name, such as `task.create`. */
    public val mutationName: String,
    /** The parsed arguments, also passed to the handler; kept here for logging. */
    public val args: JsonNode,
    /** The client's hybrid logical clock when the mutation was made, or 0. */
    public val hlc: Long,
    serverClock: Clock = Clock.systemUTC(),
) {
    private val wallTime: Instant = HybridLogicalClock.wallTime(hlc) ?: serverClock.instant()
    private var created = 0

    /**
     * When the mutation was made on the device, so a change made offline at 10:00 records 10:00.
     * Falls back to the server time when the client sent no clock. Use it instead of the server clock.
     */
    public fun now(): Instant = wallTime

    /** The next derived row id, the same one the device created optimistically. */
    public fun nextId(): UUID = DerivedIds.derive(clientId, mutationId, created++)
}
