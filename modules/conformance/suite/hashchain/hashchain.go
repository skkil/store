// Package hashchain recomputes the store log's hash chain from pulled
// versions, exactly as store_close in modules/sql computes it.
package hashchain

import (
	"crypto/sha256"
	"strconv"
	"strings"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
)

// Next returns the chain hash after version v, given prev, the hash at the
// version before it (empty at version 0):
//
//	sha256(prev ‖ "version|client|mutation|kind" ‖ body)
//
// where body is sha256 of the rejection reason for a rejection, and
// otherwise sha256 of the changes joined by "\n", each
// "seq|table|row_id|op|data" with seq counting from 1 and data empty for a
// delete.
func Next(prev []byte, v *storev1.Version) []byte {
	kind := "commit"
	var body [sha256.Size]byte
	if v.GetRejection() != nil {
		kind = "rejection"
		body = sha256.Sum256([]byte(v.GetRejection().GetReason()))
	} else {
		body = sha256.Sum256([]byte(changes(v.GetChanges())))
	}
	header := strconv.FormatUint(v.GetVersion(), 10) + "|" + v.GetClientId() + "|" +
		strconv.FormatUint(v.GetMutationId(), 10) + "|" + kind
	h := sha256.New()
	h.Write(prev)
	h.Write([]byte(header))
	h.Write(body[:])
	return h.Sum(nil)
}

func changes(list []*storev1.Change) string {
	lines := make([]string, len(list))
	for i, c := range list {
		op := "upsert"
		if c.GetOp() == storev1.Change_OP_DELETE {
			op = "delete"
		}
		lines[i] = strconv.Itoa(i+1) + "|" + c.GetTable() + "|" + c.GetRowId() + "|" + op + "|" + string(c.GetData())
	}
	return strings.Join(lines, "\n")
}
