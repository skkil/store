package org.skkil.store.push

/** An HTTP response for a Connect unary call, ready to write as-is. */
public class ConnectReply(
    /** The HTTP status. */
    public val status: Int,
    /** The `Content-Type` header. */
    public val contentType: String,
    /** The response body. */
    public val body: ByteArray,
)
