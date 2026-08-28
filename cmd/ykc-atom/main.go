package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"ykc/internal/domain"
	"ykc/internal/enforcement"
	"ykc/internal/eventledger"
	"ykc/internal/guardrail"
	"ykc/internal/monitor"
	"ykc/internal/smoke"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "snapshot":
		err = cmdSnapshot(os.Args[2:])
	case "claim":
		err = cmdClaim(os.Args[2:])
	case "smoke":
		err = cmdSmoke(os.Args[2:])
	case "replay":
		err = cmdReplay(os.Args[2:])
	case "sync-ledger":
		err = cmdSyncLedger(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ykc:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `YKC - YieldKeyCode agent guardrail prototype

Commands:
  snapshot -root . -state .ykc
  claim    -root . -state .ykc -kind tests_passed -text "tests pass" [-smoke]
  smoke    -root . -state .ykc
  replay   -state .ykc
  sync-ledger -root . -state .ykc
`)
}

// requireRoot 邊界加固：workspace root 必須存在且為目錄。
func requireRoot(root string) error {
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("root 目錄不存在: %s", root)
	}
	return nil
}

func cmdSnapshot(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	root := fs.String("root", ".", "workspace root")
	state := fs.String("state", ".ykc", "YKC state directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireRoot(*root); err != nil {
		return err
	}
	snap, err := monitor.Snapshotter{Root: *root}.Capture()
	if err != nil {
		return err
	}
	stateDir := absState(*root, *state)
	if err := snap.WriteAtomic(stateDir); err != nil {
		return err
	}
	bridge, err := eventledger.Open(stateDir, "ykc-atom")
	if err != nil {
		return err
	}
	e, err := domain.NewEnvelope(domain.EventWorkspaceSnapshot, snap.ID, snap.Root, snap)
	if err != nil {
		return err
	}
	if _, err := bridge.Append(e); err != nil {
		return err
	}
	return printJSON(snap)
}

func cmdClaim(args []string) error {
	fs := flag.NewFlagSet("claim", flag.ExitOnError)
	root := fs.String("root", ".", "workspace root")
	state := fs.String("state", ".ykc", "YKC state directory")
	kind := fs.String("kind", "", "claim kind: work_done|tests_passed|build_passed|no_errors")
	text := fs.String("text", "", "claim text")
	runSmoke := fs.Bool("smoke", false, "run smoke takeover commands if decision requests it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireRoot(*root); err != nil {
		return err
	}
	if *kind == "" {
		return fmt.Errorf("-kind is required")
	}
	stateDir := absState(*root, *state)
	bridge, err := eventledger.Open(stateDir, "ykc-atom")
	if err != nil {
		return err
	}
	history, err := bridge.ReplayEvents()
	if err != nil {
		return err
	}
	epoch := domain.LatestSnapshotEpoch(history)
	claim := domain.AgentClaim{Kind: domain.AgentClaimKind(*kind), Text: *text}
	claimEvent, err := domain.NewEnvelope(domain.EventAgentClaim, epoch, mustAbs(*root), claim)
	if err != nil {
		return err
	}
	decision := guardrail.EvaluateClaim(guardrail.StrictPolicy(), history, claimEvent)
	claimAppend, err := bridge.Append(claimEvent)
	claimEvent = claimAppend.Event
	if err != nil {
		return err
	}
	decisionEvent, err := domain.NewEnvelope(domain.EventGuardrailDecision, epoch, mustAbs(*root), decision)
	if err != nil {
		return err
	}
	if _, err := bridge.Append(decisionEvent); err != nil {
		return err
	}
	if err := enforcement.ApplyDecision(stateDir, decision); err != nil {
		return err
	}
	if decision.RunSmoke && *runSmoke {
		report, err := smoke.Runner{FailFast: true}.Run(context.Background(), smoke.DefaultRustSmoke(mustAbs(*root)))
		if err != nil {
			return err
		}
		for _, res := range report.Results {
			cmdEvent, err := domain.NewEnvelope(domain.EventCommandResult, epoch, mustAbs(*root), res)
			if err != nil {
				return err
			}
			if _, err := bridge.Append(cmdEvent); err != nil {
				return err
			}
		}
		if err := enforcement.ApplySmokeReport(stateDir, report); err != nil {
			return err
		}
	}
	return printJSON(struct {
		ClaimEvent domain.Envelope    `json:"claim_event"`
		Decision   guardrail.Decision `json:"decision"`
	}{ClaimEvent: claimEvent, Decision: decision})
}

func cmdSmoke(args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ExitOnError)
	root := fs.String("root", ".", "workspace root")
	state := fs.String("state", ".ykc", "YKC state directory")
	timeout := fs.Duration("timeout", 0, "override timeout per command")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireRoot(*root); err != nil {
		return err
	}
	stateDir := absState(*root, *state)
	bridge, err := eventledger.Open(stateDir, "ykc-atom")
	if err != nil {
		return err
	}
	history, err := bridge.ReplayEvents()
	if err != nil {
		return err
	}
	epoch := domain.LatestSnapshotEpoch(history)
	specs := smoke.DefaultRustSmoke(mustAbs(*root))
	if *timeout > 0 {
		for i := range specs {
			specs[i].Timeout = *timeout
		}
	}
	report, err := smoke.Runner{FailFast: true}.Run(context.Background(), specs)
	if err != nil {
		return err
	}
	for _, res := range report.Results {
		e, err := domain.NewEnvelope(domain.EventCommandResult, epoch, mustAbs(*root), res)
		if err != nil {
			return err
		}
		if _, err := bridge.Append(e); err != nil {
			return err
		}
	}
	if err := enforcement.ApplySmokeReport(stateDir, report); err != nil {
		return err
	}
	return printJSON(report)
}

func cmdReplay(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	state := fs.String("state", ".ykc", "YKC state directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	bridge, err := eventledger.Open(*state, "ykc-atom")
	if err != nil {
		return err
	}
	events, err := bridge.ReplayEvents()
	if err != nil {
		return err
	}
	return printJSON(events)
}

func cmdSyncLedger(args []string) error {
	fs := flag.NewFlagSet("sync-ledger", flag.ExitOnError)
	root := fs.String("root", ".", "workspace root")
	state := fs.String("state", ".ykc", "YKC state directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireRoot(*root); err != nil {
		return err
	}
	stateDir := absState(*root, *state)
	bridge, err := eventledger.Open(stateDir, "ykc-atom-sync")
	if err != nil {
		return err
	}
	res, err := bridge.SyncMissing()
	if err != nil {
		_ = printJSON(res)
		return err
	}
	return printJSON(res)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func absState(root, state string) string {
	if filepath.IsAbs(state) {
		return state
	}
	return filepath.Join(mustAbs(root), state)
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
