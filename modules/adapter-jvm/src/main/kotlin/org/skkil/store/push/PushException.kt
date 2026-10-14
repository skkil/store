package org.skkil.store.push

/** A push the server refuses as a whole; the client keeps its mutations and retries later. */
public class PushException(
    /** The Connect code sent to the client. */
    public val code: ConnectCode,
    message: String,
    cause: Throwable? = null,
) : RuntimeException(message, cause)
