package enforcement

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"ykc/internal/atomicfile"
	"ykc/internal/guardrail"
	"ykc/internal/smoke"
)

type State struct {
	UpdatedAt   time.Time             `json:"updated_at"`
	Mode        string                `json:"mode"`
	BlockWrites bool                  `json:"block_writes"`
	RunSmoke    bool                  `json:"run_smoke"`
	Violations  []guardrail.Violation `json:"violations,omitempty"`
	Reason      string                `json:"reason,omitempty"`
}

func ApplyDecision(stateDir string, decision guardrail.Decision) error {
	state := State{
		UpdatedAt:   time.Now().UTC(),
		Mode:        decision.Mode,
		BlockWrites: decision.BlockWrites,
		RunSmoke:    decision.RunSmoke,
		Violations:  decision.Violations,
	}
	if err := writeState(stateDir, state); err != nil {
		return err
	}
	if decision.BlockWrites {
		return atomicfile.WriteFileSync(blockPath(stateDir), []byte(blockText(decision)), 0o644)
	}
	_ = os.Remove(blockPath(stateDir))
	return nil
}

func ApplySmokeReport(stateDir string, report smoke.Report) error {
	state := State{UpdatedAt: time.Now().UTC()}
	if report.Passed {
		state.Mode = "observe"
		state.BlockWrites = false
		state.RunSmoke = false
		state.Reason = "smoke evidence passed; write block cleared"
		if err := writeState(stateDir, state); err != nil {
			return err
		}
		_ = os.Remove(blockPath(stateDir))
		return nil
	}
	state.Mode = "smoke_failed_blocked"
	state.BlockWrites = true
	state.RunSmoke = true
	state.Reason = "smoke evidence failed; write block remains active"
	if err := writeState(stateDir, state); err != nil {
		return err
	}
	return atomicfile.WriteFileSync(blockPath(stateDir), []byte(state.Reason+"\n"), 0o644)
}

func writeState(stateDir string, state State) error {
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFileSync(filepath.Join(stateDir, "enforcement", "state.json"), append(b, '\n'), 0o644)
}

func blockPath(stateDir string) string {
	return filepath.Join(stateDir, "enforcement", "AGENT_WRITES_BLOCKED")
}

func blockText(decision guardrail.Decision) string {
	b, _ := json.MarshalIndent(decision, "", "  ")
	return "YKC has blocked agent writes until fresh smoke evidence is available.\n" + string(b) + "\n"
}
