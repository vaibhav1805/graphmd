package knowledge

import (
	"crypto/md5" //nolint:gosec // MD5 is used for file change detection, not cryptography
	"encoding/hex"
)

// md5Hex returns the hex-encoded MD5 digest of data.
// Intended for content-change detection (not security).
func md5Hex(data []byte) string {
	h := md5.Sum(data) //nolint:gosec
	return hex.EncodeToString(h[:])
}
