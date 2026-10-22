package writepath

import (
	"testing"

	"github.com/google/uuid"
)

func TestDeriveIDMatchesTheStoreCoreVectors(t *testing.T) {
	client := uuid.MustParse("01923e4c-8b1a-7c3d-9e2f-0a1b2c3d4e5f")
	vectors := []struct {
		mutation uint64
		n        uint32
		want     string
	}{
		{1, 0, "7c95d028-8ca6-8ddf-b6a6-888317f17cb1"},
		{1, 1, "ae99a64d-15cf-8772-be5b-3c463e449794"},
		{42, 3, "12f46dfd-c2c8-87d8-a894-9c145fa02861"},
	}
	for _, v := range vectors {
		got := DeriveID(client, v.mutation, v.n)
		if got.String() != v.want {
			t.Errorf("DeriveID(m%d, %d) = %s, want %s", v.mutation, v.n, got, v.want)
		}
		if got.Version() != 8 || got.Variant() != uuid.RFC4122 {
			t.Errorf("DeriveID(m%d, %d) is version %d variant %s", v.mutation, v.n, got.Version(), got.Variant())
		}
	}
}

func TestStatusNames(t *testing.T) {
	for status, want := range map[Status]string{Applied: "applied", Duplicate: "duplicate", Rejected: "rejected", 0: "unknown"} {
		if status.String() != want {
			t.Errorf("%d.String() = %q, want %q", status, status.String(), want)
		}
	}
}
