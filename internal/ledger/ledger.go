// Package ledger 是 YKC 的唯一事實帳本實作（judge / guard 共用，避免 hash 演算法 drift）。
// append-only JSONL + hash 串鏈；單一寫者、冪等重放、可驗證完整性。
package ledger

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"
)

// Genesis 是鏈頭的初始值。
const Genesis = "GENESIS"

// Fact 是一條不可變原子事實。
type Fact struct {
	Seq      uint64          `json:"seq"`
	Type     string          `json:"type"`
	Actor    string          `json:"actor"`
	Payload  json.RawMessage `json:"payload"`
	PrevHash string          `json:"prev_hash"`
	Hash     string          `json:"hash"`
	Ts       string          `json:"ts,omitempty"` // 寫入時間（RFC3339 UTC）；不參與 hash，向後相容舊帳本
}

// Ledger：單一寫者、append-only。
type Ledger struct {
	mu       sync.Mutex
	f        *os.File
	prevHash string
	seq      uint64
}

// Open 開啟（不存在則建立）帳本並重建鏈頭以接續既有記錄。
func Open(path string) (*Ledger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	l := &Ledger{f: f, prevHash: Genesis, seq: 0}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var fact Fact
		if json.Unmarshal(sc.Bytes(), &fact) == nil {
			if fact.Seq > l.seq {
				l.seq = fact.Seq
			}
			l.prevHash = fact.Hash
		}
	}
	return l, nil
}

// factHash 是帳本唯一的 hash 定義——judge 與 guard 必須一致，故集中於此。
func factHash(prev, typ, actor string, seq uint64, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(prev))
	h.Write([]byte{0x1f})
	h.Write([]byte(typ))
	h.Write([]byte{0x1f})
	h.Write([]byte(actor))
	h.Write([]byte{0x1f})
	h.Write([]byte(strconv.FormatUint(seq, 10)))
	h.Write([]byte{0x1f})
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// Append 追加一條事實，回傳其 Seq。任一步失敗回滾 seq，保證序號連續。
func (l *Ledger) Append(typ, actor string, payload any) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	raw, err := json.Marshal(payload)
	if err != nil {
		l.seq--
		return 0, err
	}
	f := Fact{Seq: l.seq, Type: typ, Actor: actor, Payload: raw, PrevHash: l.prevHash, Ts: time.Now().UTC().Format(time.RFC3339)}
	f.Hash = factHash(f.PrevHash, f.Type, f.Actor, f.Seq, f.Payload)
	line, err := json.Marshal(f)
	if err != nil {
		l.seq--
		return 0, err
	}
	line = append(line, '\n')
	if _, err := l.f.Seek(0, 2); err != nil {
		l.seq--
		return 0, err
	}
	if _, err := l.f.Write(line); err != nil {
		l.seq--
		return 0, err
	}
	if err := l.f.Sync(); err != nil {
		l.seq--
		return 0, err
	}
	l.prevHash = f.Hash
	return f.Seq, nil
}

// Head 回傳目前鏈頭。
func (l *Ledger) Head() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prevHash
}

// Close 關閉帳本。
func (l *Ledger) Close() error { return l.f.Close() }

// VerifyChain 重放整個帳本、重算每條 hash，回傳 (是否未被竄改, 最後 hash)。
func VerifyChain(path string) (bool, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	defer f.Close()

	prev := Genesis
	ok := true
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var fact Fact
		if json.Unmarshal(sc.Bytes(), &fact) != nil {
			ok = false
			continue
		}
		if fact.PrevHash != prev {
			ok = false
		}
		if factHash(fact.PrevHash, fact.Type, fact.Actor, fact.Seq, fact.Payload) != fact.Hash {
			ok = false
		}
		prev = fact.Hash
	}
	return ok, prev, sc.Err()
}

// ReadAll 讀回全部事實（供控制台 / 信任狀態重建）。
func ReadAll(path string) []Fact {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Fact
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var fact Fact
		if json.Unmarshal(sc.Bytes(), &fact) == nil {
			out = append(out, fact)
		}
	}
	return out
}
