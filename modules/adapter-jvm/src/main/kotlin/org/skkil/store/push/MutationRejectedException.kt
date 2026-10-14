package org.skkil.store.push

/** Thrown by a [MutationHandler] when a business rule refuses the mutation; [message] is shown to the user. */
public class MutationRejectedException(
    message: String,
) : RuntimeException(message)
