package sim

import (
	"math/rand/v2"
	"strings"

	"github.com/google/uuid"

	"github.com/skkil/store/modules/conformance/writepath"
)

var words = []string{
	"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
	"india", "juliet", "kilo", "lima", "mike", "november", "oscar", "papa",
	"quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey", "yankee",
	"zulu", "한글", "naïve", "emoji 🌱", `quote "q"`, "back\\slash",
}

type renderEnv struct {
	rng        *rand.Rand
	client     uuid.UUID
	mutationID uint64
	replica    *Replica
}

func (e *renderEnv) NewID(n uint32) string {
	return writepath.DeriveID(e.client, e.mutationID, n).String()
}

func (e *renderEnv) Row(table string) string {
	ids := e.replica.IDs(table)
	if len(ids) == 0 {
		return ""
	}
	return ids[e.rng.IntN(len(ids))]
}

func (e *renderEnv) Text() string {
	n := 1 + e.rng.IntN(3)
	parts := make([]string, n)
	for i := range parts {
		parts[i] = words[e.rng.IntN(len(words))]
	}
	return strings.Join(parts, " ")
}

func (e *renderEnv) Int() int64 { return e.rng.Int64N(1_000_000) }

func (e *renderEnv) Bool() bool { return e.rng.IntN(2) == 0 }

func (e *renderEnv) ClientID() string { return e.client.String() }

func (e *renderEnv) MutationID() uint64 { return e.mutationID }
