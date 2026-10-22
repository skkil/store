package sim

import (
	"testing"
	"time"
)

func TestHybridClockGrowsAndEncodesWallTime(t *testing.T) {
	var c hybridClock
	before := time.Now().UnixMilli()
	prev := c.next()
	for range 1000 {
		next := c.next()
		if next <= prev {
			t.Fatalf("clock went from %d to %d", prev, next)
		}
		prev = next
	}
	if wall := prev >> hlcCounterBits; wall < before || wall > time.Now().UnixMilli() {
		t.Fatalf("wall time %d outside [%d, now]", wall, before)
	}
}
