// Package ledger 是 YKC 的唯一事實帳本實作（judge / guard / panel / eventledger 共用，
// 避免 hash 演算法 drift）。
//
// 不變式：
//   - append-only JSONL + hash 串鏈（事實不可改，只增）；
//   - 單一寫者：flock 排他鎖（多程序併發時第二個寫者立即失敗，而非交錯寫損鏈）；
//   - 行長有界：超過 MaxLineBytes 即顯式報錯（不再靜默截斷）；
//   - Open/ OpenVerified：開帳本前重放全鏈校驗，並比對獨立 head anchor——防「截斷+重簽末行」偽鏈。
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
	TS       string          `json:"ts,omitempty"` // 寫入時間（RFC3339 UTC）；不參與 hash，向後相容舊帳本
}

// ErrLocked 表示另一 YKC 程序正持有該帳本的寫鎖。
var ErrLocked = errors.New("ledger is locked by another YKC process")

// Ledger 是單一寫者、append-only 的 hash 鏈帳本。
type Ledger struct {
	mu       sync.Mutex
	f        *os.File
	prevHash string
	seq      uint64
}

// Open 開啟（不存在則建立）帳本並重建鏈頭以接續既有記錄。
// 取得排他寫鎖（flock）後，會完整重放 hash chain 並比對獨立 head anchor：
// 若鏈遭竄改、截斷或回滾，任何新寫入一律 fail closed。
func Open(path string) (*Ledger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := acquireLock(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	ok, head, anchor, err := VerifyAnchored(path)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !ok {
		_ = f.Close()
		return nil, fmt.Errorf("ledger verification failed (anchor=%s): tamper or corruption detected", anchor.State)
	}
	return &Ledger{f: f, prevHash: head, seq: anchor.CurrentSeq}, nil
}

// OpenVerified 是 Open 的語義別名。自 T-24 起 Open 本身已強制完整 hash-chain
// 與 head-anchor 校驗，保留此入口以維持既有 judge/guard 呼叫端相容。
func OpenVerified(path string) (*Ledger, error) { return Open(path) }

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
	f := Fact{Seq: l.seq, Type: typ, Actor: actor, Payload: raw, PrevHash: l.prevHash, TS: time.Now().UTC().Format(time.RFC3339)}
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
	// 帳本行已 fsync；接著必須把最新 head 寫入專案外的 HMAC anchor。若這步
	// 失敗，仍回傳已提交 seq 讓呼叫端知道不可盲目重試，但明確報錯而非靜默降級。
	if err := writeHeadAnchor(l.f.Name(), f.Seq, f.Hash); err != nil {
		return f.Seq, fmt.Errorf("ledger append committed but head anchor update failed: %w", err)
	}
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

// chainScan 是一次完整重放的結果。watchSeq 只供 anchor 驗證取得指定歷史序號的
// hash，不另存整條鏈，避免大帳本驗證時的無界記憶體。
type chainScan struct {
	valid       bool
	head        string
	seq         uint64
	watched     bool
	watchedHash string
}

// VerifyChain 重放整個帳本、重算每條 hash 與序號連續性，回傳
// (是否未被竄改, 最後 hash, 錯誤)。只驗證鏈本身；需要同時驗證 project 外
// head anchor 時請用 VerifyAnchored。
func VerifyChain(path string) (bool, string, error) {
	scan, err := scanLedger(path, 0)
	return scan.valid, scan.head, err
}

func scanLedger(path string, watchSeq uint64) (chainScan, error) {
	f, err := os.Open(path)
	if err != nil {
		return chainScan{}, err
	}
	defer f.Close()

	scan := chainScan{valid: true, head: Genesis}
	prev := Genesis
	expectedSeq := uint64(1)
	sc := newLineScanner(f)
	for {
		line, err := sc.next()
		if err != nil {
			return scan, err
		}
		if line == nil {
			break
		}
		var fact Fact
		if json.Unmarshal(line, &fact) != nil {
			scan.valid = false
			continue
		}
		if fact.Seq != expectedSeq {
			scan.valid = false
		}
		if fact.PrevHash != prev {
			scan.valid = false
		}
		if factHash(fact.PrevHash, fact.Type, fact.Actor, fact.Seq, fact.Payload) != fact.Hash {
			scan.valid = false
		}
		if watchSeq != 0 && fact.Seq == watchSeq {
			scan.watched = true
			scan.watchedHash = fact.Hash
		}
		prev = fact.Hash
		scan.head = fact.Hash
		scan.seq = fact.Seq
		expectedSeq++
	}
	return scan, nil
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
