package org.skkil.store.push

import com.google.protobuf.InvalidProtocolBufferException
import com.google.protobuf.util.JsonFormat
import org.skkil.store.v1.PushRequest
import org.skkil.store.v1.PushResponse
import org.skkil.store.writepath.ClientScopeMismatchException
import org.skkil.store.writepath.InvalidStoreArgumentException
import org.slf4j.LoggerFactory
import tools.jackson.databind.json.JsonMapper
import java.io.ByteArrayInputStream
import java.util.zip.GZIPInputStream

/**
 * The Connect unary protocol for `skkil.store.v1.PushService/Push`: `application/proto` and `application/json`
 * bodies, optional gzip request bodies, and Connect error JSON for anything that is not a 2xx answer.
 */
public class ConnectPushCodec(
    private val jsonMapper: JsonMapper = JsonMapper.builder().build(),
) {
    private val parser = JsonFormat.parser().ignoringUnknownFields()
    private val printer = JsonFormat.printer()

    /** Decodes [body], pushes it with [push] and encodes the response or the error as a [ConnectReply]. */
    public fun handle(
        body: ByteArray,
        contentType: String?,
        contentEncoding: String? = null,
        push: (PushRequest) -> PushResponse,
    ): ConnectReply {
        val format =
            Format.of(contentType)
                ?: return error(
                    ConnectCode.UNIMPLEMENTED,
                    "unsupported content type: $contentType",
                    UNSUPPORTED_MEDIA_TYPE,
                )
        return try {
            val request = decode(inflate(body, contentEncoding), format)
            ConnectReply(OK, format.mediaType, encode(push(request), format))
        } catch (failure: Exception) {
            error(failure)
        }
    }

    /** Decodes a request body in the given [Format]. */
    public fun decode(body: ByteArray, format: Format): PushRequest =
        when (format) {
            Format.PROTO -> PushRequest.parseFrom(body)
            Format.JSON -> PushRequest.newBuilder().also { parser.merge(body.decodeToString(), it) }.build()
        }

    /** Encodes a response body in the given [Format]. */
    public fun encode(response: PushResponse, format: Format): ByteArray =
        when (format) {
            Format.PROTO -> response.toByteArray()
            Format.JSON -> printer.print(response).encodeToByteArray()
        }

    /** The Connect error reply for [failure], hiding the details of unexpected exceptions. */
    public fun error(failure: Throwable): ConnectReply =
        when (failure) {
            is PushException -> {
                error(failure.code, failure.message.orEmpty())
            }

            is InvalidProtocolBufferException -> {
                error(ConnectCode.INVALID_ARGUMENT, "malformed request: ${failure.message}")
            }

            is ClientScopeMismatchException -> {
                error(ConnectCode.PERMISSION_DENIED, failure.message.orEmpty())
            }

            is InvalidStoreArgumentException -> {
                error(ConnectCode.INVALID_ARGUMENT, failure.message.orEmpty())
            }

            else -> {
                log.error("push failed; the client will retry the batch", failure)
                error(ConnectCode.INTERNAL, "push failed; retry later")
            }
        }

    /** The Connect error reply for [code] and [message]. */
    public fun error(code: ConnectCode, message: String, status: Int = code.httpStatus): ConnectReply {
        val body = jsonMapper.createObjectNode().put("code", code.wireName).put("message", message)
        return ConnectReply(status, Format.JSON.mediaType, jsonMapper.writeValueAsBytes(body))
    }

    private fun inflate(body: ByteArray, contentEncoding: String?): ByteArray =
        when (contentEncoding?.trim()?.lowercase()) {
            null, "", "identity" -> body
            "gzip" -> GZIPInputStream(ByteArrayInputStream(body)).use { it.readBytes() }
            else -> throw PushException(ConnectCode.UNIMPLEMENTED, "unsupported content encoding: $contentEncoding")
        }

    /** The two Connect unary codecs. */
    public enum class Format(
        /** The media type sent and expected in `Content-Type`. */
        public val mediaType: String,
    ) {
        PROTO("application/proto"),
        JSON("application/json"),
        ;

        public companion object {
            /** The format for a `Content-Type` header value, ignoring parameters, or null if unsupported. */
            public fun of(contentType: String?): Format? {
                val mediaType = contentType?.substringBefore(';')?.trim()?.lowercase() ?: return null
                return entries.firstOrNull { it.mediaType == mediaType }
            }
        }
    }

    private companion object {
        private const val OK = 200
        private const val UNSUPPORTED_MEDIA_TYPE = 415
        private val log = LoggerFactory.getLogger(ConnectPushCodec::class.java)
    }
}
