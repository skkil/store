package sim

import "time"

const hlcCounterBits = 16

type hybridClock struct {
	last int64
}

func (c *hybridClock) next() int64 {
	now := time.Now().UnixMilli() << hlcCounterBits
	if now > c.last {
		c.last = now
	} else {
		c.last++
	}
	return c.last
}
