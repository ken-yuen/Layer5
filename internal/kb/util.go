package kb

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashString 回傳字串的 sha256 十六進位（前 16 字元）。
func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}
