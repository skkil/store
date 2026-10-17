use sha2::{Digest, Sha256};
use uuid::Uuid;

/// The `n`-th row id (counting from 0) created by mutation `mutation_id` of
/// `client_id`.
///
/// `uuidv8(sha256(client_id ‖ mutation_id ‖ n))` with the client id as its 16
/// raw bytes, the mutation id as a big-endian `u64` and `n` as a big-endian
/// `u32`. The first 16 digest bytes become the UUID, with version 8 and the
/// RFC 9562 variant. Servers replaying the mutation derive the same ids.
pub fn derive_id(client_id: Uuid, mutation_id: u64, n: u32) -> Uuid {
    let mut input = [0u8; 28];
    input[..16].copy_from_slice(client_id.as_bytes());
    input[16..24].copy_from_slice(&mutation_id.to_be_bytes());
    input[24..].copy_from_slice(&n.to_be_bytes());
    let digest = Sha256::digest(input);
    let mut bytes = [0u8; 16];
    bytes.copy_from_slice(&digest[..16]);
    Uuid::new_v8(bytes)
}

#[cfg(test)]
mod tests {
    use uuid::{Uuid, Variant};

    use super::derive_id;

    const CLIENT: Uuid = Uuid::from_u128(0x0192_3e4c_8b1a_7c3d_9e2f_0a1b_2c3d_4e5f);

    #[test]
    fn is_a_version_8_rfc_uuid() {
        let id = derive_id(CLIENT, 1, 0);
        assert_eq!(id.get_version_num(), 8);
        assert_eq!(id.get_variant(), Variant::RFC4122);
    }

    #[test]
    fn is_deterministic_and_distinct_per_input() {
        assert_eq!(derive_id(CLIENT, 1, 0), derive_id(CLIENT, 1, 0));
        assert_ne!(derive_id(CLIENT, 1, 0), derive_id(CLIENT, 1, 1));
        assert_ne!(derive_id(CLIENT, 1, 0), derive_id(CLIENT, 2, 0));
        assert_ne!(derive_id(CLIENT, 1, 0), derive_id(Uuid::nil(), 1, 0));
    }

    #[test]
    fn matches_the_published_test_vectors() {
        let vectors = [
            (1, 0, "7c95d028-8ca6-8ddf-b6a6-888317f17cb1"),
            (1, 1, "ae99a64d-15cf-8772-be5b-3c463e449794"),
            (42, 3, "12f46dfd-c2c8-87d8-a894-9c145fa02861"),
        ];
        for (mutation_id, n, expected) in vectors {
            assert_eq!(derive_id(CLIENT, mutation_id, n).to_string(), expected);
        }
    }
}
