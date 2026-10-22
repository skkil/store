package scenario

import "github.com/skkil/store/modules/conformance/suite/profile"

func offline(b *builder) {
	for !b.full() {
		away := b.client()
		b.add(Step{Op: Offline, Client: away})
		b.mutations(away, 1+b.rng.IntN(4))
		b.add(Step{Op: Push, Client: away})
		for range 1 + b.rng.IntN(3) {
			c := b.client()
			if c == away {
				continue
			}
			b.mutations(c, 1+b.rng.IntN(2))
			b.add(Step{Op: Push, Client: c}, Step{Op: PullAll, Client: c})
		}
		b.add(Step{Op: Online, Client: away}, Step{Op: Push, Client: away}, Step{Op: PullAll, Client: away})
	}
}

func concurrent(b *builder) {
	for !b.full() {
		for _, c := range b.everyone() {
			b.mutations(c, 1+b.rng.IntN(3))
		}
		b.concurrently(Push, b.everyone())
		b.concurrently(PullAll, b.everyone())
	}
}

func duplicates(b *builder) {
	for !b.full() {
		c := b.client()
		b.mutations(c, 1+b.rng.IntN(3))
		b.add(Step{Op: PushLost, Client: c})
		if b.rng.IntN(2) == 0 {
			b.mutations(c, 1)
		}
		if b.rng.IntN(3) == 0 {
			b.add(Step{Op: PushLost, Client: c})
		}
		b.add(Step{Op: Push, Client: c})
		if b.rng.IntN(2) == 0 {
			b.add(Step{Op: Push, Client: c})
		}
		b.add(Step{Op: PullAll, Client: b.client()})
		if b.rng.IntN(3) == 0 {
			b.concurrently(PushLost, b.everyone())
			b.concurrently(Push, b.everyone())
		}
	}
}

func rejections(b *builder) {
	for !b.full() {
		c := b.client()
		if b.rng.IntN(2) == 0 {
			b.add(b.mutateKind(c, profile.Reject))
		}
		b.mutations(c, 1+b.rng.IntN(2))
		b.add(b.mutateKind(c, profile.Reject))
		b.add(Step{Op: Push, Client: c}, Step{Op: PullAll, Client: b.client()})
	}
}

func pokeLoss(b *builder) {
	for i := range b.s.Listening {
		b.s.Listening[i] = i%2 == 0
	}
	for !b.full() {
		writer := b.client()
		b.mutations(writer, 1+b.rng.IntN(3))
		b.add(Step{Op: Push, Client: writer}, Step{Op: Settle})
		for _, c := range b.everyone() {
			if c != writer {
				b.add(Step{Op: React, Client: c})
			}
		}
		if b.rng.IntN(3) == 0 {
			b.add(Step{Op: PullAll, Client: b.client()})
		}
	}
}

func smallPulls(b *builder) {
	for !b.full() {
		writer := b.client()
		b.mutations(writer, 1+b.rng.IntN(4))
		b.add(Step{Op: Push, Client: writer})
		for range 1 + b.rng.IntN(4) {
			b.add(Step{Op: Pull, Client: b.client(), Limit: b.smallLimit()})
		}
		if b.rng.IntN(2) == 0 {
			b.add(b.mutate(b.client()))
		}
	}
}

func reconnect(b *builder) {
	for !b.full() {
		c := b.client()
		b.mutations(c, 1+b.rng.IntN(2))
		b.add(Step{Op: Push, Client: c}, Step{Op: PullAll, Client: c})
		switch b.rng.IntN(4) {
		case 0:
			b.add(Step{Op: StaleHash, Client: c}, Step{Op: PullAll, Client: c})
		case 1:
			b.add(Step{Op: StaleAhead, Client: c}, Step{Op: PullAll, Client: c})
		case 2:
			if b.opts.EpochBumps {
				b.add(Step{Op: BumpEpoch})
				for _, other := range b.everyone() {
					b.add(Step{Op: PullAll, Client: other})
				}
				continue
			}
			fallthrough
		default:
			b.add(Step{Op: Offline, Client: c})
			b.mutations(c, 1+b.rng.IntN(2))
			b.add(Step{Op: Online, Client: c}, Step{Op: Push, Client: c})
		}
	}
}

func random(b *builder) {
	for i := range b.s.Listening {
		b.s.Listening[i] = b.rng.IntN(3) != 0
	}
	ops := []Op{Mutate, Mutate, Mutate, Mutate, Push, Push, PushLost, Pull, PullAll, Offline, Online, React, Concurrent, StaleHash, StaleAhead}
	if b.opts.EpochBumps {
		ops = append(ops, BumpEpoch)
	}
	for !b.full() {
		op := ops[b.rng.IntN(len(ops))]
		c := b.client()
		switch op {
		case Mutate:
			b.add(b.mutate(c))
		case Pull:
			b.add(Step{Op: Pull, Client: c, Limit: b.smallLimit()})
		case Concurrent:
			b.concurrently([]Op{Push, PushLost, PullAll}[b.rng.IntN(3)], b.everyone())
		case BumpEpoch:
			b.add(Step{Op: BumpEpoch})
		default:
			b.add(Step{Op: op, Client: c})
		}
	}
}
