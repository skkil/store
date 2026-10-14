package org.skkil.store.writepath

import java.sql.SQLException

/** Maps the SQLSTATEs raised by the `store_*` functions to typed [StoreException]s. */
public object StoreSqlErrors {
    private const val NOT_OPEN = "55000"
    private const val SCOPE_MISMATCH = "42501"
    private const val INVALID_ARGUMENT = "22023"

    /** Returns the [StoreException] for the first [SQLException] in [error]'s cause chain, or null if none applies. */
    public fun translate(error: Throwable): StoreException? {
        val sqlError =
            generateSequence(error) { it.cause }.filterIsInstance<SQLException>().firstOrNull() ?: return null
        val message = sqlError.message?.lineSequence()?.first()?.removePrefix("ERROR: ") ?: sqlError.sqlState
        return when (sqlError.sqlState) {
            NOT_OPEN -> MutationNotOpenException(message, error)
            SCOPE_MISMATCH -> ClientScopeMismatchException(message, error)
            INVALID_ARGUMENT -> InvalidStoreArgumentException(message, error)
            else -> null
        }
    }
}
