package org.skkil.store.push

import tools.jackson.databind.JsonNode

/** Runs one named mutation with plain SQL on synced tables, inside the write path's open mutation. */
public fun interface MutationHandler {
    /** Applies the mutation; throw an exception the [RejectionPolicy] accepts to reject it. */
    public fun handle(args: JsonNode, context: MutationContext)
}
