// dl.go：護欄的 Datalog 評估路徑——事實抽取（Go）× 違規判定（規則）。
//
// 遷移紀律（YKC「唯一實作」原則）：
//   - EvaluateClaim 自此是 EvaluateClaimWithRules(…, nil) 的薄包裝；
//     全部違規判定只存在於規則文本 rules.go（單一真相源，無第二實作）。
//   - 時間/epoch/新鮮度屬「事實計算」，留在 Go 抽取器（datalog 不做時間運算）；
//     語意上等價於舊 evidenceState 邏輯（buildEvidenceState 原樣保留共用）。
//   - 規則求值失敗 = 裁判故障 → fail-closed 接管（與解碼失敗同等對待）。

package guardrail

import (
	"sort"
	"time"

	"ykc/internal/datalog"
	"ykc/internal/domain"
)

// 證據邏輯組：聲明種類 → 可採納的命令類別組（與舊實作一一對應）。
var evidenceGroups = map[string][]domain.CommandClass{
	"test":  {domain.CommandClassTest, domain.CommandClassSmoke},
	"build": {domain.CommandClassCheck, domain.CommandClassBuild, domain.CommandClassSmoke},
}

// knownClaimKinds 是已知聲明種類（未知種類 → claim.unknown 事實）。
var knownClaimKinds = map[domain.AgentClaimKind]bool{
	domain.ClaimWorkDone:    true,
	domain.ClaimTestsPassed: true,
	domain.ClaimBuildPassed: true,
	domain.ClaimNoErrors:    true,
}

// EvaluateClaimWithRules 以「預設規則 + 附加規則」評估聲明。
// extraRules 用於 ykc serve 的 -rules 附加集（只能增補，不能削弱預設規則）。
func EvaluateClaimWithRules(policy Policy, history []domain.Envelope, claimEvent domain.Envelope, extraRules []*datalog.Rule) Decision {
	now := time.Now().UTC()
	if policy.EvidenceFreshness <= 0 {
		policy = StrictPolicy()
	}
	decision := Decision{At: now, Mode: "observe"}
	claim, err := domain.DecodePayload[domain.AgentClaim](claimEvent)
	if err != nil {
		return failClosed(now, "cannot decode agent claim: "+err.Error())
	}
	decision.ClaimKind = string(claim.Kind)
	state, err := buildEvidenceState(history)
	if err != nil {
		return failClosed(now, "cannot build evidence state: "+err.Error())
	}
	prog := datalog.NewProgram()
	if err := injectFacts(prog, state, claimEvent, claim, policy, now); err != nil {
		return failClosed(now, "cannot materialize guardrail facts: "+err.Error())
	}
	rules := DefaultRules()
	rules = append(rules, extraRules...)
	if err := prog.AddRules(rules); err != nil {
		return failClosed(now, "invalid guardrail rules: "+err.Error())
	}
	derived, err := prog.Eval()
	if err != nil {
		return failClosed(now, "guardrail rule evaluation failed: "+err.Error())
	}
	decision.Violations = violationsFromDerived(derived)
	applyActions(&decision)
	return decision
}

// injectFacts 把證據狀態物化為 ground facts（語意與舊 switch 完全等價）。
func injectFacts(p *datalog.Program, state evidenceState, claimEvent domain.Envelope, claim domain.AgentClaim, policy Policy, now time.Time) error {
	// ---- 聲明 ----
	if err := p.AddFact("claim.kind", string(claim.Kind)); err != nil {
		return err
	}
	if !knownClaimKinds[claim.Kind] {
		if err := p.AddFact("claim.unknown"); err != nil {
			return err
		}
	}
	// ---- 命令證據（新鮮度 + 同 epoch + 其後無檔案變更） ----
	fresh := func(e domain.Envelope) bool {
		return !e.At.IsZero() && now.Sub(e.At) <= policy.EvidenceFreshness && (claimEvent.Epoch == "" || e.Epoch == claimEvent.Epoch)
	}
	for group, classes := range evidenceGroups {
		var best domain.Envelope
		var bestResult domain.CommandResult
		found := false
		for _, class := range classes {
			e, ok := state.lastCommandByClass[class]
			if !ok || !fresh(e) || state.changedAfter(e.At) {
				continue
			}
			cr, err := domain.DecodePayload[domain.CommandResult](e)
			if err != nil {
				continue
			}
			if !found || e.At.After(best.At) {
				found = true
				best = e
				bestResult = cr
			}
		}
		if !found {
			continue
		}
		if err := p.AddFact("evidence.latest", group); err != nil {
			return err
		}
		if bestResult.Succeeded() {
			if err := p.AddFact("evidence.latest_ok", group); err != nil {
				return err
			}
		} else if err := p.AddFact("evidence.latest_failed", group, best.ID); err != nil {
			return err
		}
	}
	// ---- 診斷 ----
	if state.lastDiagnostic != nil {
		if err := p.AddFact("diag.present"); err != nil {
			return err
		}
		diag, derr := domain.DecodePayload[domain.DiagnosticSummary](*state.lastDiagnostic)
		if derr != nil {
			if err := p.AddFact("diag.decode_failed", state.lastDiagnostic.ID, "cannot decode latest diagnostics: "+derr.Error()); err != nil {
				return err
			}
		} else {
			if diag.HasBlockingErrors() {
				if err := p.AddFact("diag.blocking", state.lastDiagnostic.ID); err != nil {
					return err
				}
			}
			if state.prevDiagnostic != nil {
				prev, perr := domain.DecodePayload[domain.DiagnosticSummary](*state.prevDiagnostic)
				if perr == nil && diag.BuildBlockingCount > prev.BuildBlockingCount {
					if err := p.AddFact("diag.regression", state.prevDiagnostic.ID, state.lastDiagnostic.ID); err != nil {
						return err
					}
				}
			}
		}
	}
	// ---- epoch 工作證據 ----
	if state.hasCurrentEpochWork(claimEvent.Epoch) {
		if err := p.AddFact("epoch.has_work"); err != nil {
			return err
		}
	}
	return nil
}

// violationsFromDerived 把 violation/5 事實映射回 Violation（按規則宣告序穩定排序）。
func violationsFromDerived(derived []datalog.DerivedFact) []Violation {
	type pair struct {
		ruleIdx int
		v       Violation
	}
	var pairs []pair
	for _, d := range derived {
		if d.Pred != "violation" || len(d.Tuple) != 5 {
			continue
		}
		pairs = append(pairs, pair{ruleIdx: d.RuleIdx, v: Violation{
			Code:     ViolationCode(d.Tuple[0]),
			Severity: Severity(d.Tuple[1]),
			Reason:   d.Tuple[2],
			Evidence: compact([]string{d.Tuple[3], d.Tuple[4]}),
		}})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].ruleIdx != pairs[j].ruleIdx {
			return pairs[i].ruleIdx < pairs[j].ruleIdx
		}
		a, b := pairs[i].v, pairs[j].v
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Reason != b.Reason {
			return a.Reason < b.Reason
		}
		return len(a.Evidence) < len(b.Evidence)
	})
	out := make([]Violation, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.v)
	}
	return out
}
