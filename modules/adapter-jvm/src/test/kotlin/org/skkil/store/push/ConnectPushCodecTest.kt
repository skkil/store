package org.skkil.store.push

import org.skkil.store.v1.MutationResult
import org.skkil.store.v1.PushRequest
import org.skkil.store.v1.PushResponse
import org.skkil.store.writepath.ClientScopeMismatchException
import tools.jackson.databind.json.JsonMapper
import java.io.ByteArrayOutputStream
import java.util.zip.GZIPOutputStream
import kotlin.test.Test
import kotlin.test.assertEquals

class ConnectPushCodecTest {
    private val codec = ConnectPushCodec()
    private val json = JsonMapper.builder().build()
    private val request = PushRequest.newBuilder().setProtocolVersion(1).setClientId("c").build()
    private val response =
        PushResponse
            .newBuilder()
            .addResults(
                MutationResult
                    .newBuilder()
                    .setMutationId(1)
                    .setStatus(MutationResult.Status.STATUS_APPLIED)
                    .setVersion(9),
            ).build()

    @Test
    fun `binary bodies round trip`() {
        var received: PushRequest? = null
        val reply =
            codec.handle(request.toByteArray(), "application/proto") {
                received = it
                response
            }

        assertEquals(request, received)
        assertEquals(200, reply.status)
        assertEquals("application/proto", reply.contentType)
        assertEquals(response, PushResponse.parseFrom(reply.body))
    }

    @Test
    fun `json bodies round trip and ignore unknown fields`() {
        val body = """{"protocolVersion":1,"clientId":"c","futureField":true}"""
        var received: PushRequest? = null
        val reply =
            codec.handle(body.encodeToByteArray(), "application/json; charset=utf-8") {
                received = it
                response
            }

        assertEquals(request, received)
        assertEquals("application/json", reply.contentType)
        val results = json.readTree(reply.body)["results"][0]
        assertEquals("STATUS_APPLIED", results["status"].asString())
        assertEquals("9", results["version"].asString())
    }

    @Test
    fun `gzip request bodies are inflated`() {
        val compressed =
            ByteArrayOutputStream().also { out ->
                GZIPOutputStream(out).use { it.write(request.toByteArray()) }
            }
        var received: PushRequest? = null
        codec.handle(compressed.toByteArray(), "application/proto", "gzip") {
            received = it
            response
        }

        assertEquals(request, received)
    }

    @Test
    fun `an unsupported content type is 415`() {
        val reply = codec.handle(ByteArray(0), "text/plain") { response }

        assertEquals(415, reply.status)
        assertEquals("unimplemented", json.readTree(reply.body)["code"].asString())
    }

    @Test
    fun `a malformed body is an invalid argument`() {
        val reply = codec.handle("{".encodeToByteArray(), "application/json") { response }

        assertEquals(400, reply.status)
        assertEquals("invalid_argument", json.readTree(reply.body)["code"].asString())
    }

    @Test
    fun `failures map to Connect errors`() {
        assertError(400, "failed_precondition", PushException(ConnectCode.FAILED_PRECONDITION, "protocol"))
        assertError(403, "permission_denied", ClientScopeMismatchException("other scope"))
        assertError(500, "internal", IllegalStateException("secret detail"), "push failed; retry later")
    }

    private fun assertError(status: Int, code: String, failure: Exception, message: String = failure.message!!) {
        val reply = codec.handle(request.toByteArray(), "application/proto") { throw failure }
        val body = json.readTree(reply.body)

        assertEquals(status, reply.status)
        assertEquals("application/json", reply.contentType)
        assertEquals(code, body["code"].asString())
        assertEquals(message, body["message"].asString())
    }
}
