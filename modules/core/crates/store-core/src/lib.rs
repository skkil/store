//! The sans-IO client core of skkil store.
//!
//! `store-core` keeps a product's local SQLite tables in sync with its server
//! without opening a socket. It works on the product's own
//! [`rusqlite::Connection`]: it adds a few `store_*` tables, records
//! pre-images of rows touched by local mutations, queues those mutations,
//! builds `PushRequest`/`PullRequest` messages and applies the responses,
//! rebasing pending mutations over what the server decided. A host (the
//! `skkil-store-http` driver on desktop, a JavaScript host in browsers and
//! React Native) moves the bytes.
//!
//! A typical session:
//!
//! 1. [`install`], then [`register_table`] for every synced table.
//! 2. [`bind_scope`] once the user signed in.
//! 3. [`mutate`] for every user action, through the product's [`Mutator`].
//! 4. The host loops [`push_request`] → server → [`apply_push_response`] and
//!    [`pull_request`] → store service → [`apply_pull_response`].
//! 5. [`status`] and [`issues`] for the UI; [`forget`] on sign-out.
//!
//! See `docs/protocol.md` for the contract with the server.

mod clock;
mod error;
mod mutation;
mod session;
mod storage;
mod sync;

pub use clock::Hlc;
pub use error::{Error, Result};
pub use mutation::{MutationContext, MutationError, MutationOutcome, Mutator, derive_id, mutate};
pub use session::{
    SyncIssue, SyncStatus, bind_scope, dismiss_issue, forget, issues, set_product_schema_version,
    status, status_json,
};
pub use storage::{SCHEMA_VERSION, install, register_table};
pub use sync::{
    PullOutcome, PushOutcome, apply_pull_response, apply_pull_response_bytes, apply_push_response,
    apply_push_response_bytes, pull_request, pull_request_bytes, push_request, push_request_bytes,
};

pub use rusqlite;
pub use serde_json;
pub use store_protocol as protocol;
pub use uuid;
