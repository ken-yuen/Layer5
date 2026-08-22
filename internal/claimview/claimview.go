// Package claimview 把事實帳本中的 claim.verdict / trust.event / trust.reset
// 事實解碼為正規化視圖——同時相容「舊格式（扁平 payload）」與「新格式
// （eventledger bridge 信封）」，供 ykc-guard 控制台與 ykc-panel 共用
// 同一份解碼邏輯（單一實作，消除讀端 drift）。
package claimview

import (
	"encoding/json"

	"ykc/internal/eventledger"
	"ykc/internal/ledger"
)

// 三種受管理的信任事實型別（base 名稱）。
const (
	KindClaimVerdict = "claim.verdict"
	KindTrustEvent   = "trust.event"
	KindTrustReset   = "trust.reset"
)

// 新格式（bridge）事實型別 = eventledger.FactPrefix + base。
const (
	BridgeClaimVerdict = eventledger.FactPrefix + KindClaimVerdict
	BridgeTrustEvent   = eventledger.FactPrefix + KindTrustEvent
	BridgeTrustReset   = eventledger.FactPrefix + KindTrustReset
)

// Fact 是一條信任事實的正規化視圖。
type Fact struct {
	Kind    string // KindClaimVerdict | KindTrustEvent | KindTrustReset
	AgentID string
	// claim.verdict
	ClaimID  string
	Text     string
	Feature  string
	Verdict  string
	Evidence string
	Severity int
	// trust.event
	KindLabel string // 誇大/隱瞞/欺騙/偽造
	Intent    string
	Action    string
	From      int
	To        int
	// trust.reset
	Reason string
}

// Parse 解碼單一條帳本事實；非三種已知型別回傳 (_, false)。
func Parse(f ledger.Fact) (Fact, bool) {
	var base string
	switch f.Type {
	case KindClaimVerdict, BridgeClaimVerdict:
		base = KindClaimVerdict
	case KindTrustEvent, BridgeTrustEvent:
		base = KindTrustEvent
	case KindTrustReset, BridgeTrustReset:
		base = KindTrustReset
	default:
		return Fact{}, false
	}
	payload := innerPayload(f, base)
	if payload == nil {
		return Fact{}, false
	}
	switch base {
	case KindClaimVerdict:
		var v struct {
			AgentID  string `json:"agent_id"`
			ClaimID  string `json:"claim_id"`
			Text     string `json:"text"`
			Feature  string `json:"feature"`
			Verdict  string `json:"verdict"`
			Evidence string `json:"evidence"`
			Severity int    `json:"severity"`
		}
		if err := json.Unmarshal(payload, &v); err != nil || v.AgentID == "" {
			return Fact{}, false
		}
		return Fact{
			Kind: base, AgentID: v.AgentID, ClaimID: v.ClaimID, Text: v.Text,
			Feature: v.Feature, Verdict: v.Verdict, Evidence: v.Evidence, Severity: v.Severity,
		}, true
	case KindTrustEvent:
		var v struct {
			AgentID  string `json:"agent_id"`
			ClaimID  string `json:"claim_id"`
			Kind     string `json:"kind"`
			Intent   string `json:"intent"`
			From     int    `json:"from"`
			To       int    `json:"to"`
			Action   string `json:"action"`
			Severity int    `json:"severity"`
		}
		if err := json.Unmarshal(payload, &v); err != nil || v.AgentID == "" {
			return Fact{}, false
		}
		return Fact{
			Kind: base, AgentID: v.AgentID, ClaimID: v.ClaimID,
			KindLabel: v.Kind, Intent: v.Intent, From: v.From, To: v.To, Action: v.Action,
			Severity: v.Severity,
		}, true
	case KindTrustReset:
		var v struct {
			AgentID string `json:"agent_id"`
			To      int    `json:"to"`
			Reason  string `json:"reason"`
		}
		if err := json.Unmarshal(payload, &v); err != nil || v.AgentID == "" {
			return Fact{}, false
		}
		return Fact{Kind: base, AgentID: v.AgentID, To: v.To, Reason: v.Reason}, true
	}
	return Fact{}, false
}

// TrustLevel 依序重放事實、回傳某代理的當前信任等級（無紀錄 = 傳入的 default）。
// 舊格式與 bridge 格式混合出現也正確（以帳本順序最後一條為準）。
func TrustLevel(facts []ledger.Fact, agentID string, def int) int {
	level := def
	for _, f := range facts {
		v, ok := Parse(f)
		if !ok || v.AgentID != agentID {
			continue
		}
		if v.Kind == KindTrustEvent || v.Kind == KindTrustReset {
			level = v.To
		}
	}
	return level
}

// innerPayload 從舊（扁平）或新（bridge 信封）格式取出內層 payload。
func innerPayload(f ledger.Fact, base string) json.RawMessage {
	switch f.Type {
	case KindClaimVerdict, KindTrustEvent, KindTrustReset:
		return f.Payload
	default: // bridge 格式
		var bp eventledger.BridgePayload
		if err := json.Unmarshal(f.Payload, &bp); err != nil {
			return nil
		}
		if len(bp.Envelope.Payload) == 0 {
			return nil
		}
		return bp.Envelope.Payload
	}
}
