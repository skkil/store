package org.skkil.store.writepath

/** SQLSTATE `42501`: the client id already belongs to another scope. */
public class ClientScopeMismatchException(
    message: String,
    cause: Throwable? = null,
) : StoreException(message, cause)
