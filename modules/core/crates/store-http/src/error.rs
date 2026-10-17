/// Everything that can go wrong while talking to the servers.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// The request did not complete: connection, TLS, timeout or I/O.
    #[error("transport: {0}")]
    Transport(#[from] ureq::Error),
    /// Reading the response stream failed.
    #[error("io: {0}")]
    Io(#[from] std::io::Error),
    /// The server answered with a Connect error.
    #[error("{code}: {message}")]
    Rpc {
        /// The Connect error code, such as `unauthenticated` or `unavailable`.
        code: String,
        /// The server's message.
        message: String,
    },
    /// A response could not be decoded.
    #[error("decode: {0}")]
    Decode(#[from] prost::DecodeError),
    /// The response broke the Connect protocol.
    #[error("connect protocol: {0}")]
    Protocol(String),
    /// The credentials could not provide a token.
    #[error("credentials: {0}")]
    Credentials(String),
    /// The store core failed while building a request or applying a response.
    #[error("store core: {0}")]
    Core(#[from] store_core::Error),
}

impl Error {
    /// True for failures worth retrying later with the same request:
    /// transport errors and the Connect codes `unavailable`,
    /// `deadline_exceeded`, `resource_exhausted`, `aborted` and `internal`.
    pub fn is_retryable(&self) -> bool {
        match self {
            Self::Transport(_) | Self::Io(_) => true,
            Self::Rpc { code, .. } => matches!(
                code.as_str(),
                "unavailable" | "deadline_exceeded" | "resource_exhausted" | "aborted" | "internal"
            ),
            _ => false,
        }
    }
}

/// The result type of the HTTP driver.
pub type Result<T> = std::result::Result<T, Error>;
