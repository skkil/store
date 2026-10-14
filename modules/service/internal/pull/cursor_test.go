package pull

import "testing"

func TestResetReason(t *testing.T) {
	head := Head{Epoch: 2, Version: 10, Hash: []byte("h10")}
	cases := map[string]struct {
		cursor  Cursor
		logHash []byte
		want    string
	}{
		"fresh client":            {Cursor{}, nil, ""},
		"start of current epoch":  {Cursor{Epoch: 2}, nil, ""},
		"matching cursor":         {Cursor{Epoch: 2, Version: 4, Hash: []byte("h4")}, []byte("h4"), ""},
		"at head":                 {Cursor{Epoch: 2, Version: 10, Hash: []byte("h10")}, []byte("h10"), ""},
		"other epoch":             {Cursor{Epoch: 1, Version: 4, Hash: []byte("h4")}, []byte("h4"), ReasonEpochChanged},
		"epoch zero with version": {Cursor{Version: 4, Hash: []byte("h4")}, []byte("h4"), ReasonEpochChanged},
		"ahead of head":           {Cursor{Epoch: 2, Version: 11, Hash: []byte("h11")}, nil, ReasonAheadOfHead},
		"rewritten log":           {Cursor{Epoch: 2, Version: 4, Hash: []byte("old")}, []byte("h4"), ReasonHashMismatch},
		"missing log entry":       {Cursor{Epoch: 2, Version: 4, Hash: []byte("h4")}, nil, ReasonHashMismatch},
		"hash at version zero":    {Cursor{Epoch: 2, Hash: []byte("x")}, nil, ReasonHashMismatch},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := resetReason(head, tc.cursor, tc.logHash); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
