package writepath

// Status is the outcome of one device mutation.
type Status int

// The outcomes a product server reports for a mutation.
const (
	Applied Status = iota + 1
	Duplicate
	Rejected
)

func (s Status) String() string {
	switch s {
	case Applied:
		return "applied"
	case Duplicate:
		return "duplicate"
	case Rejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// Result is what the write path recorded for one device mutation.
type Result struct {
	Status Status
	// Version is the scope version that recorded the mutation. For a
	// duplicate it is the version recorded the first time.
	Version uint64
	// Reason explains a rejection to the user.
	Reason string
}
