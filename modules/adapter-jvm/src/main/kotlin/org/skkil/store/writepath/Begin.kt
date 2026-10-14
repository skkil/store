package org.skkil.store.writepath

/** The outcome of [StoreWritePath.begin]. */
public enum class Begin {
    /** The mutation is open: run it, then call [StoreWritePath.end] or [StoreWritePath.reject]. */
    BEGUN,

    /** This `(client, mutation)` was already processed: commit without running it. */
    DUPLICATE,
}
