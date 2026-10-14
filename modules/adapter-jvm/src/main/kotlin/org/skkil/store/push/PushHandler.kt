package org.skkil.store.push

import org.skkil.store.v1.Mutation
import org.skkil.store.v1.MutationResult
import org.skkil.store.v1.PushRequest
import org.skkil.store.v1.PushResponse
import org.skkil.store.writepath.Begin
import org.skkil.store.writepath.StoreWritePath
import org.springframework.transaction.support.TransactionSynchronizationManager
import org.springframework.transaction.support.TransactionTemplate
import tools.jackson.core.JacksonException
import tools.jackson.databind.JsonNode
import tools.jackson.databind.json.JsonMapper
import java.sql.Savepoint
import java.time.Clock
import java.util.UUID

/**
 * Applies a [PushRequest] for one scope: each mutation in its own transaction, in order.
 *
 * Unknown names, malformed arguments and exceptions accepted by [rejectionPolicy] become `REJECTED` results.
 * Any other exception aborts the batch and propagates, so the client retries it; mutations committed before
 * the failure come back as `DUPLICATE` on the retry.
 */
public class PushHandler(
    private val registry: MutationRegistry,
    private val writePath: StoreWritePath,
    private val transactionTemplate: TransactionTemplate,
    private val rejectionPolicy: RejectionPolicy = RejectionPolicy.DEFAULT,
    private val jsonMapper: JsonMapper = JsonMapper.builder().build(),
    private val clock: Clock = Clock.systemUTC(),
) {
    /** Applies [request] to [scope], the scope of the authenticated caller, and returns one result per mutation. */
    public fun push(scope: UUID, request: PushRequest): PushResponse {
        check(!TransactionSynchronizationManager.isActualTransactionActive()) {
            "store: push must not run inside a transaction; each mutation commits on its own"
        }
        if (request.protocolVersion != PROTOCOL_VERSION) {
            throw PushException(
                ConnectCode.FAILED_PRECONDITION,
                "protocol version ${request.protocolVersion} is not supported; this server speaks $PROTOCOL_VERSION",
            )
        }
        val client = parseClient(request.clientId)
        val results = request.mutationsList.map { mutation -> apply(scope, client, mutation) }
        return PushResponse.newBuilder().addAllResults(results).build()
    }

    private fun apply(scope: UUID, client: UUID, mutation: Mutation): MutationResult =
        checkNotNull(transactionTemplate.execute { run(scope, client, mutation) })

    private fun run(scope: UUID, client: UUID, mutation: Mutation): MutationResult {
        if (writePath.begin(scope, client, mutation.mutationId) == Begin.DUPLICATE) {
            val version = writePath.recordedVersion(scope, client, mutation.mutationId)
            return result(mutation, MutationResult.Status.STATUS_DUPLICATE, version)
        }
        val savepoint = writePath.savepoint()
        val handler =
            registry.find(mutation.name) ?: return reject(mutation, savepoint, "unknown mutation: ${mutation.name}")
        val args =
            parseArgs(mutation) ?: return reject(mutation, savepoint, "invalid arguments: expected a JSON object")
        val context = MutationContext(scope, client, mutation.mutationId, mutation.name, args, mutation.hlc, clock)
        try {
            handler.handle(args, context)
        } catch (error: Exception) {
            if (!rejectionPolicy.isRejection(error)) throw error
            return reject(mutation, savepoint, error.message ?: error.javaClass.simpleName)
        }
        writePath.release(savepoint)
        return result(mutation, MutationResult.Status.STATUS_APPLIED, writePath.end())
    }

    private fun reject(mutation: Mutation, savepoint: Savepoint, reason: String): MutationResult {
        writePath.rollbackTo(savepoint)
        val version = writePath.reject(reason)
        return result(mutation, MutationResult.Status.STATUS_REJECTED, version, reason)
    }

    private fun result(
        mutation: Mutation,
        status: MutationResult.Status,
        version: Long,
        reason: String = "",
    ): MutationResult =
        MutationResult
            .newBuilder()
            .setMutationId(mutation.mutationId)
            .setStatus(status)
            .setVersion(version)
            .setReason(reason)
            .build()

    private fun parseArgs(mutation: Mutation): JsonNode? {
        if (mutation.args.isEmpty) return jsonMapper.createObjectNode()
        return try {
            jsonMapper.readTree(mutation.args.toByteArray()).takeIf { it.isObject }
        } catch (_: JacksonException) {
            null
        }
    }

    private fun parseClient(clientId: String): UUID =
        try {
            UUID.fromString(clientId)
        } catch (error: IllegalArgumentException) {
            throw PushException(ConnectCode.INVALID_ARGUMENT, "client_id is not a UUID", error)
        }

    public companion object {
        /** The only wire protocol version this handler accepts. */
        public const val PROTOCOL_VERSION: Int = 1
    }
}
