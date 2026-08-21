# YKC 架構整合與效能設計 — 五層一體、受控外殼
### （v1.0 ｜ 2026-08-21 ｜ 回答「分開容器會否阻礙整合 / 如何保留全整合空間又逼近 Docker 效能」）

> 一句話答案：**5 層永遠住在同一個進程，以「窄介面 + 共享事實帳本 + 進程內事件匯流排」整合；只有「執行不可信代碼」這一個受控外殼會跨出進程，且它是 runtime swap（換旗標）而非部署拆解。因此整合空間完整保留、效能逼近甚至超越容器。** 分開容器是給「可重現」與「安全邊界」用的，與 5 層整合完全正交。

---

## 1. 先糾正一個會害死專案的誤解：信任邊界 ≠ 層級邊界

我前一輪的 `deploy/` 設計，容器角色是這樣（**沒有**「L1 一容器、L2 一容器」這回事）：

| 容器/沙盒 | 用途 | 屬於哪層？ |
|---|---|---|
| `ykc` 裁判鏡像（YKC 二進制 + 鎖版 Rust 工具鏈） | **整個 L1–L5 都在這一個進程裡跑** | 全部層 |
| `ykc-toolchain` 工具鏈鏡像 | 只含 rustc/cargo，供沙盒內 build 用 | 不屬任何層，是 L4 的「耗材」 |
| gVisor/Firecracker 沙盒 | 隔離**不可信代碼**（build script/proc-macro）執行 | 不是層，是 L4/L5 的執行襯底 |

**真正需要分離的，只有「可信 vs 不可信」這一條線：**
- 可信（YKC 自己）：L1–L5 全部邏輯、事實帳本、比對器——**在一個進程內，共享記憶體**。
- 不可信（代理產生的代碼、cargo build script、測試二進制）——**才需要跨進程 + 沙盒**。

這條線與「層」無關。所以「層級不相容」其實是「**信任需求不同**」，而信任需求只由「這段代碼是不是代理寫的」決定，不由「這是第幾層」決定。

---

## 2. 一圖構圖：五層一體 + 兩個受控外殼

```
┌─────────────────────────── YKC 單一進程（可信，goroutine 並行）───────────────────────────┐
│                                                                                          │
│   ┌────┐  ┌────┐  ┌────┐  ┌────┐  ┌────┐                                                │
│   │ L1 │  │ L2 │  │ L3 │  │ L4 │  │ L5 │    ← 五個 Go package，窄介面，進程內接線           │
│   └──┬─┘  └──┬─┘  └──┬─┘  └──┬─┘  └──┬─┘                                                │
│      └───────┴───────┴───┬───┴───────┘                                                  │
│                          ▼                                                               │
│              ┌────────────────────────┐        ┌──────────────────────┐                  │
│              │ 事實帳本 Fact Ledger     │◄──────►│ 進程內事件匯流排 Bus   │  (channel/NATS) │
│              │ (記憶體 + mmap bbolt)    │        └──────────────────────┘                  │
│              └───────────┬────────────┘                                                   │
│                          │ 讀=零拷貝、寫=單一寫者                                          │
│                          ▼                                                               │
│              ┌────────────────────────────────────────────┐                              │
│              │            Executor（唯一受控外殼）          │  ← 效能開關 perf dial         │
│              │  ExecTrusted ──► 直接子行程（近原生）          │                              │
│              │  ExecUntrusted ──► runsc/gVisor/Firecracker  │                              │
│              └──────────────┬─────────────────────────────┘                              │
└─────────────────────────────┼────────────────────────────────────────────────────────────┘
                              │ 只有「不可信代碼執行」才跨出進程
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
  rust-analyzer          cargo build/check       test 二進制
  （長駐協進程，常開）      （沙盒，~50ms 冷啟動）    （沙盒）
```

---

## 3. 我的實現方法：模組化單體（Modular Monolith）

### 3.1 五層 = 五個 Go package，窄介面，進程內接線

每一層只暴露一個**窄介面**，由核心在**同一進程**內組裝（已寫成可編譯骨架 `ykc-core/interfaces.go`）：

```go
// 五層窄介面（節錄）
type DependencyChecker interface {           // L1
    Check(ctx, manifest []byte) (DependencyReport, error)
}
type StructureScanner interface {            // L2
    Scan(ctx, files []string) (StructureReport, error)
}
type GuardrailEngine interface {             // L3
    Observe(facts <-chan Fact)               // 訂閱事實流
    Evaluate(ctx, f Fact) (GuardAction, error)
}
type DebugEngine interface {                 // L4
    CheckAndFix(ctx, project string) (Receipt, error)
}
type BorrowAnalyzer interface {              // L5
    Analyze(ctx, factsPath string) (BorrowGraph, error)
}

// 核心：進程內組裝
type Core struct {
    Ledger Ledger
    Bus    Bus
    L1 DependencyChecker
    L2 StructureScanner
    L3 GuardrailEngine
    L4 DebugEngine
    L5 BorrowAnalyzer
    Exec  Executor      // 唯一出進程的受控外殼
}
```

### 3.2 共享基板：事實帳本 = 全整合空間的「匯流排」

5 層**不直接互相呼叫**，而是全部讀寫**同一個事實帳本**：
- L4 寫 `crate.compile` → L3 訂閱到 → 觸發護欄 → 寫 `guard.action` → 用家控制台讀到。
- L2 寫 `struct.scan` → L4 用同一個符號表做除錯提示 → L5 用同一批 borrow 事實畫節點圖。

**進程內 = 指標級共享、零序列化**。這正是「全整合」的實體：**層與層之間沒有 IPC、沒有 JSON 序列化、沒有網路往返**——這是任何「一層一容器」永遠做不到的效能上限，也是你要的「核彈效能」的來源。

