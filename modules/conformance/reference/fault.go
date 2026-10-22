package reference

import "fmt"

// Fault deliberately breaks the reference product so tests can prove the
// conformance suite catches a broken server. Production code always runs
// FaultNone.
type Fault string

// The faults the reference product can run with.
const (
	// FaultNone follows docs/protocol.md exactly.
	FaultNone Fault = "none"
	// FaultIgnoreDuplicates runs a mutation again, as a server write, when
	// store_begin reports it was already processed: idempotency disabled.
	FaultIgnoreDuplicates Fault = "ignore-duplicates"
	// FaultSequenceVersions numbers versions from one sequence shared by every
	// scope and drawn before the scope lock, like a bigserial sequencer:
	// duplicates burn numbers and concurrent pushes commit out of order.
	FaultSequenceVersions Fault = "sequence-versions"
	// FaultDropRejections rolls back a rejected mutation entirely and reports
	// REJECTED without recording it in the log.
	FaultDropRejections Fault = "drop-rejections"
)

// Faults lists every fault, FaultNone first.
func Faults() []Fault {
	return []Fault{FaultNone, FaultIgnoreDuplicates, FaultSequenceVersions, FaultDropRejections}
}

// ParseFault returns the Fault named name.
func ParseFault(name string) (Fault, error) {
	for _, f := range Faults() {
		if string(f) == name {
			return f, nil
		}
	}
	return "", fmt.Errorf("unknown fault %q", name)
}
