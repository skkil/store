package pull

import (
	"encoding/json"
	"fmt"
	"time"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
)

type entry struct {
	version     int64
	seq         int32
	kind        string
	table       *string
	rowID       *string
	op          *string
	data        *string
	clientID    string
	mutationID  int64
	hash        []byte
	committedAt time.Time
}

const (
	kindCommit    = "commit"
	kindRejection = "rejection"
	kindPatch     = "patch"
)

type versionAssembler struct {
	versions []*storev1.Version
}

func (a *versionAssembler) add(e entry) error {
	switch e.kind {
	case kindCommit, kindRejection:
		v := &storev1.Version{
			Version:     uint64(e.version),
			ClientId:    e.clientID,
			MutationId:  uint64(e.mutationID),
			Hash:        e.hash,
			CommittedAt: e.committedAt.UnixMilli(),
		}
		if e.kind == kindRejection {
			reason, err := rejectionReason(e.data)
			if err != nil {
				return fmt.Errorf("version %d: %w", e.version, err)
			}
			v.Rejection = &storev1.Rejection{MutationId: uint64(e.mutationID), Reason: reason}
		}
		a.versions = append(a.versions, v)
	case kindPatch:
		if len(a.versions) == 0 || a.versions[len(a.versions)-1].Version != uint64(e.version) {
			return fmt.Errorf("version %d: patch without a header", e.version)
		}
		last := a.versions[len(a.versions)-1]
		last.Changes = append(last.Changes, change(e))
	default:
		return fmt.Errorf("version %d: unknown log kind %q", e.version, e.kind)
	}
	return nil
}

func change(e entry) *storev1.Change {
	c := &storev1.Change{Table: deref(e.table), RowId: deref(e.rowID), Op: storev1.Change_OP_UPSERT}
	if deref(e.op) == "delete" {
		c.Op = storev1.Change_OP_DELETE
	} else if e.data != nil {
		c.Data = []byte(*e.data)
	}
	return c
}

func rejectionReason(data *string) (string, error) {
	if data == nil {
		return "", nil
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(*data), &body); err != nil {
		return "", fmt.Errorf("rejection data: %w", err)
	}
	return body.Reason, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
