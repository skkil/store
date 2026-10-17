//! The native host for `skkil-store-core`: a blocking HTTP driver that pushes
//! to the product server, pulls from the store service and listens to its
//! poke stream, all over the Connect protocol with binary protobuf.
//!
//! [`HttpTransport`] makes single calls; [`Syncer`] runs a full sync round
//! (push everything, pull until caught up, push again if asked) against a
//! [`rusqlite::Connection`](store_core::rusqlite::Connection). Run
//! [`HttpTransport::pokes`] on its own thread and call [`Syncer::sync`] when a
//! poke is ahead of the cursor, on start, on reconnect and on a timer.
//!
//! The crate is empty on `wasm32`; browsers use the TypeScript driver.

#![cfg(not(target_arch = "wasm32"))]

mod connect;
mod credentials;
mod endpoints;
mod error;
mod pokes;
mod syncer;
mod transport;

pub use credentials::{Credentials, StaticCredentials};
pub use endpoints::Endpoints;
pub use error::{Error, Result};
pub use pokes::PokeStream;
pub use syncer::{SyncReport, Syncer};
pub use transport::HttpTransport;
