// Package pull serves the committed store_log of one scope by cursor.
package pull

import "bytes"

// Cursor is how far a client has applied the log.
type Cursor struct {
	Epoch   uint32
	Version uint64
	Hash    []byte
}

// Head is the newest committed state of a scope.
type Head struct {
	Epoch   uint32
	Version uint64
	Hash    []byte
}

// The reasons a cursor no longer matches the log.
const (
	ReasonEpochChanged = "epoch changed"
	ReasonHashMismatch = "hash mismatch"
	ReasonAheadOfHead  = "cursor ahead of head"
)

func (c Cursor) isFresh() bool {
	return c.Epoch == 0 && c.Version == 0 && len(c.Hash) == 0
}

// resetReason says why cursor must be reset against head, or "" when it
// still matches. logHash is the log's hash at cursor.Version (nil when absent).
func resetReason(head Head, cursor Cursor, logHash []byte) string {
	switch {
	case cursor.isFresh():
		return ""
	case cursor.Epoch != head.Epoch:
		return ReasonEpochChanged
	case cursor.Version > head.Version:
		return ReasonAheadOfHead
	case cursor.Version == 0 && len(cursor.Hash) != 0:
		return ReasonHashMismatch
	case cursor.Version > 0 && (logHash == nil || !bytes.Equal(cursor.Hash, logHash)):
		return ReasonHashMismatch
	default:
		return ""
	}
}
