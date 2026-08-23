// 帳本存取包裝：把「專案目錄」對映到 .ykc 狀態位置，集中路徑約定。
//
// 寫入一律走 eventledger bridge（原子事件庫 + hash 鏈投影，D4 修復：
// 消除「guard 直寫舊 ledger」造成的雙來源 drift）；
// 讀取兼容舊（扁平）與新（bridge 信封）兩種事實格式（internal/claimview）。
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ykc/internal/domain"
	"ykc/internal/eventledger"
	"ykc/internal/ledger"
)

func ledgerPath(dir string) string {
	return filepath.Join(dir, ".ykc", "ledger.jsonl")
}

// openBridge 開啟該專案的 bridge（狀態目錄 .ykc），並取得帳本寫鎖。
func openBridge(dir string) (*eventledger.Bridge, error) {
	_ = os.MkdirAll(filepath.Join(dir, ".ykc"), 0o755)
	return eventledger.Open(filepath.Join(dir, ".ykc"), "ykc-guard")
}

// appendTrustEvent 經 bridge 寫入一條信任事實（claim.verdict / trust.event /
// trust.reset）。失敗時醒目告警（裁判完整性不應被靜默破壞）。
func appendTrustEvent(br *eventledger.Bridge, dir string, kind domain.EventKind, sessionID string, payload any) error {
	ev, err := domain.NewEnvelope(kind, sessionID, dir, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 信任事件組裝失敗（%s）: %v\n", kind, err)
		return err
	}
	if _, err := br.Append(ev); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 帳本寫入失敗（%s）: %v\n", kind, err)
		return err
	}
	return nil
}

func mustAppendTrustEvent(br *eventledger.Bridge, dir string, kind domain.EventKind, sessionID string, payload any) {
	if err := appendTrustEvent(br, dir, kind, sessionID, payload); err != nil {
		// Do not compute or print a trust result after its evidence event failed
		// to reach the tamper-evident projection.
		os.Exit(1)
	}
}

// readAll 讀回全部事實（唯讀；不改鎖、不改帳本）。
func readAll(dir string) []ledger.Fact { return ledger.ReadAll(ledgerPath(dir)) }
