package org.skkil.store.push

import kotlin.reflect.KClass

/** Decides which handler exceptions are business-rule failures (rejected) rather than infrastructure failures (retried). */
public fun interface RejectionPolicy {
    /** True when [error] means the mutation can never apply and must be rejected. */
    public fun isRejection(error: Throwable): Boolean

    public companion object {
        /** Rejects [MutationRejectedException] only. */
        public val DEFAULT: RejectionPolicy = of(MutationRejectedException::class)

        /** Rejects any exception that is an instance of one of [types]. */
        public fun of(vararg types: KClass<out Throwable>): RejectionPolicy =
            RejectionPolicy { error -> types.any { it.isInstance(error) } }
    }
}
