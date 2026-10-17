//! The protobuf byte wrappers used by bindings.

use store_core::{
    apply_pull_response_bytes, apply_push_response_bytes,
    protocol::{
        Message,
        v1::{PullRequest, PushRequest},
    },
    pull_request_bytes, push_request_bytes,
    serde_json::json,
};
use store_testkit::{Client, FakeServer, PlannerMutator};

#[test]
fn a_full_sync_runs_over_bytes() {
    let mut client = Client::new();
    let mut server = FakeServer::new();
    assert_eq!(push_request_bytes(&client.conn, 10).unwrap(), None);
    client
        .mutate("project.create", json!({"name": "Bytes"}))
        .unwrap();

    let push = push_request_bytes(&client.conn, 10).unwrap().unwrap();
    let response = server.push(&PushRequest::decode(push.as_slice()).unwrap());
    let pushed = apply_push_response_bytes(&client.conn, &response.encode_to_vec()).unwrap();
    assert_eq!(pushed.applied, 1);

    let pull = pull_request_bytes(&client.conn, 10).unwrap();
    let response = server.pull(&PullRequest::decode(pull.as_slice()).unwrap());
    let pulled =
        apply_pull_response_bytes(&mut client.conn, &PlannerMutator, &response.encode_to_vec())
            .unwrap();
    assert_eq!(pulled.versions, 1);
    assert!(client.pending().is_empty());
    assert_eq!(client.state(), server.state());
}

#[test]
fn malformed_bytes_are_decode_errors() {
    let mut client = Client::new();
    assert!(matches!(
        apply_pull_response_bytes(&mut client.conn, &PlannerMutator, &[0xff, 0xff]),
        Err(store_core::Error::Decode(_))
    ));
    assert!(matches!(
        apply_push_response_bytes(&client.conn, &[0x0a]),
        Err(store_core::Error::Decode(_))
    ));
}
