// 確定性比對器（T-14 核心）：聲明 vs 專案真實狀態。
// 唯一真相來源：二進制 --help、cargo check/test、檔案系統、事實帳本。零 LLM。
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ykc/internal/rustutil"
)

type Project struct{ Dir string }

func (p Project) packageName() string { return rustutil.PackageName(p.Dir) }

func (p Project) binaryPath() string {
	return filepath.Join(p.Dir, "target", "debug", p.packageName())
}

// help 回傳二進制 --help；二進制不存在時回空字串（代表未通過編譯）。
func (p Project) help() string {
	bin := p.binaryPath()
	if _, err := os.Stat(bin); err != nil {
		return ""
	}
	so, _, _ := rustutil.Run(p.Dir, bin, "--help")
	return so
}

func (p Project) subcommands() []string { return rustutil.Subcommands(p.help()) }

func (p Project) compiles() (bool, string) {
	_, se, code := rustutil.Run(p.Dir, "cargo", "check", "--quiet")
	if code == 0 {
		return true, ""
	}
	return false, strings.TrimSpace(se)
}

func (p Project) testsPass() (bool, string) {
	_, se, code := rustutil.Run(p.Dir, "cargo", "test", "--quiet")
	if code == 0 {
		return true, ""
	}
	return false, strings.TrimSpace(se)
}

func (p Project) hasFile(rel string) bool {
	_, err := os.Stat(filepath.Join(p.Dir, rel))
	return err == nil
}

func (p Project) hasFunction(name string) bool {
	found := false
	re := regexp.MustCompile(`\bfn\s+` + regexp.QuoteMeta(name) + `\b`)
	_ = filepath.Walk(p.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if info.IsDir() && info.Name() == "target" {
			return filepath.SkipDir // 跳過編譯產物
		}
		if strings.HasSuffix(path, ".rs") {
			if b, err := os.ReadFile(path); err == nil && re.Match(b) {
				found = true
			}
		}
		return nil
	})
	return found
}

// hasReceipt：sig 是否為「真實簽發的收據」——只認三種來源：
//  1. receipt.json 的 signature / chain_hash 欄位
//  2. 帳本 receipt.issue 事實的 chain_hash
//  3. 帳本中任何事實本身的 hash
//
// 絕不做「子字串比對整個帳本」（否則聲明文字提到簽名會誤判簽名存在）。
func (p Project) hasReceipt(sig string) bool {
	if b, err := os.ReadFile(filepath.Join(p.Dir, ".ykc", "receipt.json")); err == nil {
		var r struct {
			Signature string `json:"signature"`
			ChainHash string `json:"chain_hash"`
		}
		if json.Unmarshal(b, &r) == nil && (r.Signature == sig || r.ChainHash == sig) {
			return true
		}
	}
	for _, f := range readAll(p.Dir) {
		if f.Hash == sig {
			return true
		}
		if f.Type == "receipt.issue" {
			var pl struct {
				ChainHash string `json:"chain_hash"`
			}
			if json.Unmarshal(f.Payload, &pl) == nil && pl.ChainHash == sig {
				return true
			}
		}
	}
	return false
}

// Verify 是確定性比對器：一條聲明 → 三態判決 + 嚴重度。
func (p Project) Verify(c Claim) Verdict {
	v := Verdict{ClaimID: c.ID, Text: c.Text, Feature: c.Feature}
	noBinary := func() bool { return p.help() == "" }

	switch {
	case strings.HasPrefix(c.Feature, "flag:"):
		f := strings.TrimPrefix(c.Feature, "flag:")
		switch {
		case noBinary():
			v.Verdict, v.Evidence = "contradicted", "二進制不存在（未通過編譯），無從談旗標 "+f
		case strings.Contains(p.help(), f):
			v.Verdict, v.Evidence = "verified", "help 輸出含 "+f
		default:
			v.Verdict, v.Evidence = "contradicted", "help 輸出無 "+f
		}

	case strings.HasPrefix(c.Feature, "subcommand:"):
		s := strings.TrimPrefix(c.Feature, "subcommand:")
		switch {
		case noBinary():
			v.Verdict, v.Evidence = "contradicted", "二進制不存在（未通過編譯），無從談子命令 "+s
		case contains(p.subcommands(), s):
			v.Verdict, v.Evidence = "verified", "子命令存在: "+s
		default:
			v.Verdict, v.Evidence = "contradicted", "子命令不存在: "+s
		}

	case strings.HasPrefix(c.Feature, "file:"):
		rel := strings.TrimPrefix(c.Feature, "file:")
		switch {
		case strings.ContainsRune(rel, '\x00') || filepath.IsAbs(rel):
			// 邊界加固：絕對路徑/NULL 字元一律拒絕（不 verified、不讀檔案）
			v.Verdict, v.Evidence = "contradicted", "非法檔案聲明（絕對路徑或 NULL 字元）"
		case !insideProject(p.Dir, rel):
			// 邊界加固：../ 逸出專案目錄的聲明視同虛無（防 path traversal）
			v.Verdict, v.Evidence = "contradicted", "檔案聲明逸出專案目錄（path traversal 被拒絕）"
		case p.hasFile(rel):
			v.Verdict, v.Evidence = "verified", "檔案存在: "+rel
		default:
			v.Verdict, v.Evidence = "contradicted", "檔案不存在: "+rel
		}

	case strings.HasPrefix(c.Feature, "function:"):
		n := strings.TrimPrefix(c.Feature, "function:")
		if p.hasFunction(n) {
			v.Verdict, v.Evidence = "verified", "函式存在: fn "+n
		} else {
			v.Verdict, v.Evidence = "contradicted", "原始碼中無 fn "+n
		}

	case c.Feature == "compiles":
		ok, err := p.compiles()
		if ok {
			v.Verdict, v.Evidence = "verified", "cargo check exit 0"
		} else {
			v.Verdict, v.Evidence = "contradicted", "cargo check 失敗: "+rustutil.FirstLine(err)
		}

	case c.Feature == "tests_pass":
		ok, err := p.testsPass()
		if ok {
			v.Verdict, v.Evidence = "verified", "cargo test exit 0"
		} else {
			v.Verdict, v.Evidence = "contradicted", "cargo test 失敗: "+rustutil.FirstLine(err)
		}

	case strings.HasPrefix(c.Feature, "receipt:"):
		sig := strings.TrimPrefix(c.Feature, "receipt:")
		if p.hasReceipt(sig) {
			v.Verdict, v.Evidence = "verified", "存在真實簽發收據（signature/chain_hash 匹配）"
		} else {
			v.Verdict, v.Evidence = "contradicted", "無真實簽發收據 → 偽造收據"
		}

	default:
		v.Verdict, v.Evidence = "unverifiable", "未知 feature 型別，無法機械驗證"
	}

	if v.Verdict == "contradicted" {
		v.Severity = contradictedSeverity(c.Feature)
	}
	return v
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// insideProject：rel 加入 root 後是否仍在 root 內（Clean 後前綴比對）。
// 邊界加固用：阻擋 `../../etc/passwd` 類路徑逸出。
func insideProject(root, rel string) bool {
	rr := filepath.Clean(root)
	r := filepath.Clean(filepath.Join(rr, filepath.FromSlash(rel)))
	return r == rr || strings.HasPrefix(r, rr+string(os.PathSeparator))
}
