package writepath

import "errors"

// Rejection is the error a mutation body returns when a business rule
// forbids the mutation. The write path rolls back the body's row changes and
// records a durable rejection that every device pulls.
type Rejection struct {
	Reason string
}

func (r *Rejection) Error() string { return "rejected: " + r.Reason }

// Reject returns a Rejection with reason.
func Reject(reason string) error {
	return &Rejection{Reason: reason}
}

// AsRejection reports whether err is a Rejection and returns it.
func AsRejection(err error) (*Rejection, bool) {
	var rejection *Rejection
	ok := errors.As(err, &rejection)
	return rejection, ok
}
