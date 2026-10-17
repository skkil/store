//! Single Connect calls: paths, headers, framing and error mapping.

#![cfg(not(target_arch = "wasm32"))]

mod support;

use store_core::protocol::{
    Message,
    v1::{
        Entries, MutationResult, Poke, PokesRequest, PullRequest, PullResponse, PushRequest,
        PushResponse, mutation_result::Status, pull_response::Body,
    },
};
use store_http::{Credentials, Endpoints, Error, HttpTransport, StaticCredentials};
use support::{Reply, TestServer};

fn credentials() -> StaticCredentials {
    StaticCredentials {
        product_token: "product".into(),
        sync_token: "sync".into(),
    }
}

fn transport(server: &TestServer) -> HttpTransport<StaticCredentials> {
    HttpTransport::new(
        Endpoints::new(&server.url, format!("{}/store", server.url)),
        credentials(),
    )
}

fn framed(flags: u8, data: &[u8]) -> Vec<u8> {
    [
        vec![flags],
        (data.len() as u32).to_be_bytes().to_vec(),
        data.to_vec(),
    ]
    .concat()
}

#[test]
fn push_posts_binary_proto_with_the_product_token() {
    let response = PushResponse {
        results: vec![MutationResult {
            mutation_id: 1,
            status: Status::Applied as i32,
            reason: String::new(),
            version: 3,
        }],
    };
    let body = response.encode_to_vec();
    let server = TestServer::start(move |_| Reply::proto(body.clone()));
    let request = PushRequest {
        protocol_version: 1,
        client_id: "c".into(),
        mutations: vec![],
        schema_version: 2,
    };
    assert_eq!(transport(&server).push(&request).unwrap(), response);
    let seen = &server.recorded()[0];
    assert_eq!(seen.path, "/skkil.store.v1.PushService/Push");
    assert_eq!(seen.header("Content-Type"), Some("application/proto"));
    assert_eq!(seen.header("Connect-Protocol-Version"), Some("1"));
    assert_eq!(seen.header("Authorization"), Some("Bearer product"));
    assert_eq!(PushRequest::decode(seen.body.as_slice()).unwrap(), request);
}

#[test]
fn pull_goes_to_the_store_service_with_the_sync_token() {
    let response = PullResponse {
        body: Some(Body::Entries(Entries::default())),
        last_mutation_id: 4,
        head_version: 9,
        epoch: 1,
    };
    let body = response.encode_to_vec();
    let server = TestServer::start(move |_| Reply::proto(body.clone()));
    let pulled = transport(&server).pull(&PullRequest::default()).unwrap();
    assert_eq!(pulled, response);
    let seen = &server.recorded()[0];
    assert_eq!(seen.path, "/store/skkil.store.v1.StoreService/Pull");
    assert_eq!(seen.header("Authorization"), Some("Bearer sync"));
}

#[test]
fn connect_errors_become_rpc_errors() {
    let server = TestServer::start(|_| {
        Reply::json(
            401,
            r#"{"code":"unauthenticated","message":"token expired"}"#,
        )
    });
    match transport(&server).pull(&PullRequest::default()) {
        Err(error @ Error::Rpc { .. }) => {
            assert!(!error.is_retryable());
            assert_eq!(error.to_string(), "unauthenticated: token expired");
        }
        other => panic!("expected an rpc error, got {other:?}"),
    }
}

#[test]
fn gateway_failures_are_retryable() {
    let server = TestServer::start(|_| Reply {
        status: 503,
        content_type: "text/plain",
        body: b"down".to_vec(),
    });
    let error = transport(&server)
        .push(&PushRequest::default())
        .unwrap_err();
    assert!(matches!(&error, Error::Rpc { code, .. } if code == "unavailable"));
    assert!(error.is_retryable());
}

#[test]
fn unreachable_servers_are_retryable_transport_errors() {
    let transport = HttpTransport::new(
        Endpoints::new("http://127.0.0.1:9", "http://127.0.0.1:9"),
        credentials(),
    );
    let error = transport.pull(&PullRequest::default()).unwrap_err();
    assert!(matches!(error, Error::Transport(_)));
    assert!(error.is_retryable());
}

#[test]
fn pokes_stream_until_the_server_ends_it() {
    let stream = [
        framed(0, &Poke { version: 3 }.encode_to_vec()),
        framed(0, &Poke { version: 5 }.encode_to_vec()),
        framed(2, b"{}"),
    ]
    .concat();
    let server = TestServer::start(move |_| Reply::stream(stream.clone()));
    let request = PokesRequest {
        protocol_version: 1,
        client_id: "client".into(),
    };
    let pokes: Vec<u64> = transport(&server)
        .pokes(&request)
        .unwrap()
        .map(|poke| poke.unwrap().version)
        .collect();
    assert_eq!(pokes, vec![3, 5]);
    let seen = &server.recorded()[0];
    assert_eq!(seen.path, "/store/skkil.store.v1.StoreService/Pokes");
    assert_eq!(
        seen.header("Content-Type"),
        Some("application/connect+proto")
    );
    assert_eq!(seen.header("Authorization"), Some("Bearer sync"));
    assert_eq!(seen.body[0], 0);
    assert_eq!(PokesRequest::decode(&seen.body[5..]).unwrap(), request);
}

#[test]
fn a_stream_ending_in_an_error_yields_it() {
    let stream = [
        framed(0, &Poke { version: 1 }.encode_to_vec()),
        framed(
            2,
            br#"{"error":{"code":"unauthenticated","message":"expired"}}"#,
        ),
    ]
    .concat();
    let server = TestServer::start(move |_| Reply::stream(stream.clone()));
    let mut pokes = transport(&server).pokes(&PokesRequest::default()).unwrap();
    assert_eq!(pokes.next().unwrap().unwrap().version, 1);
    assert!(
        matches!(pokes.next(), Some(Err(Error::Rpc { code, .. })) if code == "unauthenticated")
    );
    assert!(pokes.next().is_none());
}

#[test]
fn a_refused_stream_is_an_error() {
    let server =
        TestServer::start(|_| Reply::json(403, r#"{"code":"permission_denied","message":"no"}"#));
    assert!(matches!(
        transport(&server).pokes(&PokesRequest::default()),
        Err(Error::Rpc { code, .. }) if code == "permission_denied"
    ));
}

struct NoToken;

impl Credentials for NoToken {
    fn product_token(&self) -> store_http::Result<String> {
        Err(Error::Credentials("signed out".into()))
    }

    fn sync_token(&self) -> store_http::Result<String> {
        Err(Error::Credentials("signed out".into()))
    }
}

#[test]
fn credential_failures_stop_before_the_request() {
    let server = TestServer::start(|_| Reply::proto(Vec::new()));
    let transport = HttpTransport::new(Endpoints::new(&server.url, &server.url), NoToken);
    assert!(matches!(
        transport.push(&PushRequest::default()),
        Err(Error::Credentials(_))
    ));
    assert!(server.recorded().is_empty());
}
