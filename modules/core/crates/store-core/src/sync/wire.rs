//! Byte-level wrappers for bindings (wasm-bindgen, uniffi) that pass
//! protobuf-encoded messages instead of Rust types.

use prost::Message;
use rusqlite::Connection;
use store_protocol::v1::{PullResponse, PushResponse};

use crate::{
    Result,
    mutation::Mutator,
    sync::{
        PullOutcome, PushOutcome, apply_pull_response, apply_push_response, pull_request,
        push_request,
    },
};

/// [`push_request`] encoded as protobuf bytes.
pub fn push_request_bytes(conn: &Connection, limit: u32) -> Result<Option<Vec<u8>>> {
    Ok(push_request(conn, limit)?.map(|request| request.encode_to_vec()))
}

/// [`apply_push_response`] for a protobuf-encoded `PushResponse`.
pub fn apply_push_response_bytes(conn: &Connection, response: &[u8]) -> Result<PushOutcome> {
    apply_push_response(conn, &PushResponse::decode(response)?)
}

/// [`pull_request`] encoded as protobuf bytes.
pub fn pull_request_bytes(conn: &Connection, limit: u32) -> Result<Vec<u8>> {
    Ok(pull_request(conn, limit)?.encode_to_vec())
}

/// [`apply_pull_response`] for a protobuf-encoded `PullResponse`.
pub fn apply_pull_response_bytes(
    conn: &mut Connection,
    mutator: &dyn Mutator,
    response: &[u8],
) -> Result<PullOutcome> {
    apply_pull_response(conn, mutator, &PullResponse::decode(response)?)
}
