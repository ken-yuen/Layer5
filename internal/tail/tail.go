// Package tail 提供「全量 hash + 尾部保留」的輸出緩衝——YKC 所有子行程
// 輸出處理的唯一實作（internal/smoke 與 internal/sandbox 共用，消除 drift）。
//
// 語意：
//   - 每個寫入字節都進入 SHA-256（收據/指紋用，完整不截斷）；
//   - 僅保留最後 Limit 字節供人類/AI 閱讀（大輸出不爆記憶體）；
//   - Truncated() 明示是否發生截斷。
package tail

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
)

type Buffer struct {
	Limit int
	total int64
	h     hash.Hash
	buf   []byte
}

func NewBuffer(limit int) *Buffer {
	return &Buffer{Limit: limit, h: sha256.New()}
}

func (b *Buffer) Write(p []byte) (int, error) {
	_, _ = b.h.Write(p)
	b.total += int64(len(p))
	if b.Limit <= 0 {
		b.buf = append(b.buf, p...)
		return len(p), nil
	}
	if len(p) >= b.Limit {
		b.buf = append(b.buf[:0], p[len(p)-b.Limit:]...)
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.Limit {
		copy(b.buf, b.buf[len(b.buf)-b.Limit:])
		b.buf = b.buf[:b.Limit]
	}
	return len(p), nil
}

// SumHex 回傳「全部已寫入字節」的 SHA-256（非僅保留尾部）。
func (b *Buffer) SumHex() string { return hex.EncodeToString(b.h.Sum(nil)) }

// Total 回傳總字節數。
func (b *Buffer) Total() int64 { return b.total }

func (b *Buffer) Truncated() bool { return b.Limit > 0 && b.total > int64(len(b.buf)) }

func (b *Buffer) String() string {
	if b.Truncated() {
		return fmt.Sprintf("...<truncated; kept last %d of %d bytes>\n%s", len(b.buf), b.total, string(b.buf))
	}
	return string(b.buf)
}
