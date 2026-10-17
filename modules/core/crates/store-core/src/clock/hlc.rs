/// A hybrid logical clock value, encoded as `(unix_millis << 16) | counter`.
///
/// The millisecond part follows the wall clock but never goes backwards; the
/// counter orders events within one millisecond. The encoding is the one sent
/// in `Mutation.hlc` (see `docs/protocol.md`, "Clock").
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct Hlc(i64);

impl Hlc {
    /// Bits used by the counter.
    pub const COUNTER_BITS: u32 = 16;
    /// The largest counter value.
    pub const MAX_COUNTER: u16 = u16::MAX;

    /// Builds a value from its wall time and counter.
    pub const fn new(millis: i64, counter: u16) -> Self {
        Self((millis << Self::COUNTER_BITS) | counter as i64)
    }

    /// Reads a value in its wire encoding.
    pub const fn from_i64(value: i64) -> Self {
        Self(value)
    }

    /// The wire encoding.
    pub const fn as_i64(self) -> i64 {
        self.0
    }

    /// The wall time in Unix milliseconds.
    pub const fn millis(self) -> i64 {
        self.0 >> Self::COUNTER_BITS
    }

    /// The counter within the millisecond.
    pub const fn counter(self) -> u16 {
        (self.0 & Self::MAX_COUNTER as i64) as u16
    }

    /// The clock for a local event at wall time `now_millis`.
    pub fn tick(self, now_millis: i64) -> Self {
        if now_millis > self.millis() {
            Self::new(now_millis, 0)
        } else {
            self.increment(self.counter())
        }
    }

    /// The clock after receiving `remote` at wall time `now_millis`.
    pub fn observe(self, remote: Hlc, now_millis: i64) -> Self {
        let millis = now_millis.max(self.millis()).max(remote.millis());
        let local = millis == self.millis();
        let from_remote = millis == remote.millis();
        match (local, from_remote) {
            (true, true) => Self::new(millis, 0).increment(self.counter().max(remote.counter())),
            (true, false) => self.increment(self.counter()),
            (false, true) => remote.increment(remote.counter()),
            (false, false) => Self::new(millis, 0),
        }
    }

    fn increment(self, counter: u16) -> Self {
        match counter.checked_add(1) {
            Some(next) => Self::new(self.millis(), next),
            None => Self::new(self.millis() + 1, 0),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::Hlc;

    #[test]
    fn encodes_millis_and_counter() {
        let hlc = Hlc::new(1_700_000_000_123, 7);
        assert_eq!(hlc.as_i64(), (1_700_000_000_123 << 16) | 7);
        assert_eq!(hlc.millis(), 1_700_000_000_123);
        assert_eq!(hlc.counter(), 7);
        assert_eq!(Hlc::from_i64(hlc.as_i64()), hlc);
    }

    #[test]
    fn tick_follows_a_forward_clock() {
        assert_eq!(Hlc::new(100, 5).tick(200), Hlc::new(200, 0));
    }

    #[test]
    fn tick_counts_within_a_millisecond_and_when_the_clock_goes_back() {
        assert_eq!(Hlc::new(100, 5).tick(100), Hlc::new(100, 6));
        assert_eq!(Hlc::new(100, 5).tick(50), Hlc::new(100, 6));
    }

    #[test]
    fn counter_overflow_carries_into_the_millis() {
        assert_eq!(Hlc::new(100, u16::MAX).tick(100), Hlc::new(101, 0));
    }

    #[test]
    fn observe_takes_the_largest_clock() {
        assert_eq!(
            Hlc::new(100, 1).observe(Hlc::new(300, 4), 200),
            Hlc::new(300, 5)
        );
        assert_eq!(
            Hlc::new(300, 4).observe(Hlc::new(100, 9), 200),
            Hlc::new(300, 5)
        );
        assert_eq!(
            Hlc::new(100, 4).observe(Hlc::new(100, 9), 50),
            Hlc::new(100, 10)
        );
        assert_eq!(
            Hlc::new(100, 4).observe(Hlc::new(90, 9), 500),
            Hlc::new(500, 0)
        );
    }

    #[test]
    fn values_order_like_events() {
        let first = Hlc::default().tick(100);
        let second = first.tick(100);
        let third = second.tick(90);
        assert!(first < second && second < third);
    }
}
