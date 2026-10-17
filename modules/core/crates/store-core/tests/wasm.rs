//! Runs the core on `wasm32-unknown-unknown` with SQLite from `sqlite-wasm-rs`:
//! `cargo test --target wasm32-unknown-unknown -p skkil-store-core --test wasm`
//! (needs `wasm-bindgen-test-runner` and Node).

#![cfg(target_arch = "wasm32")]

use sqlite_wasm_rs as _;
use store_core::serde_json::json;
use store_testkit::{Client, FakeServer};
use wasm_bindgen_test::wasm_bindgen_test;

#[wasm_bindgen_test]
fn mutations_capture_rebase_and_converge_in_wasm() {
    let mut server = FakeServer::new();
    let mut alice = Client::new();
    let mut bob = Client::new();
    let project = alice
        .mutate("project.create", json!({"name": "Web"}))
        .unwrap();
    assert!(project.hlc.millis() > 1_600_000_000_000);
    alice.sync(&mut server);
    bob.pull(&server, 100);
    alice
        .mutate(
            "task.create",
            json!({"project_id": project.value["id"], "title": "A", "attachment_hex": "beef"}),
        )
        .unwrap();
    bob.mutate(
        "task.create",
        json!({"project_id": project.value["id"], "title": "B"}),
    )
    .unwrap();
    bob.sync(&mut server);
    alice.pull(&server, 100);
    assert_eq!(alice.state(), alice.expected(&server));
    alice.sync(&mut server);
    bob.pull(&server, 100);
    assert_eq!(alice.state(), server.state());
    assert_eq!(bob.state(), server.state());
}
