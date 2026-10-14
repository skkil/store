package org.skkil.store.writepath

/** SQLSTATE `22023`: a reserved client id or a mutation id below 1 was passed to the write path. */
public class InvalidStoreArgumentException(
    message: String,
    cause: Throwable? = null,
) : StoreException(message, cause)
