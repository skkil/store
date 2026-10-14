package org.skkil.store.token

import java.time.Instant

/** A signed sync token and when it expires. */
public class SyncToken(
    /** The compact JWT, sent as `Authorization: Bearer <token>` to Pull and Pokes. */
    public val token: String,
    /** The token's `exp`. */
    public val expiresAt: Instant,
)
