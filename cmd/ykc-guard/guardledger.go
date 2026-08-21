// 帳本存取包裝：把「專案目錄」對映到帳本路徑，集中 .ykc/ledger.jsonl 的位置約定。
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ykc/internal/ledger"
)

func ledgerPath(dir string) string {
	return filepath.Join(dir, ".ykc", "ledger.jsonl")
}

func openLedger(dir string) (*ledger.Ledger, error) {
	_ = os.MkdirAll(filepath.Join(dir, ".ykc"), 0o755)
	return ledger.Open(ledgerPath(dir))
}

func readAll(dir string) []ledger.Fact { return ledger.ReadAll(ledgerPath(dir)) }

// appendFact 追加事實；失敗時醒目告警（裁判完整性不應被靜默破壞）。
func appendFact(led *ledger.Ledger, typ, actor string, payload any) {
	if _, err := led.Append(typ, actor, payload); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 帳本寫入失敗（%s）: %v\n", typ, err)
	}
}
