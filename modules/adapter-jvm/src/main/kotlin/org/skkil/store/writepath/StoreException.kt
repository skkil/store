package org.skkil.store.writepath

/** A write-path rule enforced by the `store_*` SQL functions was broken. */
public sealed class StoreException(
    message: String,
    cause: Throwable?,
) : RuntimeException(message, cause)