### 3.3 解耦與整合如何兼得

- **解耦**：每層只依賴窄介面 + 帳本，任一一層可獨立替換（換 L2 的統計演算法，不動其他）。
- **整合**：因為共用帳本與匯流排，任何一層的產出即刻被其他層消費。
- 秘訣：**「窄介面」負責解耦，「共享基板」負責整合**。兩者不衝突。

### 3.4 唯一出進程的受控外殼：Executor + 效能開關

跨進程只有**兩類子行程**，都收斂到一個 `Executor` 介面：

| 子行程 | 生命週期 | 效能特徵 |
|---|---|---|
| rust-analyzer（LSP 協進程） | **起一次、常駐**，stdio JSON-RPC | 無冷啟動；增量 flycheck |
| cargo/rustc/測試二進制 | 每次 build 起，但走 **runtime swap** | `ExecTrusted`≈原生；`ExecUntrusted`=runsc ~50ms 冷啟動 |

```go
type ExecMode int
const (
    ExecTrusted          ExecMode = iota  // 直接子行程，近原生（Docker 級甚至更快）
    ExecUntrustedSandbox                  // runsc/gVisor/Firecracker，同程式碼路徑換旗標
)
type Executor interface {
    Exec(ctx, mode ExecMode, cmd string, args ...string) (stdout, stderr []byte, exit int)
}
```

**關鍵**：這不是「兩套部署」，是**同一個 Executor 的一枚旗標**。效能與安全不是二選一，是**一個旋鈕（perf dial）**：
- 內測/己方代碼 → `ExecTrusted`（近原生，甚至比 `docker run` 更快，因為無 daemon、無 bridge 網路）。
- 代理/第三方代碼 → `ExecUntrustedSandbox`（runsc 子行程，~50ms，無 docker daemon 往返）。

---

## 4. 「核彈效能」到底靠什麼（誠實的答案：不是靠容器）

容器從來不是效能來源，它是**隔離手段**。真正的效能來自六件事，全在進程內實現：

1. **共享記憶體事實帳本**：層間零拷貝、零序列化（容器方案每層要過一次 JSON-RPC + 網路）。
2. **goroutine 並行**：每檔案分析是獨立任務，`errgroup` 並行 + 冪等合併。
3. **增量分析**：tree-sitter 增量解析、只重分析「髒檔案」、rust-analyzer flycheck 增量診斷——**不是每次全量重掃**。
4. **快取三件套**：`target/` 重用、`CARGO_HOME` 重用、錯誤指紋去重（同一錯誤不重複送 LLM）。
5. **長駐協進程**：rust-analyzer 起一次用整天，不每次冷啟動。
6. **邊界最小化**：不可信代碼用 runsc 子行程（無 daemon、無 bridge、無 registry 往返），可信路徑直接子行程。

**對照結論**：模組化單體在「層間通訊」上**必然快於**任何多容器微服務（省掉每層一次的序列化+網路）；在「不可信代碼執行」上與 Docker 打平或更優（runsc 子行程 vs docker daemon 往返）。所以「Docker 差不多效能」這個目標，單體架構**穩贏**，不是差不多。

---

## 5. 分開容器到底留不留？——留，但只留給它該管的

| 部署產物 | 管什麼 | 與整合的關係 |
|---|---|---|
| `ykc` 裁判鏡像 | 可重現（rust-toolchain.toml 鎖版）| 無關——5 層在裡面 |
| gVisor/Firecracker | 不可信代碼安全邊界 | 無關——是 Executor 的 runtime 選項 |
| K8s Deployment | 雲端橫向擴展 | 無關——擴的是「裁判實例」，不是「層」 |

**未來若要真正橫向擴展**（多租戶 SaaS）：擴展的單位是**整個 YKC 裁判實例**（每租戶一個裁判），而不是拆層。5 層的窄介面已就位，哪天真的需要「L4 獨立成服務」時，把 `DebugEngine` 介面以 gRPC 外露即可——**介面不動、內部不重寫**。這就是「留低全整合空間供未來使用」的具體保證：**先單體，後按需拆（monolith-first, scale-out-later）**，反過來（先微服務）才是整合噩夢。

---

## 6. 測量（否則「核彈」只是形容詞）

| 指標 | 目標（P50） | 對照 |
|---|---|---|
| 增量編譯循環（改一檔→check→receipt） | < 3s | 對照全量冷 build |
| 收據產出率 | > 20 receipts/s（並行） | — |
| 層間事實讀取 | 微秒級（進程內 mmap） | 對照 gRPC 毫秒級 |
| 不可信 build 冷啟動 | ~50ms（runsc） | 對照 docker run ~1.2s |
| 裁判記憶體 | < 300MB（含工具鏈 sidecar） | — |

每個里程碑在 CI 跑這張表，數字進 `YKC_00_構圖與路線圖.md` 的 D 表。

---

## 7. 決策紀錄追加

| # | 事項 | 決定 |
|---|---|---|
| **D13** | **架構整合** | 5 層 = 進程內模組化單體（窄介面 + 共享事實帳本 + 進程內事件匯流排）；信任邊界 ≠ 層級邊界；容器只用於「可重現」與「不可信代碼沙盒」；跨進程只經唯一 Executor（perf dial：Trusted/Untrusted 一旗切換）；先單體後按需拆 |

---

*配套實作骨架：`ykc-core/interfaces.go`（已編譯驗證）——五層窄介面、共享帳本、進程內匯流排、唯一受控外殼 Executor 的 Go 定義，是「全整合空間」的可執行證明。*
