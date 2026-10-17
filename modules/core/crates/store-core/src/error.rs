use uuid::Uuid;

use crate::mutation::MutationError;

/// Everything that can go wrong in the store core.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// SQLite failed.
    #[error("sqlite: {0}")]
    Sqlite(#[from] rusqlite::Error),
    /// A JSON value could not be read or written.
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    /// A protobuf message could not be decoded.
    #[error("protobuf: {0}")]
    Decode(#[from] prost::DecodeError),
    /// The product's mutator refused or failed; nothing was queued.
    #[error("mutation failed: {0}")]
    Mutation(#[from] MutationError),
    /// Mutation arguments must be a JSON object.
    #[error("mutation arguments must be a JSON object")]
    ArgsNotObject,
    /// The local store was written by a newer version of this crate.
    #[error("local store schema {found} is newer than the supported {supported}")]
    SchemaTooNew {
        /// The version found in `store_meta`.
        found: i64,
        /// The newest version this build understands.
        supported: i64,
    },
    /// `register_table` was given a table that does not exist.
    #[error("table {0} does not exist")]
    TableNotFound(String),
    /// `register_table` was given an id column the table does not have.
    #[error("table {table} has no column {column}")]
    ColumnNotFound {
        /// The table.
        table: String,
        /// The missing column.
        column: String,
    },
    /// The id column is neither the primary key nor covered by a unique index.
    #[error("column {column} of table {table} is not unique")]
    IdNotUnique {
        /// The table.
        table: String,
        /// The id column.
        column: String,
    },
    /// Tables whose names start with `store_` belong to the core.
    #[error("table {0} is reserved for the store")]
    ReservedTable(String),
    /// The local store already belongs to another scope.
    #[error("bound to scope {bound}, not {requested}")]
    ScopeMismatch {
        /// The scope the store is bound to.
        bound: Uuid,
        /// The scope that was asked for.
        requested: Uuid,
    },
    /// The server sent a response that breaks the protocol.
    #[error("protocol violation: {0}")]
    Protocol(String),
    /// Local bookkeeping holds a value the core never writes.
    #[error("corrupt local store: {0}")]
    Corrupt(String),
}

/// The result type of the store core.
pub type Result<T> = std::result::Result<T, Error>;
