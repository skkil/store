package org.skkil.store.writepath

/** SQLSTATE `55000`: a synced table was written, or a mutation closed, while no mutation was open. */
public class MutationNotOpenException(
    message: String,
    cause: Throwable? = null,
) : StoreException(message, cause)
