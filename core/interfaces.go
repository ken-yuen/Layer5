// Package core 定義 YKC 的「全整合空間」：五層窄介面 + 共享基板 + 唯一受控外殼。
//
// 設計意圖（見 YKC_02_架構整合與效能設計.md）：
//   - 五層(L1–L5)全部住在同一個 YKC 進程，以窄介面接線；
//   - 層與層之間不直接呼叫，而是讀寫同一個事實帳本(共享基板)；
//   - 跨出進程的只有 Executor（執行不可信代碼），以 ExecMode 一旗切換
//     「直接子行程(近原生)」或「gVisor/Firecracker 沙盒」。
//   - 未來若要橫向擴展，把任一個介面以 gRPC/MCP 外露即可，介面不動、內部不重寫。
package core

import "context"

// ---------- 共享基板：事實帳本（進程內 = 零序列化讀取） ----------

// Fact 是一條不可變的原子事實，hash 串鏈防竄改。
type Fact struct {
	Seq      uint64 // 單調遞增序號
	Type     string // file.edit / crate.compile / test.run / agent.claim / guard.action ...
	Actor    string
	Payload  []byte
	PrevHash string
	Hash     string
}

// FactFilter 供查詢過濾。
type FactFilter struct {
	Types []string
	Actor string
	Since uint64
}

// FactReader 唯讀視圖（任意並行讀，零鎖競爭讀）。
type FactReader interface {
	Read(seq uint64) (*Fact, error)
	Query(f FactFilter) ([]Fact, error)
}

// FactWriter 單一寫者（原子 append，只增不改）。
type FactWriter interface {
	Append(f *Fact) (uint64, error)
}

// Ledger 是 FactReader+FactWriter 的合體——五層共享的唯一基板。
type Ledger interface {
	FactReader
	FactWriter
}

// ---------- 進程內事件匯流排 ----------

// Event 是層間的非同步通知（事實仍以 Ledger 為準，Event 只作觸發）。
type Event struct {
	Type   string
	Source string // 例如 "L4"
	Seq    uint64 // 關聯的 Fact 序號
}

// Bus 進程內 pub-sub（goroutine channel 實現，零網路）。
type Bus interface {
	Publish(e Event)
	Subscribe(types []string) <-chan Event
}

// ---------- 五層窄介面 ----------

// DependencyChecker 是 L1：依賴版本對齊與衝突風險預警。
type DependencyChecker interface {
	Check(ctx context.Context, manifest []byte) (DependencyReport, error)
}

// StructureScanner 是 L2：遍歷專案結構並統計。
type StructureScanner interface {
	Scan(ctx context.Context, files []string) (StructureReport, error)
}

// GuardrailEngine 是 L3：原子監聽 + 動態護欄（訂閱事實流，產出護欄動作）。
type GuardrailEngine interface {
	Observe(facts <-chan Fact)
	Evaluate(ctx context.Context, f Fact) (GuardAction, error)
}

// DebugEngine 是 L4：沙盒預編譯除錯（產生簽名收據）。
type DebugEngine interface {
	CheckAndFix(ctx context.Context, project string) (Receipt, error)
}

// BorrowAnalyzer 是 L5：borrow/生命週期代數仿構（datalog + 節點圖）。
type BorrowAnalyzer interface {
	Analyze(ctx context.Context, factsPath string) (BorrowGraph, error)
}

// ---------- 回報資料型別（骨架，細節後補） ----------

type DependencyReport struct{ Summary string }
type StructureReport struct{ Summary string }
type GuardAction struct {
	Level string // warn | block | takeover
	Note  string
}
type Receipt struct {
	Overall   string
	ChainHash string
	Signature string
}
type BorrowGraph struct {
	Nodes int
	Edges int
}

// ---------- 進程內組裝（模組化單體） ----------

// Core 把五層在單一進程內接線。這是「全整合空間」的實體。
type Core struct {
	Ledger Ledger
	Bus    Bus
	L1     DependencyChecker
	L2     StructureScanner
	L3     GuardrailEngine
	L4     DebugEngine
	L5     BorrowAnalyzer
	Exec   Executor // 唯一出進程的受控外殼
}

// ---------- 唯一受控外殼：Executor（效能開關 perf dial） ----------

// ExecMode 決定「執行不可信代碼」走哪條路。同一程式碼路徑，只換旗標。
type ExecMode int

const (
	// ExecTrusted 直接子行程：近原生，Docker 級甚至更快（無 daemon/bridge/registry 往返）。
	ExecTrusted ExecMode = iota
	// ExecUntrustedSandbox 經 gVisor(runsc)/Firecracker：安全邊界，~50ms 冷啟動。
	ExecUntrustedSandbox
)

// Executor 是 YKC 唯一允許跨出進程的介面。
// 所有 cargo/rustc/test 執行都經此，由 mode 決定沙盒化與否。
type Executor interface {
	Exec(ctx context.Context, mode ExecMode, cmd string, args ...string) (stdout, stderr []byte, exitCode int)
}
