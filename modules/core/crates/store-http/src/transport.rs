use std::time::Duration;

use prost::Message;
use store_core::protocol::v1::{
    PokesRequest, PullRequest, PullResponse, PushRequest, PushResponse,
};
use ureq::Agent;

use crate::{
    Credentials, Endpoints, Error, PokeStream, Result,
    connect::{
        PROTOCOL_VERSION_HEADER, STREAM_CONTENT_TYPE, UNARY_CONTENT_TYPE, envelope,
        error_from_response,
    },
};

const DEFAULT_TIMEOUT: Duration = Duration::from_secs(30);
const MAX_RESPONSE_BYTES: u64 = 64 * 1024 * 1024;

/// Single Connect calls to the product server and the store service.
pub struct HttpTransport<C> {
    agent: Agent,
    endpoints: Endpoints,
    credentials: C,
}

impl<C: Credentials> HttpTransport<C> {
    /// A transport with a 30 second timeout per unary call.
    pub fn new(endpoints: Endpoints, credentials: C) -> Self {
        Self::with_timeout(endpoints, credentials, DEFAULT_TIMEOUT)
    }

    /// A transport whose unary calls time out after `timeout`. The poke
    /// stream has no overall timeout.
    pub fn with_timeout(endpoints: Endpoints, credentials: C, timeout: Duration) -> Self {
        let agent = Agent::config_builder()
            .timeout_global(Some(timeout))
            .http_status_as_error(false)
            .build()
            .into();
        Self {
            agent,
            endpoints,
            credentials,
        }
    }

    /// Calls `PushService/Push` on the product server with the product token.
    pub fn push(&self, request: &PushRequest) -> Result<PushResponse> {
        let token = self.credentials.product_token()?;
        self.unary(&self.endpoints.push(), &token, request)
    }

    /// Calls `StoreService/Pull` on the store service with a sync token.
    pub fn pull(&self, request: &PullRequest) -> Result<PullResponse> {
        let token = self.credentials.sync_token()?;
        self.unary(&self.endpoints.pull(), &token, request)
    }

    /// Opens `StoreService/Pokes` on the store service with a sync token.
    pub fn pokes(&self, request: &PokesRequest) -> Result<PokeStream> {
        let token = self.credentials.sync_token()?;
        let mut response = self
            .agent
            .post(self.endpoints.pokes())
            .config()
            .timeout_global(None)
            .build()
            .header("Content-Type", STREAM_CONTENT_TYPE)
            .header(PROTOCOL_VERSION_HEADER, "1")
            .header("Authorization", format!("Bearer {token}"))
            .send(&envelope(&request.encode_to_vec())[..])?;
        let status = response.status().as_u16();
        if status != 200 {
            let body = response.body_mut().read_to_vec()?;
            return Err(error_from_response(status, &body));
        }
        Ok(PokeStream::new(response.into_body().into_reader()))
    }

    fn unary<Req: Message, Resp: Message + Default>(
        &self,
        url: &str,
        token: &str,
        request: &Req,
    ) -> Result<Resp> {
        let mut response = self
            .agent
            .post(url)
            .header("Content-Type", UNARY_CONTENT_TYPE)
            .header(PROTOCOL_VERSION_HEADER, "1")
            .header("Authorization", format!("Bearer {token}"))
            .send(&request.encode_to_vec()[..])?;
        let status = response.status().as_u16();
        let body = response
            .body_mut()
            .with_config()
            .limit(MAX_RESPONSE_BYTES)
            .read_to_vec()?;
        if !(200..300).contains(&status) {
            return Err(error_from_response(status, &body));
        }
        Resp::decode(body.as_slice()).map_err(Error::from)
    }
}
