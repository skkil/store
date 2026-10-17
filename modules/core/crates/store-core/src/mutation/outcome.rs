use serde_json::Value;

use crate::clock::Hlc;

/// A mutation that was applied locally and queued for push.
#[derive(Debug, Clone, PartialEq)]
pub struct MutationOutcome {
    /// The id it was queued under.
    pub mutation_id: u64,
    /// Its clock; [`Hlc::millis`] is the time the mutator saw.
    pub hlc: Hlc,
    /// What the mutator returned.
    pub value: Value,
}
