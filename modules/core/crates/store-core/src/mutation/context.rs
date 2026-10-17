use uuid::Uuid;

use crate::{clock::Hlc, mutation::derive_id};

/// What a mutator knows about the mutation it is applying.
///
/// The values are fixed when the mutation is made and replayed unchanged, so
/// ids and timestamps a mutation writes are the same on every run and on the
/// server.
#[derive(Debug, Clone)]
pub struct MutationContext {
    client_id: Uuid,
    mutation_id: u64,
    hlc: Hlc,
    replay: bool,
    created_ids: u32,
}

impl MutationContext {
    /// A context for the first run of a mutation. Servers that replay pushed
    /// mutations in Rust build it from the pushed `client_id`,
    /// `mutation_id` and `hlc`.
    pub fn new(client_id: Uuid, mutation_id: u64, hlc: Hlc) -> Self {
        Self {
            client_id,
            mutation_id,
            hlc,
            replay: false,
            created_ids: 0,
        }
    }

    pub(crate) fn replaying(mut self) -> Self {
        self.replay = true;
        self
    }

    /// The client that made the mutation.
    pub fn client_id(&self) -> Uuid {
        self.client_id
    }

    /// The mutation's id, unique per client.
    pub fn mutation_id(&self) -> u64 {
        self.mutation_id
    }

    /// The hybrid logical clock taken when the mutation was made.
    pub fn hlc(&self) -> Hlc {
        self.hlc
    }

    /// The mutation's wall time in Unix milliseconds. Use it instead of the
    /// system clock: a mutation made offline at 10:00 and replayed at 12:00
    /// still records 10:00.
    pub fn now(&self) -> i64 {
        self.hlc.millis()
    }

    /// True when the mutation is being re-applied after a pull rather than
    /// run for the first time, so one-off side effects can be skipped.
    pub fn is_replay(&self) -> bool {
        self.replay
    }

    /// The next id for a row this mutation creates: [`derive_id`] with a
    /// counter starting at 0.
    pub fn next_id(&mut self) -> Uuid {
        let id = derive_id(self.client_id, self.mutation_id, self.created_ids);
        self.created_ids += 1;
        id
    }
}

#[cfg(test)]
mod tests {
    use uuid::Uuid;

    use super::MutationContext;
    use crate::{clock::Hlc, mutation::derive_id};

    #[test]
    fn next_id_counts_from_zero() {
        let client = Uuid::from_u128(7);
        let mut ctx = MutationContext::new(client, 3, Hlc::new(1_000, 0));
        assert_eq!(ctx.next_id(), derive_id(client, 3, 0));
        assert_eq!(ctx.next_id(), derive_id(client, 3, 1));
    }

    #[test]
    fn now_is_the_wall_time_of_the_clock() {
        let ctx = MutationContext::new(Uuid::nil(), 1, Hlc::new(1_234, 9));
        assert_eq!(ctx.now(), 1_234);
        assert!(!ctx.is_replay());
        assert!(ctx.replaying().is_replay());
    }
}
