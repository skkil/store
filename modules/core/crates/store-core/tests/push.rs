//! Building push requests and recording push responses.

use store_core::{
    apply_push_response,
    protocol::{
        PROTOCOL_VERSION,
        v1::{MutationResult, PushResponse, mutation_result::Status},
    },
    pull_request, push_request,
    serde_json::json,
    set_product_schema_version,
};
use store_testkit::{Client, FakeServer};

fn client_with_projects(count: usize) -> Client {
    let mut client = Client::new();
    for index in 0..count {
        client
            .mutate("project.create", json!({"name": format!("P{index}")}))
            .unwrap();
    }
    client
}

#[test]
fn nothing_to_push_gives_none() {
    let client = Client::new();
    assert_eq!(push_request(&client.conn, 10).unwrap(), None);
}

#[test]
fn push_request_lists_pending_mutations_in_order_up_to_the_limit() {
    let client = client_with_projects(3);
    set_product_schema_version(&client.conn, 7).unwrap();
    let request = push_request(&client.conn, 2).unwrap().unwrap();
    assert_eq!(request.protocol_version, PROTOCOL_VERSION);
    assert_eq!(request.protocol_version, 1);
    assert_eq!(request.client_id, client.client_id().to_string());
    assert_eq!(request.schema_version, 7);
    let ids: Vec<u64> = request.mutations.iter().map(|m| m.mutation_id).collect();
    assert_eq!(ids, vec![1, 2]);
    assert_eq!(request.mutations[0].name, "project.create");
    assert_eq!(request.mutations[0].args, br#"{"name":"P0"}"#);
    assert_eq!(request.mutations[0].hlc, client.pending()[0].3);
}

#[test]
fn answered_mutations_stay_pending_but_are_not_pushed_again() {
    let mut client = client_with_projects(2);
    let mut server = FakeServer::new();
    let outcome = client.push(&mut server, 10);
    assert_eq!(outcome.applied, 2);
    assert_eq!(client.pending().len(), 2);
    assert_eq!(push_request(&client.conn, 10).unwrap(), None);
    client
        .mutate("project.create", json!({"name": "P2"}))
        .unwrap();
    let request = push_request(&client.conn, 10).unwrap().unwrap();
    assert_eq!(request.mutations.len(), 1);
    assert_eq!(request.mutations[0].mutation_id, 3);
}

#[test]
fn a_lost_response_re_sends_the_batch_and_the_server_deduplicates() {
    let mut client = client_with_projects(2);
    let mut server = FakeServer::new();
    let request = push_request(&client.conn, 10).unwrap().unwrap();
    server.push(&request);
    let outcome = client.push(&mut server, 10);
    assert_eq!(outcome.duplicate, 2);
    assert_eq!(outcome.applied, 0);
    assert_eq!(server.head(), 2);
    client.pull(&server, 100);
    assert!(client.pending().is_empty());
    assert_eq!(client.state(), server.state());
}

#[test]
fn the_highest_recorded_version_becomes_the_pull_min_version() {
    let client = client_with_projects(2);
    let response = PushResponse {
        results: vec![
            MutationResult {
                mutation_id: 1,
                status: Status::Applied as i32,
                reason: String::new(),
                version: 41,
            },
            MutationResult {
                mutation_id: 2,
                status: Status::Rejected as i32,
                reason: "no".into(),
                version: 42,
            },
        ],
    };
    let outcome = apply_push_response(&client.conn, &response).unwrap();
    assert_eq!(
        (outcome.applied, outcome.duplicate, outcome.rejected),
        (1, 0, 1)
    );
    assert_eq!(pull_request(&client.conn, 10).unwrap().min_version, 42);
}

#[test]
fn results_without_a_status_are_ignored() {
    let client = client_with_projects(1);
    let response = PushResponse {
        results: vec![MutationResult {
            mutation_id: 1,
            status: Status::Unspecified as i32,
            reason: String::new(),
            version: 9,
        }],
    };
    apply_push_response(&client.conn, &response).unwrap();
    assert!(push_request(&client.conn, 10).unwrap().is_some());
    assert_eq!(pull_request(&client.conn, 10).unwrap().min_version, 0);
}
