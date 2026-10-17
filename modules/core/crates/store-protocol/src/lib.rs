//! Protobuf messages of the `skkil.store.v1` wire protocol.
//!
//! The code under `generated/` is produced by `cargo run -p protogen` from
//! `modules/protocol/proto` and committed, so builds for wasm, iOS and Android
//! never need `protoc`.

/// The protocol version this crate speaks, sent in every request.
pub const PROTOCOL_VERSION: u32 = 1;

/// Messages of the `skkil.store.v1` package.
#[allow(missing_docs, clippy::all)]
pub mod v1 {
    include!("generated/skkil.store.v1.rs");
}

pub use prost::{DecodeError, EncodeError, Message};
