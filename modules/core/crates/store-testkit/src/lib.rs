//! Test support for skkil store clients: a small planner app (schema and
//! [`PlannerMutator`]), an in-memory [`FakeServer`] that applies pushed
//! mutations with the same mutator and serves pulls, and a [`Client`] that
//! drives `store-core` against it.

mod app;
mod client;
mod mutator;
mod server;
mod state;

pub use app::{CLIENT_SCHEMA, SERVER_SCHEMA, SYNCED_TABLES};
pub use client::Client;
pub use mutator::PlannerMutator;
pub use server::FakeServer;
pub use state::{AppState, expected_state};
