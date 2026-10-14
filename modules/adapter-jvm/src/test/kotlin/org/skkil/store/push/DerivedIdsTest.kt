package org.skkil.store.push

import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.CsvSource
import tools.jackson.databind.json.JsonMapper
import java.util.UUID
import kotlin.test.Test
import kotlin.test.assertEquals

class DerivedIdsTest {
    @ParameterizedTest
    @CsvSource(
        "00000000-0000-0000-0000-000000000001, 1, 0, d8523ec3-be15-8c7f-8d56-ff02afbc9a4a",
        "00000000-0000-0000-0000-000000000001, 1, 1, cf36d881-2a42-8ecc-b53c-1673618bfdf7",
        "0190f1a2-7b3c-7d4e-8f50-6a7b8c9d0e1f, 42, 0, c0c243ed-dd0c-8c63-9875-f81a8c458ef1",
        "0190f1a2-7b3c-7d4e-8f50-6a7b8c9d0e1f, -1, 7, f24bd5ae-25a0-8362-8ded-914b414c1df7",
        "01923e4c-8b1a-7c3d-9e2f-0a1b2c3d4e5f, 1, 0, 7c95d028-8ca6-8ddf-b6a6-888317f17cb1",
        "01923e4c-8b1a-7c3d-9e2f-0a1b2c3d4e5f, 1, 1, ae99a64d-15cf-8772-be5b-3c463e449794",
        "01923e4c-8b1a-7c3d-9e2f-0a1b2c3d4e5f, 42, 3, 12f46dfd-c2c8-87d8-a894-9c145fa02861",
    )
    fun `derived ids match the fixed vectors`(client: String, mutationId: Long, n: Int, expected: String) {
        val id = DerivedIds.derive(UUID.fromString(client), mutationId, n)

        assertEquals(UUID.fromString(expected), id)
        assertEquals(8, id.version())
        assertEquals(2, id.variant())
    }

    @Test
    fun `nextId counts from zero within one mutation`() {
        val client = UUID.fromString("00000000-0000-0000-0000-000000000001")
        val context =
            MutationContext(UUID.randomUUID(), client, 1, "x", JsonMapper.builder().build().createObjectNode(), 0)

        assertEquals(
            listOf(DerivedIds.derive(client, 1, 0), DerivedIds.derive(client, 1, 1)),
            listOf(context.nextId(), context.nextId()),
        )
    }
}
