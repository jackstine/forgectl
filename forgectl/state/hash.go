package state

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashBytes returns the sha256 hex digest of data. It is used to detect whether
// the reverse engineering queue file changed between QUEUE advances.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
