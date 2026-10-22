package writepath

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
)

// DeriveID returns the n-th row id (counting from 0) created by mutation
// mutationID of client: uuidv8(sha256(client ‖ mutation_id ‖ n)) with the
// client as its 16 raw bytes, the mutation id as a big-endian uint64 and n as
// a big-endian uint32. It equals derive_id in store-core and DerivedIds in
// the JVM adapter, so a server replaying a mutation creates the device's ids.
func DeriveID(client uuid.UUID, mutationID uint64, n uint32) uuid.UUID {
	var input [28]byte
	copy(input[:16], client[:])
	binary.BigEndian.PutUint64(input[16:24], mutationID)
	binary.BigEndian.PutUint32(input[24:], n)
	digest := sha256.Sum256(input[:])
	var id uuid.UUID
	copy(id[:], digest[:16])
	id[6] = id[6]&0x0f | 0x80
	id[8] = id[8]&0x3f | 0x80
	return id
}
