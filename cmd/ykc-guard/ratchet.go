// 信任棘輪 + 首擊棘輪（T-15）。
// 原則：行為分 = 嚴重度 × 意圖（不看次數）；首擊即採取消除下次傷害的措施；
// 信任只降不升；回升唯一入口 = 人類 reset（留審計）。
package main

import (
	"encoding/json"

	"ykc/internal/ledger"
)

type TrustLevel int

const (
	T0 TrustLevel = iota // 全面接管
	T1                   // 加強驗證
	T2                   // 常態硬閘門
	T3                   // 高度信任
)

func (t TrustLevel) String() string {
	switch t {
	case T0:
		return "T0 全面接管"
	case T1:
		return "T1 加強驗證"
	case T2:
		return "T2 常態硬閘門"
	default:
		return "T3 高度信任"
	}
}

func (t TrustLevel) MarshalJSON() ([]byte, error) { return json.Marshal(int(t)) }

// intentOf：意圖（第二維）。severity≥2 一律視為存心——因為代理擁有工具可查證，
// 卻仍作出與事實相反的聲明。
func intentOf(sev int) string {
	if sev <= 1 {
		return "無心（幻覺/失誤）"
	}
	return "存心（有計畫繞過驗證）"
}

type RatchetEvent struct {
	ClaimID  string     `json:"claim_id"`
	Severity int        `json:"severity"`
	Kind     string     `json:"kind"`
	Intent   string     `json:"intent"`
	From     TrustLevel `json:"from"`
	To       TrustLevel `json:"to"`
	Action   string     `json:"action"`
}

// applyRatchet：單一判決對信任等級的棘輪作用（只降不升）。
func applyRatchet(from TrustLevel, v Verdict) (TrustLevel, RatchetEvent) {
	ev := RatchetEvent{ClaimID: v.ClaimID, Severity: v.Severity, From: from, To: from}
	if v.Severity == 0 {
		ev.Kind, ev.Intent, ev.Action = "無", "—", "無"
		return from, ev
	}
	ev.Kind = severityKind(v.Severity)
	ev.Intent = intentOf(v.Severity)
	to := from
	switch v.Severity {
	case 1: // 誇大：軟護欄收縮為硬閘門；該類聲明日後一律硬驗證
		if to > T2 {
			to = T2
		}
		ev.Action = "誇大失實→軟護欄收縮為硬閘門；該類聲明日後一律硬驗證"
	case 2: // 隱瞞：加強驗證 + 需求可追溯
		if to > T1 {
			to = T1
		}
		ev.Action = "隱瞞/遺漏→加強驗證：每項聲明須附 receipt + 需求可追溯"
	case 3: // 欺騙（謊報成果）：全面接管
		to = T0
		ev.Action = "謊報成果→全面接管：剝奪敘事權，代理僅可提議編輯"
	case 4: // 偽造：首擊棘輪——物理消滅下次傷害，永不自動回升
		to = T0
		ev.Action = "偽造證據→全面接管+剝奪自我報告權（首擊棘輪）；僅人類可 reset"
	}
	ev.To = to
	return to, ev
}

// ApplyAll：對全部判決依序套用棘輪。
func ApplyAll(from TrustLevel, verdicts []Verdict) (TrustLevel, []RatchetEvent) {
	level := from
	var events []RatchetEvent
	for _, v := range verdicts {
		if v.Severity == 0 {
			continue
		}
		nl, ev := applyRatchet(level, v)
		events = append(events, ev)
		level = nl
	}
	return level, events
}

// currentTrust：從帳本重建某代理的當前信任等級（無紀錄 = T3）。
func currentTrust(facts []ledger.Fact, agentID string) TrustLevel {
	level := T3
	for _, f := range facts {
		switch f.Type {
		case "trust.event":
			var p struct {
				AgentID string `json:"agent_id"`
				To      int    `json:"to"`
			}
			if json.Unmarshal(f.Payload, &p) == nil && p.AgentID == agentID {
				level = TrustLevel(p.To)
			}
		case "trust.reset":
			var p struct {
				AgentID string `json:"agent_id"`
				To      int    `json:"to"`
			}
			if json.Unmarshal(f.Payload, &p) == nil && p.AgentID == agentID {
				level = TrustLevel(p.To)
			}
		}
	}
	return level
}
