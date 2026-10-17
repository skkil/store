/// Why a mutator did not apply a mutation.
///
/// Locally, any error rolls the mutation back and nothing is queued. During a
/// replay after a pull, the mutation stays pending but its effects are left
/// out until the server decides.
#[derive(Debug, thiserror::Error)]
pub enum MutationError {
    /// A business rule refused the mutation; the reason is shown to the user.
    #[error("{0}")]
    Rejected(String),
    /// The mutator does not know this mutation name.
    #[error("unknown mutation {0}")]
    Unknown(String),
    /// The arguments do not have the shape the mutation expects.
    #[error("invalid arguments: {0}")]
    InvalidArgs(String),
    /// SQLite failed while applying the mutation.
    #[error("sqlite: {0}")]
    Sqlite(#[from] rusqlite::Error),
    /// A JSON value could not be read or written.
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    /// Any other failure of the product's code.
    #[error("{0}")]
    Other(Box<dyn std::error::Error + Send + Sync>),
}

impl MutationError {
    /// A business-rule refusal with a reason readable by the user.
    pub fn rejected(reason: impl Into<String>) -> Self {
        Self::Rejected(reason.into())
    }
}
