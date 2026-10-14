package org.skkil.store.push

import java.util.concurrent.ConcurrentHashMap

/** The product's mutations by name, such as `task.create`. */
public class MutationRegistry {
    private val handlers = ConcurrentHashMap<String, MutationHandler>()

    /** Registers [handler] under [name]; each name may be registered once. */
    public fun register(name: String, handler: MutationHandler): MutationRegistry {
        require(name.isNotBlank()) { "store: a mutation name must not be blank" }
        check(handlers.putIfAbsent(name, handler) == null) { "store: mutation $name is already registered" }
        return this
    }

    /** The handler registered under [name], or null. */
    public fun find(name: String): MutationHandler? = handlers[name]

    /** Every registered mutation name. */
    public fun names(): Set<String> = handlers.keys.toSet()
}
