package poke

import "time"

type backoff struct {
	first, max, current time.Duration
}

func newBackoff(first, max time.Duration) *backoff {
	return &backoff{first: first, max: max}
}

func (b *backoff) next() time.Duration {
	if b.current == 0 {
		b.current = b.first
	} else {
		b.current = min(b.current*2, b.max)
	}
	return b.current
}

func (b *backoff) reset() {
	b.current = 0
}
