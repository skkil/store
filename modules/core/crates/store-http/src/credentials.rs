use crate::Result;

/// Supplies the bearer tokens for each call. Called before every request,
/// so implementations can refresh expired tokens.
pub trait Credentials {
    /// The product's own access token, sent to the product server's Push.
    fn product_token(&self) -> Result<String>;
    /// A sync token (ES256 JWT, see `docs/protocol.md`) for the store
    /// service's Pull and Pokes, usually issued by the product server.
    fn sync_token(&self) -> Result<String>;
}

/// Fixed tokens, for tests and short-lived sessions.
#[derive(Debug, Clone)]
pub struct StaticCredentials {
    /// Sent to Push.
    pub product_token: String,
    /// Sent to Pull and Pokes.
    pub sync_token: String,
}

impl Credentials for StaticCredentials {
    fn product_token(&self) -> Result<String> {
        Ok(self.product_token.clone())
    }

    fn sync_token(&self) -> Result<String> {
        Ok(self.sync_token.clone())
    }
}
