// Package ledger 是 YKC 的唯一事實帳本實作（judge / guard / panel / eventledger 共用，
// 避免 hash 演算法 drift）。
//
// 不變式：
//   - append-only JSONL + hash 串鏈（事實不可改，只增）；
//   - 單一寫者：flock 排他鎖（多程序併發時第二個寫者立即失敗，而非交錯寫損鏈）；
//   - 行長有界：超過 MaxLineBytes 即顯式報錯（不再靜默截斷）；
//   - OpenVerified：開帳本前重放全鏈校驗——防「截斷+重簽末行」偽鏈。
package ledger

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"
)

// Genesis 是鏈頭的初始值。
const Genesis = "GENESIS"

// MaxLineBytes 是單行（單條事實）的最大允許長度；超過即視為損毀/異常。
const MaxLineBytes = 16 * 1024 * 1024

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

// ErrLocked 表示另一 YKC 程序正持有該帳本的寫鎖。
var ErrLocked = errors.New("ledger is locked by another YKC process")

// Ledger：單一寫者、append-only。
type Ledger struct {
	mu       sync.Mutex
	f        *os.File
	prevHash string
	seq      uint64
}

// Open 開啟（不存在則建立）帳本並重建鏈頭以接續既有記錄。
// 取得排他寫鎖（flock）：若另一程序已持有，回傳 ErrLocked。
func Open(path string) (*Ledger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := acquireLock(f); err != nil {
		f.Close()
		return nil, err
	}
	l := &Ledger{f: f, prevHash: Genesis, seq: 0}
	sc := newLineScanner(f)
	for {
		line, err := sc.next()
		if err != nil {
			f.Close()
			return nil, err
		}
		if line == nil {
			break
		}
		var fact Fact
		if json.Unmarshal(line, &fact) == nil {
			if fact.Seq > l.seq {
				l.seq = fact.Seq
			}
			l.prevHash = fact.Hash
		}
	}
	return l, nil
}

// OpenVerified 是「校驗式開帳本」：Open 之後重放全鏈、逐條重算 hash。
// 偵測到任何竄改/損毀即失敗——judge/guard 的寫入路徑一律走此入口，
// 確保不會在偽鏈/斷鏈之上繼續追加。
func OpenVerified(path string) (*Ledger, error) {
	l, err := Open(path)
	if err != nil {
		return nil, err
	}
	ok, _, verr := VerifyChain(path)
	if verr != nil {
		l.Close()
		return nil, verr
	}
	if !ok {
		l.Close()
		return nil, errors.New("ledger chain verification failed: tamper or corruption detected")
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
	if len(raw) > MaxLineBytes {
		l.seq--
		return 0, fmt.Errorf("payload %d bytes exceeds ledger line limit %d", len(raw), MaxLineBytes)
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

// Path 回傳帳本檔案路徑。
func (l *Ledger) Path() string { return l.f.Name() }

// Head 回傳目前鏈頭。
func (l *Ledger) Head() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prevHash
}

// Close 關閉帳本（同時釋放寫鎖）。
func (l *Ledger) Close() error { return l.f.Close() }

// VerifyChain 重放整個帳本、重算每條 hash，回傳 (是否未被竄改, 最後 hash, 錯誤)。
func VerifyChain(path string) (bool, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	defer f.Close()

	prev := Genesis
	ok := true
	sc := newLineScanner(f)
	for {
		line, err := sc.next()
		if err != nil {
			return false, prev, err
		}
		if line == nil {
			break
		}
		var fact Fact
		if json.Unmarshal(line, &fact) != nil {
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
	return ok, prev, nil
}

// ReadAll 讀回全部事實（供控制台 / 信任狀態重建）。
func ReadAll(path string) []Fact {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Fact
	sc := newLineScanner(f)
	for {
		line, err := sc.next()
		if err != nil {
			break
		}
		if line == nil {
			break
		}
		var fact Fact
		if json.Unmarshal(line, &fact) == nil {
			out = append(out, fact)
		}
	}
	return out
}

// lineScanner 是「行長有界」的逐行讀取器：
// 舊版用 bufio.Scanner（1MB 上限、超限靜默報錯）；改手動 ReadBytes + 顯式上限，
// 超限回傳錯誤而非吞行。
type lineScanner struct {
	r *bufio.Reader
}

func newLineScanner(f *os.File) *lineScanner {
	return &lineScanner{r: bufio.NewReaderSize(f, 256*1024)}
}

// next 回傳 (行內容[不含換行], nil)；(nil, nil) 表示 EOF；(_, err) 表示損毀。
func (s *lineScanner) next() ([]byte, error) {
	line, err := s.r.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("ledger read: %w", err)
	}
	if errors.Is(err, io.EOF) && len(line) == 0 {
		return nil, nil // 乾淨 EOF
	}
	if len(line) > MaxLineBytes {
		return nil, fmt.Errorf("ledger line exceeds %d bytes (corrupt or oversized fact)", MaxLineBytes)
	}
	// ReadBytes 在 EOF 前讀到最後一行（無結尾換行）也回傳該行——同樣接受。
	return trimCR(line), nil
}

func trimCR(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
		if m := len(b); m > 0 && b[m-1] == '\r' {
			b = b[:m-1]
		}
	}
	return b
}
