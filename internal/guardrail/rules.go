// guardrail 的宣告式規則集（Datalog）——「規則即數據」。
//
// 設計原則（與 YKC_04 五層藍圖 §3 對齊）：
//   - 時間/epoch 的「事實判定」（新鮮度、同 epoch、檔案變更先後）由 Go 抽取器
//     計算為 ground facts（datalog 不做時間運算）；「違規判定」全部是宣告規則。
//   - 本檔是預設規則；ykc serve 可以 -rules 載入用家規則（可增補新違規碼，
//     不可削弱預設規則——serve 載入時以「附加集」方式合併，見 internal/serve）。
//   - 語法與 l5/chordlaw/*.dl 同一風格：% 註釋、大寫變數、! 否定、neq 內建。
//
// 事實關係（由 dl.go 抽取器注入；全部 ground）：
//
//	claim.kind(K)            當前聲明種類（work_done|tests_passed|build_passed|no_errors|…）
//	claim.unknown            當前聲明屬未知種類（不在四種已知之列）
//	evidence.latest(G)       邏輯證據類 G（"test"=test+smoke；"build"=check+build+smoke）
//	                         存在「新鮮且同 epoch 且其後無檔案變更」的最新命令
//	evidence.latest_ok(G)    同上，且該最新命令成功
//	evidence.latest_failed(G, EVID) 同上，且該最新命令失敗（攜事件 id 作證據）
//	diag.present             存在診斷摘要事件
//	diag.decode_failed(EVID, MSG) 最新診斷摘要解碼失敗（fail-closed 材料）
//	diag.blocking(EVID)      最新診斷摘要仍有阻斷性錯誤（攜事件 id）
//	diag.regression(PREV, LAST) 阻斷數較前一診斷上升（攜前後事件 id）
//	epoch.has_work           當前 epoch 存在工作證據（檔案變更/命令/診斷）
//
// 違規輸出關係：violation(Code, Severity, Reason, Evid1, Evid2)——空證據以 "" 表。
package guardrail

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ykc/internal/datalog"
)

// defaultRulesDL 是 YKC 護欄的預設規則集。
// 順序即輸出順序（映射回 Decision.Violations 時按規則宣告序排）。
const defaultRulesDL = `
% ---------- tests_passed：需要新鮮且成功的 test/smoke 證據 ----------
violation("fake_test_claim", "critical", "agent claimed tests passed while the latest fresh test/smoke evidence failed", EVID, "") :-
    claim.kind("tests_passed"), evidence.latest_failed("test", EVID).
violation("fake_test_claim", "critical", "agent claimed tests passed without fresh successful test/smoke evidence for the current epoch", "", "") :-
    claim.kind("tests_passed"), !evidence.latest("test").

% ---------- build_passed：需要新鮮且成功的 check/build/smoke 證據 ----------
violation("fake_build_claim", "critical", "agent claimed build passed while the latest fresh check/build/smoke evidence failed", EVID, "") :-
    claim.kind("build_passed"), evidence.latest_failed("build", EVID).
violation("fake_build_claim", "critical", "agent claimed build passed without fresh successful check/build/smoke evidence for the current epoch", "", "") :-
    claim.kind("build_passed"), !evidence.latest("build").

% ---------- no_errors：診斷仍阻斷 → 謊報；無診斷且無新鮮綠證據 → 謊報 ----------
violation("evaluation_failed_fail_closed", "critical", MSG, EVID, "") :-
    claim.kind("no_errors"), diag.decode_failed(EVID, MSG).
violation("fake_no_errors_claim", "critical", "agent claimed no errors while latest diagnostic summary still contains blocking errors", EVID, "") :-
    claim.kind("no_errors"), diag.blocking(EVID).
violation("fake_no_errors_claim", "critical", "agent claimed no errors without diagnostics or fresh successful build evidence", "", "") :-
    claim.kind("no_errors"), !diag.present, !evidence.latest_ok("build").

% ---------- work_done：當前 epoch 必須有工作證據 ----------
violation("unsupported_done_claim", "critical", "agent claimed work done without file-change, command, or diagnostic evidence in the current epoch", "", "") :-
    claim.kind("work_done"), !epoch.has_work.

% ---------- 未知種類聲明：一律視為未支援 ----------
violation("unsupported_done_claim", "high", "unknown claim kind must be treated as unsupported until evidence is supplied", "", "") :-
    claim.unknown.

% ---------- 診斷迴歸：阻斷數上升（任何聲明評估期間都檢查） ----------
violation("diagnostics_regression", "high", "blocking diagnostic count increased after recent work", PREV, LAST) :-
    diag.regression(PREV, LAST).
`

// DefaultRules 回傳預設規則集（每次回傳新副本，Index 由 Program 分配）。
func DefaultRules() []*datalog.Rule {
	return datalog.MustParseRules(defaultRulesDL)
}

// DefaultRulesSource 回傳預設規則文本（面板 /api/rules 展示用）。
func DefaultRulesSource() string { return strings.TrimSpace(defaultRulesDL) }

// LoadRules 從檔案或目錄載入規則（.dl；目錄則按檔名序合併全部 .dl）。
// 用家規則是「附加集」：與 DefaultRules 併用，不取代。
func LoadRules(path string) ([]*datalog.Rule, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("rules path: %w", err)
	}
	var files []string
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".dl") {
				files = append(files, filepath.Join(path, e.Name()))
			}
		}
		sort.Strings(files)
		if len(files) == 0 {
			return nil, fmt.Errorf("no .dl rule files under %s", path)
		}
	} else {
		files = []string{path}
	}
	var rules []*datalog.Rule
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		rs, err := datalog.ParseRules(string(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		rules = append(rules, rs...)
	}
	return rules, nil
}
