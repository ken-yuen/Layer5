package eventledger

import (
	"os"
	"testing"
)

// TestMain 把 anchor 目錄隔離到臨時路徑——測試絕不可讀寫真實的
// $YKC_HOME/anchors（宿主信任根）；否則一次 go test 就會污染甚至
// rotate 掉真實的 ledger head anchor（YKC_25 審計 D2 教訓）。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ykc-eventledger-test-anchors")
	if err != nil {
		panic("eventledger test: mkdirtemp anchors: " + err.Error())
	}
	if err := os.Setenv("YKC_ANCHOR_DIR", dir); err != nil {
		panic("eventledger test: setenv: " + err.Error())
	}
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		panic("eventledger test: cleanup anchors: " + err.Error())
	}
	os.Exit(code)
}
